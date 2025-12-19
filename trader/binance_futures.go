package trader

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"nofx/hook"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

// getBrOrderID generates unique order ID (for futures contracts)
// Format: x-{BR_ID}{TIMESTAMP}{RANDOM}
// Contract limit is 32 characters, use this limit uniformly for consistency
// Use nanosecond timestamp + random number to ensure global uniqueness (collision probability < 10^-20)
func getBrOrderID() string {
	brID := "KzrpZaP9" // Futures contract br ID

	// Calculate available space: 32 - len("x-KzrpZaP9") = 32 - 11 = 21 characters
	// Allocate: 13-digit timestamp + 8-digit random = 21 characters (perfect utilization)
	timestamp := time.Now().UnixNano() % 10000000000000 // 13-digit nanosecond timestamp

	// Generate 4-byte random number (8 hex digits)
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)

	// Format: x-KzrpZaP9{13-digit timestamp}{8-digit random}
	// Example: x-KzrpZaP91234567890123abcdef12 (exactly 31 characters)
	orderID := fmt.Sprintf("x-%s%d%s", brID, timestamp, randomHex)

	// Ensure not exceeding 32 character limit (theoretically exactly 31 characters)
	if len(orderID) > 32 {
		orderID = orderID[:32]
	}

	return orderID
}

// FuturesTrader Binance futures trader
type FuturesTrader struct {
	client *futures.Client

	// Balance cache
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// Position cache
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// Cache validity duration (15 seconds)
	cacheDuration time.Duration
}

// NewFuturesTrader Create futures trader
func NewFuturesTrader(apiKey, secretKey string, userId string) *FuturesTrader {
	client := futures.NewClient(apiKey, secretKey)

	hookRes := hook.HookExec[hook.NewBinanceTraderResult](hook.NEW_BINANCE_TRADER, userId, client)
	if hookRes != nil && hookRes.GetResult() != nil {
		client = hookRes.GetResult()
	}

	// 同步时间，避免 Timestamp ahead 错误
	syncBinanceServerTime(client)
	trader := &FuturesTrader{
		client:        client,
		cacheDuration: 15 * time.Second, // 15秒缓存
	}

	// Set dual-side position mode (Hedge Mode)
	// This is required because the code uses PositionSide (LONG/SHORT)
	if err := trader.setDualSidePosition(); err != nil {
		log.Printf("⚠️ Failed to set dual-side position mode: %v (ignore this warning if already in dual-side mode)", err)
	}

	return trader
}

// setDualSidePosition sets dual-side position mode (called during initialization)
func (t *FuturesTrader) setDualSidePosition() error {
	// Try to set dual-side position mode
	err := t.client.NewChangePositionModeService().
		DualSide(true). // true = dual-side position (Hedge Mode)
		Do(context.Background())

	if err != nil {
		// If error contains "No need to change", it means already in dual-side mode
		if strings.Contains(err.Error(), "No need to change position side") {
			log.Printf("  ✓ Account is already in dual-side position mode (Hedge Mode)")
			return nil
		}
		// Other errors are returned (but won't interrupt initialization at caller)
		return err
	}

	log.Printf("  ✓ Account switched to dual-side position mode (Hedge Mode)")
	log.Printf("  ℹ️  Dual-side mode allows holding both long and short positions simultaneously")
	return nil
}

// syncBinanceServerTime syncs Binance server time to ensure request timestamps are valid
func syncBinanceServerTime(client *futures.Client) {
	serverTime, err := client.NewServerTimeService().Do(context.Background())
	if err != nil {
		log.Printf("⚠️ Failed to sync Binance server time: %v", err)
		return
	}

	now := time.Now().UnixMilli()
	offset := now - serverTime
	client.TimeOffset = offset
	log.Printf("⏱ Binance server time synced, offset %dms", offset)
}

