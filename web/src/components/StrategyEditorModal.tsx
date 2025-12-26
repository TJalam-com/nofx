import { useState, useEffect } from 'react'
import { X as IconX, Settings, Info, ChevronDown, ChevronUp } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../lib/api'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth, isFollower } from '../contexts/AuthContext'
import type { Strategy, CreateStrategyRequest, UpdateStrategyRequest } from '../types'
import { IndicatorEditor } from './traders/IndicatorEditor'
import { PromptTemplateModal } from './PromptTemplateModal'
import { Tooltip } from './traders/Tooltip'
import type { PromptTemplate } from '../types'

interface StrategyEditorModalProps {
  isOpen: boolean
  onClose: () => void
  strategy?: Strategy | null
  onSave: () => void
}

export function StrategyEditorModal({
  isOpen,
  onClose,
  strategy,
  onSave,
}: StrategyEditorModalProps) {
  const { language } = useLanguage()
  const { user } = useAuth()
  const isEditMode = !!strategy
  const userIsFollower = user ? isFollower(user) : false
  
  // Collapsible section states
  const [showRiskManagement, setShowRiskManagement] = useState(false)
  const [showPositionSizing, setShowPositionSizing] = useState(false)
  const [showTradingRules, setShowTradingRules] = useState(false)

  const [formData, setFormData] = useState<CreateStrategyRequest>({
    name: '',
    description: '',
    system_prompt_template: 'default',
    custom_prompt: '',
    override_base_prompt: false,
    btc_eth_leverage: 5,
    altcoin_leverage: 3,
    trading_symbols: '',
    is_cross_margin: true,
    use_coin_pool: false,
    use_oi_top: false,
    use_tradingview: false,
    enable_raw_klines: true,
    enable_ema: false,
    enable_macd: false,
    enable_rsi: false,
    enable_atr: false,
    enable_volume: true,
    enable_oi: true,
    enable_funding: true,
    indicator_timeframe: '3m',
    quant_data_url: '',
    // Risk Management Configuration (defaults)
    min_risk_reward_ratio: 3.0,
    max_positions: 3,
    margin_usage_limit: 90.0,
    min_opening_amount: 12.0,
    min_opening_amount_btc_eth: 60.0,
    // Position Sizing Configuration (defaults)
    altcoin_position_min: 0.8,
    altcoin_position_max: 1.5,
    btc_eth_position_min: 5.0,
    btc_eth_position_max: 10.0,
    available_margin_multiplier: 0.88,
    // Trading Rules Configuration (defaults)
    min_confidence_for_entry: 75,
    min_holding_time_minutes: 30,
    // Sharpe Ratio Configuration (defaults)
    sharpe_ratio_config: '',
  })

  const [isSaving, setIsSaving] = useState(false)
  const [promptTemplates, setPromptTemplates] = useState<{ name: string }[]>([])
  const [userPromptTemplates, setUserPromptTemplates] = useState<PromptTemplate[]>([])
  const [showTemplateModal, setShowTemplateModal] = useState(false)
  const [editingTemplate, setEditingTemplate] = useState<PromptTemplate | null>(null)

  // Load prompt templates
  useEffect(() => {
    if (isOpen) {
      const loadTemplates = async () => {
        try {
          const [systemTemplates, userTemplates] = await Promise.all([
            api.getPromptTemplates(),
            api.getUserPromptTemplates(),
          ])
          // getPromptTemplates() returns string[], so map directly to { name: string }[]
          setPromptTemplates(systemTemplates.map((t) => ({ name: t })))
          setUserPromptTemplates(userTemplates)
        } catch (error) {
          console.error('Failed to load prompt templates:', error)
        }
      }
      loadTemplates()
    }
  }, [isOpen])

  // Load strategy data when editing
  useEffect(() => {
    if (isOpen && strategy) {
      setFormData({
        name: strategy.name,
        description: strategy.description || '',
        system_prompt_template: strategy.system_prompt_template || 'default',
        custom_prompt: strategy.custom_prompt || '',
        override_base_prompt: strategy.override_base_prompt || false,
        btc_eth_leverage: strategy.btc_eth_leverage || 5,
        altcoin_leverage: strategy.altcoin_leverage || 3,
        trading_symbols: strategy.trading_symbols || '',
        is_cross_margin: strategy.is_cross_margin ?? true,
        use_coin_pool: strategy.use_coin_pool || false,
        use_oi_top: strategy.use_oi_top || false,
        use_tradingview: strategy.use_tradingview || false,
        enable_raw_klines: strategy.enable_raw_klines ?? true,
        enable_ema: strategy.enable_ema || false,
        enable_macd: strategy.enable_macd || false,
        enable_rsi: strategy.enable_rsi || false,
        enable_atr: strategy.enable_atr || false,
        enable_volume: strategy.enable_volume ?? true,
        enable_oi: strategy.enable_oi ?? true,
        enable_funding: strategy.enable_funding ?? true,
        indicator_timeframe: strategy.indicator_timeframe || '3m',
        quant_data_url: strategy.quant_data_url || '',
        // Risk Management Configuration
        min_risk_reward_ratio: strategy.min_risk_reward_ratio ?? 3.0,
        max_positions: strategy.max_positions ?? 3,
        margin_usage_limit: strategy.margin_usage_limit ?? 90.0,
        min_opening_amount: strategy.min_opening_amount ?? 12.0,
        min_opening_amount_btc_eth: strategy.min_opening_amount_btc_eth ?? 60.0,
        // Position Sizing Configuration
        altcoin_position_min: strategy.altcoin_position_min ?? 0.8,
        altcoin_position_max: strategy.altcoin_position_max ?? 1.5,
        btc_eth_position_min: strategy.btc_eth_position_min ?? 5.0,
        btc_eth_position_max: strategy.btc_eth_position_max ?? 10.0,
        available_margin_multiplier: strategy.available_margin_multiplier ?? 0.88,
        // Trading Rules Configuration
        min_confidence_for_entry: strategy.min_confidence_for_entry ?? 75,
        min_holding_time_minutes: strategy.min_holding_time_minutes ?? 30,
        // Sharpe Ratio Configuration
        sharpe_ratio_config: strategy.sharpe_ratio_config || '',
      })
    } else if (isOpen && !strategy) {
      // Reset form for new strategy
      setFormData({
        name: '',
        description: '',
        system_prompt_template: 'default',
        custom_prompt: '',
        override_base_prompt: false,
        btc_eth_leverage: 5,
        altcoin_leverage: 3,
        trading_symbols: '',
        is_cross_margin: true,
        use_coin_pool: false,
        use_oi_top: false,
        use_tradingview: false,
        enable_raw_klines: true,
        enable_ema: false,
        enable_macd: false,
        enable_rsi: false,
        enable_atr: false,
        enable_volume: true,
        enable_oi: true,
        enable_funding: true,
        indicator_timeframe: '3m',
        quant_data_url: '',
        // Risk Management Configuration (defaults)
        min_risk_reward_ratio: 3.0,
        max_positions: 3,
        margin_usage_limit: 90.0,
        min_opening_amount: 12.0,
        min_opening_amount_btc_eth: 60.0,
        // Position Sizing Configuration (defaults)
        altcoin_position_min: 0.8,
        altcoin_position_max: 1.5,
        btc_eth_position_min: 5.0,
        btc_eth_position_max: 10.0,
        available_margin_multiplier: 0.88,
        // Trading Rules Configuration (defaults)
        min_confidence_for_entry: 75,
        min_holding_time_minutes: 30,
        // Sharpe Ratio Configuration (defaults)
        sharpe_ratio_config: '',
      })
    }
  }, [isOpen, strategy])

  const handleInputChange = (field: keyof CreateStrategyRequest, value: any) => {
    setFormData((prev) => ({ ...prev, [field]: value }))
  }

  const handleIndicatorChange = (config: any) => {
    setFormData((prev) => ({
      ...prev,
      ...config,
    }))
  }

  const handleSave = async () => {
    if (!formData.name.trim()) {
      toast.error(language === 'zh' ? '请输入策略名称' : 'Please enter strategy name')
      return
    }

    setIsSaving(true)
    try {
      if (isEditMode && strategy) {
        const updateData: UpdateStrategyRequest = {
          name: formData.name,
          description: formData.description,
          system_prompt_template: formData.system_prompt_template,
          custom_prompt: formData.custom_prompt,
          override_base_prompt: formData.override_base_prompt || false,
          btc_eth_leverage: formData.btc_eth_leverage || 5,
          altcoin_leverage: formData.altcoin_leverage || 3,
          trading_symbols: formData.trading_symbols || '',
          is_cross_margin: formData.is_cross_margin ?? true,
          use_coin_pool: formData.use_coin_pool || false,
          use_oi_top: formData.use_oi_top || false,
          use_tradingview: formData.use_tradingview || false,
          enable_raw_klines: formData.enable_raw_klines ?? true,
          enable_ema: formData.enable_ema || false,
          enable_macd: formData.enable_macd || false,
          enable_rsi: formData.enable_rsi || false,
          enable_atr: formData.enable_atr || false,
          enable_volume: formData.enable_volume ?? true,
          enable_oi: formData.enable_oi ?? true,
          enable_funding: formData.enable_funding ?? true,
          indicator_timeframe: formData.indicator_timeframe || '3m',
          quant_data_url: formData.quant_data_url || '',
          // Risk Management Configuration
          min_risk_reward_ratio: formData.min_risk_reward_ratio,
          max_positions: formData.max_positions,
          margin_usage_limit: formData.margin_usage_limit,
          min_opening_amount: formData.min_opening_amount,
          min_opening_amount_btc_eth: formData.min_opening_amount_btc_eth,
          // Position Sizing Configuration
          altcoin_position_min: formData.altcoin_position_min,
          altcoin_position_max: formData.altcoin_position_max,
          btc_eth_position_min: formData.btc_eth_position_min,
          btc_eth_position_max: formData.btc_eth_position_max,
          available_margin_multiplier: formData.available_margin_multiplier,
          // Trading Rules Configuration
          min_confidence_for_entry: formData.min_confidence_for_entry,
          min_holding_time_minutes: formData.min_holding_time_minutes,
          // Sharpe Ratio Configuration
          sharpe_ratio_config: formData.sharpe_ratio_config,
        }
        await api.updateStrategy(strategy.id, updateData)
        toast.success(language === 'zh' ? '策略更新成功' : 'Strategy updated successfully')
      } else {
        await api.createStrategy(formData)
        toast.success(language === 'zh' ? '策略创建成功' : 'Strategy created successfully')
      }
      onSave()
      onClose()
    } catch (error: any) {
      toast.error(error.message || (language === 'zh' ? '保存策略失败' : 'Failed to save strategy'))
    } finally {
      setIsSaving(false)
    }
  }

  if (!isOpen) return null

  // Combine templates, filtering out duplicates
  // First, deduplicate system templates by name
  const uniqueSystemTemplates = promptTemplates.filter((template, index, self) =>
    index === self.findIndex((t) => t.name.toLowerCase() === template.name.toLowerCase())
  )

  // Filter out system templates from userPromptTemplates that already exist in promptTemplates
  const filteredUserTemplates = userPromptTemplates
    .filter((template: PromptTemplate) => {
      // Filter out system templates that already exist in promptTemplates to avoid duplicates
      if (template.is_system) {
        const normalizedName = template.name.toLowerCase().replace(/[_-]/g, '')
        return !uniqueSystemTemplates.some(t => {
          const tNormalized = t.name.toLowerCase().replace(/[_-]/g, '')
          return tNormalized === normalizedName
        })
      }
      // Always include user-created templates
      return true
    })
    .map((t) => ({ name: t.name }))

  const allTemplates = [
    ...uniqueSystemTemplates,
    ...filteredUserTemplates,
  ]

  // Helper function to get placeholder text based on prompt type
  const getCustomPromptPlaceholder = () => {
    if (formData.use_tradingview) {
      return language === 'zh' 
        ? '输入自定义提示词（可选）。如果启用"覆盖基础提示词"，系统会自动添加 TradingView JSON 格式要求...'
        : 'Enter custom prompt (optional). If "Override Base Prompt" is enabled, TradingView JSON format requirements will be auto-added...'
    }
    return language === 'zh'
      ? '输入自定义提示词（可选）。如果启用"覆盖基础提示词"，系统会自动添加标准 JSON 格式要求...'
      : 'Enter custom prompt (optional). If "Override Base Prompt" is enabled, standard JSON format requirements will be auto-added...'
  }

  // Helper function to get tooltip content
  const getTooltipContent = () => {
    if (formData.use_tradingview) {
      return language === 'zh'
        ? '自定义提示词应包含 TradingView 信号评估策略。例如：如何评估信号质量、风险收益比检查、何时接受/拒绝/修改信号等。系统会自动处理 JSON 格式要求。'
        : 'Custom prompt should contain TradingView signal evaluation strategy. For example: how to assess signal quality, risk-reward ratio checks, when to accept/reject/modify signals, etc. The system will automatically handle JSON format requirements.'
    }
    return language === 'zh'
      ? '自定义提示词应包含您的交易策略说明。例如：趋势跟踪方法、风险管理规则、技术指标使用、止损止盈设置等。系统会自动处理 JSON 格式要求。'
      : 'Custom prompt should contain your trading strategy instructions. For example: trend following methods, risk management rules, technical indicator usage, stop loss and take profit settings, etc. The system will automatically handle JSON format requirements.'
  }

  // Helper function to get prompt examples - actual prompt instruction examples users can copy
  const getPromptExamples = () => {
    if (formData.use_tradingview) {
      // TradingView prompt example
      return language === 'zh'
        ? '# TradingView 信号评估策略\n\n评估每个 TradingView 信号时，请遵循以下原则：\n\n1. 检查风险收益比：确保止损和止盈的比例至少为 1:2\n2. 验证信号质量：确认信号符合当前市场趋势\n3. 评估账户风险：单笔交易风险不超过账户权益的 5%\n4. 如果信号不符合以上标准，请拒绝该信号\n5. 如果信号需要调整参数（如止损、止盈或仓位大小），请修改信号'
        : '# TradingView Signal Evaluation Strategy\n\nWhen evaluating each TradingView signal, follow these principles:\n\n1. Check risk-reward ratio: Ensure stop loss to take profit ratio is at least 1:2\n2. Verify signal quality: Confirm the signal aligns with current market trends\n3. Assess account risk: Single trade risk should not exceed 5% of account equity\n4. If the signal does not meet these criteria, reject it\n5. If the signal needs parameter adjustments (stop loss, take profit, or position size), modify it'
    }
    // Standard trading strategy prompt example
    return language === 'zh'
      ? '# 角色定义\n\n你是一名专业的加密货币交易 AI。你的任务是基于提供的市场数据做出交易决策。你是一位经验丰富的量化交易员，擅长技术分析和风险管理。\n\n# ⏱️ 交易频率意识\n\n- 优秀交易员：每天 2-4 笔交易 ≈ 每小时 0.1-0.2 笔\n- 每小时超过 2 笔交易 = 过度交易\n- 单笔持仓时间 ≥ 30-60 分钟\n如果你发现每个周期都在交易 → 标准太低；如果在 30 分钟内平仓 → 太冲动。\n\n# 🎯 入场标准（严格）\n\n只有在多个信号共振时才入场。自由使用任何有效的分析方法，避免单一指标、冲突信号、横盘整理或平仓后立即重新开仓——这些都是低质量行为。\n\n# 📋 决策流程\n\n1. 检查持仓 → 是否止盈/止损\n2. 扫描候选币种 + 多时间框架 → 是否存在强信号\n3. 先写思考链，然后输出结构化 JSON'
      : '# Role Definition\n\nYou are a professional cryptocurrency trading AI. Your task is to make trading decisions based on the provided market data. You are an experienced quantitative trader skilled in technical analysis and risk management.\n\n# ⏱️ Trading Frequency Awareness\n\n- Excellent traders: 2-4 trades per day ≈ 0.1-0.2 trades per hour\n- More than 2 trades per hour = overtrading\n- Single position holding time ≥ 30-60 minutes\nIf you find yourself trading every cycle → standards too low; if closing positions in less than 30 minutes → too impulsive.\n\n# 🎯 Entry Standards (Strict)\n\nOnly enter positions when multiple signals resonate. Freely use any effective analysis methods, avoid single indicators, conflicting signals, sideways consolidation, or immediately reopening positions after closing - these are low-quality behaviors.\n\n# 📋 Decision Process\n\n1. Check positions → whether to take profit/stop loss\n2. Scan candidate coins + multi-timeframe → whether strong signals exist\n3. Write chain of thought first, then output structured JSON'
  }

  return (
    <>
      <div className="fixed inset-0 z-50 flex items-center justify-center backdrop-blur-sm p-4 overflow-y-auto" style={{ background: 'rgba(0, 31, 63, 0.5)' }}>
        <div
          className="rounded-xl shadow-2xl max-w-4xl w-full my-8"
          style={{ background: 'var(--navy-dark)', border: '1px solid var(--navy-light)', maxHeight: 'calc(100vh - 4rem)' }}
          onClick={(e) => e.stopPropagation()}
        >
          {/* Header */}
          <div className="flex items-center justify-between p-6 border-b sticky top-0 z-10 rounded-t-xl" style={{ borderColor: 'var(--navy-light)', background: 'var(--navy-dark)' }}>
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-[#00CC66] to-[#00AA55] flex items-center justify-center" style={{ color: 'var(--navy-primary)' }}>
                <Settings className="w-5 h-5" />
              </div>
              <div>
                <h2 className="text-xl font-bold text-[#EAECEF]">
                  {isEditMode
                    ? language === 'zh' ? '编辑策略' : 'Edit Strategy'
                    : language === 'zh' ? '创建策略' : 'Create Strategy'}
                </h2>
                <p className="text-sm mt-1" style={{ color: 'var(--navy-light)' }}>
                  {isEditMode
                    ? language === 'zh' ? '编辑您的交易策略配置' : 'Edit your trading strategy configuration'
                    : language === 'zh' ? '创建新的交易策略' : 'Create a new trading strategy'}
                </p>
              </div>
            </div>
            <button
              onClick={onClose}
              className="w-8 h-8 rounded-lg hover:text-[#EAECEF] transition-colors flex items-center justify-center"
              style={{ 
                color: 'var(--navy-light)',
                '--hover-bg': 'var(--navy-light)' 
              } as React.CSSProperties}
            >
              <IconX className="w-4 h-4" />
            </button>
          </div>

          {/* Content */}
          <div className="p-6 space-y-6 overflow-y-auto" style={{ maxHeight: 'calc(100vh - 12rem)' }}>
            {/* Basic Info */}
            <div className="rounded-xl p-5" style={{ background: 'var(--navy-dark)', border: '1px solid var(--navy-light)' }}>
              <h3 className="text-lg font-semibold text-[#EAECEF] mb-4">
                {language === 'zh' ? '基本信息' : 'Basic Information'}
              </h3>
              <div className="space-y-4">
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">
                    {language === 'zh' ? '策略名称' : 'Strategy Name'} *
                  </label>
                  <input
                    type="text"
                    value={formData.name}
                    onChange={(e) => handleInputChange('name', e.target.value)}
                    className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                    placeholder={language === 'zh' ? '输入策略名称' : 'Enter strategy name'}
                  />
                </div>
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">
                    {language === 'zh' ? '描述' : 'Description'}
                  </label>
                  <textarea
                    value={formData.description}
                    onChange={(e) => handleInputChange('description', e.target.value)}
                    rows={3}
                    className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                    placeholder={language === 'zh' ? '输入策略描述（可选）' : 'Enter strategy description (optional)'}
                  />
                </div>
              </div>
            </div>

            {/* Trading Parameters */}
            <div className="rounded-xl p-5" style={{ background: 'var(--navy-dark)', border: '1px solid var(--navy-light)' }}>
              <h3 className="text-lg font-semibold text-[#EAECEF] mb-4">
                {language === 'zh' ? '交易参数' : 'Trading Parameters'}
              </h3>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">
                    {language === 'zh' ? 'BTC/ETH 杠杆' : 'BTC/ETH Leverage'}
                  </label>
                  <input
                    type="number"
                    min="1"
                    max="50"
                    value={formData.btc_eth_leverage}
                    onChange={(e) => handleInputChange('btc_eth_leverage', parseInt(e.target.value) || 5)}
                    className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                  />
                </div>
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">
                    {language === 'zh' ? '山寨币杠杆' : 'Altcoin Leverage'}
                  </label>
                  <input
                    type="number"
                    min="1"
                    max="20"
                    value={formData.altcoin_leverage}
                    onChange={(e) => handleInputChange('altcoin_leverage', parseInt(e.target.value) || 3)}
                    className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                  />
                </div>
                <div className="col-span-2">
                  <label className="text-sm text-[#EAECEF] block mb-2">
                    {language === 'zh' ? '交易币种 (逗号分隔)' : 'Trading Symbols (comma-separated)'}
                  </label>
                  <input
                    type="text"
                    value={formData.trading_symbols}
                    onChange={(e) => handleInputChange('trading_symbols', e.target.value)}
                    className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                    placeholder="BTCUSDT, ETHUSDT"
                  />
                </div>
                <div className="col-span-2">
                  <div className="flex items-center gap-3">
                    <input
                      type="checkbox"
                      checked={formData.is_cross_margin}
                      onChange={(e) => handleInputChange('is_cross_margin', e.target.checked)}
                      className="w-4 h-4 rounded accent-[var(--green-primary)] cursor-pointer"
                      style={{ borderColor: 'var(--navy-light)' }}
                    />
                    <label className="text-sm text-[#EAECEF] cursor-pointer">
                      {language === 'zh' ? '全仓模式' : 'Cross Margin Mode'}
                    </label>
                  </div>
                </div>
              </div>
            </div>

            {/* Risk Management Settings - Only show for non-followers */}
            {!userIsFollower && (
              <div className="rounded-xl p-5" style={{ background: 'var(--navy-dark)', border: '1px solid var(--navy-light)' }}>
                <button
                  onClick={() => setShowRiskManagement(!showRiskManagement)}
                  className="w-full flex items-center justify-between text-lg font-semibold text-[#EAECEF] mb-4"
                >
                  <span>🛡️ {language === 'zh' ? '风险管理设置' : 'Risk Management Settings'}</span>
                  {showRiskManagement ? <ChevronUp className="w-5 h-5" /> : <ChevronDown className="w-5 h-5" />}
                </button>
                {showRiskManagement && (
                  <div className="space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <label className="text-sm text-[#EAECEF] block mb-2">
                          {language === 'zh' ? '风险收益比 (最小)' : 'Risk-Reward Ratio (Minimum)'}
                        </label>
                        <select
                          value={formData.min_risk_reward_ratio || 3.0}
                          onChange={(e) => handleInputChange('min_risk_reward_ratio', parseFloat(e.target.value))}
                          className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
                          style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                        >
                          <option value={2.0}>1:2</option>
                          <option value={3.0}>1:3</option>
                          <option value={4.0}>1:4</option>
                          <option value={5.0}>1:5</option>
                        </select>
                      </div>
                      <div>
                        <label className="text-sm text-[#EAECEF] block mb-2">
                          {language === 'zh' ? '最大持仓数量' : 'Max Positions'}
                        </label>
                        <input
                          type="number"
                          min="1"
                          max="10"
                          value={formData.max_positions || 3}
                          onChange={(e) => handleInputChange('max_positions', parseInt(e.target.value) || 3)}
                          className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
                          style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                        />
                      </div>
                      <div>
                        <label className="text-sm text-[#EAECEF] block mb-2">
                          {language === 'zh' ? '保证金使用率限制 (%)' : 'Margin Usage Limit (%)'}
                        </label>
                        <input
                          type="number"
                          min="50"
                          max="100"
                          step="1"
                          value={formData.margin_usage_limit || 90.0}
                          onChange={(e) => handleInputChange('margin_usage_limit', parseFloat(e.target.value) || 90.0)}
                          className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
                          style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                        />
                      </div>
                      <div>
                        <label className="text-sm text-[#EAECEF] block mb-2">
                          {language === 'zh' ? '最小开仓金额 (山寨币, USDT)' : 'Min Opening Amount (Altcoins, USDT)'}
                        </label>
                        <input
                          type="number"
                          min="1"
                          step="0.1"
                          value={formData.min_opening_amount || 12.0}
                          onChange={(e) => handleInputChange('min_opening_amount', parseFloat(e.target.value) || 12.0)}
                          className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
                          style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                        />
                      </div>
                      <div>
                        <label className="text-sm text-[#EAECEF] block mb-2">
                          {language === 'zh' ? '最小开仓金额 (BTC/ETH, USDT)' : 'Min Opening Amount (BTC/ETH, USDT)'}
                        </label>
                        <input
                          type="number"
                          min="1"
                          step="0.1"
                          value={formData.min_opening_amount_btc_eth || 60.0}
                          onChange={(e) => handleInputChange('min_opening_amount_btc_eth', parseFloat(e.target.value) || 60.0)}
                          className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
                          style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                        />
                      </div>
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Position Sizing Settings - Only show for non-followers */}
            {!userIsFollower && (
              <div className="rounded-xl p-5" style={{ background: 'var(--navy-dark)', border: '1px solid var(--navy-light)' }}>
                <button
                  onClick={() => setShowPositionSizing(!showPositionSizing)}
                  className="w-full flex items-center justify-between text-lg font-semibold text-[#EAECEF] mb-4"
                >
                  <span>📏 {language === 'zh' ? '仓位大小设置' : 'Position Sizing Settings'}</span>
                  {showPositionSizing ? <ChevronUp className="w-5 h-5" /> : <ChevronDown className="w-5 h-5" />}
                </button>
                {showPositionSizing && (
                  <div className="space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <label className="text-sm text-[#EAECEF] block mb-2">
                          {language === 'zh' ? '山寨币仓位最小倍数' : 'Altcoin Position Min (x equity)'}
                        </label>
                        <input
                          type="number"
                          min="0.1"
                          max="5"
                          step="0.1"
                          value={formData.altcoin_position_min || 0.8}
                          onChange={(e) => handleInputChange('altcoin_position_min', parseFloat(e.target.value) || 0.8)}
                          className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
                          style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                        />
                      </div>
                      <div>
                        <label className="text-sm text-[#EAECEF] block mb-2">
                          {language === 'zh' ? '山寨币仓位最大倍数' : 'Altcoin Position Max (x equity)'}
                        </label>
                        <input
                          type="number"
                          min="0.1"
                          max="5"
                          step="0.1"
                          value={formData.altcoin_position_max || 1.5}
                          onChange={(e) => handleInputChange('altcoin_position_max', parseFloat(e.target.value) || 1.5)}
                          className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
                          style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                        />
                      </div>
                      <div>
                        <label className="text-sm text-[#EAECEF] block mb-2">
                          {language === 'zh' ? 'BTC/ETH 仓位最小倍数' : 'BTC/ETH Position Min (x equity)'}
                        </label>
                        <input
                          type="number"
                          min="1"
                          max="20"
                          step="0.1"
                          value={formData.btc_eth_position_min || 5.0}
                          onChange={(e) => handleInputChange('btc_eth_position_min', parseFloat(e.target.value) || 5.0)}
                          className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
                          style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                        />
                      </div>
                      <div>
                        <label className="text-sm text-[#EAECEF] block mb-2">
                          {language === 'zh' ? 'BTC/ETH 仓位最大倍数' : 'BTC/ETH Position Max (x equity)'}
                        </label>
                        <input
                          type="number"
                          min="1"
                          max="20"
                          step="0.1"
                          value={formData.btc_eth_position_max || 10.0}
                          onChange={(e) => handleInputChange('btc_eth_position_max', parseFloat(e.target.value) || 10.0)}
                          className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
                          style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                        />
                      </div>
                      <div className="col-span-2">
                        <label className="text-sm text-[#EAECEF] block mb-2">
                          {language === 'zh' ? '可用保证金乘数' : 'Available Margin Multiplier'}
                        </label>
                        <input
                          type="number"
                          min="0.5"
                          max="1.0"
                          step="0.01"
                          value={formData.available_margin_multiplier || 0.88}
                          onChange={(e) => handleInputChange('available_margin_multiplier', parseFloat(e.target.value) || 0.88)}
                          className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
                          style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                        />
                        <p className="text-xs mt-1" style={{ color: 'var(--navy-light)' }}>
                          {language === 'zh' 
                            ? '用于计算可用保证金的乘数（默认 0.88 = 保留 12% 用于费用和滑点）'
                            : 'Multiplier for calculating available margin (default 0.88 = reserve 12% for fees and slippage)'}
                        </p>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Trading Rules Settings - Only show for non-followers */}
            {!userIsFollower && (
              <div className="rounded-xl p-5" style={{ background: 'var(--navy-dark)', border: '1px solid var(--navy-light)' }}>
                <button
                  onClick={() => setShowTradingRules(!showTradingRules)}
                  className="w-full flex items-center justify-between text-lg font-semibold text-[#EAECEF] mb-4"
                >
                  <span>📋 {language === 'zh' ? '交易规则设置' : 'Trading Rules Settings'}</span>
                  {showTradingRules ? <ChevronUp className="w-5 h-5" /> : <ChevronDown className="w-5 h-5" />}
                </button>
                {showTradingRules && (
                  <div className="space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <label className="text-sm text-[#EAECEF] block mb-2">
                          {language === 'zh' ? '最小入场信心度' : 'Min Confidence for Entry'}
                        </label>
                        <input
                          type="number"
                          min="50"
                          max="100"
                          value={formData.min_confidence_for_entry || 75}
                          onChange={(e) => handleInputChange('min_confidence_for_entry', parseInt(e.target.value) || 75)}
                          className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
                          style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                        />
                      </div>
                      <div>
                        <label className="text-sm text-[#EAECEF] block mb-2">
                          {language === 'zh' ? '最小持仓时间 (分钟)' : 'Min Holding Time (minutes)'}
                        </label>
                        <input
                          type="number"
                          min="1"
                          max="1440"
                          value={formData.min_holding_time_minutes || 30}
                          onChange={(e) => handleInputChange('min_holding_time_minutes', parseInt(e.target.value) || 30)}
                          className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
                          style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                        />
                      </div>
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Signal Sources */}
            <div className="rounded-xl p-5" style={{ background: 'var(--navy-dark)', border: '1px solid var(--navy-light)' }}>
              <h3 className="text-lg font-semibold text-[#EAECEF] mb-4">
                {language === 'zh' ? '信号源' : 'Signal Sources'}
              </h3>
              <div className="space-y-3">
                <div className="flex items-center gap-3">
                  <input
                    type="checkbox"
                    checked={formData.use_coin_pool}
                    onChange={(e) => handleInputChange('use_coin_pool', e.target.checked)}
                    className="w-4 h-4 rounded accent-[var(--green-primary)] cursor-pointer"
                      style={{ borderColor: 'var(--navy-light)' }}
                  />
                  <label className="text-sm text-[#EAECEF] cursor-pointer">
                    {language === 'zh' ? '使用 Coin Pool' : 'Use Coin Pool'}
                  </label>
                </div>
                <div className="flex items-center gap-3">
                  <input
                    type="checkbox"
                    checked={formData.use_oi_top}
                    onChange={(e) => handleInputChange('use_oi_top', e.target.checked)}
                    className="w-4 h-4 rounded accent-[var(--green-primary)] cursor-pointer"
                      style={{ borderColor: 'var(--navy-light)' }}
                  />
                  <label className="text-sm text-[#EAECEF] cursor-pointer">
                    {language === 'zh' ? '使用 OI Top' : 'Use OI Top'}
                  </label>
                </div>
                <div className="flex items-center gap-3">
                  <input
                    type="checkbox"
                    checked={formData.use_tradingview}
                    onChange={(e) => handleInputChange('use_tradingview', e.target.checked)}
                    className="w-4 h-4 rounded accent-[var(--green-primary)] cursor-pointer"
                      style={{ borderColor: 'var(--navy-light)' }}
                  />
                  <label className="text-sm text-[#EAECEF] cursor-pointer">
                    {language === 'zh' ? '使用 TradingView' : 'Use TradingView'}
                  </label>
                </div>
              </div>
            </div>

            {/* Indicator Configuration */}
            <div className="rounded-xl p-5" style={{ background: 'var(--navy-dark)', border: '1px solid var(--navy-light)' }}>
              <h3 className="text-lg font-semibold text-[#EAECEF] mb-5 flex items-center gap-2">
                📊 {language === 'zh' ? '指标配置' : 'Indicator Configuration'}
              </h3>
              <IndicatorEditor
                config={{
                  enable_raw_klines: formData.enable_raw_klines ?? true,
                  enable_ema: formData.enable_ema ?? false,
                  enable_macd: formData.enable_macd ?? false,
                  enable_rsi: formData.enable_rsi ?? false,
                  enable_atr: formData.enable_atr ?? false,
                  enable_volume: formData.enable_volume ?? true,
                  enable_oi: formData.enable_oi ?? true,
                  enable_funding: formData.enable_funding ?? true,
                  indicator_timeframe: formData.indicator_timeframe || '3m',
                  quant_data_url: formData.quant_data_url || '',
                }}
                onChange={handleIndicatorChange}
                language={language}
              />
            </div>

            {/* Trading Prompt */}
            <div className="rounded-xl p-5" style={{ background: 'var(--navy-dark)', border: '1px solid var(--navy-light)' }}>
              <h3 className="text-lg font-semibold text-[#EAECEF] mb-4">
                {language === 'zh' ? '交易提示词' : 'Trading Prompt'}
              </h3>
              <div className="space-y-4">
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">
                    {language === 'zh' ? '系统提示词模板' : 'System Prompt Template'}
                  </label>
                  <select
                    value={formData.system_prompt_template}
                    onChange={(e) => handleInputChange('system_prompt_template', e.target.value)}
                    className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors cursor-pointer"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)', color: '#EAECEF' }}
                  >
                    {allTemplates.map((template) => (
                      <option key={template.name} value={template.name}>
                        {template.name}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <div className="flex items-center gap-2 mb-2">
                    <label className="text-sm text-[#EAECEF]">
                    {language === 'zh' ? '自定义提示词' : 'Custom Prompt'}
                  </label>
                    <Tooltip content={getTooltipContent()}>
                      <Info className="w-4 h-4 cursor-help" style={{ color: 'var(--navy-light)' }} />
                    </Tooltip>
                  </div>
                  <textarea
                    value={formData.custom_prompt}
                    onChange={(e) => handleInputChange('custom_prompt', e.target.value)}
                    rows={6}
                    className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none font-mono text-sm transition-colors resize-none"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}
                    placeholder={getCustomPromptPlaceholder()}
                  />
                  {formData.override_base_prompt && (
                    <div className="mt-2 space-y-2">
                      <p className="text-xs" style={{ color: '#EAECEF' }}>
                        {language === 'zh' 
                          ? '💡 提示：在上方输入您的交易策略说明。系统会自动处理 JSON 格式要求，您只需专注于策略内容。'
                          : '💡 Tip: Write your trading strategy instructions above. The system will automatically handle JSON format requirements - you only need to focus on your strategy content.'}
                      </p>
                      <div className="text-xs p-3 rounded-lg" style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)' }}>
                        <p className="mb-2 font-semibold" style={{ color: '#EAECEF' }}>
                          {language === 'zh' ? '提示词示例（可复制修改）：' : 'Prompt Examples (copy and modify):'}
                        </p>
                        <pre className="text-xs whitespace-pre-wrap" style={{ color: '#EAECEF', fontFamily: 'inherit' }}>
                          {getPromptExamples()}
                        </pre>
                      </div>
                    </div>
                  )}
                </div>
                <div className="flex items-center gap-3">
                  <input
                    type="checkbox"
                    checked={formData.override_base_prompt}
                    onChange={(e) => handleInputChange('override_base_prompt', e.target.checked)}
                    className="w-4 h-4 rounded accent-[var(--green-primary)] cursor-pointer"
                      style={{ borderColor: 'var(--navy-light)' }}
                  />
                  <label className="text-sm text-[#EAECEF] cursor-pointer">
                    {language === 'zh' ? '覆盖基础提示词' : 'Override Base Prompt'}
                  </label>
                </div>
              </div>
            </div>
          </div>

          {/* Footer */}
          <div className="sticky bottom-0 flex items-center justify-end gap-3 p-6 border-t rounded-b-xl" style={{ borderColor: 'var(--navy-light)', background: 'var(--navy-dark)' }}>
            <button
              onClick={onClose}
              className="px-6 py-2.5 rounded-lg transition-all duration-200 font-medium"
              style={{ 
                background: 'var(--navy-dark)', 
                border: '1px solid var(--navy-light)',
                color: '#EAECEF'
              }}
            >
              {language === 'zh' ? '取消' : 'Cancel'}
            </button>
            <button
              onClick={handleSave}
              disabled={isSaving}
              className="px-8 py-2.5 bg-gradient-to-r from-[var(--green-primary)] to-[var(--green-dark)] rounded-lg hover:from-[var(--green-dark)] hover:to-[var(--green-primary)] transition-all duration-200 disabled:cursor-not-allowed font-medium shadow-lg disabled:opacity-50"
              style={isSaving ? { 
                background: 'var(--text-disabled)',
                backgroundImage: 'none'
              } : {}}
            >
              {isSaving
                ? language === 'zh' ? '保存中...' : 'Saving...'
                : language === 'zh' ? '保存' : 'Save'}
            </button>
          </div>
        </div>
      </div>

      {/* Prompt Template Modal */}
      {showTemplateModal && (
        <PromptTemplateModal
          isOpen={showTemplateModal}
          onClose={() => {
            setShowTemplateModal(false)
            setEditingTemplate(null)
          }}
          template={editingTemplate}
          onSave={async () => {
            // Reload templates
            try {
              const [systemTemplates, userTemplates] = await Promise.all([
                api.getPromptTemplates(),
                api.getUserPromptTemplates(),
              ])
              setPromptTemplates(systemTemplates.map((t) => ({ name: t })))
              setUserPromptTemplates(userTemplates)
            } catch (error) {
              console.error('Failed to reload templates:', error)
            }
          }}
        />
      )}
    </>
  )
}

