import { useState, useMemo } from 'react'
import type { DecisionRecord } from '../types'
import { t, type Language } from '../i18n/translations'
import { UserCheck, CheckCircle, XCircle, Wrench } from 'lucide-react'
import { useAuth, isAdmin } from '../contexts/AuthContext'
import { AdminAnimationWrapper } from './animations/AdminAnimationWrapper'
import { AnimatedAIMessage } from './animations/AnimatedAIMessage'
import { ThinkingIndicator } from './animations/ThinkingIndicator'
import { ConfidenceScore } from './animations/ConfidenceScore'
import { SentimentIndicator } from './animations/SentimentIndicator'

interface DecisionCardProps {
  decision: DecisionRecord
  language: Language
}

// Helper function to translate AI error messages
function translateAIErrorMessage(message: string, language: 'en' | 'zh'): string {
  if (!message) return message

  // If message contains Chinese characters and language is English, try to translate
  if (language === 'en' && /[\u4e00-\u9fff]/.test(message)) {
    // Translate "AI拒绝: " prefix
    let translated = message.replace(/^AI拒绝:\s*/i, t('aiRejected', language))

    // Pattern: 信号价格X与当前市场价Y严重不符（相差Z%）
    const priceMismatchPattern = /信号价格([\d.]+)与当前市场价([\d.]+)严重不符（相差([\d.]+)%）/
    while (priceMismatchPattern.test(translated)) {
      const match = translated.match(priceMismatchPattern)
      if (match) {
        const signalPrice = match[1]
        const marketPrice = match[2]
        const diff = match[3]
        translated = translated.replace(
          priceMismatchPattern,
          t('signalPriceMismatch', language, {
            signalPrice,
            marketPrice,
            diff,
          })
        )
      } else {
        break
      }
    }

    // Pattern: 账户资金仅X USDT,无法满足最小交易要求
    const balancePattern = /账户资金仅([\d.]+)\s*USDT,无法满足最小交易要求/
    if (balancePattern.test(translated)) {
      const match = translated.match(balancePattern)
      if (match) {
        const balance = match[1]
        translated = translated.replace(
          balancePattern,
          t('insufficientBalance', language, { balance })
        )
      }
    }

    // Pattern: 当前技术面（...）也不支持...操作
    const techAnalysisPattern = /当前技术面（([^）]+)）也不支持([^。]+)操作/
    if (techAnalysisPattern.test(translated)) {
      const match = translated.match(techAnalysisPattern)
      if (match) {
        const details = match[1]
        const action = match[2]
        translated = translated.replace(
          techAnalysisPattern,
          t('technicalAnalysisNotSupport', language, { details, action })
        )
      }
    }

    // Clean up any remaining Chinese punctuation/conjunctions
    translated = translated.replace(/，/g, ', ').replace(/。/g, '.')

    return translated
  }

  return message
}

