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

// DatabaseInterface 定义了数据库实现需要提供的方法集合
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

// Database 配置数据库
type Database struct {
	db            *sql.DB
	cryptoService *crypto.CryptoService
}

// NewDatabase 创建配置数据库
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return nil, fmt.Errorf("启用外键失败: %w", err)
	}
	if err := tuneSQLiteConnection(db); err != nil {
		return nil, err
	}

	// 🔒 启用 WAL 模式,提高并发性能和崩溃恢复能力
	// WAL (Write-Ahead Logging) 模式的优势:
	// 1. 更好的并发性能:读操作不会被写操作阻塞
	// 2. 崩溃安全:即使在断电或强制终止时也能保证数据完整性
	// 3. 更快的写入:不需要每次都写入主数据库文件
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("启用WAL模式失败: %w", err)
	}

	// 🔒 设置 synchronous=FULL 确保数据持久性
	// FULL (2) 模式: 确保数据在关键时刻完全写入磁盘
	// 配合 WAL 模式,在保证数据安全的同时获得良好性能
	if _, err := db.Exec("PRAGMA synchronous=FULL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("设置synchronous失败: %w", err)
	}

	database := &Database{db: db}
	if err := database.createTables(); err != nil {
		return nil, fmt.Errorf("创建表失败: %w", err)
	}
	if err := database.ensureBacktestRunColumns(); err != nil {
		return nil, fmt.Errorf("初始化回测表结构失败: %w", err)
	}

	// 确保存在默认用户（用于外键约束和默认配置种子）
	if _, err := db.Exec(`
		INSERT OR IGNORE INTO users (id, email, password_hash, otp_secret, otp_verified, role)
		VALUES ('default', 'default@local', '__default__', '', 1, 'user')
	`); err != nil {
		return nil, fmt.Errorf("创建默认用户失败: %w", err)
	}

	if err := database.initDefaultData(); err != nil {
		return nil, fmt.Errorf("初始化默认数据失败: %w", err)
	}

	// 迁移提示词模板从文件到数据库
	if err := database.migratePromptTemplatesFromFiles(); err != nil {
		log.Printf("⚠️  提示词模板迁移失败: %v", err)
		// 不返回错误，允许系统继续运行
	}

	log.Printf("✅ 数据库已启用 WAL 模式和 FULL 同步,数据持久性得到保证")
	return database, nil
}

// columnExists 检查表中是否存在指定列
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

