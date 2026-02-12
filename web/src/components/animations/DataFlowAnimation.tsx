import { motion } from 'framer-motion'
import { useState, useEffect } from 'react'

interface DataFlowAnimationProps {
  currentStage?: string
  stages?: string[]
  isActive?: boolean
  className?: string
}

/**
 * Visual flow: Price → Analysis → Decision
 * Animated arrows/path showing data movement.
 * Each stage highlights as data "flows" through.
 */
export function DataFlowAnimation({
  currentStage,
  stages = ['Price', 'Analysis', 'Decision'],
  isActive = true,
  className = '',
}: DataFlowAnimationProps) {
  const [activeIndex, setActiveIndex] = useState(0)

  useEffect(() => {
    if (!isActive) return

    const interval = setInterval(() => {
      setActiveIndex((prev) => (prev + 1) % stages.length)
    }, 1000)

    return () => clearInterval(interval)
  }, [isActive, stages.length])

  const getStageIndex = (stage: string): number => {
    return stages.indexOf(stage)
  }

  const currentIndex =
    currentStage && stages.includes(currentStage)
      ? getStageIndex(currentStage)
      : activeIndex

  return (
    <div
      className={className}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '16px',
        padding: '16px',
        background: 'var(--navy-dark)',
        borderRadius: '8px',
        border: '1px solid var(--panel-border)',
        minHeight: '80px',
      }}
    >
      {stages.map((stage, index) => {
        const isActive = index === currentIndex
        const isPast = index < currentIndex

        return (
          <div
            key={stage}
            style={{
              display: 'flex',
              alignItems: 'center',
              flex: 1,
            }}
          >
            {/* Stage Box */}
            <motion.div
              style={{
                padding: '12px 20px',
                borderRadius: '6px',
                background: isActive
                  ? 'rgba(96, 165, 250, 0.2)'
                  : isPast
                    ? 'rgba(14, 203, 129, 0.1)'
                    : 'rgba(255, 255, 255, 0.05)',
                border: `2px solid ${
                  isActive
                    ? '#60a5fa'
                    : isPast
                      ? '#0ECB81'
                      : 'rgba(255, 255, 255, 0.1)'
                }`,
                color: isActive ? '#60a5fa' : isPast ? '#0ECB81' : '#848E9C',
                fontWeight: isActive ? 'bold' : 'normal',
                fontSize: '13px',
                textAlign: 'center',
                minWidth: '100px',
                position: 'relative',
                overflow: 'hidden',
              }}
              animate={
                isActive
                  ? {
                      scale: [1, 1.05, 1],
                      boxShadow: [
                        '0 0 0px rgba(96, 165, 250, 0)',
                        '0 0 20px rgba(96, 165, 250, 0.5)',
                        '0 0 0px rgba(96, 165, 250, 0)',
                      ],
                    }
                  : {}
              }
              transition={{
                duration: 1.5,
                repeat: isActive ? Infinity : 0,
                ease: 'easeInOut',
              }}
            >
              {stage}
              {/* Shimmer effect when active */}
              {isActive && (
                <motion.div
                  style={{
                    position: 'absolute',
                    top: 0,
                    left: '-100%',
                    width: '100%',
                    height: '100%',
                    background:
                      'linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.3), transparent)',
                  }}
                  animate={{
                    left: ['-100%', '200%'],
                  }}
                  transition={{
                    duration: 2,
                    repeat: Infinity,
                    ease: 'linear',
                  }}
                />
              )}
            </motion.div>

            {/* Arrow */}
            {index < stages.length - 1 && (
              <motion.div
                style={{
                  margin: '0 8px',
                  display: 'flex',
                  alignItems: 'center',
                }}
                animate={
                  isActive && index === currentIndex
                    ? {
                        x: [0, 4, 0],
                      }
                    : {}
                }
                transition={{
                  duration: 0.8,
                  repeat: isActive && index === currentIndex ? Infinity : 0,
                  ease: 'easeInOut',
                }}
              >
                <svg
                  width="24"
                  height="24"
                  viewBox="0 0 24 24"
                  fill="none"
                  style={{
                    color: isPast
                      ? '#0ECB81'
                      : isActive
                        ? '#60a5fa'
                        : '#848E9C',
                  }}
                >
                  <motion.path
                    d="M9 18L15 12L9 6"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    animate={
                      isActive && index === currentIndex
                        ? {
                            pathLength: [0, 1, 0],
                          }
                        : {
                            pathLength: isPast ? 1 : 0.3,
                          }
                    }
                    transition={{
                      duration: 1.5,
                      repeat: isActive && index === currentIndex ? Infinity : 0,
                      ease: 'easeInOut',
                    }}
                  />
                </svg>
              </motion.div>
            )}
          </div>
        )
      })}
    </div>
  )
}
