import { useEffect, useState } from 'react'
import { Copy, Check, Send, AlertCircle } from 'lucide-react'
import { api } from '../lib/api'
import { useAuth } from '../contexts/AuthContext'
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
  "quantity": "900",
  "position_size": "-900",
  "entry": "85.25",
  "sl": "90.5",
  "tp": "74.75"
}`)
  const [testResult, setTestResult] = useState<string>('')
  const [isTesting, setIsTesting] = useState(false)
  const [recentAlerts, setRecentAlerts] = useState<TradingViewAlert[]>([])
  const [selectedTraderId, setSelectedTraderId] = useState<string>('')
  const [traders, setTraders] = useState<TraderInfo[]>([])

  useEffect(() => {
    if (user && token) {
      loadWebhookInfo()
      loadTraders()
      loadRecentAlerts()
    }
  }, [user, token])

  const loadWebhookInfo = async () => {
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
      console.error('Failed to load webhook info:', error)
    }
  }

  const loadTraders = async () => {
    try {
      const traderList = await traderApi.getTraders()
      setTraders(traderList)
      if (traderList.length > 0 && !selectedTraderId) {
        setSelectedTraderId(traderList[0].trader_id)
      }
    } catch (error) {
      console.error('Failed to load traders:', error)
    }
  }

  const loadRecentAlerts = async () => {
    try {
      const alerts = await api.getRecentAlerts(
        selectedTraderId || undefined
      )
      setRecentAlerts(alerts)
    } catch (error) {
      console.error('Failed to load alerts:', error)
    }
  }

  useEffect(() => {
    if (selectedTraderId) {
      loadRecentAlerts()
      // 更新测试payload中的trader_id
      setTestPayload((prevPayload) => {
        try {
          const payloadObj = JSON.parse(prevPayload)
          payloadObj.trader_id = selectedTraderId
          return JSON.stringify(payloadObj, null, 2)
        } catch {
          return prevPayload
        }
      })
    }
  }, [selectedTraderId])

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
      // 刷新警报列表
      setTimeout(() => loadRecentAlerts(), 1000)
    } catch (error: any) {
      setTestResult(t('webhookPage.error', language, { message: error.message }))
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
        return 'text-yellow-500'
      case 'accepted':
        return 'text-blue-500'
      case 'executed':
        return 'text-green-500'
      case 'rejected':
        return 'text-red-500'
      default:
        return 'text-gray-500'
    }
  }

  if (!user || !token) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="text-center">
          <p className="text-[#848E9C]">{t('webhookPage.loginRequired', language)}</p>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-[#EAECEF]">{t('webhookPage.title', language)}</h1>
      </div>

      {/* Webhook URL & API Key */}
      <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-6">
        <h2 className="text-lg font-semibold text-[#EAECEF] mb-4">{t('webhookPage.webhookInfo', language)}</h2>
        <div className="space-y-4">
          <div>
            <label className="text-sm text-[#848E9C] block mb-2">{t('webhookPage.webhookUrl', language)}</label>
            <div className="flex items-center gap-2">
              <input
                type="text"
                value={webhookInfo?.webhook_url || ''}
                readOnly
                className="flex-1 bg-[#181A20] border border-[#2B3139] rounded px-3 py-2 text-[#EAECEF] text-sm"
              />
              <button
                onClick={() => webhookInfo && handleCopy(webhookInfo.webhook_url)}
                className="px-4 py-2 bg-[#2B3139] hover:bg-[#3A4149] rounded text-[#EAECEF] flex items-center gap-2 transition-colors"
              >
                {copied ? <Check size={16} /> : <Copy size={16} />}
              </button>
            </div>
          </div>
          <div>
            <label className="text-sm text-[#848E9C] block mb-2">{t('webhookPage.apiKey', language)}</label>
            <div className="flex items-center gap-2">
              <input
                type="text"
                value={webhookInfo?.api_key || ''}
                readOnly
                className="flex-1 bg-[#181A20] border border-[#2B3139] rounded px-3 py-2 text-[#EAECEF] text-sm font-mono"
              />
              <button
                onClick={() => webhookInfo && handleCopy(webhookInfo.api_key)}
                className="px-4 py-2 bg-[#2B3139] hover:bg-[#3A4149] rounded text-[#EAECEF] flex items-center gap-2 transition-colors"
              >
                {copied ? <Check size={16} /> : <Copy size={16} />}
              </button>
            </div>
            <p className="text-xs text-[#848E9C] mt-2">
              {t('webhookPage.apiKeyHint', language)}
            </p>
          </div>
        </div>
      </div>

      {/* Test Webhook */}
      <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-6">
        <h2 className="text-lg font-semibold text-[#EAECEF] mb-4">{t('webhookPage.testWebhook', language)}</h2>
        <div className="space-y-4">
          <div>
            <label className="text-sm text-[#848E9C] block mb-2">{t('webhookPage.selectTrader', language)}</label>
            <select
              value={selectedTraderId}
              onChange={(e) => {
                setSelectedTraderId(e.target.value)
                try {
                  const payloadObj = JSON.parse(testPayload)
                  payloadObj.trader_id = e.target.value || ''
                  setTestPayload(JSON.stringify(payloadObj, null, 2))
                } catch {
                  // ignore
                }
              }}
              className="w-full bg-[#181A20] border border-[#2B3139] rounded px-3 py-2 text-[#EAECEF]"
            >
              <option value="">{t('webhookPage.autoAssign', language)}</option>
              {traders.map((trader) => (
                <option key={trader.trader_id} value={trader.trader_id}>
                  {trader.trader_name}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="text-sm text-[#848E9C] block mb-2">{t('webhookPage.jsonPayload', language)}</label>
            <textarea
              value={testPayload}
              onChange={(e) => setTestPayload(e.target.value)}
              className="w-full h-64 bg-[#181A20] border border-[#2B3139] rounded px-3 py-2 text-[#EAECEF] font-mono text-sm"
              spellCheck={false}
            />
          </div>
          <button
            onClick={handleTestWebhook}
            disabled={isTesting}
            className="px-6 py-2 bg-[#F0B90B] hover:bg-[#FCD535] text-[#0B0E11] rounded font-semibold flex items-center gap-2 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <Send size={16} />
            {isTesting ? t('webhookPage.sending', language) : t('webhookPage.sendTest', language)}
          </button>
          {testResult && (
            <div className="mt-4 p-4 bg-[#181A20] border border-[#2B3139] rounded">
              <pre className="text-sm text-[#EAECEF] whitespace-pre-wrap">{testResult}</pre>
            </div>
          )}
        </div>
      </div>

      {/* Recent Alerts */}
      <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-[#EAECEF]">{t('webhookPage.recentAlerts', language)}</h2>
          <select
            value={selectedTraderId}
            onChange={(e) => setSelectedTraderId(e.target.value)}
            className="bg-[#181A20] border border-[#2B3139] rounded px-3 py-1 text-[#EAECEF] text-sm"
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
          <div className="text-center py-8 text-[#848E9C]">
            <AlertCircle className="mx-auto mb-2 opacity-50" size={32} />
            <p>{t('webhookPage.noAlerts', language)}</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="border-b border-[#2B3139]">
                  <th className="text-left py-2 px-4 text-sm text-[#848E9C]">{t('webhookPage.time', language)}</th>
                  <th className="text-left py-2 px-4 text-sm text-[#848E9C]">{t('webhookPage.symbol', language)}</th>
                  <th className="text-left py-2 px-4 text-sm text-[#848E9C]">{t('webhookPage.action', language)}</th>
                  <th className="text-left py-2 px-4 text-sm text-[#848E9C]">{t('webhookPage.entryPrice', language)}</th>
                  <th className="text-left py-2 px-4 text-sm text-[#848E9C]">{t('webhookPage.stopLoss', language)}</th>
                  <th className="text-left py-2 px-4 text-sm text-[#848E9C]">{t('webhookPage.takeProfit', language)}</th>
                  <th className="text-left py-2 px-4 text-sm text-[#848E9C]">{t('webhookPage.quantity', language)}</th>
                  <th className="text-left py-2 px-4 text-sm text-[#848E9C]">{t('webhookPage.status', language)}</th>
                </tr>
              </thead>
              <tbody>
                {recentAlerts.map((alert) => (
                  <tr key={alert.id} className="border-b border-[#2B3139]">
                    <td className="py-2 px-4 text-sm text-[#EAECEF]">
                      {formatDate(alert.created_at)}
                    </td>
                    <td className="py-2 px-4 text-sm text-[#EAECEF] font-mono">
                      {alert.symbol}
                    </td>
                    <td className="py-2 px-4 text-sm text-[#EAECEF]">
                      {alert.action === 'buy' ? (
                        <span className="text-green-500">{t('webhookPage.buy', language)}</span>
                      ) : (
                        <span className="text-red-500">{t('webhookPage.sell', language)}</span>
                      )}
                    </td>
                    <td className="py-2 px-4 text-sm text-[#EAECEF]">{alert.entry}</td>
                    <td className="py-2 px-4 text-sm text-[#EAECEF]">{alert.sl}</td>
                    <td className="py-2 px-4 text-sm text-[#EAECEF]">{alert.tp}</td>
                    <td className="py-2 px-4 text-sm text-[#EAECEF]">{alert.quantity}</td>
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

