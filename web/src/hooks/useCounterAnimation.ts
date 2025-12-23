import { useEffect, useState } from 'react'

interface UseCounterAnimationOptions {
  start?: number
  end: number
  duration?: number
  decimals?: number
}

export function useCounterAnimation({
  start = 0,
  end,
  duration = 2000,
  decimals = 0,
}: UseCounterAnimationOptions): number {
  // Ensure end is a valid number
  const safeEnd = typeof end === 'number' && !isNaN(end) ? end : 0
  const safeStart = typeof start === 'number' && !isNaN(start) ? start : 0
  
  const [count, setCount] = useState(safeStart)

  useEffect(() => {
    // Don't animate if end is 0 or invalid
    if (safeEnd === 0 && safeStart === 0) {
      setCount(0)
      return
    }

    let startTime: number | null = null
    let animationFrame: number

    const animate = (currentTime: number) => {
      if (startTime === null) startTime = currentTime
      const progress = Math.min((currentTime - startTime) / duration, 1)

      // 使用 easeOutExpo 缓动函数，让数字快速启动后缓慢停止
      const easeOutExpo = progress === 1 ? 1 : 1 - Math.pow(2, -10 * progress)

      const currentCount = safeStart + (safeEnd - safeStart) * easeOutExpo
      setCount(currentCount)

      if (progress < 1) {
        animationFrame = requestAnimationFrame(animate)
      } else {
        setCount(safeEnd)
      }
    }

    animationFrame = requestAnimationFrame(animate)

    return () => {
      if (animationFrame) {
        cancelAnimationFrame(animationFrame)
      }
    }
  }, [safeStart, safeEnd, duration])

  // Ensure count is a valid number before calling toFixed
  const safeCount = typeof count === 'number' && !isNaN(count) ? count : 0
  return decimals > 0 ? parseFloat(safeCount.toFixed(decimals)) : Math.floor(safeCount)
}
