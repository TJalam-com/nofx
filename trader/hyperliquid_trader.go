package trader

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sonirico/go-hyperliquid"
)

// HyperliquidTrader Hyperliquid交易器
type HyperliquidTrader struct {
	exchange      *hyperliquid.Exchange
	ctx           context.Context
	walletAddr    string
	meta          *hyperliquid.Meta // 缓存meta信息（包含精度等）
	metaMutex     sync.RWMutex      // 保护meta字段的并发访问
	isCrossMargin bool              // 是否为全仓模式
}

// NewHyperliquidTrader 创建Hyperliquid交易器
func NewHyperliquidTrader(privateKeyHex string, walletAddr string, testnet bool) (*HyperliquidTrader, error) {
	// 去掉私钥的 0x 前缀（如果有，不区分大小写）
	privateKeyHex = strings.TrimPrefix(strings.ToLower(privateKeyHex), "0x")

	// Parse private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// 选择API URL
	apiURL := hyperliquid.MainnetAPIURL
	if testnet {
		apiURL = hyperliquid.TestnetAPIURL
	}

	// Security enhancement: Implement Agent Wallet best practices
	// Reference: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/nonces-and-api-wallets
	agentAddr := crypto.PubkeyToAddress(*privateKey.Public().(*ecdsa.PublicKey)).Hex()

	if walletAddr == "" {
		return nil, fmt.Errorf("❌ Configuration error: Main wallet address (hyperliquid_wallet_addr) not provided\n" +
			"🔐 Correct configuration pattern:\n" +
			"  1. hyperliquid_private_key = Agent Private Key (for signing only, balance should be ~0)\n" +
			"  2. hyperliquid_wallet_addr = Main Wallet Address (holds funds, never expose private key)\n" +
			"💡 Please create an Agent Wallet on Hyperliquid official website and authorize it before configuration:\n" +
			"   https://app.hyperliquid.xyz/ → Settings → API Wallets")
	}

	// Check if user accidentally uses main wallet private key (security risk)
	if strings.EqualFold(walletAddr, agentAddr) {
		log.Printf("⚠️⚠️⚠️ WARNING: Main wallet address (%s) matches Agent wallet address!", maskWalletAddress(walletAddr))
		log.Printf("   This indicates you may be using your main wallet private key, which poses extremely high security risks!")
		log.Printf("   Recommendation: Immediately create a separate Agent Wallet on Hyperliquid official website")
		log.Printf("   Reference: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/nonces-and-api-wallets")
	} else {
		log.Printf("✓ Using Agent Wallet mode (secure)")
		log.Printf("  └─ Agent wallet address: %s (for signing)", maskWalletAddress(agentAddr))
		log.Printf("  └─ Main wallet address: %s (holds funds)", maskWalletAddress(walletAddr))
	}

	ctx := context.Background()

	// 创建Exchange客户端（Exchange包含Info功能）
	exchange := hyperliquid.NewExchange(
		ctx,
		privateKey,
		apiURL,
		nil,        // Meta will be fetched automatically
		"",         // vault address (empty for personal account)
		walletAddr, // wallet address
		nil,        // SpotMeta will be fetched automatically
	)

	log.Printf("✓ Hyperliquid trader initialized successfully (testnet=%v, wallet=%s)", testnet, maskWalletAddress(walletAddr))

	// Get meta information (includes precision and other configuration)
	meta, err := exchange.Info().Meta(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get meta information: %w", err)
	}

	// 🔍 Security check: Validate Agent wallet balance (should be close to 0)
	// Only check if using separate Agent wallet (not when main wallet is used as agent)
	if !strings.EqualFold(walletAddr, agentAddr) {
		agentState, err := exchange.Info().UserState(ctx, agentAddr)
		if err == nil && agentState != nil && agentState.CrossMarginSummary.AccountValue != "" {
			// Parse Agent wallet balance
			agentBalance, _ := strconv.ParseFloat(agentState.CrossMarginSummary.AccountValue, 64)

			if agentBalance > 100 {
				// Critical: Agent wallet holds too much funds
				log.Printf("🚨🚨🚨 CRITICAL SECURITY WARNING 🚨🚨🚨")
				log.Printf("   Agent wallet balance: %.2f USDC (exceeds safe threshold of 100 USDC)", agentBalance)
				log.Printf("   Agent wallet address: %s", maskWalletAddress(agentAddr))
				log.Printf("   ⚠️  Agent wallets should only be used for signing and hold minimal/zero balance")
				log.Printf("   ⚠️  High balance in Agent wallet poses security risks")
				log.Printf("   📖 Reference: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/nonces-and-api-wallets")
				log.Printf("   💡 Recommendation: Transfer funds to main wallet and keep Agent wallet balance near 0")
				return nil, fmt.Errorf("security check failed: Agent wallet balance too high (%.2f USDC), exceeds 100 USDC threshold", agentBalance)
			} else if agentBalance > 10 {
				// Warning: Agent wallet has some balance (acceptable but not ideal)
				log.Printf("⚠️  Notice: Agent wallet address (%s) has some balance: %.2f USDC", maskWalletAddress(agentAddr), agentBalance)
				log.Printf("   While not critical, it's recommended to keep Agent wallet balance near 0 for security")
			} else {
				// OK: Agent wallet balance is safe
				log.Printf("✓ Agent wallet balance is safe: %.2f USDC (near zero as recommended)", agentBalance)
			}
		} else if err != nil {
			// Failed to query agent balance - log warning but don't block initialization
			log.Printf("⚠️  Could not verify Agent wallet balance (query failed): %v", err)
			log.Printf("   Proceeding with initialization, but please manually verify Agent wallet balance is near 0")
		}
	}

	return &HyperliquidTrader{
		exchange:      exchange,
		ctx:           ctx,
		walletAddr:    walletAddr,
		meta:          meta,
		isCrossMargin: true, // 默认使用全仓模式
	}, nil
}

