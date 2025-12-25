import { useEffect, useRef, useState, memo } from 'react'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { ChevronDown, TrendingUp, X } from 'lucide-react'

// Helper to conditionally send ingest logs (only if env var is set)
const shouldLogIngest = () => {
  return import.meta.env.VITE_ENABLE_INGEST_LOGS === 'true' || 
         (typeof window !== 'undefined' && (window as any).__ENABLE_INGEST_LOGS__ === true)
}

const logIngest = (data: any) => {
  // Debug logging removed - no-op function to prevent errors if called
}

// Supported exchange list (futures format)
const EXCHANGES = [
  { id: 'BINANCE', name: 'Binance', prefix: 'BINANCE:', suffix: '.P' },
  { id: 'BYBIT', name: 'Bybit', prefix: 'BYBIT:', suffix: '.P' },
  { id: 'OKX', name: 'OKX', prefix: 'OKX:', suffix: '.P' },
  { id: 'BITGET', name: 'Bitget', prefix: 'BITGET:', suffix: '.P' },
  { id: 'MEXC', name: 'MEXC', prefix: 'MEXC:', suffix: '.P' },
  { id: 'GATEIO', name: 'Gate.io', prefix: 'GATEIO:', suffix: '.P' },
] as const

// Popular trading pairs
const POPULAR_SYMBOLS = [
  'BTCUSDT',
  'ETHUSDT',
  'SOLUSDT',
  'BNBUSDT',
  'XRPUSDT',
  'DOGEUSDT',
  'ADAUSDT',
  'AVAXUSDT',
  'DOTUSDT',
  'LINKUSDT',
  'MATICUSDT',
  'LTCUSDT',
]

// Time interval options
const INTERVALS = [
  { id: '1', label: '1m' },
  { id: '5', label: '5m' },
  { id: '15', label: '15m' },
  { id: '30', label: '30m' },
  { id: '60', label: '1H' },
  { id: '240', label: '4H' },
  { id: 'D', label: '1D' },
  { id: 'W', label: '1W' },
]

interface TradingViewChartProps {
  defaultSymbol?: string
  defaultExchange?: string
  height?: number
  showToolbar?: boolean
  embedded?: boolean // Embedded mode (hide header controls)
}

