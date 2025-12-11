import { useState, useEffect } from 'react'
import { EquityChart } from './EquityChart'
import { TradingViewChart } from './TradingViewChart'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { BarChart3, CandlestickChart } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'

interface ChartTabsProps {
  traderId: string
  selectedSymbol?: string // Symbol selected from external source
  updateKey?: number // Key to force update
  exchangeId?: string // Exchange ID
}

type ChartTab = 'equity' | 'kline'

export function ChartTabs({ traderId, selectedSymbol, updateKey, exchangeId }: ChartTabsProps) {
  const { language } = useLanguage()
  const [activeTab, setActiveTab] = useState<ChartTab>('equity')
  const [chartSymbol, setChartSymbol] = useState<string>('BTCUSDT')

  // When symbol is selected externally, automatically switch to K-line chart
  useEffect(() => {
    if (selectedSymbol) {
      setChartSymbol(selectedSymbol)
      setActiveTab('kline')
    }
  }, [selectedSymbol, updateKey])

  return (
    <div className="binance-card">
      {/* Tab Headers */}
      <div
        className="flex items-center gap-2 p-3"
        style={{
          borderBottom: '1px solid var(--panel-border)',
          background: 'var(--navy-primary)',
        }}
      >
        <button
          onClick={() => {
            setActiveTab('equity')
          }}
          className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold transition-all ${activeTab === 'equity'
            ? 'bg-green-500/10 text-green-500 border border-green-500/30 shadow-[0_0_10px_rgba(0,255,127,0.15)]'
            : 'text-gray-400 hover:text-white hover:bg-white/5 border border-transparent'
            }`}
        >
          <BarChart3 className="w-4 h-4" />
          {t('accountEquityCurve', language)}
        </button>

        <button
          onClick={() => {
            setActiveTab('kline')
          }}
          className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold transition-all ${activeTab === 'kline'
            ? 'bg-green-500/10 text-green-500 border border-green-500/30 shadow-[0_0_10px_rgba(0,255,127,0.15)]'
            : 'text-gray-400 hover:text-white hover:bg-white/5 border border-transparent'
            }`}
        >
          <CandlestickChart className="w-4 h-4" />
          {t('marketChart', language)}
        </button>
      </div>

      {/* Tab Content */}
      <div className="relative overflow-hidden min-h-[400px]">
        <AnimatePresence mode="wait">
          {activeTab === 'equity' ? (
            <motion.div
              key="equity"
              initial={{ opacity: 0, x: -20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: 20 }}
              transition={{ duration: 0.2 }}
              className="h-full"
            >
              <EquityChart traderId={traderId} />
            </motion.div>
          ) : (
            <motion.div
              key={`kline-${chartSymbol}-${exchangeId}`}
              initial={{ opacity: 0, x: 20 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -20 }}
              transition={{ duration: 0.2 }}
              className="h-full"
            >
              <TradingViewChart
                height={400}
                embedded
                defaultSymbol={chartSymbol}
                defaultExchange={exchangeId}
                key={`${chartSymbol}-${exchangeId}-${updateKey || ''}`}
              />
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  )
}

