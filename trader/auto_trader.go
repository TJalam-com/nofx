package trader

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	cfg "nofx/config"
	"nofx/decision"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/pool"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AutoTraderConfig automatic trading configuration (simplified version - AI makes all decisions)
type AutoTraderConfig struct {
	// Trader identification
	ID      string // Trader unique identifier (for log directories, etc.)
	Name    string // Trader display name
	AIModel string // AI model: "qwen" or "deepseek"

	// Exchange selection
	Exchange string // "binance", "bybit", "okx", "bitget", "hyperliquid", "aster", "lighter"

	// Binance API configuration
	BinanceAPIKey    string
	BinanceSecretKey string

	// Bybit API configuration
	BybitAPIKey    string
	BybitSecretKey string

	// OKX API configuration
	OkxAPIKey     string
	OkxSecretKey  string
	OkxPassphrase string // OKX requires passphrase

	// Bitget API configuration
	BitgetAPIKey     string
	BitgetSecretKey  string
	BitgetPassphrase string // Bitget requires passphrase (like OKX)

	// Hyperliquid configuration
	HyperliquidPrivateKey string
	HyperliquidWalletAddr string
	HyperliquidTestnet    bool

	// Aster configuration
	AsterUser       string // Aster main wallet address
	AsterSigner     string // Aster API wallet address
	AsterPrivateKey string // Aster API wallet private key

	// Lighter configuration
	LighterWalletAddr       string // Lighter main wallet address
	LighterAPIKeyPrivateKey string // Lighter API key private key
	LighterAPIKeyIndex      int    // Lighter API key index (0-254)
	LighterTestnet          bool   // Lighter testnet flag

	CoinPoolAPIURL string

	// AI configuration
	UseQwen     bool
	DeepSeekKey string
	QwenKey     string

	// Custom AI API configuration
	CustomAPIURL    string
	CustomAPIKey    string
	CustomModelName string

	// Scan configuration
	ScanInterval time.Duration // Scan interval (recommended 3 minutes)

	// Account configuration
	InitialBalance float64 // Initial balance (for calculating PnL, needs manual setting)

	// Leverage configuration
	BTCETHLeverage  int // Leverage multiplier for BTC and ETH
	AltcoinLeverage int // Leverage multiplier for altcoins

	// Risk control (only as hints, AI can decide autonomously)
	MaxDailyLoss    float64       // Maximum daily loss percentage (hint)
	MaxDrawdown     float64       // Maximum drawdown percentage (hint)
	StopTradingTime time.Duration // Pause duration after risk control triggered

	// Position mode
	IsCrossMargin bool // true=cross margin mode, false=isolated margin mode

	// Competition visibility
	ShowInCompetition bool // Whether to show in competition page

	// Currency configuration
	DefaultCoins []string // Default currency list (from database)
	TradingCoins []string // Actual trading currency list

	// System prompt template
	SystemPromptTemplate string // System prompt template name (e.g., "default", "aggressive")

	// TradingView configuration
	UseTradingView bool // Whether to use TradingView signal source (if true, will skip AI decisions)

	// Indicator configuration
	EnableRawKlines    bool   // Raw OHLCV klines (always true, required)
	EnableEMA          bool   // Enable EMA indicator
	EnableMACD         bool   // Enable MACD indicator
	EnableRSI          bool   // Enable RSI indicator
	EnableATR          bool   // Enable ATR indicator
	EnableVolume       bool   // Enable volume data
	EnableOI           bool   // Enable open interest data
	EnableFunding      bool   // Enable funding rate data
	IndicatorTimeframe string // Timeframe for indicators (e.g., "3m", "15m", "1h", "4h")
}

// ParentTradeSignal represents a trade signal from a followed trader
type ParentTradeSignal struct {
	ParentTraderID       string
	ParentTraderName     string
	SignalID             string
	Timestamp            time.Time
	Decision             *decision.Decision
	ParentEquity         float64
	ParentInitialBalance float64
}

// AutoTrader automatic trader
type AutoTrader struct {
	id                       string // Trader unique identifier
	name                     string // Trader display name
	aiModel                  string // AI model name
	exchange                 string // Exchange platform name
	showInCompetition        bool   // Whether to show in competition page
	config                   AutoTraderConfig
	trader                   Trader // Use Trader interface (supports multiple platforms)
	mcpClient                mcp.AIClient
	decisionLogger           logger.IDecisionLogger // Decision log recorder
	initialBalance           float64
	dailyPnL                 float64
	customPrompt             string   // Custom trading strategy prompt
	overrideBasePrompt       bool     // Whether to override base prompt
	systemPromptTemplate     string   // System prompt template name
	defaultCoins             []string // Default currency list (from database)
	tradingCoins             []string // Actual trading currency list
	lastResetTime            time.Time
	stopUntil                time.Time
	isRunning                bool
	startTime                time.Time                                          // System start time
	callCount                int                                                // AI call count
	positionFirstSeenTime    map[string]int64                                   // Position first seen time (symbol_side -> timestamp milliseconds)
	stopMonitorCh            chan struct{}                                      // Channel to stop monitor goroutine
	monitorWg                sync.WaitGroup                                     // WaitGroup to wait for monitor goroutine to finish
	peakPnLCache             map[string]float64                                 // Peak profit cache (symbol -> peak PnL percentage)
	peakPnLCacheMutex        sync.RWMutex                                       // Cache read-write lock
	lastBalanceSyncTime      time.Time                                          // Last balance sync time
	database                 interface{}                                        // Database reference (for automatic balance updates)
	userID                   string                                             // User ID
	triggerDecisionCh        chan string                                        // Channel for triggering immediate decision cycles (TradingView webhooks)
	tradeReplicationCallback func(traderID string, decision *decision.Decision) // Callback to replicate trades to followers
	parentTradeSignalCh      chan *ParentTradeSignal                            // Channel for parent trade signals
	isFollower               bool                                               // Cached follower status
	followedTraderID         string                                             // Cached parent trader ID
	processedSignals         map[string]time.Time                               // Deduplication cache (signal_id -> timestamp)
	processedSignalsMutex    sync.RWMutex                                       // Mutex for signal cache
	previousPositions        map[string]map[string]interface{}                   // Previous position snapshot (symbol_side -> position data)
	previousPositionsMutex   sync.RWMutex                                       // Mutex for previous positions
	loggedClosures           map[string]time.Time                               // Track logged closures to avoid duplicates (symbol_side -> timestamp)
	loggedClosuresMutex     sync.RWMutex                                       // Mutex for logged closures
}

