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

// Server HTTP API服务器
type Server struct {
	router          *gin.Engine
	traderManager   *manager.TraderManager
	database        *config.Database
	cryptoHandler   *CryptoHandler
	backtestManager *backtest.Manager
	httpServer      *http.Server
	port            int
}

// NewServer 创建API服务器
func NewServer(traderManager *manager.TraderManager, database *config.Database, cryptoService *crypto.CryptoService, backtestManager *backtest.Manager, port int) *Server {
	// 设置为Release模式（减少日志输出）
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	// 启用CORS
	router.Use(corsMiddleware())

	// 创建加密处理器
	cryptoHandler := NewCryptoHandler(cryptoService)

	s := &Server{
		router:          router,
		traderManager:   traderManager,
		database:        database,
		cryptoHandler:   cryptoHandler,
		backtestManager: backtestManager,
		port:            port,
	}

	// 设置路由
	s.setupRoutes()

	return s
}

// corsMiddleware CORS中间件
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

// setupRoutes 设置路由
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
		log.Println("✅ 静态文件服务已启用: web/dist")
	}

	// API路由组
	api := s.router.Group("/api")
	{
		// 健康检查
		api.Any("/health", s.handleHealth)

		// 管理员登录（管理员模式下使用，公共）

		// 系统支持的模型和交易所（无需认证）
		api.GET("/supported-models", s.handleGetSupportedModels)
		api.GET("/supported-exchanges", s.handleGetSupportedExchanges)

		// 系统配置（无需认证，用于前端判断是否管理员模式/注册是否开启）
		api.GET("/config", s.handleGetSystemConfig)

		// 加密相关接口（无需认证）
		api.GET("/crypto/public-key", s.cryptoHandler.HandleGetPublicKey)
		api.POST("/crypto/decrypt", s.cryptoHandler.HandleDecryptSensitiveData)

		// 系统提示词模板管理（无需认证，只读）- 仅返回系统模板名称列表
		api.GET("/prompt-templates", s.handleGetPromptTemplates)
		api.GET("/prompt-templates/:name", s.handleGetPromptTemplate)

		// 公开的竞赛数据（无需认证）
		api.GET("/traders", s.handlePublicTraderList)
		api.GET("/competition", s.handlePublicCompetition)
		api.GET("/top-traders", s.handleTopTraders)
		api.GET("/equity-history", s.handleEquityHistory)
		api.POST("/equity-history-batch", s.handleEquityHistoryBatch)
		api.GET("/traders/:id/public-config", s.handleGetPublicTraderConfig)

		// 认证相关路由（无需认证）
		api.POST("/register", s.handleRegister)
		api.POST("/login", s.handleLogin)
		api.POST("/verify-otp", s.handleVerifyOTP)
		api.POST("/complete-registration", s.handleCompleteRegistration)
		api.POST("/reset-password", s.handleResetPassword)

		// Webhook路由（无需认证，通过apikey验证）
		api.POST("/webhook/tradingview", s.handleTradingViewWebhook)
		log.Println("✅ Webhook路由已注册: POST /api/webhook/tradingview")

		// 需要认证的路由
		protected := api.Group("/", s.authMiddleware())
		{
			// 注销（加入黑名单）
			protected.POST("/logout", s.handleLogout)

			// 服务器IP查询（需要认证，用于白名单配置）
			protected.GET("/server-ip", s.handleGetServerIP)

			// AI交易员管理
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

			// AI模型配置
			protected.GET("/models", s.handleGetModelConfigs)
			protected.PUT("/models", s.handleUpdateModelConfigs)

			// 交易所配置
			protected.GET("/exchanges", s.handleGetExchangeConfigs)
			protected.PUT("/exchanges", s.handleUpdateExchangeConfigs)

			// Webhook管理（仅限非follower用户）- 必须在其他 /user 路由之前注册
			webhookGroup := protected.Group("/user", s.nonFollowerMiddleware())
			webhookGroup.GET("/webhook", s.handleGetWebhookInfo)
			webhookGroup.GET("/tradingview-alerts", s.handleGetRecentAlerts)
			webhookGroup.GET("/followers", s.handleGetUserFollowers)
			log.Println("✅ Webhook管理路由已注册: GET /api/user/webhook, GET /api/user/tradingview-alerts, GET /api/user/followers (仅限非follower用户)")

			// 获取当前用户信息（用于刷新角色等）
			protected.GET("/user/me", s.handleGetCurrentUser)

			// 用户信号源配置
			protected.GET("/user/signal-sources", s.handleGetUserSignalSource)
			protected.POST("/user/signal-sources", s.handleSaveUserSignalSource)

			// 提示词模板管理（需要认证）- 完整CRUD操作
			protected.GET("/user/prompt-templates", s.handleGetUserPromptTemplates)
			protected.GET("/user/prompt-templates/:id", s.handleGetUserPromptTemplate)
			protected.POST("/user/prompt-templates", s.handleCreatePromptTemplate)
			protected.PUT("/user/prompt-templates/:id", s.handleUpdatePromptTemplate)
			protected.DELETE("/user/prompt-templates/:id", s.handleDeletePromptTemplate)

			// Backtest路由（仅限非follower用户）
			backtestGroup := protected.Group("/backtest", s.nonFollowerMiddleware())
			s.registerBacktestRoutes(backtestGroup)

			// Admin路由（仅限admin用户）
			adminGroup := protected.Group("/admin", s.adminOnlyMiddleware())
			{
				adminGroup.GET("/traders", s.handleGetAllTraders)
				adminGroup.GET("/users", s.handleGetAllUsers)
				adminGroup.PUT("/users/:id/role", s.handleUpdateUserRole)
			}
			log.Println("✅ Backtest路由已注册: /api/backtest/* (仅限非follower用户)")

			// 指定trader的数据（使用query参数 ?trader_id=xxx）
			protected.GET("/status", s.handleStatus)
			protected.GET("/account", s.handleAccount)
			protected.GET("/positions", s.handlePositions)
			protected.POST("/positions/close", s.handleClosePosition)
			protected.GET("/decisions", s.handleDecisions)
			protected.GET("/decisions/latest", s.handleLatestDecisions)
			protected.GET("/statistics", s.handleStatistics)
			protected.GET("/performance", s.handlePerformance)

			// 复制交易相关接口
			protected.GET("/traders/:id/replication-status", s.handleGetReplicationStatus)
			protected.POST("/traders/:id/test-signal", s.handleTestSignal)
		}
	}
}

// handleHealth 健康检查
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   c.Request.Context().Value("time"),
	})
}

// handleGetSystemConfig 获取系统配置（客户端需要知道的配置）
func (s *Server) handleGetSystemConfig(c *gin.Context) {
	// 获取默认币种
	defaultCoinsStr, _ := s.database.GetSystemConfig("default_coins")
	var defaultCoins []string
	if defaultCoinsStr != "" {
		json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins)
	}
	if len(defaultCoins) == 0 {
		// 使用硬编码的默认币种
		defaultCoins = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"}
	}

	// 获取杠杆配置
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

	// 获取内测模式配置
	betaModeStr, _ := s.database.GetSystemConfig("beta_mode")
	betaMode := betaModeStr == "true"

	c.JSON(http.StatusOK, gin.H{
		"beta_mode":        betaMode,
		"default_coins":    defaultCoins,
		"btc_eth_leverage": btcEthLeverage,
		"altcoin_leverage": altcoinLeverage,
	})
}

// handleGetServerIP 获取服务器IP地址（用于白名单配置）
func (s *Server) handleGetServerIP(c *gin.Context) {
	// 尝试通过第三方API获取公网IP
	publicIP := getPublicIPFromAPI()

	// 如果第三方API失败，从网络接口获取第一个公网IP
	if publicIP == "" {
		publicIP = getPublicIPFromInterface()
	}

	// 如果还是没有获取到，返回错误
	if publicIP == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取公网IP地址"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"public_ip": publicIP,
		"message":   "请将此IP地址添加到白名单中",
	})
}

// getPublicIPFromAPI 通过第三方API获取公网IP
func getPublicIPFromAPI() string {
	// 尝试多个公网IP查询服务
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
			// 验证是否为有效的IP地址
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	return ""
}

// getPublicIPFromInterface 从网络接口获取第一个公网IP
func getPublicIPFromInterface() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		// 跳过未启用的接口和回环接口
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

			// 只考虑IPv4地址
			if ip.To4() != nil {
				ipStr := ip.String()
				// 排除私有IP地址范围
				if !isPrivateIP(ip) {
					return ipStr
				}
			}
		}
	}

	return ""
}

// isPrivateIP 判断是否为私有IP地址
func isPrivateIP(ip net.IP) bool {
	// 私有IP地址范围：
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

// getTraderFromQuery 从query参数获取trader
func (s *Server) getTraderFromQuery(c *gin.Context) (*manager.TraderManager, string, error) {
	userID := c.GetString("user_id")
	traderID := c.Query("trader_id")

	// 确保用户的交易员已加载到内存中
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, err)
	}

	if traderID == "" {
		// 如果没有指定trader_id，返回该用户的第一个trader
		ids := s.traderManager.GetTraderIDs()
		if len(ids) == 0 {
			return nil, "", fmt.Errorf("没有可用的trader")
		}

		// 获取用户的交易员列表，优先返回用户自己的交易员
		userTraders, err := s.database.GetTraders(userID)
		if err == nil && len(userTraders) > 0 {
			traderID = userTraders[0].ID
		} else {
			traderID = ids[0]
		}
	}

	return s.traderManager, traderID, nil
}

