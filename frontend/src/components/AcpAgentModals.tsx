import React, { useState } from 'react'
import { Plus, Save, Search, Trash2, X } from 'lucide-react'
import type {
  AcpAgentInput,
  AcpAgentInstance,
  AcpDetectCandidate,
  AcpDetectResponse,
} from '../types'

const OVERLAY_STYLE: React.CSSProperties = {
  position: 'fixed',
  inset: 0,
  backgroundColor: 'rgba(32, 33, 36, 0.45)',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  padding: '24px',
  zIndex: 1200,
}

const DIALOG_STYLE: React.CSSProperties = {
  backgroundColor: 'var(--surface)',
  borderRadius: '12px',
  border: '1px solid var(--border)',
  boxShadow: '0 12px 32px rgba(60, 64, 67, 0.24)',
  width: '100%',
  maxWidth: '520px',
  maxHeight: '90vh',
  overflowY: 'auto',
  padding: '20px',
  display: 'flex',
  flexDirection: 'column',
  gap: '14px',
}

const LABEL_STYLE: React.CSSProperties = {
  display: 'block',
  fontSize: '12px',
  fontWeight: 600,
  color: 'var(--text-muted)',
  marginBottom: '4px',
}

const INPUT_STYLE: React.CSSProperties = {
  width: '100%',
  padding: '7px 11px',
  fontSize: '13px',
  borderRadius: '6px',
  border: '1px solid var(--border)',
  color: 'var(--text)',
  backgroundColor: 'var(--surface)',
  boxSizing: 'border-box',
}

const ERROR_STYLE: React.CSSProperties = {
  backgroundColor: 'var(--red-bg)',
  color: 'var(--red)',
  border: '1px solid var(--border-danger, #f5c6c2)',
  borderRadius: '6px',
  padding: '8px 12px',
  fontSize: '12px',
}

const EMPTY_INPUT: AcpAgentInput = {
  name: '',
  type: '',
  binary_path: '',
  endpoint: '',
  acp_version: '',
  tools: [],
}

export interface AcpAgentEditorModalProps {
  open: boolean
  mode: 'create' | 'edit'
  initial?: AcpAgentInstance | null
  busy?: boolean
  error?: string | null
  onCancel: () => void
  onSubmit: (input: AcpAgentInput) => void
}

/**
 * Add / edit dialog for one ACP agent. Every preset field is editable: the
 * endpoint, binary path, version and negotiated tool list are user-owned, since
 * the registry only carries observational defaults.
 */
