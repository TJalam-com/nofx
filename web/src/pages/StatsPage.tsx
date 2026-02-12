import { useState, useCallback, useMemo, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import useSWR from 'swr'
import { api } from '../lib/api'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth, isAdmin } from '../contexts/AuthContext'
import { toast } from 'sonner'
import { t } from '../i18n/translations'
import {
  Play,
  Square,
  Users,
  Bot,
  RefreshCw,
  Search,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react'

interface Trader {
  trader_id: string
  trader_name: string
  user_id: string
  user_email: string
  ai_model: string
  exchange_id: string
  is_running: boolean
  initial_balance: number
}

interface User {
  id: string
  email: string
  role: string
  created_at: string
}

export default function StatsPage() {
  const { language } = useLanguage()
  const { user } = useAuth()
  const navigate = useNavigate()
  const [updatingRoles, setUpdatingRoles] = useState<Set<string>>(new Set())
  const [togglingTraders, setTogglingTraders] = useState<Set<string>>(new Set())

  // Search and pagination state for traders
  const [traderSearchQuery, setTraderSearchQuery] = useState('')
  const [traderCurrentPage, setTraderCurrentPage] = useState(1)
  const tradersPerPage = 10

  // Search and pagination state for users
  const [userSearchQuery, setUserSearchQuery] = useState('')
  const [userCurrentPage, setUserCurrentPage] = useState(1)
  const usersPerPage = 10

  const isUserAdmin = isAdmin(user)

  // Redirect non-admin users
  useEffect(() => {
    if (!isUserAdmin) {
      navigate('/traders')
    }
  }, [isUserAdmin, navigate])

  // Fetch all traders
  const {
    data: traders,
    mutate: mutateTraders,
    isLoading: tradersLoading,
  } = useSWR<Trader[]>(
    isUserAdmin ? 'admin-traders' : null,
    () => api.getAllTraders(),
    { refreshInterval: 5000 }
  )

  // Fetch all users
  const {
    data: users,
    mutate: mutateUsers,
    isLoading: usersLoading,
  } = useSWR<User[]>(
    isUserAdmin ? 'admin-users' : null,
    () => api.getAllUsers(),
    {
      refreshInterval: 10000,
    }
  )

  const handleToggleTrader = useCallback(
    async (traderId: string, running: boolean) => {
      // Add to loading set (button should be disabled, but this ensures state is tracked)
      setTogglingTraders((prev) => {
        if (prev.has(traderId)) {
          return prev // Already toggling, don't add again
        }
        return new Set(prev).add(traderId)
      })

      try {
        if (running) {
          await toast.promise(api.stopTrader(traderId), {
            loading: t('stoppingTrader', language),
            success: t('traderStopped', language),
            error: t('traderStopFailed', language),
          })
        } else {
          await toast.promise(api.startTrader(traderId), {
            loading: t('startingTrader', language),
            success: t('traderStarted', language),
            error: t('traderStartFailed', language),
          })
        }
        // Wait for backend to update state, then refresh and clear loading
        setTimeout(async () => {
          await mutateTraders()
          // Clear loading state AFTER mutation completes to prevent DOM conflicts
          setTogglingTraders((prev) => {
            const next = new Set(prev)
            next.delete(traderId)
            return next
          })
        }, 1000)
      } catch (error) {
        console.error('Failed to toggle trader:', error)
        toast.error(t('operationFailed', language))
        // Refresh state even on error to sync with backend
        setTimeout(async () => {
          await mutateTraders()
          // Clear loading state AFTER mutation completes to prevent DOM conflicts
          setTogglingTraders((prev) => {
            const next = new Set(prev)
            next.delete(traderId)
            return next
          })
        }, 500)
      }
    },
    [language, mutateTraders]
  )

  const handleRoleChange = async (userId: string, newRole: string) => {
    setUpdatingRoles((prev) => new Set(prev).add(userId))
    try {
      await api.updateUserRole(userId, newRole)
      toast.success('User role updated successfully')
      // Add a small delay before mutating to ensure backend has processed
      setTimeout(async () => {
        await mutateUsers()
      }, 500)
    } catch (error) {
      console.error('Failed to update user role:', error)
      const errorMessage =
        error instanceof Error ? error.message : 'Unknown error'
      toast.error(`Failed to update user role: ${errorMessage}`)
    } finally {
      setUpdatingRoles((prev) => {
        const next = new Set(prev)
        next.delete(userId)
        return next
      })
    }
  }

  // Filter and paginate traders
  const filteredTraders = useMemo(() => {
    if (!traders) return []
    if (!traderSearchQuery.trim()) return traders

    const query = traderSearchQuery.toLowerCase()
    return traders.filter(
      (trader) =>
        trader.trader_name.toLowerCase().includes(query) ||
        trader.user_email.toLowerCase().includes(query) ||
        trader.ai_model.toLowerCase().includes(query) ||
        trader.exchange_id.toLowerCase().includes(query) ||
        trader.trader_id.toLowerCase().includes(query)
    )
  }, [traders, traderSearchQuery])

  const paginatedTraders = useMemo(() => {
    const startIndex = (traderCurrentPage - 1) * tradersPerPage
    return filteredTraders.slice(startIndex, startIndex + tradersPerPage)
  }, [filteredTraders, traderCurrentPage, tradersPerPage])

  const traderTotalPages = Math.ceil(filteredTraders.length / tradersPerPage)

  // Filter and paginate users
  const filteredUsers = useMemo(() => {
    if (!users) return []
    if (!userSearchQuery.trim()) return users

    const query = userSearchQuery.toLowerCase()
    return users.filter(
      (user) =>
        user.email.toLowerCase().includes(query) ||
        user.id.toLowerCase().includes(query) ||
        (user.role || '').toLowerCase().includes(query)
    )
  }, [users, userSearchQuery])

  const paginatedUsers = useMemo(() => {
    const startIndex = (userCurrentPage - 1) * usersPerPage
    return filteredUsers.slice(startIndex, startIndex + usersPerPage)
  }, [filteredUsers, userCurrentPage, usersPerPage])

  const userTotalPages = Math.ceil(filteredUsers.length / usersPerPage)

  // Reset to page 1 when search query changes
  const handleTraderSearchChange = (query: string) => {
    setTraderSearchQuery(query)
    setTraderCurrentPage(1)
  }

  const handleUserSearchChange = (query: string) => {
    setUserSearchQuery(query)
    setUserCurrentPage(1)
  }

  return (
    <div className="max-w-[1920px] mx-auto px-6 py-6">
      <div className="mb-8">
        <h1
          className="text-3xl font-bold mb-2"
          style={{ color: 'var(--brand-yellow)' }}
        >
          Admin Statistics
        </h1>
        <p style={{ color: 'var(--text-secondary)' }}>
          Manage all traders and user roles
        </p>
      </div>

      {/* All Traders Section */}
      <div className="mb-8">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            <Bot className="w-6 h-6" style={{ color: 'var(--brand-yellow)' }} />
            <h2
              className="text-2xl font-bold"
              style={{ color: 'var(--brand-light-gray)' }}
            >
              All Traders
            </h2>
            {tradersLoading && (
              <RefreshCw
                className="w-5 h-5 animate-spin"
                style={{ color: 'var(--text-secondary)' }}
              />
            )}
          </div>
          <div className="relative flex-1 max-w-md ml-4">
            <Search
              className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4"
              style={{ color: 'var(--text-secondary)' }}
            />
            <input
              type="text"
              value={traderSearchQuery}
              onChange={(e) => handleTraderSearchChange(e.target.value)}
              placeholder="Search traders..."
              className="w-full pl-10 pr-4 py-2 rounded-lg border text-sm"
              style={{
                background: 'var(--input-bg)',
                borderColor: 'var(--input-border)',
                color: 'var(--text-primary)',
              }}
            />
          </div>
        </div>

        <div
          className="rounded-lg overflow-hidden"
          style={{
            background: 'var(--navy-dark)',
            border: '1px solid var(--panel-border)',
          }}
        >
          {tradersLoading ? (
            <div
              className="p-8 text-center"
              style={{ color: 'var(--text-secondary)' }}
            >
              Loading traders...
            </div>
          ) : filteredTraders.length > 0 ? (
            <>
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr
                      style={{ borderBottom: '1px solid var(--panel-border)' }}
                    >
                      <th
                        className="px-4 py-3 text-left text-sm font-semibold"
                        style={{ color: 'var(--brand-light-gray)' }}
                      >
                        Trader Name
                      </th>
                      <th
                        className="px-4 py-3 text-left text-sm font-semibold"
                        style={{ color: 'var(--brand-light-gray)' }}
                      >
                        Owner
                      </th>
                      <th
                        className="px-4 py-3 text-left text-sm font-semibold"
                        style={{ color: 'var(--brand-light-gray)' }}
                      >
                        AI Model
                      </th>
                      <th
                        className="px-4 py-3 text-left text-sm font-semibold"
                        style={{ color: 'var(--brand-light-gray)' }}
                      >
                        Exchange
                      </th>
                      <th
                        className="px-4 py-3 text-left text-sm font-semibold"
                        style={{ color: 'var(--brand-light-gray)' }}
                      >
                        Status
                      </th>
                      <th
                        className="px-4 py-3 text-left text-sm font-semibold"
                        style={{ color: 'var(--brand-light-gray)' }}
                      >
                        Balance
                      </th>
                      <th
                        className="px-4 py-3 text-center text-sm font-semibold"
                        style={{ color: 'var(--brand-light-gray)' }}
                      >
                        Actions
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {paginatedTraders.map((trader, index) => (
                      <tr
                        key={trader.trader_id}
                        style={{
                          borderBottom:
                            index < paginatedTraders.length - 1
                              ? '1px solid var(--panel-border)'
                              : 'none',
                        }}
                      >
                        <td
                          className="px-4 py-3 text-sm"
                          style={{ color: 'var(--brand-light-gray)' }}
                        >
                          {trader.trader_name}
                        </td>
                        <td
                          className="px-4 py-3 text-sm"
                          style={{ color: 'var(--text-secondary)' }}
                        >
                          {trader.user_email}
                        </td>
                        <td
                          className="px-4 py-3 text-sm"
                          style={{ color: 'var(--text-secondary)' }}
                        >
                          {trader.ai_model}
                        </td>
                        <td
                          className="px-4 py-3 text-sm"
                          style={{ color: 'var(--text-secondary)' }}
                        >
                          {trader.exchange_id}
                        </td>
                        <td className="px-4 py-3">
                          <span
                            className="px-2 py-1 rounded text-xs font-semibold"
                            style={{
                              background: trader.is_running
                                ? 'rgba(0, 200, 83, 0.2)'
                                : 'rgba(242, 153, 74, 0.2)',
                              color: trader.is_running ? '#00C853' : '#F2994A',
                            }}
                          >
                            {trader.is_running ? 'Running' : 'Stopped'}
                          </span>
                        </td>
                        <td
                          className="px-4 py-3 text-sm"
                          style={{ color: 'var(--brand-light-gray)' }}
                        >
                          ${trader.initial_balance.toLocaleString()}
                        </td>
                        <td className="px-4 py-3 text-center">
                          <button
                            onClick={() =>
                              handleToggleTrader(
                                trader.trader_id,
                                trader.is_running
                              )
                            }
                            disabled={togglingTraders.has(trader.trader_id)}
                            className="px-3 py-1.5 rounded text-sm font-semibold transition-all hover:opacity-80 flex items-center gap-2 mx-auto disabled:opacity-50 disabled:cursor-not-allowed"
                            style={{
                              background: trader.is_running
                                ? 'var(--binance-red-bg)'
                                : 'rgba(0, 200, 83, 0.2)',
                              color: trader.is_running
                                ? 'var(--binance-red)'
                                : '#00C853',
                            }}
                          >
                            {togglingTraders.has(trader.trader_id) ? (
                              <span
                                key={`loading-${trader.trader_id}`}
                                className="flex items-center gap-2"
                              >
                                <RefreshCw className="w-4 h-4 animate-spin" />
                                {trader.is_running
                                  ? t('stoppingTrader', language)
                                  : t('startingTrader', language)}
                              </span>
                            ) : trader.is_running ? (
                              <span
                                key={`stop-${trader.trader_id}`}
                                className="flex items-center gap-2"
                              >
                                <Square className="w-4 h-4" />
                                Stop
                              </span>
                            ) : (
                              <span
                                key={`start-${trader.trader_id}`}
                                className="flex items-center gap-2"
                              >
                                <Play className="w-4 h-4" />
                                Start
                              </span>
                            )}
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Pagination for Traders */}
              {traderTotalPages > 1 && (
                <div
                  className="flex items-center justify-between px-4 py-3 border-t"
                  style={{ borderColor: 'var(--panel-border)' }}
                >
                  <div
                    className="text-sm"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    Showing {(traderCurrentPage - 1) * tradersPerPage + 1} to{' '}
                    {Math.min(
                      traderCurrentPage * tradersPerPage,
                      filteredTraders.length
                    )}{' '}
                    of {filteredTraders.length} traders
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() =>
                        setTraderCurrentPage((prev) => Math.max(1, prev - 1))
                      }
                      disabled={traderCurrentPage === 1}
                      className="px-3 py-1.5 rounded border text-sm font-semibold transition-all disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1"
                      style={{
                        background: 'var(--navy-dark)',
                        borderColor: 'var(--panel-border)',
                        color: 'var(--text-primary)',
                      }}
                    >
                      <ChevronLeft className="w-4 h-4" />
                      Previous
                    </button>
                    <div className="flex items-center gap-1">
                      {Array.from(
                        { length: Math.min(5, traderTotalPages) },
                        (_, i) => {
                          let pageNum: number
                          if (traderTotalPages <= 5) {
                            pageNum = i + 1
                          } else if (traderCurrentPage <= 3) {
                            pageNum = i + 1
                          } else if (
                            traderCurrentPage >=
                            traderTotalPages - 2
                          ) {
                            pageNum = traderTotalPages - 4 + i
                          } else {
                            pageNum = traderCurrentPage - 2 + i
                          }
                          return (
                            <button
                              key={pageNum}
                              onClick={() => setTraderCurrentPage(pageNum)}
                              className={`px-3 py-1.5 rounded text-sm font-semibold transition-all ${
                                traderCurrentPage === pageNum
                                  ? 'border-2'
                                  : 'border'
                              }`}
                              style={{
                                background:
                                  traderCurrentPage === pageNum
                                    ? 'var(--brand-yellow)'
                                    : 'var(--navy-dark)',
                                borderColor:
                                  traderCurrentPage === pageNum
                                    ? 'var(--brand-yellow)'
                                    : 'var(--panel-border)',
                                color:
                                  traderCurrentPage === pageNum
                                    ? 'var(--navy-primary)'
                                    : 'var(--text-primary)',
                              }}
                            >
                              {pageNum}
                            </button>
                          )
                        }
                      )}
                    </div>
                    <button
                      onClick={() =>
                        setTraderCurrentPage((prev) =>
                          Math.min(traderTotalPages, prev + 1)
                        )
                      }
                      disabled={traderCurrentPage === traderTotalPages}
                      className="px-3 py-1.5 rounded border text-sm font-semibold transition-all disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1"
                      style={{
                        background: 'var(--navy-dark)',
                        borderColor: 'var(--panel-border)',
                        color: 'var(--text-primary)',
                      }}
                    >
                      Next
                      <ChevronRight className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              )}
            </>
          ) : (
            <div
              className="p-8 text-center"
              style={{ color: 'var(--text-secondary)' }}
            >
              {traderSearchQuery
                ? 'No traders found matching your search'
                : 'No traders found'}
            </div>
          )}
        </div>
      </div>

      {/* All Users Section */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            <Users
              className="w-6 h-6"
              style={{ color: 'var(--brand-yellow)' }}
            />
            <h2
              className="text-2xl font-bold"
              style={{ color: 'var(--brand-light-gray)' }}
            >
              All Users
            </h2>
            {usersLoading && (
              <RefreshCw
                className="w-5 h-5 animate-spin"
                style={{ color: 'var(--text-secondary)' }}
              />
            )}
          </div>
          <div className="relative flex-1 max-w-md ml-4">
            <Search
              className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4"
              style={{ color: 'var(--text-secondary)' }}
            />
            <input
              type="text"
              value={userSearchQuery}
              onChange={(e) => handleUserSearchChange(e.target.value)}
              placeholder="Search users..."
              className="w-full pl-10 pr-4 py-2 rounded-lg border text-sm"
              style={{
                background: 'var(--input-bg)',
                borderColor: 'var(--input-border)',
                color: 'var(--text-primary)',
              }}
            />
          </div>
        </div>

        <div
          className="rounded-lg overflow-hidden"
          style={{
            background: 'var(--navy-dark)',
            border: '1px solid var(--panel-border)',
          }}
        >
          {usersLoading ? (
            <div
              className="p-8 text-center"
              style={{ color: 'var(--text-secondary)' }}
            >
              Loading users...
            </div>
          ) : filteredUsers.length > 0 ? (
            <>
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr
                      style={{ borderBottom: '1px solid var(--panel-border)' }}
                    >
                      <th
                        className="px-4 py-3 text-left text-sm font-semibold"
                        style={{ color: 'var(--brand-light-gray)' }}
                      >
                        Email
                      </th>
                      <th
                        className="px-4 py-3 text-left text-sm font-semibold"
                        style={{ color: 'var(--brand-light-gray)' }}
                      >
                        User ID
                      </th>
                      <th
                        className="px-4 py-3 text-left text-sm font-semibold"
                        style={{ color: 'var(--brand-light-gray)' }}
                      >
                        Role
                      </th>
                      <th
                        className="px-4 py-3 text-left text-sm font-semibold"
                        style={{ color: 'var(--brand-light-gray)' }}
                      >
                        Created
                      </th>
                      <th
                        className="px-4 py-3 text-center text-sm font-semibold"
                        style={{ color: 'var(--brand-light-gray)' }}
                      >
                        Actions
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {paginatedUsers.map((userItem, index) => (
                      <tr
                        key={userItem.id}
                        style={{
                          borderBottom:
                            index < paginatedUsers.length - 1
                              ? '1px solid var(--panel-border)'
                              : 'none',
                        }}
                      >
                        <td
                          className="px-4 py-3 text-sm"
                          style={{ color: 'var(--brand-light-gray)' }}
                        >
                          {userItem.email}
                        </td>
                        <td
                          className="px-4 py-3 text-sm font-mono text-xs"
                          style={{ color: 'var(--text-secondary)' }}
                        >
                          {userItem.id}
                        </td>
                        <td className="px-4 py-3">
                          <span
                            className="px-2 py-1 rounded text-xs font-semibold"
                            style={{
                              background:
                                userItem.role === 'admin'
                                  ? 'rgba(0, 255, 127, 0.2)'
                                  : userItem.role === 'follower'
                                    ? 'rgba(74, 144, 226, 0.2)'
                                    : 'rgba(255, 255, 255, 0.1)',
                              color:
                                userItem.role === 'admin'
                                  ? 'var(--brand-yellow)'
                                  : userItem.role === 'follower'
                                    ? '#4A90E2'
                                    : 'var(--brand-light-gray)',
                            }}
                          >
                            {userItem.role || 'user'}
                          </span>
                        </td>
                        <td
                          className="px-4 py-3 text-sm"
                          style={{ color: 'var(--text-secondary)' }}
                        >
                          {(() => {
                            try {
                              const date = new Date(userItem.created_at)
                              if (isNaN(date.getTime())) {
                                return 'Invalid date'
                              }
                              return date.toLocaleDateString()
                            } catch (error) {
                              return 'Invalid date'
                            }
                          })()}
                        </td>
                        <td className="px-4 py-3">
                          <select
                            value={userItem.role || 'user'}
                            onChange={(e) =>
                              handleRoleChange(userItem.id, e.target.value)
                            }
                            disabled={updatingRoles.has(userItem.id)}
                            className="px-3 py-1.5 rounded text-sm font-semibold transition-all"
                            style={{
                              background: 'var(--navy-dark)',
                              border: '1px solid var(--panel-border)',
                              color: 'var(--brand-light-gray)',
                              cursor: updatingRoles.has(userItem.id)
                                ? 'not-allowed'
                                : 'pointer',
                              opacity: updatingRoles.has(userItem.id) ? 0.6 : 1,
                            }}
                          >
                            <option value="user">User</option>
                            <option value="follower">Follower</option>
                            <option value="admin">Admin</option>
                          </select>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Pagination for Users */}
              {userTotalPages > 1 && (
                <div
                  className="flex items-center justify-between px-4 py-3 border-t"
                  style={{ borderColor: 'var(--panel-border)' }}
                >
                  <div
                    className="text-sm"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    Showing {(userCurrentPage - 1) * usersPerPage + 1} to{' '}
                    {Math.min(
                      userCurrentPage * usersPerPage,
                      filteredUsers.length
                    )}{' '}
                    of {filteredUsers.length} users
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() =>
                        setUserCurrentPage((prev) => Math.max(1, prev - 1))
                      }
                      disabled={userCurrentPage === 1}
                      className="px-3 py-1.5 rounded border text-sm font-semibold transition-all disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1"
                      style={{
                        background: 'var(--navy-dark)',
                        borderColor: 'var(--panel-border)',
                        color: 'var(--text-primary)',
                      }}
                    >
                      <ChevronLeft className="w-4 h-4" />
                      Previous
                    </button>
                    <div className="flex items-center gap-1">
                      {Array.from(
                        { length: Math.min(5, userTotalPages) },
                        (_, i) => {
                          let pageNum: number
                          if (userTotalPages <= 5) {
                            pageNum = i + 1
                          } else if (userCurrentPage <= 3) {
                            pageNum = i + 1
                          } else if (userCurrentPage >= userTotalPages - 2) {
                            pageNum = userTotalPages - 4 + i
                          } else {
                            pageNum = userCurrentPage - 2 + i
                          }
                          return (
                            <button
                              key={pageNum}
                              onClick={() => setUserCurrentPage(pageNum)}
                              className={`px-3 py-1.5 rounded text-sm font-semibold transition-all ${
                                userCurrentPage === pageNum
                                  ? 'border-2'
                                  : 'border'
                              }`}
                              style={{
                                background:
                                  userCurrentPage === pageNum
                                    ? 'var(--brand-yellow)'
                                    : 'var(--navy-dark)',
                                borderColor:
                                  userCurrentPage === pageNum
                                    ? 'var(--brand-yellow)'
                                    : 'var(--panel-border)',
                                color:
                                  userCurrentPage === pageNum
                                    ? 'var(--navy-primary)'
                                    : 'var(--text-primary)',
                              }}
                            >
                              {pageNum}
                            </button>
                          )
                        }
                      )}
                    </div>
                    <button
                      onClick={() =>
                        setUserCurrentPage((prev) =>
                          Math.min(userTotalPages, prev + 1)
                        )
                      }
                      disabled={userCurrentPage === userTotalPages}
                      className="px-3 py-1.5 rounded border text-sm font-semibold transition-all disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1"
                      style={{
                        background: 'var(--navy-dark)',
                        borderColor: 'var(--panel-border)',
                        color: 'var(--text-primary)',
                      }}
                    >
                      Next
                      <ChevronRight className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              )}
            </>
          ) : (
            <div
              className="p-8 text-center"
              style={{ color: 'var(--text-secondary)' }}
            >
              {userSearchQuery
                ? 'No users found matching your search'
                : 'No users found'}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
