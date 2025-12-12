import { useState, useRef, useEffect } from 'react'
import { createPortal } from 'react-dom'
import { ChevronDown } from 'lucide-react'

interface CustomSelectProps {
  value: string
  onChange: (value: string) => void
  options: Array<{ value: string; label: string }>
  placeholder?: string
  className?: string
  style?: React.CSSProperties
}

export function CustomSelect({
  value,
  onChange,
  options,
  placeholder = 'Select...',
  className = '',
  style,
}: CustomSelectProps) {
  const [isOpen, setIsOpen] = useState(false)
  const selectRef = useRef<HTMLDivElement>(null)
  const dropdownRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        selectRef.current &&
        !selectRef.current.contains(event.target as Node) &&
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false)
      }
    }

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside)
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [isOpen])

  // Calculate dropdown position
  useEffect(() => {
    if (isOpen && selectRef.current && dropdownRef.current) {
      let rafId: number | null = null
      
      const updatePosition = () => {
        if (selectRef.current && dropdownRef.current) {
          const rect = selectRef.current.getBoundingClientRect()
          const top = rect.bottom + 4
          const left = rect.left
          const width = rect.width
          
          // #region agent log
          fetch('http://127.0.0.1:7242/ingest/39c2a80e-ec81-42f5-9ee5-0a97e070d0b3',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({location:'CustomSelect.tsx:updatePosition',message:'Dropdown position calculation',data:{rect:{top:rect.top,bottom:rect.bottom,left:rect.left,right:rect.right,width:rect.width,height:rect.height},calculated:{top,left,width},scrollY:window.scrollY,scrollX:window.scrollX,viewportHeight:window.innerHeight,viewportWidth:window.innerWidth},timestamp:Date.now(),sessionId:'debug-session',runId:'run1',hypothesisId:'A'})}).catch(()=>{});
          // #endregion
          
          dropdownRef.current.style.top = `${top}px`
          dropdownRef.current.style.left = `${left}px`
          dropdownRef.current.style.width = `${width}px`
          
          // #region agent log
          fetch('http://127.0.0.1:7242/ingest/39c2a80e-ec81-42f5-9ee5-0a97e070d0b3',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({location:'CustomSelect.tsx:updatePosition',message:'Dropdown styles applied',data:{computedTop:dropdownRef.current.style.top,computedLeft:dropdownRef.current.style.left,computedWidth:dropdownRef.current.style.width,zIndex:window.getComputedStyle(dropdownRef.current).zIndex,position:window.getComputedStyle(dropdownRef.current).position},timestamp:Date.now(),sessionId:'debug-session',runId:'run1',hypothesisId:'A'})}).catch(()=>{});
          // #endregion
        }
      }
      
      const throttledUpdate = () => {
        if (rafId !== null) return
        rafId = requestAnimationFrame(() => {
          updatePosition()
          rafId = null
        })
      }
      
      updatePosition()
      window.addEventListener('scroll', throttledUpdate, true)
      window.addEventListener('resize', throttledUpdate)
      
      return () => {
        if (rafId !== null) {
          cancelAnimationFrame(rafId)
        }
        window.removeEventListener('scroll', throttledUpdate, true)
        window.removeEventListener('resize', throttledUpdate)
      }
    }
  }, [isOpen])

  const selectedOption = options.find((opt) => opt.value === value)

  const dropdownContent = isOpen ? (
    <div
      ref={(el) => {
        dropdownRef.current = el
        if (el) {
          // #region agent log
          fetch('http://127.0.0.1:7242/ingest/39c2a80e-ec81-42f5-9ee5-0a97e070d0b3',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({location:'CustomSelect.tsx:dropdownContent',message:'Dropdown rendered',data:{isPortal:true,parentElement:el.parentElement?.tagName,zIndex:window.getComputedStyle(el).zIndex,position:window.getComputedStyle(el).position,top:el.style.top,left:el.style.left,width:el.style.width},timestamp:Date.now(),sessionId:'debug-session',runId:'run1',hypothesisId:'B'})}).catch(()=>{});
          // #endregion
        }
      }}
      className="fixed z-[9999] rounded-lg shadow-2xl overflow-hidden"
      style={{
        background: 'var(--navy-background)',
        border: '1px solid var(--panel-border)',
        maxHeight: '200px',
        overflowY: 'auto',
      }}
    >
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          onClick={() => {
            onChange(option.value)
            setIsOpen(false)
          }}
          className="w-full px-4 py-2.5 text-left text-[#EAECEF] hover:bg-[var(--navy-primary)] transition-colors"
          style={{
            background: value === option.value ? 'var(--navy-primary)' : 'transparent',
          }}
        >
          {option.label}
        </button>
      ))}
    </div>
  ) : null

  return (
    <>
      <div ref={selectRef} className={`relative ${className}`}>
        <button
          type="button"
          onClick={() => setIsOpen(!isOpen)}
          className="w-full px-4 py-2.5 rounded-lg text-[#EAECEF] focus:border-[var(--green-primary)] focus:outline-none transition-colors cursor-pointer flex items-center justify-between"
          style={{
            background: 'var(--navy-background)',
            border: '1px solid var(--panel-border)',
            ...style,
          }}
        >
          <span className={selectedOption ? '' : 'text-[#848E9C]'}>
            {selectedOption ? selectedOption.label : placeholder}
          </span>
          <ChevronDown
            className={`w-4 h-4 transition-transform ${isOpen ? 'rotate-180' : ''}`}
            style={{ color: '#848E9C' }}
          />
        </button>
      </div>
      {dropdownContent && createPortal(dropdownContent, document.body)}
    </>
  )
}

