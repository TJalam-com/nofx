package config

import (
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"nofx/crypto"
	"nofx/market"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DatabaseInterface defines the set of methods that database implementations need to provide
type DatabaseInterface interface {
	SetCryptoService(cs *crypto.CryptoService)
	CreateUser(user *User) error
	GetUserByEmail(email string) (*User, error)
	GetUserByID(userID string) (*User, error)
	GetAllUsers() ([]string, error)
	UpdateUserOTPVerified(userID string, verified bool) error
	GetAIModels(userID string) ([]*AIModelConfig, error)
	UpdateAIModel(userID, id string, enabled bool, apiKey, customAPIURL, customModelName string) error
	GetExchanges(userID string) ([]*ExchangeConfig, error)
	UpdateExchange(userID, id string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey, lighterWalletAddr, lighterPrivateKey, lighterAPIKeyPrivateKey, okxPassphrase string) error
	CreateAIModel(userID, id, name, provider string, enabled bool, apiKey, customAPIURL string) error
	CreateExchange(userID, id, name, typ string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) error
	CreateTrader(trader *TraderRecord) error
	GetTraders(userID string) ([]*TraderRecord, error)
	UpdateTraderStatus(userID, id string, isRunning bool) error
	UpdateTrader(trader *TraderRecord) error
	UpdateTraderInitialBalance(userID, id string, newBalance float64) error
	UpdateTraderCustomPrompt(userID, id string, customPrompt string, overrideBase bool) error
	DeleteTrader(userID, id string) error
	GetTraderConfig(userID, traderID string) (*TraderRecord, *AIModelConfig, *ExchangeConfig, error)
	GetSystemConfig(key string) (string, error)
	SetSystemConfig(key, value string) error
	CreateUserSignalSource(userID, coinPoolURL, oiTopURL string) error
	GetUserSignalSource(userID string) (*UserSignalSource, error)
	UpdateUserSignalSource(userID, coinPoolURL, oiTopURL string) error
	GenerateWebhookAPIKey(userID string) (string, error)
	GetUserByWebhookAPIKey(apiKey string) (*User, error)
	CreateTradingViewAlert(userID, traderID string, payload map[string]interface{}) (string, error)
	GetPendingTradingViewAlerts(traderID string) ([]TradingViewAlert, error)
	GetRecentTradingViewAlerts(userID string, traderID string, limit int) ([]TradingViewAlert, error)
	UpdateAlertStatus(alertID string, status string) error
	GetTradersWithTradingViewEnabled(userID string) ([]*TraderRecord, error)
	GetCustomCoins() []string
	LoadBetaCodesFromFile(filePath string) error
	ValidateBetaCode(code string) (bool, error)
	UseBetaCode(code, userEmail string) error
	GetBetaCodeStats() (total, used int, err error)
	GetPromptTemplates(userID string) ([]*PromptTemplateConfig, error)
	GetPromptTemplate(userID, templateID string) (*PromptTemplateConfig, error)
	CreatePromptTemplate(userID, id, name, content string, isSystem bool) error
	UpdatePromptTemplate(userID, id, name, content string) error
	DeletePromptTemplate(userID, id string) error
	Close() error
}

// Database configuration database
type Database struct {
	db            *sql.DB
	cryptoService *crypto.CryptoService
}

// NewDatabase create configuration database
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	if err := tuneSQLiteConnection(db); err != nil {
		return nil, err
	}

	// 🔒 Enable WAL mode to improve concurrent performance and crash recovery
	// WAL (Write-Ahead Logging) mode advantages:
	// 1. Better concurrent performance: read operations are not blocked by write operations
	// 2. Crash safety: ensures data integrity even during power loss or forced termination
	// 3. Faster writes: no need to write to main database file every time
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// 🔒 Set synchronous=FULL to ensure data persistence
	// FULL (2) mode: ensures data is fully written to disk at critical moments
	// Combined with WAL mode, ensures data safety while maintaining good performance
	if _, err := db.Exec("PRAGMA synchronous=FULL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set synchronous: %w", err)
	}

	database := &Database{db: db}
	if err := database.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}
	if err := database.ensureBacktestRunColumns(); err != nil {
		return nil, fmt.Errorf("failed to initialize backtest table structure: %w", err)
	}

	// Ensure default user exists (for foreign key constraints and default configuration seeding)
	if _, err := db.Exec(`
		INSERT OR IGNORE INTO users (id, email, password_hash, otp_secret, otp_verified, role)
		VALUES ('default', 'default@local', '__default__', '', 1, 'user')
	`); err != nil {
		return nil, fmt.Errorf("failed to create default user: %w", err)
	}

	if err := database.initDefaultData(); err != nil {
		return nil, fmt.Errorf("failed to initialize default data: %w", err)
	}

	// Migrate prompt templates from files to database
	if err := database.migratePromptTemplatesFromFiles(); err != nil {
		log.Printf("⚠️  Prompt template migration failed: %v", err)
		// Don't return error, allow system to continue running
	}

	log.Printf("✅ Database WAL mode and FULL synchronous enabled, data persistence guaranteed")
	return database, nil
}

