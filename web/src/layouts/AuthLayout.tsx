import { ReactNode } from 'react'
import { Outlet } from 'react-router-dom'
import { useSEO } from '../hooks/useSEO'
import HeaderBar from '../components/HeaderBar'
import Footer from '../components/Footer'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'

interface AuthLayoutProps {
  children?: ReactNode
}

export default function AuthLayout({ children }: AuthLayoutProps) {
  const { SEOComponent } = useSEO()
  const { logout } = useAuth()
  const { language } = useLanguage()

  return (
    <>
      <SEOComponent />
      <div className="min-h-screen" style={{ background: 'var(--background)' }}>
        <HeaderBar
          isLoggedIn={false}
          language={language}
          user={null}
          onLogout={logout}
          onPageChange={() => {
            // React Router handles navigation now
          }}
        />

        {/* Content with top padding to avoid overlap with fixed header */}
        <div className="pt-24">{children || <Outlet />}</div>

        {/* Footer */}
        <Footer variant="full" />
      </div>
    </>
  )
}