// GetBalance gets account balance
func (t *HyperliquidTrader) GetBalance() (map[string]interface{}, error) {
	log.Printf("🔄 Calling Hyperliquid API to get account balance...")

	// ✅ Step 1: Query Spot account balance
	spotState, err := t.exchange.Info().SpotUserState(t.ctx, t.walletAddr)
	var spotUSDCBalance float64 = 0.0
	if err != nil {
		log.Printf("⚠️ Failed to query Spot balance (may have no spot assets): %v", err)
	} else if spotState != nil && len(spotState.Balances) > 0 {
		for _, balance := range spotState.Balances {
			if balance.Coin == "USDC" {
				spotUSDCBalance, _ = strconv.ParseFloat(balance.Total, 64)
				log.Printf("✓ Found Spot balance: %.2f USDC", spotUSDCBalance)
				break
			}
		}
	}

	// ✅ Step 2: Query Perpetuals account status
	accountState, err := t.exchange.Info().UserState(t.ctx, t.walletAddr)
	if err != nil {
		log.Printf("❌ Hyperliquid Perpetuals API call failed: %v", err)
		return nil, fmt.Errorf("failed to get account information: %w", err)
	}

	// 解析余额信息（MarginSummary字段都是string）
	result := make(map[string]interface{})

	// ✅ Step 3: 根据保证金模式动态选择正确的摘要（CrossMarginSummary 或 MarginSummary）
	var accountValue, totalMarginUsed float64
	var summaryType string
	var summary interface{}

	if t.isCrossMargin {
		// Cross margin mode: use CrossMarginSummary
		accountValue, _ = strconv.ParseFloat(accountState.CrossMarginSummary.AccountValue, 64)
		totalMarginUsed, _ = strconv.ParseFloat(accountState.CrossMarginSummary.TotalMarginUsed, 64)
		summaryType = "CrossMarginSummary (Cross Margin)"
		summary = accountState.CrossMarginSummary
	} else {
		// Isolated margin mode: use MarginSummary
		accountValue, _ = strconv.ParseFloat(accountState.MarginSummary.AccountValue, 64)
		totalMarginUsed, _ = strconv.ParseFloat(accountState.MarginSummary.TotalMarginUsed, 64)
		summaryType = "MarginSummary (Isolated Margin)"
		summary = accountState.MarginSummary
	}

	// 🔍 Debug: print complete summary structure returned by API
	summaryJSON, _ := json.MarshalIndent(summary, "  ", "  ")
	log.Printf("🔍 [DEBUG] Hyperliquid API %s complete data:", summaryType)
	log.Printf("%s", string(summaryJSON))

	// ⚠️ Critical fix: accumulate true unrealized PnL from all positions
	totalUnrealizedPnl := 0.0
	for _, assetPos := range accountState.AssetPositions {
		unrealizedPnl, _ := strconv.ParseFloat(assetPos.Position.UnrealizedPnl, 64)
		totalUnrealizedPnl += unrealizedPnl
	}

	// ✅ Correct understanding of Hyperliquid fields:
	// AccountValue = Total account net value (includes idle funds + position value + unrealized PnL)
	// TotalMarginUsed = Margin used by positions (already included in AccountValue, for display only)
	//
	// To be compatible with auto_trader.go calculation logic (totalEquity = totalWalletBalance + totalUnrealizedProfit)
	// Need to return "wallet balance without unrealized PnL"
	walletBalanceWithoutUnrealized := accountValue - totalUnrealizedPnl

	// ✅ Step 4: Use Withdrawable field (PR #443)
	// Withdrawable is the official real withdrawable balance, more reliable than simple calculation
	availableBalance := 0.0
	if accountState.Withdrawable != "" {
		withdrawable, err := strconv.ParseFloat(accountState.Withdrawable, 64)
		if err == nil && withdrawable > 0 {
			availableBalance = withdrawable
			log.Printf("✓ Using Withdrawable as available balance: %.2f", availableBalance)
		}
	}

	// Fallback: if no Withdrawable, use simple calculation
	if availableBalance == 0 && accountState.Withdrawable == "" {
		availableBalance = accountValue - totalMarginUsed
		if availableBalance < 0 {
			log.Printf("⚠️ Calculated available balance is negative (%.2f), resetting to 0", availableBalance)
			availableBalance = 0
		}
	}

	// ✅ Step 5: Correctly handle Spot + Perpetuals balance
	// Important: Spot is only added to total assets, not to available balance
	//       Reason: Spot and Perpetuals are separate accounts, require manual ClassTransfer to transfer
	totalWalletBalance := walletBalanceWithoutUnrealized + spotUSDCBalance

	result["totalWalletBalance"] = totalWalletBalance    // Total assets (Perp + Spot)
	result["availableBalance"] = availableBalance        // Available balance (Perpetuals only, excluding Spot)
	result["totalUnrealizedProfit"] = totalUnrealizedPnl // Unrealized PnL (from Perpetuals only)
	result["spotBalance"] = spotUSDCBalance              // Spot balance (returned separately)

	log.Printf("✓ Hyperliquid complete account:")
	log.Printf("  • Spot balance: %.2f USDC (need to manually transfer to Perpetuals to open positions)", spotUSDCBalance)
	log.Printf("  • Perpetuals contract net value: %.2f USDC (wallet %.2f + unrealized %.2f)",
		accountValue,
		walletBalanceWithoutUnrealized,
		totalUnrealizedPnl)
	log.Printf("  • Perpetuals available balance: %.2f USDC (can be directly used for opening positions)", availableBalance)
	log.Printf("  • Margin used: %.2f USDC", totalMarginUsed)
	log.Printf("  • Total assets (Perp+Spot): %.2f USDC", totalWalletBalance)
	log.Printf("  ⭐ Total assets: %.2f USDC | Perp available: %.2f USDC | Spot balance: %.2f USDC",
		totalWalletBalance, availableBalance, spotUSDCBalance)

	return result, nil
}

