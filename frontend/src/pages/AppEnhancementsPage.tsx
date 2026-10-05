import React, { useEffect, useState } from 'react'
import { Zap, Save } from 'lucide-react'
import { ToggleSwitch } from '../components/ToggleSwitch'
import { api } from '../api'
import type { EnhancementsConfig, GUIConfig } from '../types'

const PRESET_COLORS = [
  { name: 'Google Blue', hex: '#0b57d0' },
  { name: 'Vibrant Purple', hex: '#7c3aed' },
  { name: 'Emerald Green', hex: '#059669' },
  { name: 'Warm Amber', hex: '#d97706' },
  { name: 'Coral Red', hex: '#dc2626' },
]

// Generate 10x10 color palette grid
const HUES = [210, 260, 280, 330, 0, 25, 45, 142, 170, 195]
const LIGHTNESSES = [92, 84, 76, 68, 60, 52, 44, 36, 28, 20]
const GRID_COLORS: string[][] = []
for (let r = 0; r < 10; r++) {
  const row: string[] = []
  const l = LIGHTNESSES[r]
  for (let c = 0; c < 10; c++) {
    row.push(`hsl(${HUES[c]}, 82%, ${l}%)`)
  }
  GRID_COLORS.push(row)
}

function hslToHex(hsl: string): string {
  const m = hsl.match(/hsl\((\d+),\s*(\d+)%,\s*(\d+)%\)/)
  if (!m) return '#0b57d0'
  const h = parseInt(m[1]) / 360
  const s = parseInt(m[2]) / 100
  const l = parseInt(m[3]) / 100

  let r: number, g: number, b: number
  if (s === 0) {
    r = g = b = l
  } else {
    const hue2rgb = (p: number, q: number, t: number) => {
      if (t < 0) t += 1
      if (t > 1) t -= 1
      if (t < 1 / 6) return p + (q - p) * 6 * t
      if (t < 1 / 2) return q
      if (t < 2 / 3) return p + (q - p) * (2 / 3 - t) * 6
      return p
    }
    const q = l < 0.5 ? l * (1 + s) : l + s - l * s
    const p = 2 * l - q
    r = hue2rgb(p, q, h + 1 / 3)
    g = hue2rgb(p, q, h)
    b = hue2rgb(p, q, h - 1 / 3)
  }
  const toHex = (x: number) => {
    const hex = Math.round(x * 255).toString(16)
    return hex.length === 1 ? '0' + hex : hex
  }
  return `#${toHex(r)}${toHex(g)}${toHex(b)}`
}

