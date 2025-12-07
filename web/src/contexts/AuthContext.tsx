import React, { createContext, useContext, useState, useEffect, useCallback } from 'react'
import { getSystemConfig } from '../lib/config'
import { reset401Flag, httpClient } from '../lib/httpClient'
import { api } from '../lib/api'

interface User {
  id: string
  email: string
  role?: string
}

// Helper function to check if user is a follower
export function isFollower(user: User | null): boolean {
  const result = user?.role === 'follower'
  if (user) {
    console.log('🔍 isFollower check:', { userId: user.id, role: user.role, isFollower: result })
  }
  return result
}

// Helper function to check if user is an admin
export function isAdmin(user: User | null): boolean {
  return user?.role === 'admin'
}

interface AuthContextType {
  user: User | null
  token: string | null
  login: (
    email: string,
    password: string
  ) => Promise<{
    success: boolean
    message?: string
    userID?: string
    requiresOTP?: boolean
  }>
  loginAdmin: (password: string) => Promise<{
    success: boolean
    message?: string
  }>
  register: (
    email: string,
    password: string,
    betaCode?: string
  ) => Promise<{
    success: boolean
    message?: string
    userID?: string
    otpSecret?: string
    qrCodeURL?: string
  }>
  verifyOTP: (
    userID: string,
    otpCode: string
  ) => Promise<{ success: boolean; message?: string }>
  completeRegistration: (
    userID: string,
    otpCode: string
  ) => Promise<{ success: boolean; message?: string }>
  resetPassword: (
    email: string,
    newPassword: string,
    otpCode: string
  ) => Promise<{ success: boolean; message?: string }>
  refreshUser: () => Promise<void>
  logout: () => void
  isLoading: boolean
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [token, setToken] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  // Define refreshUser early so it can be used in useEffect hooks
  const refreshUser = useCallback(async () => {
    try {
      const userInfo = await api.getCurrentUser()
      console.log('🔄 Refreshed user info from backend - Role:', userInfo.role)
      const updatedUser = { id: userInfo.id, email: userInfo.email, role: userInfo.role }
      setUser(updatedUser)
      localStorage.setItem('auth_user', JSON.stringify(updatedUser))
      console.log('✅ User role refreshed:', updatedUser.role)
    } catch (error) {
      console.error('Failed to refresh user info:', error)
      // Don't throw - allow user to continue with cached info
    }
  }, [])

  useEffect(() => {
    // Reset 401 flag on page load to allow fresh 401 handling
    reset401Flag()

    // 先检查是否为管理员模式（使用带缓存的系统配置获取）
    getSystemConfig()
      .then(() => {
        // 不再在管理员模式下模拟登录；统一检查本地存储
        const savedToken = localStorage.getItem('auth_token')
        const savedUser = localStorage.getItem('auth_user')
        if (savedToken && savedUser) {
          const parsedUser = JSON.parse(savedUser)
          console.log('🔐 Restored user from localStorage - Role:', parsedUser.role)
          console.log('🔐 Full restored user:', parsedUser)
          setToken(savedToken)
          setUser(parsedUser)
          // Verify role detection
          console.log('✅ isFollower check on restored user:', parsedUser.role === 'follower')
        }

        setIsLoading(false)
      })
      .catch((err) => {
        console.error('Failed to fetch system config:', err)
        // 发生错误时，继续检查本地存储
        const savedToken = localStorage.getItem('auth_token')
        const savedUser = localStorage.getItem('auth_user')

        if (savedToken && savedUser) {
          setToken(savedToken)
          setUser(JSON.parse(savedUser))
        }
        setIsLoading(false)
      })
  }, [])

  // Listen for unauthorized events from httpClient (401 responses)
  useEffect(() => {
    const handleUnauthorized = () => {
      console.log('Unauthorized event received - clearing auth state')
      // Clear auth state when 401 is detected
      setUser(null)
      setToken(null)
      // Note: localStorage cleanup is already done in httpClient
    }

    window.addEventListener('unauthorized', handleUnauthorized)

    return () => {
      window.removeEventListener('unauthorized', handleUnauthorized)
    }
  }, [])

