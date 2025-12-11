package api

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// handleTradingViewWebhook handles TradingView webhook requests
func (s *Server) handleTradingViewWebhook(c *gin.Context) {
	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format", "details": err.Error()})
		return
	}

	// Extract apikey
	apikey, ok := payload["apikey"].(string)
	if !ok || apikey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid apikey field"})
		return
	}

	// Validate API key and get user
	user, err := s.database.GetUserByWebhookAPIKey(apikey)
	if err != nil {
		log.Printf("⚠️ Invalid webhook API key: %s", apikey)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
		return
	}

	// Extract trader_id (optional)
	var traderID string
	if tid, ok := payload["trader_id"].(string); ok && tid != "" {
		traderID = tid
		// Verify trader belongs to this user
		_, _, _, err := s.database.GetTraderConfig(user.ID, traderID)
		if err != nil {
			log.Printf("⚠️ Trader %s does not belong to user %s", traderID, user.ID)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trader_id"})
			return
		}
	} else {
		// If trader_id not provided, find user's first trader with TradingView enabled
		traders, err := s.database.GetTradersWithTradingViewEnabled(user.ID)
		if err != nil || len(traders) == 0 {
			log.Printf("⚠️ User %s has no traders with TradingView enabled", user.ID)
			c.JSON(http.StatusBadRequest, gin.H{"error": "No trader with TradingView enabled found, please specify trader_id in webhook or enable TradingView signal in trader config"})
			return
		}
		traderID = traders[0].ID
		log.Printf("📌 Auto-assigned trader: %s", traderID)
	}

	// Validate required fields
	symbol, ok := payload["symbol"].(string)
	if !ok || symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid symbol field"})
		return
	}

	action, ok := payload["action"].(string)
	if !ok || (action != "buy" && action != "sell") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid action field (must be buy or sell)"})
		return
	}

	// Create alert record
	alertID, err := s.database.CreateTradingViewAlert(user.ID, traderID, payload)
	if err != nil {
		log.Printf("❌ Failed to create TradingView alert: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save alert", "details": err.Error()})
		return
	}

	log.Printf("✅ TradingView alert received: user=%s, trader=%s, symbol=%s, action=%s, alertID=%s", user.ID, traderID, symbol, action, alertID)
	log.Printf("INFO: Webhook received (trader=%s, symbol=%s, action=%s, alertID=%s)", traderID, symbol, action, alertID)

	// Build response object
	response := gin.H{
		"success":   true,
		"message":   "Alert received",
		"trader_id": traderID,
		"symbol":    symbol,
		"action":    action,
		"alert_id":  alertID,
	}

	// If trader has TradingView enabled, trigger immediate decision cycle
	at, err := s.traderManager.GetTrader(traderID)
	if err == nil && at != nil {
		// Confirm from database if TradingView is enabled
		traderConfig, _, _, cfgErr := s.database.GetTraderConfig(user.ID, traderID)
		if cfgErr != nil {
			log.Printf("⚠️ Failed to get trader config %s: %v", traderID, cfgErr)
			log.Printf("WARN: Skip trigger because trader config load failed (trader=%s, alertID=%s, err=%v)", traderID, alertID, cfgErr)
			response["warning"] = "Failed to get trader config, decision not triggered"
			c.JSON(http.StatusOK, response)
			return
		}

		if !traderConfig.UseTradingView {
			log.Printf("⚠️ [%s] TradingView signal not enabled, skipping trigger", traderID)
			log.Printf("WARN: Skip trigger because UseTradingView=false at DB (trader=%s, alertID=%s)", traderID, alertID)
			response["warning"] = "Trader has not enabled TradingView signal, decision not triggered"
			c.JSON(http.StatusOK, response)
			return
		}

		// Check if trader is running
		status := at.GetStatus()
		isRunning, _ := status["is_running"].(bool)
		memUseTV, _ := status["use_tradingview"].(bool)
		dbIsRunning := traderConfig.IsRunning // Running status in database
		response["trader_running"] = isRunning
		log.Printf("ℹ️ [%s] TradingView trigger check: is_running(mem)=%v, is_running(DB)=%v, UseTradingView(DB)=%v, UseTradingView(mem)=%v", traderID, isRunning, dbIsRunning, traderConfig.UseTradingView, memUseTV)
		if memUseTV != traderConfig.UseTradingView {
			log.Printf("WARN: TradingView flag mismatch between DB and memory (trader=%s, db=%v, mem=%v)", traderID, traderConfig.UseTradingView, memUseTV)
		}
		log.Printf("INFO: Trigger check (trader=%s, is_running(mem)=%v, is_running(DB)=%v, UseTradingView=%v, alertID=%s)", traderID, isRunning, dbIsRunning, traderConfig.UseTradingView, alertID)

		if !isRunning {
			// If DB status shows trader is stopped (user explicitly stopped), should not auto-start
			if !dbIsRunning {
				log.Printf("⏹️ [%s] Trader is stopped (DB status), not auto-starting to handle webhook", traderID)
				log.Printf("INFO: Trader is stopped in DB, respecting stop status (trader=%s, alertID=%s)", traderID, alertID)
				response["warning"] = "Trader is stopped, please manually start trader to handle webhook"
				c.JSON(http.StatusOK, response)
				return
			}

			// If DB status shows running but not in memory, trader may have crashed, can auto-start
			if traderConfig != nil && traderConfig.UseTradingView {
				// Reload config first to ensure latest settings
				if reloadErr := s.traderManager.ReloadTraderFromDB(s.database, user.ID, traderID); reloadErr != nil {
					log.Printf("⚠️ Failed to reload trader config before auto-start: %v", reloadErr)
					response["warning"] = "Failed to reload trader config"
					c.JSON(http.StatusOK, response)
					return
				}

				// Re-check database status (prevent state changes during reload)
				freshConfig, _, _, freshCfgErr := s.database.GetTraderConfig(user.ID, traderID)
				if freshCfgErr != nil {
					log.Printf("⚠️ Failed to get fresh trader config after reload: %v", freshCfgErr)
					response["warning"] = "Failed to get latest trader config"
					c.JSON(http.StatusOK, response)
					return
				}

				// If reload shows DB status changed to stopped, do not auto-start
				if !freshConfig.IsRunning {
					log.Printf("⏹️ [%s] Trader is stopped (DB status) after reload, not auto-starting to handle webhook", traderID)
					log.Printf("INFO: Trader is stopped in DB after reload, respecting stop status (trader=%s, alertID=%s)", traderID, alertID)
					response["warning"] = "Trader is stopped, please manually start trader to handle webhook"
					c.JSON(http.StatusOK, response)
					return
				}

				// Get fresh instance after reload
				at, err = s.traderManager.GetTrader(traderID)
				if err != nil {
					log.Printf("❌ Failed to get trader after reload: %v", err)
					response["error"] = "Failed to get trader instance"
					c.JSON(http.StatusOK, response)
					return
				}
				// Auto-start trader (TradingView mode, only when trader crashed)
				log.Printf("🔄 [%s] TradingView mode: detected trader crash, auto-restarting to handle webhook", traderConfig.Name)
				log.Printf("INFO: Auto-start trader for TradingView webhook (trader crashed, trader=%s, alertID=%s)", traderID, alertID)
				go func() {
					if err := at.Run(); err != nil {
						log.Printf("❌ [%s] Auto-start failed: %v", traderConfig.Name, err)
						// Update database status to stopped
						_ = s.database.UpdateTraderStatus(user.ID, traderID, false)
					}
				}()
				// Update database status
		_ = s.database.UpdateTraderStatus(user.ID, traderID, true)
		// Wait for trader to start
				time.Sleep(500 * time.Millisecond)
				// Check running status again
				status = at.GetStatus()
				isRunning, _ = status["is_running"].(bool)
				response["trader_running"] = isRunning

				if !isRunning {
					log.Printf("⚠️ [%s] Trader still not running after auto-start, skipping decision cycle trigger", traderConfig.Name)
					log.Printf("WARN: Auto-start failed, trader still not running (trader=%s, alertID=%s)", traderID, alertID)
					response["warning"] = "Trader auto-start failed, please check logs and manually start"
					response["trigger_error"] = "trader failed to start"
					c.JSON(http.StatusOK, response)
					return
				}

				response["warning"] = "Trader not running, auto-started (TradingView mode)"
				log.Printf("✓ [%s] Trader auto-started, ready to handle webhook", traderConfig.Name)
			} else {
				log.Printf("⚠️ [%s] Trader not running and TradingView not enabled, cannot auto-start", traderID)
				log.Printf("WARN: Trader not running and UseTradingView=false, cannot auto-start (trader=%s, alertID=%s)", traderID, alertID)
				response["warning"] = "Trader not running, please manually start trader to handle webhook"
				c.JSON(http.StatusOK, response)
				return
			}
		}

		// Trigger decision cycle
		if err := at.TriggerTradingViewDecisionCycle(alertID); err != nil {
			log.Printf("⚠️ [%s] Failed to trigger decision cycle: %v", traderID, err)
			log.Printf("ERROR: Failed to trigger TradingView decision (trader=%s, alertID=%s, err=%v)", traderID, alertID, err)
			response["trigger_error"] = err.Error()
		} else {
			log.Printf("✓ [%s] Successfully triggered TradingView decision cycle: alertID=%s", traderID, alertID)
			log.Printf("INFO: Triggered TradingView decision successfully (trader=%s, alertID=%s)", traderID, alertID)
		}
	} else {
		log.Printf("⚠️ Failed to get trader instance %s: %v", traderID, err)
		log.Printf("ERROR: Trader instance not found (trader=%s, alertID=%s, err=%v)", traderID, alertID, err)
		response["error"] = fmt.Sprintf("Failed to get trader instance: %v", err)
	}

	c.JSON(http.StatusOK, response)
}

// handleGetWebhookInfo gets user's webhook information
func (s *Server) handleGetWebhookInfo(c *gin.Context) {
	userID := c.GetString("user_id")

	// Generate or get API key
	apiKey, err := s.database.GenerateWebhookAPIKey(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate API key", "details": err.Error()})
		return
	}

	// Build webhook URL (get host from request)
	host := c.Request.Host
	scheme := "https"
	if c.Request.TLS == nil {
		scheme = "http"
	}
	webhookURL := fmt.Sprintf("%s://%s/api/webhook/tradingview", scheme, host)

	c.JSON(http.StatusOK, gin.H{
		"webhook_url": webhookURL,
		"api_key":     apiKey,
	})
}

// handleGetRecentAlerts gets recent alerts
func (s *Server) handleGetRecentAlerts(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Query("trader_id")

	// Use database method to get recent alerts
	alerts, err := s.database.GetRecentTradingViewAlerts(userID, traderID, 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query failed", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"alerts": alerts})
}
