import { useState, useEffect } from 'react'
import type { AIModel, Exchange, CreateTraderRequest, RunningTrader } from '../types'
import type { PromptTemplate } from '../types'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth, isFollower } from '../contexts/AuthContext'
import { t } from '../i18n/translations'
import { toast } from 'sonner'
import { Pencil, Plus, X as IconX, Edit2, Save } from 'lucide-react'
import { httpClient } from '../lib/httpClient'
import { api } from '../lib/api'
import { PromptTemplateModal } from './PromptTemplateModal'

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
    system_prompt_template: 'default',
    is_cross_margin: true,
    use_coin_pool: false,
    use_oi_top: false,
    use_tradingview: false,
    scan_interval_minutes: 3,
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
          console.log('🔍 Checking for copyTraderId in sessionStorage:', copyTraderId)
          
          if (copyTraderId) {
            console.log('📋 Found copyTraderId, looking for trader:', copyTraderId)
            const trader = traders.find((t) => t.trader_id === copyTraderId)
            
            if (trader) {
              console.log('✅ Found trader in running traders list:', trader)
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
                console.log('📝 Updated formData with trader copy:', newData)
                return newData
              })
              toast.success('Trader selected. All settings will be copied when you save.')
            } else {
              console.log('⚠️ Trader not found in running traders, trying public config API')
              // Try to get from public config if not in running traders
              api.getPublicTraderConfig(copyTraderId)
                .then((publicConfig) => {
                  console.log('✅ Got trader from public config:', publicConfig)
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
                    console.log('📝 DEBUG [TraderConfigModal]: Updated formData with trader copy from public config:', {
                      copyTraderId,
                      followed_trader_id: newData.followed_trader_id,
                      fullData: newData
                    })
                    return newData
                  })
                  toast.success('Trader selected. All settings will be copied when you save.')
                })
                .catch((err) => {
                  console.error('❌ Failed to get trader config for copy:', err)
                })
            }
            // Clear the sessionStorage after using it
            console.log('🧹 Clearing copyTraderId from sessionStorage')
            sessionStorage.removeItem('copyTraderId')
          } else {
            console.log('ℹ️ No copyTraderId found in sessionStorage')
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

  useEffect(() => {
    if (traderData) {
      console.log('🔍 DEBUG [TraderConfigModal]: Loading traderData:', {
        system_prompt_template: traderData.system_prompt_template,
        trader_id: traderData.trader_id,
        trader_name: traderData.trader_name,
        isEditMode: isEditMode
      })
      
      setFormData({
        ...traderData,
        // Ensure system_prompt_template has a default value if empty/undefined
        system_prompt_template: traderData.system_prompt_template || 'default',
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
          return prev // Don't reset if followed_trader_id is already set
        }
        
        // Otherwise, reset to defaults
        return {
          trader_name: '',
          ai_model: defaultAIModel,
          exchange_id: availableExchanges[0]?.id || '',
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
          followed_trader_id: '',
          initial_balance: 1000,
          scan_interval_minutes: 3,
        }
      })
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

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50 backdrop-blur-sm p-4 overflow-y-auto">
      <div
        className="bg-[#1E2329] border border-[#2B3139] rounded-xl shadow-2xl max-w-3xl w-full my-8"
        style={{ maxHeight: 'calc(100vh - 4rem)' }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-[#2B3139] bg-gradient-to-r from-[#1E2329] to-[#252B35] sticky top-0 z-10 rounded-t-xl">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-[#F0B90B] to-[#E1A706] flex items-center justify-center text-black">
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
            className="w-8 h-8 rounded-lg text-[#848E9C] hover:text-[#EAECEF] hover:bg-[#2B3139] transition-colors flex items-center justify-center"
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
            <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-5">
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
                    className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
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
          <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-5">
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
                  className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
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
                        className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
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
                    className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
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
          <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-5">
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
                          ? 'bg-[#F0B90B] text-black'
                          : 'bg-[#0B0E11] text-[#848E9C] border border-[#2B3139]'
                      }`}
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
                          ? 'bg-[#F0B90B] text-black'
                          : 'bg-[#0B0E11] text-[#848E9C] border border-[#2B3139]'
                      }`}
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
                        className="px-3 py-1 text-xs bg-[#F0B90B] text-black rounded hover:bg-[#E1A706] transition-colors disabled:bg-[#848E9C] disabled:cursor-not-allowed"
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
                      className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
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
                    <div className="w-full px-3 py-2 bg-[#1E2329] border border-[#2B3139] rounded text-[#848E9C] flex items-center gap-2">
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        className="w-4 h-4 text-[#F0B90B]"
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
                    className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
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
                    className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
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
                    className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
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
                    className="px-3 py-1 text-xs bg-[#F0B90B] text-black rounded hover:bg-[#E1A706] transition-colors"
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
                  className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
                  placeholder={t('tradingSymbolsExample', language)}
                />

                {/* 币种选择器 */}
                {showCoinSelector && (
                  <div className="mt-3 p-3 bg-[#0B0E11] border border-[#2B3139] rounded">
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
                              ? 'bg-[#F0B90B] text-black'
                              : 'bg-[#1E2329] text-[#848E9C] border border-[#2B3139] hover:border-[#F0B90B]'
                          }`}
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

          {/* Signal Sources - Only show for non-followers */}
          {!userIsFollower && (
            <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-5">
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

          {/* Trading Prompt - Show for all users (including followers) */}
          <div className="bg-[#0B0E11] border border-[#2B3139] rounded-lg p-5">
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
                      <span className="text-xs text-[#F0B90B] ml-2">
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
                    className="px-3 py-1 text-xs bg-[#F0B90B] text-black rounded hover:bg-[#E1A706] transition-colors flex items-center gap-1"
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
                    className="flex-1 px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
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
                    {userPromptTemplates.map((template: PromptTemplate) => (
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
                          className="px-3 py-2 bg-[#2B3139] text-[#EAECEF] rounded hover:bg-[#404750] transition-colors flex items-center gap-1"
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
                    background: 'rgba(240, 185, 11, 0.05)',
                    border: '1px solid rgba(240, 185, 11, 0.15)',
                  }}
                >
                  <div
                    className="text-xs font-semibold mb-1"
                    style={{ color: '#F0B90B' }}
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
                <span className="text-xs text-[#F0B90B] inline-flex items-center gap-1">
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
                <label className="text-sm text-[#EAECEF] block mb-2">
                  {formData.override_base_prompt
                    ? t('customPromptLabel', language)
                    : t('appendPromptLabel', language)}
                </label>
                <div className="space-y-2">
                  <textarea
                    value={formData.custom_prompt}
                    onChange={(e) =>
                      handleInputChange('custom_prompt', e.target.value)
                    }
                    className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none h-48 resize-y"
                    placeholder={
                      formData.override_base_prompt
                        ? t('customPromptPlaceholder', language)
                        : t('appendPromptPlaceholder', language)
                    }
                  />
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
                        className="px-3 py-1 text-xs bg-[#F0B90B] text-black rounded hover:bg-[#E1A706] transition-colors flex items-center gap-1"
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
            <div className="text-xs text-[#F0B90B] flex items-center gap-2 bg-[#1E2329] border border-[#F0B90B] border-opacity-30 rounded-lg p-3">
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
        <div className="flex justify-end gap-3 p-6 border-t border-[#2B3139] bg-gradient-to-r from-[#1E2329] to-[#252B35] sticky bottom-0 z-10 rounded-b-xl">
          <button
            onClick={onClose}
            className="px-6 py-3 bg-[#2B3139] text-[#EAECEF] rounded-lg hover:bg-[#404750] transition-all duration-200 border border-[#404750]"
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
              className="px-8 py-3 bg-gradient-to-r from-[#F0B90B] to-[#E1A706] text-black rounded-lg hover:from-[#E1A706] hover:to-[#D4951E] transition-all duration-200 disabled:bg-[#848E9C] disabled:cursor-not-allowed font-medium shadow-lg"
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