// AI交易员管理相关结构体
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
	SystemPromptTemplate string  `json:"system_prompt_template"` // 系统提示词模板名称
	IsCrossMargin        *bool   `json:"is_cross_margin"`        // 指针类型，nil表示使用默认值true
	UseCoinPool          bool    `json:"use_coin_pool"`
	UseOITop             bool    `json:"use_oi_top"`
	UseTradingView       bool    `json:"use_tradingview"`
	FollowedTraderID     string  `json:"followed_trader_id"` // 跟随的交易员ID（用于follower角色）
}

type ModelConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Enabled      bool   `json:"enabled"`
	APIKey       string `json:"apiKey,omitempty"`
	CustomAPIURL string `json:"customApiUrl,omitempty"`
}

// SafeModelConfig 安全的模型配置结构（不包含敏感信息）
type SafeModelConfig struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Provider        string `json:"provider"`
	Enabled         bool   `json:"enabled"`
	CustomAPIURL    string `json:"customApiUrl"`    // 自定义API URL（通常不敏感）
	CustomModelName string `json:"customModelName"` // 自定义模型名（不敏感）
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

// SafeExchangeConfig 安全的交易所配置结构（不包含敏感信息）
type SafeExchangeConfig struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Type                  string `json:"type"` // "cex" or "dex"
	Enabled               bool   `json:"enabled"`
	Testnet               bool   `json:"testnet,omitempty"`
	HyperliquidWalletAddr string `json:"hyperliquidWalletAddr"` // Hyperliquid钱包地址（不敏感）
	AsterUser             string `json:"asterUser"`             // Aster用户名（不敏感）
	AsterSigner           string `json:"asterSigner"`           // Aster签名者（不敏感）
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

// isPromptTemplateName 检查字符串是否是提示词模板名称而不是AI模型ID
func isPromptTemplateName(name string) bool {
	if name == "" {
		return false
	}
	// 已知的提示词模板名称
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

// getDefaultAIModelForUser 获取用户的默认AI模型ID
func (s *Server) getDefaultAIModelForUser(userID string) string {
	// 尝试获取用户启用的AI模型
	models, err := s.database.GetAIModels(userID)
	if err == nil {
		for _, model := range models {
			if model.Enabled {
				return model.ID
			}
		}
		// 如果没有启用的，返回第一个可用的
		if len(models) > 0 {
			return models[0].ID
		}
	}

	// 尝试从default用户获取
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

	// 最后回退到常见的模型ID
	return "deepseek" // 默认使用deepseek
}

// handleCreateTrader 创建新的AI交易员
func (s *Server) handleCreateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	var req CreateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Log the received request to debug followed_trader_id
	log.Printf("🔍 DEBUG [handleCreateTrader]: Received request - FollowedTraderID='%s', Name='%s', UserID='%s'", req.FollowedTraderID, req.Name, userID)

	// 如果提供了followed_trader_id，复制该交易员的所有设置
	var followedTraderUserID string
	if req.FollowedTraderID != "" {
		log.Printf("✓ DEBUG [handleCreateTrader]: FollowedTraderID is set: '%s'", req.FollowedTraderID)
		// 获取所有用户以查找被跟随的交易员
		allUserIDs, err := s.database.GetAllUsers()
		if err == nil {
			for _, uid := range allUserIDs {
				traders, err := s.database.GetTraders(uid)
				if err != nil {
					continue
				}
				for _, trader := range traders {
					if trader.ID == req.FollowedTraderID {
						// 找到被跟随的交易员，复制所有设置
						followedTraderUserID = trader.UserID
						if req.Name == "" {
							req.Name = trader.Name + " (Copy)"
						}

						// 检查AI模型ID是否是提示词模板名称
						if req.AIModelID == "" {
							if isPromptTemplateName(trader.AIModelID) {
								// 如果ai_model_id是提示词模板名称，将其映射到system_prompt_template
								log.Printf("⚠️ 检测到提示词模板名称作为AI模型ID: %s，将映射到system_prompt_template", trader.AIModelID)
								if req.SystemPromptTemplate == "" {
									req.SystemPromptTemplate = trader.AIModelID
								}
								// 使用默认AI模型
								req.AIModelID = s.getDefaultAIModelForUser(userID)
								log.Printf("✓ 已设置默认AI模型: %s", req.AIModelID)
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
						// 设置system_prompt_template
						// 如果ai_model_id已经被映射到system_prompt_template，保持那个值
						// 否则使用被跟随交易员的system_prompt_template
						if req.SystemPromptTemplate == "" {
							if trader.SystemPromptTemplate != "" {
								req.SystemPromptTemplate = trader.SystemPromptTemplate
							} else if isPromptTemplateName(trader.AIModelID) {
								// 如果被跟随交易员的ai_model_id是提示词模板，但system_prompt_template为空，使用ai_model_id
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

		// 自动确保跟随者用户有所需的AI模型和交易所配置
		if followedTraderUserID != "" && req.AIModelID != "" {
			// 检查AI模型是否存在，不存在则复制
			if err := s.database.CopyAIModelToUser(followedTraderUserID, userID, req.AIModelID); err != nil {
				log.Printf("⚠️ 自动创建AI模型配置失败: %v (用户可能需要手动配置)", err)
				// 不返回错误，允许用户稍后手动配置
			}
		}

		if followedTraderUserID != "" && req.ExchangeID != "" {
			// 检查交易所是否存在，不存在则复制
			if err := s.database.CopyExchangeToUser(followedTraderUserID, userID, req.ExchangeID); err != nil {
				log.Printf("⚠️ 自动创建交易所配置失败: %v (用户可能需要手动配置)", err)
				// 不返回错误，允许用户稍后手动配置
			}
		}
	}

	// 校验杠杆值
	if req.BTCETHLeverage < 0 || req.BTCETHLeverage > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "BTC/ETH杠杆必须在1-50倍之间"})
		return
	}
	if req.AltcoinLeverage < 0 || req.AltcoinLeverage > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "山寨币杠杆必须在1-20倍之间"})
		return
	}

	// 校验交易币种格式
	if req.TradingSymbols != "" {
		symbols := strings.Split(req.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" && !strings.HasSuffix(strings.ToUpper(symbol), "USDT") {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的币种格式: %s，必须以USDT结尾", symbol)})
				return
			}
		}
	}

	// 生成交易员ID
	traderID := fmt.Sprintf("%s_%s_%d", req.ExchangeID, req.AIModelID, time.Now().Unix())

	// 设置默认值
	isCrossMargin := true // 默认为全仓模式
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	// 设置杠杆默认值（从系统配置获取）
	btcEthLeverage := 5
	altcoinLeverage := 5
	if req.BTCETHLeverage > 0 {
		btcEthLeverage = req.BTCETHLeverage
	} else {
		// 从系统配置获取默认值
		if btcEthLeverageStr, _ := s.database.GetSystemConfig("btc_eth_leverage"); btcEthLeverageStr != "" {
			if val, err := strconv.Atoi(btcEthLeverageStr); err == nil && val > 0 {
				btcEthLeverage = val
			}
		}
	}
	if req.AltcoinLeverage > 0 {
		altcoinLeverage = req.AltcoinLeverage
	} else {
		// 从系统配置获取默认值
		if altcoinLeverageStr, _ := s.database.GetSystemConfig("altcoin_leverage"); altcoinLeverageStr != "" {
			if val, err := strconv.Atoi(altcoinLeverageStr); err == nil && val > 0 {
				altcoinLeverage = val
			}
		}
	}

	// 设置系统提示词模板默认值
	systemPromptTemplate := "default"
	if req.SystemPromptTemplate != "" {
		systemPromptTemplate = req.SystemPromptTemplate
	}

	// 设置扫描间隔默认值
	scanIntervalMinutes := req.ScanIntervalMinutes
	if scanIntervalMinutes < 3 {
		scanIntervalMinutes = 3 // 默认3分钟，且不允许小于3
	}

	// ✨ 查询交易所实际余额，覆盖用户输入
	actualBalance := req.InitialBalance // 默认使用用户输入
	exchanges, err := s.database.GetExchanges(userID)
	if err != nil {
		log.Printf("⚠️ 获取交易所配置失败，使用用户输入的初始资金: %v", err)
	}

	// 查找匹配的交易所配置
	var exchangeCfg *config.ExchangeConfig
	for _, ex := range exchanges {
		if ex.ID == req.ExchangeID {
			exchangeCfg = ex
			break
		}
	}

	if exchangeCfg == nil {
		log.Printf("⚠️ 未找到交易所 %s 的配置，使用用户输入的初始资金", req.ExchangeID)
	} else if !exchangeCfg.Enabled {
		log.Printf("⚠️ 交易所 %s 未启用，使用用户输入的初始资金", req.ExchangeID)
	} else {
		// 根据交易所类型创建临时 trader 查询余额
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
			log.Printf("⚠️ 不支持的交易所类型: %s，使用用户输入的初始资金", req.ExchangeID)
		}

		if createErr != nil {
			log.Printf("⚠️ 创建临时 trader 失败，使用用户输入的初始资金: %v", createErr)
		} else if tempTrader != nil {
			// 查询实际余额
			balanceInfo, balanceErr := tempTrader.GetBalance()
			if balanceErr != nil {
				log.Printf("⚠️ 查询交易所余额失败，使用用户输入的初始资金: %v", balanceErr)
			} else {
				// 提取可用余额 - 支持多种字段名格式
				if availableBalance, ok := balanceInfo["availableBalance"].(float64); ok && availableBalance > 0 {
					// Binance 格式: availableBalance (camelCase)
					actualBalance = availableBalance
					log.Printf("✓ 查询到交易所实际余额: %.2f USDT (用户输入: %.2f USDT)", actualBalance, req.InitialBalance)
				} else if availableBalance, ok := balanceInfo["available_balance"].(float64); ok && availableBalance > 0 {
					// 其他格式: available_balance (snake_case)
					actualBalance = availableBalance
					log.Printf("✓ 查询到交易所实际余额: %.2f USDT (用户输入: %.2f USDT)", actualBalance, req.InitialBalance)
				} else if totalBalance, ok := balanceInfo["totalWalletBalance"].(float64); ok && totalBalance > 0 {
					// Binance 格式: totalWalletBalance (camelCase)
					actualBalance = totalBalance
					log.Printf("✓ 查询到交易所总余额: %.2f USDT (用户输入: %.2f USDT)", actualBalance, req.InitialBalance)
				} else if totalBalance, ok := balanceInfo["balance"].(float64); ok && totalBalance > 0 {
					// 其他格式: balance
					actualBalance = totalBalance
					log.Printf("✓ 查询到交易所实际余额: %.2f USDT (用户输入: %.2f USDT)", actualBalance, req.InitialBalance)
				} else {
					log.Printf("⚠️ 无法从余额信息中提取可用余额，balanceInfo=%v，使用用户输入的初始资金", balanceInfo)
				}
			}
		}
	}

	// 创建交易员配置（数据库实体）
	log.Printf("🔧 DEBUG [CreateTrader]: 开始创建交易员配置, ID=%s, Name=%s, AIModel=%s, Exchange=%s, FollowedTraderID='%s', SystemPromptTemplate='%s'", traderID, req.Name, req.AIModelID, req.ExchangeID, req.FollowedTraderID, systemPromptTemplate)
	trader := &config.TraderRecord{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		InitialBalance:       actualBalance, // 使用实际查询的余额
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

	// 保存到数据库
	log.Printf("🔧 DEBUG: 准备调用 CreateTrader")
	err = s.database.CreateTrader(trader)
	if err != nil {
		log.Printf("❌ 创建交易员失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建交易员失败: %v", err)})
		return
	}
	log.Printf("✓ DEBUG [CreateTrader]: Trader saved to database successfully, FollowedTraderID='%s', SystemPromptTemplate='%s'", trader.FollowedTraderID, trader.SystemPromptTemplate)

	// 使竞赛缓存失效，确保新交易员（包括跟随者）能正确显示/过滤
	s.traderManager.InvalidateCompetitionCache()
	log.Printf("✓ DEBUG [CreateTrader]: Competition cache invalidated after creating trader")

	// 立即将新交易员加载到TraderManager中
	log.Printf("🔧 DEBUG: 准备调用 LoadUserTraders")
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户交易员到内存失败: %v", err)
		// 这里不返回错误，因为交易员已经成功创建到数据库
	}
	log.Printf("🔧 DEBUG: LoadUserTraders 完成")

	log.Printf("✓ 创建交易员成功: %s (模型: %s, 交易所: %s)", req.Name, req.AIModelID, req.ExchangeID)

	c.JSON(http.StatusCreated, gin.H{
		"trader_id":   traderID,
		"trader_name": req.Name,
		"ai_model":    req.AIModelID,
		"is_running":  false,
	})
}

