import React, { useState, useEffect } from 'react'
import {
  Radio,
  CheckCircle2,
  AlertCircle,
  Terminal,
  Copy,
  Send,
  RefreshCw,
  ShieldCheck,
  Globe,
  Wrench,
  Clock,
  Laptop,
  Code2,
  KeyRound,
  Network,
  Play,
  Square,
  Sparkles,
  Bot,
  Layers,
  FileText,
  ChevronDown,
  ChevronUp,
} from 'lucide-react'
import { api } from '../api'

interface BridgeStatusData {
  enabled: boolean
  operating_mode: string
  gateway_url: string
  tunnel_type: string
  tunnel_online: boolean
  auth_token: string
  pairing_pin: string
  surfaces: {
    desktop_app_connected: boolean
    desktop_cdp_port: number
    desktop_active_title?: string
    agy_cli_installed: boolean
    agy_cli_path?: string
    vscode_connected: boolean
    account_resilience: boolean
  }
  tunnel: {
    active: boolean
    type: string
    public_url?: string
    gateway_url: string
    error?: string
    logs?: string[]
  }
  active_lead_session?: {
    conversation_id: string
    title: string
    last_prompt: string
    submission_step: number
    status: string
    last_response?: string
    artifacts?: string[]
    updated_at: string
  }
  recent_attempts?: Array<{
    timestamp: string
    client_ip: string
    method: string
    granted: boolean
    reason: string
  }>
  invocations_count: number
  recent_invocations: Array<{
    id: string
    timestamp: string
    tool: string
    mode?: string
    args?: Record<string, any>
    duration_ms: number
    status: string
    result?: string
    error?: string
  }>
}

