import { useState, useEffect, useRef } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { motion } from 'framer-motion'
import { Menu, X, ChevronDown } from 'lucide-react'
import { t, type Language } from '../i18n/translations'
import { Container } from './Container'
import { useSystemConfig } from '../hooks/useSystemConfig'
import { useAuth, isFollower, isAdmin } from '../contexts/AuthContext'
import type { Page } from '../types'

interface HeaderBarProps {
  onLoginClick?: () => void
  isLoggedIn?: boolean
  isHomePage?: boolean
  currentPage?: Page
  language?: Language
  user?: { email: string } | null
  onLogout?: () => void
  onPageChange?: (page: Page) => void
}

export default function HeaderBar({
  isLoggedIn = false,
  isHomePage = false,
  currentPage,
  language = 'en' as Language,
  user,
  onLogout,
  onPageChange,
}: HeaderBarProps) {
  const navigate = useNavigate()
  const { user: authUser } = useAuth()
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [userDropdownOpen, setUserDropdownOpen] = useState(false)
  const userDropdownRef = useRef<HTMLDivElement>(null)
  const { config: systemConfig } = useSystemConfig()
  const registrationEnabled = systemConfig?.registration_enabled !== false
  const userIsFollower = isFollower(authUser)
  const userIsAdmin = isAdmin(authUser)

  // Close dropdown when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        userDropdownRef.current &&
        !userDropdownRef.current.contains(event.target as Node)
      ) {
        setUserDropdownOpen(false)
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [])

  return (
    <nav className="fixed top-0 w-full z-50 header-bar">
      <Container className="flex items-center justify-between h-16">
        {/* Logo */}
        <Link
          to="/"
          className="flex items-center gap-3 hover:opacity-80 transition-opacity cursor-pointer"
        >
          <img src="/icons/nofx.svg" alt="AI Trading 24x7 Logo" className="w-8 h-8" width="32" height="32" />
          <span
            className="text-xl font-bold"
            style={{ color: 'var(--brand-yellow)' }}
          >
            AI Trading 24x7
          </span>
        </Link>

        {/* Desktop Menu */}
        <div className="hidden md:flex items-center justify-between flex-1 ml-8">
          {/* Left Side - Navigation Tabs */}
          <div className="flex items-center gap-4">
            {isLoggedIn ? (
              // Main app navigation when logged in
              <>
                <button
                  key="competition-tab"
                  onClick={() => {
                    if (onPageChange) {
                      onPageChange('competition')
                    }
                    navigate('/competition')
                  }}
                  className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500"
                  style={{
                    color:
                      currentPage === 'competition'
                        ? 'var(--brand-yellow)'
                        : 'var(--brand-light-gray)',
                    padding: '8px 16px',
                    borderRadius: '8px',
                    position: 'relative',
                  }}
                  onMouseEnter={(e) => {
                    if (currentPage !== 'competition') {
                      e.currentTarget.style.color = 'var(--brand-yellow)'
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (currentPage !== 'competition') {
                      e.currentTarget.style.color = 'var(--brand-light-gray)'
                    }
                  }}
                >
                  {/* Background for selected state */}
                  <span
                    className="absolute inset-0 rounded-lg transition-opacity duration-300"
                    style={{
                      background: 'rgba(0, 51, 102, 0.3)',
                      zIndex: -1,
                      opacity: currentPage === 'competition' ? 1 : 0,
                      pointerEvents: 'none',
                    }}
                  />

                  {t('realtimeNav', language)}
                </button>

                <button
                  key="traders-tab"
                  onClick={() => {
                    if (onPageChange) {
                      onPageChange('traders')
                    }
                    navigate('/traders')
                  }}
                  className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500"
                  style={{
                    color:
                      currentPage === 'traders'
                        ? 'var(--brand-yellow)'
                        : 'var(--brand-light-gray)',
                    padding: '8px 16px',
                    borderRadius: '8px',
                    position: 'relative',
                  }}
                  onMouseEnter={(e) => {
                    if (currentPage !== 'traders') {
                      e.currentTarget.style.color = 'var(--brand-yellow)'
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (currentPage !== 'traders') {
                      e.currentTarget.style.color = 'var(--brand-light-gray)'
                    }
                  }}
                >
                  {/* Background for selected state */}
                  <span
                    className="absolute inset-0 rounded-lg transition-opacity duration-300"
                    style={{
                      background: 'rgba(0, 51, 102, 0.3)',
                      zIndex: -1,
                      opacity: currentPage === 'traders' ? 1 : 0,
                      pointerEvents: 'none',
                    }}
                  />

                  {t('configNav', language)}
                </button>

                <button
                  key="strategy-studio-tab"
                  onClick={() => {
                    if (onPageChange) {
                      onPageChange('strategy-studio')
                    }
                    navigate('/strategy-studio')
                  }}
                  className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500"
                  style={{
                    color:
                      currentPage === 'strategy-studio'
                        ? 'var(--brand-yellow)'
                        : 'var(--brand-light-gray)',
                    padding: '8px 16px',
                    borderRadius: '8px',
                    position: 'relative',
                  }}
                  onMouseEnter={(e) => {
                    if (currentPage !== 'strategy-studio') {
                      e.currentTarget.style.color = 'var(--brand-yellow)'
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (currentPage !== 'strategy-studio') {
                      e.currentTarget.style.color = 'var(--brand-light-gray)'
                    }
                  }}
                >
                  <span
                    className="absolute inset-0 rounded-lg transition-opacity duration-300"
                    style={{
                      background: 'rgba(0, 51, 102, 0.3)',
                      zIndex: -1,
                      opacity: currentPage === 'strategy-studio' ? 1 : 0,
                      pointerEvents: 'none',
                    }}
                  />
                  {language === 'zh' ? '策略' : 'Strategy'}
                </button>

                <button
                  key="trader-tab"
                  onClick={() => {
                    if (onPageChange) {
                      onPageChange('trader')
                    }
                    navigate('/dashboard')
                  }}
                  className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500"
                  style={{
                    color:
                      currentPage === 'trader'
                        ? 'var(--brand-yellow)'
                        : 'var(--brand-light-gray)',
                    padding: '8px 16px',
                    borderRadius: '8px',
                    position: 'relative',
                  }}
                  onMouseEnter={(e) => {
                    if (currentPage !== 'trader') {
                      e.currentTarget.style.color = 'var(--brand-yellow)'
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (currentPage !== 'trader') {
                      e.currentTarget.style.color = 'var(--brand-light-gray)'
                    }
                  }}
                >
                  {/* Background for selected state */}
                  <span
                    className="absolute inset-0 rounded-lg transition-opacity duration-300"
                    style={{
                      background: 'rgba(0, 51, 102, 0.3)',
                      zIndex: -1,
                      opacity: currentPage === 'trader' ? 1 : 0,
                      pointerEvents: 'none',
                    }}
                  />

                  {t('dashboardNav', language)}
                </button>

                {userIsAdmin && (
                  <>
                    <button
                      key="stats-tab"
                      onClick={() => {
                        if (onPageChange) {
                          onPageChange('stats')
                        }
                        navigate('/stats')
                      }}
                      className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500"
                      style={{
                        color:
                          currentPage === 'stats'
                            ? 'var(--brand-yellow)'
                            : 'var(--brand-light-gray)',
                        padding: '8px 16px',
                        borderRadius: '8px',
                        position: 'relative',
                      }}
                      onMouseEnter={(e) => {
                        if (currentPage !== 'stats') {
                          e.currentTarget.style.color = 'var(--brand-yellow)'
                        }
                      }}
                      onMouseLeave={(e) => {
                        if (currentPage !== 'stats') {
                          e.currentTarget.style.color = 'var(--brand-light-gray)'
                        }
                      }}
                    >
                      {/* Background for selected state */}
                      <span
                        className="absolute inset-0 rounded-lg transition-opacity duration-300"
                        style={{
                          background: 'rgba(0, 51, 102, 0.3)',
                          zIndex: -1,
                          opacity: currentPage === 'stats' ? 1 : 0,
                          pointerEvents: 'none',
                        }}
                      />

                      Stats
                    </button>
                    <button
                      key="trader-applications-tab"
                      onClick={() => {
                        if (onPageChange) {
                          onPageChange('applications')
                        }
                        navigate('/admin/trader-applications')
                      }}
                      className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500"
                      style={{
                        color:
                          currentPage === 'applications'
                            ? 'var(--brand-yellow)'
                            : 'var(--brand-light-gray)',
                        padding: '8px 16px',
                        borderRadius: '8px',
                        position: 'relative',
                      }}
                      onMouseEnter={(e) => {
                        if (currentPage !== 'applications') {
                          e.currentTarget.style.color = 'var(--brand-yellow)'
                        }
                      }}
                      onMouseLeave={(e) => {
                        if (currentPage !== 'applications') {
                          e.currentTarget.style.color = 'var(--brand-light-gray)'
                        }
                      }}
                    >
                      <span
                        className="absolute inset-0 rounded-lg transition-opacity duration-300"
                        style={{
                          background: 'rgba(0, 51, 102, 0.3)',
                          zIndex: -1,
                          opacity: currentPage === 'applications' ? 1 : 0,
                          pointerEvents: 'none',
                        }}
                      />

                      Applications
                    </button>
                    <button
                      key="articles-tab"
                      onClick={() => {
                        if (onPageChange) {
                          onPageChange('articles')
                        }
                        navigate('/admin/articles')
                      }}
                      className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500"
                      style={{
                        color:
                          currentPage === 'articles'
                            ? 'var(--brand-yellow)'
                            : 'var(--brand-light-gray)',
                        padding: '8px 16px',
                        borderRadius: '8px',
                        position: 'relative',
                      }}
                      onMouseEnter={(e) => {
                        if (currentPage !== 'articles') {
                          e.currentTarget.style.color = 'var(--brand-yellow)'
                        }
                      }}
                      onMouseLeave={(e) => {
                        if (currentPage !== 'articles') {
                          e.currentTarget.style.color = 'var(--brand-light-gray)'
                        }
                      }}
                    >
                      <span
                        className="absolute inset-0 rounded-lg transition-opacity duration-300"
                        style={{
                          background: 'rgba(0, 51, 102, 0.3)',
                          zIndex: -1,
                          opacity: currentPage === 'articles' ? 1 : 0,
                          pointerEvents: 'none',
                        }}
                      />
                      Articles
                    </button>
                  </>
                )}

                {userIsFollower && (
                  <button
                    key="become-trader-tab"
                    onClick={() => {
                      navigate('/become-trader')
                    }}
                    className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500"
                    style={{
                      color: 'var(--brand-light-gray)',
                      padding: '8px 16px',
                      borderRadius: '8px',
                      position: 'relative',
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.color = 'var(--brand-yellow)'
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.color = 'var(--brand-light-gray)'
                    }}
                  >
                    Become a Trader
                  </button>
                )}

                {!userIsFollower && (
                  <>
                    <button
                      key="followers-tab"
                      onClick={() => {
                        if (onPageChange) {
                          onPageChange('followers')
                        }
                        navigate('/followers')
                      }}
                      className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500"
                      style={{
                        color:
                          currentPage === 'followers'
                            ? 'var(--brand-yellow)'
                            : 'var(--brand-light-gray)',
                        padding: '8px 16px',
                        borderRadius: '8px',
                        position: 'relative',
                      }}
                      onMouseEnter={(e) => {
                        if (currentPage !== 'followers') {
                          e.currentTarget.style.color = 'var(--brand-yellow)'
                        }
                      }}
                      onMouseLeave={(e) => {
                        if (currentPage !== 'followers') {
                          e.currentTarget.style.color = 'var(--brand-light-gray)'
                        }
                      }}
                    >
                      {/* Background for selected state */}
                      <span
                        className="absolute inset-0 rounded-lg transition-opacity duration-300"
                        style={{
                          background: 'rgba(0, 51, 102, 0.3)',
                          zIndex: -1,
                          opacity: currentPage === 'followers' ? 1 : 0,
                          pointerEvents: 'none',
                        }}
                      />

                      Followers
                    </button>

                    <button
                      key="backtest-tab"
                      onClick={() => {
                        if (onPageChange) {
                          onPageChange('backtest')
                        }
                        navigate('/backtest')
                      }}
                      className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500"
                      style={{
                        color:
                          currentPage === 'backtest'
                            ? 'var(--brand-yellow)'
                            : 'var(--brand-light-gray)',
                        padding: '8px 16px',
                        borderRadius: '8px',
                        position: 'relative',
                      }}
                      onMouseEnter={(e) => {
                        if (currentPage !== 'backtest') {
                          e.currentTarget.style.color = 'var(--brand-yellow)'
                        }
                      }}
                      onMouseLeave={(e) => {
                        if (currentPage !== 'backtest') {
                          e.currentTarget.style.color = 'var(--brand-light-gray)'
                        }
                      }}
                    >
                      <span
                        className="absolute inset-0 rounded-lg transition-opacity duration-300"
                        style={{
                          background: 'rgba(0, 51, 102, 0.3)',
                          zIndex: -1,
                          opacity: currentPage === 'backtest' ? 1 : 0,
                          pointerEvents: 'none',
                        }}
                      />

                      Backtest
                    </button>

                    <button
                      key="webhook-tab"
                      onClick={() => {
                        if (onPageChange) {
                          onPageChange('webhook')
                        }
                        navigate('/webhook')
                      }}
                      className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500"
                      style={{
                        color:
                          currentPage === 'webhook'
                            ? 'var(--brand-yellow)'
                            : 'var(--brand-light-gray)',
                        padding: '8px 16px',
                        borderRadius: '8px',
                        position: 'relative',
                      }}
                      onMouseEnter={(e) => {
                        if (currentPage !== 'webhook') {
                          e.currentTarget.style.color = 'var(--brand-yellow)'
                        }
                      }}
                      onMouseLeave={(e) => {
                        if (currentPage !== 'webhook') {
                          e.currentTarget.style.color = 'var(--brand-light-gray)'
                        }
                      }}
                    >
                      <span
                        className="absolute inset-0 rounded-lg transition-opacity duration-300"
                        style={{
                          background: 'rgba(0, 51, 102, 0.3)',
                          zIndex: -1,
                          opacity: currentPage === 'webhook' ? 1 : 0,
                          pointerEvents: 'none',
                        }}
                      />

                      Webhook
                    </button>
                  </>
                )}
              </>
            ) : (
              // Show Live button for logged-out users on landing page
              isHomePage && (
                <button
                  key="live-tab-logged-out"
                  onClick={() => {
                    if (onPageChange) {
                      onPageChange('competition')
                    }
                    navigate('/competition')
                  }}
                  className="text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500"
                  style={{
                    color: 'var(--brand-yellow)',
                    padding: '8px 16px',
                    borderRadius: '8px',
                    position: 'relative',
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.color = 'var(--brand-yellow)'
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.color = 'var(--brand-yellow)'
                  }}
                >
                  {t('realtimeNav', language)}
                </button>
              )
            )}
          </div>

          {/* Right Side - Original Navigation Items and Login */}
          <div className="flex items-center gap-6">
            {/* Only show Features and Pricing when logged out and on home page */}
            {!isLoggedIn && isHomePage && (
              <>
                <a
                  href="#features"
                  className="text-sm transition-colors relative group"
                  style={{ color: 'var(--brand-light-gray)' }}
                >
                  {t('features', language)}
                  <span
                    className="absolute -bottom-1 left-0 w-0 h-0.5 group-hover:w-full transition-all duration-300"
                    style={{ background: 'var(--brand-yellow)' }}
                  />
                </a>
                <a
                  href="/pricing"
                  className="text-sm transition-colors relative group"
                  style={{ color: 'var(--brand-light-gray)' }}
                >
                  {t('pricingTitle', language)}
                  <span
                    className="absolute -bottom-1 left-0 w-0 h-0.5 group-hover:w-full transition-all duration-300"
                    style={{ background: 'var(--brand-yellow)' }}
                  />
                </a>
              </>
            )}

            {/* User Info and Actions */}
            {isLoggedIn && user ? (
              <div className="flex items-center gap-3">
                {/* User Info with Dropdown */}
                <div className="relative" ref={userDropdownRef}>
                  <button
                    onClick={() => setUserDropdownOpen(!userDropdownOpen)}
                    className="flex items-center gap-2 px-3 py-2 rounded transition-colors"
                    style={{
                      background: 'var(--navy-dark)',
                      border: '1px solid var(--panel-border)',
                    }}
                    onMouseEnter={(e) =>
                      (e.currentTarget.style.background =
                        'var(--navy-light)')
                    }
                    onMouseLeave={(e) =>
                      (e.currentTarget.style.background = 'var(--navy-dark)')
                    }
                  >
                    <div
                      className="w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold"
                      style={{
                        background: 'var(--brand-yellow)',
                        color: 'var(--navy-primary)',
                      }}
                    >
                      {user.email[0].toUpperCase()}
                    </div>
                    <span
                      className="text-sm"
                      style={{ color: 'var(--brand-light-gray)' }}
                    >
                      {user.email}
                    </span>
                    <ChevronDown
                      className="w-4 h-4"
                      style={{ color: 'var(--brand-light-gray)' }}
                    />
                  </button>

                  {userDropdownOpen && (
                    <div
                      className="absolute right-0 top-full mt-2 w-48 rounded-lg shadow-lg overflow-hidden z-50"
                      style={{
                        background: 'var(--navy-dark)',
                        border: '1px solid var(--panel-border)',
                      }}
                    >
                      <div
                        className="px-3 py-2 border-b"
                        style={{ borderColor: 'var(--panel-border)' }}
                      >
                        <div
                          className="text-xs"
                          style={{ color: 'var(--text-secondary)' }}
                        >
                          {t('loggedInAs', language)}
                        </div>
                        <div
                          className="text-sm font-medium"
                          style={{ color: 'var(--brand-light-gray)' }}
                        >
                          {user.email}
                        </div>
                      </div>
                      {onLogout && (
                        <button
                          onClick={() => {
                            onLogout()
                            setUserDropdownOpen(false)
                          }}
                          className="w-full px-3 py-2 text-sm font-semibold transition-colors hover:opacity-80 text-center"
                          style={{
                            background: 'var(--binance-red-bg)',
                            color: 'var(--binance-red)',
                          }}
                        >
                          {t('exitLogin', language)}
                        </button>
                      )}
                    </div>
                  )}
                </div>
              </div>
            ) : (
              /* Show login/register buttons when not logged in and not on login/register pages */
              currentPage !== 'login' &&
              currentPage !== 'register' && (
                <div className="flex items-center gap-3">
                  <a
                    href="/login"
                    className="px-3 py-2 text-sm font-medium transition-colors rounded"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {t('signIn', language)}
                  </a>
                  {registrationEnabled && (
                    <a
                      href="/register"
                      className="px-4 py-2 rounded font-semibold text-sm transition-colors hover:opacity-90"
                      style={{
                        background: 'var(--brand-yellow)',
                        color: 'var(--navy-primary)',
                      }}
                    >
                      {t('signUp', language)}
                    </a>
                  )}
                </div>
              )
            )}

          </div>
        </div>

        {/* Mobile Menu Button */}
        <motion.button
          onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
          className="sm:hidden"
          style={{ color: 'var(--brand-light-gray)' }}
          whileTap={{ scale: 0.9 }}
        >
          {mobileMenuOpen ? (
            <X className="w-6 h-6" />
          ) : (
            <Menu className="w-6 h-6" />
          )}
        </motion.button>
      </Container>

      {/* Mobile Menu */}
      <motion.div
        initial={false}
        animate={
          mobileMenuOpen
            ? { height: 'auto', opacity: 1 }
            : { height: 0, opacity: 0 }
        }
        transition={{ duration: 0.3 }}
        className="sm:hidden overflow-hidden"
        style={{
          background: 'var(--navy-dark)',
          borderTop: '1px solid rgba(0, 255, 127, 0.1)',
        }}
      >
        <div className="px-4 py-4 space-y-3">
          {/* New Navigation Tabs */}
          {isLoggedIn ? (
            <button
              key="mobile-competition-tab"
              onClick={() => {
                // Navigate using React Router (primary navigation method)
                navigate('/competition')
                // Call onPageChange if provided (for backward compatibility with App.tsx)
                if (onPageChange) {
                  onPageChange('competition')
                }
                // Close mobile menu after navigation
                setMobileMenuOpen(false)
              }}
              className="block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500"
              style={{
                color:
                  currentPage === 'competition'
                    ? 'var(--brand-yellow)'
                    : 'var(--brand-light-gray)',
                padding: '12px 16px',
                borderRadius: '8px',
                position: 'relative',
                width: '100%',
                textAlign: 'left',
              }}
            >
              {/* Background for selected state */}
              <span
                className="absolute inset-0 rounded-lg transition-opacity duration-300"
                style={{
                  background: 'rgba(0, 51, 102, 0.3)',
                  zIndex: -1,
                  opacity: currentPage === 'competition' ? 1 : 0,
                  pointerEvents: 'none',
                }}
              />

              {t('realtimeNav', language)}
            </button>
          ) : (
            <a
              key="mobile-competition-link"
              href="/competition"
              className="block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500"
              style={{
                color:
                  currentPage === 'competition'
                    ? 'var(--brand-yellow)'
                    : 'var(--brand-light-gray)',
                padding: '12px 16px',
                borderRadius: '8px',
                position: 'relative',
              }}
            >
              {/* Background for selected state */}
              <span
                className="absolute inset-0 rounded-lg transition-opacity duration-300"
                style={{
                  background: 'rgba(0, 51, 102, 0.3)',
                  zIndex: -1,
                  opacity: currentPage === 'competition' ? 1 : 0,
                  pointerEvents: 'none',
                }}
              />

              {t('realtimeNav', language)}
            </a>
          )}
          {/* Only show 配置 and 看板 when logged in */}
          {isLoggedIn && (
            <>
              <button
                key="mobile-traders-tab"
                onClick={() => {
                  if (onPageChange) {
                    onPageChange('traders')
                  }
                  navigate('/traders')
                  setMobileMenuOpen(false)
                }}
                className="block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500 hover:text-green-500"
                style={{
                  color:
                    currentPage === 'traders'
                      ? 'var(--brand-yellow)'
                      : 'var(--brand-light-gray)',
                  padding: '12px 16px',
                  borderRadius: '8px',
                  position: 'relative',
                  width: '100%',
                  textAlign: 'left',
                }}
              >
                {/* Background for selected state */}
                <span
                  className="absolute inset-0 rounded-lg transition-opacity duration-300"
                  style={{
                    background: 'rgba(0, 51, 102, 0.3)',
                    zIndex: -1,
                    opacity: currentPage === 'traders' ? 1 : 0,
                    pointerEvents: 'none',
                  }}
                />

                {t('configNav', language)}
              </button>
              <button
                key="mobile-strategy-studio-tab"
                onClick={() => {
                  if (onPageChange) {
                    onPageChange('strategy-studio')
                  }
                  navigate('/strategy-studio')
                  setMobileMenuOpen(false)
                }}
                className="block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500 hover:text-green-500"
                style={{
                  color:
                    currentPage === 'strategy-studio'
                      ? 'var(--brand-yellow)'
                      : 'var(--brand-light-gray)',
                  padding: '12px 16px',
                  borderRadius: '8px',
                  position: 'relative',
                  width: '100%',
                  textAlign: 'left',
                }}
              >
                <span
                  className="absolute inset-0 rounded-lg transition-opacity duration-300"
                  style={{
                    background: 'rgba(0, 51, 102, 0.3)',
                    zIndex: -1,
                    opacity: currentPage === 'strategy-studio' ? 1 : 0,
                    pointerEvents: 'none',
                  }}
                />
                {language === 'zh' ? '策略' : 'Strategy'}
              </button>
              <button
                key="mobile-trader-tab"
                onClick={() => {
                  if (onPageChange) {
                    onPageChange('trader')
                  }
                  navigate('/dashboard')
                  setMobileMenuOpen(false)
                }}
                className="block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500 hover:text-green-500"
                style={{
                  color:
                    currentPage === 'trader'
                      ? 'var(--brand-yellow)'
                      : 'var(--brand-light-gray)',
                  padding: '12px 16px',
                  borderRadius: '8px',
                  position: 'relative',
                  width: '100%',
                  textAlign: 'left',
                }}
              >
                {/* Background for selected state */}
                <span
                  className="absolute inset-0 rounded-lg transition-opacity duration-300"
                  style={{
                    background: 'rgba(0, 51, 102, 0.3)',
                    zIndex: -1,
                    opacity: currentPage === 'trader' ? 1 : 0,
                    pointerEvents: 'none',
                  }}
                />

                {t('dashboardNav', language)}
              </button>
              {userIsAdmin && (
                <>
                  <button
                    key="mobile-stats-tab"
                    onClick={() => {
                      if (onPageChange) {
                        onPageChange('stats')
                      }
                      navigate('/stats')
                      setMobileMenuOpen(false)
                    }}
                    className="block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500 hover:text-green-500"
                    style={{
                      color:
                        currentPage === 'stats'
                          ? 'var(--brand-yellow)'
                          : 'var(--brand-light-gray)',
                      padding: '12px 16px',
                      borderRadius: '8px',
                      position: 'relative',
                      width: '100%',
                      textAlign: 'left',
                    }}
                  >
                    <span
                      className="absolute inset-0 rounded-lg transition-opacity duration-300"
                      style={{
                        background: 'rgba(0, 51, 102, 0.3)',
                        zIndex: -1,
                        opacity: currentPage === 'stats' ? 1 : 0,
                        pointerEvents: 'none',
                      }}
                    />
                    Stats
                  </button>
                  <button
                    key="mobile-trader-applications-tab"
                    onClick={() => {
                      if (onPageChange) {
                        onPageChange('applications')
                      }
                      navigate('/admin/trader-applications')
                      setMobileMenuOpen(false)
                    }}
                    className="block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500 hover:text-green-500"
                    style={{
                      color:
                        currentPage === 'applications'
                          ? 'var(--brand-yellow)'
                          : 'var(--brand-light-gray)',
                      padding: '12px 16px',
                      borderRadius: '8px',
                      position: 'relative',
                      width: '100%',
                      textAlign: 'left',
                    }}
                  >
                    <span
                      className="absolute inset-0 rounded-lg transition-opacity duration-300"
                      style={{
                        background: 'rgba(0, 51, 102, 0.3)',
                        zIndex: -1,
                        opacity: currentPage === 'applications' ? 1 : 0,
                        pointerEvents: 'none',
                      }}
                    />
                    Applications
                  </button>
                  <button
                    key="mobile-articles-tab"
                    onClick={() => {
                      if (onPageChange) {
                        onPageChange('articles')
                      }
                      navigate('/admin/articles')
                      setMobileMenuOpen(false)
                    }}
                    className="block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500 hover:text-green-500"
                    style={{
                      color:
                        currentPage === 'articles'
                          ? 'var(--brand-yellow)'
                          : 'var(--brand-light-gray)',
                      padding: '12px 16px',
                      borderRadius: '8px',
                      position: 'relative',
                      width: '100%',
                      textAlign: 'left',
                    }}
                  >
                    <span
                      className="absolute inset-0 rounded-lg transition-opacity duration-300"
                      style={{
                        background: 'rgba(0, 51, 102, 0.3)',
                        zIndex: -1,
                        opacity: currentPage === 'articles' ? 1 : 0,
                        pointerEvents: 'none',
                      }}
                    />
                    Articles
                  </button>
                </>
              )}
              {userIsFollower && (
                <button
                  key="mobile-become-trader-tab"
                  onClick={() => {
                    navigate('/become-trader')
                    setMobileMenuOpen(false)
                  }}
                  className="block text-sm font-bold transition-all duration-300 hover:text-green-500"
                  style={{
                    color: 'var(--brand-light-gray)',
                    padding: '12px 16px',
                    borderRadius: '8px',
                    width: '100%',
                    textAlign: 'left',
                  }}
                >
                  Become a Trader
                </button>
              )}
              {!userIsFollower && (
                <button
                  key="mobile-webhook-tab"
                  onClick={() => {
                    if (onPageChange) {
                      onPageChange('webhook')
                    }
                    navigate('/webhook')
                    setMobileMenuOpen(false)
                  }}
                  className="block text-sm font-bold transition-all duration-300 relative focus:outline-2 focus:outline-green-500 hover:text-green-500"
                  style={{
                    color:
                      currentPage === 'webhook'
                        ? 'var(--brand-yellow)'
                        : 'var(--brand-light-gray)',
                    padding: '12px 16px',
                    borderRadius: '8px',
                    position: 'relative',
                    width: '100%',
                    textAlign: 'left',
                  }}
                >
                  <span
                    className="absolute inset-0 rounded-lg transition-opacity duration-300"
                    style={{
                      background: 'rgba(0, 51, 102, 0.3)',
                      zIndex: -1,
                      opacity: currentPage === 'webhook' ? 1 : 0,
                      pointerEvents: 'none',
                    }}
                  />
                  Webhook
                </button>
              )}
            </>
          )}

          {/* Original Navigation Items - Only when logged out and on home page */}
          {!isLoggedIn && isHomePage && (
            <>
              <a
                href="#features"
                className="block text-sm py-2"
                style={{ color: 'var(--brand-light-gray)' }}
              >
                {t('features', language)}
              </a>
              <a
                href="/pricing"
                className="block text-sm py-2"
                style={{ color: 'var(--brand-light-gray)' }}
              >
                {t('pricingTitle', language)}
              </a>
            </>
          )}

          {/* User info and logout for mobile when logged in */}
          {isLoggedIn && user && (
            <div
              className="mt-4 pt-4"
              style={{ borderTop: '1px solid var(--panel-border)' }}
            >
              <div
                className="flex items-center gap-2 px-3 py-2 mb-2 rounded"
                style={{ background: 'var(--panel-bg)' }}
              >
                <div
                  className="w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold"
                  style={{
                    background: 'var(--brand-yellow)',
                    color: 'var(--navy-primary)',
                  }}
                >
                  {user.email[0].toUpperCase()}
                </div>
                <div>
                  <div
                    className="text-xs"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t('loggedInAs', language)}
                  </div>
                  <div
                    className="text-sm"
                    style={{ color: 'var(--brand-light-gray)' }}
                  >
                    {user.email}
                  </div>
                </div>
              </div>
              {onLogout && (
                <button
                  onClick={() => {
                    onLogout()
                    setMobileMenuOpen(false)
                  }}
                  className="w-full px-4 py-2 rounded text-sm font-semibold transition-colors text-center"
                  style={{
                    background: 'var(--binance-red-bg)',
                    color: 'var(--binance-red)',
                  }}
                >
                  {t('exitLogin', language)}
                </button>
              )}
            </div>
          )}

          {/* Show login/register buttons when not logged in and not on login/register pages */}
          {!isLoggedIn &&
            currentPage !== 'login' &&
            currentPage !== 'register' && (
              <div className="space-y-2 mt-2">
                <a
                  href="/login"
                  className="block w-full px-4 py-2 rounded text-sm font-medium text-center transition-colors"
                  style={{
                    color: 'var(--brand-light-gray)',
                    border: '1px solid var(--brand-light-gray)',
                  }}
                  onClick={() => setMobileMenuOpen(false)}
                >
                  {t('signIn', language)}
                </a>
                {registrationEnabled && (
                  <a
                    href="/register"
                    className="block w-full px-4 py-2 rounded font-semibold text-sm text-center transition-colors"
                    style={{
                      background: 'var(--brand-yellow)',
                      color: 'var(--navy-primary)',
                    }}
                    onClick={() => setMobileMenuOpen(false)}
                  >
                    {t('signUp', language)}
                  </a>
                )}
              </div>
            )}
        </div>
      </motion.div>
    </nav>
  )
}
