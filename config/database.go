package config

import (
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"nofx/crypto"
	"nofx/market"
	"os"
	"path/filepath"
	"slices"
	"strconv"
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
	UpdateExchange(userID, id string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey, lighterWalletAddr, lighterPrivateKey, lighterAPIKeyPrivateKey string, lighterAPIKeyIndex int, okxPassphrase string) error
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
	GetTraderByID(traderID string) (*TraderRecord, error)
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
	// Ensure data directory exists for database persistence (especially in Docker)
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
	}

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

	// Migrate max_users from old default "1" to new default "0" (unlimited)
	if err := database.migrateMaxUsers(); err != nil {
		log.Printf("⚠️  Max users migration failed: %v", err)
		// Don't return error, allow system to continue running
	}

	// Migrate prompt templates from files to database
	if err := database.migratePromptTemplatesFromFiles(); err != nil {
		log.Printf("⚠️  Prompt template migration failed: %v", err)
		// Don't return error, allow system to continue running
	}

	// Migrate trader_positions table to add SL/TP price columns
	if err := database.migrateTraderPositionsSLTP(); err != nil {
		log.Printf("⚠️  Trader positions SL/TP migration failed: %v", err)
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
			lighter_api_key_index INTEGER DEFAULT 0,
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
			enable_raw_klines BOOLEAN DEFAULT 1,
			enable_ema BOOLEAN DEFAULT 0,
			enable_macd BOOLEAN DEFAULT 0,
			enable_rsi BOOLEAN DEFAULT 0,
			enable_atr BOOLEAN DEFAULT 0,
			enable_volume BOOLEAN DEFAULT 1,
			enable_oi BOOLEAN DEFAULT 1,
			enable_funding BOOLEAN DEFAULT 1,
			indicator_timeframe TEXT DEFAULT '3m',
			quant_data_url TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
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

		// Strategy table
		`CREATE TABLE IF NOT EXISTS strategies (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			system_prompt_template TEXT DEFAULT '',
			custom_prompt TEXT DEFAULT '',
			override_base_prompt BOOLEAN DEFAULT 0,
			btc_eth_leverage INTEGER DEFAULT 5,
			altcoin_leverage INTEGER DEFAULT 5,
			trading_symbols TEXT DEFAULT '',
			is_cross_margin BOOLEAN DEFAULT 1,
			use_coin_pool BOOLEAN DEFAULT 0,
			use_oi_top BOOLEAN DEFAULT 0,
			use_tradingview BOOLEAN DEFAULT 0,
			enable_raw_klines BOOLEAN DEFAULT 1,
			enable_ema BOOLEAN DEFAULT 0,
			enable_macd BOOLEAN DEFAULT 0,
			enable_rsi BOOLEAN DEFAULT 0,
			enable_atr BOOLEAN DEFAULT 0,
			enable_volume BOOLEAN DEFAULT 1,
			enable_oi BOOLEAN DEFAULT 1,
			enable_funding BOOLEAN DEFAULT 1,
			indicator_timeframe TEXT DEFAULT '3m',
			quant_data_url TEXT DEFAULT '',
			min_risk_reward_ratio REAL DEFAULT 3.0,
			max_positions INTEGER DEFAULT 3,
			margin_usage_limit REAL DEFAULT 90.0,
			min_opening_amount REAL DEFAULT 12.0,
			min_opening_amount_btc_eth REAL DEFAULT 60.0,
			altcoin_position_min REAL DEFAULT 0.8,
			altcoin_position_max REAL DEFAULT 1.5,
			btc_eth_position_min REAL DEFAULT 5.0,
			btc_eth_position_max REAL DEFAULT 10.0,
			available_margin_multiplier REAL DEFAULT 0.88,
			min_confidence_for_entry INTEGER DEFAULT 75,
			min_holding_time_minutes INTEGER DEFAULT 30,
			sharpe_ratio_config TEXT DEFAULT '',
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

		// Trader positions table
		`CREATE TABLE IF NOT EXISTS trader_positions (
			id TEXT PRIMARY KEY,
			trader_id TEXT NOT NULL,
			symbol TEXT NOT NULL,
			side TEXT NOT NULL,
			entry_price REAL NOT NULL,
			exit_price REAL,
			quantity REAL NOT NULL,
			entry_fee REAL DEFAULT 0,
			exit_fee REAL DEFAULT 0,
			realized_pnl REAL,
			leverage INTEGER DEFAULT 1,
			opened_at DATETIME NOT NULL,
			closed_at DATETIME,
			order_id_open TEXT,
			order_id_close TEXT,
			stop_loss_price REAL,
			take_profit_price REAL,
			FOREIGN KEY (trader_id) REFERENCES traders(id) ON DELETE CASCADE
		)`,

		// Pending orders table - stores unfilled limit/SL/TP orders separately from positions
		// These orders should NOT appear in closed positions or affect realized PnL calculations
		`CREATE TABLE IF NOT EXISTS pending_orders (
			id TEXT PRIMARY KEY,
			trader_id TEXT NOT NULL,
			symbol TEXT NOT NULL,
			side TEXT NOT NULL,
			order_type TEXT NOT NULL,
			trigger_price REAL,
			quantity REAL NOT NULL,
			filled_quantity REAL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'pending',
			parent_position_id TEXT,
			exchange_order_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			filled_at DATETIME,
			cancelled_at DATETIME,
			FOREIGN KEY (trader_id) REFERENCES traders(id) ON DELETE CASCADE
		)`,

		// Trader equity history table (for persistent equity curve data)
		`CREATE TABLE IF NOT EXISTS trader_equity_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trader_id TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			total_equity REAL NOT NULL,
			available_balance REAL NOT NULL,
			total_pnl REAL NOT NULL,
			total_pnl_pct REAL NOT NULL,
			position_count INTEGER DEFAULT 0,
			margin_used_pct REAL DEFAULT 0,
			cycle_number INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (trader_id) REFERENCES traders(id) ON DELETE CASCADE
		)`,

		// Articles table
		`CREATE TABLE IF NOT EXISTS articles (
			id TEXT PRIMARY KEY,
			slug TEXT UNIQUE NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			excerpt TEXT DEFAULT '',
			featured_image_url TEXT DEFAULT '',
			author_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'draft',
			meta_title TEXT DEFAULT '',
			meta_description TEXT DEFAULT '',
			meta_keywords TEXT DEFAULT '',
			og_image_url TEXT DEFAULT '',
			published_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// Indexes
		`CREATE INDEX IF NOT EXISTS idx_tradingview_alerts_user ON tradingview_alerts(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tradingview_alerts_trader ON tradingview_alerts(trader_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tradingview_alerts_status ON tradingview_alerts(status)`,
		`CREATE INDEX IF NOT EXISTS idx_trader_applications_user_id ON trader_applications(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_trader_applications_status ON trader_applications(status)`,
		`CREATE INDEX IF NOT EXISTS idx_trader_positions_trader ON trader_positions(trader_id)`,
		`CREATE INDEX IF NOT EXISTS idx_trader_positions_symbol ON trader_positions(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_trader_positions_closed_at ON trader_positions(closed_at)`,
		// Composite index for faster duplicate position closure detection
		`CREATE INDEX IF NOT EXISTS idx_trader_positions_closure_check ON trader_positions(trader_id, symbol, side, closed_at)`,
		`CREATE INDEX IF NOT EXISTS idx_trader_equity_history_trader_ts ON trader_equity_history(trader_id, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_trader_equity_history_cycle ON trader_equity_history(trader_id, cycle_number)`,
		// Pending orders indexes
		`CREATE INDEX IF NOT EXISTS idx_pending_orders_trader ON pending_orders(trader_id)`,
		`CREATE INDEX IF NOT EXISTS idx_pending_orders_symbol ON pending_orders(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_pending_orders_status ON pending_orders(status)`,
		`CREATE INDEX IF NOT EXISTS idx_pending_orders_parent ON pending_orders(parent_position_id)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_slug ON articles(slug)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_status_published ON articles(status, published_at)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_author ON articles(author_id)`,

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
		{"exchanges", "lighter_api_key_index", `ALTER TABLE exchanges ADD COLUMN lighter_api_key_index INTEGER DEFAULT 0`},
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
		// Indicator configuration fields
		{"traders", "enable_raw_klines", `ALTER TABLE traders ADD COLUMN enable_raw_klines BOOLEAN DEFAULT 1`},
		{"traders", "enable_ema", `ALTER TABLE traders ADD COLUMN enable_ema BOOLEAN DEFAULT 0`},
		{"traders", "enable_macd", `ALTER TABLE traders ADD COLUMN enable_macd BOOLEAN DEFAULT 0`},
		{"traders", "enable_rsi", `ALTER TABLE traders ADD COLUMN enable_rsi BOOLEAN DEFAULT 0`},
		{"traders", "enable_atr", `ALTER TABLE traders ADD COLUMN enable_atr BOOLEAN DEFAULT 0`},
		{"traders", "enable_volume", `ALTER TABLE traders ADD COLUMN enable_volume BOOLEAN DEFAULT 1`},
		{"traders", "enable_oi", `ALTER TABLE traders ADD COLUMN enable_oi BOOLEAN DEFAULT 1`},
		{"traders", "enable_funding", `ALTER TABLE traders ADD COLUMN enable_funding BOOLEAN DEFAULT 1`},
		{"traders", "indicator_timeframe", `ALTER TABLE traders ADD COLUMN indicator_timeframe TEXT DEFAULT '3m'`},
		{"traders", "quant_data_url", `ALTER TABLE traders ADD COLUMN quant_data_url TEXT DEFAULT ''`},
		{"traders", "show_in_competition", `ALTER TABLE traders ADD COLUMN show_in_competition BOOLEAN DEFAULT 1`},
		{"traders", "strategy_id", `ALTER TABLE traders ADD COLUMN strategy_id TEXT`},
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
	log.Printf("🔄 Checking exchanges table migration status...")
	err := d.migrateExchangesTable()
	if err != nil {
		log.Printf("❌ Failed to migrate exchanges table: %v", err)
		log.Printf("⚠️  System will continue but exchange operations may fail. Please check database structure.")
		// Don't return error - allow system to continue, UpdateExchange will handle gracefully
	} else {
		log.Printf("✅ Exchanges table migration check completed successfully")
	}

	// Fix foreign key constraint issues in traders table
	err = d.migrateTradersTable()
	if err != nil {
		log.Printf("⚠️  Failed to migrate traders table: %v", err)
	}

	// Migrate strategies table to add new configuration columns
	err = d.migrateStrategiesTable()
	if err != nil {
		log.Printf("⚠️  Failed to migrate strategies table: %v", err)
		// Don't return error - allow system to continue, but log the issue
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
		"max_users":            "0",                                                                                   // Maximum number of users allowed (0 = unlimited, default = 0)
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
	// Check if migration is in progress (exchanges_new exists)
	var newTableExists int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type='table' AND name='exchanges_new'
	`).Scan(&newTableExists)
	if err != nil {
		return err
	}

	// If migration is in progress, try to complete it
	if newTableExists > 0 {
		log.Printf("⚠️  Migration in progress (exchanges_new exists), attempting to complete...")
		// Try to complete the migration by copying data, dropping old table, and renaming
		var exchangesExists int
		err = d.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='exchanges'`).Scan(&exchangesExists)
		if err == nil && exchangesExists > 0 {
			// Old table still exists, complete the migration
			log.Printf("🔄 Completing partial migration...")
			// Copy any new data that might have been added
			_, err = d.db.Exec(`
				INSERT OR IGNORE INTO exchanges_new (id, user_id, name, type, enabled, api_key, secret_key, testnet,
				                           hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key,
				                           lighter_wallet_addr, lighter_private_key, lighter_api_key_private_key, lighter_api_key_index, okx_passphrase, created_at, updated_at)
				SELECT id, COALESCE(user_id, 'default'), name, type, enabled, api_key, secret_key, testnet,
				       COALESCE(hyperliquid_wallet_addr, ''), COALESCE(aster_user, ''), COALESCE(aster_signer, ''),
				       COALESCE(aster_private_key, ''), COALESCE(lighter_wallet_addr, ''), COALESCE(lighter_private_key, ''),
				       COALESCE(lighter_api_key_private_key, ''), COALESCE(lighter_api_key_index, 0), COALESCE(okx_passphrase, ''),
				       COALESCE(created_at, datetime('now')), COALESCE(updated_at, datetime('now'))
				FROM exchanges
			`)
			if err != nil {
				log.Printf("⚠️  Failed to copy data during migration completion: %v", err)
			}
			// Drop old table and rename
			_, err = d.db.Exec(`DROP TABLE IF EXISTS exchanges`)
			if err != nil {
				log.Printf("⚠️  Failed to drop old table: %v", err)
				return fmt.Errorf("failed to complete migration: %w", err)
			}
			_, err = d.db.Exec(`ALTER TABLE exchanges_new RENAME TO exchanges`)
			if err != nil {
				log.Printf("⚠️  Failed to rename table: %v", err)
				return fmt.Errorf("failed to complete migration: %w", err)
			}
			log.Printf("✅ Completed partial migration successfully")
			return nil
		} else {
			// Old table doesn't exist, just rename
			_, err = d.db.Exec(`ALTER TABLE exchanges_new RENAME TO exchanges`)
			if err == nil {
				log.Printf("✅ Completed migration by renaming exchanges_new")
				return nil
			}
		}
		// If we get here, migration is stuck - return error to force retry
		return fmt.Errorf("migration appears stuck (exchanges_new exists but migration incomplete)")
	}

	// Check if exchanges table exists
	var exchangesExists int
	err = d.db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type='table' AND name='exchanges'
	`).Scan(&exchangesExists)
	if err != nil {
		return err
	}

	// If exchanges table exists, check if it has composite primary key
	if exchangesExists > 0 {
		// Check if table has composite primary key by querying sqlite_master
		var tableSQL string
		err = d.db.QueryRow(`
			SELECT sql FROM sqlite_master 
			WHERE type='table' AND name='exchanges'
		`).Scan(&tableSQL)
		if err != nil {
			log.Printf("⚠️  Failed to get table SQL: %v", err)
			return err
		}

		// Check if PRIMARY KEY (id, user_id) exists in table definition
		// SQLite may format this differently, so check multiple patterns
		hasCompositeKey := strings.Contains(tableSQL, "PRIMARY KEY (id, user_id)") ||
			strings.Contains(tableSQL, "PRIMARY KEY(id,user_id)") ||
			strings.Contains(tableSQL, "PRIMARY KEY(id, user_id)") ||
			strings.Contains(tableSQL, "PRIMARY KEY (id,user_id)")

		if hasCompositeKey {
			// Migration already completed, just ensure okx_passphrase column exists
			log.Printf("✅ Exchanges table already has composite primary key, migration not needed")
			exists, err := d.columnExists("exchanges", "okx_passphrase")
			if err != nil {
				log.Printf("⚠️  Error checking if okx_passphrase column exists: %v", err)
			} else if !exists {
				log.Printf("🔄 Adding missing okx_passphrase column to exchanges table (post-migration)...")
				_, err = d.db.Exec(`ALTER TABLE exchanges ADD COLUMN okx_passphrase TEXT DEFAULT ''`)
				if err != nil {
					log.Printf("⚠️  Failed to add okx_passphrase column: %v", err)
				} else {
					log.Printf("✅ Successfully added okx_passphrase column")
				}
			}
			return nil
		}
		// Table exists but doesn't have composite primary key - need to migrate
		log.Printf("🔄 Exchanges table exists but needs migration to composite primary key...")
		log.Printf("📋 Current table structure: %s", tableSQL)
	}

	log.Printf("🔄 Starting exchanges table migration...")
	log.Printf("📋 This will convert exchanges table from single PRIMARY KEY to composite PRIMARY KEY (id, user_id)")

	// Drop exchanges_new if it exists (cleanup from previous failed migration)
	_, _ = d.db.Exec(`DROP TABLE IF EXISTS exchanges_new`)

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
			lighter_api_key_index INTEGER DEFAULT 0,
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
	log.Printf("✅ Created exchanges_new table with composite primary key")

	// Copy data to new table - handle case where old table might not have all columns
	if exchangesExists > 0 {
		log.Printf("📋 Copying data from old exchanges table to new table...")
		// Use explicit column list with COALESCE to handle missing columns gracefully
		// Handle duplicate IDs by appending user_id - if user_id is NULL or missing, use 'default'
		result, err := d.db.Exec(`
			INSERT INTO exchanges_new (id, user_id, name, type, enabled, api_key, secret_key, testnet,
			                           hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key,
			                           lighter_wallet_addr, lighter_private_key, lighter_api_key_private_key, lighter_api_key_index, okx_passphrase, created_at, updated_at)
			SELECT id, COALESCE(user_id, 'default'), name, type, enabled, api_key, secret_key, testnet,
			       COALESCE(hyperliquid_wallet_addr, ''), COALESCE(aster_user, ''), COALESCE(aster_signer, ''),
			       COALESCE(aster_private_key, ''), COALESCE(lighter_wallet_addr, ''), COALESCE(lighter_private_key, ''),
			       COALESCE(lighter_api_key_private_key, ''), COALESCE(lighter_api_key_index, 0), COALESCE(okx_passphrase, ''),
			       COALESCE(created_at, datetime('now')), COALESCE(updated_at, datetime('now'))
			FROM exchanges
		`)
		if err != nil {
			log.Printf("❌ Failed to copy data: %v", err)
			// Try to clean up
			_, _ = d.db.Exec(`DROP TABLE IF EXISTS exchanges_new`)
			return fmt.Errorf("failed to copy data: %w", err)
		}
		rowsAffected, _ := result.RowsAffected()
		log.Printf("✅ Copied %d rows from old table to new table", rowsAffected)
	}

	// Delete old table (only if it existed)
	if exchangesExists > 0 {
		log.Printf("🗑️  Dropping old exchanges table...")
		_, err = d.db.Exec(`DROP TABLE exchanges`)
		if err != nil {
			log.Printf("❌ Failed to drop old table: %v", err)
			// Try to clean up
			_, _ = d.db.Exec(`DROP TABLE IF EXISTS exchanges_new`)
			return fmt.Errorf("failed to delete old table: %w", err)
		}
		log.Printf("✅ Old table dropped successfully")
	}

	// Rename new table
	log.Printf("🔄 Renaming exchanges_new to exchanges...")
	_, err = d.db.Exec(`ALTER TABLE exchanges_new RENAME TO exchanges`)
	if err != nil {
		log.Printf("❌ Failed to rename table: %v", err)
		return fmt.Errorf("failed to rename table: %w", err)
	}
	log.Printf("✅ Table renamed successfully")

	// Ensure okx_passphrase column exists (for databases migrated before this column was added)
	exists, err := d.columnExists("exchanges", "okx_passphrase")
	if err != nil {
		log.Printf("⚠️  Error checking if okx_passphrase column exists: %v", err)
	} else if !exists {
		log.Printf("🔄 Adding missing okx_passphrase column to exchanges table...")
		_, err = d.db.Exec(`ALTER TABLE exchanges ADD COLUMN okx_passphrase TEXT DEFAULT ''`)
		if err != nil {
			log.Printf("⚠️  Failed to add okx_passphrase column: %v", err)
			// Don't return error, continue with migration
		} else {
			log.Printf("✅ Successfully added okx_passphrase column")
		}
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

	// Check if contains FOREIGN KEY constraint on user_id
	// This constraint causes issues when database is reset while user has cached JWT token
	if !strings.Contains(tableSQL, "FOREIGN KEY (user_id)") {
		// No foreign key constraint on user_id, no migration needed
		return nil
	}

	log.Printf("🔄 Starting traders table migration, removing foreign key constraint on user_id...")

	// Create new traders table without foreign key constraint on user_id
	// Include all columns to match current CREATE TABLE structure
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
			use_tradingview BOOLEAN DEFAULT 0,
			followed_trader_id TEXT,
			custom_prompt TEXT DEFAULT '',
			override_base_prompt BOOLEAN DEFAULT 0,
			system_prompt_template TEXT DEFAULT 'default',
			is_cross_margin BOOLEAN DEFAULT 1,
			enable_raw_klines BOOLEAN DEFAULT 1,
			enable_ema BOOLEAN DEFAULT 0,
			enable_macd BOOLEAN DEFAULT 0,
			enable_rsi BOOLEAN DEFAULT 0,
			enable_atr BOOLEAN DEFAULT 0,
			enable_volume BOOLEAN DEFAULT 1,
			enable_oi BOOLEAN DEFAULT 1,
			enable_funding BOOLEAN DEFAULT 1,
			indicator_timeframe TEXT DEFAULT '3m',
			quant_data_url TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create new traders table: %w", err)
	}

	// Copy data to new table, including all columns
	_, err = d.db.Exec(`
		INSERT INTO traders_new (id, user_id, name, ai_model_id, exchange_id, initial_balance, 
			scan_interval_minutes, is_running, btc_eth_leverage, altcoin_leverage, trading_symbols,
			use_coin_pool, use_oi_top, use_tradingview, followed_trader_id,
			custom_prompt, override_base_prompt, system_prompt_template, is_cross_margin,
			enable_raw_klines, enable_ema, enable_macd, enable_rsi, enable_atr,
			enable_volume, enable_oi, enable_funding, indicator_timeframe, quant_data_url,
			created_at, updated_at)
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, 
			scan_interval_minutes, is_running, 
			COALESCE(btc_eth_leverage, 5), COALESCE(altcoin_leverage, 5), 
			COALESCE(trading_symbols, ''), COALESCE(use_coin_pool, 0), COALESCE(use_oi_top, 0),
			COALESCE(use_tradingview, 0), COALESCE(followed_trader_id, ''),
			COALESCE(custom_prompt, ''), COALESCE(override_base_prompt, 0), 
			COALESCE(system_prompt_template, 'default'), COALESCE(is_cross_margin, 1),
			COALESCE(enable_raw_klines, 1), COALESCE(enable_ema, 0), COALESCE(enable_macd, 0),
			COALESCE(enable_rsi, 0), COALESCE(enable_atr, 0), COALESCE(enable_volume, 1),
			COALESCE(enable_oi, 1), COALESCE(enable_funding, 1),
			COALESCE(indicator_timeframe, '3m'), COALESCE(quant_data_url, ''),
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

	log.Printf("✅ traders table migration completed, foreign key constraint on user_id removed")
	return nil
}