// NewAutoTrader create automatic trader
func NewAutoTrader(config AutoTraderConfig, database interface{}, userID string) (*AutoTrader, error) {
	// Set default values
	if config.ID == "" {
		config.ID = "default_trader"
	}
	if config.Name == "" {
		config.Name = "Default Trader"
	}
	if config.AIModel == "" {
		if config.UseQwen {
			config.AIModel = "qwen"
		} else {
			config.AIModel = "deepseek"
		}
	}

	mcpClient := mcp.New()

	// Initialize AI based on provider
	provider := strings.ToLower(strings.TrimSpace(config.AIModel))
	if provider == "" && config.UseQwen {
		provider = "qwen"
	}

	switch provider {
	case "custom":
		// Use custom API
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		log.Printf("🤖 [%s] Using custom AI API: %s (model: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
	case "qwen":
		// Use Qwen (supports custom URL and Model)
		mcpClient = mcp.NewQwenClient()
		apiKey := config.QwenKey
		if apiKey == "" {
			apiKey = config.CustomAPIKey
		}
		mcpClient.SetAPIKey(apiKey, config.CustomAPIURL, config.CustomModelName)
		if config.CustomAPIURL != "" || config.CustomModelName != "" {
			log.Printf("🤖 [%s] Using Alibaba Cloud Qwen AI (custom URL: %s, model: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
		} else {
			log.Printf("🤖 [%s] Using Alibaba Cloud Qwen AI", config.Name)
		}
	case "grok":
		// Use Grok (supports custom URL and Model)
		mcpClient = mcp.NewGrokClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		if config.CustomAPIURL != "" || config.CustomModelName != "" {
			log.Printf("🤖 [%s] Using Grok AI (custom URL: %s, model: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
		} else {
			log.Printf("🤖 [%s] Using Grok AI", config.Name)
		}
	case "openai":
		// Use OpenAI (supports custom URL and Model)
		mcpClient = mcp.NewOpenAIClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		if config.CustomAPIURL != "" || config.CustomModelName != "" {
			log.Printf("🤖 [%s] Using OpenAI (custom URL: %s, model: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
		} else {
			log.Printf("🤖 [%s] Using OpenAI", config.Name)
		}
	case "claude":
		// Use Claude (supports custom URL and Model)
		mcpClient = mcp.NewClaudeClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		if config.CustomAPIURL != "" || config.CustomModelName != "" {
			log.Printf("🤖 [%s] Using Claude AI (custom URL: %s, model: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
		} else {
			log.Printf("🤖 [%s] Using Claude AI", config.Name)
		}
	case "gemini":
		// Use Gemini (supports custom URL and Model)
		mcpClient = mcp.NewGeminiClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		if config.CustomAPIURL != "" || config.CustomModelName != "" {
			log.Printf("🤖 [%s] Using Gemini AI (custom URL: %s, model: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
		} else {
			log.Printf("🤖 [%s] Using Gemini AI", config.Name)
		}
	case "kimi":
		// Use Kimi (supports custom URL and Model)
		mcpClient = mcp.NewKimiClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		if config.CustomAPIURL != "" || config.CustomModelName != "" {
			log.Printf("🤖 [%s] Using Kimi AI (custom URL: %s, model: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
		} else {
			log.Printf("🤖 [%s] Using Kimi AI", config.Name)
		}
	default:
		// Default to DeepSeek (supports custom URL and Model)
		mcpClient = mcp.NewDeepSeekClient()
		apiKey := config.DeepSeekKey
		if apiKey == "" {
			apiKey = config.CustomAPIKey
		}
		mcpClient.SetAPIKey(apiKey, config.CustomAPIURL, config.CustomModelName)
		if config.CustomAPIURL != "" || config.CustomModelName != "" {
			log.Printf("🤖 [%s] Using DeepSeek AI (custom URL: %s, model: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
		} else {
			log.Printf("🤖 [%s] Using DeepSeek AI", config.Name)
		}
	}

	// Initialize coin pool API
	if config.CoinPoolAPIURL != "" {
		pool.SetCoinPoolAPI(config.CoinPoolAPIURL)
	}

	// Set default exchange
	if config.Exchange == "" {
		config.Exchange = "binance"
	}

	// Create corresponding trader based on configuration
	var trader Trader
	var err error

	// Log margin mode (general)
	marginModeStr := "Cross Margin"
	if !config.IsCrossMargin {
		marginModeStr = "Isolated Margin"
	}
	log.Printf("📊 [%s] Margin mode: %s", config.Name, marginModeStr)

	switch config.Exchange {
	case "binance":
		log.Printf("🏦 [%s] Using Binance Futures trading", config.Name)
		trader = NewFuturesTrader(config.BinanceAPIKey, config.BinanceSecretKey, userID)
	case "bybit":
		log.Printf("🏦 [%s] Using Bybit Futures trading", config.Name)
		trader = NewBybitTrader(config.BybitAPIKey, config.BybitSecretKey)
	case "okx":
		log.Printf("🏦 [%s] Using OKX Futures trading", config.Name)
		trader = NewOKXTrader(config.OkxAPIKey, config.OkxSecretKey, config.OkxPassphrase)
	case "bitget":
		log.Printf("🏦 [%s] Using Bitget Futures trading", config.Name)
		trader = NewBitgetTrader(config.BitgetAPIKey, config.BitgetSecretKey, config.BitgetPassphrase)
	case "hyperliquid":
		log.Printf("🏦 [%s] Using Hyperliquid trading", config.Name)
		trader, err = NewHyperliquidTrader(config.HyperliquidPrivateKey, config.HyperliquidWalletAddr, config.HyperliquidTestnet)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Hyperliquid trader: %w", err)
		}
	case "aster":
		log.Printf("🏦 [%s] Using Aster trading", config.Name)
		trader, err = NewAsterTrader(config.AsterUser, config.AsterSigner, config.AsterPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Aster trader: %w", err)
		}
	case "lighter":
		log.Printf("🏦 [%s] Using Lighter DEX trading", config.Name)
		trader, err = NewLighterTraderV2(
			config.LighterWalletAddr,
			config.LighterAPIKeyPrivateKey,
			config.LighterAPIKeyIndex,
			config.LighterTestnet,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Lighter trader: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported exchange: %s", config.Exchange)
	}

	// Validate initial balance configuration, if not set, try to auto-fetch from exchange
	if config.InitialBalance <= 0 {
		log.Printf("📊 [%s] Initial balance not set, attempting to fetch from exchange...", config.Name)
		account, err := trader.GetBalance()
		if err != nil {
			// For Lighter, if balance fetch fails (e.g., API key not fully initialized), provide helpful error message
			if config.Exchange == "lighter" {
				log.Printf("⚠️  [%s] Failed to auto-fetch balance from Lighter: %v", config.Name, err)
				log.Printf("   This may occur if the API key slot exists but isn't fully initialized yet")
				log.Printf("   Please edit the trader and set InitialBalance manually in trader configuration")
				return nil, fmt.Errorf("initial balance not set and unable to fetch balance from exchange: %w. For Lighter, please edit the trader and set InitialBalance manually in trader configuration (go to trader settings > edit > set initial balance)", err)
			}
			return nil, fmt.Errorf("initial balance not set and unable to fetch balance from exchange: %w", err)
		} else {
			// Try multiple balance field names (different exchanges return different formats)
			balanceKeys := []string{"total_equity", "totalWalletBalance", "wallet_balance", "totalEq", "balance"}
			var foundBalance float64
			for _, key := range balanceKeys {
				if balance, ok := account[key].(float64); ok && balance > 0 {
					foundBalance = balance
					break
				}
			}
			if foundBalance > 0 {
				config.InitialBalance = foundBalance
				log.Printf("✓ [%s] Auto-fetched initial balance: %.2f USDT", config.Name, foundBalance)
			} else {
				return nil, fmt.Errorf("initial balance must be greater than 0, please set InitialBalance in configuration or ensure exchange account has balance")
			}
		}
	}

	// Initialize decision logger (create separate directory using trader ID)
	logDir := fmt.Sprintf("decision_logs/%s", config.ID)
	decisionLogger := logger.NewDecisionLogger(logDir)

	// Set default system prompt template
	systemPromptTemplate := config.SystemPromptTemplate
	if systemPromptTemplate == "" {
		// feature/partial-close-dynamic-tpsl branch defaults to adaptive (supports dynamic take profit/stop loss)
		systemPromptTemplate = "adaptive"
	}

	// Check if this trader is a follower and cache the status
	isFollower := false
	followedTraderID := ""
	if db, ok := database.(*cfg.Database); ok {
		followedID, err := db.GetTraderFollowedTraderID(config.ID)
		if err == nil && followedID != "" {
			isFollower = true
			followedTraderID = followedID
			log.Printf("👥 [%s] Detected as follower (parent: %s)", config.Name, followedID)
		}
	}

	return &AutoTrader{
		id:                    config.ID,
		name:                  config.Name,
		aiModel:               config.AIModel,
		exchange:              config.Exchange,
		showInCompetition:     config.ShowInCompetition,
		config:                config,
		trader:                trader,
		mcpClient:             mcpClient,
		decisionLogger:        decisionLogger,
		initialBalance:        config.InitialBalance,
		systemPromptTemplate:  systemPromptTemplate,
		defaultCoins:          config.DefaultCoins,
		tradingCoins:          config.TradingCoins,
		lastResetTime:         time.Now(),
		startTime:             time.Now(),
		callCount:             0,
		isRunning:             false,
		positionFirstSeenTime: make(map[string]int64),
		stopMonitorCh:         make(chan struct{}),
		monitorWg:             sync.WaitGroup{},
		peakPnLCache:          make(map[string]float64),
		peakPnLCacheMutex:     sync.RWMutex{},
		lastBalanceSyncTime:   time.Now(), // Initialize to current time
		database:              database,
		userID:                userID,
		triggerDecisionCh:     make(chan string, 100),             // Buffered channel for webhook triggers (buffer size: 100)
		parentTradeSignalCh:   make(chan *ParentTradeSignal, 200), // Buffered channel for parent signals (increased for instant TradingView forwarding)
		isFollower:            isFollower,
		followedTraderID:      followedTraderID,
		processedSignals:      make(map[string]time.Time),
		previousPositions:     make(map[string]map[string]interface{}),
		loggedClosures:         make(map[string]time.Time),
	}, nil
}

// Run runs the automatic trading main loop
func (at *AutoTrader) Run() error {
	// Validate initial balance configuration (validate at startup, allow balance=0 when loading)
	if at.initialBalance <= 0 {
		return fmt.Errorf("cannot start trader: initial balance not set, please use sync balance feature to set initial balance")
	}

	at.isRunning = true
	at.stopMonitorCh = make(chan struct{})
	at.startTime = time.Now()

	log.Println("🚀 AI-driven automatic trading system started")
	log.Printf("💰 Initial balance: %.2f USDT", at.initialBalance)
	log.Printf("⚙️  Scan interval: %v", at.config.ScanInterval)
	log.Println("🤖 AI will fully decide leverage, position size, stop loss, take profit and other parameters")
	log.Printf("INFO: Trader starting (id=%s, name=%s, UseTradingView=%v, ScanInterval=%v)", at.id, at.name, at.config.UseTradingView, at.config.ScanInterval)
	log.Printf("ℹ️ Trader mode: UseTradingView=%v (id=%s, name=%s)", at.config.UseTradingView, at.id, at.name)
	at.monitorWg.Add(1)
	defer at.monitorWg.Done()

	// Start drawdown monitoring
	at.startDrawdownMonitor()

	// Check if this is a follower trader
	if at.isFollower {
		log.Printf("👥 [%s] Follower mode: waiting for parent signals (parent=%s)",
			at.name, at.followedTraderID)

		// Increased concurrency limit for instant TradingView signal forwarding
		// Allows faster processing of burst signals while maintaining safety
		const maxConcurrentSignals = 10
		semaphore := make(chan struct{}, maxConcurrentSignals)

		for at.isRunning {
			select {
			case signal := <-at.parentTradeSignalCh:
				semaphore <- struct{}{}
				go func(s *ParentTradeSignal) {
					defer func() { <-semaphore }()
					at.processParentTradeSignalWithAI(s)
				}(signal)
			case <-at.stopMonitorCh:
				log.Printf("[%s] ⏹ Received stop signal, exiting follower mode", at.name)
				return nil
			}
		}
		return nil
	}

	// If TradingView is enabled, skip periodic scanning, wait for webhook triggers
	if at.config.UseTradingView {
		log.Println("📡 TradingView mode: waiting for webhook triggers, no periodic scanning")
		log.Printf("INFO: Entering TradingView wait mode (trader=%s, id=%s, UseTradingView=%v)", at.name, at.id, at.config.UseTradingView)
		log.Printf("ℹ️ TradingView wait loop started for trader %s (id=%s)", at.name, at.id)

		// Worker pool for concurrent webhook processing
		const maxConcurrentAlerts = 10
		semaphore := make(chan struct{}, maxConcurrentAlerts)

		for at.isRunning {
			select {
			case alertID := <-at.triggerDecisionCh:
				// Acquire semaphore
				semaphore <- struct{}{}
				// Process alert in goroutine
				go func(id string) {
					defer func() { <-semaphore }() // Release semaphore
					at.processTradingViewAlertWithAI(id)
				}(alertID)
			case <-at.stopMonitorCh:
				log.Printf("[%s] ⏹ Received stop signal, exiting automatic trading main loop", at.name)
				return nil
			}
		}
	} else {
		// Original periodic scanning logic
		log.Printf("INFO: Entering periodic scan mode (trader=%s, id=%s, UseTradingView=%v, ScanInterval=%v)", at.name, at.id, at.config.UseTradingView, at.config.ScanInterval)
		log.Printf("WARN: Trader is NOT in TradingView mode - periodic scans will run (trader=%s, id=%s)", at.name, at.id)
		log.Printf("🕒 Periodic scan mode enabled: run AI decision every %v", at.config.ScanInterval)
		ticker := time.NewTicker(at.config.ScanInterval)
		defer ticker.Stop()

		// Execute immediately on first run
		if err := at.runCycle(); err != nil {
			log.Printf("❌ Execution failed: %v", err)
		}

		for at.isRunning {
			select {
			case <-ticker.C:
				if err := at.runCycle(); err != nil {
					log.Printf("❌ Execution failed: %v", err)
				}
			case <-at.stopMonitorCh:
				log.Printf("[%s] ⏹ Received stop signal, exiting automatic trading main loop", at.name)
				return nil
			}
		}
	}

	return nil
}

// TriggerTradingViewDecisionCycle triggers TradingView decision cycle (non-blocking)
func (at *AutoTrader) TriggerTradingViewDecisionCycle(alertID string) error {
	if !at.isRunning {
		log.Printf("❌ [%s] Cannot trigger TradingView decision: trader not running (alertID=%s)", at.name, alertID)
		return fmt.Errorf("trader is not running")
	}

	select {
	case at.triggerDecisionCh <- alertID:
		log.Printf("📡 [%s] Triggered TradingView decision cycle (alertID=%s, pending=%d)", at.name, alertID, len(at.triggerDecisionCh))
		return nil
	default:
		// Channel is full, log warning but don't block
		log.Printf("⚠️ [%s] Decision trigger channel is full, skipping alertID=%s (pending=%d)", at.name, alertID, len(at.triggerDecisionCh))
		return fmt.Errorf("decision trigger channel is full")
	}
}

// TriggerParentTradeSignal sends a parent trade signal to the child trader for AI analysis (non-blocking)
func (at *AutoTrader) TriggerParentTradeSignal(signal *ParentTradeSignal) error {
	if !at.isRunning {
		log.Printf("❌ [%s] Cannot trigger parent trade signal: trader not running (signal_id=%s)", at.name, signal.SignalID)
		return fmt.Errorf("trader is not running")
	}

	// Check for duplicate signals
	at.processedSignalsMutex.RLock()
	if lastSeen, exists := at.processedSignals[signal.SignalID]; exists {
		if time.Since(lastSeen) < 5*time.Minute {
			at.processedSignalsMutex.RUnlock()
			log.Printf("⚠️ [%s] Duplicate parent signal detected (signal_id=%s, last_seen=%v ago)",
				at.name, signal.SignalID, time.Since(lastSeen))
			return fmt.Errorf("duplicate signal")
		}
	}
	at.processedSignalsMutex.RUnlock()

	select {
	case at.parentTradeSignalCh <- signal:
		log.Printf("📡 [%s] Triggered parent trade signal processing (signal_id=%s, pending=%d)",
			at.name, signal.SignalID, len(at.parentTradeSignalCh))
		return nil
	default:
		// Channel is full, log warning but don't block
		log.Printf("⚠️ [%s] Parent trade signal channel is full, skipping signal_id=%s (pending=%d)",
			at.name, signal.SignalID, len(at.parentTradeSignalCh))
		return fmt.Errorf("parent trade signal channel is full")
	}
}

// Stop stops automatic trading
func (at *AutoTrader) Stop() {
	if !at.isRunning {
		return
	}
	at.isRunning = false

	// Safe close: check if channel is already closed
	select {
	case <-at.stopMonitorCh:
		// Already closed, do nothing
	default:
		close(at.stopMonitorCh)
	}

	at.monitorWg.Wait() // Wait for monitor goroutine to finish
	log.Println("⏹ Automatic trading system stopped")
}

// runCycle runs a trading cycle (AI fully autonomous decision-making)
func (at *AutoTrader) runCycle() error {
	at.callCount++

	log.Print("\n" + strings.Repeat("=", 70) + "\n")
	if at.config.UseTradingView {
		log.Printf("⚠️ [%s] runCycle invoked while UseTradingView=true (unexpected periodic scan path)", at.name)
		log.Printf("⏰ %s - TradingView signal cycle #%d", time.Now().Format("2006-01-02 15:04:05"), at.callCount)
	} else {
		log.Printf("⏰ %s - AI decision cycle #%d", time.Now().Format("2006-01-02 15:04:05"), at.callCount)
	}
	log.Println(strings.Repeat("=", 70))

	// Create decision record
	record := &logger.DecisionRecord{
		ExecutionLog: []string{},
		Success:      true,
	}

	// 1. Check if trading needs to be stopped
	if time.Now().Before(at.stopUntil) {
		remaining := time.Until(at.stopUntil)
		log.Printf("⏸ Risk control: trading paused, %.0f minutes remaining", remaining.Minutes())
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("Risk control pause in effect, %.0f minutes remaining", remaining.Minutes())
		at.logDecisionAndSaveEquity(record)
		return nil
	}

	// 2. Reset daily PnL (reset daily)
	if time.Since(at.lastResetTime) > 24*time.Hour {
		at.dailyPnL = 0
		at.lastResetTime = time.Now()
		log.Println("📅 Daily PnL reset")
	}

	// 2.5. Check if follower (if followed_trader_id exists, skip AI decision cycle)
	// Check if this is a follower (early exit for follower traders)
	if at.isFollower {
		log.Printf("📋 [%s] Follower mode: skipping periodic cycle, waiting for parent signals", at.name)
		record.Success = true
		record.ExecutionLog = append(record.ExecutionLog, "Follower mode: waiting for parent signals")
		at.logDecisionAndSaveEquity(record)
		return nil
	}

	// 3. Check if using TradingView signals (if enabled, skip AI decision)
	if at.config.UseTradingView {
		log.Printf("📡 TradingView mode: skipping AI decision, processing TradingView alerts")
		return at.processTradingViewAlerts(record)
	}

	// 4. Collect trading context
	ctx, err := at.buildTradingContext()
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("failed to build trading context: %v", err)
		at.logDecisionAndSaveEquity(record)
		return fmt.Errorf("failed to build trading context: %w", err)
	}

	// Save account state snapshot
	record.AccountState = logger.AccountSnapshot{
		TotalBalance:          ctx.Account.TotalEquity - ctx.Account.UnrealizedPnL,
		AvailableBalance:      ctx.Account.AvailableBalance,
		TotalUnrealizedProfit: ctx.Account.UnrealizedPnL,
		PositionCount:         ctx.Account.PositionCount,
		MarginUsedPct:         ctx.Account.MarginUsedPct,
		InitialBalance:        at.initialBalance, // Record initial balance benchmark at that time
	}

	// Save position snapshot
	for _, pos := range ctx.Positions {
		record.Positions = append(record.Positions, logger.PositionSnapshot{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			PositionAmt:      pos.Quantity,
			EntryPrice:       pos.EntryPrice,
			MarkPrice:        pos.MarkPrice,
			UnrealizedProfit: pos.UnrealizedPnL,
			Leverage:         float64(pos.Leverage),
			LiquidationPrice: pos.LiquidationPrice,
		})
	}

	log.Print(strings.Repeat("=", 70))
	for _, coin := range ctx.CandidateCoins {
		record.CandidateCoins = append(record.CandidateCoins, coin.Symbol)
	}

	log.Printf("📊 Account equity: %.2f USDT | Available: %.2f USDT | Positions: %d",
		ctx.Account.TotalEquity, ctx.Account.AvailableBalance, ctx.Account.PositionCount)

	// 5. Call AI to get full decision
	log.Printf("🤖 Requesting AI analysis and decision... [Template: %s]", at.systemPromptTemplate)
	decision, err := decision.GetFullDecisionWithCustomPrompt(ctx, at.mcpClient, at.customPrompt, at.overrideBasePrompt, at.systemPromptTemplate)

	if decision != nil && decision.AIRequestDurationMs > 0 {
		record.AIRequestDurationMs = decision.AIRequestDurationMs
		log.Printf("⏱️ AI call duration: %.2f seconds", float64(record.AIRequestDurationMs)/1000)
		record.ExecutionLog = append(record.ExecutionLog,
			fmt.Sprintf("AI call duration: %d ms", record.AIRequestDurationMs))
	}

	// Save chain of thought, decision, and input prompt even if there's an error (for debugging)
	if decision != nil {
		record.SystemPrompt = decision.SystemPrompt // Save system prompt
		record.InputPrompt = decision.UserPrompt
		record.CoTTrace = decision.CoTTrace
		record.RawResponse = decision.RawResponse // Save raw AI response for debugging parse failures
		if len(decision.Decisions) > 0 {
			decisionJSON, _ := json.MarshalIndent(decision.Decisions, "", "  ")
			record.DecisionJSON = string(decisionJSON)
		}
	}

	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("failed to get AI decision: %v", err)

		// Print system prompt and AI chain of thought (even with errors, output for debugging)
		if decision != nil {
			log.Print("\n" + strings.Repeat("=", 70) + "\n")
			log.Printf("📋 System prompt [Template: %s] (error case)", at.systemPromptTemplate)
			log.Println(strings.Repeat("=", 70))
			log.Println(decision.SystemPrompt)
			log.Println(strings.Repeat("=", 70))

			if decision.CoTTrace != "" {
				log.Print("\n" + strings.Repeat("-", 70) + "\n")
				log.Println("💭 AI chain of thought analysis (error case):")
				log.Println(strings.Repeat("-", 70))
				log.Println(decision.CoTTrace)
				log.Println(strings.Repeat("-", 70))
			}
		}

		at.logDecisionAndSaveEquity(record)
		return fmt.Errorf("failed to get AI decision: %w", err)
	}

	// // 5. Print system prompt
	// log.Printf("\n" + strings.Repeat("=", 70))
	// log.Printf("📋 System prompt [Template: %s]", at.systemPromptTemplate)
	// log.Println(strings.Repeat("=", 70))
	// log.Println(decision.SystemPrompt)
	// log.Printf(strings.Repeat("=", 70) + "\n")

	// 6. Print AI chain of thought
	// log.Printf("\n" + strings.Repeat("-", 70))
	// log.Println("💭 AI chain of thought analysis:")
	// log.Println(strings.Repeat("-", 70))
	// log.Println(decision.CoTTrace)
	// log.Printf(strings.Repeat("-", 70) + "\n")

	// 7. Print AI decision
	// log.Printf("📋 AI decision list (%d items):\n", len(decision.Decisions))
	// for i, d := range decision.Decisions {
	//     log.Printf("  [%d] %s: %s - %s", i+1, d.Symbol, d.Action, d.Reasoning)
	//     if d.Action == "open_long" || d.Action == "open_short" {
	//        log.Printf("      Leverage: %dx | Position: %.2f USDT | Stop Loss: %.4f | Take Profit: %.4f",
	//           d.Leverage, d.PositionSizeUSD, d.StopLoss, d.TakeProfit)
	//     }
	// }
	log.Println()
	log.Print(strings.Repeat("-", 70))
	// 8. Sort decisions: ensure close positions before open positions (prevent position stacking exceeding limits)
	log.Print(strings.Repeat("-", 70))

	// 8. Sort decisions: ensure close positions before open positions (prevent position stacking exceeding limits)
	sortedDecisions := sortDecisionsByPriority(decision.Decisions)

	log.Println("🔄 Execution order (optimized): close positions first → then open positions")
	for i, d := range sortedDecisions {
		log.Printf("  [%d] %s %s", i+1, d.Symbol, d.Action)
	}
	log.Println()

	// Execute decisions and record results
	for _, d := range sortedDecisions {
		actionRecord := logger.DecisionAction{
			Action:    d.Action,
			Symbol:    d.Symbol,
			Quantity:  0,
			Leverage:  d.Leverage,
			Price:     0,
			Timestamp: time.Now(),
			Success:   false,
		}

		if err := at.executeDecisionWithRecord(&d, &actionRecord); err != nil {
			log.Printf("❌ Failed to execute decision (%s %s): %v", d.Symbol, d.Action, err)
			actionRecord.Error = err.Error()
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ %s %s failed: %v", d.Symbol, d.Action, err))
		} else {
			actionRecord.Success = true
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s succeeded", d.Symbol, d.Action))
			// Brief delay after successful execution
			time.Sleep(1 * time.Second)
		}

		record.Decisions = append(record.Decisions, actionRecord)
	}

	// 9. Save decision record and equity history
	if err := at.logDecisionAndSaveEquity(record); err != nil {
		log.Printf("⚠ Failed to save decision record: %v", err)
	}

	return nil
}

// buildTradingContext builds trading context
func (at *AutoTrader) buildTradingContext() (*decision.Context, error) {
	// 1. Get account information
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("failed to get account balance: %w", err)
	}

	// Get account fields
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0

	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Total Equity = wallet balance + unrealized profit/loss
	totalEquity := totalWalletBalance + totalUnrealizedProfit

	// 2. Get position information
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var positionInfos []decision.PositionInfo
	totalMarginUsed := 0.0

	// Set of current position keys (for cleaning up closed position records)
	currentPositionKeys := make(map[string]bool)

	for _, pos := range positions {
		// Safe type assertions to prevent nil interface conversion panic
		symbol, ok := pos["symbol"].(string)
		if !ok || symbol == "" {
			continue // Skip invalid position
		}
		side, ok := pos["side"].(string)
		if !ok || side == "" {
			continue // Skip invalid position
		}
		entryPrice, ok := pos["entryPrice"].(float64)
		if !ok {
			continue // Skip invalid position
		}
		markPrice, ok := pos["markPrice"].(float64)
		if !ok {
			continue // Skip invalid position
		}
		quantity, ok := pos["positionAmt"].(float64)
		if !ok {
			continue // Skip invalid position
		}
		if quantity < 0 {
			quantity = -quantity // Short position quantity is negative, convert to positive
		}

		// Skip closed positions (quantity = 0), prevent "ghost positions" from being passed to AI
		if quantity == 0 {
			continue
		}

		unrealizedPnl, ok := pos["unRealizedProfit"].(float64)
		if !ok {
			unrealizedPnl = 0 // Default to 0 if missing
		}
		liquidationPrice, ok := pos["liquidationPrice"].(float64)
		if !ok {
			liquidationPrice = 0 // Default to 0 if missing
		}

		// Calculate used margin (estimate)
		leverage := 10 // Default value, should actually get from position information
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed

		// Calculate profit/loss percentage (based on margin, considering leverage)
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		// Track position first seen time
		posKey := symbol + "_" + side
		currentPositionKeys[posKey] = true

		// Handle zero/invalid entry_time from exchange position sync
		var updateTime int64
		if entryTimeRaw, ok := pos["entry_time"].(float64); ok && entryTimeRaw > 0 {
			// Use entry_time from exchange (convert to milliseconds if needed)
			updateTime = int64(entryTimeRaw)
			at.positionFirstSeenTime[posKey] = updateTime
		} else if entryTimeRaw, ok := pos["entry_time"].(int64); ok && entryTimeRaw > 0 {
			// Handle int64 format
			updateTime = entryTimeRaw
			at.positionFirstSeenTime[posKey] = updateTime
		} else if existingTime, exists := at.positionFirstSeenTime[posKey]; exists && existingTime > 0 {
			// Use existing time if position was seen before
			updateTime = existingTime
		} else {
			// New position, record current time as fallback
			updateTime = time.Now().UnixMilli()
			at.positionFirstSeenTime[posKey] = updateTime
		}

		// Get historical peak return rate for this position
		at.peakPnLCacheMutex.RLock()
		peakPnlPct := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		positionInfos = append(positionInfos, decision.PositionInfo{
			Symbol:           symbol,
			Side:             side,
			EntryPrice:       entryPrice,
			MarkPrice:        markPrice,
			Quantity:         quantity,
			Leverage:         leverage,
			UnrealizedPnL:    unrealizedPnl,
			UnrealizedPnLPct: pnlPct,
			PeakPnLPct:       peakPnlPct,
			LiquidationPrice: liquidationPrice,
			MarginUsed:       marginUsed,
			UpdateTime:       updateTime,
		})
	}

	// Clean up closed position records
	for key := range at.positionFirstSeenTime {
		if !currentPositionKeys[key] {
			delete(at.positionFirstSeenTime, key)
		}
	}

	// 3. Get trader's candidate coin pool
	candidateCoins, err := at.getCandidateCoins()
	if err != nil {
		return nil, fmt.Errorf("failed to get candidate coins: %w", err)
	}

	// 4. Calculate total profit/loss
	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	// 5. Analyze historical performance (last 100 cycles, avoid losing trading records of long-term positions)
	// Assuming 3 minutes per cycle, 100 cycles = 5 hours, sufficient to cover most trades
	performance, err := at.decisionLogger.AnalyzePerformance(100)
	if err != nil {
		log.Printf("⚠️  Failed to analyze historical performance: %v", err)
		// Don't affect main flow, continue execution (but set performance to nil to avoid passing incorrect data)
		performance = nil
	}

	// 6. Get recent completed trades from database
	var completedTrades []decision.CompletedTrade
	if db, ok := at.database.(*cfg.Database); ok && db != nil {
		positionHistory, err := db.GetPositionHistory(at.id, 10, 0)
		if err == nil {
			for _, pos := range positionHistory {
				// Only include closed positions
				if pos.ClosedAt != nil {
					completedTrades = append(completedTrades, decision.CompletedTrade{
						Symbol:      pos.Symbol,
						Side:        pos.Side,
						EntryPrice:  pos.EntryPrice,
						ExitPrice:   pos.ExitPrice,
						Quantity:    pos.Quantity,
						RealizedPnL: pos.RealizedPnL,
						ClosedAt:    *pos.ClosedAt,
					})
				}
			}
		} else {
			// If database query fails, skip completed trades (don't break prompt)
			log.Printf("⚠️  Failed to get completed trades: %v", err)
		}
	}

	// 7. Build context
	ctx := &decision.Context{
		CurrentTime:     time.Now().Format("2006-01-02 15:04:05"),
		RuntimeMinutes:  int(time.Since(at.startTime).Minutes()),
		CallCount:       at.callCount,
		BTCETHLeverage:  at.config.BTCETHLeverage,  // Use configured leverage multiplier
		AltcoinLeverage: at.config.AltcoinLeverage, // Use configured leverage multiplier
		Account: decision.AccountInfo{
			TotalEquity:      totalEquity,
			AvailableBalance: availableBalance,
			UnrealizedPnL:    totalUnrealizedProfit,
			TotalPnL:         totalPnL,
			TotalPnLPct:      totalPnLPct,
			MarginUsed:       totalMarginUsed,
			MarginUsedPct:    marginUsedPct,
			PositionCount:    len(positionInfos),
		},
		Positions:       positionInfos,
		CompletedTrades: completedTrades,
		CandidateCoins:  candidateCoins,
		Performance:     performance, // Add historical performance analysis
	}

	return ctx, nil
}

// SetTradeReplicationCallback sets trade replication callback function
func (at *AutoTrader) SetTradeReplicationCallback(callback func(traderID string, decision *decision.Decision)) {
	at.tradeReplicationCallback = callback
}

// ExecuteDecisionWithRecord public method for external execution of decisions (for follower trading)
func (at *AutoTrader) ExecuteDecisionWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	return at.executeDecisionWithRecord(decision, actionRecord)
}

// executeDecisionWithRecord executes AI decision and records detailed information
func (at *AutoTrader) executeDecisionWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	var err error

	switch decision.Action {
	case "open_long":
		err = at.executeOpenLongWithRecord(decision, actionRecord)
	case "open_short":
		err = at.executeOpenShortWithRecord(decision, actionRecord)
	case "close_long":
		err = at.executeCloseLongWithRecord(decision, actionRecord)
	case "close_short":
		err = at.executeCloseShortWithRecord(decision, actionRecord)
	case "update_stop_loss":
		err = at.executeUpdateStopLossWithRecord(decision, actionRecord)
	case "update_take_profit":
		err = at.executeUpdateTakeProfitWithRecord(decision, actionRecord)
	case "partial_close":
		err = at.executePartialCloseWithRecord(decision, actionRecord)
	case "hold", "wait":
		// No execution needed, only record
		return nil
	default:
		return fmt.Errorf("unknown action: %s", decision.Action)
	}

	// If trade executed successfully, replicate to followers
	// Note: We check err == nil only, as actionRecord.Success is set by the caller after this function returns
	if err == nil && at.tradeReplicationCallback != nil {
		// Only replicate position-changing actions
		if decision.Action == "open_long" || decision.Action == "open_short" ||
			decision.Action == "close_long" || decision.Action == "close_short" ||
			decision.Action == "partial_close" {
			// Call replication callback in goroutine to avoid blocking
			go at.tradeReplicationCallback(at.id, decision)
		}
	}

	return err
}

// executeOpenLongWithRecord executes open long position and records detailed information
func (at *AutoTrader) executeOpenLongWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📈 Open long position: %s", decision.Symbol)

	// ⚠️ Critical: Check if there's already a position in the same symbol and direction, reject if so (prevent position stacking exceeding limits)
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
				return fmt.Errorf("❌ %s already has long position, rejecting open to prevent position stacking exceeding limits. If you need to change position, please provide close_long decision first", decision.Symbol)
			}
		}
	}

	// Get current price
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}

	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Auto-adjust position size if insufficient margin
	// Formula: totalRequired = positionSize/leverage + positionSize*0.001 + positionSize/leverage*0.01
	//        = positionSize * (1.01/leverage + 0.001)
	marginFactor := 1.01/float64(decision.Leverage) + 0.001
	maxAffordablePositionSize := availableBalance / marginFactor

	actualPositionSize := decision.PositionSizeUSD
	if actualPositionSize > maxAffordablePositionSize {
		// Use 98% of max to leave buffer for price fluctuation
		adjustedSize := maxAffordablePositionSize * 0.98
		log.Printf("  ⚠️ Position size %.2f exceeds max affordable %.2f, auto-reducing to %.2f", actualPositionSize, maxAffordablePositionSize, adjustedSize)
		actualPositionSize = adjustedSize
		decision.PositionSizeUSD = actualPositionSize
	}

	// Calculate quantity with adjusted position size
	quantity := actualPositionSize / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// Set margin mode
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		log.Printf("  ⚠️ Failed to set margin mode: %v", err)
		// Continue execution, don't affect trading
	}

	// Open position
	order, err := at.trader.OpenLong(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	// Extract order ID
	var orderIDStr string
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
		orderIDStr = fmt.Sprintf("%d", orderID)
	} else if orderID, ok := order["orderId"].(string); ok {
		orderIDStr = orderID
		// Try to parse string order ID to int64
		if parsedID, err := strconv.ParseInt(orderID, 10, 64); err == nil {
			actionRecord.OrderID = parsedID
		} else {
			actionRecord.OrderID = 0
		}
	} else {
		log.Printf("  ⚠ Warning: Could not extract order ID from order response")
		orderIDStr = ""
		actionRecord.OrderID = 0
	}

	log.Printf("  ✓ Position opened successfully, order ID: %v, quantity: %.4f", order["orderId"], quantity)

	// Poll GetOrderStatus to get actual fill price
	var actualFillPrice float64
	var actualExecutedQty float64
	var entryFee float64
	var finalOrderStatus string
	if orderIDStr != "" {
		// Wait 500ms before first poll
		time.Sleep(500 * time.Millisecond)

		// Poll up to 3 times with 1 second delay
		for attempt := 0; attempt < 3; attempt++ {
			orderStatus, err := at.trader.GetOrderStatus(decision.Symbol, orderIDStr)
			if err == nil {
				if avgPrice, ok := orderStatus["avgPrice"].(float64); ok && avgPrice > 0 {
					actualFillPrice = avgPrice
					actionRecord.Price = avgPrice
				}
				if execQty, ok := orderStatus["executedQty"].(float64); ok && execQty > 0 {
					actualExecutedQty = execQty
					actionRecord.Quantity = execQty
				}
				if commission, ok := orderStatus["commission"].(float64); ok {
					entryFee = commission
				}
				status, _ := orderStatus["status"].(string)
				finalOrderStatus = status
				if status == "FILLED" || status == "PARTIALLY_FILLED" {
					log.Printf("  ✓ Order filled: avgPrice=%.4f, executedQty=%.4f, fee=%.4f", actualFillPrice, actualExecutedQty, entryFee)
					break
				}
			}
			if attempt < 2 {
				time.Sleep(1 * time.Second)
			}
		}

		// Verify order was actually filled - use multiple fallback methods
		orderFilled := false

		// Method 1: Check status
		if finalOrderStatus == "FILLED" || finalOrderStatus == "PARTIALLY_FILLED" {
			orderFilled = true
			log.Printf("  ✓ Order status confirmed: %s", finalOrderStatus)
		}

		// Method 2: Check executed quantity (even if status is empty, executedQty > 0 means filled)
		if actualExecutedQty > 0 {
			orderFilled = true
			log.Printf("  ✓ Order executed quantity confirmed: %.4f", actualExecutedQty)
		}

		// Method 3: Fallback - check if position exists in Binance
		if !orderFilled {
			log.Printf("  🔍 Status check inconclusive (status=%s, execQty=%.4f), checking position as fallback...", finalOrderStatus, actualExecutedQty)
			positions, err := at.trader.GetPositions()
			if err == nil {
				for _, pos := range positions {
					if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
						posAmt, _ := pos["positionAmt"].(float64)
						if posAmt > 0 {
							orderFilled = true
							actualExecutedQty = posAmt
							if entryPrice, ok := pos["entryPrice"].(float64); ok && entryPrice > 0 {
								actualFillPrice = entryPrice
								actionRecord.Price = entryPrice
							}
							log.Printf("  ✓ Position verified in Binance: quantity=%.4f, entryPrice=%.4f", posAmt, actualFillPrice)
							break
						}
					}
				}
			}
		}

		// Final verification - fail only if all methods indicate order not filled
		if !orderFilled {
			return fmt.Errorf("order not filled: status=%s, orderId=%s, execQty=%.4f. Order may have been rejected, canceled, or expired", finalOrderStatus, orderIDStr, actualExecutedQty)
		}

		// Verify executed quantity is greater than 0 (final check)
		if actualExecutedQty <= 0 {
			return fmt.Errorf("order executed quantity is 0: status=%s, orderId=%s. Order was not filled", finalOrderStatus, orderIDStr)
		}

		if actualFillPrice == 0 {
			log.Printf("  ⚠ Warning: Could not get actual fill price, using market price")
			actualFillPrice = marketData.CurrentPrice
		}
	} else {
		actualFillPrice = marketData.CurrentPrice
		actualExecutedQty = quantity
	}

	// Save position record to database
	if db, ok := at.database.(*cfg.Database); ok && db != nil {
		openedAt := time.Now()
		err := db.SavePosition(at.id, decision.Symbol, "long", actualFillPrice, 0, actualExecutedQty, entryFee, 0, 0, decision.Leverage, orderIDStr, "", openedAt, nil)
		if err != nil {
			log.Printf("  ⚠ Warning: Failed to save position record: %v", err)
		}
	}

	// Record open position time
	posKey := decision.Symbol + "_long"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	// Set stop loss and take profit
	if err := at.trader.SetStopLoss(decision.Symbol, "LONG", actualExecutedQty, decision.StopLoss); err != nil {
		log.Printf("  ⚠ Failed to set stop loss: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "LONG", actualExecutedQty, decision.TakeProfit); err != nil {
		log.Printf("  ⚠ Failed to set take profit: %v", err)
	}

	return nil
}

