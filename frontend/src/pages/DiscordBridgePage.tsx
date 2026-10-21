import React, { useState, useEffect } from 'react'
import {
  Check,
  Power,
  Eye,
  EyeOff,
} from 'lucide-react'
import { ToggleSwitch } from '../components/ToggleSwitch'
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
      let cfgRes: any = null
      try {
        cfgRes = await api.getChannelsConfig(true)
      } catch (_) {
        cfgRes = await api.getChannelsConfig()
      }
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
        if (res.status) setStatus(res.status)
        setSaveSuccess(true)
        setTimeout(() => setSaveSuccess(false), 3000)
      } else {
        setTestResult({ success: false, error: (res as any)?.error || 'Failed to save configuration' })
      }
    } catch (err: any) {
      console.error('Failed to save Discord config:', err)
      setTestResult({ success: false, error: err?.message || 'Failed to save configuration' })
    } finally {
      setLoading(false)
    }
  }

  const dsStatus = status?.discord || {}

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* 1. Overview, Live Status & Enable Toggle */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)', whiteSpace: 'nowrap' }}>
                Discord Agent Chat Bridge
              </h2>
            </div>
            <p style={{ margin: '4px 0 0', fontSize: '12px', color: 'var(--text-muted)' }}>
              Direct bidirectional chat channel between Discord bots and local Antigravity autonomous agents.
            </p>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '13px', fontWeight: 600 }}>
              <span style={{ whiteSpace: 'nowrap' }}>Enable Bridge</span>
              <ToggleSwitch
                checked={Boolean(config.discord.enabled)}
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
              Gateway Status
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: dsStatus.enabled ? 'var(--green)' : 'var(--text-muted)', marginTop: '4px' }}>
              {dsStatus.status_text || (dsStatus.enabled ? 'Gateway Ready' : 'Standby')}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Discord Gateway v10
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Application ID
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
              {config.discord.application_id || 'Unconfigured'}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Developer Portal App ID
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Allowed Channel IDs
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
              {config.discord.allowed_channels?.length > 0 ? `${config.discord.allowed_channels.length} Allowed` : 'All Guild Channels'}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Restricts bot channels
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Last Message
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
              {dsStatus.last_message_at || 'Never'}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Latest agent dispatch
            </div>
          </div>
        </div>
      </div>

      {/* 2. Bot Configuration Settings */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '16px' }}>
          Bot Configuration
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <div>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
              Discord Bot Token
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '8px' }}>
              Token generated under the Bot tab in the Discord Developer Portal.
            </div>
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
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
              Application ID (Client ID)
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '8px' }}>
              Discord Developer Portal General Information Application ID.
            </div>
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
              style={{ width: '100%' }}
            />
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '16px' }}>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
                Allowed Channel IDs (Whitelist)
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '8px' }}>
                Comma-separated Discord channel IDs. Leave blank to allow any channel the bot is added to.
              </div>
              <input
                type="text"
                placeholder="e.g. 119283746501928374, 998877665544 (leave blank for all)"
                value={allowedChannelsStr}
                onChange={(e) => setAllowedChannelsStr(e.target.value)}
                style={{ width: '100%' }}
              />
            </div>

            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
                Allowed User IDs (Whitelist)
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '8px' }}>
                Comma-separated Discord user IDs. Leave blank to allow all server members.
              </div>
              <input
                type="text"
                placeholder="e.g. 3344556677889900 (leave blank to allow all)"
                value={allowedUsersStr}
                onChange={(e) => setAllowedUsersStr(e.target.value)}
                style={{ width: '100%' }}
              />
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
                <Check size={14} /> Discord settings saved
              </span>
            )}
          </div>
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
              Create Application & Bot
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', lineHeight: '1.45' }}>
              Visit the Discord Developer Portal, click <em>New Application</em>, navigate to the <em>Bot</em> section, reset and copy your Bot Token into Discord Bot Token above.
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '2px' }}>
              Privileged Gateway Intents
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', lineHeight: '1.45' }}>
              Under the Bot tab in the Developer Portal, enable <em>Message Content Intent</em> so the bot can read instructions in whitelisted channels and direct messages.
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '2px' }}>
              OAuth2 Invite & Permissions
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', lineHeight: '1.45' }}>
              Under OAuth2 URL Generator, select <code>bot</code> scope with <code>Send Messages</code> and <code>Read Message History</code>. Invite the bot to your server, mention it or DM it, and command Antigravity from Discord.
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default DiscordBridgePage
