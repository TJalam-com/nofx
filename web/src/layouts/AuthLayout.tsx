import { ReactNode } from 'react'
import { Outlet, Link } from 'react-router-dom'
import { Container } from '../components/Container'
import { useLanguage } from '../contexts/LanguageContext'
import { useSEO } from '../hooks/useSEO'
import { t } from '../i18n/translations'

interface AuthLayoutProps {
  children?: ReactNode
}

export default function AuthLayout({ children }: AuthLayoutProps) {
  const { language } = useLanguage()
  const { SEOComponent } = useSEO()

  return (
    <>
      <SEOComponent />
      <div className="min-h-screen" style={{ background: 'var(--background)' }}>
      {/* Simple Header with Logo */}
      <nav
        className="fixed top-0 w-full z-50 header-bar"
        style={{
          background: 'var(--header-bg)',
          backdropFilter: 'blur(10px)',
        }}
      >
        <Container className="flex items-center justify-between h-16">
          {/* Logo */}
          <Link
            to="/"
            className="flex items-center gap-3 hover:opacity-80 transition-opacity"
          >
            <img src="/icons/nofx.svg" alt="AI Trading Logo" className="w-8 h-8" />
            <span className="text-xl font-bold" style={{ color: 'var(--green-primary)' }}>
              AI Trading
            </span>
          </Link>

        </Container>
      </nav>

      {/* Content with top padding to avoid overlap with fixed header */}
      <div className="pt-16">{children || <Outlet />}</div>
    </div>
    </>
  )
}
