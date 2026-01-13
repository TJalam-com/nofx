package api

import (
	"fmt"
	"net/http"
	"nofx/config"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

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
func (s *Server) ensureUniqueSlug(baseSlug string, excludeID string) (string, error) {
	slug := baseSlug
	counter := 1

	for {
		exists, err := s.database.CheckSlugExists(slug, excludeID)
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

// handleGetArticles get all articles (admin only, with pagination)
func (s *Server) handleGetArticles(c *gin.Context) {
	userID, _ := c.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	status := c.Query("status") // "draft", "published", or empty for all
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	articles, err := s.database.GetArticles(status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get articles: %v", err)})
		return
	}

	// Enrich articles with author email
	articlesWithAuthor := make([]*config.Article, len(articles))
	for i, article := range articles {
		articlesWithAuthor[i] = article
		if user, err := s.database.GetUserByID(article.AuthorID); err == nil {
			articlesWithAuthor[i].AuthorEmail = user.Email
		}
	}

	c.JSON(http.StatusOK, gin.H{"articles": articlesWithAuthor})
}

// handleGetArticle get article by ID (admin only)
func (s *Server) handleGetArticle(c *gin.Context) {
	userID, _ := c.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id := c.Param("id")
	article, err := s.database.GetArticleByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
		return
	}

	// Enrich article with author email
	if user, err := s.database.GetUserByID(article.AuthorID); err == nil {
		article.AuthorEmail = user.Email
	}

	c.JSON(http.StatusOK, article)
}

// handleCreateArticle create new article (admin only)
func (s *Server) handleCreateArticle(c *gin.Context) {
	userID, _ := c.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Title            string `json:"title" binding:"required"`
		Content          string `json:"content" binding:"required"`
		Excerpt          string `json:"excerpt"`
		FeaturedImageURL string `json:"featured_image_url"`
		Status           string `json:"status"` // "draft" or "published"
		MetaTitle        string `json:"meta_title"`
		MetaDescription  string `json:"meta_description"`
		MetaKeywords     string `json:"meta_keywords"`
		OGImageURL       string `json:"og_image_url"`
		Slug             string `json:"slug"` // Optional, auto-generated if not provided
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate status
	if req.Status != "" && req.Status != "draft" && req.Status != "published" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Status must be 'draft' or 'published'"})
		return
	}
	if req.Status == "" {
		req.Status = "draft"
	}

	// Generate slug if not provided
	slug := req.Slug
	if slug == "" {
		slug = generateSlug(req.Title)
	}

	// Ensure slug is unique
	slug, err := s.ensureUniqueSlug(slug, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate unique slug: %v", err)})
		return
	}

	// Set published_at if publishing
	var publishedAt *time.Time
	if req.Status == "published" {
		now := time.Now().UTC()
		publishedAt = &now
	}

	article := &config.Article{
		ID:               uuid.New().String(),
		Slug:             slug,
		Title:            req.Title,
		Content:          req.Content,
		Excerpt:          req.Excerpt,
		FeaturedImageURL: req.FeaturedImageURL,
		AuthorID:         userID.(string),
		Status:           req.Status,
		MetaTitle:        req.MetaTitle,
		MetaDescription:  req.MetaDescription,
		MetaKeywords:     req.MetaKeywords,
		OGImageURL:       req.OGImageURL,
		PublishedAt:      publishedAt,
	}

	if err := s.database.CreateArticle(article); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create article: %v", err)})
		return
	}

	// Enrich article with author email
	if user, err := s.database.GetUserByID(article.AuthorID); err == nil {
		article.AuthorEmail = user.Email
	}

	c.JSON(http.StatusCreated, article)
}