// createTables 创建数据库表
func (d *Database) createTables() error {
	queries := []string{
		// AI模型配置表
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

		// 交易所配置表
		`CREATE TABLE IF NOT EXISTS exchanges (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			type TEXT NOT NULL, -- 'cex' or 'dex'
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			secret_key TEXT DEFAULT '',
			testnet BOOLEAN DEFAULT 0,
			-- Hyperliquid 特定字段
			hyperliquid_wallet_addr TEXT DEFAULT '',
			-- Aster 特定字段
			aster_user TEXT DEFAULT '',
			aster_signer TEXT DEFAULT '',
			aster_private_key TEXT DEFAULT '',
			-- LIGHTER 特定字段
			lighter_wallet_addr TEXT DEFAULT '',
			lighter_private_key TEXT DEFAULT '',
			lighter_api_key_private_key TEXT DEFAULT '',
			-- OKX 特定字段
			okx_passphrase TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// 用户信号源配置表
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

		// 交易员配置表
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

		// 用户表
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

		// 系统配置表
		`CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// 提示词模板表
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

		// 回测运行主表
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

		// 回测检查点
		`CREATE TABLE IF NOT EXISTS backtest_checkpoints (
			run_id TEXT PRIMARY KEY,
			payload BLOB NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (run_id) REFERENCES backtest_runs(run_id) ON DELETE CASCADE
		)`,

		// 回测权益曲线
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

		// 回测交易记录
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

		// 回测指标
		`CREATE TABLE IF NOT EXISTS backtest_metrics (
			run_id TEXT PRIMARY KEY,
			payload BLOB NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (run_id) REFERENCES backtest_runs(run_id) ON DELETE CASCADE
		)`,

		// 回测决策日志
		`CREATE TABLE IF NOT EXISTS backtest_decisions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			cycle INTEGER NOT NULL,
			payload BLOB NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (run_id) REFERENCES backtest_runs(run_id) ON DELETE CASCADE
		)`,

		// 索引
		`CREATE INDEX IF NOT EXISTS idx_backtest_runs_state ON backtest_runs(state, updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_backtest_equity_run_ts ON backtest_equity(run_id, ts)`,
		`CREATE INDEX IF NOT EXISTS idx_backtest_trades_run_ts ON backtest_trades(run_id, ts)`,
		`CREATE INDEX IF NOT EXISTS idx_backtest_decisions_run_cycle ON backtest_decisions(run_id, cycle)`,

		// 内测码表
		`CREATE TABLE IF NOT EXISTS beta_codes (
			code TEXT PRIMARY KEY,
			used BOOLEAN DEFAULT 0,
			used_by TEXT DEFAULT '',
			used_at DATETIME DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// 触发器：自动更新 updated_at
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

		// Webhook API Keys表
		`CREATE TABLE IF NOT EXISTS webhook_api_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT UNIQUE NOT NULL,
			api_key TEXT UNIQUE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// TradingView Alerts表
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

		// 交易员申请表
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

		// 索引
		`CREATE INDEX IF NOT EXISTS idx_tradingview_alerts_user ON tradingview_alerts(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tradingview_alerts_trader ON tradingview_alerts(trader_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tradingview_alerts_status ON tradingview_alerts(status)`,
		`CREATE INDEX IF NOT EXISTS idx_trader_applications_user_id ON trader_applications(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_trader_applications_status ON trader_applications(status)`,

		// 触发器：自动更新 webhook_api_keys updated_at
		`CREATE TRIGGER IF NOT EXISTS update_webhook_api_keys_updated_at
			AFTER UPDATE ON webhook_api_keys
			BEGIN
				UPDATE webhook_api_keys SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("执行SQL失败 [%s]: %w", query, err)
		}
	}

	// 为现有数据库添加新字段（向后兼容）
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
		// 检查列是否已存在，如果不存在则添加
		exists, err := d.columnExists(alterQuery.table, alterQuery.column)
		if err != nil {
			log.Printf("⚠️  检查列 %s.%s 是否存在时出错: %v", alterQuery.table, alterQuery.column, err)
			// 继续尝试添加列，可能表不存在或列已存在
		}

		if !exists {
			if _, err := d.db.Exec(alterQuery.query); err != nil {
				// 记录错误，但继续执行（列可能已存在或表不存在）
				log.Printf("⚠️  ALTER TABLE 警告 (可能已存在列 %s.%s): %v", alterQuery.table, alterQuery.column, err)
			} else {
				log.Printf("✅ 成功添加列 %s.%s", alterQuery.table, alterQuery.column)
			}
		} else {
			log.Printf("ℹ️  列 %s.%s 已存在，跳过", alterQuery.table, alterQuery.column)
		}
	}

	// 检查是否需要迁移exchanges表的主键结构
	err := d.migrateExchangesTable()
	if err != nil {
		log.Printf("⚠️ 迁移exchanges表失败: %v", err)
	}

	// 修复traders表的外键约束问题
	err = d.migrateTradersTable()
	if err != nil {
		log.Printf("⚠️ 迁移traders表失败: %v", err)
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
			return fmt.Errorf("执行 %s 失败: %w", stmt, err)
		}
	}
	return nil
}

// initDefaultData 初始化默认数据
func (d *Database) initDefaultData() error {
	// 初始化AI模型（使用default用户）
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
			return fmt.Errorf("初始化AI模型失败: %w", err)
		}
	}

	// 初始化交易所（使用default用户）
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
			return fmt.Errorf("初始化交易所失败: %w", err)
		}
	}

	// 初始化系统配置 - 创建所有字段，设置默认值，后续由config.json同步更新
	systemConfigs := map[string]string{
		"beta_mode":            "false",                                                                               // 默认关闭内测模式
		"api_server_port":      "8080",                                                                                // 默认API端口
		"use_default_coins":    "true",                                                                                // 默认使用内置币种列表
		"default_coins":        `["BTCUSDT","ETHUSDT","SOLUSDT","BNBUSDT","XRPUSDT","DOGEUSDT","ADAUSDT","HYPEUSDT"]`, // 默认币种列表（JSON格式）
		"max_daily_loss":       "10.0",                                                                                // 最大日损失百分比
		"max_drawdown":         "20.0",                                                                                // 最大回撤百分比
		"stop_trading_minutes": "60",                                                                                  // 停止交易时间（分钟）
		"btc_eth_leverage":     "5",                                                                                   // BTC/ETH杠杆倍数
		"altcoin_leverage":     "5",                                                                                   // 山寨币杠杆倍数
		"jwt_secret":           "",                                                                                    // JWT密钥，默认为空，由config.json或系统生成
		"registration_enabled": "true",                                                                                // 默认允许注册
	}

	for key, value := range systemConfigs {
		_, err := d.db.Exec(`
			INSERT OR IGNORE INTO system_config (key, value) 
			VALUES (?, ?)
		`, key, value)
		if err != nil {
			return fmt.Errorf("初始化系统配置失败: %w", err)
		}
	}

	return nil
}

// migrateExchangesTable 迁移exchanges表支持多用户
func (d *Database) migrateExchangesTable() error {
	// 检查是否已经迁移过
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type='table' AND name='exchanges_new'
	`).Scan(&count)
	if err != nil {
		return err
	}

	// 如果已经迁移过，直接返回
	if count > 0 {
		return nil
	}

	log.Printf("🔄 开始迁移exchanges表...")

	// 创建新的exchanges表，使用复合主键
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
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id, user_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("创建新exchanges表失败: %w", err)
	}

	// 复制数据到新表
	_, err = d.db.Exec(`
		INSERT INTO exchanges_new 
		SELECT * FROM exchanges
	`)
	if err != nil {
		return fmt.Errorf("复制数据失败: %w", err)
	}

	// 删除旧表
	_, err = d.db.Exec(`DROP TABLE exchanges`)
	if err != nil {
		return fmt.Errorf("删除旧表失败: %w", err)
	}

	// 重命名新表
	_, err = d.db.Exec(`ALTER TABLE exchanges_new RENAME TO exchanges`)
	if err != nil {
		return fmt.Errorf("重命名表失败: %w", err)
	}

	// 重新创建触发器
	_, err = d.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_exchanges_updated_at
			AFTER UPDATE ON exchanges
			BEGIN
				UPDATE exchanges SET updated_at = CURRENT_TIMESTAMP 
				WHERE id = NEW.id AND user_id = NEW.user_id;
			END
	`)
	if err != nil {
		return fmt.Errorf("创建触发器失败: %w", err)
	}

	log.Printf("✅ exchanges表迁移完成")
	return nil
}

// migrateTradersTable 迁移traders表，移除外键约束
func (d *Database) migrateTradersTable() error {
	// 检查traders表是否存在外键约束（通过尝试创建一个测试记录来判断）
	// 如果表已经没有外键约束，则跳过迁移
	var tableSQL string
	err := d.db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='traders'`).Scan(&tableSQL)
	if err != nil {
		// 表不存在，无需迁移
		return nil
	}

	// 检查是否包含 FOREIGN KEY (exchange_id) 或 FOREIGN KEY (ai_model_id)
	if !strings.Contains(tableSQL, "FOREIGN KEY (exchange_id)") && !strings.Contains(tableSQL, "FOREIGN KEY (ai_model_id)") {
		// 已经没有这些外键约束，无需迁移
		return nil
	}

	log.Printf("🔄 开始迁移traders表，移除外键约束...")

	// 创建新的traders表，不包含exchange_id和ai_model_id的外键约束
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
		return fmt.Errorf("创建新traders表失败: %w", err)
	}

	// 复制数据到新表
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
		// 如果复制失败，删除新表
		d.db.Exec(`DROP TABLE traders_new`)
		return fmt.Errorf("复制traders数据失败: %w", err)
	}

	// 删除旧表
	_, err = d.db.Exec(`DROP TABLE traders`)
	if err != nil {
		return fmt.Errorf("删除旧traders表失败: %w", err)
	}

	// 重命名新表
	_, err = d.db.Exec(`ALTER TABLE traders_new RENAME TO traders`)
	if err != nil {
		return fmt.Errorf("重命名traders表失败: %w", err)
	}

	log.Printf("✅ traders表迁移完成，已移除外键约束")
	return nil
}

