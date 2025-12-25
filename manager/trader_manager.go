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

// CompetitionCache competition data cache
type CompetitionCache struct {
	data      map[string]interface{}
	timestamp time.Time
	mu        sync.RWMutex
}

// TraderManager manages multiple trader instances
type TraderManager struct {
	traders          map[string]*trader.AutoTrader // key: trader ID
	loadErrors       map[string]error              // key: trader ID, stores last load error
	competitionCache *CompetitionCache
	mu               sync.RWMutex
}

// NewTraderManager creates a trader manager
func NewTraderManager() *TraderManager {
	return &TraderManager{
		traders:    make(map[string]*trader.AutoTrader),
		loadErrors: make(map[string]error),
		competitionCache: &CompetitionCache{
			data: make(map[string]interface{}),
		},
	}
}

// GetLoadError returns the last load error for a trader
func (tm *TraderManager) GetLoadError(traderID string) error {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.loadErrors[traderID]
}

// LoadTradersFromDatabase loads all traders from database to memory
func (tm *TraderManager) LoadTradersFromDatabase(database *config.Database) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Get all users
	userIDs, err := database.GetAllUsers()
	if err != nil {
		return fmt.Errorf("failed to get user list: %w", err)
	}

	log.Printf("📋 Found %d users, starting to load all trader configurations...", len(userIDs))

	var allTraders []*config.TraderRecord
	for _, userID := range userIDs {
		// Get traders for each user
		traders, err := database.GetTraders(userID)
		if err != nil {
			log.Printf("⚠️ Failed to get traders for user %s: %v", userID, err)
			continue
		}
		log.Printf("📋 User %s: %d traders", userID, len(traders))
		allTraders = append(allTraders, traders...)
	}

	log.Printf("📋 Total loaded %d trader configurations", len(allTraders))

	// Get system configuration (does not include signal sources, signal sources are now user-level)
	maxDailyLossStr, _ := database.GetSystemConfig("max_daily_loss")
	maxDrawdownStr, _ := database.GetSystemConfig("max_drawdown")
	stopTradingMinutesStr, _ := database.GetSystemConfig("stop_trading_minutes")
	defaultCoinsStr, _ := database.GetSystemConfig("default_coins")

	// Parse configuration
	maxDailyLoss := 10.0 // Default value
	if val, err := strconv.ParseFloat(maxDailyLossStr, 64); err == nil {
		maxDailyLoss = val
	}

	maxDrawdown := 20.0 // Default value
	if val, err := strconv.ParseFloat(maxDrawdownStr, 64); err == nil {
		maxDrawdown = val
	}

	stopTradingMinutes := 60 // Default value
	if val, err := strconv.Atoi(stopTradingMinutesStr); err == nil {
		stopTradingMinutes = val
	}

	// Parse default coin list
	var defaultCoins []string
	if defaultCoinsStr != "" {
		if err := json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins); err != nil {
			log.Printf("⚠️ Failed to parse default coin configuration: %v, using empty list", err)
			defaultCoins = []string{}
		}
	}

	// Get AI model and exchange configuration for each trader
	for _, traderCfg := range allTraders {
		// Get AI model configuration (using trader's user ID)
		aiModels, err := database.GetAIModels(traderCfg.UserID)
		if err != nil {
			log.Printf("⚠️  Failed to get AI model configuration: %v", err)
			continue
		}

		var aiModelCfg *config.AIModelConfig
		// Prioritize exact match on model.ID (new logic)
		for _, model := range aiModels {
			if model.ID == traderCfg.AIModelID {
				aiModelCfg = model
				break
			}
		}
		// If no exact match, try matching provider (compatible with old data)
		if aiModelCfg == nil {
			for _, model := range aiModels {
				if model.Provider == traderCfg.AIModelID {
					aiModelCfg = model
					log.Printf("⚠️  Trader %s using old provider match: %s -> %s", traderCfg.Name, traderCfg.AIModelID, model.ID)
					break
				}
			}
		}

		if aiModelCfg == nil {
			log.Printf("⚠️  Trader %s's AI model %s does not exist, skipping", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		if !aiModelCfg.Enabled {
			log.Printf("⚠️  Trader %s's AI model %s is not enabled, skipping", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		// Get exchange configuration (using trader's user ID)
		exchanges, err := database.GetExchanges(traderCfg.UserID)
		if err != nil {
			log.Printf("⚠️  Failed to get exchange configuration: %v", err)
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
			log.Printf("⚠️  Trader %s's exchange %s does not exist, skipping", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		if !exchangeCfg.Enabled {
			log.Printf("⚠️  Trader %s's exchange %s is not enabled, skipping", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		// Get user signal source configuration
		var coinPoolURL, oiTopURL string
		if userSignalSource, err := database.GetUserSignalSource(traderCfg.UserID); err == nil {
			coinPoolURL = userSignalSource.CoinPoolURL
			oiTopURL = userSignalSource.OITopURL
		} else {
			// If user has not configured signal source, use empty string
			log.Printf("🔍 User %s has not configured signal source yet", traderCfg.UserID)
		}

		// Add to TraderManager
		err = tm.addTraderFromDB(traderCfg, aiModelCfg, exchangeCfg, coinPoolURL, oiTopURL, maxDailyLoss, maxDrawdown, stopTradingMinutes, defaultCoins, database, traderCfg.UserID)
		if err != nil {
			log.Printf("❌ Failed to add trader %s: %v", traderCfg.Name, err)
			continue
		}
	}

	log.Printf("✓ Successfully loaded %d traders to memory", len(tm.traders))
	return nil
}

// addTraderFromConfig internal method: add trader from configuration (no lock, caller already locked)
func (tm *TraderManager) addTraderFromDB(traderCfg *config.TraderRecord, aiModelCfg *config.AIModelConfig, exchangeCfg *config.ExchangeConfig, coinPoolURL, oiTopURL string, maxDailyLoss, maxDrawdown float64, stopTradingMinutes int, defaultCoins []string, database *config.Database, userID string) error {
	if _, exists := tm.traders[traderCfg.ID]; exists {
		return fmt.Errorf("trader ID '%s' already exists", traderCfg.ID)
	}

	// Process trading symbol list
	var tradingCoins []string
	if traderCfg.TradingSymbols != "" {
		// Parse comma-separated trading symbol list
		symbols := strings.Split(traderCfg.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" {
				tradingCoins = append(tradingCoins, symbol)
			}
		}
	}

	// If no trading symbols specified, use default coins
	if len(tradingCoins) == 0 {
		tradingCoins = defaultCoins
	}

	// Decide whether to use signal source based on trader configuration
	var effectiveCoinPoolURL string
	if traderCfg.UseCoinPool && coinPoolURL != "" {
		effectiveCoinPoolURL = coinPoolURL
		log.Printf("✓ Trader %s enabled COIN POOL signal source: %s", traderCfg.Name, coinPoolURL)
	}

	// Build AutoTraderConfig
	traderConfig := trader.AutoTraderConfig{
		ID:                    traderCfg.ID,
		Name:                  traderCfg.Name,
		AIModel:               aiModelCfg.Provider, // Use provider as model identifier
		Exchange:              exchangeCfg.ID,      // Use exchange ID
		BinanceAPIKey:         "",
		BinanceSecretKey:      "",
		HyperliquidPrivateKey: "",
		HyperliquidTestnet:    exchangeCfg.Testnet,
		CoinPoolAPIURL:        effectiveCoinPoolURL,
		UseQwen:               aiModelCfg.Provider == "qwen",
		DeepSeekKey:           "",
		QwenKey:               "",
		CustomAPIURL:          aiModelCfg.CustomAPIURL,    // Custom API URL
		CustomModelName:       aiModelCfg.CustomModelName, // Custom model name
		ScanInterval:          time.Duration(traderCfg.ScanIntervalMinutes) * time.Minute,
		InitialBalance:        traderCfg.InitialBalance,
		BTCETHLeverage:        traderCfg.BTCETHLeverage,
		AltcoinLeverage:       traderCfg.AltcoinLeverage,
		MaxDailyLoss:          maxDailyLoss,
		MaxDrawdown:           maxDrawdown,
		StopTradingTime:       time.Duration(stopTradingMinutes) * time.Minute,
		IsCrossMargin:         traderCfg.IsCrossMargin,
		ShowInCompetition:     traderCfg.ShowInCompetition,
		DefaultCoins:          defaultCoins,
		TradingCoins:          tradingCoins,
		SystemPromptTemplate:  traderCfg.SystemPromptTemplate, // System prompt template
		UseTradingView:        traderCfg.UseTradingView,       // TradingView signal source

		// Indicator configuration
		EnableRawKlines:    traderCfg.EnableRawKlines,    // Raw OHLCV klines
		EnableEMA:          traderCfg.EnableEMA,          // Enable EMA indicator
		EnableMACD:         traderCfg.EnableMACD,         // Enable MACD indicator
		EnableRSI:          traderCfg.EnableRSI,          // Enable RSI indicator
		EnableATR:          traderCfg.EnableATR,          // Enable ATR indicator
		EnableVolume:       traderCfg.EnableVolume,       // Enable volume data
		EnableOI:           traderCfg.EnableOI,           // Enable open interest data
		EnableFunding:      traderCfg.EnableFunding,      // Enable funding rate data
		IndicatorTimeframe: traderCfg.IndicatorTimeframe, // Timeframe for indicators
	}

	// Set API keys based on exchange type
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
	} else if exchangeCfg.ID == "bitget" {
		traderConfig.BitgetAPIKey = exchangeCfg.APIKey
		traderConfig.BitgetSecretKey = exchangeCfg.SecretKey
		traderConfig.BitgetPassphrase = exchangeCfg.OkxPassphrase // Reuse passphrase field
	} else if exchangeCfg.ID == "hyperliquid" {
		traderConfig.HyperliquidPrivateKey = exchangeCfg.APIKey // hyperliquid uses APIKey to store private key
		traderConfig.HyperliquidWalletAddr = exchangeCfg.HyperliquidWalletAddr
	} else if exchangeCfg.ID == "aster" {
		traderConfig.AsterUser = exchangeCfg.AsterUser
		traderConfig.AsterSigner = exchangeCfg.AsterSigner
		traderConfig.AsterPrivateKey = exchangeCfg.AsterPrivateKey
	} else if exchangeCfg.ID == "lighter" {
		traderConfig.LighterWalletAddr = exchangeCfg.LighterWalletAddr
		traderConfig.LighterAPIKeyPrivateKey = exchangeCfg.LighterAPIKeyPrivateKey
		traderConfig.LighterAPIKeyIndex = exchangeCfg.LighterAPIKeyIndex
		traderConfig.LighterTestnet = exchangeCfg.Testnet
	}

	// Set API keys based on AI model
	if aiModelCfg.Provider == "qwen" {
		traderConfig.QwenKey = aiModelCfg.APIKey
	} else if aiModelCfg.Provider == "deepseek" {
		traderConfig.DeepSeekKey = aiModelCfg.APIKey
	} else {
		// For other providers (grok, openai, claude, gemini, kimi, custom), use CustomAPIKey
		traderConfig.CustomAPIKey = aiModelCfg.APIKey
	}

	// Create trader instance
	at, err := trader.NewAutoTrader(traderConfig, database, userID)
	if err != nil {
		return fmt.Errorf("failed to create trader: %w", err)
	}

	// Set trade replication callback function (for real-time trade replication to followers)
	at.SetTradeReplicationCallback(func(traderID string, decision *dec.Decision) {
		tm.ReplicateTradeToFollowers(traderID, decision, database)
	})

	// Set custom prompt (if any)
	if traderCfg.CustomPrompt != "" {
		at.SetCustomPrompt(traderCfg.CustomPrompt)
		at.SetOverrideBasePrompt(traderCfg.OverrideBasePrompt)
		if traderCfg.OverrideBasePrompt {
			log.Printf("✓ Custom trading strategy prompt set (override base prompt)")
		} else {
			log.Printf("✓ Custom trading strategy prompt set (supplement base prompt)")
		}
	}

	tm.traders[traderCfg.ID] = at
	log.Printf("✓ Trader '%s' (%s + %s) loaded to memory", traderCfg.Name, aiModelCfg.Provider, exchangeCfg.ID)
	return nil
}

// AddTrader adds trader from database configuration (removed old version compatibility)

// AddTraderFromDB adds trader from database configuration
func (tm *TraderManager) AddTraderFromDB(traderCfg *config.TraderRecord, aiModelCfg *config.AIModelConfig, exchangeCfg *config.ExchangeConfig, coinPoolURL, oiTopURL string, maxDailyLoss, maxDrawdown float64, stopTradingMinutes int, defaultCoins []string, database *config.Database, userID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.traders[traderCfg.ID]; exists {
		return fmt.Errorf("trader ID '%s' already exists", traderCfg.ID)
	}

	// Process trading symbol list
	var tradingCoins []string
	if traderCfg.TradingSymbols != "" {
		// Parse comma-separated trading symbol list
		symbols := strings.Split(traderCfg.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" {
				tradingCoins = append(tradingCoins, symbol)
			}
		}
	}

	// If no trading symbols specified, use default coins
	if len(tradingCoins) == 0 {
		tradingCoins = defaultCoins
	}

	// Decide whether to use signal source based on trader configuration
	var effectiveCoinPoolURL string
	if traderCfg.UseCoinPool && coinPoolURL != "" {
		effectiveCoinPoolURL = coinPoolURL
		log.Printf("✓ Trader %s enabled COIN POOL signal source: %s", traderCfg.Name, coinPoolURL)
	}

	// Build AutoTraderConfig
	traderConfig := trader.AutoTraderConfig{
		ID:                    traderCfg.ID,
		Name:                  traderCfg.Name,
		AIModel:               aiModelCfg.Provider, // Use provider as model identifier
		Exchange:              exchangeCfg.ID,      // Use exchange ID
		BinanceAPIKey:         "",
		BinanceSecretKey:      "",
		HyperliquidPrivateKey: "",
		HyperliquidTestnet:    exchangeCfg.Testnet,
		CoinPoolAPIURL:        effectiveCoinPoolURL,
		UseQwen:               aiModelCfg.Provider == "qwen",
		DeepSeekKey:           "",
		QwenKey:               "",
		CustomAPIURL:          aiModelCfg.CustomAPIURL,    // Custom API URL
		CustomModelName:       aiModelCfg.CustomModelName, // Custom model name
		ScanInterval:          time.Duration(traderCfg.ScanIntervalMinutes) * time.Minute,
		InitialBalance:        traderCfg.InitialBalance,
		BTCETHLeverage:        traderCfg.BTCETHLeverage,
		AltcoinLeverage:       traderCfg.AltcoinLeverage,
		MaxDailyLoss:          maxDailyLoss,
		MaxDrawdown:           maxDrawdown,
		StopTradingTime:       time.Duration(stopTradingMinutes) * time.Minute,
		IsCrossMargin:         traderCfg.IsCrossMargin,
		ShowInCompetition:     traderCfg.ShowInCompetition,
		DefaultCoins:          defaultCoins,
		TradingCoins:          tradingCoins,
		SystemPromptTemplate:  traderCfg.SystemPromptTemplate, // System prompt template
		UseTradingView:        traderCfg.UseTradingView,       // TradingView signal source

		// Indicator configuration
		EnableRawKlines:    traderCfg.EnableRawKlines,    // Raw OHLCV klines
		EnableEMA:          traderCfg.EnableEMA,          // Enable EMA indicator
		EnableMACD:         traderCfg.EnableMACD,         // Enable MACD indicator
		EnableRSI:          traderCfg.EnableRSI,          // Enable RSI indicator
		EnableATR:          traderCfg.EnableATR,          // Enable ATR indicator
		EnableVolume:       traderCfg.EnableVolume,       // Enable volume data
		EnableOI:           traderCfg.EnableOI,           // Enable open interest data
		EnableFunding:      traderCfg.EnableFunding,      // Enable funding rate data
		IndicatorTimeframe: traderCfg.IndicatorTimeframe, // Timeframe for indicators
	}

	// Set API keys based on exchange type
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
	} else if exchangeCfg.ID == "bitget" {
		traderConfig.BitgetAPIKey = exchangeCfg.APIKey
		traderConfig.BitgetSecretKey = exchangeCfg.SecretKey
		traderConfig.BitgetPassphrase = exchangeCfg.OkxPassphrase // Reuse passphrase field
	} else if exchangeCfg.ID == "hyperliquid" {
		traderConfig.HyperliquidPrivateKey = exchangeCfg.APIKey // hyperliquid uses APIKey to store private key
		traderConfig.HyperliquidWalletAddr = exchangeCfg.HyperliquidWalletAddr
	} else if exchangeCfg.ID == "aster" {
		traderConfig.AsterUser = exchangeCfg.AsterUser
		traderConfig.AsterSigner = exchangeCfg.AsterSigner
		traderConfig.AsterPrivateKey = exchangeCfg.AsterPrivateKey
	} else if exchangeCfg.ID == "lighter" {
		traderConfig.LighterWalletAddr = exchangeCfg.LighterWalletAddr
		traderConfig.LighterAPIKeyPrivateKey = exchangeCfg.LighterAPIKeyPrivateKey
		traderConfig.LighterAPIKeyIndex = exchangeCfg.LighterAPIKeyIndex
		traderConfig.LighterTestnet = exchangeCfg.Testnet
	}

	// Set API keys based on AI model
	if aiModelCfg.Provider == "qwen" {
		traderConfig.QwenKey = aiModelCfg.APIKey
	} else if aiModelCfg.Provider == "deepseek" {
		traderConfig.DeepSeekKey = aiModelCfg.APIKey
	} else {
		// For other providers (grok, openai, claude, gemini, kimi, custom), use CustomAPIKey
		traderConfig.CustomAPIKey = aiModelCfg.APIKey
	}

	// Create trader instance
	at, err := trader.NewAutoTrader(traderConfig, database, userID)
	if err != nil {
		return fmt.Errorf("failed to create trader: %w", err)
	}

	// Set trade replication callback function (for real-time trade replication to followers)
	at.SetTradeReplicationCallback(func(traderID string, decision *dec.Decision) {
		tm.ReplicateTradeToFollowers(traderID, decision, database)
	})

	// Set custom prompt (if any)
	if traderCfg.CustomPrompt != "" {
		at.SetCustomPrompt(traderCfg.CustomPrompt)
		at.SetOverrideBasePrompt(traderCfg.OverrideBasePrompt)
		if traderCfg.OverrideBasePrompt {
			log.Printf("✓ Custom trading strategy prompt set (override base prompt)")
		} else {
			log.Printf("✓ Custom trading strategy prompt set (supplement base prompt)")
		}
	}

	tm.traders[traderCfg.ID] = at
	log.Printf("✓ Trader '%s' (%s + %s) added", traderCfg.Name, aiModelCfg.Provider, exchangeCfg.ID)
	return nil
}

// GetTrader retrieves a trader by ID
func (tm *TraderManager) GetTrader(id string) (*trader.AutoTrader, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	t, exists := tm.traders[id]
	if !exists {
		return nil, fmt.Errorf("trader ID '%s' does not exist", id)
	}
	return t, nil
}

// GetAllTraders retrieves all traders
func (tm *TraderManager) GetAllTraders() map[string]*trader.AutoTrader {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make(map[string]*trader.AutoTrader)
	for id, t := range tm.traders {
		result[id] = t
	}
	return result
}

// GetTraderIDs retrieves all trader ID list
func (tm *TraderManager) GetTraderIDs() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	ids := make([]string, 0, len(tm.traders))
	for id := range tm.traders {
		ids = append(ids, id)
	}
	return ids
}

// StartAll starts all traders
func (tm *TraderManager) StartAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	log.Println("🚀 Starting all traders...")
	for id, t := range tm.traders {
		go func(traderID string, at *trader.AutoTrader) {
			log.Printf("▶️  Starting %s...", at.GetName())
			if err := at.Run(); err != nil {
				log.Printf("❌ %s runtime error: %v", at.GetName(), err)
			}
		}(id, t)
	}
}

// StartRunningTraders starts traders marked as running in database
func (tm *TraderManager) StartRunningTraders(database *config.Database) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	log.Println("🚀 Checking and starting traders marked as running in database...")

	// Get all users
	userIDs, err := database.GetAllUsers()
	if err != nil {
		log.Printf("⚠️ Failed to get user list: %v", err)
		return
	}

	var startedCount int
	var skippedCount int

	// Iterate through all users, find traders marked as running
	for _, userID := range userIDs {
		traders, err := database.GetTraders(userID)
		if err != nil {
			log.Printf("⚠️ Failed to get traders for user %s: %v", userID, err)
			continue
		}

		for _, traderCfg := range traders {
			// Only start traders marked as running
			if !traderCfg.IsRunning {
				skippedCount++
				continue
			}

			// Check if trader is already in memory
			at, err := tm.GetTrader(traderCfg.ID)
			if err != nil {
				log.Printf("⚠️ Trader %s (%s) not in memory, skipping start", traderCfg.Name, traderCfg.ID)
				skippedCount++
				continue
			}

			// Check if trader is already running
			status := at.GetStatus()
			if isRunning, ok := status["is_running"].(bool); ok && isRunning {
				log.Printf("ℹ️ Trader %s (%s) already running, skipping", traderCfg.Name, traderCfg.ID)
				skippedCount++
				continue
			}

			// Start trader
			startedCount++
			go func(traderID string, traderName string, at *trader.AutoTrader) {
				log.Printf("▶️  Starting trader %s (%s)...", traderName, traderID)
				if err := at.Run(); err != nil {
					log.Printf("❌ Trader %s (%s) runtime error: %v", traderName, traderID, err)
					// Update database status to stopped (start failed)
					_ = database.UpdateTraderStatus(traderCfg.UserID, traderID, false)
				}
			}(traderCfg.ID, traderCfg.Name, at)
		}
	}

	log.Printf("✅ Start completed: started %d traders, skipped %d", startedCount, skippedCount)
}

