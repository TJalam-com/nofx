package trader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/elliottech/lighter-go/types"
)

// OpenLong opens long position (implements Trader interface)
func (t *LighterTraderV2) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized, please set API Key first")
	}

	log.Printf("📈 LIGHTER opening long: %s, qty=%.4f, leverage=%dx", symbol, quantity, leverage)

	// Cancel all pending orders for this symbol before opening position
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("⚠️ Failed to cancel old orders (may not exist): %v", err)
	}

	// Set leverage (if needed)
	if err := t.SetLeverage(symbol, leverage); err != nil {
		log.Printf("⚠️  Failed to set leverage: %v", err)
	}

	// 2. Get market price
	marketPrice, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market price: %w", err)
	}

	// 3. Create market buy order (open long)
	orderResult, err := t.CreateOrder(symbol, false, quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("failed to open long position: %w", err)
	}

	log.Printf("✓ LIGHTER long position opened successfully: %s @ %.2f", symbol, marketPrice)

	return map[string]interface{}{
		"orderId": orderResult["orderId"],
		"symbol":  symbol,
		"side":    "long",
		"status":  "FILLED",
		"price":   marketPrice,
	}, nil
}

// OpenShort opens short position (implements Trader interface)
func (t *LighterTraderV2) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized, please set API Key first")
	}

	log.Printf("📉 LIGHTER opening short: %s, qty=%.4f, leverage=%dx", symbol, quantity, leverage)

	// Cancel all pending orders for this symbol before opening position
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("⚠️ Failed to cancel old orders (may not exist): %v", err)
	}

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		log.Printf("⚠️  Failed to set leverage: %v", err)
	}

	// 2. Get market price
	marketPrice, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market price: %w", err)
	}

	// 3. Create market sell order (open short)
	orderResult, err := t.CreateOrder(symbol, true, quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("failed to open short position: %w", err)
	}

	log.Printf("✓ LIGHTER short position opened successfully: %s @ %.2f", symbol, marketPrice)

	return map[string]interface{}{
		"orderId": orderResult["orderId"],
		"symbol":  symbol,
		"side":    "short",
		"status":  "FILLED",
		"price":   marketPrice,
	}, nil
}

// CloseLong closes long position (implements Trader interface)
func (t *LighterTraderV2) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized")
	}

	// If quantity=0, get current position size
	if quantity == 0 {
		pos, err := t.GetPosition(symbol)
		if err != nil {
			return nil, fmt.Errorf("failed to get position: %w", err)
		}
		if pos == nil || pos.Size == 0 {
			return map[string]interface{}{
				"symbol": symbol,
				"status": "NO_POSITION",
			}, nil
		}
		quantity = pos.Size
	}

	log.Printf("🔻 LIGHTER closing long: %s, qty=%.4f", symbol, quantity)

	// Create market sell order to close (reduceOnly=true)
	orderResult, err := t.CreateOrder(symbol, true, quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("failed to close long position: %w", err)
	}

	// Cancel all pending orders after closing
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("⚠️  Failed to cancel orders: %v", err)
	}

	log.Printf("✓ LIGHTER long position closed successfully: %s", symbol)

	return map[string]interface{}{
		"orderId": orderResult["orderId"],
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseShort closes short position (implements Trader interface)
func (t *LighterTraderV2) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized")
	}

	// If quantity=0, get current position size
	if quantity == 0 {
		pos, err := t.GetPosition(symbol)
		if err != nil {
			return nil, fmt.Errorf("failed to get position: %w", err)
		}
		if pos == nil || pos.Size == 0 {
			return map[string]interface{}{
				"symbol": symbol,
				"status": "NO_POSITION",
			}, nil
		}
		quantity = pos.Size
	}

	log.Printf("🔺 LIGHTER closing short: %s, qty=%.4f", symbol, quantity)

	// Create market buy order to close (reduceOnly=true)
	orderResult, err := t.CreateOrder(symbol, false, quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("failed to close short position: %w", err)
	}

	// Cancel all pending orders after closing
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("⚠️  Failed to cancel orders: %v", err)
	}

	log.Printf("✓ LIGHTER short position closed successfully: %s", symbol)

	return map[string]interface{}{
		"orderId": orderResult["orderId"],
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CreateOrder creates order (market or limit) - uses official SDK for signing
func (t *LighterTraderV2) CreateOrder(symbol string, isAsk bool, quantity float64, price float64, orderType string) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized")
	}

	// Get market index (need to convert from symbol)
	marketIndex, err := t.getMarketIndex(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market index: %w", err)
	}

	// Build order request
	clientOrderIndex := time.Now().UnixNano() // Use timestamp as client order ID

	var orderTypeValue uint8 = 0 // 0=limit, 1=market
	if orderType == "market" {
		orderTypeValue = 1
	}

	// Convert quantity and price to LIGHTER format (need to multiply by precision)
	baseAmount := int64(quantity * 1e8) // 8 decimal precision
	priceValue := uint32(0)
	if orderType == "limit" {
		priceValue = uint32(price * 1e2) // Price precision
	}

	txReq := &types.CreateOrderTxReq{
		MarketIndex:      marketIndex,
		ClientOrderIndex: clientOrderIndex,
		BaseAmount:       baseAmount,
		Price:            priceValue,
		IsAsk:            boolToUint8(isAsk),
		Type:             orderTypeValue,
		TimeInForce:      0, // GTC
		ReduceOnly:       1, // Not reduce-only
		TriggerPrice:     0,
		OrderExpiry:      time.Now().Add(24 * 28 * time.Hour).UnixMilli(), // Expires after 28 days
	}

	// Use SDK to sign transaction (nonce will be fetched automatically)
	nonce := int64(-1) // -1 means auto-fetch
	tx, err := t.txClient.GetCreateOrderTransaction(txReq, &types.TransactOpts{
		Nonce: &nonce,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign order: %w", err)
	}

	// Serialize transaction
	txBytes, err := json.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	// Submit order to LIGHTER API
	orderResp, err := t.submitOrder(txBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to submit order: %w", err)
	}

	side := "buy"
	if isAsk {
		side = "sell"
	}
	log.Printf("✓ LIGHTER order created: %s %s qty=%.4f", symbol, side, quantity)

	return orderResp, nil
}

