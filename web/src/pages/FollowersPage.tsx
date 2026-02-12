import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import useSWR from 'swr'
import { api } from '../lib/api'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth, isFollower } from '../contexts/AuthContext'
import { t, type Language } from '../i18n/translations'
import type { UserFollowersResponse, FollowerWithActivities } from '../types'
import { generateTraderSlug } from '../lib/utils'
import {
  Users,
  ChevronDown,
  ChevronUp,
  TrendingUp,
  TrendingDown,
  Activity,
  ExternalLink,
  RefreshCw,
  AlertCircle,
} from 'lucide-react'

export default function FollowersPage() {
  const { language } = useLanguage()
  const { user, token } = useAuth()
  const navigate = useNavigate()
  const [expandedParents, setExpandedParents] = useState<Set<string>>(new Set())

  // Redirect follower users - this page is only for parent traders
  useEffect(() => {
    if (user && isFollower(user)) {
      navigate('/dashboard', { replace: true })
      return
    }
  }, [user, navigate])

  const shouldFetch = user && token && !isFollower(user)
  console.log('🔍 DEBUG [FollowersPage]: Fetch check:', {
    hasUser: !!user,
    hasToken: !!token,
    isFollower: user ? isFollower(user) : null,
    shouldFetch,
    userId: user?.id,
    userRole: user?.role,
  })

  const {
    data: followersData,
    error,
    isLoading,
  } = useSWR<UserFollowersResponse>(
    shouldFetch ? 'user-followers' : null,
    () => {
      console.log('🔍 DEBUG [FollowersPage]: Calling api.getUserFollowers()')
      return api.getUserFollowers()
    },
    {
      refreshInterval: 30000, // Refresh every 30 seconds
      revalidateOnFocus: false,
      dedupingInterval: 20000,
      onSuccess: (data) => {
        console.log('✅ DEBUG [FollowersPage]: API call successful:', {
          parentTradersCount: data?.parent_traders?.length || 0,
          totalFollowers:
            data?.parent_traders?.reduce(
              (sum, p) => sum + p.followers.length,
              0
            ) || 0,
        })
      },
      onError: (err) => {
        console.error('❌ DEBUG [FollowersPage]: API call failed:', err)
      },
    }
  )

  // Don't render anything if user is a follower (redirecting)
  if (user && isFollower(user)) {
    return null
  }

  // Don't render if not authenticated
  if (!user || !token) {
    return null
  }

  const toggleParentExpanded = (parentId: string) => {
    const newExpanded = new Set(expandedParents)
    if (newExpanded.has(parentId)) {
      newExpanded.delete(parentId)
    } else {
      newExpanded.add(parentId)
    }
    setExpandedParents(newExpanded)
  }

  // Calculate summary statistics
  const summaryStats = followersData
    ? {
        totalFollowers: followersData.parent_traders.reduce(
          (sum, parent) => sum + parent.followers.length,
          0
        ),
        activeFollowers: followersData.parent_traders.reduce(
          (sum, parent) =>
            sum + parent.followers.filter((f) => f.is_running).length,
          0
        ),
        totalParents: followersData.parent_traders.length,
      }
    : { totalFollowers: 0, activeFollowers: 0, totalParents: 0 }

  // Loading state
  if (isLoading || (!followersData && !error)) {
    return (
      <div className="space-y-6">
        <div className="binance-card p-6 animate-pulse">
          <div className="skeleton h-8 w-48 mb-3"></div>
          <div className="flex gap-4">
            <div className="skeleton h-4 w-32"></div>
            <div className="skeleton h-4 w-24"></div>
            <div className="skeleton h-4 w-28"></div>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {[1, 2, 3].map((i) => (
            <div key={i} className="binance-card p-5 animate-pulse">
              <div className="skeleton h-4 w-24 mb-3"></div>
              <div className="skeleton h-8 w-32"></div>
            </div>
          ))}
        </div>
      </div>
    )
  }

  // Error state
  if (error) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="text-center max-w-md mx-auto px-6">
          <div
            className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center"
            style={{
              background: 'rgba(246, 70, 93, 0.1)',
              border: '2px solid rgba(246, 70, 93, 0.3)',
            }}
          >
            <AlertCircle className="w-12 h-12" style={{ color: '#F6465D' }} />
          </div>
          <h2 className="text-2xl font-bold mb-3" style={{ color: '#EAECEF' }}>
            {t('errorLoadingFollowers', language) || 'Failed to Load Followers'}
          </h2>
          <p className="text-base mb-6" style={{ color: '#848E9C' }}>
            {error instanceof Error ? error.message : 'Unknown error occurred'}
          </p>
          <button
            onClick={() => window.location.reload()}
            className="px-6 py-3 rounded-lg font-semibold transition-all hover:scale-105 active:scale-95"
            style={{
              background:
                'linear-gradient(135deg, var(--green-primary) 0%, var(--green-light) 100%)',
              color: '#0B0E11',
              boxShadow: '0 4px 12px rgba(0, 255, 127, 0.3)',
            }}
          >
            {t('retry', language) || 'Retry'}
          </button>
        </div>
      </div>
    )
  }

  // Empty state - show even if data hasn't loaded yet (but not loading)
  if (!followersData) {
    // If we're not loading and there's no error, show empty state
    if (!isLoading && !error) {
      return (
        <div className="flex items-center justify-center min-h-[60vh]">
          <div className="text-center max-w-md mx-auto px-6">
            <div
              className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center"
              style={{
                background: 'rgba(0, 255, 127, 0.1)',
                border: '2px solid rgba(0, 255, 127, 0.3)',
              }}
            >
              <Users
                className="w-12 h-12"
                style={{ color: 'var(--green-primary)' }}
              />
            </div>
            <h2
              className="text-2xl font-bold mb-3"
              style={{ color: '#EAECEF' }}
            >
              {t('noFollowersTitle', language) || 'No Followers Yet'}
            </h2>
            <p className="text-base mb-6" style={{ color: '#848E9C' }}>
              {t('noFollowersDescription', language) ||
                "You don't have any followers yet. When other users copy your traders, they will appear here."}
            </p>
          </div>
        </div>
      )
    }
    // If loading or error, return null (handled above)
    return null
  }

  // Empty state - no parent traders with followers
  if (followersData.parent_traders.length === 0) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="text-center max-w-md mx-auto px-6">
          <div
            className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center"
            style={{
              background: 'rgba(0, 255, 127, 0.1)',
              border: '2px solid rgba(0, 255, 127, 0.3)',
            }}
          >
            <Users
              className="w-12 h-12"
              style={{ color: 'var(--green-primary)' }}
            />
          </div>
          <h2 className="text-2xl font-bold mb-3" style={{ color: '#EAECEF' }}>
            {t('noFollowersTitle', language) || 'No Followers Yet'}
          </h2>
          <p className="text-base mb-6" style={{ color: '#848E9C' }}>
            {t('noFollowersDescription', language) ||
              "You don't have any followers yet. When other users copy your traders, they will appear here."}
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div
        className="mb-6 rounded p-6 animate-scale-in"
        style={{
          background:
            'linear-gradient(135deg, rgba(0, 255, 127, 0.15) 0%, rgba(51, 255, 153, 0.05) 100%)',
          border: '1px solid rgba(0, 255, 127, 0.2)',
          boxShadow: '0 0 30px rgba(0, 255, 127, 0.15)',
        }}
      >
        <div className="flex items-center justify-between mb-4">
          <h1
            className="text-3xl font-bold flex items-center gap-3"
            style={{ color: '#EAECEF' }}
          >
            <span
              className="w-12 h-12 rounded-full flex items-center justify-center"
              style={{
                background:
                  'linear-gradient(135deg, var(--green-primary) 0%, var(--green-light) 100%)',
              }}
            >
              <Users className="w-6 h-6" style={{ color: '#0B0E11' }} />
            </span>
            {t('followersPageTitle', language) || 'My Followers'}
          </h1>
          <div
            className="flex items-center gap-2 text-sm"
            style={{ color: '#848E9C' }}
          >
            <RefreshCw className="w-4 h-4" />
            <span>{t('autoRefresh', language) || 'Auto-refresh: 30s'}</span>
          </div>
        </div>
        <p className="text-sm" style={{ color: '#848E9C' }}>
          {t('followersPageDescription', language) ||
            'View all followers of your traders and their activities in one place.'}
        </p>
      </div>

      {/* Summary Stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
        <div className="binance-card p-5">
          <div className="text-sm mb-2" style={{ color: '#848E9C' }}>
            {t('totalFollowers', language) || 'Total Followers'}
          </div>
          <div className="text-2xl font-bold" style={{ color: '#EAECEF' }}>
            {summaryStats.totalFollowers}
          </div>
        </div>
        <div className="binance-card p-5">
          <div className="text-sm mb-2" style={{ color: '#848E9C' }}>
            {t('activeFollowers', language) || 'Active Followers'}
          </div>
          <div
            className="text-2xl font-bold flex items-center gap-2"
            style={{ color: '#0ECB81' }}
          >
            <Activity className="w-5 h-5" />
            {summaryStats.activeFollowers}
          </div>
        </div>
        <div className="binance-card p-5">
          <div className="text-sm mb-2" style={{ color: '#848E9C' }}>
            {t('parentTraders', language) || 'Parent Traders'}
          </div>
          <div
            className="text-2xl font-bold"
            style={{ color: 'var(--green-primary)' }}
          >
            {summaryStats.totalParents}
          </div>
        </div>
      </div>

      {/* Parent Traders List */}
      <div className="space-y-4">
        {followersData.parent_traders.map((parent) => {
          const isExpanded = expandedParents.has(parent.trader_id)
          return (
            <div
              key={parent.trader_id}
              className="binance-card overflow-hidden"
            >
              {/* Parent Trader Header */}
              <button
                onClick={() => toggleParentExpanded(parent.trader_id)}
                className="w-full p-6 flex items-center justify-between hover:opacity-80 transition-opacity"
                style={{
                  background: isExpanded
                    ? 'rgba(0, 255, 127, 0.05)'
                    : 'transparent',
                }}
              >
                <div className="flex items-center gap-4">
                  {isExpanded ? (
                    <ChevronUp
                      className="w-5 h-5"
                      style={{ color: 'var(--green-primary)' }}
                    />
                  ) : (
                    <ChevronDown
                      className="w-5 h-5"
                      style={{ color: '#848E9C' }}
                    />
                  )}
                  <div className="text-left">
                    <h3
                      className="text-xl font-bold"
                      style={{ color: '#EAECEF' }}
                    >
                      {parent.trader_name}
                    </h3>
                    <div className="text-sm mt-1" style={{ color: '#848E9C' }}>
                      {parent.followers.length}{' '}
                      {parent.followers.length === 1
                        ? t('follower', language) || 'Follower'
                        : t('followers', language) || 'Followers'}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-4">
                  <div
                    className="px-3 py-1 rounded text-sm font-semibold"
                    style={{
                      background: 'rgba(0, 255, 127, 0.1)',
                      color: 'var(--green-primary)',
                      border: '1px solid rgba(0, 255, 127, 0.2)',
                    }}
                  >
                    {parent.followers.filter((f) => f.is_running).length}{' '}
                    {t('active', language) || 'active'}
                  </div>
                </div>
              </button>

              {/* Followers List */}
              {isExpanded && (
                <div className="px-6 pb-6 space-y-4">
                  {parent.followers.map((follower) => (
                    <FollowerCard
                      key={follower.trader_id}
                      follower={follower}
                      language={language}
                      parentTraderId={parent.trader_id}
                      parentTraderName={parent.trader_name}
                    />
                  ))}
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}

interface FollowerCardProps {
  follower: FollowerWithActivities
  language: Language
  parentTraderId: string
  parentTraderName: string
}

function FollowerCard({
  follower,
  language,
  parentTraderId,
  parentTraderName,
}: FollowerCardProps) {
  const account = follower.account as any
  const hasError = 'error' in follower.account
  const navigate = useNavigate()

  const handleViewDashboard = () => {
    // Navigate to parent trader's dashboard using slug format
    const slug = generateTraderSlug(parentTraderName, parentTraderId)
    navigate(`/dashboard/${slug}`)
  }

  return (
    <div
      className="p-5 rounded-lg border"
      style={{
        background: 'var(--navy-dark)',
        borderColor: 'var(--navy-light)',
      }}
    >
      {/* Follower Header */}
      <div className="flex items-start justify-between mb-4">
        <div className="flex-1">
          <div className="flex items-center gap-3 mb-2">
            <h4 className="text-lg font-bold" style={{ color: '#EAECEF' }}>
              {follower.trader_name}
            </h4>
            {follower.is_running ? (
              <span
                className="px-2 py-1 rounded text-xs font-semibold flex items-center gap-1"
                style={{
                  background: 'rgba(14, 203, 129, 0.1)',
                  color: '#0ECB81',
                  border: '1px solid rgba(14, 203, 129, 0.2)',
                }}
              >
                <div
                  className="w-2 h-2 rounded-full"
                  style={{ background: '#0ECB81' }}
                ></div>
                {t('running', language) || 'Running'}
              </span>
            ) : (
              <span
                className="px-2 py-1 rounded text-xs font-semibold"
                style={{
                  background: 'rgba(246, 70, 93, 0.1)',
                  color: '#F6465D',
                  border: '1px solid rgba(246, 70, 93, 0.2)',
                }}
              >
                {t('stopped', language) || 'Stopped'}
              </span>
            )}
          </div>
          <div className="text-sm space-y-1" style={{ color: '#848E9C' }}>
            <div className="font-semibold" style={{ color: '#EAECEF' }}>
              {follower.user_email || follower.user_id}
            </div>
            {follower.user_email &&
              follower.user_email !== follower.user_id && (
                <div className="text-xs">
                  {t('userID', language) || 'User ID'}: {follower.user_id}
                </div>
              )}
            {follower.user_name &&
              follower.user_name !== follower.user_email && (
                <div className="text-xs">
                  {t('name', language) || 'Name'}: {follower.user_name}
                </div>
              )}
          </div>
        </div>
        <button
          onClick={handleViewDashboard}
          className="px-4 py-2 rounded text-sm font-semibold flex items-center gap-2 transition-all hover:scale-105"
          style={{
            background: 'rgba(0, 255, 127, 0.1)',
            color: 'var(--green-primary)',
            border: '1px solid rgba(0, 255, 127, 0.2)',
          }}
        >
          {t('viewDashboard', language) || 'View Dashboard'}
          <ExternalLink className="w-4 h-4" />
        </button>
      </div>

      {/* Account Info */}
      {hasError ? (
        <div
          className="mb-4 p-3 rounded text-sm"
          style={{ background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }}
        >
          {t('accountInfoUnavailable', language) ||
            'Account information unavailable'}
        </div>
      ) : (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
          <div>
            <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
              {t('totalEquity', language) || 'Total Equity'}
            </div>
            <div className="text-lg font-bold" style={{ color: '#EAECEF' }}>
              {account?.total_equity?.toFixed(2) || '0.00'} USDT
            </div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
              {t('totalPnL', language) || 'Total P&L'}
            </div>
            <div
              className={`text-lg font-bold flex items-center gap-1 ${
                (account?.total_pnl ?? 0) >= 0
                  ? 'text-green-500'
                  : 'text-red-500'
              }`}
            >
              {(account?.total_pnl ?? 0) >= 0 ? (
                <TrendingUp className="w-4 h-4" />
              ) : (
                <TrendingDown className="w-4 h-4" />
              )}
              {account?.total_pnl !== undefined && account.total_pnl >= 0
                ? '+'
                : ''}
              {account?.total_pnl?.toFixed(2) || '0.00'} USDT
            </div>
            <div className="text-xs" style={{ color: '#848E9C' }}>
              ({account?.total_pnl_pct?.toFixed(2) || '0.00'}%)
            </div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
              {t('positions', language) || 'Positions'}
            </div>
            <div className="text-lg font-bold" style={{ color: '#EAECEF' }}>
              {account?.position_count || 0}
            </div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
              {t('marginUsed', language) || 'Margin Used'}
            </div>
            <div className="text-lg font-bold" style={{ color: '#EAECEF' }}>
              {account?.margin_used_pct?.toFixed(1) || '0.0'}%
            </div>
          </div>
        </div>
      )}

      {/* Positions */}
      {follower.positions && follower.positions.length > 0 && (
        <div className="mb-4">
          <div
            className="text-sm font-semibold mb-2"
            style={{ color: '#848E9C' }}
          >
            {t('currentPositions', language) || 'Current Positions'} (
            {follower.positions.length})
          </div>
          <div className="space-y-2">
            {follower.positions.slice(0, 3).map((pos: any, idx: number) => (
              <div
                key={idx}
                className="p-3 rounded text-sm"
                style={{
                  background: 'var(--navy-primary)',
                  border: '1px solid var(--navy-light)',
                }}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="font-mono font-bold">{pos.symbol}</span>
                    <span
                      className="px-2 py-0.5 rounded text-xs font-bold"
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
                      {pos.side?.toUpperCase()}
                    </span>
                  </div>
                  <div className="text-right">
                    <div className="font-mono">
                      {pos.unrealized_pnl !== undefined &&
                      pos.unrealized_pnl >= 0
                        ? '+'
                        : ''}
                      {pos.unrealized_pnl?.toFixed(2) || '0.00'} USDT
                    </div>
                    <div className="text-xs" style={{ color: '#848E9C' }}>
                      {pos.unrealized_pnl_pct?.toFixed(2) || '0.00'}%
                    </div>
                  </div>
                </div>
              </div>
            ))}
            {follower.positions.length > 3 && (
              <div
                className="text-xs text-center pt-2"
                style={{ color: '#848E9C' }}
              >
                +{follower.positions.length - 3} {t('more', language) || 'More'}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Recent Decisions */}
      {follower.latest_decisions && follower.latest_decisions.length > 0 && (
        <div>
          <div
            className="text-sm font-semibold mb-2"
            style={{ color: '#848E9C' }}
          >
            {t('recentDecisions', language) || 'Recent Decisions'} (
            {follower.latest_decisions.length})
          </div>
          <div className="space-y-2">
            {follower.latest_decisions.slice(0, 3).map((decision, idx) => (
              <div
                key={idx}
                className="p-3 rounded text-sm"
                style={{
                  background: 'var(--navy-primary)',
                  border: '1px solid var(--navy-light)',
                }}
              >
                <div className="flex items-center justify-between mb-1">
                  <div className="flex items-center gap-2">
                    {decision.success ? (
                      <div
                        className="w-2 h-2 rounded-full"
                        style={{ background: '#0ECB81' }}
                      ></div>
                    ) : (
                      <div
                        className="w-2 h-2 rounded-full"
                        style={{ background: '#F6465D' }}
                      ></div>
                    )}
                    <span className="text-xs" style={{ color: '#848E9C' }}>
                      Cycle #{decision.cycle_number}
                    </span>
                  </div>
                  <span className="text-xs" style={{ color: '#848E9C' }}>
                    {new Date(decision.timestamp).toLocaleString()}
                  </span>
                </div>
                {decision.decisions && decision.decisions.length > 0 && (
                  <div className="text-xs mt-2" style={{ color: '#848E9C' }}>
                    {decision.decisions.length}{' '}
                    {decision.decisions.length === 1
                      ? t('decision', language) || 'decision'
                      : t('decisions', language) || 'decisions'}
                  </div>
                )}
                {decision.error_message && (
                  <div className="text-xs mt-1" style={{ color: '#F6465D' }}>
                    {decision.error_message}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