// UpdateTraderRequest 更新交易员请求
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
	FollowedTraderID     string  `json:"followed_trader_id"` // 跟随的交易员ID（用于follower角色）
	CustomPrompt         string  `json:"custom_prompt"`
	OverrideBasePrompt   bool    `json:"override_base_prompt"`
	SystemPromptTemplate string  `json:"system_prompt_template"` // 系统提示词模板名称
	IsCrossMargin        *bool   `json:"is_cross_margin"`
}

// handleUpdateTrader 更新交易员配置
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

	// 检查交易员是否存在且属于当前用户
	traders, err := s.database.GetTraders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取交易员列表失败"})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 设置默认值
	isCrossMargin := existingTrader.IsCrossMargin // 保持原值
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	// 设置杠杆默认值
	btcEthLeverage := req.BTCETHLeverage
	altcoinLeverage := req.AltcoinLeverage
	if btcEthLeverage <= 0 {
		btcEthLeverage = existingTrader.BTCETHLeverage // 保持原值
	}
	if altcoinLeverage <= 0 {
		altcoinLeverage = existingTrader.AltcoinLeverage // 保持原值
	}

	// 设置扫描间隔，允许更新
	scanIntervalMinutes := req.ScanIntervalMinutes
	if scanIntervalMinutes <= 0 {
		scanIntervalMinutes = existingTrader.ScanIntervalMinutes // 保持原值
	} else if scanIntervalMinutes < 3 {
		scanIntervalMinutes = 3
	}

	// 设置系统提示词模板，如果为空则保持原值
	log.Printf("🔍 DEBUG [UpdateTrader]: Received system_prompt_template: '%s' (existing: '%s')", req.SystemPromptTemplate, existingTrader.SystemPromptTemplate)
	systemPromptTemplate := req.SystemPromptTemplate
	if systemPromptTemplate == "" {
		systemPromptTemplate = existingTrader.SystemPromptTemplate // 保持原值
		log.Printf("⚠️ DEBUG [UpdateTrader]: system_prompt_template was empty, keeping existing value: '%s'", systemPromptTemplate)
	} else {
		log.Printf("✓ DEBUG [UpdateTrader]: Updating system_prompt_template to: '%s'", systemPromptTemplate)
	}

	// 更新交易员配置
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
		IsRunning:            existingTrader.IsRunning, // 保持原值
	}

	// 更新数据库
	err = s.database.UpdateTrader(trader)
	if err != nil {
		log.Printf("❌ DEBUG [UpdateTrader]: Database update failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新交易员失败: %v", err)})
		return
	}
	log.Printf("✓ DEBUG [UpdateTrader]: Database update succeeded for trader %s, system_prompt_template: '%s'", traderID, systemPromptTemplate)

	// 重新加载交易员到内存，确保最新配置（含 UseTradingView）生效
	if reloadErr := s.traderManager.ReloadTraderFromDB(s.database, userID, traderID); reloadErr != nil {
		log.Printf("⚠️ 重新加载交易员到内存失败: %v", reloadErr)
	}

	log.Printf("INFO: Trader config reloaded into memory (trader=%s, UseTradingView=%v, scan_interval=%d)", traderID, req.UseTradingView, scanIntervalMinutes)

	// 如果之前在运行，重启以应用新配置
	if wasRunning {
		reloaded, getErr := s.traderManager.GetTrader(traderID)
		if getErr != nil {
			log.Printf("ERROR: Trader missing after reload, cannot restart (trader=%s, err=%v)", traderID, getErr)
		} else {
			go func() {
				log.Printf("▶️  重启交易员以应用新配置 %s (%s)", traderID, reloaded.GetName())
				if err := reloaded.Run(); err != nil {
					log.Printf("❌ 重启交易员 %s 运行错误: %v", reloaded.GetName(), err)
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

	log.Printf("✓ 更新交易员成功: %s (模型: %s, 交易所: %s)", req.Name, req.AIModelID, req.ExchangeID)

	c.JSON(http.StatusOK, gin.H{
		"trader_id":       traderID,
		"trader_name":     req.Name,
		"ai_model":        req.AIModelID,
		"message":         "交易员更新成功",
		"use_tradingview": req.UseTradingView,
		"scan_interval":   scanIntervalMinutes,
		"restarted":       wasRunning,
	})
}

// handleDeleteTrader 删除交易员
func (s *Server) handleDeleteTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 从数据库删除
	err := s.database.DeleteTrader(userID, traderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("删除交易员失败: %v", err)})
		return
	}

	// 如果交易员正在运行，先停止它
	if trader, err := s.traderManager.GetTrader(traderID); err == nil {
		status := trader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); ok && isRunning {
			trader.Stop()
			log.Printf("⏹  已停止运行中的交易员: %s", traderID)
		}
		// 从内存中移除交易员
		s.traderManager.RemoveTrader(traderID)
	}

	log.Printf("✓ 交易员已删除: %s", traderID)
	c.JSON(http.StatusOK, gin.H{"message": "交易员已删除"})
}

