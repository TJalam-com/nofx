package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"nofx/auth"
	"nofx/backtest"
	"nofx/config"
	"nofx/crypto"
	"nofx/decision"
	"nofx/manager"
	"nofx/trader"
	"os"
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
		if _, err := os.Stat("./web/dist/sitemap.xml"); err == nil {
			s.router.StaticFile("/sitemap.xml", "./web/dist/sitemap.xml")
		}

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

		// System config (no authentication required, for frontend to determine admin mode/registration status)
		api.GET("/config", s.handleGetSystemConfig)

		// Crypto related endpoints (no authentication required)
		api.GET("/crypto/public-key", s.cryptoHandler.HandleGetPublicKey)
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

		// Authentication related routes (no authentication required)
		api.POST("/register", s.handleRegister)
		api.POST("/login", s.handleLogin)
		api.POST("/verify-otp", s.handleVerifyOTP)
		api.POST("/complete-registration", s.handleCompleteRegistration)
		api.POST("/reset-password", s.handleResetPassword)

		// Webhook routes (no authentication required, verified via apikey)
		api.POST("/webhook/tradingview", s.handleTradingViewWebhook)
		log.Println("✅ Webhook routes registered: POST /api/webhook/tradingview")

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
			}
			log.Println("✅ Backtest routes registered: /api/backtest/* (non-follower users only)")

			// Specific trader data (using query parameter ?trader_id=xxx)
			protected.GET("/status", s.handleStatus)
			protected.GET("/account", s.handleAccount)
			protected.GET("/positions", s.handlePositions)
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
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   c.Request.Context().Value("time"),
	})
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
	Name                 string  `json:"name" binding:"required"`
	AIModelID            string  `json:"ai_model_id" binding:"required"`
	ExchangeID           string  `json:"exchange_id" binding:"required"`
	InitialBalance       float64 `json:"initial_balance"`
	ScanIntervalMinutes  int     `json:"scan_interval_minutes"`
	BTCETHLeverage       int     `json:"btc_eth_leverage"`
	AltcoinLeverage      int     `json:"altcoin_leverage"`
	TradingSymbols       string  `json:"trading_symbols"`
	CustomPrompt         string  `json:"custom_prompt"`
	OverrideBasePrompt   bool    `json:"override_base_prompt"`
	SystemPromptTemplate string  `json:"system_prompt_template"` // System prompt template name
	IsCrossMargin        *bool   `json:"is_cross_margin"`        // Pointer type, nil means use default value true
	UseCoinPool          bool    `json:"use_coin_pool"`
	UseOITop             bool    `json:"use_oi_top"`
	UseTradingView       bool    `json:"use_tradingview"`
	FollowedTraderID     string  `json:"followed_trader_id"` // Followed trader ID (for follower role)
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

	// Generate trader ID
	traderID := fmt.Sprintf("%s_%s_%d", req.ExchangeID, req.AIModelID, time.Now().Unix())

	// Set default values
	isCrossMargin := true // Default to cross margin mode
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
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
				// Extract available balance - support multiple field name formats
				if availableBalance, ok := balanceInfo["availableBalance"].(float64); ok && availableBalance > 0 {
					// Binance format: availableBalance (camelCase)
					actualBalance = availableBalance
					log.Printf("✓ Queried exchange actual balance: %.2f USDT (user input: %.2f USDT)", actualBalance, req.InitialBalance)
				} else if availableBalance, ok := balanceInfo["available_balance"].(float64); ok && availableBalance > 0 {
					// Other format: available_balance (snake_case)
					actualBalance = availableBalance
					log.Printf("✓ Queried exchange actual balance: %.2f USDT (user input: %.2f USDT)", actualBalance, req.InitialBalance)
				} else if totalBalance, ok := balanceInfo["totalWalletBalance"].(float64); ok && totalBalance > 0 {
					// Binance format: totalWalletBalance (camelCase)
					actualBalance = totalBalance
					log.Printf("✓ Queried exchange total balance: %.2f USDT (user input: %.2f USDT)", actualBalance, req.InitialBalance)
				} else if totalBalance, ok := balanceInfo["balance"].(float64); ok && totalBalance > 0 {
					// Other format: balance
					actualBalance = totalBalance
					log.Printf("✓ Queried exchange actual balance: %.2f USDT (user input: %.2f USDT)", actualBalance, req.InitialBalance)
				} else {
					log.Printf("⚠️ Unable to extract available balance from balance info, balanceInfo=%v, using user input initial balance", balanceInfo)
				}
			}
		}
	}

	// Create trader configuration (database entity)
	log.Printf("🔧 DEBUG [CreateTrader]: Starting to create trader configuration, ID=%s, Name=%s, AIModel=%s, Exchange=%s, FollowedTraderID='%s', SystemPromptTemplate='%s'", traderID, req.Name, req.AIModelID, req.ExchangeID, req.FollowedTraderID, systemPromptTemplate)
	trader := &config.TraderRecord{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		InitialBalance:       actualBalance, // Use actual queried balance
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       req.TradingSymbols,
		UseCoinPool:          req.UseCoinPool,
		UseOITop:             req.UseOITop,
		UseTradingView:       req.UseTradingView,
		FollowedTraderID:     req.FollowedTraderID,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: systemPromptTemplate,
		IsCrossMargin:        isCrossMargin,
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            false,
	}

	// Save to database
	log.Printf("🔧 DEBUG: Preparing to call CreateTrader")
	err = s.database.CreateTrader(trader)
	if err != nil {
		log.Printf("❌ Failed to create trader: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create trader: %v", err)})
		return
	}
	log.Printf("✓ DEBUG [CreateTrader]: Trader saved to database successfully, FollowedTraderID='%s', SystemPromptTemplate='%s'", trader.FollowedTraderID, trader.SystemPromptTemplate)

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

	// Set system prompt template, keep original value if empty
	log.Printf("🔍 DEBUG [UpdateTrader]: Received system_prompt_template: '%s' (existing: '%s')", req.SystemPromptTemplate, existingTrader.SystemPromptTemplate)
	systemPromptTemplate := req.SystemPromptTemplate
	if systemPromptTemplate == "" {
		systemPromptTemplate = existingTrader.SystemPromptTemplate // Keep original value
		log.Printf("⚠️ DEBUG [UpdateTrader]: system_prompt_template was empty, keeping existing value: '%s'", systemPromptTemplate)
	} else {
		log.Printf("✓ DEBUG [UpdateTrader]: Updating system_prompt_template to: '%s'", systemPromptTemplate)
	}

	// Update trader configuration
	trader := &config.TraderRecord{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		InitialBalance:       req.InitialBalance,
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       req.TradingSymbols,
		UseCoinPool:          req.UseCoinPool,
		UseOITop:             req.UseOITop,
		UseTradingView:       req.UseTradingView,
		FollowedTraderID:     req.FollowedTraderID,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: systemPromptTemplate,
		IsCrossMargin:        isCrossMargin,
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            existingTrader.IsRunning, // Keep original value
	}

	// Update database
	err = s.database.UpdateTrader(trader)
	if err != nil {
		log.Printf("❌ DEBUG [UpdateTrader]: Database update failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update trader: %v", err)})
		return
	}
	log.Printf("✓ DEBUG [UpdateTrader]: Database update succeeded for trader %s, system_prompt_template: '%s'", traderID, systemPromptTemplate)

	// Reload trader into memory to ensure latest configuration (including UseTradingView) takes effect
	if reloadErr := s.traderManager.ReloadTraderFromDB(s.database, userID, traderID); reloadErr != nil {
		log.Printf("⚠️ Failed to reload trader into memory: %v", reloadErr)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to load latest trader configuration, please try again later"})
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

	// Extract available balance
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

// handleUpdateModelConfigs update AI model configurations (encrypted data only)
func (s *Server) handleUpdateModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")

	// Read raw request body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

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
	var req UpdateModelConfigRequest
	if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
		log.Printf("❌ Failed to parse decrypted data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
		return
	}
	log.Printf("🔓 Decrypted model configuration data (UserID: %s)", userID)

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
	}

	// Print full JSON response for debugging
	jsonData, _ := json.Marshal(exchanges)
	log.Printf("📤 Full JSON response: %s", string(jsonData))

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
		}
	}

	c.JSON(http.StatusOK, safeExchanges)
}

