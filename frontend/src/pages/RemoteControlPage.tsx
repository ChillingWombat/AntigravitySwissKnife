import React, { useState, useEffect } from 'react'
import {
  Monitor,
  ShieldCheck,
  RotateCw,
  Copy,
  Check,
  ExternalLink,
  Power,
  Tv,
  Cpu,
  Layers,
  HelpCircle,
  Settings2,
} from 'lucide-react'
import { api } from '../api'

export const RemoteControlPage: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const [copied, setCopied] = useState<string | null>(null)
  const [saveSuccess, setSaveSuccess] = useState(false)

  const [status, setStatus] = useState<any>({
    active: false,
    endpoint_url: 'http://127.0.0.1:8766/remote',
    account_resilience: true,
    current_antigravity_pid: 0,
    devtools_port: 0,
    active_google_account: '',
    clients_count: 0,
    last_rotated_at: '',
  })

  const [config, setConfig] = useState<any>({
    enabled: true,
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

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* 1. Header & Quick Status Bar */}
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          backgroundColor: '#ffffff',
          borderRadius: '12px',
          padding: '18px 24px',
          border: '1px solid var(--border)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <div
            style={{
              width: '44px',
              height: '44px',
              borderRadius: '10px',
              backgroundColor: 'var(--primary-container)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Monitor size={24} color="var(--primary)" />
          </div>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
              <h2
                style={{
                  fontSize: '17px',
                  fontWeight: 600,
                  margin: 0,
                  color: 'var(--text)',
                  whiteSpace: 'nowrap',
                }}
              >
                Antigravity Remote Control Wrapper
              </h2>
              <span
                style={{
                  fontSize: '11px',
                  padding: '2px 8px',
                  borderRadius: '12px',
                  fontWeight: 600,
                  whiteSpace: 'nowrap',
                  backgroundColor: status.active ? '#dcfce7' : 'var(--tonal)',
                  color: status.active ? '#15803d' : 'var(--text-muted)',
                }}
              >
                {status.active ? 'Listener Online' : 'Standby'}
              </span>
            </div>
            <p
              style={{
                fontSize: '13px',
                color: 'var(--text-muted)',
                margin: '4px 0 0 0',
              }}
            >
              Wraps Antigravity remote control with persistent session re-binding across Google account switches.
            </p>
          </div>
        </div>

        <button
          onClick={handleToggleEnable}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            padding: '8px 16px',
            borderRadius: '20px',
            border: 'none',
            fontSize: '13px',
            fontWeight: 600,
            cursor: 'pointer',
            whiteSpace: 'nowrap',
            backgroundColor: config.enabled ? '#fee2e2' : 'var(--primary-container)',
            color: config.enabled ? '#b91c1c' : 'var(--primary)',
          }}
        >
          <Power size={15} />
          {config.enabled ? 'Disable Remote' : 'Enable Remote'}
        </button>
      </div>

      {/* 2. Resilience Invariant & Live Metrics Grid */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(4, 1fr)',
          gap: '12px',
        }}
      >
        <div
          style={{
            backgroundColor: '#ffffff',
            borderRadius: '10px',
            padding: '14px 16px',
            border: '1px solid var(--border)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px' }}>
            <ShieldCheck size={16} color="var(--primary)" />
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
              Account Resilience
            </span>
          </div>
          <div style={{ fontSize: '15px', fontWeight: 600, color: '#16a34a' }}>
            {config.account_resilience ? 'Active (Auto-Rebind)' : 'Disabled'}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Survives account rotations
          </div>
        </div>

        <div
          style={{
            backgroundColor: '#ffffff',
            borderRadius: '10px',
            padding: '14px 16px',
            border: '1px solid var(--border)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px' }}>
            <Tv size={16} color="var(--primary)" />
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
              CDP DevTools Port
            </span>
          </div>
          <div style={{ fontSize: '15px', fontWeight: 600, color: 'var(--text)' }}>
            {status.devtools_port > 0 ? status.devtools_port : 'Idle (Auto)'}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Live browser automation bridge
          </div>
        </div>

        <div
          style={{
            backgroundColor: '#ffffff',
            borderRadius: '10px',
            padding: '14px 16px',
            border: '1px solid var(--border)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px' }}>
            <Cpu size={16} color="var(--primary)" />
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
              Host Antigravity PID
            </span>
          </div>
          <div style={{ fontSize: '15px', fontWeight: 600, color: 'var(--text)' }}>
            {status.current_antigravity_pid > 0 ? status.current_antigravity_pid : 'Idle'}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Host process monitored
          </div>
        </div>

        <div
          style={{
            backgroundColor: '#ffffff',
            borderRadius: '10px',
            padding: '14px 16px',
            border: '1px solid var(--border)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px' }}>
            <Layers size={16} color="var(--primary)" />
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
              Active Google Account
            </span>
          </div>
          <div
            style={{
              fontSize: '13px',
              fontWeight: 600,
              color: 'var(--text)',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            {status.active_google_account || 'Not signed in'}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Syncs without dropped viewer
          </div>
        </div>
      </div>

      {/* 3. Connection & Security Card */}
      <div
        style={{
          backgroundColor: '#ffffff',
          borderRadius: '12px',
          padding: '20px 24px',
          border: '1px solid var(--border)',
          display: 'flex',
          flexDirection: 'column',
          gap: '16px',
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div>
            <h3 style={{ fontSize: '15px', fontWeight: 600, margin: 0, color: 'var(--text)' }}>
              Remote Access Endpoint & Pairing Security
            </h3>
            <p style={{ fontSize: '13px', color: 'var(--text-muted)', margin: '4px 0 0 0' }}>
              Connect from external browsers or mobile devices using your local network URL and Security PIN.
            </p>
          </div>
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
              Remote Web App URL
            </span>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <code style={{ fontSize: '13px', flex: 1, color: 'var(--text)', wordBreak: 'break-all' }}>
                {status.endpoint_url || `http://${config.bind_addr}:${config.port}/remote`}
              </code>
              <button
                onClick={() => copyToClipboard(status.endpoint_url || `http://${config.bind_addr}:${config.port}/remote`, 'url')}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '4px',
                  padding: '5px 10px',
                  borderRadius: '6px',
                  border: '1px solid var(--border)',
                  backgroundColor: '#ffffff',
                  fontSize: '12px',
                  cursor: 'pointer',
                  whiteSpace: 'nowrap',
                }}
              >
                {copied === 'url' ? <Check size={14} color="#16a34a" /> : <Copy size={14} />}
                {copied === 'url' ? 'Copied' : 'Copy'}
              </button>
              <a
                href={status.endpoint_url || `http://127.0.0.1:${config.port}/remote`}
                target="_blank"
                rel="noreferrer"
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '4px',
                  padding: '5px 10px',
                  borderRadius: '6px',
                  border: '1px solid var(--border)',
                  backgroundColor: '#ffffff',
                  fontSize: '12px',
                  color: 'var(--primary)',
                  textDecoration: 'none',
                  whiteSpace: 'nowrap',
                }}
              >
                <ExternalLink size={14} />
                Open
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
                style={{
                  background: 'none',
                  border: 'none',
                  padding: 0,
                  fontSize: '11px',
                  color: 'var(--primary)',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '3px',
                }}
              >
                <RotateCw size={12} />
                Rotate PIN
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
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '4px',
                  padding: '4px 8px',
                  borderRadius: '6px',
                  border: '1px solid var(--border)',
                  backgroundColor: '#ffffff',
                  fontSize: '11px',
                  cursor: 'pointer',
                  marginLeft: 'auto',
                }}
              >
                {copied === 'pin' ? <Check size={12} color="#16a34a" /> : <Copy size={12} />}
                {copied === 'pin' ? 'Copied' : 'Copy'}
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* 4. Configuration Settings */}
      <div
        style={{
          backgroundColor: '#ffffff',
          borderRadius: '12px',
          padding: '20px 24px',
          border: '1px solid var(--border)',
          display: 'flex',
          flexDirection: 'column',
          gap: '16px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Settings2 size={18} color="var(--primary)" />
          <h3 style={{ fontSize: '15px', fontWeight: 600, margin: 0, color: 'var(--text)' }}>
            Remote Control Settings
          </h3>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '16px' }}>
          <div>
            <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
              Listening Port
            </label>
            <input
              type="number"
              value={config.port}
              onChange={(e) => setConfig({ ...config, port: parseInt(e.target.value, 10) || 8766 })}
              style={{
                width: '100%',
                padding: '8px 12px',
                borderRadius: '8px',
                border: '1px solid var(--border)',
                fontSize: '13px',
                boxSizing: 'border-box',
              }}
            />
          </div>

          <div>
            <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
              Bind Address
            </label>
            <select
              value={config.bind_addr}
              onChange={(e) => setConfig({ ...config, bind_addr: e.target.value })}
              style={{
                width: '100%',
                padding: '8px 12px',
                borderRadius: '8px',
                border: '1px solid var(--border)',
                fontSize: '13px',
                backgroundColor: '#ffffff',
                boxSizing: 'border-box',
              }}
            >
              <option value="127.0.0.1">127.0.0.1 (Loopback only)</option>
              <option value="0.0.0.0">0.0.0.0 (Local Network LAN)</option>
            </select>
          </div>

          <div>
            <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
              Stream Quality
            </label>
            <select
              value={config.quality}
              onChange={(e) => setConfig({ ...config, quality: e.target.value })}
              style={{
                width: '100%',
                padding: '8px 12px',
                borderRadius: '8px',
                border: '1px solid var(--border)',
                fontSize: '13px',
                backgroundColor: '#ffffff',
                boxSizing: 'border-box',
              }}
            >
              <option value="high">High (1080p, 30fps)</option>
              <option value="medium">Medium (720p, 24fps)</option>
              <option value="low">Low (480p, 15fps)</option>
            </select>
          </div>
        </div>

        {/* Account Resilience Toggle */}
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
            <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
              Automatically retains connected clients and re-attaches CDP session when local Google accounts rotate.
            </div>
          </div>
          <button
            onClick={() => setConfig({ ...config, account_resilience: !config.account_resilience })}
            style={{
              padding: '6px 14px',
              borderRadius: '16px',
              border: 'none',
              fontSize: '12px',
              fontWeight: 600,
              cursor: 'pointer',
              whiteSpace: 'nowrap',
              backgroundColor: config.account_resilience ? 'var(--primary-container)' : 'var(--tonal)',
              color: config.account_resilience ? 'var(--primary)' : 'var(--text-muted)',
            }}
          >
            {config.account_resilience ? 'Enabled' : 'Disabled'}
          </button>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginTop: '4px' }}>
          <button
            onClick={handleSaveConfig}
            disabled={loading}
            style={{
              padding: '8px 18px',
              borderRadius: '20px',
              border: 'none',
              fontSize: '13px',
              fontWeight: 600,
              cursor: 'pointer',
              whiteSpace: 'nowrap',
              backgroundColor: 'var(--primary)',
              color: '#ffffff',
            }}
          >
            Save Settings
          </button>
          {saveSuccess && (
            <span style={{ fontSize: '12px', color: '#16a34a', display: 'flex', alignItems: 'center', gap: '4px' }}>
              <Check size={14} /> Settings saved successfully
            </span>
          )}
        </div>
      </div>

      {/* 5. Step-by-Step Guidance */}
      <div
        style={{
          backgroundColor: '#ffffff',
          borderRadius: '12px',
          padding: '20px 24px',
          border: '1px solid var(--border)',
          display: 'flex',
          flexDirection: 'column',
          gap: '12px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <HelpCircle size={18} color="var(--primary)" />
          <h3 style={{ fontSize: '15px', fontWeight: 600, margin: 0, color: 'var(--text)' }}>
            Guidance & Operating Principles
          </h3>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
          <div style={{ display: 'flex', gap: '12px', alignItems: 'flex-start' }}>
            <span
              style={{
                width: '22px',
                height: '22px',
                borderRadius: '50%',
                backgroundColor: 'var(--primary-container)',
                color: 'var(--primary)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: '12px',
                fontWeight: 700,
                flexShrink: 0,
              }}
            >
              1
            </span>
            <div style={{ fontSize: '13px', color: 'var(--text)', lineHeight: 1.5 }}>
              <strong>Zero Invalidation on Account Switch:</strong> Antigravity's factory remote session disconnects when switching accounts because the host Google access token expires. The Swiss Knife wrapper isolates the CDP WebSocket pipe from account tokens, keeping your web session persistent across account swaps.
            </div>
          </div>

          <div style={{ display: 'flex', gap: '12px', alignItems: 'flex-start' }}>
            <span
              style={{
                width: '22px',
                height: '22px',
                borderRadius: '50%',
                backgroundColor: 'var(--primary-container)',
                color: 'var(--primary)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: '12px',
                fontWeight: 700,
                flexShrink: 0,
              }}
            >
              2
            </span>
            <div style={{ fontSize: '13px', color: 'var(--text)', lineHeight: 1.5 }}>
              <strong>Access from Mobile or Second Screen:</strong> When Bind Address is set to <code>0.0.0.0</code> or routed via Cloudflare Quick Tunnel / Tailscale, open the URL on your tablet or smartphone, enter the 6-digit Security PIN once, and monitor or steer agents seamlessly.
            </div>
          </div>

          <div style={{ display: 'flex', gap: '12px', alignItems: 'flex-start' }}>
            <span
              style={{
                width: '22px',
                height: '22px',
                borderRadius: '50%',
                backgroundColor: 'var(--primary-container)',
                color: 'var(--primary)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: '12px',
                fontWeight: 700,
                flexShrink: 0,
              }}
            >
              3
            </span>
            <div style={{ fontSize: '13px', color: 'var(--text)', lineHeight: 1.5 }}>
              <strong>Security Protocol:</strong> All connections require matching the 6-digit Security PIN. If you need to revoke access immediately, click <em>Rotate PIN</em> to terminate existing view sessions instantly.
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default RemoteControlPage
