import { useState, useEffect, useCallback } from 'react'
import type { AIModel, Exchange, CreateTraderRequest, RunningTrader, Strategy, CreateStrategyRequest } from '../types'
import type { PromptTemplate } from '../types'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth, isFollower } from '../contexts/AuthContext'
import { t } from '../i18n/translations'
import { toast } from 'sonner'
import { Pencil, Plus, X as IconX, Edit2, Save, Info } from 'lucide-react'
import { httpClient } from '../lib/httpClient'
import { api } from '../lib/api'
import { PromptTemplateModal } from './PromptTemplateModal'
import { IndicatorEditor } from './traders/IndicatorEditor'
import { Tooltip } from './traders/Tooltip'
import useSWR from 'swr'

// 提取下划线后面的名称部分
function getShortName(fullName: string): string {
  const parts = fullName.split('_')
  return parts.length > 1 ? parts[parts.length - 1] : fullName
}

interface TraderConfigData {
  trader_id?: string
  trader_name: string
  ai_model: string
  exchange_id: string
  strategy_id?: string // Strategy ID (new version)
  btc_eth_leverage: number
  altcoin_leverage: number
  trading_symbols: string
  custom_prompt: string
  override_base_prompt: boolean
  system_prompt_template: string
  is_cross_margin: boolean
  use_coin_pool: boolean
  use_oi_top: boolean
  use_tradingview: boolean
  followed_trader_id?: string // 跟随的交易员ID（用于follower角色）
  initial_balance?: number // 可选：创建时不需要，编辑时使用
  scan_interval_minutes: number
  // Indicator configuration
  enable_raw_klines?: boolean
  enable_ema?: boolean
  enable_macd?: boolean
  enable_rsi?: boolean
  enable_atr?: boolean
  enable_volume?: boolean
  enable_oi?: boolean
  enable_funding?: boolean
  indicator_timeframe?: string
  quant_data_url?: string
}

interface TraderConfigModalProps {
  isOpen: boolean
  onClose: () => void
  traderData?: TraderConfigData | null
  isEditMode?: boolean
  availableModels?: AIModel[]
  availableExchanges?: Exchange[]
  onSave?: (data: CreateTraderRequest) => Promise<void>
}

