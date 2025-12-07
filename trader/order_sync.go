package trader

import (
	"fmt"
	"nofx/config"
)

// TraderFullConfig 完整的交易员配置（包含交易所和AI模型配置）
type TraderFullConfig struct {
	Trader   *config.TraderRecord
	Exchange *config.ExchangeConfig
	AIModel  *config.AIModelConfig
}

// OrderSyncManager 订单同步管理器
type OrderSyncManager struct {
	// 可以添加其他字段
}

// NewOrderSyncManager 创建订单同步管理器
func NewOrderSyncManager() *OrderSyncManager {
	return &OrderSyncManager{}
}

// getTraderConfig 获取交易员配置
func (m *OrderSyncManager) getTraderConfig(traderID string) (*TraderFullConfig, error) {
	// 实现获取配置的逻辑
	return nil, fmt.Errorf("not implemented")
}

// createTrader 创建交易器实例
func (m *OrderSyncManager) createTrader(config *TraderFullConfig) (Trader, error) {
	exchange := config.Exchange

	// 使用 exchange.ID 判断具体的交易所，而不是 exchange.Type (cex/dex)
	switch exchange.ID {
	case "binance":
		return NewFuturesTrader(exchange.APIKey, exchange.SecretKey, config.Trader.UserID), nil
	case "bybit":
		return NewBybitTrader(exchange.APIKey, exchange.SecretKey), nil
	case "okx":
		return NewOKXTrader(exchange.APIKey, exchange.SecretKey, exchange.OkxPassphrase), nil
	case "hyperliquid":
		return NewHyperliquidTrader(exchange.APIKey, exchange.HyperliquidWalletAddr, exchange.Testnet)
	case "aster":
		return NewAsterTrader(exchange.AsterUser, exchange.AsterSigner, exchange.AsterPrivateKey)
	case "lighter":
		return NewLighterTrader(exchange.LighterPrivateKey, exchange.LighterWalletAddr, exchange.Testnet)
	default:
		return nil, fmt.Errorf("不支持的交易所: %s", exchange.ID)
	}
}

