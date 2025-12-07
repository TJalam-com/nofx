package trader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"nofx/logger"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	bybit "github.com/bybit-exchange/bybit.go.api"
)

// BybitTrader Bybit USDT 永續合約交易器
type BybitTrader struct {
	client *bybit.Client

	// 余额缓存
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// 持仓缓存
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// 交易对精度缓存 (symbol -> qtyStep)
	qtyStepCache      map[string]float64
	qtyStepCacheMutex sync.RWMutex

	// 缓存有效期（15秒）
	cacheDuration time.Duration
}

// NewBybitTrader 创建 Bybit 交易器
func NewBybitTrader(apiKey, secretKey string) *BybitTrader {
	const src = "Up000938"

	client := bybit.NewBybitHttpClient(apiKey, secretKey, bybit.WithBaseURL(bybit.MAINNET))

	// 设置 HTTP 传输
	if client != nil && client.HTTPClient != nil {
		defaultTransport := client.HTTPClient.Transport
		if defaultTransport == nil {
			defaultTransport = http.DefaultTransport
		}

		client.HTTPClient.Transport = &headerRoundTripper{
			base:      defaultTransport,
			refererID: src,
		}
	}

	trader := &BybitTrader{
		client:        client,
		cacheDuration: 15 * time.Second,
		qtyStepCache:  make(map[string]float64),
	}

	logger.Infof("🔵 [Bybit] 交易器已初始化")

	return trader
}

// headerRoundTripper 用于添加自定义 header 的 HTTP RoundTripper
type headerRoundTripper struct {
	base      http.RoundTripper
	refererID string
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Referer", h.refererID)
	return h.base.RoundTrip(req)
}

// GetBalance 获取账户余额
func (t *BybitTrader) GetBalance() (map[string]interface{}, error) {
	// 检查缓存
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		balance := t.cachedBalance
		t.balanceCacheMutex.RUnlock()
		return balance, nil
	}
	t.balanceCacheMutex.RUnlock()

	// 调用 API
	params := map[string]interface{}{
		"accountType": "UNIFIED",
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).GetAccountWallet(context.Background())
	if err != nil {
		return nil, fmt.Errorf("获取 Bybit 余额失败: %w", err)
	}

	if result.RetCode != 0 {
		return nil, fmt.Errorf("Bybit API 错误: %s", result.RetMsg)
	}

	// 提取余额信息
	resultData, ok := result.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Bybit 余额返回格式错误")
	}

	list, _ := resultData["list"].([]interface{})

	var totalEquity, availableBalance, totalWalletBalance, totalPerpUPL float64 = 0, 0, 0, 0

	if len(list) > 0 {
		account, _ := list[0].(map[string]interface{})
		if equityStr, ok := account["totalEquity"].(string); ok {
			totalEquity, _ = strconv.ParseFloat(equityStr, 64)
		}
		if availStr, ok := account["totalAvailableBalance"].(string); ok {
			availableBalance, _ = strconv.ParseFloat(availStr, 64)
		}
		// Bybit UNIFIED 账户的钱包余额字段
		if walletStr, ok := account["totalWalletBalance"].(string); ok {
			totalWalletBalance, _ = strconv.ParseFloat(walletStr, 64)
		}
		// Bybit 永续合约未实现盈亏
		if uplStr, ok := account["totalPerpUPL"].(string); ok {
			totalPerpUPL, _ = strconv.ParseFloat(uplStr, 64)
		}
	}
	// 如果没有 totalWalletBalance，使用 totalEquity
	if totalWalletBalance == 0 {
		totalWalletBalance = totalEquity
	}

	balance := map[string]interface{}{
		"totalEquity":           totalEquity,
		"totalWalletBalance":    totalWalletBalance,
		"availableBalance":     availableBalance,
		"totalUnrealizedProfit": totalPerpUPL,
		"balance":               totalEquity, // 兼容其他交易所格式
	}

	// 更新缓存
	t.balanceCacheMutex.Lock()
	t.cachedBalance = balance
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return balance, nil
}