export const GeminiBridgePage: React.FC = () => {
  const [status, setStatus] = useState<BridgeStatusData | null>(null)
  const [loading, setLoading] = useState<boolean>(true)
  const [copiedUrl, setCopiedUrl] = useState<boolean>(false)
  const [copiedConfig, setCopiedConfig] = useState<boolean>(false)
  const [copiedToken, setCopiedToken] = useState<boolean>(false)
  const [copiedPIN, setCopiedPIN] = useState<boolean>(false)

  // Tunnel controls
  const [tunnelType, setTunnelType] = useState<string>('quick')
  const [customTunnelURL, setCustomTunnelURL] = useState<string>('')
  const [tunnelActionLoading, setTunnelActionLoading] = useState<boolean>(false)
  const [showTunnelLogs, setShowTunnelLogs] = useState<boolean>(false)

  // Mode and testing
  const [modeLoading, setModeLoading] = useState<boolean>(false)
  const [rotatingAuth, setRotatingAuth] = useState<boolean>(false)
  const [testPrompt, setTestPrompt] = useState<string>('List current project status and verify bridge connectivity')
  const [testTool, setTestTool] = useState<string>('antigravity_lead_create_conversation')
  const [dispatching, setDispatching] = useState<boolean>(false)
  const [dispatchResult, setDispatchResult] = useState<string | null>(null)
  const [testingConnection, setTestingConnection] = useState<boolean>(false)

  const loadStatus = async () => {
    try {
      const res = await api.getBridgeStatus()
      setStatus(res)
      if (res?.tunnel?.type) {
        setTunnelType(res.tunnel.type)
      }
    } catch (err) {
      console.error('Failed to load bridge status:', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadStatus()
    const timer = setInterval(() => {
      loadStatus()
    }, 6000)
    return () => clearInterval(timer)
  }, [])

  const handleTestConnection = async () => {
    try {
      setTestingConnection(true)
      const res = await api.testBridgeConnection()
      if (res && res.status) {
        setStatus(res.status)
      }
      setDispatchResult(res.message || 'Connection verified')
    } catch (err: any) {
      setDispatchResult(`Verification failed: ${err?.message || 'Network error'}`)
    } finally {
      setTestingConnection(false)
    }
  }

  const handleStartTunnel = async () => {
    try {
      setTunnelActionLoading(true)
      const res = await api.startBridgeTunnel({
        type: tunnelType,
        custom_url: customTunnelURL,
      })
      if (res?.status) {
        setStatus(res.status)
      } else {
        loadStatus()
      }
    } catch (err: any) {
      alert(`Tunnel failed to start: ${err?.message || 'Unknown error'}`)
    } finally {
      setTunnelActionLoading(false)
    }
  }

  const handleStopTunnel = async () => {
    try {
      setTunnelActionLoading(true)
      const res = await api.stopBridgeTunnel()
      if (res?.status) {
        setStatus(res.status)
      } else {
        loadStatus()
      }
    } catch (err: any) {
      alert(`Failed to stop tunnel: ${err?.message || 'Unknown error'}`)
    } finally {
      setTunnelActionLoading(false)
    }
  }

  const handleRotateCredentials = async () => {
    if (!confirm('Rotate bridge authentication token and pairing PIN? Remote clients must be updated with the new credentials.')) {
      return
    }
    try {
      setRotatingAuth(true)
      const res = await api.rotateBridgeToken()
      if (res?.status) {
        setStatus(res.status)
      } else {
        loadStatus()
      }
    } catch (err: any) {
      alert(`Credential rotation failed: ${err?.message || 'Unknown error'}`)
    } finally {
      setRotatingAuth(false)
    }
  }

  const handleSetMode = async (newMode: string) => {
    try {
      setModeLoading(true)
      const res = await api.setBridgeMode(newMode)
      if (res?.status) {
        setStatus(res.status)
      } else {
        loadStatus()
      }
    } catch (err: any) {
      alert(`Failed to set mode: ${err?.message || 'Unknown error'}`)
    } finally {
      setModeLoading(false)
    }
  }

  const handleDispatch = async () => {
    if (!testPrompt.trim()) return
    try {
      setDispatching(true)
      setDispatchResult(null)
      const res = await api.dispatchBridgeTask({
        prompt: testPrompt.trim(),
        target: testTool,
      })
      if (res.success) {
        setDispatchResult(res.result || 'Task dispatched successfully!')
        loadStatus()
      } else {
        setDispatchResult(`Error: ${res.error || 'Failed to dispatch task'}`)
      }
    } catch (err: any) {
      setDispatchResult(`Dispatch failed: ${err?.message || 'Server error'}`)
    } finally {
      setDispatching(false)
    }
  }

  const mcpConfigSnippet = JSON.stringify(
    {
      mcpServers: {
        antigravity: {
          url: status?.gateway_url || 'http://127.0.0.1:8765/api/bridge/mcp',
          headers: {
            Authorization: `Bearer ${status?.auth_token || 'swiss-gemini-bridge-auth-token'}`,
          },
        },
      },
    },
    null,
    2
  )

  const copyToClipboard = (text: string, setter: (val: boolean) => void) => {
    navigator.clipboard.writeText(text)
    setter(true)
    setTimeout(() => setter(false), 2000)
  }

  if (loading && !status) {
    return (
      <div style={{ padding: '32px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '13px' }}>
        Loading Gemini Bridge status...
      </div>
    )
  }

  const isTunnelActive = Boolean(status?.tunnel?.active && status.tunnel.type !== 'loopback')
  const currentMode = status?.operating_mode || 'hybrid'

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* 1. Header & Live Gateway Status */}
      <div
        style={{
          backgroundColor: '#ffffff',
          border: '1px solid var(--border)',
          borderRadius: '10px',
          padding: '20px',
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '12px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px', flexWrap: 'wrap' }}>
              <Radio size={18} color="var(--primary)" />
              <span style={{ fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                Gemini Bridge (F41 / M15)
              </span>
              <span
                style={{
                  backgroundColor: status?.enabled ? '#e6f4ea' : '#fce8e6',
                  color: status?.enabled ? '#137333' : '#c5221f',
                  padding: '2px 8px',
                  borderRadius: '10px',
                  fontSize: '11px',
                  fontWeight: 600,
                  whiteSpace: 'nowrap',
                }}
              >
                {status?.enabled ? 'Online (Streamable HTTP)' : 'Disabled'}
              </span>
              <span
                style={{
                  backgroundColor: isTunnelActive ? '#e8f0fe' : '#f1f3f4',
                  color: isTunnelActive ? '#1a73e8' : '#5f6368',
                  padding: '2px 8px',
                  borderRadius: '10px',
                  fontSize: '11px',
                  fontWeight: 600,
                  whiteSpace: 'nowrap',
                }}
              >
                {isTunnelActive ? `Remote Internet: ${status?.tunnel?.type.toUpperCase()}` : 'Local Intranet Only'}
              </span>
              <span
                style={{
                  backgroundColor: '#fef7e0',
                  color: '#b06000',
                  padding: '2px 8px',
                  borderRadius: '10px',
                  fontSize: '11px',
                  fontWeight: 600,
                  whiteSpace: 'nowrap',
                }}
              >
                Mode: {currentMode.toUpperCase()}
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '13px', color: 'var(--text-muted)' }}>
              Two-way agent orchestration connecting Gemini App / Web (Gemini Spark) to local Antigravity surfaces across account rotations.
            </p>
          </div>

          <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
            <button
              onClick={handleTestConnection}
              disabled={testingConnection}
              style={{
                backgroundColor: 'transparent',
                border: '1px solid var(--border)',
                borderRadius: '6px',
                padding: '6px 14px',
                fontSize: '12px',
                fontWeight: 600,
                color: 'var(--text)',
                cursor: testingConnection ? 'not-allowed' : 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                whiteSpace: 'nowrap',
              }}
            >
              <RefreshCw size={13} className={testingConnection ? 'animate-spin' : ''} />
              Verify Connection
            </button>
          </div>
        </div>

        {/* Live Gateway URL & Copy */}
        <div
          style={{
            marginTop: '16px',
            backgroundColor: '#f8f9fa',
            borderRadius: '8px',
            border: '1px solid var(--border)',
            padding: '12px 16px',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            flexWrap: 'wrap',
            gap: '12px',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: '320px', flex: 1 }}>
            <Globe size={16} color="var(--primary)" />
            <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
              Active Endpoint:
            </span>
            <code style={{ fontSize: '12px', color: 'var(--primary)', fontWeight: 600, wordBreak: 'break-all' }}>
              {status?.gateway_url || 'http://127.0.0.1:8765/api/bridge/mcp'}
            </code>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <button
              onClick={() => copyToClipboard(status?.gateway_url || '', setCopiedUrl)}
              style={{
                backgroundColor: copiedUrl ? '#e6f4ea' : '#ffffff',
                border: '1px solid var(--border)',
                borderRadius: '6px',
                padding: '5px 12px',
                fontSize: '12px',
                fontWeight: 600,
                color: copiedUrl ? '#137333' : 'var(--text)',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                whiteSpace: 'nowrap',
              }}
            >
              {copiedUrl ? <CheckCircle2 size={13} /> : <Copy size={13} />}
              {copiedUrl ? 'Copied' : 'Copy Endpoint'}
            </button>
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
              <ShieldCheck size={16} color="#137333" />
              <span style={{ fontSize: '12px', fontWeight: 600, color: '#137333', whiteSpace: 'nowrap' }}>
                Account Shield Active
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* 2. Zero-Setup Remote Tunneling */}
      <div
        style={{
          backgroundColor: '#ffffff',
          border: '1px solid var(--border)',
          borderRadius: '10px',
          padding: '20px',
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '12px', marginBottom: '14px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px' }}>
              <Network size={16} color="var(--primary)" />
              <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
                Remote Internet Access (Zero-Setup & Zero-Signup)
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)' }}>
              Connect from Gemini App on your mobile phone or web when away from home. Users require zero account signups and zero external configurations.
            </p>
          </div>

          <div style={{ display: 'flex', gap: '8px' }}>
            {isTunnelActive ? (
              <button
                onClick={handleStopTunnel}
                disabled={tunnelActionLoading}
                style={{
                  backgroundColor: '#fce8e6',
                  border: '1px solid #f5c2c7',
                  borderRadius: '6px',
                  padding: '7px 16px',
                  fontSize: '12px',
                  fontWeight: 600,
                  color: '#c5221f',
                  cursor: tunnelActionLoading ? 'not-allowed' : 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  whiteSpace: 'nowrap',
                }}
              >
                <Square size={13} />
                {tunnelActionLoading ? 'Stopping...' : 'Stop Remote Access'}
              </button>
            ) : (
              <button
                onClick={handleStartTunnel}
                disabled={tunnelActionLoading}
                style={{
                  backgroundColor: 'var(--primary)',
                  border: 'none',
                  borderRadius: '6px',
                  padding: '7px 16px',
                  fontSize: '12px',
                  fontWeight: 600,
                  color: '#ffffff',
                  cursor: tunnelActionLoading ? 'not-allowed' : 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  whiteSpace: 'nowrap',
                }}
              >
                <Play size={13} />
                {tunnelActionLoading ? 'Connecting...' : 'Start Remote Access'}
              </button>
            )}
          </div>
        </div>

        {/* Tunnel Selection Grid */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '12px', marginBottom: '12px' }}>
          <div
            onClick={() => setTunnelType('quick')}
            style={{
              padding: '12px',
              borderRadius: '8px',
              border: tunnelType === 'quick' ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: tunnelType === 'quick' ? '#f8fafd' : '#ffffff',
              cursor: 'pointer',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Quick Tunnel (Recommended)
              </span>
              <span style={{ fontSize: '11px', fontWeight: 600, color: '#137333', backgroundColor: '#e6f4ea', padding: '1px 6px', borderRadius: '4px' }}>
                Zero Signup
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '11px', color: 'var(--text-muted)' }}>
              Generates trusted public HTTPS URL automatically via bundled sidecar. No accounts or software installation required.
            </p>
          </div>

          <div
            onClick={() => setTunnelType('tailscale')}
            style={{
              padding: '12px',
              borderRadius: '8px',
              border: tunnelType === 'tailscale' ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: tunnelType === 'tailscale' ? '#f8fafd' : '#ffffff',
              cursor: 'pointer',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Tailscale Network
              </span>
              <span style={{ fontSize: '11px', fontWeight: 600, color: '#1a73e8', backgroundColor: '#e8f0fe', padding: '1px 6px', borderRadius: '4px' }}>
                Private VPN
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '11px', color: 'var(--text-muted)' }}>
              Routes through your personal Tailnet or Tailscale Funnel. Ideal if your phone or client is connected to Tailscale.
            </p>
          </div>

          <div
            onClick={() => setTunnelType('loopback')}
            style={{
              padding: '12px',
              borderRadius: '8px',
              border: tunnelType === 'loopback' ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: tunnelType === 'loopback' ? '#f8fafd' : '#ffffff',
              cursor: 'pointer',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Local Intranet (Loopback)
              </span>
              <span style={{ fontSize: '11px', fontWeight: 600, color: '#5f6368', backgroundColor: '#f1f3f4', padding: '1px 6px', borderRadius: '4px' }}>
                Home WiFi
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '11px', color: 'var(--text-muted)' }}>
              Strict loopback (127.0.0.1) and LAN access. Perfect for desktop testing or devices on the same home network.
            </p>
          </div>
        </div>

        {tunnelType === 'custom' && (
          <div style={{ display: 'flex', gap: '8px', marginTop: '10px' }}>
            <input
              type="text"
              placeholder="https://your-domain.com/api/bridge/mcp"
              value={customTunnelURL}
              onChange={(e) => setCustomTunnelURL(e.target.value)}
              style={{
                flex: 1,
                padding: '8px 12px',
                fontSize: '12px',
                borderRadius: '6px',
                border: '1px solid var(--border)',
                backgroundColor: '#ffffff',
              }}
            />
          </div>
        )}

        {/* Tunnel Status & Log Collapse */}
        <div style={{ marginTop: '10px' }}>
          <button
            onClick={() => setShowTunnelLogs(!showTunnelLogs)}
            style={{
              background: 'none',
              border: 'none',
              padding: '4px 0',
              fontSize: '11px',
              color: 'var(--text-muted)',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '4px',
              whiteSpace: 'nowrap',
            }}
          >
            {showTunnelLogs ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
            {showTunnelLogs ? 'Hide Connection Logs' : 'View Tunnel Diagnostics & Logs'}
          </button>

          {showTunnelLogs && (
            <pre
              style={{
                marginTop: '8px',
                padding: '12px',
                backgroundColor: '#f8f9fa',
                borderRadius: '6px',
                border: '1px solid var(--border)',
                fontSize: '11px',
                fontFamily: 'monospace',
                maxHeight: '140px',
                overflowY: 'auto',
                color: 'var(--text)',
              }}
            >
              {status?.tunnel?.logs && status.tunnel.logs.length > 0
                ? status.tunnel.logs.join('\n')
                : 'No tunnel logs yet. Click Start Remote Access to generate an HTTPS endpoint.'}
            </pre>
          )}
        </div>
      </div>

      {/* 3. Authentication & First-Time Setup */}
      <div
        style={{
          backgroundColor: '#ffffff',
          border: '1px solid var(--border)',
          borderRadius: '10px',
          padding: '20px',
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px', flexWrap: 'wrap', gap: '10px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px' }}>
              <KeyRound size={16} color="var(--primary)" />
              <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
                Authentication & Pairing Credentials
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)' }}>
              Required when initialising or accessing the MCP on Gemini. Unauthenticated requests are rejected with 401 Unauthorized.
            </p>
          </div>

          <div style={{ display: 'flex', gap: '8px' }}>
            <button
              onClick={handleRotateCredentials}
              disabled={rotatingAuth}
              style={{
                backgroundColor: 'transparent',
                border: '1px solid var(--border)',
                borderRadius: '6px',
                padding: '6px 12px',
                fontSize: '12px',
                fontWeight: 600,
                color: 'var(--text)',
                cursor: rotatingAuth ? 'not-allowed' : 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                whiteSpace: 'nowrap',
              }}
            >
              <RefreshCw size={12} className={rotatingAuth ? 'animate-spin' : ''} />
              Rotate Credentials
            </button>
            <button
              onClick={() => copyToClipboard(mcpConfigSnippet, setCopiedConfig)}
              style={{
                backgroundColor: copiedConfig ? '#e6f4ea' : 'var(--tonal)',
                border: '1px solid var(--border)',
                borderRadius: '6px',
                padding: '6px 12px',
                fontSize: '12px',
                fontWeight: 600,
                color: copiedConfig ? '#137333' : 'var(--text)',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                whiteSpace: 'nowrap',
              }}
            >
              {copiedConfig ? <CheckCircle2 size={13} /> : <Copy size={13} />}
              {copiedConfig ? 'Copied' : 'Copy JSON Config'}
            </button>
          </div>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '14px' }}>
          {/* Pairing PIN */}
          <div
            style={{
              padding: '14px',
              backgroundColor: '#f8f9fa',
              borderRadius: '8px',
              border: '1px solid var(--border)',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '6px' }}>
              <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
                Pairing PIN (Quick First-Time Setup)
              </span>
              <button
                onClick={() => copyToClipboard(status?.pairing_pin || '', setCopiedPIN)}
                style={{
                  background: 'none',
                  border: 'none',
                  cursor: 'pointer',
                  color: copiedPIN ? '#137333' : 'var(--primary)',
                  fontSize: '11px',
                  fontWeight: 600,
                  display: 'flex',
                  alignItems: 'center',
                  gap: '4px',
                  whiteSpace: 'nowrap',
                }}
              >
                {copiedPIN ? <CheckCircle2 size={12} /> : <Copy size={12} />}
                {copiedPIN ? 'Copied' : 'Copy PIN'}
              </button>
            </div>
            <div style={{ fontSize: '24px', letterSpacing: '4px', fontWeight: 700, color: 'var(--text)', fontFamily: 'monospace' }}>
              {status?.pairing_pin || '582914'}
            </div>
            <p style={{ margin: '6px 0 0', fontSize: '11px', color: 'var(--text-muted)' }}>
              Pass as header <code>X-Pairing-PIN</code> or URL query <code>?pin={status?.pairing_pin}</code>.
            </p>
          </div>

          {/* Bearer Token */}
          <div
            style={{
              padding: '14px',
              backgroundColor: '#f8f9fa',
              borderRadius: '8px',
              border: '1px solid var(--border)',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '6px' }}>
              <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
                Bearer Token (API Key)
              </span>
              <button
                onClick={() => copyToClipboard(status?.auth_token || '', setCopiedToken)}
                style={{
                  background: 'none',
                  border: 'none',
                  cursor: 'pointer',
                  color: copiedToken ? '#137333' : 'var(--primary)',
                  fontSize: '11px',
                  fontWeight: 600,
                  display: 'flex',
                  alignItems: 'center',
                  gap: '4px',
                  whiteSpace: 'nowrap',
                }}
              >
                {copiedToken ? <CheckCircle2 size={12} /> : <Copy size={12} />}
                {copiedToken ? 'Copied' : 'Copy Token'}
              </button>
            </div>
            <code style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', wordBreak: 'break-all' }}>
              {status?.auth_token || 'swiss-gemini-auth-token'}
            </code>
            <p style={{ margin: '6px 0 0', fontSize: '11px', color: 'var(--text-muted)' }}>
              Configured via standard <code>Authorization: Bearer &lt;token&gt;</code> HTTP header.
            </p>
          </div>
        </div>

        {/* JSON Snippet */}
        <div style={{ marginTop: '14px' }}>
          <pre
            style={{
              margin: 0,
              padding: '12px',
              backgroundColor: '#f8f9fa',
              borderRadius: '6px',
              border: '1px solid var(--border)',
              fontFamily: 'monospace',
              fontSize: '11px',
              color: 'var(--text)',
              overflowX: 'auto',
            }}
          >
            {mcpConfigSnippet}
          </pre>
        </div>
      </div>

      {/* 4. Operational Mode Selection (Direct vs Lead/Messenger) */}
      <div
        style={{
          backgroundColor: '#ffffff',
          border: '1px solid var(--border)',
          borderRadius: '10px',
          padding: '20px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px' }}>
          <Bot size={16} color="var(--primary)" />
          <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
            Gemini Operating Role & Mode
          </span>
        </div>
        <p style={{ margin: '0 0 16px', fontSize: '12px', color: 'var(--text-muted)' }}>
          Configure whether Gemini in your chat works directly on the local machine or acts as a lead/messenger dispatching tasks to Antigravity and forwarding responses.
        </p>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '14px' }}>
          {/* Hybrid Mode */}
          <div
            onClick={() => handleSetMode('hybrid')}
            style={{
              padding: '14px',
              borderRadius: '8px',
              border: currentMode === 'hybrid' ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: currentMode === 'hybrid' ? '#f8fafd' : '#ffffff',
              cursor: modeLoading ? 'not-allowed' : 'pointer',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '6px' }}>
              <Sparkles size={16} color="var(--primary)" />
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Hybrid Mode (Recommended)
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.4 }}>
              Gemini has access to both direct tools and lead tools. Gemini dynamically chooses whether to work directly or command Antigravity based on your instruction.
            </p>
          </div>

          {/* Lead / Messenger Mode */}
          <div
            onClick={() => handleSetMode('lead')}
            style={{
              padding: '14px',
              borderRadius: '8px',
              border: currentMode === 'lead' ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: currentMode === 'lead' ? '#f8fafd' : '#ffffff',
              cursor: modeLoading ? 'not-allowed' : 'pointer',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '6px' }}>
              <Layers size={16} color="var(--primary)" />
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Lead & Messenger Mode
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.4 }}>
              Gemini acts as your remote team lead. It creates agent conversations in Google Antigravity 2.0, monitors progress, and forwards the completed responses and artifacts back to your chat.
            </p>
          </div>

          {/* Direct Mode */}
          <div
            onClick={() => handleSetMode('direct')}
            style={{
              padding: '14px',
              borderRadius: '8px',
              border: currentMode === 'direct' ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: currentMode === 'direct' ? '#f8fafd' : '#ffffff',
              cursor: modeLoading ? 'not-allowed' : 'pointer',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '6px' }}>
              <Terminal size={16} color="var(--primary)" />
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Direct Actuator Mode
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.4 }}>
              Gemini interacts directly with your local files and terminal (executing shell commands, editing files, UI automation) without spawning Antigravity conversations.
            </p>
          </div>
        </div>

        {/* User Interaction Guide */}
        <div
          style={{
            marginTop: '14px',
            backgroundColor: '#f8f9fa',
            borderRadius: '8px',
            border: '1px solid var(--border)',
            padding: '12px 14px',
            fontSize: '12px',
          }}
        >
          <span style={{ fontWeight: 600, color: 'var(--text)' }}>
            How to direct Gemini in chat:
          </span>
          <ul style={{ margin: '6px 0 0', paddingLeft: '18px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
            <li>
              <strong>To command Antigravity:</strong> <em>"Ask Antigravity to refactor auth and run tests"</em> or <em>"Start a new Antigravity task to inspect bug #12"</em>.
            </li>
            <li>
              <strong>To work on PC directly:</strong> <em>"Work on my PC directly and check git log"</em> or <em>"Directly inspect file /pkg/bridge/bridge.go"</em>.
            </li>
          </ul>
        </div>
      </div>

      {/* 5. Live Antigravity Lead / Messenger Monitor */}
      {status?.active_lead_session && (
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '10px',
            padding: '20px',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px', flexWrap: 'wrap', gap: '8px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Layers size={16} color="var(--primary)" />
              <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
                Active Antigravity Lead Session
              </span>
              <span
                style={{
                  backgroundColor: status.active_lead_session.status === 'completed' ? '#e6f4ea' : '#e8f0fe',
                  color: status.active_lead_session.status === 'completed' ? '#137333' : '#1a73e8',
                  padding: '2px 8px',
                  borderRadius: '10px',
                  fontSize: '11px',
                  fontWeight: 600,
                  whiteSpace: 'nowrap',
                }}
              >
                {status.active_lead_session.status === 'completed' ? 'Completed & Forwarded' : 'Generating in Antigravity...'}
              </span>
            </div>
            <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
              Conversation: <code>{status.active_lead_session.conversation_id}</code>
            </span>
          </div>

          <div style={{ padding: '12px', backgroundColor: '#f8f9fa', borderRadius: '8px', border: '1px solid var(--border)', marginBottom: '10px' }}>
            <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '2px' }}>
              Last Dispatched Prompt:
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text)', fontWeight: 500 }}>
              {status.active_lead_session.last_prompt}
            </div>
          </div>

          {status.active_lead_session.last_response && (
            <div style={{ padding: '12px', backgroundColor: '#ffffff', borderRadius: '8px', border: '1px solid var(--border)' }}>
              <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                Forwarded Antigravity Response Preview:
              </div>
              <div style={{ fontSize: '12px', color: 'var(--text)', maxHeight: '120px', overflowY: 'auto', whiteSpace: 'pre-wrap', lineHeight: 1.4 }}>
                {status.active_lead_session.last_response}
              </div>
              {status.active_lead_session.artifacts && status.active_lead_session.artifacts.length > 0 && (
                <div style={{ marginTop: '8px', display: 'flex', gap: '6px', alignItems: 'center', flexWrap: 'wrap' }}>
                  <FileText size={13} color="var(--primary)" />
                  <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)' }}>Artifacts:</span>
                  {status.active_lead_session.artifacts.map((art) => (
                    <span
                      key={art}
                      style={{
                        padding: '1px 6px',
                        backgroundColor: '#f1f3f4',
                        borderRadius: '4px',
                        fontSize: '11px',
                        color: 'var(--text)',
                      }}
                    >
                      {art}
                    </span>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* 6. Surface Connectivity Matrix */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))',
          gap: '16px',
        }}
      >
        {/* Surface 1: Desktop App */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '10px',
            padding: '16px',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Laptop size={16} color="var(--primary)" />
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Antigravity 2.0 Desktop
              </span>
            </div>
            {status?.surfaces?.desktop_app_connected ? (
              <span style={{ fontSize: '11px', color: '#137333', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '4px' }}>
                <CheckCircle2 size={13} /> CDP Connected
              </span>
            ) : (
              <span style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 500, display: 'flex', alignItems: 'center', gap: '4px' }}>
                <AlertCircle size={13} /> Standby
              </span>
            )}
          </div>
          <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
            Port: <code style={{ color: 'var(--text)' }}>{status?.surfaces?.desktop_cdp_port || '35779'}</code>
            {status?.surfaces?.desktop_active_title && (
              <div style={{ marginTop: '4px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                Window: {status.surfaces.desktop_active_title}
              </div>
            )}
          </div>
        </div>

        {/* Surface 2: CLI */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '10px',
            padding: '16px',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Terminal size={16} color="var(--primary)" />
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Antigravity CLI (agy)
              </span>
            </div>
            {status?.surfaces?.agy_cli_installed ? (
              <span style={{ fontSize: '11px', color: '#137333', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '4px' }}>
                <CheckCircle2 size={13} /> Ready
              </span>
            ) : (
              <span style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 500 }}>
                Not on PATH
              </span>
            )}
          </div>
          <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
            Path: <code style={{ color: 'var(--text)' }}>{status?.surfaces?.agy_cli_path || '~/.local/bin/agy'}</code>
          </div>
        </div>

        {/* Surface 3: VS Code Extension */}
        <div
          style={{
            backgroundColor: '#ffffff',
            border: '1px solid var(--border)',
            borderRadius: '10px',
            padding: '16px',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Code2 size={16} color="var(--primary)" />
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                VS Code Extension
              </span>
            </div>
            <span style={{ fontSize: '11px', color: '#137333', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '4px' }}>
              <CheckCircle2 size={13} /> Port 41223
            </span>
          </div>
          <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
            Language Server socket active and federated.
          </div>
        </div>
      </div>

      {/* 7. Test Dispatch Console */}
      <div
        style={{
          backgroundColor: '#ffffff',
          border: '1px solid var(--border)',
          borderRadius: '10px',
          padding: '20px',
        }}
      >
        <div style={{ marginBottom: '12px' }}>
          <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
            Test Remote Task Dispatch
          </span>
          <p style={{ margin: '2px 0 0', fontSize: '12px', color: 'var(--text-muted)' }}>
            Simulate a remote request from Gemini App to steer local Antigravity.
          </p>
        </div>

        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginBottom: '12px' }}>
          <select
            value={testTool}
            onChange={(e) => setTestTool(e.target.value)}
            style={{
              padding: '8px 12px',
              fontSize: '12px',
              borderRadius: '6px',
              border: '1px solid var(--border)',
              backgroundColor: '#ffffff',
              color: 'var(--text)',
              outline: 'none',
            }}
          >
            <option value="antigravity_lead_create_conversation">Lead: Create New Conversation</option>
            <option value="antigravity_lead_send_message">Lead: Send Follow-Up Message</option>
            <option value="local_run_command">Direct: Run Local Command</option>
          </select>

          <input
            type="text"
            value={testPrompt}
            onChange={(e) => setTestPrompt(e.target.value)}
            placeholder="Type prompt to dispatch..."
            style={{
              flex: 1,
              minWidth: '240px',
              padding: '8px 12px',
              fontSize: '12px',
              borderRadius: '6px',
              border: '1px solid var(--border)',
              outline: 'none',
              backgroundColor: '#ffffff',
              color: 'var(--text)',
            }}
          />

          <button
            onClick={handleDispatch}
            disabled={dispatching || !testPrompt.trim()}
            style={{
              backgroundColor: 'var(--primary)',
              border: 'none',
              borderRadius: '6px',
              padding: '8px 18px',
              fontSize: '12px',
              fontWeight: 600,
              color: '#ffffff',
              cursor: dispatching ? 'not-allowed' : 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              whiteSpace: 'nowrap',
            }}
          >
            <Send size={13} />
            {dispatching ? 'Dispatching...' : 'Dispatch Task'}
          </button>
        </div>

        {dispatchResult && (
          <div
            style={{
              padding: '10px 14px',
              backgroundColor: '#f8f9fa',
              border: '1px solid var(--border)',
              borderRadius: '6px',
              fontSize: '12px',
              fontFamily: 'monospace',
              color: 'var(--text)',
              whiteSpace: 'pre-wrap',
            }}
          >
            {dispatchResult}
          </div>
        )}
      </div>

      {/* 8. Exposed Tools Catalog */}
      <div
        style={{
          backgroundColor: '#ffffff',
          border: '1px solid var(--border)',
          borderRadius: '10px',
          padding: '20px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '14px' }}>
          <Wrench size={16} color="var(--primary)" />
          <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
            Exposed Model Context Protocol Tools
          </span>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '12px' }}>
          {[
            {
              name: 'antigravity_lead_create_conversation',
              type: 'Lead',
              desc: 'Creates a new conversation in local Antigravity 2.0 app and submits initial task instructions.',
            },
            {
              name: 'antigravity_lead_send_message',
              type: 'Lead',
              desc: 'Sends follow-up directions, corrections, or task assignments to Antigravity conversation.',
            },
            {
              name: 'antigravity_lead_await_response',
              type: 'Lead',
              desc: 'Waits for Antigravity response turn and forwards completed response and artifacts to Gemini.',
            },
            {
              name: 'antigravity_lead_get_status',
              type: 'Lead',
              desc: 'Inspects real-time step status, thinking, and completion state of an active Antigravity session.',
            },
            {
              name: 'local_run_command',
              type: 'Direct',
              desc: 'Directly executes a safe shell command on the host machine with timeout protection.',
            },
            {
              name: 'local_read_file',
              type: 'Direct',
              desc: 'Reads file contents from the host filesystem within workspace bounds.',
            },
            {
              name: 'local_write_file',
              type: 'Direct',
              desc: 'Writes or updates file contents on the local computer.',
            },
            {
              name: 'local_computer_use',
              type: 'Direct',
              desc: 'Controls GUI and captures screenshots via display subsystem automation.',
            },
          ].map((tool) => (
            <div
              key={tool.name}
              style={{
                padding: '12px',
                border: '1px solid var(--border)',
                borderRadius: '8px',
                backgroundColor: '#ffffff',
              }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
                <code style={{ fontSize: '12px', fontWeight: 700, color: 'var(--primary)' }}>
                  {tool.name}
                </code>
                <span
                  style={{
                    fontSize: '10px',
                    fontWeight: 600,
                    padding: '1px 6px',
                    borderRadius: '4px',
                    backgroundColor: tool.type === 'Lead' ? '#e8f0fe' : '#e6f4ea',
                    color: tool.type === 'Lead' ? '#1a73e8' : '#137333',
                  }}
                >
                  {tool.type}
                </span>
              </div>
              <p style={{ margin: '4px 0 0', fontSize: '11px', color: 'var(--text-muted)', lineHeight: 1.4 }}>
                {tool.desc}
              </p>
            </div>
          ))}
        </div>
      </div>

      {/* 9. Recent Invocations & Auth Audit Logs */}
      <div
        style={{
          backgroundColor: '#ffffff',
          border: '1px solid var(--border)',
          borderRadius: '10px',
          padding: '20px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '14px' }}>
          <Clock size={16} color="var(--primary)" />
          <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
            Remote Invocations & Access Audit ({status?.invocations_count || 0})
          </span>
        </div>

        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '12px' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border)', textAlign: 'left', color: 'var(--text-muted)' }}>
                <th style={{ padding: '8px 10px' }}>Timestamp</th>
                <th style={{ padding: '8px 10px' }}>Tool / Event</th>
                <th style={{ padding: '8px 10px' }}>Mode</th>
                <th style={{ padding: '8px 10px' }}>Duration</th>
                <th style={{ padding: '8px 10px' }}>Status</th>
                <th style={{ padding: '8px 10px' }}>Result / Detail</th>
              </tr>
            </thead>
            <tbody>
              {(!status?.recent_invocations || status.recent_invocations.length === 0) ? (
                <tr>
                  <td colSpan={6} style={{ padding: '24px', textAlign: 'center', color: 'var(--text-muted)' }}>
                    No remote invocations yet. Use the Test Dispatch console above or invoke tools from Gemini App.
                  </td>
                </tr>
              ) : (
                status.recent_invocations.map((inv) => (
                  <tr key={inv.id} style={{ borderBottom: '1px solid #f1f3f4' }}>
                    <td style={{ padding: '8px 10px', color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
                      {inv.timestamp}
                    </td>
                    <td style={{ padding: '8px 10px', fontWeight: 600, color: 'var(--text)' }}>
                      <code>{inv.tool}</code>
                    </td>
                    <td style={{ padding: '8px 10px', color: 'var(--text-muted)' }}>
                      {inv.mode || 'hybrid'}
                    </td>
                    <td style={{ padding: '8px 10px', color: 'var(--text-muted)' }}>
                      {inv.duration_ms}ms
                    </td>
                    <td style={{ padding: '8px 10px' }}>
                      <span
                        style={{
                          backgroundColor: inv.status === 'success' ? '#e6f4ea' : '#fce8e6',
                          color: inv.status === 'success' ? '#137333' : '#c5221f',
                          padding: '2px 8px',
                          borderRadius: '10px',
                          fontSize: '11px',
                          fontWeight: 600,
                          whiteSpace: 'nowrap',
                        }}
                      >
                        {inv.status}
                      </span>
                    </td>
                    <td style={{ padding: '8px 10px', color: 'var(--text-muted)', maxWidth: '300px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {inv.result || inv.error || 'Done'}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}

export default GeminiBridgePage
