import { useEffect, useState, useRef } from 'react'
import useSWR, { mutate } from 'swr'
import { AlertTriangle, LogOut, Loader2 } from 'lucide-react'
import { api } from './lib/api'
import { ChartTabs } from './components/ChartTabs'
import { AITradersPage } from './components/AITradersPage'
import { LoginPage } from './components/LoginPage'
import { RegisterPage } from './components/RegisterPage'
import { ResetPasswordPage } from './components/ResetPasswordPage'
import { CompetitionPage } from './components/CompetitionPage'
import { LandingPage } from './pages/LandingPage'
import { FAQPage } from './pages/FAQPage'
import HeaderBar from './components/HeaderBar'
import Footer from './components/Footer'
import AILearning from './components/AILearning'
import { LanguageProvider, useLanguage } from './contexts/LanguageContext'
import { AuthProvider, useAuth, isFollower } from './contexts/AuthContext'
import { ConfirmDialogProvider } from './components/ConfirmDialog'
import { t, type Language } from './i18n/translations'
import { confirmToast, notify } from './lib/notify'
import { useSystemConfig } from './hooks/useSystemConfig'
import { DecisionCard } from './components/DecisionCard'
import { BacktestPage } from './components/BacktestPage'
import WebhookPage from './pages/WebhookPage'
import FollowersPage from './pages/FollowersPage'
import StatsPage from './pages/StatsPage'
import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  TraderInfo,
} from './types'

type Page =
  | 'competition'
  | 'traders'
  | 'trader'
  | 'followers'
  | 'backtest'
  | 'webhook'
  | 'faq'
  | 'stats'
  | 'applications'
  | 'login'
  | 'register'

// 获取友好的AI模型名称
function getModelDisplayName(modelId: string): string {
  switch (modelId.toLowerCase()) {
    case 'deepseek':
      return 'DeepSeek'
    case 'qwen':
      return 'Qwen'
    case 'claude':
      return 'Claude'
    default:
      return modelId.toUpperCase()
  }
}

