package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	log.Println("🔄 Starting cleanup of duplicate strategy fields in traders table...")

	// Check database file
	dbPath := "data/data.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Fatalf("❌ Database file does not exist: %s", dbPath)
	}

	// Backup database
	backupPath := fmt.Sprintf("%s.pre_cleanup_backup", dbPath)
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

	// Get all traders with strategy_id
	rows, err := db.Query(`
		SELECT id, name, strategy_id
		FROM traders
		WHERE strategy_id IS NOT NULL AND strategy_id != ''
	`)
	if err != nil {
		log.Fatalf("❌ Failed to query traders: %v", err)
	}
	defer rows.Close()

	var traders []struct {
		ID         string
		Name       string
		StrategyID string
	}

	for rows.Next() {
		var trader struct {
			ID         string
			Name       string
			StrategyID string
		}
		if err := rows.Scan(&trader.ID, &trader.Name, &trader.StrategyID); err != nil {
			log.Fatalf("❌ Failed to scan trader: %v", err)
		}
		traders = append(traders, trader)
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("❌ Error iterating traders: %v", err)
	}

	log.Printf("📊 Found %d traders with strategy_id to clean up", len(traders))

	if len(traders) == 0 {
		log.Println("✅ No traders with strategy_id found - nothing to clean up")
		return
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("❌ Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	updatedCount := 0
	errorCount := 0

	// Clean up each trader
	for _, trader := range traders {
		// Clear all strategy-related fields when strategy_id is set
		// These fields should be NULL/default since they'll be loaded from the strategy
		_, err := tx.Exec(`
			UPDATE traders SET
				btc_eth_leverage = 0,
				altcoin_leverage = 0,
				trading_symbols = '',
				custom_prompt = '',
				override_base_prompt = 0,
				system_prompt_template = '',
				is_cross_margin = 1,
				use_coin_pool = 0,
				use_oi_top = 0,
				use_tradingview = 0,
				enable_raw_klines = 1,
				enable_ema = 0,
				enable_macd = 0,
				enable_rsi = 0,
				enable_atr = 0,
				enable_volume = 1,
				enable_oi = 1,
				enable_funding = 1,
				indicator_timeframe = '',
				quant_data_url = '',
				updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND strategy_id = ?
		`, trader.ID, trader.StrategyID)

		if err != nil {
			log.Printf("❌ Failed to update trader %s (%s): %v", trader.ID, trader.Name, err)
			errorCount++
		} else {
			log.Printf("✅ Cleaned up trader: %s (%s) - strategy_id: %s", trader.ID, trader.Name, trader.StrategyID)
			updatedCount++
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		log.Fatalf("❌ Failed to commit transaction: %v", err)
	}

	log.Println("")
	log.Printf("✅ Cleanup completed!")
	log.Printf("   Updated: %d traders", updatedCount)
	if errorCount > 0 {
		log.Printf("   Errors: %d traders", errorCount)
	}
	log.Printf("📝 Original data backed up at: %s", backupPath)
	log.Println("⚠️  Please verify system functionality before manually deleting backup file")
	log.Println("")
	log.Println("💡 Strategy Studio is now the single source of truth for all strategy-related settings")
	log.Println("💡 Traders with strategy_id will load settings from their associated strategy")
}

