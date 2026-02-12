package main

import (
	"encoding/json"
	"fmt"
	"log"
	"nofx/api"
	"nofx/auth"
	"nofx/backtest"
	"nofx/config"
	"nofx/crypto"
	"nofx/decision"
	"nofx/logger"
	"nofx/manager"
	"nofx/market"
	"nofx/mcp"
	"nofx/pool"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

// ConfigFile configuration file structure, only contains fields that need to be synced to database
// TODO Currently same as config.Config, will be replaced in the future, kept for compatibility
type ConfigFile struct {
	BetaMode           bool                  `json:"beta_mode"`
	APIServerPort      int                   `json:"api_server_port"`
	UseDefaultCoins    bool                  `json:"use_default_coins"`
	DefaultCoins       []string              `json:"default_coins"`
	CoinPoolAPIURL     string                `json:"coin_pool_api_url"`
	OITopAPIURL        string                `json:"oi_top_api_url"`
	MaxDailyLoss       float64               `json:"max_daily_loss"`
	MaxDrawdown        float64               `json:"max_drawdown"`
	StopTradingMinutes int                   `json:"stop_trading_minutes"`
	Leverage           config.LeverageConfig `json:"leverage"`
	JWTSecret          string                `json:"jwt_secret"`
	DataKLineTime      string                `json:"data_k_line_time"`
	Log                *config.LogConfig     `json:"log"` // Log configuration
}

// loadConfigFile reads and parses config.json file
func loadConfigFile() (*ConfigFile, error) {
	// Check if config.json exists
	if _, err := os.Stat("config.json"); os.IsNotExist(err) {
		log.Printf("📄 config.json does not exist, using default configuration")
		return &ConfigFile{}, nil
	}

	// Read config.json
	data, err := os.ReadFile("config.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read config.json: %w", err)
	}

	// Parse JSON
	var configFile ConfigFile
	if err := json.Unmarshal(data, &configFile); err != nil {
		return nil, fmt.Errorf("failed to parse config.json: %w", err)
	}

	return &configFile, nil
}

// syncConfigToDatabase syncs configuration to database
func syncConfigToDatabase(database *config.Database, configFile *ConfigFile) error {
	if configFile == nil {
		return nil
	}

	log.Printf("🔄 Starting to sync config.json to database...")

	// Sync each configuration item to database
	configs := map[string]string{
		"beta_mode":            fmt.Sprintf("%t", configFile.BetaMode),
		"api_server_port":      strconv.Itoa(configFile.APIServerPort),
		"use_default_coins":    fmt.Sprintf("%t", configFile.UseDefaultCoins),
		"coin_pool_api_url":    configFile.CoinPoolAPIURL,
		"oi_top_api_url":       configFile.OITopAPIURL,
		"max_daily_loss":       fmt.Sprintf("%.1f", configFile.MaxDailyLoss),
		"max_drawdown":         fmt.Sprintf("%.1f", configFile.MaxDrawdown),
		"stop_trading_minutes": strconv.Itoa(configFile.StopTradingMinutes),
	}

	// Sync default_coins (convert to JSON string for storage)
	if len(configFile.DefaultCoins) > 0 {
		defaultCoinsJSON, err := json.Marshal(configFile.DefaultCoins)
		if err == nil {
			configs["default_coins"] = string(defaultCoinsJSON)
		}
	}

	// Sync leverage configuration
	if configFile.Leverage.BTCETHLeverage > 0 {
		configs["btc_eth_leverage"] = strconv.Itoa(configFile.Leverage.BTCETHLeverage)
	}
	if configFile.Leverage.AltcoinLeverage > 0 {
		configs["altcoin_leverage"] = strconv.Itoa(configFile.Leverage.AltcoinLeverage)
	}

	// If JWT secret is not empty, also sync
	if configFile.JWTSecret != "" {
		configs["jwt_secret"] = configFile.JWTSecret
	}

	// Update database configuration
	for key, value := range configs {
		if err := database.SetSystemConfig(key, value); err != nil {
			log.Printf("⚠️  Failed to update configuration %s: %v", key, err)
		} else {
			log.Printf("✓ Synced configuration: %s = %s", key, value)
		}
	}

	log.Printf("✅ config.json sync completed")
	return nil
}

