import { useEffect, useRef, useState } from 'react'
import type {
  JsonRpcError,
  JsonRpcResponse,
  MCPTool,
  McpToolsListResult,
} from '@/types/api'

export type MCPStatus = 'idle' | 'connecting' | 'connected' | 'error'

interface RPCResult<T = unknown> {
  result?: T
  error?: JsonRpcError
  timing?: number
}

function parseMCPBody<T = unknown>(text: string, contentType: string): JsonRpcResponse<T> {
  if (contentType.includes('application/json')) {
    return JSON.parse(text) as JsonRpcResponse<T>
  }
  const jsonLine = text.split('\n').find((line) => line.startsWith('data: '))?.slice(6)
  return (jsonLine ? JSON.parse(jsonLine) : {}) as JsonRpcResponse<T>
}

export function useMCP(endpoint = '/mcp') {
  const [status, setStatus] = useState<MCPStatus>('idle')
  const [tools, setTools] = useState<MCPTool[]>([])
  const [error, setError] = useState<string | null>(null)
  const sessionId = useRef<string | null>(null)
  const id = useRef(0)

  const headers = (): Record<string, string> => {
    const value: Record<string, string> = {
      'Content-Type': 'application/json',
      Accept: 'application/json, text/event-stream',
    }
    if (sessionId.current) value['Mcp-Session-Id'] = sessionId.current
    return value
  }

  const send = async <T = unknown>(body: unknown) => {
    const t0 = performance.now()
    const res = await fetch(endpoint, {
      method: 'POST',
      headers: headers(),
      body: JSON.stringify(body),
    })
    const sid = res.headers.get('Mcp-Session-Id')
    if (sid) sessionId.current = sid
    const text = await res.text()
    const data = parseMCPBody<T>(text, res.headers.get('content-type') ?? '')
    return { data, status: res.status, ms: Math.round(performance.now() - t0) }
  }

  const call = async <T = unknown>(method: string, params?: unknown): Promise<RPCResult<T>> => {
    try {
      const response = await send<T>({ jsonrpc: '2.0', id: ++id.current, method, params })
      return {
        result: response.data.result,
        error: response.data.error,
        timing: response.ms,
      }
    } catch (e) {
      return { error: { code: -1, message: String(e) } }
    }
  }

  const connect = async () => {
    setStatus('connecting')
    setError(null)
    const r = await call('initialize', {
      protocolVersion: '2024-11-05',
      capabilities: {},
      clientInfo: { name: 'otelcontext-ui', version: '1.0.0' },
    })
    if (r.error) {
      setStatus('error')
      setError(r.error.message)
      return
    }
    setStatus('connected')
    const r2 = await call<McpToolsListResult>('tools/list', {})
    if (!r2.error) setTools(r2.result?.tools ?? [])
  }

  useEffect(() => {
    void connect()
  }, [])

  return { status, tools, error, call, connect, send }
}