// handleStartTrader 启动交易员
func (s *Server) handleStartTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	isAdmin := roleStr == "admin"
	traderID := c.Param("id")

	// 如果是admin，需要获取trader的owner userID
	var traderOwnerID string
	if isAdmin {
		allTraders, err := s.database.GetAllTraders()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取交易员列表失败"})
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
			c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
			return
		}
	} else {
		traderOwnerID = userID
	}

	// 先获取交易员基本信息，检查是否需要修复配置
	traders, _ := s.database.GetTraders(traderOwnerID)
	for _, t := range traders {
		if t.ID == traderID {
			// 检查是否需要修复：如果ai_model_id是提示词模板名称
			if isPromptTemplateName(t.AIModelID) {
				log.Printf("🔧 检测到需要修复的交易员配置: traderID=%s, ai_model_id=%s (应为提示词模板)", traderID, t.AIModelID)

				// 修复：将ai_model_id映射到system_prompt_template，使用默认AI模型
				fixedAIModelID := s.getDefaultAIModelForUser(traderOwnerID)
				fixedSystemPromptTemplate := t.AIModelID

				// 如果system_prompt_template已经存在，使用它；否则使用ai_model_id的值
				if t.SystemPromptTemplate != "" {
					fixedSystemPromptTemplate = t.SystemPromptTemplate
				}

				log.Printf("🔧 修复交易员配置: ai_model_id: %s -> %s, system_prompt_template: %s",
					t.AIModelID, fixedAIModelID, fixedSystemPromptTemplate)

				// 更新交易员配置
				t.AIModelID = fixedAIModelID
				t.SystemPromptTemplate = fixedSystemPromptTemplate
				if updateErr := s.database.UpdateTrader(t); updateErr != nil {
					log.Printf("⚠️ 修复交易员配置失败: %v", updateErr)
				} else {
					log.Printf("✓ 已修复交易员配置")
				}
			}
			break
		}
	}

	// 校验交易员是否属于当前用户（或admin），并获取最新配置
	traderCfg, aiModelCfg, exchangeCfg, err := s.database.GetTraderConfig(traderOwnerID, traderID)
	if err != nil {
		// 添加详细错误日志以帮助诊断问题
		log.Printf("❌ [%s] GetTraderConfig失败: %v (traderID=%s, userID=%s)", userID, err, traderID, userID)

		// 尝试获取基本信息以提供更详细的错误信息
		traders, _ := s.database.GetTraders(traderOwnerID)
		traderExists := false
		var missingConfig []string
		for _, t := range traders {
			if t.ID == traderID {
				traderExists = true
				log.Printf("⚠️ 交易员存在但配置不完整: traderID=%s, ai_model_id=%s, exchange_id=%s", traderID, t.AIModelID, t.ExchangeID)

				// 检查AI模型是否存在
				_, err := s.database.GetAIModel(traderOwnerID, t.AIModelID)
				if err != nil {
					missingConfig = append(missingConfig, fmt.Sprintf("AI模型 '%s'", t.AIModelID))
				}

				// 检查交易所是否存在
				_, err = s.database.GetExchangeByID(traderOwnerID, t.ExchangeID)
				if err != nil {
					missingConfig = append(missingConfig, fmt.Sprintf("交易所 '%s'", t.ExchangeID))
				}
				break
			}
		}

		if !traderExists {
			c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在或无访问权限"})
		} else if len(missingConfig) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("交易员配置不完整：缺少 %s。请检查配置设置。", strings.Join(missingConfig, " 和 ")),
			})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在或无访问权限"})
		}
		return
	}

	// 记录成功获取配置的信息（用于调试）
	log.Printf("✓ [%s] 成功获取交易员配置: traderID=%s, ai_model=%s, exchange=%s", userID, traderID, aiModelCfg.Name, exchangeCfg.Name)

	// Stop old instance if running BEFORE reload
	if oldTrader, err := s.traderManager.GetTrader(traderID); err == nil {
		status := oldTrader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); ok && isRunning {
			log.Printf("⚠️ Stopping running trader before reload (trader=%s)", traderID)
			oldTrader.Stop()
			time.Sleep(300 * time.Millisecond) // Wait for goroutine to stop
		}
	}

	// 始终用最新配置刷新内存实例（单个交易员强制重载）
	if reloadErr := s.traderManager.ReloadTraderFromDB(s.database, traderOwnerID, traderID); reloadErr != nil {
		log.Printf("⚠️ 重新加载交易员失败，无法启动: %v", reloadErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载最新交易员配置，请稍后重试"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 检查交易员是否已经在运行
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员已在运行中"})
		return
	}

	modeDesc := "周期扫描模式"
	if traderCfg.UseTradingView {
		modeDesc = "TradingView webhook等待模式"
	}
	log.Printf("INFO: Start requested with fresh config (trader=%s, UseTradingView=%v, scan_interval=%d, mode=%s)", traderID, traderCfg.UseTradingView, traderCfg.ScanIntervalMinutes, modeDesc)

	// 启动交易员
	go func() {
		log.Printf("▶️  启动交易员 %s (%s)", traderID, trader.GetName())
		if err := trader.Run(); err != nil {
			log.Printf("❌ 交易员 %s 运行错误: %v", trader.GetName(), err)
			// 更新数据库状态为停止（启动失败）
			_ = s.database.UpdateTraderStatus(traderOwnerID, traderID, false)
		}
	}()

	// 等待交易员启动并验证状态
	time.Sleep(500 * time.Millisecond)
	status = trader.GetStatus()
	isRunning, _ := status["is_running"].(bool)

	// 更新数据库状态
	err = s.database.UpdateTraderStatus(traderOwnerID, traderID, isRunning)
	if err != nil {
		log.Printf("⚠️  更新交易员状态失败: %v", err)
	}

	log.Printf("✓ 交易员 %s 已启动 (运行状态: %v)", trader.GetName(), isRunning)
	c.JSON(http.StatusOK, gin.H{
		"message":         "交易员已启动",
		"is_running":      isRunning,
		"use_tradingview": traderCfg.UseTradingView,
		"scan_interval":   traderCfg.ScanIntervalMinutes,
	})
}

// handleStopTrader 停止交易员
func (s *Server) handleStopTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	isAdmin := roleStr == "admin"
	traderID := c.Param("id")

	// 如果是admin，需要获取trader的owner userID
	var traderOwnerID string
	if isAdmin {
		allTraders, err := s.database.GetAllTraders()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取交易员列表失败"})
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
			c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
			return
		}
	} else {
		traderOwnerID = userID
	}

	// 验证交易员是否存在（不要求完整配置，只需要确认交易员属于owner）
	traders, err := s.database.GetTraders(traderOwnerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取交易员列表失败"})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在或无访问权限"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 检查交易员是否正在运行
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && !isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员已停止"})
		return
	}

	// 停止交易员
	trader.Stop()

	// 等待交易员停止并验证状态
	time.Sleep(200 * time.Millisecond)
	status = trader.GetStatus()
	isRunning, _ := status["is_running"].(bool)

	// 明确将数据库状态设置为 false（用户明确停止）
	// 无论内存中的状态如何，数据库都应该反映用户的操作
	err = s.database.UpdateTraderStatus(traderOwnerID, traderID, false)
	if err != nil {
		log.Printf("⚠️  更新交易员状态失败: %v", err)
	}

	log.Printf("⏹  交易员 %s 已停止 (内存状态: %v, 数据库状态: false)", trader.GetName(), isRunning)
	c.JSON(http.StatusOK, gin.H{
		"message":    "交易员已停止",
		"is_running": false,
	})
}

// handleUpdateTraderPrompt 更新交易员自定义Prompt
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

	// 更新数据库
	err := s.database.UpdateTraderCustomPrompt(userID, traderID, req.CustomPrompt, req.OverrideBasePrompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新自定义prompt失败: %v", err)})
		return
	}

	// 如果trader在内存中，更新其custom prompt和override设置
	trader, err := s.traderManager.GetTrader(traderID)
	if err == nil {
		trader.SetCustomPrompt(req.CustomPrompt)
		trader.SetOverrideBasePrompt(req.OverrideBasePrompt)
		log.Printf("✓ 已更新交易员 %s 的自定义prompt (覆盖基础=%v)", trader.GetName(), req.OverrideBasePrompt)
	}

	c.JSON(http.StatusOK, gin.H{"message": "自定义prompt已更新"})
}

// handleSyncBalance 同步交易所余额到initial_balance（选项B：手动同步 + 选项C：智能检测）
func (s *Server) handleSyncBalance(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	log.Printf("🔄 用户 %s 请求同步交易员 %s 的余额", userID, traderID)

	// 从数据库获取交易员配置（包含交易所信息）
	traderConfig, _, exchangeCfg, err := s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	if exchangeCfg == nil || !exchangeCfg.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易所未配置或未启用"})
		return
	}

	// 创建临时 trader 查询余额
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的交易所类型"})
		return
	}

	if createErr != nil {
		log.Printf("⚠️ 创建临时 trader 失败: %v", createErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("连接交易所失败: %v", createErr)})
		return
	}

	// 查询实际余额
	balanceInfo, balanceErr := tempTrader.GetBalance()
	if balanceErr != nil {
		log.Printf("⚠️ 查询交易所余额失败: %v", balanceErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("查询余额失败: %v", balanceErr)})
		return
	}

	// 提取可用余额
	var actualBalance float64
	if availableBalance, ok := balanceInfo["available_balance"].(float64); ok && availableBalance > 0 {
		actualBalance = availableBalance
	} else if availableBalance, ok := balanceInfo["availableBalance"].(float64); ok && availableBalance > 0 {
		actualBalance = availableBalance
	} else if totalBalance, ok := balanceInfo["balance"].(float64); ok && totalBalance > 0 {
		actualBalance = totalBalance
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取可用余额"})
		return
	}

	oldBalance := traderConfig.InitialBalance

	// ✅ 选项C：智能检测余额变化
	changePercent := ((actualBalance - oldBalance) / oldBalance) * 100
	changeType := "增加"
	if changePercent < 0 {
		changeType = "减少"
	}

	log.Printf("✓ 查询到交易所实际余额: %.2f USDT (当前配置: %.2f USDT, 变化: %.2f%%)",
		actualBalance, oldBalance, changePercent)

	// 更新数据库中的 initial_balance
	err = s.database.UpdateTraderInitialBalance(userID, traderID, actualBalance)
	if err != nil {
		log.Printf("❌ 更新initial_balance失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新余额失败"})
		return
	}

	// 重新加载交易员到内存
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 重新加载用户交易员到内存失败: %v", err)
	}

	log.Printf("✅ 已同步余额: %.2f → %.2f USDT (%s %.2f%%)", oldBalance, actualBalance, changeType, changePercent)

	c.JSON(http.StatusOK, gin.H{
		"message":        "余额同步成功",
		"old_balance":    oldBalance,
		"new_balance":    actualBalance,
		"change_percent": changePercent,
		"change_type":    changeType,
	})
}