// GetBalance gets account balance (with cache)
func (t *FuturesTrader) GetBalance() (map[string]interface{}, error) {
	// First check if cache is valid
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.balanceCacheTime)
		t.balanceCacheMutex.RUnlock()
		log.Printf("✓ Using cached account balance (cached %.1f seconds ago)", cacheAge.Seconds())
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	// Cache expired or doesn't exist, call API
	log.Printf("🔄 Cache expired, calling Binance API to get account balance...")
	account, err := t.client.NewGetAccountService().Do(context.Background())
	if err != nil {
		log.Printf("❌ Binance API call failed: %v", err)
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	result := make(map[string]interface{})
	result["totalWalletBalance"], _ = strconv.ParseFloat(account.TotalWalletBalance, 64)
	result["availableBalance"], _ = strconv.ParseFloat(account.AvailableBalance, 64)
	result["totalUnrealizedProfit"], _ = strconv.ParseFloat(account.TotalUnrealizedProfit, 64)

	log.Printf("✓ Binance API returned: total balance=%s, available=%s, unrealized PnL=%s",
		account.TotalWalletBalance,
		account.AvailableBalance,
		account.TotalUnrealizedProfit)

	// Update cache
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions get all positions (with cache)
func (t *FuturesTrader) GetPositions() ([]map[string]interface{}, error) {
	// First check if cache is valid
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.positionsCacheTime)
		t.positionsCacheMutex.RUnlock()
		log.Printf("✓ Using cached position info (cache age: %.1fs)", cacheAge.Seconds())
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	// Cache expired or missing, call API
	log.Printf("🔄 Cache expired, calling Binance API to get position info...")
	positions, err := t.client.NewGetPositionRiskService().Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var result []map[string]interface{}
	// Track symbols to query trades for position closure detection
	symbolsToCheck := make(map[string]bool)

	for _, pos := range positions {
		posAmt, _ := strconv.ParseFloat(pos.PositionAmt, 64)
		if posAmt == 0 {
			// Position shows 0, but check recent trades to confirm closure
			symbolsToCheck[pos.Symbol] = true
			continue
		}

		posMap := make(map[string]interface{})
		posMap["symbol"] = pos.Symbol
		posMap["positionAmt"], _ = strconv.ParseFloat(pos.PositionAmt, 64)
		posMap["entryPrice"], _ = strconv.ParseFloat(pos.EntryPrice, 64)
		posMap["markPrice"], _ = strconv.ParseFloat(pos.MarkPrice, 64)
		posMap["unRealizedProfit"], _ = strconv.ParseFloat(pos.UnRealizedProfit, 64)
		posMap["leverage"], _ = strconv.ParseFloat(pos.Leverage, 64)
		posMap["liquidationPrice"], _ = strconv.ParseFloat(pos.LiquidationPrice, 64)

		// Determine direction
		if posAmt > 0 {
			posMap["side"] = "long"
		} else {
			posMap["side"] = "short"
		}

		result = append(result, posMap)
	}

	// Use trade queries to verify position closures (enhancement, not replacement)
	// If trade query fails, fall back to existing position data
	if len(symbolsToCheck) > 0 {
		log.Printf("⚠️ GetRecentTrades not yet implemented for Binance Futures (checking %d symbols), using position API only", len(symbolsToCheck))
	}
	for symbol := range symbolsToCheck {
		recentTrades, err := t.GetRecentTrades(symbol, 50)
		if err != nil {
			// Trade query failed, skip enhancement but don't break existing behavior
			log.Printf("⚠️ Failed to query trades for %s (using position API only): %v", symbol, err)
			continue
		}

		// Check if there are recent closing trades (opposite side trades)
		// This helps detect positions that were closed but position API hasn't updated yet
		hasRecentClosingTrades := false
		for _, trade := range recentTrades {
			// Check if trade is recent (within last 5 minutes) and is a closing trade
			if tradeTime, ok := trade["time"].(int64); ok {
				timeDiff := time.Now().UnixMilli() - tradeTime
				if timeDiff < 5*60*1000 { // Within 5 minutes
					hasRecentClosingTrades = true
					break
				}
			}
		}

		if hasRecentClosingTrades {
			log.Printf("✓ Detected recent closing trades for %s via trade query", symbol)
		}
	}

	// Update cache
	t.positionsCacheMutex.Lock()
	t.cachedPositions = result
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return result, nil
}

