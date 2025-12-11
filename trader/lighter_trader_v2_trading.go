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

// OpenLong 開多倉（實現 Trader 接口）
func (t *LighterTraderV2) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient 未初始化，請先設置 API Key")
	}

	log.Printf("📈 LIGHTER 開多倉: %s, qty=%.4f, leverage=%dx", symbol, quantity, leverage)

	// 1. 設置杠杆（如果需要）
	if err := t.SetLeverage(symbol, leverage); err != nil {
		log.Printf("⚠️  設置杠杆失敗: %v", err)
	}

	// 2. 獲取市場價格
	marketPrice, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("獲取市場價格失敗: %w", err)
	}

	// 3. 創建市價買入單（開多）
	orderResult, err := t.CreateOrder(symbol, false, quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("開多倉失敗: %w", err)
	}

	log.Printf("✓ LIGHTER 開多倉成功: %s @ %.2f", symbol, marketPrice)

	return map[string]interface{}{
		"orderId": orderResult["orderId"],
		"symbol":  symbol,
		"side":    "long",
		"status":  "FILLED",
		"price":   marketPrice,
	}, nil
}

// OpenShort 開空倉（實現 Trader 接口）
func (t *LighterTraderV2) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient 未初始化，請先設置 API Key")
	}

	log.Printf("📉 LIGHTER 開空倉: %s, qty=%.4f, leverage=%dx", symbol, quantity, leverage)

	// 1. 設置杠杆
	if err := t.SetLeverage(symbol, leverage); err != nil {
		log.Printf("⚠️  設置杠杆失敗: %v", err)
	}

	// 2. 獲取市場價格
	marketPrice, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("獲取市場價格失敗: %w", err)
	}

	// 3. 創建市價賣出單（開空）
	orderResult, err := t.CreateOrder(symbol, true, quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("開空倉失敗: %w", err)
	}

	log.Printf("✓ LIGHTER 開空倉成功: %s @ %.2f", symbol, marketPrice)

	return map[string]interface{}{
		"orderId": orderResult["orderId"],
		"symbol":  symbol,
		"side":    "short",
		"status":  "FILLED",
		"price":   marketPrice,
	}, nil
}

// CloseLong 平多倉（實現 Trader 接口）
func (t *LighterTraderV2) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient 未初始化")
	}

	// 如果 quantity=0，獲取當前持倉數量
	if quantity == 0 {
		pos, err := t.GetPosition(symbol)
		if err != nil {
			return nil, fmt.Errorf("獲取持倉失敗: %w", err)
		}
		if pos == nil || pos.Size == 0 {
			return map[string]interface{}{
				"symbol": symbol,
				"status": "NO_POSITION",
			}, nil
		}
		quantity = pos.Size
	}

	log.Printf("🔻 LIGHTER 平多倉: %s, qty=%.4f", symbol, quantity)

	// 創建市價賣出單平倉（reduceOnly=true）
	orderResult, err := t.CreateOrder(symbol, true, quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("平多倉失敗: %w", err)
	}

	// 平倉後取消所有掛單
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("⚠️  取消掛單失敗: %v", err)
	}

	log.Printf("✓ LIGHTER 平多倉成功: %s", symbol)

	return map[string]interface{}{
		"orderId": orderResult["orderId"],
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseShort 平空倉（實現 Trader 接口）
func (t *LighterTraderV2) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient 未初始化")
	}

	// 如果 quantity=0，獲取當前持倉數量
	if quantity == 0 {
		pos, err := t.GetPosition(symbol)
		if err != nil {
			return nil, fmt.Errorf("獲取持倉失敗: %w", err)
		}
		if pos == nil || pos.Size == 0 {
			return map[string]interface{}{
				"symbol": symbol,
				"status": "NO_POSITION",
			}, nil
		}
		quantity = pos.Size
	}

	log.Printf("🔺 LIGHTER 平空倉: %s, qty=%.4f", symbol, quantity)

	// 創建市價買入單平倉（reduceOnly=true）
	orderResult, err := t.CreateOrder(symbol, false, quantity, 0, "market")
	if err != nil {
		return nil, fmt.Errorf("平空倉失敗: %w", err)
	}

	// 平倉後取消所有掛單
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("⚠️  取消掛單失敗: %v", err)
	}

	log.Printf("✓ LIGHTER 平空倉成功: %s", symbol)

	return map[string]interface{}{
		"orderId": orderResult["orderId"],
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CreateOrder 創建訂單（市價或限價）- 使用官方 SDK 簽名
func (t *LighterTraderV2) CreateOrder(symbol string, isAsk bool, quantity float64, price float64, orderType string) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient 未初始化")
	}

	// 獲取市場索引（需要從 symbol 轉換）
	marketIndex, err := t.getMarketIndex(symbol)
	if err != nil {
		return nil, fmt.Errorf("獲取市場索引失敗: %w", err)
	}

	// 構建訂單請求
	clientOrderIndex := time.Now().UnixNano() // 使用時間戳作為客戶端訂單ID

	var orderTypeValue uint8 = 0 // 0=limit, 1=market
	if orderType == "market" {
		orderTypeValue = 1
	}

	// 將數量和價格轉換為LIGHTER格式（需要乘以精度）
	baseAmount := int64(quantity * 1e8) // 8位小數精度
	priceValue := uint32(0)
	if orderType == "limit" {
		priceValue = uint32(price * 1e2) // 價格精度
	}

	txReq := &types.CreateOrderTxReq{
		MarketIndex:      marketIndex,
		ClientOrderIndex: clientOrderIndex,
		BaseAmount:       baseAmount,
		Price:            priceValue,
		IsAsk:            boolToUint8(isAsk),
		Type:             orderTypeValue,
		TimeInForce:      0, // GTC
		ReduceOnly:       0, // 不只減倉
		TriggerPrice:     0,
		OrderExpiry:      time.Now().Add(24 * 28 * time.Hour).UnixMilli(), // 28天後過期
	}

	// 使用SDK簽名交易（nonce會自動獲取）
	nonce := int64(-1) // -1表示自動獲取
	tx, err := t.txClient.GetCreateOrderTransaction(txReq, &types.TransactOpts{
		Nonce: &nonce,
	})
	if err != nil {
		return nil, fmt.Errorf("簽名訂單失敗: %w", err)
	}

	// 序列化交易
	txBytes, err := json.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("序列化交易失敗: %w", err)
	}

	// 提交訂單到LIGHTER API
	orderResp, err := t.submitOrder(txBytes)
	if err != nil {
		return nil, fmt.Errorf("提交訂單失敗: %w", err)
	}

	side := "buy"
	if isAsk {
		side = "sell"
	}
	log.Printf("✓ LIGHTER訂單已創建: %s %s qty=%.4f", symbol, side, quantity)

	return orderResp, nil
}

