//go:build ignore
// +build ignore

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"nofx/config"
	"nofx/logger"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	dbPath := flag.String("db", "data/data.db", "Database path")
	logsDir := flag.String("logs", "decision_logs", "Decision logs directory")
	dryRun := flag.Bool("dry-run", false, "Dry run mode (don't actually save to database)")
	flag.Parse()

	log.Printf("📋 Migrating decision logs to database")
	log.Printf("   Database: %s", *dbPath)
	log.Printf("   Logs directory: %s", *logsDir)
	if *dryRun {
		log.Printf("   Mode: DRY RUN (no changes will be made)")
	}

	// Open database
	database, err := config.NewDatabase(*dbPath)
	if err != nil {
		log.Fatalf("❌ Failed to open database: %v", err)
	}
	defer database.Close()

	// Verify database health
	if err := database.VerifyDatabaseHealth(); err != nil {
		log.Fatalf("❌ Database health check failed: %v", err)
	}
	log.Printf("✅ Database health check passed")

	// Check if logs directory exists
	if _, err := os.Stat(*logsDir); os.IsNotExist(err) {
		log.Fatalf("❌ Logs directory does not exist: %s", *logsDir)
	}

	// Get all trader directories
	entries, err := ioutil.ReadDir(*logsDir)
	if err != nil {
		log.Fatalf("❌ Failed to read logs directory: %v", err)
	}

	totalMigrated := 0
	totalSkipped := 0
	totalErrors := 0

	// Process each trader directory
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		traderID := entry.Name()
		traderLogDir := filepath.Join(*logsDir, traderID)

		log.Printf("📥 Processing trader: %s", traderID)

		// Check if trader exists in database
		traderRecord, err := database.GetTraderByID(traderID)
		if err != nil {
			log.Printf("⚠️  Trader %s not found in database, skipping", traderID)
			totalSkipped++
			continue
		}

		// Check existing equity history count
		existingCount, err := database.GetEquityHistoryCount(traderID)
		if err == nil && existingCount > 0 {
			log.Printf("  ℹ️  Trader %s already has %d equity history records in database", traderID, existingCount)
		}

		// Read all decision log files
		files, err := ioutil.ReadDir(traderLogDir)
		if err != nil {
			log.Printf("⚠️  Failed to read trader log directory %s: %v", traderLogDir, err)
			totalErrors++
			continue
		}

		// Create decision logger to read files
		decisionLogger := logger.NewDecisionLogger(traderLogDir)

		// Get all records (large limit to get all)
		records, err := decisionLogger.GetLatestRecords(100000)
		if err != nil {
			log.Printf("⚠️  Failed to get decision records for trader %s: %v", traderID, err)
			totalErrors++
			continue
		}

		if len(records) == 0 {
			log.Printf("  ℹ️  No decision records found for trader %s", traderID)
			continue
		}

		log.Printf("  📊 Found %d decision records for trader %s", len(records), traderID)

		// Convert to equity history records
		// We'll save them individually since EquityHistoryRecord is not exported
		initialBalance := traderRecord.InitialBalance
		if initialBalance <= 0 {
			// Try to calculate from first record if initial balance not set
			if len(records) > 0 {
				firstRecord := records[0]
				totalEquity := firstRecord.AccountState.TotalBalance + firstRecord.AccountState.TotalUnrealizedProfit
				totalPnL := firstRecord.AccountState.TotalUnrealizedProfit
				initialBalance = totalEquity - totalPnL
				if initialBalance > 0 {
					log.Printf("  ℹ️  Calculated initial balance from first record: %.2f", initialBalance)
				}
			}
		}

		// Check for duplicates - only migrate records that don't exist
		existingHistory, err := database.GetEquityHistory(traderID, 100000)
		if err != nil {
			log.Printf("⚠️  Failed to get existing history for trader %s: %v", traderID, err)
		}

		// Create map of existing records by timestamp and cycle number
		existingMap := make(map[string]bool)
		for _, existing := range existingHistory {
			key := fmt.Sprintf("%s_%d", existing.Timestamp.Format("2006-01-02 15:04:05"), existing.CycleNumber)
			existingMap[key] = true
		}

		// Filter out duplicates and prepare records for saving
		newRecordsCount := 0
		for _, record := range records {
			// Check if record already exists
			key := fmt.Sprintf("%s_%d", record.Timestamp.Format("2006-01-02 15:04:05"), record.CycleNumber)
			if existingMap[key] {
				continue
			}

			// Calculate equity and PnL
			totalEquity := record.AccountState.TotalBalance + record.AccountState.TotalUnrealizedProfit
			totalPnL := totalEquity - initialBalance
			totalPnLPct := 0.0
			if initialBalance > 0 {
				totalPnLPct = (totalPnL / initialBalance) * 100
			}

			if *dryRun {
				newRecordsCount++
				continue
			}

			// Save individual record (SaveEquityHistory handles retries)
			err := database.SaveEquityHistory(
				traderID,
				record.Timestamp,
				totalEquity,
				record.AccountState.AvailableBalance,
				totalPnL,
				totalPnLPct,
				record.AccountState.PositionCount,
				record.AccountState.MarginUsedPct,
				record.CycleNumber,
			)
			if err != nil {
				log.Printf("❌ Failed to save record for trader %s (cycle %d): %v", traderID, record.CycleNumber, err)
				totalErrors++
				continue
			}
			newRecordsCount++
		}

		if *dryRun {
			log.Printf("  🔍 DRY RUN: Would migrate %d new equity history records for trader %s", newRecordsCount, traderID)
			totalMigrated += newRecordsCount
			continue
		}

		if newRecordsCount == 0 {
			log.Printf("  ℹ️  All records already exist in database for trader %s", traderID)
			continue
		}

		log.Printf("  ✅ Completed migration for trader %s: %d records migrated", traderID, newRecordsCount)
		totalMigrated += newRecordsCount
	}

	log.Printf("")
	log.Printf("📊 Migration Summary:")
	log.Printf("   Total migrated: %d records", totalMigrated)
	log.Printf("   Total skipped: %d traders", totalSkipped)
	log.Printf("   Total errors: %d", totalErrors)
	
	if *dryRun {
		log.Printf("")
		log.Printf("ℹ️  This was a DRY RUN. No changes were made.")
		log.Printf("   Run without -dry-run flag to perform actual migration.")
	} else {
		log.Printf("")
		log.Printf("✅ Migration completed successfully!")
	}
}

