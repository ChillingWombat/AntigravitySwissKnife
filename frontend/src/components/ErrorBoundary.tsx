import { Component, type ErrorInfo, type ReactNode } from 'react'
import { AlertTriangle, RotateCw } from 'lucide-react'

interface Props {
  children: ReactNode
  fallbackTitle?: string
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  public override state: State = {
    hasError: false,
    error: null,
  }

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  public override componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('ErrorBoundary caught an unhandled error:', error, errorInfo)
  }

  private handleReset = () => {
    this.setState({ hasError: false, error: null })
  }

  public override render() {
    if (this.state.hasError) {
      return (
        <div
          style={{
            padding: '32px 24px',
            backgroundColor: '#ffffff',
            borderRadius: '8px',
            border: '1px solid #fecaca',
            maxWidth: '640px',
            margin: '24px auto',
            textAlign: 'center',
            boxShadow: '0 1px 3px rgba(0,0,0,0.05)',
          }}
        >
          <div
            style={{
              width: '48px',
              height: '48px',
              borderRadius: '24px',
              backgroundColor: '#fee2e2',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              margin: '0 auto 16px',
              color: '#dc2626',
            }}
          >
            <AlertTriangle size={24} />
          </div>
          <h3 style={{ fontSize: '16px', fontWeight: 600, color: '#111827', margin: '0 0 8px' }}>
            {this.props.fallbackTitle || 'Component Error'}
          </h3>
          <p style={{ fontSize: '13px', color: '#6b7280', margin: '0 0 16px', wordBreak: 'break-word' }}>
            {this.state.error?.message || 'An unexpected rendering error occurred.'}
          </p>
          <button
            onClick={this.handleReset}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: '6px',
              padding: '8px 16px',
              fontSize: '13px',
              fontWeight: 500,
              color: '#ffffff',
              backgroundColor: 'var(--primary, #1a73e8)',
              border: 'none',
              borderRadius: '6px',
              cursor: 'pointer',
            }}
          >
            <RotateCw size={14} />
            Try Again
          </button>
        </div>
      )
    }

    return this.props.children
  }
}