// executeOpenShortWithRecord executes open short position and records detailed information
func (at *AutoTrader) executeOpenShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📉 Open short position: %s", decision.Symbol)

	// ⚠️ Critical: Check if there's already a position in the same symbol and direction, reject if so (prevent position stacking exceeding limits)
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
				return fmt.Errorf("❌ %s already has short position, rejecting open to prevent position stacking exceeding limits. If you need to change position, please provide close_short decision first", decision.Symbol)
			}
		}
	}

	// Get current price
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}

	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Auto-adjust position size if insufficient margin
	// Formula: totalRequired = positionSize/leverage + positionSize*0.001 + positionSize/leverage*0.01
	//        = positionSize * (1.01/leverage + 0.001)
	marginFactor := 1.01/float64(decision.Leverage) + 0.001
	maxAffordablePositionSize := availableBalance / marginFactor

	actualPositionSize := decision.PositionSizeUSD
	if actualPositionSize > maxAffordablePositionSize {
		// Use 98% of max to leave buffer for price fluctuation
		adjustedSize := maxAffordablePositionSize * 0.98
		log.Printf("  ⚠️ Position size %.2f exceeds max affordable %.2f, auto-reducing to %.2f", actualPositionSize, maxAffordablePositionSize, adjustedSize)
		actualPositionSize = adjustedSize
		decision.PositionSizeUSD = actualPositionSize
	}

	// Calculate quantity with adjusted position size
	quantity := actualPositionSize / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// Set margin mode
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		log.Printf("  ⚠️ Failed to set margin mode: %v", err)
		// Continue execution, don't affect trading
	}

	// Open position
	order, err := at.trader.OpenShort(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	// Extract order ID
	var orderIDStr string
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
		orderIDStr = fmt.Sprintf("%d", orderID)
	} else if orderID, ok := order["orderId"].(string); ok {
		orderIDStr = orderID
		// Try to parse string order ID to int64
		if parsedID, err := strconv.ParseInt(orderID, 10, 64); err == nil {
			actionRecord.OrderID = parsedID
		} else {
			actionRecord.OrderID = 0
		}
	} else {
		log.Printf("  ⚠ Warning: Could not extract order ID from order response")
		orderIDStr = ""
		actionRecord.OrderID = 0
	}

	log.Printf("  ✓ Position opened successfully, order ID: %v, quantity: %.4f", order["orderId"], quantity)

	// Poll GetOrderStatus to get actual fill price
	var actualFillPrice float64
	var actualExecutedQty float64
	var entryFee float64
	var finalOrderStatus string
	if orderIDStr != "" {
		// Wait 500ms before first poll
		time.Sleep(500 * time.Millisecond)

		// Poll up to 3 times with 1 second delay
		for attempt := 0; attempt < 3; attempt++ {
			orderStatus, err := at.trader.GetOrderStatus(decision.Symbol, orderIDStr)
			if err == nil {
				if avgPrice, ok := orderStatus["avgPrice"].(float64); ok && avgPrice > 0 {
					actualFillPrice = avgPrice
					actionRecord.Price = avgPrice
				}
				if execQty, ok := orderStatus["executedQty"].(float64); ok && execQty > 0 {
					actualExecutedQty = execQty
					actionRecord.Quantity = execQty
				}
				if commission, ok := orderStatus["commission"].(float64); ok {
					entryFee = commission
				}
				status, _ := orderStatus["status"].(string)
				finalOrderStatus = status
				if status == "FILLED" || status == "PARTIALLY_FILLED" {
					log.Printf("  ✓ Order filled: avgPrice=%.4f, executedQty=%.4f, fee=%.4f", actualFillPrice, actualExecutedQty, entryFee)
					break
				}
			}
			if attempt < 2 {
				time.Sleep(1 * time.Second)
			}
		}

		// Verify order was actually filled - use multiple fallback methods
		orderFilled := false

		// Method 1: Check status
		if finalOrderStatus == "FILLED" || finalOrderStatus == "PARTIALLY_FILLED" {
			orderFilled = true
			log.Printf("  ✓ Order status confirmed: %s", finalOrderStatus)
		}

		// Method 2: Check executed quantity (even if status is empty, executedQty > 0 means filled)
		if actualExecutedQty > 0 {
			orderFilled = true
			log.Printf("  ✓ Order executed quantity confirmed: %.4f", actualExecutedQty)
		}

		// Method 3: Fallback - check if position exists in Binance
		if !orderFilled {
			log.Printf("  🔍 Status check inconclusive (status=%s, execQty=%.4f), checking position as fallback...", finalOrderStatus, actualExecutedQty)
			positions, err := at.trader.GetPositions()
			if err == nil {
				for _, pos := range positions {
					if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
						posAmt, _ := pos["positionAmt"].(float64)
						if posAmt < 0 { // Short positions have negative amount
							orderFilled = true
							actualExecutedQty = -posAmt // Convert to positive
							if entryPrice, ok := pos["entryPrice"].(float64); ok && entryPrice > 0 {
								actualFillPrice = entryPrice
								actionRecord.Price = entryPrice
							}
							log.Printf("  ✓ Position verified in Binance: quantity=%.4f, entryPrice=%.4f", actualExecutedQty, actualFillPrice)
							break
						}
					}
				}
			}
		}

		// Final verification - fail only if all methods indicate order not filled
		if !orderFilled {
			return fmt.Errorf("order not filled: status=%s, orderId=%s, execQty=%.4f. Order may have been rejected, canceled, or expired", finalOrderStatus, orderIDStr, actualExecutedQty)
		}

		// Verify executed quantity is greater than 0 (final check)
		if actualExecutedQty <= 0 {
			return fmt.Errorf("order executed quantity is 0: status=%s, orderId=%s. Order was not filled", finalOrderStatus, orderIDStr)
		}

		if actualFillPrice == 0 {
			log.Printf("  ⚠ Warning: Could not get actual fill price, using market price")
			actualFillPrice = marketData.CurrentPrice
		}
	} else {
		actualFillPrice = marketData.CurrentPrice
		actualExecutedQty = quantity
	}

	// Save position record to database
	if db, ok := at.database.(*cfg.Database); ok && db != nil {
		openedAt := time.Now()
		err := db.SavePosition(at.id, decision.Symbol, "short", actualFillPrice, 0, actualExecutedQty, entryFee, 0, 0, decision.Leverage, orderIDStr, "", openedAt, nil)
		if err != nil {
			log.Printf("  ⚠ Warning: Failed to save position record: %v", err)
		}
	}

	// Record open position time
	posKey := decision.Symbol + "_short"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	// Set stop loss and take profit
	if err := at.trader.SetStopLoss(decision.Symbol, "SHORT", actualExecutedQty, decision.StopLoss); err != nil {
		log.Printf("  ⚠ Failed to set stop loss: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "SHORT", actualExecutedQty, decision.TakeProfit); err != nil {
		log.Printf("  ⚠ Failed to set take profit: %v", err)
	}

	return nil
}