function TradingViewChartComponent({
  defaultSymbol = 'BTCUSDT',
  defaultExchange = 'BINANCE',
  height = 400,
  showToolbar = true,
  embedded = false,
}: TradingViewChartProps) {
  const { language } = useLanguage()
  const containerRef = useRef<HTMLDivElement>(null)
  const [exchange, setExchange] = useState(defaultExchange)
  const [symbol, setSymbol] = useState(defaultSymbol)
  const [timeInterval, setTimeInterval] = useState('60')
  const [customSymbol, setCustomSymbol] = useState('')
  const [showExchangeDropdown, setShowExchangeDropdown] = useState(false)
  const [showSymbolDropdown, setShowSymbolDropdown] = useState(false)
  const [isFullscreen, setIsFullscreen] = useState(false)

  // When defaultSymbol changes, update local symbol
  useEffect(() => {
    logIngest({
      location: 'TradingViewChart.tsx:70',
      message: 'Default symbol effect',
      data: { defaultSymbol, currentSymbol: symbol, willUpdate: defaultSymbol && defaultSymbol !== symbol },
      timestamp: Date.now(),
      sessionId: 'debug-session',
      runId: 'run1',
      hypothesisId: 'E'
    })
    if (defaultSymbol && defaultSymbol !== symbol) {
      setSymbol(defaultSymbol)
    }
  }, [defaultSymbol])

  // When defaultExchange changes, update local exchange
  useEffect(() => {
    if (defaultExchange && defaultExchange !== exchange) {
      const normalizedExchange = defaultExchange.toUpperCase()
      if (EXCHANGES.some(e => e.id === normalizedExchange)) {
        setExchange(normalizedExchange)
      }
    }
  }, [defaultExchange])

  // Get full exchange symbol pair (futures format: BINANCE:BTCUSDT.P)
  const getFullSymbol = () => {
    const exchangeInfo = EXCHANGES.find((e) => e.id === exchange)
    const prefix = exchangeInfo?.prefix || 'BINANCE:'
    const suffix = exchangeInfo?.suffix || '.P'
    const fullSymbol = `${prefix}${symbol}${suffix}`
    logIngest({
      location: 'TradingViewChart.tsx:91',
      message: 'Full symbol generated',
      data: { symbol, exchange, prefix, suffix, fullSymbol },
      timestamp: Date.now(),
      sessionId: 'debug-session',
      runId: 'run1',
      hypothesisId: 'C'
    })
    return fullSymbol
  }

  // Load TradingView Widget
  useEffect(() => {
    logIngest({
      location: 'TradingViewChart.tsx:95',
      message: 'Widget useEffect triggered',
      data: { symbol, exchange, timeInterval, hasContainer: !!containerRef.current },
      timestamp: Date.now(),
      sessionId: 'debug-session',
      runId: 'run1',
      hypothesisId: 'B'
    })
    if (!containerRef.current) return

    // Clear container
    containerRef.current.innerHTML = ''

    // Create widget container
    const widgetContainer = document.createElement('div')
    widgetContainer.className = 'tradingview-widget-container'
    widgetContainer.style.height = '100%'
    widgetContainer.style.width = '100%'

    const widgetDiv = document.createElement('div')
    widgetDiv.className = 'tradingview-widget-container__widget'
    widgetDiv.style.height = '100%'
    widgetDiv.style.width = '100%'

    widgetContainer.appendChild(widgetDiv)
    containerRef.current.appendChild(widgetContainer)

    // Load TradingView script
    const script = document.createElement('script')
    script.src =
      'https://s3.tradingview.com/external-embedding/embed-widget-advanced-chart.js'
    script.type = 'text/javascript'
    script.async = true
    
    const fullSymbol = getFullSymbol()
    const widgetConfig = {
      width: '100%',
      height: '100%',
      symbol: fullSymbol,
      interval: timeInterval,
      timezone: 'Etc/UTC',
      theme: 'dark',
      style: '1',
      locale: language === 'zh' ? 'zh_CN' : 'en',
      enable_publishing: false,
      backgroundColor: 'rgba(11, 14, 17, 1)',
      gridColor: 'rgba(43, 49, 57, 0.5)',
      hide_top_toolbar: !showToolbar,
      hide_legend: false,
      save_image: false,
      calendar: false,
      hide_volume: false,
      support_host: 'https://www.tradingview.com',
    }
    
    logIngest({
      location: 'TradingViewChart.tsx:142',
      message: 'Widget config created',
      data: { widgetConfig, fullSymbol },
      timestamp: Date.now(),
      sessionId: 'debug-session',
      runId: 'run1',
      hypothesisId: 'C'
    })
    
    script.innerHTML = JSON.stringify(widgetConfig)
    
    // Add error handling
    script.onerror = () => {
      logIngest({
        location: 'TradingViewChart.tsx:145',
        message: 'Script onerror fired',
        data: { fullSymbol, scriptSrc: script.src },
        timestamp: Date.now(),
        sessionId: 'debug-session',
        runId: 'run1',
        hypothesisId: 'D'
      })
      console.error('Failed to load TradingView widget script')
      if (containerRef.current) {
        containerRef.current.innerHTML = `
          <div style="display: flex; align-items: center; justify-content: center; height: 100%; color: #848E9C; flex-direction: column; gap: 8px; padding: 20px;">
            <div style="font-size: 14px;">⚠️ Failed to load TradingView chart</div>
            <div style="font-size: 12px; text-align: center;">Please check your internet connection or try refreshing the page</div>
          </div>
        `
      }
    }

    // Monitor widget initialization
    script.onload = () => {
      logIngest({
        location: 'TradingViewChart.tsx:157',
        message: 'Script onload fired',
        data: { fullSymbol },
        timestamp: Date.now(),
        sessionId: 'debug-session',
        runId: 'run1',
        hypothesisId: 'B'
      })
      // Check if widget rendered after a delay
      setTimeout(() => {
        const widgetElement = containerRef.current?.querySelector('.tradingview-widget-container__widget')
        const hasContent = widgetElement && widgetElement.children.length > 0
        logIngest({
          location: 'TradingViewChart.tsx:161',
          message: 'Widget render check',
          data: { fullSymbol, hasContent, childrenCount: widgetElement?.children.length || 0 },
          timestamp: Date.now(),
          sessionId: 'debug-session',
          runId: 'run1',
          hypothesisId: 'B'
        })
      }, 2000)
    }

    widgetContainer.appendChild(script)

    return () => {
      if (containerRef.current) {
        containerRef.current.innerHTML = ''
      }
    }
  }, [exchange, symbol, timeInterval, language, showToolbar])

  // Handle custom symbol input
  const handleCustomSymbolSubmit = () => {
    logIngest({
      location: 'TradingViewChart.tsx:167',
      message: 'Custom symbol submit called',
      data: { customSymbol: customSymbol.trim(), exchange },
      timestamp: Date.now(),
      sessionId: 'debug-session',
      runId: 'run1',
      hypothesisId: 'A'
    })
    if (customSymbol.trim()) {
      let sym = customSymbol.trim().toUpperCase()
      const originalSym = sym
      // If no USDT suffix, add automatically
      if (!sym.endsWith('USDT')) {
        sym = sym + 'USDT'
      }
      logIngest({
        location: 'TradingViewChart.tsx:174',
        message: 'Symbol transformed',
        data: { originalSym, transformedSym: sym, exchange },
        timestamp: Date.now(),
        sessionId: 'debug-session',
        runId: 'run1',
        hypothesisId: 'A'
      })
      setSymbol(sym)
      setCustomSymbol('')
      setShowSymbolDropdown(false)
    }
  }

  return (
    <div
      className={`${embedded ? '' : 'binance-card'} overflow-hidden ${embedded ? '' : 'animate-fade-in'} ${isFullscreen
          ? 'fixed inset-0 z-50 rounded-none flex flex-col'
          : height === 0 
            ? 'flex flex-col h-full'
            : ''
        }`}
      style={isFullscreen ? { background: 'var(--navy-primary)' } : height === 0 ? { height: '100%', display: 'flex', flexDirection: 'column' } : undefined}
    >
      {/* Header */}
      <div
        className="flex flex-wrap items-center gap-2 p-3 sm:p-4"
        style={{ 
          borderBottom: embedded ? 'none' : '1px solid var(--panel-border)',
          flexShrink: 0
        }}
      >
        {!embedded && (
          <div className="flex items-center gap-2">
            <TrendingUp className="w-5 h-5" style={{ color: '#00CC66' }} />
            <h3
              className="text-base sm:text-lg font-bold"
              style={{ color: '#EAECEF' }}
            >
              {t('marketChart', language)}
            </h3>
          </div>
        )}

        {/* Controls */}
        <div className={`flex flex-wrap items-center gap-2 ${embedded ? '' : 'ml-auto'}`}>
          {/* Exchange Selector */}
          <div className="relative">
            <button
              onClick={() => {
                setShowExchangeDropdown(!showExchangeDropdown)
                setShowSymbolDropdown(false)
              }}
              className="flex items-center gap-1 px-3 py-1.5 rounded text-sm font-medium transition-all"
              style={{
                background: 'var(--panel-bg)',
                border: '1px solid var(--panel-border)',
                color: '#EAECEF',
              }}
            >
              {EXCHANGES.find((e) => e.id === exchange)?.name || exchange}
              <ChevronDown className="w-4 h-4" style={{ color: '#848E9C' }} />
            </button>

            {showExchangeDropdown && (
              <div
                className="absolute top-full left-0 mt-1 py-1 rounded-lg shadow-xl z-20 min-w-[120px]"
                style={{
                  background: 'var(--panel-bg)',
                  border: '1px solid var(--panel-border)',
                }}
              >
                {EXCHANGES.map((ex) => (
                  <button
                    key={ex.id}
                    onClick={() => {
                      setExchange(ex.id)
                      setShowExchangeDropdown(false)
                    }}
                    className="w-full px-4 py-2 text-left text-sm transition-all hover:bg-opacity-50"
                    style={{
                      color: exchange === ex.id ? '#00CC66' : '#EAECEF',
                      background:
                        exchange === ex.id
                          ? 'rgba(0, 255, 127, 0.1)'
                          : 'transparent',
                    }}
                  >
                    {ex.name}
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* Symbol Selector */}
          <div className="relative">
            <button
              onClick={() => {
                setShowSymbolDropdown(!showSymbolDropdown)
                setShowExchangeDropdown(false)
              }}
              className="flex items-center gap-1 px-3 py-1.5 rounded text-sm font-bold transition-all"
              style={{
                background: 'rgba(0, 255, 127, 0.1)',
                border: '1px solid rgba(0, 255, 127, 0.3)',
                color: '#00CC66',
              }}
            >
              {symbol}
              <ChevronDown className="w-4 h-4" />
            </button>

            {showSymbolDropdown && (
              <div
                className="absolute top-full left-0 mt-1 py-2 rounded-lg shadow-xl z-20 w-[280px]"
                style={{
                  background: 'var(--panel-bg)',
                  border: '1px solid var(--panel-border)',
                }}
              >
                {/* Custom Input */}
                <div className="px-3 pb-2" style={{ borderBottom: '1px solid var(--panel-border)' }}>
                  <div className="flex gap-2">
                    <input
                      type="text"
                      value={customSymbol}
                      onChange={(e) => setCustomSymbol(e.target.value.toUpperCase())}
                      onKeyDown={(e) => e.key === 'Enter' && handleCustomSymbolSubmit()}
                      placeholder={t('enterSymbol', language)}
                      className="flex-1 px-3 py-1.5 rounded text-sm"
                      style={{
                        background: 'var(--navy-primary)',
                        border: '1px solid var(--panel-border)',
                        color: '#EAECEF',
                      }}
                    />
                    <button
                      onClick={handleCustomSymbolSubmit}
                      className="px-3 py-1.5 rounded text-sm font-medium"
                      style={{
                        background: '#00CC66',
                        color: 'var(--navy-primary)',
                      }}
                    >
                      OK
                    </button>
                  </div>
                </div>

                {/* Popular Symbols */}
                <div className="px-2 pt-2">
                  <div
                    className="text-xs px-2 py-1 mb-1"
                    style={{ color: '#848E9C' }}
                  >
                    {t('popularSymbols', language)}
                  </div>
                  <div className="grid grid-cols-3 gap-1">
                    {POPULAR_SYMBOLS.map((sym) => (
                      <button
                        key={sym}
                        onClick={() => {
                          setSymbol(sym)
                          setShowSymbolDropdown(false)
                        }}
                        className="px-2 py-1.5 rounded text-xs font-medium transition-all"
                        style={{
                          color: symbol === sym ? '#00CC66' : '#EAECEF',
                          background:
                            symbol === sym
                              ? 'rgba(0, 255, 127, 0.1)'
                              : 'rgba(43, 49, 57, 0.3)',
                        }}
                      >
                        {sym.replace('USDT', '')}
                      </button>
                    ))}
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* Interval Selector */}
          <div
            className="flex gap-0.5 p-0.5 rounded"
            style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
          >
            {INTERVALS.map((int) => (
              <button
                key={int.id}
                onClick={() => setTimeInterval(int.id)}
                className="px-2 py-1 rounded text-xs font-medium transition-all"
                style={{
                  background: timeInterval === int.id ? '#00CC66' : 'transparent',
                  color: timeInterval === int.id ? 'var(--navy-primary)' : '#848E9C',
                }}
              >
                {int.label}
              </button>
            ))}
          </div>

          {/* Fullscreen Toggle */}
          <button
            onClick={() => setIsFullscreen(!isFullscreen)}
            className="p-1.5 rounded transition-all"
            style={{
              background: isFullscreen ? '#00CC66' : 'transparent',
              color: isFullscreen ? 'var(--navy-primary)' : '#848E9C',
              border: '1px solid var(--panel-border)',
            }}
            title={isFullscreen ? t('exitFullscreen', language) : t('fullscreen', language)}
          >
            {isFullscreen ? (
              <X className="w-4 h-4" />
            ) : (
              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M8 3H5a2 2 0 00-2 2v3m18 0V5a2 2 0 00-2-2h-3m0 18h3a2 2 0 002-2v-3M3 16v3a2 2 0 002 2h3" />
              </svg>
            )}
          </button>
        </div>
      </div>

      {/* Chart Container */}
      <div
        ref={containerRef}
        style={{
          height: isFullscreen 
            ? 'calc(100vh - 65px)' 
            : height > 0 
              ? height 
              : '100%',
          minHeight: height > 0 ? height : '400px',
          flex: height === 0 ? '1 1 auto' : '0 0 auto',
          background: 'var(--navy-primary)',
          overflow: 'hidden',
        }}
      />

      {/* Click outside to close dropdowns */}
      {(showExchangeDropdown || showSymbolDropdown) && (
        <div
          className="fixed inset-0 z-10"
          onClick={() => {
            setShowExchangeDropdown(false)
            setShowSymbolDropdown(false)
          }}
        />
      )}
    </div>
  )
}

// Use memo to prevent unnecessary re-renders
export const TradingViewChart = memo(TradingViewChartComponent)

