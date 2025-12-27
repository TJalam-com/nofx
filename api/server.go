package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"nofx/auth"
	"nofx/backtest"
	"nofx/config"
	"nofx/crypto"
	"nofx/decision"
	"nofx/logger"
	"nofx/manager"
	"nofx/trader"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Server HTTP API server
type Server struct {
	router          *gin.Engine
	traderManager   *manager.TraderManager
	database        *config.Database
	cryptoHandler   *CryptoHandler
	backtestManager *backtest.Manager
	httpServer      *http.Server
	port            int
}

// NewServer Creates API server
func NewServer(traderManager *manager.TraderManager, database *config.Database, cryptoService *crypto.CryptoService, backtestManager *backtest.Manager, port int) *Server {
	// Set to Release mode (reduce log output)
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	// Enable CORS
	router.Use(corsMiddleware())

	// Create crypto handler
	cryptoHandler := NewCryptoHandler(cryptoService)

	s := &Server{
		router:          router,
		traderManager:   traderManager,
		database:        database,
		cryptoHandler:   cryptoHandler,
		backtestManager: backtestManager,
		port:            port,
	}

	// Setup routes
	s.setupRoutes()

	return s
}

// corsMiddleware CORS middleware
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// setupRoutes Setup routes
func (s *Server) setupRoutes() {
	// Serve static files in production (if web/dist exists)
	if _, err := os.Stat("web/dist"); err == nil {
		// Serve static assets from /assets (Vite's output directory)
		s.router.Static("/assets", "./web/dist/assets")

		// Serve icons directory if it exists
		if _, err := os.Stat("web/dist/icons"); err == nil {
			s.router.Static("/icons", "./web/dist/icons")
		}

		// Serve other static files (favicon, etc.)
		s.router.StaticFile("/favicon.ico", "./web/dist/favicon.ico")

		// Serve SEO files (robots.txt, sitemap.xml)
		if _, err := os.Stat("./web/dist/robots.txt"); err == nil {
			s.router.StaticFile("/robots.txt", "./web/dist/robots.txt")
		}
		// Dynamic sitemap generation (takes precedence over static file)
		s.router.GET("/sitemap.xml", s.handleSitemap)

		// Serve index.html for root and other non-API, non-asset routes (SPA routing)
		s.router.NoRoute(func(c *gin.Context) {
			// Don't serve index.html for API routes
			if strings.HasPrefix(c.Request.URL.Path, "/api") {
				c.JSON(http.StatusNotFound, gin.H{"error": "API endpoint not found"})
				return
			}
			// Don't serve index.html for asset/icon requests (they should be handled by Static above)
			if strings.HasPrefix(c.Request.URL.Path, "/assets") || strings.HasPrefix(c.Request.URL.Path, "/icons") {
				c.Status(http.StatusNotFound)
				return
			}
			// Serve index.html for frontend routes
			c.File("./web/dist/index.html")
		})
		log.Println("✅ Static file service enabled: web/dist")
	}

	// API route group
	api := s.router.Group("/api")
	{
		// Health check
		api.Any("/health", s.handleHealth)

		// Admin login (used in admin mode, public)

		// System supported models and exchanges (no authentication required)
		api.GET("/supported-models", s.handleGetSupportedModels)
		api.GET("/supported-exchanges", s.handleGetSupportedExchanges)

		// Strategy default configuration (no authentication required)
		api.GET("/strategies/default-config", s.handleGetDefaultStrategyConfig)

		// Default URLs for data sources (no authentication required)
		api.GET("/config/default-urls", s.handleGetDefaultURLs)

		// System config (no authentication required, for frontend to determine admin mode/registration status)
		api.GET("/config", s.handleGetSystemConfig)

		// Crypto related endpoints (no authentication required)
		api.GET("/crypto/public-key", s.cryptoHandler.HandleGetPublicKey)
		api.GET("/crypto/config", s.cryptoHandler.HandleGetCryptoConfig)
		api.POST("/crypto/decrypt", s.cryptoHandler.HandleDecryptSensitiveData)

		// System prompt template management (no authentication required)
		api.GET("/prompt-templates", s.handleGetPromptTemplates)
		api.GET("/prompt-templates/:name", s.handleGetPromptTemplate)

		// Public competition data (no authentication required)
		api.GET("/traders", s.handlePublicTraderList)
		api.GET("/competition", s.handlePublicCompetition)
		api.GET("/top-traders", s.handleTopTraders)
		api.GET("/equity-history", s.handleEquityHistory)
		api.POST("/equity-history-batch", s.handleEquityHistoryBatch)
		api.GET("/traders/:id/public-config", s.handleGetPublicTraderConfig)

		// Public article routes (no authentication required)
		api.GET("/articles", s.handleGetPublishedArticles)
		api.GET("/articles/:slug", s.handleGetArticleBySlug)

		// Authentication related routes (no authentication required)
		api.POST("/register", s.handleRegister)
		api.POST("/login", s.handleLogin)
		api.POST("/verify-otp", s.handleVerifyOTP)
		api.POST("/complete-registration", s.handleCompleteRegistration)
		api.POST("/reset-password", s.handleResetPassword)

		// Webhook routes (no authentication required, verified via apikey)
		api.POST("/webhook/tradingview", s.handleTradingViewWebhook)
		api.GET("/webhook/tradingview", s.handleTradingViewWebhookGET)
		log.Println("✅ Webhook routes registered: POST /api/webhook/tradingview, GET /api/webhook/tradingview")

		// Routes requiring authentication
		protected := api.Group("/", s.authMiddleware())
		{
			// Logout (add to blacklist)
			protected.POST("/logout", s.handleLogout)

			// Server IP query (requires authentication, for whitelist configuration)
			protected.GET("/server-ip", s.handleGetServerIP)

			// AI trader management
			protected.GET("/my-traders", s.handleTraderList)
			protected.GET("/running-traders", s.handleRunningTraders)
			protected.GET("/traders/:id/config", s.handleGetTraderConfig)
			protected.POST("/traders", s.handleCreateTrader)
			protected.PUT("/traders/:id", s.handleUpdateTrader)
			protected.DELETE("/traders/:id", s.handleDeleteTrader)
			protected.POST("/traders/:id/start", s.handleStartTrader)
			protected.POST("/traders/:id/stop", s.handleStopTrader)
			protected.PUT("/traders/:id/prompt", s.handleUpdateTraderPrompt)
			protected.POST("/traders/:id/sync-balance", s.handleSyncBalance)
			protected.PUT("/traders/:id/competition", s.handleToggleCompetition)

			// AI model configuration
			protected.GET("/models", s.handleGetModelConfigs)
			protected.PUT("/models", s.handleUpdateModelConfigs)

			// Exchange configuration
			protected.GET("/exchanges", s.handleGetExchangeConfigs)
			protected.PUT("/exchanges", s.handleUpdateExchangeConfigs)

			// Webhook management (non-follower users only) - must be registered before other /user routes
			webhookGroup := protected.Group("/user", s.nonFollowerMiddleware())
			webhookGroup.GET("/webhook", s.handleGetWebhookInfo)
			webhookGroup.GET("/tradingview-alerts", s.handleGetRecentAlerts)
			webhookGroup.GET("/followers", s.handleGetUserFollowers)
			log.Println("✅ Webhook management routes registered: GET /api/user/webhook, GET /api/user/tradingview-alerts, GET /api/user/followers (non-follower users only)")

			// Get current user information (for refreshing roles, etc.)
			protected.GET("/user/me", s.handleGetCurrentUser)

			// User signal source configuration
			protected.GET("/user/signal-sources", s.handleGetUserSignalSource)
			protected.POST("/user/signal-sources", s.handleSaveUserSignalSource)

			// Prompt template management (requires authentication) - full CRUD operations
			protected.GET("/user/prompt-templates", s.handleGetUserPromptTemplates)
			protected.GET("/user/prompt-templates/:id", s.handleGetUserPromptTemplate)
			protected.POST("/user/prompt-templates", s.handleCreatePromptTemplate)
			protected.PUT("/user/prompt-templates/:id", s.handleUpdatePromptTemplate)
			protected.DELETE("/user/prompt-templates/:id", s.handleDeletePromptTemplate)

			// Strategy management (requires authentication)
			protected.GET("/strategies", s.handleGetStrategies)
			protected.GET("/strategies/:id", s.handleGetStrategy)
			protected.POST("/strategies", s.handleCreateStrategy)
			protected.PUT("/strategies/:id", s.handleUpdateStrategy)
			protected.DELETE("/strategies/:id", s.handleDeleteStrategy)
			protected.POST("/strategies/import", s.handleImportStrategy)
			protected.GET("/strategies/:id/export", s.handleExportStrategy)

			// Backtest routes (non-follower users only)
			backtestGroup := protected.Group("/backtest", s.nonFollowerMiddleware())
			s.registerBacktestRoutes(backtestGroup)

			// Trader application routes (follower users only)
			protected.GET("/trader-application/my", s.handleGetMyTraderApplication)
			protected.POST("/trader-application", s.followerOnlyMiddleware(), s.handleCreateTraderApplication)

			// Admin routes (admin users only)
			adminGroup := protected.Group("/admin", s.adminOnlyMiddleware())
			{
				adminGroup.GET("/traders", s.handleGetAllTraders)
				adminGroup.GET("/users", s.handleGetAllUsers)
				adminGroup.PUT("/users/:id/role", s.handleUpdateUserRole)
				adminGroup.GET("/trader-applications", s.handleGetAllTraderApplications)
				adminGroup.PUT("/trader-applications/:id/approve", s.handleApproveTraderApplication)
				adminGroup.PUT("/trader-applications/:id/reject", s.handleRejectTraderApplication)
				// Article management
				adminGroup.GET("/articles", s.handleGetArticles)
				adminGroup.GET("/articles/:id", s.handleGetArticle)
				adminGroup.POST("/articles", s.handleCreateArticle)
				adminGroup.PUT("/articles/:id", s.handleUpdateArticle)
				adminGroup.DELETE("/articles/:id", s.handleDeleteArticle)
				adminGroup.POST("/articles/:id/publish", s.handlePublishArticle)
				adminGroup.POST("/articles/:id/unpublish", s.handleUnpublishArticle)
			}
			log.Println("✅ Backtest routes registered: /api/backtest/* (non-follower users only)")

			// Specific trader data (using query parameter ?trader_id=xxx)
			protected.GET("/status", s.handleStatus)
			protected.GET("/account", s.handleAccount)
			protected.GET("/positions", s.handlePositions)
			protected.GET("/position-history", s.handlePositionHistory)
			protected.POST("/positions/close", s.handleClosePosition)
			protected.GET("/decisions", s.handleDecisions)
			protected.GET("/decisions/latest", s.handleLatestDecisions)
			protected.GET("/statistics", s.handleStatistics)
			protected.GET("/performance", s.handlePerformance)

			// Copy trading related endpoints
			protected.GET("/traders/:id/replication-status", s.handleGetReplicationStatus)
			protected.POST("/traders/:id/test-signal", s.handleTestSignal)
		}
	}
}

// handleHealth Health check
func (s *Server) handleHealth(c *gin.Context) {
	health := gin.H{
		"status": "ok",
		"time":   c.Request.Context().Value("time"),
	}
	
	// Add data persistence verification
	if s.database != nil {
		// Verify database health
		if err := s.database.VerifyDatabaseHealth(); err != nil {
			health["database_health"] = "unhealthy"
			health["database_error"] = err.Error()
		} else {
			health["database_health"] = "healthy"
			
			// Check equity history data
			tradersWithHistory, err := s.database.GetAllTradersWithEquityHistory()
			if err == nil {
				health["equity_history_traders"] = len(tradersWithHistory)
				health["equity_history_status"] = "available"
			} else {
				health["equity_history_status"] = "error"
				health["equity_history_error"] = err.Error()
			}
		}
	}
	
	c.JSON(http.StatusOK, health)
}

// handleGetSystemConfig Get system configuration (configuration client needs to know)
func (s *Server) handleGetSystemConfig(c *gin.Context) {
	// Get default coins
	defaultCoinsStr, _ := s.database.GetSystemConfig("default_coins")
	var defaultCoins []string
	if defaultCoinsStr != "" {
		json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins)
	}
	if len(defaultCoins) == 0 {
		// Use hardcoded default coins
		defaultCoins = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"}
	}

	// Get leverage configuration
	btcEthLeverageStr, _ := s.database.GetSystemConfig("btc_eth_leverage")
	altcoinLeverageStr, _ := s.database.GetSystemConfig("altcoin_leverage")

	btcEthLeverage := 5
	if val, err := strconv.Atoi(btcEthLeverageStr); err == nil && val > 0 {
		btcEthLeverage = val
	}

	altcoinLeverage := 5
	if val, err := strconv.Atoi(altcoinLeverageStr); err == nil && val > 0 {
		altcoinLeverage = val
	}

	// Get beta mode configuration
	betaModeStr, _ := s.database.GetSystemConfig("beta_mode")
	betaMode := betaModeStr == "true"

	c.JSON(http.StatusOK, gin.H{
		"beta_mode":        betaMode,
		"default_coins":    defaultCoins,
		"btc_eth_leverage": btcEthLeverage,
		"altcoin_leverage": altcoinLeverage,
	})
}

// handleGetServerIP Get server IP address (for whitelist configuration)
func (s *Server) handleGetServerIP(c *gin.Context) {
	// Try to get public IP through third-party API
	publicIP := getPublicIPFromAPI()

	// If third-party API fails, get first public IP from network interface
	if publicIP == "" {
		publicIP = getPublicIPFromInterface()
	}

	// If still not obtained, return error
	if publicIP == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to get public IP address"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"public_ip": publicIP,
		"message":   "Please add this IP address to the whitelist",
	})
}

// getPublicIPFromAPI Get public IP through third-party API
func getPublicIPFromAPI() string {
	// Try multiple public IP query services
	services := []string{
		"https://api.ipify.org?format=text",
		"https://icanhazip.com",
		"https://ifconfig.me",
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for _, service := range services {
		resp, err := client.Get(service)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body := make([]byte, 128)
			n, err := resp.Body.Read(body)
			if err != nil && err.Error() != "EOF" {
				continue
			}

			ip := strings.TrimSpace(string(body[:n]))
			// Verify if it's a valid IP address
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	return ""
}

// getPublicIPFromInterface Get first public IP from network interface
func getPublicIPFromInterface() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		// Skip disabled interfaces and loopback interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// Only consider IPv4 addresses
			if ip.To4() != nil {
				ipStr := ip.String()
				// Exclude private IP address ranges
				if !isPrivateIP(ip) {
					return ipStr
				}
			}
		}
	}

	return ""
}

// isPrivateIP Check if it's a private IP address
func isPrivateIP(ip net.IP) bool {
	// Private IP address ranges:
	// 10.0.0.0/8
	// 172.16.0.0/12
	// 192.168.0.0/16
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	for _, cidr := range privateRanges {
		_, subnet, _ := net.ParseCIDR(cidr)
		if subnet.Contains(ip) {
			return true
		}
	}

	return false
}

// getTraderFromQuery Get trader from query parameter
func (s *Server) getTraderFromQuery(c *gin.Context) (*manager.TraderManager, string, error) {
	userID := c.GetString("user_id")
	traderID := c.Query("trader_id")

	// Ensure user's traders are loaded into memory
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ Failed to load traders for user %s: %v", userID, err)
	}

	if traderID == "" {
		// If trader_id is not specified, return the user's first trader
		ids := s.traderManager.GetTraderIDs()
		if len(ids) == 0 {
			return nil, "", fmt.Errorf("no trader available")
		}

		// Get user's trader list, prioritize returning user's own traders
		userTraders, err := s.database.GetTraders(userID)
		if err == nil && len(userTraders) > 0 {
			traderID = userTraders[0].ID
		} else {
			traderID = ids[0]
		}
	}

	return s.traderManager, traderID, nil
}

// AI trader management related structures
type CreateTraderRequest struct {
	Name                string  `json:"name" binding:"required"`
	AIModelID           string  `json:"ai_model_id" binding:"required"`
	ExchangeID          string  `json:"exchange_id" binding:"required"`
	StrategyID          string  `json:"strategy_id"` // Strategy ID (new version)
	InitialBalance      float64 `json:"initial_balance"`
	ScanIntervalMinutes int     `json:"scan_interval_minutes"`
	// The following fields are kept for backward compatibility, new version uses strategy config
	BTCETHLeverage       int    `json:"btc_eth_leverage"`
	AltcoinLeverage      int    `json:"altcoin_leverage"`
	TradingSymbols       string `json:"trading_symbols"`
	CustomPrompt         string `json:"custom_prompt"`
	OverrideBasePrompt   bool   `json:"override_base_prompt"`
	SystemPromptTemplate string `json:"system_prompt_template"` // System prompt template name
	IsCrossMargin        *bool  `json:"is_cross_margin"`        // Pointer type, nil means use default value true
	ShowInCompetition    *bool  `json:"show_in_competition"`    // Pointer type, nil means use default value true
	UseCoinPool          bool   `json:"use_coin_pool"`
	UseOITop             bool   `json:"use_oi_top"`
	UseTradingView       bool   `json:"use_tradingview"`
	FollowedTraderID     string `json:"followed_trader_id"` // Followed trader ID (for follower role)
	// Indicator configuration
	EnableRawKlines    bool   `json:"enable_raw_klines"`   // Raw OHLCV klines (always true, required)
	EnableEMA          bool   `json:"enable_ema"`          // Enable EMA indicator
	EnableMACD         bool   `json:"enable_macd"`         // Enable MACD indicator
	EnableRSI          bool   `json:"enable_rsi"`          // Enable RSI indicator
	EnableATR          bool   `json:"enable_atr"`          // Enable ATR indicator
	EnableVolume       bool   `json:"enable_volume"`       // Enable volume data
	EnableOI           bool   `json:"enable_oi"`           // Enable open interest data
	EnableFunding      bool   `json:"enable_funding"`      // Enable funding rate data
	IndicatorTimeframe string `json:"indicator_timeframe"` // Timeframe for indicators (e.g., "3m", "15m", "1h", "4h")
	QuantDataURL       string `json:"quant_data_url"`      // External quant data API URL with {symbol} placeholder
}

// Strategy management related structures
type CreateStrategyRequest struct {
	Name                 string `json:"name" binding:"required"`
	Description          string `json:"description"`
	SystemPromptTemplate string `json:"system_prompt_template"`
	CustomPrompt         string `json:"custom_prompt"`
	OverrideBasePrompt   bool   `json:"override_base_prompt"`
	BTCETHLeverage       int    `json:"btc_eth_leverage"`
	AltcoinLeverage      int    `json:"altcoin_leverage"`
	TradingSymbols       string `json:"trading_symbols"`
	IsCrossMargin        bool   `json:"is_cross_margin"`
	UseCoinPool          bool   `json:"use_coin_pool"`
	UseOITop             bool   `json:"use_oi_top"`
	UseTradingView       bool   `json:"use_tradingview"`
	EnableRawKlines      bool   `json:"enable_raw_klines"`
	EnableEMA            bool   `json:"enable_ema"`
	EnableMACD           bool   `json:"enable_macd"`
	EnableRSI            bool   `json:"enable_rsi"`
	EnableATR            bool   `json:"enable_atr"`
	EnableVolume         bool   `json:"enable_volume"`
	EnableOI             bool   `json:"enable_oi"`
	EnableFunding        bool   `json:"enable_funding"`
	IndicatorTimeframe   string `json:"indicator_timeframe"`
	QuantDataURL         string `json:"quant_data_url"`
	// Risk Management Configuration
	MinRiskRewardRatio    *float64 `json:"min_risk_reward_ratio"`    // Default: 3.0 (1:3)
	MaxPositions           *int     `json:"max_positions"`            // Default: 3
	MarginUsageLimit       *float64 `json:"margin_usage_limit"`      // Default: 90.0
	MinOpeningAmount       *float64 `json:"min_opening_amount"`      // Default: 12.0
	MinOpeningAmountBTCETH *float64 `json:"min_opening_amount_btc_eth"` // Default: 60.0
	// Position Sizing Configuration
	AltcoinPositionMin      *float64 `json:"altcoin_position_min"`    // Default: 0.8
	AltcoinPositionMax      *float64 `json:"altcoin_position_max"`    // Default: 1.5
	BTCETHPositionMin       *float64 `json:"btc_eth_position_min"`    // Default: 5.0
	BTCETHPositionMax       *float64 `json:"btc_eth_position_max"`    // Default: 10.0
	AvailableMarginMultiplier *float64 `json:"available_margin_multiplier"` // Default: 0.88
	// Trading Rules Configuration
	MinConfidenceForEntry *int     `json:"min_confidence_for_entry"` // Default: 75
	MinHoldingTimeMinutes *int     `json:"min_holding_time_minutes"` // Default: 30
	// Sharpe Ratio Configuration (JSON string)
	SharpeRatioConfig *string `json:"sharpe_ratio_config"` // JSON: { thresholds: [...] }
}

type UpdateStrategyRequest struct {
	Name                 string `json:"name" binding:"required"`
	Description          string `json:"description"`
	SystemPromptTemplate string `json:"system_prompt_template"`
	CustomPrompt         string `json:"custom_prompt"`
	OverrideBasePrompt   bool   `json:"override_base_prompt"`
	BTCETHLeverage       int    `json:"btc_eth_leverage"`
	AltcoinLeverage      int    `json:"altcoin_leverage"`
	TradingSymbols       string `json:"trading_symbols"`
	IsCrossMargin        bool   `json:"is_cross_margin"`
	UseCoinPool          bool   `json:"use_coin_pool"`
	UseOITop             bool   `json:"use_oi_top"`
	UseTradingView       bool   `json:"use_tradingview"`
	EnableRawKlines      bool   `json:"enable_raw_klines"`
	EnableEMA            bool   `json:"enable_ema"`
	EnableMACD           bool   `json:"enable_macd"`
	EnableRSI            bool   `json:"enable_rsi"`
	EnableATR            bool   `json:"enable_atr"`
	EnableVolume         bool   `json:"enable_volume"`
	EnableOI             bool   `json:"enable_oi"`
	EnableFunding        bool   `json:"enable_funding"`
	IndicatorTimeframe   string `json:"indicator_timeframe"`
	QuantDataURL         string `json:"quant_data_url"`
	// Risk Management Configuration
	MinRiskRewardRatio    *float64 `json:"min_risk_reward_ratio"`    // Default: 3.0 (1:3)
	MaxPositions           *int     `json:"max_positions"`            // Default: 3
	MarginUsageLimit       *float64 `json:"margin_usage_limit"`      // Default: 90.0
	MinOpeningAmount       *float64 `json:"min_opening_amount"`      // Default: 12.0
	MinOpeningAmountBTCETH *float64 `json:"min_opening_amount_btc_eth"` // Default: 60.0
	// Position Sizing Configuration
	AltcoinPositionMin      *float64 `json:"altcoin_position_min"`    // Default: 0.8
	AltcoinPositionMax      *float64 `json:"altcoin_position_max"`    // Default: 1.5
	BTCETHPositionMin       *float64 `json:"btc_eth_position_min"`    // Default: 5.0
	BTCETHPositionMax       *float64 `json:"btc_eth_position_max"`    // Default: 10.0
	AvailableMarginMultiplier *float64 `json:"available_margin_multiplier"` // Default: 0.88
	// Trading Rules Configuration
	MinConfidenceForEntry *int     `json:"min_confidence_for_entry"` // Default: 75
	MinHoldingTimeMinutes *int     `json:"min_holding_time_minutes"` // Default: 30
	// Sharpe Ratio Configuration (JSON string)
	SharpeRatioConfig *string `json:"sharpe_ratio_config"` // JSON: { thresholds: [...] }
}

type ImportStrategyRequest struct {
	StrategyData map[string]interface{} `json:"strategy_data" binding:"required"`
}

type ModelConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Enabled      bool   `json:"enabled"`
	APIKey       string `json:"apiKey,omitempty"`
	CustomAPIURL string `json:"customApiUrl,omitempty"`
}

// SafeModelConfig Safe model configuration structure (does not contain sensitive information)
type SafeModelConfig struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Provider        string `json:"provider"`
	Enabled         bool   `json:"enabled"`
	CustomAPIURL    string `json:"customApiUrl"`    // Custom API URL (usually not sensitive)
	CustomModelName string `json:"customModelName"` // Custom model name (not sensitive)
}

// SupportedModel Supported AI model structure matching frontend AIModel interface
type SupportedModel struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Provider        string `json:"provider"`
	Enabled         bool   `json:"enabled"`         // Always false for supported list
	CustomAPIURL    string `json:"customApiUrl"`    // Empty string for supported list
	CustomModelName string `json:"customModelName"` // Empty string for supported list
}

type ExchangeConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"` // "cex" or "dex"
	Enabled   bool   `json:"enabled"`
	APIKey    string `json:"apiKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
	Testnet   bool   `json:"testnet,omitempty"`
}

// SafeExchangeConfig Safe exchange configuration structure (does not contain sensitive information)
type SafeExchangeConfig struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Type                  string `json:"type"` // "cex" or "dex"
	Enabled               bool   `json:"enabled"`
	Testnet               bool   `json:"testnet,omitempty"`
	HyperliquidWalletAddr string `json:"hyperliquidWalletAddr"` // Hyperliquid wallet address (not sensitive)
	AsterUser             string `json:"asterUser"`             // Aster username (not sensitive)
	AsterSigner           string `json:"asterSigner"`           // Aster signer (not sensitive)
	LighterWalletAddr     string `json:"lighterWalletAddr"`     // LIGHTER wallet address (not sensitive)
}

type UpdateModelConfigRequest struct {
	Models map[string]struct {
		Enabled         bool   `json:"enabled"`
		APIKey          string `json:"api_key"`
		CustomAPIURL    string `json:"custom_api_url"`
		CustomModelName string `json:"custom_model_name"`
	} `json:"models"`
}

type UpdateExchangeConfigRequest struct {
	Exchanges map[string]struct {
		Enabled                 bool   `json:"enabled"`
		APIKey                  string `json:"api_key"`
		SecretKey               string `json:"secret_key"`
		Testnet                 bool   `json:"testnet"`
		HyperliquidWalletAddr   string `json:"hyperliquid_wallet_addr"`
		AsterUser               string `json:"aster_user"`
		AsterSigner             string `json:"aster_signer"`
		AsterPrivateKey         string `json:"aster_private_key"`
		LighterWalletAddr       string `json:"lighter_wallet_addr"`
		LighterPrivateKey       string `json:"lighter_private_key"`
		LighterAPIKeyPrivateKey string `json:"lighter_api_key_private_key"`
		LighterAPIKeyIndex      int    `json:"lighter_api_key_index"`
		OkxPassphrase           string `json:"okx_passphrase"`
	} `json:"exchanges"`
}

// isPromptTemplateName Check if string is a prompt template name rather than AI model ID
func isPromptTemplateName(name string) bool {
	if name == "" {
		return false
	}
	// Known prompt template names
	promptTemplateNames := []string{
		"risk_management",
		"risk-management",
		"riskmanagement",
		"default",
		"adaptive",
		"aggressive",
		"conservative",
		"scalping",
	}
	nameLower := strings.ToLower(strings.TrimSpace(name))
	for _, templateName := range promptTemplateNames {
		if nameLower == strings.ToLower(templateName) {
			return true
		}
	}
	return false
}

// getDefaultAIModelForUser Get user's default AI model ID
func (s *Server) getDefaultAIModelForUser(userID string) string {
	// Try to get user's enabled AI models
	models, err := s.database.GetAIModels(userID)
	if err == nil {
		for _, model := range models {
			if model.Enabled {
				return model.ID
			}
		}
		// If none enabled, return first available
		if len(models) > 0 {
			return models[0].ID
		}
	}

	// Try to get from default user
	defaultModels, err := s.database.GetAIModels("default")
	if err == nil {
		for _, model := range defaultModels {
			if model.Enabled {
				return model.ID
			}
		}
		if len(defaultModels) > 0 {
			return defaultModels[0].ID
		}
	}

	// Finally fallback to common model ID
	return "deepseek" // Default to deepseek
}

// handleCreateTrader Create new AI trader
func (s *Server) handleCreateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	var req CreateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Log the received request to debug followed_trader_id
	log.Printf("🔍 DEBUG [handleCreateTrader]: Received request - FollowedTraderID='%s', Name='%s', UserID='%s'", req.FollowedTraderID, req.Name, userID)

	// If followed_trader_id is provided, copy all settings from that trader
	var followedTraderUserID string
	if req.FollowedTraderID != "" {
		log.Printf("✓ DEBUG [handleCreateTrader]: FollowedTraderID is set: '%s'", req.FollowedTraderID)
		// Get all users to find the followed trader
		allUserIDs, err := s.database.GetAllUsers()
		if err == nil {
			for _, uid := range allUserIDs {
				traders, err := s.database.GetTraders(uid)
				if err != nil {
					continue
				}
				for _, trader := range traders {
					if trader.ID == req.FollowedTraderID {
						// Found the followed trader, copy all settings
						followedTraderUserID = trader.UserID
						if req.Name == "" {
							req.Name = trader.Name + " (Copy)"
						}

						// Check if AI model ID is a prompt template name
						if req.AIModelID == "" {
							if isPromptTemplateName(trader.AIModelID) {
								// If ai_model_id is a prompt template name, map it to system_prompt_template
								log.Printf("⚠️ Detected prompt template name as AI model ID: %s, will map to system_prompt_template", trader.AIModelID)
								if req.SystemPromptTemplate == "" {
									req.SystemPromptTemplate = trader.AIModelID
								}
								// Use default AI model
								req.AIModelID = s.getDefaultAIModelForUser(userID)
								log.Printf("✓ Default AI model set: %s", req.AIModelID)
							} else {
								req.AIModelID = trader.AIModelID
							}
						}

						if req.ExchangeID == "" {
							req.ExchangeID = trader.ExchangeID
						}
						if req.InitialBalance == 0 {
							req.InitialBalance = trader.InitialBalance
						}
						if req.ScanIntervalMinutes == 0 {
							req.ScanIntervalMinutes = trader.ScanIntervalMinutes
						}
						if req.BTCETHLeverage == 0 {
							req.BTCETHLeverage = trader.BTCETHLeverage
						}
						if req.AltcoinLeverage == 0 {
							req.AltcoinLeverage = trader.AltcoinLeverage
						}
						if req.TradingSymbols == "" {
							req.TradingSymbols = trader.TradingSymbols
						}
						if req.CustomPrompt == "" {
							req.CustomPrompt = trader.CustomPrompt
						}
						req.OverrideBasePrompt = trader.OverrideBasePrompt
						// Set system_prompt_template
						// If ai_model_id has already been mapped to system_prompt_template, keep that value
						// Otherwise use the followed trader's system_prompt_template
						if req.SystemPromptTemplate == "" {
							if trader.SystemPromptTemplate != "" {
								req.SystemPromptTemplate = trader.SystemPromptTemplate
							} else if isPromptTemplateName(trader.AIModelID) {
								// If followed trader's ai_model_id is a prompt template but system_prompt_template is empty, use ai_model_id
								req.SystemPromptTemplate = trader.AIModelID
							}
						}
						if req.IsCrossMargin == nil {
							isCross := trader.IsCrossMargin
							req.IsCrossMargin = &isCross
						}
						req.UseCoinPool = trader.UseCoinPool
						req.UseOITop = trader.UseOITop
						req.UseTradingView = trader.UseTradingView

						// Handle strategy copying
						if trader.StrategyID != "" {
							// Parent trader has a strategy_id, copy the strategy to follower's account
							parentStrategy, err := s.database.GetStrategy(trader.StrategyID, trader.UserID)
							if err == nil {
								// Create a copy of the strategy for the follower
								newStrategyID := fmt.Sprintf("strategy_%s_%d", userID, time.Now().UnixNano())
								copiedStrategy := &config.StrategyRecord{
									ID:                     newStrategyID,
									UserID:                 userID,
									Name:                   parentStrategy.Name + " (Copy)",
									Description:            parentStrategy.Description,
									SystemPromptTemplate:   parentStrategy.SystemPromptTemplate,
									CustomPrompt:           parentStrategy.CustomPrompt,
									OverrideBasePrompt:     parentStrategy.OverrideBasePrompt,
									BTCETHLeverage:         parentStrategy.BTCETHLeverage,
									AltcoinLeverage:         parentStrategy.AltcoinLeverage,
									TradingSymbols:         parentStrategy.TradingSymbols,
									IsCrossMargin:          parentStrategy.IsCrossMargin,
									UseCoinPool:            parentStrategy.UseCoinPool,
									UseOITop:               parentStrategy.UseOITop,
									UseTradingView:          parentStrategy.UseTradingView,
									EnableRawKlines:        parentStrategy.EnableRawKlines,
									EnableEMA:              parentStrategy.EnableEMA,
									EnableMACD:             parentStrategy.EnableMACD,
									EnableRSI:              parentStrategy.EnableRSI,
									EnableATR:              parentStrategy.EnableATR,
									EnableVolume:           parentStrategy.EnableVolume,
									EnableOI:               parentStrategy.EnableOI,
									EnableFunding:          parentStrategy.EnableFunding,
									IndicatorTimeframe:     parentStrategy.IndicatorTimeframe,
									QuantDataURL:           parentStrategy.QuantDataURL,
									MinRiskRewardRatio:     parentStrategy.MinRiskRewardRatio,
									MaxPositions:            parentStrategy.MaxPositions,
									MarginUsageLimit:       parentStrategy.MarginUsageLimit,
									MinOpeningAmount:        parentStrategy.MinOpeningAmount,
									MinOpeningAmountBTCETH:  parentStrategy.MinOpeningAmountBTCETH,
									AltcoinPositionMin:     parentStrategy.AltcoinPositionMin,
									AltcoinPositionMax:     parentStrategy.AltcoinPositionMax,
									BTCETHPositionMin:      parentStrategy.BTCETHPositionMin,
									BTCETHPositionMax:      parentStrategy.BTCETHPositionMax,
									AvailableMarginMultiplier: parentStrategy.AvailableMarginMultiplier,
									MinConfidenceForEntry:  parentStrategy.MinConfidenceForEntry,
									MinHoldingTimeMinutes:  parentStrategy.MinHoldingTimeMinutes,
									SharpeRatioConfig:      parentStrategy.SharpeRatioConfig,
								}
								if err := s.database.CreateStrategy(copiedStrategy); err == nil {
									req.StrategyID = newStrategyID
									log.Printf("✓ Copied strategy %s to follower account as %s", trader.StrategyID, newStrategyID)
								} else {
									log.Printf("⚠️ Failed to copy strategy: %v", err)
								}
							} else {
								log.Printf("⚠️ Failed to get parent strategy %s: %v", trader.StrategyID, err)
							}
						} else {
							// Parent trader has no strategy_id (embedded config), create strategy from parent's config
							newStrategyID := fmt.Sprintf("strategy_%s_%d", userID, time.Now().UnixNano())
							extractedStrategy := &config.StrategyRecord{
								ID:                     newStrategyID,
								UserID:                 userID,
								Name:                   trader.Name + " Strategy",
								Description:            "Strategy extracted from trader configuration",
								SystemPromptTemplate:   trader.SystemPromptTemplate,
								CustomPrompt:           trader.CustomPrompt,
								OverrideBasePrompt:     trader.OverrideBasePrompt,
								BTCETHLeverage:         trader.BTCETHLeverage,
								AltcoinLeverage:        trader.AltcoinLeverage,
								TradingSymbols:         trader.TradingSymbols,
								IsCrossMargin:          trader.IsCrossMargin,
								UseCoinPool:            trader.UseCoinPool,
								UseOITop:               trader.UseOITop,
								UseTradingView:         trader.UseTradingView,
								EnableRawKlines:        trader.EnableRawKlines,
								EnableEMA:              trader.EnableEMA,
								EnableMACD:             trader.EnableMACD,
								EnableRSI:              trader.EnableRSI,
								EnableATR:              trader.EnableATR,
								EnableVolume:           trader.EnableVolume,
								EnableOI:               trader.EnableOI,
								EnableFunding:          trader.EnableFunding,
								IndicatorTimeframe:     trader.IndicatorTimeframe,
								QuantDataURL:           trader.QuantDataURL,
								// Use default values for new configuration fields (trader doesn't have these)
								MinRiskRewardRatio:     3.0,
								MaxPositions:            3,
								MarginUsageLimit:       90.0,
								MinOpeningAmount:        12.0,
								MinOpeningAmountBTCETH:  60.0,
								AltcoinPositionMin:      0.8,
								AltcoinPositionMax:      1.5,
								BTCETHPositionMin:       5.0,
								BTCETHPositionMax:       10.0,
								AvailableMarginMultiplier: 0.88,
								MinConfidenceForEntry:   75,
								MinHoldingTimeMinutes:   30,
								SharpeRatioConfig:       "",
							}
							if err := s.database.CreateStrategy(extractedStrategy); err == nil {
								req.StrategyID = newStrategyID
								log.Printf("✓ Created strategy from parent trader config: %s", newStrategyID)
							} else {
								log.Printf("⚠️ Failed to create strategy from parent config: %v", err)
							}
						}

						// Also copy embedded fields for backward compatibility
						req.EnableRawKlines = trader.EnableRawKlines
						req.EnableEMA = trader.EnableEMA
						req.EnableMACD = trader.EnableMACD
						req.EnableRSI = trader.EnableRSI
						req.EnableATR = trader.EnableATR
						req.EnableVolume = trader.EnableVolume
						req.EnableOI = trader.EnableOI
						req.EnableFunding = trader.EnableFunding
						req.IndicatorTimeframe = trader.IndicatorTimeframe
						req.QuantDataURL = trader.QuantDataURL

						break
					}
				}
			}
		}

		// Automatically ensure follower user has required AI model and exchange configuration
		if followedTraderUserID != "" && req.AIModelID != "" {
			// Check if AI model exists, if not copy it
			if err := s.database.CopyAIModelToUser(followedTraderUserID, userID, req.AIModelID); err != nil {
				log.Printf("⚠️ Failed to automatically create AI model configuration: %v (user may need to configure manually)", err)
				// Don't return error, allow user to configure manually later
			}
		}

		if followedTraderUserID != "" && req.ExchangeID != "" {
			// Check if exchange exists, if not copy it
			if err := s.database.CopyExchangeToUser(followedTraderUserID, userID, req.ExchangeID); err != nil {
				log.Printf("⚠️ Failed to automatically create exchange configuration: %v (user may need to configure manually)", err)
				// Don't return error, allow user to configure manually later
			}
		}
	}

	// Validate leverage values
	if req.BTCETHLeverage < 0 || req.BTCETHLeverage > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "BTC/ETH leverage must be between 1-50x"})
		return
	}
	if req.AltcoinLeverage < 0 || req.AltcoinLeverage > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Altcoin leverage must be between 1-20x"})
		return
	}

	// Validate trading symbol format
	if req.TradingSymbols != "" {
		symbols := strings.Split(req.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" && !strings.HasSuffix(strings.ToUpper(symbol), "USDT") {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid symbol format: %s, must end with USDT", symbol)})
				return
			}
		}
	}

	// Validate quant data URL if present (check for {symbol} placeholder)
	// This validation warns if a quant data API URL is provided but missing {symbol} placeholder
	validateQuantDataURL := func(url string) {
		if url != "" && !strings.Contains(url, "{symbol}") {
			log.Printf("⚠️  WARNING: Quant data API URL missing {symbol} placeholder: %s", url)
		}
	}
	// Note: Call validateQuantDataURL(req.QuantDataURL) when quant_data_url field is added to CreateTraderRequest
	_ = validateQuantDataURL // Suppress unused variable warning until field is added

	// Generate trader ID
	traderID := fmt.Sprintf("%s_%s_%d", req.ExchangeID, req.AIModelID, time.Now().Unix())

	// Set default values
	isCrossMargin := true // Default to cross margin mode
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	showInCompetition := true // Default to show in competition
	if req.ShowInCompetition != nil {
		showInCompetition = *req.ShowInCompetition
	}

	// Set leverage default values (from system configuration)
	btcEthLeverage := 5
	altcoinLeverage := 5
	if req.BTCETHLeverage > 0 {
		btcEthLeverage = req.BTCETHLeverage
	} else {
		// Get default value from system configuration
		if btcEthLeverageStr, _ := s.database.GetSystemConfig("btc_eth_leverage"); btcEthLeverageStr != "" {
			if val, err := strconv.Atoi(btcEthLeverageStr); err == nil && val > 0 {
				btcEthLeverage = val
			}
		}
	}
	if req.AltcoinLeverage > 0 {
		altcoinLeverage = req.AltcoinLeverage
	} else {
		// Get default value from system configuration
		if altcoinLeverageStr, _ := s.database.GetSystemConfig("altcoin_leverage"); altcoinLeverageStr != "" {
			if val, err := strconv.Atoi(altcoinLeverageStr); err == nil && val > 0 {
				altcoinLeverage = val
			}
		}
	}

	// Set system prompt template default value
	systemPromptTemplate := "default"
	if req.SystemPromptTemplate != "" {
		systemPromptTemplate = req.SystemPromptTemplate
	}

	// Set scan interval default value
	scanIntervalMinutes := req.ScanIntervalMinutes
	if scanIntervalMinutes < 3 {
		scanIntervalMinutes = 3 // Default 3 minutes, and not allowed to be less than 3
	}

	// ✨ Query exchange actual balance, override user input
	actualBalance := req.InitialBalance // Default to use user input
	exchanges, err := s.database.GetExchanges(userID)
	if err != nil {
		log.Printf("⚠️ Failed to get exchange configuration, using user input initial balance: %v", err)
	}

	// Find matching exchange configuration
	var exchangeCfg *config.ExchangeConfig
	for _, ex := range exchanges {
		if ex.ID == req.ExchangeID {
			exchangeCfg = ex
			break
		}
	}

	if exchangeCfg == nil {
		log.Printf("⚠️ Exchange %s configuration not found, using user input initial balance", req.ExchangeID)
	} else if !exchangeCfg.Enabled {
		log.Printf("⚠️ Exchange %s is not enabled, using user input initial balance", req.ExchangeID)
	} else {
		// Create temporary trader based on exchange type to query balance
		var tempTrader trader.Trader
		var createErr error

		switch req.ExchangeID {
		case "binance":
			tempTrader = trader.NewFuturesTrader(exchangeCfg.APIKey, exchangeCfg.SecretKey, userID)
		case "hyperliquid":
			tempTrader, createErr = trader.NewHyperliquidTrader(
				exchangeCfg.APIKey, // private key
				exchangeCfg.HyperliquidWalletAddr,
				exchangeCfg.Testnet,
			)
		case "aster":
			// Debug logging for Aster configuration
			log.Printf("🔍 [Aster] Creating trader with config: user=%s (len=%d), signer=%s (len=%d), privateKey=*** (len=%d)",
				exchangeCfg.AsterUser, len(exchangeCfg.AsterUser),
				exchangeCfg.AsterSigner, len(exchangeCfg.AsterSigner),
				len(exchangeCfg.AsterPrivateKey))
			tempTrader, createErr = trader.NewAsterTrader(
				exchangeCfg.AsterUser,
				exchangeCfg.AsterSigner,
				exchangeCfg.AsterPrivateKey,
			)
		case "lighter":
			log.Printf("🔍 [Lighter] Creating trader with config: walletAddr=%s (len=%d), apiKeyPrivateKey=*** (len=%d), apiKeyIndex=%d",
				MaskWalletAddress(exchangeCfg.LighterWalletAddr), len(exchangeCfg.LighterWalletAddr),
				len(exchangeCfg.LighterAPIKeyPrivateKey), exchangeCfg.LighterAPIKeyIndex)
			tempTrader, createErr = trader.NewLighterTraderV2(
				exchangeCfg.LighterWalletAddr,
				exchangeCfg.LighterAPIKeyPrivateKey,
				exchangeCfg.LighterAPIKeyIndex,
				exchangeCfg.Testnet,
			)
		case "bybit":
			tempTrader = trader.NewBybitTrader(
				exchangeCfg.APIKey,
				exchangeCfg.SecretKey,
			)
		case "okx":
			tempTrader = trader.NewOKXTrader(
				exchangeCfg.APIKey,
				exchangeCfg.SecretKey,
				exchangeCfg.OkxPassphrase,
			)
		case "bitget":
			tempTrader = trader.NewBitgetTrader(
				exchangeCfg.APIKey,
				exchangeCfg.SecretKey,
				exchangeCfg.OkxPassphrase, // Reuse passphrase field
			)
		default:
			log.Printf("⚠️ Unsupported exchange type: %s, using user input initial balance", req.ExchangeID)
		}

		if createErr != nil {
			log.Printf("⚠️ Failed to create temporary trader, using user input initial balance: %v", createErr)
		} else if tempTrader != nil {
			// Query actual balance
			balanceInfo, balanceErr := tempTrader.GetBalance()
			if balanceErr != nil {
				log.Printf("⚠️ Failed to query exchange balance, using user input initial balance: %v", balanceErr)
			} else {
				// Extract available balance - prioritize available_balance over total_equity
				// Use available_balance (funds available for trading) instead of total_equity (includes unrealized PnL)
				if availableBalance, ok := balanceInfo["available_balance"].(float64); ok && availableBalance > 0 {
					// Snake case format: available_balance
					actualBalance = availableBalance
					log.Printf("✓ Queried exchange available balance: %.2f USDT (user input: %.2f USDT)", actualBalance, req.InitialBalance)
				} else if availableBalance, ok := balanceInfo["availableBalance"].(float64); ok && availableBalance > 0 {
					// Camel case format: availableBalance
					actualBalance = availableBalance
					log.Printf("✓ Queried exchange available balance: %.2f USDT (user input: %.2f USDT)", actualBalance, req.InitialBalance)
				} else if totalBalance, ok := balanceInfo["balance"].(float64); ok && totalBalance > 0 {
					// Fallback: generic balance field
					actualBalance = totalBalance
					log.Printf("✓ Queried exchange balance: %.2f USDT (user input: %.2f USDT)", actualBalance, req.InitialBalance)
				} else {
					log.Printf("⚠️ Unable to extract available balance from balance info, balanceInfo=%v, using user input initial balance", balanceInfo)
				}
			}
		}
	}

	// Set indicator configuration defaults
	enableRawKlines := true // Always true, required
	enableEMA := req.EnableEMA
	enableMACD := req.EnableMACD
	enableRSI := req.EnableRSI
	enableATR := req.EnableATR
	enableVolume := req.EnableVolume
	if !req.EnableVolume && !req.EnableOI && !req.EnableFunding {
		// If no indicator config provided, use defaults: disable EMA/MACD/RSI/ATR, enable volume/OI/funding
		enableVolume = true
	}
	enableOI := req.EnableOI
	if !req.EnableVolume && !req.EnableOI && !req.EnableFunding {
		enableOI = true
	}
	enableFunding := req.EnableFunding
	if !req.EnableVolume && !req.EnableOI && !req.EnableFunding {
		enableFunding = true
	}
	indicatorTimeframe := req.IndicatorTimeframe
	if indicatorTimeframe == "" {
		indicatorTimeframe = "3m"
	}
	quantDataURL := req.QuantDataURL

	// Validate quant data URL if present (check for {symbol} placeholder)
	if quantDataURL != "" && !strings.Contains(quantDataURL, "{symbol}") {
		log.Printf("⚠️  WARNING: Quant data API URL missing {symbol} placeholder: %s", quantDataURL)
	}

	// Handle strategy_id: if provided, validate it exists but don't merge settings
	// When strategy_id is set, we only save the reference - settings will be loaded from strategy when needed
	strategyID := req.StrategyID
	// #region agent log
	if strategyID != "" {
		logFile, _ := os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if logFile != nil {
			logEntry := fmt.Sprintf(`{"sessionId":"debug-session","runId":"run1","hypothesisId":"E","location":"api/server.go:1220","message":"CreateTrader received strategy_id","data":{"strategyID":"%s","userID":"%s"},"timestamp":%d}`+"\n", strategyID, userID, time.Now().UnixMilli())
			logFile.WriteString(logEntry)
			logFile.Close()
		}
	}
	// #endregion
	if strategyID != "" {
		_, err := s.database.GetStrategy(strategyID, userID)
		if err != nil {
			log.Printf("⚠️ Failed to load strategy %s: %v, proceeding without strategy", strategyID, err)
			// #region agent log
			logFile, _ := os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if logFile != nil {
				logEntry := fmt.Sprintf(`{"sessionId":"debug-session","runId":"run1","hypothesisId":"E","location":"api/server.go:1228","message":"Strategy validation failed","data":{"strategyID":"%s","error":"%v"},"timestamp":%d}`+"\n", strategyID, err, time.Now().UnixMilli())
				logFile.WriteString(logEntry)
				logFile.Close()
			}
			// #endregion
			strategyID = "" // Clear invalid strategy_id
		} else {
			log.Printf("✓ Validated strategy %s - settings will be loaded from strategy when needed", strategyID)
			// #region agent log
			logFile, _ := os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if logFile != nil {
				logEntry := fmt.Sprintf(`{"sessionId":"debug-session","runId":"run1","hypothesisId":"E","location":"api/server.go:1233","message":"Strategy validated successfully","data":{"strategyID":"%s"},"timestamp":%d}`+"\n", strategyID, time.Now().UnixMilli())
				logFile.WriteString(logEntry)
				logFile.Close()
			}
			// #endregion
		}
	}

	// Set isCrossMargin: if strategy_id is set, use default (will be loaded from strategy)
	// Otherwise, use provided value or default
	if strategyID != "" {
		isCrossMargin = true // Default value, will be loaded from strategy
	} else if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	// Create trader configuration (database entity)
	// If strategy_id is set, use defaults/empty for strategy-related fields (they'll be loaded from strategy)
	// If strategy_id is not set, use the provided values
	var traderStrategySettings struct {
		BTCETHLeverage      int
		AltcoinLeverage     int
		TradingSymbols      string
		CustomPrompt        string
		OverrideBasePrompt  bool
		SystemPromptTpl     string
		UseCoinPool         bool
		UseOITop            bool
		UseTradingView      bool
		EnableRawKlines     bool
		EnableEMA           bool
		EnableMACD          bool
		EnableRSI           bool
		EnableATR           bool
		EnableVolume        bool
		EnableOI            bool
		EnableFunding       bool
		IndicatorTimeframe  string
		QuantDataURL        string
	}

	if strategyID != "" {
		// Strategy reference mode: use defaults/empty values
		// Settings will be loaded from strategy when trader config is retrieved
		traderStrategySettings.BTCETHLeverage = 0
		traderStrategySettings.AltcoinLeverage = 0
		traderStrategySettings.TradingSymbols = ""
		traderStrategySettings.CustomPrompt = ""
		traderStrategySettings.OverrideBasePrompt = false
		traderStrategySettings.SystemPromptTpl = ""
		traderStrategySettings.UseCoinPool = false
		traderStrategySettings.UseOITop = false
		traderStrategySettings.UseTradingView = false
		traderStrategySettings.EnableRawKlines = true // Required field
		traderStrategySettings.EnableEMA = false
		traderStrategySettings.EnableMACD = false
		traderStrategySettings.EnableRSI = false
		traderStrategySettings.EnableATR = false
		traderStrategySettings.EnableVolume = true
		traderStrategySettings.EnableOI = true
		traderStrategySettings.EnableFunding = true
		traderStrategySettings.IndicatorTimeframe = ""
		traderStrategySettings.QuantDataURL = ""
		log.Printf("✓ Using strategy reference mode - strategy settings will be loaded from strategy %s", strategyID)
	} else {
		// Custom config mode: use provided values
		traderStrategySettings.BTCETHLeverage = btcEthLeverage
		traderStrategySettings.AltcoinLeverage = altcoinLeverage
		traderStrategySettings.TradingSymbols = req.TradingSymbols
		traderStrategySettings.CustomPrompt = req.CustomPrompt
		traderStrategySettings.OverrideBasePrompt = req.OverrideBasePrompt
		traderStrategySettings.SystemPromptTpl = systemPromptTemplate
		traderStrategySettings.UseCoinPool = req.UseCoinPool
		traderStrategySettings.UseOITop = req.UseOITop
		traderStrategySettings.UseTradingView = req.UseTradingView
		traderStrategySettings.EnableRawKlines = enableRawKlines
		traderStrategySettings.EnableEMA = enableEMA
		traderStrategySettings.EnableMACD = enableMACD
		traderStrategySettings.EnableRSI = enableRSI
		traderStrategySettings.EnableATR = enableATR
		traderStrategySettings.EnableVolume = enableVolume
		traderStrategySettings.EnableOI = enableOI
		traderStrategySettings.EnableFunding = enableFunding
		traderStrategySettings.IndicatorTimeframe = indicatorTimeframe
		traderStrategySettings.QuantDataURL = quantDataURL
		log.Printf("✓ Using custom config mode - saving individual settings")
	}

	log.Printf("🔧 DEBUG [CreateTrader]: Starting to create trader configuration, ID=%s, Name=%s, AIModel=%s, Exchange=%s, FollowedTraderID='%s', StrategyID='%s'", traderID, req.Name, req.AIModelID, req.ExchangeID, req.FollowedTraderID, strategyID)
	// #region agent log
	logFile, _ := os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if logFile != nil {
		logEntry := fmt.Sprintf(`{"sessionId":"debug-session","runId":"run1","hypothesisId":"F","location":"api/server.go:1311","message":"Creating TraderRecord with strategy_id","data":{"traderID":"%s","strategyID":"%s","reqStrategyID":"%s"},"timestamp":%d}`+"\n", traderID, strategyID, req.StrategyID, time.Now().UnixMilli())
		logFile.WriteString(logEntry)
		logFile.Close()
	}
	// #endregion
	trader := &config.TraderRecord{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		InitialBalance:       actualBalance, // Use actual queried balance
		BTCETHLeverage:       traderStrategySettings.BTCETHLeverage,
		AltcoinLeverage:      traderStrategySettings.AltcoinLeverage,
		TradingSymbols:       traderStrategySettings.TradingSymbols,
		UseCoinPool:          traderStrategySettings.UseCoinPool,
		UseOITop:             traderStrategySettings.UseOITop,
		UseTradingView:        traderStrategySettings.UseTradingView,
		FollowedTraderID:     req.FollowedTraderID,
		CustomPrompt:         traderStrategySettings.CustomPrompt,
		OverrideBasePrompt:   traderStrategySettings.OverrideBasePrompt,
		SystemPromptTemplate: traderStrategySettings.SystemPromptTpl,
		IsCrossMargin:        isCrossMargin,
		ShowInCompetition:    showInCompetition,
		StrategyID:           strategyID,
		ScanIntervalMinutes:  scanIntervalMinutes,
		EnableRawKlines:      traderStrategySettings.EnableRawKlines,
		EnableEMA:            traderStrategySettings.EnableEMA,
		EnableMACD:           traderStrategySettings.EnableMACD,
		EnableRSI:            traderStrategySettings.EnableRSI,
		EnableATR:            traderStrategySettings.EnableATR,
		EnableVolume:         traderStrategySettings.EnableVolume,
		EnableOI:             traderStrategySettings.EnableOI,
		EnableFunding:        traderStrategySettings.EnableFunding,
		IndicatorTimeframe:   traderStrategySettings.IndicatorTimeframe,
		QuantDataURL:         traderStrategySettings.QuantDataURL,
		IsRunning:            false,
	}

	// Save to database
	log.Printf("🔧 DEBUG: Preparing to call CreateTrader")
	// #region agent log
	logFile, _ = os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if logFile != nil {
		logEntry := fmt.Sprintf(`{"sessionId":"debug-session","runId":"run1","hypothesisId":"F","location":"api/server.go:1348","message":"About to save trader to database","data":{"traderID":"%s","traderStrategyID":"%s"},"timestamp":%d}`+"\n", trader.ID, trader.StrategyID, time.Now().UnixMilli())
		logFile.WriteString(logEntry)
		logFile.Close()
	}
	// #endregion
	err = s.database.CreateTrader(trader)
	if err != nil {
		log.Printf("❌ Failed to create trader: %v", err)
		// #region agent log
		logFile, _ = os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if logFile != nil {
			logEntry := fmt.Sprintf(`{"sessionId":"debug-session","runId":"run1","hypothesisId":"F","location":"api/server.go:1351","message":"CreateTrader database error","data":{"error":"%v","traderID":"%s"},"timestamp":%d}`+"\n", err, trader.ID, time.Now().UnixMilli())
			logFile.WriteString(logEntry)
			logFile.Close()
		}
		// #endregion
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create trader: %v", err)})
		return
	}
	log.Printf("✓ DEBUG [CreateTrader]: Trader saved to database successfully, FollowedTraderID='%s', SystemPromptTemplate='%s'", trader.FollowedTraderID, trader.SystemPromptTemplate)
	// #region agent log
	logFile, _ = os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if logFile != nil {
		logEntry := fmt.Sprintf(`{"sessionId":"debug-session","runId":"run1","hypothesisId":"F","location":"api/server.go:1354","message":"Trader saved successfully","data":{"traderID":"%s","traderStrategyID":"%s"},"timestamp":%d}`+"\n", trader.ID, trader.StrategyID, time.Now().UnixMilli())
		logFile.WriteString(logEntry)
		logFile.Close()
	}
	// #endregion

	// Invalidate competition cache to ensure new traders (including followers) are correctly displayed/filtered
	s.traderManager.InvalidateCompetitionCache()
	log.Printf("✓ DEBUG [CreateTrader]: Competition cache invalidated after creating trader")

	// Immediately load new trader into TraderManager
	log.Printf("🔧 DEBUG: Preparing to call LoadUserTraders")
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ Failed to load user traders into memory: %v", err)
		// Don't return error here, as trader has been successfully created in database
	}
	log.Printf("🔧 DEBUG: LoadUserTraders completed")

	log.Printf("✓ Trader created successfully: %s (model: %s, exchange: %s)", req.Name, req.AIModelID, req.ExchangeID)

	c.JSON(http.StatusCreated, gin.H{
		"trader_id":   traderID,
		"trader_name": req.Name,
		"ai_model":    req.AIModelID,
		"is_running":  false,
	})
}

// UpdateTraderRequest update trader request
type UpdateTraderRequest struct {
	Name                 string  `json:"name" binding:"required"`
	AIModelID            string  `json:"ai_model_id" binding:"required"`
	ExchangeID           string  `json:"exchange_id" binding:"required"`
	StrategyID           string  `json:"strategy_id"` // Strategy ID (new version)
	InitialBalance       float64 `json:"initial_balance"`
	ScanIntervalMinutes  int     `json:"scan_interval_minutes"`
	BTCETHLeverage       int     `json:"btc_eth_leverage"`
	AltcoinLeverage      int     `json:"altcoin_leverage"`
	TradingSymbols       string  `json:"trading_symbols"`
	UseCoinPool          bool    `json:"use_coin_pool"`
	UseOITop             bool    `json:"use_oi_top"`
	UseTradingView       bool    `json:"use_tradingview"`
	FollowedTraderID     string  `json:"followed_trader_id"` // Followed trader ID (for follower role)
	CustomPrompt         string  `json:"custom_prompt"`
	OverrideBasePrompt   bool    `json:"override_base_prompt"`
	SystemPromptTemplate string  `json:"system_prompt_template"` // System prompt template name
	IsCrossMargin        *bool   `json:"is_cross_margin"`
	ShowInCompetition    *bool   `json:"show_in_competition"` // Pointer type, nil means keep existing value
	// Indicator configuration
	EnableRawKlines    bool   `json:"enable_raw_klines"`   // Raw OHLCV klines (always true, required)
	EnableEMA          bool   `json:"enable_ema"`          // Enable EMA indicator
	EnableMACD         bool   `json:"enable_macd"`         // Enable MACD indicator
	EnableRSI          bool   `json:"enable_rsi"`          // Enable RSI indicator
	EnableATR          bool   `json:"enable_atr"`          // Enable ATR indicator
	EnableVolume       bool   `json:"enable_volume"`       // Enable volume data
	EnableOI           bool   `json:"enable_oi"`           // Enable open interest data
	EnableFunding      bool   `json:"enable_funding"`      // Enable funding rate data
	IndicatorTimeframe string `json:"indicator_timeframe"` // Timeframe for indicators (e.g., "3m", "15m", "1h", "4h")
	QuantDataURL       string `json:"quant_data_url"`      // External quant data API URL with {symbol} placeholder
}

// handleUpdateTrader update trader configuration
func (s *Server) handleUpdateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	var wasRunning bool
	if existing, err := s.traderManager.GetTrader(traderID); err == nil {
		if status := existing.GetStatus(); status != nil {
			if running, ok := status["is_running"].(bool); ok && running {
				wasRunning = true
				existing.Stop()
				_ = s.database.UpdateTraderStatus(userID, traderID, false)
				log.Printf("INFO: Stopped running trader before applying update (trader=%s)", traderID)
			}
		}
	}

	var req UpdateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if trader exists and belongs to current user
	traders, err := s.database.GetTraders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trader list"})
		return
	}

	var existingTrader *config.TraderRecord
	for _, trader := range traders {
		if trader.ID == traderID {
			existingTrader = trader
			break
		}
	}

	if existingTrader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	// Set default values
	isCrossMargin := existingTrader.IsCrossMargin // Keep original value
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	showInCompetition := existingTrader.ShowInCompetition // Keep original value
	if req.ShowInCompetition != nil {
		showInCompetition = *req.ShowInCompetition
	}

	// Set leverage default values
	btcEthLeverage := req.BTCETHLeverage
	altcoinLeverage := req.AltcoinLeverage
	if btcEthLeverage <= 0 {
		btcEthLeverage = existingTrader.BTCETHLeverage // Keep original value
	}
	if altcoinLeverage <= 0 {
		altcoinLeverage = existingTrader.AltcoinLeverage // Keep original value
	}

	// Set scan interval, allow update
	scanIntervalMinutes := req.ScanIntervalMinutes
	if scanIntervalMinutes <= 0 {
		scanIntervalMinutes = existingTrader.ScanIntervalMinutes // Keep original value
	} else if scanIntervalMinutes < 3 {
		scanIntervalMinutes = 3
	}

	// Handle strategy_id: determine if we're using strategy reference or custom config
	// If strategy_id changed from existing, use the new value (could be clearing it)
	// If strategy_id is same as existing, keep it
	strategyID := req.StrategyID
	// #region agent log
	logFile, _ := os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if logFile != nil {
		logEntry := fmt.Sprintf(`{"sessionId":"debug-session","runId":"run1","hypothesisId":"G","location":"api/server.go:1487","message":"UpdateTrader received strategy_id","data":{"reqStrategyID":"%s","existingStrategyID":"%s","traderID":"%s"},"timestamp":%d}`+"\n", req.StrategyID, existingTrader.StrategyID, traderID, time.Now().UnixMilli())
		logFile.WriteString(logEntry)
		logFile.Close()
	}
	// #endregion
	if strategyID == existingTrader.StrategyID {
		// StrategyID unchanged, keep existing
		strategyID = existingTrader.StrategyID
	}
	// Otherwise, strategyID is already set from req.StrategyID (could be new value or empty to clear)

	// Validate strategy_id if it's set
	if strategyID != "" {
		_, err := s.database.GetStrategy(strategyID, userID)
		if err != nil {
			log.Printf("⚠️ Failed to load strategy %s: %v, clearing strategy_id", strategyID, err)
			// #region agent log
			logFile, _ = os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if logFile != nil {
				logEntry := fmt.Sprintf(`{"sessionId":"debug-session","runId":"run1","hypothesisId":"G","location":"api/server.go:1498","message":"Strategy validation failed in UpdateTrader","data":{"strategyID":"%s","error":"%v"},"timestamp":%d}`+"\n", strategyID, err, time.Now().UnixMilli())
				logFile.WriteString(logEntry)
				logFile.Close()
			}
			// #endregion
			strategyID = "" // Clear invalid strategy_id
		} else {
			log.Printf("✓ Validated strategy %s - settings will be loaded from strategy when needed", strategyID)
			// #region agent log
			logFile, _ = os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if logFile != nil {
				logEntry := fmt.Sprintf(`{"sessionId":"debug-session","runId":"run1","hypothesisId":"G","location":"api/server.go:1501","message":"Strategy validated successfully in UpdateTrader","data":{"strategyID":"%s"},"timestamp":%d}`+"\n", strategyID, time.Now().UnixMilli())
				logFile.WriteString(logEntry)
				logFile.Close()
			}
			// #endregion
		}
	}

	// Determine if we're in strategy reference mode or custom config mode
	usingStrategy := strategyID != ""
	
	// Set isCrossMargin: if strategy_id is set, use default (will be loaded from strategy)
	// Otherwise, use provided value or keep existing
	if usingStrategy {
		isCrossMargin = true // Default value, will be loaded from strategy
	} else if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}
	
	// Set system prompt template based on mode
	var systemPromptTemplate string
	if usingStrategy {
		// Strategy reference mode: use empty/default (will be loaded from strategy)
		systemPromptTemplate = ""
		log.Printf("✓ Using strategy reference mode - strategy settings will be loaded from strategy %s", strategyID)
	} else {
		// Custom config mode: use provided value or keep existing
		systemPromptTemplate = req.SystemPromptTemplate
		if systemPromptTemplate == "" {
			systemPromptTemplate = existingTrader.SystemPromptTemplate // Keep original value
		}
		log.Printf("✓ Using custom config mode - saving individual settings")
	}

	// Validate quant data URL if present (check for {symbol} placeholder)
	// This validation warns if a quant data API URL is provided but missing {symbol} placeholder
	validateQuantDataURL := func(url string) {
		if url != "" && !strings.Contains(url, "{symbol}") {
			log.Printf("⚠️  WARNING: Quant data API URL missing {symbol} placeholder: %s", url)
		}
	}
	// Note: Call validateQuantDataURL(req.QuantDataURL) when quant_data_url field is added to UpdateTraderRequest
	_ = validateQuantDataURL // Suppress unused variable warning until field is added

	// Set indicator configuration based on mode
	var traderStrategySettings struct {
		BTCETHLeverage     int
		AltcoinLeverage    int
		TradingSymbols      string
		CustomPrompt        string
		OverrideBasePrompt  bool
		UseCoinPool         bool
		UseOITop            bool
		UseTradingView      bool
		EnableRawKlines     bool
		EnableEMA           bool
		EnableMACD          bool
		EnableRSI           bool
		EnableATR           bool
		EnableVolume        bool
		EnableOI            bool
		EnableFunding       bool
		IndicatorTimeframe  string
		QuantDataURL        string
	}

	if usingStrategy {
		// Strategy reference mode: use defaults/empty values
		// Settings will be loaded from strategy when trader config is retrieved
		traderStrategySettings.BTCETHLeverage = 0
		traderStrategySettings.AltcoinLeverage = 0
		traderStrategySettings.TradingSymbols = ""
		traderStrategySettings.CustomPrompt = ""
		traderStrategySettings.OverrideBasePrompt = false
		traderStrategySettings.UseCoinPool = false
		traderStrategySettings.UseOITop = false
		traderStrategySettings.UseTradingView = false
		traderStrategySettings.EnableRawKlines = true // Required field
		traderStrategySettings.EnableEMA = false
		traderStrategySettings.EnableMACD = false
		traderStrategySettings.EnableRSI = false
		traderStrategySettings.EnableATR = false
		traderStrategySettings.EnableVolume = true
		traderStrategySettings.EnableOI = true
		traderStrategySettings.EnableFunding = true
		traderStrategySettings.IndicatorTimeframe = ""
		traderStrategySettings.QuantDataURL = ""
	} else {
		// Custom config mode: use provided values or keep existing
		traderStrategySettings.BTCETHLeverage = btcEthLeverage
		traderStrategySettings.AltcoinLeverage = altcoinLeverage
		traderStrategySettings.TradingSymbols = req.TradingSymbols
		traderStrategySettings.CustomPrompt = req.CustomPrompt
		traderStrategySettings.OverrideBasePrompt = req.OverrideBasePrompt
		traderStrategySettings.UseCoinPool = req.UseCoinPool
		traderStrategySettings.UseOITop = req.UseOITop
		traderStrategySettings.UseTradingView = req.UseTradingView
		
		// Set indicator configuration - use existing values if not provided
		enableRawKlines := true // Always true, required
		enableEMA := req.EnableEMA
		if !req.EnableEMA && !req.EnableMACD && !req.EnableRSI && !req.EnableATR && req.QuantDataURL == "" {
			enableEMA = existingTrader.EnableEMA
		}
		enableMACD := req.EnableMACD
		if !req.EnableEMA && !req.EnableMACD && !req.EnableRSI && !req.EnableATR && req.QuantDataURL == "" {
			enableMACD = existingTrader.EnableMACD
		}
		enableRSI := req.EnableRSI
		if !req.EnableEMA && !req.EnableMACD && !req.EnableRSI && !req.EnableATR && req.QuantDataURL == "" {
			enableRSI = existingTrader.EnableRSI
		}
		enableATR := req.EnableATR
		if !req.EnableEMA && !req.EnableMACD && !req.EnableRSI && !req.EnableATR && req.QuantDataURL == "" {
			enableATR = existingTrader.EnableATR
		}
		enableVolume := req.EnableVolume
		if !req.EnableVolume && !req.EnableOI && !req.EnableFunding && req.QuantDataURL == "" {
			enableVolume = existingTrader.EnableVolume
		}
		enableOI := req.EnableOI
		if !req.EnableVolume && !req.EnableOI && !req.EnableFunding && req.QuantDataURL == "" {
			enableOI = existingTrader.EnableOI
		}
		enableFunding := req.EnableFunding
		if !req.EnableVolume && !req.EnableOI && !req.EnableFunding && req.QuantDataURL == "" {
			enableFunding = existingTrader.EnableFunding
		}
		indicatorTimeframe := req.IndicatorTimeframe
		if indicatorTimeframe == "" {
			indicatorTimeframe = existingTrader.IndicatorTimeframe
			if indicatorTimeframe == "" {
				indicatorTimeframe = "3m"
			}
		}
		quantDataURL := req.QuantDataURL
		if quantDataURL == "" {
			quantDataURL = existingTrader.QuantDataURL
		}

		// Validate quant data URL if present
		if quantDataURL != "" && !strings.Contains(quantDataURL, "{symbol}") {
			log.Printf("⚠️  WARNING: Quant data API URL missing {symbol} placeholder: %s", quantDataURL)
		}

		traderStrategySettings.EnableRawKlines = enableRawKlines
		traderStrategySettings.EnableEMA = enableEMA
		traderStrategySettings.EnableMACD = enableMACD
		traderStrategySettings.EnableRSI = enableRSI
		traderStrategySettings.EnableATR = enableATR
		traderStrategySettings.EnableVolume = enableVolume
		traderStrategySettings.EnableOI = enableOI
		traderStrategySettings.EnableFunding = enableFunding
		traderStrategySettings.IndicatorTimeframe = indicatorTimeframe
		traderStrategySettings.QuantDataURL = quantDataURL
	}

	// Update trader configuration
	trader := &config.TraderRecord{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		InitialBalance:       req.InitialBalance,
		BTCETHLeverage:       traderStrategySettings.BTCETHLeverage,
		AltcoinLeverage:      traderStrategySettings.AltcoinLeverage,
		TradingSymbols:       traderStrategySettings.TradingSymbols,
		UseCoinPool:          traderStrategySettings.UseCoinPool,
		UseOITop:             traderStrategySettings.UseOITop,
		UseTradingView:        traderStrategySettings.UseTradingView,
		FollowedTraderID:     req.FollowedTraderID,
		CustomPrompt:         traderStrategySettings.CustomPrompt,
		OverrideBasePrompt:   traderStrategySettings.OverrideBasePrompt,
		SystemPromptTemplate: systemPromptTemplate,
		IsCrossMargin:        isCrossMargin,
		ShowInCompetition:    showInCompetition,
		StrategyID:           strategyID,
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            existingTrader.IsRunning, // Keep original value
		EnableRawKlines:      traderStrategySettings.EnableRawKlines,
		EnableEMA:            traderStrategySettings.EnableEMA,
		EnableMACD:           traderStrategySettings.EnableMACD,
		EnableRSI:            traderStrategySettings.EnableRSI,
		EnableATR:            traderStrategySettings.EnableATR,
		EnableVolume:         traderStrategySettings.EnableVolume,
		EnableOI:             traderStrategySettings.EnableOI,
		EnableFunding:        traderStrategySettings.EnableFunding,
		IndicatorTimeframe:   traderStrategySettings.IndicatorTimeframe,
		QuantDataURL:         traderStrategySettings.QuantDataURL,
	}

	// Update database
	// #region agent log
	logFile, _ = os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if logFile != nil {
		logEntry := fmt.Sprintf(`{"sessionId":"debug-session","runId":"run1","hypothesisId":"G","location":"api/server.go:1690","message":"About to update trader in database","data":{"traderID":"%s","traderStrategyID":"%s"},"timestamp":%d}`+"\n", trader.ID, trader.StrategyID, time.Now().UnixMilli())
		logFile.WriteString(logEntry)
		logFile.Close()
	}
	// #endregion
	err = s.database.UpdateTrader(trader)
	if err != nil {
		log.Printf("❌ DEBUG [UpdateTrader]: Database update failed: %v", err)
		// #region agent log
		logFile, _ = os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if logFile != nil {
			logEntry := fmt.Sprintf(`{"sessionId":"debug-session","runId":"run1","hypothesisId":"G","location":"api/server.go:1693","message":"UpdateTrader database error","data":{"error":"%v","traderID":"%s"},"timestamp":%d}`+"\n", err, trader.ID, time.Now().UnixMilli())
			logFile.WriteString(logEntry)
			logFile.Close()
		}
		// #endregion
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update trader: %v", err)})
		return
	}
	log.Printf("✓ DEBUG [UpdateTrader]: Database update succeeded for trader %s, system_prompt_template: '%s'", traderID, systemPromptTemplate)
	// #region agent log
	logFile, _ = os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if logFile != nil {
		logEntry := fmt.Sprintf(`{"sessionId":"debug-session","runId":"run1","hypothesisId":"G","location":"api/server.go:1696","message":"Trader updated successfully","data":{"traderID":"%s","traderStrategyID":"%s"},"timestamp":%d}`+"\n", trader.ID, trader.StrategyID, time.Now().UnixMilli())
		logFile.WriteString(logEntry)
		logFile.Close()
	}
	// #endregion

	// #region agent log
	// Log before reloading trader
	logFile, _ = os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if logFile != nil {
		logData := map[string]interface{}{
			"location": "api/server.go:1805",
			"message": "About to reload trader after update",
			"data": map[string]interface{}{
				"trader_id": traderID,
				"user_id": userID,
			},
			"timestamp": time.Now().UnixMilli(),
			"sessionId": "debug-session",
			"runId": "run1",
			"hypothesisId": "C",
		}
		json.NewEncoder(logFile).Encode(logData)
		logFile.Close()
	}
	// #endregion

	// Reload trader into memory to ensure latest configuration (including UseTradingView) takes effect
	if reloadErr := s.traderManager.ReloadTraderFromDB(s.database, userID, traderID); reloadErr != nil {
		log.Printf("⚠️ Failed to reload trader into memory: %v", reloadErr)
		// #region agent log
		logFile2, _ := os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if logFile2 != nil {
			logData2 := map[string]interface{}{
				"location": "api/server.go:1820",
				"message": "Failed to reload trader",
				"data": map[string]interface{}{
					"trader_id": traderID,
					"error": reloadErr.Error(),
				},
				"timestamp": time.Now().UnixMilli(),
				"sessionId": "debug-session",
				"runId": "run1",
				"hypothesisId": "C",
			}
			json.NewEncoder(logFile2).Encode(logData2)
			logFile2.Close()
		}
		// #endregion
	} else {
		// #region agent log
		logFile3, _ := os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if logFile3 != nil {
			logData3 := map[string]interface{}{
				"location": "api/server.go:1840",
				"message": "Trader reloaded successfully",
				"data": map[string]interface{}{
					"trader_id": traderID,
				},
				"timestamp": time.Now().UnixMilli(),
				"sessionId": "debug-session",
				"runId": "run1",
				"hypothesisId": "C",
			}
			json.NewEncoder(logFile3).Encode(logData3)
			logFile3.Close()
		}
		// #endregion
	}

	log.Printf("INFO: Trader config reloaded into memory (trader=%s, UseTradingView=%v, scan_interval=%d)", traderID, req.UseTradingView, scanIntervalMinutes)

	// If was running before, restart to apply new configuration
	if wasRunning {
		reloaded, getErr := s.traderManager.GetTrader(traderID)
		if getErr != nil {
			log.Printf("ERROR: Trader missing after reload, cannot restart (trader=%s, err=%v)", traderID, getErr)
		} else {
			go func() {
				log.Printf("▶️  Restarting trader to apply new configuration %s (%s)", traderID, reloaded.GetName())
				if err := reloaded.Run(); err != nil {
					log.Printf("❌ Trader restart error %s: %v", reloaded.GetName(), err)
					_ = s.database.UpdateTraderStatus(userID, traderID, false)
				}
			}()
			time.Sleep(500 * time.Millisecond)
			st := reloaded.GetStatus()
			running, _ := st["is_running"].(bool)
			_ = s.database.UpdateTraderStatus(userID, traderID, running)
			log.Printf("INFO: Restart applied with new config (trader=%s, running=%v, UseTradingView=%v, scan_interval=%d)", traderID, running, req.UseTradingView, scanIntervalMinutes)
		}
	}

	log.Printf("✓ Trader updated successfully: %s (Model: %s, Exchange: %s)", req.Name, req.AIModelID, req.ExchangeID)

	c.JSON(http.StatusOK, gin.H{
		"trader_id":       traderID,
		"trader_name":     req.Name,
		"ai_model":        req.AIModelID,
		"message":         "Trader updated successfully",
		"use_tradingview": req.UseTradingView,
		"scan_interval":   scanIntervalMinutes,
		"restarted":       wasRunning,
	})
}

// handleDeleteTrader delete trader
func (s *Server) handleDeleteTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// Delete from database
	err := s.database.DeleteTrader(userID, traderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete trader: %v", err)})
		return
	}

	// If trader is running, stop it first
	if trader, err := s.traderManager.GetTrader(traderID); err == nil {
		status := trader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); ok && isRunning {
			trader.Stop()
			log.Printf("⏹  Stopped running trader: %s", traderID)
		}
		// Remove trader from memory
		s.traderManager.RemoveTrader(traderID)
	}

	log.Printf("✓ Trader deleted: %s", traderID)
	c.JSON(http.StatusOK, gin.H{"message": "Trader deleted"})
}

// handleStartTrader start trader
func (s *Server) handleStartTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	isAdmin := roleStr == "admin"
	traderID := c.Param("id")

	// If admin, need to get trader's owner userID
	var traderOwnerID string
	if isAdmin {
		allTraders, err := s.database.GetAllTraders()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trader list"})
			return
		}
		found := false
		for _, t := range allTraders {
			if t.ID == traderID {
				traderOwnerID = t.UserID
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
			return
		}
	} else {
		traderOwnerID = userID
	}

	// First get trader basic info, check if configuration needs fixing
	traders, _ := s.database.GetTraders(traderOwnerID)
	for _, t := range traders {
		if t.ID == traderID {
			// Check if fix needed: if ai_model_id is a prompt template name
			if isPromptTemplateName(t.AIModelID) {
				log.Printf("🔧 Detected trader configuration that needs fixing: traderID=%s, ai_model_id=%s (should be prompt template)", traderID, t.AIModelID)

				// Fix: map ai_model_id to system_prompt_template, use default AI model
				fixedAIModelID := s.getDefaultAIModelForUser(traderOwnerID)
				fixedSystemPromptTemplate := t.AIModelID

				// If system_prompt_template already exists, use it; otherwise use ai_model_id value
				if t.SystemPromptTemplate != "" {
					fixedSystemPromptTemplate = t.SystemPromptTemplate
				}

				log.Printf("🔧 Fixing trader configuration: ai_model_id: %s -> %s, system_prompt_template: %s",
					t.AIModelID, fixedAIModelID, fixedSystemPromptTemplate)

				// Update trader configuration
				t.AIModelID = fixedAIModelID
				t.SystemPromptTemplate = fixedSystemPromptTemplate
				if updateErr := s.database.UpdateTrader(t); updateErr != nil {
					log.Printf("⚠️ Failed to fix trader configuration: %v", updateErr)
				} else {
					log.Printf("✓ Trader configuration fixed")
				}
			}
			break
		}
	}

	// Verify trader belongs to current user (or admin) and get latest configuration
	traderCfg, aiModelCfg, exchangeCfg, err := s.database.GetTraderConfig(traderOwnerID, traderID)
	if err != nil {
		// Add detailed error log to help diagnose issues
		log.Printf("❌ [%s] GetTraderConfig failed: %v (traderID=%s, userID=%s)", userID, err, traderID, userID)

		// Try to get basic info to provide more detailed error message
		traders, _ := s.database.GetTraders(traderOwnerID)
		traderExists := false
		var missingConfig []string
		for _, t := range traders {
			if t.ID == traderID {
				traderExists = true
				log.Printf("⚠️ Trader exists but configuration incomplete: traderID=%s, ai_model_id=%s, exchange_id=%s", traderID, t.AIModelID, t.ExchangeID)

				// Check if AI model exists
				_, err := s.database.GetAIModel(traderOwnerID, t.AIModelID)
				if err != nil {
					missingConfig = append(missingConfig, fmt.Sprintf("AI model '%s'", t.AIModelID))
				}

				// Check if exchange exists
				_, err = s.database.GetExchangeByID(traderOwnerID, t.ExchangeID)
				if err != nil {
					missingConfig = append(missingConfig, fmt.Sprintf("Exchange '%s'", t.ExchangeID))
				}
				break
			}
		}

		if !traderExists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist or no access permission"})
		} else if len(missingConfig) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Trader configuration incomplete: missing %s. Please check configuration settings.", strings.Join(missingConfig, " and ")),
			})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist or no access permission"})
		}
		return
	}

	// Log successful config retrieval (for debugging)
	log.Printf("✓ [%s] Successfully retrieved trader configuration: traderID=%s, ai_model=%s, exchange=%s", userID, traderID, aiModelCfg.Name, exchangeCfg.Name)

	// Stop old instance if running BEFORE reload
	if oldTrader, err := s.traderManager.GetTrader(traderID); err == nil {
		status := oldTrader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); ok && isRunning {
			log.Printf("⚠️ Stopping running trader before reload (trader=%s)", traderID)
			oldTrader.Stop()
			time.Sleep(300 * time.Millisecond) // Wait for goroutine to stop
		}
	}

	// Always refresh memory instance with latest configuration (force reload single trader)
	if reloadErr := s.traderManager.ReloadTraderFromDB(s.database, traderOwnerID, traderID); reloadErr != nil {
		log.Printf("⚠️ Failed to reload trader, cannot start: %v", reloadErr)
		
		// Parse error to provide user-friendly message
		errMsg := reloadErr.Error()
		log.Printf("🔍 DEBUG: Error message for matching: %q", errMsg)
		
		// Check for Lighter-specific errors (case-insensitive and handle variations)
		errMsgLower := strings.ToLower(errMsg)
		if strings.Contains(errMsgLower, "lighter api key private key is required") || 
		   strings.Contains(errMsgLower, "lighter api key") && strings.Contains(errMsgLower, "required") {
			log.Printf("✅ Matched Lighter API key error")
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "LIGHTER initialization failed. Please check: 1) Wallet address is set, 2) API key private key is configured in Exchange Settings, 3) API key is generated from Lighter interface",
				"details": errMsg,
			})
			return
		}
		if strings.Contains(errMsgLower, "lighter wallet address is required") || 
		   strings.Contains(errMsgLower, "lighter wallet") && strings.Contains(errMsgLower, "required") {
			log.Printf("✅ Matched Lighter wallet address error")
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "LIGHTER initialization failed. Wallet address is required. Please configure it in Exchange Settings",
				"details": errMsg,
			})
			return
		}
		if strings.Contains(errMsgLower, "failed to initialize lighter") || 
		   strings.Contains(errMsgLower, "lighter initialization failed") {
			log.Printf("✅ Matched Lighter initialization error")
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "LIGHTER initialization failed. Please check: 1) Wallet address is correct, 2) API key private key is valid, 3) Network connectivity",
				"details": errMsg,
			})
			return
		}
		
		// Fallback for other errors
		log.Printf("⚠️ Error did not match any specific pattern, using generic error")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Unable to load latest trader configuration",
			"details": errMsg,
		})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	// Check if trader is already running
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trader is already running"})
		return
	}

	modeDesc := "Periodic scan mode"
	if traderCfg.UseTradingView {
		modeDesc = "TradingView webhook waiting mode"
	}
	log.Printf("INFO: Start requested with fresh config (trader=%s, UseTradingView=%v, scan_interval=%d, mode=%s)", traderID, traderCfg.UseTradingView, traderCfg.ScanIntervalMinutes, modeDesc)

	// Start trader
	go func() {
		log.Printf("▶️  Starting trader %s (%s)", traderID, trader.GetName())
		if err := trader.Run(); err != nil {
			log.Printf("❌ Trader %s runtime error: %v", trader.GetName(), err)
			// Update database status to stopped (start failed)
			_ = s.database.UpdateTraderStatus(traderOwnerID, traderID, false)
		}
	}()

	// Wait for trader to start and verify status
	time.Sleep(500 * time.Millisecond)
	status = trader.GetStatus()
	isRunning, _ := status["is_running"].(bool)

	// Update database status
	err = s.database.UpdateTraderStatus(traderOwnerID, traderID, isRunning)
	if err != nil {
		log.Printf("⚠️  Failed to update trader status: %v", err)
	}

	log.Printf("✓ Trader %s started (running status: %v)", trader.GetName(), isRunning)
	
	// Invalidate competition cache to ensure visibility state is reflected immediately
	s.traderManager.InvalidateCompetitionCache()
	
	c.JSON(http.StatusOK, gin.H{
		"message":         "Trader started",
		"is_running":      isRunning,
		"use_tradingview": traderCfg.UseTradingView,
		"scan_interval":   traderCfg.ScanIntervalMinutes,
	})
}

// handleStopTrader stop trader
func (s *Server) handleStopTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	isAdmin := roleStr == "admin"
	traderID := c.Param("id")

	// If admin, need to get trader's owner userID
	var traderOwnerID string
	if isAdmin {
		allTraders, err := s.database.GetAllTraders()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trader list"})
			return
		}
		found := false
		for _, t := range allTraders {
			if t.ID == traderID {
				traderOwnerID = t.UserID
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
			return
		}
	} else {
		traderOwnerID = userID
	}

	// Verify trader exists (doesn't require complete configuration, just confirm trader belongs to owner)
	traders, err := s.database.GetTraders(traderOwnerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trader list"})
		return
	}

	traderExists := false
	for _, t := range traders {
		if t.ID == traderID {
			traderExists = true
			break
		}
	}

	if !traderExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist or no access permission"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	// Check if trader is running
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && !isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trader already stopped"})
		return
	}

	// Stop trader
	trader.Stop()

	// Wait for trader to stop and verify status
	time.Sleep(200 * time.Millisecond)
	status = trader.GetStatus()
	isRunning, _ := status["is_running"].(bool)

	// Explicitly set database status to false (user explicitly stopped)
	// Regardless of memory state, database should reflect user's action
	err = s.database.UpdateTraderStatus(traderOwnerID, traderID, false)
	if err != nil {
		log.Printf("⚠️  Failed to update trader status: %v", err)
	}

	log.Printf("⏹  Trader %s stopped (memory status: %v, database status: false)", trader.GetName(), isRunning)
	
	// Invalidate competition cache to ensure visibility state is reflected immediately
	s.traderManager.InvalidateCompetitionCache()
	
	c.JSON(http.StatusOK, gin.H{
		"message":    "Trader stopped",
		"is_running": false,
	})
}

// handleUpdateTraderPrompt update trader custom prompt
func (s *Server) handleUpdateTraderPrompt(c *gin.Context) {
	traderID := c.Param("id")
	userID := c.GetString("user_id")

	var req struct {
		CustomPrompt       string `json:"custom_prompt"`
		OverrideBasePrompt bool   `json:"override_base_prompt"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update database
	err := s.database.UpdateTraderCustomPrompt(userID, traderID, req.CustomPrompt, req.OverrideBasePrompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update custom prompt: %v", err)})
		return
	}

	// If trader is in memory, update its custom prompt and override settings
	trader, err := s.traderManager.GetTrader(traderID)
	if err == nil {
		trader.SetCustomPrompt(req.CustomPrompt)
		trader.SetOverrideBasePrompt(req.OverrideBasePrompt)
		log.Printf("✓ Updated trader %s custom prompt (override base=%v)", trader.GetName(), req.OverrideBasePrompt)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Custom prompt updated"})
}