export function DecisionCard({ decision, language }: DecisionCardProps) {
  const { user } = useAuth()
  const admin = isAdmin(user)
  const [showInputPrompt, setShowInputPrompt] = useState(false)
  const [showCoT, setShowCoT] = useState(false)

  // Parse decision JSON and input prompt to extract parent signal info
  const parentSignalInfo = useMemo(() => {
    try {
      // First try to parse from decision JSON
      if (decision.decision_json) {
        const decisions = JSON.parse(decision.decision_json)
        if (Array.isArray(decisions) && decisions.length > 0) {
          const firstDecision = decisions[0]
          if (firstDecision.parent_signal_id || firstDecision.signal_decision) {
            return {
              parent_signal_id: firstDecision.parent_signal_id,
              signal_decision: firstDecision.signal_decision,
            }
          }
        }
      }
      
      // Fallback: Check input prompt for parent signal indicators
      if (decision.input_prompt) {
        const hasParentSignal = decision.input_prompt.includes('Parent Trade Signal') ||
                                 decision.input_prompt.includes('Parent Trader:') ||
                                 decision.input_prompt.includes('Signal ID:')
        if (hasParentSignal) {
          // Try to extract signal ID from prompt
          const signalIdMatch = decision.input_prompt.match(/Signal ID:\s*([a-f0-9-]+)/i)
          const signalDecisionMatch = decision.input_prompt.match(/signal_decision[":\s]+"?(accept|reject|modify)"?/i)
          
          return {
            parent_signal_id: signalIdMatch ? signalIdMatch[1] : undefined,
            signal_decision: signalDecisionMatch ? signalDecisionMatch[1].toLowerCase() : undefined,
            fromPrompt: true,
          }
        }
      }
    } catch (e) {
      // Ignore parse errors
    }
    return null
  }, [decision.decision_json, decision.input_prompt])

  return (
    <div
      className="rounded p-5 transition-all duration-300 hover:translate-y-[-2px]"
      style={{
        border: '1px solid var(--panel-border)',
        background: 'var(--panel-bg)',
        boxShadow: '0 2px 8px rgba(0, 0, 0, 0.3)',
      }}
    >
      <div className="flex items-start justify-between mb-3">
        <div>
          <div className="font-semibold flex items-center gap-2" style={{ color: '#EAECEF' }}>
            {t('cycle', language)} #{decision.cycle_number}
            {parentSignalInfo && (
              <span
                className="px-2 py-0.5 rounded text-xs flex items-center gap-1"
                style={{
                  background: 'rgba(59, 130, 246, 0.1)',
                  color: '#3b82f6',
                }}
                title="Parent trade signal"
              >
                <UserCheck size={12} />
                Parent Signal
              </span>
            )}
            {parentSignalInfo?.signal_decision && (
              <span
                className="px-2 py-0.5 rounded text-xs flex items-center gap-1"
                style={
                  parentSignalInfo.signal_decision === 'accept'
                    ? {
                        background: 'rgba(14, 203, 129, 0.1)',
                        color: '#0ECB81',
                      }
                    : parentSignalInfo.signal_decision === 'reject'
                    ? {
                        background: 'rgba(246, 70, 93, 0.1)',
                        color: '#F6465D',
                      }
                    : {
                        background: 'rgba(251, 191, 36, 0.1)',
                        color: 'var(--brand-yellow)',
                      }
                }
                title={`AI decision: ${parentSignalInfo.signal_decision}`}
              >
                {parentSignalInfo.signal_decision === 'accept' ? (
                  <CheckCircle size={12} />
                ) : parentSignalInfo.signal_decision === 'reject' ? (
                  <XCircle size={12} />
                ) : (
                  <Wrench size={12} />
                )}
                {parentSignalInfo.signal_decision.toUpperCase()}
              </span>
            )}
          </div>
          <div className="text-xs" style={{ color: '#848E9C' }}>
            {new Date(decision.timestamp).toLocaleString()}
            {parentSignalInfo?.parent_signal_id && (
              <span className="ml-2">
                • Signal ID: {parentSignalInfo.parent_signal_id.slice(0, 8)}...
              </span>
            )}
          </div>
        </div>
        <div
          className="px-3 py-1 rounded text-xs font-bold"
          style={
            decision.success
              ? { background: 'rgba(14, 203, 129, 0.1)', color: '#0ECB81' }
              : { background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }
          }
        >
          {t(decision.success ? 'success' : 'failed', language)}
        </div>
      </div>

      {decision.input_prompt && (
        <div className="mb-3">
          <button
            onClick={() => setShowInputPrompt(!showInputPrompt)}
            className="flex items-center gap-2 text-sm transition-colors"
            style={{ color: '#60a5fa' }}
          >
            <span className="font-semibold">
              📥 {t('inputPrompt', language)}
            </span>
            <span className="text-xs">
              {showInputPrompt ? t('collapse', language) : t('expand', language)}
            </span>
          </button>
          {showInputPrompt && (
            <div
              className="mt-2 rounded p-4 text-sm font-mono whitespace-pre-wrap max-h-96 overflow-y-auto"
              style={{
                background: 'var(--navy-primary)',
                border: '1px solid var(--panel-border)',
                color: '#EAECEF',
              }}
            >
              {decision.input_prompt}
            </div>
          )}
        </div>
      )}

      {decision.cot_trace && (
        <div className="mb-3">
          <div className="flex items-center justify-between mb-2">
            <button
              onClick={() => setShowCoT(!showCoT)}
              className="flex items-center gap-2 text-sm transition-colors"
              style={{ color: '#F0B90B' }}
            >
              <span className="font-semibold">
                📤 {t('aiThinking', language)}
              </span>
              <span className="text-xs">
                {showCoT ? t('collapse', language) : t('expand', language)}
              </span>
            </button>
            <AdminAnimationWrapper>
              <ThinkingIndicator isThinking={!showCoT && admin} />
            </AdminAnimationWrapper>
          </div>
          {showCoT && (
            <AdminAnimationWrapper
              staticFallback={
                <div
                  className="mt-2 rounded p-4 text-sm font-mono whitespace-pre-wrap max-h-96 overflow-y-auto"
                  style={{
                    background: 'var(--navy-primary)',
                    border: '1px solid var(--panel-border)',
                    color: '#EAECEF',
                  }}
                >
                  {decision.cot_trace}
                </div>
              }
            >
              <div
                className="mt-2 rounded p-4 text-sm font-mono max-h-96 overflow-y-auto"
                style={{
                  background: 'var(--navy-primary)',
                  border: '1px solid var(--panel-border)',
                  color: '#EAECEF',
                }}
              >
                <AnimatedAIMessage message={decision.cot_trace} typingSpeed={30} />
              </div>
            </AdminAnimationWrapper>
          )}
        </div>
      )}

      {decision.decisions && decision.decisions.length > 0 && (
        <div className="space-y-2 mb-3">
          {decision.decisions.map((action, index) => {
            // Determine sentiment based on action
            const sentiment: 'positive' | 'negative' | 'neutral' =
              action.action.includes('open') || action.success
                ? 'positive'
                : action.action.includes('close') && !action.success
                  ? 'negative'
                  : 'neutral'

            return (
              <div
                key={`${action.symbol}-${index}`}
                className="flex items-center gap-2 text-sm rounded px-3 py-2"
                style={{ background: 'var(--navy-primary)' }}
              >
                <span
                  className="font-mono font-bold"
                  style={{ color: '#EAECEF' }}
                >
                  {action.symbol}
                </span>
                <span
                  className="px-2 py-0.5 rounded text-xs font-bold"
                  style={
                    action.action.includes('open')
                      ? {
                          background: 'rgba(96, 165, 250, 0.1)',
                          color: '#60a5fa',
                        }
                      : action.action.includes('close')
                        ? {
                            background: 'rgba(14, 203, 129, 0.1)',
                            color: '#0ECB81',
                          }
                        : {
                            background: 'rgba(248, 113, 113, 0.1)',
                            color: '#F87171',
                          }
                  }
                >
                  {action.action}
                </span>
                {action.reasoning && (
                  <span
                    className="text-xs"
                    style={{ color: '#848E9C', flex: 1 }}
                  >
                    {action.reasoning}
                  </span>
                )}
                <AdminAnimationWrapper>
                  <SentimentIndicator sentiment={sentiment} intensity={action.success ? 0.8 : 0.5} />
                </AdminAnimationWrapper>
                {action.confidence !== undefined && action.confidence !== null && (
                  <AdminAnimationWrapper>
                    <div style={{ minWidth: '100px' }}>
                      <ConfidenceScore
                        confidence={
                          typeof action.confidence === 'number'
                            ? action.confidence <= 1
                              ? action.confidence * 100
                              : action.confidence
                            : 0
                        }
                        showLabel={false}
                      />
                    </div>
                  </AdminAnimationWrapper>
                )}
              </div>
            )
          })}
        </div>
      )}

      {/* Confidence Score for overall decision */}
      {admin && decision.decisions && decision.decisions.length > 0 && (
        <AdminAnimationWrapper>
          <div className="mb-3">
            {decision.decisions.some((d) => d.confidence !== undefined && d.confidence !== null) && (
              <ConfidenceScore
                confidence={
                  (() => {
                    const confidences = decision.decisions
                      .map((d) => d.confidence)
                      .filter((c): c is number => c !== undefined && c !== null)
                    if (confidences.length === 0) return 0
                    const avg = confidences.reduce((sum, c) => sum + c, 0) / confidences.length
                    // Assume confidence is 0-1 if average is <= 1, otherwise 0-100
                    return avg <= 1 ? avg * 100 : avg
                  })()
                }
              />
            )}
          </div>
        </AdminAnimationWrapper>
      )}

      {decision.execution_log && decision.execution_log.length > 0 && (
        <div
          className="rounded p-3 text-xs font-mono space-y-1"
          style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
        >
          {decision.execution_log.map((log, index) => (
            <div key={`${log}-${index}`} style={{ color: '#EAECEF' }}>
              {log}
            </div>
          ))}
        </div>
      )}

      {decision.error_message && (
        <div
          className="rounded p-3 mt-3 text-sm"
          style={{
            background: 'rgba(246, 70, 93, 0.1)',
            border: '1px solid rgba(246, 70, 93, 0.4)',
            color: '#F6465D',
          }}
        >
          ❌ {translateAIErrorMessage(decision.error_message, language)}
        </div>
      )}
    </div>
  )
}