// GetPositions 获取所有持仓
func (t *BybitTrader) GetPositions() ([]map[string]interface{}, error) {
	// 检查缓存
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		positions := t.cachedPositions
		t.positionsCacheMutex.RUnlock()
		return positions, nil
	}
	t.positionsCacheMutex.RUnlock()

	// 调用 API
	params := map[string]interface{}{
		"category":   "linear",
		"settleCoin": "USDT",
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).GetPositionList(context.Background())
	if err != nil {
		return nil, fmt.Errorf("获取 Bybit 持仓失败: %w", err)
	}

	if result.RetCode != 0 {
		return nil, fmt.Errorf("Bybit API 错误: %s", result.RetMsg)
	}

	resultData, ok := result.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Bybit 持仓返回格式错误")
	}

	list, _ := resultData["list"].([]interface{})

	var positions []map[string]interface{}

	for _, item := range list {
		pos, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		sizeStr, _ := pos["size"].(string)
		size, _ := strconv.ParseFloat(sizeStr, 64)

		// 跳过空仓位
		if size == 0 {
			continue
		}

		entryPriceStr, _ := pos["avgPrice"].(string)
		entryPrice, _ := strconv.ParseFloat(entryPriceStr, 64)

		unrealisedPnlStr, _ := pos["unrealisedPnl"].(string)
		unrealisedPnl, _ := strconv.ParseFloat(unrealisedPnlStr, 64)

		leverageStr, _ := pos["leverage"].(string)
		leverage, _ := strconv.ParseFloat(leverageStr, 64)

		// 标记价格
		markPriceStr, _ := pos["markPrice"].(string)
		markPrice, _ := strconv.ParseFloat(markPriceStr, 64)
		// 强平价格
		liqPriceStr, _ := pos["liqPrice"].(string)
		liqPrice, _ := strconv.ParseFloat(liqPriceStr, 64)

		positionSide, _ := pos["side"].(string) // Buy = LONG, Sell = SHORT

		// 转换为统一格式
		side := "LONG"
		positionAmt := size
		if positionSide == "Sell" {
			side = "SHORT"
			positionAmt = -size
		}

		position := map[string]interface{}{
			"symbol":           pos["symbol"],
			"side":             side,
			"positionAmt":      positionAmt,
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"unRealizedProfit": unrealisedPnl,
			"unrealizedPnL":    unrealisedPnl,
			"liquidationPrice": liqPrice,
			"leverage":         leverage,
		}

		positions = append(positions, position)
	}

	// 更新缓存
	t.positionsCacheMutex.Lock()
	t.cachedPositions = positions
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return positions, nil
}

// OpenLong 开多仓
func (t *BybitTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// 先设置杠杆
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("⚠️ [Bybit] 设置杠杆失败: %v", err)
	}

	// 使用 FormatQuantity 格式化数量
	qtyStr, _ := t.FormatQuantity(symbol, quantity)
	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        "Buy",
		"orderType":   "Market",
		"qty":         qtyStr,
		"positionIdx": 0, // 单向持仓模式
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).PlaceOrder(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Bybit 开多失败: %w", err)
	}

	// 清除缓存
	t.clearCache()

	return t.parseOrderResult(result)
}

// OpenShort 开空仓
func (t *BybitTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// 先设置杠杆
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("⚠️ [Bybit] 设置杠杆失败: %v", err)
	}

	// 使用 FormatQuantity 格式化数量
	qtyStr, _ := t.FormatQuantity(symbol, quantity)
	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        "Sell",
		"orderType":   "Market",
		"qty":         qtyStr,
		"positionIdx": 0, // 单向持仓模式
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).PlaceOrder(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Bybit 开空失败: %w", err)
	}

	// 清除缓存
	t.clearCache()

	return t.parseOrderResult(result)
}

// CloseLong 平多仓
func (t *BybitTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// 如果 quantity = 0，获取当前持仓数量
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
		return nil, fmt.Errorf("没有多仓可平")
	}

	// 使用 FormatQuantity 格式化数量
	qtyStr, _ := t.FormatQuantity(symbol, quantity)
	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        "Sell", // 平多用 Sell
		"orderType":   "Market",
		"qty":         qtyStr,
		"positionIdx": 0,
		"reduceOnly":  true,
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).PlaceOrder(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Bybit 平多失败: %w", err)
	}

	// 清除缓存
	t.clearCache()

	return t.parseOrderResult(result)
}

