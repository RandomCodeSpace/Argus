import { Component, type ErrorInfo, type ReactNode } from 'react'

type Props = { children: ReactNode }
type State = { error: Error | null; info: ErrorInfo | null }

/**
 * Production-grade top-level error boundary.
 *
 * Catches render/lifecycle errors anywhere in the React tree and renders a
 * friendly recovery UI instead of a blank page. Logs the error + component
 * stack to the console with a `[ErrorBoundary]` tag for triage.
 *
 * NOTE ON TELEMETRY FORWARDING: `useWebSocket` in this codebase is currently
 * receive-only (server pushes log batches to client over `/ws`). There is no
 * client->server send API exposed from the hook, so we do NOT forward errors
 * over WebSocket here to avoid adding coupling. When a bidirectional telemetry
 * channel is introduced, wire the `componentDidCatch` branch below.
 */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null, info: null }

  static getDerivedStateFromError(error: Error): Partial<State> {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    this.setState({ info })
    // eslint-disable-next-line no-console
    console.error('[ErrorBoundary] Uncaught error in React tree', {
      message: error.message,
      name: error.name,
      stack: error.stack,
      componentStack: info.componentStack,
      url: typeof window !== 'undefined' ? window.location.href : undefined,
      userAgent: typeof navigator !== 'undefined' ? navigator.userAgent : undefined,
      timestamp: new Date().toISOString(),
    })
    // TODO(telemetry): forward to server when useWebSocket exposes a send()
    // API, or via a dedicated POST /api/client-errors endpoint.
  }

  private reset = (): void => {
    this.setState({ error: null, info: null })
  }

  private reload = (): void => {
    if (typeof window !== 'undefined') {
      window.location.reload()
    }
  }

  render(): ReactNode {
    const { error, info } = this.state
    if (!error) return this.props.children

    // Inline styles ONLY — if Mantine/global CSS failed to load and is the
    // root cause, the fallback must still render correctly.
    return (
      <div
        role="alert"
        aria-live="assertive"
        style={{
          position: 'fixed',
          inset: 0,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          padding: '24px',
          background: 'var(--bg-base, #000)',
          color: 'var(--text-primary, #fff)',
          fontFamily:
            '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen, Ubuntu, sans-serif',
          zIndex: 9999,
          overflow: 'auto',
        }}
      >
        <div
          style={{
            width: '100%',
            maxWidth: '640px',
            background: 'var(--bg-card, #0a0a0a)',
            border: '1px solid var(--border, #1f1f1f)',
            borderRadius: '8px',
            padding: '32px',
            boxShadow: '0 10px 40px rgba(0, 0, 0, 0.5)',
          }}
        >
          <div
            style={{
              fontSize: '12px',
              fontWeight: 600,
              letterSpacing: '0.08em',
              textTransform: 'uppercase',
              color: 'var(--accent-error, #ff4444)',
              marginBottom: '12px',
            }}
          >
            Application error
          </div>
          <h1
            style={{
              fontSize: '24px',
              fontWeight: 600,
              margin: '0 0 12px 0',
              color: 'var(--text-primary, #fff)',
            }}
          >
            Something went wrong
          </h1>
          <p
            style={{
              fontSize: '14px',
              lineHeight: 1.6,
              color: 'var(--text-secondary, #ccc)',
              margin: '0 0 20px 0',
            }}
          >
            The UI encountered an unexpected error and could not continue
            rendering. You can try recovering without a full reload, or refresh
            the page if that fails.
          </p>

          <div
            style={{
              background: 'var(--code-bg, #050505)',
              border: '1px solid var(--border, #1f1f1f)',
              borderRadius: '6px',
              padding: '12px 14px',
              marginBottom: '20px',
              fontFamily:
                'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
              fontSize: '13px',
              color: 'var(--code-text, #ccc)',
              wordBreak: 'break-word',
            }}
          >
            <span style={{ color: 'var(--accent-error, #ff4444)' }}>
              {error.name || 'Error'}
            </span>
            : {error.message || '(no message)'}
          </div>

          {info?.componentStack && (
            <details
              style={{
                marginBottom: '24px',
                fontSize: '12px',
                color: 'var(--text-muted, #666)',
              }}
            >
              <summary
                style={{
                  cursor: 'pointer',
                  userSelect: 'none',
                  padding: '4px 0',
                  color: 'var(--text-secondary, #ccc)',
                }}
              >
                Component stack
              </summary>
              <pre
                style={{
                  marginTop: '8px',
                  padding: '12px',
                  background: 'var(--code-bg, #050505)',
                  border: '1px solid var(--border, #1f1f1f)',
                  borderRadius: '6px',
                  fontFamily:
                    'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
                  fontSize: '11px',
                  lineHeight: 1.5,
                  color: 'var(--code-text, #ccc)',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word',
                  maxHeight: '240px',
                  overflow: 'auto',
                }}
              >
                {info.componentStack}
              </pre>
            </details>
          )}

          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap' }}>
            <button
              type="button"
              onClick={this.reset}
              style={{
                appearance: 'none',
                border: '1px solid var(--color-accent, #38bdf8)',
                background: 'var(--color-accent, #38bdf8)',
                color: '#000',
                padding: '10px 18px',
                borderRadius: '6px',
                fontSize: '14px',
                fontWeight: 600,
                cursor: 'pointer',
                transition: 'background 120ms ease',
              }}
            >
              Try again
            </button>
            <button
              type="button"
              onClick={this.reload}
              style={{
                appearance: 'none',
                border: '1px solid var(--border-strong, #333)',
                background: 'transparent',
                color: 'var(--text-primary, #fff)',
                padding: '10px 18px',
                borderRadius: '6px',
                fontSize: '14px',
                fontWeight: 500,
                cursor: 'pointer',
                transition: 'border-color 120ms ease',
              }}
            >
              Reload page
            </button>
          </div>
        </div>
      </div>
    )
  }
}

export default ErrorBoundary