// executeCloseLongWithRecord executes close long position and records detailed information
func (at *AutoTrader) executeCloseLongWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🔄 Close long position: %s", decision.Symbol)

	// Get entry price from GetPositions API before closing
	var entryPrice float64
	var positionQuantity float64
	var leverage int
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
				if ep, ok := pos["entryPrice"].(float64); ok {
					entryPrice = ep
				}
				if qty, ok := pos["positionAmt"].(float64); ok {
					positionQuantity = qty
				}
				if lev, ok := pos["leverage"].(float64); ok {
					leverage = int(lev)
				}
				break
			}
		}
	}
	if entryPrice == 0 {
		log.Printf("  ⚠ Warning: Could not get entry price from positions, using market price")
		marketData, err := market.Get(decision.Symbol)
		if err != nil {
			return err
		}
		entryPrice = marketData.CurrentPrice
	}

	// Close position
	order, err := at.trader.CloseLong(decision.Symbol, 0) // 0 = close all
	if err != nil {
		return err
	}

	// Extract order ID
	var orderIDStr string
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
		orderIDStr = fmt.Sprintf("%d", orderID)
	} else if orderID, ok := order["orderId"].(string); ok {
		orderIDStr = orderID
		// Try to parse string order ID to int64
		if parsedID, err := strconv.ParseInt(orderID, 10, 64); err == nil {
			actionRecord.OrderID = parsedID
		} else {
			actionRecord.OrderID = 0
		}
	} else {
		log.Printf("  ⚠ Warning: Could not extract order ID from order response")
		orderIDStr = ""
		actionRecord.OrderID = 0
	}

	// Poll GetOrderStatus to get actual fill price
	var actualFillPrice float64
	var actualExecutedQty float64
	var exitFee float64
	if orderIDStr != "" {
		// Wait 500ms before first poll
		time.Sleep(500 * time.Millisecond)

		// Poll up to 3 times with 1 second delay
		for attempt := 0; attempt < 3; attempt++ {
			orderStatus, err := at.trader.GetOrderStatus(decision.Symbol, orderIDStr)
			if err == nil {
				if avgPrice, ok := orderStatus["avgPrice"].(float64); ok && avgPrice > 0 {
					actualFillPrice = avgPrice
					actionRecord.Price = avgPrice
				}
				if execQty, ok := orderStatus["executedQty"].(float64); ok && execQty > 0 {
					actualExecutedQty = execQty
					actionRecord.Quantity = execQty
				}
				if commission, ok := orderStatus["commission"].(float64); ok {
					exitFee = commission
				}
				status, _ := orderStatus["status"].(string)
				if status == "FILLED" || status == "PARTIALLY_FILLED" {
					log.Printf("  ✓ Order filled: avgPrice=%.4f, executedQty=%.4f, fee=%.4f", actualFillPrice, actualExecutedQty, exitFee)
					break
				}
			}
			if attempt < 2 {
				time.Sleep(1 * time.Second)
			}
		}
		if actualFillPrice == 0 {
			log.Printf("  ⚠ Warning: Could not get actual fill price, using market price")
			marketData, err := market.Get(decision.Symbol)
			if err == nil {
				actualFillPrice = marketData.CurrentPrice
			} else {
				actualFillPrice = entryPrice
			}
		}
	} else {
		marketData, err := market.Get(decision.Symbol)
		if err == nil {
			actualFillPrice = marketData.CurrentPrice
		} else {
			actualFillPrice = entryPrice
		}
		actualExecutedQty = positionQuantity
	}

	// Calculate realized P&L: (exitPrice - entryPrice) * quantity - entryFee - exitFee (for long)
	realizedPnL := (actualFillPrice-entryPrice)*actualExecutedQty - exitFee
	// Note: entryFee is not available here, would need to retrieve from position record

	// Save position record to database
	if db, ok := at.database.(*cfg.Database); ok && db != nil {
		closedAt := time.Now()
		// Try to find existing open position record to update
		openPositions, err := db.GetOpenPositions(at.id)
		if err == nil {
			for _, pos := range openPositions {
				if pos.Symbol == decision.Symbol && pos.Side == "long" {
					// Update existing position
					err := db.SavePosition(at.id, decision.Symbol, "long", pos.EntryPrice, actualFillPrice, actualExecutedQty, pos.EntryFee, exitFee, realizedPnL, leverage, pos.OrderIDOpen, orderIDStr, pos.OpenedAt, &closedAt)
					if err != nil {
						log.Printf("  ⚠ Warning: Failed to update position record: %v", err)
					}
					break
				}
			}
		}
		// If no existing position found, create new record
		if err != nil {
			openedAt := time.Now().Add(-24 * time.Hour) // Estimate opened 24h ago if not found
			err := db.SavePosition(at.id, decision.Symbol, "long", entryPrice, actualFillPrice, actualExecutedQty, 0, exitFee, realizedPnL, leverage, "", orderIDStr, openedAt, &closedAt)
			if err != nil {
				log.Printf("  ⚠ Warning: Failed to save position record: %v", err)
			}
		}
	}

	log.Printf("  ✓ Position closed successfully: entry=%.4f, exit=%.4f, pnl=%.4f", entryPrice, actualFillPrice, realizedPnL)
	return nil
}

// CloseLong manually close long position (public method for API calls)
func (at *AutoTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	return at.trader.CloseLong(symbol, quantity)
}

// CloseShort manually close short position (public method for API calls)
func (at *AutoTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return at.trader.CloseShort(symbol, quantity)
}

// executeCloseShortWithRecord executes close short position and records detailed information
func (at *AutoTrader) executeCloseShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🔄 Close short position: %s", decision.Symbol)

	// Get entry price from GetPositions API before closing
	var entryPrice float64
	var positionQuantity float64
	var leverage int
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
				if ep, ok := pos["entryPrice"].(float64); ok {
					entryPrice = ep
				}
				if qty, ok := pos["positionAmt"].(float64); ok {
					positionQuantity = qty
				}
				if lev, ok := pos["leverage"].(float64); ok {
					leverage = int(lev)
				}
				break
			}
		}
	}
	if entryPrice == 0 {
		log.Printf("  ⚠ Warning: Could not get entry price from positions, using market price")
		marketData, err := market.Get(decision.Symbol)
		if err != nil {
			return err
		}
		entryPrice = marketData.CurrentPrice
	}

	// Close position
	order, err := at.trader.CloseShort(decision.Symbol, 0) // 0 = close all
	if err != nil {
		return err
	}

	// Extract order ID
	var orderIDStr string
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
		orderIDStr = fmt.Sprintf("%d", orderID)
	} else if orderID, ok := order["orderId"].(string); ok {
		orderIDStr = orderID
		// Try to parse string order ID to int64
		if parsedID, err := strconv.ParseInt(orderID, 10, 64); err == nil {
			actionRecord.OrderID = parsedID
		} else {
			actionRecord.OrderID = 0
		}
	} else {
		log.Printf("  ⚠ Warning: Could not extract order ID from order response")
		orderIDStr = ""
		actionRecord.OrderID = 0
	}

	// Poll GetOrderStatus to get actual fill price
	var actualFillPrice float64
	var actualExecutedQty float64
	var exitFee float64
	if orderIDStr != "" {
		// Wait 500ms before first poll
		time.Sleep(500 * time.Millisecond)

		// Poll up to 3 times with 1 second delay
		for attempt := 0; attempt < 3; attempt++ {
			orderStatus, err := at.trader.GetOrderStatus(decision.Symbol, orderIDStr)
			if err == nil {
				if avgPrice, ok := orderStatus["avgPrice"].(float64); ok && avgPrice > 0 {
					actualFillPrice = avgPrice
					actionRecord.Price = avgPrice
				}
				if execQty, ok := orderStatus["executedQty"].(float64); ok && execQty > 0 {
					actualExecutedQty = execQty
					actionRecord.Quantity = execQty
				}
				if commission, ok := orderStatus["commission"].(float64); ok {
					exitFee = commission
				}
				status, _ := orderStatus["status"].(string)
				if status == "FILLED" || status == "PARTIALLY_FILLED" {
					log.Printf("  ✓ Order filled: avgPrice=%.4f, executedQty=%.4f, fee=%.4f", actualFillPrice, actualExecutedQty, exitFee)
					break
				}
			}
			if attempt < 2 {
				time.Sleep(1 * time.Second)
			}
		}
		if actualFillPrice == 0 {
			log.Printf("  ⚠ Warning: Could not get actual fill price, using market price")
			marketData, err := market.Get(decision.Symbol)
			if err == nil {
				actualFillPrice = marketData.CurrentPrice
			} else {
				actualFillPrice = entryPrice
			}
		}
	} else {
		marketData, err := market.Get(decision.Symbol)
		if err == nil {
			actualFillPrice = marketData.CurrentPrice
		} else {
			actualFillPrice = entryPrice
		}
		actualExecutedQty = positionQuantity
	}

	// Calculate realized P&L: (entryPrice - exitPrice) * quantity - entryFee - exitFee (for short)
	realizedPnL := (entryPrice-actualFillPrice)*actualExecutedQty - exitFee
	// Note: entryFee is not available here, would need to retrieve from position record

	// Save position record to database
	if db, ok := at.database.(*cfg.Database); ok && db != nil {
		closedAt := time.Now()
		// Try to find existing open position record to update
		openPositions, err := db.GetOpenPositions(at.id)
		if err == nil {
			for _, pos := range openPositions {
				if pos.Symbol == decision.Symbol && pos.Side == "short" {
					// Update existing position
					err := db.SavePosition(at.id, decision.Symbol, "short", pos.EntryPrice, actualFillPrice, actualExecutedQty, pos.EntryFee, exitFee, realizedPnL, leverage, pos.OrderIDOpen, orderIDStr, pos.OpenedAt, &closedAt)
					if err != nil {
						log.Printf("  ⚠ Warning: Failed to update position record: %v", err)
					}
					break
				}
			}
		}
		// If no existing position found, create new record
		if err != nil {
			openedAt := time.Now().Add(-24 * time.Hour) // Estimate opened 24h ago if not found
			err := db.SavePosition(at.id, decision.Symbol, "short", entryPrice, actualFillPrice, actualExecutedQty, 0, exitFee, realizedPnL, leverage, "", orderIDStr, openedAt, &closedAt)
			if err != nil {
				log.Printf("  ⚠ Warning: Failed to save position record: %v", err)
			}
		}
	}

	log.Printf("  ✓ Position closed successfully: entry=%.4f, exit=%.4f, pnl=%.4f", entryPrice, actualFillPrice, realizedPnL)
	return nil
}

// executeUpdateStopLossWithRecord executes adjust stop loss and records detailed information
func (at *AutoTrader) executeUpdateStopLossWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🎯 Adjust stop loss: %s → %.2f", decision.Symbol, decision.NewStopLoss)

	// Get current price
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// Get current positions
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	// Find target position
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, ok := pos["symbol"].(string)
		if !ok {
			continue
		}
		posAmt, ok := pos["positionAmt"].(float64)
		if !ok {
			continue
		}
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("position does not exist: %s", decision.Symbol)
	}

	// Get position direction and quantity
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// Validate new stop loss price reasonableness
	if positionSide == "LONG" && decision.NewStopLoss >= marketData.CurrentPrice {
		return fmt.Errorf("long stop loss must be below current price (current: %.2f, new stop loss: %.2f)", marketData.CurrentPrice, decision.NewStopLoss)
	}
	if positionSide == "SHORT" && decision.NewStopLoss <= marketData.CurrentPrice {
		return fmt.Errorf("short stop loss must be above current price (current: %.2f, new stop loss: %.2f)", marketData.CurrentPrice, decision.NewStopLoss)
	}

	// ⚠️ Defensive check: detect if bidirectional positions exist (shouldn't happen, but provides protection)
	var hasOppositePosition bool
	oppositeSide := ""
	for _, pos := range positions {
		symbol, ok := pos["symbol"].(string)
		if !ok {
			continue
		}
		posSide, ok := pos["side"].(string)
		if !ok {
			continue
		}
		posAmt, ok := pos["positionAmt"].(float64)
		if !ok {
			continue
		}
		if symbol == decision.Symbol && posAmt != 0 && strings.ToUpper(posSide) != positionSide {
			hasOppositePosition = true
			oppositeSide = strings.ToUpper(posSide)
			break
		}
	}

	if hasOppositePosition {
		log.Printf("  🚨 Warning: detected bidirectional positions for %s (%s + %s), this violates strategy rules",
			decision.Symbol, positionSide, oppositeSide)
		log.Printf("  🚨 Canceling stop loss orders will affect orders in both directions, please check if this was caused by manual user operation")
		log.Printf("  🚨 Recommendation: manually close one direction's position, or check if there's a system BUG")
	}

	// Cancel old stop loss orders (only delete stop loss orders, don't affect take profit orders)
	// Note: If bidirectional positions exist, this will delete stop loss orders in both directions
	if err := at.trader.CancelStopLossOrders(decision.Symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel old stop loss orders: %v", err)
		// Don't interrupt execution, continue setting new stop loss
	}

	// Call exchange API to modify stop loss
	quantity := math.Abs(positionAmt)
	err = at.trader.SetStopLoss(decision.Symbol, positionSide, quantity, decision.NewStopLoss)
	if err != nil {
		return fmt.Errorf("failed to modify stop loss: %w", err)
	}

	log.Printf("  ✓ Stop loss adjusted: %.2f (current price: %.2f)", decision.NewStopLoss, marketData.CurrentPrice)
	return nil
}

// executeUpdateTakeProfitWithRecord executes adjust take profit and records detailed information
func (at *AutoTrader) executeUpdateTakeProfitWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🎯 Adjust take profit: %s → %.2f", decision.Symbol, decision.NewTakeProfit)

	// Get current price
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// Get current positions
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	// Find target position
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, ok := pos["symbol"].(string)
		if !ok {
			continue
		}
		posAmt, ok := pos["positionAmt"].(float64)
		if !ok {
			continue
		}
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("position does not exist: %s", decision.Symbol)
	}

	// Get position direction and quantity
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// Validate new take profit price reasonableness
	if positionSide == "LONG" && decision.NewTakeProfit <= marketData.CurrentPrice {
		return fmt.Errorf("long take profit must be above current price (current: %.2f, new take profit: %.2f)", marketData.CurrentPrice, decision.NewTakeProfit)
	}
	if positionSide == "SHORT" && decision.NewTakeProfit >= marketData.CurrentPrice {
		return fmt.Errorf("short take profit must be below current price (current: %.2f, new take profit: %.2f)", marketData.CurrentPrice, decision.NewTakeProfit)
	}

	// ⚠️ Defensive check: detect if bidirectional positions exist (shouldn't happen, but provides protection)
	var hasOppositePosition bool
	oppositeSide := ""
	for _, pos := range positions {
		symbol, ok := pos["symbol"].(string)
		if !ok {
			continue
		}
		posSide, ok := pos["side"].(string)
		if !ok {
			continue
		}
		posAmt, ok := pos["positionAmt"].(float64)
		if !ok {
			continue
		}
		if symbol == decision.Symbol && posAmt != 0 && strings.ToUpper(posSide) != positionSide {
			hasOppositePosition = true
			oppositeSide = strings.ToUpper(posSide)
			break
		}
	}

	if hasOppositePosition {
		log.Printf("  🚨 Warning: detected bidirectional positions for %s (%s + %s), this violates strategy rules",
			decision.Symbol, positionSide, oppositeSide)
		log.Printf("  🚨 Canceling take profit orders will affect orders in both directions, please check if this was caused by manual user operation")
		log.Printf("  🚨 Recommendation: manually close one direction's position, or check if there's a system BUG")
	}

	// Cancel old take profit orders (only delete take profit orders, don't affect stop loss orders)
	// Note: If bidirectional positions exist, this will delete take profit orders in both directions
	if err := at.trader.CancelTakeProfitOrders(decision.Symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel old take profit orders: %v", err)
		// Don't interrupt execution, continue setting new take profit
	}

	// Call exchange API to modify take profit
	quantity := math.Abs(positionAmt)
	err = at.trader.SetTakeProfit(decision.Symbol, positionSide, quantity, decision.NewTakeProfit)
	if err != nil {
		return fmt.Errorf("failed to modify take profit: %w", err)
	}

	log.Printf("  ✓ Take profit adjusted: %.2f (current price: %.2f)", decision.NewTakeProfit, marketData.CurrentPrice)
	return nil
}