export const AcpAgentEditorModal: React.FC<AcpAgentEditorModalProps> = ({
  open,
  mode,
  initial,
  busy,
  error,
  onCancel,
  onSubmit,
}) => {
  const [form, setForm] = useState<AcpAgentInput>(() =>
    mode === 'edit' && initial
      ? {
          id: initial.id,
          name: initial.name,
          type: initial.type,
          binary_path: initial.binary_path,
          endpoint: initial.port_socket,
          acp_version: initial.acp_version,
          tools: initial.supported_tools,
        }
      : EMPTY_INPUT
  )
  const [toolsText, setToolsText] = useState(() =>
    mode === 'edit' && initial ? initial.supported_tools.join(', ') : ''
  )

  if (!open) return null

  const update = (key: keyof AcpAgentInput, value: string) => {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault()
    const tools = toolsText
      .split(',')
      .map((t) => t.trim())
      .filter((t) => t.length > 0)
    onSubmit({
      ...form,
      name: form.name.trim(),
      endpoint: form.endpoint.trim(),
      tools,
    })
  }

  return (
    <div style={OVERLAY_STYLE} role="dialog" aria-modal="true" aria-label={mode === 'create' ? 'Add ACP Agent' : 'Edit ACP Agent'}>
      <form style={DIALOG_STYLE} onSubmit={handleSubmit}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <div>
            <h3 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
              {mode === 'create' ? 'Add ACP Agent' : `Edit ${initial?.name || 'Agent'}`}
            </h3>
            <p style={{ margin: '2px 0 0', fontSize: '12px', color: 'var(--text-muted)' }}>
              Endpoints and binary paths are user-configured; nothing is assumed to be built in.
            </p>
          </div>
          <button
            type="button"
            onClick={onCancel}
            aria-label="Close dialog"
            style={{ border: 'none', background: 'transparent', cursor: 'pointer', padding: '2px', color: 'var(--text-muted)' }}
          >
            <X size={18} />
          </button>
        </div>

        {error && <div style={ERROR_STYLE}>{error}</div>}

        <div>
          <label style={LABEL_STYLE} htmlFor="acp-agent-name">
            Name
          </label>
          <input
            id="acp-agent-name"
            style={INPUT_STYLE}
            value={form.name}
            onChange={(e) => update('name', e.target.value)}
            placeholder="My Agent"
            required
          />
        </div>

        <div>
          <label style={LABEL_STYLE} htmlFor="acp-agent-type">
            Type
          </label>
          <input
            id="acp-agent-type"
            style={INPUT_STYLE}
            value={form.type || ''}
            onChange={(e) => update('type', e.target.value)}
            placeholder="Terminal Agent Daemon"
          />
        </div>

        <div>
          <label style={LABEL_STYLE} htmlFor="acp-agent-endpoint">
            Endpoint
          </label>
          <input
            id="acp-agent-endpoint"
            style={{ ...INPUT_STYLE, fontFamily: 'monospace' }}
            value={form.endpoint}
            onChange={(e) => update('endpoint', e.target.value)}
            placeholder="127.0.0.1:4096, unix:///path/to.sock, \\.\pipe\name"
            required
          />
          <p style={{ margin: '4px 0 0', fontSize: '11px', color: 'var(--text-muted)' }}>
            Accepted forms: host:port, unix://&lt;socket path&gt;, \\.\pipe\&lt;name&gt; or npipe://&lt;name&gt;.
          </p>
        </div>

        <div>
          <label style={LABEL_STYLE} htmlFor="acp-agent-binary">
            Binary Path
          </label>
          <input
            id="acp-agent-binary"
            style={{ ...INPUT_STYLE, fontFamily: 'monospace' }}
            value={form.binary_path || ''}
            onChange={(e) => update('binary_path', e.target.value)}
            placeholder="/usr/local/bin/my-agent"
          />
        </div>

        <div>
          <label style={LABEL_STYLE} htmlFor="acp-agent-version">
            ACP Version
          </label>
          <input
            id="acp-agent-version"
            style={INPUT_STYLE}
            value={form.acp_version || ''}
            onChange={(e) => update('acp_version', e.target.value)}
            placeholder="v1.2.0"
          />
        </div>

        <div>
          <label style={LABEL_STYLE} htmlFor="acp-agent-tools">
            Tools (comma separated)
          </label>
          <input
            id="acp-agent-tools"
            style={INPUT_STYLE}
            value={toolsText}
            onChange={(e) => setToolsText(e.target.value)}
            placeholder="read_file, write_file, terminal"
          />
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px', marginTop: '4px' }}>
          <button type="button" className="btn-pill-outlined" onClick={onCancel} disabled={busy}>
            Cancel
          </button>
          <button type="submit" className="btn-pill-primary" disabled={busy}>
            {mode === 'create' ? <Plus size={14} /> : <Save size={14} />}
            <span>{busy ? 'Saving...' : mode === 'create' ? 'Add Agent' : 'Save Changes'}</span>
          </button>
        </div>
      </form>
    </div>
  )
}

export interface AcpDetectModalProps {
  agent: AcpAgentInstance | null
  result: AcpDetectResponse | null
  loading: boolean
  error?: string | null
  onCancel: () => void
  onApply: (candidate: AcpDetectCandidate) => void
}

/**
 * Auto-detection dialog. Lists only endpoints the daemon actually observed for
 * the selected agent (configured endpoint, OS default, listening ports of the
 * running process, existing socket files) and lets the user save one.
 */