// SetMarginMode Set margin mode
func (t *FuturesTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	var marginType futures.MarginType
	if isCrossMargin {
		marginType = futures.MarginTypeCrossed
	} else {
		marginType = futures.MarginTypeIsolated
	}

	// Try to set margin mode
	err := t.client.NewChangeMarginTypeService().
		Symbol(symbol).
		MarginType(marginType).
		Do(context.Background())

	marginModeStr := "Cross"
	if !isCrossMargin {
		marginModeStr = "Isolated"
	}

	if err != nil {
		// If error contains "No need to change", margin mode is already the target value
		if contains(err.Error(), "No need to change margin type") {
			log.Printf("  ✓ %s margin mode is already %s", symbol, marginModeStr)
			return nil
		}
		// If there are positions, cannot change margin mode, but doesn't affect trading
		if contains(err.Error(), "Margin type cannot be changed if there exists position") {
			log.Printf("  ⚠️ %s has positions, cannot change margin mode, continuing with current mode", symbol)
			return nil
		}
		// Detect multi-asset mode (error code -4168)
		if contains(err.Error(), "Multi-Assets mode") || contains(err.Error(), "-4168") || contains(err.Error(), "4168") {
			log.Printf("  ⚠️ %s detected multi-asset mode, forcing cross margin mode", symbol)
			log.Printf("  💡 Tip: To use isolated margin mode, disable multi-asset mode in Binance")
			return nil
		}
		// Detect unified account API (Portfolio Margin)
		if contains(err.Error(), "unified") || contains(err.Error(), "portfolio") || contains(err.Error(), "Portfolio") {
			log.Printf("  ❌ %s detected unified account API, cannot trade futures", symbol)
			return fmt.Errorf("please use 'Spot & Futures Trading' API permission, not 'Unified Account API'")
		}
		log.Printf("  ⚠️ Failed to set margin mode: %v", err)
		// Don't return error, let trading continue
		return nil
	}

	log.Printf("  ✓ %s margin mode set to %s", symbol, marginModeStr)
	return nil
}

// SetLeverage Set leverage (smart detection + cooldown period)
func (t *FuturesTrader) SetLeverage(symbol string, leverage int) error {
	// First try to get current leverage (from position info)
	currentLeverage := 0
	positions, err := t.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == symbol {
				if lev, ok := pos["leverage"].(float64); ok {
					currentLeverage = int(lev)
					break
				}
			}
		}
	}

	// If current leverage is already the target leverage, skip
	if currentLeverage == leverage && currentLeverage > 0 {
		log.Printf("  ✓ %s leverage is already %dx, no need to switch", symbol, leverage)
		return nil
	}

	// Switch leverage
	_, err = t.client.NewChangeLeverageService().
		Symbol(symbol).
		Leverage(leverage).
		Do(context.Background())

	if err != nil {
		// If error contains "No need to change", leverage is already the target value
		if contains(err.Error(), "No need to change") {
			log.Printf("  ✓ %s leverage is already %dx", symbol, leverage)
			return nil
		}
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	log.Printf("  ✓ %s leverage switched to %dx", symbol, leverage)

	// Wait 5 seconds after switching leverage (to avoid cooldown errors)
	log.Printf("  ⏱ Waiting 5 seconds cooldown...")
	time.Sleep(5 * time.Second)

	return nil
}

