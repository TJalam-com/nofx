import { useState, useEffect, useRef } from 'react'
import type { Language } from '../../i18n/translations'

interface IndicatorConfig {
  enable_raw_klines: boolean
  enable_ema: boolean
  enable_macd: boolean
  enable_rsi: boolean
  enable_atr: boolean
  enable_volume: boolean
  enable_oi: boolean
  enable_funding: boolean
  indicator_timeframe: string
  quant_data_url: string
}

interface IndicatorEditorProps {
  config: IndicatorConfig
  onChange: (config: IndicatorConfig) => void
  language: Language
}

export function IndicatorEditor({
  config,
  onChange,
  language,
}: IndicatorEditorProps) {
  // Use ref to store latest onChange without including it in dependencies
  const onChangeRef = useRef(onChange)
  const isInitialMount = useRef(true)
  const isSyncingFromProps = useRef(false)
  const lastConfigRef = useRef<string>('')

  // Keep onChangeRef up to date
  useEffect(() => {
    onChangeRef.current = onChange
  }, [onChange])

  const [localConfig, setLocalConfig] = useState<IndicatorConfig>({
    ...config,
    enable_raw_klines: true, // Always true, locked
  })

  // Sync localConfig when config prop changes externally
  useEffect(() => {
    // Create a stable string representation for comparison
    const configKey = JSON.stringify({
      enable_raw_klines: config.enable_raw_klines ?? true,
      enable_ema: config.enable_ema ?? false,
      enable_macd: config.enable_macd ?? false,
      enable_rsi: config.enable_rsi ?? false,
      enable_atr: config.enable_atr ?? false,
      enable_volume: config.enable_volume ?? true,
      enable_oi: config.enable_oi ?? true,
      enable_funding: config.enable_funding ?? true,
      indicator_timeframe: config.indicator_timeframe || '3m',
      quant_data_url: config.quant_data_url || '',
    })

    // Only update if config actually changed
    if (configKey !== lastConfigRef.current) {
      lastConfigRef.current = configKey
      isSyncingFromProps.current = true
      setLocalConfig({
        ...config,
        enable_raw_klines: true, // Always ensure this is true
      })
    }
  }, [config])

  // Ensure enable_raw_klines is always true
  useEffect(() => {
    setLocalConfig((prev) => ({
      ...prev,
      enable_raw_klines: true,
    }))
  }, [])

  // Call onChange when localConfig changes (but not on initial mount or prop sync)
  useEffect(() => {
    // Skip onChange call on initial mount to prevent unnecessary parent updates
    if (isInitialMount.current) {
      isInitialMount.current = false
      return
    }

    // Skip onChange call when syncing from props (to avoid circular updates)
    if (isSyncingFromProps.current) {
      isSyncingFromProps.current = false
      return
    }

    // Use ref to call latest onChange without including it in dependencies
    onChangeRef.current(localConfig)
  }, [localConfig]) // Only depend on localConfig, not onChange

  const updateConfig = (updates: Partial<IndicatorConfig>) => {
    setLocalConfig((prev) => {
      const newConfig = { ...prev, ...updates }
      // Always ensure enable_raw_klines is true
      newConfig.enable_raw_klines = true
      return newConfig
    })
  }

  const timeframes = [
    { value: '3m', label: '3m' },
    { value: '15m', label: '15m' },
    { value: '1h', label: '1h' },
    { value: '4h', label: '4h' },
  ]

  return (
    <div className="space-y-6">
      {/* Section 1: Market Data */}
      <div
        className="p-4 rounded-lg"
        style={{
          background: 'rgba(0, 123, 255, 0.1)',
          border: '1px solid rgba(0, 123, 255, 0.3)',
        }}
      >
        <h4 className="text-sm font-bold mb-3" style={{ color: '#4A9EFF' }}>
          📊 Market Data
        </h4>
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={localConfig.enable_raw_klines}
                disabled
                className="w-4 h-4 rounded"
                style={{
                  background: 'var(--navy-primary)',
                  border: '1px solid var(--panel-border)',
                  cursor: 'not-allowed',
                }}
              />
              <label
                className="text-sm font-semibold"
                style={{ color: '#EAECEF' }}
              >
                Raw OHLCV (Open, High, Low, Close, Volume)
              </label>
            </div>
            <span
              className="text-xs px-2 py-1 rounded"
              style={{
                background: 'var(--panel-border)',
                color: '#848E9C',
              }}
            >
              Required
            </span>
          </div>
          <div className="text-xs ml-6" style={{ color: '#848E9C' }}>
            {language === 'zh'
              ? '原始K线数据是必需的，AI需要这些基础数据进行分析。'
              : 'Raw kline data is required. AI needs this fundamental data for analysis.'}
          </div>

          <div>
            <label
              className="block text-sm font-semibold mb-2"
              style={{ color: '#EAECEF' }}
            >
              Timeframe
            </label>
            <select
              value={localConfig.indicator_timeframe || '3m'}
              onChange={(e) =>
                updateConfig({ indicator_timeframe: e.target.value })
              }
              className="w-full px-3 py-2 rounded"
              style={{
                background: 'var(--navy-primary)',
                border: '1px solid var(--panel-border)',
                color: '#EAECEF',
              }}
            >
              {timeframes.map((tf) => (
                <option key={tf.value} value={tf.value}>
                  {tf.label}
                </option>
              ))}
            </select>
            <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
              {language === 'zh'
                ? '选择用于技术指标计算的时间框架'
                : 'Select the timeframe for technical indicator calculations'}
            </div>
          </div>
        </div>
      </div>

      {/* Section 2: Technical Indicators */}
      <div
        className="p-4 rounded-lg"
        style={{
          background: 'rgba(255, 193, 7, 0.1)',
          border: '1px solid rgba(255, 193, 7, 0.3)',
        }}
      >
        <h4 className="text-sm font-bold mb-3" style={{ color: '#00FF7F' }}>
          📈 Technical Indicators (Optional)
        </h4>
        <div className="text-xs mb-3" style={{ color: '#848E9C' }}>
          {language === 'zh'
            ? '这些指标是可选的。AI可以自己计算这些指标，但启用它们可以让AI更快地获取预计算的值。'
            : 'These indicators are optional. AI can calculate them itself, but enabling them allows AI to quickly access pre-calculated values.'}
        </div>
        <div className="space-y-2">
          {[
            { key: 'enable_ema', label: 'EMA (Exponential Moving Average)' },
            {
              key: 'enable_macd',
              label: 'MACD (Moving Average Convergence Divergence)',
            },
            { key: 'enable_rsi', label: 'RSI (Relative Strength Index)' },
            { key: 'enable_atr', label: 'ATR (Average True Range)' },
          ].map((indicator) => (
            <div key={indicator.key} className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={
                  localConfig[indicator.key as keyof IndicatorConfig] as boolean
                }
                onChange={(e) =>
                  updateConfig({
                    [indicator.key]: e.target.checked,
                  } as Partial<IndicatorConfig>)
                }
                className="w-4 h-4 rounded"
                style={{
                  background: 'var(--navy-primary)',
                  border: '1px solid var(--panel-border)',
                  color: '#EAECEF',
                }}
              />
              <label className="text-sm" style={{ color: '#EAECEF' }}>
                {indicator.label}
              </label>
            </div>
          ))}
        </div>
      </div>

      {/* Section 3: Market Sentiment */}
      <div
        className="p-4 rounded-lg"
        style={{
          background: 'rgba(0, 255, 127, 0.1)',
          border: '1px solid rgba(0, 255, 127, 0.3)',
        }}
      >
        <h4 className="text-sm font-bold mb-3" style={{ color: '#00FF7F' }}>
          💹 Market Sentiment
        </h4>
        <div className="space-y-2">
          {[
            {
              key: 'enable_volume',
              label: 'Volume',
              description:
                language === 'zh'
                  ? '交易量数据，用于识别异常交易活动'
                  : 'Trading volume data for identifying unusual trading activity',
            },
            {
              key: 'enable_oi',
              label: 'Open Interest (OI)',
              description:
                language === 'zh'
                  ? '持仓量数据，反映市场参与者的持仓情况'
                  : "Open interest data reflecting market participants' positions",
            },
            {
              key: 'enable_funding',
              label: 'Funding Rate',
              description:
                language === 'zh'
                  ? '资金费率，反映市场多空情绪'
                  : 'Funding rate reflecting market sentiment',
            },
          ].map((item) => (
            <div key={item.key}>
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={
                    localConfig[item.key as keyof IndicatorConfig] as boolean
                  }
                  onChange={(e) =>
                    updateConfig({
                      [item.key]: e.target.checked,
                    } as Partial<IndicatorConfig>)
                  }
                  className="w-4 h-4 rounded"
                  style={{
                    background: 'var(--navy-primary)',
                    border: '1px solid var(--panel-border)',
                    color: '#EAECEF',
                  }}
                />
                <label
                  className="text-sm font-semibold"
                  style={{ color: '#EAECEF' }}
                >
                  {item.label}
                </label>
              </div>
              <div className="text-xs ml-6 mt-1" style={{ color: '#848E9C' }}>
                {item.description}
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Section 4: Quant Data */}
      <div
        className="p-4 rounded-lg"
        style={{
          background: 'rgba(156, 39, 176, 0.1)',
          border: '1px solid rgba(156, 39, 176, 0.3)',
        }}
      >
        <h4 className="text-sm font-bold mb-3" style={{ color: '#9C27B0' }}>
          🔗 Quant Data (External API)
        </h4>
        <div className="space-y-3">
          <div>
            <label
              className="block text-sm font-semibold mb-2"
              style={{ color: '#EAECEF' }}
            >
              Quant Data API URL
            </label>
            <input
              type="url"
              value={localConfig.quant_data_url || ''}
              onChange={(e) => updateConfig({ quant_data_url: e.target.value })}
              placeholder="https://api.example.com/quant/{symbol}?include=netflow,oi,price"
              className="w-full px-3 py-2 rounded"
              style={{
                background: 'var(--navy-primary)',
                border: '1px solid var(--panel-border)',
                color: '#EAECEF',
              }}
            />
            <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
              {language === 'zh'
                ? '外部量化数据API URL。必须包含 {symbol} 占位符，系统会自动替换为交易对符号（如 BTCUSDT）。'
                : 'External quant data API URL. Must include {symbol} placeholder, which will be automatically replaced with trading pair symbol (e.g., BTCUSDT).'}
            </div>
            {localConfig.quant_data_url &&
              !localConfig.quant_data_url.includes('{symbol}') && (
                <div
                  className="text-xs mt-1 px-2 py-1 rounded"
                  style={{
                    background: 'rgba(246, 70, 93, 0.1)',
                    border: '1px solid rgba(246, 70, 93, 0.3)',
                    color: '#F6465D',
                  }}
                >
                  ⚠️{' '}
                  {language === 'zh'
                    ? '警告：URL中缺少 {symbol} 占位符'
                    : 'Warning: URL missing {symbol} placeholder'}
                </div>
              )}
          </div>
        </div>
      </div>
    </div>
  )
}
