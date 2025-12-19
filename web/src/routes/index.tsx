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
// Add error handling for failed dynamic imports to prevent app crashes
const lazyWithErrorHandling = <T extends React.ComponentType<any>>(
  importFn: () => Promise<{ default: T } | { [key: string]: T }>
) => {
  return lazy(async () => {
    try {
      const module = await importFn()
      // Normalize the module to always have a default export
      if ('default' in module) {
        return { default: module.default }
      }
      // If no default export, try to get the first named export or create a fallback
      const firstKey = Object.keys(module)[0]
      if (firstKey) {
        return { default: (module as { [key: string]: T })[firstKey] }
      }
      throw new Error('No export found in module')
    } catch (error) {
      console.error('Failed to load module:', error)
      // Return a fallback component that shows an error message
      const FallbackComponent: React.ComponentType = () => (
        <div className="flex items-center justify-center min-h-screen" style={{ background: 'var(--navy-primary)' }}>
          <div className="text-center">
            <h2 className="text-2xl font-bold mb-4" style={{ color: 'var(--text-primary)' }}>Failed to load page</h2>
            <p className="mb-4" style={{ color: 'var(--text-secondary)' }}>Please refresh the page or try again later.</p>
            <button
              onClick={() => window.location.reload()}
              className="px-4 py-2 rounded bg-blue-600 text-white hover:bg-blue-700"
            >
              Refresh Page
            </button>
          </div>
        </div>
      )
      return { default: FallbackComponent as T }
    }
  })
}

const FAQPage = lazyWithErrorHandling(() => import('../pages/FAQPage').then(m => ({ default: m.FAQPage })))
const FollowersPage = lazyWithErrorHandling(() => import('../pages/FollowersPage'))
const StatsPage = lazyWithErrorHandling(() => import('../pages/StatsPage'))
const BacktestPage = lazyWithErrorHandling(() => import('../components/BacktestPage').then(m => ({ default: m.BacktestPage })))
const WebhookPage = lazyWithErrorHandling(() => import('../pages/WebhookPage'))
const TraderApplicationPage = lazyWithErrorHandling(() => import('../pages/TraderApplicationPage'))
const AdminTraderApplicationsPage = lazyWithErrorHandling(() => import('../pages/AdminTraderApplicationsPage'))
const AdminArticlesPage = lazyWithErrorHandling(() => import('../pages/AdminArticlesPage'))
const StrategyStudioPage = lazyWithErrorHandling(() => import('../pages/StrategyStudioPage').then(m => ({ default: m.StrategyStudioPage })))
const BlogPage = lazyWithErrorHandling(() => import('../pages/BlogPage').then(m => ({ default: m.BlogPage })))
const ArticlePage = lazyWithErrorHandling(() => import('../pages/ArticlePage').then(m => ({ default: m.ArticlePage })))

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
            path: '/dashboard/:slug',
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
          {
            path: '/admin/articles',
            element: (
              <Suspense fallback={<RouteLoadingFallback />}>
                <AdminProtectedRoute><AdminArticlesPage /></AdminProtectedRoute>
              </Suspense>
            ),
          },
          {
            path: '/blog',
            element: (
              <Suspense fallback={<RouteLoadingFallback />}>
                <BlogPage />
              </Suspense>
            ),
          },
          {
            path: '/blog/:slug',
            element: (
              <Suspense fallback={<RouteLoadingFallback />}>
                <ArticlePage />
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