function App() {
  const { language } = useLanguage()
  const { user, token, logout, isLoading } = useAuth()
  const { loading: configLoading } = useSystemConfig()
  const [route, setRoute] = useState(window.location.pathname)

  // 从URL路径读取初始页面状态（支持刷新保持页面）
  const getInitialPage = (): Page => {
    const path = window.location.pathname
    const hash = window.location.hash.slice(1) // 去掉 #

    if (path === '/traders' || hash === 'traders') return 'traders'
    if (path === '/followers' || hash === 'followers') return 'followers'
    if (path === '/backtest' || hash === 'backtest') return 'backtest'
    if (path === '/webhook' || hash === 'webhook') return 'webhook'
    if (path === '/stats' || hash === 'stats') return 'stats'
    if (path === '/strategy-studio' || hash === 'strategy-studio') return 'strategy-studio'
    if (path === '/dashboard' || hash === 'trader' || hash === 'details')
      return 'trader'
    return 'competition' // 默认为竞赛页面
  }

  const [currentPage, setCurrentPage] = useState<Page>(getInitialPage())
  const [selectedTraderId, setSelectedTraderId] = useState<string | undefined>()
  const [lastUpdate, setLastUpdate] = useState<string>('--:--:--')
  const [backendAvailable, setBackendAvailable] = useState<boolean | null>(null)

  // 检查后端连接状态
  useEffect(() => {
    const checkBackendHealth = async () => {
      try {
        const response = await fetch('/api/health', {
          method: 'GET',
          signal: AbortSignal.timeout(3000), // 3秒超时
        })
        if (response.ok) {
          setBackendAvailable(true)
        } else {
          setBackendAvailable(false)
        }
      } catch (error) {
        setBackendAvailable(false)
      }
    }

    // 初始检查
    checkBackendHealth()

    // 每10秒检查一次
    const interval = setInterval(checkBackendHealth, 10000)

    return () => clearInterval(interval)
  }, [])

  // 监听URL变化，同步页面状态
  useEffect(() => {
    const handleRouteChange = () => {
      const path = window.location.pathname
      const hash = window.location.hash.slice(1)

      if (path === '/traders' || hash === 'traders') {
        setCurrentPage('traders')
      } else if (path === '/backtest' || hash === 'backtest') {
        setCurrentPage('backtest')
      } else if (path === '/webhook' || hash === 'webhook') {
        setCurrentPage('webhook')
      } else if (path === '/stats' || hash === 'stats') {
        setCurrentPage('stats')
      } else if (path === '/strategy-studio' || hash === 'strategy-studio') {
        setCurrentPage('strategy-studio')
      } else if (
        path === '/dashboard' ||
        hash === 'trader' ||
        hash === 'details'
      ) {
        setCurrentPage('trader')
      } else if (
        path === '/competition' ||
        hash === 'competition' ||
        hash === ''
      ) {
        setCurrentPage('competition')
      }
      setRoute(path)
    }

    window.addEventListener('hashchange', handleRouteChange)
    window.addEventListener('popstate', handleRouteChange)
    return () => {
      window.removeEventListener('hashchange', handleRouteChange)
      window.removeEventListener('popstate', handleRouteChange)
    }
  }, [])

  // Redirect followers away from restricted pages
  useEffect(() => {
    if (user && token && isFollower(user)) {
      const restrictedPages = ['webhook', 'backtest']
      if (restrictedPages.includes(currentPage)) {
        // Redirect to competition page
        window.history.pushState({}, '', '/competition')
        setRoute('/competition')
        setCurrentPage('competition')
      }
    }
  }, [user, token, currentPage])

  // 切换页面时更新URL hash (当前通过按钮直接调用setCurrentPage，这个函数暂时保留用于未来扩展)
  // const navigateToPage = (page: Page) => {
  //   setCurrentPage(page);
  //   window.location.hash = page === 'competition' ? '' : 'trader';
  // };

  // 获取trader列表（仅在用户登录时）
  const { data: traders, error: tradersError } = useSWR<TraderInfo[]>(
    user && token ? 'traders' : null,
    api.getTraders,
    {
      refreshInterval: 10000,
      shouldRetryOnError: false, // 避免在后端未运行时无限重试
    }
  )

  // 当获取到traders后，设置默认选中第一个
  useEffect(() => {
    if (traders && traders.length > 0 && !selectedTraderId) {
      setSelectedTraderId(traders[0].trader_id)
    }
  }, [traders, selectedTraderId])

  // 如果在trader页面，获取该trader的数据
  const { data: status } = useSWR<SystemStatus>(
    currentPage === 'trader' && selectedTraderId
      ? `status-${selectedTraderId}`
      : null,
    () => api.getStatus(selectedTraderId),
    {
      refreshInterval: 15000, // 15秒刷新（配合后端15秒缓存）
      revalidateOnFocus: false, // 禁用聚焦时重新验证，减少请求
      dedupingInterval: 10000, // 10秒去重，防止短时间内重复请求
    }
  )

  const { data: account } = useSWR<AccountInfo>(
    currentPage === 'trader' && selectedTraderId
      ? `account-${selectedTraderId}`
      : null,
    () => api.getAccount(selectedTraderId),
    {
      refreshInterval: 15000, // 15秒刷新（配合后端15秒缓存）
      revalidateOnFocus: false, // 禁用聚焦时重新验证，减少请求
      dedupingInterval: 10000, // 10秒去重，防止短时间内重复请求
    }
  )

  const { data: positions } = useSWR<Position[]>(
    currentPage === 'trader' && selectedTraderId
      ? `positions-${selectedTraderId}`
      : null,
    () => api.getPositions(selectedTraderId),
    {
      refreshInterval: 15000, // 15秒刷新（配合后端15秒缓存）
      revalidateOnFocus: false, // 禁用聚焦时重新验证，减少请求
      dedupingInterval: 10000, // 10秒去重，防止短时间内重复请求
    }
  )

  const { data: decisions } = useSWR<DecisionRecord[]>(
    currentPage === 'trader' && selectedTraderId
      ? `decisions/latest-${selectedTraderId}`
      : null,
    () => api.getLatestDecisions(selectedTraderId),
    {
      refreshInterval: 30000, // 30秒刷新（决策更新频率较低）
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  const { data: stats } = useSWR<Statistics>(
    currentPage === 'trader' && selectedTraderId
      ? `statistics-${selectedTraderId}`
      : null,
    () => api.getStatistics(selectedTraderId),
    {
      refreshInterval: 30000, // 30秒刷新（统计数据更新频率较低）
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  useEffect(() => {
    if (account) {
      const now = new Date().toLocaleTimeString()
      setLastUpdate(now)
    }
  }, [account])

  const selectedTrader = traders?.find((t) => t.trader_id === selectedTraderId)

  // Handle routing
  useEffect(() => {
    const handlePopState = () => {
      setRoute(window.location.pathname)
    }
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  // Set current page based on route for consistent navigation state
  useEffect(() => {
    if (route === '/competition') {
      setCurrentPage('competition')
    } else if (route === '/traders') {
      setCurrentPage('traders')
    } else if (route === '/dashboard') {
      setCurrentPage('trader')
    } else if (route === '/followers') {
      setCurrentPage('followers')
    }
  }, [route])

  // Show loading spinner while checking auth or config
  if (isLoading || configLoading) {
    return (
      <div
        className="min-h-screen flex items-center justify-center"
        style={{ background: '#0B0E11' }}
      >
        <div className="text-center">
          <img
            src="/icons/nofx.svg"
            alt="AI Trading 24x7 Logo"
            className="w-16 h-16 mx-auto mb-4 animate-pulse"
          />
          <p style={{ color: '#EAECEF' }}>{t('loading', language)}</p>
        </div>
      </div>
    )
  }

  // Handle specific routes regardless of authentication
  if (route === '/login') {
    return <LoginPage />
  }
  if (route === '/register') {
    return <RegisterPage />
  }
  if (route === '/faq') {
    return <FAQPage />
  }
  if (route === '/reset-password') {
    return <ResetPasswordPage />
  }
  if (route === '/competition') {
    return (
      <div
        className="min-h-screen"
        style={{ background: 'var(--navy-primary)', color: '#EAECEF' }}
      >
        <HeaderBar
          isLoggedIn={!!user}
          currentPage="competition"
          language={language}
          user={user}
          onLogout={logout}
          onPageChange={(page: Page) => {
            console.log('Competition page onPageChange called with:', page)
            console.log('Current route:', route, 'Current page:', currentPage)

            if (page === 'competition') {
              console.log('Navigating to competition')
              window.history.pushState({}, '', '/competition')
              setRoute('/competition')
              setCurrentPage('competition')
            } else if (page === 'traders') {
              console.log('Navigating to traders')
              window.history.pushState({}, '', '/traders')
              setRoute('/traders')
              setCurrentPage('traders')
            } else if (page === 'trader') {
              console.log('Navigating to trader/dashboard')
              window.history.pushState({}, '', '/dashboard')
              setRoute('/dashboard')
              setCurrentPage('trader')
            } else if (page === 'faq') {
              console.log('Navigating to faq')
              window.history.pushState({}, '', '/faq')
              setRoute('/faq')
            } else if (page === 'backtest') {
              console.log('Navigating to backtest')
              window.history.pushState({}, '', '/backtest')
              setRoute('/backtest')
              setCurrentPage('backtest')
            } else if (page === 'webhook') {
              console.log('Navigating to webhook')
              window.history.pushState({}, '', '/webhook')
              setRoute('/webhook')
              setCurrentPage('webhook')
          } else if (page === 'followers') {
            console.log('Navigating to followers')
            window.history.pushState({}, '', '/followers')
            setRoute('/followers')
            setCurrentPage('followers')
            }

            console.log(
              'After navigation - route:',
              route,
              'currentPage:',
              currentPage
            )
          }}
        />
        <main className="max-w-[1920px] mx-auto px-6 py-6 pt-24">
          <CompetitionPage />
        </main>
      </div>
    )
  }

  // Show landing page for root route
  if (route === '/' || route === '') {
    return <LandingPage />
  }

  // Allow unauthenticated users to open backtest page directly (others仍展示 Landing)
  if (!user || !token) {
    if (route === '/backtest' || currentPage === 'backtest') {
      return (
        <div
          className="min-h-screen"
          style={{ background: '#0B0E11', color: '#EAECEF' }}
        >
          <HeaderBar
            isLoggedIn={false}
            currentPage="backtest"
            language={language}
            onPageChange={(page: Page) => {
              if (page === 'competition') {
                window.history.pushState({}, '', '/competition')
                setRoute('/competition')
                setCurrentPage('competition')
              } else if (page === 'traders') {
                window.history.pushState({}, '', '/traders')
                setRoute('/traders')
                setCurrentPage('traders')
              }
            }}
          />
          <main className="max-w-[1920px] mx-auto px-6 py-6 pt-24">
            <BacktestPage />
          </main>
        </div>
      )
    }
    return <LandingPage />
  }

  // Show main app for authenticated users on other routes
  if (!user || !token) {
    // Default to landing page when not authenticated and no specific route
    return <LandingPage />
  }

  return (
    <div
      className="min-h-screen"
      style={{ background: '#0B0E11', color: '#EAECEF' }}
    >
      <HeaderBar
        isLoggedIn={!!user}
        currentPage={currentPage}
        language={language}
        user={user}
        onLogout={logout}
        onPageChange={(page: Page) => {
          console.log('Main app onPageChange called with:', page)

          if (page === 'competition') {
            window.history.pushState({}, '', '/competition')
            setRoute('/competition')
            setCurrentPage('competition')
          } else if (page === 'traders') {
            window.history.pushState({}, '', '/traders')
            setRoute('/traders')
            setCurrentPage('traders')
          } else if (page === 'trader') {
            window.history.pushState({}, '', '/dashboard')
            setRoute('/dashboard')
            setCurrentPage('trader')
          } else if (page === 'backtest') {
            window.history.pushState({}, '', '/backtest')
            setRoute('/backtest')
            setCurrentPage('backtest')
          } else if (page === 'faq') {
            window.history.pushState({}, '', '/faq')
            setRoute('/faq')
          } else if (page === 'webhook') {
            window.history.pushState({}, '', '/webhook')
            setRoute('/webhook')
            setCurrentPage('webhook')
          } else if (page === 'followers') {
            window.history.pushState({}, '', '/followers')
            setRoute('/followers')
            setCurrentPage('followers')
          } else if (page === 'stats') {
            window.history.pushState({}, '', '/stats')
            setRoute('/stats')
            setCurrentPage('stats')
          } else if (page === 'strategy-studio') {
            window.history.pushState({}, '', '/strategy-studio')
            setRoute('/strategy-studio')
            setCurrentPage('strategy-studio')
          }
        }}
      />

      {/* Backend Connection Status Banner */}
      {backendAvailable === false && (
        <div
          className="mx-auto max-w-[1920px] px-6 pt-4"
          style={{ marginTop: '80px' }}
        >
          <div
            className="flex items-center gap-3 rounded-lg border p-4"
            style={{
              background: 'rgba(246, 70, 93, 0.1)',
              borderColor: 'rgba(246, 70, 93, 0.3)',
              color: '#F6465D',
            }}
          >
            <AlertTriangle size={20} />
            <div className="flex-1">
              <div className="font-semibold">
                {language === 'zh'
                  ? '后端服务器未运行'
                  : 'Backend Server Not Running'}
              </div>
              <div className="mt-1 text-sm opacity-90">
                {language === 'zh'
                  ? '请启动后端服务器：在项目根目录运行 ./nofx 或 go run main.go'
                  : 'Please start the backend server: Run ./nofx or go run main.go from the project root'}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Main Content */}
      <main className="max-w-[1920px] mx-auto px-6 py-6 pt-24">
        {currentPage === 'competition' ? (
          <CompetitionPage />
        ) : currentPage === 'traders' ? (
          <AITradersPage
            onTraderSelect={(traderId) => {
              setSelectedTraderId(traderId)
              window.history.pushState({}, '', '/dashboard')
              setRoute('/dashboard')
              setCurrentPage('trader')
            }}
          />
        ) : currentPage === 'followers' ? (
          <FollowersPage />
        ) : currentPage === 'backtest' ? (
          <BacktestPage />
        ) : currentPage === 'webhook' ? (
          <WebhookPage />
        ) : currentPage === 'stats' ? (
          <StatsPage />
        ) : (
          <TraderDetailsPage
            selectedTrader={selectedTrader}
            status={status}
            account={account}
            positions={positions}
            decisions={decisions}
            stats={stats}
            lastUpdate={lastUpdate}
            language={language}
            traders={traders}
            tradersError={tradersError}
            selectedTraderId={selectedTraderId}
            onTraderSelect={setSelectedTraderId}
            onNavigateToTraders={() => {
              window.history.pushState({}, '', '/traders')
              setRoute('/traders')
              setCurrentPage('traders')
            }}
          />
        )}
      </main>

      {/* Footer */}
      <Footer variant="simple" />
    </div>
  )
}

// Trader Details Page Component
function TraderDetailsPage({
  selectedTrader,
  status,
  account,
  positions,
  decisions,
  lastUpdate,
  language,
  traders,
  tradersError,
  selectedTraderId,
  onTraderSelect,
  onNavigateToTraders,
}: {
  selectedTrader?: TraderInfo
  traders?: TraderInfo[]
  tradersError?: Error
  selectedTraderId?: string
  onTraderSelect: (traderId: string) => void
  onNavigateToTraders: () => void
  status?: SystemStatus
  account?: AccountInfo
  positions?: Position[]
  decisions?: DecisionRecord[]
  stats?: Statistics
  lastUpdate: string
  language: Language
}) {
  const [closingPositions, setClosingPositions] = useState<Set<string>>(new Set())
  const [selectedChartSymbol, setSelectedChartSymbol] = useState<string | undefined>(undefined)
  const [chartUpdateKey, setChartUpdateKey] = useState<number>(0)
  const chartSectionRef = useRef<HTMLDivElement>(null)

  // Handle close position
  const handleClosePosition = async (symbol: string, side: 'long' | 'short') => {
    if (!selectedTraderId) {
      notify.error(t('selectTraderFirst', language) || 'Please select a trader first')
      return
    }

    const positionKey = `${symbol}-${side}`
    if (closingPositions.has(positionKey)) {
      return // Already closing
    }

    // Confirmation dialog
    const sideText = side === 'long' ? t('long', language) : t('short', language)
    const confirmMsg =
      t('confirmClosePosition', language, { symbol, side: sideText }) ||
      `Are you sure you want to close ${side} position for ${symbol}?`
    const confirmed = await confirmToast(confirmMsg, {
      title: language === 'zh' ? '确认平仓' : 'Confirm Close',
      okText: language === 'zh' ? '确认' : 'Confirm',
      cancelText: language === 'zh' ? '取消' : 'Cancel',
    })

    if (!confirmed) {
      return
    }

    setClosingPositions((prev) => new Set(prev).add(positionKey))

    try {
      await api.closePosition(selectedTraderId, symbol, side, 0) // 0 = close all
      notify.success(
        language === 'zh' ? '平仓成功' : 'Position closed successfully'
      )
      // Use SWR mutate to refresh data instead of reloading page
      await Promise.all([
        mutate(`positions-${selectedTraderId}`),
        mutate(`account-${selectedTraderId}`),
      ])
    } catch (error: any) {
      const errorMsg =
        error.message ||
        (language === 'zh' ? '平仓失败' : 'Failed to close position')
      notify.error(errorMsg)
    } finally {
      setClosingPositions((prev) => {
        const next = new Set(prev)
        next.delete(positionKey)
        return next
      })
    }
  }

  // If API failed with error, show empty state (likely backend not running)
  if (tradersError) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="text-center max-w-md mx-auto px-6">
          {/* Icon */}
          <div
            className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center"
            style={{
              background: 'rgba(0, 255, 127, 0.1)',
              border: '2px solid rgba(0, 255, 127, 0.3)',
            }}
          >
            <svg
              className="w-12 h-12"
              style={{ color: '#00CC66' }}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
          </div>

          {/* Title */}
          <h2 className="text-2xl font-bold mb-3" style={{ color: '#EAECEF' }}>
            {t('dashboardEmptyTitle', language)}
          </h2>

          {/* Description */}
          <p className="text-base mb-6" style={{ color: '#848E9C' }}>
            {t('dashboardEmptyDescription', language)}
          </p>

          {/* CTA Button */}
          <button
            onClick={onNavigateToTraders}
            className="px-6 py-3 rounded-lg font-semibold transition-all hover:scale-105 active:scale-95"
            style={{
              background: 'linear-gradient(135deg, #00CC66 0%, #00FF7F 100%)',
              color: '#0B0E11',
              boxShadow: '0 4px 12px rgba(0, 255, 127, 0.3)',
            }}
          >
            {t('goToTradersPage', language)}
          </button>
        </div>
      </div>
    )
  }

  // If traders is loaded and empty, show empty state
  if (traders && traders.length === 0) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="text-center max-w-md mx-auto px-6">
          {/* Icon */}
          <div
            className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center"
            style={{
              background: 'rgba(0, 255, 127, 0.1)',
              border: '2px solid rgba(0, 255, 127, 0.3)',
            }}
          >
            <svg
              className="w-12 h-12"
              style={{ color: '#00CC66' }}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
          </div>

          {/* Title */}
          <h2 className="text-2xl font-bold mb-3" style={{ color: '#EAECEF' }}>
            {t('dashboardEmptyTitle', language)}
          </h2>

          {/* Description */}
          <p className="text-base mb-6" style={{ color: '#848E9C' }}>
            {t('dashboardEmptyDescription', language)}
          </p>

          {/* CTA Button */}
          <button
            onClick={onNavigateToTraders}
            className="px-6 py-3 rounded-lg font-semibold transition-all hover:scale-105 active:scale-95"
            style={{
              background: 'linear-gradient(135deg, #00CC66 0%, #00FF7F 100%)',
              color: '#0B0E11',
              boxShadow: '0 4px 12px rgba(0, 255, 127, 0.3)',
            }}
          >
            {t('goToTradersPage', language)}
          </button>
        </div>
      </div>
    )
  }

  // If traders is still loading or selectedTrader is not ready, show skeleton
  if (!selectedTrader) {
    return (
      <div className="space-y-6">
        {/* Loading Skeleton - Binance Style */}
        <div className="binance-card p-6 animate-pulse">
          <div className="skeleton h-8 w-48 mb-3"></div>
          <div className="flex gap-4">
            <div className="skeleton h-4 w-32"></div>
            <div className="skeleton h-4 w-24"></div>
            <div className="skeleton h-4 w-28"></div>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="binance-card p-5 animate-pulse">
              <div className="skeleton h-4 w-24 mb-3"></div>
              <div className="skeleton h-8 w-32"></div>
            </div>
          ))}
        </div>
        <div className="binance-card p-6 animate-pulse">
          <div className="skeleton h-6 w-40 mb-4"></div>
          <div className="skeleton h-64 w-full"></div>
        </div>
      </div>
    )
  }

  return (
    <div>
      {/* Trader Header */}
      <div
        className="mb-6 rounded p-6 animate-scale-in"
        style={{
          background:
            'linear-gradient(135deg, rgba(0, 255, 127, 0.15) 0%, rgba(0, 255, 127, 0.05) 100%)',
          border: '1px solid rgba(0, 255, 127, 0.2)',
          boxShadow: '0 0 30px rgba(0, 255, 127, 0.15)',
        }}
      >
        <div className="flex items-start justify-between mb-3">
          <h2
            className="text-2xl font-bold flex items-center gap-2"
            style={{ color: '#EAECEF' }}
          >
            <span
              className="w-10 h-10 rounded-full flex items-center justify-center text-xl"
              style={{
                background: 'linear-gradient(135deg, #00CC66 0%, #00FF7F 100%)',
              }}
            >
              🤖
            </span>
            {selectedTrader.trader_name}
          </h2>

          {/* Trader Selector */}
          {traders && traders.length > 0 && (
            <div className="flex items-center gap-2">
              <span className="text-sm" style={{ color: '#848E9C' }}>
                {t('switchTrader', language)}:
              </span>
              <select
                value={selectedTraderId}
                onChange={(e) => onTraderSelect(e.target.value)}
                className="rounded px-3 py-2 text-sm font-medium cursor-pointer transition-colors"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              >
                {traders.map((trader) => (
                  <option key={trader.trader_id} value={trader.trader_id}>
                    {trader.trader_name}
                  </option>
                ))}
              </select>
            </div>
          )}
        </div>
        <div
          className="flex items-center gap-4 text-sm"
          style={{ color: '#848E9C' }}
        >
          <span>
            AI Model:{' '}
            <span
              className="font-semibold"
              style={{
                color: selectedTrader.ai_model.includes('qwen')
                  ? '#c084fc'
                  : '#60a5fa',
              }}
            >
              {getModelDisplayName(
                selectedTrader.ai_model.split('_').pop() ||
                  selectedTrader.ai_model
              )}
            </span>
          </span>
          {status && (
            <>
              <span>•</span>
              <span>Cycles: {status.call_count}</span>
              <span>•</span>
              <span>Runtime: {status.runtime_minutes} min</span>
            </>
          )}
        </div>
      </div>

      {/* Debug Info */}
      {account && (
        <div
          className="mb-4 p-3 rounded text-xs font-mono"
          style={{ background: '#1E2329', border: '1px solid #2B3139' }}
        >
          <div style={{ color: '#848E9C' }}>
            🔄 Last Update: {lastUpdate} | Total Equity:{' '}
            {account?.total_equity?.toFixed(2) || '0.00'} | Available:{' '}
            {account?.available_balance?.toFixed(2) || '0.00'} | P&L:{' '}
            {account?.total_pnl?.toFixed(2) || '0.00'} (
            {account?.total_pnl_pct?.toFixed(2) || '0.00'}%)
          </div>
        </div>
      )}

      {/* Account Overview */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
        <StatCard
          title={t('totalEquity', language)}
          value={`${account?.total_equity?.toFixed(2) || '0.00'} USDT`}
          change={account?.total_pnl_pct || 0}
          positive={(account?.total_pnl ?? 0) > 0}
        />
        <StatCard
          title={t('availableBalance', language)}
          value={`${account?.available_balance?.toFixed(2) || '0.00'} USDT`}
          subtitle={`${account?.available_balance && account?.total_equity ? ((account.available_balance / account.total_equity) * 100).toFixed(1) : '0.0'}% ${t('free', language)}`}
        />
        <StatCard
          title={t('totalPnL', language)}
          value={`${account?.total_pnl !== undefined && account.total_pnl >= 0 ? '+' : ''}${account?.total_pnl?.toFixed(2) || '0.00'} USDT`}
          change={account?.total_pnl_pct || 0}
          positive={(account?.total_pnl ?? 0) >= 0}
        />
        <StatCard
          title={t('positions', language)}
          value={`${account?.position_count || 0}`}
          subtitle={`${t('margin', language)}: ${account?.margin_used_pct?.toFixed(1) || '0.0'}%`}
        />
      </div>

      {/* 主要内容区：左右分屏 */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
        {/* 左侧：图表 + 持仓 */}
        <div className="space-y-6">
          {/* Chart Tabs (Equity / K-line) */}
          <div
            ref={chartSectionRef}
            className="chart-container animate-slide-in scroll-mt-32"
            style={{ animationDelay: '0.1s' }}
          >
            <ChartTabs
              traderId={selectedTrader.trader_id}
              selectedSymbol={selectedChartSymbol}
              updateKey={chartUpdateKey}
              exchangeId={selectedTrader.exchange_id}
            />
          </div>

          {/* Current Positions */}
          <div
            className="binance-card p-6 animate-slide-in"
            style={{ animationDelay: '0.15s' }}
          >
            <div className="flex items-center justify-between mb-5">
              <h2
                className="text-xl font-bold flex items-center gap-2"
                style={{ color: '#EAECEF' }}
              >
                📈 {t('currentPositions', language)}
              </h2>
              {positions && positions.length > 0 && (
                <div
                  className="text-xs px-3 py-1 rounded"
                  style={{
                    background: 'rgba(0, 255, 127, 0.1)',
                    color: '#00CC66',
                    border: '1px solid rgba(0, 255, 127, 0.2)',
                  }}
                >
                  {positions.length} {t('active', language)}
                </div>
              )}
            </div>
            {positions && positions.length > 0 ? (
              <div className="overflow-x-auto">
                <table className="w-full text-xs">
                  <thead className="text-left border-b border-gray-800">
                    <tr>
                      <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-left">
                        {t('symbol', language)}
                      </th>
                      <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-center">
                        {t('side', language)}
                      </th>
                      <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-center">
                        {language === 'zh' ? '操作' : 'Action'}
                      </th>
                      <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right" title={t('entryPrice', language)}>
                        {language === 'zh' ? '入场价' : 'Entry'}
                      </th>
                      <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right" title={t('markPrice', language)}>
                        {language === 'zh' ? '标记价' : 'Mark'}
                      </th>
                      <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right" title={t('quantity', language)}>
                        {language === 'zh' ? '数量' : 'Qty'}
                      </th>
                      <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right" title={t('positionValue', language)}>
                        {language === 'zh' ? '价值' : 'Value'}
                      </th>
                      <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-center" title={t('leverage', language)}>
                        {language === 'zh' ? '杠杆' : 'Lev.'}
                      </th>
                      <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right" title={t('unrealizedPnL', language)}>
                        {language === 'zh' ? '未实现盈亏' : 'uPnL'}
                      </th>
                      <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right" title={t('liqPrice', language)}>
                        {language === 'zh' ? '强平价' : 'Liq.'}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {positions.map((pos, i) => (
                      <tr
                        key={i}
                        className="border-b border-gray-800 last:border-0 transition-colors hover:bg-opacity-10 hover:bg-green-500 cursor-pointer"
                        onClick={() => {
                          setSelectedChartSymbol(pos.symbol)
                          setChartUpdateKey(Date.now())
                          // Smooth scroll to chart with ref
                          if (chartSectionRef.current) {
                            chartSectionRef.current.scrollIntoView({ behavior: 'smooth', block: 'start' })
                          }
                        }}
                      >
                        <td className="px-1 py-3 font-mono font-semibold whitespace-nowrap text-left">
                          {pos.symbol}
                        </td>
                        <td className="px-1 py-3 whitespace-nowrap text-center">
                          <span
                            className="px-1.5 py-0.5 rounded text-[10px] font-bold"
                            style={
                              pos.side === 'long'
                                ? {
                                    background: 'rgba(14, 203, 129, 0.1)',
                                    color: '#0ECB81',
                                  }
                                : {
                                    background: 'rgba(246, 70, 93, 0.1)',
                                    color: '#F6465D',
                                  }
                            }
                          >
                            {t(
                              pos.side === 'long' ? 'long' : 'short',
                              language
                            )}
                          </span>
                        </td>
                        <td className="px-1 py-3 whitespace-nowrap text-center">
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation() // Prevent row click
                              handleClosePosition(pos.symbol, pos.side as 'long' | 'short')
                            }}
                            disabled={closingPositions.has(`${pos.symbol}-${pos.side}`)}
                            className="btn-danger inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-semibold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed mx-auto"
                            title={language === 'zh' ? '平仓' : 'Close Position'}
                          >
                            {closingPositions.has(`${pos.symbol}-${pos.side}`) ? (
                              <Loader2 className="w-3 h-3 animate-spin" />
                            ) : (
                              <LogOut className="w-3 h-3" />
                            )}
                            {language === 'zh' ? '平仓' : 'Close'}
                          </button>
                        </td>
                        <td
                          className="px-1 py-3 font-mono whitespace-nowrap text-right"
                          style={{ color: '#EAECEF' }}
                        >
                          {pos.entry_price.toFixed(4)}
                        </td>
                        <td
                          className="px-1 py-3 font-mono whitespace-nowrap text-right"
                          style={{ color: '#EAECEF' }}
                        >
                          {pos.mark_price.toFixed(4)}
                        </td>
                        <td
                          className="px-1 py-3 font-mono whitespace-nowrap text-right"
                          style={{ color: '#EAECEF' }}
                        >
                          {pos.quantity.toFixed(4)}
                        </td>
                        <td
                          className="px-1 py-3 font-mono font-bold whitespace-nowrap text-right"
                          style={{ color: '#EAECEF' }}
                        >
                          {(pos.quantity * pos.mark_price).toFixed(2)}
                        </td>
                        <td
                          className="px-1 py-3 font-mono whitespace-nowrap text-center"
                          style={{ color: '#00CC66' }}
                        >
                          {pos.leverage}x
                        </td>
                        <td className="px-1 py-3 font-mono whitespace-nowrap text-right">
                          <span
                            style={{
                              color:
                                pos.unrealized_pnl >= 0 ? '#0ECB81' : '#F6465D',
                              fontWeight: 'bold',
                            }}
                          >
                            {pos.unrealized_pnl >= 0 ? '+' : ''}
                            {pos.unrealized_pnl.toFixed(2)}
                          </span>
                        </td>
                        <td
                          className="px-1 py-3 font-mono whitespace-nowrap text-right"
                          style={{ color: '#848E9C' }}
                        >
                          {pos.liquidation_price.toFixed(4)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="text-center py-16" style={{ color: '#848E9C' }}>
                <div className="text-6xl mb-4 opacity-50">📊</div>
                <div className="text-lg font-semibold mb-2">
                  {t('noPositions', language)}
                </div>
                <div className="text-sm">
                  {t('noActivePositions', language)}
                </div>
              </div>
            )}
          </div>
        </div>
        {/* 左侧结束 */}

        {/* 右侧：Recent Decisions - 卡片容器 */}
        <div
          className="binance-card p-6 animate-slide-in h-fit lg:sticky lg:top-24 lg:max-h-[calc(100vh-120px)]"
          style={{ animationDelay: '0.2s' }}
        >
          {/* 标题 */}
          <div
            className="flex items-center gap-3 mb-5 pb-4 border-b"
            style={{ borderColor: '#2B3139' }}
          >
            <div
              className="w-10 h-10 rounded-xl flex items-center justify-center text-xl"
              style={{
                background: 'linear-gradient(135deg, #6366F1 0%, #8B5CF6 100%)',
                boxShadow: '0 4px 14px rgba(99, 102, 241, 0.4)',
              }}
            >
              🧠
            </div>
            <div>
              <h2 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
                {t('recentDecisions', language)}
              </h2>
              {decisions && decisions.length > 0 && (
                <div className="text-xs" style={{ color: '#848E9C' }}>
                  {t('lastCycles', language, { count: decisions.length })}
                </div>
              )}
            </div>
          </div>

          {/* 决策列表 - 可滚动 */}
          <div
            className="space-y-4 overflow-y-auto pr-2"
            style={{ maxHeight: 'calc(100vh - 280px)' }}
          >
            {decisions && decisions.length > 0 ? (
              decisions.map((decision, i) => (
                <DecisionCard key={i} decision={decision} language={language} />
              ))
            ) : (
              <div className="py-16 text-center">
                <div className="text-6xl mb-4 opacity-30">🧠</div>
                <div
                  className="text-lg font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  {t('noDecisionsYet', language)}
                </div>
                <div className="text-sm" style={{ color: '#848E9C' }}>
                  {t('aiDecisionsWillAppear', language)}
                </div>
              </div>
            )}
          </div>
        </div>
        {/* 右侧结束 */}
      </div>

      {/* AI Learning & Performance Analysis */}
      <div className="mb-6 animate-slide-in" style={{ animationDelay: '0.3s' }}>
        <AILearning traderId={selectedTrader.trader_id} />
      </div>
    </div>
  )
}

// Stat Card Component - Binance Style Enhanced
function StatCard({
  title,
  value,
  change,
  positive,
  subtitle,
}: {
  title: string
  value: string
  change?: number
  positive?: boolean
  subtitle?: string
}) {
  return (
    <div className="stat-card animate-fade-in">
      <div
        className="text-xs mb-2 mono uppercase tracking-wider"
        style={{ color: '#848E9C' }}
      >
        {title}
      </div>
      <div
        className="text-2xl font-bold mb-1 mono"
        style={{ color: '#EAECEF' }}
      >
        {value}
      </div>
      {change !== undefined && (
        <div className="flex items-center gap-1">
          <div
            className="text-sm mono font-bold"
            style={{ color: positive ? '#0ECB81' : '#F6465D' }}
          >
            {positive ? '▲' : '▼'} {positive ? '+' : ''}
            {change.toFixed(2)}%
          </div>
        </div>
      )}
      {subtitle && (
        <div className="text-xs mt-2 mono" style={{ color: '#848E9C' }}>
          {subtitle}
        </div>
      )}
    </div>
  )
}

// Wrap App with providers
export default function AppWithProviders() {
  return (
    <LanguageProvider>
      <AuthProvider>
        <ConfirmDialogProvider>
          <App />
        </ConfirmDialogProvider>
      </AuthProvider>
    </LanguageProvider>
  )
}