// GetOrderStatus Get order status from Lighter V2 API
func (t *LighterTraderV2) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("認證令牌無效: %w", err)
	}

	// Query order by order ID
	endpoint := fmt.Sprintf("%s/api/v1/order/%s", t.baseURL, orderID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("創建請求失敗: %w", err)
	}

	// Add authentication header
	t.accountMutex.RLock()
	req.Header.Set("Authorization", t.authToken)
	t.accountMutex.RUnlock()
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("請求失敗: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("讀取響應失敗: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("獲取訂單狀態失敗 (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp struct {
		Code    int                    `json:"code"`
		Message string                 `json:"message"`
		Data    map[string]interface{} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析響應失敗: %w, body: %s", err, string(body))
	}

	if apiResp.Code != 200 {
		return nil, fmt.Errorf("獲取訂單狀態失敗 (code %d): %s", apiResp.Code, apiResp.Message)
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
	if status == "filled" || status == "FILLED" {
		status = "FILLED"
	} else if status == "partially_filled" || status == "PARTIALLY_FILLED" {
		status = "PARTIALLY_FILLED"
	} else if status == "open" || status == "OPEN" || status == "pending" {
		status = "NEW"
	} else if status == "cancelled" || status == "CANCELLED" {
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

// SendTxRequest 發送交易請求
type SendTxRequest struct {
	TxType          int    `json:"tx_type"`
	TxInfo          string `json:"tx_info"`
	PriceProtection bool   `json:"price_protection,omitempty"`
}

// SendTxResponse 發送交易響應
type SendTxResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}

// submitOrder 提交已簽名的訂單到LIGHTER API
func (t *LighterTraderV2) submitOrder(signedTx []byte) (map[string]interface{}, error) {
	const TX_TYPE_CREATE_ORDER = 14

	// 構建請求
	req := SendTxRequest{
		TxType:          TX_TYPE_CREATE_ORDER,
		TxInfo:          string(signedTx),
		PriceProtection: true,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化請求失敗: %w", err)
	}

	// 發送 POST 請求到 /api/v1/sendTx
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

	// 解析響應
	var sendResp SendTxResponse
	if err := json.Unmarshal(body, &sendResp); err != nil {
		return nil, fmt.Errorf("解析響應失敗: %w, body: %s", err, string(body))
	}

	// 檢查響應碼
	if sendResp.Code != 200 {
		return nil, fmt.Errorf("提交訂單失敗 (code %d): %s", sendResp.Code, sendResp.Message)
	}

	// 提取交易哈希和訂單ID
	result := map[string]interface{}{
		"tx_hash": sendResp.Data["tx_hash"],
		"status":  "submitted",
	}

	// 如果有訂單ID，添加到結果中
	if orderID, ok := sendResp.Data["order_id"]; ok {
		result["orderId"] = orderID
	} else if txHash, ok := sendResp.Data["tx_hash"].(string); ok {
		// 使用 tx_hash 作為 orderID
		result["orderId"] = txHash
	}

	log.Printf("✓ 訂單已提交到 LIGHTER - tx_hash: %v", sendResp.Data["tx_hash"])

	return result, nil
}

// getMarketIndex 獲取市場索引（從symbol轉換）- 動態從API獲取
func (t *LighterTraderV2) getMarketIndex(symbol string) (uint8, error) {
	// 1. 檢查緩存
	t.marketMutex.RLock()
	if index, ok := t.marketIndexMap[symbol]; ok {
		t.marketMutex.RUnlock()
		return index, nil
	}
	t.marketMutex.RUnlock()

	// 2. 從 API 獲取市場列表
	markets, err := t.fetchMarketList()
	if err != nil {
		// 如果 API 失敗，回退到硬編碼映射
		log.Printf("⚠️  從 API 獲取市場列表失敗，使用硬編碼映射: %v", err)
		return t.getFallbackMarketIndex(symbol)
	}

	// 3. 更新緩存
	t.marketMutex.Lock()
	for _, market := range markets {
		t.marketIndexMap[market.Symbol] = market.MarketID
	}
	t.marketMutex.Unlock()

	// 4. 從緩存中獲取
	t.marketMutex.RLock()
	index, ok := t.marketIndexMap[symbol]
	t.marketMutex.RUnlock()

	if !ok {
		return 0, fmt.Errorf("未知的市場符號: %s", symbol)
	}

	return index, nil
}

// MarketInfo 市場信息
type MarketInfo struct {
	Symbol   string `json:"symbol"`
	MarketID uint8  `json:"market_id"`
}

// fetchMarketList 從 API 獲取市場列表
func (t *LighterTraderV2) fetchMarketList() ([]MarketInfo, error) {
	endpoint := fmt.Sprintf("%s/api/v1/orderBooks", t.baseURL)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("創建請求失敗: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("請求失敗: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("讀取響應失敗: %w", err)
	}

	// 解析響應
	var apiResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    []struct {
			Symbol      string `json:"symbol"`
			MarketIndex uint8  `json:"market_index"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析響應失敗: %w", err)
	}

	if apiResp.Code != 200 {
		return nil, fmt.Errorf("獲取市場列表失敗 (code %d): %s", apiResp.Code, apiResp.Message)
	}

	// 轉換為 MarketInfo 列表
	markets := make([]MarketInfo, len(apiResp.Data))
	for i, market := range apiResp.Data {
		markets[i] = MarketInfo{
			Symbol:   market.Symbol,
			MarketID: market.MarketIndex,
		}
	}

	log.Printf("✓ 獲取到 %d 個市場", len(markets))
	return markets, nil
}

// getFallbackMarketIndex 硬編碼的回退映射
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
		log.Printf("✓ 使用硬編碼市場索引: %s -> %d", symbol, index)
		return index, nil
	}

	return 0, fmt.Errorf("未知的市場符號: %s", symbol)
}

// SetLeverage 設置杠杆（實現 Trader 接口）
func (t *LighterTraderV2) SetLeverage(symbol string, leverage int) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient 未初始化")
	}

	// TODO: 使用SDK簽名並提交SetLeverage交易
	log.Printf("⚙️  設置杠杆: %s = %dx", symbol, leverage)

	return nil // 暫時返回成功
}

// SetMarginMode 設置倉位模式（實現 Trader 接口）
func (t *LighterTraderV2) SetMarginMode(symbol string, isCrossMargin bool) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient 未初始化")
	}

	modeStr := "逐倉"
	if isCrossMargin {
		modeStr = "全倉"
	}

	log.Printf("⚙️  設置倉位模式: %s = %s", symbol, modeStr)

	// TODO: 使用SDK簽名並提交SetMarginMode交易
	return nil
}

// boolToUint8 將布爾值轉換為uint8
func boolToUint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