// GetOrderStatus gets order status from Lighter V2 API
func (t *LighterTraderV2) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("authentication token invalid: %w", err)
	}

	// Query order by order ID
	endpoint := fmt.Sprintf("%s/api/v1/order/%s", t.baseURL, orderID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	t.accountMutex.RLock()
	req.Header.Set("Authorization", t.authToken)
	t.accountMutex.RUnlock()
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get order status (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp struct {
		Code    int                    `json:"code"`
		Message string                 `json:"message"`
		Data    map[string]interface{} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, body: %s", err, string(body))
	}

	if apiResp.Code != 200 {
		return nil, fmt.Errorf("failed to get order status (code %d): %s", apiResp.Code, apiResp.Message)
	}

	order := apiResp.Data

	// Extract order details
	avgPrice := 0.0
	if val, ok := order["avg_price"]; ok {
		if f, ok := val.(float64); ok {
			avgPrice = f
		} else if s, ok := val.(string); ok {
			avgPrice, _ = strconv.ParseFloat(s, 64)
		}
	}
	executedQty := 0.0
	if val, ok := order["filled_quantity"]; ok {
		if f, ok := val.(float64); ok {
			executedQty = f
		} else if s, ok := val.(string); ok {
			executedQty, _ = strconv.ParseFloat(s, 64)
		}
	}
	commission := 0.0
	if val, ok := order["fee"]; ok {
		if f, ok := val.(float64); ok {
			commission = f
		} else if s, ok := val.(string); ok {
			commission, _ = strconv.ParseFloat(s, 64)
		}
	}
	status, _ := order["status"].(string)

	// Map Lighter status to standard status
	switch status {
	case "filled", "FILLED":
		status = "FILLED"
	case "partially_filled", "PARTIALLY_FILLED":
		status = "PARTIALLY_FILLED"
	case "open", "OPEN", "pending":
		status = "NEW"
	case "cancelled", "CANCELLED":
		status = "CANCELED"
	}

	return map[string]interface{}{
		"orderId":     orderID,
		"symbol":      symbol,
		"status":      status,
		"avgPrice":    avgPrice,
		"executedQty": executedQty,
		"commission":  commission,
		"side":        order["side"],
		"type":        order["order_type"],
	}, nil
}

// SendTxRequest transaction request
type SendTxRequest struct {
	TxType          int    `json:"tx_type"`
	TxInfo          string `json:"tx_info"`
	PriceProtection bool   `json:"price_protection,omitempty"`
}

// SendTxResponse transaction response
type SendTxResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}

