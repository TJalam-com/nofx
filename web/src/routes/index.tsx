import { createBrowserRouter, Navigate, Outlet } from 'react-router-dom'
import MainLayout from '../layouts/MainLayout'
import AuthLayout from '../layouts/AuthLayout'
import { LandingPage } from '../pages/LandingPage'
import { FAQPage } from '../pages/FAQPage'
import { LoginPage } from '../components/LoginPage'
import { RegisterPage } from '../components/RegisterPage'
import { ResetPasswordPage } from '../components/ResetPasswordPage'
import { CompetitionPage } from '../components/CompetitionPage'
import { AITradersPage } from '../pages/AITradersPage'
import TraderDashboard from '../pages/TraderDashboard'
import FollowersPage from '../pages/FollowersPage'
import StatsPage from '../pages/StatsPage'
import { BacktestPage } from '../components/BacktestPage'
import WebhookPage from '../pages/WebhookPage'
import TraderApplicationPage from '../pages/TraderApplicationPage'
import AdminTraderApplicationsPage from '../pages/AdminTraderApplicationsPage'
import { useAuth, isAdmin } from '../contexts/AuthContext'
import { Navigate as NavigateComponent } from 'react-router-dom'
import { ReactNode } from 'react'
import { LanguageProvider } from '../contexts/LanguageContext'
import { AuthProvider } from '../contexts/AuthContext'
import { ConfirmDialogProvider } from '../components/ConfirmDialog'
import { ErrorBoundary } from '../components/ErrorBoundary'

// Root layout with all providers
function RootLayout() {
  return (
    <LanguageProvider>
      <AuthProvider>
        <ConfirmDialogProvider>
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
    errorElement: <ErrorBoundary />,
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
            path: '/faq',
            element: <FAQPage />,
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
            path: '/dashboard',
            element: <TraderDashboard />,
          },
          {
            path: '/followers',
            element: <FollowersPage />,
          },
          {
            path: '/backtest',
            element: <BacktestPage />,
          },
          {
            path: '/webhook',
            element: <WebhookPage />,
          },
          {
            path: '/stats',
            element: <AdminProtectedRoute><StatsPage /></AdminProtectedRoute>,
          },
          {
            path: '/become-trader',
            element: <TraderApplicationPage />,
          },
          {
            path: '/admin/trader-applications',
            element: <AdminProtectedRoute><AdminTraderApplicationsPage /></AdminProtectedRoute>,
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
