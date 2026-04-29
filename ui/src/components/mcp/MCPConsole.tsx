import { useState } from 'react'
import { Alert, Badge, Button, Card, CodeBlock, Grid, Input, Space } from '@ossrandom/design-system'
import { Check, Copy, Terminal } from 'lucide-react'

const exampleListTools = `{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list"
}`

const exampleToolCall = `{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "get_service_map",
    "arguments": { "depth": 2 }
  }
}`

function curlSnippet(url: string): string {
  return `curl -N \\
  -H "Content-Type: application/json" \\
  -H "Accept: application/json, text/event-stream" \\
  -d '${exampleListTools.replace(/\n\s*/g, ' ').trim()}' \\
  ${url}`
}

export default function MCPConsole() {
  const url = `${window.location.origin}/mcp`
  const [copied, setCopied] = useState(false)

  const copyUrl = async () => {
    await navigator.clipboard.writeText(url)
    setCopied(true)
    window.setTimeout(() => setCopied(false), 1500)
  }

  return (
    <Space direction="vertical" size="lg">
      <Card
        bordered
        padding="md"
        radius="md"
        title={
          <Space size="xs" align="center">
            <Terminal size={14} />
            <span>MCP Endpoint</span>
          </Space>
        }
        subtitle="HTTP Streamable MCP · JSON-RPC 2.0 · Server-Sent Events"
        extra={<Badge tone="info" size="sm">live</Badge>}
      >
        <Space direction="vertical" size="md">
          <Alert severity="info">
            Point any MCP-compatible client (Claude Desktop, Cursor, custom agents) at the URL below.
            Authentication: send <code>Authorization: Bearer &lt;API_KEY&gt;</code> if <code>API_KEY</code> is set.
          </Alert>

          <Space direction="vertical" size="xs">
            <Input value={url} readOnly type="url" />
            <Space justify="end">
              <Button
                variant="primary"
                size="sm"
                iconLeft={copied ? <Check size={12} /> : <Copy size={12} />}
                onClick={copyUrl}
              >
                {copied ? 'Copied' : 'Copy URL'}
              </Button>
            </Space>
          </Space>
        </Space>
      </Card>

      <Grid columns={12} gap="md">
        <Grid.Col span={4}>
          <Card bordered padding="md" radius="md" title="Discover tools" subtitle="JSON-RPC body">
            <CodeBlock language="json" code={exampleListTools} copyable />
          </Card>
        </Grid.Col>
        <Grid.Col span={4}>
          <Card bordered padding="md" radius="md" title="Invoke a tool" subtitle="JSON-RPC body">
            <CodeBlock language="json" code={exampleToolCall} copyable />
          </Card>
        </Grid.Col>
        <Grid.Col span={4}>
          <Card bordered padding="md" radius="md" title="curl example" subtitle="POST + SSE-aware Accept">
            <CodeBlock language="bash" code={curlSnippet(url)} copyable />
          </Card>
        </Grid.Col>
      </Grid>
    </Space>
  )
}
