import { createBrowserRouter, Navigate, Outlet, useLocation } from 'react-router-dom'
import { lazy, Suspense, ReactNode, useEffect } from 'react'
import MainLayout from '../layouts/MainLayout'
import AuthLayout from '../layouts/AuthLayout'
import { LandingPage } from '../pages/LandingPage'
import { LoginPage } from '../components/LoginPage'
import { RegisterPage } from '../components/RegisterPage'
import { ResetPasswordPage } from '../components/ResetPasswordPage'
import { TermsAndConditionsPage } from '../pages/TermsAndConditionsPage'
import { RiskDisclaimerPage } from '../pages/RiskDisclaimerPage'
import { LicensePage } from '../pages/LicensePage'
import { PricingPage } from '../pages/PricingPage'
import { AboutPage } from '../pages/AboutPage'
import { FeaturesPage } from '../pages/FeaturesPage'
import { SecurityPage } from '../pages/SecurityPage'
import { ContactPage } from '../pages/ContactPage'
import { CompetitionPage } from '../components/CompetitionPage'
import { AITradersPage } from '../pages/AITradersPage'
import TraderDashboard from '../pages/TraderDashboard'
import { useAuth, isAdmin } from '../contexts/AuthContext'
import { Navigate as NavigateComponent } from 'react-router-dom'
import { LanguageProvider } from '../contexts/LanguageContext'
import { AuthProvider } from '../contexts/AuthContext'
import { ConfirmDialogProvider } from '../components/ConfirmDialog'
import { ErrorBoundary } from '../components/ErrorBoundary'

// Lazy load non-critical routes for code splitting
const FAQPage = lazy(() => import('../pages/FAQPage').then(m => ({ default: m.FAQPage })))
const FollowersPage = lazy(() => import('../pages/FollowersPage'))
const StatsPage = lazy(() => import('../pages/StatsPage'))
const BacktestPage = lazy(() => import('../components/BacktestPage').then(m => ({ default: m.BacktestPage })))
const WebhookPage = lazy(() => import('../pages/WebhookPage'))
const TraderApplicationPage = lazy(() => import('../pages/TraderApplicationPage'))
const AdminTraderApplicationsPage = lazy(() => import('../pages/AdminTraderApplicationsPage'))
const StrategyStudioPage = lazy(() => import('../pages/StrategyStudioPage').then(m => ({ default: m.StrategyStudioPage })))

// Loading fallback component
function RouteLoadingFallback() {
  return (
    <div className="flex items-center justify-center min-h-screen" style={{ background: 'var(--navy-primary)' }}>
      <div className="text-center">
        <div className="spinner mx-auto mb-4" />
        <p style={{ color: 'var(--text-secondary)' }}>Loading...</p>
      </div>
    </div>
  )
}

// Scroll to top on route change
function ScrollToTop() {
  const { pathname } = useLocation()

  useEffect(() => {
    window.scrollTo({
      top: 0,
      left: 0,
      behavior: 'instant', // Use 'instant' for immediate scroll, or 'smooth' for animated scroll
    })
  }, [pathname])

  return null
}

// Root layout with all providers
function RootLayout() {
  return (
    <LanguageProvider>
      <AuthProvider>
        <ConfirmDialogProvider>
          <ScrollToTop />
          <Outlet />
        </ConfirmDialogProvider>
      </AuthProvider>
    </LanguageProvider>
  )
}

// Admin protected route wrapper
function AdminProtectedRoute({ children }: { children: ReactNode }) {
  const { user } = useAuth()
  if (!isAdmin(user)) {
    return <NavigateComponent to="/traders" replace />
  }
  return <>{children}</>
}

export const router = createBrowserRouter([
  {
    element: <RootLayout />,
    errorElement: <ErrorBoundary><div /></ErrorBoundary>,
    children: [
      {
        path: '/',
        element: <LandingPage />,
      },
      // Auth routes - using AuthLayout
      {
        element: <AuthLayout />,
        children: [
          {
            path: '/login',
            element: <LoginPage />,
          },
          {
            path: '/register',
            element: <RegisterPage />,
          },
          {
            path: '/reset-password',
            element: <ResetPasswordPage />,
          },
        ],
      },
      // Main app routes - using MainLayout with nested routes
      {
        element: <MainLayout />,
        children: [
          {
            path: '/terms',
            element: <TermsAndConditionsPage />,
          },
          {
            path: '/risk-disclaimer',
            element: <RiskDisclaimerPage />,
          },
          {
            path: '/license',
            element: <LicensePage />,
          },
          {
            path: '/pricing',
            element: <PricingPage />,
          },
          {
            path: '/about',
            element: <AboutPage />,
          },
          {
            path: '/features',
            element: <FeaturesPage />,
          },
          {
            path: '/security',
            element: <SecurityPage />,
          },
          {
            path: '/contact',
            element: <ContactPage />,
          },
          {
            path: '/faq',
            element: (
              <Suspense fallback={<RouteLoadingFallback />}>
                <FAQPage />
              </Suspense>
            ),
          },
          {
            path: '/competition',
            element: <CompetitionPage />,
          },
          {
            path: '/traders',
            element: <AITradersPage />,
          },
          {
            path: '/strategy-studio',
            element: (
              <Suspense fallback={<RouteLoadingFallback />}>
                <StrategyStudioPage />
              </Suspense>
            ),
          },
          {
            path: '/dashboard',
            element: <TraderDashboard />,
          },
          {
            path: '/followers',
            element: (
              <Suspense fallback={<RouteLoadingFallback />}>
                <FollowersPage />
              </Suspense>
            ),
          },
          {
            path: '/backtest',
            element: (
              <Suspense fallback={<RouteLoadingFallback />}>
                <BacktestPage />
              </Suspense>
            ),
          },
          {
            path: '/webhook',
            element: (
              <Suspense fallback={<RouteLoadingFallback />}>
                <WebhookPage />
              </Suspense>
            ),
          },
          {
            path: '/stats',
            element: (
              <Suspense fallback={<RouteLoadingFallback />}>
                <AdminProtectedRoute><StatsPage /></AdminProtectedRoute>
              </Suspense>
            ),
          },
          {
            path: '/become-trader',
            element: (
              <Suspense fallback={<RouteLoadingFallback />}>
                <TraderApplicationPage />
              </Suspense>
            ),
          },
          {
            path: '/admin/trader-applications',
            element: (
              <Suspense fallback={<RouteLoadingFallback />}>
                <AdminProtectedRoute><AdminTraderApplicationsPage /></AdminProtectedRoute>
              </Suspense>
            ),
          },
        ],
      },
      {
        path: '*',
        element: <Navigate to="/" replace />,
      },
    ],
  },
])
