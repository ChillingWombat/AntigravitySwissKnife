import React, { type ErrorInfo, type ReactNode } from 'react'
import { AlertTriangle, RotateCw } from 'lucide-react'

interface Props {
  children?: ReactNode
  fallbackTitle?: string
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends React.Component<Props, State> {
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
            backgroundColor: 'var(--surface, #ffffff)',
            borderRadius: '8px',
            border: '1px solid var(--border-danger)',
            maxWidth: '640px',
            margin: '24px auto',
            textAlign: 'center',
            boxShadow: 'var(--shadow-sm)',
          }}
        >
          <div
            style={{
              width: '48px',
              height: '48px',
              borderRadius: '24px',
              backgroundColor: 'var(--red-bg, #fee2e2)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              margin: '0 auto 16px',
              color: 'var(--red, #dc2626)',
            }}
          >
            <AlertTriangle size={24} />
          </div>
          <h3 style={{ fontSize: '16px', fontWeight: 600, color: 'var(--text, #111827)', margin: '0 0 8px' }}>
            {this.props.fallbackTitle || 'Component Error'}
          </h3>
          <p style={{ fontSize: '13px', color: 'var(--text-muted, #6b7280)', margin: '0 0 16px', wordBreak: 'break-word' }}>
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
              color: 'var(--on-primary, #ffffff)',
              backgroundColor: 'var(--primary, #1a73e8)',
              border: 'none',
              borderRadius: '6px',
              cursor: 'pointer',
              whiteSpace: 'nowrap',
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
