import { useEffect, useState, useMemo, useRef } from 'react'
import { motion } from 'framer-motion'
import { soundSystem } from '../../lib/sound'

interface AnimatedAIMessageProps {
  message: string
  typingSpeed?: number // ms per character
  onComplete?: () => void
  onScrollComplete?: () => void // Callback when scroll reaches bottom
  className?: string
  alwaysAnimate?: boolean // Always keep animation running
  autoScroll?: boolean // Auto-scroll as text types
  scrollContainerRef?: React.RefObject<HTMLDivElement> // Direct ref to scrollable container
}

/**
 * Enhanced AI message with smooth character-by-character typing effects.
 * Features:
 * - Smooth character-by-character typing animation
 * - Stable container positioning (no layout shifts)
 * - Proper timer cleanup to prevent memory leaks
 * - Reserved space to prevent height collapse
 * - Subtle pulse effect when complete (instead of restarting)
 */
export function AnimatedAIMessage({
  message,
  typingSpeed = 30,
  onComplete,
  onScrollComplete,
  className = '',
  alwaysAnimate = false,
  autoScroll = false,
  scrollContainerRef,
}: AnimatedAIMessageProps) {
  const [displayedLength, setDisplayedLength] = useState(0)
  const [isComplete, setIsComplete] = useState(false)
  const [isPulsing, setIsPulsing] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const timeoutRefs = useRef<ReturnType<typeof setTimeout>[]>([])
  const rafRef = useRef<number | null>(null)
  const scrollRafRef = useRef<number | null>(null)
  const scrollCompleteCalledRef = useRef(false)
  const alwaysAnimateRef = useRef(alwaysAnimate)
  const isAnimatingRef = useRef(false)
  const messageRef = useRef(message)
  const restartTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const intersectionObserverRef = useRef<IntersectionObserver | null>(null)
  // Refs for callbacks to prevent unnecessary re-renders
  const onCompleteRef = useRef(onComplete)
  const onScrollCompleteRef = useRef(onScrollComplete)
  const autoScrollRef = useRef(autoScroll)
  const scrollContainerRefRef = useRef(scrollContainerRef)

  // Calculate minimum height based on full message (to reserve space)
  const minHeight = useMemo(() => {
    if (!message) return 'auto'
    // Estimate: ~20px per line, ~80 chars per line
    const estimatedLines = Math.ceil(message.length / 80)
    // Clamp to avoid forcing parent to grow beyond its max-height
    const clamped = Math.min(Math.max(estimatedLines * 20, 80), 160)
    return `${clamped}px`
  }, [message])

  // Cleanup all timers and animation frames
  const cleanup = () => {
    timeoutRefs.current.forEach(timeout => clearTimeout(timeout))
    timeoutRefs.current = []
    if (restartTimeoutRef.current) {
      clearTimeout(restartTimeoutRef.current)
      restartTimeoutRef.current = null
    }
    if (rafRef.current !== null) {
      cancelAnimationFrame(rafRef.current)
      rafRef.current = null
    }
    if (scrollRafRef.current !== null) {
      cancelAnimationFrame(scrollRafRef.current)
      scrollRafRef.current = null
    }
    if (intersectionObserverRef.current) {
      intersectionObserverRef.current.disconnect()
      intersectionObserverRef.current = null
    }
    isAnimatingRef.current = false
  }

  // Direct ref-based auto-scroll using requestAnimationFrame
  const performAutoScroll = () => {
    if (!autoScrollRef.current || !scrollContainerRefRef.current?.current || scrollCompleteCalledRef.current) return

    const scrollContainer = scrollContainerRefRef.current.current
    const scrollHeight = scrollContainer.scrollHeight
    const clientHeight = scrollContainer.clientHeight
    const maxScrollTop = scrollHeight - clientHeight

    // If content fits without scrolling, mark as complete
    if (maxScrollTop <= 0) {
      if (!scrollCompleteCalledRef.current) {
        scrollCompleteCalledRef.current = true
        onScrollComplete?.()
      }
      return
    }

    const currentScrollTop = scrollContainer.scrollTop
    const isAtBottom = currentScrollTop >= maxScrollTop - 5

    // If already at bottom, mark as complete
    if (isAtBottom) {
      if (!scrollCompleteCalledRef.current) {
        scrollCompleteCalledRef.current = true
        onScrollComplete?.()
      }
      return
    }

    // Cancel any existing scroll animation
    if (scrollRafRef.current !== null) {
      cancelAnimationFrame(scrollRafRef.current)
    }

    // Smooth scroll using requestAnimationFrame
    const startScrollTop = currentScrollTop
    const targetScrollTop = maxScrollTop
    const distance = targetScrollTop - startScrollTop
    const startTime = performance.now()
    const duration = Math.min(500, Math.abs(distance) * 2) // Adaptive duration based on distance

    const animateScroll = (currentTime: number) => {
      const elapsed = currentTime - startTime
      const progress = Math.min(1, elapsed / duration)
      
      // Easing function (ease-out cubic)
      const eased = 1 - Math.pow(1 - progress, 3)
      const newScrollTop = startScrollTop + distance * eased

      if (scrollContainer) {
        scrollContainer.scrollTop = newScrollTop

        // Check if we've reached the bottom
        const currentMaxScrollTop = scrollContainer.scrollHeight - scrollContainer.clientHeight
        if (scrollContainer.scrollTop >= currentMaxScrollTop - 5) {
          scrollContainer.scrollTop = currentMaxScrollTop // Ensure we're exactly at bottom
          if (!scrollCompleteCalledRef.current) {
            scrollCompleteCalledRef.current = true
            onScrollCompleteRef.current?.()
          }
          scrollRafRef.current = null
          return
        }

        // Continue animation if not complete
        if (progress < 1) {
          scrollRafRef.current = requestAnimationFrame(animateScroll)
        } else {
        // Animation complete, check if we're at bottom
        if (scrollContainer.scrollTop >= currentMaxScrollTop - 5) {
          if (!scrollCompleteCalledRef.current) {
            scrollCompleteCalledRef.current = true
            onScrollCompleteRef.current?.()
          }
        }
          scrollRafRef.current = null
        }
      } else {
        scrollRafRef.current = null
      }
    }

    scrollRafRef.current = requestAnimationFrame(animateScroll)
  }

  // Set up IntersectionObserver to detect when bottom is visible (for scroll completion)
  useEffect(() => {
    if (!autoScrollRef.current || !scrollContainerRefRef.current?.current || !containerRef.current) return

    const scrollContainer = scrollContainerRefRef.current.current
    const container = containerRef.current

    // Create a sentinel element at the bottom of the content to observe
    const sentinel = document.createElement('div')
    sentinel.style.height = '1px'
    sentinel.style.position = 'absolute'
    sentinel.style.bottom = '0'
    sentinel.style.width = '100%'
    sentinel.style.pointerEvents = 'none'
    
    // Insert sentinel at the end of container
    container.appendChild(sentinel)

    // Set up IntersectionObserver
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting && !scrollCompleteCalledRef.current) {
            // Bottom is visible, scroll is complete
            scrollCompleteCalledRef.current = true
            onScrollCompleteRef.current?.()
          }
        })
      },
      {
        root: scrollContainer,
        rootMargin: '0px',
        threshold: 0.1,
      }
    )

    observer.observe(sentinel)
    intersectionObserverRef.current = observer

    return () => {
      observer.disconnect()
      if (sentinel.parentNode) {
        sentinel.parentNode.removeChild(sentinel)
      }
    }
  }, [message]) // Only depend on message, use refs for callbacks

  // Update refs when props change
  useEffect(() => {
    alwaysAnimateRef.current = alwaysAnimate
    messageRef.current = message
    
    // If alwaysAnimate becomes false, cancel any pending restart
    if (!alwaysAnimate && restartTimeoutRef.current) {
      clearTimeout(restartTimeoutRef.current)
      restartTimeoutRef.current = null
    }
  }, [alwaysAnimate, message])

  useEffect(() => {
    // Only restart if message content actually changed
    // Store previous message for comparison
    const prevMessage = messageRef.current
    
    // Check if message actually changed (reference or content length)
    const messageChanged =
      prevMessage !== message &&
      (prevMessage === null ||
        message === null ||
        !prevMessage ||
        !message ||
        prevMessage.length !== message.length ||
        prevMessage !== message)
    
    // If message hasn't changed and we're already animating, don't restart
    if (!messageChanged && isAnimatingRef.current) {
      // Update ref to new reference but don't restart animation
      if (prevMessage !== message) {
        messageRef.current = message
      }
      return
    }

    // Cancel any pending restart when message changes
    if (restartTimeoutRef.current) {
      clearTimeout(restartTimeoutRef.current)
      restartTimeoutRef.current = null
    }

    cleanup()
    setDisplayedLength(0)
    setIsComplete(false)
    setIsPulsing(false)
    scrollCompleteCalledRef.current = false
    isAnimatingRef.current = true
    messageRef.current = message

    if (!message) {
      setIsComplete(true)
      isAnimatingRef.current = false
      onComplete?.()
      return
    }

    let currentLength = 0
    let lastFrameTime = 0
    const targetFPS = 60
    const frameInterval = 1000 / targetFPS

    const animate = (currentTime: number) => {
      if (currentLength >= message.length) {
        setIsComplete(true)
        isAnimatingRef.current = false
        // Call onComplete on every cycle completion (not just first)
        onCompleteRef.current?.()
        soundSystem.playAnimationComplete()

        // Ensure scroll completion is called if it hasn't been called yet
        // This handles cases where content doesn't need scrolling or scroll completed during typing
        if (autoScrollRef.current && !scrollCompleteCalledRef.current) {
          // Perform one final scroll check
          setTimeout(() => {
            performAutoScroll()
            // If still not called, call it anyway (content might fit without scrolling)
            if (!scrollCompleteCalledRef.current) {
              scrollCompleteCalledRef.current = true
              onScrollCompleteRef.current?.()
            }
          }, 500) // Give scroll time to complete
        }

        // Only restart if alwaysAnimate is still true (check ref to avoid stale closure)
        // Don't restart if alwaysAnimate was turned off during animation
        if (alwaysAnimateRef.current) {
          setIsPulsing(true)
          const pulseTimeout = setTimeout(() => {
            setIsPulsing(false)
            // Only restart if alwaysAnimate is still true and message hasn't changed
            if (alwaysAnimateRef.current && messageRef.current === message) {
              // Restart animation after a pause
              restartTimeoutRef.current = setTimeout(() => {
                // Triple-check: alwaysAnimate still true, message unchanged
                if (alwaysAnimateRef.current && messageRef.current === message) {
                  setDisplayedLength(0)
                  setIsComplete(false)
                  setIsPulsing(false)
                  scrollCompleteCalledRef.current = false
                  isAnimatingRef.current = true
                  currentLength = 0
                  lastFrameTime = performance.now()
                  rafRef.current = requestAnimationFrame(animate)
                }
                restartTimeoutRef.current = null
              }, 2000) // 2 second pause before restart
              timeoutRefs.current.push(restartTimeoutRef.current!)
            }
          }, 1500) // Show pulse for 1.5 seconds
          timeoutRefs.current.push(pulseTimeout)
        }
        return
      }

      if (currentTime - lastFrameTime >= frameInterval) {
        // Type multiple characters per frame for smoother animation
        const charsPerFrame = Math.max(1, Math.floor(typingSpeed / frameInterval))
        currentLength = Math.min(currentLength + charsPerFrame, message.length)
        setDisplayedLength(currentLength)
        lastFrameTime = currentTime

        // Auto-scroll as text types - trigger scroll more frequently for smooth experience
        if (autoScrollRef.current && scrollContainerRefRef.current?.current) {
          // Trigger scroll immediately, but throttle with requestAnimationFrame
          if (scrollRafRef.current === null) {
            scrollRafRef.current = requestAnimationFrame(() => {
              performAutoScroll()
              scrollRafRef.current = null
            })
          }
        }
      }

      rafRef.current = requestAnimationFrame(animate)
    }

    // Start animation
    lastFrameTime = performance.now()
    rafRef.current = requestAnimationFrame(animate)

    return () => {
      cleanup()
      isAnimatingRef.current = false
    }
  }, [message, typingSpeed]) // Only depend on message and typingSpeed, use refs for callbacks

  // Get displayed text
  const displayedText = useMemo(() => {
    return message.slice(0, displayedLength)
  }, [message, displayedLength])

  return (
    <div
      ref={containerRef}
      className={className}
      style={{
        position: 'relative',
        minHeight,
        maxHeight: '240px', // keep within parent so overflow can occur
        overflowY: 'auto', // Allow vertical scroll inside the message
        overflowX: 'visible',
        willChange: 'contents',
        // Remove contain to allow scroll events to bubble to parent/page
      }}
    >
      <motion.div
        style={{
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
          lineHeight: '1.6',
          color: '#EAECEF',
          fontSize: '14px',
        }}
        animate={
          isPulsing
            ? {
                opacity: [1, 0.85, 1],
              }
            : { opacity: 1 }
        }
        transition={{
          duration: 1.5,
          repeat: isPulsing ? Infinity : 0,
          ease: 'easeInOut',
        }}
      >
        <span
          style={{
            display: 'inline-block',
            transform: 'translateZ(0)', // Force GPU acceleration
          }}
        >
          {displayedText}
        </span>

        {/* Blinking cursor */}
        {!isComplete && (
          <motion.span
            style={{
              display: 'inline-block',
              width: '2px',
              height: '1em',
              background: '#60a5fa',
              marginLeft: '2px',
              verticalAlign: 'middle',
              transform: 'translateZ(0)',
            }}
            animate={{
              opacity: [1, 0, 1],
            }}
            transition={{
              duration: 0.8,
              repeat: Infinity,
              ease: 'easeInOut',
            }}
          />
        )}
      </motion.div>
    </div>
  )
}