// StopAll stops all traders
func (tm *TraderManager) StopAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	log.Println("⏹  Stopping all traders...")
	for _, t := range tm.traders {
		t.Stop()
	}
}

// GetComparisonData retrieves comparison data
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

// GetCompetitionData retrieves competition data (all traders on platform)
func (tm *TraderManager) GetCompetitionData(database *config.Database) (map[string]interface{}, error) {
	// Check if cache is valid (within 10 seconds, ensure running status is updated timely)
	tm.competitionCache.mu.RLock()
	if time.Since(tm.competitionCache.timestamp) < 10*time.Second && len(tm.competitionCache.data) > 0 {
		// Return cached data
		cachedData := make(map[string]interface{})
		for k, v := range tm.competitionCache.data {
			cachedData[k] = v
		}
		tm.competitionCache.mu.RUnlock()
		log.Printf("📋 Returning competition data cache (cache age: %.1fs)", time.Since(tm.competitionCache.timestamp).Seconds())
		return cachedData, nil
	}
	tm.competitionCache.mu.RUnlock()

	tm.mu.RLock()

	// Get all trader list (only those with ShowInCompetition = true)
	allTraders := make([]*trader.AutoTrader, 0, len(tm.traders))
	for id, t := range tm.traders {
		if t.GetShowInCompetition() {
			allTraders = append(allTraders, t)
			logger.Infof("📋 Competition data includes trader: %s (%s)", t.GetName(), id)
		} else {
			logger.Infof("📋 Competition data excludes trader (hidden): %s (%s)", t.GetName(), id)
		}
	}
	tm.mu.RUnlock()

	log.Printf("🔄 Refetching competition data, trader count: %d", len(allTraders))

	// Concurrently get trader data
	traders := tm.getConcurrentTraderData(allTraders, database)

	// Filter out follower traders (traders with non-empty followed_trader_id)
	// Also count all follower traders (total and running)
	filteredTraders := make([]map[string]interface{}, 0, len(traders))
	filteredCount := 0
	followerTotalCount := 0
	followerRunningCount := 0
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
			followerTotalCount++ // Count all follower traders
			// Count running follower traders
			if runningRaw, ok := t["is_running"]; ok {
				if isRunning, ok := runningRaw.(bool); ok && isRunning {
					followerRunningCount++
				}
			}
			continue
		}
		// Only include non-follower traders
		log.Printf("✓ DEBUG [GetCompetitionData]: Including trader: %s (no followed_trader_id)", traderID)
		filteredTraders = append(filteredTraders, t)
	}
	traders = filteredTraders

	log.Printf("📋 After filtering follower traders, remaining trader count: %d (filtered: %d, total followers: %d, running followers: %d)", len(traders), filteredCount, followerTotalCount, followerRunningCount)

	// Sort by return percentage (descending)
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

	// Limit to top 50
	totalCount := len(traders)
	limit := 50
	if len(traders) > limit {
		traders = traders[:limit]
	}

	// Add follower count and verify equity history persistence for each trader
	for i := range traders {
		traderID, _ := traders[i]["trader_id"].(string)
		// Get follower count for this trader from database
		followers, err := database.GetFollowerTraders(traderID)
		if err == nil {
			traders[i]["followers_count"] = len(followers)
		} else {
			traders[i]["followers_count"] = 0
		}
		
		// Verify equity history exists in database for persistence
		// This ensures data survives deployments
		historyCount, err := database.GetEquityHistoryCount(traderID)
		if err == nil {
			if historyCount > 0 {
				// Get latest equity history record to ensure we have persisted data
				latestHistory, err := database.GetEquityHistory(traderID, 1)
				if err == nil && len(latestHistory) > 0 {
					latest := latestHistory[len(latestHistory)-1]
					// Use database PnL percentage if it's more recent or if real-time data is missing
					if pnlPct, ok := traders[i]["total_pnl_pct"].(float64); !ok || pnlPct == 0 {
						traders[i]["total_pnl_pct"] = latest.TotalPnLPct
						traders[i]["total_pnl"] = latest.TotalPnL
					}
					// Store that equity history exists for this trader
					traders[i]["has_equity_history"] = true
					traders[i]["equity_history_count"] = historyCount
				}
			} else {
				// No equity history yet - this is normal for new traders
				traders[i]["has_equity_history"] = false
				traders[i]["equity_history_count"] = 0
			}
		}
	}

	comparison := make(map[string]interface{})
	comparison["traders"] = traders
	comparison["count"] = len(traders)
	comparison["total_count"] = totalCount                      // Total trader count
	comparison["follower_total_count"] = followerTotalCount     // Total count of all follower traders
	comparison["follower_running_count"] = followerRunningCount // Count of running follower traders

	// Update cache
	tm.competitionCache.mu.Lock()
	tm.competitionCache.data = comparison
	tm.competitionCache.timestamp = time.Now()
	tm.competitionCache.mu.Unlock()

	return comparison, nil
}

