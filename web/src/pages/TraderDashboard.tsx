import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams, useParams } from 'react-router-dom'
import useSWR, { mutate } from 'swr'
import { api } from '../lib/api'
import { EquityChart } from '../components/EquityChart'
import { ChartTabs } from '../components/ChartTabs'
import AILearning from '../components/AILearning'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth } from '../contexts/AuthContext'
import { t, type Language } from '../i18n/translations'
import { generateTraderSlug, parseTraderSlug } from '../lib/utils'
import {
  AlertTriangle,
  Bot,
  Brain,
  RefreshCw,
  TrendingUp,
  PieChart,
  Inbox,
  Send,
  Check,
  X,
  XCircle,
} from 'lucide-react'
import { stripLeadingIcons } from '../lib/text'
import { confirmToast, notify } from '../lib/notify'
import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  TraderInfo,
} from '../types'

// 获取友好的AI模型名称
function getModelDisplayName(modelId: string): string {
  switch (modelId.toLowerCase()) {
    case 'deepseek':
      return 'DeepSeek'
    case 'qwen':
      return 'Qwen'
    case 'claude':
      return 'Claude'
    default:
      return modelId.toUpperCase()
  }
}

export default function TraderDashboard() {
  const { language } = useLanguage()
  const { user, token } = useAuth()
  const navigate = useNavigate()
  const { slug } = useParams<{ slug?: string }>()
  const [searchParams, setSearchParams] = useSearchParams()
  const [selectedTraderId, setSelectedTraderId] = useState<string | undefined>(undefined)
  const [lastUpdate, setLastUpdate] = useState<string>('--:--:--')
  const [closingPositions, setClosingPositions] = useState<Set<string>>(new Set())
  const [selectedChartSymbol, setSelectedChartSymbol] = useState<string | undefined>(undefined)
  const [chartUpdateKey, setChartUpdateKey] = useState(0)

  // Decision record limit selection (read from localStorage, default 5)
  const [decisionLimit, setDecisionLimit] = useState<number>(() => {
    const saved = localStorage.getItem('decisionLimit')
    return saved ? parseInt(saved, 10) : 5
  })

  // Save to localStorage when limit changes
  const handleLimitChange = (newLimit: number) => {
    setDecisionLimit(newLimit)
    localStorage.setItem('decisionLimit', newLimit.toString())
  }

  // Get trader list (only when user is logged in)
  const { data: traders, error: tradersError } = useSWR<TraderInfo[]>(
    user && token ? 'traders' : null,
    api.getTraders,
    {
      refreshInterval: 10000,
      shouldRetryOnError: false,
    }
  )

  // Resolve trader ID from URL (slug format or query parameter for backward compatibility)
  useEffect(() => {
    if (!traders || traders.length === 0) return

    let traderId: string | undefined = undefined

    // Priority 1: Check slug format from URL path (/dashboard/:slug)
    if (slug) {
      // Try to find trader by matching slug
      for (const trader of traders) {
        const traderSlug = generateTraderSlug(trader.trader_name, trader.trader_id)
        if (traderSlug === slug) {
          traderId = trader.trader_id
          break
        }
      }
      // If not found by slug, try parsing id4 from slug (fallback)
      if (!traderId) {
        const id4 = parseTraderSlug(slug)
        if (id4) {
          const found = traders.find((t) => t.trader_id.endsWith(id4))
          if (found) {
            traderId = found.trader_id
          }
        }
      }
    }

    // Priority 2: Check query parameter (backward compatibility)
    if (!traderId) {
      const traderFromQuery = searchParams.get('trader')
      if (traderFromQuery) {
        traderId = traderFromQuery
      }
    }

    // Priority 3: Default to first trader if none selected
    if (!traderId && traders.length > 0) {
      traderId = traders[0].trader_id
    }

    if (traderId && traderId !== selectedTraderId) {
      setSelectedTraderId(traderId)
      
      // Update URL to use slug format if not already using it
      const selectedTrader = traders.find((t) => t.trader_id === traderId)
      if (selectedTrader && (!slug || slug !== generateTraderSlug(selectedTrader.trader_name, selectedTrader.trader_id))) {
        const newSlug = generateTraderSlug(selectedTrader.trader_name, selectedTrader.trader_id)
        navigate(`/dashboard/${newSlug}`, { replace: true })
      }
    }
  }, [slug, traders, searchParams, selectedTraderId, navigate])

  // Update URL when trader is selected
  const handleTraderSelect = (traderId: string) => {
    setSelectedTraderId(traderId)
    const selectedTrader = traders?.find((t) => t.trader_id === traderId)
    if (selectedTrader) {
      const newSlug = generateTraderSlug(selectedTrader.trader_name, selectedTrader.trader_id)
      navigate(`/dashboard/${newSlug}`, { replace: true })
    }
  }

  // 如果在trader页面，获取该trader的数据
  const { data: status } = useSWR<SystemStatus>(
    user && token && selectedTraderId ? `status-${selectedTraderId}` : null,
    () => api.getStatus(selectedTraderId),
    {
      refreshInterval: 15000,
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    }
  )

  const { data: account } = useSWR<AccountInfo>(
    user && token && selectedTraderId ? `account-${selectedTraderId}` : null,
    () => api.getAccount(selectedTraderId),
    {
      refreshInterval: 15000,
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    }
  )

  const { data: positions } = useSWR<Position[]>(
    user && token && selectedTraderId ? `positions-${selectedTraderId}` : null,
    () => api.getPositions(selectedTraderId),
    {
      refreshInterval: 15000,
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    }
  )

  const { data: decisions } = useSWR<DecisionRecord[]>(
    user && token && selectedTraderId
      ? `decisions/latest-${selectedTraderId}-${decisionLimit}`
      : null,
    () => api.getLatestDecisions(selectedTraderId, decisionLimit),
    {
      refreshInterval: 30000,
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  const { data: stats } = useSWR<Statistics>(
    user && token && selectedTraderId ? `statistics-${selectedTraderId}` : null,
    () => api.getStatistics(selectedTraderId),
    {
      refreshInterval: 30000,
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  // Avoid unused variable warning
  void stats

  useEffect(() => {
    if (account) {
      const now = new Date().toLocaleTimeString()
      setLastUpdate(now)
    }
  }, [account])

  const selectedTrader = traders?.find((t) => t.trader_id === selectedTraderId)

  // Handle close position
  const handleClosePosition = async (symbol: string, side: 'long' | 'short') => {
    if (!selectedTraderId) {
      notify.error(t('selectTraderFirst', language) || 'Please select a trader first')
      return
    }

    const positionKey = `${symbol}-${side}`
    if (closingPositions.has(positionKey)) {
      return // Already closing
    }

    // Confirmation dialog
    const sideText = side === 'long' ? t('long', language) : t('short', language)
    const confirmMsg =
      t('confirmClosePosition', language, { symbol, side: sideText }) ||
      `Are you sure you want to close ${side} position for ${symbol}?`
    const confirmed = await confirmToast(confirmMsg, {
      title: language === 'zh' ? '确认平仓' : 'Confirm Close',
      okText: language === 'zh' ? '确认' : 'Confirm',
      cancelText: language === 'zh' ? '取消' : 'Cancel',
    })

    if (!confirmed) {
      return
    }

    setClosingPositions((prev) => new Set(prev).add(positionKey))

    try {
      await api.closePosition(selectedTraderId, symbol, side, 0) // 0 = close all
      notify.success(
        language === 'zh' ? '平仓成功' : 'Position closed successfully'
      )
      // 使用 SWR mutate 刷新数据而非重新加载页面
      await Promise.all([
        mutate(`positions-${selectedTraderId}`),
        mutate(`account-${selectedTraderId}`),
      ])
    } catch (error: any) {
      const errorMsg =
        error.message ||
        (language === 'zh' ? '平仓失败' : 'Failed to close position')
      notify.error(errorMsg)
    } finally {
      setClosingPositions((prev) => {
        const next = new Set(prev)
        next.delete(positionKey)
        return next
      })
    }
  }

  // If API failed with error, show empty state
  if (tradersError) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="text-center max-w-md mx-auto px-6">
          <div
            className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center"
            style={{
              background: 'rgba(0, 255, 127, 0.1)',
              border: '2px solid rgba(0, 255, 127, 0.3)',
            }}
          >
            <svg
              className="w-12 h-12"
              style={{ color: 'var(--green-primary)' }}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
          </div>
          <h2 className="text-2xl font-bold mb-3" style={{ color: '#EAECEF' }}>
            {t('dashboardEmptyTitle', language)}
          </h2>
          <p className="text-base mb-6" style={{ color: '#848E9C' }}>
            {t('dashboardEmptyDescription', language)}
          </p>
          <button
            onClick={() => navigate('/traders')}
            className="px-6 py-3 rounded-lg font-semibold transition-all hover:scale-105 active:scale-95"
            style={{
              background: 'linear-gradient(135deg, var(--green-primary) 0%, var(--green-light) 100%)',
              color: '#0B0E11',
              boxShadow: '0 4px 12px rgba(0, 255, 127, 0.3)',
            }}
          >
            {t('goToTradersPage', language)}
          </button>
        </div>
      </div>
    )
  }

  // If traders is loaded and empty, show empty state
  if (traders && traders.length === 0) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="text-center max-w-md mx-auto px-6">
          <div
            className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center"
            style={{
              background: 'rgba(0, 255, 127, 0.1)',
              border: '2px solid rgba(0, 255, 127, 0.3)',
            }}
          >
            <svg
              className="w-12 h-12"
              style={{ color: 'var(--green-primary)' }}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
          </div>
          <h2 className="text-2xl font-bold mb-3" style={{ color: '#EAECEF' }}>
            {t('dashboardEmptyTitle', language)}
          </h2>
          <p className="text-base mb-6" style={{ color: '#848E9C' }}>
            {t('dashboardEmptyDescription', language)}
          </p>
          <button
            onClick={() => navigate('/traders')}
            className="px-6 py-3 rounded-lg font-semibold transition-all hover:scale-105 active:scale-95"
            style={{
              background: 'linear-gradient(135deg, var(--green-primary) 0%, var(--green-light) 100%)',
              color: '#0B0E11',
              boxShadow: '0 4px 12px rgba(0, 255, 127, 0.3)',
            }}
          >
            {t('goToTradersPage', language)}
          </button>
        </div>
      </div>
    )
  }

  // If traders is still loading or selectedTrader is not ready, show skeleton
  if (!selectedTrader) {
    return (
      <div className="space-y-6">
        <div className="binance-card p-6 animate-pulse">
          <div className="skeleton h-8 w-48 mb-3"></div>
          <div className="flex gap-4">
            <div className="skeleton h-4 w-32"></div>
            <div className="skeleton h-4 w-24"></div>
            <div className="skeleton h-4 w-28"></div>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="binance-card p-5 animate-pulse">
              <div className="skeleton h-4 w-24 mb-3"></div>
              <div className="skeleton h-8 w-32"></div>
            </div>
          ))}
        </div>
        <div className="binance-card p-6 animate-pulse">
          <div className="skeleton h-6 w-40 mb-4"></div>
          <div className="skeleton h-64 w-full"></div>
        </div>
      </div>
    )
  }

  const highlightColor = '#60a5fa'

  return (
    <div>
      {/* Trader Header */}
      <div
        className="mb-6 rounded p-6 animate-scale-in"
        style={{
          background:
            'linear-gradient(135deg, rgba(0, 255, 127, 0.15) 0%, rgba(51, 255, 153, 0.05) 100%)',
          border: '1px solid rgba(0, 255, 127, 0.2)',
          boxShadow: '0 0 30px rgba(0, 255, 127, 0.15)',
        }}
      >
        <div className="flex items-start justify-between mb-3">
          <h2
            className="text-2xl font-bold flex items-center gap-2"
            style={{ color: '#EAECEF' }}
          >
            <span
              className="w-10 h-10 rounded-full flex items-center justify-center"
              style={{
                background: 'linear-gradient(135deg, var(--green-primary) 0%, var(--green-light) 100%)',
              }}
            >
              <Bot className="w-5 h-5" style={{ color: '#0B0E11' }} />
            </span>
            {selectedTrader.trader_name}
          </h2>

          {/* Trader Selector */}
          {traders && traders.length > 0 && (
            <div className="flex items-center gap-2">
              <span className="text-sm" style={{ color: '#848E9C' }}>
                {t('switchTrader', language)}:
              </span>
              <select
                value={selectedTraderId}
                onChange={(e) => handleTraderSelect(e.target.value)}
                className="rounded px-3 py-2 text-sm font-medium cursor-pointer transition-colors"
                style={{
                  background: 'var(--navy-dark)',
                  border: '1px solid var(--navy-light)',
                  color: '#EAECEF',
                }}
              >
                {traders.map((trader) => (
                  <option key={trader.trader_id} value={trader.trader_id}>
                    {trader.trader_name}
                  </option>
                ))}
              </select>
            </div>
          )}
        </div>
        <div
          className="flex items-center gap-4 text-sm"
          style={{ color: '#848E9C' }}
        >
          <span>
            AI Model:{' '}
            <span
              className="font-semibold"
              style={{
                color: selectedTrader.ai_model.includes('qwen')
                  ? '#c084fc'
                  : highlightColor,
              }}
            >
              {getModelDisplayName(
                selectedTrader.ai_model.split('_').pop() ||
                  selectedTrader.ai_model
              )}
            </span>
          </span>
          <span>•</span>
          <span>
            Prompt: <span className="font-semibold" style={{ color: highlightColor }}>{selectedTrader.system_prompt_template || '-'}</span>
          </span>
          {status && (
            <>
              <span>•</span>
              <span>Cycles: {status.call_count}</span>
              <span>•</span>
              <span>Runtime: {status.runtime_minutes} min</span>
            </>
          )}
        </div>
      </div>

      {/* Debug Info */}
      {account && (
        <div
          className="mb-4 p-3 rounded text-xs font-mono"
          style={{ background: 'var(--navy-dark)', border: '1px solid var(--navy-light)' }}
        >
          <div style={{ color: '#848E9C' }}>
            <RefreshCw className="inline w-4 h-4 mr-1 align-text-bottom" />
            Last Update: {lastUpdate} | Total Equity:{' '}
            {account?.total_equity?.toFixed(2) || '0.00'} | Available:{' '}
            {account?.available_balance?.toFixed(2) || '0.00'} | P&L:{' '}
            {account?.total_pnl?.toFixed(2) || '0.00'} (
            {account?.total_pnl_pct?.toFixed(2) || '0.00'}%)
          </div>
        </div>
      )}

      {/* Account Overview */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
        <StatCard
          title={t('totalEquity', language)}
          value={`${account?.total_equity?.toFixed(2) || '0.00'} USDT`}
          change={account?.total_pnl_pct || 0}
          positive={(account?.total_pnl ?? 0) > 0}
        />
        <StatCard
          title={t('availableBalance', language)}
          value={`${account?.available_balance?.toFixed(2) || '0.00'} USDT`}
          subtitle={`${account?.available_balance && account?.total_equity ? ((account.available_balance / account.total_equity) * 100).toFixed(1) : '0.0'}% ${t('free', language)}`}
        />
        <StatCard
          title={t('totalPnL', language)}
          value={`${account?.total_pnl !== undefined && account.total_pnl >= 0 ? '+' : ''}${account?.total_pnl?.toFixed(2) || '0.00'} USDT`}
          change={account?.total_pnl_pct || 0}
          positive={(account?.total_pnl ?? 0) >= 0}
        />
        <StatCard
          title={t('positions', language)}
          value={`${account?.position_count || 0}`}
          subtitle={`${t('margin', language)}: ${account?.margin_used_pct?.toFixed(1) || '0.0'}%`}
        />
      </div>

      {/* 主要内容区：左右分屏 */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
        {/* 左侧：图表 + 持仓 */}
        <div className="space-y-6">
          {/* Chart Tabs - Now includes TradingView */}
          <div className="binance-card-enhanced animate-slide-in" style={{ animationDelay: '0.1s' }}>
            <ChartTabs
              traderId={selectedTrader.trader_id}
              selectedSymbol={selectedChartSymbol}
              updateKey={chartUpdateKey}
              exchangeId={selectedTrader.exchange_id}
            />
          </div>

          {/* Current Positions */}
          <div
            className="binance-card p-6 animate-slide-in"
            style={{ animationDelay: '0.15s' }}
          >
            <div className="flex items-center justify-between mb-5">
              <h2
                className="text-xl font-bold flex items-center gap-2"
                style={{ color: '#EAECEF' }}
              >
                <TrendingUp className="w-5 h-5" style={{ color: 'var(--green-primary)' }} />
                {t('currentPositions', language)}
              </h2>
              {positions && positions.length > 0 && (
                <div
                  className="text-xs px-3 py-1 rounded"
                  style={{
                    background: 'rgba(0, 255, 127, 0.1)',
                    color: 'var(--green-primary)',
                    border: '1px solid rgba(0, 255, 127, 0.2)',
                  }}
                >
                  {positions.length} {t('active', language)}
                </div>
              )}
            </div>
            {positions && positions.length > 0 ? (
              <>
                {/* Desktop Table View - Improved Layout */}
                <div className="hidden lg:block overflow-x-auto">
                  <table className="w-full text-xs" style={{ tableLayout: 'auto', minWidth: '1000px' }}>
                    <thead className="text-left border-b" style={{ borderColor: 'var(--navy-light)' }}>
                      <tr>
                        <th className="pb-3 px-2 font-semibold whitespace-nowrap" style={{ color: '#848E9C', minWidth: '90px' }}>
                          {t('symbol', language)}
                        </th>
                        <th className="pb-3 px-2 font-semibold whitespace-nowrap" style={{ color: '#848E9C', minWidth: '70px' }}>
                          {t('side', language)}
                        </th>
                        <th className="pb-3 px-2 font-semibold whitespace-nowrap" style={{ color: '#848E9C', minWidth: '100px' }}>
                          {t('entryPrice', language)}
                        </th>
                        <th className="pb-3 px-2 font-semibold whitespace-nowrap" style={{ color: '#848E9C', minWidth: '100px' }}>
                          {t('markPrice', language)}
                        </th>
                        <th className="pb-3 px-2 font-semibold whitespace-nowrap" style={{ color: '#848E9C', minWidth: '90px' }}>
                          {t('quantity', language)}
                        </th>
                        <th className="pb-3 px-2 font-semibold whitespace-nowrap" style={{ color: '#848E9C', minWidth: '110px' }}>
                          {t('positionValue', language)}
                        </th>
                        <th className="pb-3 px-2 font-semibold whitespace-nowrap" style={{ color: '#848E9C', minWidth: '70px' }}>
                          {t('leverage', language)}
                        </th>
                        <th className="pb-3 px-2 font-semibold whitespace-nowrap" style={{ color: '#848E9C', minWidth: '120px' }}>
                          {t('unrealizedPnL', language)}
                        </th>
                        <th className="pb-3 px-2 font-semibold whitespace-nowrap" style={{ color: '#848E9C', minWidth: '100px' }}>
                          {t('liqPrice', language)}
                        </th>
                        <th className="pb-3 px-2 font-semibold text-right whitespace-nowrap" style={{ color: '#848E9C', minWidth: '90px' }}>
                          {t('action', language) || 'Action'}
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {positions.map((pos, i) => (
                        <tr
                          key={i}
                          className="border-b last:border-0 hover:bg-opacity-50 transition-colors"
                          style={{ borderColor: 'var(--navy-light)' }}
                        >
                          <td className="py-3 px-2 font-mono font-semibold whitespace-nowrap">
                            <button
                              onClick={() => {
                                setSelectedChartSymbol(pos.symbol)
                                setChartUpdateKey(prev => prev + 1)
                              }}
                              className="hover:underline transition-all cursor-pointer"
                              style={{ color: '#EAECEF' }}
                              title={language === 'zh' ? '点击查看图表' : 'Click to view chart'}
                            >
                              {pos.symbol}
                            </button>
                          </td>
                          <td className="py-3 px-2 whitespace-nowrap">
                            <span
                              className="px-2 py-1 rounded text-xs font-bold"
                              style={
                                pos.side === 'long'
                                  ? {
                                      background: 'rgba(14, 203, 129, 0.1)',
                                      color: '#0ECB81',
                                    }
                                  : {
                                      background: 'rgba(246, 70, 93, 0.1)',
                                      color: '#F6465D',
                                    }
                              }
                            >
                              {t(pos.side === 'long' ? 'long' : 'short', language)}
                            </span>
                          </td>
                          <td className="py-3 px-2 font-mono whitespace-nowrap" style={{ color: '#EAECEF' }}>
                            {pos.entry_price.toFixed(4)}
                          </td>
                          <td className="py-3 px-2 font-mono whitespace-nowrap" style={{ color: '#EAECEF' }}>
                            {pos.mark_price.toFixed(4)}
                          </td>
                          <td className="py-3 px-2 font-mono whitespace-nowrap" style={{ color: '#EAECEF' }}>
                            {pos.quantity.toFixed(4)}
                          </td>
                          <td className="py-3 px-2 font-mono font-bold whitespace-nowrap" style={{ color: '#EAECEF' }}>
                            {(pos.quantity * pos.mark_price).toFixed(2)} USDT
                          </td>
                          <td className="py-3 px-2 font-mono whitespace-nowrap" style={{ color: 'var(--green-primary)' }}>
                            {pos.leverage}x
                          </td>
                          <td className="py-3 px-2 font-mono whitespace-nowrap">
                            <span
                              style={{
                                color: pos.unrealized_pnl >= 0 ? '#0ECB81' : '#F6465D',
                                fontWeight: 'bold',
                              }}
                            >
                              {pos.unrealized_pnl >= 0 ? '+' : ''}
                              {pos.unrealized_pnl.toFixed(2)} ({pos.unrealized_pnl_pct.toFixed(2)}%)
                            </span>
                          </td>
                          <td className="py-3 px-2 font-mono whitespace-nowrap" style={{ color: '#848E9C' }}>
                            {pos.liquidation_price.toFixed(4)}
                          </td>
                          <td className="py-3 px-2 text-right whitespace-nowrap">
                            <button
                              onClick={() => handleClosePosition(pos.symbol, pos.side as 'long' | 'short')}
                              disabled={closingPositions.has(`${pos.symbol}-${pos.side}`)}
                              className="px-2 py-1 rounded text-xs font-semibold transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
                              style={{
                                background: closingPositions.has(`${pos.symbol}-${pos.side}`)
                                  ? 'rgba(132, 142, 156, 0.2)'
                                  : 'rgba(246, 70, 93, 0.15)',
                                color: closingPositions.has(`${pos.symbol}-${pos.side}`)
                                  ? '#848E9C'
                                  : '#F6465D',
                                border: `1px solid ${
                                  closingPositions.has(`${pos.symbol}-${pos.side}`)
                                    ? 'rgba(132, 142, 156, 0.3)'
                                    : 'rgba(246, 70, 93, 0.3)'
                                }`,
                              }}
                              onMouseEnter={(e) => {
                                if (!closingPositions.has(`${pos.symbol}-${pos.side}`)) {
                                  e.currentTarget.style.background = 'rgba(246, 70, 93, 0.25)'
                                  e.currentTarget.style.borderColor = 'rgba(246, 70, 93, 0.5)'
                                }
                              }}
                              onMouseLeave={(e) => {
                                if (!closingPositions.has(`${pos.symbol}-${pos.side}`)) {
                                  e.currentTarget.style.background = 'rgba(246, 70, 93, 0.15)'
                                  e.currentTarget.style.borderColor = 'rgba(246, 70, 93, 0.3)'
                                }
                              }}
                            >
                              {closingPositions.has(`${pos.symbol}-${pos.side}`) ? (
                                <>
                                  <RefreshCw className="w-3 h-3 inline-block animate-spin mr-1" />
                                  {t('closing', language) || 'Closing...'}
                                </>
                              ) : (
                                <>
                                  <XCircle className="w-3 h-3 inline-block mr-1" />
                                  {t('close', language) || 'Close'}
                                </>
                              )}
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                {/* Mobile Card View */}
                <div className="lg:hidden space-y-4">
                  {positions.map((pos, i) => (
                    <div
                      key={i}
                      className="rounded-lg p-4 border"
                      style={{
                        background: 'var(--navy-dark)',
                        borderColor: 'var(--navy-light)',
                      }}
                    >
                      <div className="flex items-start justify-between mb-3">
                        <div>
                          <button
                            onClick={() => {
                              setSelectedChartSymbol(pos.symbol)
                              setChartUpdateKey(prev => prev + 1)
                            }}
                            className="text-lg font-bold font-mono hover:underline transition-all mb-1"
                            style={{ color: '#EAECEF' }}
                          >
                            {pos.symbol}
                          </button>
                          <div className="flex items-center gap-2">
                            <span
                              className="px-2 py-1 rounded text-xs font-bold"
                              style={
                                pos.side === 'long'
                                  ? {
                                      background: 'rgba(14, 203, 129, 0.1)',
                                      color: '#0ECB81',
                                    }
                                  : {
                                      background: 'rgba(246, 70, 93, 0.1)',
                                      color: '#F6465D',
                                    }
                              }
                            >
                              {t(pos.side === 'long' ? 'long' : 'short', language)}
                            </span>
                            <span className="text-xs" style={{ color: 'var(--green-primary)' }}>
                              {pos.leverage}x
                            </span>
                          </div>
                        </div>
                        <div className="text-right">
                          <div
                            className="text-lg font-bold font-mono"
                            style={{
                              color: pos.unrealized_pnl >= 0 ? '#0ECB81' : '#F6465D',
                            }}
                          >
                            {pos.unrealized_pnl >= 0 ? '+' : ''}
                            {pos.unrealized_pnl.toFixed(2)} USDT
                          </div>
                          <div className="text-xs" style={{ color: '#848E9C' }}>
                            {pos.unrealized_pnl_pct.toFixed(2)}%
                          </div>
                        </div>
                      </div>
                      <div className="grid grid-cols-2 gap-3 text-sm mb-3">
                        <div>
                          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
                            {t('entryPrice', language)}
                          </div>
                          <div className="font-mono font-semibold" style={{ color: '#EAECEF' }}>
                            {pos.entry_price.toFixed(4)}
                          </div>
                        </div>
                        <div>
                          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
                            {t('markPrice', language)}
                          </div>
                          <div className="font-mono font-semibold" style={{ color: '#EAECEF' }}>
                            {pos.mark_price.toFixed(4)}
                          </div>
                        </div>
                        <div>
                          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
                            {t('quantity', language)}
                          </div>
                          <div className="font-mono font-semibold" style={{ color: '#EAECEF' }}>
                            {pos.quantity.toFixed(4)}
                          </div>
                        </div>
                        <div>
                          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
                            {t('positionValue', language)}
                          </div>
                          <div className="font-mono font-semibold" style={{ color: '#EAECEF' }}>
                            {(pos.quantity * pos.mark_price).toFixed(2)} USDT
                          </div>
                        </div>
                        <div className="col-span-2">
                          <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
                            {t('liqPrice', language)}
                          </div>
                          <div className="font-mono font-semibold" style={{ color: '#848E9C' }}>
                            {pos.liquidation_price.toFixed(4)}
                          </div>
                        </div>
                      </div>
                      <button
                        onClick={() => handleClosePosition(pos.symbol, pos.side as 'long' | 'short')}
                        disabled={closingPositions.has(`${pos.symbol}-${pos.side}`)}
                        className="w-full px-4 py-2 rounded text-sm font-semibold transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
                        style={{
                          background: closingPositions.has(`${pos.symbol}-${pos.side}`)
                            ? 'rgba(132, 142, 156, 0.2)'
                            : 'rgba(246, 70, 93, 0.15)',
                          color: closingPositions.has(`${pos.symbol}-${pos.side}`)
                            ? '#848E9C'
                            : '#F6465D',
                          border: `1px solid ${
                            closingPositions.has(`${pos.symbol}-${pos.side}`)
                              ? 'rgba(132, 142, 156, 0.3)'
                              : 'rgba(246, 70, 93, 0.3)'
                          }`,
                        }}
                      >
                        {closingPositions.has(`${pos.symbol}-${pos.side}`) ? (
                          <>
                            <RefreshCw className="w-4 h-4 inline-block animate-spin mr-2" />
                            {t('closing', language) || 'Closing...'}
                          </>
                        ) : (
                          <>
                            <XCircle className="w-4 h-4 inline-block mr-2" />
                            {t('close', language) || 'Close Position'}
                          </>
                        )}
                      </button>
                    </div>
                  ))}
                </div>
              </>
            ) : (
              <div className="text-center py-16" style={{ color: '#848E9C' }}>
                <div className="mb-4 opacity-50 flex justify-center">
                  <PieChart className="w-16 h-16" />
                </div>
                <div className="text-lg font-semibold mb-2">
                  {t('noPositions', language)}
                </div>
                <div className="text-sm">
                  {t('noActivePositions', language)}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* 右侧：Recent Decisions */}
        <div
          className="binance-card p-6 animate-slide-in h-fit lg:sticky lg:top-24 lg:max-h-[calc(100vh-120px)]"
          style={{ animationDelay: '0.2s' }}
        >
          <div
            className="flex items-center justify-between mb-5 pb-4 border-b"
            style={{ borderColor: 'var(--navy-light)' }}
          >
            <div className="flex items-center gap-3">
              <div
                className="w-10 h-10 rounded-xl flex items-center justify-center"
                style={{
                  background: 'linear-gradient(135deg, #6366F1 0%, #8B5CF6 100%)',
                  boxShadow: '0 4px 14px rgba(99, 102, 241, 0.4)',
                }}
              >
                <Brain className="w-5 h-5" style={{ color: '#FFFFFF' }} />
              </div>
              <div>
                <h2 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
                  {t('recentDecisions', language)}
                </h2>
                {decisions && decisions.length > 0 && (
                  <div className="text-xs" style={{ color: '#848E9C' }}>
                    {t('lastCycles', language, { count: decisions.length })}
                  </div>
                )}
              </div>
            </div>

            {/* 显示数量选择器 */}
            <div className="flex items-center gap-2">
              <span className="text-xs" style={{ color: '#848E9C' }}>
                {language === 'zh' ? '显示' : 'Show'}:
              </span>
              <select
                value={decisionLimit}
                onChange={(e) => handleLimitChange(parseInt(e.target.value, 10))}
                className="rounded px-2 py-1 text-xs font-medium cursor-pointer transition-colors"
                style={{
                  background: 'var(--navy-dark)',
                  border: '1px solid var(--navy-light)',
                  color: '#EAECEF',
                }}
              >
                <option value={5}>5</option>
                <option value={10}>10</option>
                <option value={20}>20</option>
                <option value={50}>50</option>
              </select>
              <span className="text-xs" style={{ color: '#848E9C' }}>
                {language === 'zh' ? '条' : ''}
              </span>
            </div>
          </div>

          <div
            className="space-y-4 overflow-y-auto pr-2"
            style={{ maxHeight: 'calc(100vh - 280px)' }}
          >
            {decisions && decisions.length > 0 ? (
              decisions.map((decision, i) => (
                <DecisionCard key={i} decision={decision} language={language} />
              ))
            ) : (
              <div className="py-16 text-center">
                <div className="mb-4 opacity-30 flex justify-center">
                  <Brain className="w-16 h-16" />
                </div>
                <div
                  className="text-lg font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  {t('noDecisionsYet', language)}
                </div>
                <div className="text-sm" style={{ color: '#848E9C' }}>
                  {t('aiDecisionsWillAppear', language)}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* AI Learning & Performance Analysis */}
      <div className="mb-6 animate-slide-in" style={{ animationDelay: '0.3s' }}>
        <AILearning traderId={selectedTrader.trader_id} />
      </div>
    </div>
  )
}

// Stat Card Component
function StatCard({
  title,
  value,
  change,
  positive,
  subtitle,
}: {
  title: string
  value: string
  change?: number
  positive?: boolean
  subtitle?: string
}) {
  return (
    <div className="stat-card animate-fade-in">
      <div
        className="text-xs mb-2 mono uppercase tracking-wider"
        style={{ color: '#848E9C' }}
      >
        {title}
      </div>
      <div
        className="text-2xl font-bold mb-1 mono"
        style={{ color: '#EAECEF' }}
      >
        {value}
      </div>
      {change !== undefined && (
        <div className="flex items-center gap-1">
          <div
            className="text-sm mono font-bold"
            style={{ color: positive ? '#0ECB81' : '#F6465D' }}
          >
            {positive ? '▲' : '▼'} {positive ? '+' : ''}
            {change.toFixed(2)}%
          </div>
        </div>
      )}
      {subtitle && (
        <div className="text-xs mt-2 mono" style={{ color: '#848E9C' }}>
          {subtitle}
        </div>
      )}
    </div>
  )
}

// Helper function to translate AI error messages
function translateAIErrorMessage(message: string, language: 'en' | 'zh'): string {
  if (!message) return message

  // If message contains Chinese characters and language is English, try to translate
  if (language === 'en' && /[\u4e00-\u9fff]/.test(message)) {
    // Translate "AI拒绝: " prefix
    let translated = message.replace(/^AI拒绝:\s*/i, t('aiRejected', language))

    // Pattern: 信号价格X与当前市场价Y严重不符（相差Z%）
    const priceMismatchPattern = /信号价格([\d.]+)与当前市场价([\d.]+)严重不符（相差([\d.]+)%）/
    while (priceMismatchPattern.test(translated)) {
      const match = translated.match(priceMismatchPattern)
      if (match) {
        const signalPrice = match[1]
        const marketPrice = match[2]
        const diff = match[3]
        translated = translated.replace(
          priceMismatchPattern,
          t('signalPriceMismatch', language, {
            signalPrice,
            marketPrice,
            diff,
          })
        )
      } else {
        break
      }
    }

    // Pattern: 账户资金仅X USDT,无法满足最小交易要求
    const balancePattern = /账户资金仅([\d.]+)\s*USDT,无法满足最小交易要求/
    if (balancePattern.test(translated)) {
      const match = translated.match(balancePattern)
      if (match) {
        const balance = match[1]
        translated = translated.replace(
          balancePattern,
          t('insufficientBalance', language, { balance })
        )
      }
    }

    // Pattern: 当前技术面（...）也不支持...操作
    const techAnalysisPattern = /当前技术面（([^）]+)）也不支持([^。]+)操作/
    if (techAnalysisPattern.test(translated)) {
      const match = translated.match(techAnalysisPattern)
      if (match) {
        const details = match[1]
        const action = match[2]
        translated = translated.replace(
          techAnalysisPattern,
          t('technicalAnalysisNotSupport', language, { details, action })
        )
      }
    }

    // Clean up any remaining Chinese punctuation/conjunctions
    translated = translated.replace(/，/g, ', ').replace(/。/g, '.')

    return translated
  }

  return message
}

// Decision Card Component
function DecisionCard({
  decision,
  language,
}: {
  decision: DecisionRecord
  language: Language
}) {
  const [showInputPrompt, setShowInputPrompt] = useState(false)
  const [showCoT, setShowCoT] = useState(false)

  return (
    <div
      className="rounded p-5 transition-all duration-300 hover:translate-y-[-2px]"
      style={{
        border: '1px solid var(--navy-light)',
        background: 'var(--navy-dark)',
        boxShadow: '0 2px 8px rgba(0, 0, 0, 0.3)',
      }}
    >
      {/* Header */}
      <div className="flex items-start justify-between mb-3">
        <div>
          <div className="font-semibold" style={{ color: '#EAECEF' }}>
            {t('cycle', language)} #{decision.cycle_number}
          </div>
          <div className="text-xs" style={{ color: '#848E9C' }}>
            {new Date(decision.timestamp).toLocaleString()}
          </div>
        </div>
        <div
          className="px-3 py-1 rounded text-xs font-bold"
          style={
            decision.success
              ? { background: 'rgba(14, 203, 129, 0.1)', color: '#0ECB81' }
              : { background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }
          }
        >
          {t(decision.success ? 'success' : 'failed', language)}
        </div>
      </div>

      {/* Input Prompt - Collapsible */}
      {decision.input_prompt && (
        <div className="mb-3">
          <button
            onClick={() => setShowInputPrompt(!showInputPrompt)}
            className="flex items-center gap-2 text-sm transition-colors"
            style={{ color: '#60a5fa' }}
          >
            <span className="font-semibold flex items-center gap-2">
              <Inbox className="w-4 h-4" /> {t('inputPrompt', language)}
            </span>
            <span className="text-xs">
              {showInputPrompt
                ? t('collapse', language)
                : t('expand', language)}
            </span>
          </button>
          {showInputPrompt && (
            <div
              className="mt-2 rounded p-4 text-sm font-mono whitespace-pre-wrap max-h-96 overflow-y-auto"
              style={{
                background: 'var(--navy-primary)',
                border: '1px solid var(--navy-light)',
                color: '#EAECEF',
              }}
            >
              {decision.input_prompt}
            </div>
          )}
        </div>
      )}

      {/* AI Chain of Thought - Collapsible */}
      {decision.cot_trace && (
        <div className="mb-3">
          <button
            onClick={() => setShowCoT(!showCoT)}
            className="flex items-center gap-2 text-sm transition-colors"
            style={{ color: 'var(--green-primary)' }}
          >
            <span className="font-semibold flex items-center gap-2">
              <Send className="w-4 h-4" />{' '}
              {stripLeadingIcons(t('aiThinking', language))}
            </span>
            <span className="text-xs">
              {showCoT ? t('collapse', language) : t('expand', language)}
            </span>
          </button>
          {showCoT && (
            <div
              className="mt-2 rounded p-4 text-sm font-mono whitespace-pre-wrap max-h-96 overflow-y-auto"
              style={{
                background: 'var(--navy-primary)',
                border: '1px solid var(--navy-light)',
                color: '#EAECEF',
              }}
            >
              {decision.cot_trace}
            </div>
          )}
        </div>
      )}

      {/* Decisions Actions */}
      {decision.decisions && decision.decisions.length > 0 && (
        <div className="space-y-2 mb-3">
          {decision.decisions.map((action, j) => (
            <div
              key={j}
              className="flex items-center gap-2 text-sm rounded px-3 py-2"
              style={{ background: 'var(--navy-primary)' }}
            >
              <span
                className="font-mono font-bold"
                style={{ color: '#EAECEF' }}
              >
                {action.symbol}
              </span>
              <span
                className="px-2 py-0.5 rounded text-xs font-bold"
                style={
                  action.action.includes('open')
                    ? {
                        background: 'rgba(96, 165, 250, 0.1)',
                        color: '#60a5fa',
                      }
                    : {
                        background: 'rgba(0, 255, 127, 0.1)',
                        color: 'var(--green-primary)',
                      }
                }
              >
                {action.action}
              </span>
              {action.leverage > 0 && (
                <span style={{ color: 'var(--green-primary)' }}>{action.leverage}x</span>
              )}
              {action.price > 0 && (
                <span
                  className="font-mono text-xs"
                  style={{ color: '#848E9C' }}
                >
                  @{action.price.toFixed(4)}
                </span>
              )}
              <span style={{ color: action.success ? '#0ECB81' : '#F6465D' }}>
                {action.success ? (
                  <Check className="w-3 h-3 inline" />
                ) : (
                  <X className="w-3 h-3 inline" />
                )}
              </span>
              {action.error && (
                <span className="text-xs ml-2" style={{ color: '#F6465D' }}>
                  {action.error}
                </span>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Account State Summary */}
      {decision.account_state && (
        <div
          className="flex gap-4 text-xs mb-3 rounded px-3 py-2"
          style={{ background: 'var(--navy-primary)', color: '#848E9C' }}
        >
          <span>
            {t('totalEquity', language)}: {decision.account_state.total_balance.toFixed(2)} USDT
          </span>
          <span>
            {t('availableBalance', language)}: {decision.account_state.available_balance.toFixed(2)} USDT
          </span>
          <span>
            {t('margin', language)}: {decision.account_state.margin_used_pct.toFixed(1)}%
          </span>
          <span>{t('positions', language)}: {decision.account_state.position_count}</span>
          <span
            style={{
              color:
                decision.candidate_coins &&
                decision.candidate_coins.length === 0
                  ? '#F6465D'
                  : '#848E9C',
            }}
          >
            {t('candidateCoins', language)}:{' '}
            {decision.candidate_coins?.length || 0}
          </span>
        </div>
      )}

      {/* Candidate Coins Warning */}
      {decision.candidate_coins && decision.candidate_coins.length === 0 && (
        <div
          className="text-sm rounded px-4 py-3 mb-3 flex items-start gap-3"
          style={{
            background: 'rgba(246, 70, 93, 0.1)',
            border: '1px solid rgba(246, 70, 93, 0.3)',
            color: '#F6465D',
          }}
        >
          <AlertTriangle size={16} className="flex-shrink-0 mt-0.5" />
          <div className="flex-1">
            <div className="font-semibold mb-1">
              {t('candidateCoinsZeroWarning', language)}
            </div>
            <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
              <div>{t('possibleReasons', language)}</div>
              <ul className="list-disc list-inside space-y-0.5 ml-2">
                <li>{t('coinPoolApiNotConfigured', language)}</li>
                <li>{t('apiConnectionTimeout', language)}</li>
                <li>{t('noCustomCoinsAndApiFailed', language)}</li>
              </ul>
              <div className="mt-2">
                <strong>{t('solutions', language)}</strong>
              </div>
              <ul className="list-disc list-inside space-y-0.5 ml-2">
                <li>{t('setCustomCoinsInConfig', language)}</li>
                <li>{t('orConfigureCorrectApiUrl', language)}</li>
                <li>{t('orDisableCoinPoolOptions', language)}</li>
              </ul>
            </div>
          </div>
        </div>
      )}

      {/* Execution Logs */}
      {decision.execution_log && decision.execution_log.length > 0 && (
        <div className="space-y-1">
          {decision.execution_log.map((log, k) => (
            <div
              key={k}
              className="text-xs font-mono"
              style={{
                color:
                  log.includes('✓') || log.includes('成功')
                    ? '#0ECB81'
                    : '#F6465D',
              }}
            >
              {log}
            </div>
          ))}
        </div>
      )}

      {/* Error Message */}
      {decision.error_message && (
        <div
          className="text-sm rounded px-3 py-2 mt-3 flex items-center gap-2"
          style={{ color: '#F6465D', background: 'rgba(246, 70, 93, 0.1)' }}
        >
          <XCircle className="w-4 h-4" /> {translateAIErrorMessage(decision.error_message, language)}
        </div>
      )}
    </div>
  )
}
