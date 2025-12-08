import { Trophy, TrendingUp, TrendingDown, Eye } from 'lucide-react'
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

  return (
    <div
      className="binance-card-enhanced p-6 md:p-8 relative overflow-hidden animate-slide-in"
      style={{
        animationDelay: '0.05s',
        background: isPositive
          ? 'linear-gradient(135deg, rgba(0, 255, 127, 0.08) 0%, rgba(11, 14, 17, 0.95) 50%, rgba(0, 255, 127, 0.05) 100%)'
          : 'linear-gradient(135deg, rgba(246, 70, 93, 0.08) 0%, rgba(11, 14, 17, 0.95) 50%, rgba(246, 70, 93, 0.05) 100%)',
        border: isPositive
          ? '2px solid rgba(0, 255, 127, 0.3)'
          : '2px solid rgba(246, 70, 93, 0.3)',
        boxShadow: isPositive
          ? '0 8px 32px rgba(0, 255, 127, 0.15), 0 0 0 1px rgba(0, 255, 127, 0.1)'
          : '0 8px 32px rgba(246, 70, 93, 0.15), 0 0 0 1px rgba(246, 70, 93, 0.1)',
      }}
    >
      {/* Animated background gradient - Subtle accent */}
      <div
        className="absolute top-0 right-0 w-32 h-32 md:w-40 md:h-40 opacity-20 pointer-events-none"
        style={{
          background: isPositive
            ? 'radial-gradient(circle, rgba(0, 255, 127, 0.3) 0%, transparent 70%)'
            : 'radial-gradient(circle, rgba(246, 70, 93, 0.3) 0%, transparent 70%)',
        }}
      />

      {/* Content */}
      <div className="relative z-10">
        {/* Header - Single Line: Trophy | 1 LEADER | Trader Name | View Details | LIVE */}
        <div className="flex items-center gap-3 md:gap-4 mb-6 flex-wrap">
          {/* Trophy Icon with Badge */}
          <div
            className="w-10 h-10 md:w-12 md:h-12 rounded-lg flex items-center justify-center relative flex-shrink-0"
            style={{
              background: isPositive
                ? 'linear-gradient(135deg, var(--green-primary) 0%, var(--green-light) 100%)'
                : 'linear-gradient(135deg, var(--error) 0%, rgba(246, 70, 93, 0.8) 100%)',
              boxShadow: isPositive
                ? '0 4px 16px rgba(0, 255, 127, 0.4)'
                : '0 4px 16px rgba(246, 70, 93, 0.4)',
            }}
          >
            <Trophy className="w-5 h-5 md:w-6 md:h-6" style={{ color: '#000' }} />
            <div
              className="absolute -top-1 -right-1 w-4 h-4 md:w-5 md:h-5 rounded-full flex items-center justify-center text-xs font-bold"
              style={{
                background: 'var(--gold-primary)',
                color: '#000',
                boxShadow: '0 2px 8px rgba(255, 215, 0, 0.5)',
              }}
            >
              1
            </div>
          </div>

          {/* Leader Label */}
          <div className="flex items-center gap-2 flex-shrink-0">
            <span
              className="text-xs md:text-sm font-bold mono"
              style={{ color: 'var(--gold-primary)' }}
            >
              1
            </span>
            <span
              className="text-xs md:text-sm uppercase tracking-wider font-semibold"
              style={{ color: 'var(--text-gray-light)' }}
            >
              {t('leader', language)}
            </span>
          </div>

          {/* Trader Name */}
          <div className="flex-1 min-w-0">
            <h2
              className="text-base md:text-lg lg:text-xl font-bold truncate"
              style={{ color: 'var(--text-white)' }}
            >
              {leader.trader_name}
            </h2>
          </div>

          {/* View Details Button */}
          {onViewDetails && (
            <button
              onClick={onViewDetails}
              className="flex items-center justify-center gap-1.5 md:gap-2 px-3 md:px-4 py-1.5 md:py-2 rounded-lg font-semibold text-xs md:text-sm transition-all duration-300 hover:scale-105 active:scale-95 flex-shrink-0"
              style={{
                background: 'rgba(0, 255, 127, 0.15)',
                color: 'var(--green-primary)',
                border: '1px solid rgba(0, 255, 127, 0.3)',
              }}
            >
              <Eye className="w-3.5 h-3.5 md:w-4 md:h-4" />
              <span className="hidden sm:inline">{t('viewDetails', language) || 'View Details'}</span>
              <span className="sm:hidden">{t('view', language)}</span>
            </button>
          )}

          {/* LIVE Status Badge */}
          <div className="flex items-center gap-2 flex-shrink-0">
            <div
              className={`w-2 h-2 rounded-full ${
                leader.is_running ? 'pulse-live' : ''
              }`}
              style={{
                background: leader.is_running ? 'var(--green-primary)' : 'var(--error)',
                boxShadow: leader.is_running
                  ? '0 0 8px var(--green-primary)'
                  : 'none',
              }}
            />
            <span
              className="text-xs md:text-sm font-semibold uppercase tracking-wider"
              style={{
                color: leader.is_running ? 'var(--green-primary)' : 'var(--error)',
              }}
            >
              {leader.is_running
                ? t('live', language) || 'LIVE'
                : t('stopped', language) || 'STOPPED'}
            </span>
          </div>
        </div>

        {/* Stats Grid - Equidistant spacing */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4 mt-6">
          {/* PnL Percentage */}
          <div className="col-span-2 md:col-span-1 flex flex-col">
            <div
              className="text-xs uppercase tracking-wider mb-2 font-semibold"
              style={{ color: 'var(--text-gray-light)' }}
            >
              {t('pnl', language)}
            </div>
            <div className="flex items-baseline gap-2">
              {isPositive ? (
                <TrendingUp className="w-4 h-4 md:w-5 md:h-5 flex-shrink-0 mt-0.5" style={{ color: 'var(--green-primary)' }} />
              ) : (
                <TrendingDown className="w-4 h-4 md:w-5 md:h-5 flex-shrink-0 mt-0.5" style={{ color: 'var(--error)' }} />
              )}
              <div className="flex flex-col">
                <div
                  className="text-xl md:text-2xl lg:text-3xl font-bold mono leading-tight"
                  style={{
                    color: isPositive ? 'var(--green-primary)' : 'var(--error)',
                  }}
                >
                  {isPositive ? '+' : ''}
                  {pnlPct}%
                </div>
                <div
                  className="text-xs md:text-sm mono mt-1"
                  style={{ color: 'var(--text-gray-light)' }}
                >
                  {isPositive ? '+' : ''}
                  {pnlValue} USDT
                </div>
              </div>
            </div>
          </div>

          {/* Equity */}
          <div className="flex flex-col">
            <div
              className="text-xs uppercase tracking-wider mb-2 font-semibold"
              style={{ color: 'var(--text-gray-light)' }}
            >
              {t('equity', language)}
            </div>
            <div
              className="text-lg md:text-xl lg:text-2xl font-bold mono leading-tight"
              style={{ color: 'var(--text-white)' }}
            >
              {leader.total_equity?.toFixed(2) || '0.00'}
            </div>
            <div
              className="text-xs mt-1"
              style={{ color: 'var(--text-gray-light)' }}
            >
              USDT
            </div>
          </div>

          {/* Positions */}
          <div className="flex flex-col">
            <div
              className="text-xs uppercase tracking-wider mb-2 font-semibold"
              style={{ color: 'var(--text-gray-light)' }}
            >
              {t('pos', language)}
            </div>
            <div
              className="text-lg md:text-xl lg:text-2xl font-bold mono leading-tight"
              style={{ color: 'var(--text-white)' }}
            >
              {leader.position_count}
            </div>
            <div
              className="text-xs mt-1"
              style={{ color: 'var(--text-gray-light)' }}
            >
              {leader.margin_used_pct.toFixed(1)}% {t('margin', language) || 'Margin'}
            </div>
          </div>

          {/* AI Model & Exchange */}
          <div className="flex flex-col">
            <div
              className="text-xs uppercase tracking-wider mb-2 font-semibold"
              style={{ color: 'var(--text-gray-light)' }}
            >
              {t('aiModel', language) || 'AI Model'}
            </div>
            <div
              className="text-base md:text-lg lg:text-xl font-bold truncate leading-tight"
              style={{ color: leaderColor }}
            >
              {leader.ai_model.toUpperCase()}
            </div>
            <div
              className="text-xs mt-1 mono"
              style={{ color: 'var(--text-gray-light)' }}
            >
              {leader.exchange.toUpperCase()}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

