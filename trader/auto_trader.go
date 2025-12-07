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
	"strings"
	"sync"
	"time"
)

// AutoTraderConfig 自动交易配置（简化版 - AI全权决策）
type AutoTraderConfig struct {
	// Trader标识
	ID      string // Trader唯一标识（用于日志目录等）
	Name    string // Trader显示名称
	AIModel string // AI模型: "qwen" 或 "deepseek"

	// 交易平台选择
	Exchange string // "binance", "bybit", "hyperliquid", "aster" 或 "lighter"

	// 币安API配置
	BinanceAPIKey    string
	BinanceSecretKey string

	// Bybit API配置
	BybitAPIKey    string
	BybitSecretKey string

	// OKX API配置
	OkxAPIKey     string
	OkxSecretKey string
	OkxPassphrase string // OKX requires passphrase

	// Hyperliquid配置
	HyperliquidPrivateKey string
	HyperliquidWalletAddr string
	HyperliquidTestnet    bool

	// Aster配置
	AsterUser       string // Aster主钱包地址
	AsterSigner     string // Aster API钱包地址
	AsterPrivateKey string // Aster API钱包私钥

	// LIGHTER配置
	LighterWalletAddr       string // LIGHTER钱包地址（L1 wallet）
	LighterPrivateKey       string // LIGHTER L1私钥（用于识别账户）
	LighterAPIKeyPrivateKey string // LIGHTER API Key私钥（40字节，用于签名交易）
	LighterTestnet          bool   // 是否使用testnet

	CoinPoolAPIURL string

	// AI配置
	UseQwen     bool
	DeepSeekKey string
	QwenKey     string

	// 自定义AI API配置
	CustomAPIURL    string
	CustomAPIKey    string
	CustomModelName string

	// 扫描配置
	ScanInterval time.Duration // 扫描间隔（建议3分钟）

	// 账户配置
	InitialBalance float64 // 初始金额（用于计算盈亏，需手动设置）

	// 杠杆配置
	BTCETHLeverage  int // BTC和ETH的杠杆倍数
	AltcoinLeverage int // 山寨币的杠杆倍数

	// 风险控制（仅作为提示，AI可自主决定）
	MaxDailyLoss    float64       // 最大日亏损百分比（提示）
	MaxDrawdown     float64       // 最大回撤百分比（提示）
	StopTradingTime time.Duration // 触发风控后暂停时长

	// 仓位模式
	IsCrossMargin bool // true=全仓模式, false=逐仓模式

	// 币种配置
	DefaultCoins []string // 默认币种列表（从数据库获取）
	TradingCoins []string // 实际交易币种列表

	// 系统提示词模板
	SystemPromptTemplate string // 系统提示词模板名称（如 "default", "aggressive"）

	// TradingView配置
	UseTradingView bool // 是否使用TradingView信号源（如果为true，将跳过AI决策）
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

// AutoTrader 自动交易器
type AutoTrader struct {
	id                    string // Trader唯一标识
	name                  string // Trader显示名称
	aiModel               string // AI模型名称
	exchange              string // 交易平台名称
	config                AutoTraderConfig
	trader                Trader // 使用Trader接口（支持多平台）
	mcpClient             mcp.AIClient
	decisionLogger        logger.IDecisionLogger // 决策日志记录器
	initialBalance        float64
	dailyPnL              float64
	customPrompt          string   // 自定义交易策略prompt
	overrideBasePrompt    bool     // 是否覆盖基础prompt
	systemPromptTemplate  string   // 系统提示词模板名称
	defaultCoins          []string // 默认币种列表（从数据库获取）
	tradingCoins          []string // 实际交易币种列表
	lastResetTime         time.Time
	stopUntil             time.Time
	isRunning             bool
	startTime             time.Time          // 系统启动时间
	callCount             int                // AI调用次数
	positionFirstSeenTime map[string]int64   // 持仓首次出现时间 (symbol_side -> timestamp毫秒)
	stopMonitorCh         chan struct{}      // 用于停止监控goroutine
	monitorWg             sync.WaitGroup     // 用于等待监控goroutine结束
	peakPnLCache          map[string]float64 // 最高收益缓存 (symbol -> 峰值盈亏百分比)
	peakPnLCacheMutex     sync.RWMutex       // 缓存读写锁
	lastBalanceSyncTime   time.Time          // 上次余额同步时间
	database              interface{}        // 数据库引用（用于自动更新余额）
	userID                string             // 用户ID
	triggerDecisionCh     chan string        // Channel for triggering immediate decision cycles (TradingView webhooks)
	tradeReplicationCallback func(traderID string, decision *decision.Decision) // Callback to replicate trades to followers
	parentTradeSignalCh   chan *ParentTradeSignal // Channel for parent trade signals
	isFollower            bool               // Cached follower status
	followedTraderID      string             // Cached parent trader ID
	processedSignals      map[string]time.Time // Deduplication cache (signal_id -> timestamp)
	processedSignalsMutex sync.RWMutex       // Mutex for signal cache
}

// NewAutoTrader 创建自动交易器
func NewAutoTrader(config AutoTraderConfig, database interface{}, userID string) (*AutoTrader, error) {
	// 设置默认值
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

	// 初始化AI
	if config.AIModel == "custom" {
		// 使用自定义API
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		log.Printf("🤖 [%s] 使用自定义AI API: %s (模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
	} else if config.UseQwen || config.AIModel == "qwen" {
		// 使用Qwen (支持自定义URL和Model)
		mcpClient = mcp.NewQwenClient()
		mcpClient.SetAPIKey(config.QwenKey, config.CustomAPIURL, config.CustomModelName)
		if config.CustomAPIURL != "" || config.CustomModelName != "" {
			log.Printf("🤖 [%s] 使用阿里云Qwen AI (自定义URL: %s, 模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
		} else {
			log.Printf("🤖 [%s] 使用阿里云Qwen AI", config.Name)
		}
	} else {
		// 默认使用DeepSeek (支持自定义URL和Model)
		mcpClient = mcp.NewDeepSeekClient()
		mcpClient.SetAPIKey(config.DeepSeekKey, config.CustomAPIURL, config.CustomModelName)
		if config.CustomAPIURL != "" || config.CustomModelName != "" {
			log.Printf("🤖 [%s] 使用DeepSeek AI (自定义URL: %s, 模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
		} else {
			log.Printf("🤖 [%s] 使用DeepSeek AI", config.Name)
		}
	}

	// 初始化币种池API
	if config.CoinPoolAPIURL != "" {
		pool.SetCoinPoolAPI(config.CoinPoolAPIURL)
	}

	// 设置默认交易平台
	if config.Exchange == "" {
		config.Exchange = "binance"
	}

	// 根据配置创建对应的交易器
	var trader Trader
	var err error

	// 记录仓位模式（通用）
	marginModeStr := "全仓"
	if !config.IsCrossMargin {
		marginModeStr = "逐仓"
	}
	log.Printf("📊 [%s] 仓位模式: %s", config.Name, marginModeStr)

	switch config.Exchange {
	case "binance":
		log.Printf("🏦 [%s] 使用币安合约交易", config.Name)
		trader = NewFuturesTrader(config.BinanceAPIKey, config.BinanceSecretKey, userID)
	case "bybit":
		log.Printf("🏦 [%s] 使用Bybit合约交易", config.Name)
		trader = NewBybitTrader(config.BybitAPIKey, config.BybitSecretKey)
	case "okx":
		log.Printf("🏦 [%s] 使用OKX合约交易", config.Name)
		trader = NewOKXTrader(config.OkxAPIKey, config.OkxSecretKey, config.OkxPassphrase)
	case "hyperliquid":
		log.Printf("🏦 [%s] 使用Hyperliquid交易", config.Name)
		trader, err = NewHyperliquidTrader(config.HyperliquidPrivateKey, config.HyperliquidWalletAddr, config.HyperliquidTestnet)
		if err != nil {
			return nil, fmt.Errorf("初始化Hyperliquid交易器失败: %w", err)
		}
	case "aster":
		log.Printf("🏦 [%s] 使用Aster交易", config.Name)
		trader, err = NewAsterTrader(config.AsterUser, config.AsterSigner, config.AsterPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("初始化Aster交易器失败: %w", err)
		}
	case "lighter":
		log.Printf("🏦 [%s] 使用LIGHTER交易", config.Name)

		// 優先使用 V2（需要 API Key）
		if config.LighterAPIKeyPrivateKey != "" {
			log.Printf("✓ 使用 LIGHTER SDK (V2) - 完整簽名支持")
			trader, err = NewLighterTraderV2(
				config.LighterPrivateKey,
				config.LighterWalletAddr,
				config.LighterAPIKeyPrivateKey,
				config.LighterTestnet,
			)
			if err != nil {
				return nil, fmt.Errorf("初始化LIGHTER交易器(V2)失败: %w", err)
			}
		} else {
			// 降級使用 V1（基本HTTP實現）
			log.Printf("⚠️  使用 LIGHTER 基本實現 (V1) - 功能受限，請配置 API Key")
			trader, err = NewLighterTrader(config.LighterPrivateKey, config.LighterWalletAddr, config.LighterTestnet)
			if err != nil {
				return nil, fmt.Errorf("初始化LIGHTER交易器(V1)失败: %w", err)
			}
		}
	default:
		return nil, fmt.Errorf("不支持的交易平台: %s", config.Exchange)
	}

	// 初始化决策日志记录器（使用trader ID创建独立目录）
	logDir := fmt.Sprintf("decision_logs/%s", config.ID)
	decisionLogger := logger.NewDecisionLogger(logDir)

	// 设置默认系统提示词模板
	systemPromptTemplate := config.SystemPromptTemplate
	if systemPromptTemplate == "" {
		// feature/partial-close-dynamic-tpsl 分支默认使用 adaptive（支持动态止盈止损）
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
		lastBalanceSyncTime:   time.Now(), // 初始化为当前时间
		database:              database,
		userID:                userID,
		triggerDecisionCh:     make(chan string, 100), // Buffered channel for webhook triggers (buffer size: 100)
		parentTradeSignalCh:   make(chan *ParentTradeSignal, 100), // Buffered channel for parent signals
		isFollower:            isFollower,
		followedTraderID:      followedTraderID,
		processedSignals:      make(map[string]time.Time),
	}, nil
}

// Run 运行自动交易主循环
func (at *AutoTrader) Run() error {
	// 验证初始金额配置（在启动时验证，允许加载时balance=0）
	if at.initialBalance <= 0 {
		return fmt.Errorf("无法启动交易员: 初始金额未设置，请使用同步余额功能设置初始金额")
	}

	at.isRunning = true
	at.stopMonitorCh = make(chan struct{})
	at.startTime = time.Now()

	log.Println("🚀 AI驱动自动交易系统启动")
	log.Printf("💰 初始余额: %.2f USDT", at.initialBalance)
	log.Printf("⚙️  扫描间隔: %v", at.config.ScanInterval)
	log.Println("🤖 AI将全权决定杠杆、仓位大小、止损止盈等参数")
	log.Printf("INFO: Trader starting (id=%s, name=%s, UseTradingView=%v, ScanInterval=%v)", at.id, at.name, at.config.UseTradingView, at.config.ScanInterval)
	log.Printf("ℹ️ Trader mode: UseTradingView=%v (id=%s, name=%s)", at.config.UseTradingView, at.id, at.name)
	at.monitorWg.Add(1)
	defer at.monitorWg.Done()

	// 启动回撤监控
	at.startDrawdownMonitor()

	// Check if this is a follower trader
	if at.isFollower {
		log.Printf("👥 [%s] Follower mode: waiting for parent signals (parent=%s)", 
			at.name, at.followedTraderID)
		
		const maxConcurrentSignals = 5
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

	// 如果启用TradingView，跳过周期性扫描，等待webhook触发
	if at.config.UseTradingView {
		log.Println("📡 TradingView模式：等待webhook触发，不进行周期性扫描")
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
				log.Printf("[%s] ⏹ 收到停止信号，退出自动交易主循环", at.name)
				return nil
			}
		}
	} else {
		// 原有的周期性扫描逻辑
		log.Printf("INFO: Entering periodic scan mode (trader=%s, id=%s, UseTradingView=%v, ScanInterval=%v)", at.name, at.id, at.config.UseTradingView, at.config.ScanInterval)
		log.Printf("WARN: Trader is NOT in TradingView mode - periodic scans will run (trader=%s, id=%s)", at.name, at.id)
		log.Printf("🕒 周期扫描模式已启用: 每 %v 运行一次AI决策", at.config.ScanInterval)
		ticker := time.NewTicker(at.config.ScanInterval)
		defer ticker.Stop()

		// 首次立即执行
		if err := at.runCycle(); err != nil {
			log.Printf("❌ 执行失败: %v", err)
		}

		for at.isRunning {
			select {
			case <-ticker.C:
				if err := at.runCycle(); err != nil {
					log.Printf("❌ 执行失败: %v", err)
				}
			case <-at.stopMonitorCh:
				log.Printf("[%s] ⏹ 收到停止信号，退出自动交易主循环", at.name)
				return nil
			}
		}
	}

	return nil
}

// TriggerTradingViewDecisionCycle 触发TradingView决策周期（非阻塞）
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

// Stop 停止自动交易
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
	
	at.monitorWg.Wait()     // 等待监控goroutine结束
	log.Println("⏹ 自动交易系统停止")
}

// runCycle 运行一个交易周期（使用AI全权决策）
func (at *AutoTrader) runCycle() error {
	at.callCount++

	log.Print("\n" + strings.Repeat("=", 70) + "\n")
	if at.config.UseTradingView {
		log.Printf("⚠️ [%s] runCycle invoked while UseTradingView=true (unexpected periodic scan path)", at.name)
		log.Printf("⏰ %s - TradingView信号周期 #%d", time.Now().Format("2006-01-02 15:04:05"), at.callCount)
	} else {
		log.Printf("⏰ %s - AI决策周期 #%d", time.Now().Format("2006-01-02 15:04:05"), at.callCount)
	}
	log.Println(strings.Repeat("=", 70))

	// 创建决策记录
	record := &logger.DecisionRecord{
		ExecutionLog: []string{},
		Success:      true,
	}

	// 1. 检查是否需要停止交易
	if time.Now().Before(at.stopUntil) {
		remaining := at.stopUntil.Sub(time.Now())
		log.Printf("⏸ 风险控制：暂停交易中，剩余 %.0f 分钟", remaining.Minutes())
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("风险控制暂停中，剩余 %.0f 分钟", remaining.Minutes())
		at.decisionLogger.LogDecision(record)
		return nil
	}

	// 2. 重置日盈亏（每天重置）
	if time.Since(at.lastResetTime) > 24*time.Hour {
		at.dailyPnL = 0
		at.lastResetTime = time.Now()
		log.Println("📅 日盈亏已重置")
	}

	// 2.5. 检查是否是跟随者（如果有followed_trader_id，跳过AI决策周期）
	// Check if this is a follower (early exit for follower traders)
	if at.isFollower {
		log.Printf("📋 [%s] Follower mode: skipping periodic cycle, waiting for parent signals", at.name)
		record.Success = true
		record.ExecutionLog = append(record.ExecutionLog, "Follower mode: waiting for parent signals")
		at.decisionLogger.LogDecision(record)
		return nil
	}

	// 3. 检查是否使用TradingView信号（如果启用，跳过AI决策）
	if at.config.UseTradingView {
		log.Printf("📡 TradingView模式：跳过AI决策，处理TradingView警报")
		return at.processTradingViewAlerts(record)
	}

	// 4. 收集交易上下文
	ctx, err := at.buildTradingContext()
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("构建交易上下文失败: %v", err)
		at.decisionLogger.LogDecision(record)
		return fmt.Errorf("构建交易上下文失败: %w", err)
	}

	// 保存账户状态快照
	record.AccountState = logger.AccountSnapshot{
		TotalBalance:          ctx.Account.TotalEquity - ctx.Account.UnrealizedPnL,
		AvailableBalance:      ctx.Account.AvailableBalance,
		TotalUnrealizedProfit: ctx.Account.UnrealizedPnL,
		PositionCount:         ctx.Account.PositionCount,
		MarginUsedPct:         ctx.Account.MarginUsedPct,
		InitialBalance:        at.initialBalance, // 记录当时的初始余额基准
	}

	// 保存持仓快照
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

	log.Printf("📊 账户净值: %.2f USDT | 可用: %.2f USDT | 持仓: %d",
		ctx.Account.TotalEquity, ctx.Account.AvailableBalance, ctx.Account.PositionCount)

	// 5. 调用AI获取完整决策
	log.Printf("🤖 正在请求AI分析并决策... [模板: %s]", at.systemPromptTemplate)
	decision, err := decision.GetFullDecisionWithCustomPrompt(ctx, at.mcpClient, at.customPrompt, at.overrideBasePrompt, at.systemPromptTemplate)

	if decision != nil && decision.AIRequestDurationMs > 0 {
		record.AIRequestDurationMs = decision.AIRequestDurationMs
		log.Printf("⏱️ AI调用耗时: %.2f 秒", float64(record.AIRequestDurationMs)/1000)
		record.ExecutionLog = append(record.ExecutionLog,
			fmt.Sprintf("AI调用耗时: %d ms", record.AIRequestDurationMs))
	}

	// 即使有错误，也保存思维链、决策和输入prompt（用于debug）
	if decision != nil {
		record.SystemPrompt = decision.SystemPrompt // 保存系统提示词
		record.InputPrompt = decision.UserPrompt
		record.CoTTrace = decision.CoTTrace
		if len(decision.Decisions) > 0 {
			decisionJSON, _ := json.MarshalIndent(decision.Decisions, "", "  ")
			record.DecisionJSON = string(decisionJSON)
		}
	}

	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("failed to get AI decision: %v", err)

		// 打印系统提示词和AI思维链（即使有错误，也要输出以便调试）
		if decision != nil {
			log.Print("\n" + strings.Repeat("=", 70) + "\n")
			log.Printf("📋 系统提示词 [模板: %s] (错误情况)", at.systemPromptTemplate)
			log.Println(strings.Repeat("=", 70))
			log.Println(decision.SystemPrompt)
			log.Println(strings.Repeat("=", 70))

			if decision.CoTTrace != "" {
				log.Print("\n" + strings.Repeat("-", 70) + "\n")
				log.Println("💭 AI思维链分析（错误情况）:")
				log.Println(strings.Repeat("-", 70))
				log.Println(decision.CoTTrace)
				log.Println(strings.Repeat("-", 70))
			}
		}

		at.decisionLogger.LogDecision(record)
		return fmt.Errorf("failed to get AI decision: %w", err)
	}

	// // 5. 打印系统提示词
	// log.Printf("\n" + strings.Repeat("=", 70))
	// log.Printf("📋 系统提示词 [模板: %s]", at.systemPromptTemplate)
	// log.Println(strings.Repeat("=", 70))
	// log.Println(decision.SystemPrompt)
	// log.Printf(strings.Repeat("=", 70) + "\n")

	// 6. 打印AI思维链
	// log.Printf("\n" + strings.Repeat("-", 70))
	// log.Println("💭 AI思维链分析:")
	// log.Println(strings.Repeat("-", 70))
	// log.Println(decision.CoTTrace)
	// log.Printf(strings.Repeat("-", 70) + "\n")

	// 7. 打印AI决策
	// log.Printf("📋 AI决策列表 (%d 个):\n", len(decision.Decisions))
	// for i, d := range decision.Decisions {
	//     log.Printf("  [%d] %s: %s - %s", i+1, d.Symbol, d.Action, d.Reasoning)
	//     if d.Action == "open_long" || d.Action == "open_short" {
	//        log.Printf("      杠杆: %dx | 仓位: %.2f USDT | 止损: %.4f | 止盈: %.4f",
	//           d.Leverage, d.PositionSizeUSD, d.StopLoss, d.TakeProfit)
	//     }
	// }
	log.Println()
	log.Print(strings.Repeat("-", 70))
	// 8. 对决策排序：确保先平仓后开仓（防止仓位叠加超限）
	log.Print(strings.Repeat("-", 70))

	// 8. 对决策排序：确保先平仓后开仓（防止仓位叠加超限）
	sortedDecisions := sortDecisionsByPriority(decision.Decisions)

	log.Println("🔄 执行顺序（已优化）: 先平仓→后开仓")
	for i, d := range sortedDecisions {
		log.Printf("  [%d] %s %s", i+1, d.Symbol, d.Action)
	}
	log.Println()

	// 执行决策并记录结果
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
			log.Printf("❌ 执行决策失败 (%s %s): %v", d.Symbol, d.Action, err)
			actionRecord.Error = err.Error()
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ %s %s 失败: %v", d.Symbol, d.Action, err))
		} else {
			actionRecord.Success = true
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s 成功", d.Symbol, d.Action))
			// 成功执行后短暂延迟
			time.Sleep(1 * time.Second)
		}

		record.Decisions = append(record.Decisions, actionRecord)
	}

	// 9. 保存决策记录
	if err := at.decisionLogger.LogDecision(record); err != nil {
		log.Printf("⚠ 保存决策记录失败: %v", err)
	}

	return nil
}

