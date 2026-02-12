import { useEffect, useState, forwardRef, useMemo, useRef } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useAuth } from '../../contexts/AuthContext'
import { api } from '../../lib/api'
import { DecisionRecord } from '../../types'
import { AnimatedAIMessage } from '../animations/AnimatedAIMessage'
import { ConfidenceScore } from '../animations/ConfidenceScore'
import { DataFlowAnimation } from '../animations/DataFlowAnimation'
import { ThinkingIndicator } from '../animations/ThinkingIndicator'
import { soundSystem } from '../../lib/sound'
import { CheckCircle, XCircle, Brain } from 'lucide-react'

interface AnimatedDecisionTimelineProps {
  traderId?: string
  limit?: number
  animationSpeed?: number
  maxHeight?: string
  autoScrollAiAnalysis?: boolean
  onMostRecentAnalysisComplete?: () => void // Callback when most recent AI analysis typing completes
  onMostRecentScrollComplete?: () => void // Callback when most recent AI analysis scroll completes
}

/**
 * Animated Decision Timeline with real-time updates
 * Features:
 * - Cards slide in from right
 * - Timeline connecting vertically
 * - Status badges with scale animation
 * - Expandable reasoning with typing effects
 * - Confidence score filling animation
 */
export function AnimatedDecisionTimeline({
  traderId,
  limit = 10,
  animationSpeed = 1,
  maxHeight = '600px',
  autoScrollAiAnalysis = false,
  onMostRecentAnalysisComplete,
  onMostRecentScrollComplete,
}: AnimatedDecisionTimelineProps) {
  const { user, token } = useAuth()
  const [decisions, setDecisions] = useState<DecisionRecord[]>([])
  const [expandedDecisions, setExpandedDecisions] = useState<Set<string>>(
    new Set()
  )
  const [isLoading, setIsLoading] = useState(true)
  const [processingDecision, setProcessingDecision] = useState<string | null>(
    null
  )

  // Sort decisions by timestamp descending (most recent first)
  const sortedDecisions = useMemo(() => {
    // Ensure decisions is always an array before spreading
    const safeDecisions = Array.isArray(decisions) ? decisions : []
    return [...safeDecisions].sort((a, b) => {
      const timeA = new Date(a.timestamp).getTime()
      const timeB = new Date(b.timestamp).getTime()
      return timeB - timeA // Descending order (most recent first)
    })
  }, [decisions])

  // Find the most recent decision by timestamp
  const mostRecentTimestamp = useMemo(() => {
    if (sortedDecisions.length === 0) return null
    return sortedDecisions[0].timestamp
  }, [sortedDecisions])

  // Auto-expand the most recent decision (by timestamp)
  useEffect(() => {
    if (sortedDecisions.length > 0 && mostRecentTimestamp) {
      const mostRecentDecision = sortedDecisions.find(
        (d) => d.timestamp === mostRecentTimestamp
      )
      if (mostRecentDecision) {
        const mostRecentCycle = mostRecentDecision.cycle_number.toString()
        setExpandedDecisions((prev) => {
          const next = new Set(prev)
          next.add(mostRecentCycle)
          return next
        })
      }
    }
  }, [sortedDecisions, mostRecentTimestamp])

  // Load decisions
  useEffect(() => {
    if (!user || !token || !traderId) return

    const loadDecisions = async () => {
      try {
        const data = await api.getLatestDecisions(traderId, limit)
        // Ensure we always set an array, never null or undefined
        setDecisions(Array.isArray(data) ? data : [])
      } catch (error) {
        console.error('Failed to load decisions:', error)
      } finally {
        setIsLoading(false)
      }
    }

    loadDecisions()

    // Poll for decision updates (WebSocket not implemented yet)
    const pollDecisions = async () => {
      try {
        const updatedDecisions = await api.getLatestDecisions(traderId, limit)

        // Ensure we have valid arrays before processing
        const safeUpdatedDecisions = Array.isArray(updatedDecisions)
          ? updatedDecisions
          : []
        const safeDecisions = Array.isArray(decisions) ? decisions : []

        // Check for new decisions
        if (safeUpdatedDecisions.length > safeDecisions.length) {
          const newDecisions = safeUpdatedDecisions.slice(
            0,
            safeUpdatedDecisions.length - safeDecisions.length
          )
          setDecisions(safeUpdatedDecisions)

          // Show processing animation for new decisions
          if (newDecisions.length > 0) {
            setProcessingDecision(newDecisions[0].cycle_number.toString())
            soundSystem.playNotification()
            setTimeout(() => setProcessingDecision(null), 3000)
          }
        }
      } catch (error) {
        console.error('Failed to poll decisions:', error)
      }
    }

    // Poll every 10 seconds (decisions update less frequently)
    const interval = setInterval(pollDecisions, 10000)

    return () => clearInterval(interval)
  }, [user, token, traderId, limit])

  const toggleExpanded = (decisionId: string) => {
    setExpandedDecisions((prev) => {
      const next = new Set(prev)
      if (next.has(decisionId)) {
        next.delete(decisionId)
      } else {
        next.add(decisionId)
      }
      return next
    })
  }

  if (isLoading) {
    return (
      <div className="binance-card p-6">
        <div className="skeleton h-8 w-48 mb-4"></div>
        <div className="space-y-4">
          {[...Array(3)].map((_, i) => (
            <div key={i} className="skeleton h-24 w-full rounded-lg"></div>
          ))}
        </div>
      </div>
    )
  }

  return (
    <motion.div
      className="binance-card p-6 h-full flex flex-col"
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.5 }}
    >
      <div className="flex items-center justify-between mb-6 flex-shrink-0">
        <h3
          className="text-xl font-bold flex items-center gap-2"
          style={{ color: '#EAECEF' }}
        >
          <Brain
            className="w-5 h-5"
            style={{ color: 'var(--green-primary)' }}
          />
          AI Decisions
        </h3>
        <div className="text-sm" style={{ color: '#848E9C' }}>
          {Array.isArray(decisions) ? decisions.length : 0} recent cycles
        </div>
      </div>

      <div
        className="relative flex-1 overflow-y-auto"
        style={{
          maxHeight,
          minHeight: '300px', // Ensure minimum height for content
        }}
      >
        {/* Timeline line */}
        <div
          className="absolute left-6 top-0 bottom-0 w-0.5"
          style={{ background: 'var(--navy-light)' }}
        />

        <AnimatePresence mode="popLayout">
          {!Array.isArray(decisions) || decisions.length === 0 ? (
            <motion.div
              className="text-center py-16"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
            >
              <div className="opacity-30 flex justify-center mb-4">
                <Brain className="w-16 h-16" />
              </div>
              <div style={{ color: '#848E9C' }}>No decisions yet</div>
            </motion.div>
          ) : (
            sortedDecisions.map((decision, index) => {
              // Identify most recent by comparing timestamp, not array index
              const isMostRecent =
                mostRecentTimestamp !== null &&
                decision.timestamp === mostRecentTimestamp
              const cycleId = decision.cycle_number.toString()

              return (
                <AnimatedDecisionCard
                  key={`${decision.cycle_number}-${decision.timestamp}`}
                  decision={decision}
                  index={index}
                  isExpanded={isMostRecent || expandedDecisions.has(cycleId)}
                  isProcessing={processingDecision === cycleId}
                  animationSpeed={animationSpeed}
                  onToggle={() => toggleExpanded(cycleId)}
                  alwaysShowAnimation={isMostRecent}
                  autoScrollAiAnalysis={isMostRecent && autoScrollAiAnalysis}
                  onAnalysisComplete={
                    isMostRecent ? onMostRecentAnalysisComplete : undefined
                  }
                  onScrollComplete={
                    isMostRecent ? onMostRecentScrollComplete : undefined
                  }
                />
              )
            })
          )}
        </AnimatePresence>
      </div>
    </motion.div>
  )
}