// handleToggleCompetition Toggle trader competition visibility
func (s *Server) handleToggleCompetition(c *gin.Context) {
	traderID := c.Param("id")
	userID := c.GetString("user_id")

	var req struct {
		ShowInCompetition bool `json:"show_in_competition"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update database
	err := s.database.UpdateTraderShowInCompetition(userID, traderID, req.ShowInCompetition)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update competition visibility: %v", err)})
		return
	}

	// Update in-memory trader if it exists
	if trader, err := s.traderManager.GetTrader(traderID); err == nil {
		trader.SetShowInCompetition(req.ShowInCompetition)
	}

	status := "shown"
	if !req.ShowInCompetition {
		status = "hidden"
	}
	log.Printf("✓ Trader %s competition visibility updated: %s", traderID, status)
	c.JSON(http.StatusOK, gin.H{
		"message":             "Competition visibility updated",
		"show_in_competition": req.ShowInCompetition,
	})
}

// handleSyncBalance sync exchange balance to initial_balance (Option B: manual sync + Option C: smart detection)
func (s *Server) handleSyncBalance(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	log.Printf("🔄 User %s requested balance sync for trader %s", userID, traderID)

	// Get trader configuration from database (includes exchange info)
	traderConfig, _, exchangeCfg, err := s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	if exchangeCfg == nil || !exchangeCfg.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Exchange not configured or not enabled"})
		return
	}

	// Create temporary trader to query balance
	var tempTrader trader.Trader
	var createErr error

	switch traderConfig.ExchangeID {
	case "binance":
		tempTrader = trader.NewFuturesTrader(exchangeCfg.APIKey, exchangeCfg.SecretKey, userID)
	case "hyperliquid":
		tempTrader, createErr = trader.NewHyperliquidTrader(
			exchangeCfg.APIKey,
			exchangeCfg.HyperliquidWalletAddr,
			exchangeCfg.Testnet,
		)
	case "aster":
		// Debug logging for Aster configuration
		log.Printf("🔍 [Aster] Syncing balance with config: user=%s (len=%d), signer=%s (len=%d), privateKey=*** (len=%d)",
			exchangeCfg.AsterUser, len(exchangeCfg.AsterUser),
			exchangeCfg.AsterSigner, len(exchangeCfg.AsterSigner),
			len(exchangeCfg.AsterPrivateKey))
		tempTrader, createErr = trader.NewAsterTrader(
			exchangeCfg.AsterUser,
			exchangeCfg.AsterSigner,
			exchangeCfg.AsterPrivateKey,
		)
	case "bybit":
		tempTrader = trader.NewBybitTrader(
			exchangeCfg.APIKey,
			exchangeCfg.SecretKey,
		)
	case "lighter":
		log.Printf("🔍 [Lighter] Creating trader with config: walletAddr=%s (len=%d), apiKeyPrivateKey=*** (len=%d), apiKeyIndex=%d",
			MaskWalletAddress(exchangeCfg.LighterWalletAddr), len(exchangeCfg.LighterWalletAddr),
			len(exchangeCfg.LighterAPIKeyPrivateKey), exchangeCfg.LighterAPIKeyIndex)
		tempTrader, createErr = trader.NewLighterTraderV2(
			exchangeCfg.LighterWalletAddr,
			exchangeCfg.LighterAPIKeyPrivateKey,
			exchangeCfg.LighterAPIKeyIndex,
			exchangeCfg.Testnet,
		)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported exchange type"})
		return
	}

	if createErr != nil {
		log.Printf("⚠️ Failed to create temporary trader: %v", createErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to connect to exchange: %v", createErr)})
		return
	}

	// Query actual balance
	balanceInfo, balanceErr := tempTrader.GetBalance()
	if balanceErr != nil {
		log.Printf("⚠️ Failed to query exchange balance: %v", balanceErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to query balance: %v", balanceErr)})
		return
	}

	// Extract available balance - prioritize available_balance over total_equity
	// Use available_balance (funds available for trading) instead of total_equity (includes unrealized PnL)
	var actualBalance float64
	if availableBalance, ok := balanceInfo["available_balance"].(float64); ok && availableBalance > 0 {
		actualBalance = availableBalance
	} else if availableBalance, ok := balanceInfo["availableBalance"].(float64); ok && availableBalance > 0 {
		actualBalance = availableBalance
	} else if totalBalance, ok := balanceInfo["balance"].(float64); ok && totalBalance > 0 {
		actualBalance = totalBalance
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to get available balance"})
		return
	}

	oldBalance := traderConfig.InitialBalance

	// ✅ Option C: Smart detection of balance changes
	changePercent := ((actualBalance - oldBalance) / oldBalance) * 100
	changeType := "increase"
	if changePercent < 0 {
		changeType = "decrease"
	}

	log.Printf("✓ Queried actual exchange balance: %.2f USDT (current config: %.2f USDT, change: %.2f%%)",
		actualBalance, oldBalance, changePercent)

	// Update initial_balance in database
	err = s.database.UpdateTraderInitialBalance(userID, traderID, actualBalance)
	if err != nil {
		log.Printf("❌ Failed to update initial_balance: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update balance"})
		return
	}

	// Reload traders into memory
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ Failed to reload user traders into memory: %v", err)
	}

	log.Printf("✅ Balance synced: %.2f → %.2f USDT (%s %.2f%%)", oldBalance, actualBalance, changeType, changePercent)

	c.JSON(http.StatusOK, gin.H{
		"message":        "Balance sync successful",
		"old_balance":    oldBalance,
		"new_balance":    actualBalance,
		"change_percent": changePercent,
		"change_type":    changeType,
	})
}

// handleGetModelConfigs get AI model configurations
func (s *Server) handleGetModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	userRole := c.GetString("role")
	log.Printf("🔍 Querying AI model configurations for user %s (role: %s)", userID, userRole)
	models, err := s.database.GetAIModels(userID)
	if err != nil {
		log.Printf("❌ Failed to get AI model configurations: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get AI model configurations: %v", err)})
		return
	}
	log.Printf("✅ Found %d AI model configurations", len(models))

	// If database is empty, return default models
	if len(models) == 0 {
		log.Printf("📋 Database is empty, returning default models")
		defaultModels := []SafeModelConfig{
			{ID: "deepseek", Name: "DeepSeek", Provider: "deepseek", Enabled: false, CustomAPIURL: "", CustomModelName: ""},
			{ID: "qwen", Name: "Qwen", Provider: "qwen", Enabled: false, CustomAPIURL: "", CustomModelName: ""},
			{ID: "openai", Name: "OpenAI (GPT)", Provider: "openai", Enabled: false, CustomAPIURL: "", CustomModelName: ""},
			{ID: "claude", Name: "Claude", Provider: "claude", Enabled: false, CustomAPIURL: "", CustomModelName: ""},
			{ID: "gemini", Name: "Gemini", Provider: "gemini", Enabled: false, CustomAPIURL: "", CustomModelName: ""},
			{ID: "grok", Name: "Grok", Provider: "grok", Enabled: false, CustomAPIURL: "", CustomModelName: ""},
			{ID: "kimi", Name: "Kimi", Provider: "kimi", Enabled: false, CustomAPIURL: "", CustomModelName: ""},
		}
		c.JSON(http.StatusOK, defaultModels)
		return
	}

	// Convert to safe response structure, remove sensitive information
	safeModels := make([]SafeModelConfig, len(models))
	for i, model := range models {
		safeModels[i] = SafeModelConfig{
			ID:              model.ID,
			Name:            model.Name,
			Provider:        model.Provider,
			Enabled:         model.Enabled,
			CustomAPIURL:    model.CustomAPIURL,
			CustomModelName: model.CustomModelName,
		}
	}

	c.JSON(http.StatusOK, safeModels)
}

// handleUpdateModelConfigs update AI model configurations (encrypted data only when TRANSPORT_ENCRYPTION=true)
func (s *Server) handleUpdateModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")

	// Read raw request body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Check if transport encryption is enabled
	transportEncryptionEnabled := strings.ToLower(os.Getenv("TRANSPORT_ENCRYPTION")) == "true"

	var req UpdateModelConfigRequest

	if transportEncryptionEnabled {
		// Parse encrypted payload
		var encryptedPayload crypto.EncryptedPayload
		if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil {
			log.Printf("❌ Failed to parse encrypted payload: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Request format error, encrypted transmission required"})
			return
		}

		// Verify if encrypted data
		if encryptedPayload.WrappedKey == "" {
			log.Printf("❌ Detected non-encrypted request (UserID: %s)", userID)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "This endpoint only supports encrypted transmission, please use encrypted client",
				"code":    "ENCRYPTION_REQUIRED",
				"message": "Encrypted transmission is required for security reasons",
			})
			return
		}

		// Decrypt data
		decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
		if err != nil {
			log.Printf("❌ Failed to decrypt model configuration (UserID: %s): %v", userID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decrypt data"})
			return
		}

		// Parse decrypted data
		if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
			log.Printf("❌ Failed to parse decrypted data: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
			return
		}
		log.Printf("🔓 Decrypted model configuration data (UserID: %s)", userID)
	} else {
		// Transport encryption disabled, accept plain JSON
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			log.Printf("❌ Failed to parse request data: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse request data"})
			return
		}
		log.Printf("📝 Received plain model configuration data (UserID: %s, transport encryption disabled)", userID)
	}

	// Update each model's configuration
	for modelID, modelData := range req.Models {
		err := s.database.UpdateAIModel(userID, modelID, modelData.Enabled, modelData.APIKey, modelData.CustomAPIURL, modelData.CustomModelName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update model %s: %v", modelID, err)})
			return
		}
	}

	// Reload all traders for this user to make new configuration take effect immediately
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ Failed to reload user traders into memory: %v", err)
		// Don't return error here, as model configuration has been successfully updated to database
	}

	log.Printf("✓ AI model configuration updated: %+v", req.Models)
	c.JSON(http.StatusOK, gin.H{"message": "Model configuration updated"})
}

// handleGetExchangeConfigs get exchange configurations
func (s *Server) handleGetExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	userRole := c.GetString("role")
	log.Printf("🔍 Querying exchange configurations for user %s (role: %s)", userID, userRole)
	exchanges, err := s.database.GetExchanges(userID)
	if err != nil {
		log.Printf("❌ Failed to get exchange configurations: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get exchange configurations: %v", err)})
		return
	}
	log.Printf("✅ Found %d exchange configurations", len(exchanges))
	

	// If database is empty, return default exchanges
	if len(exchanges) == 0 {
		log.Printf("📋 Database is empty, returning default exchanges")
		defaultExchanges := []SafeExchangeConfig{
			{ID: "binance", Name: "Binance Futures", Type: "binance", Enabled: false, Testnet: false, HyperliquidWalletAddr: "", AsterUser: "", AsterSigner: "", LighterWalletAddr: ""},
			{ID: "bybit", Name: "Bybit Futures", Type: "bybit", Enabled: false, Testnet: false, HyperliquidWalletAddr: "", AsterUser: "", AsterSigner: "", LighterWalletAddr: ""},
			{ID: "okx", Name: "OKX Futures", Type: "okx", Enabled: false, Testnet: false, HyperliquidWalletAddr: "", AsterUser: "", AsterSigner: "", LighterWalletAddr: ""},
			{ID: "bitget", Name: "Bitget Futures", Type: "bitget", Enabled: false, Testnet: false, HyperliquidWalletAddr: "", AsterUser: "", AsterSigner: "", LighterWalletAddr: ""},
			{ID: "hyperliquid", Name: "Hyperliquid", Type: "hyperliquid", Enabled: false, Testnet: false, HyperliquidWalletAddr: "", AsterUser: "", AsterSigner: "", LighterWalletAddr: ""},
			{ID: "aster", Name: "Aster DEX", Type: "aster", Enabled: false, Testnet: false, HyperliquidWalletAddr: "", AsterUser: "", AsterSigner: "", LighterWalletAddr: ""},
			{ID: "lighter", Name: "Lighter DEX", Type: "lighter", Enabled: false, Testnet: false, HyperliquidWalletAddr: "", AsterUser: "", AsterSigner: "", LighterWalletAddr: ""},
		}
		c.JSON(http.StatusOK, defaultExchanges)
		return
	}

	// Debug: output configuration details (masked)
	for _, ex := range exchanges {
		apiKeyMasked := ""
		if len(ex.APIKey) > 8 {
			apiKeyMasked = ex.APIKey[:8] + "..."
		}
		secretKeyMasked := ""
		if len(ex.SecretKey) > 8 {
			secretKeyMasked = ex.SecretKey[:8] + "..."
		}
		log.Printf("   └─ Exchange: %s, APIKey: %s, SecretKey: %s", ex.ID, apiKeyMasked, secretKeyMasked)
		
		// Additional debug for Lighter exchange
		if ex.ID == "lighter" {
			lighterAPIKeyMasked := ""
			if len(ex.LighterAPIKeyPrivateKey) > 8 {
				lighterAPIKeyMasked = ex.LighterAPIKeyPrivateKey[:8] + "..."
			} else if ex.LighterAPIKeyPrivateKey == "" {
				lighterAPIKeyMasked = "EMPTY"
			}
			log.Printf("      └─ Lighter specific: WalletAddr=%s, APIKeyPrivateKey: %s (length=%d), APIKeyIndex=%d",
				MaskWalletAddress(ex.LighterWalletAddr), lighterAPIKeyMasked, len(ex.LighterAPIKeyPrivateKey), ex.LighterAPIKeyIndex)
		}
	}

	// Convert to safe response structure, remove sensitive information
	safeExchanges := make([]SafeExchangeConfig, len(exchanges))
	for i, exchange := range exchanges {
		safeExchanges[i] = SafeExchangeConfig{
			ID:                    exchange.ID,
			Name:                  exchange.Name,
			Type:                  exchange.Type,
			Enabled:               exchange.Enabled,
			Testnet:               exchange.Testnet,
			HyperliquidWalletAddr: exchange.HyperliquidWalletAddr,
			AsterUser:             exchange.AsterUser,
			AsterSigner:           exchange.AsterSigner,
			LighterWalletAddr:     exchange.LighterWalletAddr,
		}
	}

	c.JSON(http.StatusOK, safeExchanges)
}

// handleUpdateExchangeConfigs update exchange configurations (encrypted data only when TRANSPORT_ENCRYPTION=true)
func (s *Server) handleUpdateExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")

	// Read raw request body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Check if transport encryption is enabled
	transportEncryptionEnabled := strings.ToLower(os.Getenv("TRANSPORT_ENCRYPTION")) == "true"

	var req UpdateExchangeConfigRequest

	if transportEncryptionEnabled {
		// Parse encrypted payload
		var encryptedPayload crypto.EncryptedPayload
		if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil {
			log.Printf("❌ Failed to parse encrypted payload: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Request format error, encrypted transmission required"})
			return
		}

		// Verify if encrypted data
		if encryptedPayload.WrappedKey == "" {
			log.Printf("❌ Detected non-encrypted request (UserID: %s)", userID)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "This endpoint only supports encrypted transmission, please use encrypted client",
				"code":    "ENCRYPTION_REQUIRED",
				"message": "Encrypted transmission is required for security reasons",
			})
			return
		}

		// Decrypt data
		decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
		if err != nil {
			log.Printf("❌ Failed to decrypt exchange configuration (UserID: %s): %v", userID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decrypt data"})
			return
		}

		// Parse decrypted data
		if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
			log.Printf("❌ Failed to parse decrypted data: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
			return
		}
		log.Printf("🔓 Decrypted exchange configuration data (UserID: %s)", userID)
	} else {
		// Transport encryption disabled, accept plain JSON
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			log.Printf("❌ Failed to parse request data: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse request data"})
			return
		}
		log.Printf("📝 Received plain exchange configuration data (UserID: %s, transport encryption disabled)", userID)
	}

	// Update each exchange's configuration
	for exchangeID, exchangeData := range req.Exchanges {
		err := s.database.UpdateExchange(userID, exchangeID, exchangeData.Enabled, exchangeData.APIKey, exchangeData.SecretKey, exchangeData.Testnet, exchangeData.HyperliquidWalletAddr, exchangeData.AsterUser, exchangeData.AsterSigner, exchangeData.AsterPrivateKey, exchangeData.LighterWalletAddr, exchangeData.LighterPrivateKey, exchangeData.LighterAPIKeyPrivateKey, exchangeData.LighterAPIKeyIndex, exchangeData.OkxPassphrase)
		if err != nil {
			// Provide more helpful error message
			errorMsg := err.Error()
			if strings.Contains(errorMsg, "migration") || strings.Contains(errorMsg, "UNIQUE constraint") {
				errorMsg = fmt.Sprintf("Database migration required: %v. Please restart the server to complete migration, or contact support if the issue persists.", err)
			}
			log.Printf("❌ Failed to update exchange %s for user %s: %v", exchangeID, userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update exchange %s: %s", exchangeID, errorMsg)})
			return
		}
	}

	// Reload all traders for this user to make new configuration take effect immediately
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ Failed to reload user traders into memory: %v", err)
		// Don't return error here, as exchange configuration has been successfully updated to database
	}

	log.Printf("✓ Exchange configuration updated: %+v", req.Exchanges)
	c.JSON(http.StatusOK, gin.H{"message": "Exchange configuration updated"})
}

// handleGetUserSignalSource get user signal source configuration
func (s *Server) handleGetUserSignalSource(c *gin.Context) {
	userID := c.GetString("user_id")
	source, err := s.database.GetUserSignalSource(userID)
	if err != nil {
		// If configuration doesn't exist, return empty configuration instead of 404 error
		c.JSON(http.StatusOK, gin.H{
			"coin_pool_url": "",
			"oi_top_url":    "",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"coin_pool_url": source.CoinPoolURL,
		"oi_top_url":    source.OITopURL,
	})
}

// handleSaveUserSignalSource save user signal source configuration
func (s *Server) handleSaveUserSignalSource(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		CoinPoolURL string `json:"coin_pool_url"`
		OITopURL    string `json:"oi_top_url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := s.database.CreateUserSignalSource(userID, req.CoinPoolURL, req.OITopURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save user signal source configuration: %v", err)})
		return
	}

	log.Printf("✓ User signal source configuration saved: user=%s, coin_pool=%s, oi_top=%s", userID, req.CoinPoolURL, req.OITopURL)
	c.JSON(http.StatusOK, gin.H{"message": "User signal source configuration saved"})
}