// handleGetModelConfigs 获取AI模型配置
func (s *Server) handleGetModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	userRole := c.GetString("role")
	log.Printf("🔍 查询用户 %s (角色: %s) 的AI模型配置", userID, userRole)
	models, err := s.database.GetAIModels(userID)
	if err != nil {
		log.Printf("❌ 获取AI模型配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取AI模型配置失败: %v", err)})
		return
	}
	log.Printf("✅ 找到 %d 个AI模型配置", len(models))

	// 转换为安全的响应结构，移除敏感信息
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

// handleUpdateModelConfigs 更新AI模型配置（仅支持加密数据）
func (s *Server) handleUpdateModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")

	// 读取原始请求体
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取请求体失败"})
		return
	}

	// 解析加密的 payload
	var encryptedPayload crypto.EncryptedPayload
	if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil {
		log.Printf("❌ 解析加密载荷失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误，必须使用加密传输"})
		return
	}

	// 验证是否为加密数据
	if encryptedPayload.WrappedKey == "" {
		log.Printf("❌ 检测到非加密请求 (UserID: %s)", userID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "此接口仅支持加密传输，请使用加密客户端",
			"code":    "ENCRYPTION_REQUIRED",
			"message": "Encrypted transmission is required for security reasons",
		})
		return
	}

	// 解密数据
	decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
	if err != nil {
		log.Printf("❌ 解密模型配置失败 (UserID: %s): %v", userID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "解密数据失败"})
		return
	}

	// 解析解密后的数据
	var req UpdateModelConfigRequest
	if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
		log.Printf("❌ 解析解密数据失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "解析解密数据失败"})
		return
	}
	log.Printf("🔓 已解密模型配置数据 (UserID: %s)", userID)

	// 更新每个模型的配置
	for modelID, modelData := range req.Models {
		err := s.database.UpdateAIModel(userID, modelID, modelData.Enabled, modelData.APIKey, modelData.CustomAPIURL, modelData.CustomModelName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新模型 %s 失败: %v", modelID, err)})
			return
		}
	}

	// 重新加载该用户的所有交易员，使新配置立即生效
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 重新加载用户交易员到内存失败: %v", err)
		// 这里不返回错误，因为模型配置已经成功更新到数据库
	}

	log.Printf("✓ AI模型配置已更新: %+v", req.Models)
	c.JSON(http.StatusOK, gin.H{"message": "模型配置已更新"})
}

// handleGetExchangeConfigs 获取交易所配置
func (s *Server) handleGetExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	userRole := c.GetString("role")
	log.Printf("🔍 查询用户 %s (角色: %s) 的交易所配置", userID, userRole)
	exchanges, err := s.database.GetExchanges(userID)
	if err != nil {
		log.Printf("❌ 获取交易所配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取交易所配置失败: %v", err)})
		return
	}
	log.Printf("✅ 找到 %d 个交易所配置", len(exchanges))

	// 调试：输出配置详情（脱敏）
	for _, ex := range exchanges {
		apiKeyMasked := ""
		if len(ex.APIKey) > 8 {
			apiKeyMasked = ex.APIKey[:8] + "..."
		}
		secretKeyMasked := ""
		if len(ex.SecretKey) > 8 {
			secretKeyMasked = ex.SecretKey[:8] + "..."
		}
		log.Printf("   └─ 交易所: %s, APIKey: %s, SecretKey: %s", ex.ID, apiKeyMasked, secretKeyMasked)
	}

	// 打印完整JSON响应用于调试
	jsonData, _ := json.Marshal(exchanges)
	log.Printf("📤 完整JSON响应: %s", string(jsonData))

	// 转换为安全的响应结构，移除敏感信息
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

// handleUpdateExchangeConfigs 更新交易所配置（仅支持加密数据）
func (s *Server) handleUpdateExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")

	// 读取原始请求体
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取请求体失败"})
		return
	}

	// 解析加密的 payload
	var encryptedPayload crypto.EncryptedPayload
	if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil {
		log.Printf("❌ 解析加密载荷失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误，必须使用加密传输"})
		return
	}

	// 验证是否为加密数据
	if encryptedPayload.WrappedKey == "" {
		log.Printf("❌ 检测到非加密请求 (UserID: %s)", userID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "此接口仅支持加密传输，请使用加密客户端",
			"code":    "ENCRYPTION_REQUIRED",
			"message": "Encrypted transmission is required for security reasons",
		})
		return
	}

	// 解密数据
	decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
	if err != nil {
		log.Printf("❌ 解密交易所配置失败 (UserID: %s): %v", userID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "解密数据失败"})
		return
	}

	// 解析解密后的数据
	var req UpdateExchangeConfigRequest
	if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
		log.Printf("❌ 解析解密数据失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "解析解密数据失败"})
		return
	}
	log.Printf("🔓 已解密交易所配置数据 (UserID: %s)", userID)

	// 更新每个交易所的配置
	for exchangeID, exchangeData := range req.Exchanges {
		err := s.database.UpdateExchange(userID, exchangeID, exchangeData.Enabled, exchangeData.APIKey, exchangeData.SecretKey, exchangeData.Testnet, exchangeData.HyperliquidWalletAddr, exchangeData.AsterUser, exchangeData.AsterSigner, exchangeData.AsterPrivateKey, exchangeData.LighterWalletAddr, exchangeData.LighterPrivateKey, exchangeData.LighterAPIKeyPrivateKey, exchangeData.OkxPassphrase)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新交易所 %s 失败: %v", exchangeID, err)})
			return
		}
	}

	// 重新加载该用户的所有交易员，使新配置立即生效
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 重新加载用户交易员到内存失败: %v", err)
		// 这里不返回错误，因为交易所配置已经成功更新到数据库
	}

	log.Printf("✓ 交易所配置已更新: %+v", req.Exchanges)
	c.JSON(http.StatusOK, gin.H{"message": "交易所配置已更新"})
}

// handleGetUserSignalSource 获取用户信号源配置
func (s *Server) handleGetUserSignalSource(c *gin.Context) {
	userID := c.GetString("user_id")
	source, err := s.database.GetUserSignalSource(userID)
	if err != nil {
		// 如果配置不存在，返回空配置而不是404错误
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

// handleSaveUserSignalSource 保存用户信号源配置
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存用户信号源配置失败: %v", err)})
		return
	}

	log.Printf("✓ 用户信号源配置已保存: user=%s, coin_pool=%s, oi_top=%s", userID, req.CoinPoolURL, req.OITopURL)
	c.JSON(http.StatusOK, gin.H{"message": "用户信号源配置已保存"})
}

// handleTraderList trader列表
func (s *Server) handleTraderList(c *gin.Context) {
	userID := c.GetString("user_id")
	traders, err := s.database.GetTraders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取交易员列表失败: %v", err)})
		return
	}

	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		// 获取实时运行状态
		isRunning := trader.IsRunning
		if at, err := s.traderManager.GetTrader(trader.ID); err == nil {
			status := at.GetStatus()
			if running, ok := status["is_running"].(bool); ok {
				isRunning = running
			}
		}

		// 返回完整的 AIModelID（如 "admin_deepseek"），不要截断
		// 前端需要完整 ID 来验证模型是否存在（与 handleGetTraderConfig 保持一致）
		result = append(result, map[string]interface{}{
			"trader_id":       trader.ID,
			"trader_name":     trader.Name,
			"ai_model":        trader.AIModelID, // 使用完整 ID
			"exchange_id":     trader.ExchangeID,
			"is_running":      isRunning,
			"initial_balance": trader.InitialBalance,
		})
	}

	c.JSON(http.StatusOK, result)
}

// handleGetAllTraders 获取所有交易员（admin only）
func (s *Server) handleGetAllTraders(c *gin.Context) {
	traders, err := s.database.GetAllTraders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取交易员列表失败: %v", err)})
		return
	}

	// 获取所有用户信息以便返回用户邮箱
	users, err := s.database.GetAllUsersWithRoles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取用户列表失败: %v", err)})
		return
	}
	userMap := make(map[string]string) // userID -> email
	for _, u := range users {
		userMap[u.ID] = u.Email
	}

	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		// 获取实时运行状态
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

// handleGetCurrentUser 获取当前认证用户信息（用于刷新角色等）
func (s *Server) handleGetCurrentUser(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未找到用户ID"})
		return
	}

	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取用户信息失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":    user.ID,
		"email": user.Email,
		"role":  user.Role,
	})
}

