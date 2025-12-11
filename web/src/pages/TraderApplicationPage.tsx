import React, { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth, isFollower } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { api } from '../lib/api'
import { toast } from 'sonner'
import type { TraderApplication, CreateTraderApplicationRequest, SocialLinks } from '../types'
import { Loader2, CheckCircle2, XCircle, Clock } from 'lucide-react'

export default function TraderApplicationPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [formData, setFormData] = useState<CreateTraderApplicationRequest>({
    name: '',
    email: '',
    description: '',
    trading_experience: '',
    strategy_overview: '',
    social_links: {},
  })
  const [socialLinks, setSocialLinks] = useState<SocialLinks>({
    twitter: '',
    telegram: '',
    discord: '',
    website: '',
  })
  const [loading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [existingApplication, setExistingApplication] = useState<TraderApplication | null>(null)
  const [loadingApplication, setLoadingApplication] = useState(true)

  // Redirect if not follower
  useEffect(() => {
    if (user && !isFollower(user)) {
      navigate('/traders')
    }
  }, [user, navigate])

  // Load existing application
  useEffect(() => {
    if (user && isFollower(user)) {
      loadApplication()
    }
  }, [user])

  const loadApplication = async () => {
    try {
      setLoadingApplication(true)
      const app = await api.getMyTraderApplication()
      setExistingApplication(app)
      if (app) {
        setFormData({
          name: app.name,
          email: app.email,
          description: app.description,
          trading_experience: app.trading_experience,
          strategy_overview: app.strategy_overview,
          social_links: app.social_links || {},
        })
        setSocialLinks(app.social_links || {
          twitter: '',
          telegram: '',
          discord: '',
          website: '',
        })
      }
    } catch (error: any) {
      console.error('Failed to load application:', error)
      toast.error(error.message || 'Failed to load application information')
    } finally {
      setLoadingApplication(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitting(true)

    try {
      // Validate required fields
      if (!formData.name.trim()) {
        toast.error('Please enter trader name')
        return
      }
      if (!formData.email.trim()) {
        toast.error('Please enter communication email')
        return
      }
      // Validate email format
      const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
      if (!emailRegex.test(formData.email.trim())) {
        toast.error('Please enter a valid email address')
        return
      }
      if (!formData.description.trim()) {
        toast.error('Please enter trader description')
        return
      }
      if (!formData.trading_experience.trim()) {
        toast.error('Please enter trading experience')
        return
      }
      if (!formData.strategy_overview.trim()) {
        toast.error('Please enter strategy overview')
        return
      }

      // Filter out empty social links
      const filteredSocialLinks: SocialLinks = {}
      Object.entries(socialLinks).forEach(([key, value]) => {
        if (value && value.trim()) {
          filteredSocialLinks[key] = value.trim()
        }
      })

      const request: CreateTraderApplicationRequest = {
        ...formData,
        social_links: Object.keys(filteredSocialLinks).length > 0 ? filteredSocialLinks : undefined,
      }

      await api.createTraderApplication(request)
      toast.success('Application submitted, waiting for admin review')
      await loadApplication()
    } catch (error: any) {
      console.error('Failed to submit application:', error)
      toast.error(error.message || 'Failed to submit application')
    } finally {
      setSubmitting(false)
    }
  }

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'pending':
        return (
          <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-semibold bg-green-500/20 text-green-400 border border-green-500/30">
            <Clock className="w-3 h-3" />
            Pending
          </span>
        )
      case 'approved':
        return (
          <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-semibold bg-green-500/20 text-green-400 border border-green-500/30">
            <CheckCircle2 className="w-3 h-3" />
            Approved
          </span>
        )
      case 'rejected':
        return (
          <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-semibold bg-red-500/20 text-red-400 border border-red-500/30">
            <XCircle className="w-3 h-3" />
            Rejected
          </span>
        )
      default:
        return null
    }
  }

  if (!user || !isFollower(user)) {
    return null
  }

  if (loadingApplication) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <Loader2 className="w-8 h-8 animate-spin text-[var(--green-primary)]" />
      </div>
    )
  }

  return (
    <div className="max-w-4xl mx-auto px-4 py-8">
      <div className="mb-6">
        <h1 className="text-3xl font-bold mb-2" style={{ color: 'var(--text-primary)' }}>
          Become a Trader
        </h1>
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          Fill in the following information to apply to become a trader and let other users follow your trading strategies
        </p>
      </div>

      {existingApplication && (
        <div
          className="mb-6 p-4 rounded-lg border"
          style={{
            background: 'var(--navy-dark)',
            borderColor: 'var(--panel-border)',
          }}
        >
          <div className="flex items-center justify-between mb-2">
            <h3 className="font-semibold" style={{ color: 'var(--text-primary)' }}>
              Current Application Status
            </h3>
            {getStatusBadge(existingApplication.status)}
          </div>
          {existingApplication.admin_notes && (
            <div className="mt-3 p-3 rounded" style={{ background: 'var(--navy-dark)', border: '1px solid var(--panel-border)' }}>
            <p className="text-sm font-semibold mb-1" style={{ color: 'var(--text-secondary)' }}>
              Admin Notes:
            </p>
            <p className="text-sm" style={{ color: 'var(--text-primary)' }}>
              {existingApplication.admin_notes}
            </p>
          </div>
          )}
          <p className="text-xs mt-2" style={{ color: 'var(--text-secondary)' }}>
            Submitted: {new Date(existingApplication.created_at).toLocaleString()}
          </p>
        </div>
      )}

      <form onSubmit={handleSubmit}>
        <div
          className="rounded-lg p-6 space-y-6"
          style={{
            background: 'var(--navy-dark)',
            border: '1px solid var(--panel-border)',
          }}
        >
          {/* Trader Name */}
          <div>
            <label
              className="block text-sm font-semibold mb-2"
              style={{ color: 'var(--text-primary)' }}
            >
              Trader Name <span className="text-red-500">*</span>
            </label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              disabled={existingApplication?.status === 'pending' || existingApplication?.status === 'approved' || submitting}
              className="w-full px-4 py-2 rounded-lg border"
              style={{
                background: 'var(--input-bg)',
                borderColor: 'var(--input-border)',
                color: 'var(--text-primary)',
              }}
              placeholder="Enter your trader display name"
              required
            />
          </div>

          {/* Communication Email */}
          <div>
            <label
              className="block text-sm font-semibold mb-2"
              style={{ color: 'var(--text-primary)' }}
            >
              Communication Email <span className="text-red-500">*</span>
            </label>
            <input
              type="email"
              value={formData.email}
              onChange={(e) => setFormData({ ...formData, email: e.target.value })}
              disabled={existingApplication?.status === 'pending' || existingApplication?.status === 'approved' || submitting}
              className="w-full px-4 py-2 rounded-lg border"
              style={{
                background: 'var(--input-bg)',
                borderColor: 'var(--input-border)',
                color: 'var(--text-primary)',
              }}
              placeholder="Enter your communication email address"
              required
            />
          </div>

          {/* Description */}
          <div>
            <label
              className="block text-sm font-semibold mb-2"
              style={{ color: 'var(--text-primary)' }}
            >
              Trader Description <span className="text-red-500">*</span>
            </label>
            <textarea
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              disabled={existingApplication?.status === 'pending' || existingApplication?.status === 'approved' || submitting}
              rows={4}
              className="w-full px-4 py-2 rounded-lg border resize-none"
              style={{
                background: 'var(--input-bg)',
                borderColor: 'var(--input-border)',
                color: 'var(--text-primary)',
              }}
              placeholder="Describe your trading style, areas of expertise, etc."
              required
            />
          </div>

          {/* Trading Experience */}
          <div>
            <label
              className="block text-sm font-semibold mb-2"
              style={{ color: 'var(--text-primary)' }}
            >
              Trading Experience <span className="text-red-500">*</span>
            </label>
            <textarea
              value={formData.trading_experience}
              onChange={(e) =>
                setFormData({ ...formData, trading_experience: e.target.value })
              }
              disabled={existingApplication?.status === 'pending' || existingApplication?.status === 'approved' || submitting}
              rows={5}
              className="w-full px-4 py-2 rounded-lg border resize-none"
              style={{
                background: 'var(--input-bg)',
                borderColor: 'var(--input-border)',
                color: 'var(--text-primary)',
              }}
              placeholder="Please describe your trading experience in detail, including years of trading, main trading pairs, past performance, etc."
              required
            />
          </div>

          {/* Strategy Overview */}
          <div>
            <label
              className="block text-sm font-semibold mb-2"
              style={{ color: 'var(--text-primary)' }}
            >
              Strategy Overview <span className="text-red-500">*</span>
            </label>
            <textarea
              value={formData.strategy_overview}
              onChange={(e) =>
                setFormData({ ...formData, strategy_overview: e.target.value })
              }
              disabled={existingApplication?.status === 'pending' || existingApplication?.status === 'approved' || submitting}
              rows={5}
              className="w-full px-4 py-2 rounded-lg border resize-none"
              style={{
                background: 'var(--input-bg)',
                borderColor: 'var(--input-border)',
                color: 'var(--text-primary)',
              }}
              placeholder="Describe your trading strategy, including trading philosophy, risk control methods, signal generation logic, etc."
              required
            />
          </div>

          {/* Social Links */}
          <div>
            <label
              className="block text-sm font-semibold mb-2"
              style={{ color: 'var(--text-primary)' }}
            >
              Social Links (Optional)
            </label>
            <div className="space-y-3">
              <div>
                <label className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>
                  Twitter
                </label>
                <input
                  type="url"
                  value={socialLinks.twitter || ''}
                  onChange={(e) =>
                    setSocialLinks({ ...socialLinks, twitter: e.target.value })
                  }
                  disabled={existingApplication?.status === 'pending' || existingApplication?.status === 'approved' || submitting}
                  className="w-full px-4 py-2 rounded-lg border"
                  style={{
                    background: 'var(--input-bg)',
                    borderColor: 'var(--input-border)',
                    color: 'var(--text-primary)',
                  }}
                  placeholder="https://twitter.com/yourusername"
                />
              </div>
              <div>
                <label className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>
                  Telegram
                </label>
                <input
                  type="url"
                  value={socialLinks.telegram || ''}
                  onChange={(e) =>
                    setSocialLinks({ ...socialLinks, telegram: e.target.value })
                  }
                  disabled={existingApplication?.status === 'pending' || existingApplication?.status === 'approved' || submitting}
                  className="w-full px-4 py-2 rounded-lg border"
                  style={{
                    background: 'var(--input-bg)',
                    borderColor: 'var(--input-border)',
                    color: 'var(--text-primary)',
                  }}
                  placeholder="https://t.me/yourusername"
                />
              </div>
              <div>
                <label className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>
                  Discord
                </label>
                <input
                  type="text"
                  value={socialLinks.discord || ''}
                  onChange={(e) =>
                    setSocialLinks({ ...socialLinks, discord: e.target.value })
                  }
                  disabled={existingApplication?.status === 'pending' || existingApplication?.status === 'approved' || submitting}
                  className="w-full px-4 py-2 rounded-lg border"
                  style={{
                    background: 'var(--input-bg)',
                    borderColor: 'var(--input-border)',
                    color: 'var(--text-primary)',
                  }}
                  placeholder="Discord username or server link"
                />
              </div>
              <div>
                <label className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>
                  Personal Website
                </label>
                <input
                  type="url"
                  value={socialLinks.website || ''}
                  onChange={(e) =>
                    setSocialLinks({ ...socialLinks, website: e.target.value })
                  }
                  disabled={existingApplication?.status === 'pending' || existingApplication?.status === 'approved' || submitting}
                  className="w-full px-4 py-2 rounded-lg border"
                  style={{
                    background: 'var(--input-bg)',
                    borderColor: 'var(--input-border)',
                    color: 'var(--text-primary)',
                  }}
                  placeholder="https://yourwebsite.com"
                />
              </div>
            </div>
          </div>

          {/* Submit Button */}
          {existingApplication?.status !== 'approved' && (
            <div className="flex gap-3 pt-4">
              <button
                type="button"
                onClick={() => navigate('/traders')}
                className="px-6 py-2 rounded-lg border font-semibold transition-colors"
                style={{
                  background: 'var(--navy-dark)',
                  borderColor: 'var(--panel-border)',
                  color: 'var(--text-primary)',
                }}
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={
                  existingApplication?.status === 'pending' || submitting || loading
                }
                className="px-6 py-2 rounded-lg font-semibold transition-opacity disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                style={{
                  background: 'var(--green-primary)',
                  color: 'var(--navy-primary)',
                }}
              >
                {submitting ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" />
                    Submitting...
                  </>
                ) : existingApplication?.status === 'pending' ? (
                  'Submitted, Waiting for Review'
                ) : (
                  'Submit Application'
                )}
              </button>
            </div>
          )}
          {existingApplication?.status === 'approved' && (
            <div className="pt-4">
              <div
                className="p-4 rounded-lg border"
                style={{
                  background: 'rgba(16, 185, 129, 0.1)',
                  borderColor: 'rgba(16, 185, 129, 0.3)',
                }}
              >
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="w-5 h-5" style={{ color: '#10B981' }} />
                  <p className="font-semibold" style={{ color: '#10B981' }}>
                    Your application has been approved! Your role has been upgraded to trader.
                  </p>
                </div>
              </div>
            </div>
          )}
        </div>
      </form>
    </div>
  )
}