// buildTradingContext 构建交易上下文
func (at *AutoTrader) buildTradingContext() (*decision.Context, error) {
	// 1. 获取账户信息
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("获取账户余额失败: %w", err)
	}

	// 获取账户字段
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

	// Total Equity = 钱包余额 + 未实现盈亏
	totalEquity := totalWalletBalance + totalUnrealizedProfit

	// 2. 获取持仓信息
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var positionInfos []decision.PositionInfo
	totalMarginUsed := 0.0

	// 当前持仓的key集合（用于清理已平仓的记录）
	currentPositionKeys := make(map[string]bool)

	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity // 空仓数量为负，转为正数
		}

		// 跳过已平仓的持仓（quantity = 0），防止"幽灵持仓"传递给AI
		if quantity == 0 {
			continue
		}

		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)

		// 计算占用保证金（估算）
		leverage := 10 // 默认值，实际应该从持仓信息获取
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed

		// 计算盈亏百分比（基于保证金，考虑杠杆）
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		// 跟踪持仓首次出现时间
		posKey := symbol + "_" + side
		currentPositionKeys[posKey] = true
		if _, exists := at.positionFirstSeenTime[posKey]; !exists {
			// 新持仓，记录当前时间
			at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()
		}
		updateTime := at.positionFirstSeenTime[posKey]

		// 获取该持仓的历史最高收益率
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

	// 清理已平仓的持仓记录
	for key := range at.positionFirstSeenTime {
		if !currentPositionKeys[key] {
			delete(at.positionFirstSeenTime, key)
		}
	}

	// 3. 获取交易员的候选币种池
	candidateCoins, err := at.getCandidateCoins()
	if err != nil {
		return nil, fmt.Errorf("获取候选币种失败: %w", err)
	}

	// 4. 计算总盈亏
	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	// 5. 分析历史表现（最近100个周期，避免长期持仓的交易记录丢失）
	// 假设每3分钟一个周期，100个周期 = 5小时，足够覆盖大部分交易
	performance, err := at.decisionLogger.AnalyzePerformance(100)
	if err != nil {
		log.Printf("⚠️  分析历史表现失败: %v", err)
		// 不影响主流程，继续执行（但设置performance为nil以避免传递错误数据）
		performance = nil
	}

	// 6. 构建上下文
	ctx := &decision.Context{
		CurrentTime:     time.Now().Format("2006-01-02 15:04:05"),
		RuntimeMinutes:  int(time.Since(at.startTime).Minutes()),
		CallCount:       at.callCount,
		BTCETHLeverage:  at.config.BTCETHLeverage,  // 使用配置的杠杆倍数
		AltcoinLeverage: at.config.AltcoinLeverage, // 使用配置的杠杆倍数
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
		Positions:      positionInfos,
		CandidateCoins: candidateCoins,
		Performance:    performance, // 添加历史表现分析
	}

	return ctx, nil
}