// migratePromptTemplatesFromFiles 从文件系统迁移提示词模板到数据库
func (d *Database) migratePromptTemplatesFromFiles() error {
	// 检查是否已经有模板在数据库中
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM prompt_templates`).Scan(&count)
	if err != nil {
		return fmt.Errorf("检查提示词模板表失败: %w", err)
	}

	// 如果已经有模板，跳过迁移
	if count > 0 {
		log.Printf("✓ 提示词模板已存在于数据库，跳过迁移")
		return nil
	}

	log.Printf("🔄 开始从文件系统迁移提示词模板到数据库...")

	// 读取prompts目录下的所有.txt文件
	promptsDir := "prompts"
	files, err := filepath.Glob(filepath.Join(promptsDir, "*.txt"))
	if err != nil {
		return fmt.Errorf("扫描提示词目录失败: %w", err)
	}

	if len(files) == 0 {
		log.Printf("⚠️  提示词目录 %s 中没有找到 .txt 文件", promptsDir)
		return nil
	}

	// 迁移每个模板文件
	migratedCount := 0
	for _, file := range files {
		// 读取文件内容
		content, err := os.ReadFile(file)
		if err != nil {
			log.Printf("⚠️  读取提示词文件失败 %s: %v", file, err)
			continue
		}

		// 提取文件名（不含扩展名）作为模板ID和名称
		fileName := filepath.Base(file)
		templateID := strings.TrimSuffix(fileName, filepath.Ext(fileName))
		templateName := templateID

		// 插入到数据库（使用default用户，标记为系统模板）
		_, err = d.db.Exec(`
			INSERT INTO prompt_templates (id, user_id, name, content, is_system, created_at, updated_at)
			VALUES (?, 'default', ?, ?, 1, datetime('now'), datetime('now'))
		`, templateID, templateName, string(content))
		if err != nil {
			log.Printf("⚠️  插入提示词模板失败 %s: %v", templateID, err)
			continue
		}

		migratedCount++
		log.Printf("  📄 已迁移提示词模板: %s", templateID)
	}

	log.Printf("✅ 提示词模板迁移完成，共迁移 %d 个模板", migratedCount)
	return nil
}

// User 用户配置
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // 不返回到前端
	OTPSecret    string    `json:"-"` // 不返回到前端
	OTPVerified  bool      `json:"otp_verified"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AIModelConfig AI模型配置
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

// ExchangeConfig 交易所配置
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
	// Aster 特定字段
	AsterUser       string `json:"asterUser"`
	AsterSigner     string `json:"asterSigner"`
	AsterPrivateKey string `json:"asterPrivateKey"`
	// LIGHTER 特定字段
	LighterWalletAddr       string `json:"lighterWalletAddr"`       // Ethereum 钱包地址 (L1)
	LighterPrivateKey       string `json:"lighterPrivateKey"`       // L1私钥（用于识别账户）
	LighterAPIKeyPrivateKey string `json:"lighterAPIKeyPrivateKey"` // API Key私钥（40字节，用于签名交易）
	// OKX 特定字段
	OkxPassphrase string    `json:"okxPassphrase"` // OKX passphrase (required for OKX)
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TraderRecord 交易员配置（数据库实体）
type TraderRecord struct {
	ID                   string    `json:"id"`
	UserID               string    `json:"user_id"`
	Name                 string    `json:"name"`
	AIModelID            string    `json:"ai_model_id"`
	ExchangeID           string    `json:"exchange_id"`
	InitialBalance       float64   `json:"initial_balance"`
	ScanIntervalMinutes  int       `json:"scan_interval_minutes"`
	IsRunning            bool      `json:"is_running"`
	BTCETHLeverage       int       `json:"btc_eth_leverage"`       // BTC/ETH杠杆倍数
	AltcoinLeverage      int       `json:"altcoin_leverage"`       // 山寨币杠杆倍数
	TradingSymbols       string    `json:"trading_symbols"`        // 交易币种，逗号分隔
	UseCoinPool          bool      `json:"use_coin_pool"`          // 是否使用COIN POOL信号源
	UseOITop             bool      `json:"use_oi_top"`             // 是否使用OI TOP信号源
	UseTradingView       bool      `json:"use_tradingview"`        // 是否使用TradingView信号源
	FollowedTraderID     string    `json:"followed_trader_id"`     // 跟随的交易员ID（用于follower角色）
	CustomPrompt         string    `json:"custom_prompt"`          // 自定义交易策略prompt
	OverrideBasePrompt   bool      `json:"override_base_prompt"`   // 是否覆盖基础prompt
	SystemPromptTemplate string    `json:"system_prompt_template"` // 系统提示词模板名称
	IsCrossMargin        bool      `json:"is_cross_margin"`        // 是否为全仓模式（true=全仓，false=逐仓）
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// PromptTemplateConfig 提示词模板配置
type PromptTemplateConfig struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	IsSystem  bool      `json:"is_system"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserSignalSource 用户信号源配置
type UserSignalSource struct {
	ID          int       `json:"id"`
	UserID      string    `json:"user_id"`
	CoinPoolURL string    `json:"coin_pool_url"`
	OITopURL    string    `json:"oi_top_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TradingViewAlert TradingView警报
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

// TraderApplication 交易员申请
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

// GenerateOTPSecret 生成OTP密钥
func GenerateOTPSecret() (string, error) {
	secret := make([]byte, 20)
	_, err := rand.Read(secret)
	if err != nil {
		return "", err
	}
	return base32.StdEncoding.EncodeToString(secret), nil
}

// CreateUser 创建用户
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

// EnsureAdminUser 确保admin用户存在（用于管理员模式）
func (d *Database) EnsureAdminUser() error {
	// 检查admin用户是否已存在
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id = 'admin'`).Scan(&count)
	if err != nil {
		return err
	}

	// 如果已存在，直接返回
	if count > 0 {
		return nil
	}

	// 创建admin用户（密码为空，因为管理员模式下不需要密码）
	adminUser := &User{
		ID:           "admin",
		Email:        "admin@localhost",
		PasswordHash: "", // 管理员模式下不使用密码
		OTPSecret:    "",
		OTPVerified:  true,
		Role:         "user",
	}

	return d.CreateUser(adminUser)
}

// GetUserByEmail 通过邮箱获取用户
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
		log.Printf("❌ GetUserByEmail 失败: email=%s, error=%v", email, err)
		return nil, err
	}
	if role.Valid {
		user.Role = role.String
		if user.Role == "" {
			log.Printf("⚠️  GetUserByEmail: 用户 %s 的 role 字段为空，使用默认值 'follower'", email)
			user.Role = "follower"
		}
	} else {
		log.Printf("⚠️  GetUserByEmail: 用户 %s 的 role 字段为 NULL，使用默认值 'follower'", email)
		user.Role = "follower"
	}
	user.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	user.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	log.Printf("✅ GetUserByEmail: 成功获取用户 email=%s, id=%s, role=%s", email, user.ID, user.Role)
	return &user, nil
}

// GetUserByID 通过ID获取用户
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
		log.Printf("❌ GetUserByID 失败: userID=%s, error=%v", userID, err)
		return nil, err
	}
	if role.Valid {
		user.Role = role.String
		if user.Role == "" {
			log.Printf("⚠️  GetUserByID: 用户 %s 的 role 字段为空，使用默认值 'follower'", userID)
			user.Role = "follower"
		}
	} else {
		log.Printf("⚠️  GetUserByID: 用户 %s 的 role 字段为 NULL，使用默认值 'follower'", userID)
		user.Role = "follower"
	}
	user.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	user.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	log.Printf("✅ GetUserByID: 成功获取用户 userID=%s, email=%s, role=%s", userID, user.Email, user.Role)
	return &user, nil
}

// GetAllUsers 获取所有用户ID列表
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

// GetAllUsersWithRoles 获取所有用户及其角色（admin使用）
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
					log.Printf("⚠️  GetAllUsersWithRoles: 无法解析 created_at '%s' for user %s: %v", createdAt, user.ID, err)
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
					log.Printf("⚠️  GetAllUsersWithRoles: 无法解析 updated_at '%s' for user %s: %v", updatedAt, user.ID, err)
					// Use current time as fallback instead of zero time
					user.UpdatedAt = time.Now()
				}
			}
		}
		users = append(users, &user)
	}
	return users, nil
}

// UpdateUserRole 更新用户角色（admin使用）
func (d *Database) UpdateUserRole(userID string, role string) error {
	// 验证角色值
	validRoles := map[string]bool{"user": true, "follower": true, "admin": true}
	if !validRoles[role] {
		return fmt.Errorf("无效的角色值: %s", role)
	}

	_, err := d.db.Exec(`
		UPDATE users SET role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, role, userID)
	return err
}

// CreateTraderApplication 创建交易员申请
func (d *Database) CreateTraderApplication(app *TraderApplication) error {
	_, err := d.db.Exec(`
		INSERT INTO trader_applications (id, user_id, name, email, description, trading_experience, strategy_overview, social_links, status, admin_notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, app.ID, app.UserID, app.Name, app.Email, app.Description, app.TradingExperience, app.StrategyOverview, app.SocialLinks, app.Status, app.AdminNotes)
	return err
}

// GetTraderApplicationByID 通过ID获取交易员申请
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

// GetTraderApplicationByUserID 通过用户ID获取交易员申请
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

// GetAllTraderApplications 获取所有交易员申请（admin使用）
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

// UpdateTraderApplicationStatus 更新交易员申请状态
func (d *Database) UpdateTraderApplicationStatus(id string, status string, adminNotes string) error {
	_, err := d.db.Exec(`
		UPDATE trader_applications SET status = ?, admin_notes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, status, adminNotes, id)
	return err
}

// GetAllTraders 获取所有交易员（admin使用）
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

// UpdateUserOTPVerified 更新用户OTP验证状态
func (d *Database) UpdateUserOTPVerified(userID string, verified bool) error {
	_, err := d.db.Exec(`UPDATE users SET otp_verified = ? WHERE id = ?`, verified, userID)
	return err
}

// UpdateUserPassword 更新用户密码
func (d *Database) UpdateUserPassword(userID, passwordHash string) error {
	_, err := d.db.Exec(`
		UPDATE users
		SET password_hash = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, passwordHash, userID)
	return err
}

// GetAIModels 获取用户的AI模型配置
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

	// 初始化为空切片而不是nil，确保JSON序列化为[]而不是null
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
		// 解析时间字符串
		model.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		model.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		// 解密API Key
		model.APIKey = d.decryptSensitiveData(model.APIKey)
		models = append(models, &model)
	}

	return models, nil
}

// GetAIModel 根据模型ID和用户ID获取单个AI模型配置，若用户下不存在则回退到default用户。
func (d *Database) GetAIModel(userID, modelID string) (*AIModelConfig, error) {
	if modelID == "" {
		return nil, fmt.Errorf("模型ID不能为空")
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
			// 解析时间字符串
			model.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
			model.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
			// 解密API Key（与 GetAIModels 行为保持一致）
			model.APIKey = d.decryptSensitiveData(model.APIKey)
			return &model, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	return nil, sql.ErrNoRows
}

// GetDefaultAIModel 获取指定用户（或默认用户）的首个启用的AI模型。
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
	return nil, fmt.Errorf("请先在系统中配置可用的AI模型")
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
	// 解析时间字符串
	model.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	model.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	// 解密API Key，避免上层拿到加密串导致下游认证失败
	model.APIKey = d.decryptSensitiveData(model.APIKey)
	return &model, nil
}

// UpdateAIModel 更新AI模型配置，如果不存在则创建用户特定配置
func (d *Database) UpdateAIModel(userID, id string, enabled bool, apiKey, customAPIURL, customModelName string) error {
	// 先尝试精确匹配 ID（新版逻辑，支持多个相同 provider 的模型）
	var existingID string
	err := d.db.QueryRow(`
		SELECT id FROM ai_models WHERE user_id = ? AND id = ? LIMIT 1
	`, userID, id).Scan(&existingID)

	if err == nil {
		// 找到了现有配置（精确匹配 ID），更新它
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
		_, err = d.db.Exec(`
			UPDATE ai_models SET enabled = ?, api_key = ?, custom_api_url = ?, custom_model_name = ?, updated_at = datetime('now')
			WHERE id = ? AND user_id = ?
		`, enabled, encryptedAPIKey, customAPIURL, customModelName, existingID, userID)
		return err
	}

	// ID 不存在，尝试兼容旧逻辑：将 id 作为 provider 查找
	provider := id
	err = d.db.QueryRow(`
		SELECT id FROM ai_models WHERE user_id = ? AND provider = ? LIMIT 1
	`, userID, provider).Scan(&existingID)

	if err == nil {
		// 找到了现有配置（通过 provider 匹配，兼容旧版），更新它
		log.Printf("⚠️  使用旧版 provider 匹配更新模型: %s -> %s", provider, existingID)
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
		_, err = d.db.Exec(`
			UPDATE ai_models SET enabled = ?, api_key = ?, custom_api_url = ?, custom_model_name = ?, updated_at = datetime('now')
			WHERE id = ? AND user_id = ?
		`, enabled, encryptedAPIKey, customAPIURL, customModelName, existingID, userID)
		return err
	}

	// 没有找到任何现有配置，创建新的
	// 推断 provider（从 id 中提取，或者直接使用 id）
	if provider == id && (provider == "deepseek" || provider == "qwen") {
		// id 本身就是 provider
		provider = id
	} else {
		// 从 id 中提取 provider（假设格式是 userID_provider 或 timestamp_userID_provider）
		parts := strings.Split(id, "_")
		if len(parts) >= 2 {
			provider = parts[len(parts)-1] // 取最后一部分作为 provider
		} else {
			provider = id
		}
	}

	// 获取模型的基本信息
	var name string
	err = d.db.QueryRow(`
		SELECT name FROM ai_models WHERE provider = ? LIMIT 1
	`, provider).Scan(&name)
	if err != nil {
		// 如果找不到基本信息，使用默认值
		switch provider {
		case "deepseek":
			name = "DeepSeek AI"
		case "qwen":
			name = "Qwen AI"
		default:
			name = provider + " AI"
		}
	}

	// 如果传入的 ID 已经是完整格式（如 "admin_deepseek_custom1"），直接使用
	// 否则生成新的 ID
	newModelID := id
	if id == provider {
		// id 就是 provider，生成新的用户特定 ID
		newModelID = fmt.Sprintf("%s_%s", userID, provider)
	}

	log.Printf("✓ 创建新的 AI 模型配置: ID=%s, Provider=%s, Name=%s", newModelID, provider, name)
	encryptedAPIKey := d.encryptSensitiveData(apiKey)
	_, err = d.db.Exec(`
		INSERT INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, newModelID, userID, name, provider, enabled, encryptedAPIKey, customAPIURL, customModelName)

	return err
}

// GetExchanges 获取用户的交易所配置
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

	// 初始化为空切片而不是nil，确保JSON序列化为[]而不是null
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

		// 解析时间字符串
		exchange.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		exchange.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

		// 解密敏感字段
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

// UpdateExchange 更新交易所配置，如果不存在则创建用户特定配置
// 🔒 安全特性：空值不会覆盖现有的敏感字段（api_key, secret_key, aster_private_key, lighter_private_key, lighter_api_key_private_key, okx_passphrase）
func (d *Database) UpdateExchange(userID, id string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey, lighterWalletAddr, lighterPrivateKey, lighterAPIKeyPrivateKey, okxPassphrase string) error {
	log.Printf("🔧 UpdateExchange: userID=%s, id=%s, enabled=%v", userID, id, enabled)

	// 构建动态 UPDATE SET 子句
	// 基础字段：总是更新
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

	// 🔒 敏感字段：只在非空时更新（保护现有数据）
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

	// WHERE 条件
	args = append(args, id, userID)

	// 构建完整的 UPDATE 语句
	query := fmt.Sprintf(`
		UPDATE exchanges SET %s
		WHERE id = ? AND user_id = ?
	`, strings.Join(setClauses, ", "))

	// 执行更新
	result, err := d.db.Exec(query, args...)
	if err != nil {
		log.Printf("❌ UpdateExchange: 更新失败: %v", err)
		return err
	}

	// 检查是否有行被更新
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("❌ UpdateExchange: 获取影响行数失败: %v", err)
		return err
	}

	log.Printf("📊 UpdateExchange: 影响行数 = %d", rowsAffected)

	// 如果没有行被更新，说明用户没有这个交易所的配置，需要创建
	if rowsAffected == 0 {
		log.Printf("💡 UpdateExchange: 没有现有记录，创建新记录")

		// 根据交易所ID确定基本信息
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

		log.Printf("🆕 UpdateExchange: 创建新记录 ID=%s, name=%s, type=%s", id, name, typ)

		// 加密敏感字段
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
		encryptedSecretKey := d.encryptSensitiveData(secretKey)
		encryptedAsterPrivateKey := d.encryptSensitiveData(asterPrivateKey)
		encryptedLighterPrivateKey := d.encryptSensitiveData(lighterPrivateKey)
		encryptedLighterAPIKeyPrivateKey := d.encryptSensitiveData(lighterAPIKeyPrivateKey)
		encryptedOkxPassphrase := d.encryptSensitiveData(okxPassphrase)

		// 创建用户特定的配置，使用原始的交易所ID
		_, err = d.db.Exec(`
			INSERT INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet,
			                       hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key,
			                       lighter_wallet_addr, lighter_private_key, lighter_api_key_private_key, okx_passphrase, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		`, id, userID, name, typ, enabled, encryptedAPIKey, encryptedSecretKey, testnet, hyperliquidWalletAddr, asterUser, asterSigner, encryptedAsterPrivateKey, lighterWalletAddr, encryptedLighterPrivateKey, encryptedLighterAPIKeyPrivateKey, encryptedOkxPassphrase)

		if err != nil {
			log.Printf("❌ UpdateExchange: 创建记录失败: %v", err)
		} else {
			log.Printf("✅ UpdateExchange: 创建记录成功")
		}
		return err
	}

	log.Printf("✅ UpdateExchange: 更新现有记录成功")
	return nil
}