// executePartialCloseWithRecord executes partial close position and records detailed information
func (at *AutoTrader) executePartialCloseWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📊 Partial close position: %s %.1f%%", decision.Symbol, decision.ClosePercentage)

	// Validate percentage range
	if decision.ClosePercentage <= 0 || decision.ClosePercentage > 100 {
		return fmt.Errorf("close percentage must be between 0-100, current: %.1f", decision.ClosePercentage)
	}

	// Get current price
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// Get current positions
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	// Find target position
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, ok := pos["symbol"].(string)
		if !ok {
			continue
		}
		posAmt, ok := pos["positionAmt"].(float64)
		if !ok {
			continue
		}
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("position does not exist: %s", decision.Symbol)
	}

	// Get position direction and quantity
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// Calculate close quantity
	totalQuantity := math.Abs(positionAmt)
	closeQuantity := totalQuantity * (decision.ClosePercentage / 100.0)
	actionRecord.Quantity = closeQuantity

	// ✅ Layer 2: Minimum position check (prevent small remainder)
	markPrice, ok := targetPosition["markPrice"].(float64)
	if !ok || markPrice <= 0 {
		return fmt.Errorf("unable to parse current price, cannot perform minimum position check")
	}

	currentPositionValue := totalQuantity * markPrice
	remainingQuantity := totalQuantity - closeQuantity
	remainingValue := remainingQuantity * markPrice

	const MIN_POSITION_VALUE = 10.0 // Minimum position value 10 USDT (aligned with exchange minimum, small positions should be fully closed)

	if remainingValue > 0 && remainingValue <= MIN_POSITION_VALUE {
		log.Printf("⚠️ Detected remaining position %.2f USDT < %.0f USDT after partial_close",
			remainingValue, MIN_POSITION_VALUE)
		log.Printf("  → Current position value: %.2f USDT, close %.1f%%, remaining: %.2f USDT",
			currentPositionValue, decision.ClosePercentage, remainingValue)
		log.Printf("  → Auto-correcting to full close to avoid uncloseable small remainder")

		// 🔄 Auto-correct to full close
		if positionSide == "LONG" {
			decision.Action = "close_long"
			log.Printf("  ✓ Corrected to: close_long")
			return at.executeCloseLongWithRecord(decision, actionRecord)
		} else {
			decision.Action = "close_short"
			log.Printf("  ✓ Corrected to: close_short")
			return at.executeCloseShortWithRecord(decision, actionRecord)
		}
	}

	// Execute close
	var order map[string]interface{}
	if positionSide == "LONG" {
		order, err = at.trader.CloseLong(decision.Symbol, closeQuantity)
	} else {
		order, err = at.trader.CloseShort(decision.Symbol, closeQuantity)
	}

	if err != nil {
		return fmt.Errorf("partial close failed: %w", err)
	}

	// Extract order ID
	var orderIDStr string
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
		orderIDStr = fmt.Sprintf("%d", orderID)
	} else if orderID, ok := order["orderId"].(string); ok {
		orderIDStr = orderID
		// Try to parse string order ID to int64
		if parsedID, err := strconv.ParseInt(orderID, 10, 64); err == nil {
			actionRecord.OrderID = parsedID
		} else {
			actionRecord.OrderID = 0
		}
	} else {
		log.Printf("  ⚠ Warning: Could not extract order ID from order response")
		orderIDStr = ""
		actionRecord.OrderID = 0
	}

	// Poll GetOrderStatus to get actual fill price
	var actualFillPrice float64
	var actualExecutedQty float64
	var exitFee float64
	if orderIDStr != "" {
		// Wait 500ms before first poll
		time.Sleep(500 * time.Millisecond)

		// Poll up to 3 times with 1 second delay
		for attempt := 0; attempt < 3; attempt++ {
			orderStatus, err := at.trader.GetOrderStatus(decision.Symbol, orderIDStr)
			if err == nil {
				if avgPrice, ok := orderStatus["avgPrice"].(float64); ok && avgPrice > 0 {
					actualFillPrice = avgPrice
					actionRecord.Price = avgPrice
				}
				if execQty, ok := orderStatus["executedQty"].(float64); ok && execQty > 0 {
					actualExecutedQty = execQty
					actionRecord.Quantity = execQty
				}
				if commission, ok := orderStatus["commission"].(float64); ok {
					exitFee = commission
				}
				status, _ := orderStatus["status"].(string)
				if status == "FILLED" || status == "PARTIALLY_FILLED" {
					log.Printf("  ✓ Order filled: avgPrice=%.4f, executedQty=%.4f, fee=%.4f", actualFillPrice, actualExecutedQty, exitFee)
					break
				}
			}
			if attempt < 2 {
				time.Sleep(1 * time.Second)
			}
		}
		if actualFillPrice == 0 {
			log.Printf("  ⚠ Warning: Could not get actual fill price, using market price")
			actualFillPrice = marketData.CurrentPrice
		}
	} else {
		actualFillPrice = marketData.CurrentPrice
		actualExecutedQty = closeQuantity
	}

	log.Printf("  ✓ Partial close succeeded: closed %.4f (%.1f%%), remaining %.4f, fillPrice=%.4f",
		actualExecutedQty, decision.ClosePercentage, remainingQuantity, actualFillPrice)

	// ✅ Step 4: Restore stop loss and take profit (prevent remaining position from being unprotected)
	// Important: Exchanges like Binance automatically cancel original TP/SL orders after partial close (due to quantity mismatch)
	// If AI provided new stop loss/take profit prices, reset protection for remaining position
	if decision.NewStopLoss > 0 {
		log.Printf("  → Restoring stop loss order for remaining position %.4f: %.2f", remainingQuantity, decision.NewStopLoss)
		err = at.trader.SetStopLoss(decision.Symbol, positionSide, remainingQuantity, decision.NewStopLoss)
		if err != nil {
			log.Printf("  ⚠️ Failed to restore stop loss: %v (does not affect close result)", err)
		}
	}

	if decision.NewTakeProfit > 0 {
		log.Printf("  → Restoring take profit order for remaining position %.4f: %.2f", remainingQuantity, decision.NewTakeProfit)
		err = at.trader.SetTakeProfit(decision.Symbol, positionSide, remainingQuantity, decision.NewTakeProfit)
		if err != nil {
			log.Printf("  ⚠️ Failed to restore take profit: %v (does not affect close result)", err)
		}
	}

	// If AI didn't provide new stop loss/take profit, log warning
	if decision.NewStopLoss <= 0 && decision.NewTakeProfit <= 0 {
		log.Printf("  ⚠️⚠️⚠️ Warning: AI did not provide new stop loss/take profit prices after partial close")
		log.Printf("  → Remaining position %.4f (value %.2f USDT) currently has no stop loss/take profit protection", remainingQuantity, remainingValue)
		log.Printf("  → Recommendation: Include new_stop_loss and new_take_profit fields in partial_close decision")
	}

	return nil
}

// GetID get trader ID
func (at *AutoTrader) GetID() string {
	return at.id
}

// GetName get trader name
func (at *AutoTrader) GetName() string {
	return at.name
}

// GetAIModel get AI model
func (at *AutoTrader) GetAIModel() string {
	return at.aiModel
}

// GetExchange get exchange
func (at *AutoTrader) GetExchange() string {
	return at.exchange
}

// GetShowInCompetition returns whether trader should be shown in competition
func (at *AutoTrader) GetShowInCompetition() bool {
	return at.showInCompetition
}

// SetShowInCompetition sets whether trader should be shown in competition
func (at *AutoTrader) SetShowInCompetition(show bool) {
	at.showInCompetition = show
}

// SetCustomPrompt set custom trading strategy prompt
func (at *AutoTrader) SetCustomPrompt(prompt string) {
	at.customPrompt = prompt
}

// SetOverrideBasePrompt set whether to override base prompt
func (at *AutoTrader) SetOverrideBasePrompt(override bool) {
	at.overrideBasePrompt = override
}

// SetSystemPromptTemplate set system prompt template
func (at *AutoTrader) SetSystemPromptTemplate(templateName string) {
	at.systemPromptTemplate = templateName
}

// GetSystemPromptTemplate get current system prompt template name
func (at *AutoTrader) GetSystemPromptTemplate() string {
	return at.systemPromptTemplate
}

// GetDecisionLogger get decision logger
func (at *AutoTrader) GetDecisionLogger() logger.IDecisionLogger {
	return at.decisionLogger
}

// saveEquityHistoryToDB saves equity history point to database if database is available
func (at *AutoTrader) saveEquityHistoryToDB(record *logger.DecisionRecord) {
	if db, ok := at.database.(*cfg.Database); ok && db != nil {
		// Calculate PnL and PnL percentage
		totalEquity := record.AccountState.TotalBalance + record.AccountState.TotalUnrealizedProfit
		totalPnL := totalEquity - at.initialBalance
		totalPnLPct := 0.0
		if at.initialBalance > 0 {
			totalPnLPct = (totalPnL / at.initialBalance) * 100
		}

		// Save to database (ignore errors - file logging is primary, DB is secondary)
		err := db.SaveEquityHistory(
			at.id,
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
			log.Printf("⚠️ [%s] Failed to save equity history to database: %v", at.name, err)
		}
	}
}

// logDecisionAndSaveEquity logs decision and saves equity history to database
func (at *AutoTrader) logDecisionAndSaveEquity(record *logger.DecisionRecord) error {
	err := at.decisionLogger.LogDecision(record)
	if err == nil {
		// Save equity history to database for persistence across deployments
		at.saveEquityHistoryToDB(record)
	}
	return err
}

// GetStatus get system status (for API)
func (at *AutoTrader) GetStatus() map[string]interface{} {
	aiProvider := "DeepSeek"
	if at.config.UseQwen {
		aiProvider = "Qwen"
	}

	return map[string]interface{}{
		"trader_id":       at.id,
		"trader_name":     at.name,
		"ai_model":        at.aiModel,
		"exchange":        at.exchange,
		"is_running":      at.isRunning,
		"use_tradingview": at.config.UseTradingView,
		"start_time":      at.startTime.Format(time.RFC3339),
		"runtime_minutes": int(time.Since(at.startTime).Minutes()),
		"call_count":      at.callCount,
		"initial_balance": at.initialBalance,
		"scan_interval":   at.config.ScanInterval.String(),
		"stop_until":      at.stopUntil.Format(time.RFC3339),
		"last_reset_time": at.lastResetTime.Format(time.RFC3339),
		"ai_provider":     aiProvider,
	}
}

// GetAccountInfo get account information (for API)
func (at *AutoTrader) GetAccountInfo() (map[string]interface{}, error) {
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	// Get account fields
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0

	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Total Equity = wallet balance + unrealized profit/loss
	totalEquity := totalWalletBalance + totalUnrealizedProfit

	// Get positions to calculate total margin
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	totalMarginUsed := 0.0
	totalUnrealizedPnLCalculated := 0.0
	for _, pos := range positions {
		// Safe type assertions to prevent nil interface conversion panic
		markPrice, ok := pos["markPrice"].(float64)
		if !ok {
			continue // Skip invalid position
		}
		quantity, ok := pos["positionAmt"].(float64)
		if !ok {
			continue // Skip invalid position
		}
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl, ok := pos["unRealizedProfit"].(float64)
		if !ok {
			unrealizedPnl = 0 // Default to 0 if missing
		}
		totalUnrealizedPnLCalculated += unrealizedPnl

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed
	}

	// Verify consistency of unrealized profit/loss (API value vs calculated from positions)
	diff := math.Abs(totalUnrealizedProfit - totalUnrealizedPnLCalculated)
	if diff > 0.1 { // Allow 0.01 USDT error
		log.Printf("⚠️ Unrealized profit/loss inconsistency: API=%.4f, calculated=%.4f, diff=%.4f",
			totalUnrealizedProfit, totalUnrealizedPnLCalculated, diff)
	}

	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	} else {
		log.Printf("⚠️ Initial Balance abnormal: %.2f, unable to calculate PNL percentage", at.initialBalance)
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	return map[string]interface{}{
		// Core fields
		"total_equity":      totalEquity,           // Account equity = wallet + unrealized
		"wallet_balance":    totalWalletBalance,    // Wallet balance (excluding unrealized profit/loss)
		"unrealized_profit": totalUnrealizedProfit, // Unrealized profit/loss (exchange API official value)
		"available_balance": availableBalance,      // Available balance

		// Profit/loss statistics
		"total_pnl":       totalPnL,          // Total PnL = equity - initial
		"total_pnl_pct":   totalPnLPct,       // Total PnL percentage
		"initial_balance": at.initialBalance, // Initial balance
		"daily_pnl":       at.dailyPnL,       // Daily PnL

		// Position information
		"position_count":  len(positions),  // Position count
		"margin_used":     totalMarginUsed, // Margin used
		"margin_used_pct": marginUsedPct,   // Margin usage rate
	}, nil
}

// GetPositions get position list (for API)
func (at *AutoTrader) GetPositions() ([]map[string]interface{}, error) {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		// Safe type assertions to prevent nil interface conversion panic
		symbol, ok := pos["symbol"].(string)
		if !ok || symbol == "" {
			continue // Skip invalid position
		}
		side, ok := pos["side"].(string)
		if !ok || side == "" {
			continue // Skip invalid position
		}
		entryPrice, ok := pos["entryPrice"].(float64)
		if !ok {
			continue // Skip invalid position
		}
		markPrice, ok := pos["markPrice"].(float64)
		if !ok {
			continue // Skip invalid position
		}
		quantity, ok := pos["positionAmt"].(float64)
		if !ok {
			continue // Skip invalid position
		}
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl, ok := pos["unRealizedProfit"].(float64)
		if !ok {
			unrealizedPnl = 0 // Default to 0 if missing
		}
		liquidationPrice, ok := pos["liquidationPrice"].(float64)
		if !ok {
			liquidationPrice = 0 // Default to 0 if missing
		}

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		// Calculate used margin
		marginUsed := (quantity * markPrice) / float64(leverage)

		// Calculate profit/loss percentage (based on margin)
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		result = append(result, map[string]interface{}{
			"symbol":             symbol,
			"side":               side,
			"entry_price":        entryPrice,
			"mark_price":         markPrice,
			"quantity":           quantity,
			"leverage":           leverage,
			"unrealized_pnl":     unrealizedPnl,
			"unrealized_pnl_pct": pnlPct,
			"liquidation_price":  liquidationPrice,
			"margin_used":        marginUsed,
		})
	}

	return result, nil
}

// calculatePnLPercentage calculates profit/loss percentage (based on margin, automatically considers leverage)
// Return rate = unrealized profit/loss / margin × 100%
func calculatePnLPercentage(unrealizedPnl, marginUsed float64) float64 {
	if marginUsed > 0 {
		return (unrealizedPnl / marginUsed) * 100
	}
	return 0.0
}

// sortDecisionsByPriority sorts decisions: close positions first, then open positions, finally hold/wait
// This avoids position stacking exceeding limits when swapping positions
func sortDecisionsByPriority(decisions []decision.Decision) []decision.Decision {
	if len(decisions) <= 1 {
		return decisions
	}

	// Define priority
	getActionPriority := func(action string) int {
		switch action {
		case "close_long", "close_short", "partial_close":
			return 1 // Highest priority: close positions first (including partial close)
		case "update_stop_loss", "update_take_profit":
			return 2 // Adjust position stop loss/take profit
		case "open_long", "open_short":
			return 3 // Secondary priority: open positions later
		case "hold", "wait":
			return 4 // Lowest priority: wait
		default:
			return 999 // Unknown actions go last
		}
	}

	// Copy decision list
	sorted := make([]decision.Decision, len(decisions))
	copy(sorted, decisions)

	// Sort by priority
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if getActionPriority(sorted[i].Action) > getActionPriority(sorted[j].Action) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// getCandidateCoins gets trader's candidate coin list
func (at *AutoTrader) getCandidateCoins() ([]decision.CandidateCoin, error) {
	if len(at.tradingCoins) == 0 {
		// Use default coin list configured in database
		var candidateCoins []decision.CandidateCoin

		if len(at.defaultCoins) > 0 {
			// Use default coins configured in database
			for _, coin := range at.defaultCoins {
				symbol := normalizeSymbol(coin)
				candidateCoins = append(candidateCoins, decision.CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"default"}, // Mark as database default coins
				})
			}
			log.Printf("📋 [%s] Using database default coins: %d coins %v",
				at.name, len(candidateCoins), at.defaultCoins)
			return candidateCoins, nil
		} else {
			// If no default coins configured in database, use AI500+OI Top as fallback
			const ai500Limit = 20 // AI500 takes top 20 highest-rated coins

			mergedPool, err := pool.GetMergedCoinPool(ai500Limit)
			if err != nil {
				return nil, fmt.Errorf("failed to get merged coin pool: %w", err)
			}

			// Build candidate coin list (include source information)
			for _, symbol := range mergedPool.AllSymbols {
				sources := mergedPool.SymbolSources[symbol]
				candidateCoins = append(candidateCoins, decision.CandidateCoin{
					Symbol:  symbol,
					Sources: sources, // "ai500" and/or "oi_top"
				})
			}

			log.Printf("📋 [%s] No default coin config in database, using AI500+OI Top: AI500 top %d + OI_Top20 = total %d candidate coins",
				at.name, ai500Limit, len(candidateCoins))
			return candidateCoins, nil
		}
	} else {
		// Use custom coin list
		var candidateCoins []decision.CandidateCoin
		for _, coin := range at.tradingCoins {
			// Ensure coin format is correct (convert to uppercase USDT pair)
			symbol := normalizeSymbol(coin)
			candidateCoins = append(candidateCoins, decision.CandidateCoin{
				Symbol:  symbol,
				Sources: []string{"custom"}, // Mark as custom source
			})
		}

		log.Printf("📋 [%s] Using custom coins: %d coins %v",
			at.name, len(candidateCoins), at.tradingCoins)
		return candidateCoins, nil
	}
}

// normalizeSymbol normalizes coin symbol (ensure ends with USDT)
func normalizeSymbol(symbol string) string {
	// Convert to uppercase
	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	// Ensure ends with USDT
	if !strings.HasSuffix(symbol, "USDT") {
		symbol = symbol + "USDT"
	}

	return symbol
}

// Start drawdown monitoring
func (at *AutoTrader) startDrawdownMonitor() {
	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()

		ticker := time.NewTicker(1 * time.Minute) // Check every minute
		defer ticker.Stop()

		log.Println("📊 Started position drawdown monitoring (checking every minute)")

		for {
			select {
			case <-ticker.C:
				at.checkPositionDrawdown()
			case <-at.stopMonitorCh:
				log.Println("⏹ Stopped position drawdown monitoring")
				return
			}
		}
	}()
}

