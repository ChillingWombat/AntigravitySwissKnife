import React, { useState, useEffect } from 'react'
import {
  RotateCw,
  Copy,
  Check,
  ExternalLink,
  Play,
  Square,
} from 'lucide-react'
import { api, isAbortError } from '../api'

export const RemoteControlPage: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const [copied, setCopied] = useState<string | null>(null)
  const [saveSuccess, setSaveSuccess] = useState(false)
  const [tunnelType, setTunnelType] = useState<string>('quick')
  const [customTunnelURL, setCustomTunnelURL] = useState<string>('')
  const [tunnelLoading, setTunnelLoading] = useState(false)

  const [status, setStatus] = useState<any>({
    active: false,
    endpoint_url: '',
    account_resilience: true,
    current_antigravity_pid: 0,
    devtools_port: 0,
    active_google_account: '',
    clients_count: 0,
    last_rotated_at: '',
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
    security_pin: '849201',
    quality: 'high',
    frame_rate: 30,
    account_resilience: true,
  })

  const fetchStatusAndConfig = async () => {
    try {
      const res = await api.getRemoteControlStatus()
      if (res && res.success) {
        if (res.status) setStatus(res.status)
        if (res.config) setConfig(res.config)
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

  const handleStartTunnel = async () => {
    try {
      setTunnelLoading(true)
      const res = await api.startRemoteTunnel({
        type: tunnelType,
        custom_url: customTunnelURL,
      })
      if (res && res.success) {
        await fetchStatusAndConfig()
      }
    } catch (err) {
      console.error('Failed to start remote tunnel:', err)
    } finally {
      setTunnelLoading(false)
    }
  }

  const handleStopTunnel = async () => {
    try {
      setTunnelLoading(true)
      const res = await api.stopRemoteTunnel()
      if (res && res.success) {
        await fetchStatusAndConfig()
      }
    } catch (err) {
      console.error('Failed to stop remote tunnel:', err)
    } finally {
      setTunnelLoading(false)
    }
  }

  const handleSaveConfig = async () => {
    setLoading(true)
    setSaveSuccess(false)
    try {
      const res = await api.updateRemoteControlConfig(config)
      if (res && res.success) {
        if (res.config) setConfig(res.config)
        setSaveSuccess(true)
        setTimeout(() => setSaveSuccess(false), 3000)
      }
    } catch (err) {
      console.error('Failed to save remote config:', err)
    } finally {
      setLoading(false)
    }
  }

  const isTunnelActive = Boolean(status?.tunnel?.active)
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
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)', whiteSpace: 'nowrap' }}>
                Antigravity Remote Control Wrapper
              </h2>
            </div>
            <p style={{ margin: '4px 0 0', fontSize: '12px', color: 'var(--text-muted)' }}>
              Wraps Antigravity remote control with persistent session re-binding across Google account switches for out-of-home access.
            </p>
          </div>
        </div>

        {/* Status Metrics Grid: 3 balanced cards */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '12px', marginBottom: '12px' }}>
          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Remote Internet Pipe
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: isTunnelActive ? 'var(--primary)' : 'var(--text-muted)', marginTop: '4px' }}>
              {isTunnelActive ? (status?.tunnel?.type || 'active').toUpperCase() : 'INACTIVE'}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              {isTunnelActive ? (status?.tunnel?.type === 'tailscale' ? 'Tailscale Private VPN' : 'Public HTTPS Tunnel') : 'Remote Access Off'}
            </div>
          </div>

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
              gap: '6px',
              backgroundColor: 'var(--canvas)',
              border: '1px solid var(--border)',
              borderRadius: '8px',
              padding: '12px 14px',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
                Security PIN
              </span>
              <button
                onClick={handleRotatePIN}
                disabled={loading}
                className="btn-pill-tonal"
                style={{ padding: '3px 8px', fontSize: '11px' }}
              >
                <RotateCw size={12} className={loading ? 'animate-spin' : ''} />
                <span>Rotate PIN</span>
              </button>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <span
                style={{
                  fontSize: '20px',
                  fontWeight: 700,
                  letterSpacing: '2px',
                  color: 'var(--primary)',
                  fontFamily: 'monospace',
                }}
              >
                {config.security_pin || '------'}
              </span>
              <button
                onClick={() => copyToClipboard(config.security_pin, 'pin')}
                className="btn-pill-tonal"
                style={{ marginLeft: 'auto', padding: '4px 10px', fontSize: '11px' }}
              >
                {copied === 'pin' ? <Check size={12} color="var(--green)" /> : <Copy size={12} />}
                <span>{copied === 'pin' ? 'Copied' : 'Copy'}</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* 2. Out-of-Home Remote Internet Access (Tunneling) */}
      <div className="google-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'nowrap', gap: '12px', marginBottom: '14px' }}>
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
            Remote Internet Access (Out-of-Home Access)
          </div>

          <div style={{ display: 'flex', gap: '8px', flexShrink: 0 }}>
            {isTunnelActive ? (
              <button
                onClick={handleStopTunnel}
                disabled={tunnelLoading}
                className="btn-pill-danger"
              >
                <Square size={13} />
                <span>{tunnelLoading ? 'Stopping...' : 'Stop Remote Access'}</span>
              </button>
            ) : (
              <button
                onClick={handleStartTunnel}
                disabled={tunnelLoading}
                className="btn-pill-primary"
              >
                <Play size={13} />
                <span>{tunnelLoading ? 'Connecting...' : 'Start Remote Access'}</span>
              </button>
            )}
          </div>
        </div>

        {/* Tunnel Options Grid */}
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
              Generates trusted public HTTPS URL automatically via bundled sidecar. No accounts or software installation required.
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
              Routes through your personal Tailnet or Tailscale Funnel. Ideal if your phone or tablet is connected to Tailscale.
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
              Self-hosted Cloudflare Tunnel, ngrok, or reverse proxy endpoint for your custom remote domain.
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
      </div>

      {/* 4. Configuration Settings */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '16px' }}>
          Remote Configuration
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '16px' }}>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
                Listening Port
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '8px' }}>
                Local loopback TCP port for remote web app server.
              </div>
              <input
                type="number"
                value={config.port}
                onChange={(e) => setConfig({ ...config, port: parseInt(e.target.value, 10) || 8766 })}
                style={{ width: '100%' }}
              />
            </div>

            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
                Stream Quality
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '8px' }}>
                Screen stream compression preset.
              </div>
              <select
                value={config.quality}
                onChange={(e) => setConfig({ ...config, quality: e.target.value })}
                style={{ width: '100%' }}
              >
                <option value="high">High (1080p, 30fps)</option>
                <option value="medium">Medium (720p, 24fps)</option>
                <option value="low">Low (480p, 15fps)</option>
              </select>
            </div>
          </div>

          <div
            style={{
              padding: '12px 14px',
              backgroundColor: 'var(--canvas)',
              borderRadius: '8px',
              border: '1px solid var(--border)',
            }}
          >
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
              Account Switches
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
              The tunnel runs in the Swiss daemon, not inside Antigravity. It stays up while Antigravity restarts for a Google account switch.
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginTop: '4px' }}>
            <button
              onClick={handleSaveConfig}
              disabled={loading}
              className="btn-pill-primary"
            >
              Save Settings
            </button>
            {saveSuccess && (
              <span style={{ fontSize: '12px', color: 'var(--green)', display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                <Check size={14} /> Settings saved successfully
              </span>
            )}
          </div>
        </div>
      </div>

      {/* 5. Step-by-Step Guidance */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '14px' }}>
          Architecture & Guidance
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '2px' }}>
              Zero Invalidation on Account Switch
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', lineHeight: '1.45' }}>
              Antigravity factory remote session disconnects when switching accounts because the host Google access token expires. The Swiss Knife wrapper isolates the CDP WebSocket pipe from account tokens, keeping your web session persistent across account swaps.
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '2px' }}>
              Out-of-Home Access from Mobile or Laptop
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', lineHeight: '1.45' }}>
              The wrapper is strictly designed for when you are away from home and away from your PC. Start a Cloudflare Quick Tunnel (public HTTPS) or connect via Tailscale Funnel, enter your 6-digit Security PIN on your mobile device, and monitor or steer agents seamlessly.
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '2px' }}>
              Security Protocol
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', lineHeight: '1.45' }}>
              All remote view connections require matching the 6-digit Security PIN. If you need to revoke access immediately, click <em>Rotate PIN</em> to terminate existing view sessions instantly.
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default RemoteControlPage