  // Listen for role-changed events from httpClient (403 responses)
  useEffect(() => {
    const handleRoleChanged = () => {
      console.log('Role changed event received - refreshing user info')
      // Refresh user info when role change is detected (403 error)
      if (token && user) {
        refreshUser().catch((err) => {
          console.error('Failed to refresh user after role change:', err)
        })
      }
    }

    window.addEventListener('role-changed', handleRoleChanged)

    return () => {
      window.removeEventListener('role-changed', handleRoleChanged)
    }
  }, [token, user, refreshUser])

  // Periodic refresh and refresh on window focus
  useEffect(() => {
    if (!token || !user) {
      return
    }

    // Refresh on window focus to catch role changes when user returns to tab
    const handleFocus = () => {
      console.log('Window focused - refreshing user info')
      refreshUser().catch((err) => {
        console.error('Failed to refresh user on focus:', err)
      })
    }

    // Periodic refresh every 60 seconds to catch role changes
    const refreshInterval = setInterval(() => {
      console.log('Periodic refresh - checking user role')
      refreshUser().catch((err) => {
        console.error('Failed to refresh user periodically:', err)
      })
    }, 60000) // 60 seconds

    window.addEventListener('focus', handleFocus)

    return () => {
      clearInterval(refreshInterval)
      window.removeEventListener('focus', handleFocus)
    }
  }, [token, user, refreshUser])

  const login = async (email: string, password: string) => {
    try {
      const response = await fetch('/api/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email, password }),
      })

      const data = await response.json()

      if (response.ok) {
        if (data.requires_otp) {
          return {
            success: true,
            userID: data.user_id,
            requiresOTP: true,
            message: data.message,
          }
        }
      } else {
        return { success: false, message: data.error }
      }
    } catch (error) {
      return { success: false, message: '登录失败，请重试' }
    }

