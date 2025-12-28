package trader

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Bitget API endpoints
const (
	bitgetBaseURL           = "https://api.bitget.com"
	bitgetAccountPath       = "/api/v2/mix/account/accounts"
	bitgetPositionPath      = "/api/v2/mix/position/all-position"
	bitgetOrderPath         = "/api/v2/mix/order/place-order"
	bitgetLeveragePath      = "/api/v2/mix/account/set-leverage"
	bitgetTickerPath        = "/api/v2/mix/market/ticker"
	bitgetContractsPath     = "/api/v2/mix/market/contracts"
	bitgetCancelOrderPath   = "/api/v2/mix/order/cancel-order"
	bitgetPendingOrdersPath = "/api/v2/mix/order/orders-pending"
	bitgetPositionModePath  = "/api/v2/mix/account/set-position-mode"
	bitgetCancelAllPath     = "/api/v2/mix/order/cancel-all-orders"
)

// BitgetTrader Bitget futures trader
type BitgetTrader struct {
	apiKey     string
	secretKey  string
	passphrase string

	// HTTP client (with proxy support)
	httpClient *http.Client

	// Balance cache
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// Position cache
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// Contract info cache
	contractsCache      map[string]*BitgetContract
	contractsCacheTime  time.Time
	contractsCacheMutex sync.RWMutex

	// Cache validity duration
	cacheDuration time.Duration
}

// BitgetContract Bitget contract information
type BitgetContract struct {
	Symbol         string  // Trading symbol
	SizeMultiplier float64 // Contract size multiplier
	MinSize        float64 // Minimum order quantity
	PricePlace     int     // Price decimal places
	SizePlace      int     // Size decimal places
}

// BitgetResponse Bitget API response
type BitgetResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// genBitgetClientOid Generate Bitget client order ID
func genBitgetClientOid() string {
	timestamp := time.Now().UnixNano() % 10000000000000
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)
	// Bitget clientOid maximum 32 characters
	orderID := fmt.Sprintf("nofx%d%s", timestamp, randomHex)
	if len(orderID) > 32 {
		orderID = orderID[:32]
	}
	return orderID
}

// NewBitgetTrader creates Bitget trader
func NewBitgetTrader(apiKey, secretKey, passphrase string) *BitgetTrader {
	trader := &BitgetTrader{
		apiKey:         apiKey,
		secretKey:      secretKey,
		passphrase:     passphrase,
		httpClient:     http.DefaultClient,
		cacheDuration:  15 * time.Second,
		contractsCache: make(map[string]*BitgetContract),
	}

	// Set one-way position mode
	if err := trader.setPositionMode(); err != nil {
		logger.Infof("⚠️ Failed to set Bitget position mode: %v (if already one-way mode, ignore)", err)
	}

	logger.Infof("🟣 [Bitget] Trader initialized")

	return trader
}

// setPositionMode Set one-way position mode
func (t *BitgetTrader) setPositionMode() error {
	body := map[string]string{
		"productType": "USDT-FUTURES",
		"posMode":     "one_way_mode",
	}

	_, err := t.doRequest("POST", bitgetPositionModePath, body)
	if err != nil {
		// If already in one-way mode, ignore error
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "not modified") {
			logger.Infof("  ✓ Bitget account is already in one-way position mode")
			return nil
		}
		return err
	}

	logger.Infof("  ✓ Bitget account changed to one-way position mode")
	return nil
}

