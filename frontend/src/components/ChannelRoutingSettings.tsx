import React from 'react'
import { FolderOpen } from 'lucide-react'

/// Which Antigravity surface answers an inbound channel message, and which project
/// it lands in. Both are stored on the channels config, so all three channels share
/// one context instead of three unrelated ones.
export interface ChannelRouting {
  agent_backend?: string
  default_project?: string
}

const BACKENDS: { key: string; label: string; hint: string }[] = [
  { key: '', label: 'Bridge default', hint: 'Use the agent chosen for the Gemini bridge' },
  { key: 'cli', label: 'Antigravity CLI', hint: 'Short-lived process, own conversation store' },
  { key: 'app', label: 'Antigravity 2.0', hint: 'Shares context with the desktop app and VS Code' },
  { key: 'acp', label: 'Antigravity ACP', hint: 'Separate conversation store' },
]

interface Props {
  config: ChannelRouting
  onChange: (next: ChannelRouting) => void
  disabled?: boolean
}

export const ChannelRoutingSettings: React.FC<Props> = ({ config, onChange, disabled }) => {
  const backend = config.agent_backend ?? ''
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
      <div>
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
          Answering agent
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(190px, 1fr))', gap: '10px', marginTop: '8px' }}>
          {BACKENDS.map((option) => {
            const active = backend === option.key
            return (
              <button
                key={option.key || 'inherit'}
                type="button"
                disabled={disabled}
                onClick={() => onChange({ ...config, agent_backend: option.key })}
                style={{
                  textAlign: 'left',
                  padding: '10px 12px',
                  borderRadius: '8px',
                  border: active ? '2px solid var(--primary)' : '1px solid var(--border)',
                  backgroundColor: 'var(--canvas)',
                  cursor: disabled ? 'progress' : 'pointer',
                }}
              >
                <div style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)', whiteSpace: 'nowrap' }}>{option.label}</div>
                <div style={{ fontSize: '11.5px', color: 'var(--text-muted)', marginTop: '2px' }}>{option.hint}</div>
              </button>
            )
          })}
        </div>
      </div>

      <div>
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
          Default project
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginTop: '6px' }}>
          <FolderOpen size={14} style={{ color: 'var(--text-muted)' }} />
          <input
            type="text"
            value={config.default_project ?? ''}
            disabled={disabled}
            placeholder="/absolute/path/to/project"
            onChange={(event) => onChange({ ...config, default_project: event.target.value })}
            style={{ flex: 1, padding: '7px 10px', borderRadius: '8px', border: '1px solid var(--border)', fontSize: '12px' }}
          />
        </div>
        <div style={{ fontSize: '11.5px', color: 'var(--text-muted)', marginTop: '4px' }}>
          Every inbound message runs in this workspace, so the three channels see one context.
        </div>
      </div>
    </div>
  )
}
