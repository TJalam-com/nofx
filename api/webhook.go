package api

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// handleTradingViewWebhook 处理TradingView webhook请求
func (s *Server) handleTradingViewWebhook(c *gin.Context) {
	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的JSON格式", "details": err.Error()})
		return
	}

	// 提取apikey
	apikey, ok := payload["apikey"].(string)
	if !ok || apikey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少或无效的apikey字段"})
		return
	}

	// 验证API key并获取用户
	user, err := s.database.GetUserByWebhookAPIKey(apikey)
	if err != nil {
		log.Printf("⚠️ 无效的webhook API key: %s", apikey)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的API key"})
		return
	}

	// 提取trader_id（可选）
	var traderID string
	if tid, ok := payload["trader_id"].(string); ok && tid != "" {
		traderID = tid
		// 验证trader是否属于该用户
		_, _, _, err := s.database.GetTraderConfig(user.ID, traderID)
		if err != nil {
			log.Printf("⚠️ 交易员 %s 不属于用户 %s", traderID, user.ID)
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的trader_id"})
			return
		}
	} else {
		// 如果没有提供trader_id，查找用户第一个启用TradingView的交易员
		traders, err := s.database.GetTradersWithTradingViewEnabled(user.ID)
		if err != nil || len(traders) == 0 {
			log.Printf("⚠️ 用户 %s 没有启用TradingView的交易员", user.ID)
			c.JSON(http.StatusBadRequest, gin.H{"error": "未找到启用TradingView的交易员，请在webhook中指定trader_id或在交易员配置中启用TradingView信号"})
			return
		}
		traderID = traders[0].ID
		log.Printf("📌 自动分配交易员: %s", traderID)
	}

	// 验证必需字段
	symbol, ok := payload["symbol"].(string)
	if !ok || symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少或无效的symbol字段"})
		return
	}

	action, ok := payload["action"].(string)
	if !ok || (action != "buy" && action != "sell") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少或无效的action字段（必须是buy或sell）"})
		return
	}

	// 创建警报记录
	alertID, err := s.database.CreateTradingViewAlert(user.ID, traderID, payload)
	if err != nil {
		log.Printf("❌ 创建TradingView警报失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存警报失败", "details": err.Error()})
		return
	}

	log.Printf("✅ TradingView警报已接收: 用户=%s, 交易员=%s, 币种=%s, 操作=%s, alertID=%s", user.ID, traderID, symbol, action, alertID)
	log.Printf("INFO: Webhook received (trader=%s, symbol=%s, action=%s, alertID=%s)", traderID, symbol, action, alertID)

	// 构建响应对象
	response := gin.H{
		"success":   true,
		"message":   "警报已接收",
		"trader_id": traderID,
		"symbol":    symbol,
		"action":    action,
		"alert_id":  alertID,
	}

	// 如果交易员启用了TradingView，触发立即决策周期
	at, err := s.traderManager.GetTrader(traderID)
	if err == nil && at != nil {
		// 从数据库确认是否开启TradingView
		traderConfig, _, _, cfgErr := s.database.GetTraderConfig(user.ID, traderID)
		if cfgErr != nil {
			log.Printf("⚠️ 无法获取交易员配置 %s: %v", traderID, cfgErr)
			log.Printf("WARN: Skip trigger because trader config load failed (trader=%s, alertID=%s, err=%v)", traderID, alertID, cfgErr)
			response["warning"] = "无法获取交易员配置，未触发决策"
			c.JSON(http.StatusOK, response)
			return
		}

		if !traderConfig.UseTradingView {
			log.Printf("⚠️ [%s] 未开启TradingView信号，跳过触发", traderID)
			log.Printf("WARN: Skip trigger because UseTradingView=false at DB (trader=%s, alertID=%s)", traderID, alertID)
			response["warning"] = "交易员未开启TradingView信号，未触发决策"
			c.JSON(http.StatusOK, response)
			return
		}

		// 检查交易员是否正在运行
		status := at.GetStatus()
		isRunning, _ := status["is_running"].(bool)
		memUseTV, _ := status["use_tradingview"].(bool)
		dbIsRunning := traderConfig.IsRunning // 数据库中的运行状态
		response["trader_running"] = isRunning
		log.Printf("ℹ️ [%s] TradingView触发检查: is_running(mem)=%v, is_running(DB)=%v, UseTradingView(DB)=%v, UseTradingView(mem)=%v", traderID, isRunning, dbIsRunning, traderConfig.UseTradingView, memUseTV)
		if memUseTV != traderConfig.UseTradingView {
			log.Printf("WARN: TradingView flag mismatch between DB and memory (trader=%s, db=%v, mem=%v)", traderID, traderConfig.UseTradingView, memUseTV)
		}
		log.Printf("INFO: Trigger check (trader=%s, is_running(mem)=%v, is_running(DB)=%v, UseTradingView=%v, alertID=%s)", traderID, isRunning, dbIsRunning, traderConfig.UseTradingView, alertID)

		if !isRunning {
			// 如果数据库状态显示交易员已停止（用户明确停止），则不应自动启动
			if !dbIsRunning {
				log.Printf("⏹️ [%s] 交易员已停止（数据库状态），不自动启动以处理webhook", traderID)
				log.Printf("INFO: Trader is stopped in DB, respecting stop status (trader=%s, alertID=%s)", traderID, alertID)
				response["warning"] = "交易员已停止，请手动启动交易员以处理webhook"
				c.JSON(http.StatusOK, response)
				return
			}

			// 如果数据库状态显示运行中但内存中未运行，说明交易员可能崩溃了，可以自动启动
			if traderConfig != nil && traderConfig.UseTradingView {
				// Reload config first to ensure latest settings
				if reloadErr := s.traderManager.ReloadTraderFromDB(s.database, user.ID, traderID); reloadErr != nil {
					log.Printf("⚠️ Failed to reload trader config before auto-start: %v", reloadErr)
					response["warning"] = "无法重新加载交易员配置"
					c.JSON(http.StatusOK, response)
					return
				}

				// 重新检查数据库状态（防止在reload期间状态发生变化）
				freshConfig, _, _, freshCfgErr := s.database.GetTraderConfig(user.ID, traderID)
				if freshCfgErr != nil {
					log.Printf("⚠️ Failed to get fresh trader config after reload: %v", freshCfgErr)
					response["warning"] = "无法获取最新交易员配置"
					c.JSON(http.StatusOK, response)
					return
				}

				// 如果重新加载后发现数据库状态已变为停止，则不自动启动
				if !freshConfig.IsRunning {
					log.Printf("⏹️ [%s] 重新加载后发现交易员已停止（数据库状态），不自动启动以处理webhook", traderID)
					log.Printf("INFO: Trader is stopped in DB after reload, respecting stop status (trader=%s, alertID=%s)", traderID, alertID)
					response["warning"] = "交易员已停止，请手动启动交易员以处理webhook"
					c.JSON(http.StatusOK, response)
					return
				}

				// Get fresh instance after reload
				at, err = s.traderManager.GetTrader(traderID)
				if err != nil {
					log.Printf("❌ Failed to get trader after reload: %v", err)
					response["error"] = "无法获取交易员实例"
					c.JSON(http.StatusOK, response)
					return
				}
				// 自动启动交易员（TradingView模式，仅在交易员崩溃时）
				log.Printf("🔄 [%s] TradingView模式：检测到交易员崩溃，自动重启以处理webhook", traderConfig.Name)
				log.Printf("INFO: Auto-start trader for TradingView webhook (trader crashed, trader=%s, alertID=%s)", traderID, alertID)
				go func() {
					if err := at.Run(); err != nil {
						log.Printf("❌ [%s] 自动启动失败: %v", traderConfig.Name, err)
						// 更新数据库状态为停止
						_ = s.database.UpdateTraderStatus(user.ID, traderID, false)
					}
				}()
				// 更新数据库状态
				_ = s.database.UpdateTraderStatus(user.ID, traderID, true)
				// 等待交易员启动
				time.Sleep(500 * time.Millisecond)
				// 再次检查运行状态
				status = at.GetStatus()
				isRunning, _ = status["is_running"].(bool)
				response["trader_running"] = isRunning

				if !isRunning {
					log.Printf("⚠️ [%s] 交易员自动启动后仍未运行，跳过触发决策周期", traderConfig.Name)
					log.Printf("WARN: Auto-start failed, trader still not running (trader=%s, alertID=%s)", traderID, alertID)
					response["warning"] = "交易员自动启动失败，请检查日志并手动启动"
					response["trigger_error"] = "trader failed to start"
					c.JSON(http.StatusOK, response)
					return
				}

				response["warning"] = "交易员未运行，已自动启动（TradingView模式）"
				log.Printf("✓ [%s] 交易员已自动启动，准备处理webhook", traderConfig.Name)
			} else {
				log.Printf("⚠️ [%s] 交易员未运行且未启用TradingView，无法自动启动", traderID)
				log.Printf("WARN: Trader not running and UseTradingView=false, cannot auto-start (trader=%s, alertID=%s)", traderID, alertID)
				response["warning"] = "交易员未运行，请手动启动交易员以处理webhook"
				c.JSON(http.StatusOK, response)
				return
			}
		}

		// 触发决策周期
		if err := at.TriggerTradingViewDecisionCycle(alertID); err != nil {
			log.Printf("⚠️ [%s] 触发决策周期失败: %v", traderID, err)
			log.Printf("ERROR: Failed to trigger TradingView decision (trader=%s, alertID=%s, err=%v)", traderID, alertID, err)
			response["trigger_error"] = err.Error()
		} else {
			log.Printf("✓ [%s] 已成功触发TradingView决策周期: alertID=%s", traderID, alertID)
			log.Printf("INFO: Triggered TradingView decision successfully (trader=%s, alertID=%s)", traderID, alertID)
		}
	} else {
		log.Printf("⚠️ 无法获取交易员实例 %s: %v", traderID, err)
		log.Printf("ERROR: Trader instance not found (trader=%s, alertID=%s, err=%v)", traderID, alertID, err)
		response["error"] = fmt.Sprintf("无法获取交易员实例: %v", err)
	}

	c.JSON(http.StatusOK, response)
}

// handleGetWebhookInfo 获取用户的webhook信息
func (s *Server) handleGetWebhookInfo(c *gin.Context) {
	userID := c.GetString("user_id")

	// 生成或获取API key
	apiKey, err := s.database.GenerateWebhookAPIKey(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成API key失败", "details": err.Error()})
		return
	}

	// 构建webhook URL（从请求中获取host）
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

// handleGetRecentAlerts 获取最近的警报
func (s *Server) handleGetRecentAlerts(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Query("trader_id")

	// 使用database方法获取最近的警报
	alerts, err := s.database.GetRecentTradingViewAlerts(userID, traderID, 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"alerts": alerts})
}