// handleTraderList trader list
func (s *Server) handleTraderList(c *gin.Context) {
	userID := c.GetString("user_id")
	traders, err := s.database.GetTraders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get trader list: %v", err)})
		return
	}

	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		// Get real-time running status
		isRunning := trader.IsRunning
		if at, err := s.traderManager.GetTrader(trader.ID); err == nil {
			status := at.GetStatus()
			if running, ok := status["is_running"].(bool); ok {
				isRunning = running
			}
		}

		// Return complete AIModelID (e.g., "admin_deepseek"), don't truncate
		// Frontend needs complete ID to verify if model exists (consistent with handleGetTraderConfig)
		result = append(result, map[string]interface{}{
			"trader_id":           trader.ID,
			"trader_name":         trader.Name,
			"ai_model":            trader.AIModelID, // Use complete ID
			"exchange_id":         trader.ExchangeID,
			"is_running":          isRunning,
			"show_in_competition": trader.ShowInCompetition,
			"initial_balance":     trader.InitialBalance,
		})
	}

	c.JSON(http.StatusOK, result)
}

// handleGetAllTraders get all traders (admin only)
func (s *Server) handleGetAllTraders(c *gin.Context) {
	traders, err := s.database.GetAllTraders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get trader list: %v", err)})
		return
	}

	// Get all user information to return user email
	users, err := s.database.GetAllUsersWithRoles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get user list: %v", err)})
		return
	}
	userMap := make(map[string]string) // userID -> email
	for _, u := range users {
		userMap[u.ID] = u.Email
	}

	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		// Get real-time running status
		isRunning := trader.IsRunning
		if at, err := s.traderManager.GetTrader(trader.ID); err == nil {
			status := at.GetStatus()
			if running, ok := status["is_running"].(bool); ok {
				isRunning = running
			}
		}

		result = append(result, map[string]interface{}{
			"trader_id":       trader.ID,
			"trader_name":     trader.Name,
			"user_id":         trader.UserID,
			"user_email":      userMap[trader.UserID],
			"ai_model":        trader.AIModelID,
			"exchange_id":     trader.ExchangeID,
			"is_running":      isRunning,
			"initial_balance": trader.InitialBalance,
		})
	}

	c.JSON(http.StatusOK, result)
}

