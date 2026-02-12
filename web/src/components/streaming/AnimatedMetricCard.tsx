import React from 'react'
import { motion } from 'framer-motion'
import { useCounterAnimation } from '../../hooks/useCounterAnimation'
import { soundSystem } from '../../lib/sound'
import { TrendingUp, TrendingDown } from 'lucide-react'

interface AnimatedMetricCardProps {
  title: string
  value: string | number
  change?: number
  positive?: boolean
  subtitle?: string
  animationSpeed?: number
  color?: string
  icon?: React.ReactNode
}

/**
 * Animated Metric Card with counting animations
 * Features:
 * - Number counting up/down smoothly
 * - Percentage changes with animated arrows
 * - Color pulses (Green/Red for profit/loss)
 * - Background glow effects
 * - Scale animations on updates
 */
export function AnimatedMetricCard({
  title,
  value,
  change,
  positive,
  subtitle,
  animationSpeed = 1,
  color = '#60a5fa',
  icon,
}: AnimatedMetricCardProps) {
  // Handle numeric values
  const isNumeric = typeof value === 'number'
  const numericValue = isNumeric
    ? value
    : parseFloat(String(value).replace(/[^\d.-]/g, '')) || 0

  const animatedValue = useCounterAnimation({
    start: 0,
    end: numericValue,
    duration: 2000 / animationSpeed,
    decimals: isNumeric && numericValue % 1 !== 0 ? 2 : 0,
  })

  const animatedChange = useCounterAnimation({
    start: 0,
    end: change || 0,
    duration: 1500 / animationSpeed,
    decimals: 1,
  })

  // Play sound effect for significant changes
  React.useEffect(() => {
    if (change && Math.abs(change) > 5) {
      soundSystem.playValueChange(change > 0)
    }
  }, [change])

  const displayValue = isNumeric
    ? animatedValue.toFixed(2)
    : String(value).replace(/\d+(\.\d+)?/, animatedValue.toFixed(2))

  // Determine if this is an "active" metric that should have continuous animation
  const isActiveMetric = title === 'Total Equity' || title === 'Daily P&L'

  return (
    <motion.div
      className="stat-card"
      style={{
        background: 'var(--navy-dark)',
        border: '1px solid var(--navy-light)',
        borderRadius: '12px',
        padding: '20px',
        position: 'relative',
        overflow: 'hidden',
        height: '120px',
        boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
        willChange: 'transform',
      }}
      whileHover={{
        scale: 1.03,
        boxShadow: '0 8px 25px rgba(0, 0, 0, 0.15)',
        borderColor: color,
      }}
      whileTap={{ scale: 0.98 }}
      animate={
        isActiveMetric
          ? {
              boxShadow: [
                '0 2px 8px rgba(0, 0, 0, 0.1)',
                '0 4px 15px rgba(0, 0, 0, 0.15)',
                '0 2px 8px rgba(0, 0, 0, 0.1)',
              ],
            }
          : {}
      }
      transition={{
        boxShadow: {
          duration: 2,
          repeat: isActiveMetric ? Infinity : 0,
          ease: 'easeInOut',
        },
      }}
    >
      {/* Background glow effect */}
      <motion.div
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          background: `radial-gradient(circle at 30% 20%, ${color}15 0%, transparent 70%)`,
          opacity: 0.5,
        }}
        animate={{
          opacity: [0.3, 0.7, 0.3],
          scale: [1, 1.05, 1],
        }}
        transition={{
          duration: 4,
          repeat: Infinity,
          ease: 'easeInOut',
        }}
      />

      {/* Pulse effect for significant changes */}
      {change && Math.abs(change) > 2 && (
        <motion.div
          style={{
            position: 'absolute',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            border: `2px solid ${color}`,
            borderRadius: '12px',
            pointerEvents: 'none',
          }}
          initial={{ opacity: 0, scale: 0.8 }}
          animate={{
            opacity: [0, 0.8, 0],
            scale: [0.8, 1.1, 0.8],
          }}
          transition={{
            duration: 2,
            repeat: Infinity,
            ease: 'easeOut',
          }}
        />
      )}

      <div style={{ position: 'relative', zIndex: 1 }}>
        {/* Header */}
        <div className="flex items-center justify-between mb-3">
          <div
            className="text-xs mono uppercase tracking-wider"
            style={{ color: '#848E9C' }}
          >
            {title}
          </div>
          {icon && (
            <motion.div
              animate={{
                rotate: [0, 3, -3, 0],
                scale: [1, 1.1, 1],
              }}
              transition={{
                duration: 3,
                repeat: Infinity,
                ease: 'easeInOut',
              }}
            >
              {icon}
            </motion.div>
          )}
        </div>

        {/* Value */}
        <motion.div
          className="text-3xl font-bold mb-2 mono"
          style={{
            color: '#EAECEF',
            lineHeight: '1.2',
            textShadow:
              change && Math.abs(change) > 5 ? `0 0 10px ${color}40` : 'none',
          }}
          animate={
            change && Math.abs(change) > 1
              ? {
                  scale: [1, 1.08, 1],
                  textShadow: ['none', `0 0 15px ${color}60`, 'none'],
                }
              : {}
          }
          transition={{
            duration: change && Math.abs(change) > 5 ? 0.8 : 0.5,
            ease: 'easeOut',
          }}
        >
          {displayValue}
        </motion.div>

        {/* Change indicator */}
        {change !== undefined && (
          <motion.div
            className="flex items-center gap-1"
            initial={{ opacity: 0, y: 10, scale: 0.8 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            transition={{
              delay: 0.3,
              duration: 0.4,
              ease: [0.34, 1.56, 0.64, 1],
            }}
          >
            <motion.div
              animate={{
                color: positive ? '#0ECB81' : '#F6465D',
                scale: [1, 1.3, 1],
                rotate: positive ? [0, 10, -10, 0] : [0, -10, 10, 0],
              }}
              transition={{
                duration: 2,
                repeat: Math.abs(change) > 3 ? Infinity : 0,
                ease: 'easeInOut',
              }}
            >
              {positive !== undefined ? (
                positive ? (
                  <TrendingUp size={14} />
                ) : (
                  <TrendingDown size={14} />
                )
              ) : (
                <div className="w-2 h-2 rounded-full bg-current" />
              )}
            </motion.div>
            <motion.span
              className="text-sm mono font-bold"
              style={{ color: positive ? '#0ECB81' : '#F6465D' }}
              animate={
                Math.abs(change) > 2
                  ? {
                      scale: [1, 1.1, 1],
                    }
                  : {}
              }
              transition={{
                duration: 1,
                repeat: Math.abs(change) > 5 ? Infinity : 0,
                ease: 'easeInOut',
              }}
            >
              {positive ? '+' : ''}
              {animatedChange.toFixed(1)}%
            </motion.span>
          </motion.div>
        )}

        {/* Subtitle */}
        {subtitle && (
          <motion.div
            className="text-xs mt-2 mono"
            style={{ color: '#848E9C' }}
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ delay: 0.4 }}
          >
            {subtitle}
          </motion.div>
        )}
      </div>
    </motion.div>
  )
}
