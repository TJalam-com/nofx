import { useState, useMemo } from 'react'
import { Trophy, Medal, Clock, Users } from 'lucide-react'
import useSWR from 'swr'
import { api } from '../lib/api'
import type { CompetitionData } from '../types'
import { ComparisonChart } from './ComparisonChart'
import { TraderConfigViewModal } from './TraderConfigViewModal'
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
      // Merge performance data from competition data
      const traderFromCompetition = competition?.traders?.find((t) => t.trader_id === traderId)
      if (traderFromCompetition) {
        setSelectedTrader({
          ...traderConfig,
          total_equity: traderFromCompetition.total_equity,
          total_pnl: traderFromCompetition.total_pnl,
          total_pnl_pct: traderFromCompetition.total_pnl_pct,
          position_count: traderFromCompetition.position_count,
          margin_used_pct: traderFromCompetition.margin_used_pct,
        } as any)
      } else {
        setSelectedTrader(traderConfig)
      }
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

  return (
    <div className="space-y-6 md:space-y-8 animate-fade-in">
      {/* Redesigned Hero Section - Multi-Card Layout */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 md:gap-6">
        {/* Card 1: Competition Title */}
        <div
          className="binance-card-enhanced p-3 md:p-4 animate-slide-in relative overflow-hidden"
          style={{
            animationDelay: '0.1s',
            background: 'linear-gradient(135deg, rgba(0, 255, 127, 0.05) 0%, var(--navy-dark) 100%)',
          }}
        >
          <div className="flex items-center gap-2.5">
            <div
              className="w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0"
              style={{
                background: 'linear-gradient(135deg, var(--green-primary) 0%, var(--green-light) 100%)',
              }}
            >
              <Trophy
                className="w-4 h-4"
                style={{ color: 'var(--navy-primary)' }}
              />
            </div>
            <div className="flex-1 min-w-0">
              <h1
                className="text-sm md:text-base font-bold truncate"
                style={{ color: 'var(--text-white)' }}
              >
                {t('aiCompetition', language)}
              </h1>
              <p className="text-xs truncate" style={{ color: 'var(--text-gray-light)' }}>
                {t('liveBattle', language)}
              </p>
            </div>
          </div>
        </div>

        {/* Card 2: Live Stats */}
        <div
          className="binance-card-enhanced p-3 md:p-4 animate-slide-in relative overflow-hidden"
          style={{
            animationDelay: '0.15s',
            background: 'linear-gradient(135deg, rgba(0, 255, 127, 0.05) 0%, var(--navy-dark) 100%)',
          }}
        >
          <div className="flex items-center gap-2.5">
            <div
              className="w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0"
              style={{
                background: 'rgba(0, 255, 127, 0.1)',
                border: '1px solid rgba(0, 255, 127, 0.2)',
              }}
            >
              <Users className="w-4 h-4" style={{ color: 'var(--green-primary)' }} />
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-baseline gap-2">
                <div
                  className="text-lg md:text-xl font-bold mono"
                  style={{ color: 'var(--text-white)' }}
                >
                  {competition.count}
                </div>
                <div className="flex items-center gap-1.5">
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
              {(competition.follower_total_count !== undefined && 
                competition.follower_total_count !== null && 
                Number(competition.follower_total_count) > 0) && (
                <div className="mt-1.5 flex items-center gap-1.5">
                  <span className="text-xs font-medium" style={{ color: 'var(--text-gray-light)' }}>
                    {Number(competition.follower_total_count)}
                  </span>
                  <span className="text-xs" style={{ color: 'var(--text-gray-light)' }}>
                    {t('followers', language) || 'followers'}
                  </span>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Card 3: Last Update */}
        <div
          className="binance-card-enhanced p-3 md:p-4 animate-slide-in relative overflow-hidden"
          style={{
            animationDelay: '0.2s',
            background: 'linear-gradient(135deg, rgba(0, 255, 127, 0.05) 0%, var(--navy-dark) 100%)',
          }}
        >
          <div className="flex items-center gap-2.5">
            <div
              className="w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0"
              style={{
                background: 'rgba(0, 255, 127, 0.1)',
                border: '1px solid rgba(0, 255, 127, 0.2)',
              }}
            >
              <Clock className="w-4 h-4" style={{ color: 'var(--green-primary)' }} />
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-baseline gap-2">
                <div
                  className="text-sm md:text-base font-bold mono"
                  style={{ color: 'var(--text-white)' }}
                >
                  {lastUpdateTime}
                </div>
                <div className="flex items-center gap-1.5">
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
        </div>
      </div>

      {/* Performance Comparison Chart - Full Width */}
      <div className="mb-6 md:mb-8">
        <div
          className="binance-card-enhanced p-6 md:p-8 animate-slide-in"
          style={{ animationDelay: '0.1s', minHeight: 'clamp(400px, calc(100vh - 350px), 800px)' }}
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
      </div>

      {/* Leaderboard - Full Width Below Chart */}
      <div className="mb-6 md:mb-8">
        <div
          className="binance-card-enhanced p-6 md:p-8 animate-slide-in"
          style={{ animationDelay: '0.2s' }}
        >
          <div className="flex items-center justify-between mb-4">
            <h2
              className="text-lg md:text-xl font-bold flex items-center gap-2"
              style={{ color: 'var(--text-white)' }}
            >
              {t('leaderboard', language)}
            </h2>
            <div
              className="text-xs px-2 py-1 rounded font-semibold pulse-live"
              style={{
                background: 'var(--success-bg)',
                color: 'var(--green-primary)',
                border: '1px solid var(--success-border)',
              }}
            >
              {t('live', language)}
            </div>
          </div>
          <div className="space-y-2">
            {sortedTraders.map((trader, index) => {
              const isLeader = index === 0
              const isSilver = index === 1
              const isBronze = index === 2
              const isFollowerTrader = !!(trader.followed_trader_id && trader.followed_trader_id !== '')

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
                  className={`rounded-lg p-2.5 md:p-3 transition-all duration-200 cursor-pointer hover:opacity-90 active:scale-[0.98] ${
                    itemClass || ''
                  }`}
                  style={{
                    background: itemClass
                      ? undefined
                      : isFollowerTrader
                      ? 'rgba(99, 102, 241, 0.08)'
                      : 'rgba(255, 255, 255, 0.02)',
                    border: itemClass
                      ? undefined
                      : isFollowerTrader
                      ? '1px solid rgba(99, 102, 241, 0.3)'
                      : '1px solid rgba(255, 255, 255, 0.05)',
                    animationDelay: `${index * 0.03}s`,
                  }}
                >
                  <div className="flex items-center justify-between gap-3">
                    {/* Rank & Name */}
                    <div className="flex items-center gap-2.5 md:gap-3 flex-1 min-w-0">
                      <div
                        className={`w-7 h-7 md:w-8 md:h-8 rounded-md flex items-center justify-center font-bold text-xs md:text-sm flex-shrink-0 ${
                          rankBadgeClass || ''
                        }`}
                        style={
                          rankBadgeClass
                            ? {}
                            : isFollowerTrader
                            ? {
                                background: 'rgba(99, 102, 241, 0.2)',
                                color: '#A5B4FC',
                                border: '1px solid rgba(99, 102, 241, 0.3)',
                              }
                            : {
                                background: 'rgba(255, 255, 255, 0.05)',
                                color: 'var(--text-gray-light)',
                              }
                        }
                      >
                        {index < 3 ? (
                          <Medal className="w-4 h-4 md:w-5 md:h-5" />
                        ) : (
                          <span>#{index + 1}</span>
                        )}
                      </div>
                      <div className="flex items-center gap-1.5 flex-1 min-w-0">
                        <div
                          className="font-semibold text-sm md:text-base truncate"
                          style={{ color: isFollowerTrader ? '#A5B4FC' : 'var(--text-white)' }}
                        >
                          {trader.trader_name}
                        </div>
                        {isFollowerTrader && (
                          <span
                            className="text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider flex-shrink-0"
                            style={{
                              background: 'rgba(99, 102, 241, 0.2)',
                              color: '#A5B4FC',
                              border: '1px solid rgba(99, 102, 241, 0.3)',
                            }}
                          >
                            {t('follower', language) || 'FOLLOWER'}
                          </span>
                        )}
                      </div>
                    </div>

                    {/* Compact Stats */}
                    <div className="flex items-center gap-3 md:gap-4">
                      {/* P&L */}
                      <div className="text-right min-w-[70px]">
                        <div
                          className="text-sm md:text-base font-bold mono"
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
                      </div>

                      {/* Positions */}
                      <div className="text-right min-w-[40px]">
                        <div
                          className="text-xs md:text-sm font-semibold mono"
                          style={{ color: 'var(--text-gray-light)' }}
                        >
                          {trader.position_count}
                        </div>
                      </div>

                      {/* Followers Count */}
                      {trader.followers_count !== undefined && trader.followers_count > 0 && (
                        <div className="text-right min-w-[50px]">
                          <div className="flex items-center gap-1 justify-end">
                            <Users className="w-3 h-3" style={{ color: 'var(--text-gray-light)' }} />
                            <div
                              className="text-xs md:text-sm font-semibold mono"
                              style={{ color: 'var(--text-gray-light)' }}
                            >
                              {trader.followers_count}
                            </div>
                          </div>
                        </div>
                      )}

                      {/* Status */}
                      <div className="flex items-center justify-center min-w-[20px]">
                        <div
                          className="w-2 h-2 rounded-full"
                          style={
                            Boolean(trader.is_running)
                              ? {
                                  background: 'var(--green-primary)',
                                  boxShadow: '0 0 8px var(--green-primary)',
                                }
                              : {
                                  background: 'var(--error)',
                                  opacity: 0.5,
                                }
                          }
                        />
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