// getConcurrentTraderData concurrently gets data for multiple traders
func (tm *TraderManager) getConcurrentTraderData(traders []*trader.AutoTrader, database *config.Database) []map[string]interface{} {
	type traderResult struct {
		index int
		data  map[string]interface{}
	}

	// Create result channel
	resultChan := make(chan traderResult, len(traders))

	// Concurrently get data for each trader
	for i, t := range traders {
		go func(index int, trader *trader.AutoTrader) {
			// Set timeout for single trader to 3 seconds
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			// Use channel to implement timeout control
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

			// Get followed_trader_id
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
				// Successfully got account information
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
				// Failed to get account information
				log.Printf("⚠️ Failed to get account information for trader %s: %v", trader.GetID(), err)
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
					"error":                  "Failed to get account data",
				}
			case <-ctx.Done():
				// Timeout
				log.Printf("⏰ Timeout getting account information for trader %s", trader.GetID())
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
					"error":                  "Timeout",
				}
			}

			resultChan <- traderResult{index: index, data: traderData}
		}(i, t)
	}

	// Collect all results
	results := make([]map[string]interface{}, len(traders))
	for i := 0; i < len(traders); i++ {
		result := <-resultChan
		results[result.index] = result.data
	}

	return results
}

// GetTopTradersData retrieves top 5 traders data (for performance comparison)
func (tm *TraderManager) GetTopTradersData(database *config.Database) (map[string]interface{}, error) {
	// Reuse competition data cache, as top 5 are filtered from all data
	competitionData, err := tm.GetCompetitionData(database)
	if err != nil {
		return nil, err
	}

	// Extract top 5 from competition data
	allTraders, ok := competitionData["traders"].([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("competition data format error")
	}

	// Limit to top 5
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

// isUserTrader checks if trader belongs to specified user
func isUserTrader(traderID, userID string) bool {
	// trader ID format: userID_traderName or randomUUID_modelName
	// For compatibility, we check prefix
	if len(traderID) >= len(userID) && traderID[:len(userID)] == userID {
		return true
	}
	// For old default user, all without explicit user prefix belong to default
	if userID == "default" && !containsUserPrefix(traderID) {
		return true
	}
	return false
}

// containsUserPrefix checks if trader ID contains user prefix
func containsUserPrefix(traderID string) bool {
	// Check if contains email format prefix (user@example.com_traderName)
	for i, ch := range traderID {
		if ch == '@' {
			// Found @ symbol, likely email prefix
			return true
		}
		if ch == '_' && i > 0 {
			// Found underscore but no @ before, might be UUID or other format
			break
		}
	}
	return false
}

// LoadUserTraders loads traders for specific user to memory
func (tm *TraderManager) LoadUserTraders(database *config.Database, userID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Get all traders for specified user
	traders, err := database.GetTraders(userID)
	if err != nil {
		return fmt.Errorf("failed to get trader list for user %s: %w", userID, err)
	}

	log.Printf("📋 Loading trader configurations for user %s: %d traders", userID, len(traders))

	// Get system configuration (does not include signal sources, signal sources are now user-level)
	maxDailyLossStr, _ := database.GetSystemConfig("max_daily_loss")
	maxDrawdownStr, _ := database.GetSystemConfig("max_drawdown")
	stopTradingMinutesStr, _ := database.GetSystemConfig("stop_trading_minutes")
	defaultCoinsStr, _ := database.GetSystemConfig("default_coins")

	// Get user signal source configuration
	var coinPoolURL, oiTopURL string
	if userSignalSource, err := database.GetUserSignalSource(userID); err == nil {
		coinPoolURL = userSignalSource.CoinPoolURL
		oiTopURL = userSignalSource.OITopURL
		log.Printf("📡 Loading signal source configuration for user %s: COIN POOL=%s, OI TOP=%s", userID, coinPoolURL, oiTopURL)
	} else {
		log.Printf("🔍 User %s has not configured signal source yet", userID)
	}

	// Parse configuration
	maxDailyLoss := 10.0 // Default value
	if val, err := strconv.ParseFloat(maxDailyLossStr, 64); err == nil {
		maxDailyLoss = val
	}

	maxDrawdown := 20.0 // Default value
	if val, err := strconv.ParseFloat(maxDrawdownStr, 64); err == nil {
		maxDrawdown = val
	}

	stopTradingMinutes := 60 // Default value
	if val, err := strconv.Atoi(stopTradingMinutesStr); err == nil {
		stopTradingMinutes = val
	}

	// Parse default coin list
	var defaultCoins []string
	if defaultCoinsStr != "" {
		if err := json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins); err != nil {
			log.Printf("⚠️ Failed to parse default coin configuration: %v, using empty list", err)
			defaultCoins = []string{}
		}
	}

	// 🔧 Performance optimization: query AI model and exchange configuration only once outside loop
	// Avoid repeated queries of same data in loop, reduce database pressure and lock hold time
	aiModels, err := database.GetAIModels(userID)
	if err != nil {
		log.Printf("⚠️ Failed to get AI model configuration for user %s: %v", userID, err)
		return fmt.Errorf("failed to get AI model configuration: %w", err)
	}

	exchanges, err := database.GetExchanges(userID)
	if err != nil {
		log.Printf("⚠️ Failed to get exchange configuration for user %s: %v", userID, err)
		return fmt.Errorf("failed to get exchange configuration: %w", err)
	}

	// Load configuration for each trader
	for _, traderCfg := range traders {
		// Check if trader already loaded
		if existingTrader, exists := tm.traders[traderCfg.ID]; exists {
			// Check if critical config changed
			existingStatus := existingTrader.GetStatus()
			existingUseTV, _ := existingStatus["use_tradingview"].(bool)
			if existingUseTV != traderCfg.UseTradingView {
				log.Printf("⚠️ Trader %s configuration changed (UseTradingView: %v -> %v), forcing reload",
					traderCfg.Name, existingUseTV, traderCfg.UseTradingView)
				// Stop and reload
				if isRunning, ok := existingStatus["is_running"].(bool); ok && isRunning {
					existingTrader.Stop()
					time.Sleep(200 * time.Millisecond)
				}
				delete(tm.traders, traderCfg.ID)
				// Continue to load below...
			} else {
				log.Printf("⚠️ Trader %s already loaded, skipping", traderCfg.Name)
				continue
			}
		}

		// Find AI model configuration from already queried list

		var aiModelCfg *config.AIModelConfig
		// Prioritize exact match on model.ID (new logic)
		for _, model := range aiModels {
			if model.ID == traderCfg.AIModelID {
				aiModelCfg = model
				break
			}
		}
		// If no exact match, try matching provider (compatible with old data)
		if aiModelCfg == nil {
			for _, model := range aiModels {
				if model.Provider == traderCfg.AIModelID {
					aiModelCfg = model
					log.Printf("⚠️  Trader %s using old provider match: %s -> %s", traderCfg.Name, traderCfg.AIModelID, model.ID)
					break
				}
			}
		}

		if aiModelCfg == nil {
			log.Printf("⚠️ Trader %s's AI model %s does not exist, skipping", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		if !aiModelCfg.Enabled {
			log.Printf("⚠️ Trader %s's AI model %s is not enabled, skipping", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		// Find exchange configuration from already queried list
		var exchangeCfg *config.ExchangeConfig
		for _, exchange := range exchanges {
			if exchange.ID == traderCfg.ExchangeID {
				exchangeCfg = exchange
				break
			}
		}

		if exchangeCfg == nil {
			log.Printf("⚠️ Trader %s's exchange %s does not exist, skipping", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		if !exchangeCfg.Enabled {
			log.Printf("⚠️ Trader %s's exchange %s is not enabled, skipping", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		// Use existing method to load trader
		err = tm.loadSingleTrader(traderCfg, aiModelCfg, exchangeCfg, coinPoolURL, oiTopURL, maxDailyLoss, maxDrawdown, stopTradingMinutes, defaultCoins, database, userID)
		if err != nil {
			log.Printf("⚠️ Failed to load trader %s: %v", traderCfg.Name, err)
			// Error is already stored in loadErrors by addTraderFromDB
		}
	}

	return nil
}

// LoadTraderByID loads a single trader with specified ID to memory
// This method automatically queries all required configurations (AI model, exchange, system config, etc.)
// Parameters:
//   - database: database instance
//   - userID: user ID
//   - traderID: trader ID
//
// Returns:
//   - error: returns error if trader does not exist, configuration is invalid, or loading fails
func (tm *TraderManager) LoadTraderByID(database *config.Database, userID, traderID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 1. Check if already loaded
	if _, exists := tm.traders[traderID]; exists {
		log.Printf("⚠️ Trader %s already loaded, skipping", traderID)
		return nil
	}

	// 2. Query trader configuration
	traders, err := database.GetTraders(userID)
	if err != nil {
		return fmt.Errorf("failed to get trader list: %w", err)
	}

	var traderCfg *config.TraderRecord
	for _, t := range traders {
		if t.ID == traderID {
			traderCfg = t
			break
		}
	}

	if traderCfg == nil {
		return fmt.Errorf("trader %s does not exist", traderID)
	}

	// 3. Query AI model configuration
	aiModels, err := database.GetAIModels(userID)
	if err != nil {
		return fmt.Errorf("failed to get AI model configuration: %w", err)
	}

	var aiModelCfg *config.AIModelConfig
	// Prioritize exact match on model.ID
	for _, model := range aiModels {
		if model.ID == traderCfg.AIModelID {
			aiModelCfg = model
			break
		}
	}
	// If no exact match, try matching provider (compatible with old data)
	if aiModelCfg == nil {
		for _, model := range aiModels {
			if model.Provider == traderCfg.AIModelID {
				aiModelCfg = model
				log.Printf("⚠️ Trader %s using old provider match: %s -> %s", traderCfg.Name, traderCfg.AIModelID, model.ID)
				break
			}
		}
	}

	if aiModelCfg == nil {
		return fmt.Errorf("AI model %s does not exist", traderCfg.AIModelID)
	}

	if !aiModelCfg.Enabled {
		return fmt.Errorf("AI model %s is not enabled", traderCfg.AIModelID)
	}

	// 4. Query exchange configuration
	exchanges, err := database.GetExchanges(userID)
	if err != nil {
		return fmt.Errorf("failed to get exchange configuration: %w", err)
	}

	var exchangeCfg *config.ExchangeConfig
	for _, exchange := range exchanges {
		if exchange.ID == traderCfg.ExchangeID {
			exchangeCfg = exchange
			break
		}
	}

	if exchangeCfg == nil {
		return fmt.Errorf("exchange %s does not exist", traderCfg.ExchangeID)
	}

	if !exchangeCfg.Enabled {
		return fmt.Errorf("exchange %s is not enabled", traderCfg.ExchangeID)
	}

	// 5. Query system configuration
	maxDailyLossStr, _ := database.GetSystemConfig("max_daily_loss")
	maxDrawdownStr, _ := database.GetSystemConfig("max_drawdown")
	stopTradingMinutesStr, _ := database.GetSystemConfig("stop_trading_minutes")
	defaultCoinsStr, _ := database.GetSystemConfig("default_coins")

	// 6. Query user signal source configuration
	var coinPoolURL, oiTopURL string
	if userSignalSource, err := database.GetUserSignalSource(userID); err == nil {
		coinPoolURL = userSignalSource.CoinPoolURL
		oiTopURL = userSignalSource.OITopURL
		log.Printf("📡 Loading signal source configuration for user %s: COIN POOL=%s, OI TOP=%s", userID, coinPoolURL, oiTopURL)
	} else {
		log.Printf("🔍 User %s has not configured signal source yet", userID)
	}

	// 7. Parse system configuration
	maxDailyLoss := 10.0 // Default value
	if val, err := strconv.ParseFloat(maxDailyLossStr, 64); err == nil {
		maxDailyLoss = val
	}

	maxDrawdown := 20.0 // Default value
	if val, err := strconv.ParseFloat(maxDrawdownStr, 64); err == nil {
		maxDrawdown = val
	}

	stopTradingMinutes := 60 // Default value
	if val, err := strconv.Atoi(stopTradingMinutesStr); err == nil {
		stopTradingMinutes = val
	}

	// Parse default coin list
	var defaultCoins []string
	if defaultCoinsStr != "" {
		if err := json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins); err != nil {
			log.Printf("⚠️ Failed to parse default coin configuration: %v, using empty list", err)
			defaultCoins = []string{}
		}
	}

	// 8. Call private method to load trader
	log.Printf("📋 Loading single trader: %s (%s)", traderCfg.Name, traderID)
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

// ReloadTraderFromDB reloads specified trader from database (regardless of whether already in memory)
// Always removes old instance to ensure latest configuration (including UseTradingView, etc.) takes effect
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
		log.Printf("✓ Trader %s removed from memory for reload", traderID)
	}

	traderCfg, aiModelCfg, exchangeCfg, err := database.GetTraderConfig(userID, traderID)
	if err != nil {
		return fmt.Errorf("failed to get trader configuration: %w", err)
	}

	// Query system configuration
	maxDailyLossStr, _ := database.GetSystemConfig("max_daily_loss")
	maxDrawdownStr, _ := database.GetSystemConfig("max_drawdown")
	stopTradingMinutesStr, _ := database.GetSystemConfig("stop_trading_minutes")
	defaultCoinsStr, _ := database.GetSystemConfig("default_coins")

	// Query user signal source configuration
	var coinPoolURL, oiTopURL string
	if userSignalSource, err := database.GetUserSignalSource(userID); err == nil {
		coinPoolURL = userSignalSource.CoinPoolURL
		oiTopURL = userSignalSource.OITopURL
		log.Printf("📡 Reloading signal source configuration for user %s: COIN POOL=%s, OI TOP=%s", userID, coinPoolURL, oiTopURL)
	} else {
		log.Printf("🔍 User %s has not configured signal source yet", userID)
	}

	// Parse system configuration
	maxDailyLoss := 10.0 // Default value
	if val, err := strconv.ParseFloat(maxDailyLossStr, 64); err == nil {
		maxDailyLoss = val
	}

	maxDrawdown := 20.0 // Default value
	if val, err := strconv.ParseFloat(maxDrawdownStr, 64); err == nil {
		maxDrawdown = val
	}

	stopTradingMinutes := 60 // Default value
	if val, err := strconv.Atoi(stopTradingMinutesStr); err == nil {
		stopTradingMinutes = val
	}

	// Parse default coin list
	var defaultCoins []string
	if defaultCoinsStr != "" {
		if err := json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins); err != nil {
			log.Printf("⚠️ Failed to parse default coin configuration: %v, using empty list", err)
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

// loadSingleTrader loads a single trader (common logic extracted from existing code)
func (tm *TraderManager) loadSingleTrader(traderCfg *config.TraderRecord, aiModelCfg *config.AIModelConfig, exchangeCfg *config.ExchangeConfig, coinPoolURL, oiTopURL string, maxDailyLoss, maxDrawdown float64, stopTradingMinutes int, defaultCoins []string, database *config.Database, userID string) error {
	// Process trading symbol list
	var tradingCoins []string
	if traderCfg.TradingSymbols != "" {
		// Parse comma-separated trading symbol list
		symbols := strings.Split(traderCfg.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" {
				tradingCoins = append(tradingCoins, symbol)
			}
		}
	}

	// If no trading symbols specified, use default coins
	if len(tradingCoins) == 0 {
		tradingCoins = defaultCoins
	}

	// Decide whether to use signal source based on trader configuration
	var effectiveCoinPoolURL string
	if traderCfg.UseCoinPool && coinPoolURL != "" {
		effectiveCoinPoolURL = coinPoolURL
		log.Printf("✓ Trader %s enabled COIN POOL signal source: %s", traderCfg.Name, coinPoolURL)
	}

	// Build AutoTraderConfig
	traderConfig := trader.AutoTraderConfig{
		ID:                   traderCfg.ID,
		Name:                 traderCfg.Name,
		AIModel:              aiModelCfg.Provider, // Use provider as model identifier
		Exchange:             exchangeCfg.ID,      // Use exchange ID
		InitialBalance:       traderCfg.InitialBalance,
		BTCETHLeverage:       traderCfg.BTCETHLeverage,
		AltcoinLeverage:      traderCfg.AltcoinLeverage,
		ScanInterval:         time.Duration(traderCfg.ScanIntervalMinutes) * time.Minute,
		CoinPoolAPIURL:       effectiveCoinPoolURL,
		CustomAPIURL:         aiModelCfg.CustomAPIURL,    // Custom API URL
		CustomModelName:      aiModelCfg.CustomModelName, // Custom model name
		UseQwen:              aiModelCfg.Provider == "qwen",
		MaxDailyLoss:         maxDailyLoss,
		MaxDrawdown:          maxDrawdown,
		StopTradingTime:      time.Duration(stopTradingMinutes) * time.Minute,
		IsCrossMargin:        traderCfg.IsCrossMargin,
		ShowInCompetition:    traderCfg.ShowInCompetition, // Competition visibility
		DefaultCoins:         defaultCoins,
		TradingCoins:         tradingCoins,
		SystemPromptTemplate: traderCfg.SystemPromptTemplate, // System prompt template
		UseTradingView:       traderCfg.UseTradingView,       // TradingView signal source
		HyperliquidTestnet:   exchangeCfg.Testnet,            // Hyperliquid testnet
	}

	// Set API keys based on exchange type
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
	} else if exchangeCfg.ID == "bitget" {
		traderConfig.BitgetAPIKey = exchangeCfg.APIKey
		traderConfig.BitgetSecretKey = exchangeCfg.SecretKey
		traderConfig.BitgetPassphrase = exchangeCfg.OkxPassphrase // Reuse passphrase field
	} else if exchangeCfg.ID == "hyperliquid" {
		traderConfig.HyperliquidPrivateKey = exchangeCfg.APIKey // hyperliquid uses APIKey to store private key
		traderConfig.HyperliquidWalletAddr = exchangeCfg.HyperliquidWalletAddr
	} else if exchangeCfg.ID == "aster" {
		traderConfig.AsterUser = exchangeCfg.AsterUser
		traderConfig.AsterSigner = exchangeCfg.AsterSigner
		traderConfig.AsterPrivateKey = exchangeCfg.AsterPrivateKey
	} else if exchangeCfg.ID == "lighter" {
		traderConfig.LighterWalletAddr = exchangeCfg.LighterWalletAddr
		traderConfig.LighterAPIKeyPrivateKey = exchangeCfg.LighterAPIKeyPrivateKey
		traderConfig.LighterAPIKeyIndex = exchangeCfg.LighterAPIKeyIndex
		traderConfig.LighterTestnet = exchangeCfg.Testnet
	}

	// Set API keys based on AI model
	if aiModelCfg.Provider == "qwen" {
		traderConfig.QwenKey = aiModelCfg.APIKey
	} else if aiModelCfg.Provider == "deepseek" {
		traderConfig.DeepSeekKey = aiModelCfg.APIKey
	} else {
		// For other providers (grok, openai, claude, gemini, kimi, custom), use CustomAPIKey
		traderConfig.CustomAPIKey = aiModelCfg.APIKey
	}

	// Create trader instance
	log.Printf("INFO: Creating AutoTrader with config (trader=%s, UseTradingView=%v, ScanInterval=%v)", traderCfg.Name, traderConfig.UseTradingView, traderConfig.ScanInterval)
	at, err := trader.NewAutoTrader(traderConfig, database, userID)
	if err != nil {
		return fmt.Errorf("failed to create trader: %w", err)
	}
	log.Printf("INFO: AutoTrader created successfully (trader=%s, id=%s, UseTradingView=%v)", traderCfg.Name, traderCfg.ID, traderConfig.UseTradingView)

	// Set trade replication callback function (for real-time trade replication to followers)
	at.SetTradeReplicationCallback(func(traderID string, decision *dec.Decision) {
		tm.ReplicateTradeToFollowers(traderID, decision, database)
	})

	// Set custom prompt (if any)
	if traderCfg.CustomPrompt != "" {
		at.SetCustomPrompt(traderCfg.CustomPrompt)
		at.SetOverrideBasePrompt(traderCfg.OverrideBasePrompt)
		if traderCfg.OverrideBasePrompt {
			log.Printf("✓ Custom trading strategy prompt set (override base prompt)")
		} else {
			log.Printf("✓ Custom trading strategy prompt set (supplement base prompt)")
		}
	}

	tm.traders[traderCfg.ID] = at
	log.Printf("✓ Trader '%s' (%s + %s) loaded to memory for user", traderCfg.Name, aiModelCfg.Provider, exchangeCfg.ID)
	return nil
}

// RemoveTrader removes specified trader from memory (does not affect database)
// Used to force reload when updating trader configuration
func (tm *TraderManager) RemoveTrader(traderID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.traders[traderID]; exists {
		delete(tm.traders, traderID)
		log.Printf("✓ Trader %s removed from memory", traderID)
		// Invalidate competition cache to ensure deleted trader no longer appears in leaderboard
		tm.InvalidateCompetitionCache()
	}
}

// InvalidateCompetitionCache invalidates competition cache
func (tm *TraderManager) InvalidateCompetitionCache() {
	tm.competitionCache.mu.Lock()
	defer tm.competitionCache.mu.Unlock()
	tm.competitionCache.data = make(map[string]interface{})
	tm.competitionCache.timestamp = time.Time{} // Reset timestamp to force refresh
	log.Printf("✓ DEBUG [InvalidateCompetitionCache]: Competition cache invalidated")
}

// GetFollowerTraders gets all trader instances following specified trader (from memory)
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

// ReplicateTradeToFollowers sends trade signal to all followers (AI analysis mode)
func (tm *TraderManager) ReplicateTradeToFollowers(followedTraderID string, decision *dec.Decision, database *config.Database) {
	// Get all follower traders from database
	followerRecords, err := database.GetFollowerTraders(followedTraderID)
	if err != nil {
		log.Printf("⚠️ [%s] Failed to get follower list: %v", followedTraderID, err)
		return
	}

	if len(followerRecords) == 0 {
		return // No followers
	}

	log.Printf("📋 [%s] Found %d followers, sending trade signal (%s %s)...", followedTraderID, len(followerRecords), decision.Symbol, decision.Action)

	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// Get parent trader info for signal context
	parentTrader, exists := tm.traders[followedTraderID]
	if !exists {
		log.Printf("⚠️ [%s] Parent trader not in memory, cannot get account information", followedTraderID)
		return
	}

	// Get parent trader account info
	parentAccount, err := parentTrader.GetAccountInfo()
	if err != nil {
		log.Printf("⚠️ [%s] Failed to get parent trader account information: %v", followedTraderID, err)
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
			log.Printf("⚠️ [%s] Follower %s (%s) not in memory, skipping", followedTraderID, followerRecord.Name, followerRecord.ID)
			skipCount++
			skippedFollowers = append(skippedFollowers, fmt.Sprintf("%s (not in memory)", followerRecord.Name))
			continue
		}

		// Check if follower is running
		status := followerTrader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); !ok || !isRunning {
			log.Printf("⚠️ [%s] Follower %s (%s) not running, skipping", followedTraderID, followerRecord.Name, followerRecord.ID)
			skipCount++
			skippedFollowers = append(skippedFollowers, fmt.Sprintf("%s (not running)", followerRecord.Name))
			continue
		}

		// Send signal to follower trader (non-blocking)
		// Deep copy the decision to avoid race conditions
		decisionCopy := &dec.Decision{
			Symbol:          signal.Decision.Symbol,
			Action:          signal.Decision.Action,
			Leverage:        signal.Decision.Leverage,
			PositionSizeUSD: signal.Decision.PositionSizeUSD,
			StopLoss:        signal.Decision.StopLoss,
			TakeProfit:      signal.Decision.TakeProfit,
			Reasoning:       signal.Decision.Reasoning,
			Confidence:      signal.Decision.Confidence,
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
			log.Printf("❌ [%s] Failed to send trade signal to follower %s (%s): %v",
				followedTraderID, followerRecord.Name, followerRecord.ID, err)
			errorCount++
			errorFollowers = append(errorFollowers, fmt.Sprintf("%s (%v)", followerRecord.Name, err))
		} else {
			log.Printf("✓ [%s] Successfully sent trade signal to follower %s (%s) (signal_id=%s)",
				followedTraderID, followerRecord.Name, followerRecord.ID, signalCopy.SignalID)
			successCount++
		}
	}

	// Summary log
	duration := time.Since(startTime)
	log.Printf("📊 [%s] Signal distribution completed: success=%d, skipped=%d, failed=%d, total=%d, duration=%v",
		followedTraderID, successCount, skipCount, errorCount, len(followerRecords), duration)

	if len(skippedFollowers) > 0 {
		log.Printf("⚠️ [%s] Skipped followers: %v", followedTraderID, skippedFollowers)
	}
	if len(errorFollowers) > 0 {
		log.Printf("❌ [%s] Failed followers: %v", followedTraderID, errorFollowers)
	}
}

// replicateTradeToFollower replicates trade to single follower (using follower's own risk management)
func (tm *TraderManager) replicateTradeToFollower(follower *trader.AutoTrader, followerID string, followerRecord *config.TraderRecord, originalDecision *dec.Decision) error {
	// Create a copy of the decision for the follower
	followerDecision := *originalDecision

	// Get follower's current balance
	balance, err := follower.GetAccountInfo()
	if err != nil {
		return fmt.Errorf("failed to get follower balance: %w", err)
	}

	totalEquity := 0.0
	if equity, ok := balance["total_equity"].(float64); ok {
		totalEquity = equity
	}

	// Use follower's initial balance to calculate scaling ratio
	followerInitialBalance := followerRecord.InitialBalance
	if followerInitialBalance <= 0 {
		return fmt.Errorf("follower initial balance invalid: %.2f", followerInitialBalance)
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
		return fmt.Errorf("failed to execute follower trade: %w", err)
	}

	return nil
}
