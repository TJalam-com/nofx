import { motion } from 'framer-motion'
import { useEffect, useState } from 'react'

interface NeuronPulseProps {
  isActive: boolean
  delay?: number
  color?: string
  size?: number
  className?: string
}

/**
 * Individual neuron node with firing animation.
 * Shows color pulse effect with radial glow expansion.
 */
export function NeuronPulse({
  isActive,
  delay = 0,
  color = '#60a5fa',
  size = 12,
  className = '',
}: NeuronPulseProps) {
  const [shouldAnimate, setShouldAnimate] = useState(false)

  useEffect(() => {
    if (isActive) {
      const timer = setTimeout(() => {
        setShouldAnimate(true)
      }, delay)
      return () => clearTimeout(timer)
    } else {
      setShouldAnimate(false)
    }
  }, [isActive, delay])

  return (
    <motion.div
      className={className}
      style={{
        width: size,
        height: size,
        borderRadius: '50%',
        background: color,
        position: 'relative',
      }}
      animate={
        shouldAnimate
          ? {
              scale: [1, 1.3, 1],
              opacity: [0.6, 1, 0.6],
              boxShadow: [
                `0 0 0px ${color}`,
                `0 0 ${size * 2}px ${color}`,
                `0 0 0px ${color}`,
              ],
            }
          : {
              scale: 1,
              opacity: 0.3,
              boxShadow: `0 0 0px ${color}`,
            }
      }
      transition={{
        duration: 2,
        repeat: shouldAnimate ? Infinity : 0,
        ease: 'easeInOut',
      }}
    >
      {/* Radial glow effect */}
      {shouldAnimate && (
        <motion.div
          style={{
            position: 'absolute',
            top: '50%',
            left: '50%',
            width: size,
            height: size,
            borderRadius: '50%',
            background: color,
            transform: 'translate(-50%, -50%)',
          }}
          animate={{
            scale: [1, 2.5, 1],
            opacity: [0.8, 0, 0.8],
          }}
          transition={{
            duration: 2,
            repeat: Infinity,
            ease: 'easeOut',
          }}
        />
      )}
    </motion.div>
  )
}
