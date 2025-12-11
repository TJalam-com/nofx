import { useMemo } from 'react'
import {
  ComposedChart,
  Area,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  ReferenceLine,
  Legend,
} from 'recharts'
import useSWR from 'swr'
import { api } from '../lib/api'
import type { CompetitionTraderData } from '../types'
import { getTraderColor } from '../utils/traderColors'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { BarChart3, TrendingUp, TrendingDown } from 'lucide-react'

interface ComparisonChartProps {
  traders: CompetitionTraderData[]
}

export function ComparisonChart({ traders }: ComparisonChartProps) {
  const { language } = useLanguage()
  // Get all trader history data - use single useSWR to concurrently request all trader data
  // Generate unique key that triggers re-request when traders change
  const tradersKey = traders
    .map((t) => t.trader_id)
    .sort()
    .join(',')

  const { data: allTraderHistories, isLoading } = useSWR(
    traders.length > 0 ? `all-equity-histories-${tradersKey}` : null,
    async () => {
      // Use batch API to get all trader history data at once
      const traderIds = traders.map((trader) => trader.trader_id)
      const batchData = await api.getEquityHistoryBatch(traderIds)

      // Convert to original format, maintain compatibility with existing code
      return traders.map((trader) => {
        return batchData.histories[trader.trader_id] || []
      })
    },
    {
      refreshInterval: 30000, // 30 second refresh (comparison chart data update frequency is lower)
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  // Convert data to structure compatible with original format
  const traderHistories = useMemo(() => {
    if (!allTraderHistories) {
      return traders.map(() => ({ data: undefined }))
    }
    return allTraderHistories.map((data) => ({ data }))
  }, [allTraderHistories, traders.length])

  // Use useMemo to automatically process data merging, directly use data object as dependency
  const combinedData = useMemo(() => {
    // Wait for all data to load
    const allLoaded = traderHistories.every((h) => h.data)
    if (!allLoaded) return []

    console.log(`[${new Date().toISOString()}] Recalculating chart data...`)

    // New approach: group by timestamp, no longer rely on cycle_number (because backend resets it)
    // Collect all timestamps
    const timestampMap = new Map<
      string,
      {
        timestamp: string
        date: string
        time: string
        traders: Map<string, { pnl_pct: number; equity: number }>
      }
    >()

    // Calculate initial balance per trader (from first data point) for fallback calculation
    const traderInitialBalances = new Map<string, number>()
    traderHistories.forEach((history, index) => {
      const trader = traders[index]
      if (!history.data || history.data.length === 0) return

      // Get initial balance from first data point (assuming it represents 0% PnL)
      const firstPoint = history.data[0]
      if (firstPoint && firstPoint.total_equity != null && firstPoint.total_equity > 0) {
        traderInitialBalances.set(trader.trader_id, firstPoint.total_equity)
      }
    })

    traderHistories.forEach((history, index) => {
      const trader = traders[index]
      if (!history.data) return

      console.log(
        `Trader ${trader.trader_id}: ${history.data.length} data points`
      )

      history.data.forEach((point: any) => {
        const ts = point.timestamp
        const dateObj = new Date(ts)

        if (!timestampMap.has(ts)) {
          const date = dateObj.toLocaleDateString('en-US', {
            month: 'short',
            day: 'numeric',
          })
          const time = dateObj.toLocaleTimeString('en-US', {
            hour: '2-digit',
            minute: '2-digit',
          })
          timestampMap.set(ts, {
            timestamp: ts,
            date,
            time,
            traders: new Map(),
          })
        }

        // Use backend returned PnL percentage if available and valid, otherwise calculate from equity
        let pnlPct = 0
        if (
          point.total_pnl_pct != null &&
          !isNaN(point.total_pnl_pct) &&
          isFinite(point.total_pnl_pct)
        ) {
          // Backend provided valid total_pnl_pct, use it
          pnlPct = point.total_pnl_pct
        } else {
          // Backend didn't provide total_pnl_pct, calculate from equity
          const initialBalance = traderInitialBalances.get(trader.trader_id)
          if (
            initialBalance != null &&
            initialBalance > 0 &&
            point.total_equity != null &&
            !isNaN(point.total_equity)
          ) {
            const pnl = point.total_equity - initialBalance
            pnlPct = (pnl / initialBalance) * 100
          }
        }

        timestampMap.get(ts)!.traders.set(trader.trader_id, {
          pnl_pct: pnlPct,
          equity: point.total_equity || 0,
        })
      })
    })

    // Sort by timestamp, convert to array
    const combined = Array.from(timestampMap.entries())
      .sort(([tsA], [tsB]) => new Date(tsA).getTime() - new Date(tsB).getTime())
      .map(([ts, data], index) => {
        const entry: any = {
          index: index + 1, // Use index instead of cycle
          date: data.date,
          time: data.time,
          timestamp: ts,
        }

        traders.forEach((trader) => {
          const traderData = data.traders.get(trader.trader_id)
          if (traderData) {
            entry[`${trader.trader_id}_pnl_pct`] = traderData.pnl_pct
            entry[`${trader.trader_id}_equity`] = traderData.equity
          }
        })

        return entry
      })

    if (combined.length > 0) {
      const lastPoint = combined[combined.length - 1]
      console.log(
        `Chart: ${combined.length} data points, last time: ${lastPoint.time}, timestamp: ${lastPoint.timestamp}`
      )
    }

    return combined
  }, [allTraderHistories, traders])

  // Calculate current PnL for each trader (for mini stats bar)
  const currentPnLData = useMemo(() => {
    if (combinedData.length === 0) return []
    const lastPoint = combinedData[combinedData.length - 1]
    return traders
      .map((trader) => {
        const pnlPct = lastPoint[`${trader.trader_id}_pnl_pct`] || 0
        return {
          trader_id: trader.trader_id,
          trader_name: trader.trader_name,
          pnl_pct: pnlPct,
        }
      })
      .sort((a, b) => b.pnl_pct - a.pnl_pct)
  }, [combinedData, traders])

  // Calculate leader stats
  const leaderStats = useMemo(() => {
    if (currentPnLData.length === 0) {
      return { leader: null, leadPnl: 0, gap: 0 }
    }
    const leader = currentPnLData[0]
    const second = currentPnLData[1]
    const gap = second ? leader.pnl_pct - second.pnl_pct : 0
    return {
      leader: leader.trader_name,
      leadPnl: leader.pnl_pct,
      gap: gap,
    }
  }, [currentPnLData])

  if (isLoading) {
    return (
      <div className="text-center py-16" style={{ color: 'var(--text-gray-light)' }}>
        <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-green-500 mb-4"></div>
        <div className="text-sm font-semibold">Loading comparison data...</div>
      </div>
    )
  }

  if (combinedData.length === 0) {
    return (
      <div className="text-center py-16" style={{ color: 'var(--text-gray-light)' }}>
        <BarChart3 className="w-12 h-12 mx-auto mb-4 opacity-60" style={{ color: 'var(--text-gray-light)' }} />
        <div className="text-lg font-semibold mb-2" style={{ color: 'var(--text-white)' }}>
          {t('noHistoricalData', language)}
        </div>
        <div className="text-sm">{t('dataWillAppear', language)}</div>
      </div>
    )
  }

  // Limit displayed data points
  const MAX_DISPLAY_POINTS = 2000
  const displayData =
    combinedData.length > MAX_DISPLAY_POINTS
      ? combinedData.slice(-MAX_DISPLAY_POINTS)
      : combinedData

  // Calculate Y-axis range ensuring zero is visible
  const calculateYDomain = () => {
    const allValues: number[] = []
    displayData.forEach((point) => {
      traders.forEach((trader) => {
        const value = point[`${trader.trader_id}_pnl_pct`]
        if (value !== undefined) {
          allValues.push(value)
        }
      })
    })

    if (allValues.length === 0) return [-5, 5]

    const minVal = Math.min(...allValues)
    const maxVal = Math.max(...allValues)
    const range = Math.max(Math.abs(maxVal), Math.abs(minVal))
    const padding = Math.max(range * 0.2, 1) // At least 1% margin

    // Ensure zero is visible
    const domainMin = Math.min(0, Math.floor(minVal - padding))
    const domainMax = Math.max(0, Math.ceil(maxVal + padding))

    return [domainMin, domainMax]
  }

  // Use unified color assignment logic (consistent with Leaderboard)
  const traderColor = (traderId: string) => getTraderColor(traders, traderId)

  // Custom Legend - Filter out Area entries to show only Line entries with trader names
  // Fix chart legend showing 0% by finding each trader's last available data point
  const CustomLegend = ({ payload }: any) => {
    if (!payload) return null

    // Filter out Area entries (they don't have a value/name property)
    // Only show Line entries that have proper trader names
    const filteredPayload = payload.filter((entry: any) => {
      // Area entries don't have a name prop, so they won't have entry.value
      // Line entries have name={trader.trader_name}, so they will have entry.value
      return entry.value != null
    })

    return (
      <ul
        className="recharts-default-legend"
        style={{
          paddingTop: '20px',
          paddingBottom: '10px',
          textAlign: 'center',
        }}
      >
        {filteredPayload.map((entry: any) => {
          const trader = traders.find((t) => t.trader_name === entry.value)
          
          // Find each trader's last available data point to avoid showing 0%
          let lastPnLPct: number | null = null
          if (combinedData.length > 0 && trader) {
            // Search backwards through combinedData to find last non-null data point
            for (let i = combinedData.length - 1; i >= 0; i--) {
              const dataPoint = combinedData[i]
              const pnlPct = dataPoint[`${trader.trader_id}_pnl_pct`]
              if (pnlPct !== undefined && pnlPct !== null && !isNaN(pnlPct)) {
                lastPnLPct = pnlPct
                break
              }
            }
          }

          return (
            <li
              key={entry.value}
              className="recharts-legend-item"
              style={{
                display: 'inline-block',
                marginRight: '20px',
                marginLeft: '20px',
              }}
            >
              <svg
                className="recharts-surface"
                width="14"
                height="14"
                viewBox="0 0 14 14"
                style={{
                  display: 'inline-block',
                  verticalAlign: 'middle',
                  marginRight: '4px',
                }}
              >
                <line
                  x1="0"
                  y1="7"
                  x2="14"
                  y2="7"
                  stroke={entry.color}
                  strokeWidth="2.5"
                />
              </svg>
              <span
                className="recharts-legend-item-text"
                style={{
                  color: entry.color,
                  fontWeight: 600,
                  fontSize: '14px',
                }}
              >
                {trader?.trader_name}{' '}
                <span style={{ opacity: 0.6 }}>
                  ({trader?.ai_model.toUpperCase()})
                </span>
                {lastPnLPct !== null && (
                  <span
                    style={{
                      marginLeft: '8px',
                      opacity: 0.8,
                      fontWeight: 500,
                      color: lastPnLPct >= 0 ? 'var(--green-primary)' : 'var(--error)',
                    }}
                  >
                    {lastPnLPct >= 0 ? '+' : ''}
                    {lastPnLPct.toFixed(2)}%
                  </span>
                )}
              </span>
            </li>
          )
        })}
      </ul>
    )
  }

  // Custom Tooltip - Enhanced with date, time, and trend icons
  const CustomTooltip = ({ active, payload }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload
      return (
        <div
          className="rounded-lg p-4 shadow-2xl border backdrop-blur-sm"
          style={{
            background: 'var(--bg-darker)',
            border: '1px solid var(--bg-panel)',
            boxShadow: '0 8px 32px rgba(0, 0, 0, 0.4)',
          }}
        >
          <div className="text-xs mb-3 font-semibold" style={{ color: 'var(--text-gray-light)' }}>
            {data.date} • {data.time} • #{data.index}
          </div>
          <div className="space-y-2">
            {traders.map((trader) => {
              const pnlPct = data[`${trader.trader_id}_pnl_pct`]
              const equity = data[`${trader.trader_id}_equity`]
              if (pnlPct === undefined) return null

              const isPositive = pnlPct >= 0
              const TrendIcon = isPositive ? TrendingUp : TrendingDown

              return (
                <div
                  key={trader.trader_id}
                  className="flex items-center justify-between gap-4 p-2 rounded-md"
                  style={{ background: 'rgba(0, 255, 127, 0.03)' }}
                >
                  <div className="flex items-center gap-2 flex-1 min-w-0">
                    <div
                      className="w-2 h-2 rounded-full flex-shrink-0"
                      style={{ background: traderColor(trader.trader_id) }}
                    />
                    <div
                      className="text-xs font-semibold truncate"
                      style={{ color: traderColor(trader.trader_id) }}
                    >
                      {trader.trader_name}
                    </div>
                  </div>
                  <div className="flex items-center gap-3 flex-shrink-0">
                    <TrendIcon
                      className="w-4 h-4"
                      style={{
                        color: isPositive ? 'var(--green-primary)' : 'var(--error)',
                      }}
                    />
                    <div
                      className="text-sm mono font-bold"
                      style={{
                        color: isPositive ? 'var(--green-primary)' : 'var(--error)',
                      }}
                    >
                      {isPositive ? '+' : ''}
                      {pnlPct.toFixed(2)}%
                    </div>
                    <div
                      className="text-xs mono"
                      style={{ color: 'var(--text-gray-light)' }}
                    >
                      {equity?.toFixed(2)}
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )
    }
    return null
  }

  // Calculate current gap
  const currentGap =
    displayData.length > 0
      ? (() => {
          const lastPoint = displayData[displayData.length - 1]
          const values = traders
            .map((t) => lastPoint[`${t.trader_id}_pnl_pct`] || 0)
            .sort((a, b) => b - a)
          return values.length >= 2 ? values[0] - values[1] : 0
        })()
      : 0

  return (
    <div className="space-y-4">
      {/* Mini Stats Bar - Show all traders with current PnL */}
      {currentPnLData.length > 0 && (
        <div
          className="flex flex-wrap gap-2 p-3 rounded-lg"
          style={{
            background: 'var(--bg-dark)',
            border: '1px solid var(--bg-panel)',
          }}
        >
          {currentPnLData.map((trader) => {
            const isPositive = trader.pnl_pct >= 0
            return (
              <div
                key={trader.trader_id}
                className="flex items-center gap-2 px-3 py-1.5 rounded-md"
                style={{
                  background: 'rgba(0, 255, 127, 0.05)',
                  border: `1px solid ${traderColor(trader.trader_id)}40`,
                }}
              >
                <div
                  className="w-2 h-2 rounded-full"
                  style={{ background: traderColor(trader.trader_id) }}
                />
                <span
                  className="text-xs font-semibold"
                  style={{ color: traderColor(trader.trader_id) }}
                >
                  {trader.trader_name}:
                </span>
                <span
                  className="text-xs font-bold mono"
                  style={{
                    color: isPositive ? 'var(--green-primary)' : 'var(--error)',
                  }}
                >
                  {isPositive ? '+' : ''}
                  {trader.pnl_pct.toFixed(2)}%
                </span>
              </div>
            )
          })}
        </div>
      )}

      {/* Chart */}
      <div
        className="relative rounded-lg overflow-hidden"
        style={{
          background: 'var(--bg-dark)',
          border: '1px solid var(--bg-panel)',
        }}
      >
        {/* Enhanced AI Trading Watermark */}
        <div
          className="absolute top-4 right-4 z-10 pointer-events-none"
          style={{
            fontSize: '20px',
            fontWeight: 'bold',
            color: 'rgba(0, 255, 127, 0.08)',
            fontFamily: 'monospace',
            letterSpacing: '2px',
          }}
        >
          AI Trading
        </div>
        <ResponsiveContainer width="100%" height={520}>
          <ComposedChart
            data={displayData}
            margin={{ top: 20, right: 30, left: 20, bottom: 50 }}
          >
            <defs>
              {traders.map((trader) => (
                <linearGradient
                  key={`gradient-${trader.trader_id}`}
                  id={`gradient-${trader.trader_id}`}
                  x1="0"
                  y1="0"
                  x2="0"
                  y2="1"
                >
                  <stop
                    offset="5%"
                    stopColor={traderColor(trader.trader_id)}
                    stopOpacity={0.4}
                  />
                  <stop
                    offset="95%"
                    stopColor={traderColor(trader.trader_id)}
                    stopOpacity={0.05}
                  />
                </linearGradient>
              ))}
            </defs>

            <CartesianGrid
              strokeDasharray="3 3"
              stroke="var(--bg-panel)"
              opacity={0.5}
            />

            <XAxis
              dataKey="time"
              stroke="var(--text-gray-light)"
              tick={{ fill: 'var(--text-gray-light)', fontSize: 11 }}
              tickLine={{ stroke: 'var(--bg-panel)' }}
              interval={Math.floor(displayData.length / 12)}
              angle={-15}
              textAnchor="end"
              height={60}
            />

            <YAxis
              stroke="var(--text-gray-light)"
              tick={{ fill: 'var(--text-gray-light)', fontSize: 12 }}
              tickLine={{ stroke: 'var(--bg-panel)' }}
              domain={calculateYDomain()}
              tickFormatter={(value) => `${value.toFixed(1)}%`}
              width={60}
            />

            <Tooltip content={<CustomTooltip />} />

            <ReferenceLine
              y={0}
              stroke="var(--text-gray-light)"
              strokeDasharray="5 5"
              strokeWidth={1.5}
              strokeOpacity={0.3}
              label={{
                value: 'Break Even',
                fill: 'var(--text-gray-light)',
                fontSize: 11,
                position: 'right',
                opacity: 0.6,
              }}
            />

            {traders.map((trader) => (
              <Area
                key={`area-${trader.trader_id}`}
                type="monotone"
                dataKey={`${trader.trader_id}_pnl_pct`}
                fill={`url(#gradient-${trader.trader_id})`}
                stroke="none"
                isAnimationActive={true}
              />
            ))}

            {traders.map((trader) => (
              <Line
                key={trader.trader_id}
                type="monotone"
                dataKey={`${trader.trader_id}_pnl_pct`}
                stroke={traderColor(trader.trader_id)}
                strokeWidth={2.5}
                dot={false}
                activeDot={{
                  r: 5,
                  fill: traderColor(trader.trader_id),
                  stroke: 'var(--bg-dark)',
                  strokeWidth: 2,
                }}
                name={trader.trader_name}
                connectNulls
              />
            ))}

            <Legend content={<CustomLegend />} />
          </ComposedChart>
        </ResponsiveContainer>
      </div>

      {/* Bottom Stats Grid: Leader, Lead PnL, Gap, Data Points */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4">
        <div
          className="p-3 md:p-4 rounded-lg transition-all duration-300 hover:scale-[1.02] border"
          style={{
            background: 'var(--bg-dark)',
            border: '1px solid var(--bg-panel)',
          }}
        >
          <div
            className="text-xs mb-2 uppercase tracking-wider font-semibold"
            style={{ color: 'var(--text-gray-light)' }}
          >
            Leader
          </div>
          <div
            className="text-base md:text-lg font-bold truncate"
            style={{ color: 'var(--text-white)' }}
          >
            {leaderStats.leader || 'N/A'}
          </div>
        </div>

        <div
          className="p-3 md:p-4 rounded-lg transition-all duration-300 hover:scale-[1.02] border"
          style={{
            background: 'var(--bg-dark)',
            border: '1px solid var(--bg-panel)',
          }}
        >
          <div
            className="text-xs mb-2 uppercase tracking-wider font-semibold"
            style={{ color: 'var(--text-gray-light)' }}
          >
            Lead PnL
          </div>
          <div
            className="text-base md:text-lg font-bold mono"
            style={{
              color:
                leaderStats.leadPnl >= 0
                  ? 'var(--green-primary)'
                  : 'var(--error)',
            }}
          >
            {leaderStats.leadPnl >= 0 ? '+' : ''}
            {leaderStats.leadPnl.toFixed(2)}%
          </div>
        </div>

        <div
          className="p-3 md:p-4 rounded-lg transition-all duration-300 hover:scale-[1.02] border"
          style={{
            background: 'var(--bg-dark)',
            border: '1px solid var(--bg-panel)',
          }}
        >
          <div
            className="text-xs mb-2 uppercase tracking-wider font-semibold"
            style={{ color: 'var(--text-gray-light)' }}
          >
            Gap
          </div>
          <div
            className="text-base md:text-lg font-bold mono"
            style={{
              color: currentGap > 1 ? 'var(--green-primary)' : 'var(--text-white)',
            }}
          >
            {currentGap.toFixed(2)}%
          </div>
        </div>

        <div
          className="p-3 md:p-4 rounded-lg transition-all duration-300 hover:scale-[1.02] border"
          style={{
            background: 'var(--bg-dark)',
            border: '1px solid var(--bg-panel)',
          }}
        >
          <div
            className="text-xs mb-2 uppercase tracking-wider font-semibold"
            style={{ color: 'var(--text-gray-light)' }}
          >
            {t('dataPoints', language)}
          </div>
          <div
            className="text-base md:text-lg font-bold mono"
            style={{ color: 'var(--text-white)' }}
          >
            {combinedData.length.toLocaleString()}
          </div>
        </div>
      </div>
    </div>
  )
}