// CloseShort 平空仓
func (t *BybitTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// 如果 quantity = 0，获取当前持仓数量
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}
		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "SHORT" {
				quantity = -pos["positionAmt"].(float64) // 空仓是负数
				break
			}
		}
	}

	if quantity <= 0 {
		return nil, fmt.Errorf("没有空仓可平")
	}

	// 使用 FormatQuantity 格式化数量
	qtyStr, _ := t.FormatQuantity(symbol, quantity)
	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        "Buy", // 平空用 Buy
		"orderType":   "Market",
		"qty":         qtyStr,
		"positionIdx": 0,
		"reduceOnly":  true,
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).PlaceOrder(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Bybit 平空失败: %w", err)
	}

	// 清除缓存
	t.clearCache()

	return t.parseOrderResult(result)
}

// SetLeverage 设置杠杆
func (t *BybitTrader) SetLeverage(symbol string, leverage int) error {
	params := map[string]interface{}{
		"category":     "linear",
		"symbol":       symbol,
		"buyLeverage":  fmt.Sprintf("%d", leverage),
		"sellLeverage": fmt.Sprintf("%d", leverage),
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).SetPositionLeverage(context.Background())
	if err != nil {
		// 如果杠杆已经是目标值，Bybit 会返回错误，忽略这种情况
		if strings.Contains(err.Error(), "leverage not modified") {
			return nil
		}
		return fmt.Errorf("设置杠杆失败: %w", err)
	}

	if result.RetCode != 0 && result.RetCode != 110043 { // 110043 = leverage not modified
		return fmt.Errorf("设置杠杆失败: %s", result.RetMsg)
	}

	return nil
}

// SetMarginMode 设置仓位模式
func (t *BybitTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	tradeMode := 1 // 逐仓
	if isCrossMargin {
		tradeMode = 0 // 全仓
	}

	params := map[string]interface{}{
		"category":  "linear",
		"symbol":    symbol,
		"tradeMode": tradeMode,
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).SwitchPositionMargin(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), "Cross/isolated margin mode is not modified") {
			return nil
		}
		return fmt.Errorf("设置保证金模式失败: %w", err)
	}

	if result.RetCode != 0 && result.RetCode != 110026 { // already in target mode
		return fmt.Errorf("设置保证金模式失败: %s", result.RetMsg)
	}

	return nil
}

// GetMarketPrice 获取市场价格
func (t *BybitTrader) GetMarketPrice(symbol string) (float64, error) {
	params := map[string]interface{}{
		"category": "linear",
		"symbol":   symbol,
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).GetMarketTickers(context.Background())
	if err != nil {
		return 0, fmt.Errorf("获取市场价格失败: %w", err)
	}

	if result.RetCode != 0 {
		return 0, fmt.Errorf("API 错误: %s", result.RetMsg)
	}

	resultData, ok := result.Result.(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("返回格式错误")
	}

	list, _ := resultData["list"].([]interface{})

	if len(list) == 0 {
		return 0, fmt.Errorf("未找到 %s 的价格数据", symbol)
	}

	ticker, _ := list[0].(map[string]interface{})
	lastPriceStr, _ := ticker["lastPrice"].(string)
	lastPrice, err := strconv.ParseFloat(lastPriceStr, 64)
	if err != nil {
		return 0, fmt.Errorf("解析价格失败: %w", err)
	}

	return lastPrice, nil
}