// handleUpdateExchangeConfigs update exchange configurations (encrypted data only)
func (s *Server) handleUpdateExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")

	// Read raw request body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

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
	var req UpdateExchangeConfigRequest
	if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
		log.Printf("❌ Failed to parse decrypted data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse decrypted data"})
		return
	}
	log.Printf("🔓 Decrypted exchange configuration data (UserID: %s)", userID)

	// Update each exchange's configuration
	for exchangeID, exchangeData := range req.Exchanges {
		err := s.database.UpdateExchange(userID, exchangeID, exchangeData.Enabled, exchangeData.APIKey, exchangeData.SecretKey, exchangeData.Testnet, exchangeData.HyperliquidWalletAddr, exchangeData.AsterUser, exchangeData.AsterSigner, exchangeData.AsterPrivateKey, exchangeData.LighterWalletAddr, exchangeData.LighterPrivateKey, exchangeData.LighterAPIKeyPrivateKey, exchangeData.OkxPassphrase)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update exchange %s: %v", exchangeID, err)})
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
			"trader_id":       trader.ID,
			"trader_name":     trader.Name,
			"ai_model":        trader.AIModelID, // Use complete ID
			"exchange_id":     trader.ExchangeID,
			"is_running":      isRunning,
			"initial_balance": trader.InitialBalance,
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
		"is_running":             isRunning,
	}

	log.Printf("🔍 DEBUG [handleGetTraderConfig]: Returning trader config - trader_id: %s, system_prompt_template: '%s' (original: '%s')", traderConfig.ID, systemPromptTemplate, traderConfig.SystemPromptTemplate)

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

