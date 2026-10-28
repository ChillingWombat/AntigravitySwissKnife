import React, { useState, useEffect } from 'react'
import {
  Radio,
  CheckCircle2,
  AlertCircle,
  Terminal,
  Copy,
  RefreshCw,
  Bot,
  Layers,
  FileText,
  ChevronDown,
  ChevronUp,
  Check,
  Ban,
  Plus,
  X,
  Info,
  Globe,
} from 'lucide-react'
import { api, isAbortError } from '../api'
import { ToggleSwitch } from '../components/ToggleSwitch'

interface BridgeStatusData {
  enabled: boolean
  operating_mode: string
  gateway_url: string
  tunnel_type: string
  tunnel_online: boolean
  auth_token: string
  pairing_pin: string
  oauth_client_id?: string
  oauth_client_secret?: string
  use_app_password?: boolean
  app_password_enabled?: boolean
  agent_backend?: string
  resolved_agent?: string
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
  http_allowlist_mode?: 'always_allow' | 'whitelist_only'
  http_allowlist?: string[]
  http_pending_approvals?: Array<{
    host: string
    method?: string
    url?: string
    reason?: string
    requested_at: string
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
  const [copiedClientId, setCopiedClientId] = useState<boolean>(false)
  const [actionFeedback, setActionFeedback] = useState<{ text: string; isError: boolean } | null>(null)
  const [newAllowHost, setNewAllowHost] = useState<string>('')
  const [agentLoading, setAgentLoading] = useState<boolean>(false)

  // Tunnel controls
  const [tunnelType, setTunnelType] = useState<string>('quick')
  const [customTunnelURL, setCustomTunnelURL] = useState<string>('')
  const [tunnelActionLoading, setTunnelActionLoading] = useState<boolean>(false)
  const [showTunnelLogs, setShowTunnelLogs] = useState<boolean>(false)

  // Mode and testing
  const [modeLoading, setModeLoading] = useState<boolean>(false)
  const [pinModeLoading, setPinModeLoading] = useState<boolean>(false)
  const [rotatingAuth, setRotatingAuth] = useState<boolean>(false)
  const wasTunnelActiveRef = React.useRef<boolean>(false)

  const loadStatus = async () => {
    try {
      const res = await api.getBridgeStatus()
      const nowActive = Boolean(res?.tunnel?.active && res?.tunnel?.type !== 'loopback')
      if (wasTunnelActiveRef.current && !nowActive && res?.tunnel?.error) {
        setActionFeedback({
          text: `Tunnel stopped automatically: ${res.tunnel.error}`,
          isError: true,
        })
      }
      wasTunnelActiveRef.current = nowActive
      setStatus(res)
      if (res?.tunnel?.type) {
        setTunnelType(res.tunnel.type)
      }
      if (res?.tunnel?.type === 'custom' && res?.tunnel?.gateway_url) {
        setCustomTunnelURL((prev) => prev || res.tunnel.gateway_url)
      }
    } catch (err) {
      if (isAbortError(err)) return
      console.error('Failed to load bridge status:', err)
    } finally {
      setLoading(false)
    }
  }

  const allowlistMode = status?.http_allowlist_mode ?? 'always_allow'
  const allowlist = status?.http_allowlist ?? []
  const pendingAllow = status?.http_pending_approvals ?? []

  const handleAllowlistModeChange = async (mode: 'always_allow' | 'whitelist_only') => {
    try {
      setActionFeedback(null)
      await api.setBridgeHTTPAllowlistMode(mode)
      await loadStatus()
      setActionFeedback({
        text: mode === 'always_allow' ? 'Outbound HTTP set to Always Allow' : 'Outbound HTTP set to Whitelist Only',
        isError: false,
      })
    } catch (err) {
      if (isAbortError(err)) return
      setActionFeedback({ text: `Mode update failed: ${(err as Error).message}`, isError: true })
    }
  }

  const handleAllowHostChange = async (action: 'allow' | 'revoke' | 'deny', host: string) => {
    try {
      setActionFeedback(null)
      await api.updateBridgeHTTPAllowlist(action, host)
      await loadStatus()
      setActionFeedback({
        text: action === 'allow' ? `${host} allowed` : action === 'deny' ? `${host} denied` : `${host} revoked`,
        isError: false,
      })
    } catch (err) {
      if (isAbortError(err)) return
      setActionFeedback({ text: `Allowlist update failed: ${(err as Error).message}`, isError: true })
    }
  }

  useEffect(() => {
    loadStatus()
    const timer = setInterval(() => {
      loadStatus()
    }, 6000)
    return () => clearInterval(timer)
  }, [])

  const handleSetAgent = async (agent: 'auto' | 'app' | 'cli') => {
    try {
      setAgentLoading(true)
      setActionFeedback(null)
      await api.setBridgeAgent(agent)
      await loadStatus()
      setActionFeedback({ text: `Default agent set to ${agent === 'auto' ? 'Automatic' : agent === 'app' ? 'Antigravity 2.0' : 'Antigravity CLI'}`, isError: false })
    } catch (err) {
      if (isAbortError(err)) return
      setActionFeedback({ text: `Agent switch failed: ${(err as Error).message}`, isError: true })
    } finally {
      setAgentLoading(false)
    }
  }

  const handleStartTunnel = async () => {
    try {
      setTunnelActionLoading(true)
      setActionFeedback(null)
      const res = await api.startBridgeTunnel({
        type: tunnelType,
        custom_url: customTunnelURL,
      })
      if (res && res.success === false) {
        throw new Error((res as any).error || 'Tunnel operation failed')
      }
      if (res?.status) {
        setStatus(res.status)
      } else {
        loadStatus()
      }
      setActionFeedback({ text: 'Remote access tunnel started successfully.', isError: false })
    } catch (err: any) {
      setActionFeedback({ text: `Tunnel failed to start: ${err?.message || 'Unknown error'}`, isError: true })
    } finally {
      setTunnelActionLoading(false)
    }
  }

  const handleStopTunnel = async () => {
    try {
      setTunnelActionLoading(true)
      setActionFeedback(null)
      const res = await api.stopBridgeTunnel()
      if (res && res.success === false) {
        throw new Error((res as any).error || 'Tunnel operation failed')
      }
      if (res?.status) {
        setStatus(res.status)
      } else {
        loadStatus()
      }
      setActionFeedback({ text: 'Remote access tunnel stopped.', isError: false })
    } catch (err: any) {
      setActionFeedback({ text: `Failed to stop tunnel: ${err?.message || 'Unknown error'}`, isError: true })
    } finally {
      setTunnelActionLoading(false)
    }
  }

  const handleRotateCredentials = async () => {
    try {
      setRotatingAuth(true)
      setActionFeedback(null)
      const res = await api.rotateBridgeToken()
      if (res && (res.status || res.auth_token || res.pairing_pin)) {
        const authToken = res.auth_token || res.status?.auth_token
        const pairingPin = res.pairing_pin || res.status?.pairing_pin
        setStatus((prev: any) => ({
          ...(res.status || prev || {}),
          ...(authToken ? { auth_token: authToken } : {}),
          ...(pairingPin ? { pairing_pin: pairingPin } : {}),
        }))
      } else {
        loadStatus()
      }
      setActionFeedback({ text: 'Bridge credentials rotated successfully. Update your remote client configuration.', isError: false })
    } catch (err: any) {
      setActionFeedback({ text: `Credential rotation failed: ${err?.message || 'Unknown error'}`, isError: true })
    } finally {
      setRotatingAuth(false)
    }
  }

  const handleSetMode = async (newMode: string) => {
    try {
      setModeLoading(true)
      setActionFeedback(null)
      const res = await api.setBridgeMode(newMode)
      if (res?.status) {
        setStatus(res.status)
      } else {
        loadStatus()
      }
    } catch (err: any) {
      setActionFeedback({ text: `Failed to set mode: ${err?.message || 'Unknown error'}`, isError: true })
    } finally {
      setModeLoading(false)
    }
  }

  const handleToggleUseAppPassword = async (useAppPassword: boolean) => {
    if (!status?.app_password_enabled) return
    const previousUseAppPassword = Boolean(status.use_app_password)
    try {
      setPinModeLoading(true)
      setActionFeedback(null)
      setStatus((prev) => (prev ? { ...prev, use_app_password: useAppPassword } : prev))
      const configRes = await api.getRemoteControlConfig()
      if (!configRes?.config) {
        throw new Error('Remote control configuration is unavailable')
      }
      const res = await api.updateRemoteControlConfig({
        ...configRes.config,
        use_app_password: useAppPassword,
      })
      if (res && res.success === false) {
        throw new Error((res as any).error || 'Failed to update PIN mode')
      }
      await loadStatus()
    } catch (err: any) {
      setStatus((prev) => (prev ? { ...prev, use_app_password: previousUseAppPassword } : prev))
      setActionFeedback({ text: `Failed to update PIN mode: ${err?.message || 'Unknown error'}`, isError: true })
    } finally {
      setPinModeLoading(false)
    }
  }

  const mcpConfigSnippet = JSON.stringify(
    {
      mcpServers: {
        'AGY Bridge': {
          url: status?.gateway_url || '',
          headers: {
            Authorization: `Bearer ${status?.auth_token || ''}`,
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
  const appPasswordEnabled = Boolean(status?.app_password_enabled)
  const useAppPassword = Boolean(appPasswordEnabled && status?.use_app_password)
  // Issue #120: loopback is never presented as an external endpoint. The MCP endpoint stays empty
  // until a tunnel exposes an HTTPS URL.
  const activeEndpoint = status?.gateway_url || ''

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* 1. Overview, Live Status, Endpoint & Pairing */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', marginBottom: '16px', flexWrap: 'wrap' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)', whiteSpace: 'nowrap' }}>
                Gemini Agent Chat Bridge
              </h2>
              <span
                title="Streamable HTTPS MCP gateway for Gemini App and Web orchestration."
                style={{ display: 'inline-flex', color: 'var(--text-subtle)', cursor: 'default' }}
              >
                <Info size={13} aria-hidden="true" />
              </span>
            </div>
          </div>

          <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '13px', fontWeight: 600 }}>
            <span style={{ whiteSpace: 'nowrap' }}>Enable Remote Access</span>
            <ToggleSwitch
              checked={isTunnelActive}
              onChange={(checked) => {
                if (checked) handleStartTunnel()
                else handleStopTunnel()
              }}
              disabled={tunnelActionLoading}
              ariaLabel="Enable Remote Access"
            />
          </label>
        </div>

        {actionFeedback && (
          <div
            style={{
              padding: '10px 14px',
              borderRadius: '8px',
              border: `1px solid ${actionFeedback.isError ? 'var(--border-danger)' : 'var(--border-success)'}`,
              backgroundColor: actionFeedback.isError ? 'var(--red-bg)' : 'var(--green-bg)',
              fontSize: '12px',
              color: actionFeedback.isError ? 'var(--red)' : 'var(--green)',
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              marginBottom: '16px',
            }}
          >
            {actionFeedback.isError ? <AlertCircle size={14} /> : <CheckCircle2 size={14} />}
            <span>{actionFeedback.text}</span>
          </div>
        )}

        {/* Status Metrics Grid */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '12px', marginBottom: '12px' }}>
          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Gateway Status
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: status?.enabled ? 'var(--green)' : 'var(--text-muted)', marginTop: '4px' }}>
              {status?.enabled ? 'Streamable HTTPS' : 'Standby'}
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Operating Mode
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
              {currentMode.toUpperCase()}
            </div>
          </div>

        </div>

        {/* Live Gateway URL & Pairing PIN */}
        <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: '12px' }}>
          <div
            style={{
              backgroundColor: 'var(--canvas)',
              borderRadius: '8px',
              border: '1px solid var(--border)',
              padding: '12px 14px',
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              flexWrap: 'wrap',
              gap: '12px',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: '0', flex: 1 }}>
              <Radio size={16} color="var(--primary)" />
              {activeEndpoint ? (
                <>
                  <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
                    Active Endpoint:
                  </span>
                  <code style={{ fontSize: '12px', color: 'var(--primary)', fontWeight: 600, wordBreak: 'break-all' }}>
                    {activeEndpoint}
                  </code>
                </>
              ) : (
                <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                  Not exposed to the internet — enable Remote Access to get an HTTPS endpoint
                </span>
              )}
            </div>

            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <button
                onClick={() => copyToClipboard(activeEndpoint, setCopiedUrl)}
                disabled={!activeEndpoint}
                className="btn-pill-tonal"
                style={{ padding: '6px 12px', fontSize: '12px', opacity: activeEndpoint ? 1 : 0.5, cursor: activeEndpoint ? 'pointer' : 'not-allowed' }}
              >
                {copiedUrl ? <CheckCircle2 size={13} color="var(--green)" /> : <Copy size={13} />}
                <span>{copiedUrl ? 'Copied' : 'Copy Endpoint'}</span>
              </button>
            </div>
          </div>

          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              justifyContent: 'space-between',
              gap: '8px',
              backgroundColor: 'var(--canvas)',
              border: '1px solid var(--border)',
              borderRadius: '8px',
              padding: '12px 14px',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '8px' }}>
              <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
                Pairing PIN (Quick Setup)
              </span>
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  opacity: appPasswordEnabled ? 1 : 0.5,
                }}
                title={
                  appPasswordEnabled
                    ? 'Toggle between your 8-character Pairing PIN and your App Access Password'
                    : 'Set an App Access Password in General Settings to enable this option'
                }
              >
                <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
                  Use App Password
                </span>
                <ToggleSwitch
                  size="sm"
                  checked={useAppPassword}
                  onChange={handleToggleUseAppPassword}
                  disabled={!appPasswordEnabled || pinModeLoading}
                  ariaLabel="Use App Password as Pairing PIN"
                />
              </div>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '8px' }}>
              <span
                style={{
                  fontSize: useAppPassword ? '14px' : '20px',
                  fontWeight: 700,
                  letterSpacing: useAppPassword ? '0.5px' : '2px',
                  color: 'var(--primary)',
                  fontFamily: 'monospace',
                  whiteSpace: 'nowrap',
                }}
              >
                {useAppPassword ? 'App Password' : (status?.pairing_pin || '--------')}
              </span>
              <button
                onClick={handleRotateCredentials}
                disabled={rotatingAuth || useAppPassword}
                className="btn-pill-tonal"
                style={{
                  marginLeft: 'auto',
                  padding: '4px 10px',
                  fontSize: '11px',
                  opacity: useAppPassword ? 0.5 : 1,
                  cursor: rotatingAuth || useAppPassword ? 'not-allowed' : 'pointer',
                }}
              >
                <RefreshCw size={12} className={rotatingAuth ? 'animate-spin' : ''} />
                <span>Rotate PIN</span>
              </button>
            </div>
            <p style={{ margin: '6px 0 0', fontSize: '11px', color: 'var(--text-muted)' }}>
              Pass as header <code>X-Pairing-PIN</code> or URL query <code>?pin={status?.pairing_pin}</code>.
            </p>
          </div>
        </div>
      </div>

      {/* 2. Access Mode Tunneling */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '14px' }}>
          Access Mode
        </div>

        {/* Tunnel Selection Grid */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '12px', marginBottom: '12px' }}>
          <div
            onClick={() => setTunnelType('quick')}
            style={{
              padding: '12px',
              borderRadius: '8px',
              border: tunnelType === 'quick' ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: tunnelType === 'quick' ? 'var(--canvas)' : '#ffffff',
              cursor: 'pointer',
            }}
          >
            <div style={{ marginBottom: '4px' }}>
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Cloudflare Quick Tunnel
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '11px', color: 'var(--text-muted)' }}>
              Spawns a temporary public HTTPS MCP endpoint (*.trycloudflare.com) via the bundled cloudflared sidecar with 4-minute keep-alive probes. No account required.
            </p>
          </div>

          <div
            onClick={() => setTunnelType('tailscale')}
            style={{
              padding: '12px',
              borderRadius: '8px',
              border: tunnelType === 'tailscale' ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: tunnelType === 'tailscale' ? 'var(--canvas)' : '#ffffff',
              cursor: 'pointer',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', marginBottom: '4px' }}>
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Tailscale Funnel
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '11px', color: 'var(--text-muted)' }}>
              Publishes a public HTTPS MCP endpoint (*.ts.net) through Tailscale Funnel so Google Gemini cloud servers can reach your local bridge.
            </p>
          </div>

          <div
            onClick={() => setTunnelType('custom')}
            style={{
              padding: '12px',
              borderRadius: '8px',
              border: tunnelType === 'custom' ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: tunnelType === 'custom' ? 'var(--canvas)' : '#ffffff',
              cursor: 'pointer',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', marginBottom: '4px' }}>
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Custom Domain
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '11px', color: 'var(--text-muted)' }}>
              Routes through a persistent Cloudflare Named Tunnel, ngrok endpoint, or custom HTTPS reverse proxy under your own domain.
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
                backgroundColor: 'var(--canvas)',
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
                : 'No tunnel logs yet. Enable Remote Access to generate an HTTPS endpoint.'}
            </pre>
          )}
        </div>
      </div>

      {/* 3. Authentication & First-Time Setup */}
      <div className="google-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px', flexWrap: 'wrap', gap: '10px' }}>
          <div>
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '4px' }}>
              Authentication & Pairing Credentials
            </div>
            <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)' }}>
              Gemini Bridge includes a built-in OAuth 2.0 / 2.1 Authorization Server: Google Gemini discovers OAuth endpoints automatically and prompts for your 8-character Pairing PIN (or App Access Password when toggled) during authorization, or you can paste the <strong>OAuth Client ID</strong> and <strong>OAuth Client Secret</strong> into Gemini's <em>Additional settings</em>.
            </p>
          </div>

          <div style={{ display: 'flex', gap: '8px' }}>
            <button
              onClick={handleRotateCredentials}
              disabled={rotatingAuth}
              className="btn-pill-tonal"
            >
              <RefreshCw size={12} className={rotatingAuth ? 'animate-spin' : ''} />
              <span>Rotate Credentials</span>
            </button>
            <button
              onClick={() => copyToClipboard(mcpConfigSnippet, setCopiedConfig)}
              disabled={!status?.auth_token}
              className="btn-pill-tonal"
              style={{ opacity: status?.auth_token ? 1 : 0.5, cursor: status?.auth_token ? 'pointer' : 'not-allowed' }}
            >
              {copiedConfig ? <CheckCircle2 size={13} color="var(--green)" /> : <Copy size={13} />}
              <span>{copiedConfig ? 'Copied' : 'Copy JSON Config'}</span>
            </button>
          </div>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '14px' }}>
          {/* OAuth Client ID */}
          <div
            style={{
              padding: '14px',
              backgroundColor: 'var(--canvas)',
              borderRadius: '8px',
              border: '1px solid var(--border)',
              display: 'flex',
              flexDirection: 'column',
              justifyContent: 'space-between',
              gap: '8px',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '6px' }}>
              <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
                OAuth Client ID
              </span>
              <button
                onClick={() => copyToClipboard(status?.oauth_client_id || 'antigravity-gemini-bridge', setCopiedClientId)}
                className="btn-pill-tonal"
                style={{ padding: '3px 8px', fontSize: '11px', cursor: 'pointer' }}
              >
                {copiedClientId ? <CheckCircle2 size={12} color="var(--green)" /> : <Copy size={12} />}
                <span>{copiedClientId ? 'Copied' : 'Copy Client ID'}</span>
              </button>
            </div>
            <code style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', wordBreak: 'break-all' }}>
              {status?.oauth_client_id || 'antigravity-gemini-bridge'}
            </code>
            <p style={{ margin: '6px 0 0', fontSize: '11px', color: 'var(--text-muted)' }}>
              Used with the Client Secret / Bearer Token in Gemini's OAuth <em>Additional settings</em>.
            </p>
          </div>

          {/* Bearer Token */}
          <div
            style={{
              padding: '14px',
              backgroundColor: 'var(--canvas)',
              borderRadius: '8px',
              border: '1px solid var(--border)',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '6px' }}>
              <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>
                OAuth Client Secret / Bearer Token
              </span>
              <button
                onClick={() => copyToClipboard(status?.oauth_client_secret || status?.auth_token || '', setCopiedToken)}
                disabled={!status?.auth_token && !status?.oauth_client_secret}
                className="btn-pill-tonal"
                style={{ padding: '3px 8px', fontSize: '11px', opacity: status?.auth_token || status?.oauth_client_secret ? 1 : 0.5, cursor: status?.auth_token || status?.oauth_client_secret ? 'pointer' : 'not-allowed' }}
              >
                {copiedToken ? <CheckCircle2 size={12} color="var(--green)" /> : <Copy size={12} />}
                <span>{copiedToken ? 'Copied' : 'Copy Token'}</span>
              </button>
            </div>
            <code style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', wordBreak: 'break-all' }}>
              {status?.oauth_client_secret || status?.auth_token || 'Unavailable'}
            </code>
            <p style={{ margin: '6px 0 0', fontSize: '11px', color: 'var(--text-muted)' }}>
              Configured via standard <code>Authorization: Bearer &lt;token&gt;</code> HTTP header or as Client Secret in Gemini OAuth <em>Additional settings</em>.
            </p>
          </div>
        </div>

        {/* JSON Snippet */}
        <div style={{ marginTop: '14px' }}>
          <pre
            style={{
              margin: 0,
              padding: '12px',
              backgroundColor: 'var(--canvas)',
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
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '4px' }}>
          Gemini Operating Role & Mode
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
              backgroundColor: currentMode === 'hybrid' ? 'var(--canvas)' : '#ffffff',
              cursor: modeLoading ? 'not-allowed' : 'pointer',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '6px' }}>
              <Bot size={16} color="var(--primary)" />
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
              backgroundColor: currentMode === 'lead' ? 'var(--canvas)' : '#ffffff',
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
              backgroundColor: currentMode === 'direct' ? 'var(--canvas)' : '#ffffff',
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

      </div>

      {/* 4b. Default Antigravity agent */}
      <div className="google-card">
        <div style={{ marginBottom: '10px' }}>
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
            Default Antigravity Agent
          </div>
          <p style={{ margin: '4px 0 0', fontSize: '12px', color: 'var(--text-muted)' }}>
            Which surface Gemini drives for conversations and model selection. Automatic prefers the CLI when it is installed, because a short-lived process costs less than the running app.
          </p>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '12px' }}>
          {([
            { key: 'auto', label: 'Automatic', hint: 'CLI when installed, otherwise the app' },
            { key: 'app', label: 'Antigravity 2.0 app', hint: status?.surfaces?.desktop_app_connected ? 'Connected' : 'App not detected' },
            { key: 'cli', label: 'Antigravity CLI', hint: status?.surfaces?.agy_cli_installed ? 'Installed' : 'Not installed' },
          ] as const).map((option) => {
            const active = (status?.agent_backend || 'auto') === option.key
            return (
              <button
                key={option.key}
                type="button"
                onClick={() => handleSetAgent(option.key)}
                disabled={agentLoading}
                style={{
                  textAlign: 'left',
                  padding: '12px',
                  borderRadius: '8px',
                  border: active ? '2px solid var(--primary)' : '1px solid var(--border)',
                  backgroundColor: active ? 'var(--canvas)' : '#ffffff',
                  cursor: agentLoading ? 'progress' : 'pointer',
                }}
              >
                <div style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)', whiteSpace: 'nowrap' }}>
                  {option.label}
                </div>
                <div style={{ marginTop: '2px', fontSize: '11px', color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
                  {option.hint}
                </div>
              </button>
            )
          })}
        </div>

        <p style={{ margin: '10px 0 0', fontSize: '11px', color: 'var(--text-muted)' }}>
          In use now: <strong>{status?.resolved_agent === 'cli' ? 'Antigravity CLI' : 'Antigravity 2.0 app'}</strong>
        </p>
      </div>

      {/* 5. Live Antigravity Lead / Messenger Monitor */}
      {status?.active_lead_session && (
        <div className="google-card">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px', flexWrap: 'wrap', gap: '8px' }}>
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
              Active Antigravity Lead Session
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <span
                className={`badge-chip ${status.active_lead_session.status === 'completed' ? 'badge-green' : 'badge-blue'}`}
              >
                {status.active_lead_session.status === 'completed' ? 'Completed & Forwarded' : 'Generating in Antigravity...'}
              </span>
              <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                Conversation: <code>{status.active_lead_session.conversation_id}</code>
              </span>
            </div>
          </div>

          <div style={{ padding: '12px', backgroundColor: 'var(--canvas)', borderRadius: '8px', border: '1px solid var(--border)', marginBottom: '10px' }}>
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
                      className="badge-chip badge-neutral"
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

      {/* 6. Recent Invocations & Access Audit Logs */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '14px' }}>
          Remote Invocations & Access Audit ({status?.invocations_count || 0})
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
                    No remote invocations yet. Invoke tools from Gemini App to populate this log.
                  </td>
                </tr>
              ) : (
                status.recent_invocations.map((inv) => (
                  <tr key={inv.id} style={{ borderBottom: '1px solid var(--border)' }}>
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
                      <span className={`badge-chip ${inv.status === 'success' ? 'badge-green' : 'badge-red'}`}>
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
      {/* 7. Outbound HTTP allowlist */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '12px', flexWrap: 'wrap', gap: '8px' }}>
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
            Outbound HTTP Allowlist
          </div>
          <div
            style={{
              display: 'inline-flex',
              backgroundColor: 'var(--tonal)',
              padding: '3px',
              borderRadius: '8px',
              gap: '2px',
              border: '1px solid var(--border)',
            }}
          >
            <button
              type="button"
              onClick={() => handleAllowlistModeChange('always_allow')}
              style={{
                borderRadius: '6px',
                padding: '6px 14px',
                fontSize: '12px',
                fontWeight: allowlistMode === 'always_allow' ? 600 : 500,
                color: allowlistMode === 'always_allow' ? 'var(--primary)' : 'var(--text-muted)',
                backgroundColor: allowlistMode === 'always_allow' ? 'var(--surface)' : 'transparent',
                boxShadow: allowlistMode === 'always_allow' ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                border: 'none',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
                transition: 'all 0.15s ease',
              }}
            >
              Always Allow
            </button>
            <button
              type="button"
              onClick={() => handleAllowlistModeChange('whitelist_only')}
              style={{
                borderRadius: '6px',
                padding: '6px 14px',
                fontSize: '12px',
                fontWeight: allowlistMode === 'whitelist_only' ? 600 : 500,
                color: allowlistMode === 'whitelist_only' ? 'var(--primary)' : 'var(--text-muted)',
                backgroundColor: allowlistMode === 'whitelist_only' ? 'var(--surface)' : 'transparent',
                boxShadow: allowlistMode === 'whitelist_only' ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
                border: 'none',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
                transition: 'all 0.15s ease',
              }}
            >
              Whitelist Only
            </button>
          </div>
        </div>

        {allowlistMode === 'always_allow' ? (
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '10px 12px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--canvas)' }}>
            <Globe size={15} style={{ color: 'var(--primary)', flexShrink: 0 }} />
            <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
              All outbound HTTP requests and domains are permitted without restriction.
            </span>
          </div>
        ) : (
          <>
            {pendingAllow.length > 0 && (
              <div style={{ marginBottom: '12px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
                {pendingAllow.map((req) => (
                  <div
                    key={req.host}
                    style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', padding: '10px 12px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--canvas)' }}
                  >
                    <div style={{ minWidth: 0 }}>
                      <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                        {req.host}
                      </div>
                      <div style={{ fontSize: '11px', color: 'var(--text-muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                        {req.reason || req.url || 'Requested by an agent'} · {req.requested_at}
                      </div>
                    </div>
                    <div style={{ display: 'flex', gap: '6px', flexShrink: 0 }}>
                      <button
                        type="button"
                        className="btn btn-sm btn-primary"
                        style={{ whiteSpace: 'nowrap' }}
                        onClick={() => handleAllowHostChange('allow', req.host)}
                      >
                        <Check size={13} /> Approve
                      </button>
                      <button
                        type="button"
                        className="btn btn-sm"
                        style={{ whiteSpace: 'nowrap' }}
                        onClick={() => handleAllowHostChange('deny', req.host)}
                      >
                        <Ban size={13} /> Deny
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}

            <div style={{ display: 'flex', gap: '8px', marginBottom: '12px', flexWrap: 'wrap' }}>
              <input
                type="text"
                className="input"
                placeholder="api.example.com or *.googleapis.com"
                value={newAllowHost}
                onChange={(e) => setNewAllowHost(e.target.value)}
                style={{ flex: '1 1 260px', minWidth: '200px' }}
              />
              <button
                type="button"
                className="btn btn-sm btn-primary"
                style={{ whiteSpace: 'nowrap' }}
                onClick={() => {
                  const host = newAllowHost.trim()
                  if (!host) return
                  handleAllowHostChange('allow', host)
                  setNewAllowHost('')
                }}
              >
                <Plus size={13} /> Add host
              </button>
            </div>

            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px' }}>
              {allowlist.length === 0 ? (
                <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>No hosts allowed yet.</span>
              ) : (
                allowlist.map((host) => (
                  <span key={host} className="badge-chip badge-neutral" style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', whiteSpace: 'nowrap' }}>
                    {host}
                    <button
                      type="button"
                      title={`Revoke ${host}`}
                      onClick={() => handleAllowHostChange('revoke', host)}
                      style={{ border: 'none', background: 'transparent', cursor: 'pointer', padding: 0, display: 'inline-flex', color: 'var(--text-muted)' }}
                    >
                      <X size={12} />
                    </button>
                  </span>
                ))
              )}
            </div>
          </>
        )}
      </div>
    </div>
  )
}

export default GeminiBridgePage