// handleUpdateArticle update article (admin only)
func (s *Server) handleUpdateArticle(c *gin.Context) {
	userID, _ := c.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id := c.Param("id")
	existingArticle, err := s.database.GetArticleByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
		return
	}

	var req struct {
		Title            string `json:"title"`
		Content          string `json:"content"`
		Excerpt          string `json:"excerpt"`
		FeaturedImageURL string `json:"featured_image_url"`
		Status           string `json:"status"`
		MetaTitle        string `json:"meta_title"`
		MetaDescription  string `json:"meta_description"`
		MetaKeywords     string `json:"meta_keywords"`
		OGImageURL       string `json:"og_image_url"`
		Slug             string `json:"slug"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update fields if provided
	if req.Title != "" {
		existingArticle.Title = req.Title
	}
	if req.Content != "" {
		existingArticle.Content = req.Content
	}
	if req.Excerpt != "" {
		existingArticle.Excerpt = req.Excerpt
	}
	if req.FeaturedImageURL != "" {
		existingArticle.FeaturedImageURL = req.FeaturedImageURL
	}
	if req.Status != "" {
		if req.Status != "draft" && req.Status != "published" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Status must be 'draft' or 'published'"})
			return
		}
		existingArticle.Status = req.Status
		// Update published_at when status is changed
		if req.Status == "published" && existingArticle.PublishedAt == nil {
			now := time.Now().UTC()
			existingArticle.PublishedAt = &now
		} else if req.Status == "draft" {
			existingArticle.PublishedAt = nil
		}
	}
	if req.MetaTitle != "" {
		existingArticle.MetaTitle = req.MetaTitle
	}
	if req.MetaDescription != "" {
		existingArticle.MetaDescription = req.MetaDescription
	}
	if req.MetaKeywords != "" {
		existingArticle.MetaKeywords = req.MetaKeywords
	}
	if req.OGImageURL != "" {
		existingArticle.OGImageURL = req.OGImageURL
	}

	// Slug cannot be changed after article creation - it remains as originally set
	// This prevents breaking existing URLs and maintains SEO consistency

	if err := s.database.UpdateArticle(existingArticle); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update article: %v", err)})
		return
	}

	// Enrich article with author email
	if user, err := s.database.GetUserByID(existingArticle.AuthorID); err == nil {
		existingArticle.AuthorEmail = user.Email
	}

	c.JSON(http.StatusOK, existingArticle)
}

// handleDeleteArticle delete article (admin only)
func (s *Server) handleDeleteArticle(c *gin.Context) {
	userID, _ := c.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id := c.Param("id")
	if err := s.database.DeleteArticle(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete article: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Article deleted successfully"})
}

// handlePublishArticle publish draft article (admin only)
func (s *Server) handlePublishArticle(c *gin.Context) {
	userID, _ := c.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id := c.Param("id")
	article, err := s.database.GetArticleByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
		return
	}

	article.Status = "published"
	now := time.Now().UTC()
	article.PublishedAt = &now

	if err := s.database.UpdateArticle(article); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to publish article: %v", err)})
		return
	}

	// Enrich article with author email
	if user, err := s.database.GetUserByID(article.AuthorID); err == nil {
		article.AuthorEmail = user.Email
	}

	c.JSON(http.StatusOK, article)
}

// handleUnpublishArticle unpublish article (admin only)
func (s *Server) handleUnpublishArticle(c *gin.Context) {
	userID, _ := c.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id := c.Param("id")
	article, err := s.database.GetArticleByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
		return
	}

	article.Status = "draft"
	article.PublishedAt = nil

	if err := s.database.UpdateArticle(article); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to unpublish article: %v", err)})
		return
	}

	// Enrich article with author email
	if user, err := s.database.GetUserByID(article.AuthorID); err == nil {
		article.AuthorEmail = user.Email
	}

	c.JSON(http.StatusOK, article)
}

// handleGetPublishedArticles list published articles (public, paginated)
func (s *Server) handleGetPublishedArticles(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	articles, err := s.database.GetPublishedArticles(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get articles: %v", err)})
		return
	}

	// Enrich articles with author email
	articlesWithAuthor := make([]*config.Article, len(articles))
	for i, article := range articles {
		articlesWithAuthor[i] = article
		if user, err := s.database.GetUserByID(article.AuthorID); err == nil {
			articlesWithAuthor[i].AuthorEmail = user.Email
		}
	}

	c.JSON(http.StatusOK, gin.H{"articles": articlesWithAuthor})
}

// handleGetArticleBySlug get published article by slug (public)
func (s *Server) handleGetArticleBySlug(c *gin.Context) {
	slug := c.Param("slug")
	article, err := s.database.GetArticleBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
		return
	}

	// Only return published articles to public
	if article.Status != "published" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
		return
	}

	// Enrich article with author email
	if user, err := s.database.GetUserByID(article.AuthorID); err == nil {
		article.AuthorEmail = user.Email
	}

	c.JSON(http.StatusOK, article)
}