// handleLatestDecisions latest decision logs (last 5, newest first)
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

	records, err := trader.GetDecisionLogger().GetLatestRecords(5)
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

	// Get as much historical data as possible (several days of data)
	// Every 3 minutes per cycle: 10000 records = approximately 20 days of data
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get historical data: %v", err),
		})
		return
	}

	// Build return history data points
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

	var history []EquityPoint
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

	// Analyze trading performance of last 100 cycles (to avoid losing trade records for long-term positions)
	// Assuming 3 minutes per cycle, 100 cycles = 5 hours, sufficient to cover most trades
	performance, err := trader.GetDecisionLogger().AnalyzePerformance(100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to analyze historical performance: %v", err),
		})
		return
	}

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

	// Ensure user's traders are loaded into memory
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ Failed to load user %s traders: %v", userID, err)
	}

	// Get all user's traders
	userTraders, err := s.database.GetTraders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get trader list: %v", err),
		})
		return
	}

	// Filter out parent traders (traders without followed_trader_id)
	var parentTraders []*config.TraderRecord
	for _, trader := range userTraders {
		if trader.FollowedTraderID == "" {
			parentTraders = append(parentTraders, trader)
		}
	}

	// Build response data
	parentTradersList := make([]gin.H, 0, len(parentTraders))

	for _, parentTrader := range parentTraders {
		// Get all followers of this parent trader
		followers, err := s.database.GetFollowerTraders(parentTrader.ID)
		if err != nil {
			log.Printf("⚠️ Failed to get trader %s follower list: %v", parentTrader.ID, err)
			continue
		}

		// If no followers, skip
		if len(followers) == 0 {
			continue
		}

		// Build follower list
		followersList := make([]gin.H, 0, len(followers))

		for _, followerRecord := range followers {
			// Get follower trader instance
			followerTrader, err := s.traderManager.GetTrader(followerRecord.ID)
			if err != nil {
				log.Printf("⚠️ Follower trader %s not in memory: %v", followerRecord.ID, err)
				// Still add basic info even if not in memory
				followersList = append(followersList, gin.H{
					"trader_id":   followerRecord.ID,
					"trader_name": followerRecord.Name,
					"user_id":     followerRecord.UserID,
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

			// Build follower information
			followerInfo := gin.H{
				"trader_id":        followerRecord.ID,
				"trader_name":      followerRecord.Name,
				"user_id":          followerRecord.UserID,
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

		parentTradersList = append(parentTradersList, parentInfo)
	}

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
	// Return system supported AI models (get from default user)
	models, err := s.database.GetAIModels("default")
	if err != nil {
		log.Printf("❌ Failed to get supported AI models: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get supported AI models"})
		return
	}

	c.JSON(http.StatusOK, models)
}

// handleGetSupportedExchanges get system supported exchange list
func (s *Server) handleGetSupportedExchanges(c *gin.Context) {
	// Return system supported exchanges (get from default user)
	exchanges, err := s.database.GetExchanges("default")
	if err != nil {
		log.Printf("❌ Failed to get supported exchanges: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get supported exchanges"})
		return
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
			HyperliquidWalletAddr: "", // Default config does not include wallet address
			AsterUser:             "", // Default config does not include user information
			AsterSigner:           "",
		}
	}

	c.JSON(http.StatusOK, safeExchanges)
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

	// Generate template ID (use userID_name format to ensure uniqueness)
	templateID := fmt.Sprintf("%s_%s", userID, strings.ToLower(strings.ReplaceAll(req.Name, " ", "_")))

	// Create template (user-created template, isSystem=false)
	err := s.database.CreatePromptTemplate(userID, templateID, req.Name, req.Content, false)
	if err != nil {
		log.Printf("❌ Failed to create prompt template: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create template"})
		return
	}

	// Get created template
	template, err := s.database.GetPromptTemplate(userID, templateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get created template"})
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
		if strings.Contains(err.Error(), "cannot update system template") || strings.Contains(err.Error(), "不能更新系统模板") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot update system template"})
			return
		}
		if strings.Contains(err.Error(), "no permission to update") || strings.Contains(err.Error(), "无权更新") {
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
		if strings.Contains(err.Error(), "cannot delete system template") || strings.Contains(err.Error(), "不能删除系统模板") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete system template"})
			return
		}
		if strings.Contains(err.Error(), "no permission to delete") || strings.Contains(err.Error(), "无权删除") {
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

	for _, traderID := range traderIDs {
		if traderID == "" {
			continue
		}

		trader, err := s.traderManager.GetTrader(traderID)
		if err != nil {
			errors[traderID] = "Trader does not exist"
			continue
		}

		// Get historical data (for comparison display, limit data volume)
		records, err := trader.GetDecisionLogger().GetLatestRecords(500)
		if err != nil {
			errors[traderID] = fmt.Sprintf("Failed to get historical data: %v", err)
			continue
		}

		// Build return history data
		history := make([]map[string]interface{}, 0, len(records))
		for _, record := range records {
			// Calculate total equity (balance + unrealized PnL)
			totalEquity := record.AccountState.TotalBalance + record.AccountState.TotalUnrealizedProfit

			history = append(history, map[string]interface{}{
				"timestamp":    record.Timestamp,
				"total_equity": totalEquity,
				"total_pnl":    record.AccountState.TotalUnrealizedProfit,
				"balance":      record.AccountState.TotalBalance,
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