// SetTradeReplicationCallback 设置交易复制回调函数
func (at *AutoTrader) SetTradeReplicationCallback(callback func(traderID string, decision *decision.Decision)) {
	at.tradeReplicationCallback = callback
}

// ExecuteDecisionWithRecord 公开方法，用于外部执行决策（用于跟随交易）
func (at *AutoTrader) ExecuteDecisionWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	return at.executeDecisionWithRecord(decision, actionRecord)
}

// executeDecisionWithRecord 执行AI决策并记录详细信息
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
		// 无需执行，仅记录
		return nil
	default:
		return fmt.Errorf("未知的action: %s", decision.Action)
	}

	// If trade executed successfully, replicate to followers
	if err == nil && actionRecord.Success && at.tradeReplicationCallback != nil {
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

// executeOpenLongWithRecord 执行开多仓并记录详细信息
func (at *AutoTrader) executeOpenLongWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📈 开多仓: %s", decision.Symbol)

	// ⚠️ 关键：检查是否已有同币种同方向持仓，如果有则拒绝开仓（防止仓位叠加超限）
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
				return fmt.Errorf("❌ %s 已有多仓，拒绝开仓以防止仓位叠加超限。如需换仓，请先给出 close_long 决策", decision.Symbol)
			}
		}
	}

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}

	// 计算数量
	quantity := decision.PositionSizeUSD / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// ⚠️ 保证金验证：防止保证金不足错误（code=-2019）
	requiredMargin := decision.PositionSizeUSD / float64(decision.Leverage)

	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("获取账户余额失败: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// 手续费估算（Taker费率 0.04%）
	estimatedFee := decision.PositionSizeUSD * 0.0004
	totalRequired := requiredMargin + estimatedFee

	if totalRequired > availableBalance {
		return fmt.Errorf("❌ 保证金不足: 需要 %.2f USDT（保证金 %.2f + 手续费 %.2f），可用 %.2f USDT",
			totalRequired, requiredMargin, estimatedFee, availableBalance)
	}

	// 设置仓位模式
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		log.Printf("  ⚠️ 设置仓位模式失败: %v", err)
		// 继续执行，不影响交易
	}

	// 开仓
	order, err := at.trader.OpenLong(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 开仓成功，订单ID: %v, 数量: %.4f", order["orderId"], quantity)

	// 记录开仓时间
	posKey := decision.Symbol + "_long"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	// 设置止损止盈
	if err := at.trader.SetStopLoss(decision.Symbol, "LONG", quantity, decision.StopLoss); err != nil {
		log.Printf("  ⚠ 设置止损失败: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "LONG", quantity, decision.TakeProfit); err != nil {
		log.Printf("  ⚠ 设置止盈失败: %v", err)
	}

	return nil
}

// executeOpenShortWithRecord 执行开空仓并记录详细信息
func (at *AutoTrader) executeOpenShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📉 开空仓: %s", decision.Symbol)

	// ⚠️ 关键：检查是否已有同币种同方向持仓，如果有则拒绝开仓（防止仓位叠加超限）
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
				return fmt.Errorf("❌ %s 已有空仓，拒绝开仓以防止仓位叠加超限。如需换仓，请先给出 close_short 决策", decision.Symbol)
			}
		}
	}

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}

	// 计算数量
	quantity := decision.PositionSizeUSD / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// ⚠️ 保证金验证：防止保证金不足错误（code=-2019）
	requiredMargin := decision.PositionSizeUSD / float64(decision.Leverage)

	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("获取账户余额失败: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// 手续费估算（Taker费率 0.04%）
	estimatedFee := decision.PositionSizeUSD * 0.0004
	totalRequired := requiredMargin + estimatedFee

	if totalRequired > availableBalance {
		return fmt.Errorf("❌ 保证金不足: 需要 %.2f USDT（保证金 %.2f + 手续费 %.2f），可用 %.2f USDT",
			totalRequired, requiredMargin, estimatedFee, availableBalance)
	}

	// 设置仓位模式
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		log.Printf("  ⚠️ 设置仓位模式失败: %v", err)
		// 继续执行，不影响交易
	}

	// 开仓
	order, err := at.trader.OpenShort(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 开仓成功，订单ID: %v, 数量: %.4f", order["orderId"], quantity)

	// 记录开仓时间
	posKey := decision.Symbol + "_short"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	// 设置止损止盈
	if err := at.trader.SetStopLoss(decision.Symbol, "SHORT", quantity, decision.StopLoss); err != nil {
		log.Printf("  ⚠ 设置止损失败: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "SHORT", quantity, decision.TakeProfit); err != nil {
		log.Printf("  ⚠ 设置止盈失败: %v", err)
	}

	return nil
}

// executeCloseLongWithRecord 执行平多仓并记录详细信息
func (at *AutoTrader) executeCloseLongWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🔄 平多仓: %s", decision.Symbol)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 平仓
	order, err := at.trader.CloseLong(decision.Symbol, 0) // 0 = 全部平仓
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 平仓成功")
	return nil
}