// GetPositions gets all positions
func (t *HyperliquidTrader) GetPositions() ([]map[string]interface{}, error) {
	// Get account status
	accountState, err := t.exchange.Info().UserState(t.ctx, t.walletAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var result []map[string]interface{}

	// 遍历所有持仓
	for _, assetPos := range accountState.AssetPositions {
		position := assetPos.Position

		// 持仓数量（string类型）
		posAmt, _ := strconv.ParseFloat(position.Szi, 64)

		if posAmt == 0 {
			continue // 跳过无持仓的
		}

		posMap := make(map[string]interface{})

		// 标准化symbol格式（Hyperliquid使用如"BTC"，我们转换为"BTCUSDT"）
		symbol := position.Coin + "USDT"
		posMap["symbol"] = symbol

		// 持仓数量和方向
		if posAmt > 0 {
			posMap["side"] = "long"
			posMap["positionAmt"] = posAmt
		} else {
			posMap["side"] = "short"
			posMap["positionAmt"] = -posAmt // 转为正数
		}

		// 价格信息（EntryPx和LiquidationPx是指针类型）
		var entryPrice, liquidationPx float64
		if position.EntryPx != nil {
			entryPrice, _ = strconv.ParseFloat(*position.EntryPx, 64)
		}
		if position.LiquidationPx != nil {
			liquidationPx, _ = strconv.ParseFloat(*position.LiquidationPx, 64)
		}

		positionValue, _ := strconv.ParseFloat(position.PositionValue, 64)
		unrealizedPnl, _ := strconv.ParseFloat(position.UnrealizedPnl, 64)

		// 计算mark price（positionValue / abs(posAmt)）
		var markPrice float64
		if posAmt != 0 {
			markPrice = positionValue / absFloat(posAmt)
		}

		posMap["entryPrice"] = entryPrice
		posMap["markPrice"] = markPrice
		posMap["unRealizedProfit"] = unrealizedPnl
		posMap["leverage"] = float64(position.Leverage.Value)
		posMap["liquidationPrice"] = liquidationPx

		result = append(result, posMap)
	}

	return result, nil
}

// SetMarginMode sets margin mode (set together with SetLeverage)
func (t *HyperliquidTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// Hyperliquid margin mode is set in SetLeverage, here we only record
	t.isCrossMargin = isCrossMargin
	marginModeStr := "Cross Margin"
	if !isCrossMargin {
		marginModeStr = "Isolated Margin"
	}
	log.Printf("  ✓ %s will use %s mode", symbol, marginModeStr)
	return nil
}

// SetLeverage sets leverage
func (t *HyperliquidTrader) SetLeverage(symbol string, leverage int) error {
	// Hyperliquid symbol format (remove USDT suffix)
	coin := convertSymbolToHyperliquid(symbol)

	// Call UpdateLeverage (leverage int, name string, isCross bool)
	// Third parameter: true=Cross Margin mode, false=Isolated Margin mode
	_, err := t.exchange.UpdateLeverage(t.ctx, leverage, coin, t.isCrossMargin)
	if err != nil {
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	log.Printf("  ✓ %s leverage switched to %dx", symbol, leverage)
	return nil
}

// refreshMetaIfNeeded refreshes Meta information when it becomes invalid (triggered when Asset ID is 0)
func (t *HyperliquidTrader) refreshMetaIfNeeded(coin string) error {
	assetID := t.exchange.Info().NameToAsset(coin)
	if assetID != 0 {
		return nil // Meta is normal, no need to refresh
	}

	log.Printf("⚠️  Asset ID for %s is 0, attempting to refresh Meta information...", coin)

	// Refresh Meta information
	meta, err := t.exchange.Info().Meta(t.ctx)
	if err != nil {
		return fmt.Errorf("failed to refresh Meta information: %w", err)
	}

	// ✅ Thread-safe: use write lock to protect meta field update
	t.metaMutex.Lock()
	t.meta = meta
	t.metaMutex.Unlock()

	log.Printf("✅ Meta information refreshed, contains %d assets", len(meta.Universe))

	// Verify Asset ID after refresh
	assetID = t.exchange.Info().NameToAsset(coin)
	if assetID == 0 {
		return fmt.Errorf("❌ Even after refreshing Meta, Asset ID for %s is still 0. Possible reasons:\n"+
			"  1. This coin is not listed on Hyperliquid\n"+
			"  2. Coin name is incorrect (should be BTC not BTCUSDT)\n"+
			"  3. API connection issue", coin)
	}

	log.Printf("✅ Asset ID check passed after refresh: %s -> %d", coin, assetID)
	return nil
}

// OpenLong opens a long position
func (t *HyperliquidTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// First cancel all pending orders for this coin
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel old pending orders: %v", err)
	}

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)

	// Get current price (for market order)
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ Critical: round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)
	log.Printf("  📏 Quantity precision processing: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

	// ⚠️ Critical: price also needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 1.01)
	log.Printf("  💰 Price precision processing: %.8f -> %.8f (5 significant figures)", price*1.01, aggressivePrice)

	// Create market buy order (using IOC limit order with aggressive price)
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: true,
		Size:  roundedQuantity, // Use rounded quantity
		Price: aggressivePrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc, // Immediate or Cancel (similar to market order)
			},
		},
		ReduceOnly: false,
	}

	_, err = t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open long position: %w", err)
	}

	log.Printf("✓ Long position opened successfully: %s quantity: %.4f", symbol, roundedQuantity)

	result := make(map[string]interface{})
	result["orderId"] = 0 // Hyperliquid没有返回order ID
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// OpenShort opens a short position
func (t *HyperliquidTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// First cancel all pending orders for this coin
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel old pending orders: %v", err)
	}

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ Critical: round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)
	log.Printf("  📏 Quantity precision processing: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

	// ⚠️ Critical: price also needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 0.99)
	log.Printf("  💰 Price precision processing: %.8f -> %.8f (5 significant figures)", price*0.99, aggressivePrice)

	// Create market sell order
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: false,
		Size:  roundedQuantity, // Use rounded quantity
		Price: aggressivePrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc,
			},
		},
		ReduceOnly: false,
	}

	_, err = t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open short position: %w", err)
	}

	log.Printf("✓ Short position opened successfully: %s quantity: %.4f", symbol, roundedQuantity)

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CloseLong closes a long position
func (t *HyperliquidTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no long position found for %s", symbol)
		}
	}

	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ Critical: round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)
	log.Printf("  📏 Quantity precision processing: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

	// ⚠️ Critical: price also needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 0.99)
	log.Printf("  💰 Price precision processing: %.8f -> %.8f (5 significant figures)", price*0.99, aggressivePrice)

	// Create close order (sell + ReduceOnly)
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: false,
		Size:  roundedQuantity, // Use rounded quantity
		Price: aggressivePrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc,
			},
		},
		ReduceOnly: true, // Only close position, don't open new one
	}

	_, err = t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to close long position: %w", err)
	}

	log.Printf("✓ Long position closed successfully: %s quantity: %.4f", symbol, roundedQuantity)

	// After closing, cancel all pending orders for this coin
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel pending orders: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CloseShort closes a short position
func (t *HyperliquidTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no short position found for %s", symbol)
		}
	}

	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ Critical: round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)
	log.Printf("  📏 Quantity precision processing: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

	// ⚠️ Critical: price also needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 1.01)
	log.Printf("  💰 Price precision processing: %.8f -> %.8f (5 significant figures)", price*1.01, aggressivePrice)

	// Create close order (buy + ReduceOnly)
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: true,
		Size:  roundedQuantity, // Use rounded quantity
		Price: aggressivePrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc,
			},
		},
		ReduceOnly: true,
	}

	_, err = t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to close short position: %w", err)
	}

	log.Printf("✓ Short position closed successfully: %s quantity: %.4f", symbol, roundedQuantity)

	// After closing, cancel all pending orders for this coin
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel pending orders: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CancelStopOrders cancels stop loss/take profit orders for this coin