// sign Generate Bitget API signature
func (t *BitgetTrader) sign(timestamp, method, requestPath, body string) string {
	preHash := timestamp + method + requestPath + body
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(preHash))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// doRequest Execute HTTP request
func (t *BitgetTrader) doRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	timestamp := strconv.FormatInt(time.Now().Unix()*1000, 10)
	signature := t.sign(timestamp, method, path, string(bodyBytes))

	req, err := http.NewRequest(method, bitgetBaseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("ACCESS-KEY", t.apiKey)
	req.Header.Set("ACCESS-SIGN", signature)
	req.Header.Set("ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("ACCESS-PASSPHRASE", t.passphrase)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var bitgetResp BitgetResponse
	if err := json.Unmarshal(respBody, &bitgetResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// code=00000 means success
	if bitgetResp.Code != "00000" {
		return nil, fmt.Errorf("bitget API error: code=%s, msg=%s", bitgetResp.Code, bitgetResp.Msg)
	}

	return bitgetResp.Data, nil
}

// GetBalance Get account balance
func (t *BitgetTrader) GetBalance() (map[string]interface{}, error) {
	// Check cache
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		t.balanceCacheMutex.RUnlock()
		logger.Infof("✓ Using cached Bitget account balance")
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	logger.Infof("🔄 Calling Bitget API to get account balance...")
	data, err := t.doRequest("GET", bitgetAccountPath+"?productType=USDT-FUTURES", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get account balance: %w", err)
	}

	var accounts []struct {
		MarginCoin       string `json:"marginCoin"`
		Available        string `json:"available"`
		Equity           string `json:"equity"`
		UnrealizedPL     string `json:"unrealizedPL"`
		MarginMode       string `json:"marginMode"`
		CrossedAvailable string `json:"crossedAvailable"`
	}

	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, fmt.Errorf("failed to parse balance data: %w", err)
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("no balance data found")
	}

	account := accounts[0]
	equity, _ := strconv.ParseFloat(account.Equity, 64)
	available, _ := strconv.ParseFloat(account.Available, 64)
	unrealizedPL, _ := strconv.ParseFloat(account.UnrealizedPL, 64)

	result := map[string]interface{}{
		"totalEquity":           equity,
		"totalWalletBalance":    equity,
		"availableBalance":      available,
		"totalUnrealizedProfit": unrealizedPL,
		"balance":               equity,
	}

	logger.Infof("✓ Bitget balance: total=%.2f, available=%.2f, unrealized=%.2f", equity, available, unrealizedPL)

	// Update cache
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions Get all positions
func (t *BitgetTrader) GetPositions() ([]map[string]interface{}, error) {
	// Check cache
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		t.positionsCacheMutex.RUnlock()
		logger.Infof("✓ Using cached Bitget position info")
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	logger.Infof("🔄 Calling Bitget API to get positions...")
	data, err := t.doRequest("GET", bitgetPositionPath+"?productType=USDT-FUTURES", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var positions []struct {
		Symbol           string `json:"symbol"`
		HoldSide         string `json:"holdSide"` // long or short
		TotalQty         string `json:"totalQty"`
		AvailableQty     string `json:"availableQty"`
		AvgOpenPrice     string `json:"avgOpenPrice"`
		Leverage         string `json:"leverage"`
		UnrealizedPL     string `json:"unrealizedPL"`
		LiquidationPrice string `json:"liquidationPrice"`
		MarkPrice        string `json:"markPrice"`
	}

	if err := json.Unmarshal(data, &positions); err != nil {
		return nil, fmt.Errorf("failed to parse position data: %w", err)
	}

	var result []map[string]interface{}

	for _, pos := range positions {
		totalQty, _ := strconv.ParseFloat(pos.TotalQty, 64)

		// Skip empty positions
		if totalQty == 0 {
			continue
		}

		entryPrice, _ := strconv.ParseFloat(pos.AvgOpenPrice, 64)
		unrealizedPL, _ := strconv.ParseFloat(pos.UnrealizedPL, 64)
		leverage, _ := strconv.ParseFloat(pos.Leverage, 64)
		markPrice, _ := strconv.ParseFloat(pos.MarkPrice, 64)
		liqPrice, _ := strconv.ParseFloat(pos.LiquidationPrice, 64)

		// Convert to unified format
		side := "LONG"
		positionAmt := totalQty
		if pos.HoldSide == "short" {
			side = "SHORT"
			positionAmt = -totalQty
		}

		position := map[string]interface{}{
			"symbol":           pos.Symbol,
			"side":             side,
			"positionAmt":      positionAmt,
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"unRealizedProfit": unrealizedPL,
			"unrealizedPnL":    unrealizedPL,
			"liquidationPrice": liqPrice,
			"leverage":         leverage,
		}

		result = append(result, position)
	}

	// Update cache
	t.positionsCacheMutex.Lock()
	t.cachedPositions = result
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return result, nil
}

// getContract Get contract information
func (t *BitgetTrader) getContract(symbol string) (*BitgetContract, error) {
	// Check cache
	t.contractsCacheMutex.RLock()
	if contract, ok := t.contractsCache[symbol]; ok {
		t.contractsCacheMutex.RUnlock()
		return contract, nil
	}
	t.contractsCacheMutex.RUnlock()

	data, err := t.doRequest("GET", bitgetContractsPath+"?productType=USDT-FUTURES", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract info: %w", err)
	}

	var contracts []struct {
		Symbol         string `json:"symbol"`
		SizeMultiplier string `json:"sizeMultiplier"`
		MinSize        string `json:"minSize"`
		PricePlace     string `json:"pricePlace"`
		SizePlace      string `json:"sizePlace"`
	}

	if err := json.Unmarshal(data, &contracts); err != nil {
		return nil, err
	}

	for _, c := range contracts {
		if c.Symbol == symbol {
			sizeMultiplier, _ := strconv.ParseFloat(c.SizeMultiplier, 64)
			minSize, _ := strconv.ParseFloat(c.MinSize, 64)
			pricePlace, _ := strconv.Atoi(c.PricePlace)
			sizePlace, _ := strconv.Atoi(c.SizePlace)

			contract := &BitgetContract{
				Symbol:         c.Symbol,
				SizeMultiplier: sizeMultiplier,
				MinSize:        minSize,
				PricePlace:     pricePlace,
				SizePlace:      sizePlace,
			}

			// Update cache
			t.contractsCacheMutex.Lock()
			t.contractsCache[symbol] = contract
			t.contractsCacheTime = time.Now()
			t.contractsCacheMutex.Unlock()

			return contract, nil
		}
	}

	return nil, fmt.Errorf("contract not found: %s", symbol)
}

// SetMarginMode Set margin mode
func (t *BitgetTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	marginMode := "isolated"
	if isCrossMargin {
		marginMode = "crossed"
	}

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  marginMode,
	}

	_, err := t.doRequest("POST", "/api/v2/mix/account/set-margin-mode", body)
	if err != nil {
		// If already in target mode, ignore error
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "not modified") {
			logger.Infof("  ✓ %s margin mode is already %s", symbol, marginMode)
			return nil
		}
		// Has positions, cannot change
		if strings.Contains(err.Error(), "position") {
			logger.Infof("  ⚠️ %s has positions, cannot change margin mode", symbol)
			return nil
		}
		return err
	}

	logger.Infof("  ✓ %s margin mode set to %s", symbol, marginMode)
	return nil
}

// SetLeverage Set leverage
func (t *BitgetTrader) SetLeverage(symbol string, leverage int) error {
	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
		"leverage":    strconv.Itoa(leverage),
	}

	_, err := t.doRequest("POST", bitgetLeveragePath, body)
	if err != nil {
		// If already same leverage, ignore
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "not modified") {
			logger.Infof("  ✓ %s leverage is already %dx", symbol, leverage)
			return nil
		}
		return err
	}

	logger.Infof("  ✓ %s leverage set to %dx", symbol, leverage)
	return nil
}

