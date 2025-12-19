package trader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/elliottech/lighter-go/types"
)

// OrderResponse represents an order from Lighter API
type OrderResponse struct {
	OrderID   string  `json:"order_id"`   // Order ID (as string for compatibility)
	Symbol    string  `json:"symbol"`     // Trading pair symbol
	Side      string  `json:"side"`       // "buy" or "sell"
	Quantity  float64 `json:"quantity"`   // Order quantity
	Price     float64 `json:"price"`      // Order price
	Status    string  `json:"status"`     // Order status
	OrderType string  `json:"order_type"` // "limit" or "market"
}

// SetStopLoss sets stop loss order (implements Trader interface)
func (t *LighterTraderV2) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	log.Printf("🛑 LIGHTER setting stop loss: %s %s qty=%.4f, stop=%.2f", symbol, positionSide, quantity, stopPrice)

	// Determine order direction (short stop loss uses buy order, long stop loss uses sell order)
	isAsk := (positionSide == "LONG" || positionSide == "long")

	// Create limit stop loss order
	_, err := t.CreateOrder(symbol, isAsk, quantity, stopPrice, "limit")
	if err != nil {
		return fmt.Errorf("failed to set stop loss: %w", err)
	}

	log.Printf("✓ LIGHTER stop loss set: %.2f", stopPrice)
	return nil
}

// SetTakeProfit sets take profit order (implements Trader interface)
func (t *LighterTraderV2) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	log.Printf("🎯 LIGHTER setting take profit: %s %s qty=%.4f, tp=%.2f", symbol, positionSide, quantity, takeProfitPrice)

	// Determine order direction (short take profit uses buy order, long take profit uses sell order)
	isAsk := (positionSide == "LONG" || positionSide == "long")

	// Create limit take profit order
	_, err := t.CreateOrder(symbol, isAsk, quantity, takeProfitPrice, "limit")
	if err != nil {
		return fmt.Errorf("failed to set take profit: %w", err)
	}

	log.Printf("✓ LIGHTER take profit set: %.2f", takeProfitPrice)
	return nil
}

// CancelAllOrders cancels all orders (implements Trader interface)
func (t *LighterTraderV2) CancelAllOrders(symbol string) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	if err := t.ensureAuthToken(); err != nil {
		return fmt.Errorf("authentication token invalid: %w", err)
	}

	// Get all active orders
	orders, err := t.GetActiveOrders(symbol)
	if err != nil {
		return fmt.Errorf("failed to get active orders: %w", err)
	}

	if len(orders) == 0 {
		log.Printf("✓ LIGHTER - No orders to cancel (no active orders)")
		return nil
	}

	// Cancel in batch
	canceledCount := 0
	for _, order := range orders {
		if err := t.CancelOrder(symbol, order.OrderID); err != nil {
			log.Printf("⚠️ Failed to cancel order (ID: %s): %v", order.OrderID, err)
		} else {
			canceledCount++
		}
	}

	log.Printf("✓ LIGHTER - Canceled %d orders", canceledCount)
	return nil
}

// CancelStopLossOrders cancels only stop loss orders (implements Trader interface)
func (t *LighterTraderV2) CancelStopLossOrders(symbol string) error {
	// LIGHTER currently cannot distinguish between stop loss and take profit orders, cancel all stop/take profit orders
	log.Printf("⚠️ LIGHTER cannot distinguish stop loss/take profit orders, will cancel all stop/take profit orders")
	return t.CancelStopOrders(symbol)
}

// CancelTakeProfitOrders cancels only take profit orders (implements Trader interface)
func (t *LighterTraderV2) CancelTakeProfitOrders(symbol string) error {
	// LIGHTER currently cannot distinguish between stop loss and take profit orders, cancel all stop/take profit orders
	log.Printf("⚠️ LIGHTER cannot distinguish stop loss/take profit orders, will cancel all stop/take profit orders")
	return t.CancelStopOrders(symbol)
}