// CancelStopLossOrders cancels only stop loss orders (Hyperliquid cannot distinguish stop loss from take profit, cancels all)
func (t *HyperliquidTrader) CancelStopLossOrders(symbol string) error {
	// Hyperliquid SDK's OpenOrder structure doesn't expose trigger field
	// Cannot distinguish stop loss from take profit orders, so cancel all pending orders for this coin
	log.Printf("  ⚠️ Hyperliquid cannot distinguish stop loss/take profit orders, will cancel all pending orders")
	return t.CancelStopOrders(symbol)
}

// CancelTakeProfitOrders cancels only take profit orders (Hyperliquid cannot distinguish stop loss from take profit, cancels all)
func (t *HyperliquidTrader) CancelTakeProfitOrders(symbol string) error {
	// Hyperliquid SDK's OpenOrder structure doesn't expose trigger field
	// Cannot distinguish stop loss from take profit orders, so cancel all pending orders for this coin
	log.Printf("  ⚠️ Hyperliquid cannot distinguish stop loss/take profit orders, will cancel all pending orders")
	return t.CancelStopOrders(symbol)
}

// CancelAllOrders cancels all pending orders for this coin
func (t *HyperliquidTrader) CancelAllOrders(symbol string) error {
	coin := convertSymbolToHyperliquid(symbol)

	// Get all pending orders
	openOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
	if err != nil {
		return fmt.Errorf("failed to get pending orders: %w", err)
	}

	// Cancel all pending orders for this coin
	for _, order := range openOrders {
		if order.Coin == coin {
			_, err := t.exchange.Cancel(t.ctx, coin, order.Oid)
			if err != nil {
				log.Printf("  ⚠ Failed to cancel order (oid=%d): %v", order.Oid, err)
			}
		}
	}

	log.Printf("  ✓ Canceled all pending orders for %s", symbol)
	return nil
}

