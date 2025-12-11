import { useState, useMemo } from 'react'
import { Trophy, Medal, Clock, Users } from 'lucide-react'
import useSWR from 'swr'
import { api } from '../lib/api'
import type { CompetitionData } from '../types'
import { ComparisonChart } from './ComparisonChart'
import { TraderConfigViewModal } from './TraderConfigViewModal'
import { LeaderSpotlight } from './LeaderSpotlight'
import { getTraderColor } from '../utils/traderColors'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth, isFollower } from '../contexts/AuthContext'
import { t } from '../i18n/translations'
import { toast } from 'sonner'

export function CompetitionPage() {
  const { language } = useLanguage()
  const { user, token } = useAuth()
  const userIsFollower = isFollower(user)
  const [selectedTrader, setSelectedTrader] = useState<any>(null)
  const [isModalOpen, setIsModalOpen] = useState(false)

  const { data: competition } = useSWR<CompetitionData>(
    'competition',
    api.getCompetition,
    {
      refreshInterval: 15000, // 15秒刷新（竞赛数据不需要太频繁更新）
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    }
  )

  // Calculate last update time - must be called before any conditional returns
  const lastUpdateTime = useMemo(() => {
    return new Date().toLocaleTimeString(language === 'zh' ? 'zh-CN' : 'en-US', {
      hour: '2-digit',
      minute: '2-digit',
    })
  }, [language])

  const handleTraderClick = async (traderId: string) => {
    try {
      const traderConfig = await api.getPublicTraderConfig(traderId)
      setSelectedTrader(traderConfig)
      setIsModalOpen(true)
    } catch (error) {
      console.error('Failed to fetch trader config:', error)
      // 对于未登录用户，不显示详细配置，这是正常行为
      // 竞赛页面主要用于查看排行榜和基本信息
    }
  }

  const closeModal = () => {
    setIsModalOpen(false)
    setSelectedTrader(null)
  }

  const handleCopyTrader = (traderId: string) => {
    // Check if this trader is a follower trader (has followed_trader_id)
    const trader = competition?.traders?.find((t) => t.trader_id === traderId)
    if (trader && (trader as any).followed_trader_id) {
      toast.error(t('cannotCopyFollowerTrader', language) || 'Cannot copy follower traders. Only original traders can be copied.')
      return
    }
    
    // Navigate to traders page and open create modal with the trader ID to copy
    // We'll use sessionStorage to pass the trader ID to copy
    if (user && token) {
      // Store the trader ID in sessionStorage before navigation
      sessionStorage.setItem('copyTraderId', traderId)
      
      // Navigate to traders page with action=copy query parameter
      // Using window.location.href ensures a full page reload, which guarantees
      // sessionStorage persists and the useEffect in AITradersPage will run
      window.location.href = '/traders?action=copy'
    } else {
      // If not logged in, redirect to login
      window.location.href = '/login?redirect=/competition'
    }
  }

  if (!competition) {
    return (
      <div className="space-y-6 md:space-y-8">
        <div className="competition-header animate-pulse">
          <div className="flex items-center justify-between">
            <div className="space-y-4 flex-1">
              <div className="skeleton h-10 w-64"></div>
              <div className="skeleton h-5 w-48"></div>
            </div>
            <div className="skeleton h-20 w-40 rounded-lg"></div>
          </div>
        </div>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 md:gap-8">
          <div className="binance-card-enhanced p-6 md:p-8">
            <div className="skeleton h-6 w-40 mb-6"></div>
            <div className="skeleton h-64 w-full rounded"></div>
          </div>
          <div className="binance-card-enhanced p-6 md:p-8">
            <div className="skeleton h-6 w-40 mb-6"></div>
            <div className="space-y-3">
              <div className="skeleton h-24 w-full rounded-lg"></div>
              <div className="skeleton h-24 w-full rounded-lg"></div>
              <div className="skeleton h-24 w-full rounded-lg"></div>
            </div>
          </div>
        </div>
      </div>
    )
  }

  // 如果有数据返回但没有交易员，显示空状态
  if (!competition.traders || competition.traders.length === 0) {
    return (
      <div className="space-y-6 md:space-y-8 animate-fade-in">
        {/* Competition Header - Enhanced */}
        <div className="competition-header">
          <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4 md:gap-6">
            <div className="flex items-center gap-4 md:gap-5">
              <div
                className="w-12 h-12 md:w-16 md:h-16 rounded-2xl flex items-center justify-center trophy-icon relative"
                style={{
                  background: 'linear-gradient(135deg, var(--green-primary) 0%, var(--green-light) 100%)',
                }}
              >
                <Trophy
                  className="w-7 h-7 md:w-9 md:h-9 relative z-10"
                  style={{ color: 'var(--navy-primary)' }}
                />
              </div>
              <div>
                <h1
                  className="text-2xl md:text-3xl font-bold flex items-center gap-3 mb-1"
                  style={{ color: 'var(--text-white)' }}
                >
                  {t('aiCompetition', language)}
                  <span
                    className="text-xs md:text-sm font-semibold px-3 py-1.5 rounded-lg"
                    style={{
                      background: 'var(--success-bg)',
                      color: 'var(--green-primary)',
                      border: '1px solid var(--success-border)',
                    }}
                  >
                    0 {t('traders', language)}
                  </span>
                </h1>
                <p className="text-sm md:text-base mt-1" style={{ color: 'var(--text-gray-light)' }}>
                  {t('liveBattle', language)}
                </p>
              </div>
            </div>
          </div>
        </div>

        {/* Empty State */}
        <div className="binance-card-enhanced p-12 md:p-16 text-center">
          <Trophy
            className="w-20 h-20 md:w-24 md:h-24 mx-auto mb-6 opacity-30"
            style={{ color: 'var(--text-gray-light)' }}
          />
          <h3 className="text-xl md:text-2xl font-bold mb-3" style={{ color: 'var(--text-white)' }}>
            {t('noTraders', language)}
          </h3>
          <p className="text-base md:text-lg" style={{ color: 'var(--text-gray-light)' }}>
            {t('createFirstTrader', language)}
          </p>
        </div>
      </div>
    )
  }

  // 按收益率排序
  const sortedTraders = [...competition.traders].sort(
    (a, b) => b.total_pnl_pct - a.total_pnl_pct
  )

  // 找出领先者
  const leader = sortedTraders[0]

  return (
    <div className="space-y-6 md:space-y-8 animate-fade-in">
      {/* Redesigned Hero Section - Multi-Card Layout */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 md:gap-6">
        {/* Card 1: Competition Title */}
        <div
          className="binance-card-enhanced p-5 md:p-6 animate-slide-in relative overflow-hidden"
          style={{
            animationDelay: '0.1s',
            background: 'linear-gradient(135deg, rgba(0, 255, 127, 0.05) 0%, var(--navy-dark) 100%)',
          }}
        >
          <div className="flex items-center gap-4">
            <div
              className="w-12 h-12 md:w-14 md:h-14 rounded-xl flex items-center justify-center trophy-icon relative flex-shrink-0"
              style={{
                background: 'linear-gradient(135deg, var(--green-primary) 0%, var(--green-light) 100%)',
              }}
            >
              <Trophy
                className="w-6 h-6 md:w-7 md:h-7 relative z-10"
                style={{ color: 'var(--navy-primary)' }}
              />
              <div
                className="absolute inset-0 rounded-xl opacity-50"
                style={{
                  background: 'radial-gradient(circle, rgba(0, 255, 127, 0.6) 0%, transparent 70%)',
                }}
              />
            </div>
            <div className="flex-1 min-w-0">
              <h1
                className="text-xl md:text-2xl font-bold mb-1 truncate"
                style={{ color: 'var(--text-white)' }}
              >
                {t('aiCompetition', language)}
              </h1>
              <p className="text-xs md:text-sm truncate" style={{ color: 'var(--text-gray-light)' }}>
                {t('liveBattle', language)}
              </p>
            </div>
          </div>
        </div>

        {/* Card 2: Live Stats */}
        <div
          className="binance-card-enhanced p-5 md:p-6 animate-slide-in relative overflow-hidden"
          style={{
            animationDelay: '0.15s',
            background: 'linear-gradient(135deg, rgba(0, 255, 127, 0.05) 0%, var(--navy-dark) 100%)',
          }}
        >
          <div className="flex items-center gap-4">
            <div
              className="w-12 h-12 md:w-14 md:h-14 rounded-xl flex items-center justify-center flex-shrink-0"
              style={{
                background: 'rgba(0, 255, 127, 0.1)',
                border: '1px solid rgba(0, 255, 127, 0.2)',
              }}
            >
              <Users className="w-6 h-6 md:w-7 md:h-7" style={{ color: 'var(--green-primary)' }} />
            </div>
            <div className="flex-1 min-w-0">
              <div
                className="text-xs uppercase tracking-wider mb-1 font-semibold"
                style={{ color: 'var(--text-gray-light)' }}
              >
                {t('traders', language)}
              </div>
              <div
                className="text-2xl md:text-3xl font-bold mono"
                style={{ color: 'var(--text-white)' }}
              >
                {competition.count}
              </div>
              <div className="flex items-center gap-2 mt-1">
                <div
                  className="w-2 h-2 rounded-full pulse-live"
                  style={{
                    background: 'var(--green-primary)',
                    boxShadow: '0 0 8px var(--green-primary)',
                  }}
                />
                <span
                  className="text-xs font-semibold"
                  style={{ color: 'var(--green-primary)' }}
                >
                  {t('live', language)}
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Card 3: Last Update */}
        <div
          className="binance-card-enhanced p-5 md:p-6 animate-slide-in relative overflow-hidden"
          style={{
            animationDelay: '0.2s',
            background: 'linear-gradient(135deg, rgba(0, 255, 127, 0.05) 0%, var(--navy-dark) 100%)',
          }}
        >
          <div className="flex items-center gap-4">
            <div
              className="w-12 h-12 md:w-14 md:h-14 rounded-xl flex items-center justify-center flex-shrink-0"
              style={{
                background: 'rgba(0, 255, 127, 0.1)',
                border: '1px solid rgba(0, 255, 127, 0.2)',
              }}
            >
              <Clock className="w-6 h-6 md:w-7 md:h-7" style={{ color: 'var(--green-primary)' }} />
            </div>
            <div className="flex-1 min-w-0">
              <div
                className="text-xs uppercase tracking-wider mb-1 font-semibold"
                style={{ color: 'var(--text-gray-light)' }}
              >
                {t('lastUpdate', language) || 'Last Update'}
              </div>
              <div
                className="text-lg md:text-xl font-bold mono"
                style={{ color: 'var(--text-white)' }}
              >
                {lastUpdateTime}
              </div>
              <div
                className="text-xs mt-1"
                style={{ color: 'var(--text-gray-light)' }}
              >
                {t('realTime', language)}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Leader Spotlight - Prominent Display */}
      {leader && (
        <LeaderSpotlight
          leader={leader}
          allTraders={sortedTraders}
          onViewDetails={() => handleTraderClick(leader.trader_id)}
        />
      )}

      {/* Left/Right Split: Performance Chart + Leaderboard */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 md:gap-8">
        {/* Left: Performance Comparison Chart */}
        <div
          className="binance-card-enhanced p-6 md:p-8 animate-slide-in"
          style={{ animationDelay: '0.1s' }}
        >
          <div className="flex items-center justify-between mb-6">
            <h2
              className="text-xl md:text-2xl font-bold flex items-center gap-2"
              style={{ color: 'var(--text-white)' }}
            >
              {t('performanceComparison', language)}
            </h2>
            <div
              className="text-xs px-2.5 py-1 rounded-md font-semibold pulse-live"
              style={{
                background: 'var(--success-bg)',
                color: 'var(--green-primary)',
                border: '1px solid var(--success-border)',
              }}
            >
              {t('realTimePnL', language)}
            </div>
          </div>
          <ComparisonChart traders={sortedTraders.slice(0, 10)} />
        </div>

        {/* Right: Leaderboard */}
        <div
          className="binance-card-enhanced p-6 md:p-8 animate-slide-in"
          style={{ animationDelay: '0.2s' }}
        >
          <div className="flex items-center justify-between mb-6">
            <h2
              className="text-xl md:text-2xl font-bold flex items-center gap-2"
              style={{ color: 'var(--text-white)' }}
            >
              {t('leaderboard', language)}
            </h2>
            <div
              className="text-xs px-3 py-1.5 rounded-md font-semibold pulse-live"
              style={{
                background: 'var(--success-bg)',
                color: 'var(--green-primary)',
                border: '1px solid var(--success-border)',
                boxShadow: '0 2px 8px var(--green-glow)',
              }}
            >
              {t('live', language)}
            </div>
          </div>
          <div className="space-y-3">
            {sortedTraders.map((trader, index) => {
              const isLeader = index === 0
              const isSilver = index === 1
              const isBronze = index === 2
              const traderColor = getTraderColor(
                sortedTraders,
                trader.trader_id
              )

              // Determine rank badge class
              let rankBadgeClass = ''
              let itemClass = ''
              if (isLeader) {
                rankBadgeClass = 'rank-badge-gold'
                itemClass = 'leaderboard-item-top'
              } else if (isSilver) {
                rankBadgeClass = 'rank-badge-silver'
                itemClass = 'leaderboard-item-silver'
              } else if (isBronze) {
                rankBadgeClass = 'rank-badge-bronze'
                itemClass = 'leaderboard-item-bronze'
              }

              return (
                <div
                  key={trader.trader_id}
                  onClick={() => handleTraderClick(trader.trader_id)}
                  className={`rounded-lg p-3 md:p-4 transition-all duration-300 cursor-pointer hover:scale-[1.01] active:scale-[0.99] ${
                    itemClass || ''
                  }`}
                  style={{
                    background: itemClass
                      ? undefined
                      : 'var(--navy-dark)',
                    border: itemClass
                      ? undefined
                      : '1px solid var(--navy-light)',
                    boxShadow: itemClass
                      ? undefined
                      : 'var(--shadow-sm)',
                    animationDelay: `${index * 0.05}s`,
                  }}
                >
                  <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 sm:gap-4">
                    {/* Rank & Name */}
                    <div className="flex items-center gap-3 md:gap-4 flex-1 min-w-0">
                      <div
                        className={`w-8 h-8 md:w-10 md:h-10 rounded-lg flex items-center justify-center font-bold text-sm md:text-base flex-shrink-0 ${
                          rankBadgeClass || ''
                        }`}
                        style={
                          rankBadgeClass
                            ? {}
                            : {
                                background: 'var(--navy-dark)',
                                border: '1px solid var(--navy-light)',
                                color: 'var(--text-gray-light)',
                              }
                        }
                      >
                        {index < 3 ? (
                          <Medal className="w-5 h-5 md:w-6 md:h-6" />
                        ) : (
                          <span>#{index + 1}</span>
                        )}
                      </div>
                      <div className="flex-1 min-w-0">
                        <div
                          className="font-bold text-sm md:text-base lg:text-lg mb-1 truncate"
                          style={{ color: 'var(--text-white)' }}
                        >
                          {trader.trader_name}
                        </div>
                        <div
                          className="text-xs md:text-sm mono font-semibold truncate"
                          style={{ color: traderColor }}
                        >
                          {trader.ai_model.toUpperCase()} +{' '}
                          {trader.exchange.toUpperCase()}
                        </div>
                      </div>
                    </div>

                    {/* Stats */}
                    <div className="flex items-center gap-2 md:gap-4 flex-wrap md:flex-nowrap">
                      {/* Total Equity */}
                      <div className="text-right rounded-md px-2 md:px-3 py-1.5 md:py-2 min-w-[70px] md:min-w-[80px]" style={{ background: 'rgba(0, 31, 63, 0.2)' }}>
                        <div className="text-xs uppercase tracking-wider mb-1 font-semibold" style={{ color: 'var(--text-gray-light)' }}>
                          {t('equity', language)}
                        </div>
                        <div
                          className="text-xs md:text-sm lg:text-base font-bold mono"
                          style={{ color: 'var(--text-white)' }}
                        >
                          {trader.total_equity?.toFixed(2) || '0.00'}
                        </div>
                      </div>

                      {/* P&L */}
                      <div className="text-right min-w-[80px] md:min-w-[110px] rounded-md px-2 md:px-3 py-1.5 md:py-2" style={{ background: 'rgba(0, 31, 63, 0.2)' }}>
                        <div className="text-xs uppercase tracking-wider mb-1 font-semibold" style={{ color: 'var(--text-gray-light)' }}>
                          {t('pnl', language)}
                        </div>
                        <div
                          className="text-base md:text-lg lg:text-xl font-bold mono mb-0.5"
                          style={{
                            color:
                              (trader.total_pnl ?? 0) >= 0
                                ? 'var(--green-primary)'
                                : 'var(--error)',
                          }}
                        >
                          {(trader.total_pnl ?? 0) >= 0 ? '+' : ''}
                          {trader.total_pnl_pct?.toFixed(2) || '0.00'}%
                        </div>
                        <div
                          className="text-xs mono hidden sm:block"
                          style={{ color: 'var(--text-gray-light)' }}
                        >
                          {(trader.total_pnl ?? 0) >= 0 ? '+' : ''}
                          {trader.total_pnl?.toFixed(2) || '0.00'}
                        </div>
                      </div>

                      {/* Positions */}
                      <div className="text-right rounded-md px-2 md:px-3 py-1.5 md:py-2 min-w-[60px] md:min-w-[70px]" style={{ background: 'rgba(0, 31, 63, 0.2)' }}>
                        <div className="text-xs uppercase tracking-wider mb-1 font-semibold" style={{ color: 'var(--text-gray-light)' }}>
                          {t('pos', language)}
                        </div>
                        <div
                          className="text-xs md:text-sm lg:text-base font-bold mono"
                          style={{ color: 'var(--text-white)' }}
                        >
                          {trader.position_count}
                        </div>
                        <div className="text-xs hidden sm:block" style={{ color: 'var(--text-gray-light)' }}>
                          {trader.margin_used_pct.toFixed(1)}%
                        </div>
                      </div>

                      {/* Status */}
                      <div className="flex items-center justify-center">
                        <div
                          className="px-2 md:px-3 py-1.5 md:py-2 rounded-md text-xs font-bold"
                          style={
                            Boolean(trader.is_running)
                              ? {
                                  background: 'var(--success-bg)',
                                  color: 'var(--green-primary)',
                                  border: '1px solid var(--success-border)',
                                  boxShadow: '0 2px 8px var(--green-glow)',
                                }
                              : {
                                  background: 'var(--error-bg)',
                                  color: 'var(--error)',
                                  border: '1px solid var(--error-border)',
                                }
                          }
                        >
                          {Boolean(trader.is_running) ? '●' : '○'}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      </div>

      {/* Head-to-Head Stats */}
      {competition.traders.length === 2 && (
        <div
          className="binance-card-enhanced p-6 md:p-8 animate-slide-in"
          style={{ animationDelay: '0.3s' }}
        >
          <h2
            className="text-xl md:text-2xl font-bold mb-6 flex items-center gap-2"
            style={{ color: 'var(--text-white)' }}
          >
            {t('headToHead', language)}
          </h2>
          <div className="grid grid-cols-2 gap-6">
            {sortedTraders.map((trader, index) => {
              const isWinning = index === 0
              const opponent = sortedTraders[1 - index]

              // Check if both values are valid numbers
              const hasValidData =
                trader.total_pnl_pct != null &&
                opponent.total_pnl_pct != null &&
                !isNaN(trader.total_pnl_pct) &&
                !isNaN(opponent.total_pnl_pct)

              const gap = hasValidData
                ? trader.total_pnl_pct - opponent.total_pnl_pct
                : NaN

              return (
                <div
                  key={trader.trader_id}
                  className="p-6 rounded-xl transition-all duration-300 hover:scale-[1.02] relative overflow-hidden"
                  style={
                    isWinning
                      ? {
                          background:
                            'linear-gradient(135deg, rgba(0, 255, 127, 0.12) 0%, var(--navy-dark) 100%)',
                          border: '2px solid var(--success-border)',
                          boxShadow: '0 4px 20px var(--green-glow)',
                        }
                      : {
                          background: 'var(--navy-dark)',
                          border: '2px solid var(--navy-light)',
                          boxShadow: 'var(--shadow-sm)',
                        }
                  }
                >
                  {isWinning && (
                    <div
                      className="absolute top-0 right-0 w-20 h-20 opacity-10"
                      style={{
                        background: 'radial-gradient(circle, rgba(14, 203, 129, 0.5) 0%, transparent 70%)',
                      }}
                    />
                  )}
                  <div className="text-center relative z-10">
                    <div
                      className="text-base md:text-lg font-bold mb-3"
                      style={{
                        color: getTraderColor(sortedTraders, trader.trader_id),
                      }}
                    >
                      {trader.trader_name}
                    </div>
                    <div
                      className="text-2xl md:text-3xl font-bold mono mb-2"
                      style={{
                        color:
                          (trader.total_pnl ?? 0) >= 0 ? 'var(--green-primary)' : 'var(--error)',
                      }}
                    >
                      {trader.total_pnl_pct != null &&
                      !isNaN(trader.total_pnl_pct)
                        ? `${trader.total_pnl_pct >= 0 ? '+' : ''}${trader.total_pnl_pct.toFixed(2)}%`
                        : '—'}
                    </div>
                    {hasValidData && isWinning && gap > 0 && (
                      <div
                        className="text-sm font-semibold px-3 py-1.5 rounded-md inline-block"
                        style={{
                          background: 'var(--success-bg)',
                          color: 'var(--green-primary)',
                          border: '1px solid var(--success-border)',
                        }}
                      >
                        {t('leadingBy', language, { gap: gap.toFixed(2) })}
                      </div>
                    )}
                    {hasValidData && !isWinning && gap < 0 && (
                      <div
                        className="text-sm font-semibold px-3 py-1.5 rounded-md inline-block"
                        style={{
                          background: 'var(--error-bg)',
                          color: 'var(--error)',
                          border: '1px solid var(--error-border)',
                        }}
                      >
                        {t('behindBy', language, {
                          gap: Math.abs(gap).toFixed(2),
                        })}
                      </div>
                    )}
                    {!hasValidData && (
                      <div
                        className="text-sm font-semibold"
                        style={{ color: 'var(--text-gray-light)' }}
                      >
                        —
                      </div>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* Trader Config View Modal */}
      <TraderConfigViewModal
        isOpen={isModalOpen}
        onClose={closeModal}
        traderData={selectedTrader}
        onCopyTrader={userIsFollower && user && token ? handleCopyTrader : undefined}
      />
    </div>
  )
}