// handleGetCurrentUser get current authenticated user information (for refreshing role, etc.)
func (s *Server) handleGetCurrentUser(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get user information: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":    user.ID,
		"email": user.Email,
		"role":  user.Role,
	})
}

// handleGetAllUsers get all users (admin only)
func (s *Server) handleGetAllUsers(c *gin.Context) {
	users, err := s.database.GetAllUsersWithRoles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get user list: %v", err)})
		return
	}

	result := make([]map[string]interface{}, 0, len(users))
	for _, user := range users {
		result = append(result, map[string]interface{}{
			"id":         user.ID,
			"email":      user.Email,
			"role":       user.Role,
			"created_at": user.CreatedAt.Format(time.RFC3339), // ISO 8601 format for JavaScript compatibility
		})
	}

	c.JSON(http.StatusOK, result)
}

// handleUpdateUserRole update user role (admin only)
func (s *Server) handleUpdateUserRole(c *gin.Context) {
	userID := c.Param("id")
	var req struct {
		Role string `json:"role"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate role value
	validRoles := map[string]bool{"user": true, "follower": true, "admin": true}
	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid role value: %s", req.Role)})
		return
	}

	err := s.database.UpdateUserRole(userID, req.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update user role: %v", err)})
		return
	}

	log.Printf("✓ Admin updated user role: userID=%s, role=%s", userID, req.Role)
	c.JSON(http.StatusOK, gin.H{"message": "User role updated"})
}

// CreateTraderApplicationRequest create trader application request
type CreateTraderApplicationRequest struct {
	Name              string            `json:"name" binding:"required"`
	Email             string            `json:"email" binding:"required,email"`
	Description       string            `json:"description" binding:"required"`
	TradingExperience string            `json:"trading_experience" binding:"required"`
	StrategyOverview  string            `json:"strategy_overview" binding:"required"`
	SocialLinks       map[string]string `json:"social_links"`
}

// handleCreateTraderApplication create trader application
func (s *Server) handleCreateTraderApplication(c *gin.Context) {
	userID := c.GetString("user_id")
	var req CreateTraderApplicationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already has a pending application
	existingApp, err := s.database.GetTraderApplicationByUserID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query application"})
		return
	}
	if existingApp != nil && existingApp.Status == "pending" {
		c.JSON(http.StatusConflict, gin.H{"error": "You already have a pending application"})
		return
	}

	// Convert social links to JSON string
	socialLinksJSON := "{}"
	if len(req.SocialLinks) > 0 {
		linksBytes, err := json.Marshal(req.SocialLinks)
		if err == nil {
			socialLinksJSON = string(linksBytes)
		}
	}

	// Create application
	appID := uuid.New().String()
	app := &config.TraderApplication{
		ID:                appID,
		UserID:            userID,
		Name:              req.Name,
		Email:             req.Email,
		Description:       req.Description,
		TradingExperience: req.TradingExperience,
		StrategyOverview:  req.StrategyOverview,
		SocialLinks:       socialLinksJSON,
		Status:            "pending",
		AdminNotes:        "",
	}

	err = s.database.CreateTraderApplication(app)
	if err != nil {
		log.Printf("❌ Failed to create trader application: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create application"})
		return
	}

	log.Printf("✓ Trader application created successfully: userID=%s, appID=%s", userID, appID)
	c.JSON(http.StatusCreated, gin.H{
		"id":      appID,
		"status":  "pending",
		"message": "Application submitted, waiting for admin review",
	})
}

// handleGetMyTraderApplication get current user's trader application
func (s *Server) handleGetMyTraderApplication(c *gin.Context) {
	userID := c.GetString("user_id")

	app, err := s.database.GetTraderApplicationByUserID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query application"})
		return
	}

	if app == nil {
		c.JSON(http.StatusOK, nil)
		return
	}

	// Parse social links
	var socialLinks map[string]string
	if app.SocialLinks != "" {
		json.Unmarshal([]byte(app.SocialLinks), &socialLinks)
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                 app.ID,
		"user_id":            app.UserID,
		"name":               app.Name,
		"email":              app.Email,
		"description":        app.Description,
		"trading_experience": app.TradingExperience,
		"strategy_overview":  app.StrategyOverview,
		"social_links":       socialLinks,
		"status":             app.Status,
		"admin_notes":        app.AdminNotes,
		"created_at":         app.CreatedAt.Format(time.RFC3339),
		"updated_at":         app.UpdatedAt.Format(time.RFC3339),
	})
}

// handleGetAllTraderApplications get all trader applications (admin only)
func (s *Server) handleGetAllTraderApplications(c *gin.Context) {
	applications, err := s.database.GetAllTraderApplications()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get application list"})
		return
	}

	result := make([]map[string]interface{}, 0)
	for _, app := range applications {
		// Get user information
		user, err := s.database.GetUserByID(app.UserID)
		if err != nil {
			log.Printf("⚠️ Failed to get user information: userID=%s, error=%v", app.UserID, err)
			continue
		}

		// Parse social links
		var socialLinks map[string]string
		if app.SocialLinks != "" {
			json.Unmarshal([]byte(app.SocialLinks), &socialLinks)
		}

		result = append(result, map[string]interface{}{
			"id":                 app.ID,
			"user_id":            app.UserID,
			"user_email":         user.Email,
			"name":               app.Name,
			"email":              app.Email,
			"description":        app.Description,
			"trading_experience": app.TradingExperience,
			"strategy_overview":  app.StrategyOverview,
			"social_links":       socialLinks,
			"status":             app.Status,
			"admin_notes":        app.AdminNotes,
			"created_at":         app.CreatedAt.Format(time.RFC3339),
			"updated_at":         app.UpdatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, result)
}

// handleApproveTraderApplication approve trader application (admin only)
func (s *Server) handleApproveTraderApplication(c *gin.Context) {
	appID := c.Param("id")
	var req struct {
		AdminNotes string `json:"admin_notes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		// Admin notes are optional, so we don't fail if not provided
		req.AdminNotes = ""
	}

	// Get application information
	app, err := s.database.GetTraderApplicationByID(appID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application does not exist"})
		return
	}

	if app.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Application status does not allow this operation"})
		return
	}

	// Update application status
	err = s.database.UpdateTraderApplicationStatus(appID, "approved", req.AdminNotes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update application status"})
		return
	}

	// Upgrade user role to "user"
	err = s.database.UpdateUserRole(app.UserID, "user")
	if err != nil {
		log.Printf("⚠️ Failed to upgrade user role: userID=%s, error=%v", app.UserID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upgrade user role"})
		return
	}

	log.Printf("✓ Admin approved trader application: appID=%s, userID=%s", appID, app.UserID)
	c.JSON(http.StatusOK, gin.H{"message": "Application approved, user role upgraded"})
}