// CancelStopOrders cancels stop loss/take profit orders for this coin (used when adjusting stop loss/take profit positions)
func (t *HyperliquidTrader) CancelStopOrders(symbol string) error {
	coin := convertSymbolToHyperliquid(symbol)

	// Get all pending orders
	openOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
	if err != nil {
		return fmt.Errorf("failed to get pending orders: %w", err)
	}

	// Note: Hyperliquid SDK's OpenOrder structure doesn't expose trigger field
	// So temporarily cancel all pending orders for this coin (including stop loss/take profit orders)
	// This is safe because all old orders should be cleaned up before setting new stop loss/take profit
	canceledCount := 0
	for _, order := range openOrders {
		if order.Coin == coin {
			_, err := t.exchange.Cancel(t.ctx, coin, order.Oid)
			if err != nil {
				log.Printf("  ⚠ Failed to cancel order (oid=%d): %v", order.Oid, err)
				continue
			}
			canceledCount++
		}
	}

	if canceledCount == 0 {
		log.Printf("  ℹ %s has no pending orders to cancel", symbol)
	} else {
		log.Printf("  ✓ Canceled %d pending orders for %s (including stop loss/take profit orders)", canceledCount, symbol)
	}

	return nil
}

// GetMarketPrice gets market price
func (t *HyperliquidTrader) GetMarketPrice(symbol string) (float64, error) {
	coin := convertSymbolToHyperliquid(symbol)

	// Get all market prices
	allMids, err := t.exchange.Info().AllMids(t.ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get price: %w", err)
	}

	// Find price for corresponding coin (allMids is map[string]string)
	if priceStr, ok := allMids[coin]; ok {
		priceFloat, err := strconv.ParseFloat(priceStr, 64)
		if err == nil {
			return priceFloat, nil
		}
		return 0, fmt.Errorf("price format error: %v", err)
	}

	return 0, fmt.Errorf("price not found for %s", symbol)
}