// OpenLong Open long position
func (t *FuturesTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// Cancel all pending orders for this symbol (clean up old stop loss/take profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel old orders (may not exist): %v", err)
	}

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// Note: Margin mode should be set by caller (AutoTrader) before opening position via SetMarginMode

	// Format quantity to correct precision
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Check if formatted quantity is 0 (prevent rounding errors)
	quantityFloat, parseErr := strconv.ParseFloat(quantityStr, 64)
	if parseErr != nil || quantityFloat <= 0 {
		return nil, fmt.Errorf("opening quantity too small, formatted to 0 (original: %.8f → formatted: %s). Please increase position size or choose a lower-priced coin", quantity, quantityStr)
	}

	// Check minimum notional value (Binance requires at least 10 USDT)
	if err := t.CheckMinNotional(symbol, quantityFloat); err != nil {
		return nil, err
	}

	// Create market buy order (using br ID)
	order, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(futures.SideTypeBuy).
		PositionSide(futures.PositionSideTypeLong).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		NewClientOrderID(getBrOrderID()).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to open long position: %w", err)
	}

	log.Printf("✓ Long position opened successfully: %s quantity: %s", symbol, quantityStr)
	log.Printf("  Order ID: %d", order.OrderID)

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	return result, nil
}

// OpenShort Open short position
func (t *FuturesTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// Cancel all pending orders for this symbol (clean up old stop loss/take profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel old orders (may not exist): %v", err)
	}

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// Note: Margin mode should be set by caller (AutoTrader) before opening position via SetMarginMode

	// Format quantity to correct precision
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Check if formatted quantity is 0 (prevent rounding errors)
	quantityFloat, parseErr := strconv.ParseFloat(quantityStr, 64)
	if parseErr != nil || quantityFloat <= 0 {
		return nil, fmt.Errorf("opening quantity too small, formatted to 0 (original: %.8f → formatted: %s). Please increase position size or choose a lower-priced coin", quantity, quantityStr)
	}

	// Check minimum notional value (Binance requires at least 10 USDT)
	if err := t.CheckMinNotional(symbol, quantityFloat); err != nil {
		return nil, err
	}

	// Create market sell order (using br ID)
	order, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(futures.SideTypeSell).
		PositionSide(futures.PositionSideTypeShort).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		NewClientOrderID(getBrOrderID()).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to open short position: %w", err)
	}

	log.Printf("✓ Short position opened successfully: %s quantity: %s", symbol, quantityStr)
	log.Printf("  Order ID: %d", order.OrderID)

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	return result, nil
}

// CloseLong Close long position
func (t *FuturesTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
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

	// Format quantity
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Create market sell order (close long, using br ID)
	order, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(futures.SideTypeSell).
		PositionSide(futures.PositionSideTypeLong).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		NewClientOrderID(getBrOrderID()).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to close long position: %w", err)
	}

	log.Printf("✓ Long position closed successfully: %s quantity: %s", symbol, quantityStr)

	// Cancel all pending orders for this symbol after closing (stop loss/take profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel orders: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	return result, nil
}

// CloseShort Close short position
func (t *FuturesTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = -pos["positionAmt"].(float64) // Short position quantity is negative, take absolute value
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no short position found for %s", symbol)
		}
	}

	// Format quantity
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Create market buy order (close short, using br ID)
	order, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(futures.SideTypeBuy).
		PositionSide(futures.PositionSideTypeShort).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		NewClientOrderID(getBrOrderID()).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to close short position: %w", err)
	}

	log.Printf("✓ Short position closed successfully: %s quantity: %s", symbol, quantityStr)

	// Cancel all pending orders for this symbol after closing (stop loss/take profit orders)
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ Failed to cancel orders: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	return result, nil
}

// GetOrderStatus Get order status from Binance Futures API
func (t *FuturesTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	orderIDInt, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid order ID: %w", err)
	}

	order, err := t.client.NewGetOrderService().
		Symbol(symbol).
		OrderID(orderIDInt).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	avgPrice, _ := strconv.ParseFloat(order.AvgPrice, 64)
	executedQty, _ := strconv.ParseFloat(order.ExecutedQuantity, 64)
	// Binance Futures Order doesn't have Commission field directly
	// Commission is typically calculated from executed quantity and fee rate
	// For now, set to 0 - actual fee should be retrieved from trade history if needed
	commission := 0.0

	// Map Binance status to standard status
	status := order.Status

	return map[string]interface{}{
		"orderId":     order.OrderID,
		"symbol":      order.Symbol,
		"status":      status,
		"avgPrice":    avgPrice,
		"executedQty": executedQty,
		"side":        string(order.Side),
		"type":        string(order.Type),
		"time":        order.Time,
		"updateTime":  order.UpdateTime,
		"commission":  commission,
	}, nil
}

