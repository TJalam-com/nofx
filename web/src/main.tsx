import React from 'react'
import ReactDOM from 'react-dom/client'
import { Toaster } from 'sonner'
import { HelmetProvider } from 'react-helmet-async'
import './index.css'
import { RouterProvider } from 'react-router-dom'
import { router } from './routes'
import { ErrorBoundary } from './components/ErrorBoundary'

// Error suppression wrapper for RouterProvider to handle known React Router DOM issues
function RouterProviderWithErrorHandling() {
  React.useEffect(() => {
    // Helper function to check if error matches the known React Router DOM issue
    const isReactRouterDOMError = (error: any): boolean => {
      if (!error) return false
      
      const errorName = error?.name || ''
      const errorMessage = 
        error?.message || 
        error?.toString() || 
        (typeof error === 'string' ? error : '')
      
      // Check for NotFoundError with removeChild DOM manipulation issues
      // This can occur from React Router navigation or browser extensions
      return (
        (errorName === 'NotFoundError' || errorMessage.includes('NotFoundError')) &&
        errorMessage.includes('removeChild') &&
        errorMessage.includes('not a child of this node')
      )
    }

    // Global error handler for unhandled React Router DOM errors
    const handleError = (event: ErrorEvent) => {
      const error = event.error || event.message
      
      if (isReactRouterDOMError(error)) {
        // This is a known issue with React Router DOM manipulation during navigation
        // It doesn't affect functionality, so we suppress it
        console.warn('React Router navigation warning (suppressed):', error?.message || error)
        event.preventDefault()
        event.stopPropagation()
        return false
      }
      return true
    }

    // Handle unhandled promise rejections (which React Router might use)
    const handleUnhandledRejection = (event: PromiseRejectionEvent) => {
      const error = event.reason
      
      if (isReactRouterDOMError(error)) {
        // Suppress known React Router DOM manipulation errors
        console.warn('React Router navigation warning (suppressed):', error?.message || error)
        event.preventDefault()
        return false
      }
      return true
    }

    // Add global error handlers
    window.addEventListener('error', handleError, true)
    window.addEventListener('unhandledrejection', handleUnhandledRejection, true)
    
    // Cleanup
    return () => {
      window.removeEventListener('error', handleError, true)
      window.removeEventListener('unhandledrejection', handleUnhandledRejection, true)
    }
  }, [])

  return <RouterProvider router={router} />
}

const App = (
  <HelmetProvider>
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
  </HelmetProvider>
)

// Only enable StrictMode in development to avoid double-rendering issues
console.log('🚀 Starting React app initialization...')
console.log('Environment:', import.meta.env.MODE)

const rootElement = document.getElementById('root')

if (!rootElement) {
  console.error('❌ Root element not found! Make sure index.html has <div id="root"></div>')
  document.body.innerHTML = '<div style="color: red; padding: 20px; font-family: monospace;">Error: Root element not found. Please check the console for details.</div>'
} else {
  console.log('✅ Root element found')
  try {
    console.log('📦 Creating React root...')
    const root = ReactDOM.createRoot(rootElement)
    console.log('✅ React root created')
    
    console.log('🎨 Rendering app...')
    if (import.meta.env.DEV) {
      root.render(<React.StrictMode>{App}</React.StrictMode>)
    } else {
      root.render(App)
    }
    console.log('✅ App rendered successfully')
  } catch (error) {
    console.error('❌ Failed to render React app:', error)
    if (error instanceof Error) {
      console.error('Error stack:', error.stack)
    }
    rootElement.innerHTML = `
      <div style="color: #F6465D; padding: 20px; font-family: monospace; background: #0B0E11; min-height: 100vh; display: flex; align-items: center; justify-content: center;">
        <div style="max-width: 600px;">
          <h1 style="color: #F6465D; margin-bottom: 10px;">Failed to load application</h1>
          <p style="color: #848E9C; margin-bottom: 20px;">${error instanceof Error ? error.message : 'Unknown error'}</p>
          <button onclick="window.location.reload()" style="padding: 10px 20px; background: #F0B90B; color: #0B0E11; border: none; border-radius: 4px; cursor: pointer; font-weight: bold;">
            Reload Page
          </button>
        </div>
      </div>
    `
  }
}
