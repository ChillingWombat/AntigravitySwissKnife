import React, { useState, useEffect } from 'react'
import {
  RotateCw,
  Copy,
  Check,
  ExternalLink,
  AlertCircle,
  CheckCircle2,
  ChevronDown,
  ChevronUp,
} from 'lucide-react'
import { api, isAbortError } from '../api'
import { ToggleSwitch } from '../components/ToggleSwitch'

export const RemoteControlPage: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const [pinModeLoading, setPinModeLoading] = useState(false)
  const [copied, setCopied] = useState<string | null>(null)
  const [tunnelType, setTunnelType] = useState<string>('quick')
  const [customTunnelURL, setCustomTunnelURL] = useState<string>('')
  const [tunnelLoading, setTunnelLoading] = useState(false)
  const [actionFeedback, setActionFeedback] = useState<{ text: string; isError: boolean } | null>(null)
  const [showTunnelLogs, setShowTunnelLogs] = useState<boolean>(false)
  const wasTunnelActiveRef = React.useRef<boolean>(false)

  const [status, setStatus] = useState<any>({
    active: false,
    endpoint_url: '',
    account_resilience: true,
    current_antigravity_pid: 0,
    devtools_port: 0,
    active_google_account: '',
    clients_count: 0,
    last_rotated_at: '',
    use_app_password: false,
    app_password_enabled: false,
    tunnel: {
      active: false,
      type: '',
      gateway_url: '',
      public_url: '',
    },
  })

  const [config, setConfig] = useState<any>({
    enabled: false,
    bind_addr: '127.0.0.1',
    port: 8766,
    security_pin: 'A7k9M2xP',
    use_app_password: false,
    quality: 'high',
    frame_rate: 30,
    account_resilience: true,
  })

  const fetchStatusAndConfig = async () => {
    try {
      const res = await api.getRemoteControlStatus()
      if (res && res.success) {
        if (res.status) {
          const nowActive = Boolean(res.status.tunnel?.active)
          if (wasTunnelActiveRef.current && !nowActive && res.status.tunnel?.error) {
            setActionFeedback({
              text: `Tunnel stopped automatically: ${res.status.tunnel.error}`,
              isError: true,
            })
          }
          wasTunnelActiveRef.current = nowActive
          setStatus(res.status)
          if (res.status.tunnel?.type) {
            setTunnelType(res.status.tunnel.type)
          }
        }
        if (res.config) {
          setConfig(res.config)
          if (res.config.tunnel_type === 'custom' && res.config.tunnel_url) {
            setCustomTunnelURL((prev) => prev || res.config.tunnel_url)
          }
        }
      }
    } catch (err) {
      if (isAbortError(err)) return
      console.error('Failed to fetch remote control status:', err)
    }
  }

  useEffect(() => {
    fetchStatusAndConfig()
    const timer = setInterval(fetchStatusAndConfig, 5000)
    return () => clearInterval(timer)
  }, [])

  const copyToClipboard = (text: string, key: string) => {
    navigator.clipboard.writeText(text)
    setCopied(key)
    setTimeout(() => setCopied(null), 2000)
  }

  const handleRotatePIN = async () => {
    setLoading(true)
    try {
      const res = await api.rotateRemotePIN()
      if (res && res.success) {
        setConfig((prev: any) => ({ ...prev, security_pin: res.security_pin }))
        if (res.config) setConfig(res.config)
      }
    } catch (err) {
      console.error('Failed to rotate remote PIN:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleToggleUseAppPassword = async (useAppPassword: boolean) => {
    if (!status?.app_password_enabled) return
    setPinModeLoading(true)
    try {
      const res = await api.updateRemoteControlConfig({
        ...config,
        use_app_password: useAppPassword,
      })
      if (res && res.success) {
        if (res.config) setConfig(res.config)
        await fetchStatusAndConfig()
      }
    } catch (err) {
      console.error('Failed to toggle remote PIN mode:', err)
    } finally {
      setPinModeLoading(false)
    }
  }

  const handleStartTunnel = async () => {
    try {
      setTunnelLoading(true)
      setActionFeedback(null)
      const res = await api.startRemoteTunnel({
        type: tunnelType,
        custom_url: customTunnelURL,
      })
      if (res && res.success) {
        await fetchStatusAndConfig()
        setActionFeedback({ text: 'Remote access tunnel started.', isError: false })
      } else {
        setActionFeedback({ text: (res as any)?.error || 'Failed to start remote tunnel.', isError: true })
      }
    } catch (err: any) {
      console.error('Failed to start remote tunnel:', err)
      setActionFeedback({ text: `Tunnel failed to start: ${err?.message || 'Unknown error'}`, isError: true })
    } finally {
      setTunnelLoading(false)
    }
  }

  const handleStopTunnel = async () => {
    try {
      setTunnelLoading(true)
      setActionFeedback(null)
      const res = await api.stopRemoteTunnel()
      if (res && res.success) {
        await fetchStatusAndConfig()
        setActionFeedback({ text: 'Remote access tunnel stopped.', isError: false })
      } else {
        setActionFeedback({ text: (res as any)?.error || 'Failed to stop remote tunnel.', isError: true })
      }
    } catch (err: any) {
      console.error('Failed to stop remote tunnel:', err)
      setActionFeedback({ text: `Failed to stop tunnel: ${err?.message || 'Unknown error'}`, isError: true })
    } finally {
      setTunnelLoading(false)
    }
  }

  const isTunnelActive = Boolean(status?.tunnel?.active)
  const appPasswordEnabled = Boolean(status?.app_password_enabled)
  const useAppPassword = Boolean(appPasswordEnabled && (config?.use_app_password || status?.use_app_password))
  // Issue #120: a loopback endpoint is never surfaced as a remote endpoint. Until an external
  // tunnel is active there is no shareable URL — the page shows guidance instead of
  // http://127.0.0.1. Externally reachable endpoints are HTTPS.
  const displayUrl = isTunnelActive
    ? (status?.tunnel?.gateway_url || status?.tunnel?.public_url || '')
    : ''

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* 1. Remote Control Wrapper, Status, Endpoint & Pairing Security */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', marginBottom: '16px', flexWrap: 'wrap' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)', whiteSpace: 'nowrap' }}>
                Antigravity Remote Control Wrapper
              </h2>
            </div>
            <p style={{ margin: '4px 0 0', fontSize: '12px', color: 'var(--text-muted)' }}>
              Streams the live Antigravity 2.0 web interface over an authenticated reverse proxy with persistent session and CSRF re-binding across Google account switches.
            </p>
          </div>

          <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '13px', fontWeight: 600 }}>
            <span style={{ whiteSpace: 'nowrap' }}>Enable Remote Access</span>
            <ToggleSwitch
              checked={isTunnelActive}
              onChange={(checked) => {
                if (checked) handleStartTunnel()
                else handleStopTunnel()
              }}
              disabled={tunnelLoading}
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

        {/* Status Metrics Grid: 2 balanced cards */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '12px', marginBottom: '12px' }}>
          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Active Google Account
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
              {status.active_google_account || 'Not signed in'}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Syncs without dropped viewer
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              DevTools Port
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
              {status.devtools_port > 0 ? status.devtools_port : 'Idle (Auto)'}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              CDP DevTools Port
            </div>
          </div>
        </div>

        {/* Remote Access Endpoint & Pairing Security */}
        <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: '12px' }}>
          {/* Endpoint URL */}
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              gap: '6px',
              backgroundColor: 'var(--canvas)',
              border: '1px solid var(--border)',
              borderRadius: '8px',
              padding: '12px 14px',
            }}
          >
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
              Remote Endpoint
            </span>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              {displayUrl ? (
                <>
                  <code style={{ fontSize: '13px', flex: 1, color: 'var(--text)', wordBreak: 'break-all' }}>
                    {displayUrl}
                  </code>
                  <button
                    onClick={() => copyToClipboard(displayUrl, 'url')}
                    className="btn-pill-tonal"
                    style={{ padding: '6px 12px', fontSize: '12px' }}
                  >
                    {copied === 'url' ? <Check size={14} color="var(--green)" /> : <Copy size={14} />}
                    <span>{copied === 'url' ? 'Copied' : 'Copy'}</span>
                  </button>
                  <a
                    href={displayUrl}
                    target="_blank"
                    rel="noreferrer"
                    className="btn-pill-outlined"
                    style={{ padding: '6px 12px', fontSize: '12px', textDecoration: 'none' }}
                  >
                    <ExternalLink size={14} />
                    <span>Open</span>
                  </a>
                </>
              ) : (
                <>
                  <span style={{ fontSize: '12px', color: 'var(--text-muted)', flex: 1 }}>
                    Not exposed to the internet — start Remote Access to get an https endpoint
                  </span>
                  <button
                    disabled
                    className="btn-pill-tonal"
                    style={{ padding: '6px 12px', fontSize: '12px', opacity: 0.5, cursor: 'not-allowed' }}
                  >
                    <Copy size={14} />
                    <span>Copy</span>
                  </button>
                </>
              )}
            </div>
          </div>

          {/* Security PIN */}
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
                Security PIN
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
                    ? 'Toggle between your 8-character Security PIN and your App Access Password'
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
                  ariaLabel="Use App Password as Security PIN"
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
                {useAppPassword ? 'App Password' : (config.security_pin || '--------')}
              </span>
              <button
                onClick={handleRotatePIN}
                disabled={loading || useAppPassword}
                className="btn-pill-tonal"
                style={{
                  marginLeft: 'auto',
                  padding: '4px 10px',
                  fontSize: '11px',
                  opacity: useAppPassword ? 0.5 : 1,
                  cursor: loading || useAppPassword ? 'not-allowed' : 'pointer',
                }}
              >
                <RotateCw size={12} className={loading ? 'animate-spin' : ''} />
                <span>Rotate PIN</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* 2. Out-of-Home Remote Internet Access (Tunneling) */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '14px' }}>
          Remote Internet Access (Out-of-Home Access)
        </div>

        {/* Tunnel Options Grid */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(210px, 1fr))', gap: '12px', marginBottom: '12px' }}>
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
              Spawns a temporary public HTTPS URL (*.trycloudflare.com) via the bundled cloudflared sidecar with 4-minute keep-alive probes. No account required.
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
              Publishes a public HTTPS endpoint (*.ts.net) through Tailscale Funnel so any browser can connect without joining your tailnet.
            </p>
          </div>

          <div
            onClick={() => setTunnelType('tailnet')}
            style={{
              padding: '12px',
              borderRadius: '8px',
              border: tunnelType === 'tailnet' ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: tunnelType === 'tailnet' ? 'var(--canvas)' : '#ffffff',
              cursor: 'pointer',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', marginBottom: '4px' }}>
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Tailscale Private Tailnet
              </span>
            </div>
            <p style={{ margin: 0, fontSize: '11px', color: 'var(--text-muted)' }}>
              Binds directly to your private Tailscale network (100.x.y.z / MagicDNS). Accessible only from devices signed into your tailnet.
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
              placeholder="https://your-domain.com/remote"
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
                maxHeight: '160px',
                overflowY: 'auto',
                whiteSpace: 'pre-wrap',
                color: 'var(--text-muted)',
                lineHeight: 1.4,
              }}
            >
              {status?.tunnel?.logs && status.tunnel.logs.length > 0
                ? status.tunnel.logs.join('\n')
                : 'No tunnel log events recorded yet. Logs will stream here during sidecar lifecycle.'}
            </pre>
          )}
        </div>
      </div>

      {/* 3. Architecture & Guidance */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '14px' }}>
          Architecture & Guidance
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '2px' }}>
              Zero Invalidation & Live Account Sync
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', lineHeight: '1.45' }}>
              The tunnel runs in the Swiss daemon, not inside Antigravity. It stays up while Antigravity restarts for a Google account switch, automatically re-binding the upstream port, CSRF token, and active account profile in your remote browser.
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '2px' }}>
              Tunnel Modes & 4-Minute Keep-Alive
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', lineHeight: '1.45' }}>
              Use Cloudflare Quick Tunnel or Tailscale Funnel for public HTTPS access from any device, Tailscale Private Tailnet for restricted VPN-only access, or Custom Domain for a fixed hostname. Background probes ping the active endpoint every 4 minutes to prevent idle timeouts and automatically stop Remote Access if a tunnel drops.
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '2px' }}>
              8-Character Security PIN & App Password Mode
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', lineHeight: '1.45' }}>
              Remote sessions authenticate with an 8-character alphanumeric Security PIN by default, or with your master App Access Password when Use App Password is enabled. Click <em>Rotate PIN</em> at any time to generate a fresh 8-character code and revoke active remote browser sessions immediately.
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default RemoteControlPage
