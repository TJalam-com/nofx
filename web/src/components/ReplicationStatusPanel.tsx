import { useState } from 'react'
import useSWR from 'swr'
import { api } from '../lib/api'
import type { ReplicationStatus, TestSignalRequest } from '../types'
import { toast } from 'sonner'
import {
  Users,
  UserCheck,
  Send,
  RefreshCw,
  AlertCircle,
  Loader,
  ChevronDown,
  ChevronUp,
} from 'lucide-react'

interface ReplicationStatusPanelProps {
  traderId: string
  traderName: string
}

export function ReplicationStatusPanel({
  traderId,
}: ReplicationStatusPanelProps) {
  const [showTestModal, setShowTestModal] = useState(false)
  const [testSignal, setTestSignal] = useState<TestSignalRequest>({
    symbol: 'BTCUSDT',
    action: 'open_long',
    leverage: 5,
    position_size_usd: 100,
    stop_loss: 0,
    take_profit: 0,
    reasoning: 'Test signal from frontend',
  })
  const [sendingSignal, setSendingSignal] = useState(false)
  const [expandedFollowers, setExpandedFollowers] = useState<string[]>([])

  const {
    data: status,
    mutate,
    error,
  } = useSWR<ReplicationStatus>(
    traderId ? `replication-status-${traderId}` : null,
    () => api.getReplicationStatus(traderId),
    { refreshInterval: 5000 }
  )

  const handleSendTestSignal = async () => {
    if (!testSignal.symbol || !testSignal.action) {
      toast.error('Symbol and action are required')
      return
    }

    setSendingSignal(true)
    try {
      const response = await api.sendTestSignal(traderId, testSignal)
      toast.success(
        `Test signal sent to ${response.followers_count} follower(s)`
      )
      setShowTestModal(false)
      mutate() // Refresh status
    } catch (err: any) {
      toast.error(err.message || 'Failed to send test signal')
    } finally {
      setSendingSignal(false)
    }
  }

  const toggleFollower = (followerId: string) => {
    setExpandedFollowers((prev) =>
      prev.includes(followerId)
        ? prev.filter((id) => id !== followerId)
        : [...prev, followerId]
    )
  }

  return (
    <div
      className="p-4 rounded"
      style={{
        background: 'rgb(11, 14, 17)',
        border: '1px solid rgb(43, 49, 57)',
      }}
    >
      {error ? (
        <div className="flex items-center gap-2 text-red-400">
          <AlertCircle size={16} />
          <span>Failed to load replication status</span>
        </div>
      ) : !status ? (
        <div className="flex items-center gap-2">
          <Loader className="animate-spin" size={16} />
          <span>Loading replication status...</span>
        </div>
      ) : (
        <>
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold flex items-center gap-2">
              <Users size={20} />
              Replication Status
            </h3>
            <button
              onClick={() => mutate()}
              className="p-2 rounded hover:bg-gray-800 transition-colors"
              title="Refresh"
            >
              <RefreshCw size={16} />
            </button>
          </div>

          {/* Parent Trader Info (if this is a child) */}
          {status.is_child && status.parent && (
            <div
              className="mb-4 p-3 rounded"
              style={{
                background: 'rgba(59, 130, 246, 0.1)',
                border: '1px solid rgba(59, 130, 246, 0.3)',
              }}
            >
              <div className="flex items-center gap-2 mb-2">
                <UserCheck size={16} style={{ color: '#3b82f6' }} />
                <span className="font-semibold">Following:</span>
                <span>{status.parent.trader_name}</span>
                {status.parent.is_running ? (
                  <span
                    className="px-2 py-0.5 rounded text-xs"
                    style={{
                      background: 'rgba(14, 203, 129, 0.1)',
                      color: '#0ECB81',
                    }}
                  >
                    Running
                  </span>
                ) : (
                  <span
                    className="px-2 py-0.5 rounded text-xs"
                    style={{
                      background: 'rgba(246, 70, 93, 0.1)',
                      color: '#F6465D',
                    }}
                  >
                    Stopped
                  </span>
                )}
              </div>
              {status.parent.error && (
                <div className="text-xs text-red-400 mt-1">
                  ⚠️ {status.parent.error}
                </div>
              )}
            </div>
          )}

          {/* Followers List (if this is a parent) */}
          {status.is_parent &&
            status.followers &&
            status.followers.length > 0 && (
              <div className="mb-4">
                <div className="flex items-center justify-between mb-2">
                  <span className="font-semibold">
                    Followers ({status.followers.length}):
                  </span>
                  <button
                    onClick={() => setShowTestModal(true)}
                    className="px-3 py-1.5 rounded text-sm font-medium flex items-center gap-2 transition-colors"
                    style={{
                      background: 'var(--brand-yellow)',
                      color: 'var(--navy-primary)',
                    }}
                  >
                    <Send size={14} />
                    Test Signal
                  </button>
                </div>
                <div className="space-y-2">
                  {status.followers.map((follower) => (
                    <div
                      key={follower.trader_id}
                      className="p-2 rounded"
                      style={{
                        background: 'rgba(43, 49, 57, 0.5)',
                        border: '1px solid rgb(43, 49, 57)',
                      }}
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => toggleFollower(follower.trader_id)}
                            className="p-1 hover:bg-gray-700 rounded"
                          >
                            {expandedFollowers.includes(follower.trader_id) ? (
                              <ChevronUp size={14} />
                            ) : (
                              <ChevronDown size={14} />
                            )}
                          </button>
                          <span className="font-medium">
                            {follower.trader_name}
                          </span>
                          {follower.is_running ? (
                            <span
                              className="px-2 py-0.5 rounded text-xs"
                              style={{
                                background: 'rgba(14, 203, 129, 0.1)',
                                color: '#0ECB81',
                              }}
                            >
                              Running
                            </span>
                          ) : (
                            <span
                              className="px-2 py-0.5 rounded text-xs"
                              style={{
                                background: 'rgba(246, 70, 93, 0.1)',
                                color: '#F6465D',
                              }}
                            >
                              Stopped
                            </span>
                          )}
                        </div>
                        <span className="text-xs text-gray-400">
                          ID: {follower.trader_id.slice(0, 8)}...
                        </span>
                      </div>
                      {follower.error && (
                        <div className="text-xs text-red-400 mt-1 ml-6">
                          ⚠️ {follower.error}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}

          {/* No followers message */}
          {status.is_parent &&
            (!status.followers || status.followers.length === 0) && (
              <div className="text-sm text-gray-400 text-center py-4">
                No followers yet
              </div>
            )}

          {/* Not a parent or child */}
          {!status.is_parent && !status.is_child && (
            <div className="text-sm text-gray-400 text-center py-4">
              This trader is not following anyone and has no followers
            </div>
          )}

          {/* Test Signal Modal */}
          {showTestModal && (
            <div
              className="fixed inset-0 flex items-center justify-center z-50"
              style={{ background: 'rgba(0, 31, 63, 0.5)' }}
              onClick={() => setShowTestModal(false)}
            >
              <div
                className="p-6 rounded max-w-md w-full mx-4"
                style={{
                  background: 'rgb(11, 14, 17)',
                  border: '1px solid rgb(43, 49, 57)',
                }}
                onClick={(e) => e.stopPropagation()}
              >
                <h3 className="text-lg font-semibold mb-4">Send Test Signal</h3>
                <div className="space-y-3">
                  <div>
                    <label className="block text-sm mb-1">Symbol</label>
                    <input
                      type="text"
                      value={testSignal.symbol}
                      onChange={(e) =>
                        setTestSignal({ ...testSignal, symbol: e.target.value })
                      }
                      className="w-full px-3 py-2 rounded"
                      style={{
                        background: 'rgb(30, 35, 41)',
                        border: '1px solid rgb(43, 49, 57)',
                        color: '#fff',
                      }}
                      placeholder="BTCUSDT"
                    />
                  </div>
                  <div>
                    <label className="block text-sm mb-1">Action</label>
                    <select
                      value={testSignal.action}
                      onChange={(e) =>
                        setTestSignal({ ...testSignal, action: e.target.value })
                      }
                      className="w-full px-3 py-2 rounded"
                      style={{
                        background: 'rgb(30, 35, 41)',
                        border: '1px solid rgb(43, 49, 57)',
                        color: '#fff',
                      }}
                    >
                      <option value="open_long">Open Long</option>
                      <option value="open_short">Open Short</option>
                      <option value="close_long">Close Long</option>
                      <option value="close_short">Close Short</option>
                    </select>
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="block text-sm mb-1">Leverage</label>
                      <input
                        type="number"
                        value={testSignal.leverage || ''}
                        onChange={(e) =>
                          setTestSignal({
                            ...testSignal,
                            leverage: parseInt(e.target.value) || undefined,
                          })
                        }
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: 'rgb(30, 35, 41)',
                          border: '1px solid rgb(43, 49, 57)',
                          color: '#fff',
                        }}
                        placeholder="5"
                      />
                    </div>
                    <div>
                      <label className="block text-sm mb-1">
                        Position Size (USD)
                      </label>
                      <input
                        type="number"
                        value={testSignal.position_size_usd || ''}
                        onChange={(e) =>
                          setTestSignal({
                            ...testSignal,
                            position_size_usd:
                              parseFloat(e.target.value) || undefined,
                          })
                        }
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: 'rgb(30, 35, 41)',
                          border: '1px solid rgb(43, 49, 57)',
                          color: '#fff',
                        }}
                        placeholder="100"
                      />
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="block text-sm mb-1">Stop Loss</label>
                      <input
                        type="number"
                        value={testSignal.stop_loss || ''}
                        onChange={(e) =>
                          setTestSignal({
                            ...testSignal,
                            stop_loss: parseFloat(e.target.value) || undefined,
                          })
                        }
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: 'rgb(30, 35, 41)',
                          border: '1px solid rgb(43, 49, 57)',
                          color: '#fff',
                        }}
                        placeholder="0"
                      />
                    </div>
                    <div>
                      <label className="block text-sm mb-1">Take Profit</label>
                      <input
                        type="number"
                        value={testSignal.take_profit || ''}
                        onChange={(e) =>
                          setTestSignal({
                            ...testSignal,
                            take_profit:
                              parseFloat(e.target.value) || undefined,
                          })
                        }
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: 'rgb(30, 35, 41)',
                          border: '1px solid rgb(43, 49, 57)',
                          color: '#fff',
                        }}
                        placeholder="0"
                      />
                    </div>
                  </div>
                  <div>
                    <label className="block text-sm mb-1">Reasoning</label>
                    <textarea
                      value={testSignal.reasoning || ''}
                      onChange={(e) =>
                        setTestSignal({
                          ...testSignal,
                          reasoning: e.target.value,
                        })
                      }
                      className="w-full px-3 py-2 rounded"
                      style={{
                        background: 'rgb(30, 35, 41)',
                        border: '1px solid rgb(43, 49, 57)',
                        color: '#fff',
                      }}
                      rows={3}
                      placeholder="Test signal reasoning..."
                    />
                  </div>
                </div>
                <div className="flex gap-3 mt-6">
                  <button
                    onClick={() => setShowTestModal(false)}
                    className="flex-1 px-4 py-2 rounded font-medium transition-colors"
                    style={{
                      background: 'rgb(43, 49, 57)',
                      color: '#fff',
                    }}
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleSendTestSignal}
                    disabled={sendingSignal}
                    className="flex-1 px-4 py-2 rounded font-medium transition-colors flex items-center justify-center gap-2"
                    style={{
                      background: 'var(--brand-yellow)',
                      color: 'var(--navy-primary)',
                    }}
                  >
                    {sendingSignal ? (
                      <>
                        <Loader className="animate-spin" size={16} />
                        Sending...
                      </>
                    ) : (
                      <>
                        <Send size={16} />
                        Send Signal
                      </>
                    )}
                  </button>
                </div>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}
