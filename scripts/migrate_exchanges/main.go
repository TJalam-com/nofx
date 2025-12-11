package main

import (
	"log"
	"nofx/config"
	"os"
)

func main() {
	log.Println("🔄 Starting exchanges table migration...")

	// Check database file
	dbPath := "data/data.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Fatalf("❌ Database file does not exist: %s", dbPath)
	}

	// Initialize database (this will trigger migration on startup)
	log.Printf("📂 Opening database: %s", dbPath)
	database, err := config.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("❌ Failed to open database: %v", err)
	}
	defer database.Close()

	log.Println("✅ Database opened successfully")
	log.Println("✅ Migration check completed - if migration was needed, it has been applied")
	log.Println("")
	log.Println("💡 You can verify the migration by checking the server logs when you restart the server")
	log.Println("💡 Look for: '✅ Exchanges table migration check completed successfully'")
}