// migrateMaxUsers migrate max_users from old default "1" to new default "0" (unlimited)
func (d *Database) migrateMaxUsers() error {
	var currentValue string
	err := d.db.QueryRow(`SELECT value FROM system_config WHERE key = 'max_users'`).Scan(&currentValue)
	if err != nil {
		if err == sql.ErrNoRows {
			// Key doesn't exist, will be created by initDefaultData with new default
			return nil
		}
		return fmt.Errorf("failed to check max_users value: %w", err)
	}

	// If still at old default "1", update to "0" (unlimited)
	if currentValue == "1" {
		log.Printf("🔄 Migrating max_users from '1' to '0' (unlimited)...")
		_, err := d.db.Exec(`UPDATE system_config SET value = '0' WHERE key = 'max_users'`)
		if err != nil {
			return fmt.Errorf("failed to update max_users: %w", err)
		}
		log.Printf("✅ Successfully migrated max_users to unlimited (0)")
	}

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

// migrateTraderPositionsSLTP adds stop_loss_price and take_profit_price columns to trader_positions table
func (d *Database) migrateTraderPositionsSLTP() error {
	// Check if stop_loss_price column already exists
	exists, err := d.columnExists("trader_positions", "stop_loss_price")
	if err != nil {
		return fmt.Errorf("failed to check for stop_loss_price column: %w", err)
	}

	if exists {
		// Columns already exist, skip migration
		return nil
	}

	log.Printf("🔄 Adding stop_loss_price and take_profit_price columns to trader_positions table...")

	// Add stop_loss_price column
	_, err = d.db.Exec(`ALTER TABLE trader_positions ADD COLUMN stop_loss_price REAL`)
	if err != nil {
		return fmt.Errorf("failed to add stop_loss_price column: %w", err)
	}

	// Add take_profit_price column
	_, err = d.db.Exec(`ALTER TABLE trader_positions ADD COLUMN take_profit_price REAL`)
	if err != nil {
		return fmt.Errorf("failed to add take_profit_price column: %w", err)
	}

	log.Printf("✅ Successfully added stop_loss_price and take_profit_price columns to trader_positions table")
	return nil
}

// migrateStrategiesTable migrate strategies table to add new configuration columns
func (d *Database) migrateStrategiesTable() error {
	log.Printf("🔄 Checking strategies table migration status...")
	
	// List of new columns to add with their definitions
	newColumns := []struct {
		name    string
		definition string
	}{
		{"min_risk_reward_ratio", "REAL DEFAULT 3.0"},
		{"max_positions", "INTEGER DEFAULT 3"},
		{"margin_usage_limit", "REAL DEFAULT 90.0"},
		{"min_opening_amount", "REAL DEFAULT 12.0"},
		{"min_opening_amount_btc_eth", "REAL DEFAULT 60.0"},
		{"altcoin_position_min", "REAL DEFAULT 0.8"},
		{"altcoin_position_max", "REAL DEFAULT 1.5"},
		{"btc_eth_position_min", "REAL DEFAULT 5.0"},
		{"btc_eth_position_max", "REAL DEFAULT 10.0"},
		{"available_margin_multiplier", "REAL DEFAULT 0.88"},
		{"min_confidence_for_entry", "INTEGER DEFAULT 75"},
		{"min_holding_time_minutes", "INTEGER DEFAULT 30"},
		{"sharpe_ratio_config", "TEXT DEFAULT ''"},
	}
	
	migratedCount := 0
	for _, col := range newColumns {
		exists, err := d.columnExists("strategies", col.name)
		if err != nil {
			log.Printf("⚠️  Error checking if column strategies.%s exists: %v", col.name, err)
			continue
		}
		
		if !exists {
			alterQuery := fmt.Sprintf("ALTER TABLE strategies ADD COLUMN %s %s", col.name, col.definition)
			if _, err := d.db.Exec(alterQuery); err != nil {
				log.Printf("⚠️  Failed to add column strategies.%s: %v", col.name, err)
				continue
			}
			log.Printf("✅ Successfully added column strategies.%s", col.name)
			migratedCount++
		} else {
			log.Printf("ℹ️  Column strategies.%s already exists, skipping", col.name)
		}
	}
	
	if migratedCount > 0 {
		log.Printf("✅ Strategies table migration completed, added %d columns", migratedCount)
	} else {
		log.Printf("✅ Strategies table migration check completed, all columns already exist")
	}
	
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

// Article article/blog post
type Article struct {
	ID               string     `json:"id"`
	Slug             string     `json:"slug"`
	Title            string     `json:"title"`
	Content          string     `json:"content"`
	Excerpt          string     `json:"excerpt"`
	FeaturedImageURL string     `json:"featured_image_url"`
	AuthorID         string     `json:"author_id"`
	AuthorEmail      string     `json:"author_email,omitempty"` // Populated by API handlers
	Status           string     `json:"status"`                 // "draft" or "published"
	MetaTitle        string     `json:"meta_title"`
	MetaDescription  string     `json:"meta_description"`
	MetaKeywords     string     `json:"meta_keywords"`
	OGImageURL       string     `json:"og_image_url"`
	PublishedAt      *time.Time `json:"published_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
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
	LighterWalletAddr       string `json:"lighterWalletAddr"`       // Ethereum wallet address (for display/logging purposes only, not used for account lookup)
	LighterPrivateKey       string `json:"lighterPrivateKey"`       // Deprecated - no longer used (kept for backward compatibility)
	LighterAPIKeyPrivateKey string `json:"lighterAPIKeyPrivateKey"` // API Key private key (40 bytes, for signing transactions)
	LighterAPIKeyIndex      int    `json:"lighterAPIKeyIndex"`      // API Key index (default 0, range 0-254)
	// OKX specific fields
	OkxPassphrase string    `json:"okxPassphrase"` // OKX passphrase (required for OKX)
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TraderRecord trader configuration (database entity)
type TraderRecord struct {
	ID                   string  `json:"id"`
	UserID               string  `json:"user_id"`
	Name                 string  `json:"name"`
	AIModelID            string  `json:"ai_model_id"`
	ExchangeID           string  `json:"exchange_id"`
	InitialBalance       float64 `json:"initial_balance"`
	ScanIntervalMinutes  int     `json:"scan_interval_minutes"`
	IsRunning            bool    `json:"is_running"`
	BTCETHLeverage       int     `json:"btc_eth_leverage"`       // BTC/ETH leverage multiplier
	AltcoinLeverage      int     `json:"altcoin_leverage"`       // Altcoin leverage multiplier
	TradingSymbols       string  `json:"trading_symbols"`        // Trading symbols, comma-separated
	UseCoinPool          bool    `json:"use_coin_pool"`          // Whether to use COIN POOL signal source
	UseOITop             bool    `json:"use_oi_top"`             // Whether to use OI TOP signal source
	UseTradingView       bool    `json:"use_tradingview"`        // Whether to use TradingView signal source
	FollowedTraderID     string  `json:"followed_trader_id"`     // Followed trader ID (for follower role)
	CustomPrompt         string  `json:"custom_prompt"`          // Custom trading strategy prompt
	OverrideBasePrompt   bool    `json:"override_base_prompt"`   // Whether to override base prompt
	SystemPromptTemplate string  `json:"system_prompt_template"` // System prompt template name
	IsCrossMargin        bool    `json:"is_cross_margin"`        // Whether cross margin mode (true=cross, false=isolated)
	ShowInCompetition    bool    `json:"show_in_competition"`    // Whether to show in competition page
	StrategyID           string  `json:"strategy_id"`            // Strategy ID (nullable, references strategies table)
	// Indicator configuration
	EnableRawKlines    bool      `json:"enable_raw_klines"`   // Raw OHLCV klines (always true, required)
	EnableEMA          bool      `json:"enable_ema"`          // Enable EMA indicator
	EnableMACD         bool      `json:"enable_macd"`         // Enable MACD indicator
	EnableRSI          bool      `json:"enable_rsi"`          // Enable RSI indicator
	EnableATR          bool      `json:"enable_atr"`          // Enable ATR indicator
	EnableVolume       bool      `json:"enable_volume"`       // Enable volume data
	EnableOI           bool      `json:"enable_oi"`           // Enable open interest data
	EnableFunding      bool      `json:"enable_funding"`      // Enable funding rate data
	IndicatorTimeframe string    `json:"indicator_timeframe"` // Timeframe for indicators (e.g., "3m", "15m", "1h", "4h")
	QuantDataURL       string    `json:"quant_data_url"`      // External quant data API URL with {symbol} placeholder
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// StrategyRecord strategy configuration (database entity)
type StrategyRecord struct {
	ID                   string    `json:"id"`
	UserID               string    `json:"user_id"`
	Name                 string    `json:"name"`
	Description          string    `json:"description"`
	SystemPromptTemplate string    `json:"system_prompt_template"`
	CustomPrompt         string    `json:"custom_prompt"`
	OverrideBasePrompt   bool      `json:"override_base_prompt"`
	BTCETHLeverage       int       `json:"btc_eth_leverage"`
	AltcoinLeverage      int       `json:"altcoin_leverage"`
	TradingSymbols       string    `json:"trading_symbols"`
	IsCrossMargin        bool      `json:"is_cross_margin"`
	UseCoinPool          bool      `json:"use_coin_pool"`
	UseOITop             bool      `json:"use_oi_top"`
	UseTradingView        bool      `json:"use_tradingview"`
	EnableRawKlines      bool      `json:"enable_raw_klines"`
	EnableEMA            bool      `json:"enable_ema"`
	EnableMACD           bool      `json:"enable_macd"`
	EnableRSI            bool      `json:"enable_rsi"`
	EnableATR            bool      `json:"enable_atr"`
	EnableVolume         bool      `json:"enable_volume"`
	EnableOI             bool      `json:"enable_oi"`
	EnableFunding        bool      `json:"enable_funding"`
	IndicatorTimeframe   string    `json:"indicator_timeframe"`
	QuantDataURL         string    `json:"quant_data_url"`
	// Risk Management Configuration
	MinRiskRewardRatio    float64 `json:"min_risk_reward_ratio"`    // Default: 3.0 (1:3)
	MaxPositions           int     `json:"max_positions"`            // Default: 3
	MarginUsageLimit       float64 `json:"margin_usage_limit"`      // Default: 90.0
	MinOpeningAmount       float64 `json:"min_opening_amount"`      // Default: 12.0
	MinOpeningAmountBTCETH float64 `json:"min_opening_amount_btc_eth"` // Default: 60.0
	// Position Sizing Configuration
	AltcoinPositionMin      float64 `json:"altcoin_position_min"`    // Default: 0.8
	AltcoinPositionMax      float64 `json:"altcoin_position_max"`    // Default: 1.5
	BTCETHPositionMin       float64 `json:"btc_eth_position_min"`    // Default: 5.0
	BTCETHPositionMax       float64 `json:"btc_eth_position_max"`    // Default: 10.0
	AvailableMarginMultiplier float64 `json:"available_margin_multiplier"` // Default: 0.88
	// Trading Rules Configuration
	MinConfidenceForEntry int     `json:"min_confidence_for_entry"` // Default: 75
	MinHoldingTimeMinutes int     `json:"min_holding_time_minutes"` // Default: 30
	// Sharpe Ratio Configuration (JSON string)
	SharpeRatioConfig string `json:"sharpe_ratio_config"` // JSON: { thresholds: [...] }
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
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
	UserID       string     `json:"user_id"`
	TraderID     string     `json:"trader_id"`
	TraderName   string     `json:"trader_name"`
	RawPayload   string     `json:"raw_payload"`
	Symbol       string     `json:"symbol"`
	Action       string     `json:"action"`
	Exchange     string     `json:"exchange"`
	Entry        float64    `json:"entry"`
	SL           float64    `json:"sl"`
	TP           float64    `json:"tp"`
	Quantity     float64    `json:"quantity"`
	PositionSize float64    `json:"position_size"`
	PriceType    string     `json:"pricetype"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	ProcessedAt  *time.Time `json:"processed_at"`
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

// GetUserCount returns the total number of users
func (d *Database) GetUserCount() (int, error) {
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
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

// parseArticleTimestamp parses article timestamp with multiple format fallbacks
func parseArticleTimestamp(dateStr string, fieldName string, articleID string) time.Time {
	if dateStr == "" {
		log.Printf("⚠️  parseArticleTimestamp: empty %s for article %s, using current time", fieldName, articleID)
		return time.Now().UTC()
	}

	// Try standard SQLite format first
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", dateStr, time.UTC); err == nil {
		return parsed
	}

	// Try with microseconds
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05.000000", dateStr, time.UTC); err == nil {
		return parsed
	}

	// Try RFC3339 format (ISO 8601)
	if parsed, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return parsed.UTC()
	}

	// Try RFC3339Nano
	if parsed, err := time.Parse(time.RFC3339Nano, dateStr); err == nil {
		return parsed.UTC()
	}

	log.Printf("⚠️  parseArticleTimestamp: unable to parse %s '%s' for article %s, using current time", fieldName, dateStr, articleID)
	return time.Now().UTC()
}

// CreateArticle create article
func (d *Database) CreateArticle(article *Article) error {
	var publishedAt interface{}
	if article.PublishedAt != nil {
		publishedAt = article.PublishedAt.Format("2006-01-02 15:04:05")
	}
	_, err := d.db.Exec(`
		INSERT INTO articles (id, slug, title, content, excerpt, featured_image_url, author_id, status, 
			meta_title, meta_description, meta_keywords, og_image_url, published_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, article.ID, article.Slug, article.Title, article.Content, article.Excerpt, article.FeaturedImageURL,
		article.AuthorID, article.Status, article.MetaTitle, article.MetaDescription, article.MetaKeywords,
		article.OGImageURL, publishedAt)
	return err
}

// GetArticleByID get article by ID
func (d *Database) GetArticleByID(id string) (*Article, error) {
	var article Article
	var createdAt, updatedAt string
	var publishedAtStr sql.NullString

	err := d.db.QueryRow(`
		SELECT id, slug, title, content, excerpt, featured_image_url, author_id, status,
			meta_title, meta_description, meta_keywords, og_image_url, published_at, created_at, updated_at
		FROM articles WHERE id = ?
	`, id).Scan(
		&article.ID, &article.Slug, &article.Title, &article.Content, &article.Excerpt,
		&article.FeaturedImageURL, &article.AuthorID, &article.Status,
		&article.MetaTitle, &article.MetaDescription, &article.MetaKeywords, &article.OGImageURL,
		&publishedAtStr, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	if publishedAtStr.Valid {
		publishedAt, err := time.ParseInLocation("2006-01-02 15:04:05", publishedAtStr.String, time.UTC)
		if err == nil {
			article.PublishedAt = &publishedAt
		}
	}
	article.CreatedAt = parseArticleTimestamp(createdAt, "created_at", article.ID)
	article.UpdatedAt = parseArticleTimestamp(updatedAt, "updated_at", article.ID)

	return &article, nil
}

// GetArticleBySlug get article by slug
func (d *Database) GetArticleBySlug(slug string) (*Article, error) {
	var article Article
	var createdAt, updatedAt string
	var publishedAtStr sql.NullString

	err := d.db.QueryRow(`
		SELECT id, slug, title, content, excerpt, featured_image_url, author_id, status,
			meta_title, meta_description, meta_keywords, og_image_url, published_at, created_at, updated_at
		FROM articles WHERE slug = ?
	`, slug).Scan(
		&article.ID, &article.Slug, &article.Title, &article.Content, &article.Excerpt,
		&article.FeaturedImageURL, &article.AuthorID, &article.Status,
		&article.MetaTitle, &article.MetaDescription, &article.MetaKeywords, &article.OGImageURL,
		&publishedAtStr, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	if publishedAtStr.Valid {
		publishedAt, err := time.ParseInLocation("2006-01-02 15:04:05", publishedAtStr.String, time.UTC)
		if err == nil {
			article.PublishedAt = &publishedAt
		}
	}
	article.CreatedAt = parseArticleTimestamp(createdAt, "created_at", article.ID)
	article.UpdatedAt = parseArticleTimestamp(updatedAt, "updated_at", article.ID)

	return &article, nil
}

// GetArticles get articles with optional status filter and pagination
func (d *Database) GetArticles(status string, limit, offset int) ([]*Article, error) {
	var rows *sql.Rows
	var err error

	if status != "" {
		rows, err = d.db.Query(`
			SELECT id, slug, title, content, excerpt, featured_image_url, author_id, status,
				meta_title, meta_description, meta_keywords, og_image_url, published_at, created_at, updated_at
			FROM articles WHERE status = ? ORDER BY created_at DESC LIMIT ? OFFSET ?
		`, status, limit, offset)
	} else {
		rows, err = d.db.Query(`
			SELECT id, slug, title, content, excerpt, featured_image_url, author_id, status,
				meta_title, meta_description, meta_keywords, og_image_url, published_at, created_at, updated_at
			FROM articles ORDER BY created_at DESC LIMIT ? OFFSET ?
		`, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []*Article
	for rows.Next() {
		var article Article
		var createdAt, updatedAt string
		var publishedAtStr sql.NullString

		err := rows.Scan(
			&article.ID, &article.Slug, &article.Title, &article.Content, &article.Excerpt,
			&article.FeaturedImageURL, &article.AuthorID, &article.Status,
			&article.MetaTitle, &article.MetaDescription, &article.MetaKeywords, &article.OGImageURL,
			&publishedAtStr, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		if publishedAtStr.Valid {
			publishedAt, err := time.ParseInLocation("2006-01-02 15:04:05", publishedAtStr.String, time.UTC)
			if err == nil {
				article.PublishedAt = &publishedAt
			}
		}
		article.CreatedAt = parseArticleTimestamp(createdAt, "created_at", article.ID)
		article.UpdatedAt = parseArticleTimestamp(updatedAt, "updated_at", article.ID)

		articles = append(articles, &article)
	}

	return articles, rows.Err()
}

// GetPublishedArticles get published articles with pagination
func (d *Database) GetPublishedArticles(limit, offset int) ([]*Article, error) {
	rows, err := d.db.Query(`
		SELECT id, slug, title, content, excerpt, featured_image_url, author_id, status,
			meta_title, meta_description, meta_keywords, og_image_url, published_at, created_at, updated_at
		FROM articles WHERE status = 'published' ORDER BY published_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []*Article
	for rows.Next() {
		var article Article
		var createdAt, updatedAt string
		var publishedAtStr sql.NullString

		err := rows.Scan(
			&article.ID, &article.Slug, &article.Title, &article.Content, &article.Excerpt,
			&article.FeaturedImageURL, &article.AuthorID, &article.Status,
			&article.MetaTitle, &article.MetaDescription, &article.MetaKeywords, &article.OGImageURL,
			&publishedAtStr, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		if publishedAtStr.Valid {
			publishedAt, err := time.ParseInLocation("2006-01-02 15:04:05", publishedAtStr.String, time.UTC)
			if err == nil {
				article.PublishedAt = &publishedAt
			}
		}
		article.CreatedAt = parseArticleTimestamp(createdAt, "created_at", article.ID)
		article.UpdatedAt = parseArticleTimestamp(updatedAt, "updated_at", article.ID)

		articles = append(articles, &article)
	}

	return articles, rows.Err()
}

// UpdateArticle update article
func (d *Database) UpdateArticle(article *Article) error {
	var publishedAt interface{}
	if article.PublishedAt != nil {
		publishedAt = article.PublishedAt.Format("2006-01-02 15:04:05")
	}
	_, err := d.db.Exec(`
		UPDATE articles SET slug = ?, title = ?, content = ?, excerpt = ?, featured_image_url = ?,
			status = ?, meta_title = ?, meta_description = ?, meta_keywords = ?, og_image_url = ?,
			published_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, article.Slug, article.Title, article.Content, article.Excerpt, article.FeaturedImageURL,
		article.Status, article.MetaTitle, article.MetaDescription, article.MetaKeywords,
		article.OGImageURL, publishedAt, article.ID)
	return err
}

// DeleteArticle delete article
func (d *Database) DeleteArticle(id string) error {
	_, err := d.db.Exec(`DELETE FROM articles WHERE id = ?`, id)
	return err
}

// CheckSlugExists check if slug exists (excluding given article ID)
func (d *Database) CheckSlugExists(slug string, excludeID string) (bool, error) {
	var count int
	var err error
	if excludeID != "" {
		err = d.db.QueryRow(`SELECT COUNT(*) FROM articles WHERE slug = ? AND id != ?`, slug, excludeID).Scan(&count)
	} else {
		err = d.db.QueryRow(`SELECT COUNT(*) FROM articles WHERE slug = ?`, slug).Scan(&count)
	}
	if err != nil {
		return false, err
	}
	return count > 0, nil
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
		       COALESCE(is_cross_margin, 1) as is_cross_margin,
		       COALESCE(show_in_competition, 1) as show_in_competition,
		       COALESCE(enable_raw_klines, 1) as enable_raw_klines,
		       COALESCE(enable_ema, 0) as enable_ema, COALESCE(enable_macd, 0) as enable_macd,
		       COALESCE(enable_rsi, 0) as enable_rsi, COALESCE(enable_atr, 0) as enable_atr,
		       COALESCE(enable_volume, 1) as enable_volume, COALESCE(enable_oi, 1) as enable_oi,
		       COALESCE(enable_funding, 1) as enable_funding,
		       COALESCE(indicator_timeframe, '3m') as indicator_timeframe,
		       COALESCE(quant_data_url, '') as quant_data_url,
		       COALESCE(strategy_id, '') as strategy_id,
		       created_at, updated_at
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
			&trader.IsCrossMargin, &trader.ShowInCompetition,
			&trader.EnableRawKlines, &trader.EnableEMA, &trader.EnableMACD,
			&trader.EnableRSI, &trader.EnableATR, &trader.EnableVolume,
			&trader.EnableOI, &trader.EnableFunding, &trader.IndicatorTimeframe,
			&trader.QuantDataURL,
			&trader.StrategyID,
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
		// Preserve existing API key if new key is empty
		var encryptedAPIKey string
		if apiKey == "" {
			// Get existing encrypted API key from database
			var existingEncryptedKey sql.NullString
			err := d.db.QueryRow(`SELECT api_key FROM ai_models WHERE id = ? AND user_id = ?`, existingID, userID).Scan(&existingEncryptedKey)
			if err == nil && existingEncryptedKey.Valid && existingEncryptedKey.String != "" {
				// Use existing encrypted key (don't re-encrypt)
				encryptedAPIKey = existingEncryptedKey.String
			} else {
				// No existing key, encrypt empty string
				encryptedAPIKey = d.encryptSensitiveData("")
			}
		} else {
			// New key provided, encrypt it
			encryptedAPIKey = d.encryptSensitiveData(apiKey)
		}
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
		// Preserve existing API key if new key is empty
		var encryptedAPIKey string
		if apiKey == "" {
			// Get existing encrypted API key from database
			var existingEncryptedKey sql.NullString
			err := d.db.QueryRow(`SELECT api_key FROM ai_models WHERE id = ? AND user_id = ?`, existingID, userID).Scan(&existingEncryptedKey)
			if err == nil && existingEncryptedKey.Valid && existingEncryptedKey.String != "" {
				// Use existing encrypted key (don't re-encrypt)
				encryptedAPIKey = existingEncryptedKey.String
			} else {
				// No existing key, encrypt empty string
				encryptedAPIKey = d.encryptSensitiveData("")
			}
		} else {
			// New key provided, encrypt it
			encryptedAPIKey = d.encryptSensitiveData(apiKey)
		}
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
		       COALESCE(lighter_api_key_index, 0) as lighter_api_key_index,
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
			&exchange.LighterAPIKeyPrivateKey, &exchange.LighterAPIKeyIndex, &exchange.OkxPassphrase,
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
func (d *Database) UpdateExchange(userID, id string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey, lighterWalletAddr, lighterPrivateKey, lighterAPIKeyPrivateKey string, lighterAPIKeyIndex int, okxPassphrase string) error {
	log.Printf("🔧 UpdateExchange: userID=%s, id=%s, enabled=%v", userID, id, enabled)

	// Build dynamic UPDATE SET clause
	// Basic fields: always update
	setClauses := []string{
		"enabled = ?",
		"testnet = ?",
		"hyperliquid_wallet_addr = ?",
		"aster_user = ?",
		"aster_signer = ?",
		"updated_at = datetime('now')",
	}
	args := []interface{}{enabled, testnet, hyperliquidWalletAddr, asterUser, asterSigner}
	
	// Lighter wallet address: always update (it's not sensitive, but we need to allow setting it)
	// First verify the column exists - defensive check for migration issues
	columnExists, err := d.columnExists("exchanges", "lighter_wallet_addr")
	if err != nil {
		log.Printf("⚠️ UpdateExchange: Error checking lighter_wallet_addr column: %v", err)
	} else if !columnExists {
		log.Printf("🔄 UpdateExchange: lighter_wallet_addr column doesn't exist, adding it...")
		_, alterErr := d.db.Exec(`ALTER TABLE exchanges ADD COLUMN lighter_wallet_addr TEXT DEFAULT ''`)
		if alterErr != nil {
			log.Printf("❌ UpdateExchange: Failed to add lighter_wallet_addr column: %v", alterErr)
		} else {
			log.Printf("✅ UpdateExchange: Successfully added lighter_wallet_addr column")
		}
	}
	
	setClauses = append(setClauses, "lighter_wallet_addr = ?")
	args = append(args, lighterWalletAddr)
	
	// Lighter API key index: always update (it's not sensitive)
	// First verify the column exists - defensive check for migration issues
	columnExistsIndex, err := d.columnExists("exchanges", "lighter_api_key_index")
	if err != nil {
		log.Printf("⚠️ UpdateExchange: Error checking lighter_api_key_index column: %v", err)
	} else if !columnExistsIndex {
		log.Printf("🔄 UpdateExchange: lighter_api_key_index column doesn't exist, adding it...")
		_, alterErr := d.db.Exec(`ALTER TABLE exchanges ADD COLUMN lighter_api_key_index INTEGER DEFAULT 0`)
		if alterErr != nil {
			log.Printf("❌ UpdateExchange: Failed to add lighter_api_key_index column: %v", alterErr)
		} else {
			log.Printf("✅ UpdateExchange: Successfully added lighter_api_key_index column")
		}
	}
	
	setClauses = append(setClauses, "lighter_api_key_index = ?")
	args = append(args, lighterAPIKeyIndex)

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
		log.Printf("❌ UpdateExchange: Query was: %s", query)
		log.Printf("❌ UpdateExchange: Args were: %v", args)
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
		case "okx":
			name = "OKX Futures"
			typ = "cex"
		default:
			name = id + " Exchange"
			typ = "cex"
		}

		log.Printf("🆕 UpdateExchange: creating new record ID=%s, name=%s, type=%s", id, name, typ)

		// Check table structure to determine INSERT strategy
		var tableSQL string
		checkErr := d.db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='exchanges'`).Scan(&tableSQL)
		hasCompositeKey := false
		if checkErr == nil {
			hasCompositeKey = strings.Contains(tableSQL, "PRIMARY KEY (id, user_id)")
			if hasCompositeKey {
				log.Printf("✅ UpdateExchange: detected composite primary key structure")
			} else {
				log.Printf("⚠️ UpdateExchange: detected old single primary key structure, using INSERT OR IGNORE")
			}
		} else {
			log.Printf("⚠️ UpdateExchange: failed to check table structure: %v, assuming composite key", checkErr)
			hasCompositeKey = true // Default to new structure
		}

		// Ensure okx_passphrase column exists before INSERT (defensive check)
		exists, err := d.columnExists("exchanges", "okx_passphrase")
		if err != nil {
			log.Printf("⚠️  Error checking if okx_passphrase column exists: %v", err)
		} else if !exists {
			log.Printf("🔄 Adding missing okx_passphrase column to exchanges table (defensive check)...")
			_, err = d.db.Exec(`ALTER TABLE exchanges ADD COLUMN okx_passphrase TEXT DEFAULT ''`)
			if err != nil {
				log.Printf("⚠️  Failed to add okx_passphrase column: %v", err)
				// Continue anyway, will fail on INSERT if column truly missing
			} else {
				log.Printf("✅ Successfully added okx_passphrase column")
			}
		}

		// Encrypt sensitive fields
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
		encryptedSecretKey := d.encryptSensitiveData(secretKey)
		encryptedAsterPrivateKey := d.encryptSensitiveData(asterPrivateKey)
		// Lighter fields kept for backward compatibility but not used
		encryptedLighterPrivateKey := d.encryptSensitiveData(lighterPrivateKey)
		encryptedLighterAPIKeyPrivateKey := d.encryptSensitiveData(lighterAPIKeyPrivateKey)
		encryptedOkxPassphrase := d.encryptSensitiveData(okxPassphrase)

		// Create user-specific configuration
		// Use INSERT OR IGNORE for old structure to handle UNIQUE constraint gracefully
		insertQuery := `
			INSERT INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet,
			                       hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key,
			                       lighter_wallet_addr, lighter_private_key, lighter_api_key_private_key, lighter_api_key_index, okx_passphrase, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		`

		// For old structure (single PRIMARY KEY), use INSERT OR IGNORE to handle conflicts
		if !hasCompositeKey {
			insertQuery = strings.Replace(insertQuery, "INSERT INTO", "INSERT OR IGNORE INTO", 1)
		}

		_, err = d.db.Exec(insertQuery, id, userID, name, typ, enabled, encryptedAPIKey, encryptedSecretKey, testnet, hyperliquidWalletAddr, asterUser, asterSigner, encryptedAsterPrivateKey, lighterWalletAddr, encryptedLighterPrivateKey, encryptedLighterAPIKeyPrivateKey, lighterAPIKeyIndex, encryptedOkxPassphrase)

		if err != nil {
			log.Printf("❌ UpdateExchange: failed to create record: %v", err)
			return err
		}

		// Verify the record was actually created (INSERT OR IGNORE may silently ignore)
		var recordExists int
		verifyErr := d.db.QueryRow(`SELECT COUNT(*) FROM exchanges WHERE id = ? AND user_id = ?`, id, userID).Scan(&recordExists)
		if verifyErr != nil {
			log.Printf("⚠️ UpdateExchange: failed to verify record creation: %v", verifyErr)
			return verifyErr
		}

		if recordExists == 0 {
			// INSERT was ignored (old structure - record exists for different user_id)
			// This shouldn't happen with composite key, but can happen with old structure
			log.Printf("⚠️ UpdateExchange: INSERT was ignored (record may exist for different user_id in old structure)")

			if !hasCompositeKey {
				// Old structure detected - try to trigger migration
				log.Printf("🔄 UpdateExchange: old structure detected, attempting to trigger migration...")
				migrateErr := d.migrateExchangesTable()
				if migrateErr != nil {
					log.Printf("❌ UpdateExchange: migration failed: %v", migrateErr)
					// Check if record exists for any user_id (old structure limitation)
					var anyRecordExists int
					checkErr := d.db.QueryRow(`SELECT COUNT(*) FROM exchanges WHERE id = ?`, id).Scan(&anyRecordExists)
					if checkErr == nil && anyRecordExists > 0 {
						return fmt.Errorf("exchange '%s' already exists for another user. Database migration is required to support multiple users. Migration attempt failed: %v", id, migrateErr)
					}
					return fmt.Errorf("failed to create exchange configuration and migration failed: %v", migrateErr)
				}

				// Verify migration actually completed by checking table structure again
				var tableSQLAfterMigration string
				verifyMigrateErr := d.db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='exchanges'`).Scan(&tableSQLAfterMigration)
				if verifyMigrateErr != nil {
					log.Printf("⚠️ UpdateExchange: failed to verify migration completion: %v", verifyMigrateErr)
				} else {
					hasCompositeKeyAfterMigration := strings.Contains(tableSQLAfterMigration, "PRIMARY KEY (id, user_id)") ||
						strings.Contains(tableSQLAfterMigration, "PRIMARY KEY(id,user_id)") ||
						strings.Contains(tableSQLAfterMigration, "PRIMARY KEY(id, user_id)") ||
						strings.Contains(tableSQLAfterMigration, "PRIMARY KEY (id,user_id)")

					if !hasCompositeKeyAfterMigration {
						log.Printf("❌ UpdateExchange: migration reported success but table structure still shows old format")
						return fmt.Errorf("migration did not complete successfully - table structure unchanged. Please check database manually")
					}
					log.Printf("✅ UpdateExchange: migration verified - table now has composite primary key")
				}

				// Migration succeeded, retry INSERT with new structure
				log.Printf("✅ UpdateExchange: migration completed, retrying INSERT...")
				_, retryErr := d.db.Exec(`
					INSERT INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet,
					                       hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key,
					                       lighter_wallet_addr, lighter_private_key, lighter_api_key_private_key, lighter_api_key_index, okx_passphrase, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
				`, id, userID, name, typ, enabled, encryptedAPIKey, encryptedSecretKey, testnet, hyperliquidWalletAddr, asterUser, asterSigner, encryptedAsterPrivateKey, lighterWalletAddr, encryptedLighterPrivateKey, encryptedLighterAPIKeyPrivateKey, lighterAPIKeyIndex, encryptedOkxPassphrase)

				if retryErr != nil {
					return fmt.Errorf("failed to create record after migration: %v", retryErr)
				}

				// Verify it was created
				var verifyAfterMigration int
				verifyErr := d.db.QueryRow(`SELECT COUNT(*) FROM exchanges WHERE id = ? AND user_id = ?`, id, userID).Scan(&verifyAfterMigration)
				if verifyErr != nil || verifyAfterMigration == 0 {
					return fmt.Errorf("record not created after migration, verification failed")
				}

				log.Printf("✅ UpdateExchange: record created successfully after migration")
				return nil
			}

			// For composite key, this shouldn't happen, but try UPDATE as fallback
			log.Printf("🔄 UpdateExchange: attempting UPDATE as fallback")
			result, updateErr := d.db.Exec(query, args...)
			if updateErr != nil {
				return fmt.Errorf("failed to create or update: insert ignored and update failed: %v", updateErr)
			}
			rowsAffected, _ := result.RowsAffected()
			if rowsAffected == 0 {
				return fmt.Errorf("failed to create exchange configuration: record not created and update affected no rows")
			}
			log.Printf("✅ UpdateExchange: updated existing record after insert ignore")
		} else {
			log.Printf("✅ UpdateExchange: record created successfully")
		}

		return nil
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
		       COALESCE(lighter_api_key_index, 0) as lighter_api_key_index,
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
		&exchange.LighterAPIKeyPrivateKey, &exchange.LighterAPIKeyIndex, &exchange.OkxPassphrase,
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

// ErrDuplicateTemplateID is returned when a template with the same ID already exists
var ErrDuplicateTemplateID = errors.New("template ID already exists")

// ErrForeignKeyViolation is returned when a foreign key constraint is violated
var ErrForeignKeyViolation = errors.New("foreign key constraint violation")

// CreatePromptTemplate create new prompt template
func (d *Database) CreatePromptTemplate(userID, id, name, content string, isSystem bool) error {
	// Check if template ID already exists
	var existingID string
	err := d.db.QueryRow(`SELECT id FROM prompt_templates WHERE id = ?`, id).Scan(&existingID)
	if err == nil {
		// Template ID already exists
		return ErrDuplicateTemplateID
	}
	if !errors.Is(err, sql.ErrNoRows) {
		// Unexpected database error
		return fmt.Errorf("failed to check for existing template: %w", err)
	}

	// Attempt to insert the template
	_, err = d.db.Exec(`
		INSERT INTO prompt_templates (id, user_id, name, content, is_system, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, id, userID, name, content, isSystem)
	if err != nil {
		// Check for constraint violations
		errStr := err.Error()
		if strings.Contains(errStr, "UNIQUE constraint") || strings.Contains(errStr, "PRIMARY KEY") {
			return ErrDuplicateTemplateID
		}
		if strings.Contains(errStr, "FOREIGN KEY constraint") {
			return fmt.Errorf("%w: user_id '%s' does not exist", ErrForeignKeyViolation, userID)
		}
		return fmt.Errorf("failed to create prompt template: %w", err)
	}
	return nil
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

// CreateStrategy create strategy
func (d *Database) CreateStrategy(strategy *StrategyRecord) error {
	// Ensure enable_raw_klines is always true
	strategy.EnableRawKlines = true
	// Set defaults if not provided
	if strategy.IndicatorTimeframe == "" {
		strategy.IndicatorTimeframe = "3m"
	}
	// Set defaults for new configuration fields if not set
	if strategy.MinRiskRewardRatio == 0 {
		strategy.MinRiskRewardRatio = 3.0
	}
	if strategy.MaxPositions == 0 {
		strategy.MaxPositions = 3
	}
	if strategy.MarginUsageLimit == 0 {
		strategy.MarginUsageLimit = 90.0
	}
	if strategy.MinOpeningAmount == 0 {
		strategy.MinOpeningAmount = 12.0
	}
	if strategy.MinOpeningAmountBTCETH == 0 {
		strategy.MinOpeningAmountBTCETH = 60.0
	}
	if strategy.AltcoinPositionMin == 0 {
		strategy.AltcoinPositionMin = 0.8
	}
	if strategy.AltcoinPositionMax == 0 {
		strategy.AltcoinPositionMax = 1.5
	}
	if strategy.BTCETHPositionMin == 0 {
		strategy.BTCETHPositionMin = 5.0
	}
	if strategy.BTCETHPositionMax == 0 {
		strategy.BTCETHPositionMax = 10.0
	}
	if strategy.AvailableMarginMultiplier == 0 {
		strategy.AvailableMarginMultiplier = 0.88
	}
	if strategy.MinConfidenceForEntry == 0 {
		strategy.MinConfidenceForEntry = 75
	}
	if strategy.MinHoldingTimeMinutes == 0 {
		strategy.MinHoldingTimeMinutes = 30
	}
	
	// Try to insert with new fields, fallback to old schema if columns don't exist
	_, err := d.db.Exec(`
		INSERT INTO strategies (id, user_id, name, description, system_prompt_template, custom_prompt, override_base_prompt,
			btc_eth_leverage, altcoin_leverage, trading_symbols, is_cross_margin,
			use_coin_pool, use_oi_top, use_tradingview,
			enable_raw_klines, enable_ema, enable_macd, enable_rsi, enable_atr,
			enable_volume, enable_oi, enable_funding,
			indicator_timeframe, quant_data_url,
			min_risk_reward_ratio, max_positions, margin_usage_limit, min_opening_amount, min_opening_amount_btc_eth,
			altcoin_position_min, altcoin_position_max, btc_eth_position_min, btc_eth_position_max,
			available_margin_multiplier, min_confidence_for_entry, min_holding_time_minutes, sharpe_ratio_config)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, strategy.ID, strategy.UserID, strategy.Name, strategy.Description,
		strategy.SystemPromptTemplate, strategy.CustomPrompt, strategy.OverrideBasePrompt,
		strategy.BTCETHLeverage, strategy.AltcoinLeverage, strategy.TradingSymbols, strategy.IsCrossMargin,
		strategy.UseCoinPool, strategy.UseOITop, strategy.UseTradingView,
		strategy.EnableRawKlines, strategy.EnableEMA, strategy.EnableMACD, strategy.EnableRSI, strategy.EnableATR,
		strategy.EnableVolume, strategy.EnableOI, strategy.EnableFunding,
		strategy.IndicatorTimeframe, strategy.QuantDataURL,
		strategy.MinRiskRewardRatio, strategy.MaxPositions, strategy.MarginUsageLimit,
		strategy.MinOpeningAmount, strategy.MinOpeningAmountBTCETH,
		strategy.AltcoinPositionMin, strategy.AltcoinPositionMax,
		strategy.BTCETHPositionMin, strategy.BTCETHPositionMax,
		strategy.AvailableMarginMultiplier, strategy.MinConfidenceForEntry,
		strategy.MinHoldingTimeMinutes, strategy.SharpeRatioConfig)
	
	return err
}

// GetStrategy get strategy by ID (user-scoped)
func (d *Database) GetStrategy(strategyID, userID string) (*StrategyRecord, error) {
	var strategy StrategyRecord
	var createdAt, updatedAt string
	err := d.db.QueryRow(`
		SELECT id, user_id, name, description, system_prompt_template, custom_prompt, override_base_prompt,
			COALESCE(btc_eth_leverage, 5) as btc_eth_leverage,
			COALESCE(altcoin_leverage, 5) as altcoin_leverage,
			COALESCE(trading_symbols, '') as trading_symbols,
			COALESCE(is_cross_margin, 1) as is_cross_margin,
			COALESCE(use_coin_pool, 0) as use_coin_pool,
			COALESCE(use_oi_top, 0) as use_oi_top,
			COALESCE(use_tradingview, 0) as use_tradingview,
			COALESCE(enable_raw_klines, 1) as enable_raw_klines,
			COALESCE(enable_ema, 0) as enable_ema,
			COALESCE(enable_macd, 0) as enable_macd,
			COALESCE(enable_rsi, 0) as enable_rsi,
			COALESCE(enable_atr, 0) as enable_atr,
			COALESCE(enable_volume, 1) as enable_volume,
			COALESCE(enable_oi, 1) as enable_oi,
			COALESCE(enable_funding, 1) as enable_funding,
			COALESCE(indicator_timeframe, '3m') as indicator_timeframe,
			COALESCE(quant_data_url, '') as quant_data_url,
			COALESCE(min_risk_reward_ratio, 3.0) as min_risk_reward_ratio,
			COALESCE(max_positions, 3) as max_positions,
			COALESCE(margin_usage_limit, 90.0) as margin_usage_limit,
			COALESCE(min_opening_amount, 12.0) as min_opening_amount,
			COALESCE(min_opening_amount_btc_eth, 60.0) as min_opening_amount_btc_eth,
			COALESCE(altcoin_position_min, 0.8) as altcoin_position_min,
			COALESCE(altcoin_position_max, 1.5) as altcoin_position_max,
			COALESCE(btc_eth_position_min, 5.0) as btc_eth_position_min,
			COALESCE(btc_eth_position_max, 10.0) as btc_eth_position_max,
			COALESCE(available_margin_multiplier, 0.88) as available_margin_multiplier,
			COALESCE(min_confidence_for_entry, 75) as min_confidence_for_entry,
			COALESCE(min_holding_time_minutes, 30) as min_holding_time_minutes,
			COALESCE(sharpe_ratio_config, '') as sharpe_ratio_config,
			created_at, updated_at
		FROM strategies
		WHERE id = ? AND user_id = ?
	`, strategyID, userID).Scan(
		&strategy.ID, &strategy.UserID, &strategy.Name, &strategy.Description,
		&strategy.SystemPromptTemplate, &strategy.CustomPrompt, &strategy.OverrideBasePrompt,
		&strategy.BTCETHLeverage, &strategy.AltcoinLeverage, &strategy.TradingSymbols, &strategy.IsCrossMargin,
		&strategy.UseCoinPool, &strategy.UseOITop, &strategy.UseTradingView,
		&strategy.EnableRawKlines, &strategy.EnableEMA, &strategy.EnableMACD,
		&strategy.EnableRSI, &strategy.EnableATR, &strategy.EnableVolume,
		&strategy.EnableOI, &strategy.EnableFunding, &strategy.IndicatorTimeframe,
		&strategy.QuantDataURL,
		&strategy.MinRiskRewardRatio, &strategy.MaxPositions, &strategy.MarginUsageLimit,
		&strategy.MinOpeningAmount, &strategy.MinOpeningAmountBTCETH,
		&strategy.AltcoinPositionMin, &strategy.AltcoinPositionMax,
		&strategy.BTCETHPositionMin, &strategy.BTCETHPositionMax,
		&strategy.AvailableMarginMultiplier, &strategy.MinConfidenceForEntry,
		&strategy.MinHoldingTimeMinutes, &strategy.SharpeRatioConfig,
		&createdAt, &updatedAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	// Ensure enable_raw_klines is always true
	strategy.EnableRawKlines = true
	// Parse time string with error handling
	if createdAt != "" {
		if parsed, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
			strategy.CreatedAt = parsed
		} else {
			// Fallback: try RFC3339 format or set to current time
			if parsed, err := time.Parse(time.RFC3339, createdAt); err == nil {
				strategy.CreatedAt = parsed
			} else {
				strategy.CreatedAt = time.Now()
			}
		}
	} else {
		strategy.CreatedAt = time.Now()
	}
	if updatedAt != "" {
		if parsed, err := time.Parse("2006-01-02 15:04:05", updatedAt); err == nil {
			strategy.UpdatedAt = parsed
		} else {
			// Fallback: try RFC3339 format or set to current time
			if parsed, err := time.Parse(time.RFC3339, updatedAt); err == nil {
				strategy.UpdatedAt = parsed
			} else {
				strategy.UpdatedAt = time.Now()
			}
		}
	} else {
		strategy.UpdatedAt = time.Now()
	}
	return &strategy, nil
}

// LoadStrategyIntoTrader loads strategy settings into a TraderRecord when strategy_id is set
// This ensures Strategy Studio is the single source of truth for strategy-related settings
func (d *Database) LoadStrategyIntoTrader(trader *TraderRecord) error {
	if trader.StrategyID == "" {
		// No strategy_id set, trader uses custom configuration
		return nil
	}

	// Load strategy from database
	strategy, err := d.GetStrategy(trader.StrategyID, trader.UserID)
	if err != nil {
		return fmt.Errorf("failed to load strategy %s: %w", trader.StrategyID, err)
	}

	// Merge strategy settings into trader record
	// These settings come from Strategy Studio (single source of truth)
	// Ensure SystemPromptTemplate has a default value if empty
	if strategy.SystemPromptTemplate == "" {
		trader.SystemPromptTemplate = "default"
	} else {
		trader.SystemPromptTemplate = strategy.SystemPromptTemplate
	}
	trader.CustomPrompt = strategy.CustomPrompt
	trader.OverrideBasePrompt = strategy.OverrideBasePrompt
	trader.BTCETHLeverage = strategy.BTCETHLeverage
	trader.AltcoinLeverage = strategy.AltcoinLeverage
	trader.TradingSymbols = strategy.TradingSymbols
	trader.IsCrossMargin = strategy.IsCrossMargin
	trader.UseCoinPool = strategy.UseCoinPool
	trader.UseOITop = strategy.UseOITop
	trader.UseTradingView = strategy.UseTradingView
	trader.EnableRawKlines = strategy.EnableRawKlines
	trader.EnableEMA = strategy.EnableEMA
	trader.EnableMACD = strategy.EnableMACD
	trader.EnableRSI = strategy.EnableRSI
	trader.EnableATR = strategy.EnableATR
	trader.EnableVolume = strategy.EnableVolume
	trader.EnableOI = strategy.EnableOI
	trader.EnableFunding = strategy.EnableFunding
	trader.IndicatorTimeframe = strategy.IndicatorTimeframe
	trader.QuantDataURL = strategy.QuantDataURL

	return nil
}

// GetStrategies get all strategies for a user
func (d *Database) GetStrategies(userID string) ([]*StrategyRecord, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, name, description, system_prompt_template, custom_prompt, override_base_prompt,
			COALESCE(btc_eth_leverage, 5) as btc_eth_leverage,
			COALESCE(altcoin_leverage, 5) as altcoin_leverage,
			COALESCE(trading_symbols, '') as trading_symbols,
			COALESCE(is_cross_margin, 1) as is_cross_margin,
			COALESCE(use_coin_pool, 0) as use_coin_pool,
			COALESCE(use_oi_top, 0) as use_oi_top,
			COALESCE(use_tradingview, 0) as use_tradingview,
			COALESCE(enable_raw_klines, 1) as enable_raw_klines,
			COALESCE(enable_ema, 0) as enable_ema,
			COALESCE(enable_macd, 0) as enable_macd,
			COALESCE(enable_rsi, 0) as enable_rsi,
			COALESCE(enable_atr, 0) as enable_atr,
			COALESCE(enable_volume, 1) as enable_volume,
			COALESCE(enable_oi, 1) as enable_oi,
			COALESCE(enable_funding, 1) as enable_funding,
			COALESCE(indicator_timeframe, '3m') as indicator_timeframe,
			COALESCE(quant_data_url, '') as quant_data_url,
			COALESCE(min_risk_reward_ratio, 3.0) as min_risk_reward_ratio,
			COALESCE(max_positions, 3) as max_positions,
			COALESCE(margin_usage_limit, 90.0) as margin_usage_limit,
			COALESCE(min_opening_amount, 12.0) as min_opening_amount,
			COALESCE(min_opening_amount_btc_eth, 60.0) as min_opening_amount_btc_eth,
			COALESCE(altcoin_position_min, 0.8) as altcoin_position_min,
			COALESCE(altcoin_position_max, 1.5) as altcoin_position_max,
			COALESCE(btc_eth_position_min, 5.0) as btc_eth_position_min,
			COALESCE(btc_eth_position_max, 10.0) as btc_eth_position_max,
			COALESCE(available_margin_multiplier, 0.88) as available_margin_multiplier,
			COALESCE(min_confidence_for_entry, 75) as min_confidence_for_entry,
			COALESCE(min_holding_time_minutes, 30) as min_holding_time_minutes,
			COALESCE(sharpe_ratio_config, '') as sharpe_ratio_config,
			created_at, updated_at
		FROM strategies
		WHERE user_id = ?
		ORDER BY updated_at DESC
	`, userID)
	
	
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var strategies []*StrategyRecord
	for rows.Next() {
		var strategy StrategyRecord
		var createdAt, updatedAt string
		
		// Try scanning with new fields first
		err := rows.Scan(
			&strategy.ID, &strategy.UserID, &strategy.Name, &strategy.Description,
			&strategy.SystemPromptTemplate, &strategy.CustomPrompt, &strategy.OverrideBasePrompt,
			&strategy.BTCETHLeverage, &strategy.AltcoinLeverage, &strategy.TradingSymbols, &strategy.IsCrossMargin,
			&strategy.UseCoinPool, &strategy.UseOITop, &strategy.UseTradingView,
			&strategy.EnableRawKlines, &strategy.EnableEMA, &strategy.EnableMACD,
			&strategy.EnableRSI, &strategy.EnableATR, &strategy.EnableVolume,
			&strategy.EnableOI, &strategy.EnableFunding, &strategy.IndicatorTimeframe,
			&strategy.QuantDataURL,
			&strategy.MinRiskRewardRatio, &strategy.MaxPositions, &strategy.MarginUsageLimit,
			&strategy.MinOpeningAmount, &strategy.MinOpeningAmountBTCETH,
			&strategy.AltcoinPositionMin, &strategy.AltcoinPositionMax,
			&strategy.BTCETHPositionMin, &strategy.BTCETHPositionMax,
			&strategy.AvailableMarginMultiplier, &strategy.MinConfidenceForEntry,
			&strategy.MinHoldingTimeMinutes, &strategy.SharpeRatioConfig,
			&createdAt, &updatedAt,
		)
		
		// If scan fails (old schema), try with old fields
		if err != nil {
			// Reset new fields to defaults
			strategy.MinRiskRewardRatio = 3.0
			strategy.MaxPositions = 3
			strategy.MarginUsageLimit = 90.0
			strategy.MinOpeningAmount = 12.0
			strategy.MinOpeningAmountBTCETH = 60.0
			strategy.AltcoinPositionMin = 0.8
			strategy.AltcoinPositionMax = 1.5
			strategy.BTCETHPositionMin = 5.0
			strategy.BTCETHPositionMax = 10.0
			strategy.AvailableMarginMultiplier = 0.88
			strategy.MinConfidenceForEntry = 75
			strategy.MinHoldingTimeMinutes = 30
			strategy.SharpeRatioConfig = ""
			
			err = rows.Scan(
				&strategy.ID, &strategy.UserID, &strategy.Name, &strategy.Description,
				&strategy.SystemPromptTemplate, &strategy.CustomPrompt, &strategy.OverrideBasePrompt,
				&strategy.BTCETHLeverage, &strategy.AltcoinLeverage, &strategy.TradingSymbols, &strategy.IsCrossMargin,
				&strategy.UseCoinPool, &strategy.UseOITop, &strategy.UseTradingView,
				&strategy.EnableRawKlines, &strategy.EnableEMA, &strategy.EnableMACD,
				&strategy.EnableRSI, &strategy.EnableATR, &strategy.EnableVolume,
				&strategy.EnableOI, &strategy.EnableFunding, &strategy.IndicatorTimeframe,
				&strategy.QuantDataURL,
				&createdAt, &updatedAt,
			)
		}
		if err != nil {
			return nil, err
		}
		// Ensure enable_raw_klines is always true
		strategy.EnableRawKlines = true
		// Parse time string with error handling
		if createdAt != "" {
			if parsed, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
				strategy.CreatedAt = parsed
			} else {
				// Fallback: try RFC3339 format or set to current time
				if parsed, err := time.Parse(time.RFC3339, createdAt); err == nil {
					strategy.CreatedAt = parsed
				} else {
					strategy.CreatedAt = time.Now()
				}
			}
		} else {
			strategy.CreatedAt = time.Now()
		}
		if updatedAt != "" {
			if parsed, err := time.Parse("2006-01-02 15:04:05", updatedAt); err == nil {
				strategy.UpdatedAt = parsed
			} else {
				// Fallback: try RFC3339 format or set to current time
				if parsed, err := time.Parse(time.RFC3339, updatedAt); err == nil {
					strategy.UpdatedAt = parsed
				} else {
					strategy.UpdatedAt = time.Now()
				}
			}
		} else {
			strategy.UpdatedAt = time.Now()
		}
		strategies = append(strategies, &strategy)
	}
	return strategies, nil
}

// UpdateStrategy update strategy
func (d *Database) UpdateStrategy(strategy *StrategyRecord) error {
	// Ensure enable_raw_klines is always true
	strategy.EnableRawKlines = true
	
	// Try to update with new fields first, fallback to old schema if columns don't exist
	_, err := d.db.Exec(`
		UPDATE strategies SET
			name = ?, description = ?, system_prompt_template = ?, custom_prompt = ?, override_base_prompt = ?,
			btc_eth_leverage = ?, altcoin_leverage = ?, trading_symbols = ?, is_cross_margin = ?,
			use_coin_pool = ?, use_oi_top = ?, use_tradingview = ?,
			enable_raw_klines = ?, enable_ema = ?, enable_macd = ?, enable_rsi = ?, enable_atr = ?,
			enable_volume = ?, enable_oi = ?, enable_funding = ?,
			indicator_timeframe = ?, quant_data_url = ?,
			min_risk_reward_ratio = ?, max_positions = ?, margin_usage_limit = ?,
			min_opening_amount = ?, min_opening_amount_btc_eth = ?,
			altcoin_position_min = ?, altcoin_position_max = ?,
			btc_eth_position_min = ?, btc_eth_position_max = ?,
			available_margin_multiplier = ?, min_confidence_for_entry = ?,
			min_holding_time_minutes = ?, sharpe_ratio_config = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`, strategy.Name, strategy.Description,
		strategy.SystemPromptTemplate, strategy.CustomPrompt, strategy.OverrideBasePrompt,
		strategy.BTCETHLeverage, strategy.AltcoinLeverage, strategy.TradingSymbols, strategy.IsCrossMargin,
		strategy.UseCoinPool, strategy.UseOITop, strategy.UseTradingView,
		strategy.EnableRawKlines, strategy.EnableEMA, strategy.EnableMACD, strategy.EnableRSI, strategy.EnableATR,
		strategy.EnableVolume, strategy.EnableOI, strategy.EnableFunding,
		strategy.IndicatorTimeframe, strategy.QuantDataURL,
		strategy.MinRiskRewardRatio, strategy.MaxPositions, strategy.MarginUsageLimit,
		strategy.MinOpeningAmount, strategy.MinOpeningAmountBTCETH,
		strategy.AltcoinPositionMin, strategy.AltcoinPositionMax,
		strategy.BTCETHPositionMin, strategy.BTCETHPositionMax,
		strategy.AvailableMarginMultiplier, strategy.MinConfidenceForEntry,
		strategy.MinHoldingTimeMinutes, strategy.SharpeRatioConfig,
		strategy.ID, strategy.UserID)
	
	// If error due to missing columns, try with old schema (backward compatibility)
	if err != nil && strings.Contains(err.Error(), "no such column") {
		_, err = d.db.Exec(`
			UPDATE strategies SET
				name = ?, description = ?, system_prompt_template = ?, custom_prompt = ?, override_base_prompt = ?,
				btc_eth_leverage = ?, altcoin_leverage = ?, trading_symbols = ?, is_cross_margin = ?,
				use_coin_pool = ?, use_oi_top = ?, use_tradingview = ?,
				enable_raw_klines = ?, enable_ema = ?, enable_macd = ?, enable_rsi = ?, enable_atr = ?,
				enable_volume = ?, enable_oi = ?, enable_funding = ?,
				indicator_timeframe = ?, quant_data_url = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND user_id = ?
		`, strategy.Name, strategy.Description,
			strategy.SystemPromptTemplate, strategy.CustomPrompt, strategy.OverrideBasePrompt,
			strategy.BTCETHLeverage, strategy.AltcoinLeverage, strategy.TradingSymbols, strategy.IsCrossMargin,
			strategy.UseCoinPool, strategy.UseOITop, strategy.UseTradingView,
			strategy.EnableRawKlines, strategy.EnableEMA, strategy.EnableMACD, strategy.EnableRSI, strategy.EnableATR,
			strategy.EnableVolume, strategy.EnableOI, strategy.EnableFunding,
			strategy.IndicatorTimeframe, strategy.QuantDataURL,
			strategy.ID, strategy.UserID)
	}
	return err
}

// DeleteStrategy delete strategy (only if not used by any traders)
func (d *Database) DeleteStrategy(strategyID, userID string) error {
	// Check if strategy is used by any traders (only check traders belonging to the same user)
	traders, err := d.GetTradersUsingStrategy(strategyID, userID)
	if err != nil {
		return fmt.Errorf("failed to check strategy usage: %w", err)
	}
	if len(traders) > 0 {
		return fmt.Errorf("cannot delete strategy: %d trader(s) are using it", len(traders))
	}
	_, err = d.db.Exec(`DELETE FROM strategies WHERE id = ? AND user_id = ?`, strategyID, userID)
	return err
}

// GetTradersUsingStrategy get all traders using a specific strategy (filtered by user_id)
func (d *Database) GetTradersUsingStrategy(strategyID, userID string) ([]*TraderRecord, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
		       COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
		       COALESCE(trading_symbols, '') as trading_symbols,
		       COALESCE(use_coin_pool, 0) as use_coin_pool, COALESCE(use_oi_top, 0) as use_oi_top,
		       COALESCE(use_tradingview, 0) as use_tradingview,
		       COALESCE(followed_trader_id, '') as followed_trader_id,
		       COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, 0) as override_base_prompt,
		       COALESCE(system_prompt_template, 'default') as system_prompt_template,
		       COALESCE(is_cross_margin, 1) as is_cross_margin,
		       COALESCE(show_in_competition, 1) as show_in_competition,
		       COALESCE(enable_raw_klines, 1) as enable_raw_klines,
		       COALESCE(enable_ema, 0) as enable_ema, COALESCE(enable_macd, 0) as enable_macd,
		       COALESCE(enable_rsi, 0) as enable_rsi, COALESCE(enable_atr, 0) as enable_atr,
		       COALESCE(enable_volume, 1) as enable_volume, COALESCE(enable_oi, 1) as enable_oi,
		       COALESCE(enable_funding, 1) as enable_funding,
		       COALESCE(indicator_timeframe, '3m') as indicator_timeframe,
		       COALESCE(quant_data_url, '') as quant_data_url,
		       COALESCE(strategy_id, '') as strategy_id,
		       created_at, updated_at
		FROM traders
		WHERE strategy_id = ? AND user_id = ?
	`, strategyID, userID)
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
			&trader.IsCrossMargin, &trader.ShowInCompetition,
			&trader.EnableRawKlines, &trader.EnableEMA, &trader.EnableMACD,
			&trader.EnableRSI, &trader.EnableATR, &trader.EnableVolume,
			&trader.EnableOI, &trader.EnableFunding, &trader.IndicatorTimeframe,
			&trader.QuantDataURL, &trader.StrategyID,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		// Ensure enable_raw_klines is always true
		trader.EnableRawKlines = true
		// Parse time string
		trader.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		trader.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		traders = append(traders, &trader)
	}

	// Load strategy settings for each trader (all traders use the same strategy_id)
	// Strategy Studio is single source of truth
	for _, trader := range traders {
		if trader.StrategyID != "" {
			if err := d.LoadStrategyIntoTrader(trader); err != nil {
				log.Printf("⚠️ Failed to load strategy %s for trader %s in GetTradersUsingStrategy: %v, using trader's stored settings", trader.StrategyID, trader.ID, err)
				// Continue with trader's stored values as fallback
			} else {
				log.Printf("✓ Loaded strategy %s settings for trader %s in GetTradersUsingStrategy (pure reference mode)", trader.StrategyID, trader.ID)
			}
		}
	}

	return traders, nil
}

// CreateTrader create trader
func (d *Database) CreateTrader(trader *TraderRecord) error {
	// Ensure enable_raw_klines is always true
	trader.EnableRawKlines = true
	// Set defaults if not provided
	if trader.IndicatorTimeframe == "" {
		trader.IndicatorTimeframe = "3m"
	}
	// Set default ShowInCompetition to true (database default handles this, but ensure consistency)
	_, err := d.db.Exec(`
		INSERT INTO traders (id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running, btc_eth_leverage, altcoin_leverage, trading_symbols, use_coin_pool, use_oi_top, use_tradingview, followed_trader_id, custom_prompt, override_base_prompt, system_prompt_template, is_cross_margin, show_in_competition, enable_raw_klines, enable_ema, enable_macd, enable_rsi, enable_atr, enable_volume, enable_oi, enable_funding, indicator_timeframe, quant_data_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, trader.ID, trader.UserID, trader.Name, trader.AIModelID, trader.ExchangeID, trader.InitialBalance, trader.ScanIntervalMinutes, trader.IsRunning, trader.BTCETHLeverage, trader.AltcoinLeverage, trader.TradingSymbols, trader.UseCoinPool, trader.UseOITop, trader.UseTradingView, trader.FollowedTraderID, trader.CustomPrompt, trader.OverrideBasePrompt, trader.SystemPromptTemplate, trader.IsCrossMargin, trader.ShowInCompetition, trader.EnableRawKlines, trader.EnableEMA, trader.EnableMACD, trader.EnableRSI, trader.EnableATR, trader.EnableVolume, trader.EnableOI, trader.EnableFunding, trader.IndicatorTimeframe, trader.QuantDataURL)
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
		       COALESCE(is_cross_margin, 1) as is_cross_margin,
		       COALESCE(show_in_competition, 1) as show_in_competition,
		       COALESCE(enable_raw_klines, 1) as enable_raw_klines,
		       COALESCE(enable_ema, 0) as enable_ema, COALESCE(enable_macd, 0) as enable_macd,
		       COALESCE(enable_rsi, 0) as enable_rsi, COALESCE(enable_atr, 0) as enable_atr,
		       COALESCE(enable_volume, 1) as enable_volume, COALESCE(enable_oi, 1) as enable_oi,
		       COALESCE(enable_funding, 1) as enable_funding,
		       COALESCE(indicator_timeframe, '3m') as indicator_timeframe,
		       COALESCE(quant_data_url, '') as quant_data_url,
		       COALESCE(strategy_id, '') as strategy_id,
		       created_at, updated_at
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
			&trader.IsCrossMargin, &trader.ShowInCompetition,
			&trader.EnableRawKlines, &trader.EnableEMA, &trader.EnableMACD,
			&trader.EnableRSI, &trader.EnableATR, &trader.EnableVolume,
			&trader.EnableOI, &trader.EnableFunding, &trader.IndicatorTimeframe,
			&trader.QuantDataURL,
			&trader.StrategyID,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		// Ensure enable_raw_klines is always true
		trader.EnableRawKlines = true
		// Parse time string
		trader.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		trader.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		traders = append(traders, &trader)
	}

	// Load strategy settings for each trader if strategy_id is set (Strategy Studio is single source of truth)
	for _, trader := range traders {
		if trader.StrategyID != "" {
			if err := d.LoadStrategyIntoTrader(trader); err != nil {
				log.Printf("⚠️ Failed to load strategy %s for trader %s in GetTraders: %v, using trader's stored settings", trader.StrategyID, trader.ID, err)
				// Continue with trader's stored values as fallback
			} else {
				log.Printf("✓ Loaded strategy %s settings for trader %s in GetTraders (pure reference mode)", trader.StrategyID, trader.ID)
			}
		}
	}

	return traders, nil
}

// GetFollowerTraders get all traders following specified trader list
func (d *Database) GetFollowerTraders(followedTraderID string) ([]*TraderRecord, error) {
	log.Printf("🔍 DEBUG [GetFollowerTraders]: Querying followers for parent trader ID: '%s'", followedTraderID)

	// Query handles both NULL and empty string by using COALESCE in SELECT, but WHERE clause needs to handle both
	// SQLite: NULL != '' and '' != NULL, so we need to check both cases
	rows, err := d.db.Query(`
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
		       COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
		       COALESCE(trading_symbols, '') as trading_symbols,
		       COALESCE(use_coin_pool, 0) as use_coin_pool, COALESCE(use_oi_top, 0) as use_oi_top,
		       COALESCE(use_tradingview, 0) as use_tradingview,
		       COALESCE(followed_trader_id, '') as followed_trader_id,
		       COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, 0) as override_base_prompt,
		       COALESCE(system_prompt_template, 'default') as system_prompt_template,
		       COALESCE(is_cross_margin, 1) as is_cross_margin,
		       COALESCE(show_in_competition, 1) as show_in_competition,
		       COALESCE(enable_raw_klines, 1) as enable_raw_klines,
		       COALESCE(enable_ema, 0) as enable_ema, COALESCE(enable_macd, 0) as enable_macd,
		       COALESCE(enable_rsi, 0) as enable_rsi, COALESCE(enable_atr, 0) as enable_atr,
		       COALESCE(enable_volume, 1) as enable_volume, COALESCE(enable_oi, 1) as enable_oi,
		       COALESCE(enable_funding, 1) as enable_funding,
		       COALESCE(indicator_timeframe, '3m') as indicator_timeframe,
		       COALESCE(quant_data_url, '') as quant_data_url,
		       COALESCE(strategy_id, '') as strategy_id,
		       created_at, updated_at
		FROM traders 
		WHERE COALESCE(followed_trader_id, '') = ?
		ORDER BY created_at DESC
	`, followedTraderID)
	if err != nil {
		log.Printf("❌ DEBUG [GetFollowerTraders]: Query failed for parent trader ID '%s': %v", followedTraderID, err)
		return nil, err
	}
	defer rows.Close()

	var traders []*TraderRecord
	rowCount := 0
	for rows.Next() {
		rowCount++
		var trader TraderRecord
		var createdAt, updatedAt string
		err := rows.Scan(
			&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
			&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
			&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
			&trader.UseCoinPool, &trader.UseOITop, &trader.UseTradingView,
			&trader.FollowedTraderID,
			&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
			&trader.IsCrossMargin, &trader.ShowInCompetition,
			&trader.EnableRawKlines, &trader.EnableEMA, &trader.EnableMACD,
			&trader.EnableRSI, &trader.EnableATR, &trader.EnableVolume,
			&trader.EnableOI, &trader.EnableFunding, &trader.IndicatorTimeframe,
			&trader.QuantDataURL, &trader.StrategyID,
			&createdAt, &updatedAt,
		)
		if err != nil {
			log.Printf("❌ DEBUG [GetFollowerTraders]: Row scan failed for parent trader ID '%s', row %d: %v", followedTraderID, rowCount, err)
			return nil, err
		}
		// Ensure enable_raw_klines is always true
		trader.EnableRawKlines = true
		// Parse time string
		trader.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		trader.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		traders = append(traders, &trader)
		log.Printf("✅ DEBUG [GetFollowerTraders]: Found follower - ID: '%s', Name: '%s', UserID: '%s', FollowedTraderID: '%s', IsRunning: %v",
			trader.ID, trader.Name, trader.UserID, trader.FollowedTraderID, trader.IsRunning)
	}

	log.Printf("📊 DEBUG [GetFollowerTraders]: Query completed for parent trader ID '%s' - Found %d followers", followedTraderID, len(traders))
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
	// Ensure enable_raw_klines is always true
	trader.EnableRawKlines = true
	log.Printf("🔍 DEBUG [UpdateTrader DB]: Updating trader %s (user_id: %s) with system_prompt_template: '%s'", trader.ID, trader.UserID, trader.SystemPromptTemplate)
	result, err := d.db.Exec(`
		UPDATE traders SET
			name = ?, ai_model_id = ?, exchange_id = ?,
			initial_balance = ?, scan_interval_minutes = ?, btc_eth_leverage = ?, altcoin_leverage = ?,
			trading_symbols = ?, use_coin_pool = ?, use_oi_top = ?, use_tradingview = ?,
			followed_trader_id = ?,
			custom_prompt = ?, override_base_prompt = ?,
			system_prompt_template = ?, is_cross_margin = ?, show_in_competition = ?,
			enable_raw_klines = ?, enable_ema = ?, enable_macd = ?, enable_rsi = ?, enable_atr = ?,
			enable_volume = ?, enable_oi = ?, enable_funding = ?,
			indicator_timeframe = ?, quant_data_url = ?, strategy_id = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`, trader.Name, trader.AIModelID, trader.ExchangeID,
		trader.InitialBalance, trader.ScanIntervalMinutes, trader.BTCETHLeverage, trader.AltcoinLeverage,
		trader.TradingSymbols, trader.UseCoinPool, trader.UseOITop, trader.UseTradingView,
		trader.FollowedTraderID,
		trader.CustomPrompt, trader.OverrideBasePrompt,
		trader.SystemPromptTemplate, trader.IsCrossMargin, trader.ShowInCompetition,
		trader.EnableRawKlines, trader.EnableEMA, trader.EnableMACD, trader.EnableRSI, trader.EnableATR,
		trader.EnableVolume, trader.EnableOI, trader.EnableFunding,
		trader.IndicatorTimeframe, trader.QuantDataURL, trader.StrategyID,
		trader.ID, trader.UserID)
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

// UpdateTraderShowInCompetition updates trader competition visibility
func (d *Database) UpdateTraderShowInCompetition(userID, id string, showInCompetition bool) error {
	_, err := d.db.Exec(`UPDATE traders SET show_in_competition = ? WHERE id = ? AND user_id = ?`, showInCompetition, id, userID)
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
			COALESCE(t.enable_raw_klines, 1) as enable_raw_klines,
			COALESCE(t.enable_ema, 0) as enable_ema, COALESCE(t.enable_macd, 0) as enable_macd,
			COALESCE(t.enable_rsi, 0) as enable_rsi, COALESCE(t.enable_atr, 0) as enable_atr,
			COALESCE(t.enable_volume, 1) as enable_volume, COALESCE(t.enable_oi, 1) as enable_oi,
			COALESCE(t.enable_funding, 1) as enable_funding,
			COALESCE(t.indicator_timeframe, '3m') as indicator_timeframe,
			COALESCE(t.quant_data_url, '') as quant_data_url,
			COALESCE(t.strategy_id, '') as strategy_id,
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
			COALESCE(e.lighter_api_key_index, 0) as lighter_api_key_index,
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
		&trader.EnableRawKlines, &trader.EnableEMA, &trader.EnableMACD,
		&trader.EnableRSI, &trader.EnableATR, &trader.EnableVolume,
		&trader.EnableOI, &trader.EnableFunding, &trader.IndicatorTimeframe,
		&trader.QuantDataURL, &trader.StrategyID,
		&traderCreatedAt, &traderUpdatedAt,
		&aiModel.ID, &aiModel.UserID, &aiModel.Name, &aiModel.Provider, &aiModel.Enabled, &aiModel.APIKey,
		&aiModel.CustomAPIURL, &aiModel.CustomModelName,
		&aiModelCreatedAt, &aiModelUpdatedAt,
		&exchange.ID, &exchange.UserID, &exchange.Name, &exchange.Type, &exchange.Enabled,
		&exchange.APIKey, &exchange.SecretKey, &exchange.Testnet,
		&exchange.HyperliquidWalletAddr, &exchange.AsterUser, &exchange.AsterSigner, &exchange.AsterPrivateKey,
		&exchange.LighterWalletAddr, &exchange.LighterPrivateKey, &exchange.LighterAPIKeyPrivateKey, &exchange.LighterAPIKeyIndex,
		&exchange.OkxPassphrase,
		&exchangeCreatedAt, &exchangeUpdatedAt,
	)

	if err != nil {
		return nil, nil, nil, err
	}

	// Ensure enable_raw_klines is always true
	trader.EnableRawKlines = true
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

	// Load strategy settings if strategy_id is set (Strategy Studio is single source of truth)
	// This ensures we return strategy settings from the strategy record, not from trader's stored duplicates
	if trader.StrategyID != "" {
		if err := d.LoadStrategyIntoTrader(&trader); err != nil {
			// Log error but continue - fallback to trader's stored values (for backward compatibility)
			log.Printf("⚠️ Failed to load strategy %s for trader %s in GetTraderConfig: %v, using trader's stored settings", trader.StrategyID, trader.ID, err)
		} else {
			log.Printf("✓ Loaded strategy %s settings for trader %s in GetTraderConfig (pure reference mode)", trader.StrategyID, trader.ID)
		}
	}

	return &trader, &aiModel, &exchange, nil
}

// GetTraderByID get trader by ID without userID (for public competition data)
func (d *Database) GetTraderByID(traderID string) (*TraderRecord, error) {
	var trader TraderRecord
	var createdAt, updatedAt string

	err := d.db.QueryRow(`
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
		       COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
		       COALESCE(trading_symbols, '') as trading_symbols,
		       COALESCE(use_coin_pool, 0) as use_coin_pool, COALESCE(use_oi_top, 0) as use_oi_top,
		       COALESCE(use_tradingview, 0) as use_tradingview,
		       COALESCE(followed_trader_id, '') as followed_trader_id,
		       COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, 0) as override_base_prompt,
		       COALESCE(system_prompt_template, 'default') as system_prompt_template,
		       COALESCE(is_cross_margin, 1) as is_cross_margin,
		       COALESCE(enable_raw_klines, 1) as enable_raw_klines,
		       COALESCE(enable_ema, 0) as enable_ema, COALESCE(enable_macd, 0) as enable_macd,
		       COALESCE(enable_rsi, 0) as enable_rsi, COALESCE(enable_atr, 0) as enable_atr,
		       COALESCE(enable_volume, 1) as enable_volume, COALESCE(enable_oi, 1) as enable_oi,
		       COALESCE(enable_funding, 1) as enable_funding,
		       COALESCE(indicator_timeframe, '3m') as indicator_timeframe,
		       COALESCE(quant_data_url, '') as quant_data_url,
		       COALESCE(strategy_id, '') as strategy_id,
		       created_at, updated_at
		FROM traders WHERE id = ?
	`, traderID).Scan(
		&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
		&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
		&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
		&trader.UseCoinPool, &trader.UseOITop, &trader.UseTradingView,
		&trader.FollowedTraderID,
		&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
		&trader.IsCrossMargin,
		&trader.EnableRawKlines, &trader.EnableEMA, &trader.EnableMACD,
		&trader.EnableRSI, &trader.EnableATR, &trader.EnableVolume,
		&trader.EnableOI, &trader.EnableFunding, &trader.IndicatorTimeframe,
		&trader.QuantDataURL, &trader.StrategyID,
		&createdAt, &updatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Ensure enable_raw_klines is always true
	trader.EnableRawKlines = true
	// Parse time string
	trader.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	trader.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	// Load strategy settings if strategy_id is set (Strategy Studio is single source of truth)
	if trader.StrategyID != "" {
		if err := d.LoadStrategyIntoTrader(&trader); err != nil {
			log.Printf("⚠️ Failed to load strategy %s for trader %s in GetTraderByID: %v, using trader's stored settings", trader.StrategyID, trader.ID, err)
			// Continue with trader's stored values as fallback
		} else {
			log.Printf("✓ Loaded strategy %s settings for trader %s in GetTraderByID (pure reference mode)", trader.StrategyID, trader.ID)
		}
	}

	return &trader, nil
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
	// Generate unique ID: trader_id + timestamp + random component to prevent collisions
	// This ensures uniqueness even when multiple traders process webhooks simultaneously
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random component: %w", err)
	}
	randomInt := uint32(randomBytes[0])<<24 | uint32(randomBytes[1])<<16 | uint32(randomBytes[2])<<8 | uint32(randomBytes[3])
	id := fmt.Sprintf("%s_%d_%d", traderID, time.Now().UnixNano(), randomInt)

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

// parseFloat helper function: parse float64 from interface{}, handling NaN and invalid values
func parseFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		// Check for NaN or Infinity
		if math.IsNaN(val) || math.IsInf(val, 0) {
			return 0
		}
		return val
	case float32:
		f := float64(val)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0
		}
		return f
	case string:
		// Handle string representations of NaN, null, etc.
		val = strings.TrimSpace(strings.ToLower(val))
		if val == "" || val == "null" || val == "nan" || val == "undefined" {
			return 0
		}
		// Try to parse as float
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return 0
			}
			return f
		}
		// Fallback to Sscanf
		var f float64
		if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return 0
			}
			return f
		}
		return 0
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	default:
		return 0
	}
}