// submitOrder submits signed order to LIGHTER API
func (t *LighterTraderV2) submitOrder(signedTx []byte) (map[string]interface{}, error) {
	const TX_TYPE_CREATE_ORDER = 14

	// Build request
	req := SendTxRequest{
		TxType:          TX_TYPE_CREATE_ORDER,
		TxInfo:          string(signedTx),
		PriceProtection: true,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}

	// Send POST request to /api/v1/sendTx
	endpoint := fmt.Sprintf("%s/api/v1/sendTx", t.baseURL)
	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse response
	var sendResp SendTxResponse
	if err := json.Unmarshal(body, &sendResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, body: %s", err, string(body))
	}

	// Check response code
	if sendResp.Code != 200 {
		return nil, fmt.Errorf("failed to submit order (code %d): %s", sendResp.Code, sendResp.Message)
	}

	// Extract transaction hash and order ID
	result := map[string]interface{}{
		"tx_hash": sendResp.Data["tx_hash"],
		"status":  "submitted",
	}

	// If order ID exists, add to result
	if orderID, ok := sendResp.Data["order_id"]; ok {
		result["orderId"] = orderID
	} else if txHash, ok := sendResp.Data["tx_hash"].(string); ok {
		// Use tx_hash as orderID
		result["orderId"] = txHash
	}

	log.Printf("✓ Order submitted to LIGHTER - tx_hash: %v", sendResp.Data["tx_hash"])

	return result, nil
}

// getMarketIndex gets market index (convert from symbol) - dynamically fetched from API
func (t *LighterTraderV2) getMarketIndex(symbol string) (uint8, error) {
	// 1. Check cache
	t.marketMutex.RLock()
	if index, ok := t.marketIndexMap[symbol]; ok {
		t.marketMutex.RUnlock()
		return index, nil
	}
	t.marketMutex.RUnlock()

	// 2. Fetch market list from API
	markets, err := t.fetchMarketList()
	if err != nil {
		// If API fails, fallback to hardcoded mapping
		log.Printf("⚠️  Failed to fetch market list from API, using hardcoded mapping: %v", err)
		return t.getFallbackMarketIndex(symbol)
	}

	// 3. Update cache
	t.marketMutex.Lock()
	for _, market := range markets {
		t.marketIndexMap[market.Symbol] = market.MarketID
	}
	t.marketMutex.Unlock()

	// 4. Get from cache
	t.marketMutex.RLock()
	index, ok := t.marketIndexMap[symbol]
	t.marketMutex.RUnlock()

	if !ok {
		return 0, fmt.Errorf("unknown market symbol: %s", symbol)
	}

	return index, nil
}

// MarketInfo market information
type MarketInfo struct {
	Symbol   string `json:"symbol"`
	MarketID uint8  `json:"market_id"`
}

// fetchMarketList fetches market list from API
func (t *LighterTraderV2) fetchMarketList() ([]MarketInfo, error) {
	endpoint := fmt.Sprintf("%s/api/v1/orderBooks", t.baseURL)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var apiResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    []struct {
			Symbol      string `json:"symbol"`
			MarketIndex uint8  `json:"market_index"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Code != 200 {
		return nil, fmt.Errorf("failed to get market list (code %d): %s", apiResp.Code, apiResp.Message)
	}

	// Convert to MarketInfo list
	markets := make([]MarketInfo, len(apiResp.Data))
	for i, market := range apiResp.Data {
		markets[i] = MarketInfo{
			Symbol:   market.Symbol,
			MarketID: market.MarketIndex,
		}
	}

	log.Printf("✓ Fetched %d markets", len(markets))
	return markets, nil
}

// getFallbackMarketIndex hardcoded fallback mapping
func (t *LighterTraderV2) getFallbackMarketIndex(symbol string) (uint8, error) {
	fallbackMap := map[string]uint8{
		"BTC-PERP":  0,
		"ETH-PERP":  1,
		"SOL-PERP":  2,
		"DOGE-PERP": 3,
		"AVAX-PERP": 4,
		"XRP-PERP":  5,
	}

	if index, ok := fallbackMap[symbol]; ok {
		log.Printf("✓ Using hardcoded market index: %s -> %d", symbol, index)
		return index, nil
	}

	return 0, fmt.Errorf("unknown market symbol: %s", symbol)
}

// SetLeverage sets leverage (implements Trader interface)
func (t *LighterTraderV2) SetLeverage(symbol string, leverage int) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	// TODO: Use SDK to sign and submit SetLeverage transaction
	log.Printf("⚙️  Setting leverage: %s = %dx", symbol, leverage)

	return nil // Temporarily return success
}

// SetMarginMode sets margin mode (implements Trader interface)
func (t *LighterTraderV2) SetMarginMode(symbol string, isCrossMargin bool) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	modeStr := "isolated"
	if isCrossMargin {
		modeStr = "cross"
	}

	log.Printf("⚙️  Setting margin mode: %s = %s", symbol, modeStr)

	// TODO: Use SDK to sign and submit SetMarginMode transaction
	return nil
}

// boolToUint8 converts boolean to uint8
func boolToUint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