// CloseLong 手动平多仓（公开方法，供API调用）
func (at *AutoTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	return at.trader.CloseLong(symbol, quantity)
}

// CloseShort 手动平空仓（公开方法，供API调用）
func (at *AutoTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return at.trader.CloseShort(symbol, quantity)
}

// executeCloseShortWithRecord 执行平空仓并记录详细信息
func (at *AutoTrader) executeCloseShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🔄 平空仓: %s", decision.Symbol)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 平仓
	order, err := at.trader.CloseShort(decision.Symbol, 0) // 0 = 全部平仓
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 平仓成功")
	return nil
}

// executeUpdateStopLossWithRecord 执行调整止损并记录详细信息
func (at *AutoTrader) executeUpdateStopLossWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🎯 调整止损: %s → %.2f", decision.Symbol, decision.NewStopLoss)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找目标持仓
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("持仓不存在: %s", decision.Symbol)
	}

	// 获取持仓方向和数量
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// 验证新止损价格合理性
	if positionSide == "LONG" && decision.NewStopLoss >= marketData.CurrentPrice {
		return fmt.Errorf("多单止损必须低于当前价格 (当前: %.2f, 新止损: %.2f)", marketData.CurrentPrice, decision.NewStopLoss)
	}
	if positionSide == "SHORT" && decision.NewStopLoss <= marketData.CurrentPrice {
		return fmt.Errorf("空单止损必须高于当前价格 (当前: %.2f, 新止损: %.2f)", marketData.CurrentPrice, decision.NewStopLoss)
	}

	// ⚠️ 防御性检查：检测是否存在双向持仓（不应该出现，但提供保护）
	var hasOppositePosition bool
	oppositeSide := ""
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posSide, _ := pos["side"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 && strings.ToUpper(posSide) != positionSide {
			hasOppositePosition = true
			oppositeSide = strings.ToUpper(posSide)
			break
		}
	}

	if hasOppositePosition {
		log.Printf("  🚨 警告：检测到 %s 存在双向持仓（%s + %s），这违反了策略规则",
			decision.Symbol, positionSide, oppositeSide)
		log.Printf("  🚨 取消止损单将影响两个方向的订单，请检查是否为用户手动操作导致")
		log.Printf("  🚨 建议：手动平掉其中一个方向的持仓，或检查系统是否有BUG")
	}

	// 取消旧的止损单（只删除止损单，不影响止盈单）
	// 注意：如果存在双向持仓，这会删除两个方向的止损单
	if err := at.trader.CancelStopLossOrders(decision.Symbol); err != nil {
		log.Printf("  ⚠ 取消旧止损单失败: %v", err)
		// 不中断执行，继续设置新止损
	}

	// 调用交易所 API 修改止损
	quantity := math.Abs(positionAmt)
	err = at.trader.SetStopLoss(decision.Symbol, positionSide, quantity, decision.NewStopLoss)
	if err != nil {
		return fmt.Errorf("修改止损失败: %w", err)
	}

	log.Printf("  ✓ 止损已调整: %.2f (当前价格: %.2f)", decision.NewStopLoss, marketData.CurrentPrice)
	return nil
}

// executeUpdateTakeProfitWithRecord 执行调整止盈并记录详细信息
func (at *AutoTrader) executeUpdateTakeProfitWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🎯 调整止盈: %s → %.2f", decision.Symbol, decision.NewTakeProfit)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找目标持仓
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("持仓不存在: %s", decision.Symbol)
	}

	// 获取持仓方向和数量
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// 验证新止盈价格合理性
	if positionSide == "LONG" && decision.NewTakeProfit <= marketData.CurrentPrice {
		return fmt.Errorf("多单止盈必须高于当前价格 (当前: %.2f, 新止盈: %.2f)", marketData.CurrentPrice, decision.NewTakeProfit)
	}
	if positionSide == "SHORT" && decision.NewTakeProfit >= marketData.CurrentPrice {
		return fmt.Errorf("空单止盈必须低于当前价格 (当前: %.2f, 新止盈: %.2f)", marketData.CurrentPrice, decision.NewTakeProfit)
	}

	// ⚠️ 防御性检查：检测是否存在双向持仓（不应该出现，但提供保护）
	var hasOppositePosition bool
	oppositeSide := ""
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posSide, _ := pos["side"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 && strings.ToUpper(posSide) != positionSide {
			hasOppositePosition = true
			oppositeSide = strings.ToUpper(posSide)
			break
		}
	}

	if hasOppositePosition {
		log.Printf("  🚨 警告：检测到 %s 存在双向持仓（%s + %s），这违反了策略规则",
			decision.Symbol, positionSide, oppositeSide)
		log.Printf("  🚨 取消止盈单将影响两个方向的订单，请检查是否为用户手动操作导致")
		log.Printf("  🚨 建议：手动平掉其中一个方向的持仓，或检查系统是否有BUG")
	}

	// 取消旧的止盈单（只删除止盈单，不影响止损单）
	// 注意：如果存在双向持仓，这会删除两个方向的止盈单
	if err := at.trader.CancelTakeProfitOrders(decision.Symbol); err != nil {
		log.Printf("  ⚠ 取消旧止盈单失败: %v", err)
		// 不中断执行，继续设置新止盈
	}

	// 调用交易所 API 修改止盈
	quantity := math.Abs(positionAmt)
	err = at.trader.SetTakeProfit(decision.Symbol, positionSide, quantity, decision.NewTakeProfit)
	if err != nil {
		return fmt.Errorf("修改止盈失败: %w", err)
	}

	log.Printf("  ✓ 止盈已调整: %.2f (当前价格: %.2f)", decision.NewTakeProfit, marketData.CurrentPrice)
	return nil
}

