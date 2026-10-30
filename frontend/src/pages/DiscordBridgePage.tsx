import React, { useState, useEffect } from 'react'
import { ChannelRoutingSettings } from '../components/ChannelRoutingSettings'
import {
  Check,
  Power,
  Eye,
  EyeOff,
  AlertCircle,
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
      } catch {
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
    const previousEnabled = Boolean(config.discord.enabled)
    const nextEnabled = !previousEnabled
    const newConfig = {
      ...config,
      discord: { ...config.discord, enabled: nextEnabled },
    }
    setTestResult(null)
    setConfig(newConfig)
    try {
      await api.updateChannelsConfig(newConfig)
      await fetchStatusAndConfig()
    } catch (err: any) {
      setConfig((prev: any) => ({
        ...prev,
        discord: { ...prev.discord, enabled: previousEnabled },
      }))
      setTestResult({
        success: false,
        error: `Failed to toggle Discord bridge: ${err?.message || 'Unknown error'}`,
      })
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
      {status?.discord?.inbound_supported === false && (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            padding: '10px 14px',
            borderRadius: '8px',
            border: '1px solid var(--border)',
            backgroundColor: 'var(--canvas)',
            fontSize: '12px',
            color: 'var(--text-muted)',
          }}
        >
          <AlertCircle size={14} />
          <span>Outbound only: this build posts to Discord but cannot receive messages from it. Use WhatsApp or Telegram for inbound chat.</span>
        </div>
      )}

      {/* 1. Overview, Live Status & Enable Toggle */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)', whiteSpace: 'nowrap' }}>
                Discord Agent Chat Bridge
              </h2>
            </div>
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
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Application ID
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
              {config.discord.application_id || 'Unconfigured'}
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Allowed Channel IDs
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
              {config.discord.allowed_channels?.length > 0 ? `${config.discord.allowed_channels.length} Allowed` : 'All Guild Channels'}
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Last Message
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
              {dsStatus.last_message_at || 'Never'}
            </div>
          </div>
        </div>
      </div>

      {/* 2. Bot Configuration Settings */}
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
                <Check size={14} /> Discord settings saved
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
          <div>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '8px' }}>
              Discord Bot Token
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
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '8px' }}>
              Application ID (Client ID)
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
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '8px' }}>
                Allowed Channel IDs (Whitelist)
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
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '8px' }}>
                Allowed User IDs (Whitelist)
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

export default DiscordBridgePage
