package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// extractTraderIDs extracts and normalizes trader IDs from payload
// Supports both trader_id (string) and trader_ids (array) for backward compatibility
func (s *Server) extractTraderIDs(payload map[string]interface{}, userID string) ([]string, error) {
	var traderIDs []string

	// Debug: Log what we received
	if traderIDsRaw, exists := payload["trader_ids"]; exists {
		log.Printf("🔍 extractTraderIDs: Found trader_ids field, type=%T, value=%v", traderIDsRaw, traderIDsRaw)
	}
	if traderIDRaw, exists := payload["trader_id"]; exists {
		log.Printf("🔍 extractTraderIDs: Found trader_id field, type=%T, value=%v", traderIDRaw, traderIDRaw)
	}

	// Check for trader_ids array first (new format)
	// Handle both []interface{} and []string types from JSON unmarshaling
	if traderIDsRaw, exists := payload["trader_ids"]; exists {
		log.Printf("🔍 extractTraderIDs: Found trader_ids field, type=%T", traderIDsRaw)
		
		// Try []interface{} first (common case)
		if traderIDsArray, ok := traderIDsRaw.([]interface{}); ok && len(traderIDsArray) > 0 {
			log.Printf("🔍 extractTraderIDs: Successfully parsed trader_ids as []interface{}, count=%d", len(traderIDsArray))
			for _, tid := range traderIDsArray {
				if tidStr, ok := tid.(string); ok && tidStr != "" {
					// Verify trader belongs to this user
					_, _, _, err := s.database.GetTraderConfig(userID, tidStr)
					if err != nil {
						log.Printf("⚠️ Trader %s does not belong to user %s", tidStr, userID)
						continue // Skip invalid trader IDs but continue processing others
					}
					traderIDs = append(traderIDs, tidStr)
				}
			}
			if len(traderIDs) > 0 {
				log.Printf("🔍 extractTraderIDs: Returning %d trader IDs from trader_ids array: %v", len(traderIDs), traderIDs)
				return traderIDs, nil
			}
		} else if traderIDsStringArray, ok := traderIDsRaw.([]string); ok && len(traderIDsStringArray) > 0 {
			// Handle []string type (alternative JSON unmarshaling)
			log.Printf("🔍 extractTraderIDs: Successfully parsed trader_ids as []string, count=%d", len(traderIDsStringArray))
			for _, tidStr := range traderIDsStringArray {
				if tidStr != "" {
					// Verify trader belongs to this user
					_, _, _, err := s.database.GetTraderConfig(userID, tidStr)
					if err != nil {
						log.Printf("⚠️ Trader %s does not belong to user %s", tidStr, userID)
						continue // Skip invalid trader IDs but continue processing others
					}
					traderIDs = append(traderIDs, tidStr)
				}
			}
			if len(traderIDs) > 0 {
				log.Printf("🔍 extractTraderIDs: Returning %d trader IDs from trader_ids []string array: %v", len(traderIDs), traderIDs)
				return traderIDs, nil
			}
		} else {
			log.Printf("⚠️ extractTraderIDs: trader_ids found but not a valid array type (got %T)", traderIDsRaw)
		}
	} else {
		log.Printf("🔍 extractTraderIDs: trader_ids field not found in payload")
	}

	// Fallback to single trader_id (backward compatibility)
	if tid, ok := payload["trader_id"].(string); ok && tid != "" {
		log.Printf("🔍 extractTraderIDs: Using single trader_id: %s", tid)
		// Verify trader belongs to this user
		_, _, _, err := s.database.GetTraderConfig(userID, tid)
		if err != nil {
			log.Printf("⚠️ Trader %s does not belong to user %s", tid, userID)
			return nil, fmt.Errorf("Invalid trader_id")
		}
		return []string{tid}, nil
	}

	// If no trader IDs provided, auto-assign first trader with TradingView enabled
	log.Printf("🔍 extractTraderIDs: No trader_id or trader_ids found, attempting auto-assign")
	traders, err := s.database.GetTradersWithTradingViewEnabled(userID)
	if err != nil || len(traders) == 0 {
		log.Printf("⚠️ User %s has no traders with TradingView enabled", userID)
		return nil, fmt.Errorf("No trader with TradingView enabled found, please specify trader_id or trader_ids in webhook or enable TradingView signal in trader config")
	}
	traderIDs = []string{traders[0].ID}
	log.Printf("📌 Auto-assigned trader: %s", traders[0].ID)
	return traderIDs, nil
}

