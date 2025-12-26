import { useState, useEffect, useCallback } from 'react'
import type { AIModel, Exchange, CreateTraderRequest, RunningTrader } from '../types'
import type { PromptTemplate } from '../types'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth, isFollower } from '../contexts/AuthContext'
import { t } from '../i18n/translations'
import { toast } from 'sonner'
import { Pencil, Plus, X as IconX } from 'lucide-react'
import { httpClient } from '../lib/httpClient'
import { api } from '../lib/api'
import { PromptTemplateModal } from './PromptTemplateModal'
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
  const [_selectedCoins, setSelectedCoins] = useState<string[]>([])
  const [isFetchingBalance, setIsFetchingBalance] = useState(false)
  const [balanceFetchError, setBalanceFetchError] = useState<string>('')
  const [showTemplateModal, setShowTemplateModal] = useState(false)
  const [editingTemplate, setEditingTemplate] = useState<PromptTemplate | null>(null)
  const [runningTraders, setRunningTraders] = useState<RunningTrader[]>([])
  const [selectedTraderToCopy, setSelectedTraderToCopy] = useState<string>('')
  const [loadingRunningTraders, setLoadingRunningTraders] = useState(false)
  const [selectedStrategyId, setSelectedStrategyId] = useState<string>('')

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

  // Handle strategy selection - pure reference model: only set strategy_id
  const handleStrategyChange = async (strategyId: string) => {
    // #region agent log
    fetch('http://127.0.0.1:7242/ingest/39c2a80e-ec81-42f5-9ee5-0a97e070d0b3',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({location:'TraderConfigModal.tsx:365',message:'handleStrategyChange called',data:{strategyId,selectedStrategyId},timestamp:Date.now(),sessionId:'debug-session',runId:'run1',hypothesisId:'A'})}).catch(()=>{});
    // #endregion
    setSelectedStrategyId(strategyId)
    if (!strategyId) {
      // Clearing strategy - restore custom config fields (keep current values or defaults)
      setFormData((prev) => ({
        ...prev,
        strategy_id: undefined,
        // Keep existing values or set defaults for custom config
        btc_eth_leverage: prev.btc_eth_leverage || 5,
        altcoin_leverage: prev.altcoin_leverage || 3,
        trading_symbols: prev.trading_symbols || '',
        custom_prompt: prev.custom_prompt || '',
        override_base_prompt: prev.override_base_prompt || false,
        system_prompt_template: prev.system_prompt_template || 'default',
        is_cross_margin: prev.is_cross_margin ?? true,
        use_coin_pool: prev.use_coin_pool || false,
        use_oi_top: prev.use_oi_top || false,
        use_tradingview: prev.use_tradingview || false,
        enable_raw_klines: prev.enable_raw_klines ?? true,
        enable_ema: prev.enable_ema || false,
        enable_macd: prev.enable_macd || false,
        enable_rsi: prev.enable_rsi || false,
        enable_atr: prev.enable_atr || false,
        enable_volume: prev.enable_volume ?? true,
        enable_oi: prev.enable_oi ?? true,
        enable_funding: prev.enable_funding ?? true,
        indicator_timeframe: prev.indicator_timeframe || '3m',
        quant_data_url: prev.quant_data_url || '',
      }))
      toast.success(language === 'zh' ? '策略已清除，可使用自定义配置' : 'Strategy cleared, using custom config')
      return
    }

    try {
      // Validate strategy exists
      const strategy = await api.getStrategy(strategyId)
      // Strategy selected - clear strategy-related fields from formData
      // Strategy Studio is the single source of truth
      setFormData((prev) => {
        // #region agent log
        fetch('http://127.0.0.1:7242/ingest/39c2a80e-ec81-42f5-9ee5-0a97e070d0b3',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({location:'TraderConfigModal.tsx:403',message:'Setting strategy_id in formData',data:{strategyId,prevStrategyId:prev.strategy_id},timestamp:Date.now(),sessionId:'debug-session',runId:'run1',hypothesisId:'A'})}).catch(()=>{});
        // #endregion
        return {
        ...prev,
        strategy_id: strategyId,
        // Clear strategy-related fields (they'll be loaded from strategy when needed)
        btc_eth_leverage: 0,
        altcoin_leverage: 0,
        trading_symbols: '',
        custom_prompt: '',
        override_base_prompt: false,
        system_prompt_template: '',
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
        indicator_timeframe: '',
        quant_data_url: '',
        }
      })
      // #region agent log
      fetch('http://127.0.0.1:7242/ingest/39c2a80e-ec81-42f5-9ee5-0a97e070d0b3',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({location:'TraderConfigModal.tsx:428',message:'Strategy selected successfully',data:{strategyId,strategyName:strategy.name},timestamp:Date.now(),sessionId:'debug-session',runId:'run1',hypothesisId:'A'})}).catch(()=>{});
      // #endregion
      toast.success(language === 'zh' ? `策略 "${strategy.name}" 已选择，设置将从策略加载` : `Strategy "${strategy.name}" selected, settings will be loaded from strategy`)
    } catch (error: any) {
      toast.error(error.message || (language === 'zh' ? '加载策略失败' : 'Failed to load strategy'))
      setSelectedStrategyId('')
      setFormData((prev) => ({
        ...prev,
        strategy_id: undefined,
      }))
    }
  }


  useEffect(() => {
    if (traderData) {
      // #region agent log
      fetch('http://127.0.0.1:7242/ingest/39c2a80e-ec81-42f5-9ee5-0a97e070d0b3',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({location:'TraderConfigModal.tsx:451',message:'Loading traderData in useEffect',data:{trader_id:traderData.trader_id,strategy_id:traderData.strategy_id,hasStrategyId:!!traderData.strategy_id,isEditMode,currentFormDataStrategyId:formData.strategy_id},timestamp:Date.now(),sessionId:'debug-session',runId:'run1',hypothesisId:'A'})}).catch(()=>{});
      // #endregion
      console.log('🔍 DEBUG [TraderConfigModal]: Loading traderData:', {
        system_prompt_template: traderData.system_prompt_template,
        trader_id: traderData.trader_id,
        trader_name: traderData.trader_name,
        strategy_id: traderData.strategy_id,
        isEditMode: isEditMode
      })
      
      // If trader has strategy_id, only load trader-specific fields (Strategy Studio is single source of truth)
      if (traderData.strategy_id) {
        // #region agent log
        fetch('http://127.0.0.1:7242/ingest/39c2a80e-ec81-42f5-9ee5-0a97e070d0b3',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({location:'TraderConfigModal.tsx:461',message:'Setting formData with strategy_id',data:{strategy_id:traderData.strategy_id,trader_id:traderData.trader_id},timestamp:Date.now(),sessionId:'debug-session',runId:'run1',hypothesisId:'A'})}).catch(()=>{});
        // #endregion
        setFormData({
          trader_id: traderData.trader_id,
          trader_name: traderData.trader_name,
          ai_model: traderData.ai_model,
          exchange_id: traderData.exchange_id,
          initial_balance: traderData.initial_balance,
          scan_interval_minutes: traderData.scan_interval_minutes,
          followed_trader_id: traderData.followed_trader_id,
          strategy_id: traderData.strategy_id,
          // Set defaults for strategy-related fields (they'll be loaded from strategy when needed)
          btc_eth_leverage: 0,
          altcoin_leverage: 0,
          trading_symbols: '',
          custom_prompt: '',
          override_base_prompt: false,
          system_prompt_template: '',
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
          indicator_timeframe: '',
          quant_data_url: '',
        })
        setSelectedStrategyId(traderData.strategy_id)
      } else {
        // #region agent log
        fetch('http://127.0.0.1:7242/ingest/39c2a80e-ec81-42f5-9ee5-0a97e070d0b3',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({location:'TraderConfigModal.tsx:494',message:'Setting formData without strategy_id (custom config)',data:{trader_id:traderData.trader_id,hasStrategyId:false},timestamp:Date.now(),sessionId:'debug-session',runId:'run1',hypothesisId:'D'})}).catch(()=>{});
        // #endregion
        // No strategy_id - load all fields (custom configuration)
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
        setSelectedStrategyId('')
      }
      
      console.log('🔍 DEBUG [TraderConfigModal]: Set formData with system_prompt_template:', traderData.system_prompt_template || 'default')
      
      // 设置已选择的币种 (only if no strategy_id - strategy manages trading symbols)
      if (!traderData.strategy_id && traderData.trading_symbols) {
        const coins = traderData.trading_symbols
          .split(',')
          .map((s) => s.trim())
          .filter((s) => s)
        setSelectedCoins(coins)
      } else {
        setSelectedCoins([])
      }
    } else if (!isEditMode) {
      // For followers, auto-select risk_management as system prompt template
      const defaultAIModel = filteredModels.length > 0 
        ? getDefaultAIModel()
        : (availableModels[0]?.id || '')
      
      // Preserve followed_trader_id if it was already set (from copy trader flow)
      // Use functional update to avoid clearing it when dependencies change
      setFormData((prev) => {
        // #region agent log
        fetch('http://127.0.0.1:7242/ingest/39c2a80e-ec81-42f5-9ee5-0a97e070d0b3',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({location:'TraderConfigModal.tsx:536',message:'Resetting formData in useEffect (create mode)',data:{prevStrategyId:prev.strategy_id,prevFollowedTraderId:prev.followed_trader_id,hasFollowedTraderId:!!prev.followed_trader_id},timestamp:Date.now(),sessionId:'debug-session',runId:'run1',hypothesisId:'A'})}).catch(()=>{});
        // #endregion
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
  // Note: handleIndicatorChange is currently unused but kept for future IndicatorEditor integration

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

  // Note: handleCoinToggle is currently unused but kept for future coin selector UI

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

    // Require strategy selection for non-followers when creating new trader
    if (!userIsFollower && !isEditMode && !formData.strategy_id) {
      toast.error(language === 'zh' 
        ? '请先选择策略或创建新策略。所有交易配置必须在策略工作室中管理。'
        : 'Please select a strategy or create a new one first. All trading configuration must be managed in Strategy Studio.')
      return
    }

    setIsSaving(true)
    try {
      const hasStrategy = !!formData.strategy_id
      // #region agent log
      fetch('http://127.0.0.1:7242/ingest/39c2a80e-ec81-42f5-9ee5-0a97e070d0b3',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({location:'TraderConfigModal.tsx:786',message:'handleSave called - formData check',data:{strategy_id:formData.strategy_id,hasStrategy,selectedStrategyId,isEditMode},timestamp:Date.now(),sessionId:'debug-session',runId:'run1',hypothesisId:'B'})}).catch(()=>{});
      // #endregion
      console.log('🔍 DEBUG [TraderConfigModal]: Form data before save:', {
        strategy_id: formData.strategy_id,
        hasStrategy,
        followed_trader_id: formData.followed_trader_id,
      })
      
      // Pure reference model: if strategy_id is set, only send strategy_id and trader-specific fields
      // If strategy_id is not set, send individual settings (only allowed for followers or existing traders)
      const saveData: CreateTraderRequest = {
        name: formData.trader_name,
        ai_model_id: formData.ai_model,
        exchange_id: formData.exchange_id,
        scan_interval_minutes: formData.scan_interval_minutes,
        followed_trader_id: formData.followed_trader_id,
      }

      if (hasStrategy) {
        // Strategy reference mode: only send strategy_id
        saveData.strategy_id = formData.strategy_id
        // #region agent log
        fetch('http://127.0.0.1:7242/ingest/39c2a80e-ec81-42f5-9ee5-0a97e070d0b3',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({location:'TraderConfigModal.tsx:805',message:'Strategy reference mode - setting strategy_id in saveData',data:{strategy_id:formData.strategy_id,saveDataStrategyId:saveData.strategy_id},timestamp:Date.now(),sessionId:'debug-session',runId:'run1',hypothesisId:'B'})}).catch(()=>{});
        // #endregion
        console.log('✓ Using strategy reference mode - sending only strategy_id:', formData.strategy_id)
      } else {
        // Custom config mode: send individual settings (only for followers or existing traders without strategy)
        saveData.btc_eth_leverage = formData.btc_eth_leverage
        saveData.altcoin_leverage = formData.altcoin_leverage
        saveData.trading_symbols = formData.trading_symbols
        saveData.custom_prompt = formData.custom_prompt
        saveData.override_base_prompt = formData.override_base_prompt
        saveData.system_prompt_template = formData.system_prompt_template
        saveData.is_cross_margin = formData.is_cross_margin
        saveData.use_coin_pool = formData.use_coin_pool
        saveData.use_oi_top = formData.use_oi_top
        saveData.use_tradingview = formData.use_tradingview
        // Indicator configuration
        saveData.enable_raw_klines = formData.enable_raw_klines ?? true
        saveData.enable_ema = formData.enable_ema ?? false
        saveData.enable_macd = formData.enable_macd ?? false
        saveData.enable_rsi = formData.enable_rsi ?? false
        saveData.enable_atr = formData.enable_atr ?? false
        saveData.enable_volume = formData.enable_volume ?? true
        saveData.enable_oi = formData.enable_oi ?? true
        saveData.enable_funding = formData.enable_funding ?? true
        saveData.indicator_timeframe = formData.indicator_timeframe || '3m'
        saveData.quant_data_url = formData.quant_data_url || ''
        console.log('✓ Using custom config mode - sending individual settings')
      }

      // 只在编辑模式时包含initial_balance（用于手动更新）
      if (isEditMode && formData.initial_balance !== undefined) {
        saveData.initial_balance = formData.initial_balance
      }

      // #region agent log
      fetch('http://127.0.0.1:7242/ingest/39c2a80e-ec81-42f5-9ee5-0a97e070d0b3',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({location:'TraderConfigModal.tsx:840',message:'Save data before API call',data:{strategy_id:saveData.strategy_id,hasStrategy,isEditMode,fullSaveData:saveData},timestamp:Date.now(),sessionId:'debug-session',runId:'run1',hypothesisId:'C'})}).catch(()=>{});
      // #endregion
      console.log('🔍 DEBUG [TraderConfigModal]: Save data being sent:', {
        strategy_id: saveData.strategy_id,
        hasStrategy,
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

  // Note: getTooltipContent and getPromptExamples are currently unused but kept for future UI enhancements

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
              {/* 第一行：初始余额 */}
              <div className="grid grid-cols-2 gap-4">
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
                    {language === 'zh' ? '选择策略' : 'Select Strategy'}
                    <span className="text-xs text-[var(--green-primary)] ml-2">*</span>
                  </label>
                  <select
                    value={selectedStrategyId}
                    onChange={(e) => {
                      // #region agent log
                      fetch('http://127.0.0.1:7242/ingest/39c2a80e-ec81-42f5-9ee5-0a97e070d0b3',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({location:'TraderConfigModal.tsx:1167',message:'Strategy select onChange triggered',data:{selectedValue:e.target.value,currentSelectedStrategyId:selectedStrategyId,currentFormDataStrategyId:formData.strategy_id,isEditMode},timestamp:Date.now(),sessionId:'debug-session',runId:'run1',hypothesisId:'D'})}).catch(()=>{});
                      // #endregion
                      handleStrategyChange(e.target.value)
                    }}
                    className="w-full px-3 py-2 rounded text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none"
                    style={{ background: 'var(--navy-primary)', border: '1px solid var(--navy-light)', color: '#EAECEF' }}
                  >
                    <option value="">{language === 'zh' ? '-- 请选择策略 --' : '-- Please Select a Strategy --'}</option>
                    {strategies?.map((strategy) => (
                      <option key={strategy.id} value={strategy.id}>
                        {strategy.name}
                      </option>
                    ))}
                  </select>
                  {!selectedStrategyId && (
                    <div className="mt-2 p-3 rounded" style={{ background: 'rgba(255, 193, 7, 0.1)', border: '1px solid rgba(255, 193, 7, 0.3)' }}>
                      <div className="text-xs" style={{ color: '#FFC107' }}>
                        {language === 'zh' 
                          ? '⚠️ 请先选择策略或创建新策略。所有交易配置（杠杆、币种、信号源、指标、提示词等）都在策略工作室中管理。'
                          : '⚠️ Please select a strategy or create a new one first. All trading configuration (leverage, symbols, signal sources, indicators, prompts, etc.) is managed in Strategy Studio.'}
                      </div>
                      <div className="mt-2">
                        <a
                          href="/strategy-studio"
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-1 px-2 py-1 text-xs bg-[var(--green-primary)] rounded hover:bg-[var(--green-dark)] transition-colors"
                          style={{ color: 'var(--navy-primary)' }}
                        >
                          <svg xmlns="http://www.w3.org/2000/svg" className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2">
                            <path strokeLinecap="round" strokeLinejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                          </svg>
                          {language === 'zh' ? '前往策略工作室' : 'Go to Strategy Studio'}
                        </a>
                      </div>
                    </div>
                  )}
                  {selectedStrategyId && (
                    <div className="mt-2 p-4 rounded" style={{ background: 'rgba(0, 255, 127, 0.1)', border: '1px solid rgba(0, 255, 127, 0.3)' }}>
                      <div className="text-xs font-semibold mb-2" style={{ color: 'var(--green-primary)' }}>
                        {language === 'zh' ? '✓ 策略模式已启用' : '✓ Strategy Mode Active'}
                      </div>
                      <div className="text-xs mb-3" style={{ color: '#848E9C' }}>
                        {language === 'zh' 
                          ? `所有交易配置由策略 "${strategies?.find(s => s.id === selectedStrategyId)?.name || selectedStrategyId}" 管理。以下设置已从交易员配置中移除：`
                          : `All trading configuration is managed by strategy "${strategies?.find(s => s.id === selectedStrategyId)?.name || selectedStrategyId}". The following settings have been removed from trader config:`}
                      </div>
                      <div className="text-xs mb-3 space-y-1" style={{ color: '#848E9C' }}>
                        <div>• {language === 'zh' ? '交易配置' : 'Trading Configuration'} ({language === 'zh' ? '杠杆、交易币种、保证金模式' : 'leverage, trading symbols, margin mode'})</div>
                        <div>• {language === 'zh' ? '信号源' : 'Signal Sources'} ({language === 'zh' ? '币池、OI Top、TradingView' : 'coin pool, OI top, TradingView'})</div>
                        <div>• {language === 'zh' ? '指标配置' : 'Indicator Configuration'}</div>
                        <div>• {language === 'zh' ? '交易提示词' : 'Trading Prompts'} ({language === 'zh' ? '系统模板、自定义提示词' : 'system template, custom prompt'})</div>
                      </div>
                      <div className="text-xs" style={{ color: '#848E9C' }}>
                        {language === 'zh' 
                          ? '策略更新将自动应用到所有使用该策略的交易员。在策略工作室中编辑策略以修改这些设置。'
                          : 'Strategy updates will automatically apply to all traders using this strategy. Edit the strategy in Strategy Studio to modify these settings.'}
                      </div>
                      <div className="mt-3">
                        <a
                          href={`/strategy-studio${selectedStrategyId ? `?strategy=${selectedStrategyId}` : ''}`}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-2 px-3 py-1.5 text-xs bg-[var(--green-primary)] rounded hover:bg-[var(--green-dark)] transition-colors"
                          style={{ color: 'var(--navy-primary)' }}
                        >
                          <svg xmlns="http://www.w3.org/2000/svg" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2">
                            <path strokeLinecap="round" strokeLinejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                          </svg>
                          {language === 'zh' ? '在策略工作室中编辑' : 'Edit in Strategy Studio'}
                        </a>
                      </div>
                    </div>
                  )}
                  {!selectedStrategyId && (
                    <div className="text-xs text-[#848E9C] mt-2">
                      {language === 'zh' 
                        ? '选择策略后，所有策略相关设置将从交易员配置中移除，由策略统一管理。如需自定义设置，请创建新策略。' 
                        : 'When a strategy is selected, all strategy-related settings will be removed from trader config and managed by the strategy. To customize settings, create a new strategy.'}
                    </div>
                  )}
                </div>
              </div>
            </div>
          )}

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

    </div>
  )
}