// SetStopLoss 设置止损单
func (t *BybitTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	side := "Sell" // LONG 止损用 Sell
	if positionSide == "SHORT" {
		side = "Buy" // SHORT 止损用 Buy
	}

	// 获取当前价格来确定 triggerDirection
	currentPrice, err := t.GetMarketPrice(symbol)
	if err != nil {
		return err
	}

	triggerDirection := 2 // 价格下跌触发（默认多单止损）
	if stopPrice > currentPrice {
		triggerDirection = 1 // 价格上涨触发（空单止损）
	}

	// 使用 FormatQuantity 格式化数量
	qtyStr, _ := t.FormatQuantity(symbol, quantity)
	params := map[string]interface{}{
		"category":         "linear",
		"symbol":           symbol,
		"side":             side,
		"orderType":        "Market",
		"qty":              qtyStr,
		"triggerPrice":     fmt.Sprintf("%v", stopPrice),
		"triggerDirection": triggerDirection,
		"triggerBy":        "LastPrice",
		"reduceOnly":       true,
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).PlaceOrder(context.Background())
	if err != nil {
		return fmt.Errorf("设置止损失败: %w", err)
	}

	if result.RetCode != 0 {
		return fmt.Errorf("设置止损失败: %s", result.RetMsg)
	}

	logger.Infof("  ✓ [Bybit] 止损单已设置: %s @ %.2f", symbol, stopPrice)
	return nil
}

// SetTakeProfit 设置止盈单
func (t *BybitTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	side := "Sell" // LONG 止盈用 Sell
	if positionSide == "SHORT" {
		side = "Buy" // SHORT 止盈用 Buy
	}

	// 获取当前价格来确定 triggerDirection
	currentPrice, err := t.GetMarketPrice(symbol)
	if err != nil {
		return err
	}

	triggerDirection := 1 // 价格上涨触发（默认多单止盈）
	if takeProfitPrice < currentPrice {
		triggerDirection = 2 // 价格下跌触发（空单止盈）
	}

	// 使用 FormatQuantity 格式化数量
	qtyStr, _ := t.FormatQuantity(symbol, quantity)
	params := map[string]interface{}{
		"category":         "linear",
		"symbol":           symbol,
		"side":             side,
		"orderType":        "Market",
		"qty":              qtyStr,
		"triggerPrice":     fmt.Sprintf("%v", takeProfitPrice),
		"triggerDirection": triggerDirection,
		"triggerBy":        "LastPrice",
		"reduceOnly":       true,
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).PlaceOrder(context.Background())
	if err != nil {
		return fmt.Errorf("设置止盈失败: %w", err)
	}

	if result.RetCode != 0 {
		return fmt.Errorf("设置止盈失败: %s", result.RetMsg)
	}

	logger.Infof("  ✓ [Bybit] 止盈单已设置: %s @ %.2f", symbol, takeProfitPrice)
	return nil
}

// CancelStopLossOrders 取消止损单
func (t *BybitTrader) CancelStopLossOrders(symbol string) error {
	return t.cancelConditionalOrders(symbol, "StopLoss")
}

// CancelTakeProfitOrders 取消止盈单
func (t *BybitTrader) CancelTakeProfitOrders(symbol string) error {
	return t.cancelConditionalOrders(symbol, "TakeProfit")
}

// CancelAllOrders 取消所有挂单
func (t *BybitTrader) CancelAllOrders(symbol string) error {
	params := map[string]interface{}{
		"category": "linear",
		"symbol":   symbol,
	}

	_, err := t.client.NewUtaBybitServiceWithParams(params).CancelAllOrders(context.Background())
	if err != nil {
		return fmt.Errorf("取消所有订单失败: %w", err)
	}

	return nil
}

// CancelStopOrders 取消所有止盈止损单
func (t *BybitTrader) CancelStopOrders(symbol string) error {
	if err := t.CancelStopLossOrders(symbol); err != nil {
		logger.Infof("⚠️ [Bybit] 取消止损单失败: %v", err)
	}
	if err := t.CancelTakeProfitOrders(symbol); err != nil {
		logger.Infof("⚠️ [Bybit] 取消止盈单失败: %v", err)
	}
	return nil
}

