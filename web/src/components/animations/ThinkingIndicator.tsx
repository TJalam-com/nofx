import { motion } from 'framer-motion'

interface ThinkingIndicatorProps {
  isThinking: boolean
  text?: string
  className?: string
}

/**
 * "Thinking..." text with animated dots.
 * Dots morph between shapes (circle → ellipse → circle).
 * Each dot animates with slight delay for wave effect.
 */
export function ThinkingIndicator({
  isThinking,
  text = 'Thinking',
  className = '',
}: ThinkingIndicatorProps) {
  if (!isThinking) return null

  return (
    <div
      className={className}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        color: '#60a5fa',
        fontSize: '14px',
        fontWeight: '500',
      }}
    >
      <span>{text}</span>
      <div style={{ display: 'flex', gap: '4px', alignItems: 'center' }}>
        {[0, 1, 2].map((index) => (
          <motion.div
            key={index}
            style={{
              width: '6px',
              height: '6px',
              borderRadius: '50%',
              background: '#60a5fa',
            }}
            animate={{
              scaleY: [1, 1.5, 1],
              scaleX: [1, 0.8, 1],
              opacity: [0.5, 1, 0.5],
            }}
            transition={{
              duration: 1.5,
              repeat: Infinity,
              delay: index * 0.2,
              ease: 'easeInOut',
            }}
          />
        ))}
      </div>
    </div>
  )
}
