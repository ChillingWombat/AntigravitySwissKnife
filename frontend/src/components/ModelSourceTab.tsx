import React, { useEffect, useState } from 'react'
import { AlertCircle, AlertTriangle, Copy, Check, Play, RefreshCw } from 'lucide-react'
import { api } from '../api'

type AppKey = 'antigravity' | 'vscode'

interface AppProfile {
  source: 'native' | 'custom'
  subagents: 'app' | 'cli'
}

interface Integration {
  mode: 'profile_isolation' | 'api_proxy'
  apps: Record<AppKey, AppProfile>
  proxy: { scope: 'antigravity' | 'external'; pin?: string }
}

const DEFAULT_INTEGRATION: Integration = {
  mode: 'profile_isolation',
  apps: {
    antigravity: { source: 'native', subagents: 'app' },
    vscode: { source: 'native', subagents: 'app' },
  },
  proxy: { scope: 'antigravity', pin: '' },
}

const wrappingTextStyle: React.CSSProperties = {
  whiteSpace: 'normal',
  overflowWrap: 'break-word',
  wordBreak: 'break-word',
  minWidth: 0,
}

const cardStyle: React.CSSProperties = {
  ...wrappingTextStyle,
  overflow: 'hidden',
  backgroundColor: 'var(--surface)',
  border: '1px solid var(--border)',
  borderRadius: '12px',
  padding: '18px 20px',
  display: 'flex',
  flexDirection: 'column',
  gap: '14px',
}

const optionCardStyle: React.CSSProperties = {
  ...wrappingTextStyle,
  overflow: 'hidden',
  textAlign: 'left',
  padding: '14px',
  borderRadius: '10px',
  backgroundColor: 'var(--canvas)',
  cursor: 'pointer',
}

const sectionLabelStyle: React.CSSProperties = {
  fontSize: '11px',
  fontWeight: 700,
  color: 'var(--text-muted)',
  letterSpacing: '0.8px',
  textTransform: 'uppercase',
}

const segmentedStyle: React.CSSProperties = {
  display: 'inline-flex',
  gap: '2px',
  padding: '3px',
  borderRadius: '8px',
  backgroundColor: 'var(--tonal)',
}

function segmentStyle(active: boolean): React.CSSProperties {
  return {
    border: 'none',
    borderRadius: '6px',
    padding: '6px 14px',
    fontSize: '12px',
    fontWeight: active ? 600 : 500,
    color: active ? 'var(--primary)' : 'var(--text-muted)',
    backgroundColor: active ? 'var(--surface)' : 'transparent',
    cursor: 'pointer',
    whiteSpace: 'nowrap',
  }
}

const actionStyle: React.CSSProperties = {
  display: 'inline-flex',
  alignItems: 'center',
  gap: '6px',
  border: '1px solid var(--border)',
  borderRadius: '18px',
  padding: '6px 14px',
  fontSize: '12px',
  fontWeight: 600,
  color: 'var(--text)',
  backgroundColor: 'var(--surface)',
  cursor: 'pointer',
  whiteSpace: 'nowrap',
}

const riskStyle = (tone: 'orange' | 'red'): React.CSSProperties => ({
  display: 'inline-flex',
  alignItems: 'center',
  gap: '6px',
  fontSize: '11px',
  fontWeight: 600,
  whiteSpace: 'nowrap',
  color: tone === 'orange' ? '#b45309' : '#b91c1c',
})