// Check position drawdown situation
func (at *AutoTrader) checkPositionDrawdown() {
	// Detect and log position closures (SL/TP)
	at.detectAndLogPositionClosures()

	// Get current positions
	positions, err := at.trader.GetPositions()
	if err != nil {
		log.Printf("❌ Drawdown monitoring: failed to get positions: %v", err)
		return
	}

	for _, pos := range positions {
		// Safe type assertions to prevent nil interface conversion panic
		symbol, ok := pos["symbol"].(string)
		if !ok || symbol == "" {
			continue // Skip invalid position
		}
		side, ok := pos["side"].(string)
		if !ok || side == "" {
			continue // Skip invalid position
		}
		entryPrice, ok := pos["entryPrice"].(float64)
		if !ok {
			continue // Skip invalid position
		}
		markPrice, ok := pos["markPrice"].(float64)
		if !ok {
			continue // Skip invalid position
		}
		quantity, ok := pos["positionAmt"].(float64)
		if !ok {
			continue // Skip invalid position
		}
		if quantity < 0 {
			quantity = -quantity // Short position quantity is negative, convert to positive
		}

		// Skip closed positions (quantity = 0)
		if quantity == 0 {
			continue
		}

		// Calculate current profit/loss percentage
		leverage := 10 // Default value
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		var currentPnLPct float64
		if side == "long" {
			currentPnLPct = ((markPrice - entryPrice) / entryPrice) * float64(leverage) * 100
		} else {
			currentPnLPct = ((entryPrice - markPrice) / entryPrice) * float64(leverage) * 100
		}

		// Construct position unique identifier (distinguish long/short)
		posKey := symbol + "_" + side

		// Get historical peak profit for this position
		at.peakPnLCacheMutex.RLock()
		peakPnLPct, exists := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		if !exists {
			// If no historical peak record, use current profit/loss as initial value
			peakPnLPct = currentPnLPct
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		} else {
			// Update peak cache
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		}

		// Calculate drawdown (decline from peak)
		var drawdownPct float64
		if peakPnLPct > 0 && currentPnLPct < peakPnLPct {
			drawdownPct = ((peakPnLPct - currentPnLPct) / peakPnLPct) * 100
		}

		// Check close condition: profit > 5% and drawdown > 40%
		if currentPnLPct > 5.0 && drawdownPct >= 40.0 {
			log.Printf("🚨 Triggered drawdown close condition: %s %s | Current profit: %.2f%% | Peak profit: %.2f%% | Drawdown: %.2f%%",
				symbol, side, currentPnLPct, peakPnLPct, drawdownPct)

			// Execute close
			if err := at.emergencyClosePosition(symbol, side); err != nil {
				log.Printf("❌ Drawdown close failed (%s %s): %v", symbol, side, err)
			} else {
				log.Printf("✅ Drawdown close succeeded: %s %s", symbol, side)
				// Clear position cache after close
				at.ClearPeakPnLCache(symbol, side)
			}
		} else if currentPnLPct > 5.0 {
			// Record situations approaching close condition (for debugging)
			log.Printf("📊 Drawdown monitoring: %s %s | Profit: %.2f%% | Peak: %.2f%% | Drawdown: %.2f%%",
				symbol, side, currentPnLPct, peakPnLPct, drawdownPct)
		}
	}
}

// detectAndLogPositionClosures detects positions that were closed by SL/TP and logs them as auto-close actions
func (at *AutoTrader) detectAndLogPositionClosures() {
	// Get current positions
	currentPositions, err := at.trader.GetPositions()
	if err != nil {
		log.Printf("⚠️ Position closure detection: failed to get positions: %v", err)
		return
	}

	// Build current position keys map (symbol_side -> true)
	currentPositionKeys := make(map[string]bool)
	currentPositionsMap := make(map[string]map[string]interface{})

	for _, pos := range currentPositions {
		symbol, ok := pos["symbol"].(string)
		if !ok || symbol == "" {
			continue
		}
		side, ok := pos["side"].(string)
		if !ok || side == "" {
			continue
		}
		quantity, ok := pos["positionAmt"].(float64)
		if !ok {
			continue
		}
		if quantity < 0 {
			quantity = -quantity
		}
		// Skip closed positions (quantity = 0)
		if quantity == 0 {
			continue
		}

		posKey := symbol + "_" + side
		currentPositionKeys[posKey] = true
		currentPositionsMap[posKey] = pos
	}

	// Get previous positions snapshot
	at.previousPositionsMutex.RLock()
	previousPositions := make(map[string]map[string]interface{})
	for k, v := range at.previousPositions {
		previousPositions[k] = v
	}
	at.previousPositionsMutex.RUnlock()

	// Detect disappeared positions (were in previous but not in current)
	var disappearedPositions []string
	for posKey := range previousPositions {
		if !currentPositionKeys[posKey] {
			disappearedPositions = append(disappearedPositions, posKey)
		}
	}

	if len(disappearedPositions) == 0 {
		// Update snapshot and return
		at.previousPositionsMutex.Lock()
		at.previousPositions = currentPositionsMap
		at.previousPositionsMutex.Unlock()
		return
	}

	// Get database reference
	db, ok := at.database.(*cfg.Database)
	if !ok {
		log.Printf("⚠️ Position closure detection: database type error")
		// Update snapshot anyway
		at.previousPositionsMutex.Lock()
		at.previousPositions = currentPositionsMap
		at.previousPositionsMutex.Unlock()
		return
	}

	// Check for closures within last 10 minutes
	timeWindow := 10 * time.Minute
	cutoffTime := time.Now().Add(-timeWindow)

	// Get recent position history
	recentHistory, err := db.GetPositionHistory(at.id, 50, 0)
	if err != nil {
		log.Printf("⚠️ Position closure detection: failed to get position history: %v", err)
		// Update snapshot anyway
		at.previousPositionsMutex.Lock()
		at.previousPositions = currentPositionsMap
		at.previousPositionsMutex.Unlock()
		return
	}

	// Process each disappeared position
	for _, posKey := range disappearedPositions {
		// Check if we've already logged this closure (deduplication)
		at.loggedClosuresMutex.RLock()
		lastLogged, alreadyLogged := at.loggedClosures[posKey]
		at.loggedClosuresMutex.RUnlock()

		if alreadyLogged && time.Since(lastLogged) < timeWindow {
			continue // Already logged recently, skip
		}

		// Parse symbol and side from posKey (format: symbol_side)
		// Split from right to handle symbols that may contain underscores
		lastUnderscore := strings.LastIndex(posKey, "_")
		if lastUnderscore == -1 || lastUnderscore == len(posKey)-1 {
			continue
		}
		symbol := posKey[:lastUnderscore]
		side := posKey[lastUnderscore+1:]
		if side != "long" && side != "short" {
			continue // Invalid side
		}

		// Check decision logs first to see if this was already logged as a manual or auto-close
		records, err := at.decisionLogger.GetLatestRecords(50)
		alreadyLoggedInDecisions := false
		if err == nil {
			for _, record := range records {
				for _, action := range record.Decisions {
					if action.Symbol == symbol &&
						(action.Action == "close_long" || action.Action == "close_short" ||
							action.Action == "auto_close_long" || action.Action == "auto_close_short") &&
						action.Timestamp.After(cutoffTime) {
						// Already logged, skip
						alreadyLoggedInDecisions = true
						break
					}
				}
				if alreadyLoggedInDecisions {
					break
				}
			}
		}

		if alreadyLoggedInDecisions {
			continue // Skip, already logged
		}

		// Find matching closed position in history
		var closedPosition *cfg.PositionRecord
		for _, histPos := range recentHistory {
			if histPos.Symbol == symbol && histPos.Side == side && histPos.ClosedAt != nil {
				// Check if closure is within time window
				if histPos.ClosedAt.After(cutoffTime) {
					closedPosition = histPos
					break
				}
			}
		}

		// If no closure found in history, try to get from previous snapshot
		if closedPosition == nil {
			prevPos, exists := previousPositions[posKey]
			if exists {
				// Try to find closure by checking if position was recently closed
				// Use previous position data to create a closure record
				entryPrice, _ := prevPos["entryPrice"].(float64)
				quantity, _ := prevPos["positionAmt"].(float64)
				if quantity < 0 {
					quantity = -quantity
				}
				leverage, _ := prevPos["leverage"].(float64)

				// Get current market price as exit price estimate
				marketData, err := market.Get(symbol)
				if err != nil {
					log.Printf("⚠️ Position closure detection: failed to get market data for %s: %v", symbol, err)
					continue
				}

				// Calculate P&L estimate
				var realizedPnL float64
				if side == "long" {
					realizedPnL = quantity * (marketData.CurrentPrice - entryPrice)
				} else {
					realizedPnL = quantity * (entryPrice - marketData.CurrentPrice)
				}

				// Create closure record from snapshot
				closedAt := time.Now()
				closedPosition = &cfg.PositionRecord{
					Symbol:      symbol,
					Side:        side,
					EntryPrice:  entryPrice,
					ExitPrice:   marketData.CurrentPrice,
					Quantity:    quantity,
					Leverage:    int(leverage),
					RealizedPnL: realizedPnL,
					ClosedAt:    &closedAt,
				}
			}
		}

		if closedPosition == nil {
			continue // No closure found, skip
		}

		// Update database position record with closure information
		// First, try to find the open position record in database
		openPositions, err := db.GetOpenPositions(at.id)
		var dbPosition *cfg.PositionRecord
		if err == nil {
			for _, pos := range openPositions {
				if pos.Symbol == symbol && pos.Side == side {
					dbPosition = pos
					break
				}
			}
		}

		// Determine close time
		closeTime := time.Now()
		if closedPosition.ClosedAt != nil {
			closeTime = *closedPosition.ClosedAt
		}

		// Get actual exit price - prefer from closedPosition (position history) if available,
		// otherwise use market price estimate
		actualExitPrice := closedPosition.ExitPrice
		if actualExitPrice <= 0 {
			// Fallback to market price if exit price not available
			marketData, err := market.Get(symbol)
			if err == nil {
				actualExitPrice = marketData.CurrentPrice
			} else {
				log.Printf("⚠️ Position closure detection: failed to get market data for %s: %v", symbol, err)
				actualExitPrice = closedPosition.EntryPrice // Last resort fallback
			}
		}

		// Calculate realized PnL accurately
		var realizedPnL float64
		var entryPrice float64
		var quantity float64
		var leverage int
		var entryFee float64
		var exitFee float64

		if dbPosition != nil {
			// Use data from database position record
			entryPrice = dbPosition.EntryPrice
			quantity = dbPosition.Quantity
			leverage = dbPosition.Leverage
			entryFee = dbPosition.EntryFee
			exitFee = dbPosition.ExitFee

			// Calculate realized PnL: (exitPrice - entryPrice) * quantity - fees
			if side == "long" {
				realizedPnL = (actualExitPrice - entryPrice) * quantity - entryFee - exitFee
			} else {
				realizedPnL = (entryPrice - actualExitPrice) * quantity - entryFee - exitFee
			}

			// Update database position record with closure information
			err := db.SavePosition(
				at.id,
				symbol,
				side,
				entryPrice,
				actualExitPrice,
				quantity,
				entryFee,
				exitFee,
				realizedPnL,
				leverage,
				dbPosition.OrderIDOpen,
				"", // OrderIDClose not available for auto-closed positions
				dbPosition.OpenedAt,
				&closeTime,
			)
			if err != nil {
				log.Printf("⚠️ Position closure detection: failed to update database position record for %s: %v", posKey, err)
			} else {
				log.Printf("✓ Updated database position record for %s %s (closed_at=%s)", symbol, side, closeTime.Format("2006-01-02 15:04:05"))
			}
		} else {
			// No database record found, use data from closedPosition
			entryPrice = closedPosition.EntryPrice
			quantity = closedPosition.Quantity
			leverage = closedPosition.Leverage
			realizedPnL = closedPosition.RealizedPnL

			// Recalculate PnL with actual exit price if it was estimated
			if closedPosition.ExitPrice != actualExitPrice {
				if side == "long" {
					realizedPnL = (actualExitPrice - entryPrice) * quantity
				} else {
					realizedPnL = (entryPrice - actualExitPrice) * quantity
				}
			}

			// Create new position record in database (estimated open time)
			estimatedOpenTime := closeTime.Add(-24 * time.Hour) // Estimate opened 24h ago if not found
			err := db.SavePosition(
				at.id,
				symbol,
				side,
				entryPrice,
				actualExitPrice,
				quantity,
				0, // Entry fee not available
				0, // Exit fee not available
				realizedPnL,
				leverage,
				"", // OrderIDOpen not available
				"", // OrderIDClose not available
				estimatedOpenTime,
				&closeTime,
			)
			if err != nil {
				log.Printf("⚠️ Position closure detection: failed to create database position record for %s: %v", posKey, err)
			} else {
				log.Printf("✓ Created database position record for %s %s (estimated open_time=%s, closed_at=%s)", symbol, side, estimatedOpenTime.Format("2006-01-02 15:04:05"), closeTime.Format("2006-01-02 15:04:05"))
			}
		}

		// Update closedPosition with accurate values for decision record
		closedPosition.ExitPrice = actualExitPrice
		closedPosition.RealizedPnL = realizedPnL
		closedPosition.ClosedAt = &closeTime

		// Cancel remaining stop orders (SL/TP) for this symbol when position is auto-closed
		// When SL is hit, TP order should be cancelled, and vice versa
		// Using CancelStopOrders to cancel both SL and TP orders for safety
		if err := at.trader.CancelStopOrders(symbol); err != nil {
			log.Printf("⚠️ Position closure detection: failed to cancel remaining stop orders for %s: %v", symbol, err)
			// Don't fail the closure detection if order cancellation fails - log and continue
		} else {
			log.Printf("✓ Cancelled remaining stop orders (SL/TP) for %s after auto-close", symbol)
		}

		// Determine action type
		action := "auto_close_long"
		if side == "short" {
			action = "auto_close_short"
		}

		// Create decision action with accurate data
		decisionAction := logger.DecisionAction{
			Action:    action,
			Symbol:    closedPosition.Symbol,
			Quantity:  quantity,
			Leverage:  leverage,
			Price:     actualExitPrice,
			Timestamp: closeTime,
			Success:   true,
		}

		// Create minimal decision record
		record := &logger.DecisionRecord{
			Timestamp:    closeTime,
			CycleNumber:  0, // Auto-close doesn't belong to a cycle
			Decisions:    []logger.DecisionAction{decisionAction},
			ExecutionLog: []string{"Auto-closed by SL/TP"},
			Success:      true,
		}

		// Log the decision
		if err := at.logDecisionAndSaveEquity(record); err != nil {
			log.Printf("⚠️ Position closure detection: failed to log auto-close for %s: %v", posKey, err)
			continue
		}

		log.Printf("✅ Position closure detected and logged: %s %s (entry=%.4f, exit=%.4f, pnl=%.4f)",
			symbol, side, entryPrice, actualExitPrice, realizedPnL)

		// Mark as logged
		at.loggedClosuresMutex.Lock()
		at.loggedClosures[posKey] = closeTime
		at.loggedClosuresMutex.Unlock()

		// Clean up old logged closures (older than 1 hour)
		at.loggedClosuresMutex.Lock()
		for k, v := range at.loggedClosures {
			if time.Since(v) > time.Hour {
				delete(at.loggedClosures, k)
			}
		}
		at.loggedClosuresMutex.Unlock()
	}

	// Update previous positions snapshot
	at.previousPositionsMutex.Lock()
	at.previousPositions = currentPositionsMap
	at.previousPositionsMutex.Unlock()
}

// Emergency close position function
func (at *AutoTrader) emergencyClosePosition(symbol, side string) error {
	switch side {
	case "long":
		order, err := at.trader.CloseLong(symbol, 0) // 0 = close all
		if err != nil {
			return err
		}
		log.Printf("✅ Emergency close long succeeded, order ID: %v", order["orderId"])
	case "short":
		order, err := at.trader.CloseShort(symbol, 0) // 0 = close all
		if err != nil {
			return err
		}
		log.Printf("✅ Emergency close short succeeded, order ID: %v", order["orderId"])
	default:
		return fmt.Errorf("unknown position direction: %s", side)
	}

	return nil
}

// GetPeakPnLCache get peak profit cache
func (at *AutoTrader) GetPeakPnLCache() map[string]float64 {
	at.peakPnLCacheMutex.RLock()
	defer at.peakPnLCacheMutex.RUnlock()

	// Return copy of cache
	cache := make(map[string]float64)
	for k, v := range at.peakPnLCache {
		cache[k] = v
	}
	return cache
}

// UpdatePeakPnL update peak profit cache
func (at *AutoTrader) UpdatePeakPnL(symbol, side string, currentPnLPct float64) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	if peak, exists := at.peakPnLCache[posKey]; exists {
		// Update peak (if long, take larger value; if short, currentPnLPct is negative, compare anyway)
		if currentPnLPct > peak {
			at.peakPnLCache[posKey] = currentPnLPct
		}
	} else {
		// First record
		at.peakPnLCache[posKey] = currentPnLPct
	}
}

// ClearPeakPnLCache clear peak cache for specified position
func (at *AutoTrader) ClearPeakPnLCache(symbol, side string) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	delete(at.peakPnLCache, posKey)
}