// executePartialCloseWithRecord 执行部分平仓并记录详细信息
func (at *AutoTrader) executePartialCloseWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📊 部分平仓: %s %.1f%%", decision.Symbol, decision.ClosePercentage)

	// 验证百分比范围
	if decision.ClosePercentage <= 0 || decision.ClosePercentage > 100 {
		return fmt.Errorf("平仓百分比必须在 0-100 之间，当前: %.1f", decision.ClosePercentage)
	}

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找目标持仓
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("持仓不存在: %s", decision.Symbol)
	}

	// 获取持仓方向和数量
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// 计算平仓数量
	totalQuantity := math.Abs(positionAmt)
	closeQuantity := totalQuantity * (decision.ClosePercentage / 100.0)
	actionRecord.Quantity = closeQuantity

	// ✅ Layer 2: 最小仓位检查（防止产生小额剩余）
	markPrice, ok := targetPosition["markPrice"].(float64)
	if !ok || markPrice <= 0 {
		return fmt.Errorf("无法解析当前价格，无法执行最小仓位检查")
	}

	currentPositionValue := totalQuantity * markPrice
	remainingQuantity := totalQuantity - closeQuantity
	remainingValue := remainingQuantity * markPrice

	const MIN_POSITION_VALUE = 10.0 // 最小持仓价值 10 USDT（對齊交易所底线，小仓位建议直接全平）

	if remainingValue > 0 && remainingValue <= MIN_POSITION_VALUE {
		log.Printf("⚠️ 检测到 partial_close 后剩余仓位 %.2f USDT < %.0f USDT",
			remainingValue, MIN_POSITION_VALUE)
		log.Printf("  → 当前仓位价值: %.2f USDT, 平仓 %.1f%%, 剩余: %.2f USDT",
			currentPositionValue, decision.ClosePercentage, remainingValue)
		log.Printf("  → 自动修正为全部平仓，避免产生无法平仓的小额剩余")

		// 🔄 自动修正为全部平仓
		if positionSide == "LONG" {
			decision.Action = "close_long"
			log.Printf("  ✓ 已修正为: close_long")
			return at.executeCloseLongWithRecord(decision, actionRecord)
		} else {
			decision.Action = "close_short"
			log.Printf("  ✓ 已修正为: close_short")
			return at.executeCloseShortWithRecord(decision, actionRecord)
		}
	}

	// 执行平仓
	var order map[string]interface{}
	if positionSide == "LONG" {
		order, err = at.trader.CloseLong(decision.Symbol, closeQuantity)
	} else {
		order, err = at.trader.CloseShort(decision.Symbol, closeQuantity)
	}

	if err != nil {
		return fmt.Errorf("部分平仓失败: %w", err)
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 部分平仓成功: 平仓 %.4f (%.1f%%), 剩余 %.4f",
		closeQuantity, decision.ClosePercentage, remainingQuantity)

	// ✅ Step 4: 恢复止盈止损（防止剩余仓位裸奔）
	// 重要：币安等交易所在部分平仓后会自动取消原有的 TP/SL 订单（因为数量不匹配）
	// 如果 AI 提供了新的止损止盈价格，则为剩余仓位重新设置保护
	if decision.NewStopLoss > 0 {
		log.Printf("  → 为剩余仓位 %.4f 恢复止损单: %.2f", remainingQuantity, decision.NewStopLoss)
		err = at.trader.SetStopLoss(decision.Symbol, positionSide, remainingQuantity, decision.NewStopLoss)
		if err != nil {
			log.Printf("  ⚠️ 恢复止损失败: %v（不影响平仓结果）", err)
		}
	}

	if decision.NewTakeProfit > 0 {
		log.Printf("  → 为剩余仓位 %.4f 恢复止盈单: %.2f", remainingQuantity, decision.NewTakeProfit)
		err = at.trader.SetTakeProfit(decision.Symbol, positionSide, remainingQuantity, decision.NewTakeProfit)
		if err != nil {
			log.Printf("  ⚠️ 恢复止盈失败: %v（不影响平仓结果）", err)
		}
	}

	// 如果 AI 没有提供新的止盈止损，记录警告
	if decision.NewStopLoss <= 0 && decision.NewTakeProfit <= 0 {
		log.Printf("  ⚠️⚠️⚠️ 警告: 部分平仓后AI未提供新的止盈止损价格")
		log.Printf("  → 剩余仓位 %.4f (价值 %.2f USDT) 目前没有止盈止损保护", remainingQuantity, remainingValue)
		log.Printf("  → 建议: 在 partial_close 决策中包含 new_stop_loss 和 new_take_profit 字段")
	}

	return nil
}

// GetID 获取trader ID
func (at *AutoTrader) GetID() string {
	return at.id
}

// GetName 获取trader名称
func (at *AutoTrader) GetName() string {
	return at.name
}

// GetAIModel 获取AI模型
func (at *AutoTrader) GetAIModel() string {
	return at.aiModel
}

// GetExchange 获取交易所
func (at *AutoTrader) GetExchange() string {
	return at.exchange
}

// SetCustomPrompt 设置自定义交易策略prompt
func (at *AutoTrader) SetCustomPrompt(prompt string) {
	at.customPrompt = prompt
}

// SetOverrideBasePrompt 设置是否覆盖基础prompt
func (at *AutoTrader) SetOverrideBasePrompt(override bool) {
	at.overrideBasePrompt = override
}

// SetSystemPromptTemplate 设置系统提示词模板
func (at *AutoTrader) SetSystemPromptTemplate(templateName string) {
	at.systemPromptTemplate = templateName
}

// GetSystemPromptTemplate 获取当前系统提示词模板名称
func (at *AutoTrader) GetSystemPromptTemplate() string {
	return at.systemPromptTemplate
}

// GetDecisionLogger 获取决策日志记录器
func (at *AutoTrader) GetDecisionLogger() logger.IDecisionLogger {
	return at.decisionLogger
}

// GetStatus 获取系统状态（用于API）
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

// GetAccountInfo 获取账户信息（用于API）
func (at *AutoTrader) GetAccountInfo() (map[string]interface{}, error) {
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("获取余额失败: %w", err)
	}

	// 获取账户字段
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

	// Total Equity = 钱包余额 + 未实现盈亏
	totalEquity := totalWalletBalance + totalUnrealizedProfit

	// 获取持仓计算总保证金
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	totalMarginUsed := 0.0
	totalUnrealizedPnLCalculated := 0.0
	for _, pos := range positions {
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		totalUnrealizedPnLCalculated += unrealizedPnl

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed
	}

	// 验证未实现盈亏的一致性（API值 vs 从持仓计算）
	diff := math.Abs(totalUnrealizedProfit - totalUnrealizedPnLCalculated)
	if diff > 0.1 { // 允许0.01 USDT的误差
		log.Printf("⚠️ 未实现盈亏不一致: API=%.4f, 计算=%.4f, 差异=%.4f",
			totalUnrealizedProfit, totalUnrealizedPnLCalculated, diff)
	}

	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	} else {
		log.Printf("⚠️ Initial Balance异常: %.2f，无法计算PNL百分比", at.initialBalance)
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	return map[string]interface{}{
		// 核心字段
		"total_equity":      totalEquity,           // 账户净值 = wallet + unrealized
		"wallet_balance":    totalWalletBalance,    // 钱包余额（不含未实现盈亏）
		"unrealized_profit": totalUnrealizedProfit, // 未实现盈亏（交易所API官方值）
		"available_balance": availableBalance,      // 可用余额

		// 盈亏统计
		"total_pnl":       totalPnL,          // 总盈亏 = equity - initial
		"total_pnl_pct":   totalPnLPct,       // 总盈亏百分比
		"initial_balance": at.initialBalance, // 初始余额
		"daily_pnl":       at.dailyPnL,       // 日盈亏

		// 持仓信息
		"position_count":  len(positions),  // 持仓数量
		"margin_used":     totalMarginUsed, // 保证金占用
		"margin_used_pct": marginUsedPct,   // 保证金使用率
	}, nil
}