// GetRecentTrades Get recent trades from Binance Futures API for position closure detection
// Note: This is a placeholder implementation. The actual Binance Futures SDK may have different method names.
// For now, this returns an empty slice to avoid breaking existing code.
// Position closure detection will fall back to using position API only.
func (t *FuturesTrader) GetRecentTrades(symbol string, limit int) ([]map[string]interface{}, error) {
	// TODO: Implement using correct Binance Futures API method when available
	// For now, return empty slice - position closure detection will use position API only
	// Note: Warning logged at GetPositions level to avoid spam
	return []map[string]interface{}{}, nil
}

// CancelStopLossOrders cancels only stop-loss orders (doesn't affect take-profit orders)
func (t *FuturesTrader) CancelStopLossOrders(symbol string) error {
	canceledCount := 0
	var cancelErrors []error

	// Cancel regular stop-loss orders (legacy support)
	orders, err := t.client.NewListOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err == nil {
		for _, order := range orders {
			orderTypeStr := string(order.Type)
			// Check for stop-loss order types using string comparison (legacy orders)
			if orderTypeStr == "STOP_MARKET" || orderTypeStr == "STOP" {
				_, err := t.client.NewCancelOrderService().
					Symbol(symbol).
					OrderID(order.OrderID).
					Do(context.Background())

				if err != nil {
					errMsg := fmt.Sprintf("Order ID %d: %v", order.OrderID, err)
					cancelErrors = append(cancelErrors, fmt.Errorf("%s", errMsg))
					log.Printf("  ⚠ Failed to cancel stop-loss order: %s", errMsg)
					continue
				}

				canceledCount++
				log.Printf("  ✓ Canceled stop-loss order (Order ID: %d, Type: %s, Side: %s)", order.OrderID, orderTypeStr, order.PositionSide)
			}
		}
	} else {
		log.Printf("  ⚠ Failed to get regular orders: %v", err)
	}

	// Cancel algo stop-loss orders
	algoOrders, err := t.client.NewListOpenAlgoOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err == nil {
		for _, algoOrder := range algoOrders {
			// Check for stop-loss algo order types
			if algoOrder.OrderType == futures.AlgoOrderTypeStopMarket || algoOrder.OrderType == futures.AlgoOrderTypeStop {
				_, err := t.client.NewCancelAlgoOrderService().
					AlgoID(algoOrder.AlgoId).
					Do(context.Background())

				if err != nil {
					errMsg := fmt.Sprintf("Algo ID %d: %v", algoOrder.AlgoId, err)
					cancelErrors = append(cancelErrors, fmt.Errorf("%s", errMsg))
					log.Printf("  ⚠ Failed to cancel stop-loss algo order: %s", errMsg)
					continue
				}

				canceledCount++
				log.Printf("  ✓ Canceled stop-loss algo order (Algo ID: %d, Type: %s, Side: %s)", algoOrder.AlgoId, algoOrder.OrderType, algoOrder.PositionSide)
			}
		}
	} else {
		log.Printf("  ⚠ Failed to get algo orders: %v", err)
	}

	if canceledCount == 0 && len(cancelErrors) == 0 {
		log.Printf("  ℹ %s has no stop-loss orders to cancel", symbol)
	} else if canceledCount > 0 {
		log.Printf("  ✓ Canceled %d stop-loss orders for %s", canceledCount, symbol)
	}

	// If all cancellations failed, return error
	if len(cancelErrors) > 0 && canceledCount == 0 {
		return fmt.Errorf("failed to cancel stop-loss orders: %v", cancelErrors)
	}

	return nil
}