// processTradingViewAlerts processes TradingView alerts (skip AI decision)
func (at *AutoTrader) processTradingViewAlerts(record *logger.DecisionRecord) error {
	// Get database reference
	db, ok := at.database.(*cfg.Database)
	if !ok {
		record.Success = false
		record.ErrorMessage = "unable to access database"
		at.logDecisionAndSaveEquity(record)
		return fmt.Errorf("database type error")
	}

	// Get pending alerts
	alerts, err := db.GetPendingTradingViewAlerts(at.id)
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("failed to get TradingView alerts: %v", err)
		at.logDecisionAndSaveEquity(record)
		return fmt.Errorf("failed to get alerts: %w", err)
	}

	if len(alerts) == 0 {
		log.Println("📭 No pending TradingView alerts")
		record.ExecutionLog = append(record.ExecutionLog, "No pending TradingView alerts")
		at.logDecisionAndSaveEquity(record)
		return nil
	}

	log.Printf("📨 Found %d pending TradingView alerts", len(alerts))

	// Get account information (for risk check)
	balance, err := at.trader.GetBalance()
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("failed to get account information: %v", err)
		at.logDecisionAndSaveEquity(record)
		return fmt.Errorf("failed to get account information: %w", err)
	}

	// Extract account information from balance map
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0

	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	} else if equity, ok := balance["total_equity"].(float64); ok {
		totalWalletBalance = equity
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	} else if unrealized, ok := balance["unrealized_pnl"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	} else if avail, ok := balance["available_balance"].(float64); ok {
		availableBalance = avail
	}

	totalEquity := totalWalletBalance + totalUnrealizedProfit
	marginUsedPct := 0.0
	if marginUsed, ok := balance["margin_used"].(float64); ok && totalEquity > 0 {
		marginUsedPct = (marginUsed / totalEquity) * 100
	}

	// Get position count
	positions, _ := at.trader.GetPositions()
	positionCount := len(positions)

	record.AccountState = logger.AccountSnapshot{
		TotalBalance:          totalEquity - totalUnrealizedProfit,
		AvailableBalance:      availableBalance,
		TotalUnrealizedProfit: totalUnrealizedProfit,
		PositionCount:         positionCount,
		MarginUsedPct:         marginUsedPct,
		InitialBalance:        at.initialBalance,
	}

	// Convert alerts to decision actions
	var decisions []decision.Decision
	for _, alert := range alerts {
		d := at.convertAlertToDecision(alert)
		if d != nil {
			decisions = append(decisions, *d)
			// Mark alert as accepted
			_ = db.UpdateAlertStatus(alert.ID, "accepted")
		}
	}

	if len(decisions) == 0 {
		log.Println("⚠️ No valid trading actions")
		record.ExecutionLog = append(record.ExecutionLog, "No valid trading actions")
		at.logDecisionAndSaveEquity(record)
		return nil
	}

	// Execute trades
	log.Printf("🔄 Executing %d TradingView trading actions", len(decisions))
	for i, d := range decisions {
		actionRecord := logger.DecisionAction{
			Action:    d.Action,
			Symbol:    d.Symbol,
			Quantity:  0,
			Leverage:  d.Leverage,
			Price:     0,
			Timestamp: time.Now(),
			Success:   false,
		}

		if err := at.executeDecisionWithRecord(&d, &actionRecord); err != nil {
			log.Printf("❌ Failed to execute TradingView trade (%s %s): %v", d.Symbol, d.Action, err)
			actionRecord.Error = err.Error()
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ %s %s failed: %v", d.Symbol, d.Action, err))
		} else {
			actionRecord.Success = true
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s succeeded", d.Symbol, d.Action))
			// Mark alert as executed
			if i < len(alerts) {
				_ = db.UpdateAlertStatus(alerts[i].ID, "executed")
			}
			time.Sleep(1 * time.Second)
		}

		record.Decisions = append(record.Decisions, actionRecord)
	}

	// Save decision record
	if err := at.logDecisionAndSaveEquity(record); err != nil {
		log.Printf("⚠ Failed to save decision record: %v", err)
	}

	return nil
}

// convertAlertToDecision converts TradingView alert to decision
func (at *AutoTrader) convertAlertToDecision(alert cfg.TradingViewAlert) *decision.Decision {
	// Map action: "buy" -> "open_long", "sell" -> "open_short"
	var action string
	switch alert.Action {
	case "buy":
		action = "open_long"
	case "sell":
		action = "open_short"
	default:
		log.Printf("⚠️ Unknown action: %s", alert.Action)
		return nil
	}

	// Determine quantity (prefer position_size, if 0 use quantity)
	quantity := alert.PositionSize
	if quantity == 0 {
		quantity = alert.Quantity
	}
	if quantity == 0 {
		log.Printf("⚠️ Alert quantity is 0: %s", alert.Symbol)
		return nil
	}

	// Determine leverage (based on coin)
	leverage := at.config.AltcoinLeverage
	if alert.Symbol == "BTCUSDT" || alert.Symbol == "ETHUSDT" {
		leverage = at.config.BTCETHLeverage
	}

	// Calculate position size (USD)
	positionSizeUSD := quantity * alert.Entry

	return &decision.Decision{
		Action:          action,
		Symbol:          alert.Symbol,
		Leverage:        leverage,
		PositionSizeUSD: positionSizeUSD,
		StopLoss:        alert.SL,
		TakeProfit:      alert.TP,
		Reasoning:       fmt.Sprintf("TradingView alert: %s %s @ %.2f", alert.Symbol, action, alert.Entry),
	}
}

// processTradingViewAlertWithAI processes a single TradingView alert using AI analysis
func (at *AutoTrader) processTradingViewAlertWithAI(alertID string) {
	log.Printf("🤖 [%s] Starting AI analysis of TradingView alert: %s", at.name, alertID)

	// Get database reference
	db, ok := at.database.(*cfg.Database)
	if !ok {
		log.Printf("❌ [%s] Database type error", at.name)
		return
	}

	// Get alert
	alert, err := db.GetTradingViewAlertByID(alertID)
	if err != nil {
		log.Printf("❌ [%s] Failed to get alert: %v", at.name, err)
		return
	}

	// Update status to analyzing
	if err := db.UpdateAlertStatus(alertID, "analyzing"); err != nil {
		log.Printf("⚠️ [%s] Failed to update alert status: %v", at.name, err)
	}

	// Create decision record
	record := &logger.DecisionRecord{
		Timestamp:    time.Now(),
		Success:      false,
		ExecutionLog: []string{},
		Decisions:    []logger.DecisionAction{},
	}

	// Build trading context (for this symbol only)
	tradingCtx, err := at.buildTradingContextForSymbol(alert.Symbol, alert)
	if err != nil {
		log.Printf("❌ [%s] Failed to build trading context: %v", at.name, err)
		record.ErrorMessage = fmt.Sprintf("Failed to build trading context: %v", err)
		_ = db.UpdateAlertStatus(alertID, "error")
		at.logDecisionAndSaveEquity(record)
		return
	}

	// Debug: Log tradingCtx.Account values before populating AccountState
	log.Printf("🔍 [%s] TradingView Signal - tradingCtx.Account values: TotalEquity=%.2f, AvailableBalance=%.2f, UnrealizedPnL=%.2f, PositionCount=%d, MarginUsedPct=%.2f%%",
		at.name, tradingCtx.Account.TotalEquity, tradingCtx.Account.AvailableBalance,
		tradingCtx.Account.UnrealizedPnL, tradingCtx.Account.PositionCount, tradingCtx.Account.MarginUsedPct)

	// Save account state snapshot
	record.AccountState = logger.AccountSnapshot{
		TotalBalance:          tradingCtx.Account.TotalEquity - tradingCtx.Account.UnrealizedPnL,
		AvailableBalance:      tradingCtx.Account.AvailableBalance,
		TotalUnrealizedProfit: tradingCtx.Account.UnrealizedPnL,
		PositionCount:         tradingCtx.Account.PositionCount,
		MarginUsedPct:         tradingCtx.Account.MarginUsedPct,
		InitialBalance:        at.initialBalance,
	}

	// Debug: Log record.AccountState values after assignment
	log.Printf("🔍 [%s] TradingView Signal - record.AccountState populated: TotalBalance=%.2f, AvailableBalance=%.2f, PositionCount=%d, MarginUsedPct=%.2f%%, InitialBalance=%.2f",
		at.name, record.AccountState.TotalBalance, record.AccountState.AvailableBalance,
		record.AccountState.PositionCount, record.AccountState.MarginUsedPct, record.AccountState.InitialBalance)

	// Save position snapshot
	for _, pos := range tradingCtx.Positions {
		record.Positions = append(record.Positions, logger.PositionSnapshot{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			PositionAmt:      pos.Quantity,
			EntryPrice:       pos.EntryPrice,
			MarkPrice:        pos.MarkPrice,
			UnrealizedProfit: pos.UnrealizedPnL,
			Leverage:         float64(pos.Leverage),
			LiquidationPrice: pos.LiquidationPrice,
		})
	}

	// Save candidate coins
	for _, coin := range tradingCtx.CandidateCoins {
		record.CandidateCoins = append(record.CandidateCoins, coin.Symbol)
	}

	// Build user prompt (contains TradingView signal data)
	userPrompt := at.buildTradingViewUserPrompt(tradingCtx, alert)

	// Build system prompt (contains TradingView signal analysis instructions)
	systemPrompt := decision.BuildSystemPromptWithTradingView(
		tradingCtx.Account.TotalEquity,
		tradingCtx.BTCETHLeverage,
		tradingCtx.AltcoinLeverage,
		at.customPrompt,
		at.overrideBasePrompt,
		at.systemPromptTemplate,
		tradingCtx.PromptVariant,
	)

	// Call AI
	aiCallStart := time.Now()
	aiResponse, err := at.mcpClient.CallWithMessages(systemPrompt, userPrompt)
	aiCallDuration := time.Since(aiCallStart)
	if err != nil {
		log.Printf("❌ [%s] AI call failed: %v", at.name, err)
		record.ErrorMessage = fmt.Sprintf("AI call failed: %v", err)
		_ = db.UpdateAlertStatus(alertID, "error")
		at.logDecisionAndSaveEquity(record)
		return
	}

	log.Printf("✅ [%s] AI call succeeded, duration: %v", at.name, aiCallDuration)
	record.AIRequestDurationMs = aiCallDuration.Milliseconds()
	record.RawResponse = aiResponse // Save raw AI response for debugging parse failures

	// Parse AI response
	fullDecision, err := decision.ParseFullDecisionResponse(tradingCtx, aiResponse)
	if err != nil {
		log.Printf("❌ [%s] failed to parse AI response: %v", at.name, err)
		record.ErrorMessage = fmt.Sprintf("failed to parse AI response: %v", err)
		record.SystemPrompt = systemPrompt
		record.InputPrompt = userPrompt
		_ = db.UpdateAlertStatus(alertID, "error")
		at.logDecisionAndSaveEquity(record)
		return
	}

	// Populate record fields from fullDecision
	record.SystemPrompt = systemPrompt
	record.InputPrompt = userPrompt
	record.CoTTrace = fullDecision.CoTTrace
	if len(fullDecision.Decisions) > 0 {
		decisionJSON, _ := json.MarshalIndent(fullDecision.Decisions, "", "  ")
		record.DecisionJSON = string(decisionJSON)
	}

	// Check if there are any decisions
	if len(fullDecision.Decisions) == 0 {
		log.Printf("⚠️ [%s] AI returned no decisions", at.name)
		record.ErrorMessage = "AI returned no decisions"
		_ = db.UpdateAlertStatus(alertID, "rejected")
		// Debug: Log AccountState before saving (no decisions case)
		log.Printf("🔍 [%s] TradingView Signal - Before LogDecision (no decisions): AccountState TotalBalance=%.2f, AvailableBalance=%.2f, PositionCount=%d",
			at.name, record.AccountState.TotalBalance, record.AccountState.AvailableBalance, record.AccountState.PositionCount)
		at.logDecisionAndSaveEquity(record)
		return
	}

	// Get first decision (should be only one)
	aiDecision := fullDecision.Decisions[0]

	// Handle safe fallback "wait" action (when AI doesn't provide valid JSON)
	if aiDecision.Action == "wait" && aiDecision.Reasoning != "" && strings.Contains(aiDecision.Reasoning, "model did not output structured JSON") {
		log.Printf("⚠️ [%s] AI failed to output valid JSON, entering safe wait mode: %s", at.name, aiDecision.Reasoning)
		record.ErrorMessage = aiDecision.Reasoning
		_ = db.UpdateAlertStatus(alertID, "error")
		// Debug: Log AccountState before saving (safe fallback case)
		log.Printf("🔍 [%s] TradingView Signal - Before LogDecision (safe fallback): AccountState TotalBalance=%.2f, AvailableBalance=%.2f, PositionCount=%d",
			at.name, record.AccountState.TotalBalance, record.AccountState.AvailableBalance, record.AccountState.PositionCount)
		at.logDecisionAndSaveEquity(record)
		return
	}

	// Check signal decision
	if aiDecision.SignalDecision == "reject" {
		log.Printf("❌ [%s] AI rejected TradingView signal: %s", at.name, aiDecision.Reasoning)
		record.ErrorMessage = fmt.Sprintf("AI rejected: %s", aiDecision.Reasoning)
		_ = db.UpdateAlertStatus(alertID, "rejected")
		// Debug: Log AccountState before saving (reject case)
		log.Printf("🔍 [%s] TradingView Signal - Before LogDecision (reject): AccountState TotalBalance=%.2f, AvailableBalance=%.2f, PositionCount=%d",
			at.name, record.AccountState.TotalBalance, record.AccountState.AvailableBalance, record.AccountState.PositionCount)
		at.logDecisionAndSaveEquity(record)
		return
	}

	// AI accepts or modifies signal
	if aiDecision.SignalDecision == "accept" || aiDecision.SignalDecision == "modify" {
		acceptModify := map[string]string{"accept": "accepted", "modify": "modified"}
		log.Printf("✅ [%s] AI %s TradingView signal", at.name, acceptModify[aiDecision.SignalDecision])

		// Execute decision
		actionRecord := logger.DecisionAction{
			Action:    aiDecision.Action,
			Symbol:    aiDecision.Symbol,
			Quantity:  0,
			Leverage:  aiDecision.Leverage,
			Price:     0,
			Timestamp: time.Now(),
			Success:   false,
		}

		if err := at.executeDecisionWithRecord(&aiDecision, &actionRecord); err != nil {
			log.Printf("❌ [%s] Failed to execute decision: %v", at.name, err)
			actionRecord.Error = err.Error()
			record.ErrorMessage = fmt.Sprintf("Execution failed: %v", err)
			_ = db.UpdateAlertStatus(alertID, "error")
		} else {
			actionRecord.Success = true
			_ = db.UpdateAlertStatus(alertID, "executed")
			log.Printf("✅ [%s] TradingView signal executed", at.name)
		}

		record.Decisions = append(record.Decisions, actionRecord)
		record.Success = actionRecord.Success
	}

	// Save decision record
	// Debug: Log AccountState before saving (final save)
	log.Printf("🔍 [%s] TradingView Signal - Before LogDecision (final): AccountState TotalBalance=%.2f, AvailableBalance=%.2f, PositionCount=%d, MarginUsedPct=%.2f%%",
		at.name, record.AccountState.TotalBalance, record.AccountState.AvailableBalance,
		record.AccountState.PositionCount, record.AccountState.MarginUsedPct)
	if err := at.logDecisionAndSaveEquity(record); err != nil {
		log.Printf("⚠️ [%s] Failed to save decision record: %v", at.name, err)
	}
}