// getQtyStep 获取交易对的数量步长
func (t *BybitTrader) getQtyStep(symbol string) float64 {
	// 先检查缓存
	t.qtyStepCacheMutex.RLock()
	if step, ok := t.qtyStepCache[symbol]; ok {
		t.qtyStepCacheMutex.RUnlock()
		return step
	}
	t.qtyStepCacheMutex.RUnlock()

	// 直接调用公开 API 获取合约信息
	url := fmt.Sprintf("https://api.bybit.com/v5/market/instruments-info?category=linear&symbol=%s", symbol)
	resp, err := http.Get(url)
	if err != nil {
		logger.Infof("⚠️ [Bybit] 获取 %s 精度信息失败: %v", symbol, err)
		return 1 // 默认整数
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 1
	}

	var result struct {
		RetCode int `json:"retCode"`
		Result  struct {
			List []struct {
				LotSizeFilter struct {
					QtyStep string `json:"qtyStep"`
				} `json:"lotSizeFilter"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 1
	}

	if result.RetCode != 0 || len(result.Result.List) == 0 {
		return 1
	}

	qtyStep, _ := strconv.ParseFloat(result.Result.List[0].LotSizeFilter.QtyStep, 64)
	if qtyStep <= 0 {
		qtyStep = 1
	}

	// 缓存结果
	t.qtyStepCacheMutex.Lock()
	t.qtyStepCache[symbol] = qtyStep
	t.qtyStepCacheMutex.Unlock()

	logger.Infof("🔵 [Bybit] %s qtyStep: %v", symbol, qtyStep)
	return qtyStep
}

// FormatQuantity 格式化数量
func (t *BybitTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	// 获取该币种的 qtyStep
	qtyStep := t.getQtyStep(symbol)

	// 根据 qtyStep 对齐数量（向下取整到最近的 step）
	alignedQty := math.Floor(quantity/qtyStep) * qtyStep

	// 计算需要的小数位数
	decimals := 0
	if qtyStep < 1 {
		stepStr := strconv.FormatFloat(qtyStep, 'f', -1, 64)
		if idx := strings.Index(stepStr, "."); idx >= 0 {
			decimals = len(stepStr) - idx - 1
		}
	}

	// 格式化
	format := fmt.Sprintf("%%.%df", decimals)
	formatted := fmt.Sprintf(format, alignedQty)
	return formatted, nil
}

// 辅助方法

func (t *BybitTrader) clearCache() {
	t.balanceCacheMutex.Lock()
	t.cachedBalance = nil
	t.balanceCacheMutex.Unlock()

	t.positionsCacheMutex.Lock()
	t.cachedPositions = nil
	t.positionsCacheMutex.Unlock()
}

func (t *BybitTrader) parseOrderResult(result *bybit.ServerResponse) (map[string]interface{}, error) {
	if result.RetCode != 0 {
		return nil, fmt.Errorf("下单失败: %s", result.RetMsg)
	}

	resultData, ok := result.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("返回格式错误")
	}

	orderId, _ := resultData["orderId"].(string)

	return map[string]interface{}{
		"orderId": orderId,
		"status":  "NEW",
	}, nil
}

func (t *BybitTrader) cancelConditionalOrders(symbol string, orderType string) error {
	// 先获取所有条件单
	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"orderFilter": "StopOrder", // 条件单
	}

	result, err := t.client.NewUtaBybitServiceWithParams(params).GetOpenOrders(context.Background())
	if err != nil {
		return fmt.Errorf("获取条件单失败: %w", err)
	}

	if result.RetCode != 0 {
		return nil // 没有订单
	}

	resultData, ok := result.Result.(map[string]interface{})
	if !ok {
		return nil
	}

	list, _ := resultData["list"].([]interface{})

	// 取消匹配的订单
	for _, item := range list {
		order, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		orderId, _ := order["orderId"].(string)
		stopOrderType, _ := order["stopOrderType"].(string)

		// 根据类型筛选
		shouldCancel := false
		if orderType == "StopLoss" && (stopOrderType == "StopLoss" || stopOrderType == "Stop") {
			shouldCancel = true
		}
		if orderType == "TakeProfit" && (stopOrderType == "TakeProfit" || stopOrderType == "PartialTakeProfit") {
			shouldCancel = true
		}

		if shouldCancel && orderId != "" {
			cancelParams := map[string]interface{}{
				"category": "linear",
				"symbol":   symbol,
				"orderId":  orderId,
			}
			t.client.NewUtaBybitServiceWithParams(cancelParams).CancelOrder(context.Background())
		}
	}

	return nil
}
