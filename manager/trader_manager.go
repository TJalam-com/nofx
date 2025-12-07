package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"nofx/config"
	dec "nofx/decision"
	"nofx/logger"
	"nofx/trader"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CompetitionCache 竞赛数据缓存
type CompetitionCache struct {
	data      map[string]interface{}
	timestamp time.Time
	mu        sync.RWMutex
}

// TraderManager 管理多个trader实例
type TraderManager struct {
	traders          map[string]*trader.AutoTrader // key: trader ID
	competitionCache *CompetitionCache
	mu               sync.RWMutex
}

// NewTraderManager 创建trader管理器
func NewTraderManager() *TraderManager {
	return &TraderManager{
		traders: make(map[string]*trader.AutoTrader),
		competitionCache: &CompetitionCache{
			data: make(map[string]interface{}),
		},
	}
}

// LoadTradersFromDatabase 从数据库加载所有交易员到内存
func (tm *TraderManager) LoadTradersFromDatabase(database *config.Database) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 获取所有用户
	userIDs, err := database.GetAllUsers()
	if err != nil {
		return fmt.Errorf("获取用户列表失败: %w", err)
	}

	log.Printf("📋 发现 %d 个用户，开始加载所有交易员配置...", len(userIDs))

	var allTraders []*config.TraderRecord
	for _, userID := range userIDs {
		// 获取每个用户的交易员
		traders, err := database.GetTraders(userID)
		if err != nil {
			log.Printf("⚠️ 获取用户 %s 的交易员失败: %v", userID, err)
			continue
		}
		log.Printf("📋 用户 %s: %d 个交易员", userID, len(traders))
		allTraders = append(allTraders, traders...)
	}

	log.Printf("📋 总共加载 %d 个交易员配置", len(allTraders))

	// 获取系统配置（不包含信号源，信号源现在为用户级别）
	maxDailyLossStr, _ := database.GetSystemConfig("max_daily_loss")
	maxDrawdownStr, _ := database.GetSystemConfig("max_drawdown")
	stopTradingMinutesStr, _ := database.GetSystemConfig("stop_trading_minutes")
	defaultCoinsStr, _ := database.GetSystemConfig("default_coins")

	// 解析配置
	maxDailyLoss := 10.0 // 默认值
	if val, err := strconv.ParseFloat(maxDailyLossStr, 64); err == nil {
		maxDailyLoss = val
	}

	maxDrawdown := 20.0 // 默认值
	if val, err := strconv.ParseFloat(maxDrawdownStr, 64); err == nil {
		maxDrawdown = val
	}

	stopTradingMinutes := 60 // 默认值
	if val, err := strconv.Atoi(stopTradingMinutesStr); err == nil {
		stopTradingMinutes = val
	}

	// 解析默认币种列表
	var defaultCoins []string
	if defaultCoinsStr != "" {
		if err := json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins); err != nil {
			log.Printf("⚠️ 解析默认币种配置失败: %v，使用空列表", err)
			defaultCoins = []string{}
		}
	}

	// 为每个交易员获取AI模型和交易所配置
	for _, traderCfg := range allTraders {
		// 获取AI模型配置（使用交易员所属的用户ID）
		aiModels, err := database.GetAIModels(traderCfg.UserID)
		if err != nil {
			log.Printf("⚠️  获取AI模型配置失败: %v", err)
			continue
		}

		var aiModelCfg *config.AIModelConfig
		// 优先精确匹配 model.ID（新版逻辑）
		for _, model := range aiModels {
			if model.ID == traderCfg.AIModelID {
				aiModelCfg = model
				break
			}
		}
		// 如果没有精确匹配，尝试匹配 provider（兼容旧数据）
		if aiModelCfg == nil {
			for _, model := range aiModels {
				if model.Provider == traderCfg.AIModelID {
					aiModelCfg = model
					log.Printf("⚠️  交易员 %s 使用旧版 provider 匹配: %s -> %s", traderCfg.Name, traderCfg.AIModelID, model.ID)
					break
				}
			}
		}

		if aiModelCfg == nil {
			log.Printf("⚠️  交易员 %s 的AI模型 %s 不存在，跳过", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		if !aiModelCfg.Enabled {
			log.Printf("⚠️  交易员 %s 的AI模型 %s 未启用，跳过", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		// 获取交易所配置（使用交易员所属的用户ID）
		exchanges, err := database.GetExchanges(traderCfg.UserID)
		if err != nil {
			log.Printf("⚠️  获取交易所配置失败: %v", err)
			continue
		}

		var exchangeCfg *config.ExchangeConfig
		for _, exchange := range exchanges {
			if exchange.ID == traderCfg.ExchangeID {
				exchangeCfg = exchange
				break
			}
		}

		if exchangeCfg == nil {
			log.Printf("⚠️  交易员 %s 的交易所 %s 不存在，跳过", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		if !exchangeCfg.Enabled {
			log.Printf("⚠️  交易员 %s 的交易所 %s 未启用，跳过", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		// 获取用户信号源配置
		var coinPoolURL, oiTopURL string
		if userSignalSource, err := database.GetUserSignalSource(traderCfg.UserID); err == nil {
			coinPoolURL = userSignalSource.CoinPoolURL
			oiTopURL = userSignalSource.OITopURL
		} else {
			// 如果用户没有配置信号源，使用空字符串
			log.Printf("🔍 用户 %s 暂未配置信号源", traderCfg.UserID)
		}

		// 添加到TraderManager
		err = tm.addTraderFromDB(traderCfg, aiModelCfg, exchangeCfg, coinPoolURL, oiTopURL, maxDailyLoss, maxDrawdown, stopTradingMinutes, defaultCoins, database, traderCfg.UserID)
		if err != nil {
			log.Printf("❌ 添加交易员 %s 失败: %v", traderCfg.Name, err)
			continue
		}
	}

	log.Printf("✓ 成功加载 %d 个交易员到内存", len(tm.traders))
	return nil
}

// addTraderFromConfig 内部方法：从配置添加交易员（不加锁，因为调用方已加锁）
func (tm *TraderManager) addTraderFromDB(traderCfg *config.TraderRecord, aiModelCfg *config.AIModelConfig, exchangeCfg *config.ExchangeConfig, coinPoolURL, oiTopURL string, maxDailyLoss, maxDrawdown float64, stopTradingMinutes int, defaultCoins []string, database *config.Database, userID string) error {
	if _, exists := tm.traders[traderCfg.ID]; exists {
		return fmt.Errorf("trader ID '%s' 已存在", traderCfg.ID)
	}

	// 处理交易币种列表
	var tradingCoins []string
	if traderCfg.TradingSymbols != "" {
		// 解析逗号分隔的交易币种列表
		symbols := strings.Split(traderCfg.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" {
				tradingCoins = append(tradingCoins, symbol)
			}
		}
	}

	// 如果没有指定交易币种，使用默认币种
	if len(tradingCoins) == 0 {
		tradingCoins = defaultCoins
	}

	// 根据交易员配置决定是否使用信号源
	var effectiveCoinPoolURL string
	if traderCfg.UseCoinPool && coinPoolURL != "" {
		effectiveCoinPoolURL = coinPoolURL
		log.Printf("✓ 交易员 %s 启用 COIN POOL 信号源: %s", traderCfg.Name, coinPoolURL)
	}

	// 构建AutoTraderConfig
	traderConfig := trader.AutoTraderConfig{
		ID:                    traderCfg.ID,
		Name:                  traderCfg.Name,
		AIModel:               aiModelCfg.Provider, // 使用provider作为模型标识
		Exchange:              exchangeCfg.ID,      // 使用exchange ID
		BinanceAPIKey:         "",
		BinanceSecretKey:      "",
		HyperliquidPrivateKey: "",
		HyperliquidTestnet:    exchangeCfg.Testnet,
		CoinPoolAPIURL:        effectiveCoinPoolURL,
		UseQwen:               aiModelCfg.Provider == "qwen",
		DeepSeekKey:           "",
		QwenKey:               "",
		CustomAPIURL:          aiModelCfg.CustomAPIURL,    // 自定义API URL
		CustomModelName:       aiModelCfg.CustomModelName, // 自定义模型名称
		ScanInterval:          time.Duration(traderCfg.ScanIntervalMinutes) * time.Minute,
		InitialBalance:        traderCfg.InitialBalance,
		BTCETHLeverage:        traderCfg.BTCETHLeverage,
		AltcoinLeverage:       traderCfg.AltcoinLeverage,
		MaxDailyLoss:          maxDailyLoss,
		MaxDrawdown:           maxDrawdown,
		StopTradingTime:       time.Duration(stopTradingMinutes) * time.Minute,
		IsCrossMargin:         traderCfg.IsCrossMargin,
		DefaultCoins:          defaultCoins,
		TradingCoins:          tradingCoins,
		SystemPromptTemplate:  traderCfg.SystemPromptTemplate, // 系统提示词模板
		UseTradingView:        traderCfg.UseTradingView,        // TradingView信号源
	}

	// 根据交易所类型设置API密钥
	if exchangeCfg.ID == "binance" {
		traderConfig.BinanceAPIKey = exchangeCfg.APIKey
		traderConfig.BinanceSecretKey = exchangeCfg.SecretKey
	} else if exchangeCfg.ID == "bybit" {
		traderConfig.BybitAPIKey = exchangeCfg.APIKey
		traderConfig.BybitSecretKey = exchangeCfg.SecretKey
	} else if exchangeCfg.ID == "okx" {
		traderConfig.OkxAPIKey = exchangeCfg.APIKey
		traderConfig.OkxSecretKey = exchangeCfg.SecretKey
		traderConfig.OkxPassphrase = exchangeCfg.OkxPassphrase
	} else if exchangeCfg.ID == "hyperliquid" {
		traderConfig.HyperliquidPrivateKey = exchangeCfg.APIKey // hyperliquid用APIKey存储private key
		traderConfig.HyperliquidWalletAddr = exchangeCfg.HyperliquidWalletAddr
	} else if exchangeCfg.ID == "aster" {
		traderConfig.AsterUser = exchangeCfg.AsterUser
		traderConfig.AsterSigner = exchangeCfg.AsterSigner
		traderConfig.AsterPrivateKey = exchangeCfg.AsterPrivateKey
	} else if exchangeCfg.ID == "lighter" {
		traderConfig.LighterPrivateKey = exchangeCfg.LighterPrivateKey
		traderConfig.LighterWalletAddr = exchangeCfg.LighterWalletAddr
		traderConfig.LighterAPIKeyPrivateKey = exchangeCfg.LighterAPIKeyPrivateKey
		traderConfig.LighterTestnet = exchangeCfg.Testnet
	}

	// 根据AI模型设置API密钥
	if aiModelCfg.Provider == "qwen" {
		traderConfig.QwenKey = aiModelCfg.APIKey
	} else if aiModelCfg.Provider == "deepseek" {
		traderConfig.DeepSeekKey = aiModelCfg.APIKey
	}

	// 创建trader实例
	at, err := trader.NewAutoTrader(traderConfig, database, userID)
	if err != nil {
		return fmt.Errorf("创建trader失败: %w", err)
	}

	// 设置交易复制回调函数（用于实时复制交易到跟随者）
	at.SetTradeReplicationCallback(func(traderID string, decision *dec.Decision) {
		tm.ReplicateTradeToFollowers(traderID, decision, database)
	})

	// 设置自定义prompt（如果有）
	if traderCfg.CustomPrompt != "" {
		at.SetCustomPrompt(traderCfg.CustomPrompt)
		at.SetOverrideBasePrompt(traderCfg.OverrideBasePrompt)
		if traderCfg.OverrideBasePrompt {
			log.Printf("✓ 已设置自定义交易策略prompt (覆盖基础prompt)")
		} else {
			log.Printf("✓ 已设置自定义交易策略prompt (补充基础prompt)")
		}
	}

	tm.traders[traderCfg.ID] = at
	log.Printf("✓ Trader '%s' (%s + %s) 已加载到内存", traderCfg.Name, aiModelCfg.Provider, exchangeCfg.ID)
	return nil
}

// AddTrader 从数据库配置添加trader (移除旧版兼容性)

// AddTraderFromDB 从数据库配置添加trader
func (tm *TraderManager) AddTraderFromDB(traderCfg *config.TraderRecord, aiModelCfg *config.AIModelConfig, exchangeCfg *config.ExchangeConfig, coinPoolURL, oiTopURL string, maxDailyLoss, maxDrawdown float64, stopTradingMinutes int, defaultCoins []string, database *config.Database, userID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.traders[traderCfg.ID]; exists {
		return fmt.Errorf("trader ID '%s' 已存在", traderCfg.ID)
	}

	// 处理交易币种列表
	var tradingCoins []string
	if traderCfg.TradingSymbols != "" {
		// 解析逗号分隔的交易币种列表
		symbols := strings.Split(traderCfg.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" {
				tradingCoins = append(tradingCoins, symbol)
			}
		}
	}

	// 如果没有指定交易币种，使用默认币种
	if len(tradingCoins) == 0 {
		tradingCoins = defaultCoins
	}

	// 根据交易员配置决定是否使用信号源
	var effectiveCoinPoolURL string
	if traderCfg.UseCoinPool && coinPoolURL != "" {
		effectiveCoinPoolURL = coinPoolURL
		log.Printf("✓ 交易员 %s 启用 COIN POOL 信号源: %s", traderCfg.Name, coinPoolURL)
	}

	// 构建AutoTraderConfig
	traderConfig := trader.AutoTraderConfig{
		ID:                    traderCfg.ID,
		Name:                  traderCfg.Name,
		AIModel:               aiModelCfg.Provider, // 使用provider作为模型标识
		Exchange:              exchangeCfg.ID,      // 使用exchange ID
		BinanceAPIKey:         "",
		BinanceSecretKey:      "",
		HyperliquidPrivateKey: "",
		HyperliquidTestnet:    exchangeCfg.Testnet,
		CoinPoolAPIURL:        effectiveCoinPoolURL,
		UseQwen:               aiModelCfg.Provider == "qwen",
		DeepSeekKey:           "",
		QwenKey:               "",
		CustomAPIURL:          aiModelCfg.CustomAPIURL,    // 自定义API URL
		CustomModelName:       aiModelCfg.CustomModelName, // 自定义模型名称
		ScanInterval:          time.Duration(traderCfg.ScanIntervalMinutes) * time.Minute,
		InitialBalance:        traderCfg.InitialBalance,
		BTCETHLeverage:        traderCfg.BTCETHLeverage,
		AltcoinLeverage:       traderCfg.AltcoinLeverage,
		MaxDailyLoss:          maxDailyLoss,
		MaxDrawdown:           maxDrawdown,
		StopTradingTime:       time.Duration(stopTradingMinutes) * time.Minute,
		IsCrossMargin:         traderCfg.IsCrossMargin,
		DefaultCoins:          defaultCoins,
		TradingCoins:          tradingCoins,
		SystemPromptTemplate:  traderCfg.SystemPromptTemplate, // 系统提示词模板
		UseTradingView:        traderCfg.UseTradingView,        // TradingView信号源
	}

	// 根据交易所类型设置API密钥
	if exchangeCfg.ID == "binance" {
		traderConfig.BinanceAPIKey = exchangeCfg.APIKey
		traderConfig.BinanceSecretKey = exchangeCfg.SecretKey
	} else if exchangeCfg.ID == "bybit" {
		traderConfig.BybitAPIKey = exchangeCfg.APIKey
		traderConfig.BybitSecretKey = exchangeCfg.SecretKey
	} else if exchangeCfg.ID == "okx" {
		traderConfig.OkxAPIKey = exchangeCfg.APIKey
		traderConfig.OkxSecretKey = exchangeCfg.SecretKey
		traderConfig.OkxPassphrase = exchangeCfg.OkxPassphrase
	} else if exchangeCfg.ID == "hyperliquid" {
		traderConfig.HyperliquidPrivateKey = exchangeCfg.APIKey // hyperliquid用APIKey存储private key
		traderConfig.HyperliquidWalletAddr = exchangeCfg.HyperliquidWalletAddr
	} else if exchangeCfg.ID == "aster" {
		traderConfig.AsterUser = exchangeCfg.AsterUser
		traderConfig.AsterSigner = exchangeCfg.AsterSigner
		traderConfig.AsterPrivateKey = exchangeCfg.AsterPrivateKey
	} else if exchangeCfg.ID == "lighter" {
		traderConfig.LighterPrivateKey = exchangeCfg.LighterPrivateKey
		traderConfig.LighterWalletAddr = exchangeCfg.LighterWalletAddr
		traderConfig.LighterAPIKeyPrivateKey = exchangeCfg.LighterAPIKeyPrivateKey
		traderConfig.LighterTestnet = exchangeCfg.Testnet
	}

	// 根据AI模型设置API密钥
	if aiModelCfg.Provider == "qwen" {
		traderConfig.QwenKey = aiModelCfg.APIKey
	} else if aiModelCfg.Provider == "deepseek" {
		traderConfig.DeepSeekKey = aiModelCfg.APIKey
	}

	// 创建trader实例
	at, err := trader.NewAutoTrader(traderConfig, database, userID)
	if err != nil {
		return fmt.Errorf("创建trader失败: %w", err)
	}

	// 设置交易复制回调函数（用于实时复制交易到跟随者）
	at.SetTradeReplicationCallback(func(traderID string, decision *dec.Decision) {
		tm.ReplicateTradeToFollowers(traderID, decision, database)
	})

	// 设置自定义prompt（如果有）
	if traderCfg.CustomPrompt != "" {
		at.SetCustomPrompt(traderCfg.CustomPrompt)
		at.SetOverrideBasePrompt(traderCfg.OverrideBasePrompt)
		if traderCfg.OverrideBasePrompt {
			log.Printf("✓ 已设置自定义交易策略prompt (覆盖基础prompt)")
		} else {
			log.Printf("✓ 已设置自定义交易策略prompt (补充基础prompt)")
		}
	}

	tm.traders[traderCfg.ID] = at
	log.Printf("✓ Trader '%s' (%s + %s) 已添加", traderCfg.Name, aiModelCfg.Provider, exchangeCfg.ID)
	return nil
}

// GetTrader 获取指定ID的trader
func (tm *TraderManager) GetTrader(id string) (*trader.AutoTrader, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	t, exists := tm.traders[id]
	if !exists {
		return nil, fmt.Errorf("trader ID '%s' 不存在", id)
	}
	return t, nil
}

// GetAllTraders 获取所有trader
func (tm *TraderManager) GetAllTraders() map[string]*trader.AutoTrader {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make(map[string]*trader.AutoTrader)
	for id, t := range tm.traders {
		result[id] = t
	}
	return result
}

// GetTraderIDs 获取所有trader ID列表
func (tm *TraderManager) GetTraderIDs() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	ids := make([]string, 0, len(tm.traders))
	for id := range tm.traders {
		ids = append(ids, id)
	}
	return ids
}

// StartAll 启动所有trader
func (tm *TraderManager) StartAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	log.Println("🚀 启动所有Trader...")
	for id, t := range tm.traders {
		go func(traderID string, at *trader.AutoTrader) {
			log.Printf("▶️  启动 %s...", at.GetName())
			if err := at.Run(); err != nil {
				log.Printf("❌ %s 运行错误: %v", at.GetName(), err)
			}
		}(id, t)
	}
}

// StartRunningTraders 启动数据库中标记为运行状态的交易员
func (tm *TraderManager) StartRunningTraders(database *config.Database) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	log.Println("🚀 检查并启动数据库中标记为运行状态的交易员...")

	// 获取所有用户
	userIDs, err := database.GetAllUsers()
	if err != nil {
		log.Printf("⚠️ 获取用户列表失败: %v", err)
		return
	}

	var startedCount int
	var skippedCount int

	// 遍历所有用户，查找标记为运行状态的交易员
	for _, userID := range userIDs {
		traders, err := database.GetTraders(userID)
		if err != nil {
			log.Printf("⚠️ 获取用户 %s 的交易员失败: %v", userID, err)
			continue
		}

		for _, traderCfg := range traders {
			// 只启动标记为运行状态的交易员
			if !traderCfg.IsRunning {
				skippedCount++
				continue
			}

			// 检查交易员是否已在内存中
			at, err := tm.GetTrader(traderCfg.ID)
			if err != nil {
				log.Printf("⚠️ 交易员 %s (%s) 不在内存中，跳过启动", traderCfg.Name, traderCfg.ID)
				skippedCount++
				continue
			}

			// 检查交易员是否已经在运行
			status := at.GetStatus()
			if isRunning, ok := status["is_running"].(bool); ok && isRunning {
				log.Printf("ℹ️ 交易员 %s (%s) 已在运行中，跳过", traderCfg.Name, traderCfg.ID)
				skippedCount++
				continue
			}

			// 启动交易员
			startedCount++
			go func(traderID string, traderName string, at *trader.AutoTrader) {
				log.Printf("▶️  启动交易员 %s (%s)...", traderName, traderID)
				if err := at.Run(); err != nil {
					log.Printf("❌ 交易员 %s (%s) 运行错误: %v", traderName, traderID, err)
					// 更新数据库状态为停止（启动失败）
					_ = database.UpdateTraderStatus(traderCfg.UserID, traderID, false)
				}
			}(traderCfg.ID, traderCfg.Name, at)
		}
	}

	log.Printf("✅ 启动完成: 已启动 %d 个交易员，跳过 %d 个", startedCount, skippedCount)
}

// StopAll 停止所有trader
func (tm *TraderManager) StopAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	log.Println("⏹  停止所有Trader...")
	for _, t := range tm.traders {
		t.Stop()
	}
}

// GetComparisonData 获取对比数据
func (tm *TraderManager) GetComparisonData() (map[string]interface{}, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	comparison := make(map[string]interface{})
	traders := make([]map[string]interface{}, 0, len(tm.traders))

	for _, t := range tm.traders {
		account, err := t.GetAccountInfo()
		if err != nil {
			continue
		}

		status := t.GetStatus()

		traders = append(traders, map[string]interface{}{
			"trader_id":       t.GetID(),
			"trader_name":     t.GetName(),
			"ai_model":        t.GetAIModel(),
			"exchange":        t.GetExchange(),
			"total_equity":    account["total_equity"],
			"total_pnl":       account["total_pnl"],
			"total_pnl_pct":   account["total_pnl_pct"],
			"position_count":  account["position_count"],
			"margin_used_pct": account["margin_used_pct"],
			"call_count":      status["call_count"],
			"is_running":      status["is_running"],
		})
	}

	comparison["traders"] = traders
	comparison["count"] = len(traders)

	return comparison, nil
}

// GetCompetitionData 获取竞赛数据（全平台所有交易员）
func (tm *TraderManager) GetCompetitionData(database *config.Database) (map[string]interface{}, error) {
	// 检查缓存是否有效（10秒内，确保运行状态及时更新）
	tm.competitionCache.mu.RLock()
	if time.Since(tm.competitionCache.timestamp) < 10*time.Second && len(tm.competitionCache.data) > 0 {
		// 返回缓存数据
		cachedData := make(map[string]interface{})
		for k, v := range tm.competitionCache.data {
			cachedData[k] = v
		}
		tm.competitionCache.mu.RUnlock()
		log.Printf("📋 返回竞赛数据缓存 (缓存时间: %.1fs)", time.Since(tm.competitionCache.timestamp).Seconds())
		return cachedData, nil
	}
	tm.competitionCache.mu.RUnlock()

	tm.mu.RLock()

	// 获取所有交易员列表
	allTraders := make([]*trader.AutoTrader, 0, len(tm.traders))
	for _, t := range tm.traders {
		allTraders = append(allTraders, t)
	}
	tm.mu.RUnlock()

	log.Printf("🔄 重新获取竞赛数据，交易员数量: %d", len(allTraders))

	// 并发获取交易员数据
	traders := tm.getConcurrentTraderData(allTraders, database)

	// 过滤掉跟随者交易员（followed_trader_id不为空的交易员）
	filteredTraders := make([]map[string]interface{}, 0, len(traders))
	filteredCount := 0
	for _, t := range traders {
		traderID, _ := t["trader_id"].(string)
		followedTraderIDRaw := t["followed_trader_id"]
		
		log.Printf("🔍 DEBUG [GetCompetitionData]: Checking trader %s, followed_trader_id type: %T, value: %v", traderID, followedTraderIDRaw, followedTraderIDRaw)
		
		// Handle different types: string, nil, or empty
		var followedTraderID string
		switch v := followedTraderIDRaw.(type) {
		case string:
			followedTraderID = v
			log.Printf("  ✓ Trader %s: followed_trader_id is string: '%s'", traderID, v)
		case nil:
			followedTraderID = ""
			log.Printf("  ⚠️ Trader %s: followed_trader_id is nil", traderID)
		default:
			// Try to convert to string
			if str, ok := v.(string); ok {
				followedTraderID = str
				log.Printf("  ✓ Trader %s: followed_trader_id converted to string: '%s'", traderID, str)
			} else {
				log.Printf("⚠️ DEBUG [GetCompetitionData]: Trader %s - followed_trader_id unexpected type: %T, value: %v, treating as empty", traderID, v, v)
				followedTraderID = ""
			}
		}
		
		if followedTraderID != "" {
			log.Printf("🚫 DEBUG [GetCompetitionData]: Filtering out follower trader: %s (followed_trader_id: '%s')", traderID, followedTraderID)
			filteredCount++
			continue
		}
		// 只包含非跟随者交易员
		log.Printf("✓ DEBUG [GetCompetitionData]: Including trader: %s (no followed_trader_id)", traderID)
		filteredTraders = append(filteredTraders, t)
	}
	traders = filteredTraders

	log.Printf("📋 过滤跟随者交易员后，剩余交易员数量: %d (已过滤: %d)", len(traders), filteredCount)

	// 按收益率排序（降序）
	sort.Slice(traders, func(i, j int) bool {
		pnlPctI, okI := traders[i]["total_pnl_pct"].(float64)
		pnlPctJ, okJ := traders[j]["total_pnl_pct"].(float64)
		if !okI {
			pnlPctI = 0
		}
		if !okJ {
			pnlPctJ = 0
		}
		return pnlPctI > pnlPctJ
	})

	// 限制返回前50名
	totalCount := len(traders)
	limit := 50
	if len(traders) > limit {
		traders = traders[:limit]
	}

	comparison := make(map[string]interface{})
	comparison["traders"] = traders
	comparison["count"] = len(traders)
	comparison["total_count"] = totalCount // 总交易员数量

	// 更新缓存
	tm.competitionCache.mu.Lock()
	tm.competitionCache.data = comparison
	tm.competitionCache.timestamp = time.Now()
	tm.competitionCache.mu.Unlock()

	return comparison, nil
}

// getConcurrentTraderData 并发获取多个交易员的数据
func (tm *TraderManager) getConcurrentTraderData(traders []*trader.AutoTrader, database *config.Database) []map[string]interface{} {
	type traderResult struct {
		index int
		data  map[string]interface{}
	}

	// 创建结果通道
	resultChan := make(chan traderResult, len(traders))

	// 并发获取每个交易员的数据
	for i, t := range traders {
		go func(index int, trader *trader.AutoTrader) {
			// 设置单个交易员的超时时间为3秒
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			// 使用通道来实现超时控制
			accountChan := make(chan map[string]interface{}, 1)
			errorChan := make(chan error, 1)

			go func() {
				account, err := trader.GetAccountInfo()
				if err != nil {
					errorChan <- err
				} else {
					accountChan <- account
				}
			}()

			status := trader.GetStatus()
			
			// 获取followed_trader_id
			followedTraderID := ""
			if database != nil {
				if fid, err := database.GetTraderFollowedTraderID(trader.GetID()); err == nil {
					followedTraderID = fid
					if fid != "" {
						log.Printf("🔍 DEBUG [getConcurrentTraderData]: Trader %s has followed_trader_id: '%s'", trader.GetID(), followedTraderID)
					}
				} else {
					log.Printf("⚠️ DEBUG [getConcurrentTraderData]: Failed to get followed_trader_id for trader %s: %v", trader.GetID(), err)
					// If error, check if trader exists - might be a new trader not yet in DB
					followedTraderID = ""
				}
			} else {
				log.Printf("⚠️ DEBUG [getConcurrentTraderData]: Database is nil, cannot get followed_trader_id for trader %s", trader.GetID())
			}
			
			var traderData map[string]interface{}

			select {
			case account := <-accountChan:
				// 成功获取账户信息
				traderData = map[string]interface{}{
					"trader_id":              trader.GetID(),
					"trader_name":            trader.GetName(),
					"ai_model":               trader.GetAIModel(),
					"exchange":               trader.GetExchange(),
					"total_equity":           account["total_equity"],
					"total_pnl":              account["total_pnl"],
					"total_pnl_pct":          account["total_pnl_pct"],
					"position_count":         account["position_count"],
					"margin_used_pct":        account["margin_used_pct"],
					"is_running":             status["is_running"],
					"system_prompt_template": trader.GetSystemPromptTemplate(),
					"followed_trader_id":     followedTraderID,
				}
			case err := <-errorChan:
				// 获取账户信息失败
				log.Printf("⚠️ 获取交易员 %s 账户信息失败: %v", trader.GetID(), err)
				traderData = map[string]interface{}{
					"trader_id":              trader.GetID(),
					"trader_name":            trader.GetName(),
					"ai_model":               trader.GetAIModel(),
					"exchange":               trader.GetExchange(),
					"total_equity":           0.0,
					"total_pnl":              0.0,
					"total_pnl_pct":          0.0,
					"position_count":         0,
					"margin_used_pct":        0.0,
					"is_running":             status["is_running"],
					"system_prompt_template": trader.GetSystemPromptTemplate(),
					"followed_trader_id":     followedTraderID,
					"error":                  "账户数据获取失败",
				}
			case <-ctx.Done():
				// 超时
				log.Printf("⏰ 获取交易员 %s 账户信息超时", trader.GetID())
				traderData = map[string]interface{}{
					"trader_id":              trader.GetID(),
					"trader_name":            trader.GetName(),
					"ai_model":               trader.GetAIModel(),
					"exchange":               trader.GetExchange(),
					"total_equity":           0.0,
					"total_pnl":              0.0,
					"total_pnl_pct":          0.0,
					"position_count":         0,
					"margin_used_pct":        0.0,
					"is_running":             status["is_running"],
					"system_prompt_template": trader.GetSystemPromptTemplate(),
					"followed_trader_id":     followedTraderID,
					"error":                  "获取超时",
				}
			}

			resultChan <- traderResult{index: index, data: traderData}
		}(i, t)
	}

	// 收集所有结果
	results := make([]map[string]interface{}, len(traders))
	for i := 0; i < len(traders); i++ {
		result := <-resultChan
		results[result.index] = result.data
	}

	return results
}

// GetTopTradersData 获取前5名交易员数据（用于表现对比）
func (tm *TraderManager) GetTopTradersData(database *config.Database) (map[string]interface{}, error) {
	// 复用竞赛数据缓存，因为前5名是从全部数据中筛选出来的
	competitionData, err := tm.GetCompetitionData(database)
	if err != nil {
		return nil, err
	}

	// 从竞赛数据中提取前5名
	allTraders, ok := competitionData["traders"].([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("竞赛数据格式错误")
	}

	// 限制返回前5名
	limit := 5
	topTraders := allTraders
	if len(allTraders) > limit {
		topTraders = allTraders[:limit]
	}

	result := map[string]interface{}{
		"traders": topTraders,
		"count":   len(topTraders),
	}

	return result, nil
}

// isUserTrader 检查trader是否属于指定用户
func isUserTrader(traderID, userID string) bool {
	// trader ID格式: userID_traderName 或 randomUUID_modelName
	// 为了兼容性，我们检查前缀
	if len(traderID) >= len(userID) && traderID[:len(userID)] == userID {
		return true
	}
	// 对于老的default用户，所有没有明确用户前缀的都属于default
	if userID == "default" && !containsUserPrefix(traderID) {
		return true
	}
	return false
}

// containsUserPrefix 检查trader ID是否包含用户前缀
func containsUserPrefix(traderID string) bool {
	// 检查是否包含邮箱格式的前缀（user@example.com_traderName）
	for i, ch := range traderID {
		if ch == '@' {
			// 找到@符号，说明可能是email前缀
			return true
		}
		if ch == '_' && i > 0 {
			// 找到下划线但前面没有@，可能是UUID或其他格式
			break
		}
	}
	return false
}

// LoadUserTraders 为特定用户加载交易员到内存
func (tm *TraderManager) LoadUserTraders(database *config.Database, userID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 获取指定用户的所有交易员
	traders, err := database.GetTraders(userID)
	if err != nil {
		return fmt.Errorf("获取用户 %s 的交易员列表失败: %w", userID, err)
	}

	log.Printf("📋 为用户 %s 加载交易员配置: %d 个", userID, len(traders))

	// 获取系统配置（不包含信号源，信号源现在为用户级别）
	maxDailyLossStr, _ := database.GetSystemConfig("max_daily_loss")
	maxDrawdownStr, _ := database.GetSystemConfig("max_drawdown")
	stopTradingMinutesStr, _ := database.GetSystemConfig("stop_trading_minutes")
	defaultCoinsStr, _ := database.GetSystemConfig("default_coins")

	// 获取用户信号源配置
	var coinPoolURL, oiTopURL string
	if userSignalSource, err := database.GetUserSignalSource(userID); err == nil {
		coinPoolURL = userSignalSource.CoinPoolURL
		oiTopURL = userSignalSource.OITopURL
		log.Printf("📡 加载用户 %s 的信号源配置: COIN POOL=%s, OI TOP=%s", userID, coinPoolURL, oiTopURL)
	} else {
		log.Printf("🔍 用户 %s 暂未配置信号源", userID)
	}

	// 解析配置
	maxDailyLoss := 10.0 // 默认值
	if val, err := strconv.ParseFloat(maxDailyLossStr, 64); err == nil {
		maxDailyLoss = val
	}

	maxDrawdown := 20.0 // 默认值
	if val, err := strconv.ParseFloat(maxDrawdownStr, 64); err == nil {
		maxDrawdown = val
	}

	stopTradingMinutes := 60 // 默认值
	if val, err := strconv.Atoi(stopTradingMinutesStr); err == nil {
		stopTradingMinutes = val
	}

	// 解析默认币种列表
	var defaultCoins []string
	if defaultCoinsStr != "" {
		if err := json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins); err != nil {
			log.Printf("⚠️ 解析默认币种配置失败: %v，使用空列表", err)
			defaultCoins = []string{}
		}
	}

	// 🔧 性能优化：在循环外只查询一次AI模型和交易所配置
	// 避免在循环中重复查询相同的数据，减少数据库压力和锁持有时间
	aiModels, err := database.GetAIModels(userID)
	if err != nil {
		log.Printf("⚠️ 获取用户 %s 的AI模型配置失败: %v", userID, err)
		return fmt.Errorf("获取AI模型配置失败: %w", err)
	}

	exchanges, err := database.GetExchanges(userID)
	if err != nil {
		log.Printf("⚠️ 获取用户 %s 的交易所配置失败: %v", userID, err)
		return fmt.Errorf("获取交易所配置失败: %w", err)
	}

	// 为每个交易员加载配置
	for _, traderCfg := range traders {
		// 检查是否已经加载过这个交易员
		if existingTrader, exists := tm.traders[traderCfg.ID]; exists {
			// Check if critical config changed
			existingStatus := existingTrader.GetStatus()
			existingUseTV, _ := existingStatus["use_tradingview"].(bool)
			if existingUseTV != traderCfg.UseTradingView {
				log.Printf("⚠️ 交易员 %s 配置已变更 (UseTradingView: %v -> %v)，强制重新加载", 
					traderCfg.Name, existingUseTV, traderCfg.UseTradingView)
				// Stop and reload
				if isRunning, ok := existingStatus["is_running"].(bool); ok && isRunning {
					existingTrader.Stop()
					time.Sleep(200 * time.Millisecond)
				}
				delete(tm.traders, traderCfg.ID)
				// Continue to load below...
			} else {
				log.Printf("⚠️ 交易员 %s 已经加载，跳过", traderCfg.Name)
				continue
			}
		}

		// 从已查询的列表中查找AI模型配置

		var aiModelCfg *config.AIModelConfig
		// 优先精确匹配 model.ID（新版逻辑）
		for _, model := range aiModels {
			if model.ID == traderCfg.AIModelID {
				aiModelCfg = model
				break
			}
		}
		// 如果没有精确匹配，尝试匹配 provider（兼容旧数据）
		if aiModelCfg == nil {
			for _, model := range aiModels {
				if model.Provider == traderCfg.AIModelID {
					aiModelCfg = model
					log.Printf("⚠️  交易员 %s 使用旧版 provider 匹配: %s -> %s", traderCfg.Name, traderCfg.AIModelID, model.ID)
					break
				}
			}
		}

		if aiModelCfg == nil {
			log.Printf("⚠️ 交易员 %s 的AI模型 %s 不存在，跳过", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		if !aiModelCfg.Enabled {
			log.Printf("⚠️ 交易员 %s 的AI模型 %s 未启用，跳过", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		// 从已查询的列表中查找交易所配置
		var exchangeCfg *config.ExchangeConfig
		for _, exchange := range exchanges {
			if exchange.ID == traderCfg.ExchangeID {
				exchangeCfg = exchange
				break
			}
		}

		if exchangeCfg == nil {
			log.Printf("⚠️ 交易员 %s 的交易所 %s 不存在，跳过", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		if !exchangeCfg.Enabled {
			log.Printf("⚠️ 交易员 %s 的交易所 %s 未启用，跳过", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		// 使用现有的方法加载交易员
		err = tm.loadSingleTrader(traderCfg, aiModelCfg, exchangeCfg, coinPoolURL, oiTopURL, maxDailyLoss, maxDrawdown, stopTradingMinutes, defaultCoins, database, userID)
		if err != nil {
			log.Printf("⚠️ 加载交易员 %s 失败: %v", traderCfg.Name, err)
		}
	}

	return nil
}

// LoadTraderByID 加载指定ID的单个交易员到内存
// 此方法会自动查询所需的所有配置（AI模型、交易所、系统配置等）
// 参数:
//   - database: 数据库实例
//   - userID: 用户ID
//   - traderID: 交易员ID
//
// 返回:
//   - error: 如果交易员不存在、配置无效或加载失败则返回错误
func (tm *TraderManager) LoadTraderByID(database *config.Database, userID, traderID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 1. 检查是否已加载
	if _, exists := tm.traders[traderID]; exists {
		log.Printf("⚠️ 交易员 %s 已经加载，跳过", traderID)
		return nil
	}

	// 2. 查询交易员配置
	traders, err := database.GetTraders(userID)
	if err != nil {
		return fmt.Errorf("获取交易员列表失败: %w", err)
	}

	var traderCfg *config.TraderRecord
	for _, t := range traders {
		if t.ID == traderID {
			traderCfg = t
			break
		}
	}

	if traderCfg == nil {
		return fmt.Errorf("交易员 %s 不存在", traderID)
	}

	// 3. 查询AI模型配置
	aiModels, err := database.GetAIModels(userID)
	if err != nil {
		return fmt.Errorf("获取AI模型配置失败: %w", err)
	}

	var aiModelCfg *config.AIModelConfig
	// 优先精确匹配 model.ID
	for _, model := range aiModels {
		if model.ID == traderCfg.AIModelID {
			aiModelCfg = model
			break
		}
	}
	// 如果没有精确匹配，尝试匹配 provider（兼容旧数据）
	if aiModelCfg == nil {
		for _, model := range aiModels {
			if model.Provider == traderCfg.AIModelID {
				aiModelCfg = model
				log.Printf("⚠️ 交易员 %s 使用旧版 provider 匹配: %s -> %s", traderCfg.Name, traderCfg.AIModelID, model.ID)
				break
			}
		}
	}

	if aiModelCfg == nil {
		return fmt.Errorf("AI模型 %s 不存在", traderCfg.AIModelID)
	}

	if !aiModelCfg.Enabled {
		return fmt.Errorf("AI模型 %s 未启用", traderCfg.AIModelID)
	}

	// 4. 查询交易所配置
	exchanges, err := database.GetExchanges(userID)
	if err != nil {
		return fmt.Errorf("获取交易所配置失败: %w", err)
	}

	var exchangeCfg *config.ExchangeConfig
	for _, exchange := range exchanges {
		if exchange.ID == traderCfg.ExchangeID {
			exchangeCfg = exchange
			break
		}
	}

	if exchangeCfg == nil {
		return fmt.Errorf("交易所 %s 不存在", traderCfg.ExchangeID)
	}

	if !exchangeCfg.Enabled {
		return fmt.Errorf("交易所 %s 未启用", traderCfg.ExchangeID)
	}

	// 5. 查询系统配置
	maxDailyLossStr, _ := database.GetSystemConfig("max_daily_loss")
	maxDrawdownStr, _ := database.GetSystemConfig("max_drawdown")
	stopTradingMinutesStr, _ := database.GetSystemConfig("stop_trading_minutes")
	defaultCoinsStr, _ := database.GetSystemConfig("default_coins")

	// 6. 查询用户信号源配置
	var coinPoolURL, oiTopURL string
	if userSignalSource, err := database.GetUserSignalSource(userID); err == nil {
		coinPoolURL = userSignalSource.CoinPoolURL
		oiTopURL = userSignalSource.OITopURL
		log.Printf("📡 加载用户 %s 的信号源配置: COIN POOL=%s, OI TOP=%s", userID, coinPoolURL, oiTopURL)
	} else {
		log.Printf("🔍 用户 %s 暂未配置信号源", userID)
	}

	// 7. 解析系统配置
	maxDailyLoss := 10.0 // 默认值
	if val, err := strconv.ParseFloat(maxDailyLossStr, 64); err == nil {
		maxDailyLoss = val
	}

	maxDrawdown := 20.0 // 默认值
	if val, err := strconv.ParseFloat(maxDrawdownStr, 64); err == nil {
		maxDrawdown = val
	}

	stopTradingMinutes := 60 // 默认值
	if val, err := strconv.Atoi(stopTradingMinutesStr); err == nil {
		stopTradingMinutes = val
	}

	// 解析默认币种列表
	var defaultCoins []string
	if defaultCoinsStr != "" {
		if err := json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins); err != nil {
			log.Printf("⚠️ 解析默认币种配置失败: %v，使用空列表", err)
			defaultCoins = []string{}
		}
	}

	// 8. 调用私有方法加载交易员
	log.Printf("📋 加载单个交易员: %s (%s)", traderCfg.Name, traderID)
	return tm.loadSingleTrader(
		traderCfg,
		aiModelCfg,
		exchangeCfg,
		coinPoolURL,
		oiTopURL,
		maxDailyLoss,
		maxDrawdown,
		stopTradingMinutes,
		defaultCoins,
		database,
		userID,
	)
}

// ReloadTraderFromDB 重新从数据库加载指定交易员（无论是否已在内存）
// 总是移除旧实例，确保最新配置（含 UseTradingView 等）生效
func (tm *TraderManager) ReloadTraderFromDB(database *config.Database, userID, traderID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if oldTrader, exists := tm.traders[traderID]; exists {
		// Stop old instance BEFORE deleting
		status := oldTrader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); ok && isRunning {
			log.Printf("⚠️ Stopping old trader instance before reload (trader=%s)", traderID)
			oldTrader.Stop()
			time.Sleep(300 * time.Millisecond) // Wait for goroutine to stop
		}
		delete(tm.traders, traderID)
		log.Printf("✓ Trader %s 已从内存中移除以重新加载", traderID)
	}

	traderCfg, aiModelCfg, exchangeCfg, err := database.GetTraderConfig(userID, traderID)
	if err != nil {
		return fmt.Errorf("获取交易员配置失败: %w", err)
	}

	// 查询系统配置
	maxDailyLossStr, _ := database.GetSystemConfig("max_daily_loss")
	maxDrawdownStr, _ := database.GetSystemConfig("max_drawdown")
	stopTradingMinutesStr, _ := database.GetSystemConfig("stop_trading_minutes")
	defaultCoinsStr, _ := database.GetSystemConfig("default_coins")

	// 查询用户信号源配置
	var coinPoolURL, oiTopURL string
	if userSignalSource, err := database.GetUserSignalSource(userID); err == nil {
		coinPoolURL = userSignalSource.CoinPoolURL
		oiTopURL = userSignalSource.OITopURL
		log.Printf("📡 重新加载用户 %s 的信号源配置: COIN POOL=%s, OI TOP=%s", userID, coinPoolURL, oiTopURL)
	} else {
		log.Printf("🔍 用户 %s 暂未配置信号源", userID)
	}

	// 解析系统配置
	maxDailyLoss := 10.0 // 默认值
	if val, err := strconv.ParseFloat(maxDailyLossStr, 64); err == nil {
		maxDailyLoss = val
	}

	maxDrawdown := 20.0 // 默认值
	if val, err := strconv.ParseFloat(maxDrawdownStr, 64); err == nil {
		maxDrawdown = val
	}

	stopTradingMinutes := 60 // 默认值
	if val, err := strconv.Atoi(stopTradingMinutesStr); err == nil {
		stopTradingMinutes = val
	}

	// 解析默认币种列表
	var defaultCoins []string
	if defaultCoinsStr != "" {
		if err := json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins); err != nil {
			log.Printf("⚠️ 解析默认币种配置失败: %v，使用空列表", err)
			defaultCoins = []string{}
		}
	}

	log.Printf("INFO: Reload trader from DB (trader=%s, UseTradingView=%v, scan_interval=%d)", traderID, traderCfg.UseTradingView, traderCfg.ScanIntervalMinutes)

	return tm.loadSingleTrader(
		traderCfg,
		aiModelCfg,
		exchangeCfg,
		coinPoolURL,
		oiTopURL,
		maxDailyLoss,
		maxDrawdown,
		stopTradingMinutes,
		defaultCoins,
		database,
		userID,
	)
}

// loadSingleTrader 加载单个交易员（从现有代码提取的公共逻辑）
func (tm *TraderManager) loadSingleTrader(traderCfg *config.TraderRecord, aiModelCfg *config.AIModelConfig, exchangeCfg *config.ExchangeConfig, coinPoolURL, oiTopURL string, maxDailyLoss, maxDrawdown float64, stopTradingMinutes int, defaultCoins []string, database *config.Database, userID string) error {
	// 处理交易币种列表
	var tradingCoins []string
	if traderCfg.TradingSymbols != "" {
		// 解析逗号分隔的交易币种列表
		symbols := strings.Split(traderCfg.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" {
				tradingCoins = append(tradingCoins, symbol)
			}
		}
	}

	// 如果没有指定交易币种，使用默认币种
	if len(tradingCoins) == 0 {
		tradingCoins = defaultCoins
	}

	// 根据交易员配置决定是否使用信号源
	var effectiveCoinPoolURL string
	if traderCfg.UseCoinPool && coinPoolURL != "" {
		effectiveCoinPoolURL = coinPoolURL
		log.Printf("✓ 交易员 %s 启用 COIN POOL 信号源: %s", traderCfg.Name, coinPoolURL)
	}

	// 构建AutoTraderConfig
	traderConfig := trader.AutoTraderConfig{
		ID:                   traderCfg.ID,
		Name:                 traderCfg.Name,
		AIModel:              aiModelCfg.Provider, // 使用provider作为模型标识
		Exchange:             exchangeCfg.ID,      // 使用exchange ID
		InitialBalance:       traderCfg.InitialBalance,
		BTCETHLeverage:       traderCfg.BTCETHLeverage,
		AltcoinLeverage:      traderCfg.AltcoinLeverage,
		ScanInterval:         time.Duration(traderCfg.ScanIntervalMinutes) * time.Minute,
		CoinPoolAPIURL:       effectiveCoinPoolURL,
		CustomAPIURL:         aiModelCfg.CustomAPIURL,    // 自定义API URL
		CustomModelName:      aiModelCfg.CustomModelName, // 自定义模型名称
		UseQwen:              aiModelCfg.Provider == "qwen",
		MaxDailyLoss:         maxDailyLoss,
		MaxDrawdown:          maxDrawdown,
		StopTradingTime:      time.Duration(stopTradingMinutes) * time.Minute,
		IsCrossMargin:        traderCfg.IsCrossMargin,
		DefaultCoins:         defaultCoins,
		TradingCoins:         tradingCoins,
		SystemPromptTemplate: traderCfg.SystemPromptTemplate, // 系统提示词模板
		UseTradingView:        traderCfg.UseTradingView,        // TradingView信号源
		HyperliquidTestnet:   exchangeCfg.Testnet,            // Hyperliquid测试网
	}

	// 根据交易所类型设置API密钥
	if exchangeCfg.ID == "binance" {
		traderConfig.BinanceAPIKey = exchangeCfg.APIKey
		traderConfig.BinanceSecretKey = exchangeCfg.SecretKey
	} else if exchangeCfg.ID == "bybit" {
		traderConfig.BybitAPIKey = exchangeCfg.APIKey
		traderConfig.BybitSecretKey = exchangeCfg.SecretKey
	} else if exchangeCfg.ID == "okx" {
		traderConfig.OkxAPIKey = exchangeCfg.APIKey
		traderConfig.OkxSecretKey = exchangeCfg.SecretKey
		traderConfig.OkxPassphrase = exchangeCfg.OkxPassphrase
	} else if exchangeCfg.ID == "hyperliquid" {
		traderConfig.HyperliquidPrivateKey = exchangeCfg.APIKey // hyperliquid用APIKey存储private key
		traderConfig.HyperliquidWalletAddr = exchangeCfg.HyperliquidWalletAddr
	} else if exchangeCfg.ID == "aster" {
		traderConfig.AsterUser = exchangeCfg.AsterUser
		traderConfig.AsterSigner = exchangeCfg.AsterSigner
		traderConfig.AsterPrivateKey = exchangeCfg.AsterPrivateKey
	} else if exchangeCfg.ID == "lighter" {
		traderConfig.LighterPrivateKey = exchangeCfg.LighterPrivateKey
		traderConfig.LighterWalletAddr = exchangeCfg.LighterWalletAddr
		traderConfig.LighterAPIKeyPrivateKey = exchangeCfg.LighterAPIKeyPrivateKey
		traderConfig.LighterTestnet = exchangeCfg.Testnet
	}

	// 根据AI模型设置API密钥
	if aiModelCfg.Provider == "qwen" {
		traderConfig.QwenKey = aiModelCfg.APIKey
	} else if aiModelCfg.Provider == "deepseek" {
		traderConfig.DeepSeekKey = aiModelCfg.APIKey
	}

	// 创建trader实例
	log.Printf("INFO: Creating AutoTrader with config (trader=%s, UseTradingView=%v, ScanInterval=%v)", traderCfg.Name, traderConfig.UseTradingView, traderConfig.ScanInterval)
	at, err := trader.NewAutoTrader(traderConfig, database, userID)
	if err != nil {
		return fmt.Errorf("创建trader失败: %w", err)
	}
	log.Printf("INFO: AutoTrader created successfully (trader=%s, id=%s, UseTradingView=%v)", traderCfg.Name, traderCfg.ID, traderConfig.UseTradingView)

	// 设置交易复制回调函数（用于实时复制交易到跟随者）
	at.SetTradeReplicationCallback(func(traderID string, decision *dec.Decision) {
		tm.ReplicateTradeToFollowers(traderID, decision, database)
	})

	// 设置自定义prompt（如果有）
	if traderCfg.CustomPrompt != "" {
		at.SetCustomPrompt(traderCfg.CustomPrompt)
		at.SetOverrideBasePrompt(traderCfg.OverrideBasePrompt)
		if traderCfg.OverrideBasePrompt {
			log.Printf("✓ 已设置自定义交易策略prompt (覆盖基础prompt)")
		} else {
			log.Printf("✓ 已设置自定义交易策略prompt (补充基础prompt)")
		}
	}

	tm.traders[traderCfg.ID] = at
	log.Printf("✓ Trader '%s' (%s + %s) 已为用户加载到内存", traderCfg.Name, aiModelCfg.Provider, exchangeCfg.ID)
	return nil
}

// RemoveTrader 从内存中移除指定的trader（不影响数据库）
// 用于更新trader配置时强制重新加载
func (tm *TraderManager) RemoveTrader(traderID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.traders[traderID]; exists {
		delete(tm.traders, traderID)
		log.Printf("✓ Trader %s 已从内存中移除", traderID)
		// 使竞赛缓存失效，确保删除的交易员不再出现在排行榜中
		tm.InvalidateCompetitionCache()
	}
}

// InvalidateCompetitionCache 使竞赛缓存失效
func (tm *TraderManager) InvalidateCompetitionCache() {
	tm.competitionCache.mu.Lock()
	defer tm.competitionCache.mu.Unlock()
	tm.competitionCache.data = make(map[string]interface{})
	tm.competitionCache.timestamp = time.Time{} // Reset timestamp to force refresh
	log.Printf("✓ DEBUG [InvalidateCompetitionCache]: Competition cache invalidated")
}

// GetFollowerTraders 获取所有跟随指定交易员的交易员实例（从内存中）
func (tm *TraderManager) GetFollowerTraders(followedTraderID string) []*trader.AutoTrader {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var followers []*trader.AutoTrader
	for _, t := range tm.traders {
		// Note: We can't check followed_trader_id from memory directly
		// This method is mainly for getting instances, actual filtering happens in ReplicateTradeToFollowers
		followers = append(followers, t)
	}
	return followers
}

// ReplicateTradeToFollowers 将交易信号发送到所有跟随者（AI分析模式）
func (tm *TraderManager) ReplicateTradeToFollowers(followedTraderID string, decision *dec.Decision, database *config.Database) {
	// Get all follower traders from database
	followerRecords, err := database.GetFollowerTraders(followedTraderID)
	if err != nil {
		log.Printf("⚠️ [%s] 获取跟随者列表失败: %v", followedTraderID, err)
		return
	}

	if len(followerRecords) == 0 {
		return // No followers
	}

	log.Printf("📋 [%s] 发现 %d 个跟随者，发送交易信号 (%s %s)...", followedTraderID, len(followerRecords), decision.Symbol, decision.Action)

	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// Get parent trader info for signal context
	parentTrader, exists := tm.traders[followedTraderID]
	if !exists {
		log.Printf("⚠️ [%s] 父交易员不在内存中，无法获取账户信息", followedTraderID)
		return
	}

	// Get parent trader account info
	parentAccount, err := parentTrader.GetAccountInfo()
	if err != nil {
		log.Printf("⚠️ [%s] 获取父交易员账户信息失败: %v", followedTraderID, err)
		return
	}

	parentEquity := 0.0
	if equity, ok := parentAccount["total_equity"].(float64); ok {
		parentEquity = equity
	}

	// Get parent trader initial balance from database
	parentInitialBalance := 0.0
	if allUserIDs, err := database.GetAllUsers(); err == nil {
		for _, uid := range allUserIDs {
			traders, err := database.GetTraders(uid)
			if err != nil {
				continue
			}
			for _, traderCfg := range traders {
				if traderCfg.ID == followedTraderID {
					parentInitialBalance = traderCfg.InitialBalance
					break
				}
			}
			if parentInitialBalance > 0 {
				break
			}
		}
	}

	// Create parent trade signal
	signal := &trader.ParentTradeSignal{
		ParentTraderID:       followedTraderID,
		ParentTraderName:     parentTrader.GetName(),
		SignalID:             uuid.New().String(),
		Timestamp:            time.Now(),
		Decision:             decision,
		ParentEquity:         parentEquity,
		ParentInitialBalance: parentInitialBalance,
	}

	// Send signal to each follower trader
	startTime := time.Now()
	successCount := 0
	skipCount := 0
	errorCount := 0
	var skippedFollowers []string
	var errorFollowers []string

	for _, followerRecord := range followerRecords {
		followerTrader, exists := tm.traders[followerRecord.ID]
		if !exists {
			log.Printf("⚠️ [%s] 跟随者 %s (%s) 未在内存中，跳过", followedTraderID, followerRecord.Name, followerRecord.ID)
			skipCount++
			skippedFollowers = append(skippedFollowers, fmt.Sprintf("%s (not in memory)", followerRecord.Name))
			continue
		}

		// Check if follower is running
		status := followerTrader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); !ok || !isRunning {
			log.Printf("⚠️ [%s] 跟随者 %s (%s) 未运行，跳过", followedTraderID, followerRecord.Name, followerRecord.ID)
			skipCount++
			skippedFollowers = append(skippedFollowers, fmt.Sprintf("%s (not running)", followerRecord.Name))
			continue
		}

		// Send signal to follower trader (non-blocking)
		// Deep copy the decision to avoid race conditions
		decisionCopy := &dec.Decision{
			Symbol:         signal.Decision.Symbol,
			Action:         signal.Decision.Action,
			Leverage:       signal.Decision.Leverage,
			PositionSizeUSD: signal.Decision.PositionSizeUSD,
			StopLoss:       signal.Decision.StopLoss,
			TakeProfit:     signal.Decision.TakeProfit,
			Reasoning:      signal.Decision.Reasoning,
			Confidence:     signal.Decision.Confidence,
		}
		
		signalCopy := &trader.ParentTradeSignal{
			ParentTraderID:       signal.ParentTraderID,
			ParentTraderName:     signal.ParentTraderName,
			SignalID:             uuid.New().String(), // Unique signal ID for each follower
			Timestamp:            time.Now(),
			Decision:             decisionCopy,
			ParentEquity:         signal.ParentEquity,
			ParentInitialBalance: signal.ParentInitialBalance,
		}
		
		if err := followerTrader.TriggerParentTradeSignal(signalCopy); err != nil {
			log.Printf("❌ [%s] 发送交易信号到跟随者 %s (%s) 失败: %v", 
				followedTraderID, followerRecord.Name, followerRecord.ID, err)
			errorCount++
			errorFollowers = append(errorFollowers, fmt.Sprintf("%s (%v)", followerRecord.Name, err))
		} else {
			log.Printf("✓ [%s] 成功发送交易信号到跟随者 %s (%s) (signal_id=%s)", 
				followedTraderID, followerRecord.Name, followerRecord.ID, signalCopy.SignalID)
			successCount++
		}
	}

	// Summary log
	duration := time.Since(startTime)
	log.Printf("📊 [%s] 信号分发完成: 成功=%d, 跳过=%d, 失败=%d, 总计=%d, 耗时=%v", 
		followedTraderID, successCount, skipCount, errorCount, len(followerRecords), duration)
	
	if len(skippedFollowers) > 0 {
		log.Printf("⚠️ [%s] 跳过的跟随者: %v", followedTraderID, skippedFollowers)
	}
	if len(errorFollowers) > 0 {
		log.Printf("❌ [%s] 失败的跟随者: %v", followedTraderID, errorFollowers)
	}
}

// replicateTradeToFollower 将交易复制到单个跟随者（使用跟随者自己的风险管理）
func (tm *TraderManager) replicateTradeToFollower(follower *trader.AutoTrader, followerID string, followerRecord *config.TraderRecord, originalDecision *dec.Decision) error {
	// Create a copy of the decision for the follower
	followerDecision := *originalDecision

	// Get follower's current balance
	balance, err := follower.GetAccountInfo()
	if err != nil {
		return fmt.Errorf("获取跟随者余额失败: %w", err)
	}

	totalEquity := 0.0
	if equity, ok := balance["total_equity"].(float64); ok {
		totalEquity = equity
	}

	// Use follower's initial balance to calculate scaling ratio
	followerInitialBalance := followerRecord.InitialBalance
	if followerInitialBalance <= 0 {
		return fmt.Errorf("跟随者初始余额无效: %.2f", followerInitialBalance)
	}

	// Calculate balance ratio (current equity / initial balance)
	// This represents how the follower's account has performed
	balanceRatio := totalEquity / followerInitialBalance
	
	// Cap scaling at reasonable limits to prevent over-leveraging
	if balanceRatio > 1.5 {
		balanceRatio = 1.5 // Cap at 150%
	}
	if balanceRatio < 0.1 {
		balanceRatio = 0.1 // Minimum 10% to ensure meaningful position
	}

	// Scale position size proportionally
	// If original trader used X% of their balance, follower uses same X% of their (scaled) balance
	followerDecision.PositionSizeUSD = originalDecision.PositionSizeUSD * balanceRatio

	// Use follower's own leverage settings based on symbol type
	if strings.Contains(strings.ToUpper(originalDecision.Symbol), "BTC") || 
	   strings.Contains(strings.ToUpper(originalDecision.Symbol), "ETH") {
		followerDecision.Leverage = followerRecord.BTCETHLeverage
	} else {
		followerDecision.Leverage = followerRecord.AltcoinLeverage
	}

	// Execute the trade using follower's own trader instance
	actionRecord := logger.DecisionAction{
		Action:    followerDecision.Action,
		Symbol:    followerDecision.Symbol,
		Quantity:  0,
		Leverage:  followerDecision.Leverage,
		Price:     0,
		Timestamp: time.Now(),
		Success:   false,
	}

	if err := follower.ExecuteDecisionWithRecord(&followerDecision, &actionRecord); err != nil {
		return fmt.Errorf("执行跟随交易失败: %w", err)
	}

	return nil
}