// CancelStopOrders cancels stop loss/take profit orders for this symbol (implements Trader interface)
func (t *LighterTraderV2) CancelStopOrders(symbol string) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	if err := t.ensureAuthToken(); err != nil {
		return fmt.Errorf("authentication token invalid: %w", err)
	}

	// Get active orders
	orders, err := t.GetActiveOrders(symbol)
	if err != nil {
		return fmt.Errorf("failed to get active orders: %w", err)
	}

	canceledCount := 0
	for _, order := range orders {
		// TODO: Check order type, only cancel stop loss/take profit orders
		// Currently canceling all orders
		if err := t.CancelOrder(symbol, order.OrderID); err != nil {
			log.Printf("⚠️ Failed to cancel order (ID: %s): %v", order.OrderID, err)
		} else {
			canceledCount++
		}
	}

	log.Printf("✓ LIGHTER - Canceled %d stop loss/take profit orders", canceledCount)
	return nil
}

// GetActiveOrders gets active orders
func (t *LighterTraderV2) GetActiveOrders(symbol string) ([]OrderResponse, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("authentication token invalid: %w", err)
	}

	// Get market index
	marketIndex, err := t.getMarketIndex(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market index: %w", err)
	}

	// Build request URL
	endpoint := fmt.Sprintf("%s/api/v1/accountActiveOrders?account_index=%d&market_id=%d",
		t.baseURL, t.accountIndex, marketIndex)

	// Send GET request
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Set("Authorization", t.authToken)
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
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    []OrderResponse `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, body: %s", err, string(body))
	}

	if apiResp.Code != 200 {
		return nil, fmt.Errorf("failed to get active orders (code %d): %s", apiResp.Code, apiResp.Message)
	}

	log.Printf("✓ LIGHTER - Retrieved %d active orders", len(apiResp.Data))
	return apiResp.Data, nil
}

// CancelOrder cancels a single order
func (t *LighterTraderV2) CancelOrder(symbol, orderID string) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	// Get market index
	marketIndex, err := t.getMarketIndex(symbol)
	if err != nil {
		return fmt.Errorf("failed to get market index: %w", err)
	}

	// Convert orderID to int64
	orderIndex, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid order ID: %w", err)
	}

	// Build cancel order request
	txReq := &types.CancelOrderTxReq{
		MarketIndex: marketIndex,
		Index:       orderIndex,
	}

	// Use SDK to sign transaction
	nonce := int64(-1) // -1 means auto-fetch
	tx, err := t.txClient.GetCancelOrderTransaction(txReq, &types.TransactOpts{
		Nonce: &nonce,
	})
	if err != nil {
		return fmt.Errorf("failed to sign cancel order: %w", err)
	}

	// Serialize transaction
	txBytes, err := json.Marshal(tx)
	if err != nil {
		return fmt.Errorf("failed to serialize transaction: %w", err)
	}

	// Submit cancel order to LIGHTER API
	_, err = t.submitCancelOrder(txBytes)
	if err != nil {
		return fmt.Errorf("failed to submit cancel order: %w", err)
	}

	log.Printf("✓ LIGHTER order canceled - ID: %s", orderID)
	return nil
}

// submitCancelOrder submits signed cancel order to LIGHTER API
func (t *LighterTraderV2) submitCancelOrder(signedTx []byte) (map[string]interface{}, error) {
	const TX_TYPE_CANCEL_ORDER = 15

	// Build request
	req := SendTxRequest{
		TxType:          TX_TYPE_CANCEL_ORDER,
		TxInfo:          string(signedTx),
		PriceProtection: false, // Cancel order doesn't need price protection
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
		return nil, fmt.Errorf("failed to submit cancel order (code %d): %s", sendResp.Code, sendResp.Message)
	}

	result := map[string]interface{}{
		"tx_hash": sendResp.Data["tx_hash"],
		"status":  "cancelled",
	}

	log.Printf("✓ Cancel order submitted to LIGHTER - tx_hash: %v", sendResp.Data["tx_hash"])
	return result, nil
}