// OpenLong Open long position
func (t *BitgetTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// Set leverage first
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("  ⚠️ Failed to set leverage: %v", err)
	}

	// Get contract info for quantity formatting
	contract, err := t.getContract(symbol)
	if err != nil {
		logger.Infof("  ⚠️ Failed to get contract info: %v, using default formatting", err)
	}

	// Format quantity
	qtyStr := t.formatQuantity(quantity, contract)

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  "crossed",
		"marginCoin":  "USDT",
		"size":        qtyStr,
		"side":        "buy", // Long position
		"orderType":   "market",
		"force":       "gtc",
		"clientOid":   genBitgetClientOid(),
	}

	data, err := t.doRequest("POST", bitgetOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to open long position: %w", err)
	}

	var order struct {
		OrderId   string `json:"orderId"`
		ClientOid string `json:"clientOid"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	logger.Infof("✓ Bitget open long position successful: %s size: %s", symbol, qtyStr)
	logger.Infof("  Order ID: %s", order.OrderId)

	// Clear cache
	t.clearCache()

	return map[string]interface{}{
		"orderId": order.OrderId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// OpenShort Open short position
func (t *BitgetTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// Set leverage first
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("  ⚠️ Failed to set leverage: %v", err)
	}

	// Get contract info for quantity formatting
	contract, err := t.getContract(symbol)
	if err != nil {
		logger.Infof("  ⚠️ Failed to get contract info: %v, using default formatting", err)
	}

	// Format quantity
	qtyStr := t.formatQuantity(quantity, contract)

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  "crossed",
		"marginCoin":  "USDT",
		"size":        qtyStr,
		"side":        "sell", // Short position
		"orderType":   "market",
		"force":       "gtc",
		"clientOid":   genBitgetClientOid(),
	}

	data, err := t.doRequest("POST", bitgetOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to open short position: %w", err)
	}

	var order struct {
		OrderId   string `json:"orderId"`
		ClientOid string `json:"clientOid"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	logger.Infof("✓ Bitget open short position successful: %s size: %s", symbol, qtyStr)
	logger.Infof("  Order ID: %s", order.OrderId)

	// Clear cache
	t.clearCache()

	return map[string]interface{}{
		"orderId": order.OrderId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseLong Close long position
func (t *BitgetTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity = 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}
		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "LONG" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}
	}

	if quantity <= 0 {
		return nil, fmt.Errorf("no long position to close")
	}

	// Get contract info for quantity formatting
	contract, err := t.getContract(symbol)
	if err != nil {
		logger.Infof("  ⚠️ Failed to get contract info: %v, using default formatting", err)
	}

	// Format quantity
	qtyStr := t.formatQuantity(quantity, contract)

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  "crossed",
		"marginCoin":  "USDT",
		"size":        qtyStr,
		"side":        "sell", // Close long with sell
		"orderType":   "market",
		"force":       "gtc",
		"reduceOnly":  true,
		"clientOid":   genBitgetClientOid(),
	}

	data, err := t.doRequest("POST", bitgetOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to close long position: %w", err)
	}

	var order struct {
		OrderId   string `json:"orderId"`
		ClientOid string `json:"clientOid"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	logger.Infof("✓ Bitget close long position successful: %s size: %s", symbol, qtyStr)

	// Clear cache
	t.clearCache()

	return map[string]interface{}{
		"orderId": order.OrderId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseShort Close short position
func (t *BitgetTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity = 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}
		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "SHORT" {
				quantity = -pos["positionAmt"].(float64) // Short position is negative
				break
			}
		}
	}

	if quantity <= 0 {
		return nil, fmt.Errorf("no short position to close")
	}

	// Get contract info for quantity formatting
	contract, err := t.getContract(symbol)
	if err != nil {
		logger.Infof("  ⚠️ Failed to get contract info: %v, using default formatting", err)
	}

	// Format quantity
	qtyStr := t.formatQuantity(quantity, contract)

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  "crossed",
		"marginCoin":  "USDT",
		"size":        qtyStr,
		"side":        "buy", // Close short with buy
		"orderType":   "market",
		"force":       "gtc",
		"reduceOnly":  true,
		"clientOid":   genBitgetClientOid(),
	}

	data, err := t.doRequest("POST", bitgetOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to close short position: %w", err)
	}

	var order struct {
		OrderId   string `json:"orderId"`
		ClientOid string `json:"clientOid"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	logger.Infof("✓ Bitget close short position successful: %s size: %s", symbol, qtyStr)

	// Clear cache
	t.clearCache()

	return map[string]interface{}{
		"orderId": order.OrderId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// GetMarketPrice Get market price
func (t *BitgetTrader) GetMarketPrice(symbol string) (float64, error) {
	data, err := t.doRequest("GET", bitgetTickerPath+"?symbol="+symbol+"&productType=USDT-FUTURES", nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get market price: %w", err)
	}

	var tickers []struct {
		Last string `json:"last"`
	}

	if err := json.Unmarshal(data, &tickers); err != nil {
		return 0, fmt.Errorf("failed to parse ticker data: %w", err)
	}

	if len(tickers) == 0 {
		return 0, fmt.Errorf("price data not found for %s", symbol)
	}

	price, err := strconv.ParseFloat(tickers[0].Last, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse price: %w", err)
	}

	return price, nil
}

// SetStopLoss Set stop loss order
func (t *BitgetTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	// Determine holdSide based on position side
	holdSide := "long"
	if positionSide == "SHORT" {
		holdSide = "short"
	}

	// Get contract info for quantity formatting
	contract, err := t.getContract(symbol)
	if err != nil {
		logger.Infof("  ⚠️ Failed to get contract info: %v, using default formatting", err)
	}

	// Format quantity
	qtyStr := t.formatQuantity(quantity, contract)
	triggerPriceStr := fmt.Sprintf("%.8f", stopPrice)

	body := map[string]interface{}{
		"marginCoin":   "USDT",
		"productType":  "usdt-futures",
		"symbol":       symbol,
		"planType":     "loss_plan",
		"triggerPrice": triggerPriceStr,
		"triggerType":  "fill_price",
		"executePrice": "0",
		"holdSide":     holdSide,
		"size":         qtyStr,
		"clientOid":    genBitgetClientOid(),
	}

	_, err = t.doRequest("POST", "/api/v2/mix/order/place-tpsl-order", body)
	if err != nil {
		return fmt.Errorf("failed to set stop loss: %w", err)
	}

	logger.Infof("  ✓ [Bitget] Stop loss order set: %s @ %.2f", symbol, stopPrice)
	return nil
}

// SetTakeProfit Set take profit order
func (t *BitgetTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	// Determine holdSide based on position side
	holdSide := "long"
	if positionSide == "SHORT" {
		holdSide = "short"
	}

	// Get contract info for quantity formatting
	contract, err := t.getContract(symbol)
	if err != nil {
		logger.Infof("  ⚠️ Failed to get contract info: %v, using default formatting", err)
	}

	// Format quantity
	qtyStr := t.formatQuantity(quantity, contract)
	triggerPriceStr := fmt.Sprintf("%.8f", takeProfitPrice)

	body := map[string]interface{}{
		"marginCoin":   "USDT",
		"productType":  "usdt-futures",
		"symbol":       symbol,
		"planType":     "profit_plan",
		"triggerPrice": triggerPriceStr,
		"triggerType":  "fill_price",
		"executePrice": "0",
		"holdSide":     holdSide,
		"size":         qtyStr,
		"clientOid":    genBitgetClientOid(),
	}

	_, err = t.doRequest("POST", "/api/v2/mix/order/place-tpsl-order", body)
	if err != nil {
		return fmt.Errorf("failed to set take profit: %w", err)
	}

	logger.Infof("  ✓ [Bitget] Take profit order set: %s @ %.2f", symbol, takeProfitPrice)
	return nil
}

// CancelStopLossOrders Cancel stop loss orders
func (t *BitgetTrader) CancelStopLossOrders(symbol string) error {
	return t.cancelTPSLOrders(symbol, "stop_loss")
}

// CancelTakeProfitOrders Cancel take profit orders
func (t *BitgetTrader) CancelTakeProfitOrders(symbol string) error {
	return t.cancelTPSLOrders(symbol, "take_profit")
}

// CancelAllOrders Cancel all pending orders
func (t *BitgetTrader) CancelAllOrders(symbol string) error {
	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
	}

	_, err := t.doRequest("POST", bitgetCancelAllPath, body)
	if err != nil {
		return fmt.Errorf("failed to cancel all orders: %w", err)
	}

	return nil
}

// CancelStopOrders Cancel all stop loss and take profit orders
func (t *BitgetTrader) CancelStopOrders(symbol string) error {
	if err := t.CancelStopLossOrders(symbol); err != nil {
		logger.Infof("⚠️ [Bitget] Failed to cancel stop loss orders: %v", err)
	}
	if err := t.CancelTakeProfitOrders(symbol); err != nil {
		logger.Infof("⚠️ [Bitget] Failed to cancel take profit orders: %v", err)
	}
	return nil
}

// FormatQuantity Format quantity
func (t *BitgetTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	contract, err := t.getContract(symbol)
	if err != nil {
		// Fallback to default formatting
		return fmt.Sprintf("%.8f", quantity), nil
	}

	return t.formatQuantity(quantity, contract), nil
}

// formatQuantity Format quantity using contract info
func (t *BitgetTrader) formatQuantity(quantity float64, contract *BitgetContract) string {
	if contract == nil {
		return fmt.Sprintf("%.8f", quantity)
	}

	// Use sizePlace for decimal places
	decimals := contract.SizePlace
	if decimals < 0 {
		decimals = 8
	}
	if decimals > 8 {
		decimals = 8
	}

	format := fmt.Sprintf("%%.%df", decimals)
	return fmt.Sprintf(format, quantity)
}

// GetOrderStatus Get order status from Bitget API
func (t *BitgetTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	data, err := t.doRequest("GET", "/api/v2/mix/order/detail?orderId="+orderID+"&symbol="+symbol+"&productType=USDT-FUTURES", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	var order struct {
		OrderId   string `json:"orderId"`
		Symbol    string `json:"symbol"`
		Status    string `json:"status"`
		AvgPrice  string `json:"avgPrice"`
		FilledQty string `json:"filledQty"`
		Fee       string `json:"fee"`
		Side      string `json:"side"`
		OrderType string `json:"orderType"`
	}

	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order data: %w", err)
	}

	avgPrice, _ := strconv.ParseFloat(order.AvgPrice, 64)
	executedQty, _ := strconv.ParseFloat(order.FilledQty, 64)
	commission, _ := strconv.ParseFloat(order.Fee, 64)

	// Map Bitget status to standard status
	status := strings.ToUpper(order.Status)
	if status == "FILLED" {
		status = "FILLED"
	} else if status == "PARTIAL_FILLED" {
		status = "PARTIALLY_FILLED"
	} else if status == "NEW" {
		status = "NEW"
	} else if status == "CANCELED" {
		status = "CANCELED"
	}

	return map[string]interface{}{
		"orderId":     order.OrderId,
		"symbol":      symbol,
		"status":      status,
		"avgPrice":    avgPrice,
		"executedQty": executedQty,
		"commission":  commission,
		"side":        order.Side,
		"type":        order.OrderType,
	}, nil
}

// GetOrderHistory Get all orders (including filled) from Bitget API
func (t *BitgetTrader) GetOrderHistory(symbol string, limit int, startTime, endTime *time.Time) ([]map[string]interface{}, error) {
	path := "/api/v2/mix/order/orders-history"
	params := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
	}

	if limit > 0 {
		params["pageSize"] = limit
	} else {
		params["pageSize"] = 50
	}
	if startTime != nil {
		params["startTime"] = startTime.UnixMilli()
	}
	if endTime != nil {
		params["endTime"] = endTime.UnixMilli()
	}

	data, err := t.doRequest("GET", path, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get order history: %w", err)
	}

	var response struct {
		Data struct {
			Orders []struct {
				OrderId    string `json:"orderId"`
				Symbol     string `json:"symbol"`
				Status     string `json:"status"`
				AvgPrice   string `json:"avgPrice"`
				FilledQty  string `json:"filledQty"`
				Price      string `json:"price"`
				StopPrice  string `json:"stopPrice"`
				Size       string `json:"size"`
				Side       string `json:"side"`
				OrderType  string `json:"orderType"`
				CTime      string `json:"cTime"`
				UTime      string `json:"uTime"`
				PosSide    string `json:"posSide"`
			} `json:"orders"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse order history: %w", err)
	}

	result := make([]map[string]interface{}, 0, len(response.Data.Orders))
	for _, order := range response.Data.Orders {
		avgPrice, _ := strconv.ParseFloat(order.AvgPrice, 64)
		executedQty, _ := strconv.ParseFloat(order.FilledQty, 64)
		price, _ := strconv.ParseFloat(order.Price, 64)
		stopPrice, _ := strconv.ParseFloat(order.StopPrice, 64)
		origQty, _ := strconv.ParseFloat(order.Size, 64)
		cTime, _ := strconv.ParseInt(order.CTime, 10, 64)
		uTime, _ := strconv.ParseInt(order.UTime, 10, 64)

		status := strings.ToUpper(order.Status)
		if status == "FILLED" {
			status = "FILLED"
		}

		result = append(result, map[string]interface{}{
			"orderId":      order.OrderId,
			"symbol":       order.Symbol,
			"status":       status,
			"type":         order.OrderType,
			"side":         order.Side,
			"positionSide": order.PosSide,
			"avgPrice":     avgPrice,
			"executedQty":  executedQty,
			"price":        price,
			"stopPrice":    stopPrice,
			"origQty":      origQty,
			"time":         cTime,
			"updateTime":   uTime,
		})
	}

	return result, nil
}

// GetUserTrades Get user trade history (executed trades) from Bitget API
func (t *BitgetTrader) GetUserTrades(symbol string, limit int, startTime, endTime *time.Time) ([]map[string]interface{}, error) {
	path := "/api/v2/mix/order/fills"
	params := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
	}

	if limit > 0 {
		params["pageSize"] = limit
	} else {
		params["pageSize"] = 50
	}
	if startTime != nil {
		params["startTime"] = startTime.UnixMilli()
	}
	if endTime != nil {
		params["endTime"] = endTime.UnixMilli()
	}

	data, err := t.doRequest("GET", path, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get user trades: %w", err)
	}

	var response struct {
		Data []struct {
			TradeId string `json:"tradeId"`
			OrderId string `json:"orderId"`
			Symbol  string `json:"symbol"`
			Price   string `json:"price"`
			Size    string `json:"size"`
			Fee     string `json:"fee"`
			Side    string `json:"side"`
			CTime   string `json:"cTime"`
			PosSide string `json:"posSide"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse user trades: %w", err)
	}

	result := make([]map[string]interface{}, 0, len(response.Data))
	for _, trade := range response.Data {
		price, _ := strconv.ParseFloat(trade.Price, 64)
		qty, _ := strconv.ParseFloat(trade.Size, 64)
		commission, _ := strconv.ParseFloat(trade.Fee, 64)
		cTime, _ := strconv.ParseInt(trade.CTime, 10, 64)

		result = append(result, map[string]interface{}{
			"id":              trade.TradeId,
			"orderId":         trade.OrderId,
			"symbol":          trade.Symbol,
			"price":           price,
			"qty":             qty,
			"commission":      commission,
			"commissionAsset": "USDT",
			"time":            cTime,
			"isBuyer":         trade.Side == "open_long" || trade.Side == "close_short",
			"isMaker":         false, // Bitget doesn't provide this
			"positionSide":    trade.PosSide,
		})
	}

	return result, nil
}

// GetOpenOrders Get all open (unfilled) orders from Bitget
func (t *BitgetTrader) GetOpenOrders(symbol string) ([]map[string]interface{}, error) {
	path := "/api/v2/mix/order/orders-pending"
	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
	}
	if symbol != "" {
		params["symbol"] = symbol
	}

	data, err := t.doRequest("GET", path, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get open orders: %w", err)
	}

	var response struct {
		Data []struct {
			OrderId   string `json:"orderId"`
			Symbol    string `json:"symbol"`
			OrderType string `json:"orderType"`
			Side      string `json:"side"`
			Price     string `json:"price"`
			Size      string `json:"size"`
			FilledQty string `json:"filledQty"`
			Status    string `json:"status"`
			CTime     string `json:"cTime"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse open orders: %w", err)
	}

	result := make([]map[string]interface{}, 0, len(response.Data))
	for _, order := range response.Data {
		orderCategory := "limit"
		if order.OrderType == "stop" {
			orderCategory = "stop_loss"
		} else if order.OrderType == "profit" {
			orderCategory = "take_profit"
		}

		side := "long"
		if order.Side == "open_short" || order.Side == "close_long" {
			side = "short"
		}

		triggerPrice, _ := strconv.ParseFloat(order.Price, 64)
		qty, _ := strconv.ParseFloat(order.Size, 64)
		filledQty, _ := strconv.ParseFloat(order.FilledQty, 64)
		cTime, _ := strconv.ParseInt(order.CTime, 10, 64)

		result = append(result, map[string]interface{}{
			"id":              order.OrderId,
			"symbol":          order.Symbol,
			"side":            side,
			"order_type":      orderCategory,
			"trigger_price":   triggerPrice,
			"quantity":        qty,
			"filled_quantity": filledQty,
			"status":          order.Status,
			"exchange_order_id": order.OrderId,
			"created_at":      time.Unix(cTime/1000, 0).Format("2006-01-02 15:04:05"),
		})
	}

	return result, nil
}

// Helper methods

func (t *BitgetTrader) clearCache() {
	t.balanceCacheMutex.Lock()
	t.cachedBalance = nil
	t.balanceCacheMutex.Unlock()

	t.positionsCacheMutex.Lock()
	t.cachedPositions = nil
	t.positionsCacheMutex.Unlock()
}

func (t *BitgetTrader) cancelTPSLOrders(symbol string, orderType string) error {
	// Get all TPSL orders first
	data, err := t.doRequest("GET", "/api/v2/mix/order/current-plan?symbol="+symbol+"&productType=usdt-futures", nil)
	if err != nil {
		return fmt.Errorf("failed to get TPSL orders: %w", err)
	}

	var orders []struct {
		OrderId  string `json:"orderId"`
		PlanType string `json:"planType"` // profit_plan or loss_plan
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil
	}

	// Cancel matching orders
	for _, order := range orders {
		shouldCancel := false
		if orderType == "stop_loss" && order.PlanType == "loss_plan" {
			shouldCancel = true
		}
		if orderType == "take_profit" && order.PlanType == "profit_plan" {
			shouldCancel = true
		}

		if shouldCancel && order.OrderId != "" {
			body := map[string]interface{}{
				"orderId":     order.OrderId,
				"symbol":      symbol,
				"productType": "usdt-futures",
			}
			t.doRequest("POST", "/api/v2/mix/order/cancel-plan", body)
		}
	}

	return nil
}