// processTraderWebhook processes webhook for a single trader
func (s *Server) processTraderWebhook(userID, traderID string, payload map[string]interface{}, symbol, action string) gin.H {
	log.Printf("🔄 Processing webhook for trader: %s (symbol=%s, action=%s)", traderID, symbol, action)
	
	// Create alert record with retry logic for UNIQUE constraint errors
	var alertID string
	var err error
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		alertID, err = s.database.CreateTradingViewAlert(userID, traderID, payload)
		if err == nil {
			break // Success
		}
		
		// Check if it's a UNIQUE constraint error (collision)
		errStr := err.Error()
		if strings.Contains(errStr, "UNIQUE constraint") || strings.Contains(errStr, "constraint failed") {
			if attempt < maxRetries {
				log.Printf("⚠️ Alert ID collision detected for trader %s (attempt %d/%d), retrying...", traderID, attempt, maxRetries)
				time.Sleep(time.Duration(attempt) * time.Millisecond) // Small delay before retry
				continue
			}
			log.Printf("❌ Failed to create TradingView alert for trader %s after %d attempts: %v", traderID, maxRetries, err)
		} else {
			// Other error, don't retry
			log.Printf("❌ Failed to create TradingView alert for trader %s: %v", traderID, err)
			break
		}
	}
	
	if err != nil {
		return gin.H{
			"trader_id": traderID,
			"success":   false,
			"error":     fmt.Sprintf("Failed to save alert: %v", err),
		}
	}

	log.Printf("✅ TradingView alert received: user=%s, trader=%s, symbol=%s, action=%s, alertID=%s", userID, traderID, symbol, action, alertID)
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
	if err != nil {
		log.Printf("⚠️ Failed to get trader instance %s: %v", traderID, err)
		log.Printf("ERROR: Trader instance not found (trader=%s, alertID=%s, err=%v)", traderID, alertID, err)
		response["error"] = fmt.Sprintf("Failed to get trader instance: %v", err)
		response["success"] = false
		return response
	}

	// Confirm from database if TradingView is enabled
	traderConfig, _, _, cfgErr := s.database.GetTraderConfig(userID, traderID)
	if cfgErr != nil {
		log.Printf("⚠️ Failed to get trader config %s: %v", traderID, cfgErr)
		log.Printf("WARN: Skip trigger because trader config load failed (trader=%s, alertID=%s, err=%v)", traderID, alertID, cfgErr)
		response["warning"] = "Failed to get trader config, decision not triggered"
		return response
	}

	if !traderConfig.UseTradingView {
		log.Printf("⚠️ [%s] TradingView signal not enabled, skipping trigger", traderID)
		log.Printf("WARN: Skip trigger because UseTradingView=false at DB (trader=%s, alertID=%s)", traderID, alertID)
		response["warning"] = "Trader has not enabled TradingView signal, decision not triggered"
		return response
	}

	// Check if trader is running
	status := at.GetStatus()
	isRunning, _ := status["is_running"].(bool)
	memUseTV, _ := status["use_tradingview"].(bool)
	dbIsRunning := traderConfig.IsRunning // Running status in database
	response["trader_running"] = isRunning
	response["triggered"] = false
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
			return response
		}

		// If DB status shows running but not in memory, trader may have crashed, can auto-start
		if traderConfig != nil && traderConfig.UseTradingView {
			// Reload config first to ensure latest settings
			if reloadErr := s.traderManager.ReloadTraderFromDB(s.database, userID, traderID); reloadErr != nil {
				log.Printf("⚠️ Failed to reload trader config before auto-start: %v", reloadErr)
				response["warning"] = "Failed to reload trader config"
				return response
			}

			// Re-check database status (prevent state changes during reload)
			freshConfig, _, _, freshCfgErr := s.database.GetTraderConfig(userID, traderID)
			if freshCfgErr != nil {
				log.Printf("⚠️ Failed to get fresh trader config after reload: %v", freshCfgErr)
				response["warning"] = "Failed to get latest trader config"
				return response
			}

			// If reload shows DB status changed to stopped, do not auto-start
			if !freshConfig.IsRunning {
				log.Printf("⏹️ [%s] Trader is stopped (DB status) after reload, not auto-starting to handle webhook", traderID)
				log.Printf("INFO: Trader is stopped in DB after reload, respecting stop status (trader=%s, alertID=%s)", traderID, alertID)
				response["warning"] = "Trader is stopped, please manually start trader to handle webhook"
				return response
			}

			// Get fresh instance after reload
			at, err = s.traderManager.GetTrader(traderID)
			if err != nil {
				log.Printf("❌ Failed to get trader after reload: %v", err)
				response["error"] = "Failed to get trader instance"
				return response
			}
			// Auto-start trader (TradingView mode, only when trader crashed)
			log.Printf("🔄 [%s] TradingView mode: detected trader crash, auto-restarting to handle webhook", traderConfig.Name)
			log.Printf("INFO: Auto-start trader for TradingView webhook (trader crashed, trader=%s, alertID=%s)", traderID, alertID)
			go func() {
				if err := at.Run(); err != nil {
					log.Printf("❌ [%s] Auto-start failed: %v", traderConfig.Name, err)
					// Update database status to stopped
					_ = s.database.UpdateTraderStatus(userID, traderID, false)
				}
			}()
			// Update database status
			_ = s.database.UpdateTraderStatus(userID, traderID, true)
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
				return response
			}

			response["warning"] = "Trader not running, auto-started (TradingView mode)"
			log.Printf("✓ [%s] Trader auto-started, ready to handle webhook", traderConfig.Name)
		} else {
			log.Printf("⚠️ [%s] Trader not running and TradingView not enabled, cannot auto-start", traderID)
			log.Printf("WARN: Trader not running and UseTradingView=false, cannot auto-start (trader=%s, alertID=%s)", traderID, alertID)
			response["warning"] = "Trader not running, please manually start trader to handle webhook"
			return response
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
		log.Printf("📝 Decision will be logged for trader %s after AI analysis completes", traderID)
		response["triggered"] = true
	}

	return response
}

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

	// Extract and normalize trader_ids (support both trader_id string and trader_ids array)
	traderIDs, err := s.extractTraderIDs(payload, user.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("📋 Webhook received: %d trader(s) to process: %v", len(traderIDs), traderIDs)

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

	// Process traders in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := []gin.H{}

	for _, traderID := range traderIDs {
		wg.Add(1)
		go func(tid string) {
			defer wg.Done()
			result := s.processTraderWebhook(user.ID, tid, payload, symbol, action)
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(traderID)
	}

	wg.Wait()

	log.Printf("✅ Webhook processing complete: %d trader(s) processed, %d result(s) returned", len(traderIDs), len(results))

	// Build response - maintain backward compatibility for single trader
	if len(traderIDs) == 1 && len(results) == 1 {
		// Single trader format (backward compatible)
		c.JSON(http.StatusOK, results[0])
		return
	}

	// Multiple traders format
	successful := 0
	failed := 0
	for _, result := range results {
		if success, ok := result["success"].(bool); ok && success {
			successful++
		} else {
			failed++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Alerts received for multiple traders",
		"total_traders": len(traderIDs),
		"successful":    successful,
		"failed":        failed,
		"results":       results,
	})
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