interface AnimatedDecisionCardProps {
  decision: DecisionRecord
  index: number
  isExpanded: boolean
  isProcessing: boolean
  animationSpeed: number
  onToggle: () => void
  alwaysShowAnimation?: boolean // Always show typing animation for most recent
  autoScrollAiAnalysis?: boolean // Enable auto-scroll for AI analysis
  onAnalysisComplete?: () => void // Callback when AI analysis typing completes
  onScrollComplete?: () => void // Callback when AI analysis scroll completes
}

const AnimatedDecisionCard = forwardRef<
  HTMLDivElement,
  AnimatedDecisionCardProps
>(function AnimatedDecisionCard(
  {
    decision,
    index,
    isExpanded,
    isProcessing,
    animationSpeed,
    onToggle,
    alwaysShowAnimation = false,
    autoScrollAiAnalysis = false,
    onAnalysisComplete,
    onScrollComplete,
  },
  ref
) {
  const scrollContainerRef = useRef<HTMLDivElement>(null)
  const isSuccess = decision.success
  const hasConfidence = decision.decisions?.some(
    (d) => d.confidence !== undefined
  )
  const avgConfidence = hasConfidence
    ? decision.decisions?.reduce((sum, d) => sum + (d.confidence || 0), 0) /
      (decision.decisions?.length || 1)
    : undefined

  return (
    <motion.div
      ref={ref}
      className="relative mb-6"
      initial={{ opacity: 0, x: 50 }}
      animate={{ opacity: 1, x: 0 }}
      exit={{ opacity: 0, x: -50 }}
      transition={{
        duration: 0.4,
        delay: index * 0.1,
      }}
    >
      {/* Timeline dot */}
      <motion.div
        className="absolute left-4 w-4 h-4 rounded-full border-2 border-white"
        style={{
          background: isSuccess ? '#0ECB81' : '#F6465D',
          top: '24px',
          zIndex: 10,
        }}
        animate={isProcessing ? { scale: [1, 1.3, 1] } : {}}
        transition={{ duration: 1, repeat: isProcessing ? Infinity : 0 }}
      />

      {/* Card */}
      <motion.div
        className="ml-12 rounded-lg p-5 mb-4 cursor-pointer flex flex-col"
        style={{
          background: 'var(--navy-dark)',
          border: '1px solid var(--navy-light)',
          boxShadow: '0 2px 4px rgba(0, 0, 0, 0.1)',
          minHeight: '200px', // Increased to accommodate always-visible section
          willChange: 'transform', // Optimize for hover animations
          // Remove contain to allow scroll events to bubble to page
        }}
        whileHover={{ scale: 1.01, boxShadow: '0 4px 8px rgba(0, 0, 0, 0.15)' }}
        onClick={onToggle}
        animate={
          isProcessing
            ? {
                boxShadow: [
                  '0 0 0px rgba(96, 165, 250, 0)',
                  '0 0 20px rgba(96, 165, 250, 0.5)',
                  '0 0 0px rgba(96, 165, 250, 0)',
                ],
              }
            : {}
        }
        transition={{ duration: 2, repeat: isProcessing ? Infinity : 0 }}
      >
        {/* Header */}
        <div className="flex items-start justify-between mb-3">
          <div>
            <div className="font-semibold" style={{ color: '#EAECEF' }}>
              Cycle #{decision.cycle_number}
            </div>
            <div className="text-xs" style={{ color: '#848E9C' }}>
              {new Date(decision.timestamp).toLocaleString()}
            </div>
          </div>

          <motion.div
            className="flex items-center gap-2"
            animate={{ scale: isProcessing ? [1, 1.1, 1] : [1] }}
            transition={{ duration: 0.5 }}
          >
            {/* Status Badge */}
            <motion.div
              className="px-3 py-1 rounded text-xs font-bold"
              style={{
                background: isSuccess
                  ? 'rgba(14, 203, 129, 0.1)'
                  : 'rgba(246, 70, 93, 0.1)',
                color: isSuccess ? '#0ECB81' : '#F6465D',
              }}
              animate={{ scale: [1, 1.05, 1] }}
              transition={{ duration: 0.3, delay: 0.2 }}
            >
              {isSuccess ? (
                <>
                  <CheckCircle className="w-3 h-3 inline mr-1" />
                  SUCCESS
                </>
              ) : (
                <>
                  <XCircle className="w-3 h-3 inline mr-1" />
                  FAILED
                </>
              )}
            </motion.div>

            {/* Confidence Score */}
            {avgConfidence !== undefined && (
              <div className="w-16">
                <ConfidenceScore
                  confidence={avgConfidence * 100}
                  duration={1500 / animationSpeed}
                  showLabel={false}
                />
              </div>
            )}
          </motion.div>
        </div>

        {/* Processing Indicator */}
        {isProcessing && (
          <div className="mb-3">
            <ThinkingIndicator
              isThinking={true}
              text="Processing decision..."
            />
          </div>
        )}

        {/* Actions Summary */}
        {decision.decisions && decision.decisions.length > 0 && (
          <div className="flex flex-wrap gap-2 mb-3">
            {decision.decisions.map((action, actionIndex) => (
              <motion.div
                key={actionIndex}
                className="flex items-center gap-2 text-sm rounded px-3 py-2"
                style={{ background: 'var(--navy-primary)' }}
                initial={{ opacity: 0, scale: 0.8 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ delay: actionIndex * 0.1 }}
              >
                <span
                  className="font-mono font-bold"
                  style={{ color: '#EAECEF' }}
                >
                  {action.symbol}
                </span>
                <span
                  className="px-2 py-0.5 rounded text-xs font-bold"
                  style={{
                    background: action.action.includes('open')
                      ? 'rgba(96, 165, 250, 0.1)'
                      : 'rgba(0, 255, 127, 0.1)',
                    color: action.action.includes('open')
                      ? '#60a5fa'
                      : 'var(--green-primary)',
                  }}
                >
                  {action.action}
                </span>
                {action.price > 0 && (
                  <span
                    className="font-mono text-xs"
                    style={{ color: '#848E9C' }}
                  >
                    @{action.price.toFixed(4)}
                  </span>
                )}
              </motion.div>
            ))}
          </div>
        )}

        {/* Expandable Content - Input Prompt and Account State */}
        <AnimatePresence>
          {isExpanded && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              transition={{ duration: 0.2 }}
              style={{
                overflow: 'hidden',
                willChange: 'opacity, height',
              }}
            >
              <div
                className="border-t pt-4 mt-4"
                style={{ borderColor: 'var(--navy-light)' }}
              >
                {/* Account State */}
                {decision.account_state && (
                  <div className="text-xs mb-4" style={{ color: '#848E9C' }}>
                    Equity: ${decision.account_state.total_balance.toFixed(2)} |
                    Available: $
                    {decision.account_state.available_balance.toFixed(2)} |
                    Positions: {decision.account_state.position_count}
                  </div>
                )}
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        {/* Always Visible Section - AI Reasoning and DataFlowAnimation */}
        {(alwaysShowAnimation || isExpanded || decision.cot_trace) && (
          <div
            className="border-t pt-4 mt-auto flex flex-col"
            style={{ borderColor: 'var(--navy-light)', flex: '1 1 auto' }}
          >
            {/* Chain of Thought */}
            {decision.cot_trace && (
              <div
                ref={scrollContainerRef}
                className="mb-4 flex-1"
                style={{
                  minHeight: '100px',
                  maxHeight: '300px',
                  overflowY: 'auto',
                }}
              >
                <div
                  className="text-sm font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  AI Reasoning
                </div>
                <div style={{ position: 'relative' }}>
                  <AnimatedAIMessage
                    message={decision.cot_trace}
                    typingSpeed={30 / animationSpeed}
                    className="text-sm"
                    alwaysAnimate={alwaysShowAnimation}
                    autoScroll={autoScrollAiAnalysis}
                    scrollContainerRef={scrollContainerRef}
                    onComplete={onAnalysisComplete}
                    onScrollComplete={onScrollComplete}
                  />
                </div>
              </div>
            )}

            {/* Processing Flow - Always at bottom */}
            <div
              className="mt-auto"
              style={{
                flexShrink: 0,
                position: 'sticky',
                bottom: 0,
                background: 'var(--navy-dark)',
                borderTop: '1px solid var(--navy-light)',
                paddingTop: '8px',
                paddingBottom: '8px',
                zIndex: 5,
              }}
            >
              <div
                className="text-sm font-semibold mb-2"
                style={{ color: '#EAECEF' }}
              >
                Decision Process
              </div>
              <DataFlowAnimation
                currentStage={isProcessing ? 'Decision' : 'Complete'}
                isActive={isProcessing || alwaysShowAnimation}
              />
            </div>
          </div>
        )}
      </motion.div>
    </motion.div>
  )
})