// handleGetAllUsers 获取所有用户（admin only）
func (s *Server) handleGetAllUsers(c *gin.Context) {
	users, err := s.database.GetAllUsersWithRoles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取用户列表失败: %v", err)})
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

// handleUpdateUserRole 更新用户角色（admin only）
func (s *Server) handleUpdateUserRole(c *gin.Context) {
	userID := c.Param("id")
	var req struct {
		Role string `json:"role"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证角色值
	validRoles := map[string]bool{"user": true, "follower": true, "admin": true}
	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的角色值: %s", req.Role)})
		return
	}

	err := s.database.UpdateUserRole(userID, req.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新用户角色失败: %v", err)})
		return
	}

	log.Printf("✓ Admin更新用户角色: userID=%s, role=%s", userID, req.Role)
	c.JSON(http.StatusOK, gin.H{"message": "用户角色已更新"})
}

// handleRunningTraders 获取所有运行中的交易员（用于follower选择信号源）
func (s *Server) handleRunningTraders(c *gin.Context) {
	// 获取所有用户
	userIDs, err := s.database.GetAllUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取用户列表失败: %v", err)})
		return
	}

	result := make([]map[string]interface{}, 0)

	// 遍历所有用户，获取他们的运行中的交易员
	for _, userID := range userIDs {
		// 获取用户信息
		user, err := s.database.GetUserByID(userID)
		if err != nil {
			continue // 跳过无法获取的用户
		}

		// 跳过follower用户的交易员，只返回"user"角色的交易员
		if user.Role == "follower" {
			continue
		}

		// 获取该用户的所有交易员
		traders, err := s.database.GetTraders(userID)
		if err != nil {
			continue // 跳过无法获取交易员的用户
		}

		// 过滤出运行中的交易员（排除跟随者交易员，只返回父交易员）
		for _, trader := range traders {
			// 跳过跟随者交易员，只返回父交易员（没有followed_trader_id的交易员）
			if trader.FollowedTraderID != "" {
				continue
			}

			// 检查数据库中的运行状态
			isRunning := trader.IsRunning

			// 检查内存中的实时运行状态
			if at, err := s.traderManager.GetTrader(trader.ID); err == nil {
				status := at.GetStatus()
				if running, ok := status["is_running"].(bool); ok {
					isRunning = running
				}
			}

			// 只返回运行中的交易员
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

// handleGetTraderConfig 获取交易员详细配置
func (s *Server) handleGetTraderConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员ID不能为空"})
		return
	}

	traderConfig, _, _, err := s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("获取交易员配置失败: %v", err)})
		return
	}

	// 获取实时运行状态
	isRunning := traderConfig.IsRunning
	if at, err := s.traderManager.GetTrader(traderID); err == nil {
		status := at.GetStatus()
		if running, ok := status["is_running"].(bool); ok {
			isRunning = running
		}
	}

	// 返回完整的模型ID，不做转换，保持与前端模型列表一致
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

// handleStatus 系统状态
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

// handleAccount 账户信息
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

	log.Printf("📊 收到账户信息请求 [%s]", trader.GetName())
	account, err := trader.GetAccountInfo()
	if err != nil {
		log.Printf("❌ 获取账户信息失败 [%s]: %v", trader.GetName(), err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取账户信息失败: %v", err),
		})
		return
	}

	log.Printf("✓ 返回账户信息 [%s]: 净值=%.2f, 可用=%.2f, 盈亏=%.2f (%.2f%%)",
		trader.GetName(),
		account["total_equity"],
		account["available_balance"],
		account["total_pnl"],
		account["total_pnl_pct"])
	c.JSON(http.StatusOK, account)
}

// handlePositions 持仓列表
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
			"error": fmt.Sprintf("获取持仓列表失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, positions)
}

// handleClosePosition 手动平仓
func (s *Server) handleClosePosition(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	// 获取trader_id（从query参数或body）
	traderID := c.Query("trader_id")
	if traderID == "" {
		var req struct {
			TraderID string `json:"trader_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 trader_id 参数"})
			return
		}
		traderID = req.TraderID
	}

	// 获取请求体
	var closeReq struct {
		Symbol   string  `json:"symbol" binding:"required"`
		Side     string  `json:"side" binding:"required"` // "long" or "short"
		Quantity float64 `json:"quantity"`                // 0 = close all
	}

	if err := c.ShouldBindJSON(&closeReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("请求参数错误: %v", err)})
		return
	}

	// 验证side参数
	if closeReq.Side != "long" && closeReq.Side != "short" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "side 必须是 'long' 或 'short'"})
		return
	}

	// 获取交易员
	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 验证交易员所有权（检查数据库）
	traderCfg, _, _, err := s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权操作此交易员"})
		return
	}

	log.Printf("📊 [%s] 手动平仓请求: symbol=%s, side=%s, quantity=%.4f", traderCfg.Name, closeReq.Symbol, closeReq.Side, closeReq.Quantity)

	// 执行平仓
	var result map[string]interface{}
	if closeReq.Side == "long" {
		result, err = trader.CloseLong(closeReq.Symbol, closeReq.Quantity)
	} else {
		result, err = trader.CloseShort(closeReq.Symbol, closeReq.Quantity)
	}

	if err != nil {
		log.Printf("❌ [%s] 平仓失败: %v", traderCfg.Name, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("平仓失败: %v", err),
		})
		return
	}

	log.Printf("✓ [%s] 平仓成功: symbol=%s, side=%s", traderCfg.Name, closeReq.Symbol, closeReq.Side)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "平仓成功",
		"result":  result,
	})
}

// handleDecisions 决策日志列表
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

	// 获取所有历史决策记录（无限制）
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取决策日志失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, records)
}

// handleLatestDecisions 最新决策日志（最近5条，最新的在前）
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
			"error": fmt.Sprintf("获取决策日志失败: %v", err),
		})
		return
	}

	// 反转数组，让最新的在前面（用于列表显示）
	// GetLatestRecords返回的是从旧到新（用于图表），这里需要从新到旧
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	c.JSON(http.StatusOK, records)
}

// handleStatistics 统计信息
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
			"error": fmt.Sprintf("获取统计信息失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// handleEquityHistory 收益率历史数据
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

	// 获取尽可能多的历史数据（几天的数据）
	// 每3分钟一个周期：10000条 = 约20天的数据
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取历史数据失败: %v", err),
		})
		return
	}

	// 构建收益率历史数据点
	type EquityPoint struct {
		Timestamp        string  `json:"timestamp"`
		TotalEquity      float64 `json:"total_equity"`      // 账户净值（wallet + unrealized）
		AvailableBalance float64 `json:"available_balance"` // 可用余额
		TotalPnL         float64 `json:"total_pnl"`         // 总盈亏（相对初始余额）
		TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
		PositionCount    int     `json:"position_count"`    // 持仓数量
		MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
		CycleNumber      int     `json:"cycle_number"`
	}

	// 从AutoTrader获取初始余额（用于计算盈亏百分比）
	initialBalance := 0.0
	if status := trader.GetStatus(); status != nil {
		if ib, ok := status["initial_balance"].(float64); ok && ib > 0 {
			initialBalance = ib
		}
	}

	// 如果无法从status获取，且有历史记录，则从第一条记录获取
	if initialBalance == 0 && len(records) > 0 {
		// 第一条记录的equity作为初始余额
		initialBalance = records[0].AccountState.TotalBalance
	}

	// 如果初始余额为0，返回空数组（账户无资金或未开始交易）
	if initialBalance == 0 {
		// 如果没有历史记录，返回空数组
		if len(records) == 0 {
			c.JSON(http.StatusOK, []EquityPoint{})
			return
		}
		// 如果有历史记录但初始余额为0，使用1.0作为基准避免除零错误
		// PNL百分比将显示为0%或基于第一个记录
		initialBalance = 1.0
	}

	var history []EquityPoint
	for _, record := range records {
		// TotalBalance字段实际存储的是TotalEquity
		totalEquity := record.AccountState.TotalBalance
		// TotalUnrealizedProfit字段实际存储的是TotalPnL（相对初始余额）
		totalPnL := record.AccountState.TotalUnrealizedProfit

		// 计算盈亏百分比
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

// handlePerformance AI历史表现分析（用于展示AI学习和反思）
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

	// 分析最近100个周期的交易表现（避免长期持仓的交易记录丢失）
	// 假设每3分钟一个周期，100个周期 = 5小时，足够覆盖大部分交易
	performance, err := trader.GetDecisionLogger().AnalyzePerformance(100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("分析历史表现失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, performance)
}

// handleGetReplicationStatus 获取交易复制状态
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取跟随关系失败: %v", err)})
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
			result["error"] = fmt.Sprintf("获取跟随者列表失败: %v", err)
		}
		result["parent"] = nil
	}

	c.JSON(http.StatusOK, result)
}

// handleTestSignal 手动触发测试信号
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取跟随者列表失败: %v", err)})
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

// handleGetUserFollowers 获取用户所有交易员的跟随者列表及其活动
func (s *Server) handleGetUserFollowers(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无法获取用户ID"})
		return
	}

	// 确保用户的交易员已加载到内存中
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, err)
	}

	// 获取用户的所有交易员
	userTraders, err := s.database.GetTraders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取交易员列表失败: %v", err),
		})
		return
	}

	// 过滤出父交易员（没有followed_trader_id的交易员）
	var parentTraders []*config.TraderRecord
	for _, trader := range userTraders {
		if trader.FollowedTraderID == "" {
			parentTraders = append(parentTraders, trader)
		}
	}

	// 构建响应数据
	parentTradersList := make([]gin.H, 0, len(parentTraders))

	for _, parentTrader := range parentTraders {
		// 获取该父交易员的所有跟随者
		followers, err := s.database.GetFollowerTraders(parentTrader.ID)
		if err != nil {
			log.Printf("⚠️ 获取交易员 %s 的跟随者列表失败: %v", parentTrader.ID, err)
			continue
		}

		// 如果没有跟随者，跳过
		if len(followers) == 0 {
			continue
		}

		// 构建跟随者列表
		followersList := make([]gin.H, 0, len(followers))

		for _, followerRecord := range followers {
			// 获取跟随者交易员实例
			followerTrader, err := s.traderManager.GetTrader(followerRecord.ID)
			if err != nil {
				log.Printf("⚠️ 跟随者交易员 %s 不在内存中: %v", followerRecord.ID, err)
				// 仍然添加基本信息，即使不在内存中
				followersList = append(followersList, gin.H{
					"trader_id":   followerRecord.ID,
					"trader_name": followerRecord.Name,
					"user_id":     followerRecord.UserID,
					"is_running":  false,
					"error":       "Follower trader not found in memory",
				})
				continue
			}

			// 获取运行状态
			status := followerTrader.GetStatus()
			isRunning := false
			if running, ok := status["is_running"].(bool); ok {
				isRunning = running
			}

			// 获取账户信息
			var accountInfo gin.H
			account, err := followerTrader.GetAccountInfo()
			if err != nil {
				log.Printf("⚠️ 获取跟随者 %s 账户信息失败: %v", followerRecord.ID, err)
				accountInfo = gin.H{"error": "无法获取账户信息"}
			} else {
				accountInfo = account
			}

			// 获取最新决策（最近5条）
			var latestDecisions []interface{}
			decisionLogger := followerTrader.GetDecisionLogger()
			if decisionLogger != nil {
				records, err := decisionLogger.GetLatestRecords(5)
				if err != nil {
					log.Printf("⚠️ 获取跟随者 %s 决策记录失败: %v", followerRecord.ID, err)
				} else {
					// 转换为JSON可序列化的格式
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

			// 获取持仓列表
			var positions []interface{}
			positionsData, err := followerTrader.GetPositions()
			if err != nil {
				log.Printf("⚠️ 获取跟随者 %s 持仓列表失败: %v", followerRecord.ID, err)
			} else {
				positions = make([]interface{}, 0, len(positionsData))
				for _, pos := range positionsData {
					positions = append(positions, pos)
				}
			}

			// 构建跟随者信息
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

		// 构建父交易员信息
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

// authMiddleware JWT认证中间件
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少Authorization头"})
			c.Abort()
			return
		}

		// 检查Bearer token格式
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的Authorization格式"})
			c.Abort()
			return
		}

		tokenString := tokenParts[1]

		// 黑名单检查
		if auth.IsTokenBlacklisted(tokenString) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token已失效，请重新登录"})
			c.Abort()
			return
		}

		// 验证JWT token
		claims, err := auth.ValidateJWT(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的token: " + err.Error()})
			c.Abort()
			return
		}

		// 获取用户信息（包括角色）
		user, err := s.database.GetUserByID(claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无法获取用户信息"})
			c.Abort()
			return
		}

		// 将用户信息存储到上下文中
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", user.Role)
		c.Next()
	}
}

