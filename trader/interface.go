package trader

import "time"

// Trader 交易器统一接口
// 支持多个交易平台（币安、Hyperliquid等）
type Trader interface {
	// GetBalance 获取账户余额
	GetBalance() (map[string]interface{}, error)

	// GetPositions 获取所有持仓
	GetPositions() ([]map[string]interface{}, error)

	// OpenLong 开多仓
	OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error)

	// OpenShort 开空仓
	OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error)

	// CloseLong 平多仓（quantity=0表示全部平仓）
	CloseLong(symbol string, quantity float64) (map[string]interface{}, error)

	// CloseShort 平空仓（quantity=0表示全部平仓）
	CloseShort(symbol string, quantity float64) (map[string]interface{}, error)

	// SetLeverage 设置杠杆
	SetLeverage(symbol string, leverage int) error

	// SetMarginMode 设置仓位模式 (true=全仓, false=逐仓)
	SetMarginMode(symbol string, isCrossMargin bool) error

	// GetMarketPrice 获取市场价格
	GetMarketPrice(symbol string) (float64, error)

	// SetStopLoss 设置止损单
	SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error

	// SetTakeProfit 设置止盈单
	SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error

	// CancelStopLossOrders 仅取消止损单（修复 BUG：调整止损时不删除止盈）
	CancelStopLossOrders(symbol string) error

	// CancelTakeProfitOrders 仅取消止盈单（修复 BUG：调整止盈时不删除止损）
	CancelTakeProfitOrders(symbol string) error

	// CancelAllOrders 取消该币种的所有挂单
	CancelAllOrders(symbol string) error

	// CancelStopOrders 取消该币种的止盈/止损单（用于调整止盈止损位置）
	CancelStopOrders(symbol string) error

	// FormatQuantity 格式化数量到正确的精度
	FormatQuantity(symbol string, quantity float64) (string, error)

	// GetOrderStatus Get order status from exchange
	// Returns map with: avgPrice (float64), executedQty (float64), commission (float64), status (string)
	GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error)

	// GetOrderHistory Get order history from exchange (all orders, including filled)
	// Returns orders sorted by time (newest first)
	// limit: maximum number of orders to return (0 = use exchange default)
	// startTime, endTime: optional time range filters (nil = no filter)
	GetOrderHistory(symbol string, limit int, startTime, endTime *time.Time) ([]map[string]interface{}, error)

	// GetUserTrades Get user trade history (executed trades)
	// Returns trades sorted by time (newest first)
	// limit: maximum number of trades to return (0 = use exchange default)
	// startTime, endTime: optional time range filters (nil = no filter)
	GetUserTrades(symbol string, limit int, startTime, endTime *time.Time) ([]map[string]interface{}, error)
}
