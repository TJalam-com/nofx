import { useState } from 'react'
import { Trophy, Medal } from 'lucide-react'
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
    console.log('🔄 handleCopyTrader called with traderId:', traderId)
    console.log('👤 User:', user ? { id: user.id, email: user.email, role: user.role } : 'null')
    console.log('🔑 Token exists:', !!token)
    
    // Check if this trader is a follower trader (has followed_trader_id)
    const trader = competition?.traders?.find((t) => t.trader_id === traderId)
    if (trader && (trader as any).followed_trader_id) {
      toast.error(t('cannotCopyFollowerTrader', language) || 'Cannot copy follower traders. Only original traders can be copied.')
      return
    }
    
    // Navigate to traders page and open create modal with the trader ID to copy
    // We'll use sessionStorage to pass the trader ID to copy
    if (user && token) {
      try {
        sessionStorage.setItem('copyTraderId', traderId)
        console.log('✅ Stored copyTraderId in sessionStorage:', traderId)
        console.log('🔍 Verifying sessionStorage:', sessionStorage.getItem('copyTraderId'))
        
        console.log('🚀 Attempting navigation to /traders?action=copy')
        
        // Use window.location.href for reliable navigation (works with React Router)
        // This ensures the page actually navigates and the useEffect in AITradersPage will run
        window.location.href = '/traders?action=copy'
        console.log('✅ window.location.href set successfully')
      } catch (error) {
        console.error('❌ Error in handleCopyTrader:', error)
        // Fallback navigation
        console.log('🔄 Using fallback navigation with window.location.href')
        sessionStorage.setItem('copyTraderId', traderId)
        window.location.href = '/traders?action=copy'
      }
    } else {
      console.log('❌ User not logged in, redirecting to login')
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
                  background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
                }}
              >
                <Trophy
                  className="w-7 h-7 md:w-9 md:h-9 relative z-10"
                  style={{ color: '#000' }}
                />
              </div>
              <div>
                <h1
                  className="text-2xl md:text-3xl font-bold flex items-center gap-3 mb-1"
                  style={{ color: '#EAECEF' }}
                >
                  {t('aiCompetition', language)}
                  <span
                    className="text-xs md:text-sm font-semibold px-3 py-1.5 rounded-lg"
                    style={{
                      background: 'rgba(240, 185, 11, 0.2)',
                      color: '#F0B90B',
                      border: '1px solid rgba(240, 185, 11, 0.3)',
                    }}
                  >
                    0 {t('traders', language)}
                  </span>
                </h1>
                <p className="text-sm md:text-base mt-1" style={{ color: '#848E9C' }}>
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
            style={{ color: '#848E9C' }}
          />
          <h3 className="text-xl md:text-2xl font-bold mb-3" style={{ color: '#EAECEF' }}>
            {t('noTraders', language)}
          </h3>
          <p className="text-base md:text-lg" style={{ color: '#848E9C' }}>
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
      {/* Competition Header - Enhanced */}
      <div className="competition-header">
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4 md:gap-6">
          <div className="flex items-center gap-4 md:gap-5">
            <div
              className="w-12 h-12 md:w-16 md:h-16 rounded-2xl flex items-center justify-center trophy-icon relative"
              style={{
                background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
              }}
            >
              <Trophy
                className="w-7 h-7 md:w-9 md:h-9 relative z-10"
                style={{ color: '#000' }}
              />
              <div
                className="absolute inset-0 rounded-2xl opacity-50"
                style={{
                  background: 'radial-gradient(circle, rgba(240, 185, 11, 0.6) 0%, transparent 70%)',
                }}
              />
            </div>
            <div>
              <h1
                className="text-2xl md:text-3xl font-bold flex items-center gap-3 mb-1"
                style={{ color: '#EAECEF' }}
              >
                {t('aiCompetition', language)}
                <span
                  className="text-xs md:text-sm font-semibold px-3 py-1.5 rounded-lg pulse-live"
                  style={{
                    background: 'rgba(240, 185, 11, 0.2)',
                    color: '#F0B90B',
                    border: '1px solid rgba(240, 185, 11, 0.3)',
                    boxShadow: '0 2px 8px rgba(240, 185, 11, 0.2)',
                  }}
                >
                  {competition.count} {t('traders', language)}
                </span>
              </h1>
              <p className="text-sm md:text-base mt-1" style={{ color: '#848E9C' }}>
                {t('liveBattle', language)}
              </p>
            </div>
          </div>
          <div className="text-left md:text-right w-full md:w-auto bg-black/30 rounded-lg px-4 py-3 border border-yellow-500/20">
            <div className="text-xs uppercase tracking-wider mb-2 font-semibold" style={{ color: '#848E9C' }}>
              {t('leader', language)}
            </div>
            <div
              className="text-lg md:text-xl font-bold mb-1"
              style={{ color: '#F0B90B' }}
            >
              {leader?.trader_name}
            </div>
            <div
              className="text-base md:text-lg font-semibold mono"
              style={{
                color: (leader?.total_pnl ?? 0) >= 0 ? '#0ECB81' : '#F6465D',
              }}
            >
              {(leader?.total_pnl ?? 0) >= 0 ? '+' : ''}
              {leader?.total_pnl_pct?.toFixed(2) || '0.00'}%
            </div>
          </div>
        </div>
      </div>

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
              style={{ color: '#EAECEF' }}
            >
              {t('performanceComparison', language)}
            </h2>
            <div
              className="text-xs px-2.5 py-1 rounded-md font-semibold pulse-live"
              style={{
                background: 'rgba(14, 203, 129, 0.15)',
                color: '#0ECB81',
                border: '1px solid rgba(14, 203, 129, 0.3)',
              }}
            >
              {t('realTimePnL', language)}
            </div>
          </div>
          <ComparisonChart traders={sortedTraders.slice(0, 5)} />
        </div>

        {/* Right: Leaderboard */}
        <div
          className="binance-card-enhanced p-6 md:p-8 animate-slide-in"
          style={{ animationDelay: '0.2s' }}
        >
          <div className="flex items-center justify-between mb-6">
            <h2
              className="text-xl md:text-2xl font-bold flex items-center gap-2"
              style={{ color: '#EAECEF' }}
            >
              {t('leaderboard', language)}
            </h2>
            <div
              className="text-xs px-3 py-1.5 rounded-md font-semibold pulse-live"
              style={{
                background: 'rgba(240, 185, 11, 0.15)',
                color: '#F0B90B',
                border: '1px solid rgba(240, 185, 11, 0.3)',
                boxShadow: '0 2px 8px rgba(240, 185, 11, 0.2)',
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
                  className={`rounded-lg p-4 transition-all duration-300 cursor-pointer ${
                    itemClass || ''
                  }`}
                  style={{
                    background: itemClass
                      ? undefined
                      : '#0B0E11',
                    border: itemClass
                      ? undefined
                      : '1px solid #2B3139',
                    boxShadow: itemClass
                      ? undefined
                      : '0 1px 4px rgba(0, 0, 0, 0.3)',
                    animationDelay: `${index * 0.05}s`,
                  }}
                >
                  <div className="flex items-center justify-between">
                    {/* Rank & Name */}
                    <div className="flex items-center gap-4">
                      <div
                        className={`w-8 h-8 md:w-10 md:h-10 rounded-lg flex items-center justify-center font-bold text-sm md:text-base ${
                          rankBadgeClass || ''
                        }`}
                        style={
                          rankBadgeClass
                            ? {}
                            : {
                                background: '#1E2329',
                                border: '1px solid #2B3139',
                                color: '#848E9C',
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
                          className="font-bold text-base md:text-lg mb-1 truncate"
                          style={{ color: '#EAECEF' }}
                        >
                          {trader.trader_name}
                        </div>
                        <div
                          className="text-xs md:text-sm mono font-semibold"
                          style={{ color: traderColor }}
                        >
                          {trader.ai_model.toUpperCase()} +{' '}
                          {trader.exchange.toUpperCase()}
                        </div>
                      </div>
                    </div>

                    {/* Stats */}
                    <div className="flex items-center gap-3 md:gap-4 flex-wrap md:flex-nowrap">
                      {/* Total Equity */}
                      <div className="text-right bg-black/20 rounded-md px-3 py-2 min-w-[80px]">
                        <div className="text-xs uppercase tracking-wider mb-1 font-semibold" style={{ color: '#848E9C' }}>
                          {t('equity', language)}
                        </div>
                        <div
                          className="text-sm md:text-base font-bold mono"
                          style={{ color: '#EAECEF' }}
                        >
                          {trader.total_equity?.toFixed(2) || '0.00'}
                        </div>
                      </div>

                      {/* P&L */}
                      <div className="text-right min-w-[90px] md:min-w-[110px] bg-black/20 rounded-md px-3 py-2">
                        <div className="text-xs uppercase tracking-wider mb-1 font-semibold" style={{ color: '#848E9C' }}>
                          {t('pnl', language)}
                        </div>
                        <div
                          className="text-lg md:text-xl font-bold mono mb-0.5"
                          style={{
                            color:
                              (trader.total_pnl ?? 0) >= 0
                                ? '#0ECB81'
                                : '#F6465D',
                          }}
                        >
                          {(trader.total_pnl ?? 0) >= 0 ? '+' : ''}
                          {trader.total_pnl_pct?.toFixed(2) || '0.00'}%
                        </div>
                        <div
                          className="text-xs mono"
                          style={{ color: '#848E9C' }}
                        >
                          {(trader.total_pnl ?? 0) >= 0 ? '+' : ''}
                          {trader.total_pnl?.toFixed(2) || '0.00'}
                        </div>
                      </div>

                      {/* Positions */}
                      <div className="text-right bg-black/20 rounded-md px-3 py-2 min-w-[70px]">
                        <div className="text-xs uppercase tracking-wider mb-1 font-semibold" style={{ color: '#848E9C' }}>
                          {t('pos', language)}
                        </div>
                        <div
                          className="text-sm md:text-base font-bold mono"
                          style={{ color: '#EAECEF' }}
                        >
                          {trader.position_count}
                        </div>
                        <div className="text-xs" style={{ color: '#848E9C' }}>
                          {trader.margin_used_pct.toFixed(1)}%
                        </div>
                      </div>

                      {/* Status */}
                      <div className="flex items-center justify-center">
                        <div
                          className="px-3 py-2 rounded-md text-xs font-bold"
                          style={
                            Boolean(trader.is_running)
                              ? {
                                  background: 'rgba(14, 203, 129, 0.15)',
                                  color: '#0ECB81',
                                  border: '1px solid rgba(14, 203, 129, 0.3)',
                                  boxShadow: '0 2px 8px rgba(14, 203, 129, 0.2)',
                                }
                              : {
                                  background: 'rgba(246, 70, 93, 0.15)',
                                  color: '#F6465D',
                                  border: '1px solid rgba(246, 70, 93, 0.3)',
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
            style={{ color: '#EAECEF' }}
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
                            'linear-gradient(135deg, rgba(14, 203, 129, 0.12) 0%, rgba(11, 14, 17, 0.95) 100%)',
                          border: '2px solid rgba(14, 203, 129, 0.4)',
                          boxShadow: '0 4px 20px rgba(14, 203, 129, 0.2)',
                        }
                      : {
                          background: '#0B0E11',
                          border: '2px solid #2B3139',
                          boxShadow: '0 2px 8px rgba(0, 0, 0, 0.3)',
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
                          (trader.total_pnl ?? 0) >= 0 ? '#0ECB81' : '#F6465D',
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
                          background: 'rgba(14, 203, 129, 0.15)',
                          color: '#0ECB81',
                          border: '1px solid rgba(14, 203, 129, 0.3)',
                        }}
                      >
                        {t('leadingBy', language, { gap: gap.toFixed(2) })}
                      </div>
                    )}
                    {hasValidData && !isWinning && gap < 0 && (
                      <div
                        className="text-sm font-semibold px-3 py-1.5 rounded-md inline-block"
                        style={{
                          background: 'rgba(246, 70, 93, 0.15)',
                          color: '#F6465D',
                          border: '1px solid rgba(246, 70, 93, 0.3)',
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
                        style={{ color: '#848E9C' }}
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