// handleRejectTraderApplication reject trader application (admin only)
func (s *Server) handleRejectTraderApplication(c *gin.Context) {
	appID := c.Param("id")
	var req struct {
		AdminNotes string `json:"admin_notes" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rejection reason must be provided"})
		return
	}

	// Get application information
	app, err := s.database.GetTraderApplicationByID(appID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application does not exist"})
		return
	}

	if app.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Application status does not allow this operation"})
		return
	}

	// Update application status
	err = s.database.UpdateTraderApplicationStatus(appID, "rejected", req.AdminNotes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update application status"})
		return
	}

	log.Printf("✓ Admin rejected trader application: appID=%s, userID=%s", appID, app.UserID)
	c.JSON(http.StatusOK, gin.H{"message": "Application rejected"})
}

// handleRunningTraders get all running traders (for follower to select signal source)
func (s *Server) handleRunningTraders(c *gin.Context) {
	// Get all users
	userIDs, err := s.database.GetAllUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get user list: %v", err)})
		return
	}

	result := make([]map[string]interface{}, 0)

	// Iterate through all users, get their running traders
	for _, userID := range userIDs {
		// Get user information
		user, err := s.database.GetUserByID(userID)
		if err != nil {
			continue // Skip users that cannot be retrieved
		}

		// Skip follower users' traders, only return "user" role traders
		if user.Role == "follower" {
			continue
		}

		// Get all traders for this user
		traders, err := s.database.GetTraders(userID)
		if err != nil {
			continue // Skip users that cannot get traders
		}

		// Filter out running traders (exclude follower traders, only return parent traders)
		for _, trader := range traders {
			// Skip follower traders, only return parent traders (traders without followed_trader_id)
			if trader.FollowedTraderID != "" {
				continue
			}

			// Check running status in database
			isRunning := trader.IsRunning

			// Check real-time running status in memory
			if at, err := s.traderManager.GetTrader(trader.ID); err == nil {
				status := at.GetStatus()
				if running, ok := status["is_running"].(bool); ok {
					isRunning = running
				}
			}

			// Only return running traders
			if isRunning {
				result = append(result, map[string]interface{}{
					"trader_id":   trader.ID,
					"trader_name": trader.Name,
					"user_id":     userID,
					"user_email":  user.Email,
				})
			}
		}
	}

	c.JSON(http.StatusOK, result)
}

// handleGetTraderConfig get trader detailed configuration
func (s *Server) handleGetTraderConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trader ID cannot be empty"})
		return
	}

	traderConfig, _, _, err := s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Failed to get trader configuration: %v", err)})
		return
	}

	// Load strategy settings if strategy_id exists (pure reference model)
	if traderConfig.StrategyID != "" {
		strategy, err := s.database.GetStrategy(traderConfig.StrategyID, userID)
		if err == nil {
			// Use strategy settings directly - trader record only stores trader-specific fields
			traderConfig.SystemPromptTemplate = strategy.SystemPromptTemplate
			traderConfig.CustomPrompt = strategy.CustomPrompt
			traderConfig.OverrideBasePrompt = strategy.OverrideBasePrompt
			traderConfig.BTCETHLeverage = strategy.BTCETHLeverage
			traderConfig.AltcoinLeverage = strategy.AltcoinLeverage
			traderConfig.TradingSymbols = strategy.TradingSymbols
			traderConfig.IsCrossMargin = strategy.IsCrossMargin
			traderConfig.UseCoinPool = strategy.UseCoinPool
			traderConfig.UseOITop = strategy.UseOITop
			traderConfig.UseTradingView = strategy.UseTradingView
			traderConfig.EnableRawKlines = strategy.EnableRawKlines
			traderConfig.EnableEMA = strategy.EnableEMA
			traderConfig.EnableMACD = strategy.EnableMACD
			traderConfig.EnableRSI = strategy.EnableRSI
			traderConfig.EnableATR = strategy.EnableATR
			traderConfig.EnableVolume = strategy.EnableVolume
			traderConfig.EnableOI = strategy.EnableOI
			traderConfig.EnableFunding = strategy.EnableFunding
			traderConfig.IndicatorTimeframe = strategy.IndicatorTimeframe
			traderConfig.QuantDataURL = strategy.QuantDataURL
			log.Printf("✓ Loaded strategy %s settings for trader %s (pure reference mode)", traderConfig.StrategyID, traderID)
		} else {
			log.Printf("⚠️ Failed to load strategy %s for trader %s: %v, using trader's stored settings", traderConfig.StrategyID, traderID, err)
			// Fallback: if strategy fails to load, use trader's stored settings (for backward compatibility)
		}
	}

	// Get real-time running status
	isRunning := traderConfig.IsRunning
	if at, err := s.traderManager.GetTrader(traderID); err == nil {
		status := at.GetStatus()
		if running, ok := status["is_running"].(bool); ok {
			isRunning = running
		}
	}

	// Return complete model ID, no conversion, keep consistent with frontend model list
	aiModelID := traderConfig.AIModelID

	// Ensure system_prompt_template always has a value
	systemPromptTemplate := traderConfig.SystemPromptTemplate
	if systemPromptTemplate == "" {
		systemPromptTemplate = "default" // Default to "default" if empty
	}

	result := map[string]interface{}{
		"trader_id":              traderConfig.ID,
		"trader_name":            traderConfig.Name,
		"ai_model":               aiModelID,
		"exchange_id":            traderConfig.ExchangeID,
		"initial_balance":        traderConfig.InitialBalance,
		"scan_interval_minutes":  traderConfig.ScanIntervalMinutes,
		"btc_eth_leverage":       traderConfig.BTCETHLeverage,
		"altcoin_leverage":       traderConfig.AltcoinLeverage,
		"trading_symbols":        traderConfig.TradingSymbols,
		"custom_prompt":          traderConfig.CustomPrompt,
		"override_base_prompt":   traderConfig.OverrideBasePrompt,
		"system_prompt_template": systemPromptTemplate,
		"is_cross_margin":        traderConfig.IsCrossMargin,
		"use_coin_pool":          traderConfig.UseCoinPool,
		"use_oi_top":             traderConfig.UseOITop,
		"use_tradingview":        traderConfig.UseTradingView,
		"followed_trader_id":     traderConfig.FollowedTraderID,
		"strategy_id":            traderConfig.StrategyID,
		"is_running":             isRunning,
		// Indicator configuration
		"enable_raw_klines":   traderConfig.EnableRawKlines,
		"enable_ema":          traderConfig.EnableEMA,
		"enable_macd":         traderConfig.EnableMACD,
		"enable_rsi":          traderConfig.EnableRSI,
		"enable_atr":          traderConfig.EnableATR,
		"enable_volume":       traderConfig.EnableVolume,
		"enable_oi":           traderConfig.EnableOI,
		"enable_funding":      traderConfig.EnableFunding,
		"indicator_timeframe": traderConfig.IndicatorTimeframe,
		"quant_data_url":      traderConfig.QuantDataURL,
	}

	log.Printf("🔍 DEBUG [handleGetTraderConfig]: Returning trader config - trader_id: %s, strategy_id: '%s', system_prompt_template: '%s' (original: '%s')", traderConfig.ID, traderConfig.StrategyID, systemPromptTemplate, traderConfig.SystemPromptTemplate)

	c.JSON(http.StatusOK, result)
}

// handleStatus system status
func (s *Server) handleStatus(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	status := trader.GetStatus()
	c.JSON(http.StatusOK, status)
}

// handleAccount account information
func (s *Server) handleAccount(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("📊 Received account information request [%s]", trader.GetName())
	account, err := trader.GetAccountInfo()
	if err != nil {
		log.Printf("❌ Failed to get account information [%s]: %v", trader.GetName(), err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get account information: %v", err),
		})
		return
	}

	log.Printf("✓ Returning account information [%s]: equity=%.2f, available=%.2f, pnl=%.2f (%.2f%%)",
		trader.GetName(),
		account["total_equity"],
		account["available_balance"],
		account["total_pnl"],
		account["total_pnl_pct"])
	c.JSON(http.StatusOK, account)
}

// handlePositions position list
func (s *Server) handlePositions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	positions, err := trader.GetPositions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get position list: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, positions)
}

// handlePositionHistory get position history (all positions including closed)
func (s *Server) handlePositionHistory(c *gin.Context) {
	// #region agent log
	func() {
		f, _ := os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			json.NewEncoder(f).Encode(map[string]interface{}{"sessionId": "debug-session", "runId": "run1", "hypothesisId": "A", "location": "api/server.go:3394", "message": "handlePositionHistory entry", "data": map[string]interface{}{"traderID": c.Query("trader_id"), "limit": c.Query("limit"), "offset": c.Query("offset")}, "timestamp": time.Now().UnixMilli()})
			f.Close()
		}
	}()
	// #endregion
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get limit and offset from query parameters
	limit := 100 // Default limit
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	offset := 0 // Default offset
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	// #region agent log
	func() {
		f, _ := os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			json.NewEncoder(f).Encode(map[string]interface{}{"sessionId": "debug-session", "runId": "run1", "hypothesisId": "A", "location": "api/server.go:3423", "message": "Before GetPositionHistory call", "data": map[string]interface{}{"traderID": traderID, "limit": limit, "offset": offset, "databaseNil": s.database == nil}, "timestamp": time.Now().UnixMilli()})
			f.Close()
		}
	}()
	// #endregion

	// Get position history from database
	if s.database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Database not available",
		})
		return
	}

	// Get closed positions (with pagination)
	closedPositions, err := s.database.GetPositionHistory(traderID, limit, offset)
	// #region agent log
	func() {
		f, _ := os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			json.NewEncoder(f).Encode(map[string]interface{}{"sessionId": "debug-session", "runId": "run1", "hypothesisId": "A,B", "location": "api/server.go:3443", "message": "After GetPositionHistory call", "data": map[string]interface{}{"traderID": traderID, "err": func() string { if err != nil { return err.Error() } else { return "nil" } }(), "closedPositionCount": len(closedPositions)}, "timestamp": time.Now().UnixMilli()})
			f.Close()
		}
	}()
	// #endregion
	if err != nil {
		log.Printf("❌ Failed to get position history for trader %s (limit=%d, offset=%d): %v", traderID, limit, offset, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get position history: %v", err),
		})
		return
	}

	// Get open positions (all of them, no pagination needed for open positions)
	openPositions, err := s.database.GetOpenPositions(traderID)
	if err != nil {
		log.Printf("❌ Failed to get open positions for trader %s: %v", traderID, err)
		// Continue with closed positions only if open positions query fails
		openPositions = []*config.PositionRecord{}
	}

	// Get actual current positions from exchange to verify which are really open
	var exchangePositionsMap map[string]bool
	trader, err := s.traderManager.GetTrader(traderID)
	if err == nil {
		currentPositions, err := trader.GetPositions()
		if err == nil {
			// Build a map of symbol_side -> true for positions that exist on exchange
			exchangePositionsMap = make(map[string]bool)
			for _, pos := range currentPositions {
				symbol, ok1 := pos["symbol"].(string)
				side, ok2 := pos["side"].(string)
				quantity, ok3 := pos["positionAmt"].(float64)
				if ok1 && ok2 && ok3 {
					// Only consider positions with non-zero quantity
					if quantity < 0 {
						quantity = -quantity
					}
					if quantity > 0.0001 {
						key := symbol + "_" + side
						exchangePositionsMap[key] = true
					}
				}
			}
		}
	}

	// Convert to JSON-friendly format
	type PositionHistoryItem struct {
		ID              string    `json:"id"`
		TraderID        string    `json:"trader_id"`
		Symbol          string    `json:"symbol"`
		Side            string    `json:"side"`
		EntryPrice      float64   `json:"entry_price"`
		ExitPrice       float64   `json:"exit_price"`
		Quantity        float64   `json:"quantity"`
		EntryFee        float64   `json:"entry_fee"`
		ExitFee         float64   `json:"exit_fee"`
		RealizedPnL     float64   `json:"realized_pnl"`
		Leverage        int       `json:"leverage"`
		OpenedAt        string    `json:"opened_at"`
		ClosedAt        *string   `json:"closed_at,omitempty"`
		OrderIDOpen     string    `json:"order_id_open"`
		OrderIDClose    string    `json:"order_id_close"`
		StopLossPrice   float64   `json:"stop_loss_price"`
		TakeProfitPrice float64   `json:"take_profit_price"`
		IsClosed        bool      `json:"is_closed"`
		Status          string    `json:"status"` // "open" or "closed"
	}

	// Merge closed and open positions, then sort by opened_at DESC
	allPositions := make([]*config.PositionRecord, 0, len(closedPositions)+len(openPositions))
	allPositions = append(allPositions, closedPositions...)
	allPositions = append(allPositions, openPositions...)

	// Sort by opened_at DESC (most recent first)
	sort.Slice(allPositions, func(i, j int) bool {
		return allPositions[i].OpenedAt.After(allPositions[j].OpenedAt)
	})

	// Apply pagination after merging (if needed, but typically we want all open positions)
	// For now, we'll include all open positions and apply limit/offset to the merged result
	startIdx := offset
	endIdx := offset + limit
	if startIdx > len(allPositions) {
		startIdx = len(allPositions)
	}
	if endIdx > len(allPositions) {
		endIdx = len(allPositions)
	}
	paginatedPositions := allPositions[startIdx:endIdx]

	result := make([]PositionHistoryItem, 0, len(paginatedPositions))
	// #region agent log
	func() {
		f, _ := os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			json.NewEncoder(f).Encode(map[string]interface{}{"sessionId": "debug-session", "runId": "run1", "hypothesisId": "C,D", "location": "api/server.go:3483", "message": "Before result conversion loop", "data": map[string]interface{}{"totalPositionCount": len(allPositions), "closedCount": len(closedPositions), "openCount": len(openPositions), "paginatedCount": len(paginatedPositions)}, "timestamp": time.Now().UnixMilli()})
			f.Close()
		}
	}()
	// #endregion
	for i, pos := range paginatedPositions {
		// #region agent log
		func() {
			f, _ := os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if f != nil {
				json.NewEncoder(f).Encode(map[string]interface{}{"sessionId": "debug-session", "runId": "run1", "hypothesisId": "C", "location": "api/server.go:3457", "message": "Processing position", "data": map[string]interface{}{"index": i, "id": pos.ID, "openedAtZero": pos.OpenedAt.IsZero(), "closedAtNil": pos.ClosedAt == nil}, "timestamp": time.Now().UnixMilli()})
				f.Close()
			}
		}()
		// #endregion
		closedAtStr := ""
		if pos.ClosedAt != nil {
			closedAtStr = pos.ClosedAt.Format("2006-01-02 15:04:05")
		}

		openedAtStr := pos.OpenedAt.Format("2006-01-02 15:04:05")
		// #region agent log
		func() {
			f, _ := os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if f != nil {
				json.NewEncoder(f).Encode(map[string]interface{}{"sessionId": "debug-session", "runId": "run1", "hypothesisId": "C", "location": "api/server.go:3461", "message": "After time formatting", "data": map[string]interface{}{"index": i, "openedAtStr": openedAtStr, "closedAtStr": closedAtStr}, "timestamp": time.Now().UnixMilli()})
				f.Close()
			}
		}()
		// #endregion

		// Determine status based on whether position is closed in database AND still open on exchange
		status := "closed"
		exitPrice := pos.ExitPrice
		
		// If position is closed but exit_price is 0, try to get it from historical trades
		if (pos.ClosedAt != nil || exitPrice == 0) && trader != nil && exitPrice == 0 {
			// Only query if position was opened recently (within last 7 days) to avoid too many API calls
			if time.Since(pos.OpenedAt) < 7*24*time.Hour {
				endTime := time.Now()
				startTime := pos.OpenedAt.Add(-24 * time.Hour)
				
				trades, err := trader.GetUserTrades(pos.Symbol, 100, &startTime, &endTime)
				if err == nil && len(trades) > 0 {
					// Find matching closure trades
					var totalClosedQty float64
					var totalExitValue float64
					
					for _, trade := range trades {
						tradeTime, ok := trade["time"].(int64)
						if !ok {
							continue
						}
						tradeTimestamp := time.Unix(tradeTime/1000, 0)
						
						// Only consider trades after position was opened
						if tradeTimestamp.Before(pos.OpenedAt) {
							continue
						}
						
						// Check if trade matches closure direction
						isBuyer, _ := trade["isBuyer"].(bool)
						tradeQty, _ := trade["qty"].(float64)
						tradePrice, _ := trade["price"].(float64)
						
						matchesClosure := false
						if pos.Side == "long" && !isBuyer {
							// Closing long position with sell trade
							matchesClosure = true
						} else if pos.Side == "short" && isBuyer {
							// Closing short position with buy trade
							matchesClosure = true
						}
						
						if matchesClosure {
							totalClosedQty += tradeQty
							totalExitValue += tradeQty * tradePrice
						}
					}
					
					// Calculate average exit price if we found matching trades
					if totalClosedQty > 0 {
						avgExitPrice := totalExitValue / totalClosedQty
						if avgExitPrice > 0 {
							exitPrice = avgExitPrice
							log.Printf("✅ Found exit price from historical trades for %s %s: %.4f", pos.Symbol, pos.Side, exitPrice)
						}
					}
				}
			}
		}
		
		if pos.ClosedAt == nil {
			// Position is marked as open in database, but check if it's actually open on exchange
			status = "open"
			if exchangePositionsMap != nil {
				// Check if this position exists in exchange positions
				posKey := pos.Symbol + "_" + pos.Side
				if !exchangePositionsMap[posKey] {
					// Position is not in exchange, so it's actually closed
					status = "closed"
					log.Printf("🔍 Position %s %s marked as open in DB but not found on exchange, marking as closed", pos.Symbol, pos.Side)
				}
			}
		}

		result = append(result, PositionHistoryItem{
			ID:              pos.ID,
			TraderID:        pos.TraderID,
			Symbol:          pos.Symbol,
			Side:            pos.Side,
			EntryPrice:      pos.EntryPrice,
			ExitPrice:       exitPrice,
			Quantity:        pos.Quantity,
			EntryFee:        pos.EntryFee,
			ExitFee:         pos.ExitFee,
			RealizedPnL:     pos.RealizedPnL,
			Leverage:        pos.Leverage,
			OpenedAt:         openedAtStr,
			ClosedAt:        &closedAtStr,
			OrderIDOpen:     pos.OrderIDOpen,
			OrderIDClose:    pos.OrderIDClose,
			StopLossPrice:   pos.StopLossPrice,
			TakeProfitPrice: pos.TakeProfitPrice,
			IsClosed:        pos.ClosedAt != nil,
			Status:          status,
		})
	}
	// #region agent log
	func() {
		f, _ := os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			json.NewEncoder(f).Encode(map[string]interface{}{"sessionId": "debug-session", "runId": "run1", "hypothesisId": "D", "location": "api/server.go:3483", "message": "Before JSON response", "data": map[string]interface{}{"resultCount": len(result)}, "timestamp": time.Now().UnixMilli()})
			f.Close()
		}
	}()
	// #endregion

	c.JSON(http.StatusOK, result)
	// #region agent log
	func() {
		f, _ := os.OpenFile("d:\\nofx\\nofx\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			json.NewEncoder(f).Encode(map[string]interface{}{"sessionId": "debug-session", "runId": "run1", "hypothesisId": "D", "location": "api/server.go:3485", "message": "After JSON response", "data": map[string]interface{}{}, "timestamp": time.Now().UnixMilli()})
			f.Close()
		}
	}()
	// #endregion
}

// handleClosePosition manual close position
func (s *Server) handleClosePosition(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get trader_id (from query parameter or body)
	traderID := c.Query("trader_id")
	if traderID == "" {
		var req struct {
			TraderID string `json:"trader_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing trader_id parameter"})
			return
		}
		traderID = req.TraderID
	}

	// Get request body
	var closeReq struct {
		Symbol   string  `json:"symbol" binding:"required"`
		Side     string  `json:"side" binding:"required"` // "long" or "short"
		Quantity float64 `json:"quantity"`                // 0 = close all
	}

	if err := c.ShouldBindJSON(&closeReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Request parameter error: %v", err)})
		return
	}

	// Validate side parameter
	if closeReq.Side != "long" && closeReq.Side != "short" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "side must be 'long' or 'short'"})
		return
	}

	// Get trader
	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	// Verify trader ownership (check database)
	traderCfg, _, _, err := s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to operate this trader"})
		return
	}

	log.Printf("📊 [%s] Manual close position request: symbol=%s, side=%s, quantity=%.4f", traderCfg.Name, closeReq.Symbol, closeReq.Side, closeReq.Quantity)

	// Execute close position
	var result map[string]interface{}
	if closeReq.Side == "long" {
		result, err = trader.CloseLong(closeReq.Symbol, closeReq.Quantity)
	} else {
		result, err = trader.CloseShort(closeReq.Symbol, closeReq.Quantity)
	}

	if err != nil {
		log.Printf("❌ [%s] Close position failed: %v", traderCfg.Name, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Close position failed: %v", err),
		})
		return
	}

	log.Printf("✓ [%s] Close position successful: symbol=%s, side=%s", traderCfg.Name, closeReq.Symbol, closeReq.Side)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Close position successful",
		"result":  result,
	})
}

// handleDecisions decision log list
func (s *Server) handleDecisions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Get all historical decision records (unlimited)
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get decision logs: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, records)
}

// handleLatestDecisions latest decision logs (configurable limit, newest first)
func (s *Server) handleLatestDecisions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Get limit from query parameter, default to 5
	limit := 5
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	records, err := trader.GetDecisionLogger().GetLatestRecords(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get decision logs: %v", err),
		})
		return
	}

	// Reverse array to put newest first (for list display)
	// GetLatestRecords returns from old to new (for charts), here we need from new to old
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	c.JSON(http.StatusOK, records)
}

// handleStatistics statistics information
func (s *Server) handleStatistics(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	stats, err := trader.GetDecisionLogger().GetStatistics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get statistics: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// handleEquityHistory return history data
func (s *Server) handleEquityHistory(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Define EquityPoint type for return data
	type EquityPoint struct {
		Timestamp        string  `json:"timestamp"`
		TotalEquity      float64 `json:"total_equity"`      // Account equity (wallet + unrealized)
		AvailableBalance float64 `json:"available_balance"` // Available balance
		TotalPnL         float64 `json:"total_pnl"`         // Total PnL (relative to initial balance)
		TotalPnLPct      float64 `json:"total_pnl_pct"`     // Total PnL percentage
		PositionCount    int     `json:"position_count"`    // Position count
		MarginUsedPct    float64 `json:"margin_used_pct"`   // Margin usage percentage
		CycleNumber      int     `json:"cycle_number"`
	}

	// Try to get equity history from database first (persistent across deployments)
	var history []EquityPoint
	var records []*logger.DecisionRecord
	
	if s.database != nil {
		dbRecords, err := s.database.GetEquityHistory(traderID, 10000)
		if err == nil && len(dbRecords) > 0 {
			// Convert database records to EquityPoint format
			history = make([]EquityPoint, 0, len(dbRecords))
			for _, dbRec := range dbRecords {
				history = append(history, EquityPoint{
					Timestamp:        dbRec.Timestamp.Format("2006-01-02 15:04:05"),
					TotalEquity:      dbRec.TotalEquity,
					AvailableBalance: dbRec.AvailableBalance,
					TotalPnL:         dbRec.TotalPnL,
					TotalPnLPct:      dbRec.TotalPnLPct,
					PositionCount:    dbRec.PositionCount,
					MarginUsedPct:    dbRec.MarginUsedPct,
					CycleNumber:      dbRec.CycleNumber,
				})
			}
		}
	}

	// Fallback to file-based logs if database is empty or unavailable
	if len(history) == 0 {
		// Get as much historical data as possible (several days of data)
		// Every 3 minutes per cycle: 10000 records = approximately 20 days of data
		records, err = trader.GetDecisionLogger().GetLatestRecords(10000)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to get historical data: %v", err),
			})
			return
		}
	}

	// Process file-based records if database was empty
	if len(history) == 0 && len(records) > 0 {
		// Get initial balance from AutoTrader (for calculating PnL percentage)
		initialBalance := 0.0
		if status := trader.GetStatus(); status != nil {
			if ib, ok := status["initial_balance"].(float64); ok && ib > 0 {
				initialBalance = ib
			}
		}

		// If cannot get from status and have historical records, get from first record
		if initialBalance == 0 && len(records) > 0 {
			// First record's equity as initial balance
			initialBalance = records[0].AccountState.TotalBalance
		}

		// If initial balance is 0, return empty array (account has no funds or hasn't started trading)
		if initialBalance == 0 {
			// If no historical records, return empty array
			if len(records) == 0 {
				c.JSON(http.StatusOK, []EquityPoint{})
				return
			}
			// If have historical records but initial balance is 0, use 1.0 as baseline to avoid division by zero
			// PNL percentage will show as 0% or based on first record
			initialBalance = 1.0
		}

		// Convert file-based records to EquityPoint format
		history = make([]EquityPoint, 0, len(records))
		for _, record := range records {
			// TotalBalance field actually stores TotalEquity
			totalEquity := record.AccountState.TotalBalance
			// TotalUnrealizedProfit field actually stores TotalPnL (relative to initial balance)
			totalPnL := record.AccountState.TotalUnrealizedProfit

			// Calculate PnL percentage
			totalPnLPct := 0.0
			if initialBalance > 0 {
				totalPnLPct = (totalPnL / initialBalance) * 100
			}

			history = append(history, EquityPoint{
				Timestamp:        record.Timestamp.Format("2006-01-02 15:04:05"),
				TotalEquity:      totalEquity,
				AvailableBalance: record.AccountState.AvailableBalance,
				TotalPnL:         totalPnL,
				TotalPnLPct:      totalPnLPct,
				PositionCount:    record.AccountState.PositionCount,
				MarginUsedPct:    record.AccountState.MarginUsedPct,
				CycleNumber:      record.CycleNumber,
			})
		}
	}

	// Get initial balance for real-time data point calculation
	initialBalance := 0.0
	if status := trader.GetStatus(); status != nil {
		if ib, ok := status["initial_balance"].(float64); ok && ib > 0 {
			initialBalance = ib
		}
	}
	// Fallback: calculate from first history record if available
	if initialBalance == 0 && len(history) > 0 {
		initialBalance = history[0].TotalEquity - history[0].TotalPnL
	}

	// Add real-time data point from current account balance for chart/leaderboard consistency
	accountInfo, err := trader.GetAccountInfo()
	if err == nil {
		currentTotalEquity, _ := accountInfo["total_equity"].(float64)
		currentAvailableBalance, _ := accountInfo["available_balance"].(float64)
		currentTotalPnL, _ := accountInfo["total_pnl"].(float64)
		currentTotalPnLPct, _ := accountInfo["total_pnl_pct"].(float64)
		currentPositionCount, _ := accountInfo["position_count"].(int)
		currentMarginUsedPct, _ := accountInfo["margin_used_pct"].(float64)

		// Use current timestamp for real-time data point
		currentTimestamp := time.Now().Format("2006-01-02 15:04:05")

		// Calculate PnL percentage if not provided or recalculate using initial balance
		if currentTotalPnLPct == 0 && initialBalance > 0 && currentTotalEquity > 0 {
			currentTotalPnL = currentTotalEquity - initialBalance
			currentTotalPnLPct = (currentTotalPnL / initialBalance) * 100
		}

		// Get the last cycle number and increment it, or use 0 if no history
		lastCycleNumber := 0
		if len(history) > 0 {
			lastCycleNumber = history[len(history)-1].CycleNumber + 1
		}

		history = append(history, EquityPoint{
			Timestamp:        currentTimestamp,
			TotalEquity:      currentTotalEquity,
			AvailableBalance: currentAvailableBalance,
			TotalPnL:         currentTotalPnL,
			TotalPnLPct:      currentTotalPnLPct,
			PositionCount:    currentPositionCount,
			MarginUsedPct:    currentMarginUsedPct,
			CycleNumber:      lastCycleNumber,
		})
	}

	c.JSON(http.StatusOK, history)
}

// handlePerformance AI historical performance analysis (for displaying AI learning and reflection)
func (s *Server) handlePerformance(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Analyze trading performance of last 500 cycles (to avoid losing trade records for long-term positions)
	// Assuming 3 minutes per cycle, 500 cycles = ~25 hours, sufficient to cover most trades
	// Also use database positions as fallback/supplement for complete data
	var database interface{}
	if s.database != nil {
		database = s.database
		log.Printf("📊 [handlePerformance] Database available for trader %s", traderID)
	} else {
		log.Printf("⚠️ [handlePerformance] Database is nil for trader %s - performance analysis will only use decision records", traderID)
	}
	
	performance, err := trader.GetDecisionLogger().AnalyzePerformance(500, traderID, database)
	if err != nil {
		log.Printf("❌ [handlePerformance] Failed to analyze performance for trader %s: %v", traderID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to analyze historical performance: %v", err),
		})
		return
	}

	log.Printf("📊 [handlePerformance] Performance analysis complete for trader %s: total_trades=%d, winning=%d, losing=%d", 
		traderID, performance.TotalTrades, performance.WinningTrades, performance.LosingTrades)
	
	c.JSON(http.StatusOK, performance)
}

// handleGetReplicationStatus get replication status
func (s *Server) handleGetReplicationStatus(c *gin.Context) {
	traderID := c.Param("id")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trader_id is required"})
		return
	}

	// Get trader
	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Check if this is a child trader (has followed_trader_id)
	followedTraderID, err := s.database.GetTraderFollowedTraderID(traderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get follow relationship: %v", err)})
		return
	}

	result := gin.H{
		"trader_id":   traderID,
		"trader_name": trader.GetName(),
		"is_child":    followedTraderID != "",
		"is_parent":   false,
	}

	// If this is a child trader, get parent info
	if followedTraderID != "" {
		parentTrader, err := s.traderManager.GetTrader(followedTraderID)
		if err == nil {
			parentStatus := parentTrader.GetStatus()
			isRunning := false
			if running, ok := parentStatus["is_running"].(bool); ok {
				isRunning = running
			}
			result["parent"] = gin.H{
				"trader_id":   followedTraderID,
				"trader_name": parentTrader.GetName(),
				"is_running":  isRunning,
			}
		} else {
			result["parent"] = gin.H{
				"trader_id":   followedTraderID,
				"trader_name": "Unknown",
				"is_running":  false,
				"error":       "Parent trader not found in memory",
			}
		}
		result["followers"] = []gin.H{}
	} else {
		// If this is a parent trader, get followers list
		followers, err := s.database.GetFollowerTraders(traderID)
		if err == nil {
			followersList := make([]gin.H, 0, len(followers))
			for _, followerRecord := range followers {
				followerTrader, err := s.traderManager.GetTrader(followerRecord.ID)
				if err == nil {
					status := followerTrader.GetStatus()
					isRunning := false
					if running, ok := status["is_running"].(bool); ok {
						isRunning = running
					}
					followersList = append(followersList, gin.H{
						"trader_id":   followerRecord.ID,
						"trader_name": followerRecord.Name,
						"is_running":  isRunning,
						"user_id":     followerRecord.UserID,
					})
				} else {
					followersList = append(followersList, gin.H{
						"trader_id":   followerRecord.ID,
						"trader_name": followerRecord.Name,
						"is_running":  false,
						"user_id":     followerRecord.UserID,
						"error":       "Follower trader not found in memory",
					})
				}
			}
			result["followers"] = followersList
			result["is_parent"] = len(followersList) > 0
		} else {
			result["followers"] = []gin.H{}
			result["error"] = fmt.Sprintf("Failed to get follower list: %v", err)
		}
		result["parent"] = nil
	}

	c.JSON(http.StatusOK, result)
}

// handleTestSignal manually trigger test signal
func (s *Server) handleTestSignal(c *gin.Context) {
	traderID := c.Param("id")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trader_id is required"})
		return
	}

	// Verify trader exists
	_, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Check if this trader has followers
	followers, err := s.database.GetFollowerTraders(traderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get follower list: %v", err)})
		return
	}

	if len(followers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This trader has no followers"})
		return
	}

	// Parse request body
	var testSignal struct {
		Symbol          string  `json:"symbol"`
		Action          string  `json:"action"`
		Leverage        int     `json:"leverage"`
		PositionSizeUSD float64 `json:"position_size_usd"`
		StopLoss        float64 `json:"stop_loss"`
		TakeProfit      float64 `json:"take_profit"`
		Reasoning       string  `json:"reasoning"`
	}

	if err := c.ShouldBindJSON(&testSignal); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	// Validate required fields
	if testSignal.Symbol == "" || testSignal.Action == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol and action are required"})
		return
	}

	// Create test decision
	testDecision := &decision.Decision{
		Symbol:          testSignal.Symbol,
		Action:          testSignal.Action,
		Leverage:        testSignal.Leverage,
		PositionSizeUSD: testSignal.PositionSizeUSD,
		StopLoss:        testSignal.StopLoss,
		TakeProfit:      testSignal.TakeProfit,
		Reasoning:       testSignal.Reasoning,
		Confidence:      85, // Default confidence for test signals
	}

	if testDecision.Reasoning == "" {
		testDecision.Reasoning = fmt.Sprintf("Test signal: %s %s", testSignal.Action, testSignal.Symbol)
	}

	// Send signal to followers
	s.traderManager.ReplicateTradeToFollowers(traderID, testDecision, s.database)

	c.JSON(http.StatusOK, gin.H{
		"message":         fmt.Sprintf("Test signal sent to %d followers", len(followers)),
		"signal":          testDecision,
		"followers_count": len(followers),
	})
}