// nonFollowerMiddleware 限制只有非follower用户才能访问
func (s *Server) nonFollowerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			role = "user" // 默认角色
		}

		roleStr, ok := role.(string)
		if !ok {
			roleStr = "user"
		}

		if roleStr == "follower" {
			c.JSON(http.StatusForbidden, gin.H{"error": "此功能仅限非follower用户使用"})
			c.Abort()
			return
		}

		c.Next()
	}
}


// adminOnlyMiddleware 限制只有admin用户才能访问
func (s *Server) adminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			role = "user" // 默认角色
		}

		roleStr, ok := role.(string)
		if !ok {
			roleStr = "user"
		}

		if roleStr != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "此功能仅限admin用户使用"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// handleLogout 将当前token加入黑名单
func (s *Server) handleLogout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少Authorization头"})
		return
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的Authorization格式"})
		return
	}
	tokenString := parts[1]
	claims, err := auth.ValidateJWT(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的token"})
		return
	}
	var exp time.Time
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	} else {
		exp = time.Now().Add(24 * time.Hour)
	}
	auth.BlacklistToken(tokenString, exp)
	c.JSON(http.StatusOK, gin.H{"message": "已登出"})
}

// handleRegister 处理用户注册请求
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

	// 检查是否开启了内测模式
	betaModeStr, _ := s.database.GetSystemConfig("beta_mode")
	if betaModeStr == "true" {
		// 内测模式下必须提供有效的内测码
		if req.BetaCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "内测期间，注册需要提供内测码"})
			return
		}

		// 验证内测码
		isValid, err := s.database.ValidateBetaCode(req.BetaCode)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "验证内测码失败"})
			return
		}
		if !isValid {
			c.JSON(http.StatusBadRequest, gin.H{"error": "内测码无效或已被使用"})
			return
		}
	}

	// 检查邮箱是否已存在
	_, err := s.database.GetUserByEmail(req.Email)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "邮箱已被注册"})
		return
	}

	// 生成密码哈希
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
		return
	}

	// 生成OTP密钥
	otpSecret, err := auth.GenerateOTPSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OTP密钥生成失败"})
		return
	}

	// 创建用户（未验证OTP状态）
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败: " + err.Error()})
		return
	}

	// 如果是内测模式，标记内测码为已使用
	betaModeStr2, _ := s.database.GetSystemConfig("beta_mode")
	if betaModeStr2 == "true" && req.BetaCode != "" {
		err := s.database.UseBetaCode(req.BetaCode, req.Email)
		if err != nil {
			log.Printf("⚠️ 标记内测码为已使用失败: %v", err)
			// 这里不返回错误，因为用户已经创建成功
		} else {
			log.Printf("✓ 内测码 %s 已被用户 %s 使用", req.BetaCode, req.Email)
		}
	}

	// 返回OTP设置信息
	qrCodeURL := auth.GetOTPQRCodeURL(otpSecret, req.Email)
	c.JSON(http.StatusOK, gin.H{
		"user_id":     userID,
		"email":       req.Email,
		"otp_secret":  otpSecret,
		"qr_code_url": qrCodeURL,
		"message":     "请使用Google Authenticator扫描二维码并验证OTP",
	})
}

// handleCompleteRegistration 完成注册（验证OTP）
func (s *Server) handleCompleteRegistration(c *gin.Context) {
	var req struct {
		UserID  string `json:"user_id" binding:"required"`
		OTPCode string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户信息
	user, err := s.database.GetUserByID(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 验证OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OTP验证码错误"})
		return
	}

	// 更新用户OTP验证状态
	err = s.database.UpdateUserOTPVerified(req.UserID, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户状态失败"})
		return
	}

	// 生成JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
		return
	}

	// 初始化用户的默认模型和交易所配置
	err = s.initUserDefaultConfigs(user.ID)
	if err != nil {
		log.Printf("初始化用户默认配置失败: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"message": "注册完成",
	})
}

// handleLogin 处理用户登录请求
func (s *Server) handleLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户信息
	user, err := s.database.GetUserByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}

	// 验证密码
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}

	// 检查OTP是否已验证
	if !user.OTPVerified {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":              "账户未完成OTP设置",
			"user_id":            user.ID,
			"requires_otp_setup": true,
		})
		return
	}

	// 返回需要OTP验证的状态
	c.JSON(http.StatusOK, gin.H{
		"user_id":      user.ID,
		"email":        user.Email,
		"message":      "请输入Google Authenticator验证码",
		"requires_otp": true,
	})
}

// handleVerifyOTP 验证OTP并完成登录
func (s *Server) handleVerifyOTP(c *gin.Context) {
	var req struct {
		UserID  string `json:"user_id" binding:"required"`
		OTPCode string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户信息
	user, err := s.database.GetUserByID(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 验证OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误"})
		return
	}

	// 生成JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"message": "登录成功",
	})
}

// handleResetPassword 重置密码（通过邮箱 + OTP 验证）
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

	// 查询用户
	user, err := s.database.GetUserByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "邮箱不存在"})
		return
	}

	// 验证 OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Google Authenticator 验证码错误"})
		return
	}

	// 生成新密码哈希
	newPasswordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
		return
	}

	// 更新密码
	err = s.database.UpdateUserPassword(user.ID, newPasswordHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码更新失败"})
		return
	}

	log.Printf("✓ 用户 %s 密码已重置", user.Email)
	c.JSON(http.StatusOK, gin.H{"message": "密码重置成功，请使用新密码登录"})
}

// initUserDefaultConfigs 为新用户初始化默认的模型和交易所配置
func (s *Server) initUserDefaultConfigs(userID string) error {
	// 注释掉自动创建默认配置，让用户手动添加
	// 这样新用户注册后不会自动有配置项
	log.Printf("用户 %s 注册完成，等待手动配置AI模型和交易所", userID)
	return nil
}

// handleGetSupportedModels 获取系统支持的AI模型列表
func (s *Server) handleGetSupportedModels(c *gin.Context) {
	// 返回系统支持的AI模型（从default用户获取）
	models, err := s.database.GetAIModels("default")
	if err != nil {
		log.Printf("❌ 获取支持的AI模型失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取支持的AI模型失败"})
		return
	}

	c.JSON(http.StatusOK, models)
}

// handleGetSupportedExchanges 获取系统支持的交易所列表
func (s *Server) handleGetSupportedExchanges(c *gin.Context) {
	// 返回系统支持的交易所（从default用户获取）
	exchanges, err := s.database.GetExchanges("default")
	if err != nil {
		log.Printf("❌ 获取支持的交易所失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取支持的交易所失败"})
		return
	}

	// 转换为安全的响应结构，移除敏感信息
	safeExchanges := make([]SafeExchangeConfig, len(exchanges))
	for i, exchange := range exchanges {
		safeExchanges[i] = SafeExchangeConfig{
			ID:                    exchange.ID,
			Name:                  exchange.Name,
			Type:                  exchange.Type,
			Enabled:               exchange.Enabled,
			Testnet:               exchange.Testnet,
			HyperliquidWalletAddr: "", // 默认配置不包含钱包地址
			AsterUser:             "", // 默认配置不包含用户信息
			AsterSigner:           "",
		}
	}

	c.JSON(http.StatusOK, safeExchanges)
}