    return { success: false, message: '未知错误' }
  }

  const loginAdmin = async (password: string) => {
    try {
      const response = await fetch('/api/admin-login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password }),
      })
      const data = await response.json()
      if (response.ok) {
        // Reset 401 flag on successful login
        reset401Flag()

        const userInfo = {
          id: data.user_id || 'admin',
          email: data.email || 'admin@localhost',
          role: data.role || 'user',
        }
        setToken(data.token)
        setUser(userInfo)
        localStorage.setItem('auth_token', data.token)
        localStorage.setItem('auth_user', JSON.stringify(userInfo))

        // Check and redirect to returnUrl if exists
        const returnUrl = sessionStorage.getItem('returnUrl')
        if (returnUrl) {
          sessionStorage.removeItem('returnUrl')
          window.history.pushState({}, '', returnUrl)
          window.dispatchEvent(new PopStateEvent('popstate'))
        } else {
          // 跳转到仪表盘
          window.history.pushState({}, '', '/dashboard')
          window.dispatchEvent(new PopStateEvent('popstate'))
        }
        return { success: true }
      } else {
        return { success: false, message: data.error || '登录失败' }
      }
    } catch (e) {
      return { success: false, message: '登录失败，请重试' }
    }
  }

  const register = async (
    email: string,
    password: string,
    betaCode?: string
  ) => {
    const requestBody: {
      email: string
      password: string
      beta_code?: string
    } = { email, password }
    if (betaCode) {
      requestBody.beta_code = betaCode
    }

    const result = await httpClient.post<{
      user_id: string
      otp_secret: string
      qr_code_url: string
      message: string
    }>('/api/register', requestBody)

    if (result.success && result.data) {
      return {
        success: true,
        userID: result.data.user_id,
        otpSecret: result.data.otp_secret,
        qrCodeURL: result.data.qr_code_url,
        message: result.message || result.data.message,
      }
    }

    // Only business errors reach here (system/network errors were intercepted)
    return {
      success: false,
      message: result.message || 'Registration failed',
    }
  }

  const verifyOTP = async (userID: string, otpCode: string) => {
    try {
      const response = await fetch('/api/verify-otp', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ user_id: userID, otp_code: otpCode }),
      })

      const data = await response.json()

      if (response.ok) {
        // Reset 401 flag on successful login
        reset401Flag()

        // 登录成功，保存token和用户信息
        const userInfo = { id: data.user_id, email: data.email, role: data.role || 'user' }
        console.log('🔐 Login successful - User role:', userInfo.role)
        console.log('🔐 Full user info:', userInfo)
        setToken(data.token)
        setUser(userInfo)
        localStorage.setItem('auth_token', data.token)
        localStorage.setItem('auth_user', JSON.stringify(userInfo))
        // Verify role is stored correctly
        const storedUser = localStorage.getItem('auth_user')
        if (storedUser) {
          const parsed = JSON.parse(storedUser)
          console.log('✅ Role stored in localStorage:', parsed.role)
          console.log('✅ isFollower check:', parsed.role === 'follower')
        }
        
        // Debug: Log role detection
        console.log('🔍 Role Detection:', {
          role: userInfo.role,
          isFollower: userInfo.role === 'follower',
          storedUser: JSON.parse(localStorage.getItem('auth_user') || '{}')
        })

        // Check and redirect to returnUrl if exists
        const returnUrl = sessionStorage.getItem('returnUrl')
        if (returnUrl) {
          sessionStorage.removeItem('returnUrl')
          window.history.pushState({}, '', returnUrl)
          window.dispatchEvent(new PopStateEvent('popstate'))
        } else {
          // 跳转到配置页面
          window.history.pushState({}, '', '/traders')
          window.dispatchEvent(new PopStateEvent('popstate'))
        }

        return { success: true, message: data.message }
      } else {
        return { success: false, message: data.error }
      }
    } catch (error) {
      return { success: false, message: 'OTP验证失败，请重试' }
    }
  }

  const completeRegistration = async (userID: string, otpCode: string) => {
    try {
      const response = await fetch('/api/complete-registration', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ user_id: userID, otp_code: otpCode }),
      })

      const data = await response.json()

      if (response.ok) {
        // Reset 401 flag on successful login
        reset401Flag()

        // 注册完成，自动登录
        const userInfo = { id: data.user_id, email: data.email, role: data.role || 'user' }
        console.log('🔐 Registration complete - User role:', userInfo.role)
        console.log('🔐 Full user info:', userInfo)
        setToken(data.token)
        setUser(userInfo)
        localStorage.setItem('auth_token', data.token)
        localStorage.setItem('auth_user', JSON.stringify(userInfo))
        // Verify role is stored correctly
        const storedUser = localStorage.getItem('auth_user')
        if (storedUser) {
          const parsed = JSON.parse(storedUser)
          console.log('✅ Role stored in localStorage:', parsed.role)
          console.log('✅ isFollower check:', parsed.role === 'follower')
        }

        // Check and redirect to returnUrl if exists
        const returnUrl = sessionStorage.getItem('returnUrl')
        if (returnUrl) {
          sessionStorage.removeItem('returnUrl')
          window.history.pushState({}, '', returnUrl)
          window.dispatchEvent(new PopStateEvent('popstate'))
        } else {
          // 跳转到配置页面
          window.history.pushState({}, '', '/traders')
          window.dispatchEvent(new PopStateEvent('popstate'))
        }

        return { success: true, message: data.message }
      } else {
        return { success: false, message: data.error }
      }
    } catch (error) {
      return { success: false, message: '注册完成失败，请重试' }
    }
  }

  const resetPassword = async (
    email: string,
    newPassword: string,
    otpCode: string
  ) => {
    try {
      const response = await fetch('/api/reset-password', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          email,
          new_password: newPassword,
          otp_code: otpCode,
        }),
      })

      const data = await response.json()

      if (response.ok) {
        return { success: true, message: data.message }
      } else {
        return { success: false, message: data.error }
      }
    } catch (error) {
      return { success: false, message: '密码重置失败，请重试' }
    }
  }

  const logout = () => {
    const savedToken = localStorage.getItem('auth_token')
    if (savedToken) {
      fetch('/api/logout', {
        method: 'POST',
        headers: { Authorization: `Bearer ${savedToken}` },
      }).catch(() => {
        /* ignore network errors on logout */
      })
    }
    setUser(null)
    setToken(null)
    localStorage.removeItem('auth_token')
    localStorage.removeItem('auth_user')
  }

  return (
    <AuthContext.Provider
      value={{
        user,
        token,
        login,
        loginAdmin,
        register,
        verifyOTP,
        completeRegistration,
        resetPassword,
        refreshUser,
        logout,
        isLoading,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
