//go:build ignore
// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"nofx/config"
	"os"
	"path/filepath"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run backup_equity_history.go <database_path> [output_file]")
		fmt.Println("Example: go run backup_equity_history.go data/data.db backup_equity_history_20250101.json")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	outputFile := ""
	if len(os.Args) >= 3 {
		outputFile = os.Args[2]
	} else {
		// Generate default filename with timestamp
		timestamp := time.Now().Format("20060102_150405")
		outputFile = fmt.Sprintf("backup_equity_history_%s.json", timestamp)
	}

	log.Printf("📋 Opening database: %s", dbPath)
	database, err := config.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("❌ Failed to open database: %v", err)
	}
	defer database.Close()

	// Verify database health
	if err := database.VerifyDatabaseHealth(); err != nil {
		log.Fatalf("❌ Database health check failed: %v", err)
	}
	log.Printf("✅ Database health check passed")

	// Get all traders with equity history
	traderIDs, err := database.GetAllTradersWithEquityHistory()
	if err != nil {
		log.Fatalf("❌ Failed to get traders with equity history: %v", err)
	}

	if len(traderIDs) == 0 {
		log.Printf("⚠️  No traders with equity history found")
		os.Exit(0)
	}

	log.Printf("📊 Found %d trader(s) with equity history", len(traderIDs))

	// Backup data structure
	backup := map[string]interface{}{
		"backup_timestamp": time.Now().Format(time.RFC3339),
		"database_path":    dbPath,
		"trader_count":     len(traderIDs),
		"traders":          make(map[string]interface{}),
	}

	totalRecords := 0

	// Backup equity history for each trader
	for _, traderID := range traderIDs {
		log.Printf("📥 Backing up equity history for trader: %s", traderID)
		
		// Get all equity history (no limit for backup)
		history, err := database.GetEquityHistory(traderID, 100000)
		if err != nil {
			log.Printf("⚠️  Failed to get equity history for trader %s: %v", traderID, err)
			continue
		}

		// Convert to JSON-serializable format
		historyData := make([]map[string]interface{}, 0, len(history))
		for _, rec := range history {
			historyData = append(historyData, map[string]interface{}{
				"timestamp":         rec.Timestamp.Format("2006-01-02 15:04:05"),
				"total_equity":      rec.TotalEquity,
				"available_balance": rec.AvailableBalance,
				"total_pnl":         rec.TotalPnL,
				"total_pnl_pct":     rec.TotalPnLPct,
				"position_count":    rec.PositionCount,
				"margin_used_pct":   rec.MarginUsedPct,
				"cycle_number":      rec.CycleNumber,
			})
		}

		tradersMap := backup["traders"].(map[string]interface{})
		tradersMap[traderID] = map[string]interface{}{
			"trader_id":      traderID,
			"record_count":   len(historyData),
			"equity_history": historyData,
		}

		totalRecords += len(historyData)
		log.Printf("  ✓ Backed up %d records for trader %s", len(historyData), traderID)
	}

	backup["total_records"] = totalRecords

	// Write backup to file
	log.Printf("💾 Writing backup to: %s", outputFile)
	
	// Ensure output directory exists
	outputDir := filepath.Dir(outputFile)
	if outputDir != "." && outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			log.Fatalf("❌ Failed to create output directory: %v", err)
		}
	}

	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		log.Fatalf("❌ Failed to marshal backup data: %v", err)
	}

	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		log.Fatalf("❌ Failed to write backup file: %v", err)
	}

	log.Printf("✅ Backup completed successfully!")
	log.Printf("   File: %s", outputFile)
	log.Printf("   Traders: %d", len(traderIDs))
	log.Printf("   Total records: %d", totalRecords)
	log.Printf("   File size: %.2f KB", float64(len(data))/1024)
}