// GetPendingTradingViewAlerts get pending alerts for specified trader
func (d *Database) GetPendingTradingViewAlerts(traderID string) ([]TradingViewAlert, error) {
	rows, err := d.db.Query(`
		SELECT a.id, a.user_id, a.trader_id, COALESCE(t.name, '') as trader_name, a.raw_payload, a.symbol, a.action, a.exchange,
			a.entry, a.sl, a.tp, a.quantity, a.position_size, a.pricetype, a.status,
			a.created_at, a.processed_at
		FROM tradingview_alerts a
		LEFT JOIN traders t ON a.trader_id = t.id
		WHERE a.trader_id = ? AND a.status = 'pending'
		ORDER BY a.created_at ASC
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
		// Use NullFloat64 for nullable numeric fields
		var entry, sl, tp, quantity, positionSize sql.NullFloat64

		err := rows.Scan(
			&alert.ID, &alert.UserID, &alert.TraderID, &alert.TraderName, &alert.RawPayload,
			&alert.Symbol, &alert.Action, &alert.Exchange,
			&entry, &sl, &tp, &quantity,
			&positionSize, &alert.PriceType, &alert.Status,
			&createdAt, &processedAt,
		)
		if err != nil {
			continue
		}

		// Convert NullFloat64 to float64
		if entry.Valid {
			alert.Entry = entry.Float64
		}
		if sl.Valid {
			alert.SL = sl.Float64
		}
		if tp.Valid {
			alert.TP = tp.Float64
		}
		if quantity.Valid {
			alert.Quantity = quantity.Float64
		}
		if positionSize.Valid {
			alert.PositionSize = positionSize.Float64
		}

		// Try parsing with ISO 8601 format first (RFC3339), then fall back to SQLite datetime format
		parsedTime, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			parsedTime, err = time.Parse("2006-01-02 15:04:05", createdAt)
			if err != nil {
				parsedTime = time.Time{} // Zero time on failure
			}
		}
		alert.CreatedAt = parsedTime
		if processedAt.Valid {
			t, err := time.Parse(time.RFC3339, processedAt.String)
			if err != nil {
				t, _ = time.Parse("2006-01-02 15:04:05", processedAt.String)
			}
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
	// Use NullFloat64 for nullable numeric fields
	var entry, sl, tp, quantity, positionSize sql.NullFloat64

	err := d.db.QueryRow(`
		SELECT a.id, a.user_id, a.trader_id, COALESCE(t.name, '') as trader_name, a.raw_payload, a.symbol, a.action, a.exchange,
			a.entry, a.sl, a.tp, a.quantity, a.position_size, a.pricetype, a.status,
			a.created_at, a.processed_at
		FROM tradingview_alerts a
		LEFT JOIN traders t ON a.trader_id = t.id
		WHERE a.id = ?
	`, alertID).Scan(
		&alert.ID, &alert.UserID, &alert.TraderID, &alert.TraderName, &alert.RawPayload,
		&alert.Symbol, &alert.Action, &alert.Exchange,
		&entry, &sl, &tp, &quantity,
		&positionSize, &alert.PriceType, &alert.Status,
		&createdAt, &processedAt,
	)
	if err != nil {
		return nil, err
	}

	// Convert NullFloat64 to float64
	if entry.Valid {
		alert.Entry = entry.Float64
	}
	if sl.Valid {
		alert.SL = sl.Float64
	}
	if tp.Valid {
		alert.TP = tp.Float64
	}
	if quantity.Valid {
		alert.Quantity = quantity.Float64
	}
	if positionSize.Valid {
		alert.PositionSize = positionSize.Float64
	}

	// Try parsing with ISO 8601 format first (RFC3339), then fall back to SQLite datetime format
	parsedTime, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		parsedTime, err = time.Parse("2006-01-02 15:04:05", createdAt)
		if err != nil {
			parsedTime = time.Time{} // Zero time on failure
		}
	}
	alert.CreatedAt = parsedTime
	if processedAt.Valid {
		t, err := time.Parse(time.RFC3339, processedAt.String)
		if err != nil {
			t, _ = time.Parse("2006-01-02 15:04:05", processedAt.String)
		}
		alert.ProcessedAt = &t
	}

	return &alert, nil
}

// GetRecentTradingViewAlerts get recent TradingView alerts with trader names
func (d *Database) GetRecentTradingViewAlerts(userID string, traderID string, limit int) ([]TradingViewAlert, error) {
	query := `
		SELECT a.id, a.user_id, a.trader_id, COALESCE(t.name, '') as trader_name, a.raw_payload, a.symbol, a.action, a.exchange,
			a.entry, a.sl, a.tp, a.quantity, a.position_size, a.pricetype, a.status,
			a.created_at, a.processed_at
		FROM tradingview_alerts a
		LEFT JOIN traders t ON a.trader_id = t.id
		WHERE a.user_id = ?
	`
	args := []interface{}{userID}

	if traderID != "" {
		query += " AND a.trader_id = ?"
		args = append(args, traderID)
	}

	query += " ORDER BY a.created_at DESC LIMIT ?"
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
		// Use NullFloat64 for nullable numeric fields
		var entry, sl, tp, quantity, positionSize sql.NullFloat64

		err := rows.Scan(
			&alert.ID, &alert.UserID, &alert.TraderID, &alert.TraderName, &alert.RawPayload,
			&alert.Symbol, &alert.Action, &alert.Exchange,
			&entry, &sl, &tp, &quantity,
			&positionSize, &alert.PriceType, &alert.Status,
			&createdAt, &processedAt,
		)
		if err != nil {
			continue
		}

		// Convert NullFloat64 to float64
		if entry.Valid {
			alert.Entry = entry.Float64
		}
		if sl.Valid {
			alert.SL = sl.Float64
		}
		if tp.Valid {
			alert.TP = tp.Float64
		}
		if quantity.Valid {
			alert.Quantity = quantity.Float64
		}
		if positionSize.Valid {
			alert.PositionSize = positionSize.Float64
		}

		// Try parsing with ISO 8601 format first (RFC3339), then fall back to SQLite datetime format
		parsedTime, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			// Fall back to SQLite datetime format
			parsedTime, err = time.Parse("2006-01-02 15:04:05", createdAt)
			if err != nil {
				parsedTime = time.Time{} // Zero time on failure
			}
		}
		alert.CreatedAt = parsedTime
		if processedAt.Valid {
			// Try parsing with ISO 8601 format first (RFC3339), then fall back to SQLite datetime format
			t, err := time.Parse(time.RFC3339, processedAt.String)
			if err != nil {
				t, _ = time.Parse("2006-01-02 15:04:05", processedAt.String)
			}
			alert.ProcessedAt = &t
		}


		alerts = append(alerts, alert)
	}

	return alerts, nil
}

// GetTradersWithTradingViewEnabled get user's traders with TradingView enabled list
func (d *Database) GetTradersWithTradingViewEnabled(userID string) ([]*TraderRecord, error) {
	// Query traders where TradingView is enabled either directly on the trader
	// OR via the trader's linked strategy (Strategy Studio is single source of truth)
	rows, err := d.db.Query(`
		SELECT t.id, t.user_id, t.name, t.ai_model_id, t.exchange_id, t.initial_balance,
			t.scan_interval_minutes, t.is_running, t.btc_eth_leverage, t.altcoin_leverage,
			t.trading_symbols, t.use_coin_pool, t.use_oi_top, t.use_tradingview,
			t.custom_prompt, t.override_base_prompt, t.system_prompt_template, t.is_cross_margin,
			COALESCE(t.enable_raw_klines, 1) as enable_raw_klines,
			COALESCE(t.enable_ema, 0) as enable_ema, COALESCE(t.enable_macd, 0) as enable_macd,
			COALESCE(t.enable_rsi, 0) as enable_rsi, COALESCE(t.enable_atr, 0) as enable_atr,
			COALESCE(t.enable_volume, 1) as enable_volume, COALESCE(t.enable_oi, 1) as enable_oi,
			COALESCE(t.enable_funding, 1) as enable_funding,
			COALESCE(t.indicator_timeframe, '3m') as indicator_timeframe,
			COALESCE(t.quant_data_url, '') as quant_data_url,
			COALESCE(t.strategy_id, '') as strategy_id,
			t.created_at, t.updated_at
		FROM traders t
		LEFT JOIN strategies s ON t.strategy_id = s.id AND t.strategy_id != ''
		WHERE t.user_id = ? AND (t.use_tradingview = 1 OR (t.strategy_id != '' AND s.use_tradingview = 1))
		ORDER BY t.created_at ASC
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
			&trader.EnableRawKlines, &trader.EnableEMA, &trader.EnableMACD,
			&trader.EnableRSI, &trader.EnableATR, &trader.EnableVolume,
			&trader.EnableOI, &trader.EnableFunding, &trader.IndicatorTimeframe,
			&trader.QuantDataURL, &trader.StrategyID,
			&createdAt, &updatedAt,
		)
		if err != nil {
			continue
		}
		// Ensure enable_raw_klines is always true
		trader.EnableRawKlines = true
		trader.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		trader.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		// Load strategy settings if strategy_id is set (Strategy Studio is single source of truth)
		if trader.StrategyID != "" {
			if err := d.LoadStrategyIntoTrader(&trader); err != nil {
				log.Printf("⚠️ Failed to load strategy %s for trader %s in GetTradersWithTradingViewEnabled: %v", trader.StrategyID, trader.ID, err)
			}
		}
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

// PositionRecord represents a position record in the database
type PositionRecord struct {
	ID            string
	TraderID      string
	Symbol        string
	Side          string
	EntryPrice    float64
	ExitPrice     float64
	Quantity      float64
	EntryFee      float64
	ExitFee       float64
	RealizedPnL   float64
	Leverage      int
	OpenedAt      time.Time
	ClosedAt      *time.Time
	OrderIDOpen   string
	OrderIDClose  string
	StopLossPrice float64
	TakeProfitPrice float64
}

// SavePosition save position record to database
func (d *Database) SavePosition(traderID, symbol, side string, entryPrice, exitPrice, quantity, entryFee, exitFee, realizedPnL float64, leverage int, orderIDOpen, orderIDClose string, openedAt time.Time, closedAt *time.Time, stopLossPrice, takeProfitPrice float64) error {
	id := fmt.Sprintf("%s_%s_%d", traderID, symbol, openedAt.Unix())

	var closedAtStr interface{}
	if closedAt != nil {
		closedAtStr = closedAt.Format("2006-01-02 15:04:05")
	}

	var exitPriceVal interface{}
	if exitPrice > 0 {
		exitPriceVal = exitPrice
	}

	var realizedPnLVal interface{}
	if realizedPnL != 0 {
		realizedPnLVal = realizedPnL
	}

	var stopLossPriceVal interface{}
	if stopLossPrice > 0 {
		stopLossPriceVal = stopLossPrice
	}

	var takeProfitPriceVal interface{}
	if takeProfitPrice > 0 {
		takeProfitPriceVal = takeProfitPrice
	}

	query := `
		INSERT OR REPLACE INTO trader_positions 
		(id, trader_id, symbol, side, entry_price, exit_price, quantity, entry_fee, exit_fee, realized_pnl, leverage, opened_at, closed_at, order_id_open, order_id_close, stop_loss_price, take_profit_price)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := d.db.Exec(query, id, traderID, symbol, side, entryPrice, exitPriceVal, quantity, entryFee, exitFee, realizedPnLVal, leverage, openedAt.Format("2006-01-02 15:04:05"), closedAtStr, orderIDOpen, orderIDClose, stopLossPriceVal, takeProfitPriceVal)
	return err
}

// GetPositionHistory get position history for a trader
func (d *Database) GetPositionHistory(traderID string, limit, offset int) ([]*PositionRecord, error) {
	query := `
		SELECT id, trader_id, symbol, side, entry_price, exit_price, quantity, entry_fee, exit_fee, realized_pnl, leverage, opened_at, closed_at, order_id_open, order_id_close, stop_loss_price, take_profit_price
		FROM trader_positions
		WHERE trader_id = ?
		ORDER BY opened_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := d.db.Query(query, traderID, limit, offset)
	if err != nil {
		log.Printf("❌ GetPositionHistory: Query failed for trader %s (limit=%d, offset=%d): %v", traderID, limit, offset, err)
		return nil, err
	}
	defer rows.Close()

	var positions []*PositionRecord
	rowIndex := 0
	for rows.Next() {
		var pos PositionRecord
		var closedAtStr sql.NullString
		var exitPrice sql.NullFloat64
		var realizedPnL sql.NullFloat64
		var stopLossPrice sql.NullFloat64
		var takeProfitPrice sql.NullFloat64

		err := rows.Scan(&pos.ID, &pos.TraderID, &pos.Symbol, &pos.Side, &pos.EntryPrice, &exitPrice, &pos.Quantity, &pos.EntryFee, &pos.ExitFee, &realizedPnL, &pos.Leverage, &pos.OpenedAt, &closedAtStr, &pos.OrderIDOpen, &pos.OrderIDClose, &stopLossPrice, &takeProfitPrice)
		if err != nil {
			log.Printf("❌ GetPositionHistory: Scan failed for trader %s: %v", traderID, err)
			return nil, err
		}

		// Handle nullable exit_price
		if exitPrice.Valid {
			pos.ExitPrice = exitPrice.Float64
		} else {
			pos.ExitPrice = 0
		}

		// Handle nullable realized_pnl
		if realizedPnL.Valid {
			pos.RealizedPnL = realizedPnL.Float64
		} else {
			pos.RealizedPnL = 0
		}

		if closedAtStr.Valid {
			// Try parsing as ISO 8601 format first (e.g., "2025-12-31T12:30:32Z")
			var closedAt time.Time
			var err error
			closedAt, err = time.Parse(time.RFC3339, closedAtStr.String)
			if err != nil {
				// Fallback to custom format (e.g., "2006-01-02 15:04:05")
				closedAt, err = time.Parse("2006-01-02 15:04:05", closedAtStr.String)
			}
			if err == nil {
				pos.ClosedAt = &closedAt
			}
		}

		if stopLossPrice.Valid {
			pos.StopLossPrice = stopLossPrice.Float64
		}

		if takeProfitPrice.Valid {
			pos.TakeProfitPrice = takeProfitPrice.Float64
		}

		positions = append(positions, &pos)
		rowIndex++
	}

	if err := rows.Err(); err != nil {
		log.Printf("❌ GetPositionHistory: Error iterating rows for trader %s: %v", traderID, err)
		return nil, err
	}
	return positions, nil
}

// HasRecentlyClosedPosition checks if there's already a closed position for the given symbol/side
// within the specified time window. This helps prevent duplicate closed position records.
func (d *Database) HasRecentlyClosedPosition(traderID, symbol, side string, timeWindowMinutes int) (bool, error) {
	if d.db == nil {
		return false, fmt.Errorf("database not initialized")
	}

	// Calculate cutoff time
	cutoffTime := time.Now().Add(-time.Duration(timeWindowMinutes) * time.Minute)

	query := `
		SELECT COUNT(*) FROM trader_positions
		WHERE trader_id = ? 
		AND symbol = ? 
		AND side = ? 
		AND closed_at IS NOT NULL
		AND closed_at >= ?
	`

	var count int
	err := d.db.QueryRow(query, traderID, symbol, side, cutoffTime.Format("2006-01-02 15:04:05")).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check for recently closed position: %w", err)
	}

	return count > 0, nil
}