// CancelTakeProfitOrders cancels only take-profit orders (doesn't affect stop-loss orders)
func (t *FuturesTrader) CancelTakeProfitOrders(symbol string) error {
	canceledCount := 0
	var cancelErrors []error

	// Cancel regular take-profit orders (legacy support)
	orders, err := t.client.NewListOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err == nil {
		for _, order := range orders {
			orderTypeStr := string(order.Type)
			// Check for take-profit order types using string comparison (legacy orders)
			if orderTypeStr == "TAKE_PROFIT_MARKET" || orderTypeStr == "TAKE_PROFIT" {
				_, err := t.client.NewCancelOrderService().
					Symbol(symbol).
					OrderID(order.OrderID).
					Do(context.Background())

				if err != nil {
					errMsg := fmt.Sprintf("Order ID %d: %v", order.OrderID, err)
					cancelErrors = append(cancelErrors, fmt.Errorf("%s", errMsg))
					log.Printf("  ⚠ Failed to cancel take-profit order: %s", errMsg)
					continue
				}

				canceledCount++
				log.Printf("  ✓ Canceled take-profit order (Order ID: %d, Type: %s, Side: %s)", order.OrderID, orderTypeStr, order.PositionSide)
			}
		}
	} else {
		log.Printf("  ⚠ Failed to get regular orders: %v", err)
	}

	// Cancel algo take-profit orders
	algoOrders, err := t.client.NewListOpenAlgoOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err == nil {
		for _, algoOrder := range algoOrders {
			// Check for take-profit algo order types
			if algoOrder.OrderType == futures.AlgoOrderTypeTakeProfitMarket || algoOrder.OrderType == futures.AlgoOrderTypeTakeProfit {
				_, err := t.client.NewCancelAlgoOrderService().
					AlgoID(algoOrder.AlgoId).
					Do(context.Background())

				if err != nil {
					errMsg := fmt.Sprintf("Algo ID %d: %v", algoOrder.AlgoId, err)
					cancelErrors = append(cancelErrors, fmt.Errorf("%s", errMsg))
					log.Printf("  ⚠ Failed to cancel take-profit algo order: %s", errMsg)
					continue
				}

				canceledCount++
				log.Printf("  ✓ Canceled take-profit algo order (Algo ID: %d, Type: %s, Side: %s)", algoOrder.AlgoId, algoOrder.OrderType, algoOrder.PositionSide)
			}
		}
	} else {
		log.Printf("  ⚠ Failed to get algo orders: %v", err)
	}

	if canceledCount == 0 && len(cancelErrors) == 0 {
		log.Printf("  ℹ %s has no take-profit orders to cancel", symbol)
	} else if canceledCount > 0 {
		log.Printf("  ✓ Canceled %d take-profit orders for %s", canceledCount, symbol)
	}

	// If all cancellations failed, return error
	if len(cancelErrors) > 0 && canceledCount == 0 {
		return fmt.Errorf("failed to cancel take-profit orders: %v", cancelErrors)
	}

	return nil
}

// CancelAllOrders cancels all pending orders for this symbol
func (t *FuturesTrader) CancelAllOrders(symbol string) error {
	err := t.client.NewCancelAllOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to cancel orders: %w", err)
	}

	log.Printf("  ✓ Canceled all pending orders for %s", symbol)
	return nil
}

