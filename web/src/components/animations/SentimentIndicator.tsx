import { motion } from 'framer-motion'
import { TrendingUp, TrendingDown, Minus } from 'lucide-react'

interface SentimentIndicatorProps {
  sentiment: 'positive' | 'negative' | 'neutral'
  intensity?: number // 0-1, affects animation intensity
  className?: string
  size?: number
}

/**
 * Animated sentiment icons (positive/negative/neutral).
 * Pulse and scale animations.
 * Color transitions based on sentiment.
 */
export function SentimentIndicator({
  sentiment,
  intensity = 0.5,
  className = '',
  size = 20,
}: SentimentIndicatorProps) {
  const getIcon = () => {
    switch (sentiment) {
      case 'positive':
        return <TrendingUp size={size} />
      case 'negative':
        return <TrendingDown size={size} />
      default:
        return <Minus size={size} />
    }
  }

  const getColor = () => {
    switch (sentiment) {
      case 'positive':
        return '#0ECB81'
      case 'negative':
        return '#F6465D'
      default:
        return '#848E9C'
    }
  }

  const animationIntensity = Math.max(0.3, Math.min(1, intensity))

  return (
    <motion.div
      className={className}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        color: getColor(),
      }}
      animate={{
        scale: [1, 1 + 0.2 * animationIntensity, 1],
        rotate: sentiment === 'positive' ? [0, 5, -5, 0] : sentiment === 'negative' ? [0, -5, 5, 0] : 0,
      }}
      transition={{
        duration: 1.5,
        repeat: Infinity,
        ease: 'easeInOut',
      }}
    >
      <motion.div
        animate={{
          opacity: [0.7, 1, 0.7],
        }}
        transition={{
          duration: 2,
          repeat: Infinity,
          ease: 'easeInOut',
        }}
      >
        {getIcon()}
      </motion.div>
    </motion.div>
  )
}
