import { useState, useEffect } from 'react'
import { t, type Language } from '../../i18n/translations'
import { useAuth, isFollower } from '../../contexts/AuthContext'
import { api } from '../../lib/api'
import type { RunningTrader, AIModel, Exchange } from '../../types'

interface SignalSourceModalProps {
  coinPoolUrl: string
  oiTopUrl: string
  onSave: (coinPoolUrl: string, oiTopUrl: string) => void
  onClose: () => void
  language: Language
  configuredModels?: AIModel[]
  configuredExchanges?: Exchange[]
}

export function SignalSourceModal({
  coinPoolUrl,
  oiTopUrl,
  onSave,
  onClose,
  language,
  configuredModels = [],
  configuredExchanges = [],
}: SignalSourceModalProps) {
  const { user } = useAuth()
  const userIsFollower = isFollower(user)
  const [coinPool, setCoinPool] = useState(coinPoolUrl || '')
  const [oiTop, setOiTop] = useState(oiTopUrl || '')
  const [runningTraders, setRunningTraders] = useState<RunningTrader[]>([])
  const [selectedTraderId, setSelectedTraderId] = useState<string>('')
  const [loading, setLoading] = useState(false)

  // Check if follower has configured models and exchanges
  const hasConfiguredModels = configuredModels.length > 0
  const hasConfiguredExchanges = configuredExchanges.length > 0
  const canSaveAsFollower = userIsFollower 
    ? (hasConfiguredModels && hasConfiguredExchanges && selectedTraderId !== '')
    : true

  // Load running traders for followers
  useEffect(() => {
    if (userIsFollower) {
      setLoading(true)
      api
        .getRunningTraders()
        .then((traders) => {
          setRunningTraders(traders)
          setLoading(false)
        })
        .catch((err) => {
          console.error('Failed to load running traders:', err)
          setLoading(false)
        })
    }
  }, [userIsFollower])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!canSaveAsFollower) {
      return // Prevent submission if validation fails
    }
    if (userIsFollower) {
      // For followers, we don't save to user signal source
      // The selected trader will be used when creating/editing a trader
      // For now, we'll just close the modal
      // The actual saving will happen in TraderConfigModal
      onClose()
    } else {
      onSave(coinPool.trim(), oiTop.trim())
    }
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4 overflow-y-auto">
      <div
        className="bg-gray-800 rounded-lg w-full max-w-lg relative my-8"
        style={{
          background: '#1E2329',
          maxHeight: 'calc(100vh - 4rem)',
        }}
      >
        <h3 className="text-xl font-bold mb-4" style={{ color: '#EAECEF' }}>
          {t('signalSourceConfig', language)}
        </h3>

        <form onSubmit={handleSubmit} className="px-6 pb-6">
          <div
            className="space-y-4 overflow-y-auto"
            style={{ maxHeight: 'calc(100vh - 16rem)' }}
          >
            {userIsFollower ? (
              // Follower mode: Show trader selection
              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  Select Trader to Follow
                </label>
                {loading ? (
                  <div className="text-sm" style={{ color: '#848E9C' }}>
                    Loading running traders...
                  </div>
                ) : runningTraders.length === 0 ? (
                  <div className="text-sm" style={{ color: '#848E9C' }}>
                    No running traders available
                  </div>
                ) : (
                  <select
                    value={selectedTraderId}
                    onChange={(e) => setSelectedTraderId(e.target.value)}
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: '#0B0E11',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                  >
                    <option value="">-- Select a trader --</option>
                    {runningTraders.map((trader) => (
                      <option key={trader.trader_id} value={trader.trader_id}>
                        {trader.trader_name} ({trader.user_email})
                      </option>
                    ))}
                  </select>
                )}
                <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                  Select a running trader from any user to copy their signals.
                  This will be used when creating a new trader.
                </div>
                {userIsFollower && (!hasConfiguredModels || !hasConfiguredExchanges) && (
                  <div
                    className="p-3 rounded mt-3"
                    style={{
                      background: 'rgba(246, 70, 93, 0.1)',
                      border: '1px solid rgba(246, 70, 93, 0.2)',
                    }}
                  >
                    <div
                      className="text-sm font-semibold mb-1"
                      style={{ color: '#F6465D' }}
                    >
                      ⚠️ Configuration Required
                    </div>
                    <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
                      {!hasConfiguredModels && (
                        <div>• Please configure at least one AI model with API key</div>
                      )}
                      {!hasConfiguredExchanges && (
                        <div>• Please configure at least one exchange with API keys</div>
                      )}
                      <div className="mt-2">
                        You must configure both before selecting a trader to follow.
                      </div>
                    </div>
                  </div>
                )}
              </div>
            ) : (
              // Regular user mode: Show URL inputs
              <>
                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
                  >
                    COIN POOL URL
                  </label>
                  <input
                    type="url"
                    value={coinPool}
                    onChange={(e) => setCoinPool(e.target.value)}
                    placeholder="https://api.example.com/coinpool"
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: '#0B0E11',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                  />
                  <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                    {t('coinPoolDescription', language)}
                  </div>
                </div>

                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
                  >
                    OI TOP URL
                  </label>
                  <input
                    type="url"
                    value={oiTop}
                    onChange={(e) => setOiTop(e.target.value)}
                    placeholder="https://api.example.com/oitop"
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: '#0B0E11',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                    }}
                  />
                  <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                    {t('oiTopDescription', language)}
                  </div>
                </div>

                <div
                  className="p-4 rounded"
                  style={{
                    background: 'rgba(240, 185, 11, 0.1)',
                    border: '1px solid rgba(240, 185, 11, 0.2)',
                  }}
                >
                  <div
                    className="text-sm font-semibold mb-2"
                    style={{ color: '#F0B90B' }}
                  >
                    ℹ️ {t('information', language)}
                  </div>
                  <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
                    <div>{t('signalSourceInfo1', language)}</div>
                    <div>{t('signalSourceInfo2', language)}</div>
                    <div>{t('signalSourceInfo3', language)}</div>
                  </div>
                </div>
              </>
            )}
          </div>

          <div
            className="flex gap-3 mt-6 pt-4 sticky bottom-0"
            style={{ background: '#1E2329' }}
          >
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              disabled={!canSaveAsFollower}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50 disabled:cursor-not-allowed"
              style={{ 
                background: canSaveAsFollower ? '#F0B90B' : '#2B3139', 
                color: canSaveAsFollower ? '#000' : '#848E9C' 
              }}
            >
              {t('save', language)}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