export const AppEnhancementsPage: React.FC = () => {
  const [config, setConfig] = useState<EnhancementsConfig | null>(null)
  const [guiConfig, setGuiConfig] = useState<GUIConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [applying, setApplying] = useState(false)
  const [statusMsg, setStatusMsg] = useState<{ text: string; type: 'success' | 'error' } | null>(null)
  const [previewHover, setPreviewHover] = useState<number | null>(null)
  const [projects, setProjects] = useState<string[]>([])

  useEffect(() => {
    loadConfig()
  }, [])

  const loadConfig = async () => {
    try {
      setLoading(true)
      const [data, projList, gData] = await Promise.all([
        api.getEnhancements(),
        api.getGUIProjects().catch(() => []),
        api.getGUIConfig().catch(() => null),
      ])
      setConfig(data)
      setProjects((projList || []).map((p) => p.name))
      if (gData) setGuiConfig(gData)
    } catch (err: any) {
      setStatusMsg({ text: 'Failed to load enhancements config: ' + err.message, type: 'error' })
    } finally {
      setLoading(false)
    }
  }

  const handleSave = async () => {
    try {
      setSaving(true)
      const promises: Promise<any>[] = []
      if (config) promises.push(api.updateEnhancements(config))
      if (guiConfig) promises.push(api.updateGUIConfig(guiConfig))
      await Promise.all(promises)
      setStatusMsg({ text: 'Settings saved successfully', type: 'success' })
      setTimeout(() => setStatusMsg(null), 3500)
    } catch (err: any) {
      setStatusMsg({ text: 'Failed to save settings: ' + err.message, type: 'error' })
    } finally {
      setSaving(false)
    }
  }

  const handleApplyLive = async () => {
    try {
      setApplying(true)
      const promises: Promise<any>[] = []
      if (config) promises.push(api.updateEnhancements(config))
      if (guiConfig) promises.push(api.updateGUIConfig(guiConfig))
      await Promise.all(promises)
      const res = await api.applyGUI().catch(() => api.applyEnhancements())
      setStatusMsg({
        text: res.message || 'Successfully injected and applied enhancements live to Antigravity!',
        type: 'success',
      })
      setTimeout(() => setStatusMsg(null), 4000)
    } catch (err: any) {
      setStatusMsg({ text: 'Live injection error: ' + err.message, type: 'error' })
    } finally {
      setApplying(false)
    }
  }

  if (loading || !config) {
    return (
      <div style={{ padding: '32px', textAlign: 'center', color: '#64748b' }}>
        <div style={{ fontSize: '14px', fontWeight: 500 }}>Loading App Enhancements...</div>
      </div>
    )
  }

  const jb = config.prompt_jump_bar
  const activeColor =
    jb.color_mode === 'default'
      ? '#64748b'
      : jb.color_mode === 'project'
      ? '#059669' // Sample project emerald green for preview
      : jb.custom_color || '#0b57d0'

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* 1. Header Information & Actions Card */}
      <div className="google-card" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
            App Enhancements & Usability
          </div>
          <div style={{ fontSize: '13px', color: 'var(--text)', marginTop: '4px' }}>
            Usability add-ons, prompt jump navigation, tool visual density, and project color tab customization for Antigravity 2.0.
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <button
            onClick={handleApplyLive}
            disabled={applying}
            className="btn-pill-tonal"
            style={{ padding: '7px 16px', fontSize: '12px' }}
          >
            <Zap size={14} className={applying ? 'spin' : ''} />
            <span>{applying ? 'Injecting...' : 'Apply Live in Antigravity'}</span>
          </button>
          <button
            onClick={handleSave}
            disabled={saving}
            className="btn-pill-primary"
            style={{ padding: '7px 18px', fontSize: '12px' }}
          >
            <Save size={14} />
            <span>{saving ? 'Saving...' : 'Save Settings'}</span>
          </button>
        </div>
      </div>

      {statusMsg && (
        <div
          style={{
            padding: '10px 16px',
            borderRadius: '8px',
            fontSize: '13px',
            fontWeight: 500,
            background: statusMsg.type === 'success' ? 'var(--green-bg)' : '#fce8e6',
            color: statusMsg.type === 'success' ? 'var(--green)' : '#b3261e',
            border: `1px solid ${statusMsg.type === 'success' ? '#bbf7d0' : '#fecaca'}`,
          }}
        >
          {statusMsg.text}
        </div>
      )}

      {/* Feature 1: Quick Prompt Jump Bar */}
      <div className="google-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: '#1e293b' }}>
                Quick Prompt Jump Bar (Devin Style)
              </h2>
              <span
                style={{
                  fontSize: '11px',
                  fontWeight: 600,
                  padding: '2px 8px',
                  borderRadius: '12px',
                  background: jb.enabled ? 'var(--green-bg)' : '#f1f5f9',
                  color: jb.enabled ? 'var(--green)' : '#64748b',
                }}
              >
                {jb.enabled ? 'Enabled' : 'Disabled'}
              </span>
            </div>
            <p style={{ margin: '4px 0 0', fontSize: '13px', color: '#64748b' }}>
              Horizontal dash lines in conversation margin allowing instant jump to any user prompt turn.
              Dynamically highlights the lowest (latest) prompt currently on screen as you scroll.
            </p>
          </div>

          <ToggleSwitch
            checked={jb.enabled}
            onChange={(checked) =>
              setConfig({
                ...config,
                prompt_jump_bar: { ...jb, enabled: checked },
              })
            }
          />
        </div>

        {jb.enabled && (
          <div style={{ borderTop: '1px solid #f1f5f9', paddingTop: '18px' }}>
            {/* Options grid */}
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))',
                gap: '16px',
                marginBottom: '20px',
              }}
            >
              <label
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '10px',
                  fontSize: '13px',
                  color: '#334155',
                  cursor: 'pointer',
                }}
              >
                <ToggleSwitch
                  size="sm"
                  checked={jb.sync_scroll}
                  onChange={(checked) =>
                    setConfig({
                      ...config,
                      prompt_jump_bar: { ...jb, sync_scroll: checked },
                    })
                  }
                />
                <span>Sync with Scroll (highlights lowest prompt on screen)</span>
              </label>

              <label
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '10px',
                  fontSize: '13px',
                  color: '#334155',
                  cursor: 'pointer',
                }}
              >
                <ToggleSwitch
                  size="sm"
                  checked={jb.show_tooltip}
                  onChange={(checked) =>
                    setConfig({
                      ...config,
                      prompt_jump_bar: { ...jb, show_tooltip: checked },
                    })
                  }
                />
                <span>Show Preview Tooltip on Hover</span>
              </label>

              <label
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '10px',
                  fontSize: '13px',
                  color: '#334155',
                  cursor: 'pointer',
                }}
              >
                <ToggleSwitch
                  size="sm"
                  checked={jb.focus_pulse}
                  onChange={(checked) =>
                    setConfig({
                      ...config,
                      prompt_jump_bar: { ...jb, focus_pulse: checked },
                    })
                  }
                />
                <span>Pulse Highlight Target Prompt Card on Jump</span>
              </label>

              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                <span style={{ fontSize: '13px', color: '#334155' }}>Line Width:</span>
                <input
                  type="number"
                  min="10"
                  max="28"
                  value={jb.dash_width || 14}
                  onChange={(e) =>
                    setConfig({
                      ...config,
                      prompt_jump_bar: { ...jb, dash_width: parseInt(e.target.value) || 14 },
                    })
                  }
                  style={{
                    width: '60px',
                    padding: '4px 8px',
                    borderRadius: '6px',
                    border: '1px solid #cbd5e1',
                    fontSize: '12px',
                  }}
                />
                <span style={{ fontSize: '12px', color: '#94a3b8' }}>px (default 14px)</span>
              </div>
            </div>

            {/* Color Mode Selection (User Request) */}
            <div
              style={{
                background: '#f8fafc',
                borderRadius: '10px',
                border: '1px solid #e2e8f0',
                padding: '16px 20px',
                marginBottom: '20px',
              }}
            >
              <div style={{ fontSize: '13px', fontWeight: 700, color: '#1e293b', marginBottom: '10px' }}>
                Active & Hover Line Color
              </div>
              <p style={{ margin: '0 0 12px', fontSize: '12px', color: '#64748b' }}>
                Inactive lines remain subtle grey (1.5px). Choose the accent color for the active indicator (3.5px) and
                hovering state:
              </p>

              <div style={{ display: 'flex', gap: '20px', flexWrap: 'wrap', marginBottom: '14px' }}>
                <label
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                    fontSize: '13px',
                    color: '#1e293b',
                    cursor: 'pointer',
                  }}
                >
                  <input
                    type="radio"
                    name="color_mode"
                    value="default"
                    checked={jb.color_mode === 'default'}
                    onChange={() =>
                      setConfig({
                        ...config,
                        prompt_jump_bar: { ...jb, color_mode: 'default' },
                      })
                    }
                    style={{ accentColor: '#0b57d0' }}
                  />
                  <span>Default (Slate Grey)</span>
                </label>

                <label
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                    fontSize: '13px',
                    color: '#1e293b',
                    cursor: 'pointer',
                  }}
                >
                  <input
                    type="radio"
                    name="color_mode"
                    value="project"
                    checked={jb.color_mode === 'project'}
                    onChange={() =>
                      setConfig({
                        ...config,
                        prompt_jump_bar: { ...jb, color_mode: 'project' },
                      })
                    }
                    style={{ accentColor: '#0b57d0' }}
                  />
                  <span>Match Project Color (dynamically adapts per project)</span>
                </label>

                <label
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                    fontSize: '13px',
                    color: '#1e293b',
                    cursor: 'pointer',
                  }}
                >
                  <input
                    type="radio"
                    name="color_mode"
                    value="custom"
                    checked={jb.color_mode === 'custom'}
                    onChange={() =>
                      setConfig({
                        ...config,
                        prompt_jump_bar: { ...jb, color_mode: 'custom' },
                      })
                    }
                    style={{ accentColor: '#0b57d0' }}
                  />
                  <span>Custom Color (applied across all projects)</span>
                </label>
              </div>

              {/* Custom Color Palette (Preset + 10x10 Grid) */}
              {jb.color_mode === 'custom' && (
                <div
                  style={{
                    background: '#ffffff',
                    padding: '14px',
                    borderRadius: '8px',
                    border: '1px solid #cbd5e1',
                    marginTop: '10px',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '12px' }}>
                    <span style={{ fontSize: '11px', fontWeight: 700, color: '#64748b', textTransform: 'uppercase' }}>
                      Preset Colors:
                    </span>
                    <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                      {PRESET_COLORS.map((p) => (
                        <div
                          key={p.hex}
                          onClick={() =>
                            setConfig({
                              ...config,
                              prompt_jump_bar: { ...jb, custom_color: p.hex },
                            })
                          }
                          title={p.name}
                          style={{
                            width: '24px',
                            height: '24px',
                            borderRadius: '50%',
                            background: p.hex,
                            cursor: 'pointer',
                            border: jb.custom_color === p.hex ? '2px solid #0f172a' : '2px solid transparent',
                            transform: jb.custom_color === p.hex ? 'scale(1.15)' : 'scale(1)',
                            transition: 'transform 0.12s',
                          }}
                        />
                      ))}
                    </div>

                    <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <span style={{ fontSize: '12px', color: '#64748b' }}>Selected:</span>
                      <div
                        style={{
                          width: '18px',
                          height: '18px',
                          borderRadius: '4px',
                          background: jb.custom_color || '#0b57d0',
                          border: '1px solid #cbd5e1',
                        }}
                      />
                      <input
                        type="text"
                        value={jb.custom_color || '#0b57d0'}
                        onChange={(e) =>
                          setConfig({
                            ...config,
                            prompt_jump_bar: { ...jb, custom_color: e.target.value },
                          })
                        }
                        style={{
                          width: '85px',
                          padding: '3px 6px',
                          borderRadius: '4px',
                          border: '1px solid #cbd5e1',
                          fontSize: '12px',
                          fontFamily: 'monospace',
                        }}
                      />
                    </div>
                  </div>

                  {/* 10x10 Palette */}
                  <div>
                    <span
                      style={{
                        display: 'block',
                        fontSize: '11px',
                        fontWeight: 700,
                        color: '#64748b',
                        textTransform: 'uppercase',
                        marginBottom: '6px',
                      }}
                    >
                      10&times;10 Extended Palette:
                    </span>
                    <div
                      style={{
                        display: 'grid',
                        gridTemplateColumns: 'repeat(10, 20px)',
                        gap: '4px',
                        padding: '6px',
                        background: '#f8fafc',
                        borderRadius: '6px',
                        border: '1px solid #e2e8f0',
                        width: 'fit-content',
                      }}
                    >
                      {GRID_COLORS.map((row, r) =>
                        row.map((cellHsl, c) => {
                          const hex = hslToHex(cellHsl)
                          const isSelected = jb.custom_color?.toLowerCase() === hex.toLowerCase()
                          return (
                            <div
                              key={`${r}-${c}`}
                              onClick={() =>
                                setConfig({
                                  ...config,
                                  prompt_jump_bar: { ...jb, custom_color: hex },
                                })
                              }
                              style={{
                                width: '20px',
                                height: '20px',
                                borderRadius: '3px',
                                background: cellHsl,
                                cursor: 'pointer',
                                border: isSelected ? '2px solid #0f172a' : '1px solid rgba(0,0,0,0.06)',
                                transform: isSelected ? 'scale(1.2)' : 'scale(1)',
                                zIndex: isSelected ? 2 : 1,
                                transition: 'transform 0.1s',
                              }}
                            />
                          )
                        })
                      )}
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* Interactive Live Preview Component */}
            <div
              style={{
                background: '#f8fafc',
                borderRadius: '8px',
                border: '1px dashed #cbd5e1',
                padding: '16px',
                display: 'flex',
                alignItems: 'center',
                gap: '24px',
              }}
            >
              <div style={{ fontSize: '12px', fontWeight: 600, color: '#475569' }}>Interactive Gutter Preview:</div>

              {/* Sample Prompt Jump Bar */}
              <div
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '6px',
                  padding: '6px 4px',
                  background: 'transparent',
                  userSelect: 'none',
                }}
              >
                {[0, 1, 2, 3, 4].map((idx) => {
                  const isActive = idx === 2
                  const isHovered = previewHover === idx
                  const h = isActive ? '3.5px' : '1.5px'
                  const bg = isActive || isHovered ? activeColor : 'rgba(100, 116, 139, 0.42)'
                  return (
                    <div
                      key={idx}
                      onMouseEnter={() => setPreviewHover(idx)}
                      onMouseLeave={() => setPreviewHover(null)}
                      title={`Preview Prompt #${idx + 1}`}
                      style={{
                        width: `${jb.dash_width || 14}px`,
                        height: h,
                        borderRadius: '2px',
                        background: bg,
                        cursor: 'pointer',
                        transition: 'height 0.15s, background 0.15s',
                      }}
                    />
                  )
                })}
              </div>

              <div style={{ fontSize: '12px', color: '#64748b', lineHeight: 1.4 }}>
                Line #3 is active (<strong>{jb.dash_width || 14}px &times; 3.5px</strong> in{' '}
                <span style={{ color: activeColor, fontWeight: 700 }}>{activeColor}</span>). Hover over lines to test
                hover accent. All inactive lines stay thin (<strong>1.5px</strong>) in grey.
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Feature 2: Thinking Process & Tool Calls Color Density */}
      <div className="google-card">
        <div style={{ marginBottom: '16px' }}>
          <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
            Thinking & Tool Execution Visual Density
          </h2>
          <p style={{ margin: '4px 0 0', fontSize: '13px', color: 'var(--text-muted)' }}>
            Decrease visual dominance of intermediate tool steps, command runs, and thinking blocks so the final
            answer clearly stands out.
          </p>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '14px' }}>
          {[
            {
              id: 'muted',
              title: 'Greyed Out / Muted (Recommended)',
              desc: 'Dims intermediate tool steps and thoughts with subtle grayscale & 48% opacity. Hovering reveals full content.',
            },
            {
              id: 'hidden',
              title: 'Hide Completely',
              desc: 'Completely hides intermediate thinking rows and command runs for clean, distraction-free reading of answers.',
            },
            {
              id: 'normal',
              title: 'Standard Density',
              desc: 'Preserves default Antigravity high-contrast tool rows and collapsible thinking blocks.',
            },
          ].map((opt) => (
            <div
              key={opt.id}
              onClick={() =>
                setConfig({
                  ...config,
                  tool_density_mode: opt.id as any,
                })
              }
              style={{
                padding: '14px 16px',
                borderRadius: '8px',
                border: `1.5px solid ${config.tool_density_mode === opt.id ? '#0b57d0' : '#e2e8f0'}`,
                background: config.tool_density_mode === opt.id ? '#f0f7ff' : '#ffffff',
                cursor: 'pointer',
                transition: 'border-color 0.15s, background 0.15s',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px' }}>
                <input
                  type="radio"
                  name="tool_density"
                  checked={config.tool_density_mode === opt.id}
                  onChange={() => {}}
                  style={{ accentColor: '#0b57d0' }}
                />
                <span style={{ fontSize: '13px', fontWeight: 700, color: '#1e293b' }}>{opt.title}</span>
              </div>
              <p style={{ margin: 0, fontSize: '12px', color: '#64748b', lineHeight: 1.4 }}>{opt.desc}</p>
            </div>
          ))}
        </div>
      </div>

      {/* Feature 3: Conversation Turn Breaker Line */}
      <div className="google-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div>
            <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
              Conversation Turn Breaker Line
            </h2>
            <p style={{ margin: '4px 0 0', fontSize: '13px', color: 'var(--text-muted)' }}>
              Inserts a clean horizontal divider separator above each new user prompt, clearly delineating the previous
              agent response from your new prompt.
            </p>
          </div>

          <ToggleSwitch
            checked={config.breaker_line_enabled}
            onChange={(checked) =>
              setConfig({
                ...config,
                breaker_line_enabled: checked,
              })
            }
          />
        </div>
      </div>

      {/* Feature 4: Predefined Default Project for New Conversations */}
      <div className="google-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '16px' }}>
          <div>
            <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
              Predefined Default Project for New Conversations
            </h2>
            <p style={{ margin: '4px 0 0', fontSize: '13px', color: 'var(--text-muted)', maxWidth: '600px' }}>
              Set a fixed predefined project when clicking the "+ New Conversation" button or pressing Ctrl+N / Cmd+N.
              By default, Antigravity picks the last opened chat's project; configuring this anchors new draft chats to your preferred project automatically.
            </p>
          </div>

          <div style={{ minWidth: '240px' }}>
            <select
              value={config.default_new_project || ''}
              onChange={(e) =>
                setConfig({
                  ...config,
                  default_new_project: e.target.value,
                })
              }
              style={{
                width: '100%',
                height: '38px',
                padding: '0 12px',
                borderRadius: '8px',
                border: '1.5px solid #cbd5e1',
                background: '#f8fafc',
                fontSize: '13px',
                fontWeight: 600,
                color: 'var(--text)',
                cursor: 'pointer',
              }}
            >
              <option value="">Auto (Antigravity Default)</option>
              {projects.map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </select>
          </div>
        </div>
      </div>

      {/* Feature 5: Project Colors & Active Conversation Tab Indicator */}
      {guiConfig && (
        <div className="google-card">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '20px' }}>
            <div>
              <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
                Project Colors & Active Conversation Indicator
              </h2>
              <p style={{ margin: '4px 0 0', fontSize: '13px', color: 'var(--text-muted)' }}>
                Assign custom accent colors to projects and configure how the current open conversation tab is highlighted in the sidebar.
              </p>
            </div>

            <ToggleSwitch
              checked={guiConfig.color_styling_enabled}
              onChange={(checked) =>
                setGuiConfig({
                  ...guiConfig,
                  color_styling_enabled: checked,
                })
              }
            />
          </div>

          {guiConfig.color_styling_enabled && (
            <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1.4fr) minmax(280px, 1fr)', gap: '28px', marginTop: '16px' }}>
              {/* Settings Controls */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: '18px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '13px', fontWeight: 600, color: '#1e293b', marginBottom: '8px' }}>
                    Open Conversation Highlight Mode:
                  </label>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                    {/* Mode 1: Denser Background */}
                    <div
                      onClick={() =>
                        setGuiConfig({
                          ...guiConfig,
                          active_conversation_indicator: 'background',
                        })
                      }
                      style={{
                        display: 'flex',
                        alignItems: 'flex-start',
                        gap: '12px',
                        padding: '12px 14px',
                        borderRadius: '8px',
                        border: `1.5px solid ${
                          guiConfig.active_conversation_indicator !== 'border' ? '#0b57d0' : '#e2e8f0'
                        }`,
                        background:
                          guiConfig.active_conversation_indicator !== 'border' ? '#eff6ff' : '#f8fafc',
                        cursor: 'pointer',
                        transition: 'all 0.15s',
                      }}
                    >
                      <input
                        type="radio"
                        name="active_indicator"
                        checked={guiConfig.active_conversation_indicator !== 'border'}
                        onChange={() =>
                          setGuiConfig({
                            ...guiConfig,
                            active_conversation_indicator: 'background',
                          })
                        }
                        style={{ marginTop: '2px', accentColor: '#0b57d0', cursor: 'pointer' }}
                      />
                      <div>
                        <div style={{ fontSize: '13px', fontWeight: 600, color: '#1e293b' }}>
                          Darker / Denser Background Tint (Default)
                        </div>
                        <div style={{ fontSize: '12px', color: '#64748b', marginTop: '2px', lineHeight: 1.4 }}>
                          Deepens the background color of the active conversation tab compared to ordinary tabs.
                        </div>
                      </div>
                    </div>

                    {/* Mode 2: Denser Border Outline */}
                    <div
                      onClick={() =>
                        setGuiConfig({
                          ...guiConfig,
                          active_conversation_indicator: 'border',
                        })
                      }
                      style={{
                        display: 'flex',
                        alignItems: 'flex-start',
                        gap: '12px',
                        padding: '12px 14px',
                        borderRadius: '8px',
                        border: `1.5px solid ${
                          guiConfig.active_conversation_indicator === 'border' ? '#0b57d0' : '#e2e8f0'
                        }`,
                        background:
                          guiConfig.active_conversation_indicator === 'border' ? '#eff6ff' : '#f8fafc',
                        cursor: 'pointer',
                        transition: 'all 0.15s',
                      }}
                    >
                      <input
                        type="radio"
                        name="active_indicator"
                        checked={guiConfig.active_conversation_indicator === 'border'}
                        onChange={() =>
                          setGuiConfig({
                            ...guiConfig,
                            active_conversation_indicator: 'border',
                          })
                        }
                        style={{ marginTop: '2px', accentColor: '#0b57d0', cursor: 'pointer' }}
                      />
                      <div>
                        <div style={{ fontSize: '13px', fontWeight: 600, color: '#1e293b' }}>
                          Denser Border Outline (Light Background)
                        </div>
                        <div style={{ fontSize: '12px', color: '#64748b', marginTop: '2px', lineHeight: 1.4 }}>
                          Adds a border matching the denser project color, while the background remains as light as ordinary tabs.
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                {/* Bold text option */}
                <div style={{ paddingTop: '6px', borderTop: '1px solid #f1f5f9' }}>
                  <label
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '10px',
                      cursor: 'pointer',
                      fontSize: '13px',
                      color: '#1e293b',
                      fontWeight: 500,
                    }}
                  >
                    <ToggleSwitch
                      size="sm"
                      checked={guiConfig.active_conversation_bold ?? true}
                      onChange={(checked) =>
                        setGuiConfig({
                          ...guiConfig,
                          active_conversation_bold: checked,
                        })
                      }
                    />
                    <span>Bold text on current open conversation tab</span>
                  </label>
                  <p style={{ margin: '3px 0 0 42px', fontSize: '11px', color: '#64748b' }}>
                    When unchecked, the open conversation tab title uses regular font weight matching ordinary tabs.
                  </p>
                </div>

                {/* Solid left edge option */}
                <div style={{ paddingTop: '6px', borderTop: '1px solid #f1f5f9' }}>
                  <label
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '10px',
                      cursor: 'pointer',
                      fontSize: '13px',
                      color: '#1e293b',
                      fontWeight: 500,
                    }}
                  >
                    <ToggleSwitch
                      size="sm"
                      checked={guiConfig.solid_left_edge ?? false}
                      onChange={(checked) =>
                        setGuiConfig({
                          ...guiConfig,
                          solid_left_edge: checked,
                        })
                      }
                    />
                    <span>Solid 3px color bar on left edge of conversation tabs</span>
                  </label>
                </div>

                {/* Opacity slider */}
                <div style={{ paddingTop: '6px', borderTop: '1px solid #f1f5f9' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
                    <label style={{ fontSize: '13px', fontWeight: 500, color: '#1e293b' }}>
                      Conversation Tab Tint Opacity:
                    </label>
                    <span style={{ fontSize: '12px', fontWeight: 600, color: '#0b57d0' }}>
                      {Math.round((guiConfig.tint_opacity || 0.14) * 100)}%
                    </span>
                  </div>
                  <input
                    type="range"
                    min="5"
                    max="35"
                    value={Math.round((guiConfig.tint_opacity || 0.14) * 100)}
                    onChange={(e) =>
                      setGuiConfig({
                        ...guiConfig,
                        tint_opacity: parseInt(e.target.value) / 100,
                      })
                    }
                    style={{ width: '100%', cursor: 'pointer', accentColor: '#0b57d0' }}
                  />
                </div>
              </div>

              {/* Real-time Interactive Preview */}
              <div
                style={{
                  background: '#f8fafc',
                  border: '1px solid #e2e8f0',
                  borderRadius: '10px',
                  padding: '16px',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '12px',
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ fontSize: '11px', fontWeight: 700, color: '#64748b', letterSpacing: '0.5px', textTransform: 'uppercase' }}>
                    Sidebar Live Preview
                  </span>
                  <span style={{ fontSize: '11px', color: '#94a3b8' }}>Antigravity 2.0</span>
                </div>

                {/* Preview Sidebar Snippet */}
                <div
                  style={{
                    background: '#ffffff',
                    border: '1px solid #cbd5e1',
                    borderRadius: '10px',
                    padding: '10px',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '4px',
                    boxShadow: '0 2px 4px rgba(0,0,0,0.04)',
                  }}
                >
                  {/* Project Header */}
                  <div
                    style={{
                      background: '#0b57d0',
                      color: '#ffffff',
                      borderRadius: '8px',
                      padding: '6px 10px',
                      fontSize: '13px',
                      fontWeight: 600,
                      display: 'flex',
                      alignItems: 'center',
                      gap: '8px',
                      userSelect: 'none',
                    }}
                  >
                    <span>📁</span>
                    <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      Antigravity Swiss Knife
                    </span>
                  </div>

                  {/* Active Open Conversation Tab */}
                  <div
                    style={{
                      backgroundColor:
                        guiConfig.active_conversation_indicator === 'border'
                          ? `rgba(11, 87, 208, ${guiConfig.tint_opacity || 0.14})`
                          : `rgba(11, 87, 208, ${(guiConfig.tint_opacity || 0.14) + 0.16})`,
                      border:
                        guiConfig.active_conversation_indicator === 'border'
                          ? '1.5px solid rgba(11, 87, 208, 0.60)'
                          : '1.5px solid transparent',
                      borderLeft: guiConfig.solid_left_edge
                        ? '3px solid #0b57d0'
                        : guiConfig.active_conversation_indicator === 'border'
                        ? '1.5px solid rgba(11, 87, 208, 0.60)'
                        : '1.5px solid transparent',
                      borderRadius: '8px',
                      padding: '6px 10px',
                      fontSize: '13px',
                      fontWeight: (guiConfig.active_conversation_bold ?? true) ? 700 : 400,
                      color: '#0f172a',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      boxSizing: 'border-box',
                      transition: 'all 0.15s ease',
                      userSelect: 'none',
                    }}
                  >
                    <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      Task Completion Check
                    </span>
                    <span style={{ fontSize: '11px', color: '#64748b', opacity: 0.7 }}>⟳</span>
                  </div>

                  {/* Ordinary Conversation Tab 1 */}
                  <div
                    style={{
                      backgroundColor: `rgba(11, 87, 208, ${guiConfig.tint_opacity || 0.14})`,
                      border: '1.5px solid transparent',
                      borderLeft: guiConfig.solid_left_edge ? '3px solid #0b57d0' : '1.5px solid transparent',
                      borderRadius: '8px',
                      padding: '6px 10px',
                      fontSize: '13px',
                      fontWeight: 400,
                      color: '#475569',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      boxSizing: 'border-box',
                      userSelect: 'none',
                    }}
                  >
                    <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      Matching Font and UI ...
                    </span>
                    <span style={{ fontSize: '11px', color: '#64748b' }}>6h</span>
                  </div>

                  {/* Ordinary Conversation Tab 2 */}
                  <div
                    style={{
                      backgroundColor: `rgba(11, 87, 208, ${guiConfig.tint_opacity || 0.14})`,
                      border: '1.5px solid transparent',
                      borderLeft: guiConfig.solid_left_edge ? '3px solid #0b57d0' : '1.5px solid transparent',
                      borderRadius: '8px',
                      padding: '6px 10px',
                      fontSize: '13px',
                      fontWeight: 400,
                      color: '#475569',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      boxSizing: 'border-box',
                      userSelect: 'none',
                    }}
                  >
                    <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      Antigravity Manager Pl...
                    </span>
                    <span style={{ fontSize: '11px', color: '#64748b' }}>6h</span>
                  </div>
                </div>

                <div style={{ fontSize: '11px', color: '#64748b', textAlign: 'center', lineHeight: 1.4 }}>
                  {guiConfig.active_conversation_indicator === 'border' ? (
                    <span>
                      ✓ Active tab has <strong>denser border outline</strong> with <strong>light background</strong>
                    </span>
                  ) : (
                    <span>
                      ✓ Active tab has <strong>darker/denser background tint</strong>
                    </span>
                  )}
                  {(guiConfig.active_conversation_bold ?? true) ? ' (bold title)' : ' (regular title)'}
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
