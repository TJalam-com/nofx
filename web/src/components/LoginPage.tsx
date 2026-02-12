import React, { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { Eye, EyeOff } from 'lucide-react'
import { Input } from './ui/input'
import { toast } from 'sonner'
import { useSystemConfig } from '../hooks/useSystemConfig'

// Helper function to translate backend error messages
function translateBackendError(message: string, language: 'en' | 'zh'): string {
  if (!message) return message

  const errorMap: Record<string, { en: string; zh: string }> = {
    邮箱已被注册: { en: 'Email already registered', zh: '邮箱已被注册' },
    'email already registered': {
      en: 'Email already registered',
      zh: '邮箱已被注册',
    },
    登录失败: { en: 'Login failed', zh: '登录失败' },
    '登录失败，请重试': {
      en: 'Login failed, please try again',
      zh: '登录失败，请重试',
    },
    未知错误: { en: 'Unknown error', zh: '未知错误' },
    注册失败: { en: 'Registration failed', zh: '注册失败' },
  }

  // Check for exact matches first
  if (errorMap[message]) {
    return errorMap[message][language]
  }

  // Check for partial matches
  const lowerMessage = message.toLowerCase()
  for (const [key, translations] of Object.entries(errorMap)) {
    if (lowerMessage.includes(key.toLowerCase()) || message.includes(key)) {
      return translations[language]
    }
  }

  // If message contains Chinese characters and language is English, try to translate common patterns
  if (language === 'en' && /[\u4e00-\u9fff]/.test(message)) {
    if (message.includes('邮箱已被注册') || message.includes('已注册')) {
      return 'Email already registered'
    }
    if (message.includes('登录失败')) {
      return 'Login failed, please try again'
    }
    if (message.includes('注册失败')) {
      return 'Registration failed'
    }
  }

  return message
}

export function LoginPage() {
  const { language } = useLanguage()
  const { login, loginAdmin, verifyOTP, user } = useAuth()
  const navigate = useNavigate()
  const [step, setStep] = useState<'login' | 'otp'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [otpCode, setOtpCode] = useState('')
  const [userID, setUserID] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [adminPassword, setAdminPassword] = useState('')
  const adminMode = false
  const { config: systemConfig } = useSystemConfig()
  const registrationEnabled = systemConfig?.registration_enabled !== false
  const [expiredToastId, setExpiredToastId] = useState<string | number | null>(
    null
  )

  // Redirect if already logged in (unless redirected here due to 401/session expiry)
  useEffect(() => {
    if (user && sessionStorage.getItem('from401') !== 'true') {
      navigate('/traders', { replace: true })
    }
  }, [user, navigate])

  // Show notification if user was redirected here due to 401
  useEffect(() => {
    if (sessionStorage.getItem('from401') === 'true') {
      const id = toast.warning(t('sessionExpired', language), {
        duration: Infinity, // Keep showing until user dismisses or logs in
      })
      setExpiredToastId(id)
      sessionStorage.removeItem('from401')
    }
  }, [language])

  const handleAdminLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    const result = await loginAdmin(adminPassword)
    if (!result.success) {
      const backendMsg = result.message || ''
      const translatedMsg = translateBackendError(backendMsg, language)
      const msg = translatedMsg || t('loginFailed', language)
      setError(msg)
      toast.error(msg)
    } else {
      // Dismiss the "login expired" toast on successful login
      if (expiredToastId) {
        toast.dismiss(expiredToastId)
      }
    }
    setLoading(false)
  }

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const result = await login(email, password)

    if (result.success) {
      if (result.requiresOTP && result.userID) {
        setUserID(result.userID)
        setStep('otp')
      } else {
        // Dismiss the "login expired" toast on successful login (no OTP required)
        if (expiredToastId) {
          toast.dismiss(expiredToastId)
        }
      }
    } else {
      const backendMsg = result.message || ''
      const translatedMsg = translateBackendError(backendMsg, language)
      const msg = translatedMsg || t('loginFailed', language)
      setError(msg)
      toast.error(msg)
    }

    setLoading(false)
  }

  const handleOTPVerify = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const result = await verifyOTP(userID, otpCode)

    if (!result.success) {
      const backendMsg = result.message || ''
      const translatedMsg = translateBackendError(backendMsg, language)
      const msg = translatedMsg || t('verificationFailed', language)
      setError(msg)
      toast.error(msg)
    } else {
      // Dismiss the "login expired" toast on successful OTP verification
      if (expiredToastId) {
        toast.dismiss(expiredToastId)
      }
    }
    // 成功的话AuthContext会自动处理登录状态

    setLoading(false)
  }

  return (
    <div
      className="flex items-center justify-center py-12"
      style={{
        minHeight: 'calc(100vh - 64px)',
        background: 'var(--navy-primary)',
      }}
    >
      <div className="w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="w-16 h-16 mx-auto mb-4 flex items-center justify-center">
            <img
              src="/icons/nofx.svg"
              alt="AI Trading 24x7 Logo"
              className="w-16 h-16 object-contain"
              width="64"
              height="64"
            />
          </div>
          <h1
            className="text-2xl font-bold"
            style={{ color: 'var(--brand-light-gray)' }}
          >
            {t('loginTitle', language)}
          </h1>
          <p
            className="text-sm mt-2"
            style={{ color: 'var(--text-secondary)' }}
          >
            {step === 'login'
              ? t('loginSubtitle', language)
              : t('loginOTPSubtitle', language)}
          </p>
        </div>

        {/* Login Form */}
        <div
          className="rounded-lg p-6"
          style={{
            background: 'var(--navy-dark)',
            border: '1px solid var(--panel-border)',
          }}
        >
          {adminMode ? (
            <form onSubmit={handleAdminLogin} className="space-y-4">
              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: 'var(--brand-light-gray)' }}
                >
                  {t('adminPassword', language)}
                </label>
                <input
                  type="password"
                  value={adminPassword}
                  onChange={(e) => setAdminPassword(e.target.value)}
                  className="w-full px-3 py-2 rounded"
                  style={{
                    background: 'var(--navy-primary)',
                    border: '1px solid var(--panel-border)',
                    color: 'var(--brand-light-gray)',
                  }}
                  placeholder={t('enterAdminPassword', language)}
                  required
                />
              </div>

              {error && (
                <div
                  className="text-sm px-3 py-2 rounded"
                  style={{
                    background: 'var(--binance-red-bg)',
                    color: 'var(--binance-red)',
                  }}
                >
                  {error}
                </div>
              )}

              <button
                type="submit"
                disabled={loading}
                className="w-full px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50"
                style={{
                  background: 'var(--green-primary)',
                  color: 'var(--navy-primary)',
                }}
              >
                {loading ? t('loading', language) : t('loginButton', language)}
              </button>
            </form>
          ) : step === 'login' ? (
            <form onSubmit={handleLogin} className="space-y-4">
              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: 'var(--brand-light-gray)' }}
                >
                  {t('email', language)}
                </label>
                <Input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder={t('emailPlaceholder', language)}
                  required
                />
              </div>

              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: 'var(--brand-light-gray)' }}
                >
                  {t('password', language)}
                </label>
                <div className="relative">
                  <Input
                    type={showPassword ? 'text' : 'password'}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="pr-10"
                    placeholder={t('passwordPlaceholder', language)}
                    required
                  />
                  <button
                    type="button"
                    aria-label={
                      showPassword
                        ? t('hidePassword', language)
                        : t('showPassword', language)
                    }
                    onMouseDown={(e) => e.preventDefault()}
                    onClick={() => setShowPassword((v) => !v)}
                    className="absolute inset-y-0 right-2 w-8 h-10 flex items-center justify-center rounded bg-transparent p-0 m-0 border-0 outline-none focus:outline-none focus:ring-0 appearance-none cursor-pointer btn-icon"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {showPassword ? <EyeOff size={18} /> : <Eye size={18} />}
                  </button>
                </div>
                <div className="text-right mt-2">
                  <button
                    type="button"
                    onClick={() => navigate('/reset-password')}
                    className="text-xs hover:underline transition-colors"
                    style={{ color: 'var(--green-primary)' }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.color = 'var(--green-light)'
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.color = 'var(--green-primary)'
                    }}
                  >
                    {t('forgotPassword', language)}
                  </button>
                </div>
              </div>

              {error && (
                <div
                  className="text-sm px-3 py-2 rounded"
                  style={{
                    background: 'var(--binance-red-bg)',
                    color: 'var(--binance-red)',
                  }}
                >
                  {error}
                </div>
              )}

              <button
                type="submit"
                disabled={loading}
                className="w-full px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50"
                style={{
                  background: 'var(--green-primary)',
                  color: 'var(--navy-primary)',
                }}
              >
                {loading ? t('loading', language) : t('loginButton', language)}
              </button>
            </form>
          ) : (
            <form onSubmit={handleOTPVerify} className="space-y-4">
              <div className="text-center mb-4">
                <div className="text-4xl mb-2">📱</div>
                <p className="text-sm" style={{ color: '#848E9C' }}>
                  {t('scanQRCodeInstructions', language)}
                  <br />
                  {t('enterOTPCode', language)}
                </p>
              </div>

              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: 'var(--brand-light-gray)' }}
                >
                  {t('otpCode', language)}
                </label>
                <input
                  type="text"
                  value={otpCode}
                  onChange={(e) =>
                    setOtpCode(e.target.value.replace(/\D/g, '').slice(0, 6))
                  }
                  className="w-full px-3 py-2 rounded text-center text-2xl font-mono"
                  style={{
                    background: 'var(--navy-primary)',
                    border: '1px solid var(--panel-border)',
                    color: 'var(--brand-light-gray)',
                  }}
                  placeholder={t('otpPlaceholder', language)}
                  maxLength={6}
                  required
                />
              </div>

              {error && (
                <div
                  className="text-sm px-3 py-2 rounded"
                  style={{
                    background: 'var(--binance-red-bg)',
                    color: 'var(--binance-red)',
                  }}
                >
                  {error}
                </div>
              )}

              <div className="flex gap-3">
                <button
                  type="button"
                  onClick={() => setStep('login')}
                  className="flex-1 px-4 py-2 rounded text-sm font-semibold"
                  style={{
                    background: 'var(--navy-light)',
                    color: 'var(--text-secondary)',
                  }}
                >
                  {t('back', language)}
                </button>
                <button
                  type="submit"
                  disabled={loading || otpCode.length !== 6}
                  className="flex-1 px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50"
                  style={{
                    background: 'var(--green-primary)',
                    color: 'var(--navy-primary)',
                  }}
                >
                  {loading ? t('loading', language) : t('verifyOTP', language)}
                </button>
              </div>
            </form>
          )}
        </div>

        {/* Register Link */}
        {!adminMode && registrationEnabled && (
          <div className="text-center mt-6">
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              {t('noAccount', language)}{' '}
              <button
                type="button"
                onClick={(e) => {
                  e.preventDefault()
                  e.stopPropagation()
                  navigate('/register')
                }}
                className="font-semibold hover:underline transition-colors"
                style={{ color: 'var(--green-primary)' }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.color = 'var(--green-light)'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.color = 'var(--green-primary)'
                }}
              >
                {t('registerNowButton', language)}
              </button>
            </p>
          </div>
        )}
      </div>
    </div>
  )
}
