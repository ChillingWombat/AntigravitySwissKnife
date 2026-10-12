import React, { useState, useEffect } from 'react'
import {
  MessageSquare,
  Power,
  ShieldCheck,
  RotateCw,
  Check,
  HelpCircle,
  Eye,
  EyeOff,
  Radio,
  Clock,
  Layers,
  Settings2,
} from 'lucide-react'
import { api } from '../api'

export const WhatsAppBridgePage: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const [showToken, setShowToken] = useState(false)
  const [testResult, setTestResult] = useState<any>(null)
  const [saveSuccess, setSaveSuccess] = useState(false)

  const [status, setStatus] = useState<any>({
    whatsapp: {
      enabled: false,
      connected: false,
      identity: '',
      status_text: 'Inactive',
      last_message_at: '',
    },
  })

  const [config, setConfig] = useState<any>({
    whatsapp: {
      enabled: false,
      mode: 'mcp',
      phone_number_id: '',
      access_token: '',
      verify_token: 'antigravity-whatsapp-verify',
      allowed_numbers: [],
    },
  })

  const [allowedNumbersStr, setAllowedNumbersStr] = useState('')

  const fetchStatusAndConfig = async () => {
    try {
      const res = await api.getChannelsStatus()
      if (res && res.status) {
        setStatus(res.status)
      }
      const cfgRes = await api.getChannelsConfig()
      if (cfgRes && cfgRes.config) {
        setConfig(cfgRes.config)
        if (cfgRes.config.whatsapp?.allowed_numbers) {
          setAllowedNumbersStr(cfgRes.config.whatsapp.allowed_numbers.join(', '))
        }
      }
    } catch (err) {
      console.error('Failed to load WhatsApp channel data:', err)
    }
  }

  useEffect(() => {
    fetchStatusAndConfig()
  }, [])

  const handleToggleEnable = async () => {
    const nextEnabled = !config.whatsapp.enabled
    const newConfig = {
      ...config,
      whatsapp: { ...config.whatsapp, enabled: nextEnabled },
    }
    setConfig(newConfig)
    try {
      await api.updateChannelsConfig(newConfig)
      await fetchStatusAndConfig()
    } catch (err) {
      console.error('Failed to toggle WhatsApp bridge:', err)
    }
  }

  const handleSaveConfig = async () => {
    setLoading(true)
    setSaveSuccess(false)
    const numbers = allowedNumbersStr
      .split(/[,;\n]/)
      .map((s) => s.trim())
      .filter(Boolean)

    const newConfig = {
      ...config,
      whatsapp: {
        ...config.whatsapp,
        allowed_numbers: numbers,
      },
    }

    try {
      const res = await api.updateChannelsConfig(newConfig)
      if (res && res.success) {
        if (res.config) setConfig(res.config)
        setSaveSuccess(true)
        setTimeout(() => setSaveSuccess(false), 3000)
      }
    } catch (err) {
      console.error('Failed to save WhatsApp config:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleTestConnection = async () => {
    setLoading(true)
    setTestResult(null)
    try {
      const res = await api.testChannel('whatsapp')
      setTestResult(res)
    } catch (err: any) {
      setTestResult({ success: false, error: err?.message || 'Connection test failed' })
    } finally {
      setLoading(false)
    }
  }

  const waStatus = status?.whatsapp || {}

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* 1. Header & Enable Bar */}
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
              backgroundColor: '#dcfce7',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <MessageSquare size={24} color="#15803d" />
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
                WhatsApp Agent Chat Bridge
              </h2>
              <span
                style={{
                  fontSize: '11px',
                  padding: '2px 8px',
                  borderRadius: '12px',
                  fontWeight: 600,
                  whiteSpace: 'nowrap',
                  backgroundColor: waStatus.enabled ? '#dcfce7' : 'var(--tonal)',
                  color: waStatus.enabled ? '#15803d' : 'var(--text-muted)',
                }}
              >
                {waStatus.enabled ? (waStatus.connected ? 'Connected' : 'Listening') : 'Disabled'}
              </span>
            </div>
            <p
              style={{
                fontSize: '13px',
                color: 'var(--text-muted)',
                margin: '4px 0 0 0',
              }}
            >
              Direct bidirectional chat channel between WhatsApp and local Antigravity autonomous agents.
            </p>
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <button
            onClick={handleTestConnection}
            disabled={loading}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              padding: '8px 14px',
              borderRadius: '20px',
              border: '1px solid var(--border)',
              backgroundColor: '#ffffff',
              fontSize: '13px',
              fontWeight: 500,
              cursor: 'pointer',
              whiteSpace: 'nowrap',
            }}
          >
            <RotateCw size={14} className={loading ? 'animate-spin' : ''} />
            Test Bridge
          </button>
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
              backgroundColor: config.whatsapp.enabled ? '#fee2e2' : 'var(--primary-container)',
              color: config.whatsapp.enabled ? '#b91c1c' : 'var(--primary)',
            }}
          >
            <Power size={15} />
            {config.whatsapp.enabled ? 'Disable Channel' : 'Enable Channel'}
          </button>
        </div>
      </div>

      {/* 2. Status & Metric Cards */}
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
            <Radio size={16} color="var(--primary)" />
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
              Connection Status
            </span>
          </div>
          <div style={{ fontSize: '15px', fontWeight: 600, color: waStatus.enabled ? '#16a34a' : 'var(--text-muted)' }}>
            {waStatus.status_text || (waStatus.enabled ? 'Online' : 'Standby')}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            {config.whatsapp.mode.toUpperCase()} Transport
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
              Identity & Phone
            </span>
          </div>
          <div style={{ fontSize: '14px', fontWeight: 600, color: 'var(--text)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {waStatus.identity || (config.whatsapp.phone_number_id ? `ID: ${config.whatsapp.phone_number_id}` : 'Local MCP Bridge')}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Verified sender target
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
            <ShieldCheck size={16} color="var(--primary)" />
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
              Whitelisted Senders
            </span>
          </div>
          <div style={{ fontSize: '15px', fontWeight: 600, color: 'var(--text)' }}>
            {config.whatsapp.allowed_numbers?.length > 0 ? `${config.whatsapp.allowed_numbers.length} Numbers` : 'Open (Any Sender)'}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Access policy enforcement
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
            <Clock size={16} color="var(--primary)" />
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
              Last Interaction
            </span>
          </div>
          <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
            {waStatus.last_message_at || 'Never'}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Bidirectional message log
          </div>
        </div>
      </div>

      {testResult && (
        <div
          style={{
            padding: '12px 16px',
            borderRadius: '8px',
            border: `1px solid ${testResult.success ? '#bbf7d0' : '#fecaca'}`,
            backgroundColor: testResult.success ? '#f0fdf4' : '#fef2f2',
            fontSize: '13px',
            color: testResult.success ? '#15803d' : '#b91c1c',
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
          }}
        >
          {testResult.success ? <Check size={16} /> : <Power size={16} />}
          <span>{testResult.message || testResult.error}</span>
        </div>
      )}

      {/* 3. Settings Form */}
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
            WhatsApp Channel Settings
          </h3>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '16px' }}>
          <div>
            <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
              Integration Mode
            </label>
            <select
              value={config.whatsapp.mode}
              onChange={(e) =>
                setConfig({
                  ...config,
                  whatsapp: { ...config.whatsapp, mode: e.target.value },
                })
              }
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
              <option value="mcp">Local MCP Server (whatsapp-mcp via Baileys/Web)</option>
              <option value="cloud_api">Meta Cloud API (Official Graph API)</option>
              <option value="webhook">Custom Inbound Webhook</option>
            </select>
          </div>

          <div>
            <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
              Phone Number ID / Sender ID
            </label>
            <input
              type="text"
              placeholder="e.g. 108234812398402 (leave blank for local MCP)"
              value={config.whatsapp.phone_number_id}
              onChange={(e) =>
                setConfig({
                  ...config,
                  whatsapp: { ...config.whatsapp, phone_number_id: e.target.value },
                })
              }
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
        </div>

        {config.whatsapp.mode === 'cloud_api' && (
          <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: '16px' }}>
            <div>
              <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
                Meta Permanent Access Token
              </label>
              <div style={{ position: 'relative' }}>
                <input
                  type={showToken ? 'text' : 'password'}
                  placeholder="EAA..."
                  value={config.whatsapp.access_token}
                  onChange={(e) =>
                    setConfig({
                      ...config,
                      whatsapp: { ...config.whatsapp, access_token: e.target.value },
                    })
                  }
                  style={{
                    width: '100%',
                    padding: '8px 40px 8px 12px',
                    borderRadius: '8px',
                    border: '1px solid var(--border)',
                    fontSize: '13px',
                    boxSizing: 'border-box',
                  }}
                />
                <button
                  type="button"
                  onClick={() => setShowToken(!showToken)}
                  style={{
                    position: 'absolute',
                    right: '10px',
                    top: '50%',
                    transform: 'translateY(-50%)',
                    background: 'none',
                    border: 'none',
                    cursor: 'pointer',
                    color: 'var(--text-muted)',
                  }}
                >
                  {showToken ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
            </div>

            <div>
              <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
                Webhook Verification Token
              </label>
              <input
                type="text"
                value={config.whatsapp.verify_token}
                onChange={(e) =>
                  setConfig({
                    ...config,
                    whatsapp: { ...config.whatsapp, verify_token: e.target.value },
                  })
                }
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
          </div>
        )}

        <div>
          <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
            Allowed Phone Numbers (Whitelist)
          </label>
          <input
            type="text"
            placeholder="+1234567890, +61400000000 (leave blank to allow all paired contacts)"
            value={allowedNumbersStr}
            onChange={(e) => setAllowedNumbersStr(e.target.value)}
            style={{
              width: '100%',
              padding: '8px 12px',
              borderRadius: '8px',
              border: '1px solid var(--border)',
              fontSize: '13px',
              boxSizing: 'border-box',
            }}
          />
          <span style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px', display: 'block' }}>
            Comma-separated international numbers with country code. Only whitelisted numbers can issue prompts to Antigravity agents.
          </span>
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
              <Check size={14} /> WhatsApp settings saved
            </span>
          )}
        </div>
      </div>

      {/* 4. Guidance Section */}
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
            WhatsApp Bridge Setup Guidance
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
              <strong>Choose Your Mode:</strong> The default <code>Local MCP Server</code> uses the built-in WhatsApp MCP driver (which connects through WhatsApp Web session stored in your keyring). Meta Cloud API connects directly to Meta's Business Platform for high-volume automated routing.
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
              <strong>Whitelist Numbers:</strong> To ensure privacy and security, specify your personal phone number in the whitelist. Messages from non-whitelisted numbers will be dropped silently without starting agent tasks.
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
              <strong>Agent Conversations:</strong> When you send a message via WhatsApp, Antigravity creates an autonomous session on your local host, runs the required tools, and forwards conversational answers and execution results back into your WhatsApp chat.
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default WhatsAppBridgePage
