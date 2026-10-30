import React, { useState, useEffect } from 'react'
import { ChannelRoutingSettings } from '../components/ChannelRoutingSettings'
import {
  Check,
  Power,
  Eye,
  EyeOff,
} from 'lucide-react'
import { ToggleSwitch } from '../components/ToggleSwitch'
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
    const previousEnabled = Boolean(config.whatsapp.enabled)
    const nextEnabled = !previousEnabled
    const newConfig = {
      ...config,
      whatsapp: { ...config.whatsapp, enabled: nextEnabled },
    }
    setTestResult(null)
    setConfig(newConfig)
    try {
      await api.updateChannelsConfig(newConfig)
      await fetchStatusAndConfig()
    } catch (err: any) {
      setConfig((prev: any) => ({
        ...prev,
        whatsapp: { ...prev.whatsapp, enabled: previousEnabled },
      }))
      setTestResult({
        success: false,
        error: `Failed to toggle WhatsApp bridge: ${err?.message || 'Unknown error'}`,
      })
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
        if (res.status) setStatus(res.status)
        setSaveSuccess(true)
        setTimeout(() => setSaveSuccess(false), 3000)
      } else {
        setTestResult({ success: false, error: (res as any)?.error || 'Failed to save configuration' })
      }
    } catch (err: any) {
      console.error('Failed to save WhatsApp config:', err)
      setTestResult({ success: false, error: err?.message || 'Failed to save configuration' })
    } finally {
      setLoading(false)
    }
  }

  const waStatus = status?.whatsapp || {}

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* 1. Overview, Live Status & Enable Toggle */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)', whiteSpace: 'nowrap' }}>
                WhatsApp Agent Chat Bridge
              </h2>
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '13px', fontWeight: 600 }}>
              <span style={{ whiteSpace: 'nowrap' }}>Enable Bridge</span>
              <ToggleSwitch
                checked={Boolean(config.whatsapp.enabled)}
                onChange={handleToggleEnable}
              />
            </label>
          </div>
        </div>

        {testResult && (
          <div
            style={{
              padding: '10px 14px',
              borderRadius: '8px',
              border: testResult.success ? '1px solid var(--border-success)' : '1px solid var(--border-danger)',
              backgroundColor: testResult.success ? 'var(--green-bg)' : 'var(--red-bg)',
              fontSize: '12px',
              color: testResult.success ? 'var(--green)' : 'var(--red)',
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              marginBottom: '16px',
            }}
          >
            {testResult.success ? <Check size={14} /> : <Power size={14} />}
            <span>{testResult.message || testResult.error}</span>
          </div>
        )}

        {/* Status Metrics Grid (harmonized with BrainCachePage & TokenMonitorPage) */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: '12px' }}>
          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Connection Status
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: waStatus.enabled ? 'var(--green)' : 'var(--text-muted)', marginTop: '4px' }}>
              {waStatus.status_text || (waStatus.enabled ? 'Online' : 'Standby')}
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Verified Identity
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
              {waStatus.identity || (config.whatsapp.phone_number_id ? `ID: ${config.whatsapp.phone_number_id}` : 'Local MCP Bridge')}
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Whitelisted Senders
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
              {config.whatsapp.allowed_numbers?.length > 0 ? `${config.whatsapp.allowed_numbers.length} Numbers` : 'Open (Any Sender)'}
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Last Interaction
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
              {waStatus.last_message_at || 'Never'}
            </div>
          </div>
        </div>
      </div>

      {/* 2. Channel Configuration Settings */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', marginBottom: '16px', flexWrap: 'wrap' }}>
          <div>
            <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)', whiteSpace: 'nowrap' }}>
              Setup
            </h2>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            {saveSuccess && (
              <span style={{ fontSize: '12px', color: 'var(--green)', display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                <Check size={14} /> WhatsApp settings saved
              </span>
            )}
            <button
              type="button"
              onClick={handleSaveConfig}
              disabled={loading}
              className="btn-pill-primary"
            >
              Save Settings
            </button>
          </div>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '16px' }}>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '8px' }}>
                Integration Mode
              </div>
              <select
                value={config.whatsapp.mode}
                onChange={(e) =>
                  setConfig({
                    ...config,
                    whatsapp: { ...config.whatsapp, mode: e.target.value },
                  })
                }
                style={{ width: '100%' }}
              >
                <option value="mcp">Local MCP Server (whatsapp-mcp via Baileys/Web)</option>
                <option value="cloud_api">Meta Cloud API (Official Graph API)</option>
                <option value="webhook">Custom Inbound Webhook</option>
              </select>
            </div>

            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '8px' }}>
                Phone Number ID / Sender ID
              </div>
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
                style={{ width: '100%' }}
              />
            </div>
          </div>

          {config.whatsapp.mode === 'cloud_api' ? (
            <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: '16px' }}>
              <div>
                <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '8px' }}>
                  Meta Permanent Access Token
                </div>
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
                    style={{ width: '100%', paddingRight: '36px' }}
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
                      display: 'flex',
                      alignItems: 'center',
                    }}
                  >
                    {showToken ? <EyeOff size={16} /> : <Eye size={16} />}
                  </button>
                </div>
              </div>

              <div>
                <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '8px' }}>
                  Webhook Verification Token
                </div>
                <input
                  type="text"
                  value={config.whatsapp.verify_token}
                  onChange={(e) =>
                    setConfig({
                      ...config,
                      whatsapp: { ...config.whatsapp, verify_token: e.target.value },
                    })
                  }
                  style={{ width: '100%' }}
                />
              </div>
            </div>
          ) : (
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '8px' }}>
                Webhook Verification Token
              </div>
              <input
                type="text"
                value={config.whatsapp.verify_token}
                onChange={(e) =>
                  setConfig({
                    ...config,
                    whatsapp: { ...config.whatsapp, verify_token: e.target.value },
                  })
                }
                style={{ width: '100%' }}
              />
            </div>
          )}

          <div>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '8px' }}>
              Allowed Phone Numbers (Whitelist)
            </div>
            <input
              type="text"
              placeholder="+1234567890, +61400000000 (leave blank to allow all paired contacts)"
              value={allowedNumbersStr}
              onChange={(e) => setAllowedNumbersStr(e.target.value)}
              style={{ width: '100%' }}
            />
          </div>

          <ChannelRoutingSettings
            config={config}
            disabled={loading}
            onChange={(next) => setConfig((prev: any) => ({ ...prev, ...next }))}
          />
        </div>
      </div>

    </div>
  )
}

export default WhatsAppBridgePage
