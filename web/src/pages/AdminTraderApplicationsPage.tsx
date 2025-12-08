import { useState, useCallback, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import useSWR from 'swr'
import { api } from '../lib/api'
import { useAuth, isAdmin } from '../contexts/AuthContext'
import { toast } from 'sonner'
import type { TraderApplication } from '../types'
import {
  CheckCircle2,
  XCircle,
  Clock,
  RefreshCw,
  ChevronDown,
  ChevronUp,
  Search,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react'

export default function AdminTraderApplicationsPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [filterStatus, setFilterStatus] = useState<'all' | 'pending' | 'approved' | 'rejected'>(
    'all'
  )
  const [expandedApps, setExpandedApps] = useState<Set<string>>(new Set())
  const [approvingApps, setApprovingApps] = useState<Set<string>>(new Set())
  const [rejectingApps, setRejectingApps] = useState<Set<string>>(new Set())
  const [adminNotes, setAdminNotes] = useState<Record<string, string>>({})
  const [showRejectModal, setShowRejectModal] = useState<string | null>(null)
  
  // Search and pagination state
  const [searchQuery, setSearchQuery] = useState('')
  const [currentPage, setCurrentPage] = useState(1)
  const applicationsPerPage = 10

  // Redirect if not admin
  if (!isAdmin(user)) {
    navigate('/traders')
    return null
  }

  // Fetch all trader applications
  const {
    data: applications,
    mutate: mutateApplications,
    isLoading: applicationsLoading,
  } = useSWR<TraderApplication[]>(
    user ? 'admin-trader-applications' : null,
    () => api.getAllTraderApplications(),
    { refreshInterval: 10000 }
  )

  // Filter applications by status and search query
  const filteredApplications = useMemo(() => {
    if (!applications) return []
    
    let filtered = applications
    
    // Filter by status
    if (filterStatus !== 'all') {
      filtered = filtered.filter((app) => app.status === filterStatus)
    }
    
    // Filter by search query
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase()
      filtered = filtered.filter(
        (app) =>
          app.name.toLowerCase().includes(query) ||
          app.email.toLowerCase().includes(query) ||
          (app.user_email || '').toLowerCase().includes(query) ||
          app.user_id.toLowerCase().includes(query) ||
          app.description.toLowerCase().includes(query) ||
          app.trading_experience.toLowerCase().includes(query) ||
          app.strategy_overview.toLowerCase().includes(query)
      )
    }
    
    return filtered
  }, [applications, filterStatus, searchQuery])

  // Paginate filtered applications
  const paginatedApplications = useMemo(() => {
    const startIndex = (currentPage - 1) * applicationsPerPage
    return filteredApplications.slice(startIndex, startIndex + applicationsPerPage)
  }, [filteredApplications, currentPage, applicationsPerPage])

  const totalPages = Math.ceil(filteredApplications.length / applicationsPerPage)

  // Reset to page 1 when search query or filter status changes
  const handleSearchChange = (query: string) => {
    setSearchQuery(query)
    setCurrentPage(1)
  }

  const handleFilterChange = (status: 'all' | 'pending' | 'approved' | 'rejected') => {
    setFilterStatus(status)
    setCurrentPage(1)
  }

  const toggleExpand = (appId: string) => {
    setExpandedApps((prev) => {
      const next = new Set(prev)
      if (next.has(appId)) {
        next.delete(appId)
      } else {
        next.add(appId)
      }
      return next
    })
  }

  const handleApprove = useCallback(
    async (appId: string) => {
      setApprovingApps((prev) => new Set(prev).add(appId))
      try {
        const notes = adminNotes[appId] || ''
        await toast.promise(api.approveTraderApplication(appId, notes), {
          loading: 'Approving application...',
          success: 'Application approved, user role upgraded',
          error: 'Failed to approve application',
        })
        // Clear admin notes before mutation
        setAdminNotes((prev) => {
          const next = { ...prev }
          delete next[appId]
          return next
        })
        // Wait a bit before mutating to ensure state is stable
        await new Promise((resolve) => setTimeout(resolve, 100))
        await mutateApplications()
      } catch (error: any) {
        console.error('Failed to approve application:', error)
      } finally {
        // Delay clearing loading state to prevent DOM conflicts
        setTimeout(() => {
          setApprovingApps((prev) => {
            const next = new Set(prev)
            next.delete(appId)
            return next
          })
        }, 200)
      }
    },
    [adminNotes, mutateApplications]
  )

  const handleReject = useCallback(
    async (appId: string) => {
      const notes = adminNotes[appId] || ''
      if (!notes.trim()) {
        toast.error('Please provide rejection reason')
        return
      }

      setRejectingApps((prev) => new Set(prev).add(appId))
      try {
        await toast.promise(api.rejectTraderApplication(appId, notes), {
          loading: 'Rejecting application...',
          success: 'Application rejected',
          error: 'Failed to reject application',
        })
        // Close modal before mutation
        setShowRejectModal(null)
        // Clear admin notes before mutation
        setAdminNotes((prev) => {
          const next = { ...prev }
          delete next[appId]
          return next
        })
        // Wait a bit before mutating to ensure state is stable
        await new Promise((resolve) => setTimeout(resolve, 100))
        await mutateApplications()
      } catch (error: any) {
        console.error('Failed to reject application:', error)
      } finally {
        // Delay clearing loading state to prevent DOM conflicts
        setTimeout(() => {
          setRejectingApps((prev) => {
            const next = new Set(prev)
            next.delete(appId)
            return next
          })
        }, 200)
      }
    },
    [adminNotes, mutateApplications]
  )

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'pending':
        return (
          <span className="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-semibold bg-yellow-500/20 text-yellow-400 border border-yellow-500/30">
            <Clock className="w-3 h-3" />
            Pending
          </span>
        )
      case 'approved':
        return (
          <span className="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-semibold bg-green-500/20 text-green-400 border border-green-500/30">
            <CheckCircle2 className="w-3 h-3" />
            Approved
          </span>
        )
      case 'rejected':
        return (
          <span className="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-semibold bg-red-500/20 text-red-400 border border-red-500/30">
            <XCircle className="w-3 h-3" />
            Rejected
          </span>
        )
      default:
        return null
    }
  }

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-3xl font-bold mb-2" style={{ color: 'var(--text-primary)' }}>
            Application Management
          </h1>
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            Review and manage user applications
          </p>
        </div>
        <div className="flex items-center gap-3">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4" style={{ color: 'var(--text-secondary)' }} />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => handleSearchChange(e.target.value)}
              placeholder="Search applications..."
              className="pl-10 pr-4 py-2 rounded-lg border text-sm"
              style={{
                background: 'var(--input-bg)',
                borderColor: 'var(--input-border)',
                color: 'var(--text-primary)',
              }}
            />
          </div>
          <button
            onClick={() => mutateApplications()}
            disabled={applicationsLoading}
            className="px-4 py-2 rounded-lg border font-semibold transition-colors flex items-center gap-2 disabled:opacity-50"
            style={{
              background: 'var(--panel-bg)',
              borderColor: 'var(--panel-border)',
              color: 'var(--text-primary)',
            }}
          >
            <RefreshCw
              className={`w-4 h-4 ${applicationsLoading ? 'animate-spin' : ''}`}
            />
            Refresh
          </button>
        </div>
      </div>

      {/* Filter Tabs */}
      <div className="flex gap-2 mb-6 border-b" style={{ borderColor: 'var(--panel-border)' }}>
        {(['all', 'pending', 'approved', 'rejected'] as const).map((status) => (
          <button
            key={status}
            onClick={() => handleFilterChange(status)}
            className={`px-4 py-2 text-sm font-semibold border-b-2 transition-colors ${
              filterStatus === status
                ? 'border-[var(--green-primary)] text-[var(--green-primary)]'
                : 'border-transparent'
            }`}
            style={{
              color:
                filterStatus === status
                  ? 'var(--green-primary)'
                  : 'var(--text-secondary)',
            }}
          >
            {status === 'all'
              ? 'All'
              : status === 'pending'
              ? 'Pending'
              : status === 'approved'
              ? 'Approved'
              : 'Rejected'}
            {status !== 'all' &&
              applications &&
              ` (${applications.filter((a) => a.status === status).length})`}
          </button>
        ))}
      </div>

      {/* Applications List */}
      {applicationsLoading ? (
        <div className="flex items-center justify-center py-12">
          <RefreshCw className="w-8 h-8 animate-spin text-[var(--green-primary)]" />
        </div>
      ) : filteredApplications.length === 0 ? (
        <div
          className="text-center py-12 rounded-lg border"
          style={{
            background: 'var(--panel-bg)',
            borderColor: 'var(--panel-border)',
          }}
        >
          <p style={{ color: 'var(--text-secondary)' }}>
            {searchQuery
              ? 'No applications found matching your search'
              : filterStatus === 'all'
              ? 'No applications'
              : `No ${filterStatus === 'pending' ? 'pending' : filterStatus === 'approved' ? 'approved' : 'rejected'} applications`}
          </p>
        </div>
      ) : (
        <>
          <div className="space-y-4">
            {paginatedApplications.map((app) => {
            const isExpanded = expandedApps.has(app.id)
            const isApproving = approvingApps.has(app.id)
            const isRejecting = rejectingApps.has(app.id)
            const canAction = app.status === 'pending' && !isApproving && !isRejecting

            return (
              <div
                key={app.id}
                className="rounded-lg border p-4"
                style={{
                  background: 'var(--panel-bg)',
                  borderColor: 'var(--panel-border)',
                }}
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center gap-3 mb-2">
                      <h3 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
                        {app.name}
                      </h3>
                      {getStatusBadge(app.status)}
                    </div>
                    <div className="text-sm space-y-1" style={{ color: 'var(--text-secondary)' }}>
                      <p>
                        <span className="font-semibold">User:</span> {app.user_email || app.user_id}
                      </p>
                      <p>
                        <span className="font-semibold">Communication Email:</span> {app.email}
                      </p>
                      <p>
                        <span className="font-semibold">Submitted:</span>{' '}
                        {new Date(app.created_at).toLocaleString()}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    {(canAction || isApproving || isRejecting) && (
                      <>
                        {(app.status === 'pending' || isApproving) && (
                          <button
                            onClick={() => handleApprove(app.id)}
                            disabled={isApproving || app.status !== 'pending'}
                            className="px-4 py-2 rounded-lg text-sm font-semibold transition-opacity disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                            style={{
                              background: 'var(--green-primary)',
                              color: 'var(--navy-primary)',
                            }}
                          >
                            {isApproving ? (
                              <>
                                <RefreshCw className="w-4 h-4 animate-spin" />
                                Approving...
                              </>
                            ) : (
                              <>
                                <CheckCircle2 className="w-4 h-4" />
                                Approve
                              </>
                            )}
                          </button>
                        )}
                        {(app.status === 'pending' || isRejecting) && (
                          <button
                            onClick={() => setShowRejectModal(app.id)}
                            disabled={isRejecting || app.status !== 'pending'}
                            className="px-4 py-2 rounded-lg text-sm font-semibold transition-opacity disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2 border"
                            style={{
                              background: 'var(--panel-bg)',
                              borderColor: 'var(--panel-border)',
                              color: 'var(--text-primary)',
                            }}
                          >
                            <XCircle className="w-4 h-4" />
                            Reject
                          </button>
                        )}
                      </>
                    )}
                    <button
                      onClick={() => toggleExpand(app.id)}
                      className="p-2 rounded-lg border transition-colors"
                      style={{
                        background: 'var(--panel-bg)',
                        borderColor: 'var(--panel-border)',
                        color: 'var(--text-primary)',
                      }}
                    >
                      {isExpanded ? (
                        <ChevronUp className="w-4 h-4" />
                      ) : (
                        <ChevronDown className="w-4 h-4" />
                      )}
                    </button>
                  </div>
                </div>

                {isExpanded && (
                  <div className="mt-4 pt-4 border-t space-y-4" style={{ borderColor: 'var(--panel-border)' }}>
                    <div>
                      <h4 className="text-sm font-semibold mb-2" style={{ color: 'var(--text-primary)' }}>
                        Trader Description
                      </h4>
                      <p className="text-sm whitespace-pre-wrap" style={{ color: 'var(--text-secondary)' }}>
                        {app.description}
                      </p>
                    </div>
                    <div>
                      <h4 className="text-sm font-semibold mb-2" style={{ color: 'var(--text-primary)' }}>
                        Trading Experience
                      </h4>
                      <p className="text-sm whitespace-pre-wrap" style={{ color: 'var(--text-secondary)' }}>
                        {app.trading_experience}
                      </p>
                    </div>
                    <div>
                      <h4 className="text-sm font-semibold mb-2" style={{ color: 'var(--text-primary)' }}>
                        Strategy Overview
                      </h4>
                      <p className="text-sm whitespace-pre-wrap" style={{ color: 'var(--text-secondary)' }}>
                        {app.strategy_overview}
                      </p>
                    </div>
                    {app.social_links && Object.keys(app.social_links).length > 0 && (
                      <div>
                        <h4 className="text-sm font-semibold mb-2" style={{ color: 'var(--text-primary)' }}>
                          Social Links
                        </h4>
                        <div className="flex flex-wrap gap-2">
                          {Object.entries(app.social_links).map(([key, value]) => (
                            <a
                              key={key}
                              href={value}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-sm px-3 py-1 rounded border transition-colors hover:opacity-80"
                              style={{
                                background: 'var(--panel-bg)',
                                borderColor: 'var(--panel-border)',
                                color: 'var(--green-primary)',
                              }}
                            >
                              {key}: {value}
                            </a>
                          ))}
                        </div>
                      </div>
                    )}
                    {app.admin_notes && (
                      <div>
                        <h4 className="text-sm font-semibold mb-2" style={{ color: 'var(--text-primary)' }}>
                          Admin Notes
                        </h4>
                        <p className="text-sm whitespace-pre-wrap" style={{ color: 'var(--text-secondary)' }}>
                          {app.admin_notes}
                        </p>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )
            })}
          </div>
          
          {/* Pagination */}
          {totalPages > 1 && (
            <div className="flex items-center justify-between mt-6 px-4 py-3 rounded-lg border" style={{ 
              background: 'var(--panel-bg)',
              borderColor: 'var(--panel-border)',
            }}>
              <div className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                Showing {(currentPage - 1) * applicationsPerPage + 1} to {Math.min(currentPage * applicationsPerPage, filteredApplications.length)} of {filteredApplications.length} applications
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setCurrentPage((prev) => Math.max(1, prev - 1))}
                  disabled={currentPage === 1}
                  className="px-3 py-1.5 rounded border text-sm font-semibold transition-all disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1"
                  style={{
                    background: 'var(--panel-bg)',
                    borderColor: 'var(--panel-border)',
                    color: 'var(--text-primary)',
                  }}
                >
                  <ChevronLeft className="w-4 h-4" />
                  Previous
                </button>
                <div className="flex items-center gap-1">
                  {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                    let pageNum: number
                    if (totalPages <= 5) {
                      pageNum = i + 1
                    } else if (currentPage <= 3) {
                      pageNum = i + 1
                    } else if (currentPage >= totalPages - 2) {
                      pageNum = totalPages - 4 + i
                    } else {
                      pageNum = currentPage - 2 + i
                    }
                    return (
                      <button
                        key={pageNum}
                        onClick={() => setCurrentPage(pageNum)}
                        className={`px-3 py-1.5 rounded text-sm font-semibold transition-all ${
                          currentPage === pageNum ? 'border-2' : 'border'
                        }`}
                        style={{
                          background: currentPage === pageNum ? 'var(--green-primary)' : 'var(--panel-bg)',
                          borderColor: currentPage === pageNum ? 'var(--green-primary)' : 'var(--panel-border)',
                          color: currentPage === pageNum ? 'var(--navy-primary)' : 'var(--text-primary)',
                        }}
                      >
                        {pageNum}
                      </button>
                    )
                  })}
                </div>
                <button
                  onClick={() => setCurrentPage((prev) => Math.min(totalPages, prev + 1))}
                  disabled={currentPage === totalPages}
                  className="px-3 py-1.5 rounded border text-sm font-semibold transition-all disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1"
                  style={{
                    background: 'var(--panel-bg)',
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
      )}

      {/* Reject Modal */}
      {showRejectModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div
            className="max-w-md w-full rounded-lg p-6 border"
            style={{
              background: 'var(--panel-bg)',
              borderColor: 'var(--panel-border)',
            }}
          >
            <h3 className="text-lg font-semibold mb-4" style={{ color: 'var(--text-primary)' }}>
              Reject Application
            </h3>
            <p className="text-sm mb-4" style={{ color: 'var(--text-secondary)' }}>
              Please provide rejection reason (required)
            </p>
            <textarea
              value={adminNotes[showRejectModal] || ''}
              onChange={(e) =>
                setAdminNotes({ ...adminNotes, [showRejectModal]: e.target.value })
              }
              rows={4}
              className="w-full px-4 py-2 rounded-lg border resize-none mb-4"
              style={{
                background: 'var(--input-bg)',
                borderColor: 'var(--input-border)',
                color: 'var(--text-primary)',
              }}
              placeholder="Enter rejection reason..."
            />
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => {
                  setShowRejectModal(null)
                  setAdminNotes((prev) => {
                    const next = { ...prev }
                    delete next[showRejectModal]
                    return next
                  })
                }}
                className="px-4 py-2 rounded-lg border font-semibold transition-colors"
                style={{
                  background: 'var(--panel-bg)',
                  borderColor: 'var(--panel-border)',
                  color: 'var(--text-primary)',
                }}
              >
                Cancel
              </button>
              <button
                onClick={() => handleReject(showRejectModal)}
                disabled={!adminNotes[showRejectModal]?.trim() || rejectingApps.has(showRejectModal)}
                className="px-4 py-2 rounded-lg font-semibold text-white transition-opacity disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                style={{ background: '#ef4444' }}
              >
                {rejectingApps.has(showRejectModal) ? (
                  <>
                    <RefreshCw className="w-4 h-4 animate-spin" />
                    Rejecting...
                  </>
                ) : (
                  <>
                    <XCircle className="w-4 h-4" />
                    Confirm Reject
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

