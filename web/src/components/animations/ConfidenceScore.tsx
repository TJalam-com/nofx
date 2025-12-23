import { motion } from 'framer-motion'
import { useCounterAnimation } from '../../hooks/useCounterAnimation'

interface ConfidenceScoreProps {
  confidence: number // 0-100
  duration?: number // Animation duration in ms
  className?: string
  showLabel?: boolean
}

/**
 * Incremental confidence score animation (0% → target%).
 * Visual progress bar with gradient fill.
 * Pulsing glow effect when reaching high confidence (>80%).
 */
export function ConfidenceScore({
  confidence,
  duration = 1500,
  className = '',
  showLabel = true,
}: ConfidenceScoreProps) {
  const animatedValue = useCounterAnimation({
    start: 0,
    end: confidence,
    duration,
    decimals: 1,
  })

  const isHighConfidence = confidence >= 80
  const normalizedConfidence = Math.max(0, Math.min(100, confidence))

  return (
    <div className={className} style={{ width: '100%' }}>
      {showLabel && (
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            marginBottom: '8px',
          }}
        >
          <span
            style={{
              fontSize: '13px',
              fontWeight: '500',
              color: '#848E9C',
            }}
          >
            Confidence
          </span>
          <motion.span
            style={{
              fontSize: '16px',
              fontWeight: 'bold',
              color: isHighConfidence ? '#0ECB81' : '#60a5fa',
            }}
            animate={
              isHighConfidence
                ? {
                    scale: [1, 1.1, 1],
                  }
                : {}
            }
            transition={{
              duration: 2,
              repeat: isHighConfidence ? Infinity : 0,
              ease: 'easeInOut',
            }}
          >
            {animatedValue.toFixed(1)}%
          </motion.span>
        </div>
      )}

      {/* Progress Bar */}
      <div
        style={{
          width: '100%',
          height: '8px',
          background: 'rgba(255, 255, 255, 0.05)',
          borderRadius: '4px',
          overflow: 'hidden',
          position: 'relative',
        }}
      >
        <motion.div
          style={{
            height: '100%',
            width: `${normalizedConfidence}%`,
            background: isHighConfidence
              ? 'linear-gradient(90deg, #0ECB81, #10B981)'
              : 'linear-gradient(90deg, #60a5fa, #3b82f6)',
            borderRadius: '4px',
            position: 'relative',
          }}
          initial={{ width: 0 }}
          animate={{ width: `${normalizedConfidence}%` }}
          transition={{ duration: duration / 1000, ease: 'easeOut' }}
        >
          {/* Shimmer effect */}
          <motion.div
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
              background:
                'linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.3), transparent)',
            }}
            animate={{
              x: ['-100%', '200%'],
            }}
            transition={{
              duration: 2,
              repeat: Infinity,
              ease: 'linear',
            }}
          />
        </motion.div>

        {/* Pulsing glow for high confidence */}
        {isHighConfidence && (
          <motion.div
            style={{
              position: 'absolute',
              top: '50%',
              left: `${normalizedConfidence}%`,
              width: '20px',
              height: '20px',
              borderRadius: '50%',
              background: '#0ECB81',
              transform: 'translate(-50%, -50%)',
              filter: 'blur(8px)',
              opacity: 0.6,
            }}
            animate={{
              scale: [1, 1.5, 1],
              opacity: [0.6, 0.9, 0.6],
            }}
            transition={{
              duration: 1.5,
              repeat: Infinity,
              ease: 'easeInOut',
            }}
          />
        )}
      </div>
    </div>
  )
}