// SetStopLoss sets stop loss order
func (t *HyperliquidTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	coin := convertSymbolToHyperliquid(symbol)

	isBuy := positionSide == "SHORT" // Short position stop loss = buy, long position stop loss = sell

	// ⚠️ Critical: round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)

	// ⚠️ Critical: price also needs to be processed to 5 significant figures
	roundedStopPrice := t.roundPriceToSigfigs(stopPrice)

	// Create stop loss order (Trigger Order)
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: isBuy,
		Size:  roundedQuantity,  // Use rounded quantity
		Price: roundedStopPrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Trigger: &hyperliquid.TriggerOrderType{
				TriggerPx: roundedStopPrice,
				IsMarket:  true,
				Tpsl:      "sl", // stop loss
			},
		},
		ReduceOnly: true,
	}

	_, err := t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return fmt.Errorf("failed to set stop loss: %w", err)
	}

	log.Printf("  Stop loss price set: %.4f", roundedStopPrice)
	return nil
}

// SetTakeProfit sets take profit order
func (t *HyperliquidTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	coin := convertSymbolToHyperliquid(symbol)

	isBuy := positionSide == "SHORT" // Short position take profit = buy, long position take profit = sell

	// ⚠️ Critical: round quantity according to coin precision requirements
	roundedQuantity := t.roundToSzDecimals(coin, quantity)

	// ⚠️ Critical: price also needs to be processed to 5 significant figures
	roundedTakeProfitPrice := t.roundPriceToSigfigs(takeProfitPrice)

	// Create take profit order (Trigger Order)
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: isBuy,
		Size:  roundedQuantity,        // Use rounded quantity
		Price: roundedTakeProfitPrice, // Use processed price
		OrderType: hyperliquid.OrderType{
			Trigger: &hyperliquid.TriggerOrderType{
				TriggerPx: roundedTakeProfitPrice,
				IsMarket:  true,
				Tpsl:      "tp", // take profit
			},
		},
		ReduceOnly: true,
	}

	_, err := t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return fmt.Errorf("failed to set take profit: %w", err)
	}

	log.Printf("  Take profit price set: %.4f", roundedTakeProfitPrice)
	return nil
}