// GetPositions 获取持仓列表（用于API）
func (at *AutoTrader) GetPositions() ([]map[string]interface{}, error) {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		// 计算占用保证金
		marginUsed := (quantity * markPrice) / float64(leverage)

		// 计算盈亏百分比（基于保证金）
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

// calculatePnLPercentage 计算盈亏百分比（基于保证金，自动考虑杠杆）
// 收益率 = 未实现盈亏 / 保证金 × 100%
func calculatePnLPercentage(unrealizedPnl, marginUsed float64) float64 {
	if marginUsed > 0 {
		return (unrealizedPnl / marginUsed) * 100
	}
	return 0.0
}

// sortDecisionsByPriority 对决策排序：先平仓，再开仓，最后hold/wait
// 这样可以避免换仓时仓位叠加超限
func sortDecisionsByPriority(decisions []decision.Decision) []decision.Decision {
	if len(decisions) <= 1 {
		return decisions
	}

	// 定义优先级
	getActionPriority := func(action string) int {
		switch action {
		case "close_long", "close_short", "partial_close":
			return 1 // 最高优先级：先平仓（包括部分平仓）
		case "update_stop_loss", "update_take_profit":
			return 2 // 调整持仓止盈止损
		case "open_long", "open_short":
			return 3 // 次优先级：后开仓
		case "hold", "wait":
			return 4 // 最低优先级：观望
		default:
			return 999 // 未知动作放最后
		}
	}

	// 复制决策列表
	sorted := make([]decision.Decision, len(decisions))
	copy(sorted, decisions)

	// 按优先级排序
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if getActionPriority(sorted[i].Action) > getActionPriority(sorted[j].Action) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// getCandidateCoins 获取交易员的候选币种列表
func (at *AutoTrader) getCandidateCoins() ([]decision.CandidateCoin, error) {
	if len(at.tradingCoins) == 0 {
		// 使用数据库配置的默认币种列表
		var candidateCoins []decision.CandidateCoin

		if len(at.defaultCoins) > 0 {
			// 使用数据库中配置的默认币种
			for _, coin := range at.defaultCoins {
				symbol := normalizeSymbol(coin)
				candidateCoins = append(candidateCoins, decision.CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"default"}, // 标记为数据库默认币种
				})
			}
			log.Printf("📋 [%s] 使用数据库默认币种: %d个币种 %v",
				at.name, len(candidateCoins), at.defaultCoins)
			return candidateCoins, nil
		} else {
			// 如果数据库中没有配置默认币种，则使用AI500+OI Top作为fallback
			const ai500Limit = 20 // AI500取前20个评分最高的币种

			mergedPool, err := pool.GetMergedCoinPool(ai500Limit)
			if err != nil {
				return nil, fmt.Errorf("获取合并币种池失败: %w", err)
			}

			// 构建候选币种列表（包含来源信息）
			for _, symbol := range mergedPool.AllSymbols {
				sources := mergedPool.SymbolSources[symbol]
				candidateCoins = append(candidateCoins, decision.CandidateCoin{
					Symbol:  symbol,
					Sources: sources, // "ai500" 和/或 "oi_top"
				})
			}

			log.Printf("📋 [%s] 数据库无默认币种配置，使用AI500+OI Top: AI500前%d + OI_Top20 = 总计%d个候选币种",
				at.name, ai500Limit, len(candidateCoins))
			return candidateCoins, nil
		}
	} else {
		// 使用自定义币种列表
		var candidateCoins []decision.CandidateCoin
		for _, coin := range at.tradingCoins {
			// 确保币种格式正确（转为大写USDT交易对）
			symbol := normalizeSymbol(coin)
			candidateCoins = append(candidateCoins, decision.CandidateCoin{
				Symbol:  symbol,
				Sources: []string{"custom"}, // 标记为自定义来源
			})
		}

		log.Printf("📋 [%s] 使用自定义币种: %d个币种 %v",
			at.name, len(candidateCoins), at.tradingCoins)
		return candidateCoins, nil
	}
}

// normalizeSymbol 标准化币种符号（确保以USDT结尾）
func normalizeSymbol(symbol string) string {
	// 转为大写
	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	// 确保以USDT结尾
	if !strings.HasSuffix(symbol, "USDT") {
		symbol = symbol + "USDT"
	}

	return symbol
}

// 启动回撤监控
func (at *AutoTrader) startDrawdownMonitor() {
	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()

		ticker := time.NewTicker(1 * time.Minute) // 每分钟检查一次
		defer ticker.Stop()

		log.Println("📊 启动持仓回撤监控（每分钟检查一次）")

		for {
			select {
			case <-ticker.C:
				at.checkPositionDrawdown()
			case <-at.stopMonitorCh:
				log.Println("⏹ 停止持仓回撤监控")
				return
			}
		}
	}()
}

// 检查持仓回撤情况
func (at *AutoTrader) checkPositionDrawdown() {
	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		log.Printf("❌ 回撤监控：获取持仓失败: %v", err)
		return
	}

	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity // 空仓数量为负，转为正数
		}

		// 计算当前盈亏百分比
		leverage := 10 // 默认值
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		var currentPnLPct float64
		if side == "long" {
			currentPnLPct = ((markPrice - entryPrice) / entryPrice) * float64(leverage) * 100
		} else {
			currentPnLPct = ((entryPrice - markPrice) / entryPrice) * float64(leverage) * 100
		}

		// 构造持仓唯一标识（区分多空）
		posKey := symbol + "_" + side

		// 获取该持仓的历史最高收益
		at.peakPnLCacheMutex.RLock()
		peakPnLPct, exists := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		if !exists {
			// 如果没有历史最高记录，使用当前盈亏作为初始值
			peakPnLPct = currentPnLPct
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		} else {
			// 更新峰值缓存
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		}

		// 计算回撤（从最高点下跌的幅度）
		var drawdownPct float64
		if peakPnLPct > 0 && currentPnLPct < peakPnLPct {
			drawdownPct = ((peakPnLPct - currentPnLPct) / peakPnLPct) * 100
		}

		// 检查平仓条件：收益大于5%且回撤超过40%
		if currentPnLPct > 5.0 && drawdownPct >= 40.0 {
			log.Printf("🚨 触发回撤平仓条件: %s %s | 当前收益: %.2f%% | 最高收益: %.2f%% | 回撤: %.2f%%",
				symbol, side, currentPnLPct, peakPnLPct, drawdownPct)

			// 执行平仓
			if err := at.emergencyClosePosition(symbol, side); err != nil {
				log.Printf("❌ 回撤平仓失败 (%s %s): %v", symbol, side, err)
			} else {
				log.Printf("✅ 回撤平仓成功: %s %s", symbol, side)
				// 平仓后清理该持仓的缓存
				at.ClearPeakPnLCache(symbol, side)
			}
		} else if currentPnLPct > 5.0 {
			// 记录接近平仓条件的情况（用于调试）
			log.Printf("📊 回撤监控: %s %s | 收益: %.2f%% | 最高: %.2f%% | 回撤: %.2f%%",
				symbol, side, currentPnLPct, peakPnLPct, drawdownPct)
		}
	}
}

// 紧急平仓函数
func (at *AutoTrader) emergencyClosePosition(symbol, side string) error {
	switch side {
	case "long":
		order, err := at.trader.CloseLong(symbol, 0) // 0 = 全部平仓
		if err != nil {
			return err
		}
		log.Printf("✅ 紧急平多仓成功，订单ID: %v", order["orderId"])
	case "short":
		order, err := at.trader.CloseShort(symbol, 0) // 0 = 全部平仓
		if err != nil {
			return err
		}
		log.Printf("✅ 紧急平空仓成功，订单ID: %v", order["orderId"])
	default:
		return fmt.Errorf("未知的持仓方向: %s", side)
	}

	return nil
}

// GetPeakPnLCache 获取最高收益缓存
func (at *AutoTrader) GetPeakPnLCache() map[string]float64 {
	at.peakPnLCacheMutex.RLock()
	defer at.peakPnLCacheMutex.RUnlock()

	// 返回缓存的副本
	cache := make(map[string]float64)
	for k, v := range at.peakPnLCache {
		cache[k] = v
	}
	return cache
}

