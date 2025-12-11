import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import useSWR from 'swr'
import { toast } from 'sonner'
import { api } from '../lib/api'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth, isFollower } from '../contexts/AuthContext'
import { useTradersConfigStore, useTradersModalStore } from '../stores'
import { useTraderActions } from '../hooks/useTraderActions'
import { TraderConfigModal } from '../components/TraderConfigModal'
import {
  SignalSourceModal,
  ModelConfigModal,
  ExchangeConfigModal,
} from '../components/traders'
import { PageHeader } from '../components/traders/sections/PageHeader'
import { SignalSourceWarning } from '../components/traders/sections/SignalSourceWarning'
import { AIModelsSection } from '../components/traders/sections/AIModelsSection'
import { ExchangesSection } from '../components/traders/sections/ExchangesSection'
import { TradersGrid } from '../components/traders/sections/TradersGrid'

interface AITradersPageProps {
  onTraderSelect?: (traderId: string) => void
}

export function AITradersPage({ onTraderSelect }: AITradersPageProps) {
  const { language } = useLanguage()
  const { user, token, isLoading } = useAuth()
  const navigate = useNavigate()

  // Zustand stores
  const {
    allModels,
    allExchanges,
    supportedModels,
    supportedExchanges,
    configuredModels,
    configuredExchanges,
    userSignalSource,
    loadConfigs,
    setAllModels,
    setAllExchanges,
    setUserSignalSource,
  } = useTradersConfigStore()

  const {
    showCreateModal,
    showEditModal,
    showModelModal,
    showExchangeModal,
    showSignalSourceModal,
    editingModel,
    editingExchange,
    editingTrader,
    setShowCreateModal,
    setShowEditModal,
    setShowModelModal,
    setShowExchangeModal,
    setShowSignalSourceModal,
    setEditingModel,
    setEditingExchange,
    setEditingTrader,
  } = useTradersModalStore()

  // SWR for traders data
  const { data: traders, mutate: mutateTraders } = useSWR(
    user && token ? 'traders' : null,
    api.getTraders,
    { refreshInterval: 5000 }
  )

  // Track when configurations are ready
  const [configsReady, setConfigsReady] = useState(false)

  // Load configurations
  useEffect(() => {
    let cancelled = false
    const loadConfigsAsync = async () => {
      try {
        await loadConfigs(user, token)
        if (!cancelled) {
          setConfigsReady(true)
        }
      } catch (error) {
        console.error('Failed to load configs:', error)
        // Still mark as ready even on error to avoid blocking copy flow indefinitely
        if (!cancelled) {
          setConfigsReady(true)
        }
      }
    }
    
    // Reset configsReady when user/token changes
    setConfigsReady(false)
    loadConfigsAsync()
    
    return () => {
      cancelled = true
    }
  }, [user, token, loadConfigs])

  // Check if we should open create modal with a trader to copy (from CompetitionPage)
  useEffect(() => {
    // Wait for auth to finish loading before checking user/token
    if (isLoading) {
      return
    }
    
    // Wait for configs to be ready before checking copy intent
    // This prevents premature "missing config" errors when configs are still loading
    if (!configsReady) {
      const copyTraderId = sessionStorage.getItem('copyTraderId')
      const urlParams = new URLSearchParams(window.location.search)
      const action = urlParams.get('action')
      const hasCopyIntent = copyTraderId || action === 'copy'
      
      if (hasCopyIntent) {
        console.log('⏳ AITradersPage - Configs not ready yet, waiting before checking copy intent')
      }
      return
    }
    
    const copyTraderId = sessionStorage.getItem('copyTraderId')
    const urlParams = new URLSearchParams(window.location.search)
    const action = urlParams.get('action')
    
    // Early return if there's no copy intent - avoid unnecessary checks and logs
    const hasCopyIntent = copyTraderId || action === 'copy'
    if (!hasCopyIntent) {
      return // Normal page load, no copy action - exit silently
    }
    
    // Only log when there's actual copy intent
    console.log('🔍 AITradersPage - Checking copy action:', {
      copyTraderId,
      action,
      user: user ? { id: user.id, email: user.email, role: user.role } : null,
      token: !!token,
      showCreateModal,
      isLoading,
      configsReady,
      allModelsCount: allModels?.length || 0,
      allExchangesCount: allExchanges?.length || 0,
      currentPath: window.location.pathname,
      currentSearch: window.location.search
    })
    
    if (copyTraderId && action === 'copy' && user && token && !showCreateModal) {
      // Check if AI models and exchanges are configured
      const enabledModels = allModels?.filter((m) => m.enabled) || []
      const enabledExchanges =
        allExchanges?.filter((e) => {
          if (!e.enabled) return false
          if (e.id === 'aster') {
            return e.asterUser?.trim() && e.asterSigner?.trim()
          }
          if (e.id === 'hyperliquid') {
            return e.hyperliquidWalletAddr?.trim()
          }
          return true
        }) || []
      
      if (enabledModels.length === 0 || enabledExchanges.length === 0) {
        console.log('❌ AITradersPage - Missing AI model or exchange configuration (configs loaded but none enabled)')
        toast.error('Please configure an AI model and exchange first. The copy action will be available after configuration.')
        // Don't clear copyTraderId - keep it so user can configure and retry
        // Only clean up URL query params
        window.history.replaceState({}, '', '/traders')
        return
      }
      
      console.log('✅ AITradersPage - All conditions met, opening create modal with copyTraderId:', copyTraderId)
      setShowCreateModal(true)
      // Clean up URL query param but keep copyTraderId in sessionStorage for TraderConfigModal
      // The TraderConfigModal will clear copyTraderId after successfully loading the trader config
      window.history.replaceState({}, '', '/traders')
    } else {
      // Only log errors when we're actually expecting a copy action
      if (hasCopyIntent) {
        if (!copyTraderId) console.log('❌ AITradersPage - No copyTraderId found in sessionStorage')
        if (action !== 'copy') console.log('❌ AITradersPage - Action is not "copy", got:', action)
        if (!user) console.log('❌ AITradersPage - User not available')
        if (!token) console.log('❌ AITradersPage - Token not available')
        if (showCreateModal) console.log('⚠️ AITradersPage - Create modal already open')
      }
    }
  }, [user, token, showCreateModal, setShowCreateModal, allModels, allExchanges, isLoading, configsReady])

  // Business logic hook
  const {
    isModelInUse,
    isExchangeInUse,
    handleCreateTrader,
    handleEditTrader,
    handleSaveEditTrader,
    handleDeleteTrader,
    handleToggleTrader,
    handleAddModel,
    handleAddExchange,
    handleModelClick,
    handleExchangeClick,
    handleSaveModel,
    handleDeleteModel,
    handleSaveExchange,
    handleDeleteExchange,
    handleSaveSignalSource,
  } = useTraderActions({
    traders,
    allModels,
    allExchanges,
    supportedModels,
    supportedExchanges,
    language,
    mutateTraders,
    setAllModels,
    setAllExchanges,
    setUserSignalSource,
    setShowCreateModal,
    setShowEditModal,
    setShowModelModal,
    setShowExchangeModal,
    setShowSignalSourceModal,
    setEditingModel,
    setEditingExchange,
    editingTrader,
    setEditingTrader,
  })

  // 计算派生状态
  const enabledModels = allModels?.filter((m) => m.enabled) || []
  const enabledExchanges =
    allExchanges?.filter((e) => {
      if (!e.enabled) return false
      if (e.id === 'aster') {
        return e.asterUser?.trim() && e.asterSigner?.trim()
      }
      if (e.id === 'hyperliquid') {
        return e.hyperliquidWalletAddr?.trim()
      }
      return true
    }) || []

  // 检查是否需要显示信号源警告
  const showSignalWarning =
    traders?.some((t) => t.use_coin_pool || t.use_oi_top) &&
    !userSignalSource.coinPoolUrl &&
    !userSignalSource.oiTopUrl

  // 处理交易员查看
  const handleTraderSelect = (traderId: string) => {
    if (onTraderSelect) {
      onTraderSelect(traderId)
    } else {
      navigate(`/dashboard?trader=${traderId}`)
    }
  }

  // Handle copy trader from SignalSourceModal
  const handleCopyTraderFromSignalSource = (traderId: string) => {
    // Store the trader ID in sessionStorage (same pattern as CompetitionPage)
    sessionStorage.setItem('copyTraderId', traderId)
    // Close SignalSourceModal and open TraderConfigModal
    setShowSignalSourceModal(false)
    setShowCreateModal(true)
  }

  return (
    <div className="space-y-4 md:space-y-6 animate-fade-in">
      {/* Header */}
      <PageHeader
        language={language}
        tradersCount={traders?.length || 0}
        configuredModelsCount={configuredModels.length}
        configuredExchangesCount={configuredExchanges.length}
        onAddModel={handleAddModel}
        onAddExchange={handleAddExchange}
        onConfigureSignalSource={() => setShowSignalSourceModal(true)}
        onCreateTrader={() => setShowCreateModal(true)}
      />

      {/* Signal Source Warning */}
      {showSignalWarning && (
        <SignalSourceWarning
          language={language}
          onConfigure={() => setShowSignalSourceModal(true)}
        />
      )}

      {/* Configuration Status */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 md:gap-6">
        <AIModelsSection
          language={language}
          configuredModels={configuredModels}
          isModelInUse={isModelInUse}
          onModelClick={handleModelClick}
        />

        <ExchangesSection
          language={language}
          configuredExchanges={configuredExchanges}
          isExchangeInUse={isExchangeInUse}
          onExchangeClick={handleExchangeClick}
        />
      </div>

      {/* Traders Grid */}
      <TradersGrid
        language={language}
        traders={traders}
        onTraderSelect={handleTraderSelect}
        onEditTrader={handleEditTrader}
        onDeleteTrader={handleDeleteTrader}
        onToggleTrader={handleToggleTrader}
      />

      {/* Become a Trader Card for Followers */}
      {user && isFollower(user) && (
        <div
          className="rounded-lg p-6 border-2"
          style={{
            background: 'linear-gradient(135deg, rgba(34, 197, 94, 0.1) 0%, rgba(34, 197, 94, 0.05) 100%)',
            borderColor: 'var(--green-primary)',
          }}
        >
          <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
            <div className="flex-1">
              <h3 className="text-xl font-bold mb-2" style={{ color: 'var(--text-primary)' }}>
                Become a Trader
              </h3>
              <p className="text-sm mb-2" style={{ color: 'var(--text-secondary)' }}>
                Currently, as a follower, you can only copy trades from other traders. Apply to become a trader and unlock the ability to create your own AI trading bots, configure trading strategies, and let others follow your trades.
              </p>
              <p className="text-xs mb-4" style={{ color: 'var(--text-secondary)' }}>
                After you submit your application, an admin will review it. Once approved, your role will be upgraded and you'll gain access to all trader features.
              </p>
            </div>
            <button
              onClick={() => navigate('/become-trader')}
              className="px-6 py-3 rounded-lg font-semibold transition-opacity hover:opacity-90 whitespace-nowrap"
              style={{
                background: 'var(--green-primary)',
                color: 'var(--navy-primary)',
              }}
            >
              Apply Now
            </button>
          </div>
        </div>
      )}

      {/* Modals */}
      <TraderConfigModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        isEditMode={false}
        availableModels={enabledModels}
        availableExchanges={enabledExchanges}
        onSave={handleCreateTrader}
      />

      <TraderConfigModal
        isOpen={showEditModal}
        onClose={() => setShowEditModal(false)}
        isEditMode={true}
        traderData={editingTrader}
        availableModels={enabledModels}
        availableExchanges={enabledExchanges}
        onSave={handleSaveEditTrader}
      />

      {showModelModal && (
        <ModelConfigModal
          allModels={supportedModels}
          configuredModels={allModels}
          editingModelId={editingModel}
          onSave={handleSaveModel}
          onDelete={handleDeleteModel}
          onClose={() => setShowModelModal(false)}
          language={language}
        />
      )}

      {showExchangeModal && (
        <ExchangeConfigModal
          allExchanges={supportedExchanges}
          editingExchangeId={editingExchange}
          onSave={handleSaveExchange}
          onDelete={handleDeleteExchange}
          onClose={() => setShowExchangeModal(false)}
          language={language}
        />
      )}

      {showSignalSourceModal && (
        <SignalSourceModal
          coinPoolUrl={userSignalSource.coinPoolUrl}
          oiTopUrl={userSignalSource.oiTopUrl}
          onSave={handleSaveSignalSource}
          onClose={() => setShowSignalSourceModal(false)}
          language={language}
          configuredModels={configuredModels}
          configuredExchanges={configuredExchanges}
          onCopyTrader={user && isFollower(user) ? handleCopyTraderFromSignalSource : undefined}
        />
      )}
    </div>
  )
}