// handleGetUserFollowers get user's all traders' follower list and their activities
func (s *Server) handleGetUserFollowers(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unable to get user ID"})
		return
	}

	log.Printf("🔍 DEBUG [handleGetUserFollowers]: Starting request for user ID: '%s'", userID)

	// Ensure user's traders are loaded into memory
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ Failed to load user %s traders: %v", userID, err)
	}

	// Get all user's traders
	userTraders, err := s.database.GetTraders(userID)
	if err != nil {
		log.Printf("❌ DEBUG [handleGetUserFollowers]: Failed to get traders for user '%s': %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get trader list: %v", err),
		})
		return
	}

	log.Printf("📊 DEBUG [handleGetUserFollowers]: Found %d total traders for user '%s'", len(userTraders), userID)

	// Filter out parent traders (traders without followed_trader_id)
	var parentTraders []*config.TraderRecord
	for _, trader := range userTraders {
		log.Printf("🔍 DEBUG [handleGetUserFollowers]: Checking trader ID: '%s', Name: '%s', FollowedTraderID: '%s'",
			trader.ID, trader.Name, trader.FollowedTraderID)
		if trader.FollowedTraderID == "" {
			parentTraders = append(parentTraders, trader)
			log.Printf("✅ DEBUG [handleGetUserFollowers]: Trader '%s' is a parent trader (no followed_trader_id)", trader.ID)
		} else {
			log.Printf("ℹ️ DEBUG [handleGetUserFollowers]: Trader '%s' is a follower (followed_trader_id: '%s'), skipping", trader.ID, trader.FollowedTraderID)
		}
	}

	log.Printf("📊 DEBUG [handleGetUserFollowers]: Found %d parent traders for user '%s'", len(parentTraders), userID)

	// Get all user information to return user email and name for followers
	users, err := s.database.GetAllUsersWithRoles()
	userMap := make(map[string]gin.H) // userID -> {email, name}
	if err == nil {
		for _, u := range users {
			userMap[u.ID] = gin.H{
				"email": u.Email,
				"name":  u.Email, // Use email as name if no separate name field
			}
		}
	} else {
		log.Printf("⚠️ Failed to get user list for follower info: %v", err)
	}

	// Build response data
	parentTradersList := make([]gin.H, 0, len(parentTraders))

	for _, parentTrader := range parentTraders {
		log.Printf("🔍 DEBUG [handleGetUserFollowers]: Querying followers for parent trader ID: '%s', Name: '%s'",
			parentTrader.ID, parentTrader.Name)

		// Get all followers of this parent trader
		followers, err := s.database.GetFollowerTraders(parentTrader.ID)
		if err != nil {
			log.Printf("❌ DEBUG [handleGetUserFollowers]: Failed to get trader '%s' follower list: %v", parentTrader.ID, err)
			continue
		}

		log.Printf("📊 DEBUG [handleGetUserFollowers]: Parent trader '%s' has %d followers", parentTrader.ID, len(followers))

		// If no followers, skip
		if len(followers) == 0 {
			log.Printf("⏭️ DEBUG [handleGetUserFollowers]: Skipping parent trader '%s' - no followers", parentTrader.ID)
			continue
		}

		// Build follower list
		followersList := make([]gin.H, 0, len(followers))

		for _, followerRecord := range followers {
			// Get follower trader instance
			followerTrader, err := s.traderManager.GetTrader(followerRecord.ID)
			if err != nil {
				log.Printf("⚠️ Follower trader %s not in memory: %v", followerRecord.ID, err)
				// Get user email and name for this follower
				userEmail := ""
				userName := ""
				if userInfo, exists := userMap[followerRecord.UserID]; exists {
					if email, ok := userInfo["email"].(string); ok {
						userEmail = email
					}
					if name, ok := userInfo["name"].(string); ok {
						userName = name
					}
				}

				// Still add basic info even if not in memory
				followersList = append(followersList, gin.H{
					"trader_id":   followerRecord.ID,
					"trader_name": followerRecord.Name,
					"user_id":     followerRecord.UserID,
					"user_email":  userEmail,
					"user_name":   userName,
					"is_running":  false,
					"error":       "Follower trader not found in memory",
				})
				continue
			}

			// Get running status
			status := followerTrader.GetStatus()
			isRunning := false
			if running, ok := status["is_running"].(bool); ok {
				isRunning = running
			}

			// Get account information
			var accountInfo gin.H
			account, err := followerTrader.GetAccountInfo()
			if err != nil {
				log.Printf("⚠️ Failed to get follower %s account information: %v", followerRecord.ID, err)
				accountInfo = gin.H{"error": "Unable to get account information"}
			} else {
				accountInfo = account
			}

			// Get latest decisions (last 5)
			var latestDecisions []interface{}
			decisionLogger := followerTrader.GetDecisionLogger()
			if decisionLogger != nil {
				records, err := decisionLogger.GetLatestRecords(5)
				if err != nil {
					log.Printf("⚠️ Failed to get follower %s decision records: %v", followerRecord.ID, err)
				} else {
					// Convert to JSON serializable format
					latestDecisions = make([]interface{}, 0, len(records))
					for _, record := range records {
						latestDecisions = append(latestDecisions, gin.H{
							"timestamp":     record.Timestamp,
							"cycle_number":  record.CycleNumber,
							"success":       record.Success,
							"error_message": record.ErrorMessage,
							"decisions":     record.Decisions,
						})
					}
				}
			}

			// Get position list
			var positions []interface{}
			positionsData, err := followerTrader.GetPositions()
			if err != nil {
				log.Printf("⚠️ Failed to get follower %s position list: %v", followerRecord.ID, err)
			} else {
				positions = make([]interface{}, 0, len(positionsData))
				for _, pos := range positionsData {
					positions = append(positions, pos)
				}
			}

			// Get user email and name for this follower
			userEmail := ""
			userName := ""
			if userInfo, exists := userMap[followerRecord.UserID]; exists {
				if email, ok := userInfo["email"].(string); ok {
					userEmail = email
				}
				if name, ok := userInfo["name"].(string); ok {
					userName = name
				}
			}

			// Build follower information
			followerInfo := gin.H{
				"trader_id":        followerRecord.ID,
				"trader_name":      followerRecord.Name,
				"user_id":          followerRecord.UserID,
				"user_email":       userEmail,
				"user_name":        userName,
				"is_running":       isRunning,
				"account":          accountInfo,
				"latest_decisions": latestDecisions,
				"positions":        positions,
			}

			followersList = append(followersList, followerInfo)
		}

		// Build parent trader information
		parentInfo := gin.H{
			"trader_id":   parentTrader.ID,
			"trader_name": parentTrader.Name,
			"followers":   followersList,
		}

		log.Printf("✅ DEBUG [handleGetUserFollowers]: Added parent trader '%s' with %d followers to response",
			parentTrader.ID, len(followersList))
		parentTradersList = append(parentTradersList, parentInfo)
	}

	log.Printf("📊 DEBUG [handleGetUserFollowers]: Returning %d parent traders with followers for user '%s'",
		len(parentTradersList), userID)

	c.JSON(http.StatusOK, gin.H{
		"parent_traders": parentTradersList,
	})
}

// authMiddleware JWT authentication middleware
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
			c.Abort()
			return
		}

		// Check Bearer token format
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization format"})
			c.Abort()
			return
		}

		tokenString := tokenParts[1]

		// Blacklist check
		if auth.IsTokenBlacklisted(tokenString) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token has expired, please login again"})
			c.Abort()
			return
		}

		// Validate JWT token
		claims, err := auth.ValidateJWT(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token: " + err.Error()})
			c.Abort()
			return
		}

		// Get user information (including role)
		user, err := s.database.GetUserByID(claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unable to get user information"})
			c.Abort()
			return
		}

		// Store user information in context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", user.Role)
		c.Next()
	}
}

// nonFollowerMiddleware restrict access to non-follower users only
func (s *Server) nonFollowerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			role = "user" // Default role
		}

		roleStr, ok := role.(string)
		if !ok {
			roleStr = "user"
		}

		if roleStr == "follower" {
			c.JSON(http.StatusForbidden, gin.H{"error": "This feature is only available to non-follower users"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// followerOnlyMiddleware restrict access to follower users only
func (s *Server) followerOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unable to get user role"})
			c.Abort()
			return
		}

		roleStr, ok := role.(string)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid user role"})
			c.Abort()
			return
		}

		if roleStr != "follower" {
			c.JSON(http.StatusForbidden, gin.H{"error": "This feature is only available to follower users"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// adminOnlyMiddleware restrict access to admin users only
func (s *Server) adminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			role = "user" // Default role
		}

		roleStr, ok := role.(string)
		if !ok {
			roleStr = "user"
		}

		if roleStr != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "This feature is only available to admin users"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// handleLogout add current token to blacklist
func (s *Server) handleLogout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
		return
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization format"})
		return
	}
	tokenString := parts[1]
	claims, err := auth.ValidateJWT(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}
	var exp time.Time
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	} else {
		exp = time.Now().Add(24 * time.Hour)
	}
	auth.BlacklistToken(tokenString, exp)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}

// handleRegister handle user registration request
func (s *Server) handleRegister(c *gin.Context) {

	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
		BetaCode string `json:"beta_code"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if beta mode is enabled
	betaModeStr, _ := s.database.GetSystemConfig("beta_mode")
	if betaModeStr == "true" {
		// Beta mode requires a valid beta code
		if req.BetaCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Registration requires a beta code during beta period"})
			return
		}

		// Validate beta code
		isValid, err := s.database.ValidateBetaCode(req.BetaCode)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate beta code"})
			return
		}
		if !isValid {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Beta code is invalid or already used"})
			return
		}
	}

	// Check max users limit
	maxUsersStr, _ := s.database.GetSystemConfig("max_users")
	if maxUsersStr != "" {
		maxUsers, err := strconv.Atoi(maxUsersStr)
		if err == nil && maxUsers > 0 {
			userCount, err := s.database.GetUserCount()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check user count"})
				return
			}
			if userCount >= maxUsers {
				c.JSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("Registration limit reached. Maximum %d user(s) allowed.", maxUsers)})
				return
			}
		}
	}

	// Check if email already exists
	_, err := s.database.GetUserByEmail(req.Email)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	// Generate password hash
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// Generate OTP secret
	otpSecret, err := auth.GenerateOTPSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate OTP secret"})
		return
	}

	// Create user (OTP not verified)
	userID := uuid.New().String()
	user := &config.User{
		ID:           userID,
		Email:        req.Email,
		PasswordHash: passwordHash,
		OTPSecret:    otpSecret,
		OTPVerified:  false,
	}

	err = s.database.CreateUser(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user: " + err.Error()})
		return
	}

	// If beta mode, mark beta code as used
	betaModeStr2, _ := s.database.GetSystemConfig("beta_mode")
	if betaModeStr2 == "true" && req.BetaCode != "" {
		err := s.database.UseBetaCode(req.BetaCode, req.Email)
		if err != nil {
			log.Printf("⚠️ Failed to mark beta code as used: %v", err)
			// Don't return error here, as user has been successfully created
		} else {
			log.Printf("✓ Beta code %s has been used by user %s", req.BetaCode, req.Email)
		}
	}

	// Return OTP setup information
	qrCodeURL := auth.GetOTPQRCodeURL(otpSecret, req.Email)
	c.JSON(http.StatusOK, gin.H{
		"user_id":     userID,
		"email":       req.Email,
		"otp_secret":  otpSecret,
		"qr_code_url": qrCodeURL,
		"message":     "Please use Google Authenticator to scan QR code and verify OTP",
	})
}

// handleCompleteRegistration complete registration (verify OTP)
func (s *Server) handleCompleteRegistration(c *gin.Context) {
	var req struct {
		UserID  string `json:"user_id" binding:"required"`
		OTPCode string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user information
	user, err := s.database.GetUserByID(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
		return
	}

	// Verify OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OTP verification code is incorrect"})
		return
	}

	// Update user OTP verification status
	err = s.database.UpdateUserOTPVerified(req.UserID, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user status"})
		return
	}

	// Generate JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Initialize user's default model and exchange configurations
	err = s.initUserDefaultConfigs(user.ID)
	if err != nil {
		log.Printf("Failed to initialize user default configurations: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"message": "Registration completed",
	})
}

// handleLogin handle user login request
func (s *Server) handleLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user information
	user, err := s.database.GetUserByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email or password incorrect"})
		return
	}

	// Verify password
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email or password incorrect"})
		return
	}

	// Check if OTP is verified
	if !user.OTPVerified {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":              "Account OTP setup incomplete",
			"user_id":            user.ID,
			"requires_otp_setup": true,
		})
		return
	}

	// Return status requiring OTP verification
	c.JSON(http.StatusOK, gin.H{
		"user_id":      user.ID,
		"email":        user.Email,
		"message":      "Please enter Google Authenticator code",
		"requires_otp": true,
	})
}

// handleVerifyOTP verify OTP and complete login
func (s *Server) handleVerifyOTP(c *gin.Context) {
	var req struct {
		UserID  string `json:"user_id" binding:"required"`
		OTPCode string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user information
	user, err := s.database.GetUserByID(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User does not exist"})
		return
	}

	// Verify OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification code is incorrect"})
		return
	}

	// Generate JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"message": "Login successful",
	})
}

// handleResetPassword reset password (via email + OTP verification)
func (s *Server) handleResetPassword(c *gin.Context) {
	var req struct {
		Email       string `json:"email" binding:"required,email"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
		OTPCode     string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Query user
	user, err := s.database.GetUserByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email does not exist"})
		return
	}

	// Verify OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Google Authenticator verification code is incorrect"})
		return
	}

	// Generate new password hash
	newPasswordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// Update password
	err = s.database.UpdateUserPassword(user.ID, newPasswordHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	log.Printf("✓ User %s password reset", user.Email)
	c.JSON(http.StatusOK, gin.H{"message": "Password reset successful, please login with new password"})
}

// initUserDefaultConfigs initialize default model and exchange configurations for new user
func (s *Server) initUserDefaultConfigs(userID string) error {
	// Commented out automatic creation of default config, let users add manually
	// So new users won't automatically have config items after registration
	log.Printf("User %s registration completed, waiting for manual configuration of AI model and exchange", userID)
	return nil
}

// handleGetSupportedModels get system supported AI model list
func (s *Server) handleGetSupportedModels(c *gin.Context) {
	// Return static list of supported AI models
	supportedModels := []SupportedModel{
		{ID: "deepseek", Name: "DeepSeek", Provider: "deepseek", Enabled: false, CustomAPIURL: "", CustomModelName: ""},
		{ID: "qwen", Name: "Qwen", Provider: "qwen", Enabled: false, CustomAPIURL: "", CustomModelName: ""},
		{ID: "openai", Name: "OpenAI (GPT)", Provider: "openai", Enabled: false, CustomAPIURL: "", CustomModelName: ""},
		{ID: "claude", Name: "Claude", Provider: "claude", Enabled: false, CustomAPIURL: "", CustomModelName: ""},
		{ID: "gemini", Name: "Gemini", Provider: "gemini", Enabled: false, CustomAPIURL: "", CustomModelName: ""},
		{ID: "grok", Name: "Grok", Provider: "grok", Enabled: false, CustomAPIURL: "", CustomModelName: ""},
		{ID: "kimi", Name: "Kimi", Provider: "kimi", Enabled: false, CustomAPIURL: "", CustomModelName: ""},
	}

	c.JSON(http.StatusOK, supportedModels)
}

// handleGetSupportedExchanges get system supported exchange list
func (s *Server) handleGetSupportedExchanges(c *gin.Context) {
	// Return static list of supported exchanges
	supportedExchanges := []SafeExchangeConfig{
		{ID: "binance", Name: "Binance Futures", Type: "binance", Enabled: false, Testnet: false, HyperliquidWalletAddr: "", AsterUser: "", AsterSigner: "", LighterWalletAddr: ""},
		{ID: "bybit", Name: "Bybit Futures", Type: "bybit", Enabled: false, Testnet: false, HyperliquidWalletAddr: "", AsterUser: "", AsterSigner: "", LighterWalletAddr: ""},
		{ID: "okx", Name: "OKX Futures", Type: "okx", Enabled: false, Testnet: false, HyperliquidWalletAddr: "", AsterUser: "", AsterSigner: "", LighterWalletAddr: ""},
		{ID: "bitget", Name: "Bitget Futures", Type: "bitget", Enabled: false, Testnet: false, HyperliquidWalletAddr: "", AsterUser: "", AsterSigner: "", LighterWalletAddr: ""},
		{ID: "hyperliquid", Name: "Hyperliquid", Type: "hyperliquid", Enabled: false, Testnet: false, HyperliquidWalletAddr: "", AsterUser: "", AsterSigner: "", LighterWalletAddr: ""},
		{ID: "aster", Name: "Aster DEX", Type: "aster", Enabled: false, Testnet: false, HyperliquidWalletAddr: "", AsterUser: "", AsterSigner: "", LighterWalletAddr: ""},
		{ID: "lighter", Name: "Lighter DEX", Type: "lighter", Enabled: false, Testnet: false, HyperliquidWalletAddr: "", AsterUser: "", AsterSigner: "", LighterWalletAddr: ""},
	}

	c.JSON(http.StatusOK, supportedExchanges)
}

// handleGetDefaultStrategyConfig get default strategy configuration based on language
func (s *Server) handleGetDefaultStrategyConfig(c *gin.Context) {
	// Get language parameter from query string, default to "en"
	lang := c.DefaultQuery("lang", "en")

	// Validate language parameter
	if lang != "zh" && lang != "en" {
		lang = "en" // Default to English if invalid
	}

	// Get default strategy configuration
	config := GetDefaultStrategyConfig(lang)

	c.JSON(http.StatusOK, config)
}

// getDefaultAPIURLs returns default API URLs, reading from environment variables if set
func getDefaultAPIURLs() map[string]string {
	coinPoolURL := os.Getenv("COIN_POOL_API_URL")
	if coinPoolURL == "" {
		coinPoolURL = "http://nofxaios.com:30006/api/coinpool"
	}

	oiTopURL := os.Getenv("OI_TOP_API_URL")
	if oiTopURL == "" {
		oiTopURL = "http://nofxaios.com:30006/api/oi/top"
	}

	quantDataURL := os.Getenv("QUANT_DATA_API_URL")
	if quantDataURL == "" {
		quantDataURL = "http://nofxaios.com:30006/api/coin/{symbol}?include=netflow,oi,price"
	}

	return map[string]string{
		"coin_pool_url":  coinPoolURL,
		"oi_top_url":     oiTopURL,
		"quant_data_url": quantDataURL,
	}
}

// handleGetDefaultURLs get default URLs for data sources
func (s *Server) handleGetDefaultURLs(c *gin.Context) {
	// Return default URLs for Coin Pool, OI Top, and Quant Data APIs
	// URLs can be configured via environment variables: COIN_POOL_API_URL, OI_TOP_API_URL, QUANT_DATA_API_URL
	defaultURLs := getDefaultAPIURLs()
	c.JSON(http.StatusOK, defaultURLs)
}

// handleGetStrategies get all strategies for the current user
func (s *Server) handleGetStrategies(c *gin.Context) {
	userID := c.GetString("user_id")
	strategies, err := s.database.GetStrategies(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get strategies: %v", err)})
		return
	}
	c.JSON(http.StatusOK, strategies)
}

// handleGetStrategy get strategy by ID
func (s *Server) handleGetStrategy(c *gin.Context) {
	userID := c.GetString("user_id")
	strategyID := c.Param("id")
	strategy, err := s.database.GetStrategy(strategyID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Strategy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get strategy: %v", err)})
		return
	}
	c.JSON(http.StatusOK, strategy)
}

// handleCreateStrategy create new strategy
func (s *Server) handleCreateStrategy(c *gin.Context) {
	userID := c.GetString("user_id")
	var req CreateStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate strategy ID
	strategyID := fmt.Sprintf("strategy_%s_%d", userID, time.Now().UnixNano())

	// Set defaults
	if req.IndicatorTimeframe == "" {
		req.IndicatorTimeframe = "3m"
	}
	if req.BTCETHLeverage == 0 {
		req.BTCETHLeverage = 5
	}
	if req.AltcoinLeverage == 0 {
		req.AltcoinLeverage = 3
	}

	// Set default values for new configuration fields
	minRiskRewardRatio := 3.0
	if req.MinRiskRewardRatio != nil {
		minRiskRewardRatio = *req.MinRiskRewardRatio
	}
	maxPositions := 3
	if req.MaxPositions != nil {
		maxPositions = *req.MaxPositions
	}
	marginUsageLimit := 90.0
	if req.MarginUsageLimit != nil {
		marginUsageLimit = *req.MarginUsageLimit
	}
	minOpeningAmount := 12.0
	if req.MinOpeningAmount != nil {
		minOpeningAmount = *req.MinOpeningAmount
	}
	minOpeningAmountBTCETH := 60.0
	if req.MinOpeningAmountBTCETH != nil {
		minOpeningAmountBTCETH = *req.MinOpeningAmountBTCETH
	}
	altcoinPositionMin := 0.8
	if req.AltcoinPositionMin != nil {
		altcoinPositionMin = *req.AltcoinPositionMin
	}
	altcoinPositionMax := 1.5
	if req.AltcoinPositionMax != nil {
		altcoinPositionMax = *req.AltcoinPositionMax
	}
	btcEthPositionMin := 5.0
	if req.BTCETHPositionMin != nil {
		btcEthPositionMin = *req.BTCETHPositionMin
	}
	btcEthPositionMax := 10.0
	if req.BTCETHPositionMax != nil {
		btcEthPositionMax = *req.BTCETHPositionMax
	}
	availableMarginMultiplier := 0.88
	if req.AvailableMarginMultiplier != nil {
		availableMarginMultiplier = *req.AvailableMarginMultiplier
	}
	minConfidenceForEntry := 75
	if req.MinConfidenceForEntry != nil {
		minConfidenceForEntry = *req.MinConfidenceForEntry
	}
	minHoldingTimeMinutes := 30
	if req.MinHoldingTimeMinutes != nil {
		minHoldingTimeMinutes = *req.MinHoldingTimeMinutes
	}
	sharpeRatioConfig := ""
	if req.SharpeRatioConfig != nil {
		sharpeRatioConfig = *req.SharpeRatioConfig
	}

	strategy := &config.StrategyRecord{
		ID:                     strategyID,
		UserID:                 userID,
		Name:                   req.Name,
		Description:            req.Description,
		SystemPromptTemplate:   req.SystemPromptTemplate,
		CustomPrompt:           req.CustomPrompt,
		OverrideBasePrompt:     req.OverrideBasePrompt,
		BTCETHLeverage:         req.BTCETHLeverage,
		AltcoinLeverage:        req.AltcoinLeverage,
		TradingSymbols:         req.TradingSymbols,
		IsCrossMargin:          req.IsCrossMargin,
		UseCoinPool:            req.UseCoinPool,
		UseOITop:               req.UseOITop,
		UseTradingView:         req.UseTradingView,
		EnableRawKlines:        req.EnableRawKlines,
		EnableEMA:              req.EnableEMA,
		EnableMACD:             req.EnableMACD,
		EnableRSI:              req.EnableRSI,
		EnableATR:              req.EnableATR,
		EnableVolume:           req.EnableVolume,
		EnableOI:               req.EnableOI,
		EnableFunding:           req.EnableFunding,
		IndicatorTimeframe:     req.IndicatorTimeframe,
		QuantDataURL:           req.QuantDataURL,
		MinRiskRewardRatio:     minRiskRewardRatio,
		MaxPositions:            maxPositions,
		MarginUsageLimit:       marginUsageLimit,
		MinOpeningAmount:        minOpeningAmount,
		MinOpeningAmountBTCETH:  minOpeningAmountBTCETH,
		AltcoinPositionMin:     altcoinPositionMin,
		AltcoinPositionMax:     altcoinPositionMax,
		BTCETHPositionMin:      btcEthPositionMin,
		BTCETHPositionMax:      btcEthPositionMax,
		AvailableMarginMultiplier: availableMarginMultiplier,
		MinConfidenceForEntry:  minConfidenceForEntry,
		MinHoldingTimeMinutes:  minHoldingTimeMinutes,
		SharpeRatioConfig:      sharpeRatioConfig,
	}

	err := s.database.CreateStrategy(strategy)
	if err != nil {
		log.Printf("❌ Failed to create strategy: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create strategy: %v", err)})
		return
	}

	// Reload strategy from database to ensure we return exactly what was saved
	createdStrategy, err := s.database.GetStrategy(strategy.ID, userID)
	if err != nil {
		log.Printf("❌ Failed to reload strategy after creation: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to reload strategy: %v", err)})
		return
	}
	
	c.JSON(http.StatusCreated, createdStrategy)
}