// CancelStopOrders cancels stop-loss/take-profit orders for this symbol (used to adjust stop positions)
func (t *FuturesTrader) CancelStopOrders(symbol string) error {
	canceledCount := 0

	// Cancel regular stop-loss/take-profit orders (legacy support)
	orders, err := t.client.NewListOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err == nil {
		for _, order := range orders {
			orderTypeStr := string(order.Type)
			// Check for stop-loss and take-profit order types using string comparison (legacy orders)
			if orderTypeStr == "STOP_MARKET" ||
				orderTypeStr == "TAKE_PROFIT_MARKET" ||
				orderTypeStr == "STOP" ||
				orderTypeStr == "TAKE_PROFIT" {

				_, err := t.client.NewCancelOrderService().
					Symbol(symbol).
					OrderID(order.OrderID).
					Do(context.Background())

				if err != nil {
					log.Printf("  ⚠ Failed to cancel order %d: %v", order.OrderID, err)
					continue
				}

				canceledCount++
				log.Printf("  ✓ Canceled stop-loss/take-profit order for %s (Order ID: %d, Type: %s)",
					symbol, order.OrderID, orderTypeStr)
			}
		}
	} else {
		log.Printf("  ⚠ Failed to get regular orders: %v", err)
	}

	// Cancel algo stop-loss/take-profit orders
	algoOrders, err := t.client.NewListOpenAlgoOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err == nil {
		for _, algoOrder := range algoOrders {
			// Check for stop-loss and take-profit algo order types
			if algoOrder.OrderType == futures.AlgoOrderTypeStopMarket ||
				algoOrder.OrderType == futures.AlgoOrderTypeTakeProfitMarket ||
				algoOrder.OrderType == futures.AlgoOrderTypeStop ||
				algoOrder.OrderType == futures.AlgoOrderTypeTakeProfit {

				_, err := t.client.NewCancelAlgoOrderService().
					AlgoID(algoOrder.AlgoId).
					Do(context.Background())

				if err != nil {
					log.Printf("  ⚠ Failed to cancel algo order %d: %v", algoOrder.AlgoId, err)
					continue
				}

				canceledCount++
				log.Printf("  ✓ Canceled stop-loss/take-profit algo order for %s (Algo ID: %d, Type: %s)",
					symbol, algoOrder.AlgoId, algoOrder.OrderType)
			}
		}
	} else {
		log.Printf("  ⚠ Failed to get algo orders: %v", err)
	}

	if canceledCount == 0 {
		log.Printf("  ℹ %s has no stop-loss/take-profit orders to cancel", symbol)
	} else {
		log.Printf("  ✓ Canceled %d stop-loss/take-profit orders for %s", canceledCount, symbol)
	}

	return nil
}

// GetMarketPrice gets market price
func (t *FuturesTrader) GetMarketPrice(symbol string) (float64, error) {
	prices, err := t.client.NewListPricesService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return 0, fmt.Errorf("failed to get price: %w", err)
	}

	if len(prices) == 0 {
		return 0, fmt.Errorf("price not found")
	}

	price, err := strconv.ParseFloat(prices[0].Price, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}

// CalculatePositionSize Calculate position size
func (t *FuturesTrader) CalculatePositionSize(balance, riskPercent, price float64, leverage int) float64 {
	riskAmount := balance * (riskPercent / 100.0)
	positionValue := riskAmount * float64(leverage)
	quantity := positionValue / price
	return quantity
}

// SetStopLoss Set stop loss order
func (t *FuturesTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	var side futures.SideType
	var posSide futures.PositionSideType

	if positionSide == "LONG" {
		side = futures.SideTypeSell
		posSide = futures.PositionSideTypeLong
	} else {
		side = futures.SideTypeBuy
		posSide = futures.PositionSideTypeShort
	}

	// 格式化数量
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	_, err = t.client.NewCreateAlgoOrderService().
		Symbol(symbol).
		Side(side).
		PositionSide(posSide).
		Type(futures.AlgoOrderTypeStopMarket).
		TriggerPrice(fmt.Sprintf("%.8f", stopPrice)).
		Quantity(quantityStr).
		WorkingType(futures.WorkingTypeContractPrice).
		ClosePosition(true).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to set stop-loss: %w", err)
	}

	log.Printf("  Stop-loss price set: %.4f", stopPrice)
	return nil
}

