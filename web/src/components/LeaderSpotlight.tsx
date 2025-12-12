import { Trophy, TrendingUp, TrendingDown, Users } from 'lucide-react'
import type { CompetitionTraderData } from '../types'
import { getTraderColor } from '../utils/traderColors'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'

interface LeaderSpotlightProps {
  leader: CompetitionTraderData
  allTraders: CompetitionTraderData[]
  onViewDetails?: () => void
}

export function LeaderSpotlight({ leader, allTraders, onViewDetails }: LeaderSpotlightProps) {
  const { language } = useLanguage()
  const leaderColor = getTraderColor(allTraders, leader.trader_id)
  const isPositive = (leader.total_pnl ?? 0) >= 0
  const pnlPct = leader.total_pnl_pct?.toFixed(2) || '0.00'
  const pnlValue = leader.total_pnl?.toFixed(2) || '0.00'
  const isFollowerTrader = !!(leader.followed_trader_id && leader.followed_trader_id !== '')

  return (
    <div
      className="binance-card-enhanced p-3 relative overflow-hidden animate-slide-in"
      style={{
        animationDelay: '0.05s',
        background: isFollowerTrader
          ? 'linear-gradient(135deg, rgba(99, 102, 241, 0.08) 0%, rgba(11, 14, 17, 0.95) 50%, rgba(99, 102, 241, 0.05) 100%)'
          : isPositive
          ? 'linear-gradient(135deg, rgba(0, 255, 127, 0.08) 0%, rgba(11, 14, 17, 0.95) 50%, rgba(0, 255, 127, 0.05) 100%)'
          : 'linear-gradient(135deg, rgba(246, 70, 93, 0.08) 0%, rgba(11, 14, 17, 0.95) 50%, rgba(246, 70, 93, 0.05) 100%)',
        border: isFollowerTrader
          ? '1px solid rgba(99, 102, 241, 0.4)'
          : isPositive
          ? '1px solid rgba(0, 255, 127, 0.3)'
          : '1px solid rgba(246, 70, 93, 0.3)',
      }}
    >
      {/* Animated background gradient - Subtle accent */}
      <div
        className="absolute top-0 right-0 w-24 h-24 opacity-15 pointer-events-none"
        style={{
          background: isPositive
            ? 'radial-gradient(circle, rgba(0, 255, 127, 0.3) 0%, transparent 70%)'
            : 'radial-gradient(circle, rgba(246, 70, 93, 0.3) 0%, transparent 70%)',
        }}
      />

      {/* Content - Single Compact Row with Equidistant Spacing */}
      <div className="relative z-10 flex items-center justify-between gap-3">
        {/* Trophy Icon or Follower Badge */}
        <div
          className="w-7 h-7 rounded-md flex items-center justify-center flex-shrink-0 relative"
          style={{
            background: isFollowerTrader
              ? 'linear-gradient(135deg, #6366F1 0%, #8B5CF6 100%)'
              : isPositive
              ? 'linear-gradient(135deg, var(--green-primary) 0%, var(--green-light) 100%)'
              : 'linear-gradient(135deg, var(--error) 0%, rgba(246, 70, 93, 0.8) 100%)',
          }}
        >
          {isFollowerTrader ? (
            <Users className="w-3.5 h-3.5" style={{ color: '#FFFFFF' }} />
          ) : (
            <Trophy className="w-3.5 h-3.5" style={{ color: 'var(--navy-primary)' }} />
          )}
        </div>
        
        {/* Trader Name with Follower Badge */}
        <div className="flex items-center gap-1.5 flex-shrink-0 min-w-0">
          <h2
            className="text-sm md:text-base font-bold truncate"
            style={{ color: isFollowerTrader ? '#A5B4FC' : 'var(--text-white)' }}
          >
            {leader.trader_name}
          </h2>
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

        {/* Stats - Equidistantly Spaced */}
        <div className="flex items-center justify-between flex-1 gap-4 px-4">
          {/* P&L */}
          <div className="flex items-center gap-1.5 flex-shrink-0">
            {isPositive ? (
              <TrendingUp className="w-3 h-3 flex-shrink-0" style={{ color: 'var(--green-primary)' }} />
            ) : (
              <TrendingDown className="w-3 h-3 flex-shrink-0" style={{ color: 'var(--error)' }} />
            )}
            <span
              className="text-sm md:text-base font-bold mono whitespace-nowrap"
              style={{
                color: isPositive ? 'var(--green-primary)' : 'var(--error)',
              }}
            >
              {isPositive ? '+' : ''}
              {pnlPct}%
            </span>
          </div>

          {/* Equity */}
          <div className="flex items-center gap-1.5 flex-shrink-0">
            <span className="text-xs text-[#848E9C] whitespace-nowrap">{t('equity', language)}</span>
            <span className="text-sm md:text-base font-bold mono whitespace-nowrap" style={{ color: 'var(--text-white)' }}>
              {leader.total_equity?.toFixed(2) || '0.00'}
            </span>
          </div>

          {/* Positions */}
          <div className="flex items-center gap-1.5 flex-shrink-0">
            <span className="text-xs text-[#848E9C] whitespace-nowrap">{t('pos', language)}</span>
            <span className="text-sm md:text-base font-bold mono whitespace-nowrap" style={{ color: 'var(--text-white)' }}>
              {leader.position_count}
            </span>
          </div>
        </div>

        {/* LIVE Status */}
        <div className="flex items-center gap-1.5 flex-shrink-0">
          <div
            className={`w-2 h-2 rounded-full ${
              leader.is_running ? 'pulse-live' : ''
            }`}
            style={{
              background: leader.is_running ? 'var(--green-primary)' : 'var(--error)',
              boxShadow: leader.is_running
                ? '0 0 6px var(--green-primary)'
                : 'none',
            }}
          />
          <span
            className="text-xs font-semibold whitespace-nowrap"
            style={{
              color: leader.is_running ? 'var(--green-primary)' : 'var(--error)',
            }}
          >
            {t('live', language) || 'LIVE'}
          </span>
        </div>
      </div>
    </div>
  )
}