// handleUpdateStrategy update strategy
func (s *Server) handleUpdateStrategy(c *gin.Context) {
	userID := c.GetString("user_id")
	strategyID := c.Param("id")
	var req UpdateStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing strategy to ensure it exists and belongs to user
	existingStrategy, err := s.database.GetStrategy(strategyID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Strategy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get strategy: %v", err)})
		return
	}

	// Update fields
	existingStrategy.Name = req.Name
	existingStrategy.Description = req.Description
	existingStrategy.SystemPromptTemplate = req.SystemPromptTemplate
	existingStrategy.CustomPrompt = req.CustomPrompt
	existingStrategy.OverrideBasePrompt = req.OverrideBasePrompt
	existingStrategy.BTCETHLeverage = req.BTCETHLeverage
	existingStrategy.AltcoinLeverage = req.AltcoinLeverage
	existingStrategy.TradingSymbols = req.TradingSymbols
	existingStrategy.IsCrossMargin = req.IsCrossMargin
	existingStrategy.UseCoinPool = req.UseCoinPool
	existingStrategy.UseOITop = req.UseOITop
	existingStrategy.UseTradingView = req.UseTradingView
	existingStrategy.EnableRawKlines = req.EnableRawKlines
	existingStrategy.EnableEMA = req.EnableEMA
	existingStrategy.EnableMACD = req.EnableMACD
	existingStrategy.EnableRSI = req.EnableRSI
	existingStrategy.EnableATR = req.EnableATR
	existingStrategy.EnableVolume = req.EnableVolume
	existingStrategy.EnableOI = req.EnableOI
	existingStrategy.EnableFunding = req.EnableFunding
	existingStrategy.IndicatorTimeframe = req.IndicatorTimeframe
	existingStrategy.QuantDataURL = req.QuantDataURL
	// Update new configuration fields (only if provided)
	if req.MinRiskRewardRatio != nil {
		existingStrategy.MinRiskRewardRatio = *req.MinRiskRewardRatio
	}
	if req.MaxPositions != nil {
		existingStrategy.MaxPositions = *req.MaxPositions
	}
	if req.MarginUsageLimit != nil {
		existingStrategy.MarginUsageLimit = *req.MarginUsageLimit
	}
	if req.MinOpeningAmount != nil {
		existingStrategy.MinOpeningAmount = *req.MinOpeningAmount
	}
	if req.MinOpeningAmountBTCETH != nil {
		existingStrategy.MinOpeningAmountBTCETH = *req.MinOpeningAmountBTCETH
	}
	if req.AltcoinPositionMin != nil {
		existingStrategy.AltcoinPositionMin = *req.AltcoinPositionMin
	}
	if req.AltcoinPositionMax != nil {
		existingStrategy.AltcoinPositionMax = *req.AltcoinPositionMax
	}
	if req.BTCETHPositionMin != nil {
		existingStrategy.BTCETHPositionMin = *req.BTCETHPositionMin
	}
	if req.BTCETHPositionMax != nil {
		existingStrategy.BTCETHPositionMax = *req.BTCETHPositionMax
	}
	if req.AvailableMarginMultiplier != nil {
		existingStrategy.AvailableMarginMultiplier = *req.AvailableMarginMultiplier
	}
	if req.MinConfidenceForEntry != nil {
		existingStrategy.MinConfidenceForEntry = *req.MinConfidenceForEntry
	}
	if req.MinHoldingTimeMinutes != nil {
		existingStrategy.MinHoldingTimeMinutes = *req.MinHoldingTimeMinutes
	}
	if req.SharpeRatioConfig != nil {
		existingStrategy.SharpeRatioConfig = *req.SharpeRatioConfig
	}

	err = s.database.UpdateStrategy(existingStrategy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update strategy: %v", err)})
		return
	}

	c.JSON(http.StatusOK, existingStrategy)
}

// handleDeleteStrategy delete strategy
func (s *Server) handleDeleteStrategy(c *gin.Context) {
	userID := c.GetString("user_id")
	strategyID := c.Param("id")
	err := s.database.DeleteStrategy(strategyID, userID)
	if err != nil {
		if strings.Contains(err.Error(), "cannot delete strategy") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete strategy: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Strategy deleted successfully"})
}

// handleImportStrategy import strategy from JSON
func (s *Server) handleImportStrategy(c *gin.Context) {
	userID := c.GetString("user_id")
	var req ImportStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Extract strategy data from JSON
	strategyData := req.StrategyData
	name, ok := strategyData["name"].(string)
	if !ok || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Strategy name is required"})
		return
	}

	// Generate new strategy ID
	strategyID := fmt.Sprintf("strategy_%s_%d", userID, time.Now().UnixNano())

	// Create strategy from imported data
	strategy := &config.StrategyRecord{
		ID:                   strategyID,
		UserID:               userID,
		Name:                 name,
		Description:          getStringFromMap(strategyData, "description"),
		SystemPromptTemplate: getStringFromMap(strategyData, "system_prompt_template"),
		CustomPrompt:         getStringFromMap(strategyData, "custom_prompt"),
		OverrideBasePrompt:   getBoolFromMap(strategyData, "override_base_prompt"),
		BTCETHLeverage:       getIntFromMap(strategyData, "btc_eth_leverage", 5),
		AltcoinLeverage:      getIntFromMap(strategyData, "altcoin_leverage", 3),
		TradingSymbols:       getStringFromMap(strategyData, "trading_symbols"),
		IsCrossMargin:        getBoolFromMap(strategyData, "is_cross_margin"),
		UseCoinPool:          getBoolFromMap(strategyData, "use_coin_pool"),
		UseOITop:             getBoolFromMap(strategyData, "use_oi_top"),
		UseTradingView:       getBoolFromMap(strategyData, "use_tradingview"),
		EnableRawKlines:      getBoolFromMap(strategyData, "enable_raw_klines"),
		EnableEMA:            getBoolFromMap(strategyData, "enable_ema"),
		EnableMACD:           getBoolFromMap(strategyData, "enable_macd"),
		EnableRSI:            getBoolFromMap(strategyData, "enable_rsi"),
		EnableATR:            getBoolFromMap(strategyData, "enable_atr"),
		EnableVolume:         getBoolFromMap(strategyData, "enable_volume"),
		EnableOI:             getBoolFromMap(strategyData, "enable_oi"),
		EnableFunding:        getBoolFromMap(strategyData, "enable_funding"),
		IndicatorTimeframe:   getStringFromMap(strategyData, "indicator_timeframe"),
		QuantDataURL:         getStringFromMap(strategyData, "quant_data_url"),
	}

	// Set defaults
	if strategy.IndicatorTimeframe == "" {
		strategy.IndicatorTimeframe = "3m"
	}

	err := s.database.CreateStrategy(strategy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to import strategy: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, strategy)
}

// handleExportStrategy export strategy as JSON
func (s *Server) handleExportStrategy(c *gin.Context) {
	userID := c.GetString("user_id")
	strategyID := c.Param("id")
	strategy, err := s.database.GetStrategy(strategyID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Strategy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get strategy: %v", err)})
		return
	}

	// Convert to map for JSON export (exclude user_id and timestamps for cleaner export)
	exportData := map[string]interface{}{
		"name":                   strategy.Name,
		"description":            strategy.Description,
		"system_prompt_template": strategy.SystemPromptTemplate,
		"custom_prompt":          strategy.CustomPrompt,
		"override_base_prompt":   strategy.OverrideBasePrompt,
		"btc_eth_leverage":       strategy.BTCETHLeverage,
		"altcoin_leverage":       strategy.AltcoinLeverage,
		"trading_symbols":        strategy.TradingSymbols,
		"is_cross_margin":        strategy.IsCrossMargin,
		"use_coin_pool":          strategy.UseCoinPool,
		"use_oi_top":             strategy.UseOITop,
		"use_tradingview":        strategy.UseTradingView,
		"enable_raw_klines":      strategy.EnableRawKlines,
		"enable_ema":             strategy.EnableEMA,
		"enable_macd":            strategy.EnableMACD,
		"enable_rsi":             strategy.EnableRSI,
		"enable_atr":             strategy.EnableATR,
		"enable_volume":          strategy.EnableVolume,
		"enable_oi":              strategy.EnableOI,
		"enable_funding":         strategy.EnableFunding,
		"indicator_timeframe":    strategy.IndicatorTimeframe,
		"quant_data_url":         strategy.QuantDataURL,
	}

	c.JSON(http.StatusOK, exportData)
}

// Helper functions for import
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getBoolFromMap(m map[string]interface{}, key string) bool {
	if val, ok := m[key].(bool); ok {
		return val
	}
	return false
}

func getIntFromMap(m map[string]interface{}, key string, defaultValue int) int {
	if val, ok := m[key].(float64); ok {
		return int(val)
	}
	return defaultValue
}

// Start start server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("🌐 API server started at http://localhost%s", addr)
	log.Printf("📊 API documentation:")
	log.Printf("  • GET  /api/health           - Health check")
	log.Printf("  • GET  /api/traders          - Public AI trader leaderboard top 50 (no auth required)")
	log.Printf("  • GET  /api/competition      - Public competition data (no auth required)")
	log.Printf("  • GET  /api/top-traders      - Top 5 trader data (no auth required, for performance comparison)")
	log.Printf("  • GET  /api/equity-history?trader_id=xxx - Public return history data (no auth required, for competition)")
	log.Printf("  • GET  /api/equity-history-batch?trader_ids=a,b,c - Batch get history data (no auth required, performance comparison optimization)")
	log.Printf("  • GET  /api/traders/:id/public-config - Public trader configuration (no auth required, no sensitive information)")
	log.Printf("  • POST /api/traders          - Create new AI trader")
	log.Printf("  • DELETE /api/traders/:id    - Delete AI trader")
	log.Printf("  • POST /api/traders/:id/start - Start AI trader")
	log.Printf("  • POST /api/traders/:id/stop  - Stop AI trader")
	log.Printf("  • GET  /api/models           - Get AI model configuration")
	log.Printf("  • PUT  /api/models           - Update AI model configuration")
	log.Printf("  • GET  /api/exchanges        - Get exchange configuration")
	log.Printf("  • PUT  /api/exchanges        - Update exchange configuration")
	log.Printf("  • GET  /api/status?trader_id=xxx     - Specified trader system status")
	log.Printf("  • GET  /api/account?trader_id=xxx    - Specified trader account information")
	log.Printf("  • GET  /api/positions?trader_id=xxx  - Specified trader position list")
	log.Printf("  • GET  /api/decisions?trader_id=xxx  - Specified trader decision logs")
	log.Printf("  • GET  /api/decisions/latest?trader_id=xxx - Specified trader latest decisions")
	log.Printf("  • GET  /api/statistics?trader_id=xxx - Specified trader statistics")
	log.Printf("  • GET  /api/performance?trader_id=xxx - Specified trader AI learning performance analysis")
	log.Printf("  • POST /api/webhook/tradingview      - TradingView webhook endpoint (no auth required, uses apikey)")
	log.Printf("  • GET  /api/webhook/tradingview      - Webhook endpoint info and testing (no auth required)")
	log.Println()

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shutdown server
func (s *Server) Shutdown() error {
	if s.httpServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// handleGetPromptTemplates get all system prompt template list (public, no auth required)
func (s *Server) handleGetPromptTemplates(c *gin.Context) {
	// Get templates from decision package (will merge database and filesystem)
	templates := decision.GetAllPromptTemplates()

	// Convert to response format
	response := make([]map[string]interface{}, 0, len(templates))
	for _, tmpl := range templates {
		response = append(response, map[string]interface{}{
			"name": tmpl.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": response,
	})
}

// handleGetPromptTemplate get prompt template content by name (public, no auth required)
func (s *Server) handleGetPromptTemplate(c *gin.Context) {
	templateName := c.Param("name")

	template, err := decision.GetPromptTemplate(templateName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Template does not exist: %s", templateName)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":    template.Name,
		"content": template.Content,
	})
}

// handleGetUserPromptTemplates get user prompt template list (including system templates and user-created templates)
func (s *Server) handleGetUserPromptTemplates(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	templates, err := s.database.GetPromptTemplates(userID)
	if err != nil {
		log.Printf("❌ Failed to get prompt template list: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get template list"})
		return
	}

	// Convert to response format
	response := make([]map[string]interface{}, 0, len(templates))
	for _, tmpl := range templates {
		response = append(response, map[string]interface{}{
			"id":         tmpl.ID,
			"name":       tmpl.Name,
			"content":    tmpl.Content,
			"is_system":  tmpl.IsSystem,
			"created_at": tmpl.CreatedAt.Format("2006-01-02 15:04:05"),
			"updated_at": tmpl.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": response,
	})
}

// handleGetUserPromptTemplate get specified prompt template
func (s *Server) handleGetUserPromptTemplate(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	templateID := c.Param("id")
	if templateID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Template ID cannot be empty"})
		return
	}

	template, err := s.database.GetPromptTemplate(userID, templateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Template does not exist: %s", templateID)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         template.ID,
		"name":       template.Name,
		"content":    template.Content,
		"is_system":  template.IsSystem,
		"created_at": template.CreatedAt.Format("2006-01-02 15:04:05"),
		"updated_at": template.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
}

// handleCreatePromptTemplate create new prompt template
func (s *Server) handleCreatePromptTemplate(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Name    string `json:"name" binding:"required"`
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Request parameter error: %v", err)})
		return
	}

	// Validate input
	req.Name = strings.TrimSpace(req.Name)
	req.Content = strings.TrimSpace(req.Content)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Template name cannot be empty"})
		return
	}
	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Template content cannot be empty"})
		return
	}

	// Generate base template ID (use userID_name format to ensure uniqueness)
	baseName := req.Name
	baseTemplateID := fmt.Sprintf("%s_%s", userID, strings.ToLower(strings.ReplaceAll(baseName, " ", "_")))

	// Try to create template, handling duplicates by auto-incrementing name
	templateID := baseTemplateID
	templateName := baseName
	maxAttempts := 100 // Prevent infinite loop
	attempt := 0

	for attempt < maxAttempts {
		err := s.database.CreatePromptTemplate(userID, templateID, templateName, req.Content, false)
		if err == nil {
			// Success - template created
			break
		}

		// Check if it's a duplicate ID error
		if errors.Is(err, config.ErrDuplicateTemplateID) {
			// Generate new name with suffix
			attempt++
			if attempt >= maxAttempts {
				log.Printf("❌ Failed to create prompt template after %d attempts: %v", maxAttempts, err)
				c.JSON(http.StatusConflict, gin.H{
					"error": fmt.Sprintf("Unable to create template. Too many templates with similar names exist. Please choose a more unique name."),
				})
				return
			}

			// Try with incremented suffix
			if attempt == 1 {
				templateName = fmt.Sprintf("%s (1)", baseName)
			} else {
				templateName = fmt.Sprintf("%s (%d)", baseName, attempt)
			}
			// Generate template ID: replace spaces with underscores, remove parentheses, convert to lowercase
			sanitizedName := strings.ToLower(strings.ReplaceAll(templateName, " ", "_"))
			sanitizedName = strings.ReplaceAll(sanitizedName, "(", "")
			sanitizedName = strings.ReplaceAll(sanitizedName, ")", "")
			templateID = fmt.Sprintf("%s_%s", userID, sanitizedName)
			continue
		}

		// Handle foreign key violation
		if errors.Is(err, config.ErrForeignKeyViolation) {
			log.Printf("❌ Failed to create prompt template - foreign key violation: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid user. Please log in again.",
			})
			return
		}

		// Other database errors
		log.Printf("❌ Failed to create prompt template: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to create template: %v", err),
		})
		return
	}

	// Get created template
	template, err := s.database.GetPromptTemplate(userID, templateID)
	if err != nil {
		log.Printf("❌ Failed to get created template: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Template created but failed to retrieve. Please refresh the template list."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         template.ID,
		"name":       template.Name,
		"content":    template.Content,
		"is_system":  template.IsSystem,
		"created_at": template.CreatedAt.Format("2006-01-02 15:04:05"),
		"updated_at": template.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
}

// handleUpdatePromptTemplate update prompt template (can only update user-created templates)
func (s *Server) handleUpdatePromptTemplate(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	templateID := c.Param("id")
	if templateID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Template ID cannot be empty"})
		return
	}

	var req struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Request parameter error: %v", err)})
		return
	}

	// At least one field needs to be updated
	if req.Name == "" && req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one of name or content must be provided"})
		return
	}

	// If only updating content, keep original name
	if req.Name == "" {
		existingTemplate, err := s.database.GetPromptTemplate(userID, templateID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Template does not exist"})
			return
		}
		req.Name = existingTemplate.Name
	}

	// If only updating name, keep original content
	if req.Content == "" {
		existingTemplate, err := s.database.GetPromptTemplate(userID, templateID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Template does not exist"})
			return
		}
		req.Content = existingTemplate.Content
	}

	err := s.database.UpdatePromptTemplate(userID, templateID, req.Name, req.Content)
	if err != nil {
		log.Printf("❌ Failed to update prompt template: %v", err)
		if strings.Contains(err.Error(), "cannot update system template") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot update system template"})
			return
		}
		if strings.Contains(err.Error(), "no permission to update") {
			c.JSON(http.StatusForbidden, gin.H{"error": "No permission to update this template"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update template"})
		return
	}

	// Get updated template
	template, err := s.database.GetPromptTemplate(userID, templateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get updated template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         template.ID,
		"name":       template.Name,
		"content":    template.Content,
		"is_system":  template.IsSystem,
		"created_at": template.CreatedAt.Format("2006-01-02 15:04:05"),
		"updated_at": template.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
}

// handleDeletePromptTemplate delete prompt template (can only delete user-created templates)
func (s *Server) handleDeletePromptTemplate(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	templateID := c.Param("id")
	if templateID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Template ID cannot be empty"})
		return
	}

	err := s.database.DeletePromptTemplate(userID, templateID)
	if err != nil {
		log.Printf("❌ Failed to delete prompt template: %v", err)
		if strings.Contains(err.Error(), "cannot delete system template") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete system template"})
			return
		}
		if strings.Contains(err.Error(), "no permission to delete") {
			c.JSON(http.StatusForbidden, gin.H{"error": "No permission to delete this template"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Template deleted"})
}

// handlePublicTraderList get public trader list (no auth required)
func (s *Server) handlePublicTraderList(c *gin.Context) {
	// Get trader information from all users
	competition, err := s.traderManager.GetCompetitionData(s.database)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get trader list: %v", err),
		})
		return
	}

	// Get traders array
	tradersData, exists := competition["traders"]
	if !exists {
		c.JSON(http.StatusOK, []map[string]interface{}{})
		return
	}

	traders, ok := tradersData.([]map[string]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Trader data format error",
		})
		return
	}

	// Return trader basic information, filter sensitive information
	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		result = append(result, map[string]interface{}{
			"trader_id":       trader["trader_id"],
			"trader_name":     trader["trader_name"],
			"ai_model":        trader["ai_model"],
			"exchange":        trader["exchange"],
			"is_running":      trader["is_running"],
			"total_equity":    trader["total_equity"],
			"total_pnl":       trader["total_pnl"],
			"total_pnl_pct":   trader["total_pnl_pct"],
			"position_count":  trader["position_count"],
			"margin_used_pct": trader["margin_used_pct"],
		})
	}

	c.JSON(http.StatusOK, result)
}

// handlePublicCompetition get public competition data (no auth required)
func (s *Server) handlePublicCompetition(c *gin.Context) {
	competition, err := s.traderManager.GetCompetitionData(s.database)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get competition data: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, competition)
}

// handleTopTraders get top 5 trader data (no auth required, for performance comparison)
func (s *Server) handleTopTraders(c *gin.Context) {
	topTraders, err := s.traderManager.GetTopTradersData(s.database)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get top 10 trader data: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, topTraders)
}

// handleEquityHistoryBatch batch get multiple traders' return history data (no auth required, for performance comparison)
func (s *Server) handleEquityHistoryBatch(c *gin.Context) {
	var requestBody struct {
		TraderIDs []string `json:"trader_ids"`
	}

	// Try to parse POST request JSON body
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		// If JSON parsing fails, try to get from query parameter (compatible with GET request)
		traderIDsParam := c.Query("trader_ids")
		if traderIDsParam == "" {
			// If trader_ids not specified, return top 5 historical data
			topTraders, err := s.traderManager.GetTopTradersData(s.database)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("Failed to get top 5 traders: %v", err),
				})
				return
			}

			traders, ok := topTraders["traders"].([]map[string]interface{})
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Trader data format error"})
				return
			}

			// Extract trader IDs
			traderIDs := make([]string, 0, len(traders))
			for _, trader := range traders {
				if traderID, ok := trader["trader_id"].(string); ok {
					traderIDs = append(traderIDs, traderID)
				}
			}

			result := s.getEquityHistoryForTraders(traderIDs)
			c.JSON(http.StatusOK, result)
			return
		}

		// Parse comma-separated trader IDs
		requestBody.TraderIDs = strings.Split(traderIDsParam, ",")
		for i := range requestBody.TraderIDs {
			requestBody.TraderIDs[i] = strings.TrimSpace(requestBody.TraderIDs[i])
		}
	}

	// Limit to maximum 20 traders to prevent oversized requests
	if len(requestBody.TraderIDs) > 20 {
		requestBody.TraderIDs = requestBody.TraderIDs[:20]
	}

	result := s.getEquityHistoryForTraders(requestBody.TraderIDs)
	c.JSON(http.StatusOK, result)
}

// getEquityHistoryForTraders get historical data for multiple traders
func (s *Server) getEquityHistoryForTraders(traderIDs []string) map[string]interface{} {
	result := make(map[string]interface{})
	histories := make(map[string]interface{})
	errors := make(map[string]string)

	// Generate consistent timestamp for all real-time data points in batch requests
	consistentTimestamp := time.Now().Format("2006-01-02 15:04:05")

	for _, traderID := range traderIDs {
		if traderID == "" {
			continue
		}

		trader, err := s.traderManager.GetTrader(traderID)
		if err != nil {
			errors[traderID] = "Trader does not exist"
			continue
		}

		// Try to get equity history from database first (persistent across deployments)
		var history []map[string]interface{}
		var records []*logger.DecisionRecord

		if s.database != nil {
			dbRecords, err := s.database.GetEquityHistory(traderID, 500)
			if err == nil && len(dbRecords) > 0 {
				// Convert database records to history format
				history = make([]map[string]interface{}, 0, len(dbRecords))
				for _, dbRec := range dbRecords {
					history = append(history, map[string]interface{}{
						"timestamp":     dbRec.Timestamp.Format("2006-01-02 15:04:05"),
						"total_equity":  dbRec.TotalEquity,
						"total_pnl":     dbRec.TotalPnL,
						"total_pnl_pct": dbRec.TotalPnLPct,
						"balance":       dbRec.TotalEquity - dbRec.TotalPnL, // Approximate balance
					})
				}
			}
		}

		// Fallback to file-based logs if database is empty or unavailable
		if len(history) == 0 {
			// Get historical data (for comparison display, limit data volume)
			records, err = trader.GetDecisionLogger().GetLatestRecords(500)
			if err != nil {
				errors[traderID] = fmt.Sprintf("Failed to get historical data: %v", err)
				continue
			}
		}

		// Get initial balance from database
		initialBalance := 0.0
		traderRecord, err := s.database.GetTraderByID(traderID)
		if err == nil && traderRecord != nil {
			initialBalance = traderRecord.InitialBalance
		}

		// Process file-based records if database was empty
		if len(history) == 0 && len(records) > 0 {
			// Fallback to first snapshot equity if initial_balance not set
			if initialBalance == 0 {
				firstRecord := records[0]
				initialBalance = firstRecord.AccountState.TotalBalance + firstRecord.AccountState.TotalUnrealizedProfit
			}

			// Build return history data
			history = make([]map[string]interface{}, 0, len(records))
			for _, record := range records {
				// Calculate total equity (balance + unrealized PnL)
				totalEquity := record.AccountState.TotalBalance + record.AccountState.TotalUnrealizedProfit

				// Calculate total PnL percentage using initial_balance
				totalPnLPct := 0.0
				if initialBalance > 0 {
					totalPnL := totalEquity - initialBalance
					totalPnLPct = (totalPnL / initialBalance) * 100
				}

				history = append(history, map[string]interface{}{
					"timestamp":     record.Timestamp.Format("2006-01-02 15:04:05"),
					"total_equity":  totalEquity,
					"total_pnl":     totalEquity - initialBalance,
					"total_pnl_pct": totalPnLPct,
					"balance":       record.AccountState.TotalBalance,
				})
			}
		}

		// Add real-time data point from current account balance for chart/leaderboard consistency
		accountInfo, err := trader.GetAccountInfo()
		if err == nil {
			currentTotalEquity, _ := accountInfo["total_equity"].(float64)
			currentTotalPnL, _ := accountInfo["total_pnl"].(float64)
			currentTotalPnLPct, _ := accountInfo["total_pnl_pct"].(float64)

			// Calculate PnL percentage if not provided or recalculate using initial balance
			if currentTotalPnLPct == 0 && initialBalance > 0 && currentTotalEquity > 0 {
				currentTotalPnL = currentTotalEquity - initialBalance
				currentTotalPnLPct = (currentTotalPnL / initialBalance) * 100
			}

			// Use consistent timestamp for all real-time data points in batch requests
			history = append(history, map[string]interface{}{
				"timestamp":     consistentTimestamp,
				"total_equity":  currentTotalEquity,
				"total_pnl":     currentTotalPnL,
				"total_pnl_pct": currentTotalPnLPct,
				"balance":       currentTotalEquity - currentTotalPnL, // Approximate balance
			})
		}

		histories[traderID] = history
	}

	result["histories"] = histories
	result["count"] = len(histories)
	if len(errors) > 0 {
		result["errors"] = errors
	}

	return result
}

// handleGetPublicTraderConfig get public trader configuration information (no auth required, no sensitive information)
func (s *Server) handleGetPublicTraderConfig(c *gin.Context) {
	traderID := c.Param("id")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trader ID cannot be empty"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trader does not exist"})
		return
	}

	// Get trader status information
	status := trader.GetStatus()

	// Only return public configuration information, no sensitive data like API keys
	result := map[string]interface{}{
		"trader_id":   trader.GetID(),
		"trader_name": trader.GetName(),
		"ai_model":    trader.GetAIModel(),
		"exchange":    trader.GetExchange(),
		"is_running":  status["is_running"],
		"ai_provider": status["ai_provider"],
		"start_time":  status["start_time"],
	}

	c.JSON(http.StatusOK, result)
}
