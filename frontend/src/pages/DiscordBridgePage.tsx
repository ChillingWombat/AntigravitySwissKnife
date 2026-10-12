import React, { useState, useEffect } from 'react'
import {
  MessageSquareCode,
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

export const DiscordBridgePage: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const [showToken, setShowToken] = useState(false)
  const [testResult, setTestResult] = useState<any>(null)
  const [saveSuccess, setSaveSuccess] = useState(false)

  const [status, setStatus] = useState<any>({
    discord: {
      enabled: false,
      connected: false,
      identity: '',
      status_text: 'Inactive',
      last_message_at: '',
    },
  })

  const [config, setConfig] = useState<any>({
    discord: {
      enabled: false,
      bot_token: '',
      application_id: '',
      allowed_channels: [],
      allowed_users: [],
    },
  })

  const [allowedChannelsStr, setAllowedChannelsStr] = useState('')
  const [allowedUsersStr, setAllowedUsersStr] = useState('')

  const fetchStatusAndConfig = async () => {
    try {
      const res = await api.getChannelsStatus()
      if (res && res.status) {
        setStatus(res.status)
      }
      const cfgRes = await api.getChannelsConfig()
      if (cfgRes && cfgRes.config) {
        setConfig(cfgRes.config)
        if (cfgRes.config.discord?.allowed_channels) {
          setAllowedChannelsStr(cfgRes.config.discord.allowed_channels.join(', '))
        }
        if (cfgRes.config.discord?.allowed_users) {
          setAllowedUsersStr(cfgRes.config.discord.allowed_users.join(', '))
        }
      }
    } catch (err) {
      console.error('Failed to load Discord channel data:', err)
    }
  }

  useEffect(() => {
    fetchStatusAndConfig()
  }, [])

  const handleToggleEnable = async () => {
    const nextEnabled = !config.discord.enabled
    const newConfig = {
      ...config,
      discord: { ...config.discord, enabled: nextEnabled },
    }
    setConfig(newConfig)
    try {
      await api.updateChannelsConfig(newConfig)
      await fetchStatusAndConfig()
    } catch (err) {
      console.error('Failed to toggle Discord bridge:', err)
    }
  }

  const handleSaveConfig = async () => {
    setLoading(true)
    setSaveSuccess(false)
    const channels = allowedChannelsStr
      .split(/[,;\n]/)
      .map((s) => s.trim())
      .filter(Boolean)
    const users = allowedUsersStr
      .split(/[,;\n]/)
      .map((s) => s.trim())
      .filter(Boolean)

    const newConfig = {
      ...config,
      discord: {
        ...config.discord,
        allowed_channels: channels,
        allowed_users: users,
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
      console.error('Failed to save Discord config:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleTestConnection = async () => {
    setLoading(true)
    setTestResult(null)
    try {
      const res = await api.testChannel('discord')
      setTestResult(res)
    } catch (err: any) {
      setTestResult({ success: false, error: err?.message || 'Connection test failed' })
    } finally {
      setLoading(false)
    }
  }

  const dsStatus = status?.discord || {}

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
              backgroundColor: '#ede9fe',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <MessageSquareCode size={24} color="#7c3aed" />
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
                Discord Agent Chat Bridge
              </h2>
              <span
                style={{
                  fontSize: '11px',
                  padding: '2px 8px',
                  borderRadius: '12px',
                  fontWeight: 600,
                  whiteSpace: 'nowrap',
                  backgroundColor: dsStatus.enabled ? '#dcfce7' : 'var(--tonal)',
                  color: dsStatus.enabled ? '#15803d' : 'var(--text-muted)',
                }}
              >
                {dsStatus.enabled ? (dsStatus.connected ? 'Gateway Connected' : 'Gateway Ready') : 'Disabled'}
              </span>
            </div>
            <p
              style={{
                fontSize: '13px',
                color: 'var(--text-muted)',
                margin: '4px 0 0 0',
              }}
            >
              Control Antigravity autonomous agents and stream execution logs into Discord channels and DMs.
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
            Test Bot
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
              backgroundColor: config.discord.enabled ? '#fee2e2' : 'var(--primary-container)',
              color: config.discord.enabled ? '#b91c1c' : 'var(--primary)',
            }}
          >
            <Power size={15} />
            {config.discord.enabled ? 'Disable Channel' : 'Enable Channel'}
          </button>
        </div>
      </div>

      {/* 2. Status Cards */}
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
              Gateway Status
            </span>
          </div>
          <div style={{ fontSize: '15px', fontWeight: 600, color: dsStatus.enabled ? '#16a34a' : 'var(--text-muted)' }}>
            {dsStatus.status_text || (dsStatus.enabled ? 'Gateway Ready' : 'Standby')}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Discord WebSocket Gateway v10
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
              Application ID
            </span>
          </div>
          <div style={{ fontSize: '14px', fontWeight: 600, color: 'var(--text)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {config.discord.application_id || 'Unset'}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Developer Portal App ID
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
              Whitelisted Channels
            </span>
          </div>
          <div style={{ fontSize: '15px', fontWeight: 600, color: 'var(--text)' }}>
            {config.discord.allowed_channels?.length > 0 ? `${config.discord.allowed_channels.length} Channels` : 'All Guild Channels'}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Security scope filter
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
              Last Message
            </span>
          </div>
          <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
            {dsStatus.last_message_at || 'Never'}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Discord event listener
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
            Discord Bot Settings
          </h3>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: '16px' }}>
          <div>
            <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
              Discord Bot Token
            </label>
            <div style={{ position: 'relative' }}>
              <input
                type={showToken ? 'text' : 'password'}
                placeholder="e.g. MTIzNDU2Nzg5MDEyMzQ1Njc4OQ..."
                value={config.discord.bot_token}
                onChange={(e) =>
                  setConfig({
                    ...config,
                    discord: { ...config.discord, bot_token: e.target.value },
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
              Application ID / Client ID
            </label>
            <input
              type="text"
              placeholder="e.g. 1092837465019283"
              value={config.discord.application_id}
              onChange={(e) =>
                setConfig({
                  ...config,
                  discord: { ...config.discord, application_id: e.target.value },
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

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '16px' }}>
          <div>
            <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
              Allowed Channel IDs (Whitelist)
            </label>
            <input
              type="text"
              placeholder="e.g. 119283746501928374, 998877665544 (leave blank for all)"
              value={allowedChannelsStr}
              onChange={(e) => setAllowedChannelsStr(e.target.value)}
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
              Comma-separated numerical Discord channel IDs where the bot is permitted to respond.
            </span>
          </div>

          <div>
            <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
              Allowed User IDs (Whitelist)
            </label>
            <input
              type="text"
              placeholder="e.g. 3344556677889900 (leave blank to allow all server members)"
              value={allowedUsersStr}
              onChange={(e) => setAllowedUsersStr(e.target.value)}
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
              Restricts agent invocation to specific Discord user accounts.
            </span>
          </div>
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
              <Check size={14} /> Discord settings saved
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
            Discord Bot Setup Guidance
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
              <strong>Create Application & Bot:</strong> Visit the Discord Developer Portal, click <em>New Application</em>, and navigate to the <em>Bot</em> section. Reset and copy your Bot Token.
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
              <strong>Enable Privileged Intents:</strong> Under the Bot tab in the Developer Portal, toggle on <em>Message Content Intent</em> so the bot can read instructions in whitelisted channels.
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
              <strong>Invite & Converse:</strong> Under OAuth2 URL Generator, check <code>bot</code> scope with <code>Send Messages</code> and <code>Read Message History</code>. Invite the bot to your server, mention it or DM it, and command Antigravity from Discord!
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default DiscordBridgePage
