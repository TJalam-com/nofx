import { Component, ErrorInfo, ReactNode } from 'react'
import { AlertTriangle } from 'lucide-react'

interface Props {
  children?: ReactNode
  fallback?: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    // Suppress known React Router DOM manipulation errors that don't affect functionality
    // This can occur from React Router navigation or browser extensions (like installHook.js)
    if (
      error.name === 'NotFoundError' &&
      error.message.includes('removeChild') &&
      (error.message.includes('not a child of this node') || 
       error.message.includes('not a child'))
    ) {
      // This is a known issue with React Router DOM manipulation during navigation
      // It doesn't affect functionality, so we suppress it
      console.warn('React Router navigation warning (suppressed):', error.message)
      return { hasError: false, error: null }
    }
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    // Suppress known React Router DOM manipulation errors
    // This can occur from React Router navigation or browser extensions (like installHook.js)
    if (
      error.name === 'NotFoundError' &&
      error.message.includes('removeChild') &&
      (error.message.includes('not a child of this node') || 
       error.message.includes('not a child'))
    ) {
      console.warn('React Router navigation warning (suppressed):', error.message)
      return
    }
    console.error('ErrorBoundary caught an error:', error, errorInfo)
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback
      }

      return (
        <div
          className="min-h-screen flex items-center justify-center p-6"
          style={{ background: '#0B0E11', color: '#EAECEF' }}
        >
          <div className="max-w-md w-full">
            <div
              className="rounded-lg border p-6"
              style={{
                background: 'rgba(246, 70, 93, 0.1)',
                borderColor: 'rgba(246, 70, 93, 0.3)',
              }}
            >
              <div className="flex items-center gap-3 mb-4">
                <AlertTriangle className="w-6 h-6" style={{ color: '#F6465D' }} />
                <h2 className="text-xl font-bold" style={{ color: '#F6465D' }}>
                  Something went wrong
                </h2>
              </div>
              <p className="text-sm mb-4" style={{ color: '#848E9C' }}>
                {this.state.error?.message || 'An unexpected error occurred'}
              </p>
              <button
                onClick={() => {
                  this.setState({ hasError: false, error: null })
                  window.location.reload()
                }}
                className="px-4 py-2 rounded font-semibold transition-all hover:scale-105"
                style={{
                  background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
                  color: '#0B0E11',
                }}
              >
                Reload Page
              </button>
            </div>
          </div>
        </div>
      )
    }

    return this.props.children || null
  }
}