// IsPositionClosedByID checks if a specific position (by ID) is already marked as closed.
// This provides precise idempotency checking using the unique position ID.
func (d *Database) IsPositionClosedByID(positionID string) (bool, error) {
	if d.db == nil {
		return false, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT closed_at FROM trader_positions
		WHERE id = ?
	`

	var closedAt sql.NullString
	err := d.db.QueryRow(query, positionID).Scan(&closedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // Position not found, so not closed
		}
		return false, fmt.Errorf("failed to check position status: %w", err)
	}

	return closedAt.Valid && closedAt.String != "", nil
}

// HasClosedPositionWithPnL checks if a closed position exists with valid PnL (non-zero).
// This helps identify legitimate closures vs potentially erroneous 0-PnL records.
func (d *Database) HasClosedPositionWithPnL(traderID, symbol, side string, openedAt time.Time) (bool, error) {
	if d.db == nil {
		return false, fmt.Errorf("database not initialized")
	}

	// Generate expected position ID
	positionID := fmt.Sprintf("%s_%s_%d", traderID, symbol, openedAt.Unix())

	query := `
		SELECT realized_pnl, exit_price FROM trader_positions
		WHERE id = ? AND closed_at IS NOT NULL
	`

	var realizedPnL sql.NullFloat64
	var exitPrice sql.NullFloat64
	err := d.db.QueryRow(query, positionID).Scan(&realizedPnL, &exitPrice)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // Position not found
		}
		return false, fmt.Errorf("failed to check position PnL: %w", err)
	}

	// Consider valid if either PnL is non-zero OR exit price is set
	hasValidPnL := realizedPnL.Valid && realizedPnL.Float64 != 0
	hasValidExitPrice := exitPrice.Valid && exitPrice.Float64 > 0

	return hasValidPnL || hasValidExitPrice, nil
}

// PendingOrderRecord represents a pending/unfilled order in the database
type PendingOrderRecord struct {
	ID              string
	TraderID        string
	Symbol          string
	Side            string
	OrderType       string // "stop_loss", "take_profit", "limit", "market"
	TriggerPrice    float64
	Quantity        float64
	FilledQuantity  float64
	Status          string // "pending", "filled", "cancelled", "rejected"
	ParentPositionID string
	ExchangeOrderID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	FilledAt        *time.Time
	CancelledAt     *time.Time
}

// SavePendingOrder saves or updates a pending order in the database
func (d *Database) SavePendingOrder(order *PendingOrderRecord) error {
	if d.db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `
		INSERT OR REPLACE INTO pending_orders 
		(id, trader_id, symbol, side, order_type, trigger_price, quantity, filled_quantity, status, parent_position_id, exchange_order_id, created_at, updated_at, filled_at, cancelled_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var filledAtStr, cancelledAtStr interface{}
	if order.FilledAt != nil {
		filledAtStr = order.FilledAt.Format("2006-01-02 15:04:05")
	}
	if order.CancelledAt != nil {
		cancelledAtStr = order.CancelledAt.Format("2006-01-02 15:04:05")
	}

	_, err := d.db.Exec(query,
		order.ID,
		order.TraderID,
		order.Symbol,
		order.Side,
		order.OrderType,
		order.TriggerPrice,
		order.Quantity,
		order.FilledQuantity,
		order.Status,
		order.ParentPositionID,
		order.ExchangeOrderID,
		order.CreatedAt.Format("2006-01-02 15:04:05"),
		time.Now().Format("2006-01-02 15:04:05"),
		filledAtStr,
		cancelledAtStr,
	)
	return err
}

// GetPendingOrders retrieves pending orders for a trader
func (d *Database) GetPendingOrders(traderID string) ([]*PendingOrderRecord, error) {
	if d.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT id, trader_id, symbol, side, order_type, trigger_price, quantity, filled_quantity, status, parent_position_id, exchange_order_id, created_at, updated_at, filled_at, cancelled_at
		FROM pending_orders
		WHERE trader_id = ? AND status = 'pending'
		ORDER BY created_at DESC
	`

	rows, err := d.db.Query(query, traderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*PendingOrderRecord
	for rows.Next() {
		var order PendingOrderRecord
		var filledAtStr, cancelledAtStr sql.NullString
		err := rows.Scan(
			&order.ID, &order.TraderID, &order.Symbol, &order.Side,
			&order.OrderType, &order.TriggerPrice, &order.Quantity, &order.FilledQuantity,
			&order.Status, &order.ParentPositionID, &order.ExchangeOrderID,
			&order.CreatedAt, &order.UpdatedAt, &filledAtStr, &cancelledAtStr,
		)
		if err != nil {
			continue
		}
		if filledAtStr.Valid {
			t, _ := time.Parse("2006-01-02 15:04:05", filledAtStr.String)
			order.FilledAt = &t
		}
		if cancelledAtStr.Valid {
			t, _ := time.Parse("2006-01-02 15:04:05", cancelledAtStr.String)
			order.CancelledAt = &t
		}
		orders = append(orders, &order)
	}

	return orders, nil
}

// UpdatePendingOrderStatus updates the status of a pending order
func (d *Database) UpdatePendingOrderStatus(orderID, status string, filledAt *time.Time) error {
	if d.db == nil {
		return fmt.Errorf("database not initialized")
	}

	var query string
	var args []interface{}

	if status == "filled" && filledAt != nil {
		query = `UPDATE pending_orders SET status = ?, filled_at = ?, updated_at = ? WHERE id = ?`
		args = []interface{}{status, filledAt.Format("2006-01-02 15:04:05"), time.Now().Format("2006-01-02 15:04:05"), orderID}
	} else if status == "cancelled" {
		query = `UPDATE pending_orders SET status = ?, cancelled_at = ?, updated_at = ? WHERE id = ?`
		args = []interface{}{status, time.Now().Format("2006-01-02 15:04:05"), time.Now().Format("2006-01-02 15:04:05"), orderID}
	} else {
		query = `UPDATE pending_orders SET status = ?, updated_at = ? WHERE id = ?`
		args = []interface{}{status, time.Now().Format("2006-01-02 15:04:05"), orderID}
	}

	_, err := d.db.Exec(query, args...)
	return err
}

// GetPendingOrderByExchangeID retrieves a pending order by exchange order ID
func (d *Database) GetPendingOrderByExchangeID(traderID, exchangeOrderID string) (*PendingOrderRecord, error) {
	if d.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT id, trader_id, symbol, side, order_type, trigger_price, quantity, filled_quantity, status, parent_position_id, exchange_order_id, created_at, updated_at, filled_at, cancelled_at
		FROM pending_orders
		WHERE trader_id = ? AND exchange_order_id = ?
		LIMIT 1
	`

	var order PendingOrderRecord
	var filledAtStr, cancelledAtStr sql.NullString
	err := d.db.QueryRow(query, traderID, exchangeOrderID).Scan(
		&order.ID, &order.TraderID, &order.Symbol, &order.Side,
		&order.OrderType, &order.TriggerPrice, &order.Quantity, &order.FilledQuantity,
		&order.Status, &order.ParentPositionID, &order.ExchangeOrderID,
		&order.CreatedAt, &order.UpdatedAt, &filledAtStr, &cancelledAtStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if filledAtStr.Valid {
		t, _ := time.Parse("2006-01-02 15:04:05", filledAtStr.String)
		order.FilledAt = &t
	}
	if cancelledAtStr.Valid {
		t, _ := time.Parse("2006-01-02 15:04:05", cancelledAtStr.String)
		order.CancelledAt = &t
	}

	return &order, nil
}

// DeletePendingOrdersForPosition deletes all pending orders associated with a position
func (d *Database) DeletePendingOrdersForPosition(traderID, symbol, side string) error {
	if d.db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `DELETE FROM pending_orders WHERE trader_id = ? AND symbol = ? AND side = ? AND status = 'pending'`
	result, err := d.db.Exec(query, traderID, symbol, side)
	if err != nil {
		return err
	}

	_, _ = result.RowsAffected()
	return nil
}

// EquityHistoryRecord represents an equity history point in the database
type EquityHistoryRecord struct {
	ID              int64
	TraderID        string
	Timestamp       time.Time
	TotalEquity     float64
	AvailableBalance float64
	TotalPnL        float64
	TotalPnLPct     float64
	PositionCount   int
	MarginUsedPct   float64
	CycleNumber     int
	CreatedAt       time.Time
}

// SaveEquityHistory saves an equity history point to the database with retry logic
// CRITICAL: Validates data to prevent invalid snapshots that cause -100% PnL in competition chart
func (d *Database) SaveEquityHistory(traderID string, timestamp time.Time, totalEquity, availableBalance, totalPnL, totalPnLPct float64, positionCount int, marginUsedPct float64, cycleNumber int) error {
	// VALIDATION: Prevent saving invalid equity snapshots
	// These checks prevent -100% PnL errors in competition chart
	if totalEquity <= 0 {
		log.Printf("⚠️ SaveEquityHistory: Skipping invalid snapshot for trader %s - totalEquity is %.4f (must be > 0)", traderID, totalEquity)
		return nil // Skip silently - don't save invalid data
	}
	if availableBalance < 0 {
		log.Printf("⚠️ SaveEquityHistory: Skipping invalid snapshot for trader %s - availableBalance is %.4f (must be >= 0)", traderID, availableBalance)
		return nil
	}
	// Check for extreme PnL values that indicate data errors
	if totalPnLPct <= -100 {
		log.Printf("⚠️ SaveEquityHistory: Skipping invalid snapshot for trader %s - totalPnLPct is %.2f%% (indicates data error, equity=%.4f, balance=%.4f)",
			traderID, totalPnLPct, totalEquity, availableBalance)
		return nil
	}
	// Validate timestamp is reasonable
	if timestamp.IsZero() || timestamp.Before(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) {
		log.Printf("⚠️ SaveEquityHistory: Skipping invalid snapshot for trader %s - timestamp is invalid: %v", traderID, timestamp)
		return nil
	}

	query := `
		INSERT INTO trader_equity_history 
		(trader_id, timestamp, total_equity, available_balance, total_pnl, total_pnl_pct, position_count, margin_used_pct, cycle_number)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	
	maxRetries := 3
	var lastErr error
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := d.db.Exec(query, traderID, timestamp.Format("2006-01-02 15:04:05"), totalEquity, availableBalance, totalPnL, totalPnLPct, positionCount, marginUsedPct, cycleNumber)
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// Check if it's a transient error (database locked, busy, etc.)
		errStr := err.Error()
		isTransient := strings.Contains(errStr, "database is locked") ||
			strings.Contains(errStr, "database disk image is malformed") ||
			strings.Contains(errStr, "disk I/O error")
		
		if !isTransient || attempt >= maxRetries {
			break
		}
		
		// Wait before retry (exponential backoff)
		waitTime := time.Duration(attempt*50) * time.Millisecond
		time.Sleep(waitTime)
	}
	
	return fmt.Errorf("failed to save equity history after %d attempts: %w", maxRetries, lastErr)
}

// GetEquityHistory retrieves equity history for a trader (latest N records, oldest to newest)
// CRITICAL: Filters out invalid records to prevent -100% PnL in competition chart
func (d *Database) GetEquityHistory(traderID string, limit int) ([]*EquityHistoryRecord, error) {
	if limit <= 0 {
		limit = 10000 // Default to 10000 records
	}
	
	// Query with validation filters to exclude invalid snapshots
	// This prevents -100% PnL errors in competition chart
	query := `
		SELECT id, trader_id, timestamp, total_equity, available_balance, total_pnl, total_pnl_pct, position_count, margin_used_pct, cycle_number, created_at
		FROM trader_equity_history
		WHERE trader_id = ?
			AND total_equity > 0
			AND total_pnl_pct > -100
		ORDER BY timestamp ASC, cycle_number ASC
		LIMIT ?
	`
	
	rows, err := d.db.Query(query, traderID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*EquityHistoryRecord
	for rows.Next() {
		var rec EquityHistoryRecord
		var timestampStr string
		var createdAtStr string

		err := rows.Scan(&rec.ID, &rec.TraderID, &timestampStr, &rec.TotalEquity, &rec.AvailableBalance, &rec.TotalPnL, &rec.TotalPnLPct, &rec.PositionCount, &rec.MarginUsedPct, &rec.CycleNumber, &createdAtStr)
		if err != nil {
			return nil, err
		}

		rec.Timestamp, err = time.Parse("2006-01-02 15:04:05", timestampStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %w", err)
		}

		rec.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAtStr)
		if err != nil {
			// CreatedAt parsing error is not critical, continue
			rec.CreatedAt = time.Now()
		}

		records = append(records, &rec)
	}

	return records, rows.Err()
}

// CleanupInvalidEquityHistory removes invalid equity history records
// This should be run once to clean up historical bad data
func (d *Database) CleanupInvalidEquityHistory() (int64, error) {
	if d.db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	// Delete records with invalid equity or extreme PnL
	query := `
		DELETE FROM trader_equity_history 
		WHERE total_equity <= 0 
			OR total_pnl_pct <= -100
			OR available_balance < 0
	`
	
	result, err := d.db.Exec(query)
	if err != nil {
		return 0, err
	}
	
	deleted, _ := result.RowsAffected()
	if deleted > 0 {
		log.Printf("🧹 CleanupInvalidEquityHistory: Deleted %d invalid equity history records", deleted)
	}
	return deleted, nil
}

// VerifyDatabaseHealth checks if database file exists and is accessible
func (d *Database) VerifyDatabaseHealth() error {
	if d.db == nil {
		return fmt.Errorf("database connection is nil")
	}
	
	// Test database connection
	if err := d.db.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	
	// Verify critical tables exist
	tables := []string{"trader_equity_history", "traders", "users"}
	for _, table := range tables {
		var count int
		err := d.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='%s'", table)).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check table %s: %w", table, err)
		}
		if count == 0 {
			return fmt.Errorf("critical table %s does not exist", table)
		}
	}
	
	return nil
}

// GetEquityHistoryCount returns the total number of equity history records for a trader
func (d *Database) GetEquityHistoryCount(traderID string) (int, error) {
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(*) 
		FROM trader_equity_history 
		WHERE trader_id = ?
	`, traderID).Scan(&count)
	return count, err
}

// GetAllTradersWithEquityHistory returns list of trader IDs that have equity history
func (d *Database) GetAllTradersWithEquityHistory() ([]string, error) {
	rows, err := d.db.Query(`
		SELECT DISTINCT trader_id 
		FROM trader_equity_history 
		ORDER BY trader_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var traderIDs []string
	for rows.Next() {
		var traderID string
		if err := rows.Scan(&traderID); err != nil {
			continue
		}
		traderIDs = append(traderIDs, traderID)
	}
	return traderIDs, rows.Err()
}

// BatchSaveEquityHistory saves multiple equity history points in a single transaction
// This is more efficient than individual saves and ensures atomicity
func (d *Database) BatchSaveEquityHistory(records []*EquityHistoryRecord) error {
	if len(records) == 0 {
		return nil
	}
	
	// Start transaction
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	
	query := `
		INSERT INTO trader_equity_history 
		(trader_id, timestamp, total_equity, available_balance, total_pnl, total_pnl_pct, position_count, margin_used_pct, cycle_number)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	
	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()
	
	// Insert all records
	for _, rec := range records {
		_, err := stmt.Exec(
			rec.TraderID,
			rec.Timestamp.Format("2006-01-02 15:04:05"),
			rec.TotalEquity,
			rec.AvailableBalance,
			rec.TotalPnL,
			rec.TotalPnLPct,
			rec.PositionCount,
			rec.MarginUsedPct,
			rec.CycleNumber,
		)
		if err != nil {
			return fmt.Errorf("failed to insert record for trader %s: %w", rec.TraderID, err)
		}
	}
	
	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return nil
}

// GetOpenPositions get open positions for a trader
func (d *Database) GetOpenPositions(traderID string) ([]*PositionRecord, error) {
	query := `
		SELECT id, trader_id, symbol, side, entry_price, exit_price, quantity, entry_fee, exit_fee, realized_pnl, leverage, opened_at, closed_at, order_id_open, order_id_close, stop_loss_price, take_profit_price
		FROM trader_positions
		WHERE trader_id = ? AND closed_at IS NULL
		ORDER BY opened_at DESC
	`

	rows, err := d.db.Query(query, traderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []*PositionRecord
	for rows.Next() {
		var pos PositionRecord
		var closedAtStr sql.NullString
		var exitPrice sql.NullFloat64
		var realizedPnL sql.NullFloat64
		var stopLossPrice sql.NullFloat64
		var takeProfitPrice sql.NullFloat64

		err := rows.Scan(&pos.ID, &pos.TraderID, &pos.Symbol, &pos.Side, &pos.EntryPrice, &exitPrice, &pos.Quantity, &pos.EntryFee, &pos.ExitFee, &realizedPnL, &pos.Leverage, &pos.OpenedAt, &closedAtStr, &pos.OrderIDOpen, &pos.OrderIDClose, &stopLossPrice, &takeProfitPrice)
		if err != nil {
			return nil, err
		}

		// Handle nullable exit_price
		if exitPrice.Valid {
			pos.ExitPrice = exitPrice.Float64
		} else {
			pos.ExitPrice = 0
		}

		// Handle nullable realized_pnl
		if realizedPnL.Valid {
			pos.RealizedPnL = realizedPnL.Float64
		} else {
			pos.RealizedPnL = 0
		}

		if closedAtStr.Valid {
			closedAt, err := time.Parse("2006-01-02 15:04:05", closedAtStr.String)
			if err == nil {
				pos.ClosedAt = &closedAt
			}
		}

		if stopLossPrice.Valid {
			pos.StopLossPrice = stopLossPrice.Float64
		}

		if takeProfitPrice.Valid {
			pos.TakeProfitPrice = takeProfitPrice.Float64
		}

		positions = append(positions, &pos)
	}

	return positions, rows.Err()
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
