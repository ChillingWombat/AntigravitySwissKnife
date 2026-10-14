import React, { useState, useEffect } from 'react'
import {
  RotateCw,
  Copy,
  Check,
  ExternalLink,
  Play,
  Square,
  Network,
} from 'lucide-react'
import { ToggleSwitch } from '../components/ToggleSwitch'
import { api } from '../api'

export const RemoteControlPage: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const [copied, setCopied] = useState<string | null>(null)
  const [saveSuccess, setSaveSuccess] = useState(false)
  const [tunnelType, setTunnelType] = useState<string>('quick')
  const [customTunnelURL, setCustomTunnelURL] = useState<string>('')
  const [tunnelLoading, setTunnelLoading] = useState(false)

  const [status, setStatus] = useState<any>({
    active: false,
    endpoint_url: 'http://127.0.0.1:8766/remote',
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

  const handleToggleEnable = async () => {
    const nextEnabled = !config.enabled
    const newConfig = { ...config, enabled: nextEnabled }
    setConfig(newConfig)
    try {
      await api.updateRemoteControlConfig(newConfig)
      await fetchStatusAndConfig()
    } catch (err) {
      console.error('Failed to update remote config:', err)
    }
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
  const displayUrl = isTunnelActive
    ? (status?.tunnel?.gateway_url || status?.tunnel?.public_url || status.endpoint_url)
    : (status.endpoint_url || `http://127.0.0.1:${config.port}/remote`)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* 1. Header & Live Status Bar */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)', whiteSpace: 'nowrap' }}>
                Antigravity Remote Control Wrapper
              </h2>
              <span className={`badge-chip ${status.active ? 'badge-green' : 'badge-neutral'}`}>
                {status.active ? 'Listener Online' : 'Standby'}
              </span>
              {isTunnelActive && (
                <span className="badge-chip badge-blue">
                  Remote Tunnel Active
                </span>
              )}
            </div>
            <p style={{ margin: '4px 0 0', fontSize: '12px', color: 'var(--text-muted)' }}>
              Wraps Antigravity remote control with persistent session re-binding across Google account switches for out-of-home access.
            </p>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '13px', fontWeight: 600 }}>
              <span style={{ whiteSpace: 'nowrap' }}>Enable Remote</span>
              <ToggleSwitch
                checked={Boolean(config.enabled)}
                onChange={handleToggleEnable}
              />
            </label>
          </div>
        </div>

        {/* Status Metrics Grid */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '12px' }}>
          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Account Resilience
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: config.account_resilience ? 'var(--green)' : 'var(--text-muted)', marginTop: '4px' }}>
              {config.account_resilience ? 'Auto-Rebind Active' : 'Disabled'}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Survives account rotations
            </div>
          </div>

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
              DevTools Port
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
              {status.devtools_port > 0 ? status.devtools_port : 'Idle (Auto)'}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              CDP DevTools Port
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Host Antigravity PID
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
              {status.current_antigravity_pid > 0 ? status.current_antigravity_pid : 'Idle'}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Host process monitored
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
        </div>
      </div>

      {/* 2. Connection Endpoint & Security Card */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '16px' }}>
          Remote Access Endpoint & Pairing Security
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: '16px' }}>
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
              {isTunnelActive ? 'Remote Public HTTPS URL (Out-of-Home)' : 'Local Host URL'}
            </span>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
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

      {/* 3. Out-of-Home Remote Internet Access (Tunneling) */}
      <div className="google-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '12px', marginBottom: '14px' }}>
          <div>
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '4px' }}>
              Remote Internet Access (Out-of-Home Access)
            </div>
            <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)' }}>
              Dedicated strictly for when you are away from home and away from your PC. Access your Antigravity sessions securely via public HTTPS or private VPN.
            </p>
          </div>

          <div style={{ display: 'flex', gap: '8px' }}>
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
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                <Network size={14} color="var(--primary)" />
                <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                  Quick Tunnel (Recommended)
                </span>
              </div>
              <span className="badge-chip badge-green">
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
              backgroundColor: tunnelType === 'tailscale' ? 'var(--canvas)' : '#ffffff',
              cursor: 'pointer',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Tailscale Network
              </span>
              <span className="badge-chip badge-blue">
                Private VPN
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
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
              <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                Custom HTTPS Tunnel
              </span>
              <span className="badge-chip badge-neutral">
                Reverse Proxy
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

          {/* Account Resilience Toggle (Label Left, Switch Right) */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '12px 14px',
              backgroundColor: 'var(--canvas)',
              borderRadius: '8px',
              border: '1px solid var(--border)',
            }}
          >
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                Account-Switch Resilience Mode
              </div>
              <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                Automatically retains connected clients and re-attaches CDP session when local Google accounts rotate.
              </div>
            </div>
            <ToggleSwitch
              checked={Boolean(config.account_resilience)}
              onChange={() => setConfig({ ...config, account_resilience: !config.account_resilience })}
            />
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
              The wrapper is strictly designed for when you are away from home and away from your PC. Start a Quick Tunnel (public HTTPS) or connect via Tailscale Network, enter your 6-digit Security PIN on your mobile device, and monitor or steer agents seamlessly.
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
