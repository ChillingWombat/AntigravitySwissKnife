import React, { useState, useEffect } from 'react'
import {
  Send,
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
      const cfgRes = await api.getChannelsConfig()
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
        setSaveSuccess(true)
        setTimeout(() => setSaveSuccess(false), 3000)
      }
    } catch (err) {
      console.error('Failed to save Telegram config:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleTestConnection = async () => {
    setLoading(true)
    setTestResult(null)
    try {
      const res = await api.testChannel('telegram')
      setTestResult(res)
    } catch (err: any) {
      setTestResult({ success: false, error: err?.message || 'Connection test failed' })
    } finally {
      setLoading(false)
    }
  }

  const tgStatus = status?.telegram || {}

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
              backgroundColor: '#e0f2fe',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Send size={24} color="#0284c7" />
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
                Telegram Agent Chat Bridge
              </h2>
              <span
                style={{
                  fontSize: '11px',
                  padding: '2px 8px',
                  borderRadius: '12px',
                  fontWeight: 600,
                  whiteSpace: 'nowrap',
                  backgroundColor: tgStatus.enabled ? '#dcfce7' : 'var(--tonal)',
                  color: tgStatus.enabled ? '#15803d' : 'var(--text-muted)',
                }}
              >
                {tgStatus.enabled ? (tgStatus.connected ? 'Connected' : 'Polling Active') : 'Disabled'}
              </span>
            </div>
            <p
              style={{
                fontSize: '13px',
                color: 'var(--text-muted)',
                margin: '4px 0 0 0',
              }}
            >
              Control Antigravity autonomous agents and receive real-time answers via a private Telegram bot.
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
              backgroundColor: config.telegram.enabled ? '#fee2e2' : 'var(--primary-container)',
              color: config.telegram.enabled ? '#b91c1c' : 'var(--primary)',
            }}
          >
            <Power size={15} />
            {config.telegram.enabled ? 'Disable Channel' : 'Enable Channel'}
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
              Bot Status
            </span>
          </div>
          <div style={{ fontSize: '15px', fontWeight: 600, color: tgStatus.enabled ? '#16a34a' : 'var(--text-muted)' }}>
            {tgStatus.status_text || (tgStatus.enabled ? 'Long Polling Active' : 'Standby')}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            {config.telegram.polling ? 'Zero-Port Polling' : 'Webhook Mode'}
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
              Bot Identity
            </span>
          </div>
          <div style={{ fontSize: '14px', fontWeight: 600, color: 'var(--text)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {tgStatus.identity || (config.telegram.bot_token ? 'Configured Token' : 'Unconfigured')}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Telegram Bot Father API
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
              Allowed Chat IDs
            </span>
          </div>
          <div style={{ fontSize: '15px', fontWeight: 600, color: 'var(--text)' }}>
            {config.telegram.allowed_chat_ids?.length > 0 ? `${config.telegram.allowed_chat_ids.length} Allowed` : 'Open Whitelist'}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Restricts bot interaction
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
            {tgStatus.last_message_at || 'Never'}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Latest agent dispatch
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
            Telegram Bot Settings
          </h3>
        </div>

        <div>
          <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
            Bot Token (from @BotFather)
          </label>
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

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 2fr', gap: '16px' }}>
          <div>
            <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
              Transport Method
            </label>
            <select
              value={config.telegram.polling ? 'polling' : 'webhook'}
              onChange={(e) =>
                setConfig({
                  ...config,
                  telegram: { ...config.telegram, polling: e.target.value === 'polling' },
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
              <option value="polling">Long Polling (Zero setup, no ports needed)</option>
              <option value="webhook">Inbound Webhook</option>
            </select>
          </div>

          {!config.telegram.polling && (
            <div>
              <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
                Webhook URL
              </label>
              <input
                type="text"
                placeholder="https://your-domain.com/api/bridge/telegram/webhook"
                value={config.telegram.webhook_url}
                onChange={(e) =>
                  setConfig({
                    ...config,
                    telegram: { ...config.telegram, webhook_url: e.target.value },
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
          )}
        </div>

        <div>
          <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
            Allowed Chat IDs / User Whitelist
          </label>
          <input
            type="text"
            placeholder="e.g. 123456789, @yourusername (leave blank to allow any authenticated user)"
            value={allowedChatIdsStr}
            onChange={(e) => setAllowedChatIdsStr(e.target.value)}
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
            Comma-separated numerical Telegram Chat IDs. Prevents unauthorized users from commanding local Antigravity agents.
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
              <Check size={14} /> Telegram settings saved
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
            Telegram Bot Setup Guidance
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
              <strong>Create Your Bot:</strong> Open Telegram and send <code>/newbot</code> to <code>@BotFather</code>. Follow the prompts to assign a name and handle. BotFather will provide an HTTP API token.
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
              <strong>Long Polling Simplicity:</strong> Keep <code>Long Polling</code> selected. It establishes an outbound TLS connection to Telegram servers so you don't need any public IP, port forward, or reverse proxy.
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
              <strong>Chat Whitelisting:</strong> Add your numerical Telegram user ID to the whitelist to prevent external users from querying your local system. Send <code>/start</code> to your bot in Telegram to begin chatting with Antigravity agents!
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default TelegramBridgePage
