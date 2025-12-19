import { useState, useRef, useEffect } from 'react'
import { createPortal } from 'react-dom'

interface TooltipProps {
  content: string
  children: React.ReactNode
}

export function Tooltip({ content, children }: TooltipProps) {
  const [show, setShow] = useState(false)
  const [position, setPosition] = useState({ top: 0, left: 0 })
  const triggerRef = useRef<HTMLDivElement>(null)
  const tooltipRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (show && triggerRef.current) {
      const updatePosition = () => {
        if (triggerRef.current) {
          const rect = triggerRef.current.getBoundingClientRect()
          const tooltipWidth = 256 // w-64 = 16rem = 256px
          const viewportWidth = window.innerWidth

          // Calculate center position
          let left = rect.left + rect.width / 2

          // Adjust if tooltip would go off-screen horizontally
          if (left - tooltipWidth / 2 < 8) {
            left = tooltipWidth / 2 + 8
          } else if (left + tooltipWidth / 2 > viewportWidth - 8) {
            left = viewportWidth - tooltipWidth / 2 - 8
          }

          // Position above the icon with margin
          const top = rect.top - 8

          setPosition({ top, left })
        }
      }

      updatePosition()
      
      // Update position on scroll or resize
      const handleScroll = () => updatePosition()
      const handleResize = () => updatePosition()

      window.addEventListener('scroll', handleScroll, true)
      window.addEventListener('resize', handleResize)

      return () => {
        window.removeEventListener('scroll', handleScroll, true)
        window.removeEventListener('resize', handleResize)
      }
    }
  }, [show])

  return (
    <>
      <div className="relative inline-block" ref={triggerRef}>
        <div
          onMouseEnter={() => setShow(true)}
          onMouseLeave={() => setShow(false)}
          onClick={() => setShow(!show)}
        >
          {children}
        </div>
      </div>
      {show &&
        createPortal(
          <div
            ref={tooltipRef}
            className="fixed z-[9999] px-3 py-2 text-sm rounded-lg shadow-lg w-64 pointer-events-none"
            style={{
              top: `${position.top}px`,
              left: `${position.left}px`,
              transform: 'translate(-50%, -100%)',
              background: 'var(--panel-border)',
              color: '#EAECEF',
              border: '1px solid #474D57',
              marginTop: '-8px', // Space between icon and tooltip
            }}
          >
            {content}
            <div
              className="absolute left-1/2 transform -translate-x-1/2 top-full"
              style={{
                width: 0,
                height: 0,
                borderLeft: '6px solid transparent',
                borderRight: '6px solid transparent',
                borderTop: '6px solid var(--panel-border)',
              }}
            />
          </div>,
          document.body
        )}
    </>
  )
}