// SetTakeProfit sets take-profit order
func (t *FuturesTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	var side futures.SideType
	var posSide futures.PositionSideType

	if positionSide == "LONG" {
		side = futures.SideTypeSell
		posSide = futures.PositionSideTypeLong
	} else {
		side = futures.SideTypeBuy
		posSide = futures.PositionSideTypeShort
	}

	// 格式化数量
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	_, err = t.client.NewCreateAlgoOrderService().
		Symbol(symbol).
		Side(side).
		PositionSide(posSide).
		Type(futures.AlgoOrderTypeTakeProfitMarket).
		TriggerPrice(fmt.Sprintf("%.8f", takeProfitPrice)).
		Quantity(quantityStr).
		WorkingType(futures.WorkingTypeContractPrice).
		ClosePosition(true).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to set take-profit: %w", err)
	}

	log.Printf("  Take-profit price set: %.4f", takeProfitPrice)
	return nil
}

// GetMinNotional gets minimum notional value (Binance requirement)
func (t *FuturesTrader) GetMinNotional(symbol string) float64 {
	// Use conservative default value of 10 USDT to ensure orders pass exchange validation
	return 10.0
}

// CheckMinNotional checks if order meets minimum notional value requirement
func (t *FuturesTrader) CheckMinNotional(symbol string, quantity float64) error {
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return fmt.Errorf("failed to get market price: %w", err)
	}

	notionalValue := quantity * price
	minNotional := t.GetMinNotional(symbol)

	if notionalValue < minNotional {
		return fmt.Errorf(
			"order value %.2f USDT is below minimum requirement %.2f USDT (quantity: %.4f, price: %.4f)",
			notionalValue, minNotional, quantity, price,
		)
	}

	return nil
}

// GetSymbolPrecision gets quantity precision for trading pair
func (t *FuturesTrader) GetSymbolPrecision(symbol string) (int, error) {
	exchangeInfo, err := t.client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		return 0, fmt.Errorf("failed to get exchange rules: %w", err)
	}

	for _, s := range exchangeInfo.Symbols {
		if s.Symbol == symbol {
			// Get precision from LOT_SIZE filter
			for _, filter := range s.Filters {
				if filter["filterType"] == "LOT_SIZE" {
					stepSize := filter["stepSize"].(string)
					precision := calculatePrecision(stepSize)
					log.Printf("  %s quantity precision: %d (stepSize: %s)", symbol, precision, stepSize)
					return precision, nil
				}
			}
		}
	}

	log.Printf("  ⚠ %s precision info not found, using default precision 3", symbol)
	return 3, nil // Default precision is 3
}

// calculatePrecision Calculate precision from stepSize
func calculatePrecision(stepSize string) int {
	// Remove trailing zeros
	stepSize = trimTrailingZeros(stepSize)

	// Find decimal point
	dotIndex := -1
	for i := 0; i < len(stepSize); i++ {
		if stepSize[i] == '.' {
			dotIndex = i
			break
		}
	}

	// If no decimal point or decimal point at the end, precision is 0
	if dotIndex == -1 || dotIndex == len(stepSize)-1 {
		return 0
	}

	// Return number of digits after decimal point
	return len(stepSize) - dotIndex - 1
}

// trimTrailingZeros Remove trailing zeros
func trimTrailingZeros(s string) string {
	// If no decimal point, return directly
	if !stringContains(s, ".") {
		return s
	}

	// Traverse from back to front, remove trailing zeros
	for len(s) > 0 && s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}

	// 如果最后一位是小数点，也去掉
	if len(s) > 0 && s[len(s)-1] == '.' {
		s = s[:len(s)-1]
	}

	return s
}

// FormatQuantity 格式化数量到正确的精度
func (t *FuturesTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	precision, err := t.GetSymbolPrecision(symbol)
	if err != nil {
		// 如果获取失败，使用默认格式
		return fmt.Sprintf("%.3f", quantity), nil
	}

	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, quantity), nil
}

// 辅助函数
func contains(s, substr string) bool {
	return len(s) >= len(substr) && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