// GetOrderStatus Get order status from Hyperliquid API
// Note: Hyperliquid doesn't return order IDs, so orderID parameter is ignored
// This function queries open orders and matches by symbol
func (t *HyperliquidTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	coin := convertSymbolToHyperliquid(symbol)

	// Query open orders for this symbol
	openOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get open orders: %w", err)
	}

	// Find matching order by coin
	for _, order := range openOrders {
		if order.Coin == coin {
			// Get current price for avgPrice estimation
			price, err := t.GetMarketPrice(symbol)
			if err != nil {
				price = 0
			}

			// Hyperliquid doesn't provide avgPrice in open orders, use current price as fallback
			// For filled orders, we'd need to query user fills, but that's complex
			// This is a limitation of Hyperliquid API
			// Note: OpenOrder struct fields may vary by SDK version
			// Using 0 as fallback for executedQty since field may not be available
			return map[string]interface{}{
				"orderId":     fmt.Sprintf("%d", order.Oid),
				"symbol":      symbol,
				"status":      "NEW", // Open orders are NEW
				"avgPrice":    price, // Fallback to current price
				"executedQty": 0.0,   // Not available in open orders
				"commission":  0,     // Not available in open orders
			}, nil
		}
	}

	// Order not found in open orders - might be filled
	// For filled orders, we'd need to query user fills, but that's complex
	// Return error indicating order not found
	return nil, fmt.Errorf("order not found (Hyperliquid limitation: order IDs not available)")
}

// GetOrderHistory Get all orders (including filled) from Hyperliquid
// Note: Hyperliquid doesn't have a direct order history API, so we return empty
// Position closure detection will rely on position snapshots instead
func (t *HyperliquidTrader) GetOrderHistory(symbol string, limit int, startTime, endTime *time.Time) ([]map[string]interface{}, error) {
	// Hyperliquid doesn't provide order history API
	// We can only query open orders, not historical filled orders
	// Return empty slice - position closure detection will use position snapshots
	return []map[string]interface{}{}, nil
}