// UpdatePeakPnL 更新最高收益缓存
func (at *AutoTrader) UpdatePeakPnL(symbol, side string, currentPnLPct float64) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	if peak, exists := at.peakPnLCache[posKey]; exists {
		// 更新峰值（如果是多头，取较大值；如果是空头，currentPnLPct为负，也要比较）
		if currentPnLPct > peak {
			at.peakPnLCache[posKey] = currentPnLPct
		}
	} else {
		// 首次记录
		at.peakPnLCache[posKey] = currentPnLPct
	}
}

// ClearPeakPnLCache 清除指定持仓的峰值缓存
func (at *AutoTrader) ClearPeakPnLCache(symbol, side string) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	delete(at.peakPnLCache, posKey)
}

// processTradingViewAlerts 处理TradingView警报（跳过AI决策）
func (at *AutoTrader) processTradingViewAlerts(record *logger.DecisionRecord) error {
	// 获取数据库引用
	db, ok := at.database.(*cfg.Database)
	if !ok {
		record.Success = false
		record.ErrorMessage = "无法访问数据库"
		at.decisionLogger.LogDecision(record)
		return fmt.Errorf("数据库类型错误")
	}

	// 获取待处理的警报
	alerts, err := db.GetPendingTradingViewAlerts(at.id)
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("获取TradingView警报失败: %v", err)
		at.decisionLogger.LogDecision(record)
		return fmt.Errorf("获取警报失败: %w", err)
	}

	if len(alerts) == 0 {
		log.Println("📭 没有待处理的TradingView警报")
		record.ExecutionLog = append(record.ExecutionLog, "没有待处理的TradingView警报")
		at.decisionLogger.LogDecision(record)
		return nil
	}

	log.Printf("📨 找到 %d 个待处理的TradingView警报", len(alerts))

	// 获取账户信息（用于风险检查）
	balance, err := at.trader.GetBalance()
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("获取账户信息失败: %v", err)
		at.decisionLogger.LogDecision(record)
		return fmt.Errorf("获取账户信息失败: %w", err)
	}

	// 从balance map中提取账户信息
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

	// 获取持仓数量
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

	// 转换警报为决策动作
	var decisions []decision.Decision
	for _, alert := range alerts {
		d := at.convertAlertToDecision(alert)
		if d != nil {
			decisions = append(decisions, *d)
			// 标记警报为已接受
			_ = db.UpdateAlertStatus(alert.ID, "accepted")
		}
	}

	if len(decisions) == 0 {
		log.Println("⚠️ 没有有效的交易动作")
		record.ExecutionLog = append(record.ExecutionLog, "没有有效的交易动作")
		at.decisionLogger.LogDecision(record)
		return nil
	}

	// 执行交易
	log.Printf("🔄 执行 %d 个TradingView交易动作", len(decisions))
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
			log.Printf("❌ 执行TradingView交易失败 (%s %s): %v", d.Symbol, d.Action, err)
			actionRecord.Error = err.Error()
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ %s %s 失败: %v", d.Symbol, d.Action, err))
		} else {
			actionRecord.Success = true
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s 成功", d.Symbol, d.Action))
			// 标记警报为已执行
			if i < len(alerts) {
				_ = db.UpdateAlertStatus(alerts[i].ID, "executed")
			}
			time.Sleep(1 * time.Second)
		}

		record.Decisions = append(record.Decisions, actionRecord)
	}

	// 保存决策记录
	if err := at.decisionLogger.LogDecision(record); err != nil {
		log.Printf("⚠ 保存决策记录失败: %v", err)
	}

	return nil
}

// convertAlertToDecision 将TradingView警报转换为决策
func (at *AutoTrader) convertAlertToDecision(alert cfg.TradingViewAlert) *decision.Decision {
	// 映射action: "buy" -> "open_long", "sell" -> "open_short"
	var action string
	if alert.Action == "buy" {
		action = "open_long"
	} else if alert.Action == "sell" {
		action = "open_short"
	} else {
		log.Printf("⚠️ 未知的action: %s", alert.Action)
		return nil
	}

	// 确定数量（优先使用position_size，如果为0则使用quantity）
	quantity := alert.PositionSize
	if quantity == 0 {
		quantity = alert.Quantity
	}
	if quantity == 0 {
		log.Printf("⚠️ 警报数量为0: %s", alert.Symbol)
		return nil
	}

	// 确定杠杆（根据币种选择）
	leverage := at.config.AltcoinLeverage
	if alert.Symbol == "BTCUSDT" || alert.Symbol == "ETHUSDT" {
		leverage = at.config.BTCETHLeverage
	}

	// 计算仓位大小（USD）
	positionSizeUSD := quantity * alert.Entry

	return &decision.Decision{
		Action:         action,
		Symbol:         alert.Symbol,
		Leverage:       leverage,
		PositionSizeUSD: positionSizeUSD,
		StopLoss:       alert.SL,
		TakeProfit:     alert.TP,
		Reasoning:      fmt.Sprintf("TradingView警报: %s %s @ %.2f", alert.Symbol, action, alert.Entry),
	}
}