export function TraderConfigModal({
  isOpen,
  onClose,
  traderData,
  isEditMode = false,
  availableModels = [],
  availableExchanges = [],
  onSave,
}: TraderConfigModalProps) {
  const { language } = useLanguage()
  const { user } = useAuth()
  const userIsFollower = isFollower(user)
  const [formData, setFormData] = useState<TraderConfigData>({
    trader_name: '',
    ai_model: '',
    exchange_id: '',
    btc_eth_leverage: 5,
    altcoin_leverage: 3,
    trading_symbols: '',
    custom_prompt: '',
    override_base_prompt: false,
    system_prompt_template: userIsFollower ? 'risk_management' : 'default',
    is_cross_margin: true,
    use_coin_pool: false,
    use_oi_top: false,
    use_tradingview: false,
    scan_interval_minutes: 3,
    // Indicator configuration defaults
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
  })
  const [isSaving, setIsSaving] = useState(false)
  const [availableCoins, setAvailableCoins] = useState<string[]>([])
  const [selectedCoins, setSelectedCoins] = useState<string[]>([])
  const [showCoinSelector, setShowCoinSelector] = useState(false)
  const [promptTemplates, setPromptTemplates] = useState<{ name: string }[]>([])
  const [userPromptTemplates, setUserPromptTemplates] = useState<PromptTemplate[]>([])
  const [isFetchingBalance, setIsFetchingBalance] = useState(false)
  const [balanceFetchError, setBalanceFetchError] = useState<string>('')
  const [showTemplateModal, setShowTemplateModal] = useState(false)
  const [editingTemplate, setEditingTemplate] = useState<PromptTemplate | null>(null)
  const [runningTraders, setRunningTraders] = useState<RunningTrader[]>([])
  const [selectedTraderToCopy, setSelectedTraderToCopy] = useState<string>('')
  const [loadingRunningTraders, setLoadingRunningTraders] = useState(false)
  const [selectedStrategyId, setSelectedStrategyId] = useState<string>('')
  const [showSaveAsStrategyModal, setShowSaveAsStrategyModal] = useState(false)

  // Load strategies for strategy selection
  const { data: strategies } = useSWR(
    isOpen ? 'strategies' : null,
    api.getStrategies,
    { revalidateOnFocus: false }
  )

  // Filter out prompt template names from AI models (they're not real AI models)
  const isPromptTemplateName = (name: string): boolean => {
    if (!name) return false
    const promptTemplateNames = [
      'risk_management',
      'risk-management',
      'riskmanagement',
      'default',
      'adaptive',
      'aggressive',
      'conservative',
      'scalping',
    ]
    const nameLower = name.toLowerCase().trim()
    return promptTemplateNames.some((templateName) => 
      nameLower === templateName.toLowerCase()
    )
  }

  // Filter models: exclude prompt template names, show real AI models
  const filteredModels = availableModels.filter((model) => {
    // Exclude models that are actually prompt template names
    if (isPromptTemplateName(model.id) || isPromptTemplateName(model.name || '')) {
      return false
    }
    return true
  })

  // Helper function to get default AI model for followers
  const getDefaultAIModel = (): string => {
    // Try to find deepseek or qwen first (common models)
    const preferredModel = filteredModels.find(
      (m) => (m.id || m.name || '').toLowerCase().includes('deepseek') ||
             (m.id || m.name || '').toLowerCase().includes('qwen')
    )
    if (preferredModel) return preferredModel.id
    
    // Fall back to first available model
    if (filteredModels.length > 0) return filteredModels[0].id
    
    return ''
  }

  // Auto-select default AI model and risk_management prompt template for followers when modal opens
  useEffect(() => {
    if (userIsFollower && !isEditMode && isOpen && !formData.ai_model && filteredModels.length > 0) {
      const defaultModel = getDefaultAIModel()
      if (defaultModel) {
        setFormData((prev) => ({
          ...prev,
          ai_model: defaultModel,
          // Auto-select risk_management as system prompt template for followers
          system_prompt_template: prev.system_prompt_template || 'risk_management',
        }))
      }
    }
  }, [userIsFollower, isEditMode, isOpen, filteredModels.length])

  // Ensure risk_management is set for followers when modal opens (create mode)
  useEffect(() => {
    if (userIsFollower && !isEditMode && isOpen) {
      setFormData((prev) => {
        // Always ensure risk_management for followers in create mode
        if (prev.system_prompt_template !== 'risk_management') {
          return {
            ...prev,
            system_prompt_template: 'risk_management'
          }
        }
        return prev
      })
    }
  }, [userIsFollower, isEditMode, isOpen])

  // Load default strategy config when creating new trader (not in edit mode)
  useEffect(() => {
    if (!isEditMode && isOpen && !formData.custom_prompt) {
      api
        .getDefaultStrategyConfig(language)
        .then((config) => {
          setFormData((prev) => ({
            ...prev,
            // Don't prefill custom_prompt - users should write their own or use examples
            // For followers, always preserve risk_management; for others use config or current value
            system_prompt_template: userIsFollower 
              ? 'risk_management' 
              : (config.prompt_template || prev.system_prompt_template),
          }))
        })
        .catch((err) => {
          console.error('Failed to load default strategy config:', err)
          // Don't show error to user, just use empty default
        })
    }
  }, [isEditMode, isOpen, language, userIsFollower])

  // Load running traders for followers only
  useEffect(() => {
    console.log('🔄 TraderConfigModal useEffect triggered:', {
      userIsFollower,
      isEditMode,
      isOpen,
      user: user ? { id: user.id, role: user.role } : null
    })
    
    if (userIsFollower && !isEditMode && isOpen) {
      console.log('✅ Conditions met, loading running traders for follower')
      setLoadingRunningTraders(true)
      api
        .getRunningTraders()
        .then((traders) => {
          console.log('📋 Loaded running traders:', traders.length, 'traders')
          setRunningTraders(traders)
          setLoadingRunningTraders(false)
          
          // Check if we should pre-select a trader from sessionStorage (from CompetitionPage)
          const copyTraderId = sessionStorage.getItem('copyTraderId')
          
          if (copyTraderId) {
            const trader = traders.find((t) => t.trader_id === copyTraderId)
            
            if (trader) {
              setSelectedTraderToCopy(copyTraderId)
              // Set the followed_trader_id directly
              setFormData((prev) => {
                const newData = {
                  ...prev,
                  followed_trader_id: copyTraderId,
                  trader_name: prev.trader_name || `${trader.trader_name} (Copy)`,
                  // Auto-select default risk management model and exchange if not already selected
                  ai_model: prev.ai_model || getDefaultAIModel(),
                  exchange_id: prev.exchange_id || (availableExchanges.length > 0 ? availableExchanges[0].id : ''),
                }
                return newData
              })
              toast.success('Trader selected. All settings will be copied when you save.')
              // Clear sessionStorage only after successful load
              sessionStorage.removeItem('copyTraderId')
            } else {
              // Try to get from public config if not in running traders
              api.getPublicTraderConfig(copyTraderId)
                .then((publicConfig) => {
                  setSelectedTraderToCopy(copyTraderId)
                  setFormData((prev) => {
                    const newData = {
                      ...prev,
                      followed_trader_id: copyTraderId,
                      trader_name: prev.trader_name || `${publicConfig.trader_name || 'Trader'} (Copy)`,
                      // Auto-select default risk management model and exchange if not already selected
                      ai_model: prev.ai_model || getDefaultAIModel(),
                      exchange_id: prev.exchange_id || (availableExchanges.length > 0 ? availableExchanges[0].id : ''),
                    }
                    return newData
                  })
                  toast.success('Trader selected. All settings will be copied when you save.')
                  // Clear sessionStorage only after successful load
                  sessionStorage.removeItem('copyTraderId')
                })
                .catch((err) => {
                  console.error('❌ Failed to get trader config for copy:', err)
                  toast.error('Failed to load trader configuration')
                  // Clear sessionStorage on error to prevent retry loops
                  sessionStorage.removeItem('copyTraderId')
                })
            }
          }
        })
        .catch((err) => {
          console.error('❌ Failed to load running traders:', err)
          setLoadingRunningTraders(false)
        })
    } else {
      console.log('⏭️ Skipping running traders load:', {
        reason: !userIsFollower ? 'not a follower' : isEditMode ? 'edit mode' : 'modal not open'
      })
    }
  }, [userIsFollower, isEditMode, isOpen])

  // Handle copying a trader
  const handleCopyTrader = async (traderId: string) => {
    if (!traderId) {
      setFormData((prev) => ({
        ...prev,
        followed_trader_id: '',
      }))
      return
    }

    try {
      // Find the trader in the running traders list
      const trader = runningTraders.find((t) => t.trader_id === traderId)
      
      if (!trader) {
        // If not in running traders list, try to get it from public config (for CompetitionPage)
        try {
          const publicConfig = await api.getPublicTraderConfig(traderId)
          setFormData((prev) => ({
            ...prev,
            followed_trader_id: traderId,
            trader_name: prev.trader_name || `${publicConfig.trader_name || 'Trader'} (Copy)`,
            // Auto-select default risk management model and exchange if not already selected
            ai_model: prev.ai_model || getDefaultAIModel(),
            exchange_id: prev.exchange_id || (availableExchanges.length > 0 ? availableExchanges[0].id : ''),
          }))
          toast.success('Trader selected. All settings will be copied when you save.')
          return
        } catch (err) {
          console.error('Failed to get trader config:', err)
          toast.error('Trader not found')
          return
        }
      }

      // Set the followed_trader_id - the backend will copy all settings when creating
      setFormData((prev) => {
        const newData = {
          ...prev,
          followed_trader_id: traderId,
          trader_name: prev.trader_name || `${trader.trader_name} (Copy)`,
          // Auto-select default AI model and exchange if not already selected
          ai_model: prev.ai_model || getDefaultAIModel(),
          exchange_id: prev.exchange_id || (availableExchanges.length > 0 ? availableExchanges[0].id : ''),
        }
        console.log('📝 DEBUG [TraderConfigModal]: Setting followed_trader_id:', {
          traderId,
          followed_trader_id: newData.followed_trader_id,
          fullData: newData
        })
        return newData
      })
      
      toast.success('Trader selected. All settings will be copied when you save.')
    } catch (error) {
      console.error('Failed to copy trader:', error)
      toast.error('Failed to copy trader settings')
    }
  }

  // Handle strategy selection
  const handleStrategyChange = async (strategyId: string) => {
    setSelectedStrategyId(strategyId)
    if (!strategyId) {
      setFormData((prev) => ({
        ...prev,
        strategy_id: undefined,
      }))
      return
    }

    try {
      const strategy = await api.getStrategy(strategyId)
      // Merge strategy config into form (form values take precedence)
      setFormData((prev) => ({
        ...prev,
        strategy_id: strategyId,
        // Only update if not already set
        system_prompt_template: prev.system_prompt_template || strategy.system_prompt_template,
        custom_prompt: prev.custom_prompt || strategy.custom_prompt,
        override_base_prompt: strategy.override_base_prompt,
        btc_eth_leverage: prev.btc_eth_leverage || strategy.btc_eth_leverage,
        altcoin_leverage: prev.altcoin_leverage || strategy.altcoin_leverage,
        trading_symbols: prev.trading_symbols || strategy.trading_symbols,
        is_cross_margin: strategy.is_cross_margin,
        use_coin_pool: strategy.use_coin_pool,
        use_oi_top: strategy.use_oi_top,
        use_tradingview: strategy.use_tradingview,
        enable_raw_klines: strategy.enable_raw_klines,
        enable_ema: strategy.enable_ema,
        enable_macd: strategy.enable_macd,
        enable_rsi: strategy.enable_rsi,
        enable_atr: strategy.enable_atr,
        enable_volume: strategy.enable_volume,
        enable_oi: strategy.enable_oi,
        enable_funding: strategy.enable_funding,
        indicator_timeframe: prev.indicator_timeframe || strategy.indicator_timeframe,
        quant_data_url: prev.quant_data_url || strategy.quant_data_url,
      }))
      toast.success(language === 'zh' ? '策略已加载' : 'Strategy loaded')
    } catch (error: any) {
      toast.error(error.message || (language === 'zh' ? '加载策略失败' : 'Failed to load strategy'))
      setSelectedStrategyId('')
    }
  }

  // Handle saving current config as strategy
  const handleSaveAsStrategy = async (strategyName: string, description?: string) => {
    try {
      const strategyData: CreateStrategyRequest = {
        name: strategyName,
        description: description || '',
        system_prompt_template: formData.system_prompt_template,
        custom_prompt: formData.custom_prompt,
        override_base_prompt: formData.override_base_prompt,
        btc_eth_leverage: formData.btc_eth_leverage,
        altcoin_leverage: formData.altcoin_leverage,
        trading_symbols: formData.trading_symbols,
        is_cross_margin: formData.is_cross_margin,
        use_coin_pool: formData.use_coin_pool,
        use_oi_top: formData.use_oi_top,
        use_tradingview: formData.use_tradingview,
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
      }
      const newStrategy = await api.createStrategy(strategyData)
      toast.success(language === 'zh' ? '策略已保存' : 'Strategy saved')
      setShowSaveAsStrategyModal(false)
      // Optionally select the newly created strategy
      setSelectedStrategyId(newStrategy.id)
      handleStrategyChange(newStrategy.id)
    } catch (error: any) {
      toast.error(error.message || (language === 'zh' ? '保存策略失败' : 'Failed to save strategy'))
    }
  }

  useEffect(() => {
    if (traderData) {
      console.log('🔍 DEBUG [TraderConfigModal]: Loading traderData:', {
        system_prompt_template: traderData.system_prompt_template,
        trader_id: traderData.trader_id,
        trader_name: traderData.trader_name,
        strategy_id: traderData.strategy_id,
        isEditMode: isEditMode
      })
      
      setFormData({
        ...traderData,
        // Ensure system_prompt_template has a default value if empty/undefined
        system_prompt_template: traderData.system_prompt_template || 'default',
        // Ensure indicator config has defaults if not present
        enable_raw_klines: traderData.enable_raw_klines ?? true,
        enable_ema: traderData.enable_ema ?? false,
        enable_macd: traderData.enable_macd ?? false,
        enable_rsi: traderData.enable_rsi ?? false,
        enable_atr: traderData.enable_atr ?? false,
        enable_volume: traderData.enable_volume ?? true,
        enable_oi: traderData.enable_oi ?? true,
        enable_funding: traderData.enable_funding ?? true,
        indicator_timeframe: traderData.indicator_timeframe || '3m',
        quant_data_url: traderData.quant_data_url || '',
      })
      
      console.log('🔍 DEBUG [TraderConfigModal]: Set formData with system_prompt_template:', traderData.system_prompt_template || 'default')
      
      // 设置已选择的币种
      if (traderData.trading_symbols) {
        const coins = traderData.trading_symbols
          .split(',')
          .map((s) => s.trim())
          .filter((s) => s)
        setSelectedCoins(coins)
      }
    } else if (!isEditMode) {
      // For followers, auto-select risk_management as system prompt template
      const defaultAIModel = filteredModels.length > 0 
        ? getDefaultAIModel()
        : (availableModels[0]?.id || '')
      
      // Preserve followed_trader_id if it was already set (from copy trader flow)
      // Use functional update to avoid clearing it when dependencies change
      setFormData((prev) => {
        // Only reset if followed_trader_id is not already set
        if (prev.followed_trader_id) {
          console.log('🔍 DEBUG [TraderConfigModal]: Preserving followed_trader_id:', prev.followed_trader_id)
          // Still ensure risk_management for followers even when preserving followed_trader_id
          if (userIsFollower && prev.system_prompt_template !== 'risk_management') {
            return {
              ...prev,
              system_prompt_template: 'risk_management'
            }
          }
          return prev // Don't reset if followed_trader_id is already set
        }
        
        // Otherwise, reset to defaults
        return {
          trader_name: '',
          ai_model: defaultAIModel,
          exchange_id: availableExchanges[0]?.id || '',
          btc_eth_leverage: 5,
          // Indicator configuration defaults
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
          altcoin_leverage: 3,
          trading_symbols: '',
          custom_prompt: '',
          override_base_prompt: false,
          system_prompt_template: userIsFollower ? 'risk_management' : 'default',
          is_cross_margin: true,
          use_coin_pool: false,
          use_oi_top: false,
          use_tradingview: false,
          followed_trader_id: '',
          initial_balance: 1000,
          scan_interval_minutes: 3,
          strategy_id: undefined,
        }
      })
      setSelectedStrategyId('')
    }
  }, [traderData, isEditMode, availableModels, availableExchanges, userIsFollower, filteredModels.length])

  // 获取系统配置中的币种列表
  useEffect(() => {
    const fetchConfig = async () => {
      try {
        const result = await httpClient.get<{ default_coins?: string[] }>(
          '/api/config'
        )
        if (result.success && result.data?.default_coins) {
          setAvailableCoins(result.data.default_coins)
        } else {
          // 使用默认币种列表
          setAvailableCoins([
            'BTCUSDT',
            'ETHUSDT',
            'SOLUSDT',
            'BNBUSDT',
            'XRPUSDT',
            'DOGEUSDT',
            'ADAUSDT',
          ])
        }
      } catch (error) {
        console.error('Failed to fetch config:', error)
        // 使用默认币种列表
        setAvailableCoins([
          'BTCUSDT',
          'ETHUSDT',
          'SOLUSDT',
          'BNBUSDT',
          'XRPUSDT',
          'DOGEUSDT',
          'ADAUSDT',
        ])
      }
    }
    fetchConfig()
  }, [])

  // 获取系统提示词模板列表和用户模板
  useEffect(() => {
    const fetchPromptTemplates = async () => {
      try {
        // 获取系统模板（公开）
        const publicResult = await httpClient.get<{ templates?: { name: string }[] }>(
          '/api/prompt-templates'
        )
        if (publicResult.success && publicResult.data?.templates) {
          setPromptTemplates(publicResult.data.templates)
        } else {
          setPromptTemplates([{ name: 'default' }, { name: 'aggressive' }])
        }

        // 获取用户模板（需要认证）
        try {
          const userTemplates = await api.getUserPromptTemplates()
          setUserPromptTemplates(userTemplates)
        } catch (error) {
          // 如果未登录或获取失败，忽略错误
          console.log('Failed to fetch user templates:', error)
        }
      } catch (error) {
        console.error('Failed to fetch prompt templates:', error)
        setPromptTemplates([{ name: 'default' }, { name: 'aggressive' }])
      }
    }
    fetchPromptTemplates()
  }, [])

  // Memoize onChange callback to prevent infinite loops in IndicatorEditor
  // IMPORTANT: This hook must be called BEFORE any conditional returns
  const handleIndicatorChange = useCallback((config: {
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
  }) => {
    setFormData((prev) => ({
      ...prev,
      enable_raw_klines: config.enable_raw_klines,
      enable_ema: config.enable_ema,
      enable_macd: config.enable_macd,
      enable_rsi: config.enable_rsi,
      enable_atr: config.enable_atr,
      enable_volume: config.enable_volume,
      enable_oi: config.enable_oi,
      enable_funding: config.enable_funding,
      indicator_timeframe: config.indicator_timeframe,
      quant_data_url: config.quant_data_url,
    }))
  }, []) // Empty deps since we use functional setFormData

  // Early return AFTER all hooks
  if (!isOpen) return null

  const handleInputChange = (field: keyof TraderConfigData, value: any) => {
    setFormData((prev) => ({ ...prev, [field]: value }))

    // 如果是直接编辑trading_symbols，同步更新selectedCoins
    if (field === 'trading_symbols') {
      const coins = value
        .split(',')
        .map((s: string) => s.trim())
        .filter((s: string) => s)
      setSelectedCoins(coins)
    }
  }

  const handleCoinToggle = (coin: string) => {
    setSelectedCoins((prev) => {
      const newCoins = prev.includes(coin)
        ? prev.filter((c) => c !== coin)
        : [...prev, coin]

      // 同时更新 formData.trading_symbols
      const symbolsString = newCoins.join(',')
      setFormData((current) => ({ ...current, trading_symbols: symbolsString }))

      return newCoins
    })
  }

  const handleFetchCurrentBalance = async () => {
    if (!isEditMode || !traderData?.trader_id) {
      setBalanceFetchError(t('editModeOnlyError', language))
      return
    }

    setIsFetchingBalance(true)
    setBalanceFetchError('')

    try {
      const result = await httpClient.get<{
        total_equity?: number
        balance?: number
      }>(`/api/account?trader_id=${traderData.trader_id}`)

      if (result.success && result.data) {
        // total_equity = 当前账户净值（包含未实现盈亏）
        // 这应该作为新的初始余额
        const currentBalance =
          result.data.total_equity || result.data.balance || 0

        setFormData((prev) => ({ ...prev, initial_balance: currentBalance }))
        toast.success(t('balanceFetched', language))
      } else {
        throw new Error(result.message || t('balanceFetchFailed', language))
      }
    } catch (error) {
      console.error('获取余额失败:', error)
      setBalanceFetchError(t('balanceFetchError', language))
      // Note: Network/system errors already shown via toast by httpClient
    } finally {
      setIsFetchingBalance(false)
    }
  }

  const handleSave = async () => {
    if (!onSave) return

    setIsSaving(true)
    try {
      console.log('🔍 DEBUG [TraderConfigModal]: Form data before save:', {
        system_prompt_template: formData.system_prompt_template,
        followed_trader_id: formData.followed_trader_id,
        allFormData: formData
      })
      
      const saveData: CreateTraderRequest = {
        name: formData.trader_name,
        ai_model_id: formData.ai_model,
        exchange_id: formData.exchange_id,
        strategy_id: formData.strategy_id,
        btc_eth_leverage: formData.btc_eth_leverage,
        altcoin_leverage: formData.altcoin_leverage,
        trading_symbols: formData.trading_symbols,
        custom_prompt: formData.custom_prompt,
        override_base_prompt: formData.override_base_prompt,
        system_prompt_template: formData.system_prompt_template,
        is_cross_margin: formData.is_cross_margin,
        use_coin_pool: formData.use_coin_pool,
        use_oi_top: formData.use_oi_top,
        use_tradingview: formData.use_tradingview,
        followed_trader_id: formData.followed_trader_id,
        scan_interval_minutes: formData.scan_interval_minutes,
        // Indicator configuration
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
      }

      // 只在编辑模式时包含initial_balance（用于手动更新）
      if (isEditMode && formData.initial_balance !== undefined) {
        saveData.initial_balance = formData.initial_balance
      }

      console.log('🔍 DEBUG [TraderConfigModal]: Save data being sent:', {
        system_prompt_template: saveData.system_prompt_template,
        followed_trader_id: saveData.followed_trader_id,
        isEditMode: isEditMode,
        fullSaveData: saveData
      })

      await toast.promise(onSave(saveData), {
        loading: t('savingTrader', language),
        success: t('traderSaved', language),
        error: t('traderSaveFailed', language),
      })
      onClose()
    } catch (error) {
      console.error('保存失败:', error)
    } finally {
      setIsSaving(false)
    }
  }

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
    <div className="fixed inset-0 z-50 flex items-center justify-center backdrop-blur-sm p-4 overflow-y-auto" style={{ background: 'rgba(0, 31, 63, 0.5)' }}>
      <div
        className="border rounded-xl shadow-2xl max-w-3xl w-full my-8"
        style={{ background: 'var(--navy-dark)', borderColor: 'var(--panel-border)', maxHeight: 'calc(100vh - 4rem)' }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b sticky top-0 z-10 rounded-t-xl"
        style={{ borderColor: 'var(--panel-border)', background: 'var(--navy-dark)' }}>
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-[var(--green-primary)] to-[var(--green-dark)] flex items-center justify-center" style={{ color: 'var(--navy-primary)' }}>
              {isEditMode ? (
                <Pencil className="w-5 h-5" />
              ) : (
                <Plus className="w-5 h-5" />
              )}
            </div>
            <div>
              <h2 className="text-xl font-bold text-[#EAECEF]">
                {isEditMode ? t('editTrader', language) : t('createTrader', language)}
              </h2>
              <p className="text-sm text-[#848E9C] mt-1">
                {isEditMode ? t('editTraderSubtitle', language) : t('createTraderSubtitle', language)}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="w-8 h-8 rounded-lg text-[#848E9C] hover:text-[#EAECEF] transition-colors flex items-center justify-center"
            style={{ '--hover-bg': 'var(--panel-border)' } as React.CSSProperties}
          >
            <IconX className="w-4 h-4" />
          </button>
        </div>

        {/* Content */}
        <div
          className="p-6 space-y-8 overflow-y-auto"
          style={{ maxHeight: 'calc(100vh - 16rem)' }}
        >
          {/* Copy Trader Section (Followers only, create mode) */}
          {userIsFollower && !isEditMode && (
            <div className="rounded-lg p-5" style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}>
              <h3 className="text-lg font-semibold text-[#EAECEF] mb-4 flex items-center gap-2">
                📋 Copy Trader
              </h3>
              <div>
                <label className="text-sm text-[#EAECEF] block mb-2">
                  Select a running trader to copy
                </label>
                {loadingRunningTraders ? (
                  <div className="text-sm text-[#848E9C]">Loading traders...</div>
                ) : runningTraders.length === 0 ? (
                  <div className="text-sm text-[#848E9C]">
                    No running traders available to copy
                  </div>
                ) : (
                  <select
                    value={selectedTraderToCopy}
                    onChange={(e) => {
                      setSelectedTraderToCopy(e.target.value)
                      if (e.target.value) {
                        handleCopyTrader(e.target.value)
                      }
                    }}
                    className="w-full px-3 py-2 rounded text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
                  >
                    <option value="">-- Select a trader to copy --</option>
                    {runningTraders.map((trader) => (
                      <option key={trader.trader_id} value={trader.trader_id}>
                        {trader.trader_name} ({trader.user_email})
                      </option>
                    ))}
                  </select>
                )}
                <div className="text-xs text-[#848E9C] mt-2">
                  {userIsFollower 
                    ? 'Select a running trader from any user to copy all their settings. You can then customize the exchange and AI risk management model.'
                    : 'Select a running trader from any user to copy all their settings. You can customize the exchange, AI model, and other configurations.'}
                </div>
              </div>
            </div>
          )}

          {/* Basic Info */}
          <div className="rounded-lg p-5" style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}>
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-5 flex items-center gap-2">
              🤖 {t('basicConfig', language)}
            </h3>
            <div className="space-y-4">
              <div>
                <label className="text-sm text-[#EAECEF] block mb-2">
                  {t('traderNameLabel', language)}
                </label>
                <input
                  type="text"
                  value={formData.trader_name}
                  onChange={(e) =>
                    handleInputChange('trader_name', e.target.value)
                  }
                  className="w-full px-3 py-2 rounded text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none"
                  style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
                  placeholder={t('traderNamePlaceholder', language)}
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <div>
                    <label className="text-sm text-[#EAECEF] block mb-2">
                      {t('aiModelLabel', language)}
                    </label>
                    {filteredModels.length === 0 ? (
                      <div className="text-sm text-[#848E9C] p-2">
                        No AI models available. Please configure an AI model.
                      </div>
                    ) : (
                      <select
                        value={formData.ai_model}
                        onChange={(e) =>
                          handleInputChange('ai_model', e.target.value)
                        }
                        className="w-full px-3 py-2 rounded text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)', color: '#EAECEF' }}
                      >
                        {filteredModels.map((model) => (
                          <option key={model.id} value={model.id}>
                            {getShortName(model.name || model.id).toUpperCase()}
                          </option>
                        ))}
                      </select>
                    )}
                  </div>
                </div>
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">
                    {t('exchangeLabel', language)}
                  </label>
                  <select
                    value={formData.exchange_id}
                    onChange={(e) =>
                      handleInputChange('exchange_id', e.target.value)
                    }
                    className="w-full px-3 py-2 rounded text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)', color: '#EAECEF' }}
                  >
                    {availableExchanges.map((exchange) => (
                      <option key={exchange.id} value={exchange.id}>
                        {getShortName(
                          exchange.name || exchange.id
                        ).toUpperCase()}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            </div>
          </div>

          {/* Trading Configuration */}
          <div className="rounded-lg p-5" style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}>
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-5 flex items-center gap-2">
              ⚖️ {t('tradingConfig', language)}
            </h3>
            <div className="space-y-4">
              {/* 第一行：保证金模式和初始余额 */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">
                    {t('marginModeLabel', language)}
                  </label>
                  <div className="flex gap-2">
                    <button
                      type="button"
                      onClick={() => handleInputChange('is_cross_margin', true)}
                        className={`flex-1 px-3 py-2 rounded text-sm ${
                        formData.is_cross_margin
                          ? 'bg-[var(--green-primary)]'
                          : 'text-[#848E9C]'
                      }`}
                        style={formData.is_cross_margin 
                          ? { color: 'var(--navy-primary)' }
                          : { background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
                    >
                      {t('crossMarginMode', language)}
                    </button>
                    <button
                      type="button"
                      onClick={() =>
                        handleInputChange('is_cross_margin', false)
                      }
                      className={`flex-1 px-3 py-2 rounded text-sm ${
                        !formData.is_cross_margin
                          ? 'bg-[var(--green-primary)]'
                          : 'text-[#848E9C]'
                      }`}
                      style={!formData.is_cross_margin 
                        ? { color: 'var(--navy-primary)' }
                        : { background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
                    >
                      {t('isolatedMarginMode', language)}
                    </button>
                  </div>
                </div>
                {isEditMode && (
                  <div>
                    <div className="flex items-center justify-between mb-2">
                      <label className="text-sm text-[#EAECEF]">
                        {t('initialBalanceEditLabel', language)}
                      </label>
                      <button
                        type="button"
                        onClick={handleFetchCurrentBalance}
                        disabled={isFetchingBalance}
                        className="px-3 py-1 text-xs bg-[var(--green-primary)] rounded hover:bg-[var(--green-dark)] transition-colors disabled:bg-[#848E9C] disabled:cursor-not-allowed"
                        style={{ color: 'var(--navy-primary)' }}
                      >
                        {isFetchingBalance ? t('fetchingBalance', language) : t('fetchCurrentBalance', language)}
                      </button>
                    </div>
                    <input
                      type="number"
                      value={formData.initial_balance || 0}
                      onChange={(e) =>
                        handleInputChange(
                          'initial_balance',
                          Number(e.target.value)
                        )
                      }
                      onBlur={(e) => {
                        // Force minimum value on blur
                        const value = Number(e.target.value)
                        if (value < 100) {
                          handleInputChange('initial_balance', 100)
                        }
                      }}
                      className="w-full px-3 py-2 rounded text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
                      min="100"
                      step="0.01"
                    />
                    <p className="text-xs text-[#848E9C] mt-1">
                      {t('initialBalanceEditNote', language)}
                    </p>
                    {balanceFetchError && (
                      <p className="text-xs text-red-500 mt-1">
                        {balanceFetchError}
                      </p>
                    )}
                  </div>
                )}
                {!isEditMode && (
                  <div>
                    <label className="text-sm text-[#EAECEF] mb-2 block">
                      {t('initialBalanceLabel', language)}
                    </label>
                    <div className="w-full px-3 py-2 rounded text-[#848E9C] flex items-center gap-2"
                    style={{ background: 'var(--navy-dark)', border: '1px solid var(--panel-border)' }}>
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        className="w-4 h-4 text-[var(--green-primary)]"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      >
                        <circle cx="12" cy="12" r="10" />
                        <line x1="12" x2="12" y1="8" y2="12" />
                        <line x1="12" x2="12.01" y1="16" y2="16" />
                      </svg>
                      <span className="text-sm">
                        {t('initialBalanceAutoNote', language)}
                      </span>
                    </div>
                  </div>
                )}
              </div>

              {/* 第二行：AI 扫描决策间隔 */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">
                    {t('aiScanInterval', language)}
                  </label>
                  <input
                    type="number"
                    value={formData.scan_interval_minutes}
                    onChange={(e) => {
                      const parsedValue = Number(e.target.value)
                      const safeValue = Number.isFinite(parsedValue)
                        ? Math.max(3, parsedValue)
                        : 3
                      handleInputChange('scan_interval_minutes', safeValue)
                    }}
                    className="w-full px-3 py-2 rounded text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
                    min="3"
                    max="60"
                    step="1"
                  />
                  <p className="text-xs text-gray-500 mt-1">
                    {t('scanIntervalRecommend', language)}
                  </p>
                </div>
                <div></div>
              </div>

              {/* 第三行：杠杆设置 */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">
                    {t('btcEthLeverageLabel', language)}
                  </label>
                  <input
                    type="number"
                    value={formData.btc_eth_leverage}
                    onChange={(e) =>
                      handleInputChange(
                        'btc_eth_leverage',
                        Number(e.target.value)
                      )
                    }
                    className="w-full px-3 py-2 rounded text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
                    min="1"
                    max="125"
                  />
                </div>
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">
                    {t('altcoinLeverageLabel', language)}
                  </label>
                  <input
                    type="number"
                    value={formData.altcoin_leverage}
                    onChange={(e) =>
                      handleInputChange(
                        'altcoin_leverage',
                        Number(e.target.value)
                      )
                    }
                    className="w-full px-3 py-2 rounded text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
                    min="1"
                    max="75"
                  />
                </div>
              </div>

              {/* 第三行：交易币种 */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-sm text-[#EAECEF]">
                    {t('tradingSymbolsLabel', language)}
                  </label>
                  <button
                    type="button"
                    onClick={() => setShowCoinSelector(!showCoinSelector)}
                    className="px-3 py-1 text-xs bg-[var(--green-primary)] rounded hover:bg-[var(--green-dark)] transition-colors"
                    style={{ color: 'var(--navy-primary)' }}
                  >
                    {showCoinSelector ? t('collapseSelect', language) : t('quickSelect', language)}
                  </button>
                </div>
                <input
                  type="text"
                  value={formData.trading_symbols}
                  onChange={(e) =>
                    handleInputChange('trading_symbols', e.target.value)
                  }
                  className="w-full px-3 py-2 rounded text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none"
                  style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
                  placeholder={t('tradingSymbolsExample', language)}
                />

                {/* 币种选择器 */}
                {showCoinSelector && (
                  <div className="mt-3 p-3 rounded" style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}>
                    <div className="text-xs text-[#848E9C] mb-2">
                      {t('coinSelectorTitle', language)}
                    </div>
                    <div className="flex flex-wrap gap-2">
                      {availableCoins.map((coin) => (
                        <button
                          key={coin}
                          type="button"
                          onClick={() => handleCoinToggle(coin)}
                          className={`px-2 py-1 text-xs rounded transition-colors ${
                            selectedCoins.includes(coin)
                              ? 'bg-[var(--green-primary)]'
                              : 'text-[#848E9C] hover:border-[var(--green-primary)]'
                          }`}
                          style={selectedCoins.includes(coin) ? { color: 'var(--navy-primary)' } : undefined}
                        >
                          {coin.replace('USDT', '')}
                        </button>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* Strategy Selection - Only show for non-followers */}
          {!userIsFollower && (
            <div className="rounded-lg p-5" style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}>
              <h3 className="text-lg font-semibold text-[#EAECEF] mb-4 flex items-center gap-2">
                🎯 {language === 'zh' ? '策略选择' : 'Strategy Selection'}
              </h3>
              <div className="space-y-3">
                <div>
                  <label className="text-sm text-[#EAECEF] block mb-2">
                    {language === 'zh' ? '选择策略（可选）' : 'Select Strategy (Optional)'}
                  </label>
                  <select
                    value={selectedStrategyId}
                    onChange={(e) => handleStrategyChange(e.target.value)}
                    className="w-full px-3 py-2 rounded text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)', color: '#EAECEF' }}
                  >
                    <option value="">{language === 'zh' ? '-- 不使用策略（自定义配置）--' : '-- None (Use Custom Config) --'}</option>
                    {strategies?.map((strategy) => (
                      <option key={strategy.id} value={strategy.id}>
                        {strategy.name}
                      </option>
                    ))}
                  </select>
                  <div className="text-xs text-[#848E9C] mt-2">
                    {language === 'zh' 
                      ? '选择一个策略将自动填充配置。您仍可以覆盖任何设置。' 
                      : 'Selecting a strategy will auto-fill configuration. You can still override any settings.'}
                  </div>
                </div>
                <div>
                  <button
                    type="button"
                    onClick={() => setShowSaveAsStrategyModal(true)}
                    className="px-4 py-2 text-sm bg-[var(--green-primary)] rounded hover:bg-[var(--green-dark)] transition-colors flex items-center gap-2"
                    style={{ color: 'var(--navy-primary)' }}
                  >
                    <Save className="w-4 h-4" />
                    {language === 'zh' ? '保存为策略' : 'Save as Strategy'}
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Signal Sources - Only show for non-followers */}
          {!userIsFollower && (
            <div className="rounded-lg p-5" style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}>
              <h3 className="text-lg font-semibold text-[#EAECEF] mb-5 flex items-center gap-2">
                📡 {t('signalSourceConfigSection', language)}
              </h3>
              <div className="grid grid-cols-2 gap-4">
                <div className="flex items-center gap-3">
                  <input
                    type="checkbox"
                    checked={formData.use_coin_pool}
                    onChange={(e) =>
                      handleInputChange('use_coin_pool', e.target.checked)
                    }
                    className="w-4 h-4"
                  />
                  <label className="text-sm text-[#EAECEF]">
                    {t('useCoinPoolSignal', language)}
                  </label>
                </div>
                <div className="flex items-center gap-3">
                  <input
                    type="checkbox"
                    checked={formData.use_oi_top}
                    onChange={(e) =>
                      handleInputChange('use_oi_top', e.target.checked)
                    }
                    className="w-4 h-4"
                  />
                  <label className="text-sm text-[#EAECEF]">
                    {t('useOITopSignal', language)}
                  </label>
                </div>
                <div className="flex items-center gap-3">
                  <input
                    type="checkbox"
                    checked={formData.use_tradingview}
                    onChange={(e) =>
                      handleInputChange('use_tradingview', e.target.checked)
                    }
                    className="w-4 h-4"
                  />
                  <label className="text-sm text-[#EAECEF]">
                    {t('useTradingViewSignal', language)}
                  </label>
                </div>
              </div>
            </div>
          )}

          {/* Indicator Configuration */}
          <div className="rounded-lg p-5" style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}>
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

          {/* Trading Prompt - Show for all users (including followers) */}
          <div className="rounded-lg p-5" style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}>
            <h3 className="text-lg font-semibold text-[#EAECEF] mb-5 flex items-center gap-2">
              💬 {t('tradingPromptSection', language)}
            </h3>
            <div className="space-y-4">
              {/* 系统提示词模板选择 */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-sm text-[#EAECEF]">
                    {t('systemPromptTemplate', language)}
                    {userIsFollower && (
                      <span className="text-xs text-[var(--green-primary)] ml-2">
                        (Risk Management Recommended)
                      </span>
                    )}
                  </label>
                  <button
                    type="button"
                    onClick={() => {
                      setEditingTemplate(null)
                      setShowTemplateModal(true)
                    }}
                    className="px-3 py-1 text-xs bg-[var(--green-primary)] rounded hover:bg-[var(--green-dark)] transition-colors flex items-center gap-1"
                    style={{ color: 'var(--navy-primary)' }}
                  >
                    <Plus className="w-3 h-3" />
                    {t('createTemplate', language) || 'Create Template'}
                  </button>
                </div>
                <div className="flex gap-2">
                  <select
                    value={formData.system_prompt_template}
                    onChange={(e) => {
                      console.log('🔍 DEBUG [TraderConfigModal]: System prompt template changed:', {
                        oldValue: formData.system_prompt_template,
                        newValue: e.target.value
                      })
                      handleInputChange('system_prompt_template', e.target.value)
                    }}
                    className="flex-1 px-3 py-2 rounded text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)', color: '#EAECEF' }}
                  >
                    {promptTemplates.map((template) => {
                      const getTemplateName = (name: string) => {
                        const keyMap: Record<string, string> = {
                          default: 'promptTemplateDefault',
                          adaptive: 'promptTemplateAdaptive',
                          adaptive_relaxed: 'promptTemplateAdaptiveRelaxed',
                          Hansen: 'promptTemplateHansen',
                          nof1: 'promptTemplateNof1',
                          taro_long_prompts: 'promptTemplateTaroLong',
                          risk_management: 'Risk Management',
                          'risk-management': 'Risk Management',
                        }
                        const key = keyMap[name]
                        if (key && key !== 'Risk Management') {
                          return t(key, language)
                        }
                        if (name?.toLowerCase().includes('risk')) {
                          return 'Risk Management'
                        }
                        return name.charAt(0).toUpperCase() + name.slice(1)
                      }

                      return (
                        <option key={template.name} value={template.name}>
                          {getTemplateName(template.name)}
                        </option>
                      )
                    })}
                    {/* Ensure risk_management is available even if not in promptTemplates */}
                    {!promptTemplates.some(t => t.name === 'risk_management' || t.name === 'risk-management') && (
                      <option value="risk_management">Risk Management</option>
                    )}
                    {userPromptTemplates
                      .filter((template: PromptTemplate) => {
                        // Filter out system templates that already exist in promptTemplates to avoid duplicates
                        if (template.is_system) {
                          const normalizedName = template.name.toLowerCase().replace(/[_-]/g, '')
                          return !promptTemplates.some(t => {
                            const tNormalized = t.name.toLowerCase().replace(/[_-]/g, '')
                            return tNormalized === normalizedName
                          })
                        }
                        // Always include user-created templates
                        return true
                      })
                      .map((template: PromptTemplate) => (
                        <option key={template.id} value={template.id}>
                          {template.name} {template.is_system ? '(System)' : '(User)'}
                        </option>
                      ))}
                  </select>
                  {(() => {
                    const selectedTemplate = userPromptTemplates.find(
                      (t: PromptTemplate) => t.id === formData.system_prompt_template
                    )
                    if (selectedTemplate && !selectedTemplate.is_system) {
                      return (
                        <button
                          type="button"
                          onClick={() => {
                            setEditingTemplate(selectedTemplate)
                            setShowTemplateModal(true)
                          }}
                          className="px-3 py-2 text-[#EAECEF] rounded transition-colors flex items-center gap-1"
                          style={{ background: 'var(--panel-border)' }}
                          title={t('editTemplate', language) || 'Edit Template'}
                        >
                          <Edit2 className="w-4 h-4" />
                        </button>
                      )
                    }
                    return null
                  })()}
                </div>

                {/* 動態描述區域 */}
                <div
                  className="mt-2 p-3 rounded"
                  style={{
                    background: 'rgba(0, 255, 127, 0.05)',
                    border: '1px solid rgba(0, 255, 127, 0.15)',
                  }}
                >
                  <div
                    className="text-xs font-semibold mb-1"
                    style={{ color: 'var(--green-primary)' }}
                  >
                    {(() => {
                      const titleKeyMap: Record<string, string> = {
                        default: 'promptDescDefault',
                        adaptive: 'promptDescAdaptive',
                        adaptive_relaxed: 'promptDescAdaptiveRelaxed',
                        Hansen: 'promptDescHansen',
                        nof1: 'promptDescNof1',
                        taro_long_prompts: 'promptDescTaroLong',
                        risk_management: 'Risk Management',
                        'risk-management': 'Risk Management',
                      }
                      const key = titleKeyMap[formData.system_prompt_template]
                      if (key && key !== 'Risk Management') {
                        return t(key, language)
                      }
                      if (formData.system_prompt_template?.toLowerCase().includes('risk')) {
                        return 'Risk Management'
                      }
                      return t('promptDescDefault', language)
                    })()}
                  </div>
                  <div className="text-xs" style={{ color: '#848E9C' }}>
                    {(() => {
                      const contentKeyMap: Record<string, string> = {
                        default: 'promptDescDefaultContent',
                        adaptive: 'promptDescAdaptiveContent',
                        adaptive_relaxed: 'promptDescAdaptiveRelaxedContent',
                        Hansen: 'promptDescHansenContent',
                        nof1: 'promptDescNof1Content',
                        taro_long_prompts: 'promptDescTaroLongContent',
                        risk_management: 'Optimized for follower traders to manage risk and position sizing when copying trades.',
                        'risk-management': 'Optimized for follower traders to manage risk and position sizing when copying trades.',
                      }
                      const key = contentKeyMap[formData.system_prompt_template]
                      if (key && !key.includes('Optimized')) {
                        return t(key, language)
                      }
                      if (formData.system_prompt_template?.toLowerCase().includes('risk')) {
                        return 'Optimized for follower traders to manage risk and position sizing when copying trades.'
                      }
                      return t('promptDescDefaultContent', language)
                    })()}
                  </div>
                </div>
                <p className="text-xs text-[#848E9C] mt-1">
                  {t('promptTemplateDescription', language)}
                </p>
              </div>

              <div className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={formData.override_base_prompt}
                  onChange={(e) =>
                    handleInputChange('override_base_prompt', e.target.checked)
                  }
                  className="w-4 h-4"
                />
                <label className="text-sm text-[#EAECEF]">{t('overrideBasePrompt', language)}</label>
                <span className="text-xs text-[var(--green-primary)] inline-flex items-center gap-1">
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    className="w-3.5 h-3.5"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z" />
                    <line x1="12" x2="12" y1="9" y2="13" />
                    <line x1="12" x2="12.01" y1="17" y2="17" />
                  </svg>{' '}
                  {t('overrideBasePromptWarning', language)}
                </span>
              </div>
              <div>
                <div className="flex items-center gap-2 mb-2">
                  <label className="text-sm text-[#EAECEF]">
                  {formData.override_base_prompt
                    ? t('customPromptLabel', language)
                    : t('appendPromptLabel', language)}
                </label>
                  {formData.override_base_prompt && (
                    <Tooltip content={getTooltipContent()}>
                      <Info className="w-4 h-4 cursor-help" style={{ color: 'var(--navy-light)' }} />
                    </Tooltip>
                  )}
                </div>
                <div className="space-y-2">
                  <textarea
                    value={formData.custom_prompt}
                    onChange={(e) =>
                      handleInputChange('custom_prompt', e.target.value)
                    }
                    className="w-full px-3 py-2 rounded text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none h-48 resize-y"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
                    placeholder=""
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
                  {formData.custom_prompt && (
                    <div className="flex items-center justify-between">
                      <span className="text-xs text-[#848E9C]">
                        {formData.custom_prompt.length} {t('characters', language) || 'characters'}
                      </span>
                      <button
                        type="button"
                        onClick={async () => {
                          if (!formData.custom_prompt.trim()) {
                            toast.error(t('templateContentRequired', language) || 'Template content is required')
                            return
                          }
                          try {
                            const template = await api.createPromptTemplate({
                              name: `Custom ${new Date().toLocaleString()}`,
                              content: formData.custom_prompt,
                            })
                            toast.success(t('templateCreated', language) || 'Template created')
                            // Refresh templates
                            const userTemplates = await api.getUserPromptTemplates()
                            setUserPromptTemplates(userTemplates)
                            // Optionally select the new template
                            handleInputChange('system_prompt_template', template.id)
                          } catch (error) {
                            console.error('Failed to save as template:', error)
                          }
                        }}
                        className="px-3 py-1 text-xs bg-[var(--green-primary)] rounded hover:bg-[var(--green-dark)] transition-colors flex items-center gap-1"
                    style={{ color: 'var(--navy-primary)' }}
                      >
                        <Save className="w-3 h-3" />
                        {t('saveAsTemplate', language) || 'Save as Template'}
                      </button>
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>

        </div>

        {/* Validation Messages */}
        {(!formData.trader_name || !formData.ai_model || !formData.exchange_id) && (
          <div className="px-6 pb-2">
            <div className="text-xs text-[var(--green-primary)] flex items-center gap-2 rounded-lg p-3"
            style={{ background: 'var(--navy-dark)', border: '1px solid rgba(0, 204, 102, 0.3)' }}>
              <svg className="w-4 h-4 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clipRule="evenodd" />
              </svg>
              <span>
                Please fill in all required fields:{' '}
                {!formData.trader_name && <span className="font-semibold">Trader Name</span>}
                {!formData.trader_name && (!formData.ai_model || !formData.exchange_id) && ', '}
                {!formData.ai_model && <span className="font-semibold">AI Model</span>}
                {!formData.ai_model && !formData.exchange_id && ', '}
                {!formData.exchange_id && <span className="font-semibold">Exchange</span>}
              </span>
            </div>
          </div>
        )}

        {/* Footer */}
        <div className="flex justify-end gap-3 p-6 border-t sticky bottom-0 z-10 rounded-b-xl"
        style={{ borderColor: 'var(--panel-border)', background: 'var(--navy-dark)' }}>
          <button
            onClick={onClose}
            className="px-6 py-3 text-[#EAECEF] rounded-lg transition-all duration-200"
            style={{ background: 'var(--panel-border)', border: '1px solid var(--panel-border)' }}
          >
            {t('cancel', language)}
          </button>
          {onSave && (
            <button
              onClick={handleSave}
              disabled={
                isSaving ||
                !formData.trader_name ||
                !formData.ai_model ||
                !formData.exchange_id
              }
              className="px-8 py-3 bg-gradient-to-r from-[var(--green-primary)] to-[var(--green-dark)] rounded-lg hover:from-[var(--green-dark)] hover:to-[var(--green-primary)] transition-all duration-200 disabled:bg-[#848E9C] disabled:cursor-not-allowed font-medium shadow-lg"
              style={{ color: 'var(--navy-primary)' }}
            >
              {isSaving ? t('savingTrader', language) : isEditMode ? t('save', language) : t('createTrader', language)}
            </button>
          )}
        </div>
      </div>

      {/* Prompt Template Modal */}
      <PromptTemplateModal
        isOpen={showTemplateModal}
        onClose={() => {
          setShowTemplateModal(false)
          setEditingTemplate(null)
        }}
        template={editingTemplate}
        onSave={async () => {
          // Refresh templates
          const userTemplates = await api.getUserPromptTemplates()
          setUserPromptTemplates(userTemplates)
        }}
        onDelete={async () => {
          // Refresh templates
          const userTemplates = await api.getUserPromptTemplates()
          setUserPromptTemplates(userTemplates)
          // Reset to default if deleted template was selected
          if (editingTemplate && formData.system_prompt_template === editingTemplate.id) {
            handleInputChange('system_prompt_template', 'default')
          }
        }}
      />

      {/* Save as Strategy Modal */}
      {showSaveAsStrategyModal && (
        <SaveAsStrategyModal
          isOpen={showSaveAsStrategyModal}
          onClose={() => setShowSaveAsStrategyModal(false)}
          onSave={handleSaveAsStrategy}
          language={language}
        />
      )}
    </div>
  )
}

// Save as Strategy Modal Component
interface SaveAsStrategyModalProps {
  isOpen: boolean
  onClose: () => void
  onSave: (name: string, description?: string) => void
  language: string
}

function SaveAsStrategyModal({ isOpen, onClose, onSave, language }: SaveAsStrategyModalProps) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')

  useEffect(() => {
    if (isOpen) {
      setName('')
      setDescription('')
    }
  }, [isOpen])

  if (!isOpen) return null

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) {
      toast.error(language === 'zh' ? '请输入策略名称' : 'Please enter strategy name')
      return
    }
    onSave(name.trim(), description.trim() || undefined)
  }

  return (
    <div 
      className="fixed inset-0 z-[300] flex items-center justify-center backdrop-blur-sm p-4 overflow-y-auto" 
      style={{ background: 'rgba(0, 31, 63, 0.5)' }}
      onClick={(e) => {
        if (e.target === e.currentTarget) {
          onClose()
        }
      }}
    >
      <div
        className="relative w-full max-w-md rounded-xl shadow-2xl"
        style={{ 
          background: 'var(--navy-dark)', 
          border: '1px solid var(--panel-border)',
          maxHeight: 'calc(100vh - 4rem)'
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b sticky top-0 z-10 rounded-t-xl"
          style={{ borderColor: 'var(--panel-border)', background: 'var(--navy-dark)' }}>
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-[var(--green-primary)] to-[var(--green-dark)] flex items-center justify-center" style={{ color: 'var(--navy-primary)' }}>
              <Save className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-xl font-bold text-[#EAECEF]">
                {language === 'zh' ? '保存为策略' : 'Save as Strategy'}
              </h2>
              <p className="text-sm text-[#848E9C] mt-1">
                {language === 'zh' ? '将当前配置保存为可重用的策略' : 'Save current configuration as a reusable strategy'}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="w-8 h-8 rounded-lg text-[#848E9C] hover:text-[#EAECEF] transition-colors flex items-center justify-center"
            style={{ '--hover-bg': 'var(--panel-border)' } as React.CSSProperties}
          >
            <IconX className="w-4 h-4" />
          </button>
        </div>

        {/* Content */}
        <form 
          onSubmit={handleSubmit} 
          className="p-6 space-y-4"
          onClick={(e) => e.stopPropagation()}
        >
          <div>
            <label className="text-sm font-medium text-[#EAECEF] block mb-2">
              {language === 'zh' ? '策略名称' : 'Strategy Name'} <span className="text-[var(--green-primary)]">*</span>
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors"
              style={{ background: 'var(--navy-background)', border: '1px solid var(--panel-border)' }}
              placeholder={language === 'zh' ? '输入策略名称' : 'Enter strategy name'}
              required
              autoFocus
            />
          </div>
          <div>
            <label className="text-sm font-medium text-[#EAECEF] block mb-2">
              {language === 'zh' ? '描述（可选）' : 'Description (Optional)'}
            </label>
            <textarea
              value={description}
              onChange={(e) => {
                e.stopPropagation()
                setDescription(e.target.value)
              }}
              onKeyDown={(e) => {
                e.stopPropagation()
              }}
              onKeyUp={(e) => {
                e.stopPropagation()
              }}
              onClick={(e) => {
                e.stopPropagation()
              }}
              onFocus={(e) => {
                e.stopPropagation()
              }}
              className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors resize-none"
              style={{ background: 'var(--navy-background)', border: '1px solid var(--panel-border)' }}
              placeholder={language === 'zh' ? '输入策略描述' : 'Enter strategy description'}
              rows={3}
            />
          </div>

          {/* Footer */}
          <div className="flex justify-end gap-3 pt-4 border-t" style={{ borderColor: 'var(--panel-border)' }}>
            <button
              type="button"
              onClick={onClose}
              className="px-6 py-2.5 rounded-lg text-[#EAECEF] transition-colors hover:bg-[var(--panel-border)] font-medium"
            >
              {language === 'zh' ? '取消' : 'Cancel'}
            </button>
            <button
              type="submit"
              className="px-6 py-2.5 bg-gradient-to-r from-[var(--green-primary)] to-[var(--green-dark)] rounded-lg hover:from-[var(--green-dark)] hover:to-[var(--green-primary)] transition-all duration-200 font-semibold shadow-lg"
              style={{ color: 'var(--navy-primary)' }}
            >
              {language === 'zh' ? '保存' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
