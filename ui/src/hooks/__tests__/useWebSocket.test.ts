import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'

// Lightweight WebSocket mock local to this test file — no global setup
// changes. Tracks all instances so assertions can reach into the last
// constructed socket.
class MockWebSocket {
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSING = 2
  static readonly CLOSED = 3

  static readonly instances: MockWebSocket[] = []

  readyState = MockWebSocket.CONNECTING
  url: string
  onopen: ((ev: Event) => void) | null = null
  onmessage: ((ev: MessageEvent<string>) => void) | null = null
  onerror: ((ev: Event) => void) | null = null
  onclose: ((ev: CloseEvent) => void) | null = null
  send = vi.fn<(data: string) => void>()
  close = vi.fn<() => void>(() => {
    this.readyState = MockWebSocket.CLOSED
    // Fire close handler asynchronously, mirroring the browser.
    queueMicrotask(() => this.onclose?.(new CloseEvent('close')))
  })

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }

  // Test helpers ---------------------------------------------------
  simulateOpen() {
    this.readyState = MockWebSocket.OPEN
    this.onopen?.(new Event('open'))
  }

  simulateClose() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.(new CloseEvent('close'))
  }

  simulateMessage(data: unknown) {
    const ev = new MessageEvent<string>('message', { data: JSON.stringify(data) })
    this.onmessage?.(ev)
  }
}

const OriginalWebSocket = globalThis.WebSocket

beforeEach(() => {
  MockWebSocket.instances.length = 0
  vi.stubGlobal('WebSocket', MockWebSocket as unknown as typeof WebSocket)
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  globalThis.WebSocket = OriginalWebSocket
})

// Import after the stub is set up so the module picks up the mocked
// WebSocket global references (it doesn't — the hook reads the global
// fresh inside the effect — but resetting order keeps things tidy).
import { useWebSocket } from '../useWebSocket'

const latestSocket = () =>
  MockWebSocket.instances[MockWebSocket.instances.length - 1]

describe('useWebSocket', () => {
  it('transitions connecting → connected on open', () => {
    const onLogs = vi.fn()
    const { result } = renderHook(() => useWebSocket(onLogs))

    // One instance created synchronously.
    expect(MockWebSocket.instances).toHaveLength(1)
    expect(result.current.status).toBe('connecting')

    act(() => {
      latestSocket().simulateOpen()
    })

    expect(result.current.status).toBe('connected')
    expect(result.current.current).toBe(latestSocket())
  })

  it('reconnects after a server-initiated close', () => {
    const onLogs = vi.fn()
    const { result } = renderHook(() => useWebSocket(onLogs))

    act(() => {
      latestSocket().simulateOpen()
    })
    expect(result.current.status).toBe('connected')

    act(() => {
      latestSocket().simulateClose()
    })
    expect(result.current.status).toBe('reconnecting')
    expect(MockWebSocket.instances).toHaveLength(1)

    // First reconnect attempt fires at 100ms (2^0 * 100).
    act(() => {
      vi.advanceTimersByTime(100)
    })
    expect(MockWebSocket.instances).toHaveLength(2)
  })

  it('applies exponential backoff across repeated failures', () => {
    const onLogs = vi.fn()
    renderHook(() => useWebSocket(onLogs))

    // Attempt 1: close immediately → 100ms delay to create a new socket.
    act(() => {
      latestSocket().simulateClose()
    })
    act(() => {
      vi.advanceTimersByTime(99)
    })
    expect(MockWebSocket.instances).toHaveLength(1)
    act(() => {
      vi.advanceTimersByTime(1)
    })
    expect(MockWebSocket.instances).toHaveLength(2)

    // Attempt 2: close → 200ms.
    act(() => {
      latestSocket().simulateClose()
    })
    act(() => {
      vi.advanceTimersByTime(199)
    })
    expect(MockWebSocket.instances).toHaveLength(2)
    act(() => {
      vi.advanceTimersByTime(1)
    })
    expect(MockWebSocket.instances).toHaveLength(3)

    // Attempt 3: close → 400ms.
    act(() => {
      latestSocket().simulateClose()
    })
    act(() => {
      vi.advanceTimersByTime(399)
    })
    expect(MockWebSocket.instances).toHaveLength(3)
    act(() => {
      vi.advanceTimersByTime(1)
    })
    expect(MockWebSocket.instances).toHaveLength(4)
  })

  it('sends a ping heartbeat 30s after open', () => {
    const onLogs = vi.fn()
    renderHook(() => useWebSocket(onLogs))

    act(() => {
      latestSocket().simulateOpen()
    })
    expect(latestSocket().send).not.toHaveBeenCalled()

    act(() => {
      vi.advanceTimersByTime(30_000)
    })

    expect(latestSocket().send).toHaveBeenCalledTimes(1)
    const sent = latestSocket().send.mock.calls[0][0]
    expect(JSON.parse(sent)).toEqual({ type: 'ping' })
  })

  it('clears timers and stops reconnecting on unmount', () => {
    const onLogs = vi.fn()
    const { unmount } = renderHook(() => useWebSocket(onLogs))

    act(() => {
      latestSocket().simulateOpen()
    })
    const socket = latestSocket()

    unmount()

    expect(socket.close).toHaveBeenCalled()

    // Advance well past any pending heartbeat or reconnect backoff — no
    // new sockets should be created.
    const initialCount = MockWebSocket.instances.length
    act(() => {
      vi.advanceTimersByTime(60_000)
    })
    expect(MockWebSocket.instances).toHaveLength(initialCount)
  })
})
