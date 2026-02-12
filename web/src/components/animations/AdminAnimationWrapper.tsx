import { useAuth, isAdmin } from '../../contexts/AuthContext'
import { ReactNode } from 'react'

interface AdminAnimationWrapperProps {
  children: ReactNode
  staticFallback?: ReactNode
  className?: string
}

/**
 * Wrapper component that gates animations behind admin role check.
 * Renders animated version for admins, static fallback for others.
 *
 * Usage:
 * <AdminAnimationWrapper staticFallback={<StaticComponent />}>
 *   <AnimatedComponent />
 * </AdminAnimationWrapper>
 */
export function AdminAnimationWrapper({
  children,
  staticFallback,
  className,
}: AdminAnimationWrapperProps) {
  const { user } = useAuth()
  const admin = isAdmin(user)

  if (!admin) {
    return staticFallback ? (
      <div className={className}>{staticFallback}</div>
    ) : null
  }

  return <div className={className}>{children}</div>
}