// GetUserTrades Get user trade history (executed trades) from Hyperliquid
// Note: Hyperliquid API doesn't provide easy access to historical trades
// Position closure detection will rely on position snapshots instead
func (t *HyperliquidTrader) GetUserTrades(symbol string, limit int, startTime, endTime *time.Time) ([]map[string]interface{}, error) {
	// Hyperliquid doesn't provide easy access to historical trades via SDK
	// Return empty slice - position closure detection will use position snapshots
	return []map[string]interface{}{}, nil
}

// GetOpenOrders Get all open (unfilled) orders from Hyperliquid
// Note: Hyperliquid SDK may not directly expose open orders API
func (t *HyperliquidTrader) GetOpenOrders(symbol string) ([]map[string]interface{}, error) {
	// Hyperliquid doesn't provide easy access to open orders via current SDK
	// Return empty slice for now - pending orders will show as empty
	// TODO: Implement when Hyperliquid SDK supports open orders query
	return []map[string]interface{}{}, nil
}

// FormatQuantity 格式化数量到正确的精度
func (t *HyperliquidTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	coin := convertSymbolToHyperliquid(symbol)
	szDecimals := t.getSzDecimals(coin)

	// 使用szDecimals格式化数量
	formatStr := fmt.Sprintf("%%.%df", szDecimals)
	return fmt.Sprintf(formatStr, quantity), nil
}

// getSzDecimals gets the quantity precision for a coin
func (t *HyperliquidTrader) getSzDecimals(coin string) int {
	// Refresh meta if needed (before acquiring read lock to avoid deadlock)
	if err := t.refreshMetaIfNeeded(coin); err != nil {
		log.Printf("⚠️  Failed to refresh meta information: %v, using default precision 4", err)
		// Continue with default precision if refresh fails
	}

	// ✅ Thread-safe: use read lock to protect meta field access
	t.metaMutex.RLock()
	defer t.metaMutex.RUnlock()

	if t.meta == nil {
		log.Printf("⚠️  Meta information is empty, using default precision 4")
		return 4 // Default precision
	}

	// Find corresponding coin in meta.Universe
	for _, asset := range t.meta.Universe {
		if asset.Name == coin {
			return asset.SzDecimals
		}
	}

	log.Printf("⚠️  Precision information not found for %s, using default precision 4", coin)
	return 4 // Default precision
}

// roundToSzDecimals rounds quantity to correct precision
func (t *HyperliquidTrader) roundToSzDecimals(coin string, quantity float64) float64 {
	szDecimals := t.getSzDecimals(coin)

	// 计算倍数（10^szDecimals）
	multiplier := 1.0
	for i := 0; i < szDecimals; i++ {
		multiplier *= 10.0
	}

	// 四舍五入
	return float64(int(quantity*multiplier+0.5)) / multiplier
}

// roundPriceToSigfigs rounds price to 5 significant figures
// Hyperliquid requires prices to use 5 significant figures
func (t *HyperliquidTrader) roundPriceToSigfigs(price float64) float64 {
	if price == 0 {
		return 0
	}

	const sigfigs = 5 // Hyperliquid standard: 5 significant figures

	// Calculate the magnitude of the price
	var magnitude float64
	if price < 0 {
		magnitude = -price
	} else {
		magnitude = price
	}

	// Calculate required multiplier
	multiplier := 1.0
	for magnitude >= 10 {
		magnitude /= 10
		multiplier /= 10
	}
	for magnitude < 1 {
		magnitude *= 10
		multiplier *= 10
	}

	// 应用有效数字精度
	for i := 0; i < sigfigs-1; i++ {
		multiplier *= 10
	}

	// 四舍五入
	rounded := float64(int(price*multiplier+0.5)) / multiplier
	return rounded
}

// convertSymbolToHyperliquid 将标准symbol转换为Hyperliquid格式
// 例如: "BTCUSDT" -> "BTC"
func convertSymbolToHyperliquid(symbol string) string {
	// 去掉USDT后缀
	if len(symbol) > 4 && symbol[len(symbol)-4:] == "USDT" {
		return symbol[:len(symbol)-4]
	}
	return symbol
}

// absFloat 返回浮点数的绝对值
func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