// columnExists check if specified column exists in table
func (d *Database) columnExists(tableName, columnName string) (bool, error) {
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info(?)
		WHERE name = ?
	`, tableName, columnName).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// createTables create database tables
func (d *Database) createTables() error {
	queries := []string{
		// AI model configuration table
		`CREATE TABLE IF NOT EXISTS ai_models (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			provider TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// Exchange configuration table
		`CREATE TABLE IF NOT EXISTS exchanges (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			type TEXT NOT NULL, -- 'cex' or 'dex'
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			secret_key TEXT DEFAULT '',
			testnet BOOLEAN DEFAULT 0,
			-- Hyperliquid specific fields
			hyperliquid_wallet_addr TEXT DEFAULT '',
			-- Aster specific fields
			aster_user TEXT DEFAULT '',
			aster_signer TEXT DEFAULT '',
			aster_private_key TEXT DEFAULT '',
			-- LIGHTER specific fields
			lighter_wallet_addr TEXT DEFAULT '',
			lighter_private_key TEXT DEFAULT '',
			lighter_api_key_private_key TEXT DEFAULT '',
			-- OKX specific fields
			okx_passphrase TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// User signal source configuration table
		`CREATE TABLE IF NOT EXISTS user_signal_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			coin_pool_url TEXT DEFAULT '',
			oi_top_url TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id)
		)`,

		// Trader configuration table
		`CREATE TABLE IF NOT EXISTS traders (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			ai_model_id TEXT NOT NULL,
			exchange_id TEXT NOT NULL,
			initial_balance REAL NOT NULL,
			scan_interval_minutes INTEGER DEFAULT 3,
			is_running BOOLEAN DEFAULT 0,
			btc_eth_leverage INTEGER DEFAULT 5,
			altcoin_leverage INTEGER DEFAULT 5,
			trading_symbols TEXT DEFAULT '',
			use_coin_pool BOOLEAN DEFAULT 0,
			use_oi_top BOOLEAN DEFAULT 0,
			use_tradingview BOOLEAN DEFAULT 0,
			followed_trader_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			otp_secret TEXT,
			otp_verified BOOLEAN DEFAULT 0,
			role TEXT DEFAULT 'follower',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// System configuration table
		`CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Prompt template table
		`CREATE TABLE IF NOT EXISTS prompt_templates (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			content TEXT NOT NULL,
			is_system BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// Backtest run main table
		`CREATE TABLE IF NOT EXISTS backtest_runs (
			run_id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			config_json TEXT NOT NULL DEFAULT '',
			state TEXT NOT NULL DEFAULT 'created',
			label TEXT DEFAULT '',
			symbol_count INTEGER DEFAULT 0,
			decision_tf TEXT DEFAULT '',
			processed_bars INTEGER DEFAULT 0,
			progress_pct REAL DEFAULT 0,
			equity_last REAL DEFAULT 0,
			max_drawdown_pct REAL DEFAULT 0,
			liquidated BOOLEAN DEFAULT 0,
			liquidation_note TEXT DEFAULT '',
			prompt_template TEXT DEFAULT '',
			custom_prompt TEXT DEFAULT '',
			override_prompt BOOLEAN DEFAULT 0,
			ai_provider TEXT DEFAULT '',
			ai_model TEXT DEFAULT '',
			last_error TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Backtest checkpoint
		`CREATE TABLE IF NOT EXISTS backtest_checkpoints (
			run_id TEXT PRIMARY KEY,
			payload BLOB NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (run_id) REFERENCES backtest_runs(run_id) ON DELETE CASCADE
		)`,

		// Backtest equity curve
		`CREATE TABLE IF NOT EXISTS backtest_equity (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			ts INTEGER NOT NULL,
			equity REAL NOT NULL,
			available REAL NOT NULL,
			pnl REAL NOT NULL,
			pnl_pct REAL NOT NULL,
			dd_pct REAL NOT NULL,
			cycle INTEGER NOT NULL,
			FOREIGN KEY (run_id) REFERENCES backtest_runs(run_id) ON DELETE CASCADE
		)`,

		// Backtest trade records
		`CREATE TABLE IF NOT EXISTS backtest_trades (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			ts INTEGER NOT NULL,
			symbol TEXT NOT NULL,
			action TEXT NOT NULL,
			side TEXT DEFAULT '',
			qty REAL DEFAULT 0,
			price REAL DEFAULT 0,
			fee REAL DEFAULT 0,
			slippage REAL DEFAULT 0,
			order_value REAL DEFAULT 0,
			realized_pnl REAL DEFAULT 0,
			leverage INTEGER DEFAULT 0,
			cycle INTEGER DEFAULT 0,
			position_after REAL DEFAULT 0,
			liquidation BOOLEAN DEFAULT 0,
			note TEXT DEFAULT '',
			FOREIGN KEY (run_id) REFERENCES backtest_runs(run_id) ON DELETE CASCADE
		)`,

		// Backtest metrics
		`CREATE TABLE IF NOT EXISTS backtest_metrics (
			run_id TEXT PRIMARY KEY,
			payload BLOB NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (run_id) REFERENCES backtest_runs(run_id) ON DELETE CASCADE
		)`,

		// Backtest decision logs
		`CREATE TABLE IF NOT EXISTS backtest_decisions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			cycle INTEGER NOT NULL,
			payload BLOB NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (run_id) REFERENCES backtest_runs(run_id) ON DELETE CASCADE
		)`,

		// Indexes
		`CREATE INDEX IF NOT EXISTS idx_backtest_runs_state ON backtest_runs(state, updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_backtest_equity_run_ts ON backtest_equity(run_id, ts)`,
		`CREATE INDEX IF NOT EXISTS idx_backtest_trades_run_ts ON backtest_trades(run_id, ts)`,
		`CREATE INDEX IF NOT EXISTS idx_backtest_decisions_run_cycle ON backtest_decisions(run_id, cycle)`,

		// Beta code table
		`CREATE TABLE IF NOT EXISTS beta_codes (
			code TEXT PRIMARY KEY,
			used BOOLEAN DEFAULT 0,
			used_by TEXT DEFAULT '',
			used_at DATETIME DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Triggers: automatically update updated_at
		`CREATE TRIGGER IF NOT EXISTS update_users_updated_at
			AFTER UPDATE ON users
			BEGIN
				UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_ai_models_updated_at
			AFTER UPDATE ON ai_models
			BEGIN
				UPDATE ai_models SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_exchanges_updated_at
			AFTER UPDATE ON exchanges
			BEGIN
				UPDATE exchanges SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_traders_updated_at
			AFTER UPDATE ON traders
			BEGIN
				UPDATE traders SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_user_signal_sources_updated_at
			AFTER UPDATE ON user_signal_sources
			BEGIN
				UPDATE user_signal_sources SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_system_config_updated_at
			AFTER UPDATE ON system_config
			BEGIN
				UPDATE system_config SET updated_at = CURRENT_TIMESTAMP WHERE key = NEW.key;
			END`,

		// Webhook API Keys table
		`CREATE TABLE IF NOT EXISTS webhook_api_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT UNIQUE NOT NULL,
			api_key TEXT UNIQUE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// TradingView Alerts table
		`CREATE TABLE IF NOT EXISTS tradingview_alerts (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			trader_id TEXT,
			raw_payload TEXT NOT NULL,
			symbol TEXT NOT NULL,
			action TEXT NOT NULL,
			exchange TEXT,
			entry REAL,
			sl REAL,
			tp REAL,
			quantity REAL,
			position_size REAL,
			pricetype TEXT,
			status TEXT DEFAULT 'pending',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			processed_at DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// Trader application table
		`CREATE TABLE IF NOT EXISTS trader_applications (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			description TEXT NOT NULL,
			trading_experience TEXT NOT NULL,
			strategy_overview TEXT NOT NULL,
			social_links TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			admin_notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// Indexes
		`CREATE INDEX IF NOT EXISTS idx_tradingview_alerts_user ON tradingview_alerts(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tradingview_alerts_trader ON tradingview_alerts(trader_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tradingview_alerts_status ON tradingview_alerts(status)`,
		`CREATE INDEX IF NOT EXISTS idx_trader_applications_user_id ON trader_applications(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_trader_applications_status ON trader_applications(status)`,

		// Trigger: automatically update webhook_api_keys updated_at
		`CREATE TRIGGER IF NOT EXISTS update_webhook_api_keys_updated_at
			AFTER UPDATE ON webhook_api_keys
			BEGIN
				UPDATE webhook_api_keys SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute SQL [%s]: %w", query, err)
		}
	}

	// Add new fields to existing database (backward compatibility)
	alterQueries := []struct {
		table  string
		column string
		query  string
	}{
		{"users", "role", `ALTER TABLE users ADD COLUMN role TEXT DEFAULT 'follower'`},
		{"exchanges", "hyperliquid_wallet_addr", `ALTER TABLE exchanges ADD COLUMN hyperliquid_wallet_addr TEXT DEFAULT ''`},
		{"exchanges", "aster_user", `ALTER TABLE exchanges ADD COLUMN aster_user TEXT DEFAULT ''`},
		{"exchanges", "aster_signer", `ALTER TABLE exchanges ADD COLUMN aster_signer TEXT DEFAULT ''`},
		{"exchanges", "aster_private_key", `ALTER TABLE exchanges ADD COLUMN aster_private_key TEXT DEFAULT ''`},
		{"exchanges", "lighter_wallet_addr", `ALTER TABLE exchanges ADD COLUMN lighter_wallet_addr TEXT DEFAULT ''`},
		{"exchanges", "lighter_private_key", `ALTER TABLE exchanges ADD COLUMN lighter_private_key TEXT DEFAULT ''`},
		{"exchanges", "lighter_api_key_private_key", `ALTER TABLE exchanges ADD COLUMN lighter_api_key_private_key TEXT DEFAULT ''`},
		{"exchanges", "okx_passphrase", `ALTER TABLE exchanges ADD COLUMN okx_passphrase TEXT DEFAULT ''`},
		{"traders", "custom_prompt", `ALTER TABLE traders ADD COLUMN custom_prompt TEXT DEFAULT ''`},
		{"traders", "override_base_prompt", `ALTER TABLE traders ADD COLUMN override_base_prompt BOOLEAN DEFAULT 0`},
		{"traders", "is_cross_margin", `ALTER TABLE traders ADD COLUMN is_cross_margin BOOLEAN DEFAULT 1`},
		{"traders", "use_default_coins", `ALTER TABLE traders ADD COLUMN use_default_coins BOOLEAN DEFAULT 1`},
		{"traders", "custom_coins", `ALTER TABLE traders ADD COLUMN custom_coins TEXT DEFAULT ''`},
		{"traders", "btc_eth_leverage", `ALTER TABLE traders ADD COLUMN btc_eth_leverage INTEGER DEFAULT 5`},
		{"traders", "altcoin_leverage", `ALTER TABLE traders ADD COLUMN altcoin_leverage INTEGER DEFAULT 5`},
		{"traders", "trading_symbols", `ALTER TABLE traders ADD COLUMN trading_symbols TEXT DEFAULT ''`},
		{"traders", "use_coin_pool", `ALTER TABLE traders ADD COLUMN use_coin_pool BOOLEAN DEFAULT 0`},
		{"traders", "use_oi_top", `ALTER TABLE traders ADD COLUMN use_oi_top BOOLEAN DEFAULT 0`},
		{"traders", "use_tradingview", `ALTER TABLE traders ADD COLUMN use_tradingview BOOLEAN DEFAULT 0`},
		{"traders", "system_prompt_template", `ALTER TABLE traders ADD COLUMN system_prompt_template TEXT DEFAULT 'default'`},
		{"traders", "followed_trader_id", `ALTER TABLE traders ADD COLUMN followed_trader_id TEXT`},
		{"ai_models", "custom_api_url", `ALTER TABLE ai_models ADD COLUMN custom_api_url TEXT DEFAULT ''`},
		{"ai_models", "custom_model_name", `ALTER TABLE ai_models ADD COLUMN custom_model_name TEXT DEFAULT ''`},
	}

	for _, alterQuery := range alterQueries {
		// Check if column exists, add if it doesn't exist
		exists, err := d.columnExists(alterQuery.table, alterQuery.column)
		if err != nil {
			log.Printf("⚠️  Error checking if column %s.%s exists: %v", alterQuery.table, alterQuery.column, err)
			// Continue trying to add column, table may not exist or column may already exist
		}

		if !exists {
			if _, err := d.db.Exec(alterQuery.query); err != nil {
				// Log error but continue execution (column may already exist or table may not exist)
				log.Printf("⚠️  ALTER TABLE warning (column %s.%s may already exist): %v", alterQuery.table, alterQuery.column, err)
			} else {
				log.Printf("✅ Successfully added column %s.%s", alterQuery.table, alterQuery.column)
			}
		} else {
			log.Printf("ℹ️  Column %s.%s already exists, skipping", alterQuery.table, alterQuery.column)
		}
	}

	// Check if exchanges table primary key structure migration is needed
	err := d.migrateExchangesTable()
	if err != nil {
		log.Printf("⚠️  Failed to migrate exchanges table: %v", err)
	}

	// Fix foreign key constraint issues in traders table
	err = d.migrateTradersTable()
	if err != nil {
		log.Printf("⚠️  Failed to migrate traders table: %v", err)
	}

	return nil
}

func (d *Database) ensureBacktestRunColumns() error {
	addColumn := func(table, column, definition string) error {
		exists, err := columnExists(d.db, table, column)
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
		_, err = d.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
		return err
	}
	if err := addColumn("backtest_runs", "label", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := addColumn("backtest_runs", "last_error", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := addColumn("backtest_trades", "leverage", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	return nil
}

func columnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid        int
			name       string
			ctype      string
			notnull    int
			dfltValue  any
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &primaryKey); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func tuneSQLiteConnection(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	statements := []string{
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA journal_mode = WAL`,
		`PRAGMA synchronous = NORMAL`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("failed to execute %s: %w", stmt, err)
		}
	}
	return nil
}

// initDefaultData initialize default data
func (d *Database) initDefaultData() error {
	// Initialize AI models (using default user)
	aiModels := []struct {
		id, name, provider string
	}{
		{"deepseek", "DeepSeek", "deepseek"},
		{"qwen", "Qwen", "qwen"},
		// Note: "risk_management" is a prompt template, not an AI model
	}

	for _, model := range aiModels {
		_, err := d.db.Exec(`
			INSERT OR IGNORE INTO ai_models (id, user_id, name, provider, enabled) 
			VALUES (?, 'default', ?, ?, 0)
		`, model.id, model.name, model.provider)
		if err != nil {
			return fmt.Errorf("failed to initialize AI model: %w", err)
		}
	}

	// Initialize exchanges (using default user)
	exchanges := []struct {
		id, name, typ string
	}{
		{"binance", "Binance Futures", "binance"},
		{"bybit", "Bybit Futures", "bybit"},
		{"hyperliquid", "Hyperliquid", "hyperliquid"},
		{"aster", "Aster DEX", "aster"},
		{"lighter", "LIGHTER DEX", "lighter"},
		{"okx", "OKX Futures", "okx"},
	}

	for _, exchange := range exchanges {
		_, err := d.db.Exec(`
			INSERT OR IGNORE INTO exchanges (id, user_id, name, type, enabled) 
			VALUES (?, 'default', ?, ?, 0)
		`, exchange.id, exchange.name, exchange.typ)
		if err != nil {
			return fmt.Errorf("failed to initialize exchange: %w", err)
		}
	}

	// Initialize system configuration - create all fields, set default values, later synced by config.json
	systemConfigs := map[string]string{
		"beta_mode":            "false",                                                                               // Default beta mode off
		"api_server_port":      "8080",                                                                                // Default API port
		"use_default_coins":    "true",                                                                                // Default use built-in coin list
		"default_coins":        `["BTCUSDT","ETHUSDT","SOLUSDT","BNBUSDT","XRPUSDT","DOGEUSDT","ADAUSDT","HYPEUSDT"]`, // Default coin list (JSON format)
		"max_daily_loss":       "10.0",                                                                                // Maximum daily loss percentage
		"max_drawdown":         "20.0",                                                                                // Maximum drawdown percentage
		"stop_trading_minutes": "60",                                                                                  // Stop trading time (minutes)
		"btc_eth_leverage":     "5",                                                                                   // BTC/ETH leverage multiplier
		"altcoin_leverage":     "5",                                                                                   // Altcoin leverage multiplier
		"jwt_secret":           "",                                                                                    // JWT secret, empty by default, generated by config.json or system
		"registration_enabled": "true",                                                                                // Default allow registration
	}

	for key, value := range systemConfigs {
		_, err := d.db.Exec(`
			INSERT OR IGNORE INTO system_config (key, value) 
			VALUES (?, ?)
		`, key, value)
		if err != nil {
			return fmt.Errorf("failed to initialize system configuration: %w", err)
		}
	}

	return nil
}

// migrateExchangesTable migrate exchanges table to support multi-user
func (d *Database) migrateExchangesTable() error {
	// Check if already migrated (check if exchanges table has composite primary key)
	var tableSQL string
	err := d.db.QueryRow(`
		SELECT sql FROM sqlite_master 
		WHERE type='table' AND name='exchanges'
	`).Scan(&tableSQL)
	if err == nil && strings.Contains(tableSQL, "PRIMARY KEY (id, user_id)") {
		// Already migrated, clean up any existing exchanges_new table (if previous migration failed)
		d.db.Exec(`DROP TABLE IF EXISTS exchanges_new`)
		return nil
	}

	// If exchanges_new table exists, previous migration may have failed, clean it up first
	_, err = d.db.Exec(`DROP TABLE IF EXISTS exchanges_new`)
	if err != nil {
		log.Printf("⚠️ Failed to clean up exchanges_new table: %v", err)
	}

	log.Printf("🔄 Starting exchanges table migration...")

	// Create new exchanges table with composite primary key
	_, err = d.db.Exec(`
		CREATE TABLE exchanges_new (
			id TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			secret_key TEXT DEFAULT '',
			testnet BOOLEAN DEFAULT 0,
			hyperliquid_wallet_addr TEXT DEFAULT '',
			aster_user TEXT DEFAULT '',
			aster_signer TEXT DEFAULT '',
			aster_private_key TEXT DEFAULT '',
			lighter_wallet_addr TEXT DEFAULT '',
			lighter_private_key TEXT DEFAULT '',
			lighter_api_key_private_key TEXT DEFAULT '',
			okx_passphrase TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id, user_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create new exchanges table: %w", err)
	}

	// Copy data to new table
	_, err = d.db.Exec(`
		INSERT INTO exchanges_new 
		SELECT * FROM exchanges
	`)
	if err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	// Delete old table
	_, err = d.db.Exec(`DROP TABLE exchanges`)
	if err != nil {
		return fmt.Errorf("failed to delete old table: %w", err)
	}

	// Rename new table
	_, err = d.db.Exec(`ALTER TABLE exchanges_new RENAME TO exchanges`)
	if err != nil {
		return fmt.Errorf("failed to rename table: %w", err)
	}

	// Recreate trigger
	_, err = d.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_exchanges_updated_at
			AFTER UPDATE ON exchanges
			BEGIN
				UPDATE exchanges SET updated_at = CURRENT_TIMESTAMP 
				WHERE id = NEW.id AND user_id = NEW.user_id;
			END
	`)
	if err != nil {
		return fmt.Errorf("failed to create trigger: %w", err)
	}

	log.Printf("✅ exchanges table migration completed")
	return nil
}

// migrateTradersTable migrate traders table, remove foreign key constraints
func (d *Database) migrateTradersTable() error {
	// Check if traders table has foreign key constraints (by checking table SQL)
	// If table no longer has foreign key constraints, skip migration
	var tableSQL string
	err := d.db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='traders'`).Scan(&tableSQL)
	if err != nil {
		// Table doesn't exist, no migration needed
		return nil
	}

	// Check if contains FOREIGN KEY (exchange_id) or FOREIGN KEY (ai_model_id)
	if !strings.Contains(tableSQL, "FOREIGN KEY (exchange_id)") && !strings.Contains(tableSQL, "FOREIGN KEY (ai_model_id)") {
		// No longer has these foreign key constraints, no migration needed
		return nil
	}

	log.Printf("🔄 Starting traders table migration, removing foreign key constraints...")

	// Create new traders table without foreign key constraints on exchange_id and ai_model_id
	_, err = d.db.Exec(`
		CREATE TABLE traders_new (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			ai_model_id TEXT NOT NULL,
			exchange_id TEXT NOT NULL,
			initial_balance REAL NOT NULL,
			scan_interval_minutes INTEGER DEFAULT 3,
			is_running BOOLEAN DEFAULT 0,
			btc_eth_leverage INTEGER DEFAULT 5,
			altcoin_leverage INTEGER DEFAULT 5,
			trading_symbols TEXT DEFAULT '',
			use_coin_pool BOOLEAN DEFAULT 0,
			use_oi_top BOOLEAN DEFAULT 0,
			custom_prompt TEXT DEFAULT '',
			override_base_prompt BOOLEAN DEFAULT 0,
			system_prompt_template TEXT DEFAULT 'default',
			is_cross_margin BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create new traders table: %w", err)
	}

	// Copy data to new table
	_, err = d.db.Exec(`
		INSERT INTO traders_new (id, user_id, name, ai_model_id, exchange_id, initial_balance, 
			scan_interval_minutes, is_running, btc_eth_leverage, altcoin_leverage, trading_symbols,
			use_coin_pool, use_oi_top, custom_prompt, override_base_prompt, system_prompt_template,
			is_cross_margin, created_at, updated_at)
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, 
			scan_interval_minutes, is_running, 
			COALESCE(btc_eth_leverage, 5), COALESCE(altcoin_leverage, 5), 
			COALESCE(trading_symbols, ''), COALESCE(use_coin_pool, 0), COALESCE(use_oi_top, 0),
			COALESCE(custom_prompt, ''), COALESCE(override_base_prompt, 0), 
			COALESCE(system_prompt_template, 'default'), COALESCE(is_cross_margin, 1),
			created_at, updated_at
		FROM traders
	`)
	if err != nil {
		// If copy fails, delete new table
		d.db.Exec(`DROP TABLE traders_new`)
		return fmt.Errorf("failed to copy traders data: %w", err)
	}

	// Delete old table
	_, err = d.db.Exec(`DROP TABLE traders`)
	if err != nil {
		return fmt.Errorf("failed to delete old traders table: %w", err)
	}

	// Rename new table
	_, err = d.db.Exec(`ALTER TABLE traders_new RENAME TO traders`)
	if err != nil {
		return fmt.Errorf("failed to rename traders table: %w", err)
	}

	log.Printf("✅ traders table migration completed, foreign key constraints removed")
	return nil
}

// migratePromptTemplatesFromFiles migrate prompt templates from filesystem to database
func (d *Database) migratePromptTemplatesFromFiles() error {
	// Check if templates already exist in database
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM prompt_templates`).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check prompt templates table: %w", err)
	}

	// If templates already exist, skip migration
	if count > 0 {
		log.Printf("✓ Prompt templates already exist in database, skipping migration")
		return nil
	}

	log.Printf("🔄 Starting to migrate prompt templates from filesystem to database...")

	// Read all .txt files from prompts directory
	promptsDir := "prompts"
	files, err := filepath.Glob(filepath.Join(promptsDir, "*.txt"))
	if err != nil {
		return fmt.Errorf("failed to scan prompts directory: %w", err)
	}

	if len(files) == 0 {
		log.Printf("⚠️  No .txt files found in prompts directory %s", promptsDir)
		return nil
	}

	// Migrate each template file
	migratedCount := 0
	for _, file := range files {
		// Read file content
		content, err := os.ReadFile(file)
		if err != nil {
			log.Printf("⚠️  Failed to read prompt file %s: %v", file, err)
			continue
		}

		// Extract filename (without extension) as template ID and name
		fileName := filepath.Base(file)
		templateID := strings.TrimSuffix(fileName, filepath.Ext(fileName))
		templateName := templateID

		// Insert into database (use default user, mark as system template)
		_, err = d.db.Exec(`
			INSERT INTO prompt_templates (id, user_id, name, content, is_system, created_at, updated_at)
			VALUES (?, 'default', ?, ?, 1, datetime('now'), datetime('now'))
		`, templateID, templateName, string(content))
		if err != nil {
			log.Printf("⚠️  Failed to insert prompt template %s: %v", templateID, err)
			continue
		}

		migratedCount++
		log.Printf("  📄 Migrated prompt template: %s", templateID)
	}

	log.Printf("✅ Prompt template migration completed, migrated %d templates", migratedCount)
	return nil
}

// User user configuration
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // Not returned to frontend
	OTPSecret    string    `json:"-"` // Not returned to frontend
	OTPVerified  bool      `json:"otp_verified"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AIModelConfig AI model configuration
type AIModelConfig struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	Name            string    `json:"name"`
	Provider        string    `json:"provider"`
	Enabled         bool      `json:"enabled"`
	APIKey          string    `json:"apiKey"`
	CustomAPIURL    string    `json:"customApiUrl"`
	CustomModelName string    `json:"customModelName"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ExchangeConfig exchange configuration
type ExchangeConfig struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Enabled   bool   `json:"enabled"`
	APIKey    string `json:"apiKey"`    // For Binance: API Key; For Hyperliquid: Agent Private Key (should have ~0 balance)
	SecretKey string `json:"secretKey"` // For Binance: Secret Key; Not used for Hyperliquid
	Testnet   bool   `json:"testnet"`
	// Hyperliquid Agent Wallet configuration (following official best practices)
	// Reference: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/nonces-and-api-wallets
	HyperliquidWalletAddr string `json:"hyperliquidWalletAddr"` // Main Wallet Address (holds funds, never expose private key)
	// Aster specific fields
	AsterUser       string `json:"asterUser"`
	AsterSigner     string `json:"asterSigner"`
	AsterPrivateKey string `json:"asterPrivateKey"`
	// LIGHTER specific fields
	LighterWalletAddr       string `json:"lighterWalletAddr"`       // Ethereum wallet address (L1)
	LighterPrivateKey       string `json:"lighterPrivateKey"`       // L1 private key (for account identification)
	LighterAPIKeyPrivateKey string `json:"lighterAPIKeyPrivateKey"` // API Key private key (40 bytes, for signing transactions)
	// OKX specific fields
	OkxPassphrase string    `json:"okxPassphrase"` // OKX passphrase (required for OKX)
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TraderRecord trader configuration (database entity)
type TraderRecord struct {
	ID                   string    `json:"id"`
	UserID               string    `json:"user_id"`
	Name                 string    `json:"name"`
	AIModelID            string    `json:"ai_model_id"`
	ExchangeID           string    `json:"exchange_id"`
	InitialBalance       float64   `json:"initial_balance"`
	ScanIntervalMinutes  int       `json:"scan_interval_minutes"`
	IsRunning            bool      `json:"is_running"`
	BTCETHLeverage       int       `json:"btc_eth_leverage"`       // BTC/ETH leverage multiplier
	AltcoinLeverage      int       `json:"altcoin_leverage"`       // Altcoin leverage multiplier
	TradingSymbols       string    `json:"trading_symbols"`        // Trading symbols, comma-separated
	UseCoinPool          bool      `json:"use_coin_pool"`          // Whether to use COIN POOL signal source
	UseOITop             bool      `json:"use_oi_top"`             // Whether to use OI TOP signal source
	UseTradingView       bool      `json:"use_tradingview"`        // Whether to use TradingView signal source
	FollowedTraderID     string    `json:"followed_trader_id"`     // Followed trader ID (for follower role)
	CustomPrompt         string    `json:"custom_prompt"`          // Custom trading strategy prompt
	OverrideBasePrompt   bool      `json:"override_base_prompt"`   // Whether to override base prompt
	SystemPromptTemplate string    `json:"system_prompt_template"` // System prompt template name
	IsCrossMargin        bool      `json:"is_cross_margin"`        // Whether cross margin mode (true=cross, false=isolated)
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// PromptTemplateConfig prompt template configuration
type PromptTemplateConfig struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	IsSystem  bool      `json:"is_system"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserSignalSource user signal source configuration
type UserSignalSource struct {
	ID          int       `json:"id"`
	UserID      string    `json:"user_id"`
	CoinPoolURL string    `json:"coin_pool_url"`
	OITopURL    string    `json:"oi_top_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TradingViewAlert TradingView alert
type TradingViewAlert struct {
	ID           string     `json:"id"`
	UserID      string     `json:"user_id"`
	TraderID    string     `json:"trader_id"`
	RawPayload  string     `json:"raw_payload"`
	Symbol      string     `json:"symbol"`
	Action      string     `json:"action"`
	Exchange    string     `json:"exchange"`
	Entry       float64    `json:"entry"`
	SL          float64    `json:"sl"`
	TP          float64    `json:"tp"`
	Quantity    float64    `json:"quantity"`
	PositionSize float64  `json:"position_size"`
	PriceType   string     `json:"pricetype"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	ProcessedAt *time.Time `json:"processed_at"`
}

// TraderApplication trader application
type TraderApplication struct {
	ID                string    `json:"id"`
	UserID            string    `json:"user_id"`
	Name              string    `json:"name"`
	Email             string    `json:"email"`
	Description       string    `json:"description"`
	TradingExperience string    `json:"trading_experience"`
	StrategyOverview  string    `json:"strategy_overview"`
	SocialLinks       string    `json:"social_links"` // JSON string
	Status            string    `json:"status"`       // 'pending', 'approved', 'rejected'
	AdminNotes        string    `json:"admin_notes"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// GenerateOTPSecret generate OTP secret
func GenerateOTPSecret() (string, error) {
	secret := make([]byte, 20)
	_, err := rand.Read(secret)
	if err != nil {
		return "", err
	}
	return base32.StdEncoding.EncodeToString(secret), nil
}

// CreateUser create user
func (d *Database) CreateUser(user *User) error {
	// Set default role if not specified
	role := user.Role
	if role == "" {
		role = "follower"
	}
	_, err := d.db.Exec(`
		INSERT INTO users (id, email, password_hash, otp_secret, otp_verified, role)
		VALUES (?, ?, ?, ?, ?, ?)
	`, user.ID, user.Email, user.PasswordHash, user.OTPSecret, user.OTPVerified, role)
	return err
}

// EnsureAdminUser ensure admin user exists (for admin mode)
func (d *Database) EnsureAdminUser() error {
	// Check if admin user already exists
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id = 'admin'`).Scan(&count)
	if err != nil {
		return err
	}

	// If already exists, return directly
	if count > 0 {
		return nil
	}

	// Create admin user (password empty, as admin mode doesn't require password)
	adminUser := &User{
		ID:           "admin",
		Email:        "admin@localhost",
		PasswordHash: "", // No password used in admin mode
		OTPSecret:    "",
		OTPVerified:  true,
		Role:         "user",
	}

	return d.CreateUser(adminUser)
}

// GetUserByEmail get user by email
func (d *Database) GetUserByEmail(email string) (*User, error) {
	var user User
	var createdAt, updatedAt string
	var role sql.NullString
	err := d.db.QueryRow(`
		SELECT id, email, password_hash, otp_secret, otp_verified, COALESCE(role, 'follower') as role, created_at, updated_at
		FROM users WHERE email = ?
	`, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.OTPSecret,
		&user.OTPVerified, &role, &createdAt, &updatedAt,
	)
	if err != nil {
		log.Printf("❌ GetUserByEmail failed: email=%s, error=%v", email, err)
		return nil, err
	}
	if role.Valid {
		user.Role = role.String
		if user.Role == "" {
			log.Printf("⚠️  GetUserByEmail: user %s role field is empty, using default 'follower'", email)
			user.Role = "follower"
		}
	} else {
		log.Printf("⚠️  GetUserByEmail: user %s role field is NULL, using default 'follower'", email)
		user.Role = "follower"
	}
	user.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	user.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	log.Printf("✅ GetUserByEmail: successfully retrieved user email=%s, id=%s, role=%s", email, user.ID, user.Role)
	return &user, nil
}

// GetUserByID get user by ID
func (d *Database) GetUserByID(userID string) (*User, error) {
	var user User
	var createdAt, updatedAt string
	var role sql.NullString
	err := d.db.QueryRow(`
		SELECT id, email, password_hash, otp_secret, otp_verified, COALESCE(role, 'follower') as role, created_at, updated_at
		FROM users WHERE id = ?
	`, userID).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.OTPSecret,
		&user.OTPVerified, &role, &createdAt, &updatedAt,
	)
	if err != nil {
		log.Printf("❌ GetUserByID failed: userID=%s, error=%v", userID, err)
		return nil, err
	}
	if role.Valid {
		user.Role = role.String
		if user.Role == "" {
			log.Printf("⚠️  GetUserByID: user %s role field is empty, using default 'follower'", userID)
			user.Role = "follower"
		}
	} else {
		log.Printf("⚠️  GetUserByID: user %s role field is NULL, using default 'follower'", userID)
		user.Role = "follower"
	}
	user.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	user.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	log.Printf("✅ GetUserByID: successfully retrieved user userID=%s, email=%s, role=%s", userID, user.Email, user.Role)
	return &user, nil
}

// GetAllUsers get all user ID list
func (d *Database) GetAllUsers() ([]string, error) {
	rows, err := d.db.Query(`SELECT id FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, nil
}

// GetAllUsersWithRoles get all users and their roles (for admin use)
func (d *Database) GetAllUsersWithRoles() ([]*User, error) {
	rows, err := d.db.Query(`
		SELECT id, email, password_hash, otp_secret, otp_verified, COALESCE(role, 'follower') as role, created_at, updated_at
		FROM users ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		var createdAt, updatedAt string
		var role sql.NullString
		err := rows.Scan(
			&user.ID, &user.Email, &user.PasswordHash, &user.OTPSecret,
			&user.OTPVerified, &role, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		if role.Valid {
			user.Role = role.String
			if user.Role == "" {
				user.Role = "follower"
			}
		} else {
			user.Role = "follower"
		}
		// Parse created_at with error handling and multiple format support
		user.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
		if err != nil {
			// Try RFC3339 format (ISO 8601) as fallback
			user.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
			if err != nil {
				// Try parsing with location
				user.CreatedAt, err = time.ParseInLocation("2006-01-02 15:04:05", createdAt, time.UTC)
				if err != nil {
					log.Printf("⚠️  GetAllUsersWithRoles: unable to parse created_at '%s' for user %s: %v", createdAt, user.ID, err)
					// Use current time as fallback instead of zero time
					user.CreatedAt = time.Now()
				}
			}
		}
		// Parse updated_at with error handling and multiple format support
		user.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt)
		if err != nil {
			// Try RFC3339 format (ISO 8601) as fallback
			user.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
			if err != nil {
				// Try parsing with location
				user.UpdatedAt, err = time.ParseInLocation("2006-01-02 15:04:05", updatedAt, time.UTC)
				if err != nil {
					log.Printf("⚠️  GetAllUsersWithRoles: unable to parse updated_at '%s' for user %s: %v", updatedAt, user.ID, err)
					// Use current time as fallback instead of zero time
					user.UpdatedAt = time.Now()
				}
			}
		}
		users = append(users, &user)
	}
	return users, nil
}

// UpdateUserRole update user role (for admin use)
func (d *Database) UpdateUserRole(userID string, role string) error {
	// Validate role value
	validRoles := map[string]bool{"user": true, "follower": true, "admin": true}
	if !validRoles[role] {
		return fmt.Errorf("invalid role value: %s", role)
	}

	_, err := d.db.Exec(`
		UPDATE users SET role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, role, userID)
	return err
}

// CreateTraderApplication create trader application
func (d *Database) CreateTraderApplication(app *TraderApplication) error {
	_, err := d.db.Exec(`
		INSERT INTO trader_applications (id, user_id, name, email, description, trading_experience, strategy_overview, social_links, status, admin_notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, app.ID, app.UserID, app.Name, app.Email, app.Description, app.TradingExperience, app.StrategyOverview, app.SocialLinks, app.Status, app.AdminNotes)
	return err
}

// GetTraderApplicationByID get trader application by ID
func (d *Database) GetTraderApplicationByID(id string) (*TraderApplication, error) {
	var app TraderApplication
	var createdAt, updatedAt string
	err := d.db.QueryRow(`
		SELECT id, user_id, name, email, description, trading_experience, strategy_overview, social_links, status, admin_notes, created_at, updated_at
		FROM trader_applications WHERE id = ?
	`, id).Scan(
		&app.ID, &app.UserID, &app.Name, &app.Email, &app.Description, &app.TradingExperience,
		&app.StrategyOverview, &app.SocialLinks, &app.Status, &app.AdminNotes,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	app.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	app.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &app, nil
}

// GetTraderApplicationByUserID get trader application by user ID
func (d *Database) GetTraderApplicationByUserID(userID string) (*TraderApplication, error) {
	var app TraderApplication
	var createdAt, updatedAt string
	err := d.db.QueryRow(`
		SELECT id, user_id, name, email, description, trading_experience, strategy_overview, social_links, status, admin_notes, created_at, updated_at
		FROM trader_applications WHERE user_id = ? ORDER BY created_at DESC LIMIT 1
	`, userID).Scan(
		&app.ID, &app.UserID, &app.Name, &app.Email, &app.Description, &app.TradingExperience,
		&app.StrategyOverview, &app.SocialLinks, &app.Status, &app.AdminNotes,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	app.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	app.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &app, nil
}

// GetAllTraderApplications get all trader applications (for admin use)
func (d *Database) GetAllTraderApplications() ([]*TraderApplication, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, name, email, description, trading_experience, strategy_overview, social_links, status, admin_notes, created_at, updated_at
		FROM trader_applications ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var applications []*TraderApplication
	for rows.Next() {
		var app TraderApplication
		var createdAt, updatedAt string
		err := rows.Scan(
			&app.ID, &app.UserID, &app.Name, &app.Email, &app.Description, &app.TradingExperience,
			&app.StrategyOverview, &app.SocialLinks, &app.Status, &app.AdminNotes,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		app.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		app.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		applications = append(applications, &app)
	}
	return applications, nil
}

// UpdateTraderApplicationStatus update trader application status
func (d *Database) UpdateTraderApplicationStatus(id string, status string, adminNotes string) error {
	_, err := d.db.Exec(`
		UPDATE trader_applications SET status = ?, admin_notes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, status, adminNotes, id)
	return err
}

// GetAllTraders get all traders (for admin use)
func (d *Database) GetAllTraders() ([]*TraderRecord, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
		       COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
		       COALESCE(trading_symbols, '') as trading_symbols,
		       COALESCE(use_coin_pool, 0) as use_coin_pool, COALESCE(use_oi_top, 0) as use_oi_top,
		       COALESCE(use_tradingview, 0) as use_tradingview,
		       COALESCE(followed_trader_id, '') as followed_trader_id,
		       COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, 0) as override_base_prompt,
		       COALESCE(system_prompt_template, 'default') as system_prompt_template,
		       COALESCE(is_cross_margin, 1) as is_cross_margin, created_at, updated_at
		FROM traders ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traders []*TraderRecord
	for rows.Next() {
		var trader TraderRecord
		var createdAt, updatedAt string
		err := rows.Scan(
			&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
			&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
			&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
			&trader.UseCoinPool, &trader.UseOITop, &trader.UseTradingView,
			&trader.FollowedTraderID,
			&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
			&trader.IsCrossMargin,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		trader.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		trader.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		traders = append(traders, &trader)
	}

	return traders, nil
}

// UpdateUserOTPVerified update user OTP verification status
func (d *Database) UpdateUserOTPVerified(userID string, verified bool) error {
	_, err := d.db.Exec(`UPDATE users SET otp_verified = ? WHERE id = ?`, verified, userID)
	return err
}

// UpdateUserPassword update user password
func (d *Database) UpdateUserPassword(userID, passwordHash string) error {
	_, err := d.db.Exec(`
		UPDATE users
		SET password_hash = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, passwordHash, userID)
	return err
}

// GetAIModels get user's AI model configurations
func (d *Database) GetAIModels(userID string) ([]*AIModelConfig, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, name, provider, enabled, api_key,
		       COALESCE(custom_api_url, '') as custom_api_url,
		       COALESCE(custom_model_name, '') as custom_model_name,
		       created_at, updated_at
		FROM ai_models WHERE user_id = ? ORDER BY id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Initialize as empty slice instead of nil, ensure JSON serialization is [] instead of null
	models := make([]*AIModelConfig, 0)
	for rows.Next() {
		var model AIModelConfig
		var createdAt, updatedAt string
		err := rows.Scan(
			&model.ID, &model.UserID, &model.Name, &model.Provider,
			&model.Enabled, &model.APIKey, &model.CustomAPIURL, &model.CustomModelName,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		// Parse time string
		model.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		model.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		// Decrypt API Key
		model.APIKey = d.decryptSensitiveData(model.APIKey)
		models = append(models, &model)
	}

	return models, nil
}

// GetAIModel get single AI model configuration by model ID and user ID, fallback to default user if not found for user.
func (d *Database) GetAIModel(userID, modelID string) (*AIModelConfig, error) {
	if modelID == "" {
		return nil, fmt.Errorf("model ID cannot be empty")
	}

	candidates := []string{}
	if userID != "" {
		candidates = append(candidates, userID)
	}
	if userID != "default" {
		candidates = append(candidates, "default")
	}
	if len(candidates) == 0 {
		candidates = append(candidates, "default")
	}

	for _, uid := range candidates {
		var model AIModelConfig
		var createdAt, updatedAt string
		err := d.db.QueryRow(`
			SELECT id, user_id, name, provider, enabled, api_key,
			       COALESCE(custom_api_url, ''), COALESCE(custom_model_name, ''), created_at, updated_at
			FROM ai_models
			WHERE user_id = ? AND id = ?
			LIMIT 1
		`, uid, modelID).Scan(
			&model.ID,
			&model.UserID,
			&model.Name,
			&model.Provider,
			&model.Enabled,
			&model.APIKey,
			&model.CustomAPIURL,
			&model.CustomModelName,
			&createdAt,
			&updatedAt,
		)
		if err == nil {
			// Parse time string
			model.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
			model.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
			// Decrypt API Key (consistent with GetAIModels behavior)
			model.APIKey = d.decryptSensitiveData(model.APIKey)
			return &model, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	return nil, sql.ErrNoRows
}

// GetDefaultAIModel get first enabled AI model for specified user (or default user).
func (d *Database) GetDefaultAIModel(userID string) (*AIModelConfig, error) {
	if userID == "" {
		userID = "default"
	}
	model, err := d.firstEnabledAIModel(userID)
	if err == nil {
		return model, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if userID != "default" {
		return d.firstEnabledAIModel("default")
	}
	return nil, fmt.Errorf("please configure an available AI model in the system first")
}

func (d *Database) firstEnabledAIModel(userID string) (*AIModelConfig, error) {
	var model AIModelConfig
	var createdAt, updatedAt string
	err := d.db.QueryRow(`
		SELECT id, user_id, name, provider, enabled, api_key,
		       COALESCE(custom_api_url, ''), COALESCE(custom_model_name, ''), created_at, updated_at
		FROM ai_models
		WHERE user_id = ? AND enabled = 1
		ORDER BY datetime(updated_at) DESC, id ASC
		LIMIT 1
	`, userID).Scan(
		&model.ID,
		&model.UserID,
		&model.Name,
		&model.Provider,
		&model.Enabled,
		&model.APIKey,
		&model.CustomAPIURL,
		&model.CustomModelName,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	// Parse time string
	model.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	model.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	// Decrypt API Key to avoid downstream authentication failure with encrypted string
	model.APIKey = d.decryptSensitiveData(model.APIKey)
	return &model, nil
}

// UpdateAIModel update AI model configuration, create user-specific configuration if it doesn't exist
func (d *Database) UpdateAIModel(userID, id string, enabled bool, apiKey, customAPIURL, customModelName string) error {
	// First try exact ID match (new logic, supports multiple models with same provider)
	var existingID string
	err := d.db.QueryRow(`
		SELECT id FROM ai_models WHERE user_id = ? AND id = ? LIMIT 1
	`, userID, id).Scan(&existingID)

	if err == nil {
		// Found existing configuration (exact ID match), update it
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
		_, err = d.db.Exec(`
			UPDATE ai_models SET enabled = ?, api_key = ?, custom_api_url = ?, custom_model_name = ?, updated_at = datetime('now')
			WHERE id = ? AND user_id = ?
		`, enabled, encryptedAPIKey, customAPIURL, customModelName, existingID, userID)
		return err
	}

	// ID doesn't exist, try backward compatibility: use id as provider to search
	provider := id
	err = d.db.QueryRow(`
		SELECT id FROM ai_models WHERE user_id = ? AND provider = ? LIMIT 1
	`, userID, provider).Scan(&existingID)

	if err == nil {
		// Found existing configuration (matched by provider, backward compatible), update it
		log.Printf("⚠️  Using old provider matching to update model: %s -> %s", provider, existingID)
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
		_, err = d.db.Exec(`
			UPDATE ai_models SET enabled = ?, api_key = ?, custom_api_url = ?, custom_model_name = ?, updated_at = datetime('now')
			WHERE id = ? AND user_id = ?
		`, enabled, encryptedAPIKey, customAPIURL, customModelName, existingID, userID)
		return err
	}

	// No existing configuration found, create new one
	// Infer provider (extract from id, or use id directly)
	if provider == id && (provider == "deepseek" || provider == "qwen") {
		// id itself is the provider
		provider = id
	} else {
		// Extract provider from id (assume format is userID_provider or timestamp_userID_provider)
		parts := strings.Split(id, "_")
		if len(parts) >= 2 {
			provider = parts[len(parts)-1] // Take last part as provider
		} else {
			provider = id
		}
	}

	// Get model basic information
	var name string
	err = d.db.QueryRow(`
		SELECT name FROM ai_models WHERE provider = ? LIMIT 1
	`, provider).Scan(&name)
	if err != nil {
		// If basic information not found, use default values
		switch provider {
		case "deepseek":
			name = "DeepSeek AI"
		case "qwen":
			name = "Qwen AI"
		default:
			name = provider + " AI"
		}
	}

	// If the passed ID is already in full format (e.g., "admin_deepseek_custom1"), use it directly
	// Otherwise generate new ID
	newModelID := id
	if id == provider {
		// id is the provider, generate new user-specific ID
		newModelID = fmt.Sprintf("%s_%s", userID, provider)
	}

	log.Printf("✓ Created new AI model configuration: ID=%s, Provider=%s, Name=%s", newModelID, provider, name)
	encryptedAPIKey := d.encryptSensitiveData(apiKey)
	_, err = d.db.Exec(`
		INSERT INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, newModelID, userID, name, provider, enabled, encryptedAPIKey, customAPIURL, customModelName)

	return err
}

// GetExchanges get user's exchange configurations
func (d *Database) GetExchanges(userID string) ([]*ExchangeConfig, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, name, type, enabled, api_key, secret_key, testnet,
		       COALESCE(hyperliquid_wallet_addr, '') as hyperliquid_wallet_addr,
		       COALESCE(aster_user, '') as aster_user,
		       COALESCE(aster_signer, '') as aster_signer,
		       COALESCE(aster_private_key, '') as aster_private_key,
		       COALESCE(lighter_wallet_addr, '') as lighter_wallet_addr,
		       COALESCE(lighter_private_key, '') as lighter_private_key,
		       COALESCE(lighter_api_key_private_key, '') as lighter_api_key_private_key,
		       COALESCE(okx_passphrase, '') as okx_passphrase,
		       created_at, updated_at
		FROM exchanges WHERE user_id = ? ORDER BY id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Initialize as empty slice instead of nil, ensure JSON serialization is [] instead of null
	exchanges := make([]*ExchangeConfig, 0)
	for rows.Next() {
		var exchange ExchangeConfig
		var createdAt, updatedAt string
		err := rows.Scan(
			&exchange.ID, &exchange.UserID, &exchange.Name, &exchange.Type,
			&exchange.Enabled, &exchange.APIKey, &exchange.SecretKey, &exchange.Testnet,
			&exchange.HyperliquidWalletAddr, &exchange.AsterUser,
			&exchange.AsterSigner, &exchange.AsterPrivateKey,
			&exchange.LighterWalletAddr, &exchange.LighterPrivateKey,
			&exchange.LighterAPIKeyPrivateKey, &exchange.OkxPassphrase,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Parse time string
		exchange.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		exchange.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

		// Decrypt sensitive fields
		exchange.APIKey = d.decryptSensitiveData(exchange.APIKey)
		exchange.SecretKey = d.decryptSensitiveData(exchange.SecretKey)
		exchange.AsterPrivateKey = d.decryptSensitiveData(exchange.AsterPrivateKey)
		exchange.LighterPrivateKey = d.decryptSensitiveData(exchange.LighterPrivateKey)
		exchange.LighterAPIKeyPrivateKey = d.decryptSensitiveData(exchange.LighterAPIKeyPrivateKey)
		exchange.OkxPassphrase = d.decryptSensitiveData(exchange.OkxPassphrase)

		exchanges = append(exchanges, &exchange)
	}

	return exchanges, nil
}

// UpdateExchange update exchange configuration, create user-specific configuration if it doesn't exist
// 🔒 Security feature: empty values will not overwrite existing sensitive fields (api_key, secret_key, aster_private_key, lighter_private_key, lighter_api_key_private_key, okx_passphrase)
func (d *Database) UpdateExchange(userID, id string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey, lighterWalletAddr, lighterPrivateKey, lighterAPIKeyPrivateKey, okxPassphrase string) error {
	log.Printf("🔧 UpdateExchange: userID=%s, id=%s, enabled=%v", userID, id, enabled)

	// Build dynamic UPDATE SET clause
	// Basic fields: always update
	setClauses := []string{
		"enabled = ?",
		"testnet = ?",
		"hyperliquid_wallet_addr = ?",
		"aster_user = ?",
		"aster_signer = ?",
		"lighter_wallet_addr = ?",
		"updated_at = datetime('now')",
	}
	args := []interface{}{enabled, testnet, hyperliquidWalletAddr, asterUser, asterSigner, lighterWalletAddr}

	// 🔒 Sensitive fields: only update when non-empty (protect existing data)
	if apiKey != "" {
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
		setClauses = append(setClauses, "api_key = ?")
		args = append(args, encryptedAPIKey)
	}

	if secretKey != "" {
		encryptedSecretKey := d.encryptSensitiveData(secretKey)
		setClauses = append(setClauses, "secret_key = ?")
		args = append(args, encryptedSecretKey)
	}

	if asterPrivateKey != "" {
		encryptedAsterPrivateKey := d.encryptSensitiveData(asterPrivateKey)
		setClauses = append(setClauses, "aster_private_key = ?")
		args = append(args, encryptedAsterPrivateKey)
	}

	if lighterPrivateKey != "" {
		encryptedLighterPrivateKey := d.encryptSensitiveData(lighterPrivateKey)
		setClauses = append(setClauses, "lighter_private_key = ?")
		args = append(args, encryptedLighterPrivateKey)
	}

	if lighterAPIKeyPrivateKey != "" {
		encryptedLighterAPIKeyPrivateKey := d.encryptSensitiveData(lighterAPIKeyPrivateKey)
		setClauses = append(setClauses, "lighter_api_key_private_key = ?")
		args = append(args, encryptedLighterAPIKeyPrivateKey)
	}

	if okxPassphrase != "" {
		encryptedOkxPassphrase := d.encryptSensitiveData(okxPassphrase)
		setClauses = append(setClauses, "okx_passphrase = ?")
		args = append(args, encryptedOkxPassphrase)
	}

	// WHERE condition
	args = append(args, id, userID)

	// Build complete UPDATE statement
	query := fmt.Sprintf(`
		UPDATE exchanges SET %s
		WHERE id = ? AND user_id = ?
	`, strings.Join(setClauses, ", "))

	// Execute update
	result, err := d.db.Exec(query, args...)
	if err != nil {
		log.Printf("❌ UpdateExchange: update failed: %v", err)
		return err
	}

	// Check if any rows were updated
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("❌ UpdateExchange: failed to get affected rows: %v", err)
		return err
	}

	log.Printf("📊 UpdateExchange: affected rows = %d", rowsAffected)

	// If no rows were updated, user doesn't have configuration for this exchange, need to create
	if rowsAffected == 0 {
		log.Printf("💡 UpdateExchange: no existing record, creating new record")

		// Determine basic information based on exchange ID
		var name, typ string
		switch id {
		case "binance":
			name = "Binance Futures"
			typ = "cex"
		case "bybit":
			name = "Bybit Futures"
			typ = "cex"
		case "hyperliquid":
			name = "Hyperliquid"
			typ = "dex"
		case "aster":
			name = "Aster DEX"
			typ = "dex"
		case "lighter":
			name = "LIGHTER DEX"
			typ = "dex"
		case "okx":
			name = "OKX Futures"
			typ = "cex"
		default:
			name = id + " Exchange"
			typ = "cex"
		}

		log.Printf("🆕 UpdateExchange: creating new record ID=%s, name=%s, type=%s", id, name, typ)

		// Encrypt sensitive fields
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
		encryptedSecretKey := d.encryptSensitiveData(secretKey)
		encryptedAsterPrivateKey := d.encryptSensitiveData(asterPrivateKey)
		encryptedLighterPrivateKey := d.encryptSensitiveData(lighterPrivateKey)
		encryptedLighterAPIKeyPrivateKey := d.encryptSensitiveData(lighterAPIKeyPrivateKey)
		encryptedOkxPassphrase := d.encryptSensitiveData(okxPassphrase)

		// Create user-specific configuration, using original exchange ID
		_, err = d.db.Exec(`
			INSERT INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet,
			                       hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key,
			                       lighter_wallet_addr, lighter_private_key, lighter_api_key_private_key, okx_passphrase, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		`, id, userID, name, typ, enabled, encryptedAPIKey, encryptedSecretKey, testnet, hyperliquidWalletAddr, asterUser, asterSigner, encryptedAsterPrivateKey, lighterWalletAddr, encryptedLighterPrivateKey, encryptedLighterAPIKeyPrivateKey, encryptedOkxPassphrase)

		if err != nil {
			log.Printf("❌ UpdateExchange: failed to create record: %v", err)
		} else {
			log.Printf("✅ UpdateExchange: record created successfully")
		}
		return err
	}

	log.Printf("✅ UpdateExchange: updated existing record successfully")
	return nil
}

// CreateAIModel create AI model configuration
func (d *Database) CreateAIModel(userID, id, name, provider string, enabled bool, apiKey, customAPIURL string) error {
	_, err := d.db.Exec(`
		INSERT OR IGNORE INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url) 
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, userID, name, provider, enabled, apiKey, customAPIURL)
	return err
}

// CreateExchange create exchange configuration
func (d *Database) CreateExchange(userID, id, name, typ string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) error {
	// Encrypt sensitive fields
	encryptedAPIKey := d.encryptSensitiveData(apiKey)
	encryptedSecretKey := d.encryptSensitiveData(secretKey)
	encryptedAsterPrivateKey := d.encryptSensitiveData(asterPrivateKey)

	_, err := d.db.Exec(`
		INSERT OR IGNORE INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet, hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, lighter_wallet_addr, lighter_private_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', '')
	`, id, userID, name, typ, enabled, encryptedAPIKey, encryptedSecretKey, testnet, hyperliquidWalletAddr, asterUser, asterSigner, encryptedAsterPrivateKey)
	return err
}

// GetExchangeByID get single exchange configuration by exchange ID and user ID
func (d *Database) GetExchangeByID(userID, exchangeID string) (*ExchangeConfig, error) {
	if exchangeID == "" {
		return nil, fmt.Errorf("exchange ID cannot be empty")
	}

	var exchange ExchangeConfig
	var createdAt, updatedAt string
	err := d.db.QueryRow(`
		SELECT id, user_id, name, type, enabled, api_key, secret_key, testnet,
		       COALESCE(hyperliquid_wallet_addr, '') as hyperliquid_wallet_addr,
		       COALESCE(aster_user, '') as aster_user,
		       COALESCE(aster_signer, '') as aster_signer,
		       COALESCE(aster_private_key, '') as aster_private_key,
		       COALESCE(lighter_wallet_addr, '') as lighter_wallet_addr,
		       COALESCE(lighter_private_key, '') as lighter_private_key,
		       COALESCE(lighter_api_key_private_key, '') as lighter_api_key_private_key,
		       COALESCE(okx_passphrase, '') as okx_passphrase,
		       created_at, updated_at
		FROM exchanges
		WHERE user_id = ? AND id = ?
		LIMIT 1
	`, userID, exchangeID).Scan(
		&exchange.ID, &exchange.UserID, &exchange.Name, &exchange.Type,
		&exchange.Enabled, &exchange.APIKey, &exchange.SecretKey, &exchange.Testnet,
		&exchange.HyperliquidWalletAddr, &exchange.AsterUser,
		&exchange.AsterSigner, &exchange.AsterPrivateKey,
		&exchange.LighterWalletAddr, &exchange.LighterPrivateKey,
		&exchange.LighterAPIKeyPrivateKey, &exchange.OkxPassphrase,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse time string
	exchange.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	exchange.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	// Decrypt sensitive fields
	exchange.APIKey = d.decryptSensitiveData(exchange.APIKey)
	exchange.SecretKey = d.decryptSensitiveData(exchange.SecretKey)
	exchange.AsterPrivateKey = d.decryptSensitiveData(exchange.AsterPrivateKey)
	exchange.LighterPrivateKey = d.decryptSensitiveData(exchange.LighterPrivateKey)
	exchange.LighterAPIKeyPrivateKey = d.decryptSensitiveData(exchange.LighterAPIKeyPrivateKey)
	exchange.OkxPassphrase = d.decryptSensitiveData(exchange.OkxPassphrase)

	return &exchange, nil
}

// CopyAIModelToUser copy AI model configuration from source user to target user (does not copy API key)
func (d *Database) CopyAIModelToUser(sourceUserID, targetUserID, modelID string) error {
	// Get source user's AI model configuration
	sourceModel, err := d.GetAIModel(sourceUserID, modelID)
	if err != nil {
		return fmt.Errorf("failed to get source AI model configuration: %w", err)
	}

	// Check if target user already has this model
	_, err = d.GetAIModel(targetUserID, modelID)
	if err == nil {
		// Model already exists, no need to copy
		log.Printf("✓ AI model %s already exists for target user %s", modelID, targetUserID)
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check target AI model: %w", err)
	}

	// Create model configuration (do not copy API key, use empty string)
	err = d.CreateAIModel(
		targetUserID,
		sourceModel.ID,
		sourceModel.Name,
		sourceModel.Provider,
		sourceModel.Enabled,
		"", // Do not copy API key
		sourceModel.CustomAPIURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create AI model configuration: %w", err)
	}

	log.Printf("✓ Created AI model configuration for follower user %s: %s (%s)", targetUserID, sourceModel.Name, sourceModel.Provider)
	return nil
}

// CopyExchangeToUser copy exchange configuration from source user to target user (does not copy API keys and secrets)
func (d *Database) CopyExchangeToUser(sourceUserID, targetUserID, exchangeID string) error {
	// Get source user's exchange configuration
	sourceExchange, err := d.GetExchangeByID(sourceUserID, exchangeID)
	if err != nil {
		return fmt.Errorf("failed to get source exchange configuration: %w", err)
	}

	// Check if target user already has this exchange
	_, err = d.GetExchangeByID(targetUserID, exchangeID)
	if err == nil {
		// Exchange already exists, no need to copy
		log.Printf("✓ Exchange %s already exists for target user %s", exchangeID, targetUserID)
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check target exchange: %w", err)
	}

	// Create exchange configuration (do not copy API keys and secrets, use empty strings)
	err = d.CreateExchange(
		targetUserID,
		sourceExchange.ID,
		sourceExchange.Name,
		sourceExchange.Type,
		sourceExchange.Enabled,
		"", // Do not copy API key
		"", // Do not copy Secret key
		sourceExchange.Testnet,
		sourceExchange.HyperliquidWalletAddr,
		sourceExchange.AsterUser,
		sourceExchange.AsterSigner,
		"", // Do not copy Aster private key
	)
	if err != nil {
		return fmt.Errorf("failed to create exchange configuration: %w", err)
	}

	log.Printf("✓ Created exchange configuration for follower user %s: %s (%s)", targetUserID, sourceExchange.Name, sourceExchange.Type)
	return nil
}

// GetPromptTemplates get user prompt templates (including system templates)
func (d *Database) GetPromptTemplates(userID string) ([]*PromptTemplateConfig, error) {
	// Get system templates (user_id='default') and user templates
	rows, err := d.db.Query(`
		SELECT id, user_id, name, content, is_system, created_at, updated_at
		FROM prompt_templates
		WHERE user_id = ? OR (user_id = 'default' AND is_system = 1)
		ORDER BY is_system DESC, name ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []*PromptTemplateConfig
	for rows.Next() {
		var template PromptTemplateConfig
		var createdAt, updatedAt string
		err := rows.Scan(
			&template.ID, &template.UserID, &template.Name, &template.Content,
			&template.IsSystem, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		// Parse time string
		template.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		template.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		templates = append(templates, &template)
	}

	return templates, nil
}

// GetPromptTemplate get specified prompt template
func (d *Database) GetPromptTemplate(userID, templateID string) (*PromptTemplateConfig, error) {
	// Allow getting system templates (user_id='default') or user's own templates
	var template PromptTemplateConfig
	var createdAt, updatedAt string
	err := d.db.QueryRow(`
		SELECT id, user_id, name, content, is_system, created_at, updated_at
		FROM prompt_templates
		WHERE id = ? AND (user_id = ? OR (user_id = 'default' AND is_system = 1))
		LIMIT 1
	`, templateID, userID).Scan(
		&template.ID, &template.UserID, &template.Name, &template.Content,
		&template.IsSystem, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("prompt template does not exist: %s", templateID)
		}
		return nil, err
	}

	// Parse time string
	template.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	template.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	return &template, nil
}

// CreatePromptTemplate create new prompt template
func (d *Database) CreatePromptTemplate(userID, id, name, content string, isSystem bool) error {
	_, err := d.db.Exec(`
		INSERT INTO prompt_templates (id, user_id, name, content, is_system, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, id, userID, name, content, isSystem)
	return err
}

// UpdatePromptTemplate update prompt template (can only update user-created templates, cannot update system templates)
func (d *Database) UpdatePromptTemplate(userID, id, name, content string) error {
	// Check if template exists and belongs to user, and is not a system template
	var isSystem bool
	var templateUserID string
	err := d.db.QueryRow(`
		SELECT user_id, is_system FROM prompt_templates WHERE id = ?
	`, id).Scan(&templateUserID, &isSystem)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("prompt template does not exist: %s", id)
		}
		return err
	}

	// Cannot update system templates
	if isSystem {
		return fmt.Errorf("cannot update system template: %s", id)
	}

	// Can only update own templates
	if templateUserID != userID {
		return fmt.Errorf("no permission to update this template: %s", id)
	}

	// Update template
	_, err = d.db.Exec(`
		UPDATE prompt_templates
		SET name = ?, content = ?, updated_at = datetime('now')
		WHERE id = ? AND user_id = ? AND is_system = 0
	`, name, content, id, userID)
	return err
}

// DeletePromptTemplate delete prompt template (can only delete user-created templates, cannot delete system templates)
func (d *Database) DeletePromptTemplate(userID, id string) error {
	// Check if template exists and belongs to user, and is not a system template
	var isSystem bool
	var templateUserID string
	err := d.db.QueryRow(`
		SELECT user_id, is_system FROM prompt_templates WHERE id = ?
	`, id).Scan(&templateUserID, &isSystem)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("prompt template does not exist: %s", id)
		}
		return err
	}

	// Cannot delete system templates
	if isSystem {
		return fmt.Errorf("cannot delete system template: %s", id)
	}

	// Can only delete own templates
	if templateUserID != userID {
		return fmt.Errorf("no permission to delete this template: %s", id)
	}

	// Delete template
	_, err = d.db.Exec(`DELETE FROM prompt_templates WHERE id = ? AND user_id = ? AND is_system = 0`, id, userID)
	return err
}

// CreateTrader create trader
func (d *Database) CreateTrader(trader *TraderRecord) error {
	_, err := d.db.Exec(`
		INSERT INTO traders (id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running, btc_eth_leverage, altcoin_leverage, trading_symbols, use_coin_pool, use_oi_top, use_tradingview, followed_trader_id, custom_prompt, override_base_prompt, system_prompt_template, is_cross_margin)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, trader.ID, trader.UserID, trader.Name, trader.AIModelID, trader.ExchangeID, trader.InitialBalance, trader.ScanIntervalMinutes, trader.IsRunning, trader.BTCETHLeverage, trader.AltcoinLeverage, trader.TradingSymbols, trader.UseCoinPool, trader.UseOITop, trader.UseTradingView, trader.FollowedTraderID, trader.CustomPrompt, trader.OverrideBasePrompt, trader.SystemPromptTemplate, trader.IsCrossMargin)
	return err
}

// GetTraders get user's traders
func (d *Database) GetTraders(userID string) ([]*TraderRecord, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
		       COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
		       COALESCE(trading_symbols, '') as trading_symbols,
		       COALESCE(use_coin_pool, 0) as use_coin_pool, COALESCE(use_oi_top, 0) as use_oi_top,
		       COALESCE(use_tradingview, 0) as use_tradingview,
		       COALESCE(followed_trader_id, '') as followed_trader_id,
		       COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, 0) as override_base_prompt,
		       COALESCE(system_prompt_template, 'default') as system_prompt_template,
		       COALESCE(is_cross_margin, 1) as is_cross_margin, created_at, updated_at
		FROM traders WHERE user_id = ? ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traders []*TraderRecord
	for rows.Next() {
		var trader TraderRecord
		var createdAt, updatedAt string
		err := rows.Scan(
			&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
			&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
			&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
			&trader.UseCoinPool, &trader.UseOITop, &trader.UseTradingView,
			&trader.FollowedTraderID,
			&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
			&trader.IsCrossMargin,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		// Parse time string
		trader.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		trader.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		traders = append(traders, &trader)
	}

	return traders, nil
}

// GetFollowerTraders get all traders following specified trader list
func (d *Database) GetFollowerTraders(followedTraderID string) ([]*TraderRecord, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
		       COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
		       COALESCE(trading_symbols, '') as trading_symbols,
		       COALESCE(use_coin_pool, 0) as use_coin_pool, COALESCE(use_oi_top, 0) as use_oi_top,
		       COALESCE(use_tradingview, 0) as use_tradingview,
		       COALESCE(followed_trader_id, '') as followed_trader_id,
		       COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, 0) as override_base_prompt,
		       COALESCE(system_prompt_template, 'default') as system_prompt_template,
		       COALESCE(is_cross_margin, 1) as is_cross_margin, created_at, updated_at
		FROM traders 
		WHERE followed_trader_id = ? AND is_running = 1
		ORDER BY created_at DESC
	`, followedTraderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traders []*TraderRecord
	for rows.Next() {
		var trader TraderRecord
		var createdAt, updatedAt string
		err := rows.Scan(
			&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
			&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
			&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
			&trader.UseCoinPool, &trader.UseOITop, &trader.UseTradingView,
			&trader.FollowedTraderID,
			&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
			&trader.IsCrossMargin,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		// Parse time string
		trader.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		trader.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		traders = append(traders, &trader)
	}

	return traders, nil
}

// GetTraderFollowedTraderID get trader's followed_trader_id
func (d *Database) GetTraderFollowedTraderID(traderID string) (string, error) {
	var followedTraderID string
	err := d.db.QueryRow(`
		SELECT COALESCE(followed_trader_id, '') 
		FROM traders 
		WHERE id = ?
	`, traderID).Scan(&followedTraderID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("⚠️ DEBUG [GetTraderFollowedTraderID]: Trader %s not found in database", traderID)
			return "", nil
		}
		log.Printf("❌ DEBUG [GetTraderFollowedTraderID]: Error querying trader %s: %v", traderID, err)
		return "", err
	}
	if followedTraderID != "" {
		log.Printf("✓ DEBUG [GetTraderFollowedTraderID]: Trader %s has followed_trader_id: '%s'", traderID, followedTraderID)
	}
	return followedTraderID, nil
}

// UpdateTraderStatus update trader status
func (d *Database) UpdateTraderStatus(userID, id string, isRunning bool) error {
	_, err := d.db.Exec(`UPDATE traders SET is_running = ? WHERE id = ? AND user_id = ?`, isRunning, id, userID)
	return err
}

// UpdateTrader update trader configuration
func (d *Database) UpdateTrader(trader *TraderRecord) error {
	log.Printf("🔍 DEBUG [UpdateTrader DB]: Updating trader %s (user_id: %s) with system_prompt_template: '%s'", trader.ID, trader.UserID, trader.SystemPromptTemplate)
	result, err := d.db.Exec(`
		UPDATE traders SET
			name = ?, ai_model_id = ?, exchange_id = ?,
			scan_interval_minutes = ?, btc_eth_leverage = ?, altcoin_leverage = ?,
			trading_symbols = ?, use_coin_pool = ?, use_oi_top = ?, use_tradingview = ?,
			followed_trader_id = ?,
			custom_prompt = ?, override_base_prompt = ?,
			system_prompt_template = ?, is_cross_margin = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`, trader.Name, trader.AIModelID, trader.ExchangeID,
		trader.ScanIntervalMinutes, trader.BTCETHLeverage, trader.AltcoinLeverage,
		trader.TradingSymbols, trader.UseCoinPool, trader.UseOITop, trader.UseTradingView,
		trader.FollowedTraderID,
		trader.CustomPrompt, trader.OverrideBasePrompt,
		trader.SystemPromptTemplate, trader.IsCrossMargin, trader.ID, trader.UserID)
	if err != nil {
		log.Printf("❌ DEBUG [UpdateTrader DB]: Update failed for trader %s: %v", trader.ID, err)
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	log.Printf("✓ DEBUG [UpdateTrader DB]: Update succeeded for trader %s, rows affected: %d, system_prompt_template: '%s'", trader.ID, rowsAffected, trader.SystemPromptTemplate)
	return nil
}

// UpdateTraderCustomPrompt update trader custom prompt
func (d *Database) UpdateTraderCustomPrompt(userID, id string, customPrompt string, overrideBase bool) error {
	_, err := d.db.Exec(`UPDATE traders SET custom_prompt = ?, override_base_prompt = ? WHERE id = ? AND user_id = ?`, customPrompt, overrideBase, id, userID)
	return err
}

// UpdateTraderInitialBalance update trader initial balance (manual update only)
// ⚠️ Note: system will not automatically call this method, only for users to manually synchronize after deposit/withdrawal
func (d *Database) UpdateTraderInitialBalance(userID, id string, newBalance float64) error {
	_, err := d.db.Exec(`UPDATE traders SET initial_balance = ? WHERE id = ? AND user_id = ?`, newBalance, id, userID)
	return err
}

// DeleteTrader delete trader
func (d *Database) DeleteTrader(userID, id string) error {
	_, err := d.db.Exec(`DELETE FROM traders WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// GetTraderConfig get trader complete configuration (including AI model and exchange information)
func (d *Database) GetTraderConfig(userID, traderID string) (*TraderRecord, *AIModelConfig, *ExchangeConfig, error) {
	var trader TraderRecord
	var aiModel AIModelConfig
	var exchange ExchangeConfig
	var traderCreatedAt, traderUpdatedAt string
	var aiModelCreatedAt, aiModelUpdatedAt string
	var exchangeCreatedAt, exchangeUpdatedAt string

	err := d.db.QueryRow(`
		SELECT
			t.id, t.user_id, t.name, t.ai_model_id, t.exchange_id, t.initial_balance, t.scan_interval_minutes, t.is_running,
			COALESCE(t.btc_eth_leverage, 5) as btc_eth_leverage,
			COALESCE(t.altcoin_leverage, 5) as altcoin_leverage,
			COALESCE(t.trading_symbols, '') as trading_symbols,
			COALESCE(t.use_coin_pool, 0) as use_coin_pool,
			COALESCE(t.use_oi_top, 0) as use_oi_top,
			COALESCE(t.use_tradingview, 0) as use_tradingview,
			COALESCE(t.followed_trader_id, '') as followed_trader_id,
			COALESCE(t.custom_prompt, '') as custom_prompt,
			COALESCE(t.override_base_prompt, 0) as override_base_prompt,
			COALESCE(t.system_prompt_template, 'default') as system_prompt_template,
			COALESCE(t.is_cross_margin, 1) as is_cross_margin,
			t.created_at, t.updated_at,
			a.id, a.user_id, a.name, a.provider, a.enabled, a.api_key,
			COALESCE(a.custom_api_url, '') as custom_api_url,
			COALESCE(a.custom_model_name, '') as custom_model_name,
			a.created_at, a.updated_at,
			e.id, e.user_id, e.name, e.type, e.enabled, e.api_key, e.secret_key, e.testnet,
			COALESCE(e.hyperliquid_wallet_addr, '') as hyperliquid_wallet_addr,
			COALESCE(e.aster_user, '') as aster_user,
			COALESCE(e.aster_signer, '') as aster_signer,
			COALESCE(e.aster_private_key, '') as aster_private_key,
			COALESCE(e.lighter_wallet_addr, '') as lighter_wallet_addr,
			COALESCE(e.lighter_private_key, '') as lighter_private_key,
			COALESCE(e.lighter_api_key_private_key, '') as lighter_api_key_private_key,
			COALESCE(e.okx_passphrase, '') as okx_passphrase,
			e.created_at, e.updated_at
		FROM traders t
		JOIN ai_models a ON t.ai_model_id = a.id AND t.user_id = a.user_id
		JOIN exchanges e ON t.exchange_id = e.id AND t.user_id = e.user_id
		WHERE t.id = ? AND t.user_id = ?
	`, traderID, userID).Scan(
		&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
		&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
		&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
		&trader.UseCoinPool, &trader.UseOITop, &trader.UseTradingView,
		&trader.FollowedTraderID,
		&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
		&trader.IsCrossMargin,
		&traderCreatedAt, &traderUpdatedAt,
		&aiModel.ID, &aiModel.UserID, &aiModel.Name, &aiModel.Provider, &aiModel.Enabled, &aiModel.APIKey,
		&aiModel.CustomAPIURL, &aiModel.CustomModelName,
		&aiModelCreatedAt, &aiModelUpdatedAt,
		&exchange.ID, &exchange.UserID, &exchange.Name, &exchange.Type, &exchange.Enabled,
		&exchange.APIKey, &exchange.SecretKey, &exchange.Testnet,
		&exchange.HyperliquidWalletAddr, &exchange.AsterUser, &exchange.AsterSigner, &exchange.AsterPrivateKey,
		&exchange.LighterWalletAddr, &exchange.LighterPrivateKey, &exchange.LighterAPIKeyPrivateKey,
		&exchange.OkxPassphrase,
		&exchangeCreatedAt, &exchangeUpdatedAt,
	)

	if err != nil {
		return nil, nil, nil, err
	}

	// Parse time string
	trader.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", traderCreatedAt)
	trader.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", traderUpdatedAt)
	aiModel.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", aiModelCreatedAt)
	aiModel.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", aiModelUpdatedAt)
	exchange.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", exchangeCreatedAt)
	exchange.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", exchangeUpdatedAt)

	// Decrypt sensitive data
	aiModel.APIKey = d.decryptSensitiveData(aiModel.APIKey)
	exchange.APIKey = d.decryptSensitiveData(exchange.APIKey)
	exchange.SecretKey = d.decryptSensitiveData(exchange.SecretKey)
	exchange.AsterPrivateKey = d.decryptSensitiveData(exchange.AsterPrivateKey)
	exchange.LighterPrivateKey = d.decryptSensitiveData(exchange.LighterPrivateKey)
	exchange.LighterAPIKeyPrivateKey = d.decryptSensitiveData(exchange.LighterAPIKeyPrivateKey)
	exchange.OkxPassphrase = d.decryptSensitiveData(exchange.OkxPassphrase)

	return &trader, &aiModel, &exchange, nil
}

// GetSystemConfig get system configuration
func (d *Database) GetSystemConfig(key string) (string, error) {
	var value string
	err := d.db.QueryRow(`SELECT value FROM system_config WHERE key = ?`, key).Scan(&value)
	return value, err
}

// SetSystemConfig set system configuration
func (d *Database) SetSystemConfig(key, value string) error {
	_, err := d.db.Exec(`
		INSERT OR REPLACE INTO system_config (key, value) VALUES (?, ?)
	`, key, value)
	return err
}

// CreateUserSignalSource create user signal source configuration
func (d *Database) CreateUserSignalSource(userID, coinPoolURL, oiTopURL string) error {
	_, err := d.db.Exec(`
		INSERT OR REPLACE INTO user_signal_sources (user_id, coin_pool_url, oi_top_url, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, userID, coinPoolURL, oiTopURL)
	return err
}

// GetUserSignalSource get user signal source configuration
func (d *Database) GetUserSignalSource(userID string) (*UserSignalSource, error) {
	var source UserSignalSource
	var createdAt, updatedAt string
	err := d.db.QueryRow(`
		SELECT id, user_id, coin_pool_url, oi_top_url, created_at, updated_at
		FROM user_signal_sources WHERE user_id = ?
	`, userID).Scan(
		&source.ID, &source.UserID, &source.CoinPoolURL, &source.OITopURL,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	source.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	source.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &source, nil
}

// UpdateUserSignalSource update user signal source configuration
func (d *Database) UpdateUserSignalSource(userID, coinPoolURL, oiTopURL string) error {
	_, err := d.db.Exec(`
		UPDATE user_signal_sources SET coin_pool_url = ?, oi_top_url = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ?
	`, coinPoolURL, oiTopURL, userID)
	return err
}

// GenerateWebhookAPIKey generate or get user's Webhook API Key
func (d *Database) GenerateWebhookAPIKey(userID string) (string, error) {
	// First check if already exists
	var existingKey string
	err := d.db.QueryRow(`SELECT api_key FROM webhook_api_keys WHERE user_id = ?`, userID).Scan(&existingKey)
	if err == nil {
		return existingKey, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("failed to query API key: %w", err)
	}

	// Generate 64-character hex API key
	bytes := make([]byte, 32) // 32 bytes = 64 hex characters
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random number: %w", err)
	}
	apiKey := fmt.Sprintf("%x", bytes)

	// Insert into database
	_, err = d.db.Exec(`
		INSERT INTO webhook_api_keys (user_id, api_key, created_at, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, userID, apiKey)
	if err != nil {
		return "", fmt.Errorf("failed to save API key: %w", err)
	}

	return apiKey, nil
}

// GetUserByWebhookAPIKey get user by API key
func (d *Database) GetUserByWebhookAPIKey(apiKey string) (*User, error) {
	var userID string
	err := d.db.QueryRow(`SELECT user_id FROM webhook_api_keys WHERE api_key = ?`, apiKey).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("invalid API key: %w", err)
	}

	return d.GetUserByID(userID)
}

// CreateTradingViewAlert create TradingView alert, return alert ID
func (d *Database) CreateTradingViewAlert(userID, traderID string, payload map[string]interface{}) (string, error) {
	// Generate UUID
	id := fmt.Sprintf("%d", time.Now().UnixNano())

	// Parse payload field
	rawPayloadBytes, _ := json.Marshal(payload)
	rawPayload := string(rawPayloadBytes)

	symbol, _ := payload["symbol"].(string)
	action, _ := payload["action"].(string)
	exchange, _ := payload["exchange"].(string)
	pricetype, _ := payload["pricetype"].(string)

	// Parse numeric fields
	entry := parseFloat(payload["entry"])
	sl := parseFloat(payload["sl"])
	tp := parseFloat(payload["tp"])
	quantity := parseFloat(payload["quantity"])
	positionSize := parseFloat(payload["position_size"])

	_, err := d.db.Exec(`
		INSERT INTO tradingview_alerts (
			id, user_id, trader_id, raw_payload, symbol, action, exchange,
			entry, sl, tp, quantity, position_size, pricetype, status, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', CURRENT_TIMESTAMP)
	`, id, userID, traderID, rawPayload, symbol, action, exchange,
		entry, sl, tp, quantity, positionSize, pricetype)
	if err != nil {
		return "", err
	}
	return id, nil
}

// parseFloat helper function: parse float64 from interface{}
func parseFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}

// GetPendingTradingViewAlerts get pending alerts for specified trader
func (d *Database) GetPendingTradingViewAlerts(traderID string) ([]TradingViewAlert, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, trader_id, raw_payload, symbol, action, exchange,
			entry, sl, tp, quantity, position_size, pricetype, status,
			created_at, processed_at
		FROM tradingview_alerts
		WHERE trader_id = ? AND status = 'pending'
		ORDER BY created_at ASC
	`, traderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []TradingViewAlert
	for rows.Next() {
		var alert TradingViewAlert
		var createdAt string
		var processedAt sql.NullString

		err := rows.Scan(
			&alert.ID, &alert.UserID, &alert.TraderID, &alert.RawPayload,
			&alert.Symbol, &alert.Action, &alert.Exchange,
			&alert.Entry, &alert.SL, &alert.TP, &alert.Quantity,
			&alert.PositionSize, &alert.PriceType, &alert.Status,
			&createdAt, &processedAt,
		)
		if err != nil {
			continue
		}

		alert.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		if processedAt.Valid {
			t, _ := time.Parse("2006-01-02 15:04:05", processedAt.String)
			alert.ProcessedAt = &t
		}

		alerts = append(alerts, alert)
	}

	return alerts, nil
}

// UpdateAlertStatus update alert status
func (d *Database) UpdateAlertStatus(alertID string, status string) error {
	now := time.Now()
	_, err := d.db.Exec(`
		UPDATE tradingview_alerts
		SET status = ?, processed_at = ?
		WHERE id = ?
	`, status, now, alertID)
	return err
}

// GetTradingViewAlertByID get TradingView alert by ID
func (d *Database) GetTradingViewAlertByID(alertID string) (*TradingViewAlert, error) {
	var alert TradingViewAlert
	var createdAt string
	var processedAt sql.NullString

	err := d.db.QueryRow(`
		SELECT id, user_id, trader_id, raw_payload, symbol, action, exchange,
			entry, sl, tp, quantity, position_size, pricetype, status,
			created_at, processed_at
		FROM tradingview_alerts
		WHERE id = ?
	`, alertID).Scan(
		&alert.ID, &alert.UserID, &alert.TraderID, &alert.RawPayload,
		&alert.Symbol, &alert.Action, &alert.Exchange,
		&alert.Entry, &alert.SL, &alert.TP, &alert.Quantity,
		&alert.PositionSize, &alert.PriceType, &alert.Status,
		&createdAt, &processedAt,
	)
	if err != nil {
		return nil, err
	}

	alert.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	if processedAt.Valid {
		t, _ := time.Parse("2006-01-02 15:04:05", processedAt.String)
		alert.ProcessedAt = &t
	}

	return &alert, nil
}

// GetRecentTradingViewAlerts get recent TradingView alerts
func (d *Database) GetRecentTradingViewAlerts(userID string, traderID string, limit int) ([]TradingViewAlert, error) {
	query := `
		SELECT id, user_id, trader_id, raw_payload, symbol, action, exchange,
			entry, sl, tp, quantity, position_size, pricetype, status,
			created_at, processed_at
		FROM tradingview_alerts
		WHERE user_id = ?
	`
	args := []interface{}{userID}

	if traderID != "" {
		query += " AND trader_id = ?"
		args = append(args, traderID)
	}

	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []TradingViewAlert
	for rows.Next() {
		var alert TradingViewAlert
		var createdAt string
		var processedAt sql.NullString

		err := rows.Scan(
			&alert.ID, &alert.UserID, &alert.TraderID, &alert.RawPayload,
			&alert.Symbol, &alert.Action, &alert.Exchange,
			&alert.Entry, &alert.SL, &alert.TP, &alert.Quantity,
			&alert.PositionSize, &alert.PriceType, &alert.Status,
			&createdAt, &processedAt,
		)
		if err != nil {
			continue
		}

		alert.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		if processedAt.Valid {
			t, _ := time.Parse("2006-01-02 15:04:05", processedAt.String)
			alert.ProcessedAt = &t
		}

		alerts = append(alerts, alert)
	}

	return alerts, nil
}

// GetTradersWithTradingViewEnabled get user's traders with TradingView enabled list
func (d *Database) GetTradersWithTradingViewEnabled(userID string) ([]*TraderRecord, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance,
			scan_interval_minutes, is_running, btc_eth_leverage, altcoin_leverage,
			trading_symbols, use_coin_pool, use_oi_top, use_tradingview,
			custom_prompt, override_base_prompt, system_prompt_template, is_cross_margin,
			created_at, updated_at
		FROM traders
		WHERE user_id = ? AND use_tradingview = 1
		ORDER BY created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traders []*TraderRecord
	for rows.Next() {
		var trader TraderRecord
		var createdAt, updatedAt string

		err := rows.Scan(
			&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID,
			&trader.ExchangeID, &trader.InitialBalance, &trader.ScanIntervalMinutes,
			&trader.IsRunning, &trader.BTCETHLeverage, &trader.AltcoinLeverage,
			&trader.TradingSymbols, &trader.UseCoinPool, &trader.UseOITop,
			&trader.UseTradingView, &trader.CustomPrompt, &trader.OverrideBasePrompt,
			&trader.SystemPromptTemplate, &trader.IsCrossMargin,
			&createdAt, &updatedAt,
		)
		if err != nil {
			continue
		}

		trader.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		trader.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		traders = append(traders, &trader)
	}

	return traders, nil
}

// GetCustomCoins get all trader-customized currencies
func (d *Database) GetCustomCoins() []string {
	var symbol string
	var symbols []string
	_ = d.db.QueryRow(`
		SELECT GROUP_CONCAT(custom_coins , ',') as symbol
		FROM main.traders where custom_coins != ''
	`).Scan(&symbol)
	// Detect if user has not configured currencies - compatibility
	if symbol == "" {
		symbolJSON, _ := d.GetSystemConfig("default_coins")
		if err := json.Unmarshal([]byte(symbolJSON), &symbols); err != nil {
			log.Printf("⚠️  Failed to parse default_coins configuration: %v, using hardcoded default values", err)
			symbols = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"}
		}
	}
	// filter Symbol
	for _, s := range strings.Split(symbol, ",") {
		if s == "" {
			continue
		}
		coin := market.Normalize(s)
		if !slices.Contains(symbols, coin) {
			symbols = append(symbols, coin)
		}
	}
	return symbols
}

// Close close database connection
// Conn return underlying *sql.DB for modules that need to execute custom queries
func (d *Database) Conn() *sql.DB {
	return d.db
}

func (d *Database) Close() error {
	return d.db.Close()
}

// LoadBetaCodesFromFile load beta codes from file to database
func (d *Database) LoadBetaCodesFromFile(filePath string) error {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read beta code file: %w", err)
	}

	// Split beta codes by line
	lines := strings.Split(string(content), "\n")
	var codes []string
	for _, line := range lines {
		code := strings.TrimSpace(line)
		if code != "" && !strings.HasPrefix(code, "#") {
			codes = append(codes, code)
		}
	}

	// Batch insert beta codes
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO beta_codes (code) VALUES (?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	insertedCount := 0
	for _, code := range codes {
		result, err := stmt.Exec(code)
		if err != nil {
			log.Printf("Failed to insert beta code %s: %v", code, err)
			continue
		}

		if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
			insertedCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("✅ Successfully loaded %d beta codes to database (total %d)", insertedCount, len(codes))
	return nil
}

// ValidateBetaCode validate if beta code is valid and unused
func (d *Database) ValidateBetaCode(code string) (bool, error) {
	var used bool
	err := d.db.QueryRow(`SELECT used FROM beta_codes WHERE code = ?`, code).Scan(&used)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // Beta code does not exist
		}
		return false, err
	}
	return !used, nil // Beta code exists and is unused
}

// UseBetaCode use beta code (mark as used)
func (d *Database) UseBetaCode(code, userEmail string) error {
	result, err := d.db.Exec(`
		UPDATE beta_codes SET used = 1, used_by = ?, used_at = CURRENT_TIMESTAMP 
		WHERE code = ? AND used = 0
	`, userEmail, code)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("beta code is invalid or already used")
	}

	return nil
}

// GetBetaCodeStats get beta code statistics
func (d *Database) GetBetaCodeStats() (total, used int, err error) {
	err = d.db.QueryRow(`SELECT COUNT(*) FROM beta_codes`).Scan(&total)
	if err != nil {
		return 0, 0, err
	}

	err = d.db.QueryRow(`SELECT COUNT(*) FROM beta_codes WHERE used = 1`).Scan(&used)
	if err != nil {
		return 0, 0, err
	}

	return total, used, nil
}

// SetCryptoService set encryption service
func (d *Database) SetCryptoService(cs *crypto.CryptoService) {
	d.cryptoService = cs
}

// encryptSensitiveData encrypt sensitive data for storage
func (d *Database) encryptSensitiveData(plaintext string) string {
	if d.cryptoService == nil || plaintext == "" {
		return plaintext
	}

	encrypted, err := d.cryptoService.EncryptForStorage(plaintext)
	if err != nil {
		log.Printf("⚠️ Encryption failed: %v", err)
		return plaintext // Return plaintext as fallback
	}

	return encrypted
}

// decryptSensitiveData decrypt sensitive data
func (d *Database) decryptSensitiveData(encrypted string) string {
	if d.cryptoService == nil || encrypted == "" {
		return encrypted
	}

	// If not encrypted format, return directly
	if !d.cryptoService.IsEncryptedStorageValue(encrypted) {
		return encrypted
	}

	decrypted, err := d.cryptoService.DecryptFromStorage(encrypted)
	if err != nil {
		log.Printf("⚠️ Decryption failed: %v", err)
		return encrypted // Return encrypted text as fallback
	}

	return decrypted
}