export const ModelSourceTab: React.FC = () => {
  const [integration, setIntegration] = useState<Integration>(DEFAULT_INTEGRATION)
  const [endpoint, setEndpoint] = useState('')
  const [geminiBase, setGeminiBase] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [launching, setLaunching] = useState<AppKey | null>(null)
  const [feedback, setFeedback] = useState<{ text: string; isError: boolean } | null>(null)
  const [copied, setCopied] = useState(false)
  const [copiedGemini, setCopiedGemini] = useState(false)
  const [cliStatus, setCliStatus] = useState<{ installed: boolean; configured: boolean; cli_path: string } | null>(null)
  const [configuringCli, setConfiguringCli] = useState(false)

  const load = async () => {
    setLoading(true)
    try {
      const res = await api.getCustomModelIntegration()
      if (res?.integration) setIntegration({ ...DEFAULT_INTEGRATION, ...res.integration })
      setEndpoint(res?.endpoint || '')
      setGeminiBase(res?.gemini_base || '')
      try {
        const cliRes = await api.getCLIGatewayStatus()
        if (cliRes?.status) setCliStatus(cliRes.status)
      } catch {}
    } catch (err) {
      setFeedback({ text: `Could not load settings: ${(err as Error).message}`, isError: true })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  const save = async (next: Integration) => {
    setIntegration(next)
    setSaving(true)
    setFeedback(null)
    try {
      const res = await api.updateCustomModelIntegration(next)
      if (res?.integration) setIntegration({ ...DEFAULT_INTEGRATION, ...res.integration })
    } catch (err) {
      setFeedback({ text: `Save failed: ${(err as Error).message}`, isError: true })
    } finally {
      setSaving(false)
    }
  }

  const setApp = (app: AppKey, patch: Partial<AppProfile>) => {
    const next: Integration = {
      ...integration,
      apps: { ...integration.apps, [app]: { ...integration.apps[app], ...patch } },
    }
    save(next)
  }

  const launch = async (app: AppKey) => {
    setLaunching(app)
    setFeedback(null)
    try {
      const res = await api.launchCustomModelApp(app)
      const source = res?.source === 'custom' ? 'custom models' : 'native models'
      const note = res?.launch_note ? ` ${res.launch_note}` : ''
      const settings = res?.settings ? ` (${res.settings})` : ''
      setFeedback({ text: `Started ${app === 'antigravity' ? 'Antigravity 2.0' : 'VS Code'} with ${source}${settings}.${note}`, isError: false })
    } catch (err) {
      setFeedback({ text: `Launch failed: ${(err as Error).message}`, isError: true })
    } finally {
      setLaunching(null)
    }
  }

  const handleSetupCli = async () => {
    setConfiguringCli(true)
    try {
      const res = await api.setupCLIGateway()
      if (res?.status) {
        setCliStatus(res.status)
        setFeedback({ text: 'AGY CLI integration successfully configured.', isError: false })
      }
    } catch (err) {
      setFeedback({ text: `CLI configuration failed: ${(err as Error).message}`, isError: true })
    } finally {
      setConfiguringCli(false)
    }
  }

  const copyEndpoint = async () => {
    try {
      const url = endpoint || 'http://127.0.0.1:8765/v1'
      await navigator.clipboard.writeText(url)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      setFeedback({ text: 'Clipboard is not available', isError: true })
    }
  }

  const copyGemini = async () => {
    try {
      const url = geminiBase || 'http://127.0.0.1:8765/v1beta'
      await navigator.clipboard.writeText(url)
      setCopiedGemini(true)
      setTimeout(() => setCopiedGemini(false), 1500)
    } catch {
      setFeedback({ text: 'Clipboard is not available', isError: true })
    }
  }

  const appRow = (app: AppKey, label: string, showDivider = true) => {
    const profile = integration.apps[app]
    const isCustom = profile.source === 'custom'
    return (
      <div style={{ ...wrappingTextStyle, display: 'flex', flexDirection: 'column', gap: '10px', paddingTop: showDivider ? '10px' : 0, borderTop: showDivider ? '1px solid var(--border)' : undefined }}>
        <div style={{ ...wrappingTextStyle, display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '14px', flexWrap: 'wrap' }}>
          <span style={{ ...wrappingTextStyle, fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>{label}</span>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
            <div style={segmentedStyle}>
              <button style={segmentStyle(!isCustom)} onClick={() => setApp(app, { source: 'native' })}>
                Native models
              </button>
              <button style={segmentStyle(isCustom)} onClick={() => setApp(app, { source: 'custom' })}>
                Custom models
              </button>
            </div>
            <button style={actionStyle} onClick={() => launch(app)} disabled={launching === app}>
              <Play size={13} />
              {launching === app ? 'Starting…' : 'Launch'}
            </button>
          </div>
        </div>
      </div>
    )
  }

  const isProxy = integration.mode === 'api_proxy'
  const isExternal = integration.proxy.scope === 'external'

  return (
    <div style={{ ...wrappingTextStyle, display: 'flex', flexDirection: 'column', gap: '16px' }}>
      {feedback && (
        <div
          style={{
            padding: '10px 14px',
            borderRadius: '8px',
            fontSize: '12px',
            border: '1px solid var(--border)',
            color: feedback.isError ? '#b91c1c' : 'var(--text)',
            backgroundColor: 'var(--canvas)',
          }}
        >
          {feedback.text}
        </div>
      )}

      <div style={cardStyle}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap' }}>
          <span style={sectionLabelStyle}>Integration mode</span>
          <button style={actionStyle} onClick={load} disabled={loading || saving}>
            <RefreshCw size={13} /> Reload
          </button>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: '12px' }}>
          <button
            onClick={() => save({ ...integration, mode: 'profile_isolation' })}
            style={{
              ...optionCardStyle,
              textAlign: 'left',
              padding: '14px',
              borderRadius: '10px',
              border: !isProxy ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: 'var(--canvas)',
              cursor: 'pointer',
            }}
          >
            <div style={{ ...wrappingTextStyle, fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>Profile Isolation: One Catalog per Window</div>
            <p style={{ ...wrappingTextStyle, margin: '4px 0 0', fontSize: '11.5px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
              Each app window runs on one catalog with either Native models or Custom models, never both. Launch separate windows to use each source.
            </p>
          </button>
          <button
            onClick={() => save({ ...integration, mode: 'api_proxy' })}
            style={{
              ...optionCardStyle,
              textAlign: 'left',
              padding: '14px',
              borderRadius: '10px',
              border: isProxy ? '2px solid var(--primary)' : '1px solid var(--border)',
              backgroundColor: 'var(--canvas)',
              cursor: 'pointer',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
              <span style={{ ...wrappingTextStyle, fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>API Proxy</span>
              <span style={riskStyle('orange')}>
                <AlertTriangle size={12} /> Potential Ban Risk
              </span>
            </div>
            <p style={{ ...wrappingTextStyle, margin: '4px 0 0', fontSize: '11.5px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
              Routes Antigravity traffic through the local proxy, so one catalog per window can serve both Native and Custom models at the same time. Proxied traffic carries a potential ban risk.
            </p>
          </button>
        </div>
      </div>

      {!isProxy && (
        <div style={cardStyle}>
          <div style={{ ...wrappingTextStyle, paddingBottom: '10px', borderBottom: '1px solid var(--border)' }}>
            <span style={{ ...sectionLabelStyle, textTransform: 'none' }}>Antigravity Apps</span>
          </div>
          {appRow('antigravity', 'Antigravity 2.0', false)}
          {appRow('vscode', 'VS Code')}
        </div>
      )}

      {isProxy && (
        <div style={cardStyle}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap' }}>
            <span style={sectionLabelStyle}>Proxy scope</span>
            <div style={segmentedStyle}>
              <button
                style={segmentStyle(!isExternal)}
                onClick={() => save({ ...integration, proxy: { ...integration.proxy, scope: 'antigravity' } })}
              >
                Antigravity Only
              </button>
              <button
                style={{
                  ...segmentStyle(isExternal),
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '6px',
                }}
                onClick={() => save({ ...integration, proxy: { ...integration.proxy, scope: 'external' } })}
              >
                <span>Global Use</span>
                <span
                  title="Increased Ban Risk"
                  aria-label="Increased Ban Risk"
                  style={{ display: 'inline-flex', alignItems: 'center' }}
                >
                  <AlertCircle
                    size={12}
                    color="#b91c1c"
                    className="text-red-600"
                    style={{ flexShrink: 0 }}
                  />
                </span>
              </button>
            </div>
          </div>

          {isExternal && (
            <>
              <div style={{ ...wrappingTextStyle, display: 'flex', flexDirection: 'column', gap: '8px', paddingTop: '8px', borderTop: '1px solid var(--border)' }}>
                <span style={sectionLabelStyle}>OpenAI protocol</span>
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
                  <code style={{ ...wrappingTextStyle, fontSize: '12px', color: 'var(--text)' }}>
                    {endpoint || 'http://127.0.0.1:8765/v1'}
                  </code>
                  <button
                    type="button"
                    style={{
                      border: 'none',
                      background: 'transparent',
                      padding: '2px',
                      display: 'inline-flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      color: 'var(--text-muted)',
                      cursor: 'pointer',
                    }}
                    onClick={copyEndpoint}
                    title={copied ? 'Copied' : 'Copy'}
                    aria-label="Copy"
                  >
                    {copied ? <Check size={13} color="var(--green)" /> : <Copy size={13} />}
                  </button>
                </div>
              </div>

              <div style={{ ...wrappingTextStyle, display: 'flex', flexDirection: 'column', gap: '8px', paddingTop: '8px', borderTop: '1px solid var(--border)' }}>
                <span style={sectionLabelStyle}>Gemini protocol</span>
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
                  <code style={{ ...wrappingTextStyle, fontSize: '12px', color: 'var(--text)' }}>
                    {geminiBase || 'http://127.0.0.1:8765/v1beta'}
                  </code>
                  <button
                    type="button"
                    style={{
                      border: 'none',
                      background: 'transparent',
                      padding: '2px',
                      display: 'inline-flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      color: 'var(--text-muted)',
                      cursor: 'pointer',
                    }}
                    onClick={copyGemini}
                    title={copiedGemini ? 'Copied' : 'Copy'}
                    aria-label="Copy"
                  >
                    {copiedGemini ? <Check size={13} color="var(--green)" /> : <Copy size={13} />}
                  </button>
                </div>
              </div>

              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '14px', flexWrap: 'wrap', paddingTop: '8px', borderTop: '1px solid var(--border)' }}>
                <span style={{ ...wrappingTextStyle, fontSize: '12px', color: 'var(--text)' }}>Private Key</span>
                <input
                  type="password"
                  value={integration.proxy.pin || ''}
                  onChange={(e) => setIntegration({ ...integration, proxy: { ...integration.proxy, pin: e.target.value } })}
                  onBlur={() => save(integration)}
                  placeholder="Required"
                  style={{ flex: '1 1 240px', minWidth: '200px', padding: '7px 10px', borderRadius: '8px', border: '1px solid var(--border)', fontSize: '12px' }}
                />
              </div>
            </>
          )}
        </div>
      )}

      <div
        role="note"
        style={{ ...cardStyle, padding: '14px 16px', gap: '8px' }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '10px', flexWrap: 'wrap' }}>
          <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>AGY CLI</span>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span
              style={{
                fontSize: '11px',
                fontWeight: 600,
                padding: '2px 8px',
                borderRadius: '4px',
                backgroundColor: cliStatus?.configured ? 'var(--green-bg)' : 'var(--tonal)',
                color: cliStatus?.configured ? 'var(--green)' : 'var(--text-muted)',
                whiteSpace: 'nowrap',
              }}
            >
              {cliStatus?.configured ? 'Configured' : (cliStatus?.installed ? 'Not Configured' : 'Not Detected')}
            </span>
            {!cliStatus?.configured && (
              <button
                type="button"
                onClick={handleSetupCli}
                disabled={configuringCli}
                style={{
                  ...actionStyle,
                  fontSize: '11px',
                  padding: '4px 10px',
                  borderRadius: '6px',
                }}
              >
                {configuringCli ? 'Configuring…' : 'Auto-configure CLI'}
              </button>
            )}
          </div>
        </div>
        <span style={{ ...wrappingTextStyle, fontSize: '11.5px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
          Runs independently and can use Native and Custom models at the same time in one session, without API Proxy mode or window profile isolation.
        </span>
      </div>
    </div>
  )
}
