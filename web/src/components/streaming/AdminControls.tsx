import { useState, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Settings, Palette, Eye, Volume2, VolumeX, Zap, Brain } from 'lucide-react'
import { StreamingConfig } from '../../types'

interface AdminControlsProps {
  isMinimized: boolean
  onMinimize: () => void
  config: StreamingConfig
  onConfigChange: (config: Partial<StreamingConfig>) => void
}

/**
 * Admin Controls Panel for streaming customization
 * Features:
 * - Theme selector (Dark/Light/Cyberpunk)
 * - Animation speed control (0.5x - 2x)
 * - Widget visibility toggles
 * - Layout mode selector
 * - Sound effects toggle
 * - Watermark settings
 */
export function AdminControls({
  isMinimized,
  onMinimize,
  config,
  onConfigChange,
}: AdminControlsProps) {
  const [isExpanded, setIsExpanded] = useState(!isMinimized)

  useEffect(() => {
    setIsExpanded(!isMinimized)
  }, [isMinimized])

  const updateConfig = (updates: Partial<StreamingConfig>) => {
    onConfigChange({ ...config, ...updates })
  }

  return (
    <motion.div
      className="fixed top-4 right-4 z-50"
      initial={{ opacity: 0, scale: 0.9 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ duration: 0.3 }}
    >
      {/* Minimize Button */}
      <motion.button
        onClick={onMinimize}
        className="absolute -top-2 -left-2 w-8 h-8 rounded-full flex items-center justify-center shadow-lg"
        style={{
          background: 'var(--navy-dark)',
          border: '1px solid var(--navy-light)',
          color: '#EAECEF',
        }}
        whileHover={{ scale: 1.1 }}
        whileTap={{ scale: 0.9 }}
      >
        <Settings className="w-4 h-4" />
      </motion.button>

      <AnimatePresence>
        {isExpanded && (
          <motion.div
            initial={{ opacity: 0, scale: 0.9, y: -20 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.9, y: -20 }}
            transition={{ duration: 0.2 }}
            className="rounded-lg p-4 shadow-xl"
            style={{
              background: 'var(--navy-dark)',
              border: '1px solid var(--navy-light)',
              minWidth: '300px',
              maxHeight: 'calc(100vh - 100px)',
              overflowY: 'auto',
              overflowX: 'hidden',
            }}
          >
            <div className="flex items-center justify-between mb-4">
              <h3 className="font-bold" style={{ color: '#EAECEF' }}>
                Streaming Controls
              </h3>
              <motion.button
                onClick={() => updateConfig({ is_streaming: !config.is_streaming })}
                className="px-3 py-1 rounded text-xs font-bold transition-all"
                style={{
                  background: config.is_streaming
                    ? 'rgba(14, 203, 129, 0.2)'
                    : 'rgba(246, 70, 93, 0.2)',
                  color: config.is_streaming ? '#0ECB81' : '#F6465D',
                  border: `1px solid ${config.is_streaming ? '#0ECB81' : '#F6465D'}`,
                }}
                whileHover={{ scale: 1.05 }}
                whileTap={{ scale: 0.95 }}
              >
                {config.is_streaming ? 'STREAMING' : 'STOPPED'}
              </motion.button>
            </div>

            <div className="space-y-4" style={{ paddingBottom: '8px' }}>
              {/* Theme Selector */}
              <div>
                <label className="block text-sm font-medium mb-2" style={{ color: '#EAECEF' }}>
                  <Palette className="w-4 h-4 inline mr-2" />
                  Theme
                </label>
                <select
                  value={config.theme}
                  onChange={(e) => updateConfig({ theme: e.target.value as any })}
                  className="w-full rounded px-3 py-2 text-sm"
                  style={{
                    background: 'var(--navy-primary)',
                    border: '1px solid var(--navy-light)',
                    color: '#EAECEF',
                  }}
                >
                  <option value="dark">Dark</option>
                  <option value="light">Light</option>
                  <option value="cyberpunk">Cyberpunk</option>
                </select>
              </div>

              {/* Animation Speed */}
              <div>
                <label className="block text-sm font-medium mb-2" style={{ color: '#EAECEF' }}>
                  <Zap className="w-4 h-4 inline mr-2" />
                  Animation Speed: {config.animation_speed}x
                </label>
                <input
                  type="range"
                  min="0.5"
                  max="2"
                  step="0.1"
                  value={config.animation_speed}
                  onChange={(e) => updateConfig({ animation_speed: parseFloat(e.target.value) })}
                  className="w-full"
                />
                <div className="flex justify-between text-xs mt-1" style={{ color: '#848E9C' }}>
                  <span>Slow</span>
                  <span>Fast</span>
                </div>
              </div>

              {/* Widget Visibility */}
              <div>
                <label className="block text-sm font-medium mb-2" style={{ color: '#EAECEF' }}>
                  <Eye className="w-4 h-4 inline mr-2" />
                  Widget Visibility
                </label>
                <div className="grid grid-cols-2 gap-2">
                  {[
                    { key: 'equity', label: 'Equity Chart' },
                    { key: 'positions', label: 'Positions' },
                    { key: 'decisions', label: 'AI Decisions' },
                    { key: 'metrics', label: 'Metrics' },
                    { key: 'ai_process', label: 'AI Process' },
                  ].map(({ key, label }) => (
                    <label key={key} className="flex items-center gap-2 text-sm cursor-pointer">
                      <input
                        type="checkbox"
                        checked={config.widget_visibility[key as keyof typeof config.widget_visibility]}
                        onChange={(e) => updateConfig({
                          widget_visibility: {
                            ...config.widget_visibility,
                            [key]: e.target.checked,
                          },
                        })}
                      />
                      <span style={{ color: '#EAECEF' }}>{label}</span>
                    </label>
                  ))}
                </div>
              </div>

              {/* Layout Mode */}
              <div>
                <label className="block text-sm font-medium mb-2" style={{ color: '#EAECEF' }}>
                  Layout Mode
                </label>
                <select
                  value={config.layout_type}
                  onChange={(e) => updateConfig({ layout_type: e.target.value as any })}
                  className="w-full rounded px-3 py-2 text-sm"
                  style={{
                    background: 'var(--navy-primary)',
                    border: '1px solid var(--navy-light)',
                    color: '#EAECEF',
                  }}
                >
                  <option value="dashboard">Dashboard</option>
                  <option value="chart_focus">Chart Focus</option>
                  <option value="position_focus">Position Focus</option>
                </select>
              </div>

              {/* Sound Effects */}
              <div className="flex items-center justify-between">
                <label className="text-sm font-medium" style={{ color: '#EAECEF' }}>
                  {config.sound_enabled ? (
                    <Volume2 className="w-4 h-4 inline mr-2" />
                  ) : (
                    <VolumeX className="w-4 h-4 inline mr-2" />
                  )}
                  Sound Effects
                </label>
                <button
                  onClick={() => updateConfig({ sound_enabled: !config.sound_enabled })}
                  className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                    config.sound_enabled ? 'bg-green-500' : 'bg-gray-600'
                  }`}
                >
                  <span
                    className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                      config.sound_enabled ? 'translate-x-6' : 'translate-x-1'
                    }`}
                  />
                </button>
              </div>

              {/* Auto-Switch Charts */}
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <label className="text-sm font-medium" style={{ color: '#EAECEF' }}>
                    <Zap className="w-4 h-4 inline mr-2" />
                    Auto-Switch Charts
                  </label>
                  <button
                    onClick={() => updateConfig({ auto_switch_charts: !config.auto_switch_charts })}
                    className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                      config.auto_switch_charts ? 'bg-green-500' : 'bg-gray-600'
                    }`}
                  >
                    <span
                      className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                        config.auto_switch_charts ? 'translate-x-6' : 'translate-x-1'
                      }`}
                    />
                  </button>
                </div>

                {config.auto_switch_charts && (
                  <div>
                    <label className="block text-sm font-medium mb-2" style={{ color: '#EAECEF' }}>
                      Switch Interval (seconds)
                    </label>
                    <input
                      type="range"
                      min="5"
                      max="30"
                      step="5"
                      value={config.auto_switch_interval}
                      onChange={(e) => updateConfig({ auto_switch_interval: parseInt(e.target.value) })}
                      className="w-full h-2 bg-gray-600 rounded-lg appearance-none cursor-pointer slider"
                    />
                    <div className="flex justify-between text-xs mt-1" style={{ color: '#848E9C' }}>
                      <span>5s</span>
                      <span className="font-bold">{config.auto_switch_interval}s</span>
                      <span>30s</span>
                    </div>
                  </div>
                )}
              </div>

              {/* Auto-Scroll AI Analysis */}
              <div className="flex items-center justify-between">
                <label className="text-sm font-medium" style={{ color: '#EAECEF' }}>
                  <Brain className="w-4 h-4 inline mr-2" />
                  Auto-Scroll AI Analysis
                </label>
                <button
                  onClick={() => {
                    updateConfig({ auto_scroll_ai_analysis: !config.auto_scroll_ai_analysis })
                  }}
                  className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                    config.auto_scroll_ai_analysis ? 'bg-green-500' : 'bg-gray-600'
                  }`}
                >
                  <span
                    className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                      config.auto_scroll_ai_analysis ? 'translate-x-6' : 'translate-x-1'
                    }`}
                  />
                </button>
              </div>

              {/* Auto-Scroll Page */}
              <div className="flex items-center justify-between">
                <label className="text-sm font-medium" style={{ color: '#EAECEF' }}>
                  <Brain className="w-4 h-4 inline mr-2" />
                  Auto-Scroll Page (human-like)
                </label>
                <button
                  onClick={() => {
                    updateConfig({ auto_scroll_page: !config.auto_scroll_page })
                  }}
                  className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                    config.auto_scroll_page ? 'bg-green-500' : 'bg-gray-600'
                  }`}
                >
                  <span
                    className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                      config.auto_scroll_page ? 'translate-x-6' : 'translate-x-1'
                    }`}
                  />
                </button>
              </div>

              {config.auto_scroll_page && (
                <div className="mt-2">
                  <label className="block text-sm font-medium mb-2" style={{ color: '#EAECEF' }}>
                    Page Scroll Speed ({config.auto_scroll_page_speed.toFixed(1)}x)
                  </label>
                  <input
                    type="range"
                    min="0.5"
                    max="2"
                    step="0.1"
                    value={config.auto_scroll_page_speed}
                    onChange={(e) => updateConfig({ auto_scroll_page_speed: parseFloat(e.target.value) })}
                    className="w-full h-2 bg-gray-600 rounded-lg appearance-none cursor-pointer slider"
                  />
                  <div className="flex justify-between text-xs mt-1" style={{ color: '#848E9C' }}>
                    <span>0.5x</span>
                    <span className="font-bold">1x</span>
                    <span>2x</span>
                  </div>
                </div>
              )}

              {/* Watermark */}
              <div>
                <label className="block text-sm font-medium mb-2" style={{ color: '#EAECEF' }}>
                  Watermark Text
                </label>
                <input
                  type="text"
                  value={config.watermark_text}
                  onChange={(e) => updateConfig({ watermark_text: e.target.value })}
                  className="w-full rounded px-3 py-2 text-sm"
                  style={{
                    background: 'var(--navy-primary)',
                    border: '1px solid var(--navy-light)',
                    color: '#EAECEF',
                  }}
                  placeholder="Enter watermark text..."
                />
              </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  )
}

