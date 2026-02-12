import { useEffect, useState } from 'react'
import { Copy, Check, Send, AlertCircle } from 'lucide-react'
import { api } from '../lib/api'
import { useAuth, isFollower } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { api as traderApi } from '../lib/api'
import { t } from '../i18n/translations'
import type { TraderInfo } from '../types'

interface WebhookInfo {
  webhook_url: string
  api_key: string
}

interface TradingViewAlert {
  id: string
  symbol: string
  action: string
  entry: number
  sl: number
  tp: number
  quantity: number
  status: string
  created_at: string
  trader_id?: string
  trader_name?: string
}

export default function WebhookPage() {
  const { user, token } = useAuth()
  const { language } = useLanguage()
  const [webhookInfo, setWebhookInfo] = useState<WebhookInfo | null>(null)
  const [copied, setCopied] = useState(false)
  const [testPayload, setTestPayload] = useState(`{
  "apikey": "",
  "trader_id": "",
  "botname": "botTradingview",
  "symbol": "SOLUSDT",
  "action": "sell",
  "exchange": "BINANCE(FUTURE)",
  "pricetype": "LIMIT",
  "quantity": "0.015",
  "position_size": "-0.015",
  "entry": "131.29",
  "sl": "140",
  "tp": "120"
}`)
  const [testResult, setTestResult] = useState<string>('')
  const [isTesting, setIsTesting] = useState(false)
  const [recentAlerts, setRecentAlerts] = useState<TradingViewAlert[]>([])
  const [selectedTraderIds, setSelectedTraderIds] = useState<string[]>([])
  const [traders, setTraders] = useState<TraderInfo[]>([])

  useEffect(() => {
    // Only load data if user is not a follower
    if (user && token && !isFollower(user)) {
      loadWebhookInfo()
      loadTraders()
      loadRecentAlerts()
    }
  }, [user, token])

  const loadWebhookInfo = async () => {
    // Skip if user is a follower
    if (isFollower(user)) {
      return
    }
    try {
      const info = await api.getWebhookInfo()
      setWebhookInfo(info)
      // 更新测试payload中的apikey
      setTestPayload((prevPayload) => {
        try {
          const payloadObj = JSON.parse(prevPayload)
          payloadObj.apikey = info.api_key
          return JSON.stringify(payloadObj, null, 2)
        } catch {
          // 如果解析失败，返回原始值
          return prevPayload
        }
      })
    } catch (error) {
      // Silently handle 403 errors for followers
      if (error instanceof Error && error.message === 'Permission denied') {
        return
      }
      console.error('Failed to load webhook info:', error)
    }
  }

  const loadTraders = async () => {
    try {
      const traderList = await traderApi.getTraders()
      setTraders(traderList)
    } catch (error) {
      console.error('Failed to load traders:', error)
    }
  }

  const loadRecentAlerts = async () => {
    // Skip if user is a follower
    if (isFollower(user)) {
      return
    }
    try {
      // If single trader selected, load alerts for that trader
      // If multiple traders selected or none selected, load all alerts
      const traderId =
        selectedTraderIds.length === 1 ? selectedTraderIds[0] : undefined
      const alerts = await api.getRecentAlerts(traderId)
      setRecentAlerts(alerts)
    } catch (error) {
      // Silently handle 403 errors for followers
      if (error instanceof Error && error.message === 'Permission denied') {
        return
      }
      console.error('Failed to load alerts:', error)
    }
  }

  useEffect(() => {
    // Only load alerts if user is not a follower
    if (!isFollower(user)) {
      loadRecentAlerts()
      // Update test payload with trader_ids or trader_id
      setTestPayload((prevPayload) => {
        try {
          const payloadObj = JSON.parse(prevPayload)
          // Remove old trader_id/trader_ids fields
          delete payloadObj.trader_id
          delete payloadObj.trader_ids

          // Add appropriate field based on selection
          if (selectedTraderIds.length === 0) {
            // No selection - omit trader field (auto-assign)
          } else if (selectedTraderIds.length === 1) {
            // Single trader - use trader_id for backward compatibility
            payloadObj.trader_id = selectedTraderIds[0]
          } else {
            // Multiple traders - use trader_ids array
            payloadObj.trader_ids = selectedTraderIds
          }
          return JSON.stringify(payloadObj, null, 2)
        } catch {
          return prevPayload
        }
      })
    }
  }, [selectedTraderIds, user])

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleTestWebhook = async () => {
    setIsTesting(true)
    setTestResult('')
    try {
      const payload = JSON.parse(testPayload)
      const result = await api.testWebhook(payload)
      setTestResult(JSON.stringify(result, null, 2))
      // Refresh alerts list after successful webhook test to show new alerts
      setTimeout(() => {
        loadRecentAlerts()
      }, 1500) // Slightly longer delay to ensure backend has processed
    } catch (error: any) {
      setTestResult(
        t('webhookPage.error', language, { message: error.message })
      )
    } finally {
      setIsTesting(false)
    }
  }

  const formatDate = (dateString: string) => {
    try {
      const date = new Date(dateString)
      return date.toLocaleString(language === 'zh' ? 'zh-CN' : 'en-US')
    } catch {
      return dateString
    }
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'pending':
        return 'text-gold'
      case 'accepted':
        return 'text-blue-500'
      case 'executed':
        return 'text-profit'
      case 'rejected':
        return 'text-loss'
      default:
        return ''
    }
  }

  if (!user || !token) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="text-center">
          <p style={{ color: 'var(--text-secondary)' }}>
            {t('webhookPage.loginRequired', language)}
          </p>
        </div>
      </div>
    )
  }

  // Show access denied message for followers
  if (isFollower(user)) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="text-center space-y-4">
          <AlertCircle
            className="mx-auto mb-4 opacity-50"
            size={48}
            style={{ color: 'var(--text-secondary)' }}
          />
          <h2
            className="text-xl font-semibold"
            style={{ color: 'var(--text-primary)' }}
          >
            {t('webhookPage.accessDenied', language) || 'Access Restricted'}
          </h2>
          <p style={{ color: 'var(--text-secondary)' }}>
            {t('webhookPage.followerRestriction', language) ||
              'This feature is only available to traders. Please upgrade your account to access webhook management.'}
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="flex items-center justify-between">
        <h1
          className="text-2xl font-bold"
          style={{ color: 'var(--text-primary)' }}
        >
          {t('webhookPage.title', language)}
        </h1>
      </div>

      {/* Webhook URL & API Key */}
      <div className="nofx-card p-6">
        <h2
          className="text-lg font-semibold mb-4"
          style={{ color: 'var(--text-primary)' }}
        >
          {t('webhookPage.webhookInfo', language)}
        </h2>
        <div className="space-y-4">
          <div>
            <label
              className="text-sm block mb-2"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('webhookPage.webhookUrl', language)}
            </label>
            <div className="flex items-center gap-2">
              <input
                type="text"
                value={webhookInfo?.webhook_url || ''}
                readOnly
                className="flex-1 rounded px-3 py-2 text-sm"
                style={{
                  background: 'var(--navy-dark)',
                  border: '1px solid var(--panel-border)',
                  color: 'var(--text-primary)',
                }}
              />
              <button
                onClick={() =>
                  webhookInfo && handleCopy(webhookInfo.webhook_url)
                }
                className="px-4 py-2 rounded flex items-center gap-2 transition-colors"
                style={{
                  background: 'var(--navy-dark)',
                  color: 'var(--text-primary)',
                }}
                onMouseEnter={(e) =>
                  (e.currentTarget.style.background = 'var(--navy-light)')
                }
                onMouseLeave={(e) =>
                  (e.currentTarget.style.background = 'var(--navy-dark)')
                }
              >
                {copied ? <Check size={16} /> : <Copy size={16} />}
              </button>
            </div>
          </div>
          <div>
            <label
              className="text-sm block mb-2"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('webhookPage.apiKey', language)}
            </label>
            <div className="flex items-center gap-2">
              <input
                type="text"
                value={webhookInfo?.api_key || ''}
                readOnly
                className="flex-1 rounded px-3 py-2 text-sm font-mono"
                style={{
                  background: 'var(--navy-dark)',
                  border: '1px solid var(--panel-border)',
                  color: 'var(--text-primary)',
                }}
              />
              <button
                onClick={() => webhookInfo && handleCopy(webhookInfo.api_key)}
                className="px-4 py-2 rounded flex items-center gap-2 transition-colors"
                style={{
                  background: 'var(--navy-dark)',
                  color: 'var(--text-primary)',
                }}
                onMouseEnter={(e) =>
                  (e.currentTarget.style.background = 'var(--navy-light)')
                }
                onMouseLeave={(e) =>
                  (e.currentTarget.style.background = 'var(--navy-dark)')
                }
              >
                {copied ? <Check size={16} /> : <Copy size={16} />}
              </button>
            </div>
            <p
              className="text-xs mt-2"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('webhookPage.apiKeyHint', language)}
            </p>
          </div>
        </div>
      </div>

      {/* Test Webhook */}
      <div className="nofx-card p-6">
        <h2
          className="text-lg font-semibold mb-4"
          style={{ color: 'var(--text-primary)' }}
        >
          {t('webhookPage.testWebhook', language)}
        </h2>
        <div className="space-y-4">
          <div>
            <label
              className="text-sm block mb-2"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('webhookPage.selectTrader', language)}
              {selectedTraderIds.length > 0 && (
                <span
                  className="ml-2 text-xs"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  ({selectedTraderIds.length}{' '}
                  {selectedTraderIds.length === 1 ? 'trader' : 'traders'}{' '}
                  selected)
                </span>
              )}
            </label>
            <select
              multiple
              value={selectedTraderIds}
              onChange={(e) => {
                const selected = Array.from(
                  e.target.selectedOptions,
                  (option) => option.value
                )
                setSelectedTraderIds(selected)
              }}
              className="w-full rounded px-3 py-2 min-h-[100px]"
              style={{
                background: 'var(--navy-dark)',
                border: '1px solid var(--panel-border)',
                color: 'var(--text-primary)',
              }}
            >
              {traders.map((trader) => (
                <option key={trader.trader_id} value={trader.trader_id}>
                  {trader.trader_name}
                </option>
              ))}
            </select>
            <p
              className="text-xs mt-1"
              style={{ color: 'var(--text-secondary)' }}
            >
              Hold Ctrl/Cmd to select multiple traders. Leave empty for
              auto-assign.
            </p>
          </div>
          <div>
            <label
              className="text-sm block mb-2"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('webhookPage.jsonPayload', language)}
            </label>
            <textarea
              value={testPayload}
              onChange={(e) => setTestPayload(e.target.value)}
              className="w-full h-64 rounded px-3 py-2 font-mono text-sm"
              style={{
                background: 'var(--navy-dark)',
                border: '1px solid var(--panel-border)',
                color: 'var(--text-primary)',
              }}
              spellCheck={false}
            />
          </div>
          <button
            onClick={handleTestWebhook}
            disabled={isTesting}
            className="btn-primary px-6 py-2 rounded font-semibold flex items-center gap-2 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <Send size={16} />
            {isTesting
              ? t('webhookPage.sending', language)
              : t('webhookPage.sendTest', language)}
          </button>
          {testResult && (
            <div
              className="mt-4 p-4 rounded"
              style={{
                background: 'var(--navy-dark)',
                border: '1px solid var(--panel-border)',
              }}
            >
              <div
                className="mb-2 text-sm font-semibold"
                style={{ color: 'var(--text-primary)' }}
              >
                Response:
              </div>
              <pre
                className="text-sm whitespace-pre-wrap"
                style={{ color: 'var(--text-primary)' }}
              >
                {testResult}
              </pre>
            </div>
          )}
        </div>
      </div>

      {/* Recent Alerts */}
      <div className="nofx-card p-6">
        <div className="flex items-center justify-between mb-4">
          <h2
            className="text-lg font-semibold"
            style={{ color: 'var(--text-primary)' }}
          >
            {t('webhookPage.recentAlerts', language)}
          </h2>
          <select
            value={selectedTraderIds.length === 1 ? selectedTraderIds[0] : ''}
            onChange={(e) => {
              const value = e.target.value
              setSelectedTraderIds(value ? [value] : [])
            }}
            className="rounded px-3 py-1 text-sm"
            style={{
              background: 'var(--navy-dark)',
              border: '1px solid var(--panel-border)',
              color: 'var(--text-primary)',
            }}
          >
            <option value="">{t('webhookPage.allTraders', language)}</option>
            {traders.map((trader) => (
              <option key={trader.trader_id} value={trader.trader_id}>
                {trader.trader_name}
              </option>
            ))}
          </select>
        </div>
        {recentAlerts.length === 0 ? (
          <div
            className="text-center py-8"
            style={{ color: 'var(--text-secondary)' }}
          >
            <AlertCircle className="mx-auto mb-2 opacity-50" size={32} />
            <p>{t('webhookPage.noAlerts', language)}</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr style={{ borderBottom: '1px solid var(--panel-border)' }}>
                  <th
                    className="text-left py-2 px-4 text-sm"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t('webhookPage.time', language)}
                  </th>
                  <th
                    className="text-left py-2 px-4 text-sm"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    Trader
                  </th>
                  <th
                    className="text-left py-2 px-4 text-sm"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t('webhookPage.symbol', language)}
                  </th>
                  <th
                    className="text-left py-2 px-4 text-sm"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t('webhookPage.action', language)}
                  </th>
                  <th
                    className="text-left py-2 px-4 text-sm"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t('webhookPage.entryPrice', language)}
                  </th>
                  <th
                    className="text-left py-2 px-4 text-sm"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t('webhookPage.stopLoss', language)}
                  </th>
                  <th
                    className="text-left py-2 px-4 text-sm"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t('webhookPage.takeProfit', language)}
                  </th>
                  <th
                    className="text-left py-2 px-4 text-sm"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t('webhookPage.quantity', language)}
                  </th>
                  <th
                    className="text-left py-2 px-4 text-sm"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t('webhookPage.status', language)}
                  </th>
                </tr>
              </thead>
              <tbody>
                {recentAlerts.map((alert) => (
                  <tr
                    key={alert.id}
                    style={{ borderBottom: '1px solid var(--panel-border)' }}
                  >
                    <td
                      className="py-2 px-4 text-sm"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      {formatDate(alert.created_at)}
                    </td>
                    <td
                      className="py-2 px-4 text-sm"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      {alert.trader_name || '-'}
                    </td>
                    <td
                      className="py-2 px-4 text-sm font-mono"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      {alert.symbol}
                    </td>
                    <td
                      className="py-2 px-4 text-sm"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      {alert.action === 'buy' ? (
                        <span className="text-profit">
                          {t('webhookPage.buy', language)}
                        </span>
                      ) : (
                        <span className="text-loss">
                          {t('webhookPage.sell', language)}
                        </span>
                      )}
                    </td>
                    <td
                      className="py-2 px-4 text-sm"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      {alert.entry}
                    </td>
                    <td
                      className="py-2 px-4 text-sm"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      {alert.sl}
                    </td>
                    <td
                      className="py-2 px-4 text-sm"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      {alert.tp}
                    </td>
                    <td
                      className="py-2 px-4 text-sm"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      {alert.quantity}
                    </td>
                    <td className="py-2 px-4 text-sm">
                      <span className={getStatusColor(alert.status)}>
                        {alert.status === 'pending'
                          ? t('webhookPage.pending', language)
                          : alert.status === 'accepted'
                            ? t('webhookPage.accepted', language)
                            : alert.status === 'executed'
                              ? t('webhookPage.executed', language)
                              : alert.status === 'rejected'
                                ? t('webhookPage.rejected', language)
                                : alert.status}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