// loadBetaCodesToDatabase loads beta code file to database
func loadBetaCodesToDatabase(database *config.Database) error {
	betaCodeFile := "beta_codes.txt"

	// Check if beta code file exists
	if _, err := os.Stat(betaCodeFile); os.IsNotExist(err) {
		log.Printf("📄 Beta code file %s does not exist, skipping load", betaCodeFile)
		return nil
	}

	// Get file information
	fileInfo, err := os.Stat(betaCodeFile)
	if err != nil {
		return fmt.Errorf("failed to get beta code file information: %w", err)
	}

	log.Printf("🔄 Found beta code file %s (%.1f KB), starting to load...", betaCodeFile, float64(fileInfo.Size())/1024)

	// Load beta codes to database
	err = database.LoadBetaCodesFromFile(betaCodeFile)
	if err != nil {
		return fmt.Errorf("failed to load beta codes: %w", err)
	}

	// Display statistics
	total, used, err := database.GetBetaCodeStats()
	if err != nil {
		log.Printf("⚠️  Failed to get beta code statistics: %v", err)
	} else {
		log.Printf("✅ Beta code loading completed: total %d, used %d, remaining %d", total, used, total-used)
	}

	return nil
}

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║    🤖 AI Multi-Model Trading System - DeepSeek & Qwen   ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Load environment variables from .env file if present (for local/dev runs)
	// In Docker Compose, variables are injected by the runtime and this is harmless.
	_ = godotenv.Load()

	// Initialize database configuration
	// Default path is data/data.db to ensure database persistence in Docker volume
	// In Docker/Render, WORKDIR is /app and /app/data is typically mounted as a persistent disk.
	dbPath := "data/data.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	// Allow overriding DB path via environment variable (higher priority than CLI args)
	if envDBPath := strings.TrimSpace(os.Getenv("DB_PATH")); envDBPath != "" {
		dbPath = envDBPath
	}

	// Log absolute path for easier debugging in container environments (Render, Docker, etc.)
	absDBPath, absErr := filepath.Abs(dbPath)
	if absErr != nil {
		log.Printf("⚠️  Could not resolve absolute DB path from %q: %v", dbPath, absErr)
	} else {
		log.Printf("📁 Resolved database path: %s (from %s)", absDBPath, dbPath)
	}

	// Read configuration file
	configFile, err := loadConfigFile()
	if err != nil {
		log.Fatalf("❌ Failed to read config.json: %v", err)
	}

	// Initialize logger early
	if err := logger.InitFromLogConfig(configFile.Log); err != nil {
		log.Printf("⚠️  Failed to initialize logger: %v, using default logger", err)
	}

	log.Printf("📋 Initializing configuration database: %s", dbPath)
	
	// Verify database file exists and is accessible before initialization
	if _, err := os.Stat(dbPath); err == nil {
		log.Printf("✅ Database file exists: %s", dbPath)
	} else if os.IsNotExist(err) {
		log.Printf("ℹ️  Database file does not exist, will be created: %s", dbPath)
	} else {
		log.Printf("⚠️  Could not check database file: %v", err)
	}
	
	database, err := config.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("❌ Failed to initialize database: %v", err)
	}
	defer database.Close()
	backtest.UseDatabase(database.Conn())
	
	// Verify database health after initialization
	if err := database.VerifyDatabaseHealth(); err != nil {
		log.Printf("⚠️  Database health check failed: %v", err)
	} else {
		log.Printf("✅ Database health check passed")
	}

	// Initialize encryption service
	log.Printf("🔐 Initializing encryption service...")

	// Ensure encryption keys are on persistent disk:
	// If legacy secrets exist in ./secrets but not yet in ./data/secrets, migrate them once.
	legacySecretsDir := "secrets"
	persistentSecretsDir := "data/secrets"
	if stat, err := os.Stat(legacySecretsDir); err == nil && stat.IsDir() {
		if err := os.MkdirAll(persistentSecretsDir, 0700); err != nil {
			log.Printf("⚠️  Failed to create persistent secrets directory %s: %v", persistentSecretsDir, err)
		} else {
			type filePair struct {
				src string
				dst string
			}
			pairs := []filePair{
				{src: "secrets/rsa_key", dst: "data/secrets/rsa_key"},
				{src: "secrets/rsa_key.pub", dst: "data/secrets/rsa_key.pub"},
				{src: "secrets/data_key", dst: "data/secrets/data_key"},
			}
			for _, p := range pairs {
				if _, err := os.Stat(p.src); err == nil {
					if _, err := os.Stat(p.dst); os.IsNotExist(err) {
						if data, err := os.ReadFile(p.src); err == nil {
							if err := os.WriteFile(p.dst, data, 0600); err != nil {
								log.Printf("⚠️  Failed to migrate %s to %s: %v", p.src, p.dst, err)
							} else {
								log.Printf("🔁 Migrated legacy secret %s -> %s", p.src, p.dst)
							}
						} else {
							log.Printf("⚠️  Failed to read legacy secret %s: %v", p.src, err)
						}
					}
				}
			}
		}
	}

	cryptoService, err := crypto.NewCryptoService("data/secrets/rsa_key")
	if err != nil {
		log.Fatalf("❌ Failed to initialize encryption service: %v", err)
	}
	database.SetCryptoService(cryptoService)
	log.Printf("✅ Encryption service initialized successfully")

	// Sync config.json to database
	if err := syncConfigToDatabase(database, configFile); err != nil {
		log.Printf("⚠️  Failed to sync config.json to database: %v", err)
	}

	// Load beta codes to database
	if err := loadBetaCodesToDatabase(database); err != nil {
		log.Printf("⚠️  Failed to load beta codes to database: %v", err)
	}

	// Get system configuration
	useDefaultCoinsStr, _ := database.GetSystemConfig("use_default_coins")
	useDefaultCoins := useDefaultCoinsStr == "true"
	apiPortStr, _ := database.GetSystemConfig("api_server_port")

	// Set JWT secret (prioritize environment variable)
	jwtSecret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if jwtSecret == "" {
		// Fallback to database configuration
		jwtSecret, _ = database.GetSystemConfig("jwt_secret")
		if jwtSecret == "" {
			jwtSecret = "your-jwt-secret-key-change-in-production-make-it-long-and-random"
			log.Printf("⚠️  Using default JWT secret, recommend using encryption setup script to generate secure key")
		} else {
			log.Printf("🔑 Using JWT secret from database")
		}
	} else {
		log.Printf("🔑 Using JWT secret from environment variable")
	}
	auth.SetJWTSecret(jwtSecret)

	// Admin mode requires admin password, exit if missing

	log.Printf("✓ Configuration database initialized successfully")
	fmt.Println()

	// Read default coin list from database
	defaultCoinsJSON, _ := database.GetSystemConfig("default_coins")
	var defaultCoins []string

	if defaultCoinsJSON != "" {
		// Try to parse from JSON
		if err := json.Unmarshal([]byte(defaultCoinsJSON), &defaultCoins); err != nil {
			log.Printf("⚠️  Failed to parse default_coins configuration: %v, using hardcoded default values", err)
			defaultCoins = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"}
		} else {
			log.Printf("✓ Loaded default coin list from database (%d coins): %v", len(defaultCoins), defaultCoins)
		}
	} else {
		// If not configured in database, use hardcoded default values
		defaultCoins = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"}
		log.Printf("⚠️  default_coins not configured in database, using hardcoded default values")
	}

	pool.SetDefaultCoins(defaultCoins)
	// Set whether to use default coins
	pool.SetUseDefaultCoins(useDefaultCoins)
	if useDefaultCoins {
		log.Printf("✓ Default coin list enabled")
	}

	// Set coin pool API URL
	coinPoolAPIURL, _ := database.GetSystemConfig("coin_pool_api_url")
	if coinPoolAPIURL != "" {
		pool.SetCoinPoolAPI(coinPoolAPIURL)
		log.Printf("✓ AI500 coin pool API configured")
	}

	oiTopAPIURL, _ := database.GetSystemConfig("oi_top_api_url")
	if oiTopAPIURL != "" {
		pool.SetOITopAPI(oiTopAPIURL)
		log.Printf("✓ OI Top API configured")
	}

	// Create TraderManager and BacktestManager
	cfgForAI, cfgErr := config.LoadConfig("config.json")
	if cfgErr != nil {
		log.Printf("⚠️  Failed to load config.json for AI client: %v", cfgErr)
	}

	// Initialize kline count from config (default: 10)
	if cfgForAI != nil && cfgForAI.KlineCount > 0 {
		market.SetKlineCount(cfgForAI.KlineCount)
		log.Printf("✓ Kline count configured: %d", cfgForAI.KlineCount)
	} else {
		log.Printf("✓ Using default kline count: 10")
	}

	traderManager := manager.NewTraderManager()
	mcpClient := newSharedMCPClient(cfgForAI)
	backtestManager := backtest.NewManager(mcpClient)
	if err := backtestManager.RestoreRuns(); err != nil {
		log.Printf("⚠️  Failed to restore backtest history: %v", err)
	}

	// Initialize prompt manager to allow loading templates from database
	decision.GetGlobalPromptManager().SetDatabase(database)

	// Start market data stream BEFORE loading traders to prevent WSMonitor nil pointer
	// Market monitor must be initialized before traders try to access it
	log.Printf("📡 Starting market data monitor...")
	go market.NewWSMonitor(150).Start(database.GetCustomCoins())
	// Give market monitor a moment to initialize
	time.Sleep(500 * time.Millisecond)

	// Load all traders from database to memory
	err = traderManager.LoadTradersFromDatabase(database)
	if err != nil {
		log.Fatalf("❌ Failed to load traders: %v", err)
	}

	// Get all trader configurations from database (for display, using default user)
	traders, err := database.GetTraders("default")
	if err != nil {
		log.Fatalf("❌ Failed to get trader list: %v", err)
	}

	// Verify equity history data exists for active traders
	log.Printf("🔍 Verifying equity history data persistence...")
	tradersWithHistory, err := database.GetAllTradersWithEquityHistory()
	if err != nil {
		log.Printf("⚠️  Failed to check equity history: %v", err)
	} else {
		log.Printf("📊 Found equity history for %d trader(s)", len(tradersWithHistory))
		if len(tradersWithHistory) > 0 {
			for _, traderID := range tradersWithHistory {
				count, err := database.GetEquityHistoryCount(traderID)
				if err == nil {
					log.Printf("  • Trader %s: %d equity history records", traderID, count)
				}
			}
		}
	}
	
	// Check if any active traders are missing equity history
	if len(traders) > 0 {
		missingHistory := []string{}
		for _, trader := range traders {
			count, err := database.GetEquityHistoryCount(trader.ID)
			if err == nil && count == 0 {
				missingHistory = append(missingHistory, trader.ID)
			}
		}
		if len(missingHistory) > 0 {
			log.Printf("⚠️  %d trader(s) have no equity history yet: %v", len(missingHistory), missingHistory)
			log.Printf("   This is normal for newly created traders. History will be created after first decision cycle.")
		}
	}

	// Display loaded trader information
	fmt.Println()
	fmt.Println("🤖 AI Trader Configurations in Database:")
	if len(traders) == 0 {
		fmt.Println("  • No configured traders, please create via Web interface")
	} else {
		for _, trader := range traders {
			status := "Stopped"
			if trader.IsRunning {
				status = "Running"
			}
			fmt.Printf("  • %s (%s + %s) - Initial Balance: %.0f USDT [%s]\n",
				trader.Name, strings.ToUpper(trader.AIModelID), strings.ToUpper(trader.ExchangeID),
				trader.InitialBalance, status)
		}
	}

	// Create initialization context
	// TODO: Pass actual configuration, currently not actually used, all module initialization will pass configuration through context in the future
	// ctx := bootstrap.NewContext(&config.Config{})

	// // Execute all initialization hooks
	// if err := bootstrap.Run(ctx); err != nil {
	// 	log.Fatalf("Initialization failed: %v", err)
	// }

	fmt.Println()
	fmt.Println("🤖 AI Full Decision Mode:")
	fmt.Printf("  • AI will autonomously decide leverage for each trade (altcoins max 5x, BTC/ETH max 5x)\n")
	fmt.Println("  • AI will autonomously decide position size for each trade")
	fmt.Println("  • AI will autonomously set stop loss and take profit prices")
	fmt.Println("  • AI will make comprehensive analysis based on market data, technical indicators, and account status")
	fmt.Println()
	fmt.Println("⚠️  Risk Warning: AI automated trading has risks, recommend testing with small funds!")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	// Get API server port (priority: PORT (Render/PaaS) > NOFX_BACKEND_PORT > database config > default)
	apiPort := 8080 // Default port

	// 1. Highest priority: PORT env var (set by Render.com and other PaaS platforms)
	if envPort := strings.TrimSpace(os.Getenv("PORT")); envPort != "" {
		if port, err := strconv.Atoi(envPort); err == nil && port > 0 {
			apiPort = port
			log.Printf("🔌 Using PORT environment variable: %d (PaaS/Render)", apiPort)
		} else {
			log.Printf("⚠️  PORT environment variable is invalid: %s", envPort)
		}
	} else if envPort := strings.TrimSpace(os.Getenv("NOFX_BACKEND_PORT")); envPort != "" {
		// 2. NOFX_BACKEND_PORT (Docker Compose / manual deployment)
		if port, err := strconv.Atoi(envPort); err == nil && port > 0 {
			apiPort = port
			log.Printf("🔌 Using environment variable port: %d (NOFX_BACKEND_PORT)", apiPort)
		} else {
			log.Printf("⚠️  Environment variable NOFX_BACKEND_PORT is invalid: %s", envPort)
		}
	} else if apiPortStr != "" {
		// 3. Read from database configuration (synced from config.json)
		if port, err := strconv.Atoi(apiPortStr); err == nil && port > 0 {
			apiPort = port
			log.Printf("🔌 Using database configuration port: %d (api_server_port)", apiPort)
		}
	} else {
		log.Printf("🔌 Using default port: %d", apiPort)
	}

	// Create and start API server
	apiServer := api.NewServer(traderManager, database, cryptoService, backtestManager, apiPort)
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Printf("❌ API server error: %v", err)
		}
	}()

	// Setup graceful shutdown
	defer logger.Shutdown()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start traders configured as running in database
	traderManager.StartRunningTraders(database)

	// Wait for shutdown signal
	<-sigChan
	fmt.Println()
	fmt.Println()
	log.Println("📛 Shutdown signal received, gracefully shutting down...")

	// Step 1: Stop all traders
	log.Println("⏸️  Stopping all traders...")
	traderManager.StopAll()
	log.Println("✅ All traders stopped")

	// Step 2: Shutdown API server
	log.Println("🛑 Stopping API server...")
	if err := apiServer.Shutdown(); err != nil {
		log.Printf("⚠️  Error shutting down API server: %v", err)
	} else {
		log.Println("✅ API server safely shut down")
	}

	// Step 3: Close database connection (ensure all writes completed)
	log.Println("💾 Closing database connection...")
	if err := database.Close(); err != nil {
		log.Printf("❌ Failed to close database: %v", err)
	} else {
		log.Println("✅ Database safely closed, all data persisted")
	}

	fmt.Println()
	fmt.Println("👋 Thank you for using AI Trading System!")
}

// newSharedMCPClient creates a shared MCP AI client (for backtesting)
func newSharedMCPClient(_ *config.Config) mcp.AIClient {
	return mcp.NewClient()
}