// CreateAIModel 创建AI模型配置
func (d *Database) CreateAIModel(userID, id, name, provider string, enabled bool, apiKey, customAPIURL string) error {
	_, err := d.db.Exec(`
		INSERT OR IGNORE INTO ai_models (id, user_id, name, provider, enabled, api_key, custom_api_url) 
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, userID, name, provider, enabled, apiKey, customAPIURL)
	return err
}

// CreateExchange 创建交易所配置
func (d *Database) CreateExchange(userID, id, name, typ string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) error {
	// 加密敏感字段
	encryptedAPIKey := d.encryptSensitiveData(apiKey)
	encryptedSecretKey := d.encryptSensitiveData(secretKey)
	encryptedAsterPrivateKey := d.encryptSensitiveData(asterPrivateKey)

	_, err := d.db.Exec(`
		INSERT OR IGNORE INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet, hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, lighter_wallet_addr, lighter_private_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', '')
	`, id, userID, name, typ, enabled, encryptedAPIKey, encryptedSecretKey, testnet, hyperliquidWalletAddr, asterUser, asterSigner, encryptedAsterPrivateKey)
	return err
}

// GetExchangeByID 根据交易所ID和用户ID获取单个交易所配置
func (d *Database) GetExchangeByID(userID, exchangeID string) (*ExchangeConfig, error) {
	if exchangeID == "" {
		return nil, fmt.Errorf("交易所ID不能为空")
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

	// 解析时间字符串
	exchange.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	exchange.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	// 解密敏感字段
	exchange.APIKey = d.decryptSensitiveData(exchange.APIKey)
	exchange.SecretKey = d.decryptSensitiveData(exchange.SecretKey)
	exchange.AsterPrivateKey = d.decryptSensitiveData(exchange.AsterPrivateKey)
	exchange.LighterPrivateKey = d.decryptSensitiveData(exchange.LighterPrivateKey)
	exchange.LighterAPIKeyPrivateKey = d.decryptSensitiveData(exchange.LighterAPIKeyPrivateKey)
	exchange.OkxPassphrase = d.decryptSensitiveData(exchange.OkxPassphrase)

	return &exchange, nil
}

// CopyAIModelToUser 将AI模型配置从源用户复制到目标用户（不复制API密钥）
func (d *Database) CopyAIModelToUser(sourceUserID, targetUserID, modelID string) error {
	// 获取源用户的AI模型配置
	sourceModel, err := d.GetAIModel(sourceUserID, modelID)
	if err != nil {
		return fmt.Errorf("获取源AI模型配置失败: %w", err)
	}

	// 检查目标用户是否已有该模型
	_, err = d.GetAIModel(targetUserID, modelID)
	if err == nil {
		// 模型已存在，不需要复制
		log.Printf("✓ AI模型 %s 已存在于目标用户 %s", modelID, targetUserID)
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("检查目标AI模型失败: %w", err)
	}

	// 创建模型配置（不复制API密钥，使用空字符串）
	err = d.CreateAIModel(
		targetUserID,
		sourceModel.ID,
		sourceModel.Name,
		sourceModel.Provider,
		sourceModel.Enabled,
		"", // 不复制API密钥
		sourceModel.CustomAPIURL,
	)
	if err != nil {
		return fmt.Errorf("创建AI模型配置失败: %w", err)
	}

	log.Printf("✓ 已为跟随者用户 %s 创建AI模型配置: %s (%s)", targetUserID, sourceModel.Name, sourceModel.Provider)
	return nil
}

// CopyExchangeToUser 将交易所配置从源用户复制到目标用户（不复制API密钥和密钥）
func (d *Database) CopyExchangeToUser(sourceUserID, targetUserID, exchangeID string) error {
	// 获取源用户的交易所配置
	sourceExchange, err := d.GetExchangeByID(sourceUserID, exchangeID)
	if err != nil {
		return fmt.Errorf("获取源交易所配置失败: %w", err)
	}

	// 检查目标用户是否已有该交易所
	_, err = d.GetExchangeByID(targetUserID, exchangeID)
	if err == nil {
		// 交易所已存在，不需要复制
		log.Printf("✓ 交易所 %s 已存在于目标用户 %s", exchangeID, targetUserID)
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("检查目标交易所失败: %w", err)
	}

	// 创建交易所配置（不复制API密钥和密钥，使用空字符串）
	err = d.CreateExchange(
		targetUserID,
		sourceExchange.ID,
		sourceExchange.Name,
		sourceExchange.Type,
		sourceExchange.Enabled,
		"", // 不复制API密钥
		"", // 不复制Secret密钥
		sourceExchange.Testnet,
		sourceExchange.HyperliquidWalletAddr,
		sourceExchange.AsterUser,
		sourceExchange.AsterSigner,
		"", // 不复制Aster私钥
	)
	if err != nil {
		return fmt.Errorf("创建交易所配置失败: %w", err)
	}

	log.Printf("✓ 已为跟随者用户 %s 创建交易所配置: %s (%s)", targetUserID, sourceExchange.Name, sourceExchange.Type)
	return nil
}

// GetPromptTemplates 获取用户的提示词模板（包括系统模板）
func (d *Database) GetPromptTemplates(userID string) ([]*PromptTemplateConfig, error) {
	// 获取系统模板（user_id='default'）和用户模板
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
		// 解析时间字符串
		template.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		template.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		templates = append(templates, &template)
	}

	return templates, nil
}

// GetPromptTemplate 获取指定的提示词模板
func (d *Database) GetPromptTemplate(userID, templateID string) (*PromptTemplateConfig, error) {
	// 允许获取系统模板（user_id='default'）或用户自己的模板
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
			return nil, fmt.Errorf("提示词模板不存在: %s", templateID)
		}
		return nil, err
	}

	// 解析时间字符串
	template.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	template.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	return &template, nil
}

// CreatePromptTemplate 创建新的提示词模板
func (d *Database) CreatePromptTemplate(userID, id, name, content string, isSystem bool) error {
	_, err := d.db.Exec(`
		INSERT INTO prompt_templates (id, user_id, name, content, is_system, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, id, userID, name, content, isSystem)
	return err
}

// UpdatePromptTemplate 更新提示词模板（只能更新用户创建的模板，不能更新系统模板）
func (d *Database) UpdatePromptTemplate(userID, id, name, content string) error {
	// 检查模板是否存在且属于该用户，且不是系统模板
	var isSystem bool
	var templateUserID string
	err := d.db.QueryRow(`
		SELECT user_id, is_system FROM prompt_templates WHERE id = ?
	`, id).Scan(&templateUserID, &isSystem)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("提示词模板不存在: %s", id)
		}
		return err
	}

	// 不允许更新系统模板
	if isSystem {
		return fmt.Errorf("不能更新系统模板: %s", id)
	}

	// 只能更新自己的模板
	if templateUserID != userID {
		return fmt.Errorf("无权更新此模板: %s", id)
	}

	// 更新模板
	_, err = d.db.Exec(`
		UPDATE prompt_templates
		SET name = ?, content = ?, updated_at = datetime('now')
		WHERE id = ? AND user_id = ? AND is_system = 0
	`, name, content, id, userID)
	return err
}

