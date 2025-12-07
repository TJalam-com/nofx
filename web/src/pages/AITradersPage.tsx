import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import useSWR from 'swr'
import { toast } from 'sonner'
import { api } from '../lib/api'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth } from '../contexts/AuthContext'
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
  const { user, token } = useAuth()
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

  // Load configurations
  useEffect(() => {
    loadConfigs(user, token)
  }, [user, token, loadConfigs])

  // Check if we should open create modal with a trader to copy (from CompetitionPage)
  useEffect(() => {
    const copyTraderId = sessionStorage.getItem('copyTraderId')
    const urlParams = new URLSearchParams(window.location.search)
    const action = urlParams.get('action')
    
    console.log('🔍 AITradersPage - Checking copy action:', {
      copyTraderId,
      action,
      user: user ? { id: user.id, email: user.email, role: user.role } : null,
      token: !!token,
      showCreateModal,
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
        console.log('❌ AITradersPage - Missing AI model or exchange configuration')
        toast.error('Please configure an AI model and exchange first')
        // Clear copyTraderId from sessionStorage
        sessionStorage.removeItem('copyTraderId')
        // Clean up URL query params
        window.history.replaceState({}, '', '/traders')
        return
      }
      
      console.log('✅ AITradersPage - All conditions met, opening create modal with copyTraderId:', copyTraderId)
      setShowCreateModal(true)
      // Clean up URL query param but keep copyTraderId in sessionStorage for TraderConfigModal
      window.history.replaceState({}, '', '/traders')
      // The TraderConfigModal will handle the copyTraderId from sessionStorage
    } else {
      if (!copyTraderId) console.log('❌ AITradersPage - No copyTraderId found in sessionStorage')
      if (action !== 'copy') console.log('❌ AITradersPage - Action is not "copy", got:', action)
      if (!user) console.log('❌ AITradersPage - User not available')
      if (!token) console.log('❌ AITradersPage - Token not available')
      if (showCreateModal) console.log('⚠️ AITradersPage - Create modal already open')
    }
  }, [user, token, showCreateModal, setShowCreateModal, allModels, allExchanges])

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
        />
      )}
    </div>
  )
}
