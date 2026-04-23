import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { ErrorBoundary } from '../ErrorBoundary'

function Boom({ shouldThrow }: { shouldThrow: boolean }) {
  if (shouldThrow) {
    throw new Error('kaboom-from-child')
  }
  return <div>child-ok</div>
}

/**
 * Test harness that lets the test toggle whether the child throws without
 * remounting ErrorBoundary. Used to verify that "Try again" can recover when
 * the underlying cause is fixed.
 */
function Harness() {
  const [shouldThrow, setShouldThrow] = useState(true)
  return (
    <div>
      <button type="button" onClick={() => setShouldThrow(false)}>
        fix-it
      </button>
      <ErrorBoundary>
        <Boom shouldThrow={shouldThrow} />
      </ErrorBoundary>
    </div>
  )
}

describe('ErrorBoundary', () => {
  let errorSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    // Suppress React's own error logging + our boundary's console.error so
    // test output stays clean. We still assert behavior via the DOM.
    errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    errorSpy.mockRestore()
  })

  it('renders children when there is no error', () => {
    render(
      <ErrorBoundary>
        <Boom shouldThrow={false} />
      </ErrorBoundary>,
    )
    expect(screen.getByText('child-ok')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('renders the fallback UI when a child throws', () => {
    render(
      <ErrorBoundary>
        <Boom shouldThrow={true} />
      </ErrorBoundary>,
    )
    const alert = screen.getByRole('alert')
    expect(alert).toBeInTheDocument()
    expect(screen.getByText('Something went wrong')).toBeInTheDocument()
    expect(alert).toHaveTextContent('kaboom-from-child')
    expect(
      screen.getByRole('button', { name: /try again/i }),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: /reload page/i }),
    ).toBeInTheDocument()
  })

  it('logs the error with a [ErrorBoundary] tag', () => {
    render(
      <ErrorBoundary>
        <Boom shouldThrow={true} />
      </ErrorBoundary>,
    )
    const calls = errorSpy.mock.calls as unknown[][]
    const tagged = calls.find(
      (args) => typeof args[0] === 'string' && (args[0] as string).includes('[ErrorBoundary]'),
    )
    expect(tagged).toBeDefined()
  })

  it('"Try again" resets state and re-renders children once the cause is fixed', async () => {
    const user = userEvent.setup()
    render(<Harness />)

    // Initial render shows fallback because the child threw.
    expect(screen.getByRole('alert')).toBeInTheDocument()

    // Fix the underlying cause, then click "Try again".
    await user.click(screen.getByRole('button', { name: /fix-it/i }))
    await user.click(screen.getByRole('button', { name: /try again/i }))

    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByText('child-ok')).toBeInTheDocument()
  })
})
