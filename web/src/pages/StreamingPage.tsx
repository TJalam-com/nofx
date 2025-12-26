import { useEffect, useState, useRef } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { motion, AnimatePresence } from 'framer-motion'
import useSWR from 'swr'
import { useAuth, isAdmin } from '../contexts/AuthContext'
import { ChartTabs } from '../components/ChartTabs'
import { AnimatedMetricCard } from '../components/streaming/AnimatedMetricCard'
import { AnimatedPositionTable } from '../components/streaming/AnimatedPositionTable'
import { AnimatedDecisionTimeline } from '../components/streaming/AnimatedDecisionTimeline'
import { AdminControls } from '../components/streaming/AdminControls'
import { api } from '../lib/api'
import { AccountInfo, StreamingConfig, TraderInfo } from '../types'
import { generateTraderSlug, parseTraderSlug } from '../lib/utils'
import { soundSystem } from '../lib/sound'
import { TrendingUp, DollarSign, Activity, PieChart, Loader2, AlertTriangle } from 'lucide-react'

export default function StreamingPage() {
  const { user, token } = useAuth()
  const navigate = useNavigate()
  const { slug } = useParams<{ slug?: string }>()
  const [selectedTraderId, setSelectedTraderId] = useState<string | undefined>(undefined)
  const [account, setAccount] = useState<AccountInfo | null>(null)
  const [traderNotFound, setTraderNotFound] = useState(false)

  // Get trader list (only when user is logged in)
  const { data: traders, error: tradersError, isLoading: tradersLoading } = useSWR<TraderInfo[]>(
    user && token ? 'traders' : null,
    api.getTraders,
    {
      refreshInterval: 10000,
      shouldRetryOnError: false,
    }
  )
  const [config, setConfig] = useState<StreamingConfig>({
    id: '',
    admin_id: user?.id || '',
    is_streaming: true,
    stream_mode: 'full',
    show_decisions: true,
    show_positions: true,
    animation_speed: 2,
    theme: 'dark',
    auto_switch_charts: true,
    auto_switch_interval: 10,
    auto_scroll_ai_analysis: true,
    auto_scroll_page: false,
    auto_scroll_page_speed: 1,
    sound_enabled: true,
    watermark_text: 'AI Trading 24×7 powered by Nofx',
    widget_visibility: {
      equity: true,
      positions: true,
      decisions: true,
      metrics: true,
      ai_process: false,
    },
    layout_type: 'chart_focus',
  })
  const [adminControlsMinimized, setAdminControlsMinimized] = useState(false)
  const [selectedChartSymbol] = useState<string | undefined>(undefined)
  const [chartUpdateKey] = useState(0)
  const [chartAnimationComplete, setChartAnimationComplete] = useState(false)
  const [aiAnalysisComplete, setAiAnalysisComplete] = useState(false)
  const [aiScrollComplete, setAiScrollComplete] = useState(false)

  // Check admin access
  useEffect(() => {
    if (!isAdmin(user)) {
      navigate('/dashboard')
      return
    }
  }, [user, navigate])

  // Update sound system when config changes
  useEffect(() => {
    soundSystem.setEnabled(config.sound_enabled)
  }, [config.sound_enabled])

  // Reset animation completion flags when config changes
  useEffect(() => {
    if (config.auto_scroll_page) {
      setChartAnimationComplete(false)
      setAiAnalysisComplete(false)
    }
  }, [config.auto_scroll_page, selectedTraderId])

  // Ref to track if auto-scroll is still enabled (to cancel promises)
  const autoScrollEnabledRef = useRef(false)
  const waitForAnimationsTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Human-like auto-scroll for full page (waits for animations to complete each cycle)
  useEffect(() => {
    if (!config.auto_scroll_page) {
      autoScrollEnabledRef.current = false
      return
    }

    // Mark as enabled
    autoScrollEnabledRef.current = true


    let rafId: number | null = null
    let timeoutId: ReturnType<typeof setTimeout> | null = null
    let scrollCycleActive = false

    const getDocumentHeight = () =>
      Math.max(document.body.scrollHeight, document.documentElement.scrollHeight)

    const easeOutCubic = (t: number) => 1 - Math.pow(1 - t, 3)

    const waitForAnimations = (): Promise<void> => {
      return new Promise((resolve, reject) => {
        // Clear any existing timeout
        if (waitForAnimationsTimeoutRef.current) {
          clearTimeout(waitForAnimationsTimeoutRef.current)
          waitForAnimationsTimeoutRef.current = null
        }

        let checkTimeoutId: ReturnType<typeof setTimeout> | null = null
        let aiScrollWaited = false
        let chartCycleWaited = false
        const aiScrollStartTime = performance.now()
        let chartAnimationStartTime = performance.now()
        const AI_SCROLL_TIMEOUT = 5000 // 5 seconds timeout for AI scroll
        const CHART_ANIMATION_TIMEOUT = 10000 // 10 seconds timeout for chart animation
        
        const checkAnimations = () => {
          // Check if auto-scroll was disabled - cancel if so
          if (!autoScrollEnabledRef.current) {
            if (checkTimeoutId) clearTimeout(checkTimeoutId)
            reject(new Error('Auto-scroll disabled'))
            return
          }

          // Step 1: Wait for AI scroll to complete first (individual window auto-scroll)
          // Add timeout: if AI scroll doesn't complete within 5 seconds, skip it
          if (!aiScrollWaited) {
            const elapsed = performance.now() - aiScrollStartTime
            if (elapsed >= AI_SCROLL_TIMEOUT) {
              // Timeout reached, skip AI scroll wait
              aiScrollWaited = true
              chartAnimationStartTime = performance.now() // Start timing chart animation when AI scroll times out
              checkAnimations()
              return
            }
            
            if (aiScrollComplete) {
              aiScrollWaited = true
              chartAnimationStartTime = performance.now() // Start timing chart animation when AI scroll completes
              // Wait a moment for scroll to fully settle
              checkTimeoutId = setTimeout(() => {
                if (autoScrollEnabledRef.current) {
                  checkAnimations()
                } else {
                  reject(new Error('Auto-scroll disabled'))
                }
              }, 500)
              return
            } else {
              // Keep checking for AI scroll completion
              checkTimeoutId = setTimeout(() => {
                if (autoScrollEnabledRef.current) {
                  checkAnimations()
                } else {
                  reject(new Error('Auto-scroll disabled'))
                }
              }, 100)
              return
            }
          }
          
          // Step 2: After AI scroll completes, wait for 1 cycle of tab switch to complete
          // Add timeout: if chart animation doesn't complete within 10 seconds, skip it
          if (!chartCycleWaited) {
            const chartElapsed = performance.now() - chartAnimationStartTime
            if (chartElapsed >= CHART_ANIMATION_TIMEOUT) {
              // Timeout reached, skip chart animation wait but still wait 2 seconds before resolving
              chartCycleWaited = true
              // Wait 2 seconds before resolving (same as when chart animation completes)
              checkTimeoutId = setTimeout(() => {
                if (autoScrollEnabledRef.current) {
                  resolve()
                } else {
                  reject(new Error('Auto-scroll disabled'))
                }
              }, 2000)
              waitForAnimationsTimeoutRef.current = checkTimeoutId
              return
            }
            
            if (chartAnimationComplete) {
              chartCycleWaited = true
              // Step 3: Wait 2 seconds after tab switch cycle completion
              checkTimeoutId = setTimeout(() => {
                if (autoScrollEnabledRef.current) {
                  resolve()
                } else {
                  reject(new Error('Auto-scroll disabled'))
                }
              }, 2000)
              waitForAnimationsTimeoutRef.current = checkTimeoutId
              return
            } else {
              // Keep checking for chart animation completion
              checkTimeoutId = setTimeout(() => {
                if (autoScrollEnabledRef.current) {
                  checkAnimations()
                } else {
                  reject(new Error('Auto-scroll disabled'))
                }
              }, 100)
              return
            }
          }
        }
        checkAnimations()
      })
    }

    const scrollSegment = async () => {
      const maxScroll = getDocumentHeight() - window.innerHeight
      const current = window.scrollY

      // If near bottom, pause then jump to top and wait for next animation cycle
      if (current >= maxScroll - 50) {
        timeoutId = setTimeout(() => {
          window.scrollTo({ top: 0, behavior: 'smooth' })
          // Reset completion flags to wait for next animation cycle
          setChartAnimationComplete(false)
          setAiAnalysisComplete(false)
          setAiScrollComplete(false)
          // Wait for the full sequence: AI scroll -> Tab switch cycle -> 2 seconds -> Page scroll
          waitForAnimations().then(() => {
            if (autoScrollEnabledRef.current && scrollCycleActive) {
              scrollCycleActive = true
              startScrollCycle()
            }
          }).catch(() => {
            // Auto-scroll was disabled, ignore
          })
        }, (1500 + Math.random() * 800) / config.auto_scroll_page_speed)
        return
      }

      const speed = config.auto_scroll_page_speed || 1
      const delta = (200 + Math.random() * 300) * speed // 200-500px scaled by speed
      const target = Math.min(maxScroll, current + delta)
      const duration = (800 + Math.random() * 800) / speed // faster speed => shorter duration
      const start = performance.now()
      const startY = current
      const distance = target - startY

      const animate = (time: number) => {
        const elapsed = time - start
        const progress = Math.min(1, elapsed / duration)
        const eased = easeOutCubic(progress)
        window.scrollTo({ top: startY + distance * eased })
        if (progress < 1) {
          rafId = requestAnimationFrame(animate)
        } else {
          // After scrolling segment, reset flags and wait for next animation cycle
          setChartAnimationComplete(false)
          setAiAnalysisComplete(false)
          setAiScrollComplete(false)
          // Wait for the full sequence: AI scroll -> Tab switch cycle -> 2 seconds -> Page scroll
          waitForAnimations().then(() => {
            if (autoScrollEnabledRef.current && scrollCycleActive) {
              scrollCycleActive = true
              startScrollCycle()
            }
          }).catch(() => {
            // Auto-scroll was disabled, ignore
          })
        }
      }

      rafId = requestAnimationFrame(animate)
    }

    const startScrollCycle = () => {
      if (!scrollCycleActive || !autoScrollEnabledRef.current) return
      scrollSegment()
    }

    // Wait for the proper sequence: AI scroll -> Tab switch cycle -> 2 seconds -> Page scroll
    // Don't start scrolling immediately - wait for the full sequence to complete
    waitForAnimations().then(() => {
      if (autoScrollEnabledRef.current) {
        scrollCycleActive = true
        startScrollCycle()
      }
    }).catch(() => {
      // Auto-scroll was disabled, ignore
    })

    return () => {
      autoScrollEnabledRef.current = false
      scrollCycleActive = false
      if (rafId) cancelAnimationFrame(rafId)
      if (timeoutId) clearTimeout(timeoutId)
      if (waitForAnimationsTimeoutRef.current) {
        clearTimeout(waitForAnimationsTimeoutRef.current)
        waitForAnimationsTimeoutRef.current = null
      }
    }
  }, [config.auto_scroll_page, config.auto_scroll_page_speed, chartAnimationComplete, aiAnalysisComplete, aiScrollComplete])

  // Resolve trader ID from slug
  useEffect(() => {
    if (!slug) {
      // No slug provided, redirect to dashboard
      navigate('/dashboard')
      return
    }

    // Ensure traders is an array before processing
    const safeTraders = traders || []
    if (safeTraders.length === 0) {
      // Traders list not loaded yet, wait
      return
    }

    let traderId: string | undefined = undefined

    // Priority 1: Check slug format from URL path (/stream/:slug)
    // Try to find trader by matching slug
    for (const trader of safeTraders) {
      const traderSlug = generateTraderSlug(trader.trader_name, trader.trader_id)
      if (traderSlug === slug) {
        traderId = trader.trader_id
        break
      }
    }

    // Priority 2: Fallback to id4 matching
    if (!traderId) {
      const id4 = parseTraderSlug(slug)
      if (id4) {
        const found = safeTraders.find((t) => t.trader_id.endsWith(id4))
        if (found) {
          traderId = found.trader_id
        }
      }
    }

    if (traderId) {
      setSelectedTraderId(traderId)
      setTraderNotFound(false)
    } else {
      // Trader not found
      setTraderNotFound(true)
      setSelectedTraderId(undefined)
    }
  }, [slug, traders, navigate])

  // Load account data
  useEffect(() => {
    if (!selectedTraderId) return

    const loadAccount = async () => {
      try {
        const accountData = await api.getAccount(selectedTraderId)
        setAccount(accountData)
      } catch (error) {
        console.error('Failed to load account:', error)
      }
    }

    loadAccount()

    // Poll for account updates (WebSocket not implemented yet)
    const pollAccount = async () => {
      try {
        const updatedAccount = await api.getAccount(selectedTraderId)
        setAccount(updatedAccount)
      } catch (error) {
        console.error('Failed to poll account data:', error)
      }
    }

    // Poll every 3 seconds for account updates
    const interval = setInterval(pollAccount, 3000)

    return () => clearInterval(interval)
  }, [selectedTraderId])

  const handleConfigChange = (newConfig: Partial<StreamingConfig>) => {
    setConfig((prev: StreamingConfig) => ({ ...prev, ...newConfig }))
    // Here you would save to backend
    console.log('Config updated:', newConfig)
  }

  // Loading state: traders list is loading
  if (tradersLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen" style={{ background: 'var(--navy-primary)' }}>
        <div className="text-center">
          <Loader2 className="w-8 h-8 mx-auto mb-4 animate-spin" style={{ color: 'var(--green-primary)' }} />
          <h2 className="text-2xl font-bold mb-2" style={{ color: '#EAECEF' }}>Loading Stream...</h2>
          <p className="text-sm" style={{ color: '#848E9C' }}>Resolving trader information</p>
        </div>
      </div>
    )
  }

  // Error state: traders list failed to load
  if (tradersError) {
    return (
      <div className="flex items-center justify-center min-h-screen" style={{ background: 'var(--navy-primary)' }}>
        <div className="text-center">
          <AlertTriangle className="w-8 h-8 mx-auto mb-4" style={{ color: '#F6465D' }} />
          <h2 className="text-2xl font-bold mb-2" style={{ color: '#EAECEF' }}>Failed to Load Traders</h2>
          <p className="text-sm mb-4" style={{ color: '#848E9C' }}>Unable to fetch trader list</p>
          <button
            onClick={() => navigate('/dashboard')}
            className="px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
            style={{
              background: 'var(--navy-dark)',
              border: '1px solid var(--navy-light)',
              color: '#EAECEF',
            }}
          >
            Go to Dashboard
          </button>
        </div>
      </div>
    )
  }

  // Error state: trader not found
  if (traderNotFound || !selectedTraderId) {
    return (
      <div className="flex items-center justify-center min-h-screen" style={{ background: 'var(--navy-primary)' }}>
        <div className="text-center">
          <AlertTriangle className="w-8 h-8 mx-auto mb-4" style={{ color: '#F6465D' }} />
          <h2 className="text-2xl font-bold mb-2" style={{ color: '#EAECEF' }}>Trader Not Found</h2>
          <p className="text-sm mb-4" style={{ color: '#848E9C' }}>
            The trader "{slug}" could not be found
          </p>
          <button
            onClick={() => navigate('/dashboard')}
            className="px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
            style={{
              background: 'var(--navy-dark)',
              border: '1px solid var(--navy-light)',
              color: '#EAECEF',
            }}
          >
            Go to Dashboard
          </button>
        </div>
      </div>
    )
  }

  const layoutClasses: Record<string, string> = {
    dashboard: 'grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 lg:gap-6 items-start',
    chart_focus: 'flex flex-col gap-4 lg:gap-6',
    position_focus: 'grid grid-cols-1 lg:grid-cols-2 gap-4 lg:gap-6 items-start',
  }

  return (
    <div className="min-h-screen" style={{ background: 'var(--navy-primary)' }}>
      {/* Compact Header */}
      <motion.div
        className="sticky top-0 z-40 py-2 px-4"
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        style={{
          background: 'var(--navy-primary)',
          borderBottom: '1px solid var(--navy-light)',
          boxShadow: '0 2px 4px rgba(0, 0, 0, 0.1)'
        }}
      >
        <div className="flex items-center justify-between max-w-[1920px] mx-auto">
          <div className="flex items-center gap-3">
            <motion.div
              animate={{ rotate: config.is_streaming ? 360 : 0 }}
              transition={{ duration: 2, repeat: config.is_streaming ? Infinity : 0, ease: 'linear' }}
            >
              <Activity className="w-4 h-4" style={{ color: 'var(--green-primary)' }} />
            </motion.div>
            <span className="text-sm font-bold" style={{ color: '#EAECEF' }}>
              Live Trading Stream - {(traders || []).find(t => t.trader_id === selectedTraderId)?.trader_name || selectedTraderId}
            </span>
          </div>

          <div className="flex items-center gap-3">
            <motion.div
              className="flex items-center gap-1.5 px-2 py-1 rounded text-xs font-semibold"
              style={{
                background: config.is_streaming
                  ? 'rgba(14, 203, 129, 0.1)'
                  : 'rgba(246, 70, 93, 0.1)',
                border: `1px solid ${config.is_streaming ? '#0ECB81' : '#F6465D'}`,
              }}
              animate={config.is_streaming ? { opacity: [1, 0.7, 1] } : {}}
              transition={{ duration: 1, repeat: config.is_streaming ? Infinity : 0 }}
            >
              <motion.div
                className="w-1.5 h-1.5 rounded-full"
                style={{ background: config.is_streaming ? '#0ECB81' : '#F6465D' }}
                animate={config.is_streaming ? { scale: [1, 1.2, 1] } : {}}
                transition={{ duration: 0.8, repeat: config.is_streaming ? Infinity : 0 }}
              />
              <span style={{ color: '#EAECEF' }}>
                {config.is_streaming ? 'LIVE' : 'OFFLINE'}
              </span>
            </motion.div>

            <button
              onClick={() => navigate('/dashboard')}
              className="px-3 py-1 rounded text-xs font-semibold transition-all hover:scale-105"
              style={{
                background: 'var(--navy-dark)',
                border: '1px solid var(--navy-light)',
                color: '#EAECEF',
              }}
            >
              Exit
            </button>
          </div>
        </div>
      </motion.div>

      {/* Main Content */}
      <div className="pt-2 pb-4 px-4 md:pt-4 md:pb-6 md:px-6 lg:pt-6 lg:pb-8 lg:px-8">
        <motion.div
          className={layoutClasses[config.layout_type]}
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 0.5 }}
        >
          {/* Equity Chart - Always visible in dashboard layout */}
          <AnimatePresence>
            {config.widget_visibility.equity && (
              <motion.div
                className={config.layout_type === 'chart_focus' ? 'col-span-full' : 'md:col-span-2 lg:col-span-2'}
                initial={{ opacity: 0, scale: 0.9 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.9 }}
                transition={{ duration: 0.3 }}
              >
                <div className="h-fit min-h-[400px]">
                  <ChartTabs
                    traderId={selectedTraderId}
                    selectedSymbol={selectedChartSymbol}
                    updateKey={chartUpdateKey}
                    exchangeId={(traders || []).find(t => t.trader_id === selectedTraderId)?.exchange_id}
                    autoSwitchEnabled={config.auto_switch_charts}
                    autoSwitchInterval={config.auto_switch_interval}
                    onTabSwitchComplete={() => {
                      if (config.auto_scroll_page) {
                        setChartAnimationComplete(true)
                      }
                    }}
                  />
                </div>
              </motion.div>
            )}
          </AnimatePresence>

          {/* Metrics Cards */}
          <AnimatePresence>
            {config.widget_visibility.metrics && (
              <motion.div
                className="lg:col-span-1"
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: 20 }}
                transition={{ duration: 0.3, delay: 0.1 }}
              >
                <div className="binance-card p-6 h-fit min-h-[200px] flex flex-col">
                  <h3 className="text-xl font-bold mb-4 flex items-center gap-2" style={{ color: '#EAECEF' }}>
                    <Activity className="w-5 h-5" style={{ color: 'var(--green-primary)' }} />
                    Metrics
                  </h3>
                  <motion.div
                    className="grid grid-cols-2 lg:grid-cols-4 gap-4 flex-1"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ duration: 0.3 }}
                  >
                    {[
                      {
                        title: "Total Equity",
                        value: account?.total_equity || 0,
                        change: account?.total_pnl_pct,
                        positive: (account?.total_pnl || 0) >= 0,
                        color: "#0ECB81",
                        icon: <DollarSign size={16} />
                      },
                      {
                        title: "Available Balance",
                        value: account?.available_balance || 0,
                        subtitle: `${account?.available_balance && account?.total_equity ? ((account.available_balance / account.total_equity) * 100).toFixed(1) : '0.0'}% free`,
                        color: "#60a5fa"
                      },
                      {
                        title: "Positions",
                        value: account?.position_count || 0,
                        subtitle: `Margin: ${account?.margin_used_pct?.toFixed(1) || '0.0'}%`,
                        color: "#c084fc",
                        icon: <PieChart size={16} />
                      },
                      {
                        title: "Daily P&L",
                        value: account?.daily_pnl || 0,
                        change: account?.daily_pnl && account?.total_equity ? (account.daily_pnl / account.total_equity) * 100 : 0,
                        positive: (account?.daily_pnl || 0) >= 0,
                        color: "#f59e0b",
                        icon: <TrendingUp size={16} />
                      }
                    ].map((metric, index) => (
                      <motion.div
                        key={metric.title}
                        initial={{
                          opacity: 0,
                          y: 20,
                          scale: 0.9
                        }}
                        animate={{
                          opacity: 1,
                          y: 0,
                          scale: 1
                        }}
                        transition={{
                          duration: 0.4,
                          delay: index * 0.1,
                          ease: [0.34, 1.56, 0.64, 1] // Bounce effect
                        }}
                        whileHover={{
                          scale: 1.02,
                          transition: { duration: 0.2 }
                        }}
                        whileTap={{ scale: 0.98 }}
                      >
                        <AnimatedMetricCard
                          title={metric.title}
                          value={metric.value}
                          change={metric.change}
                          positive={metric.positive}
                          subtitle={metric.subtitle}
                          animationSpeed={config.animation_speed}
                          color={metric.color}
                          icon={metric.icon}
                        />
                      </motion.div>
                    ))}
                  </motion.div>
                </div>
              </motion.div>
            )}
          </AnimatePresence>

          {/* Positions Table */}
          <AnimatePresence>
            {config.widget_visibility.positions && (
              <motion.div
                className={config.layout_type === 'position_focus' ? 'col-span-full' : 'md:col-span-1 lg:col-span-1'}
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: 20 }}
                transition={{ duration: 0.3, delay: 0.2 }}
              >
                <div className="binance-card p-6 h-fit min-h-[400px]">
                  <AnimatedPositionTable
                    traderId={selectedTraderId}
                    animationSpeed={config.animation_speed}
                    maxHeight="500px"
                  />
                </div>
              </motion.div>
            )}
          </AnimatePresence>

          {/* AI Decisions Timeline */}
          <AnimatePresence>
            {config.widget_visibility.decisions && (
              <motion.div
                className={config.layout_type === 'chart_focus' ? 'col-span-full' : 'md:col-span-1 lg:col-span-1'}
                initial={{ opacity: 0, x: -20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -20 }}
                transition={{ duration: 0.3, delay: 0.3 }}
              >
                <div className="h-fit min-h-[400px]">
                  <AnimatedDecisionTimeline
                    traderId={selectedTraderId}
                    limit={10}
                    animationSpeed={config.animation_speed}
                    maxHeight="500px"
                    autoScrollAiAnalysis={config.auto_scroll_ai_analysis}
                    onMostRecentAnalysisComplete={() => {
                      if (config.auto_scroll_page) {
                        setAiAnalysisComplete(true)
                      }
                    }}
                    onMostRecentScrollComplete={() => {
                      if (config.auto_scroll_page) {
                        setAiScrollComplete(true)
                      }
                    }}
                  />
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </motion.div>
      </div>

      {/* Watermark */}
      <AnimatePresence>
        {config.watermark_text && (
          <motion.div
            className="fixed bottom-4 right-4 text-sm font-bold pointer-events-none"
            style={{
              color: 'rgba(0, 255, 127, 0.3)',
              transform: 'rotate(-15deg)',
            }}
            initial={{ opacity: 0, scale: 0.5 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.5 }}
            transition={{ duration: 0.5 }}
          >
            {config.watermark_text}
          </motion.div>
        )}
      </AnimatePresence>

      {/* Admin Controls */}
      {isAdmin(user) && (
        <AdminControls
          isMinimized={adminControlsMinimized}
          onMinimize={() => setAdminControlsMinimized(!adminControlsMinimized)}
          config={config}
          onConfigChange={handleConfigChange}
        />
      )}
    </div>
  )
}

