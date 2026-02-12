import { useEffect, useState, forwardRef } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useAuth } from '../../contexts/AuthContext'
import { api } from '../../lib/api'
import { Position } from '../../types'
import { useCounterAnimation } from '../../hooks/useCounterAnimation'
import { XCircle, TrendingUp } from 'lucide-react'
import { confirmToast, notify } from '../../lib/notify'

interface AnimatedPositionTableProps {
  traderId?: string
  animationSpeed?: number
  maxHeight?: string
}

/**
 * Animated Position Table with real-time updates
 * Features:
 * - New rows slide in with highlight
 * - P&L numbers animate (Red → Green transitions)
 * - Entry price animations
 * - Liquidation price glowing alerts
 * - Close button with ripple effect
 */
export function AnimatedPositionTable({
  traderId,
  animationSpeed = 1,
  maxHeight = '400px',
}: AnimatedPositionTableProps) {
  const { user, token } = useAuth()
  const [positions, setPositions] = useState<Position[]>([])
  const [closingPositions, setClosingPositions] = useState<Set<string>>(
    new Set()
  )
  const [newPositionIds, setNewPositionIds] = useState<Set<string>>(new Set())

  // Load positions
  useEffect(() => {
    if (!user || !token || !traderId) return

    const loadPositions = async () => {
      try {
        const data = await api.getPositions(traderId)
        // Ensure we always set an array, never null or undefined
        setPositions(Array.isArray(data) ? data : [])
      } catch (error) {
        console.error('Failed to load positions:', error)
      }
    }

    loadPositions()

    // Poll for position updates (WebSocket not implemented yet)
    const pollPositions = async () => {
      try {
        const updatedPositions = await api.getPositions(traderId)

        // Ensure we have valid arrays before processing
        const safeUpdatedPositions = Array.isArray(updatedPositions)
          ? updatedPositions
          : []
        const safePositions = Array.isArray(positions) ? positions : []

        // Mark new positions for animation
        const existingIds = new Set(
          safePositions.map((p) => `${p.symbol}-${p.side}`)
        )
        const newIds = safeUpdatedPositions
          .filter((p) => !existingIds.has(`${p.symbol}-${p.side}`))
          .map((p) => `${p.symbol}-${p.side}`)

        if (newIds.length > 0) {
          setNewPositionIds(new Set(newIds))
          // Remove highlight after animation
          setTimeout(() => {
            setNewPositionIds((prev) => {
              const next = new Set(prev)
              newIds.forEach((id) => next.delete(id))
              return next
            })
          }, 3000)
        }

        setPositions(safeUpdatedPositions)
      } catch (error) {
        console.error('Failed to poll positions:', error)
      }
    }

    // Poll every 5 seconds
    const interval = setInterval(pollPositions, 5000)

    return () => clearInterval(interval)
  }, [user, token, traderId, positions])

  const handleClosePosition = async (
    symbol: string,
    side: 'long' | 'short'
  ) => {
    if (!traderId) return

    const positionKey = `${symbol}-${side}`
    if (closingPositions.has(positionKey)) return

    const confirmed = await confirmToast(
      `Close ${side} position for ${symbol}?`,
      { title: 'Confirm Close' }
    )

    if (!confirmed) return

    setClosingPositions((prev) => new Set(prev).add(positionKey))

    try {
      await api.closePosition(traderId, symbol, side, 0)
      notify.success('Position closed successfully')

      // Update positions
      const updated = await api.getPositions(traderId)
      // Ensure we always set an array, never null or undefined
      setPositions(Array.isArray(updated) ? updated : [])
    } catch (error: any) {
      notify.error(error.message || 'Failed to close position')
    } finally {
      setClosingPositions((prev) => {
        const next = new Set(prev)
        next.delete(positionKey)
        return next
      })
    }
  }

  return (
    <motion.div
      className="binance-card p-6 h-full flex flex-col"
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.5 }}
    >
      <div className="flex items-center justify-between mb-6 flex-shrink-0">
        <h3
          className="text-xl font-bold flex items-center gap-2"
          style={{ color: '#EAECEF' }}
        >
          <TrendingUp
            className="w-5 h-5"
            style={{ color: 'var(--green-primary)' }}
          />
          Positions ({Array.isArray(positions) ? positions.length : 0})
        </h3>
      </div>

      <div className="overflow-y-auto flex-1" style={{ maxHeight }}>
        <AnimatePresence mode="popLayout">
          {!Array.isArray(positions) || positions.length === 0 ? (
            <motion.div
              className="text-center py-16"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
            >
              <div className="opacity-30 flex justify-center mb-4">
                <TrendingUp className="w-16 h-16" />
              </div>
              <div style={{ color: '#848E9C' }}>No active positions</div>
            </motion.div>
          ) : (
            (Array.isArray(positions) ? positions : []).map(
              (position, index) => (
                <AnimatedPositionRow
                  key={`${position.symbol}-${position.side}`}
                  position={position}
                  index={index}
                  isNew={newPositionIds.has(
                    `${position.symbol}-${position.side}`
                  )}
                  isClosing={closingPositions.has(
                    `${position.symbol}-${position.side}`
                  )}
                  animationSpeed={animationSpeed}
                  onClose={() =>
                    handleClosePosition(
                      position.symbol,
                      position.side as 'long' | 'short'
                    )
                  }
                />
              )
            )
          )}
        </AnimatePresence>
      </div>
    </motion.div>
  )
}

