package trader

import (
	"fmt"
)

// PositionSyncManager 持仓同步管理器
type PositionSyncManager struct {
	// 可以添加其他字段
}

// NewPositionSyncManager 创建持仓同步管理器
func NewPositionSyncManager() *PositionSyncManager {
	return &PositionSyncManager{}
}

// getTraderConfig 获取交易员配置
func (m *PositionSyncManager) getTraderConfig(traderID string) (*TraderFullConfig, error) {
	// 实现获取配置的逻辑
	return nil, fmt.Errorf("not implemented")
}

// createTrader 创建交易器实例
func (m *PositionSyncManager) createTrader(config *TraderFullConfig) (Trader, error) {
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

