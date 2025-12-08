import { useMemo } from 'react'
import {
  LineChart,
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
import { BarChart3 } from 'lucide-react'

interface ComparisonChartProps {
  traders: CompetitionTraderData[]
}

export function ComparisonChart({ traders }: ComparisonChartProps) {
  const { language } = useLanguage()
  // 获取所有trader的历史数据 - 使用单个useSWR并发请求所有trader数据
  // 生成唯一的key，当traders变化时会触发重新请求
  const tradersKey = traders
    .map((t) => t.trader_id)
    .sort()
    .join(',')

  const { data: allTraderHistories, isLoading } = useSWR(
    traders.length > 0 ? `all-equity-histories-${tradersKey}` : null,
    async () => {
      // 使用批量API一次性获取所有trader的历史数据
      const traderIds = traders.map((trader) => trader.trader_id)
      const batchData = await api.getEquityHistoryBatch(traderIds)

      // 转换为原格式，保持与原有代码兼容
      return traders.map((trader) => {
        return batchData.histories[trader.trader_id] || []
      })
    },
    {
      refreshInterval: 30000, // 30秒刷新（对比图表数据更新频率较低）
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  // 将数据转换为与原格式兼容的结构
  const traderHistories = useMemo(() => {
    if (!allTraderHistories) {
      return traders.map(() => ({ data: undefined }))
    }
    return allTraderHistories.map((data) => ({ data }))
  }, [allTraderHistories, traders.length])

  // 使用useMemo自动处理数据合并，直接使用data对象作为依赖
  const combinedData = useMemo(() => {
    // 等待所有数据加载完成
    const allLoaded = traderHistories.every((h) => h.data)
    if (!allLoaded) return []

    console.log(`[${new Date().toISOString()}] Recalculating chart data...`)

    // 新方案：按时间戳分组，不再依赖 cycle_number（因为后端会重置）
    // 收集所有时间戳
    const timestampMap = new Map<
      string,
      {
        timestamp: string
        time: string
        traders: Map<string, { pnl_pct: number; equity: number }>
      }
    >()

    traderHistories.forEach((history, index) => {
      const trader = traders[index]
      if (!history.data) return

      console.log(
        `Trader ${trader.trader_id}: ${history.data.length} data points`
      )

      history.data.forEach((point: any) => {
        const ts = point.timestamp

        if (!timestampMap.has(ts)) {
          const time = new Date(ts).toLocaleTimeString('zh-CN', {
            hour: '2-digit',
            minute: '2-digit',
          })
          timestampMap.set(ts, {
            timestamp: ts,
            time,
            traders: new Map(),
          })
        }

        // 直接使用后端返回的盈亏百分比，不要在前端重新计算
        timestampMap.get(ts)!.traders.set(trader.trader_id, {
          pnl_pct: point.total_pnl_pct || 0,
          equity: point.total_equity,
        })
      })
    })

    // 按时间戳排序，转换为数组
    const combined = Array.from(timestampMap.entries())
      .sort(([tsA], [tsB]) => new Date(tsA).getTime() - new Date(tsB).getTime())
      .map(([ts, data], index) => {
        const entry: any = {
          index: index + 1, // 使用序号代替cycle
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

  if (isLoading) {
    return (
      <div className="text-center py-16" style={{ color: 'var(--text-gray-light)' }}>
        <div className="spinner mx-auto mb-4"></div>
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

  // 限制显示数据点
  const MAX_DISPLAY_POINTS = 2000
  const displayData =
    combinedData.length > MAX_DISPLAY_POINTS
      ? combinedData.slice(-MAX_DISPLAY_POINTS)
      : combinedData

  // 计算Y轴范围
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
    const padding = Math.max(range * 0.2, 1) // 至少留1%余量

    return [Math.floor(minVal - padding), Math.ceil(maxVal + padding)]
  }

  // 使用统一的颜色分配逻辑（与Leaderboard保持一致）
  const traderColor = (traderId: string) => getTraderColor(traders, traderId)

  // 自定义Tooltip - Enhanced Binance Style
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
            {data.time} • #{data.index}
          </div>
          <div className="space-y-2">
            {traders.map((trader) => {
              const pnlPct = data[`${trader.trader_id}_pnl_pct`]
              const equity = data[`${trader.trader_id}_equity`]
              if (pnlPct === undefined) return null

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
                    <div
                      className="text-sm mono font-bold"
                      style={{ color: pnlPct >= 0 ? 'var(--green-primary)' : 'var(--error)' }}
                    >
                      {pnlPct >= 0 ? '+' : ''}
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

  // 计算当前差距
  const currentGap =
    displayData.length > 0
      ? (() => {
          const lastPoint = displayData[displayData.length - 1]
          const values = traders.map(
            (t) => lastPoint[`${t.trader_id}_pnl_pct`] || 0
          )
          return Math.abs(values[0] - values[1])
        })()
      : 0

  return (
    <div className="space-y-4">
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
          <LineChart
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

            <Legend
              wrapperStyle={{ paddingTop: '20px', paddingBottom: '10px' }}
              iconType="line"
              iconSize={12}
              formatter={(value, entry: any) => {
                const traderId = traders.find(
                  (t) => value === t.trader_name
                )?.trader_id
                const trader = traders.find((t) => t.trader_id === traderId)
                return (
                  <span
                    className="text-sm"
                    style={{
                      color: entry.color,
                      fontWeight: 600,
                    }}
                  >
                    {trader?.trader_name} <span style={{ opacity: 0.6 }}>({trader?.ai_model.toUpperCase()})</span>
                  </span>
                )
              }}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>

      {/* Enhanced Stats Grid */}
      <div
        className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4"
      >
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
            {t('comparisonMode', language)}
          </div>
          <div
            className="text-base md:text-lg font-bold"
            style={{ color: 'var(--text-white)' }}
          >
            PnL %
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
            {t('currentGap', language)}
          </div>
          <div
            className="text-base md:text-lg font-bold mono"
            style={{ color: currentGap > 1 ? 'var(--green-primary)' : 'var(--text-white)' }}
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
            {t('displayRange', language)}
          </div>
          <div
            className="text-base md:text-lg font-bold mono"
            style={{ color: 'var(--text-white)' }}
          >
            {combinedData.length > MAX_DISPLAY_POINTS
              ? `${t('recent', language)} ${MAX_DISPLAY_POINTS.toLocaleString()}`
              : t('allData', language)}
          </div>
        </div>
      </div>
    </div>
  )
}
