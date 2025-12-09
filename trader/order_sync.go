package trader

import (
	"nofx/config"
)

// TraderFullConfig complete trader configuration (includes exchange and AI model configuration)
type TraderFullConfig struct {
	Trader   *config.TraderRecord
	Exchange *config.ExchangeConfig
	AIModel  *config.AIModelConfig
}

// OrderSyncManager order sync manager
type OrderSyncManager struct {
	// Can add other fields
}

// NewOrderSyncManager create order sync manager
func NewOrderSyncManager() *OrderSyncManager {
	return &OrderSyncManager{}
}

