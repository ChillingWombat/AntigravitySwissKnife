import React, { useState, useEffect } from 'react'
import {
  Check,
  Power,
  Eye,
  EyeOff,
} from 'lucide-react'
import { ToggleSwitch } from '../components/ToggleSwitch'
import { api } from '../api'

export const TelegramBridgePage: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const [showToken, setShowToken] = useState(false)
  const [testResult, setTestResult] = useState<any>(null)
  const [saveSuccess, setSaveSuccess] = useState(false)

  const [status, setStatus] = useState<any>({
    telegram: {
      enabled: false,
      connected: false,
      identity: '',
      status_text: 'Inactive',
      last_message_at: '',
    },
  })

  const [config, setConfig] = useState<any>({
    telegram: {
      enabled: false,
      bot_token: '',
      allowed_chat_ids: [],
      polling: true,
      webhook_url: '',
    },
  })

  const [allowedChatIdsStr, setAllowedChatIdsStr] = useState('')

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
        if (cfgRes.config.telegram?.allowed_chat_ids) {
          setAllowedChatIdsStr(cfgRes.config.telegram.allowed_chat_ids.join(', '))
        }
      }
    } catch (err) {
      console.error('Failed to load Telegram channel data:', err)
    }
  }

  useEffect(() => {
    fetchStatusAndConfig()
  }, [])

  const handleToggleEnable = async () => {
    const nextEnabled = !config.telegram.enabled
    const newConfig = {
      ...config,
      telegram: { ...config.telegram, enabled: nextEnabled },
    }
    setConfig(newConfig)
    try {
      await api.updateChannelsConfig(newConfig)
      await fetchStatusAndConfig()
    } catch (err) {
      console.error('Failed to toggle Telegram bridge:', err)
    }
  }

  const handleSaveConfig = async () => {
    setLoading(true)
    setSaveSuccess(false)
    const chatIds = allowedChatIdsStr
      .split(/[,;\n]/)
      .map((s) => s.trim())
      .filter(Boolean)

    const newConfig = {
      ...config,
      telegram: {
        ...config.telegram,
        allowed_chat_ids: chatIds,
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
      console.error('Failed to save Telegram config:', err)
      setTestResult({ success: false, error: err?.message || 'Failed to save configuration' })
    } finally {
      setLoading(false)
    }
  }

  const tgStatus = status?.telegram || {}

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* 1. Overview, Live Status & Enable Toggle */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)', whiteSpace: 'nowrap' }}>
                Telegram Agent Chat Bridge
              </h2>
            </div>
            <p style={{ margin: '4px 0 0', fontSize: '12px', color: 'var(--text-muted)' }}>
              Direct bidirectional chat channel between Telegram bots and local Antigravity autonomous agents.
            </p>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '13px', fontWeight: 600 }}>
              <span style={{ whiteSpace: 'nowrap' }}>Enable Bridge</span>
              <ToggleSwitch
                checked={Boolean(config.telegram.enabled)}
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
              Bot Status
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: tgStatus.enabled ? 'var(--green)' : 'var(--text-muted)', marginTop: '4px' }}>
              {tgStatus.status_text || (tgStatus.enabled ? 'Long Polling Active' : 'Standby')}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              {config.telegram.polling ? 'Zero-Port Polling' : 'Webhook Mode'}
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Bot Identity
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
              {tgStatus.identity || (config.telegram.bot_token ? 'Configured Token' : 'Unconfigured')}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Telegram Bot Father API
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Allowed Chat IDs
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
              {config.telegram.allowed_chat_ids?.length > 0 ? `${config.telegram.allowed_chat_ids.length} Allowed` : 'Open Whitelist'}
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Restricts bot interaction
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, textTransform: 'uppercase' }}>
              Last Message
            </div>
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
              {tgStatus.last_message_at || 'Never'}
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
              Bot Token (from @BotFather)
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '8px' }}>
              API token generated by Telegram's official @BotFather bot for your custom assistant.
            </div>
            <div style={{ position: 'relative' }}>
              <input
                type={showToken ? 'text' : 'password'}
                placeholder="e.g. 123456789:ABCdefGhIJKlmNoPQRsTUVwxyZ"
                value={config.telegram.bot_token}
                onChange={(e) =>
                  setConfig({
                    ...config,
                    telegram: { ...config.telegram, bot_token: e.target.value },
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

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '16px' }}>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
                Delivery Mode
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '8px' }}>
                Long polling requires zero open inbound firewall ports.
              </div>
              <select
                value={config.telegram.polling ? 'polling' : 'webhook'}
                onChange={(e) =>
                  setConfig({
                    ...config,
                    telegram: { ...config.telegram, polling: e.target.value === 'polling' },
                  })
                }
                style={{ width: '100%' }}
              >
                <option value="polling">Long Polling (Zero Network Configuration)</option>
                <option value="webhook">Webhook (Direct Inbound Push)</option>
              </select>
            </div>

            {!config.telegram.polling && (
              <div>
                <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
                  Webhook URL
                </div>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '8px' }}>
                  Publicly reachable HTTPS endpoint for Telegram push notifications.
                </div>
                <input
                  type="text"
                  placeholder="https://your-domain.com/api/telegram"
                  value={config.telegram.webhook_url}
                  onChange={(e) =>
                    setConfig({
                      ...config,
                      telegram: { ...config.telegram, webhook_url: e.target.value },
                    })
                  }
                  style={{ width: '100%' }}
                />
              </div>
            )}
          </div>

          <div>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
              Allowed Chat IDs (Whitelist)
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '8px' }}>
              Comma-separated numeric user or group IDs. Leave empty to allow any chat that interacts with your bot.
            </div>
            <input
              type="text"
              placeholder="e.g. 12345678, 98765432 (leave blank to allow all)"
              value={allowedChatIdsStr}
              onChange={(e) => setAllowedChatIdsStr(e.target.value)}
              style={{ width: '100%' }}
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
                <Check size={14} /> Telegram settings saved
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
              Bot Token Setup
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', lineHeight: '1.45' }}>
              Open Telegram and search for <code>@BotFather</code>. Send <code>/newbot</code>, follow the prompts to choose a name and username, and paste the generated API token into Bot Token above.
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '2px' }}>
              Zero-Port Long Polling
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', lineHeight: '1.45' }}>
              Long Polling opens an outbound connection to Telegram's cloud servers. It requires zero router port-forwarding, zero dynamic DNS, and zero reverse proxy configuration.
            </div>
          </div>

          <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px 14px' }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)', marginBottom: '2px' }}>
              Chat Whitelisting & Privacy
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', lineHeight: '1.45' }}>
              Add your numerical Telegram user ID to Allowed Chat IDs to prevent unauthorized queries. Send <code>/start</code> to your bot in Telegram to begin chatting with Antigravity agents.
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default TelegramBridgePage