interface AnimatedPositionRowProps {
  position: Position
  index: number
  isNew: boolean
  isClosing: boolean
  animationSpeed: number
  onClose: () => void
}

const AnimatedPositionRow = forwardRef<
  HTMLDivElement,
  AnimatedPositionRowProps
>(function AnimatedPositionRow(
  { position, index, isNew, isClosing, animationSpeed, onClose },
  ref
) {
  const animatedPnl = useCounterAnimation({
    start: 0,
    end:
      typeof position.unrealized_pnl === 'number' ? position.unrealized_pnl : 0,
    duration: 1500 / animationSpeed,
    decimals: 2,
  })

  const animatedPnlPct = useCounterAnimation({
    start: 0,
    end:
      typeof position.unrealized_pnl_pct === 'number'
        ? position.unrealized_pnl_pct
        : 0,
    duration: 1200 / animationSpeed,
    decimals: 2,
  })

  const isProfit = position.unrealized_pnl >= 0
  const liqDistance =
    (Math.abs(position.mark_price - position.liquidation_price) /
      position.mark_price) *
    100
  const isNearLiquidation = liqDistance < 5 // Within 5% of liquidation

  return (
    <motion.div
      ref={ref}
      className="rounded-lg p-5 mb-4"
      style={{
        background: 'var(--navy-dark)',
        border: `1px solid ${isNew ? 'rgba(0, 255, 127, 0.5)' : 'var(--navy-light)'}`,
        position: 'relative',
        overflow: 'hidden',
        boxShadow: isNew
          ? '0 4px 12px rgba(0, 255, 127, 0.2)'
          : '0 2px 4px rgba(0, 0, 0, 0.1)',
      }}
      initial={{ opacity: 0, x: -20 }}
      animate={{
        opacity: 1,
        x: 0,
        boxShadow: isNew ? '0 0 20px rgba(0, 255, 127, 0.3)' : 'none',
      }}
      exit={{ opacity: 0, x: 20 }}
      transition={{
        duration: 0.3,
        delay: index * 0.1,
      }}
      whileHover={{ scale: 1.01 }}
    >
      {/* New position highlight */}
      {isNew && (
        <motion.div
          style={{
            position: 'absolute',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            background:
              'linear-gradient(45deg, rgba(0, 255, 127, 0.1), transparent)',
            pointerEvents: 'none',
          }}
          animate={{ opacity: [0.5, 0, 0.5] }}
          transition={{ duration: 2, repeat: Infinity }}
        />
      )}

      {/* Liquidation warning glow */}
      {isNearLiquidation && (
        <motion.div
          style={{
            position: 'absolute',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            background: 'rgba(246, 70, 93, 0.1)',
            pointerEvents: 'none',
          }}
          animate={{ opacity: [0.3, 0.6, 0.3] }}
          transition={{ duration: 1, repeat: Infinity }}
        />
      )}

      <div style={{ position: 'relative', zIndex: 1 }}>
        {/* Header */}
        <div className="flex items-start justify-between mb-3">
          <div>
            <div className="flex items-center gap-2 mb-1">
              <span
                className="text-lg font-bold font-mono"
                style={{ color: '#EAECEF' }}
              >
                {position.symbol}
              </span>
              <motion.span
                className="px-2 py-1 rounded text-xs font-bold"
                style={{
                  background:
                    position.side === 'long'
                      ? 'rgba(14, 203, 129, 0.1)'
                      : 'rgba(246, 70, 93, 0.1)',
                  color: position.side === 'long' ? '#0ECB81' : '#F6465D',
                }}
                animate={isNew ? { scale: [1, 1.1, 1] } : {}}
                transition={{ duration: 0.5 }}
              >
                {position.side.toUpperCase()}
              </motion.span>
              <span
                className="text-xs"
                style={{ color: 'var(--green-primary)' }}
              >
                {position.leverage}x
              </span>
            </div>
          </div>

          {/* P&L */}
          <div className="text-right">
            <motion.div
              className="text-xl font-bold font-mono"
              style={{ color: isProfit ? '#0ECB81' : '#F6465D' }}
              animate={{ scale: isNew ? [1, 1.2, 1] : [1] }}
              transition={{ duration: 0.5 }}
            >
              {isProfit ? '+' : ''}${animatedPnl.toFixed(2)}
            </motion.div>
            <div className="text-xs" style={{ color: '#848E9C' }}>
              {animatedPnlPct.toFixed(2)}%
            </div>
          </div>
        </div>

        {/* Details Grid */}
        <div className="grid grid-cols-2 gap-3 mb-3">
          <div>
            <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
              Entry Price
            </div>
            <div
              className="font-mono font-semibold"
              style={{ color: '#EAECEF' }}
            >
              ${position.entry_price.toFixed(4)}
            </div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
              Mark Price
            </div>
            <div
              className="font-mono font-semibold"
              style={{ color: '#EAECEF' }}
            >
              ${position.mark_price.toFixed(4)}
            </div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
              Quantity
            </div>
            <div
              className="font-mono font-semibold"
              style={{ color: '#EAECEF' }}
            >
              {position.quantity.toFixed(4)}
            </div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
              Liq. Price
              {isNearLiquidation && (
                <motion.span
                  className="ml-1 text-xs"
                  style={{ color: '#F6465D' }}
                  animate={{ opacity: [1, 0.5, 1] }}
                  transition={{ duration: 0.8, repeat: Infinity }}
                >
                  ⚠️
                </motion.span>
              )}
            </div>
            <div
              className="font-mono font-semibold"
              style={{ color: isNearLiquidation ? '#F6465D' : '#848E9C' }}
            >
              ${position.liquidation_price.toFixed(4)}
            </div>
          </div>
        </div>

        {/* Close Button */}
        <motion.button
          onClick={onClose}
          disabled={isClosing}
          className="w-full px-4 py-2 rounded text-sm font-semibold transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
          style={{
            background: isClosing
              ? 'rgba(132, 142, 156, 0.2)'
              : 'rgba(246, 70, 93, 0.15)',
            color: isClosing ? '#848E9C' : '#F6465D',
            border: `1px solid ${isClosing ? 'rgba(132, 142, 156, 0.3)' : 'rgba(246, 70, 93, 0.3)'}`,
          }}
          whileHover={!isClosing ? { scale: 1.02 } : {}}
          whileTap={!isClosing ? { scale: 0.98 } : {}}
        >
          {isClosing ? (
            <>
              <motion.div
                className="inline-block w-4 h-4 mr-2"
                animate={{ rotate: 360 }}
                transition={{ duration: 1, repeat: Infinity, ease: 'linear' }}
              >
                ⟳
              </motion.div>
              Closing...
            </>
          ) : (
            <>
              <XCircle className="w-4 h-4 inline-block mr-2" />
              Close Position
            </>
          )}
        </motion.button>
      </div>
    </motion.div>
  )
})