export const AcpDetectModal: React.FC<AcpDetectModalProps> = ({
  agent,
  result,
  loading,
  error,
  onCancel,
  onApply,
}) => {
  if (!agent) return null

  return (
    <div style={OVERLAY_STYLE} role="dialog" aria-modal="true" aria-label="Detect ACP endpoint">
      <div style={DIALOG_STYLE}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <div>
            <h3 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
              Auto-detect endpoint for {agent.name}
            </h3>
            <p style={{ margin: '2px 0 0', fontSize: '12px', color: 'var(--text-muted)' }}>
              {result && result.pid > 0
                ? `Running process detected (PID ${result.pid}).`
                : 'No matching process detected; only configured and default endpoints are listed.'}
            </p>
          </div>
          <button
            type="button"
            onClick={onCancel}
            aria-label="Close dialog"
            style={{ border: 'none', background: 'transparent', cursor: 'pointer', padding: '2px', color: 'var(--text-muted)' }}
          >
            <X size={18} />
          </button>
        </div>

        {error && <div style={ERROR_STYLE}>{error}</div>}

        {loading ? (
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '13px', color: 'var(--text-muted)' }}>
            <Search size={14} className="animate-spin" />
            <span>Probing local endpoints...</span>
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {(result?.candidates || []).map((candidate) => (
              <div
                key={candidate.endpoint}
                style={{
                  border: '1px solid var(--border)',
                  borderRadius: '8px',
                  padding: '10px 12px',
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  gap: '12px',
                }}
              >
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontFamily: 'monospace', fontSize: '12px', color: 'var(--text)', wordBreak: 'break-all' }}>
                    {candidate.endpoint}
                  </div>
                  <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                    {candidate.kind} · {candidate.note}
                  </div>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
                  <span
                    style={{
                      fontSize: '10px',
                      fontWeight: 700,
                      textTransform: 'uppercase',
                      padding: '2px 6px',
                      borderRadius: '4px',
                      backgroundColor: candidate.reachable ? '#e6f4ea' : '#f1f3f4',
                      color: candidate.reachable ? '#137333' : '#5f6368',
                    }}
                  >
                    {candidate.reachable ? 'Reachable' : 'No answer'}
                  </span>
                  <button
                    type="button"
                    className="btn-pill-tonal"
                    onClick={() => onApply(candidate)}
                    style={{ padding: '4px 10px', fontSize: '11px' }}
                  >
                    Use
                  </button>
                </div>
              </div>
            ))}
            {result && result.candidates.length === 0 && (
              <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                Nothing observed. Start the agent, or set the endpoint manually.
              </div>
            )}
          </div>
        )}

        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '4px' }}>
          <button type="button" className="btn-pill-outlined" onClick={onCancel}>
            Close
          </button>
        </div>
      </div>
    </div>
  )
}

/** Small confirm dialog used for delete and bulk setup actions. */
export interface AcpConfirmModalProps {
  open: boolean
  title: string
  message: string
  confirmLabel: string
  busy?: boolean
  danger?: boolean
  onCancel: () => void
  onConfirm: () => void
}

export const AcpConfirmModal: React.FC<AcpConfirmModalProps> = ({
  open,
  title,
  message,
  confirmLabel,
  busy,
  danger,
  onCancel,
  onConfirm,
}) => {
  if (!open) return null
  return (
    <div style={OVERLAY_STYLE} role="dialog" aria-modal="true" aria-label={title}>
      <div style={{ ...DIALOG_STYLE, maxWidth: '420px' }}>
        <h3 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>{title}</h3>
        <p style={{ margin: 0, fontSize: '13px', color: 'var(--text-muted)', lineHeight: 1.5 }}>{message}</p>
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
          <button type="button" className="btn-pill-outlined" onClick={onCancel} disabled={busy}>
            Cancel
          </button>
          <button
            type="button"
            className={danger ? 'btn-pill-danger' : 'btn-pill-primary'}
            onClick={onConfirm}
            disabled={busy}
          >
            {danger ? <Trash2 size={14} /> : <Search size={14} />}
            <span>{busy ? 'Working...' : confirmLabel}</span>
          </button>
        </div>
      </div>
    </div>
  )
}