// processParentTradeSignalWithAI processes a parent trade signal using AI analysis
func (at *AutoTrader) processParentTradeSignalWithAI(signal *ParentTradeSignal) {
	log.Printf("🤖 [%s] Analyzing parent signal: %s %s (signal_id=%s)",
		at.name, signal.Decision.Action, signal.Decision.Symbol, signal.SignalID)

	// Mark signal as processed
	at.processedSignalsMutex.Lock()
	at.processedSignals[signal.SignalID] = time.Now()
	// Clean old entries (keep last 100)
	if len(at.processedSignals) > 100 {
		oldestTime := time.Now().Add(-1 * time.Hour)
		for id, t := range at.processedSignals {
			if t.Before(oldestTime) {
				delete(at.processedSignals, id)
			}
		}
	}
	at.processedSignalsMutex.Unlock()

	// Create decision record
	record := &logger.DecisionRecord{
		Timestamp:    time.Now(),
		Success:      false,
		ExecutionLog: []string{},
		Decisions:    []logger.DecisionAction{},
	}

	// Build trading context
	tradingCtx, err := at.buildTradingContextForSymbol(signal.Decision.Symbol, nil)
	if err != nil {
		log.Printf("❌ [%s] Failed to build context: %v", at.name, err)
		record.ErrorMessage = fmt.Sprintf("Failed to build context: %v", err)
		at.logDecisionAndSaveEquity(record)
		return
	}

	// Ensure market data for signal symbol and BTCUSDT
	if _, exists := tradingCtx.MarketDataMap[signal.Decision.Symbol]; !exists {
		marketData, err := market.Get(signal.Decision.Symbol)
		if err != nil {
			log.Printf("⚠️ [%s] Failed to get market data for %s: %v",
				at.name, signal.Decision.Symbol, err)
		} else {
			tradingCtx.MarketDataMap[signal.Decision.Symbol] = marketData
		}
	}

	// Ensure BTCUSDT market data is available
	if _, exists := tradingCtx.MarketDataMap["BTCUSDT"]; !exists {
		btcData, err := market.Get("BTCUSDT")
		if err == nil {
			tradingCtx.MarketDataMap["BTCUSDT"] = btcData
		}
	}

	// Debug: Log tradingCtx.Account values before populating AccountState
	log.Printf("🔍 [%s] Parent Signal - tradingCtx.Account values: TotalEquity=%.2f, AvailableBalance=%.2f, UnrealizedPnL=%.2f, PositionCount=%d, MarginUsedPct=%.2f%%",
		at.name, tradingCtx.Account.TotalEquity, tradingCtx.Account.AvailableBalance,
		tradingCtx.Account.UnrealizedPnL, tradingCtx.Account.PositionCount, tradingCtx.Account.MarginUsedPct)

	// Save account state snapshot
	record.AccountState = logger.AccountSnapshot{
		TotalBalance:          tradingCtx.Account.TotalEquity - tradingCtx.Account.UnrealizedPnL,
		AvailableBalance:      tradingCtx.Account.AvailableBalance,
		TotalUnrealizedProfit: tradingCtx.Account.UnrealizedPnL,
		PositionCount:         tradingCtx.Account.PositionCount,
		MarginUsedPct:         tradingCtx.Account.MarginUsedPct,
		InitialBalance:        at.initialBalance,
	}

	// Debug: Log record.AccountState values after assignment
	log.Printf("🔍 [%s] Parent Signal - record.AccountState populated: TotalBalance=%.2f, AvailableBalance=%.2f, PositionCount=%d, MarginUsedPct=%.2f%%, InitialBalance=%.2f",
		at.name, record.AccountState.TotalBalance, record.AccountState.AvailableBalance,
		record.AccountState.PositionCount, record.AccountState.MarginUsedPct, record.AccountState.InitialBalance)

	// Save position snapshot
	for _, pos := range tradingCtx.Positions {
		record.Positions = append(record.Positions, logger.PositionSnapshot{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			PositionAmt:      pos.Quantity,
			EntryPrice:       pos.EntryPrice,
			MarkPrice:        pos.MarkPrice,
			UnrealizedProfit: pos.UnrealizedPnL,
			Leverage:         float64(pos.Leverage),
			LiquidationPrice: pos.LiquidationPrice,
		})
	}

	// Save candidate coins
	for _, coin := range tradingCtx.CandidateCoins {
		record.CandidateCoins = append(record.CandidateCoins, coin.Symbol)
	}

	// Build user prompt with parent signal info
	userPrompt := at.buildParentSignalUserPrompt(tradingCtx, signal)

	// Build system prompt using risk_management template
	systemPrompt := decision.BuildSystemPromptWithParentSignal(
		tradingCtx.Account.TotalEquity,
		tradingCtx.BTCETHLeverage,
		tradingCtx.AltcoinLeverage,
		at.customPrompt,
		at.overrideBasePrompt,
		"risk_management",
		tradingCtx.PromptVariant,
	)

	// Call AI
	aiCallStart := time.Now()
	aiResponse, err := at.mcpClient.CallWithMessages(systemPrompt, userPrompt)
	aiCallDuration := time.Since(aiCallStart)

	if err != nil {
		log.Printf("❌ [%s] AI call failed: %v", at.name, err)
		record.ErrorMessage = fmt.Sprintf("AI call failed: %v", err)
		at.logDecisionAndSaveEquity(record)
		return
	}

	record.AIRequestDurationMs = aiCallDuration.Milliseconds()
	record.RawResponse = aiResponse // Save raw AI response for debugging parse failures

	// Parse AI response
	fullDecision, err := decision.ParseFullDecisionResponse(tradingCtx, aiResponse)
	if err != nil {
		log.Printf("❌ [%s] Failed to parse AI response: %v", at.name, err)
		record.ErrorMessage = fmt.Sprintf("Failed to parse: %v", err)
		record.SystemPrompt = systemPrompt
		record.InputPrompt = userPrompt
		at.logDecisionAndSaveEquity(record)
		return
	}

	// Populate record fields from fullDecision
	record.SystemPrompt = systemPrompt
	record.InputPrompt = userPrompt
	record.CoTTrace = fullDecision.CoTTrace
	if len(fullDecision.Decisions) > 0 {
		decisionJSON, _ := json.MarshalIndent(fullDecision.Decisions, "", "  ")
		record.DecisionJSON = string(decisionJSON)
	}

	// Process decision
	if len(fullDecision.Decisions) == 0 {
		log.Printf("⚠️ [%s] AI returned no decisions", at.name)
		return
	}

	aiDecision := fullDecision.Decisions[0]
	aiDecision.ParentSignalID = signal.SignalID

	// Save decision details to record
	record.InputPrompt = userPrompt
	record.SystemPrompt = systemPrompt
	record.CoTTrace = fullDecision.CoTTrace
	if len(fullDecision.Decisions) > 0 {
		decisionJSON, _ := json.MarshalIndent(fullDecision.Decisions, "", "  ")
		record.DecisionJSON = string(decisionJSON)
	}

	// Handle signal decision
	if aiDecision.SignalDecision == "reject" {
		log.Printf("❌ [%s] AI rejected signal: %s", at.name, aiDecision.Reasoning)
		record.ErrorMessage = fmt.Sprintf("AI rejected: %s", aiDecision.Reasoning)
		record.Success = true
		at.logDecisionAndSaveEquity(record)
		return
	}

	// Execute decision (accept or modify)
	log.Printf("✅ [%s] AI decision: %s signal (%s)",
		at.name, aiDecision.SignalDecision, aiDecision.Action)

	actionRecord := &logger.DecisionAction{
		Action:    aiDecision.Action,
		Symbol:    aiDecision.Symbol,
		Timestamp: time.Now(),
	}

	if err := at.executeDecisionWithRecord(&aiDecision, actionRecord); err != nil {
		log.Printf("❌ [%s] Execution failed: %v", at.name, err)
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("Execution failed: %v", err)
		actionRecord.Error = err.Error()
	} else {
		log.Printf("✅ [%s] Successfully executed parent signal", at.name)
		record.Success = true
		actionRecord.Success = true
	}

	record.Decisions = append(record.Decisions, *actionRecord)
	at.decisionLogger.LogDecision(record)
}

// buildTradingContextForSymbol builds trading context for specific symbol
func (at *AutoTrader) buildTradingContextForSymbol(symbol string, _ *cfg.TradingViewAlert) (*decision.Context, error) {
	// 1. Get account information
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("failed to get account balance: %w", err)
	}

	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0

	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	totalEquity := totalWalletBalance + totalUnrealizedProfit

	// 2. Get position information
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var positionInfos []decision.PositionInfo
	totalMarginUsed := 0.0
	for _, pos := range positions {
		// Safe type assertions to prevent nil interface conversion panic
		symbolPos, ok := pos["symbol"].(string)
		if !ok || symbolPos == "" {
			continue // Skip invalid position
		}
		side, ok := pos["side"].(string)
		if !ok || side == "" {
			continue // Skip invalid position
		}
		entryPrice, ok := pos["entryPrice"].(float64)
		if !ok {
			continue // Skip invalid position
		}
		markPrice, ok := pos["markPrice"].(float64)
		if !ok {
			continue // Skip invalid position
		}
		quantity, ok := pos["positionAmt"].(float64)
		if !ok {
			continue // Skip invalid position
		}
		if quantity < 0 {
			quantity = -quantity
		}
		if quantity == 0 {
			continue
		}

		unrealizedPnl, ok := pos["unRealizedProfit"].(float64)
		if !ok {
			unrealizedPnl = 0 // Default to 0 if missing
		}
		liquidationPrice, ok := pos["liquidationPrice"].(float64)
		if !ok {
			liquidationPrice = 0 // Default to 0 if missing
		}

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		posKey := symbolPos + "_" + side

		// Handle zero/invalid entry_time from exchange position sync
		var updateTime int64
		if entryTimeRaw, ok := pos["entry_time"].(float64); ok && entryTimeRaw > 0 {
			// Use entry_time from exchange (convert to milliseconds if needed)
			updateTime = int64(entryTimeRaw)
			at.positionFirstSeenTime[posKey] = updateTime
		} else if entryTimeRaw, ok := pos["entry_time"].(int64); ok && entryTimeRaw > 0 {
			// Handle int64 format
			updateTime = entryTimeRaw
			at.positionFirstSeenTime[posKey] = updateTime
		} else if existingTime, exists := at.positionFirstSeenTime[posKey]; exists && existingTime > 0 {
			// Use existing time if position was seen before
			updateTime = existingTime
		} else {
			// Fallback: use current time if no entry_time and not previously seen
			updateTime = time.Now().UnixMilli()
			at.positionFirstSeenTime[posKey] = updateTime
		}

		at.peakPnLCacheMutex.RLock()
		peakPnlPct := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		positionInfos = append(positionInfos, decision.PositionInfo{
			Symbol:           symbolPos,
			Side:             side,
			EntryPrice:       entryPrice,
			MarkPrice:        markPrice,
			Quantity:         quantity,
			Leverage:         leverage,
			UnrealizedPnL:    unrealizedPnl,
			UnrealizedPnLPct: pnlPct,
			PeakPnLPct:       peakPnlPct,
			LiquidationPrice: liquidationPrice,
			MarginUsed:       marginUsed,
			UpdateTime:       updateTime,
		})
	}

	// 3. Get market data for this symbol
	marketData, err := market.Get(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market data: %w", err)
	}

	marketDataMap := make(map[string]*market.Data)
	marketDataMap[symbol] = marketData

	// 4. Calculate margin used percentage
	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	// 5. Build context
	ctx := &decision.Context{
		CurrentTime:    time.Now().Format("2006-01-02 15:04:05"),
		CallCount:      at.callCount,
		RuntimeMinutes: int(time.Since(at.startTime).Minutes()),
		Account: decision.AccountInfo{
			TotalEquity:      totalEquity,
			AvailableBalance: availableBalance,
			TotalPnLPct:      ((totalEquity - at.initialBalance) / at.initialBalance) * 100,
			MarginUsedPct:    marginUsedPct,
			PositionCount:    len(positions),
		},
		Positions:       positionInfos,
		MarketDataMap:   marketDataMap,
		CandidateCoins:  []decision.CandidateCoin{{Symbol: symbol, Sources: []string{"tradingview"}}},
		BTCETHLeverage:  at.config.BTCETHLeverage,
		AltcoinLeverage: at.config.AltcoinLeverage,
		PromptVariant:   "",
	}

	return ctx, nil
}

// buildTradingViewUserPrompt builds TradingView-specific user prompt
func (at *AutoTrader) buildTradingViewUserPrompt(ctx *decision.Context, alert *cfg.TradingViewAlert) string {
	var sb strings.Builder

	// System status
	sb.WriteString(fmt.Sprintf("Time: %s | Cycle: #%d | Runtime: %d minutes\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// TradingView signal details
	sb.WriteString("## TradingView Signal Received\n\n")
	sb.WriteString(fmt.Sprintf("Alert ID: %s\n", alert.ID))
	sb.WriteString(fmt.Sprintf("Symbol: %s\n", alert.Symbol))
	sb.WriteString(fmt.Sprintf("Action: %s (%s)\n", alert.Action, map[string]string{"buy": "Open Long", "sell": "Open Short"}[alert.Action]))
	sb.WriteString(fmt.Sprintf("Entry Price: %.4f\n", alert.Entry))
	sb.WriteString(fmt.Sprintf("Stop Loss: %.4f\n", alert.SL))
	sb.WriteString(fmt.Sprintf("Take Profit: %.4f\n", alert.TP))
	sb.WriteString(fmt.Sprintf("Quantity: %.4f\n", alert.Quantity))
	if alert.PositionSize > 0 {
		sb.WriteString(fmt.Sprintf("Position Size: %.2f USDT\n", alert.PositionSize))
	}
	sb.WriteString("\n")

	// BTC market (if different)
	if alert.Symbol != "BTCUSDT" {
		if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
			sb.WriteString(fmt.Sprintf("BTC: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD: %.4f | RSI: %.2f\n\n",
				btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
				btcData.CurrentMACD, btcData.CurrentRSI7))
		}
	}

	// Account status
	sb.WriteString(fmt.Sprintf("Account: Equity %.2f | Balance %.2f (%.1f%%) | P&L %+.2f%% | Margin %.1f%% | Positions %d\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// Current positions
	if len(ctx.Positions) > 0 {
		sb.WriteString("## Current Positions\n")
		for i, pos := range ctx.Positions {
			sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Current %.4f | Quantity %.4f | P&L %+.2f%% | Leverage %dx\n\n",
				i+1, pos.Symbol, strings.ToUpper(pos.Side),
				pos.EntryPrice, pos.MarkPrice, pos.Quantity, pos.UnrealizedPnLPct, pos.Leverage))
		}
	} else {
		sb.WriteString("Current Positions: None\n\n")
	}

	// Target symbol's complete market data
	if marketData, ok := ctx.MarketDataMap[alert.Symbol]; ok {
		sb.WriteString(fmt.Sprintf("## %s Market Data\n\n", alert.Symbol))
		// Use trader-specific indicator configuration
		indicatorConfig := &market.IndicatorConfig{
			EnableRawKlines:    at.config.EnableRawKlines,
			EnableEMA:          at.config.EnableEMA,
			EnableMACD:         at.config.EnableMACD,
			EnableRSI:          at.config.EnableRSI,
			EnableATR:          at.config.EnableATR,
			EnableVolume:       at.config.EnableVolume,
			EnableOI:           at.config.EnableOI,
			EnableFunding:      at.config.EnableFunding,
			IndicatorTimeframe: at.config.IndicatorTimeframe,
		}
		sb.WriteString(market.FormatWithIndicators(marketData, indicatorConfig))
		sb.WriteString("\n\n")
	}

	// Decision request
	sb.WriteString("---\n\n")
	sb.WriteString("Please analyze this TradingView signal and decide: accept, reject, or modify.\n")
	sb.WriteString("If accepting, use the parameters provided by the signal; if modifying, use parameters you deem more appropriate.\n")
	sb.WriteString("The JSON output must include a signal_decision field (value: \"accept\", \"reject\", or \"modify\").\n")
	sb.WriteString("Now please analyze and output your decision (chain of thought + JSON)\n")

	return sb.String()
}

// buildParentSignalUserPrompt builds the user prompt for parent trade signal analysis
func (at *AutoTrader) buildParentSignalUserPrompt(ctx *decision.Context, signal *ParentTradeSignal) string {
	var sb strings.Builder

	sb.WriteString("# Parent Trade Signal Analysis\n\n")
	sb.WriteString(fmt.Sprintf("Parent Trader: %s (ID: %s)\n",
		signal.ParentTraderName, signal.ParentTraderID))
	sb.WriteString(fmt.Sprintf("Signal ID: %s\n", signal.SignalID))
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n", signal.Timestamp.Format(time.RFC3339)))

	// Indicate if this is a TradingView-originated signal (instant forwarding)
	if signal.Decision.TradingViewSignalID != "" {
		sb.WriteString(fmt.Sprintf("⚡ Signal Source: TradingView Alert (Alert ID: %s) - Instant Forwarding\n", signal.Decision.TradingViewSignalID))
		sb.WriteString("This signal was forwarded instantly from a TradingView alert. The parent trader may still be processing the same alert.\n")
	} else {
		sb.WriteString("Signal Source: Parent Trader AI Decision (Post-Execution Replication)\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Parent's Trade Decision\n\n")
	sb.WriteString(fmt.Sprintf("- Symbol: %s\n", signal.Decision.Symbol))
	sb.WriteString(fmt.Sprintf("- Action: %s\n", signal.Decision.Action))
	sb.WriteString(fmt.Sprintf("- Leverage: %dx\n", signal.Decision.Leverage))
	sb.WriteString(fmt.Sprintf("- Position Size: %.2f USDT\n", signal.Decision.PositionSizeUSD))
	sb.WriteString(fmt.Sprintf("- Stop Loss: %.4f\n", signal.Decision.StopLoss))
	sb.WriteString(fmt.Sprintf("- Take Profit: %.4f\n", signal.Decision.TakeProfit))
	sb.WriteString(fmt.Sprintf("- Reasoning: %s\n\n", signal.Decision.Reasoning))

	if signal.ParentEquity > 0 {
		sb.WriteString(fmt.Sprintf("- Parent Account Equity: %.2f USDT\n", signal.ParentEquity))
		if signal.ParentInitialBalance > 0 {
			parentPnlPct := ((signal.ParentEquity - signal.ParentInitialBalance) / signal.ParentInitialBalance) * 100
			sb.WriteString(fmt.Sprintf("- Parent Total P&L: %.2f%%\n", parentPnlPct))
		}
	}

	sb.WriteString("\n## Your Account Status\n\n")
	sb.WriteString(fmt.Sprintf("- Total Equity: %.2f USDT\n", ctx.Account.TotalEquity))
	sb.WriteString(fmt.Sprintf("- Available Balance: %.2f USDT\n", ctx.Account.AvailableBalance))
	sb.WriteString(fmt.Sprintf("- Position Count: %d\n", ctx.Account.PositionCount))
	sb.WriteString(fmt.Sprintf("- Margin Used: %.2f%%\n\n", ctx.Account.MarginUsedPct))

	// Add current positions
	if len(ctx.Positions) > 0 {
		sb.WriteString("## Current Positions\n\n")
		for i, pos := range ctx.Positions {
			sb.WriteString(fmt.Sprintf("%d. %s %s | Entry: %.4f | Mark: %.4f | Quantity: %.4f | P&L: %+.2f%% | Leverage: %dx\n",
				i+1, pos.Symbol, strings.ToUpper(pos.Side),
				pos.EntryPrice, pos.MarkPrice, pos.Quantity, pos.UnrealizedPnLPct, pos.Leverage))
		}
		sb.WriteString("\n")
	}

	// Add market data
	if marketData, exists := ctx.MarketDataMap[signal.Decision.Symbol]; exists {
		sb.WriteString("## Current Market Data\n\n")
		sb.WriteString(fmt.Sprintf("- Symbol: %s\n", signal.Decision.Symbol))
		sb.WriteString(fmt.Sprintf("- Current Price: %.4f\n", marketData.CurrentPrice))
		// Use PriceChange1h or PriceChange4h if available, otherwise calculate from price
		if marketData.PriceChange1h != 0 {
			sb.WriteString(fmt.Sprintf("- 1h Change: %.2f%%\n", marketData.PriceChange1h))
		}
		if marketData.PriceChange4h != 0 {
			sb.WriteString(fmt.Sprintf("- 4h Change: %.2f%%\n", marketData.PriceChange4h))
		}
		// Add indicators based on trader configuration
		if at.config.EnableRSI && marketData.CurrentRSI7 > 0 {
			sb.WriteString(fmt.Sprintf("- RSI(7): %.2f\n", marketData.CurrentRSI7))
		}
		if at.config.EnableMACD && marketData.CurrentMACD != 0 {
			sb.WriteString(fmt.Sprintf("- MACD: %.4f\n", marketData.CurrentMACD))
		}
		if at.config.EnableEMA && marketData.CurrentEMA20 > 0 {
			sb.WriteString(fmt.Sprintf("- EMA(20): %.4f\n", marketData.CurrentEMA20))
		}
		sb.WriteString("\n")
	}

	// Add BTC market data if different symbol
	if signal.Decision.Symbol != "BTCUSDT" {
		if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
			sb.WriteString("## BTC Market Context\n\n")
			sb.WriteString(fmt.Sprintf("- BTC Price: %.2f\n", btcData.CurrentPrice))
			sb.WriteString(fmt.Sprintf("- 1h Change: %+.2f%%\n", btcData.PriceChange1h))
			sb.WriteString(fmt.Sprintf("- 4h Change: %+.2f%%\n", btcData.PriceChange4h))
			if btcData.CurrentRSI7 > 0 {
				sb.WriteString(fmt.Sprintf("- RSI(7): %.2f\n", btcData.CurrentRSI7))
			}
			if btcData.CurrentMACD != 0 {
				sb.WriteString(fmt.Sprintf("- MACD: %.4f\n", btcData.CurrentMACD))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