// Start 启动服务器
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("🌐 API服务器启动在 http://localhost%s", addr)
	log.Printf("📊 API文档:")
	log.Printf("  • GET  /api/health           - 健康检查")
	log.Printf("  • GET  /api/traders          - 公开的AI交易员排行榜前50名（无需认证）")
	log.Printf("  • GET  /api/competition      - 公开的竞赛数据（无需认证）")
	log.Printf("  • GET  /api/top-traders      - 前5名交易员数据（无需认证，表现对比用）")
	log.Printf("  • GET  /api/equity-history?trader_id=xxx - 公开的收益率历史数据（无需认证，竞赛用）")
	log.Printf("  • GET  /api/equity-history-batch?trader_ids=a,b,c - 批量获取历史数据（无需认证，表现对比优化）")
	log.Printf("  • GET  /api/traders/:id/public-config - 公开的交易员配置（无需认证，不含敏感信息）")
	log.Printf("  • POST /api/traders          - 创建新的AI交易员")
	log.Printf("  • DELETE /api/traders/:id    - 删除AI交易员")
	log.Printf("  • POST /api/traders/:id/start - 启动AI交易员")
	log.Printf("  • POST /api/traders/:id/stop  - 停止AI交易员")
	log.Printf("  • GET  /api/models           - 获取AI模型配置")
	log.Printf("  • PUT  /api/models           - 更新AI模型配置")
	log.Printf("  • GET  /api/exchanges        - 获取交易所配置")
	log.Printf("  • PUT  /api/exchanges        - 更新交易所配置")
	log.Printf("  • GET  /api/status?trader_id=xxx     - 指定trader的系统状态")
	log.Printf("  • GET  /api/account?trader_id=xxx    - 指定trader的账户信息")
	log.Printf("  • GET  /api/positions?trader_id=xxx  - 指定trader的持仓列表")
	log.Printf("  • GET  /api/decisions?trader_id=xxx  - 指定trader的决策日志")
	log.Printf("  • GET  /api/decisions/latest?trader_id=xxx - 指定trader的最新决策")
	log.Printf("  • GET  /api/statistics?trader_id=xxx - 指定trader的统计信息")
	log.Printf("  • GET  /api/performance?trader_id=xxx - 指定trader的AI学习表现分析")
	log.Println()

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	return s.httpServer.ListenAndServe()
}

// Shutdown 优雅关闭服务器
func (s *Server) Shutdown() error {
	if s.httpServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// handleGetPromptTemplates 获取所有系统提示词模板列表（公开，无需认证）
func (s *Server) handleGetPromptTemplates(c *gin.Context) {
	// 从decision包获取模板（会合并数据库和文件系统）
	templates := decision.GetAllPromptTemplates()

	// 转换为响应格式
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

// handleGetPromptTemplate 获取指定名称的提示词模板内容（公开，无需认证）
func (s *Server) handleGetPromptTemplate(c *gin.Context) {
	templateName := c.Param("name")

	template, err := decision.GetPromptTemplate(templateName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("模板不存在: %s", templateName)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":    template.Name,
		"content": template.Content,
	})
}

// handleGetUserPromptTemplates 获取用户的提示词模板列表（包括系统模板和用户创建的模板）
func (s *Server) handleGetUserPromptTemplates(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	templates, err := s.database.GetPromptTemplates(userID)
	if err != nil {
		log.Printf("❌ 获取提示词模板列表失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取模板列表失败"})
		return
	}

	// 转换为响应格式
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

// handleGetUserPromptTemplate 获取指定的提示词模板
func (s *Server) handleGetUserPromptTemplate(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	templateID := c.Param("id")
	if templateID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "模板ID不能为空"})
		return
	}

	template, err := s.database.GetPromptTemplate(userID, templateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("模板不存在: %s", templateID)})
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

// handleCreatePromptTemplate 创建新的提示词模板
func (s *Server) handleCreatePromptTemplate(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	var req struct {
		Name    string `json:"name" binding:"required"`
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("请求参数错误: %v", err)})
		return
	}

	// 生成模板ID（使用userID_name格式，确保唯一性）
	templateID := fmt.Sprintf("%s_%s", userID, strings.ToLower(strings.ReplaceAll(req.Name, " ", "_")))

	// 创建模板（用户创建的模板，isSystem=false）
	err := s.database.CreatePromptTemplate(userID, templateID, req.Name, req.Content, false)
	if err != nil {
		log.Printf("❌ 创建提示词模板失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建模板失败"})
		return
	}

	// 获取创建的模板
	template, err := s.database.GetPromptTemplate(userID, templateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取创建的模板失败"})
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

// handleUpdatePromptTemplate 更新提示词模板（只能更新用户创建的模板）
func (s *Server) handleUpdatePromptTemplate(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	templateID := c.Param("id")
	if templateID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "模板ID不能为空"})
		return
	}

	var req struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("请求参数错误: %v", err)})
		return
	}

	// 至少需要更新一个字段
	if req.Name == "" && req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少需要提供name或content"})
		return
	}

	// 如果只更新content，保持原有name
	if req.Name == "" {
		existingTemplate, err := s.database.GetPromptTemplate(userID, templateID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "模板不存在"})
			return
		}
		req.Name = existingTemplate.Name
	}

	// 如果只更新name，保持原有content
	if req.Content == "" {
		existingTemplate, err := s.database.GetPromptTemplate(userID, templateID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "模板不存在"})
			return
		}
		req.Content = existingTemplate.Content
	}

	err := s.database.UpdatePromptTemplate(userID, templateID, req.Name, req.Content)
	if err != nil {
		log.Printf("❌ 更新提示词模板失败: %v", err)
		if strings.Contains(err.Error(), "不能更新系统模板") {
			c.JSON(http.StatusForbidden, gin.H{"error": "不能更新系统模板"})
			return
		}
		if strings.Contains(err.Error(), "无权更新") {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权更新此模板"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新模板失败"})
		return
	}

	// 获取更新后的模板
	template, err := s.database.GetPromptTemplate(userID, templateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取更新的模板失败"})
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

// handleDeletePromptTemplate 删除提示词模板（只能删除用户创建的模板）
func (s *Server) handleDeletePromptTemplate(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	templateID := c.Param("id")
	if templateID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "模板ID不能为空"})
		return
	}

	err := s.database.DeletePromptTemplate(userID, templateID)
	if err != nil {
		log.Printf("❌ 删除提示词模板失败: %v", err)
		if strings.Contains(err.Error(), "不能删除系统模板") {
			c.JSON(http.StatusForbidden, gin.H{"error": "不能删除系统模板"})
			return
		}
		if strings.Contains(err.Error(), "无权删除") {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权删除此模板"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除模板失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "模板已删除"})
}

// handlePublicTraderList 获取公开的交易员列表（无需认证）
func (s *Server) handlePublicTraderList(c *gin.Context) {
	// 从所有用户获取交易员信息
	competition, err := s.traderManager.GetCompetitionData(s.database)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取交易员列表失败: %v", err),
		})
		return
	}

	// 获取traders数组
	tradersData, exists := competition["traders"]
	if !exists {
		c.JSON(http.StatusOK, []map[string]interface{}{})
		return
	}

	traders, ok := tradersData.([]map[string]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "交易员数据格式错误",
		})
		return
	}

	// 返回交易员基本信息，过滤敏感信息
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

// handlePublicCompetition 获取公开的竞赛数据（无需认证）
func (s *Server) handlePublicCompetition(c *gin.Context) {
	competition, err := s.traderManager.GetCompetitionData(s.database)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取竞赛数据失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, competition)
}

// handleTopTraders 获取前5名交易员数据（无需认证，用于表现对比）
func (s *Server) handleTopTraders(c *gin.Context) {
	topTraders, err := s.traderManager.GetTopTradersData(s.database)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取前10名交易员数据失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, topTraders)
}

// handleEquityHistoryBatch 批量获取多个交易员的收益率历史数据（无需认证，用于表现对比）
func (s *Server) handleEquityHistoryBatch(c *gin.Context) {
	var requestBody struct {
		TraderIDs []string `json:"trader_ids"`
	}

	// 尝试解析POST请求的JSON body
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		// 如果JSON解析失败，尝试从query参数获取（兼容GET请求）
		traderIDsParam := c.Query("trader_ids")
		if traderIDsParam == "" {
			// 如果没有指定trader_ids，则返回前5名的历史数据
			topTraders, err := s.traderManager.GetTopTradersData(s.database)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("获取前5名交易员失败: %v", err),
				})
				return
			}

			traders, ok := topTraders["traders"].([]map[string]interface{})
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "交易员数据格式错误"})
				return
			}

			// 提取trader IDs
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

		// 解析逗号分隔的trader IDs
		requestBody.TraderIDs = strings.Split(traderIDsParam, ",")
		for i := range requestBody.TraderIDs {
			requestBody.TraderIDs[i] = strings.TrimSpace(requestBody.TraderIDs[i])
		}
	}

	// 限制最多20个交易员，防止请求过大
	if len(requestBody.TraderIDs) > 20 {
		requestBody.TraderIDs = requestBody.TraderIDs[:20]
	}

	result := s.getEquityHistoryForTraders(requestBody.TraderIDs)
	c.JSON(http.StatusOK, result)
}

// getEquityHistoryForTraders 获取多个交易员的历史数据
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
			errors[traderID] = "交易员不存在"
			continue
		}

		// 获取历史数据（用于对比展示，限制数据量）
		records, err := trader.GetDecisionLogger().GetLatestRecords(500)
		if err != nil {
			errors[traderID] = fmt.Sprintf("获取历史数据失败: %v", err)
			continue
		}

		// 构建收益率历史数据
		history := make([]map[string]interface{}, 0, len(records))
		for _, record := range records {
			// 计算总权益（余额+未实现盈亏）
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

// handleGetPublicTraderConfig 获取公开的交易员配置信息（无需认证，不包含敏感信息）
func (s *Server) handleGetPublicTraderConfig(c *gin.Context) {
	traderID := c.Param("id")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员ID不能为空"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 获取交易员的状态信息
	status := trader.GetStatus()

	// 只返回公开的配置信息，不包含API密钥等敏感数据
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
