import React from 'react'
import ReactDOM from 'react-dom/client'
import { Toaster } from 'sonner'
import './index.css'
import { RouterProvider } from 'react-router-dom'
import { router } from './routes'
import { ErrorBoundary } from './components/ErrorBoundary'

// Error suppression wrapper for RouterProvider to handle known React Router DOM issues
function RouterProviderWithErrorHandling() {
  React.useEffect(() => {
    // Global error handler for unhandled React Router DOM errors
    const handleError = (event: ErrorEvent) => {
      const error = event.error || event.message
      const errorMessage = typeof error === 'string' ? error : error?.message || ''
      
      // Suppress known React Router DOM manipulation errors that occur during navigation
      if (
        errorMessage.includes('removeChild') &&
        errorMessage.includes('not a child of this node')
      ) {
        // This is a known issue with React Router and React.StrictMode in development
        // It doesn't affect functionality, so we suppress it
        console.warn('React Router navigation warning (suppressed):', errorMessage)
        event.preventDefault()
        event.stopPropagation()
        return false
      }
      return true
    }

    // Add global error handler
    window.addEventListener('error', handleError, true)
    
    // Cleanup
    return () => {
      window.removeEventListener('error', handleError, true)
    }
  }, [])

  return <RouterProvider router={router} />
}

const App = (
  <ErrorBoundary>
    <RouterProviderWithErrorHandling />
    <Toaster
      theme="dark"
      richColors
      closeButton
      position="top-center"
      duration={2200}
      toastOptions={{
        className: 'nofx-toast',
        style: {
          background: '#0b0e11',
          border: '1px solid var(--panel-border)',
          color: 'var(--text-primary)',
        },
      }}
    />
  </ErrorBoundary>
)

// Only enable StrictMode in development to avoid double-rendering issues
const root = ReactDOM.createRoot(document.getElementById('root')!)

if (import.meta.env.DEV) {
  root.render(<React.StrictMode>{App}</React.StrictMode>)
} else {
  root.render(App)
}
