package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	_ "modernc.org/sqlite"
)

func main() {
	log.Println("🔄 Starting article slugs migration...")

	// Check database file
	dbPath := "data/data.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Fatalf("❌ Database file does not exist: %s", dbPath)
	}

	// Backup database
	backupPath := fmt.Sprintf("%s.pre_slug_migration_backup", dbPath)
	log.Printf("📦 Backing up database to: %s", backupPath)

	input, err := os.ReadFile(dbPath)
	if err != nil {
		log.Fatalf("❌ Failed to read database: %v", err)
	}

	if err := os.WriteFile(backupPath, input, 0600); err != nil {
		log.Fatalf("❌ Backup failed: %v", err)
	}
	log.Println("✅ Backup created successfully")

	// Open database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("❌ Failed to open database: %v", err)
	}
	defer db.Close()

	// Get all articles
	rows, err := db.Query(`
		SELECT id, slug, title
		FROM articles
		ORDER BY created_at ASC
	`)
	if err != nil {
		log.Fatalf("❌ Failed to query articles: %v", err)
	}
	defer rows.Close()

	var articles []struct {
		ID    string
		Slug  string
		Title string
	}

	for rows.Next() {
		var article struct {
			ID    string
			Slug  string
			Title string
		}
		if err := rows.Scan(&article.ID, &article.Slug, &article.Title); err != nil {
			log.Fatalf("❌ Failed to scan article: %v", err)
		}
		articles = append(articles, article)
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("❌ Error iterating articles: %v", err)
	}

	log.Printf("📊 Found %d articles to migrate", len(articles))

	if len(articles) == 0 {
		log.Println("✅ No articles to migrate")
		return
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("❌ Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	updatedCount := 0
	skippedCount := 0

	// Process each article
	for _, article := range articles {
		// Generate sanitized slug from existing slug (or title if slug is empty)
		source := article.Slug
		if source == "" {
			source = article.Title
		}

		newSlug := generateSlug(source)

		// If slug hasn't changed, skip
		if newSlug == article.Slug {
			log.Printf("ℹ️  Article '%s' (ID: %s) - slug already valid, skipping", article.Title, article.ID)
			skippedCount++
			continue
		}

		// Ensure slug is unique
		uniqueSlug, err := ensureUniqueSlug(tx, newSlug, article.ID)
		if err != nil {
			log.Fatalf("❌ Failed to ensure unique slug for article %s: %v", article.ID, err)
		}

		// Update slug
		_, err = tx.Exec(`
			UPDATE articles
			SET slug = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, uniqueSlug, article.ID)
		if err != nil {
			log.Fatalf("❌ Failed to update slug for article %s: %v", article.ID, err)
		}

		if uniqueSlug != newSlug {
			log.Printf("✅ Article '%s' (ID: %s) - slug updated: '%s' -> '%s' (made unique)", article.Title, article.ID, article.Slug, uniqueSlug)
		} else {
			log.Printf("✅ Article '%s' (ID: %s) - slug updated: '%s' -> '%s'", article.Title, article.ID, article.Slug, uniqueSlug)
		}
		updatedCount++
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		log.Fatalf("❌ Failed to commit transaction: %v", err)
	}

	log.Println("")
	log.Printf("✅ Migration completed successfully!")
	log.Printf("📊 Updated: %d articles", updatedCount)
	log.Printf("📊 Skipped: %d articles (already valid)", skippedCount)
	log.Printf("📝 Original data backed up at: %s", backupPath)
	log.Println("⚠️  Please verify system functionality before manually deleting backup file")
}

// generateSlug generates a URL-friendly slug from a title
func generateSlug(title string) string {
	// Convert to lowercase
	slug := strings.ToLower(title)

	// Replace spaces and underscores with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")

	// Remove special characters, keep only alphanumeric and hyphens
	reg := regexp.MustCompile(`[^a-z0-9\-]`)
	slug = reg.ReplaceAllString(slug, "")

	// Remove multiple consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")

	// Remove leading and trailing hyphens
	slug = strings.Trim(slug, "-")

	// Ensure slug is not empty
	if slug == "" {
		slug = "article"
	}

	return slug
}

// ensureUniqueSlug ensures slug is unique by appending a number if needed
func ensureUniqueSlug(tx *sql.Tx, baseSlug string, excludeID string) (string, error) {
	slug := baseSlug
	counter := 1

	for {
		exists, err := checkSlugExists(tx, slug, excludeID)
		if err != nil {
			return "", err
		}
		if !exists {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", baseSlug, counter)
		counter++
		if counter > 1000 {
			return "", fmt.Errorf("unable to generate unique slug")
		}
	}
}

// checkSlugExists check if slug exists (excluding given article ID)
func checkSlugExists(tx *sql.Tx, slug string, excludeID string) (bool, error) {
	var count int
	var err error
	if excludeID != "" {
		err = tx.QueryRow(`SELECT COUNT(*) FROM articles WHERE slug = ? AND id != ?`, slug, excludeID).Scan(&count)
	} else {
		err = tx.QueryRow(`SELECT COUNT(*) FROM articles WHERE slug = ?`, slug).Scan(&count)
	}
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
