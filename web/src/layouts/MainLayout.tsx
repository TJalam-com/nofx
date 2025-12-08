import { ReactNode } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import HeaderBar from '../components/HeaderBar'
import { Container } from '../components/Container'
import Footer from '../components/Footer'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth } from '../contexts/AuthContext'
import { useSEO } from '../hooks/useSEO'

interface MainLayoutProps {
  children?: ReactNode
}

export default function MainLayout({ children }: MainLayoutProps) {
  const { language, setLanguage } = useLanguage()
  const { user, logout } = useAuth()
  const location = useLocation()
  const { SEOComponent } = useSEO()

  // 根据路径自动判断当前页面
  const getCurrentPage = (): 'competition' | 'traders' | 'trader' | 'followers' | 'faq' | 'stats' => {
    if (location.pathname === '/faq') return 'faq'
    if (location.pathname === '/traders') return 'traders'
    if (location.pathname === '/dashboard') return 'trader'
    if (location.pathname === '/followers') return 'followers'
    if (location.pathname === '/stats') return 'stats'
    if (location.pathname === '/competition') return 'competition'
    return 'competition' // 默认
  }

  return (
    <>
      <SEOComponent />
      <div
        className="min-h-screen"
        style={{ background: 'var(--navy-primary)', color: 'var(--text-primary)' }}
      >
      <HeaderBar
        isLoggedIn={!!user}
        currentPage={getCurrentPage()}
        language={language}
        onLanguageChange={setLanguage}
        user={user}
        onLogout={logout}
        onPageChange={() => {
          // React Router handles navigation now
        }}
      />

      {/* Main Content */}
      <Container as="main" className="py-6 pt-24">
        {children || <Outlet />}
      </Container>

      {/* Footer */}
      <Footer variant="full" />
    </div>
    </>
  )
}