// DeletePromptTemplate 删除提示词模板（只能删除用户创建的模板，不能删除系统模板）
func (d *Database) DeletePromptTemplate(userID, id string) error {
	// 检查模板是否存在且属于该用户，且不是系统模板
	var isSystem bool
	var templateUserID string
	err := d.db.QueryRow(`
		SELECT user_id, is_system FROM prompt_templates WHERE id = ?
	`, id).Scan(&templateUserID, &isSystem)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("提示词模板不存在: %s", id)
		}
		return err
	}

	// 不允许删除系统模板
	if isSystem {
		return fmt.Errorf("不能删除系统模板: %s", id)
	}

	// 只能删除自己的模板
	if templateUserID != userID {
		return fmt.Errorf("无权删除此模板: %s", id)
	}

	// 删除模板
	_, err = d.db.Exec(`DELETE FROM prompt_templates WHERE id = ? AND user_id = ? AND is_system = 0`, id, userID)
	return err
}

// CreateTrader 创建交易员
func (d *Database) CreateTrader(trader *TraderRecord) error {
	_, err := d.db.Exec(`
		INSERT INTO traders (id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running, btc_eth_leverage, altcoin_leverage, trading_symbols, use_coin_pool, use_oi_top, use_tradingview, followed_trader_id, custom_prompt, override_base_prompt, system_prompt_template, is_cross_margin)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, trader.ID, trader.UserID, trader.Name, trader.AIModelID, trader.ExchangeID, trader.InitialBalance, trader.ScanIntervalMinutes, trader.IsRunning, trader.BTCETHLeverage, trader.AltcoinLeverage, trader.TradingSymbols, trader.UseCoinPool, trader.UseOITop, trader.UseTradingView, trader.FollowedTraderID, trader.CustomPrompt, trader.OverrideBasePrompt, trader.SystemPromptTemplate, trader.IsCrossMargin)
	return err
}

// GetTraders 获取用户的交易员
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
		// 解析时间字符串
		trader.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		trader.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		traders = append(traders, &trader)
	}

	return traders, nil
}

// GetFollowerTraders 获取所有跟随指定交易员的交易员列表
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
		// 解析时间字符串
		trader.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		trader.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		traders = append(traders, &trader)
	}

	return traders, nil
}

// GetTraderFollowedTraderID 获取交易员的followed_trader_id
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

// UpdateTraderStatus 更新交易员状态
func (d *Database) UpdateTraderStatus(userID, id string, isRunning bool) error {
	_, err := d.db.Exec(`UPDATE traders SET is_running = ? WHERE id = ? AND user_id = ?`, isRunning, id, userID)
	return err
}

// UpdateTrader 更新交易员配置
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

// UpdateTraderCustomPrompt 更新交易员自定义Prompt
func (d *Database) UpdateTraderCustomPrompt(userID, id string, customPrompt string, overrideBase bool) error {
	_, err := d.db.Exec(`UPDATE traders SET custom_prompt = ?, override_base_prompt = ? WHERE id = ? AND user_id = ?`, customPrompt, overrideBase, id, userID)
	return err
}

// UpdateTraderInitialBalance 更新交易员初始余额（仅支持手动更新）
// ⚠️ 注意：系统不会自动调用此方法，仅供用户在充值/提现后手动同步使用
func (d *Database) UpdateTraderInitialBalance(userID, id string, newBalance float64) error {
	_, err := d.db.Exec(`UPDATE traders SET initial_balance = ? WHERE id = ? AND user_id = ?`, newBalance, id, userID)
	return err
}

// DeleteTrader 删除交易员
func (d *Database) DeleteTrader(userID, id string) error {
	_, err := d.db.Exec(`DELETE FROM traders WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// GetTraderConfig 获取交易员完整配置（包含AI模型和交易所信息）
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

	// 解析时间字符串
	trader.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", traderCreatedAt)
	trader.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", traderUpdatedAt)
	aiModel.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", aiModelCreatedAt)
	aiModel.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", aiModelUpdatedAt)
	exchange.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", exchangeCreatedAt)
	exchange.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", exchangeUpdatedAt)

	// 解密敏感数据
	aiModel.APIKey = d.decryptSensitiveData(aiModel.APIKey)
	exchange.APIKey = d.decryptSensitiveData(exchange.APIKey)
	exchange.SecretKey = d.decryptSensitiveData(exchange.SecretKey)
	exchange.AsterPrivateKey = d.decryptSensitiveData(exchange.AsterPrivateKey)
	exchange.LighterPrivateKey = d.decryptSensitiveData(exchange.LighterPrivateKey)
	exchange.LighterAPIKeyPrivateKey = d.decryptSensitiveData(exchange.LighterAPIKeyPrivateKey)
	exchange.OkxPassphrase = d.decryptSensitiveData(exchange.OkxPassphrase)

	return &trader, &aiModel, &exchange, nil
}

// GetSystemConfig 获取系统配置
func (d *Database) GetSystemConfig(key string) (string, error) {
	var value string
	err := d.db.QueryRow(`SELECT value FROM system_config WHERE key = ?`, key).Scan(&value)
	return value, err
}

// SetSystemConfig 设置系统配置
func (d *Database) SetSystemConfig(key, value string) error {
	_, err := d.db.Exec(`
		INSERT OR REPLACE INTO system_config (key, value) VALUES (?, ?)
	`, key, value)
	return err
}

// CreateUserSignalSource 创建用户信号源配置
func (d *Database) CreateUserSignalSource(userID, coinPoolURL, oiTopURL string) error {
	_, err := d.db.Exec(`
		INSERT OR REPLACE INTO user_signal_sources (user_id, coin_pool_url, oi_top_url, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, userID, coinPoolURL, oiTopURL)
	return err
}

// GetUserSignalSource 获取用户信号源配置
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

// UpdateUserSignalSource 更新用户信号源配置
func (d *Database) UpdateUserSignalSource(userID, coinPoolURL, oiTopURL string) error {
	_, err := d.db.Exec(`
		UPDATE user_signal_sources SET coin_pool_url = ?, oi_top_url = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ?
	`, coinPoolURL, oiTopURL, userID)
	return err
}

// GenerateWebhookAPIKey 生成或获取用户的Webhook API Key
func (d *Database) GenerateWebhookAPIKey(userID string) (string, error) {
	// 先检查是否已存在
	var existingKey string
	err := d.db.QueryRow(`SELECT api_key FROM webhook_api_keys WHERE user_id = ?`, userID).Scan(&existingKey)
	if err == nil {
		return existingKey, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("查询API key失败: %w", err)
	}

	// 生成64字符的hex API key
	bytes := make([]byte, 32) // 32 bytes = 64 hex characters
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("生成随机数失败: %w", err)
	}
	apiKey := fmt.Sprintf("%x", bytes)

	// 插入数据库
	_, err = d.db.Exec(`
		INSERT INTO webhook_api_keys (user_id, api_key, created_at, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, userID, apiKey)
	if err != nil {
		return "", fmt.Errorf("保存API key失败: %w", err)
	}

	return apiKey, nil
}

// GetUserByWebhookAPIKey 通过API key获取用户
func (d *Database) GetUserByWebhookAPIKey(apiKey string) (*User, error) {
	var userID string
	err := d.db.QueryRow(`SELECT user_id FROM webhook_api_keys WHERE api_key = ?`, apiKey).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("无效的API key: %w", err)
	}

	return d.GetUserByID(userID)
}

// CreateTradingViewAlert 创建TradingView警报，返回警报ID
func (d *Database) CreateTradingViewAlert(userID, traderID string, payload map[string]interface{}) (string, error) {
	// 生成UUID
	id := fmt.Sprintf("%d", time.Now().UnixNano())

	// 解析payload字段
	rawPayloadBytes, _ := json.Marshal(payload)
	rawPayload := string(rawPayloadBytes)

	symbol, _ := payload["symbol"].(string)
	action, _ := payload["action"].(string)
	exchange, _ := payload["exchange"].(string)
	pricetype, _ := payload["pricetype"].(string)

	// 解析数值字段
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

// parseFloat 辅助函数：从interface{}解析float64
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

// GetPendingTradingViewAlerts 获取指定交易员的待处理警报
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

// UpdateAlertStatus 更新警报状态
func (d *Database) UpdateAlertStatus(alertID string, status string) error {
	now := time.Now()
	_, err := d.db.Exec(`
		UPDATE tradingview_alerts
		SET status = ?, processed_at = ?
		WHERE id = ?
	`, status, now, alertID)
	return err
}

// GetTradingViewAlertByID 根据ID获取TradingView警报
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

// GetRecentTradingViewAlerts 获取最近的TradingView警报
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

// GetTradersWithTradingViewEnabled 获取用户启用了TradingView的交易员列表
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

// GetCustomCoins 获取所有交易员自定义币种 / Get all trader-customized currencies
func (d *Database) GetCustomCoins() []string {
	var symbol string
	var symbols []string
	_ = d.db.QueryRow(`
		SELECT GROUP_CONCAT(custom_coins , ',') as symbol
		FROM main.traders where custom_coins != ''
	`).Scan(&symbol)
	// 检测用户是否未配置币种 - 兼容性
	if symbol == "" {
		symbolJSON, _ := d.GetSystemConfig("default_coins")
		if err := json.Unmarshal([]byte(symbolJSON), &symbols); err != nil {
			log.Printf("⚠️  解析default_coins配置失败: %v，使用硬编码默认值", err)
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

// Close 关闭数据库连接
// Conn 返回底层 *sql.DB，供需要执行自定义查询的模块使用。
func (d *Database) Conn() *sql.DB {
	return d.db
}

func (d *Database) Close() error {
	return d.db.Close()
}

// LoadBetaCodesFromFile 从文件加载内测码到数据库
func (d *Database) LoadBetaCodesFromFile(filePath string) error {
	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取内测码文件失败: %w", err)
	}

	// 按行分割内测码
	lines := strings.Split(string(content), "\n")
	var codes []string
	for _, line := range lines {
		code := strings.TrimSpace(line)
		if code != "" && !strings.HasPrefix(code, "#") {
			codes = append(codes, code)
		}
	}

	// 批量插入内测码
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO beta_codes (code) VALUES (?)`)
	if err != nil {
		return fmt.Errorf("准备语句失败: %w", err)
	}
	defer stmt.Close()

	insertedCount := 0
	for _, code := range codes {
		result, err := stmt.Exec(code)
		if err != nil {
			log.Printf("插入内测码 %s 失败: %v", code, err)
			continue
		}

		if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
			insertedCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	log.Printf("✅ 成功加载 %d 个内测码到数据库 (总计 %d 个)", insertedCount, len(codes))
	return nil
}

// ValidateBetaCode 验证内测码是否有效且未使用
func (d *Database) ValidateBetaCode(code string) (bool, error) {
	var used bool
	err := d.db.QueryRow(`SELECT used FROM beta_codes WHERE code = ?`, code).Scan(&used)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // 内测码不存在
		}
		return false, err
	}
	return !used, nil // 内测码存在且未使用
}

// UseBetaCode 使用内测码（标记为已使用）
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
		return fmt.Errorf("内测码无效或已被使用")
	}

	return nil
}

// GetBetaCodeStats 获取内测码统计信息
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

// SetCryptoService 设置加密服务
func (d *Database) SetCryptoService(cs *crypto.CryptoService) {
	d.cryptoService = cs
}

// encryptSensitiveData 加密敏感数据用于存储
func (d *Database) encryptSensitiveData(plaintext string) string {
	if d.cryptoService == nil || plaintext == "" {
		return plaintext
	}

	encrypted, err := d.cryptoService.EncryptForStorage(plaintext)
	if err != nil {
		log.Printf("⚠️ 加密失败: %v", err)
		return plaintext // 返回明文作为降级处理
	}

	return encrypted
}

// decryptSensitiveData 解密敏感数据
func (d *Database) decryptSensitiveData(encrypted string) string {
	if d.cryptoService == nil || encrypted == "" {
		return encrypted
	}

	// 如果不是加密格式，直接返回
	if !d.cryptoService.IsEncryptedStorageValue(encrypted) {
		return encrypted
	}

	decrypted, err := d.cryptoService.DecryptFromStorage(encrypted)
	if err != nil {
		log.Printf("⚠️ 解密失败: %v", err)
		return encrypted // 返回加密文本作为降级处理
	}

	return decrypted
}