// processTradingViewAlertWithAI 处理单个TradingView警报，使用AI分析
func (at *AutoTrader) processTradingViewAlertWithAI(alertID string) {
	log.Printf("🤖 [%s] 开始AI分析TradingView警报: %s", at.name, alertID)

	// 获取数据库引用
	db, ok := at.database.(*cfg.Database)
	if !ok {
		log.Printf("❌ [%s] 数据库类型错误", at.name)
		return
	}

	// 获取警报
	alert, err := db.GetTradingViewAlertByID(alertID)
	if err != nil {
		log.Printf("❌ [%s] 获取警报失败: %v", at.name, err)
		return
	}

	// 更新状态为 analyzing
	if err := db.UpdateAlertStatus(alertID, "analyzing"); err != nil {
		log.Printf("⚠️ [%s] 更新警报状态失败: %v", at.name, err)
	}

	// 创建决策记录
	record := &logger.DecisionRecord{
		Timestamp:    time.Now(),
		Success:      false,
		ExecutionLog: []string{},
		Decisions:    []logger.DecisionAction{},
	}

	// 构建交易上下文（仅针对该币种）
	tradingCtx, err := at.buildTradingContextForSymbol(alert.Symbol, alert)
	if err != nil {
		log.Printf("❌ [%s] 构建交易上下文失败: %v", at.name, err)
		record.ErrorMessage = fmt.Sprintf("构建交易上下文失败: %v", err)
		_ = db.UpdateAlertStatus(alertID, "error")
		at.decisionLogger.LogDecision(record)
		return
	}

	// 构建用户提示词（包含TradingView信号数据）
	userPrompt := at.buildTradingViewUserPrompt(tradingCtx, alert)

	// 构建系统提示词（包含TradingView信号分析说明）
	systemPrompt := decision.BuildSystemPromptWithTradingView(
		tradingCtx.Account.TotalEquity,
		tradingCtx.BTCETHLeverage,
		tradingCtx.AltcoinLeverage,
		at.customPrompt,
		at.overrideBasePrompt,
		at.systemPromptTemplate,
		tradingCtx.PromptVariant,
	)

	// 调用AI
	aiCallStart := time.Now()
	aiResponse, err := at.mcpClient.CallWithMessages(systemPrompt, userPrompt)
	aiCallDuration := time.Since(aiCallStart)
	if err != nil {
		log.Printf("❌ [%s] AI调用失败: %v", at.name, err)
		record.ErrorMessage = fmt.Sprintf("AI调用失败: %v", err)
		_ = db.UpdateAlertStatus(alertID, "error")
		at.decisionLogger.LogDecision(record)
		return
	}

	log.Printf("✅ [%s] AI调用成功，耗时: %v", at.name, aiCallDuration)
	record.AIRequestDurationMs = aiCallDuration.Milliseconds()

	// 解析AI响应
	fullDecision, err := decision.ParseFullDecisionResponse(tradingCtx, aiResponse)
	if err != nil {
		log.Printf("❌ [%s] failed to parse AI response: %v", at.name, err)
		record.ErrorMessage = fmt.Sprintf("failed to parse AI response: %v", err)
		_ = db.UpdateAlertStatus(alertID, "error")
		at.decisionLogger.LogDecision(record)
		return
	}

	// 检查是否有决策
	if len(fullDecision.Decisions) == 0 {
		log.Printf("⚠️ [%s] AI未返回任何决策", at.name)
		record.ErrorMessage = "AI未返回任何决策"
		_ = db.UpdateAlertStatus(alertID, "rejected")
		at.decisionLogger.LogDecision(record)
		return
	}

	// 获取第一个决策（应该只有一个）
	aiDecision := fullDecision.Decisions[0]

	// 检查信号决策
	if aiDecision.SignalDecision == "reject" {
		log.Printf("❌ [%s] AI拒绝TradingView信号: %s", at.name, aiDecision.Reasoning)
		record.ErrorMessage = fmt.Sprintf("AI拒绝: %s", aiDecision.Reasoning)
		_ = db.UpdateAlertStatus(alertID, "rejected")
		at.decisionLogger.LogDecision(record)
		return
	}

	// AI接受或修改信号
	if aiDecision.SignalDecision == "accept" || aiDecision.SignalDecision == "modify" {
		log.Printf("✅ [%s] AI%sTradingView信号", at.name, 
			map[string]string{"accept": "接受", "modify": "修改"}[aiDecision.SignalDecision])

		// 执行决策
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
			log.Printf("❌ [%s] 执行决策失败: %v", at.name, err)
			actionRecord.Error = err.Error()
			record.ErrorMessage = fmt.Sprintf("执行失败: %v", err)
			_ = db.UpdateAlertStatus(alertID, "error")
		} else {
			actionRecord.Success = true
			_ = db.UpdateAlertStatus(alertID, "executed")
			log.Printf("✅ [%s] TradingView信号已执行", at.name)
		}

		record.Decisions = append(record.Decisions, actionRecord)
		record.Success = actionRecord.Success
	}

	// 保存决策记录
	if err := at.decisionLogger.LogDecision(record); err != nil {
		log.Printf("⚠️ [%s] 保存决策记录失败: %v", at.name, err)
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
		at.decisionLogger.LogDecision(record)
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
		at.decisionLogger.LogDecision(record)
		return
	}
	
	record.AIRequestDurationMs = aiCallDuration.Milliseconds()
	
	// Parse AI response
	fullDecision, err := decision.ParseFullDecisionResponse(tradingCtx, aiResponse)
	if err != nil {
		log.Printf("❌ [%s] Failed to parse AI response: %v", at.name, err)
		record.ErrorMessage = fmt.Sprintf("Failed to parse: %v", err)
		at.decisionLogger.LogDecision(record)
		return
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
		at.decisionLogger.LogDecision(record)
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

// buildTradingContextForSymbol 为特定币种构建交易上下文
func (at *AutoTrader) buildTradingContextForSymbol(symbol string, alert *cfg.TradingViewAlert) (*decision.Context, error) {
	// 1. 获取账户信息
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("获取账户余额失败: %w", err)
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

	// 2. 获取持仓信息
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var positionInfos []decision.PositionInfo
	for _, pos := range positions {
		symbolPos := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		if quantity == 0 {
			continue
		}

		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		posKey := symbolPos + "_" + side
		updateTime := at.positionFirstSeenTime[posKey]
		if updateTime == 0 {
			updateTime = time.Now().UnixMilli()
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

	// 3. 获取该币种的市场数据
	marketData, err := market.Get(symbol)
	if err != nil {
		return nil, fmt.Errorf("获取市场数据失败: %w", err)
	}

	marketDataMap := make(map[string]*market.Data)
	marketDataMap[symbol] = marketData

	// 4. 构建上下文
	ctx := &decision.Context{
		CurrentTime:     time.Now().Format("2006-01-02 15:04:05"),
		CallCount:       at.callCount,
		RuntimeMinutes:  int(time.Since(at.startTime).Minutes()),
		Account: decision.AccountInfo{
			TotalEquity:      totalEquity,
			AvailableBalance: availableBalance,
			TotalPnLPct:      ((totalEquity - at.initialBalance) / at.initialBalance) * 100,
			MarginUsedPct:    (totalWalletBalance / totalEquity) * 100,
			PositionCount:    len(positions),
		},
		Positions:      positionInfos,
		MarketDataMap:  marketDataMap,
		CandidateCoins: []decision.CandidateCoin{{Symbol: symbol, Sources: []string{"tradingview"}}},
		BTCETHLeverage: at.config.BTCETHLeverage,
		AltcoinLeverage: at.config.AltcoinLeverage,
		PromptVariant:  "",
	}

	return ctx, nil
}

// buildTradingViewUserPrompt 构建TradingView专用的用户提示词
func (at *AutoTrader) buildTradingViewUserPrompt(ctx *decision.Context, alert *cfg.TradingViewAlert) string {
	var sb strings.Builder

	// 系统状态
	sb.WriteString(fmt.Sprintf("时间: %s | 周期: #%d | 运行: %d分钟\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// TradingView信号详情
	sb.WriteString("## TradingView信号接收\n\n")
	sb.WriteString(fmt.Sprintf("警报ID: %s\n", alert.ID))
	sb.WriteString(fmt.Sprintf("币种: %s\n", alert.Symbol))
	sb.WriteString(fmt.Sprintf("操作: %s (%s)\n", alert.Action, map[string]string{"buy": "开多", "sell": "开空"}[alert.Action]))
	sb.WriteString(fmt.Sprintf("入场价: %.4f\n", alert.Entry))
	sb.WriteString(fmt.Sprintf("止损价: %.4f\n", alert.SL))
	sb.WriteString(fmt.Sprintf("止盈价: %.4f\n", alert.TP))
	sb.WriteString(fmt.Sprintf("数量: %.4f\n", alert.Quantity))
	if alert.PositionSize > 0 {
		sb.WriteString(fmt.Sprintf("仓位大小: %.2f USDT\n", alert.PositionSize))
	}
	sb.WriteString("\n")

	// BTC 市场（如果不同）
	if alert.Symbol != "BTCUSDT" {
		if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
			sb.WriteString(fmt.Sprintf("BTC: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD: %.4f | RSI: %.2f\n\n",
				btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
				btcData.CurrentMACD, btcData.CurrentRSI7))
		}
	}

	// 账户状态
	sb.WriteString(fmt.Sprintf("账户: 净值%.2f | 余额%.2f (%.1f%%) | 盈亏%+.2f%% | 保证金%.1f%% | 持仓%d个\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// 当前持仓
	if len(ctx.Positions) > 0 {
		sb.WriteString("## 当前持仓\n")
		for i, pos := range ctx.Positions {
			sb.WriteString(fmt.Sprintf("%d. %s %s | 入场价%.4f 当前价%.4f | 数量%.4f | 盈亏%+.2f%% | 杠杆%dx\n\n",
				i+1, pos.Symbol, strings.ToUpper(pos.Side),
				pos.EntryPrice, pos.MarkPrice, pos.Quantity, pos.UnrealizedPnLPct, pos.Leverage))
		}
	} else {
		sb.WriteString("当前持仓: 无\n\n")
	}

	// 目标币种的完整市场数据
	if marketData, ok := ctx.MarketDataMap[alert.Symbol]; ok {
		sb.WriteString(fmt.Sprintf("## %s 市场数据\n\n", alert.Symbol))
		sb.WriteString(market.Format(marketData))
		sb.WriteString("\n\n")
	}

	// 决策请求
	sb.WriteString("---\n\n")
	sb.WriteString("请分析这个TradingView信号，并决定：接受(accept)、拒绝(reject)或修改(modify)。\n")
	sb.WriteString("如果接受，使用信号提供的参数；如果修改，使用你认为更合适的参数。\n")
	sb.WriteString("在JSON输出中必须包含 signal_decision 字段（值为 \"accept\"、\"reject\" 或 \"modify\"）。\n")
	sb.WriteString("现在请分析并输出决策（思维链 + JSON）\n")

	return sb.String()
}

// buildParentSignalUserPrompt builds the user prompt for parent trade signal analysis
func (at *AutoTrader) buildParentSignalUserPrompt(ctx *decision.Context, signal *ParentTradeSignal) string {
	var sb strings.Builder
	
	sb.WriteString("# Parent Trade Signal Analysis\n\n")
	sb.WriteString(fmt.Sprintf("Parent Trader: %s (ID: %s)\n", 
		signal.ParentTraderName, signal.ParentTraderID))
	sb.WriteString(fmt.Sprintf("Signal ID: %s\n", signal.SignalID))
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n\n", signal.Timestamp.Format(time.RFC3339)))
	
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
		if marketData.CurrentRSI7 > 0 {
			sb.WriteString(fmt.Sprintf("- RSI(7): %.2f\n", marketData.CurrentRSI7))
		}
		if marketData.CurrentMACD != 0 {
			sb.WriteString(fmt.Sprintf("- MACD: %.4f\n", marketData.CurrentMACD))
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

