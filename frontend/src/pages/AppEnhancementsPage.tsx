import React, { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import {
  Save,
  Archive,
  Folder,
  Plus,
  MoreVertical,
  Layers,
  Split,
  CheckCircle2,
  FileText,
  ChevronDown,
  ChevronUp,
  ChevronRight,
} from 'lucide-react'
import { GithubIcon } from '../components/GithubIcon'
import { ToggleSwitch } from '../components/ToggleSwitch'
import { api } from '../api'
import type { EnhancementsConfig, GUIConfig } from '../types'

const PRESET_COLORS = [
  { name: 'Slate Black', hex: '#0f172a' },
  { name: 'Slate Grey', hex: '#64748b' },
  { name: 'Google Blue', hex: '#0b57d0' },
  { name: 'Vibrant Purple', hex: '#7c3aed' },
  { name: 'Emerald Green', hex: '#059669' },
  { name: 'Warm Amber', hex: '#d97706' },
  { name: 'Coral Red', hex: '#dc2626' },
]

// Generate 10x10 color palette grid
const GREYSCALE = ['#ffffff', '#f4f4f5', '#e4e4e7', '#cbd5e1', '#94a3b8', '#64748b', '#475569', '#334155', '#1e293b', '#09090b']
const CHROMATIC_HUES = [0, 40, 80, 120, 160, 200, 240, 280, 320]
const LIGHTNESSES = [92, 84, 76, 68, 60, 52, 44, 36, 28, 20]
const GRID_COLORS: string[][] = []
for (let r = 0; r < 10; r++) {
  const row: string[] = []
  const l = LIGHTNESSES[r]
  for (let c = 0; c < 10; c++) {
    if (c === 0) {
      row.push(GREYSCALE[r])
    } else {
      row.push(`hsl(${CHROMATIC_HUES[c - 1]}, 82%, ${l}%)`)
    }
  }
  GRID_COLORS.push(row)
}

function hslToHex(hsl: string): string {
  if (hsl.startsWith('#')) return hsl
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

function parseColorWithAlpha(colorStr: string): { r: number; g: number; b: number; alpha: number } {
  const c = (colorStr || '').trim()
  if (c.startsWith('#')) {
    const hex = c.slice(1)
    if (hex.length === 3) {
      const r = parseInt(hex[0] + hex[0], 16)
      const g = parseInt(hex[1] + hex[1], 16)
      const b = parseInt(hex[2] + hex[2], 16)
      return { r, g, b, alpha: 1.0 }
    }
    if (hex.length === 4) {
      const r = parseInt(hex[0] + hex[0], 16)
      const g = parseInt(hex[1] + hex[1], 16)
      const b = parseInt(hex[2] + hex[2], 16)
      const a = parseInt(hex[3] + hex[3], 16) / 255.0
      return { r, g, b, alpha: a }
    }
    if (hex.length === 6) {
      const r = parseInt(hex.slice(0, 2), 16)
      const g = parseInt(hex.slice(2, 4), 16)
      const b = parseInt(hex.slice(4, 6), 16)
      return { r, g, b, alpha: 1.0 }
    }
    if (hex.length === 8) {
      const r = parseInt(hex.slice(0, 2), 16)
      const g = parseInt(hex.slice(2, 4), 16)
      const b = parseInt(hex.slice(4, 6), 16)
      const a = parseInt(hex.slice(6, 8), 16) / 255.0
      return { r, g, b, alpha: a }
    }
  } else if (c.startsWith('rgba(')) {
    const parts = c.slice(5, -1).split(',')
    if (parts.length === 4) {
      return {
        r: parseInt(parts[0].trim(), 10) || 0,
        g: parseInt(parts[1].trim(), 10) || 0,
        b: parseInt(parts[2].trim(), 10) || 0,
        alpha: parseFloat(parts[3].trim()) || 1.0,
      }
    }
  } else if (c.startsWith('rgb(')) {
    const parts = c.slice(4, -1).split(',')
    if (parts.length === 3) {
      return {
        r: parseInt(parts[0].trim(), 10) || 0,
        g: parseInt(parts[1].trim(), 10) || 0,
        b: parseInt(parts[2].trim(), 10) || 0,
        alpha: 1.0,
      }
    }
  }
  return { r: 11, g: 87, b: 208, alpha: 1.0 }
}

function calculateColorMultiplier(r: number, g: number, b: number, alpha: number): number {
  const alphaMult = Math.min(1.0, Math.max(0.05, alpha <= 0 ? 1.0 : alpha))
  const y = (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255.0
  let lightnessMult = 1.0
  if (y > 0.6) {
    lightnessMult = Math.max(0.2, 1.0 - (y - 0.6) * 0.75)
  }
  return Math.min(1.0, Math.max(0.1, alphaMult * lightnessMult))
}

interface AppEnhancementsPageProps {
  activeCategoryTab?: number // 0: Chat View, 1: Project Panel, 2: Overview Panel, 3: Chat History
}

export const AppEnhancementsPage: React.FC<AppEnhancementsPageProps> = ({
  activeCategoryTab = 0,
}) => {
  const [config, setConfig] = useState<EnhancementsConfig | null>(null)
  const [guiConfig, setGuiConfig] = useState<GUIConfig | null>(null)
  const [chatNavigatorMode, setChatNavigatorMode] = useState<'panel' | 'line'>('panel')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [applying, setApplying] = useState(false)
  const [statusMsg, setStatusMsg] = useState<{ text: string; type: 'success' | 'error' } | null>(null)
  const [previewHover, setPreviewHover] = useState<number | null>(null)
  const [projects, setProjects] = useState<string[]>([])
  const [archiving, setArchiving] = useState(false)
  const [archiveResult, setArchiveResult] = useState<string | null>(null)
  const [previewExpanded, setPreviewExpanded] = useState(false)

  const activeProjectName = projects[0] || 'Default Workspace'
  const secondaryProjectName = projects[1] || 'Secondary Workspace'
  const baseTintOpacity = guiConfig?.tint_opacity ?? 0.15
  const previewProjColor = (guiConfig?.project_colors && activeProjectName in guiConfig.project_colors)
    ? guiConfig.project_colors[activeProjectName]
    : (Object.values(guiConfig?.project_colors || {})[0] || '#0b57d0')
  const secondaryProjColor = (guiConfig?.project_colors && secondaryProjectName in guiConfig.project_colors)
    ? guiConfig.project_colors[secondaryProjectName]
    : '#059669'
  const parsedProjColor = parseColorWithAlpha(previewProjColor)
  const parsedSecondaryColor = parseColorWithAlpha(secondaryProjColor)
  const projColorLum = (0.2126 * parsedProjColor.r + 0.7152 * parsedProjColor.g + 0.0722 * parsedProjColor.b) / 255.0
  const secondaryColorLum = (0.2126 * parsedSecondaryColor.r + 0.7152 * parsedSecondaryColor.g + 0.0722 * parsedSecondaryColor.b) / 255.0
  const projCardTextColor = projColorLum > 0.6 ? '#0f172a' : '#ffffff'
  const secondaryCardTextColor = secondaryColorLum > 0.6 ? '#0f172a' : '#ffffff'
  const projAccentTextColor = projColorLum > 0.6 ? '#334155' : previewProjColor
  const projColorMult = calculateColorMultiplier(parsedProjColor.r, parsedProjColor.g, parsedProjColor.b, parsedProjColor.alpha)
  const effectivePreviewOpacity = Number((baseTintOpacity * projColorMult).toFixed(3))
  const effectiveActiveOpacity = Number((Math.min(1.0, (baseTintOpacity + 0.16) * projColorMult)).toFixed(3))

  const [portalTarget, setPortalTarget] = useState<HTMLElement | null>(null)

  useEffect(() => {
    setPortalTarget(document.getElementById('top-bar-right'))
  }, [])

  const handleTriggerAutoArchive = async () => {
    try {
      setArchiving(true)
      setArchiveResult(null)
      const horizon = guiConfig?.auto_archive_horizon || '14d'
      const res = await api.autoArchiveConversations(horizon)
      if (res.success) {
        setArchiveResult(res.message || `Archived ${res.archived_count} conversation(s)`)
      } else {
        setArchiveResult(res.message || 'No stale conversations found')
      }
    } catch (err: any) {
      setStatusMsg({ text: 'Auto-archive failed: ' + err.message, type: 'error' })
    } finally {
      setArchiving(false)
    }
  }

  const loadConfig = async () => {
    try {
      setLoading(true)
      const [data, projList, gData] = await Promise.all([
        api.getEnhancements(),
        api.getGUIProjects().catch(() => []),
        api.getGUIConfig().catch(() => null),
      ])
      // The overview-panel section no longer has an on/off switch — stored `false`
      // would silently disable its styling, so normalize it back on.
      if (data?.overview_panel && !data.overview_panel.enabled) {
        data.overview_panel = { ...data.overview_panel, enabled: true }
      }
      setConfig(data)
      setProjects((projList || []).map((p) => p.name))
      if (gData) {
        setChatNavigatorMode(gData.chat_navigator_mode === 'line' ? 'line' : 'panel')
        setGuiConfig({
          ...gData,
          enabled: gData.enabled ?? true,
          color_styling_enabled: gData.color_styling_enabled ?? true,
          replace_see_all_triangle: gData.replace_see_all_triangle ?? true,
          consistent_project_spacing: gData.consistent_project_spacing ?? true,
          consistent_project_spacing_line: gData.consistent_project_spacing_line ?? true,
        })
      }
    } catch (err: any) {
      setStatusMsg({ text: 'Failed to load enhancements config: ' + err.message, type: 'error' })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadConfig()
  }, [])

  const isValidHexColor = (c?: string) => {
    if (!c) return true
    return /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/.test(c.trim())
  }

  const handleUpdateNavigatorMode = async (nextMode: 'panel' | 'line') => {
    setChatNavigatorMode(nextMode)
    try {
      await api.updateGUIConfig({ chat_navigator_mode: nextMode })
      setGuiConfig((current) => (current ? { ...current, chat_navigator_mode: nextMode } : current))
    } catch (err: any) {
      setStatusMsg({ text: 'Failed to save Chat Navigator mode: ' + err.message, type: 'error' })
    }
  }

  const updateConfig = async (patch: Partial<EnhancementsConfig>) => {
    if (!config) return
    const previousConfig = config
    const nextConfig = { ...config, ...patch }
    setConfig(nextConfig)
    try {
      await api.updateEnhancements(nextConfig)
    } catch (err: any) {
      setConfig(previousConfig)
      setStatusMsg({ text: 'Failed to update enhancement: ' + err.message, type: 'error' })
    }
  }

  const handleSaveAndApply = async () => {
    if (config?.prompt_jump_bar?.custom_color && !isValidHexColor(config.prompt_jump_bar.custom_color)) {
      setStatusMsg({ text: 'Invalid hex color format for prompt jump bar (expected e.g. #0b57d0)', type: 'error' })
      return
    }
    try {
      setSaving(true)
      setApplying(true)
      const promises: Promise<any>[] = []
      if (config) promises.push(api.updateEnhancements(config))
      if (guiConfig) promises.push(api.updateGUIConfig(guiConfig))
      await Promise.all(promises)
      const res = await api.applyGUI().catch(() => api.applyEnhancements())
      setStatusMsg({
        text: res.message || 'Settings saved locally and applied to Antigravity!',
        type: 'success',
      })
      setTimeout(() => setStatusMsg(null), 4000)
    } catch (err: any) {
      setStatusMsg({ text: 'Failed to save & apply settings: ' + err.message, type: 'error' })
    } finally {
      setSaving(false)
      setApplying(false)
    }
  }

  const handleAuxTabsFormatChange = async (format: 'icon' | 'icon_and_name') => {
    if (!config) return
    const updatedOp = { ...op, aux_tabs_format: format }
    const updatedConfig: EnhancementsConfig = {
      ...config,
      overview_panel: updatedOp,
    }
    setConfig(updatedConfig)
    try {
      localStorage.setItem('antigravity_swiss_aux_tab_format', format)
      window.dispatchEvent(new CustomEvent('swiss-aux-tab-format-updated', { detail: { format } }))
    } catch {}
    try {
      await api.updateEnhancements(updatedConfig)
    } catch (err: any) {
      console.error('Failed to auto-save aux_tabs_format:', err)
    }
  }

  if (loading || !config) {
    return (
      <div style={{ padding: '32px', textAlign: 'center', color: 'var(--text-muted)' }}>
        <div style={{ fontSize: '14px', fontWeight: 500 }}>Loading App Enhancements...</div>
      </div>
    )
  }

  const jb = config.prompt_jump_bar
  const contextWindowIndicatorEnabled = config.context_window_indicator_enabled !== false
  const contextWindowClickToCompactEnabled =
    config.context_window_click_to_compact_enabled !== false
  const rawOp = config.overview_panel
  const op = {
    enabled: rawOp ? rawOp.enabled : true,
    division_style: rawOp?.division_style || 'divider_line',
    zone_border_radius: rawOp?.zone_border_radius || 8,
    zone_border_color: rawOp?.zone_border_color || '#e2e8f0',
    zone_background_contrast: rawOp?.zone_background_contrast || 'whiter',
    zone_padding: rawOp?.zone_padding || 10,
    zone_gap: rawOp?.zone_gap || 10,
    replace_see_all_triangle: rawOp?.replace_see_all_triangle !== false,
    aux_tabs_format: rawOp?.aux_tabs_format || 'icon',
  }
  const rawActiveColor =
    jb.color_mode === 'default'
      ? '#64748b'
      : jb.color_mode === 'project'
      ? '#059669' // Sample project emerald green for preview
      : jb.custom_color || '#0b57d0'
  const parsedActiveRgb = parseColorWithAlpha(rawActiveColor)
  const activeColorLum = (0.2126 * parsedActiveRgb.r + 0.7152 * parsedActiveRgb.g + 0.0722 * parsedActiveRgb.b) / 255.0
  const activeColor = activeColorLum > 0.85 ? '#334155' : rawActiveColor

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {portalTarget &&
        createPortal(
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <button
              onClick={handleSaveAndApply}
              disabled={saving || applying}
              className="btn-pill-primary"
              style={{ padding: '7px 18px', fontSize: '12px', whiteSpace: 'nowrap' }}
            >
              <Save size={14} />
              <span style={{ whiteSpace: 'nowrap' }}>{saving || applying ? 'Saving & Applying...' : 'Save & Apply'}</span>
            </button>
          </div>,
          portalTarget
        )}

      {statusMsg && (
        <div
          style={{
            padding: '10px 16px',
            borderRadius: '8px',
            fontSize: '13px',
            fontWeight: 500,
            background: statusMsg.type === 'success' ? 'var(--green-bg)' : 'var(--red-bg)',
            color: statusMsg.type === 'success' ? 'var(--green)' : 'var(--red)',
            border: `1px solid ${statusMsg.type === 'success' ? 'var(--border-success)' : 'var(--border-danger)'}`,
          }}
        >
          {statusMsg.text}
        </div>
      )}

      {/* Category 1: Chat View */}
      {activeCategoryTab === 0 && (
        <>
          {/* Feature 1: Chat Navigator */}
          <div className="google-card">
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            marginBottom: jb.enabled ? '16px' : '0',
          }}
        >
          <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
            Chat Navigator
          </h2>

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
          <div style={{ borderTop: '1px solid var(--border)', paddingTop: '18px' }}>
            {/* Optional interaction setting: Pulse Highlight Target Prompt Card on Jump */}
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                background: 'var(--canvas)',
                borderRadius: '10px',
                border: '1px solid var(--border)',
                padding: '12px 16px',
                marginBottom: '20px',
              }}
            >
              <label
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  width: '100%',
                  cursor: 'pointer',
                  fontSize: '13px',
                  color: 'var(--text)',
                }}
              >
                <span style={{ fontWeight: 500 }}>Pulse Highlight Target Prompt Card On Jump</span>
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
              </label>
            </div>

            {/* Chat navigator display mode */}
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                gap: '12px',
                padding: '12px 16px',
                marginBottom: '20px',
                border: '1px solid var(--border)',
                borderRadius: '10px',
                background: 'var(--canvas)',
              }}
            >
              <label
                htmlFor="chat-navigator-mode"
                style={{ fontSize: '13px', fontWeight: 500, color: 'var(--text)' }}
              >
                Navigator Style
              </label>
              <select
                id="chat-navigator-mode"
                aria-label="Navigator Style"
                value={chatNavigatorMode}
                onChange={(e) => handleUpdateNavigatorMode(e.target.value === 'line' ? 'line' : 'panel')}
                style={{
                  padding: '6px 10px',
                  fontSize: '12px',
                  borderRadius: '6px',
                  border: '1px solid var(--border)',
                  backgroundColor: 'var(--surface, #ffffff)',
                  color: 'var(--text)',
                  cursor: 'pointer',
                }}
              >
                <option value="panel">Panel (Default)</option>
                <option value="line">Single Line</option>
              </select>
            </div>

            {/* 2-Column Layout: Settings on Left, Fixed Vertical Divider, Interactive Gutter Preview on Right */}
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'minmax(0, 1fr) 1px 360px',
                gap: '24px 12px',
                alignItems: 'stretch',
              }}
            >
              {/* LEFT COLUMN: Color Mode and Line Dimension Settings */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                {/* Color Mode Selection (User Request: removed default tag) */}
                <div
                  style={{
                    background: 'var(--canvas)',
                    borderRadius: '10px',
                    border: '1px solid var(--border)',
                    padding: '16px 20px',
                  }}
                >
                  <div style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)', marginBottom: '12px' }}>
                    Active & Hover Line Color
                  </div>

                  <div style={{ display: 'flex', gap: '20px', flexWrap: 'wrap', marginBottom: '14px' }}>
                    <label
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '8px',
                        fontSize: '13px',
                        color: 'var(--text)',
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
                      <span>Slate Grey</span>
                    </label>

                    <label
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '8px',
                        fontSize: '13px',
                        color: 'var(--text)',
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
                      <span>Match Project Color</span>
                    </label>

                    <label
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '8px',
                        fontSize: '13px',
                        color: 'var(--text)',
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
                      <span>Custom Color</span>
                    </label>
                  </div>

                  {/* Custom Color Palette */}
                  {jb.color_mode === 'custom' && (
                    <div
                      style={{
                        background: 'var(--surface, #ffffff)',
                        padding: '14px',
                        borderRadius: '8px',
                        border: '1px solid var(--border)',
                        marginTop: '10px',
                      }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '12px', flexWrap: 'wrap' }}>
                        <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
                          Presets:
                        </span>
                        <div style={{ display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap' }}>
                          {PRESET_COLORS.map((p) => {
                            const isSel = jb.custom_color?.toLowerCase() === p.hex.toLowerCase()
                            return (
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
                                  width: '22px',
                                  height: '22px',
                                  borderRadius: '50%',
                                  background: p.hex,
                                  cursor: 'pointer',
                                  boxSizing: 'border-box',
                                  border: isSel ? '2px solid var(--text)' : '2px solid transparent',
                                  boxShadow: isSel && p.hex === '#0f172a' ? 'inset 0 0 0 1.5px #ffffff' : 'none',
                                  transform: isSel ? 'scale(1.15)' : 'scale(1)',
                                  transition: 'transform 0.12s',
                                }}
                              />
                            )
                          })}
                        </div>

                        <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '8px' }}>
                          <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>Hex:</span>
                          <div
                            style={{
                              width: '18px',
                              height: '18px',
                              borderRadius: '4px',
                              background: isValidHexColor(jb.custom_color) ? (jb.custom_color || '#0b57d0') : '#0b57d0',
                              border: '1px solid var(--border)',
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
                              width: '80px',
                              padding: '3px 6px',
                              borderRadius: '4px',
                              border: '1px solid var(--border)',
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
                            color: 'var(--text-muted)',
                            textTransform: 'uppercase',
                            marginBottom: '6px',
                          }}
                        >
                          10&times;10 Extended Palette:
                        </span>
                        <div
                          style={{
                            display: 'grid',
                            gridTemplateColumns: 'repeat(10, 18px)',
                            gap: '3px',
                            padding: '6px',
                            background: 'var(--canvas)',
                            borderRadius: '6px',
                            border: '1px solid var(--border)',
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
                                  title={hex}
                                  style={{
                                    width: '18px',
                                    height: '18px',
                                    borderRadius: '2px',
                                    boxSizing: 'border-box',
                                    background: cellHsl,
                                    cursor: 'pointer',
                                    border: isSelected ? '2px solid var(--text)' : '1px solid rgba(0,0,0,0.08)',
                                    boxShadow:
                                      isSelected && (hex.toLowerCase() === '#09090b' || hex.toLowerCase() === '#1e293b')
                                        ? 'inset 0 0 0 1px #ffffff'
                                        : 'none',
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

                {/* Line Dimensions Settings (Width & Thickness) */}
                <div
                  style={{
                    background: 'var(--canvas)',
                    borderRadius: '10px',
                    border: '1px solid var(--border)',
                    padding: '16px 20px',
                  }}
                >
                  <div style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)', marginBottom: '14px' }}>
                    Line Dimensions
                  </div>

                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '16px' }}>
                    {/* Line Width */}
                    <div>
                      <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
                        Dash Width:
                      </label>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                        <input
                          type="number"
                          min="8"
                          max="36"
                          value={jb.dash_width || 14}
                          onChange={(e) =>
                            setConfig({
                              ...config,
                              prompt_jump_bar: { ...jb, dash_width: parseInt(e.target.value) || 14 },
                            })
                          }
                          style={{
                            width: '70px',
                            padding: '5px 8px',
                            borderRadius: '6px',
                            border: '1px solid var(--border)',
                            fontSize: '13px',
                          }}
                        />
                        <span style={{ fontSize: '12px', color: '#94a3b8' }}>px (Default: 14)</span>
                      </div>
                    </div>

                    {/* Active Line Thickness */}
                    <div>
                      <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
                        Active Thickness:
                      </label>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                        <input
                          type="number"
                          min="1.5"
                          max="8"
                          step="0.5"
                          value={jb.dash_thickness || 2.5}
                          onChange={(e) =>
                            setConfig({
                              ...config,
                              prompt_jump_bar: { ...jb, dash_thickness: parseFloat(e.target.value) || 2.5 },
                            })
                          }
                          style={{
                            width: '70px',
                            padding: '5px 8px',
                            borderRadius: '6px',
                            border: '1px solid var(--border)',
                            fontSize: '13px',
                          }}
                        />
                        <span style={{ fontSize: '12px', color: '#94a3b8' }}>px (Default: 2.5)</span>
                      </div>
                    </div>

                    {/* Inactive Line Thickness */}
                    <div>
                      <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', display: 'block', marginBottom: '6px' }}>
                        Inactive Thickness:
                      </label>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                        <input
                          type="number"
                          min="1"
                          max="4"
                          step="0.5"
                          value={jb.inactive_thickness || 1.5}
                          onChange={(e) =>
                            setConfig({
                              ...config,
                              prompt_jump_bar: { ...jb, inactive_thickness: parseFloat(e.target.value) || 1.5 },
                            })
                          }
                          style={{
                            width: '70px',
                            padding: '5px 8px',
                            borderRadius: '6px',
                            border: '1px solid var(--border)',
                            fontSize: '13px',
                          }}
                        />
                        <span style={{ fontSize: '12px', color: '#94a3b8' }}>px (Default: 1.5)</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              {/* VERTICAL DIVIDER */}
              <div style={{ width: '1px', backgroundColor: 'var(--border)', alignSelf: 'stretch' }} />

              {/* RIGHT COLUMN: Interactive Gutter Live Preview Component */}
              <div
                style={{
                  width: '360px',
                  boxSizing: 'border-box',
                  background: 'var(--canvas)',
                  borderRadius: '12px',
                  border: '1px dashed var(--border)',
                  padding: '20px',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '16px',
                  position: 'sticky',
                  top: '20px',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <div style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>Interactive Gutter Preview</div>
                </div>

                <div
                  style={{
                    backgroundColor: '#ffffff',
                    borderRadius: '8px',
                    border: '1px solid var(--border)',
                    padding: '20px',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '24px',
                  }}
                >
                  {/* Sample Prompt Jump Bar */}
                  <div
                    style={{
                      display: 'flex',
                      flexDirection: 'column',
                      gap: '7px',
                      padding: '8px 4px',
                      background: 'transparent',
                      userSelect: 'none',
                    }}
                  >
                    {[0, 1, 2, 3, 4].map((idx) => {
                      const isActive = idx === 2
                      const isHovered = previewHover === idx
                      const activeH = `${jb.dash_thickness || 2.5}px`
                      const inactiveH = `${jb.inactive_thickness || 1.5}px`
                      const h = isActive ? activeH : inactiveH
                      const dashWidth = jb.dash_width || 14
                      const bg = isActive || isHovered ? activeColor : 'rgba(100, 116, 139, 0.42)'
                      return (
                        <div
                          key={idx}
                          onMouseEnter={() => setPreviewHover(idx)}
                          onMouseLeave={() => setPreviewHover(null)}
                          title={`Preview Prompt #${idx + 1}`}
                          style={{
                            width: `${isHovered ? Math.max(22, dashWidth + 8) : dashWidth}px`,
                            height: h,
                            borderRadius: '2px',
                            background: bg,
                            cursor: 'pointer',
                            transition: 'width 0.18s cubic-bezier(0.4, 0, 0.2, 1), background 0.15s ease',
                          }}
                        />
                      )
                    })}
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Feature 2: Context Window Indicator */}
      <div className="google-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
            Context Window Indicator
          </h2>

          <ToggleSwitch
            checked={contextWindowIndicatorEnabled}
            onChange={(checked) =>
              setConfig({
                ...config,
                context_window_indicator_enabled: checked,
              })
            }
          />
        </div>

        {contextWindowIndicatorEnabled && (
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: '16px',
              marginTop: '18px',
              padding: '12px 16px',
              borderRadius: '10px',
              border: '1px solid var(--border)',
              background: 'var(--canvas)',
            }}
          >
            <div>
              <span style={{ display: 'block', fontSize: '13px', fontWeight: 500, color: 'var(--text)' }}>
                Click Ring To Compact
              </span>
            </div>
            <ToggleSwitch
              size="sm"
              checked={contextWindowClickToCompactEnabled}
              onChange={(checked) =>
                setConfig({
                  ...config,
                  context_window_click_to_compact_enabled: checked,
                })
              }
            />
          </div>
        )}
      </div>

      {/* Feature 3: Tool Call Density Control */}
      <div className="google-card">
        <div style={{ marginBottom: '16px' }}>
          <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
            Tool Call Density Control
          </h2>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '14px' }}>
          {[
            {
              id: 'muted',
              title: 'Greyed Out / Muted',
              desc: 'Dims intermediate tool steps and thoughts with subtle grayscale & 50% opacity. Hovering reveals full content.',
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
                border: `1px solid ${config.tool_density_mode === opt.id ? 'var(--primary)' : 'var(--border)'}`,
                background: config.tool_density_mode === opt.id ? 'var(--accent-subtle, #f0f7ff)' : 'var(--surface, #ffffff)',
                cursor: 'pointer',
                transition: 'border-color 0.15s, background 0.15s',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <input
                  type="radio"
                  name="tool_density"
                  checked={config.tool_density_mode === opt.id}
                  onChange={() => {}}
                  style={{ accentColor: '#0b57d0' }}
                />
                <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>{opt.title}</span>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Feature 3: Turn Breaker Line */}
      <div className="google-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
            Turn Breaker Line
          </h2>

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
    </>
  )}

  {/* Category 2: Project Panel */}
  {activeCategoryTab === 1 && (
    <>
      <div className="google-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '16px', flexWrap: 'wrap' }}>
          <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)', whiteSpace: 'nowrap' }}>
            Target Project in Automations
          </h2>
          <ToggleSwitch
            checked={config.automation_target_project_enabled !== false}
            onChange={(val) => updateConfig({ automation_target_project_enabled: val })}
          />
        </div>
      </div>

      {/* Feature 5: Project Colors & Active Conversation Tab Indicator */}
      {guiConfig && (
        <div className="google-card">
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              marginBottom: (guiConfig.color_styling_enabled ?? true) ? '20px' : 0,
            }}
          >
            <div>
              <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
                Project Colors & Active Conversation Indicator
              </h2>
            </div>

            <ToggleSwitch
              checked={guiConfig.color_styling_enabled ?? true}
              onChange={(checked) =>
                setGuiConfig({
                  ...guiConfig,
                  color_styling_enabled: checked,
                  enabled: checked,
                })
              }
            />
          </div>

          {(guiConfig.color_styling_enabled ?? true) && (
            <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) 1px 360px', gap: '24px 12px', alignItems: 'stretch', marginTop: '16px' }}>
              {/* Settings Controls */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: '18px' }}>
                {/* Mode Selection */}
                <div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                    {/* Mode 1: Accent Background / Fill */}
                    <div
                      onClick={() =>
                        setGuiConfig({
                          ...guiConfig,
                          active_conversation_indicator: 'background',
                        })
                      }
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '12px',
                        padding: '12px 14px',
                        borderRadius: '8px',
                        border: `1px solid ${
                          guiConfig.active_conversation_indicator === 'background' ? 'var(--primary)' : 'var(--border)'
                        }`,
                        background:
                          guiConfig.active_conversation_indicator === 'background' ? 'var(--accent-subtle, #eff6ff)' : '#f8fafc',
                        cursor: 'pointer',
                        transition: 'all 0.15s',
                      }}
                    >
                      <div style={{ flex: 1 }}>
                        <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                          Accent Background / Fill
                        </span>
                      </div>
                      <input
                        type="radio"
                        name="active_indicator"
                        checked={guiConfig.active_conversation_indicator === 'background'}
                        onChange={() =>
                          setGuiConfig({
                            ...guiConfig,
                            active_conversation_indicator: 'background',
                          })
                        }
                        style={{ accentColor: '#0b57d0', cursor: 'pointer', flexShrink: 0 }}
                      />
                    </div>

                    {/* Mode 2: Border Outline */}
                    <div
                      onClick={() =>
                        setGuiConfig({
                          ...guiConfig,
                          active_conversation_indicator: 'border',
                        })
                      }
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '12px',
                        padding: '12px 14px',
                        borderRadius: '8px',
                        border: `1px solid ${
                          guiConfig.active_conversation_indicator === 'border' ? 'var(--primary)' : 'var(--border)'
                        }`,
                        background:
                          guiConfig.active_conversation_indicator === 'border' ? 'var(--accent-subtle, #eff6ff)' : '#f8fafc',
                        cursor: 'pointer',
                        transition: 'all 0.15s',
                      }}
                    >
                      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '8px' }}>
                        <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                          Border Outline
                        </div>
                        {guiConfig.active_conversation_indicator === 'border' && (
                          <div
                            style={{ display: 'flex', alignItems: 'center', gap: '6px' }}
                            onClick={(e) => e.stopPropagation()}
                          >
                            <span style={{ fontSize: '12px', color: 'var(--text-muted)', fontWeight: 500 }}>Width:</span>
                            <select
                              value={guiConfig.active_conversation_border_width || '2px'}
                              onChange={(e) =>
                                setGuiConfig({
                                  ...guiConfig,
                                  active_conversation_border_width: e.target.value,
                                })
                              }
                              style={{
                                padding: '4px 8px',
                                borderRadius: '6px',
                                border: '1px solid var(--border)',
                                background: 'var(--surface, #ffffff)',
                                fontSize: '12px',
                                fontWeight: 600,
                                color: 'var(--text)',
                                cursor: 'pointer',
                              }}
                            >
                              <option value="1px">1px</option>
                              <option value="1.5px">1.5px</option>
                              <option value="2px">2px</option>
                              <option value="3px">3px</option>
                            </select>
                          </div>
                        )}
                      </div>
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
                        style={{ accentColor: '#0b57d0', cursor: 'pointer', flexShrink: 0 }}
                      />
                    </div>

                    {/* Mode 3: Left Accent Bar */}
                    <div
                      onClick={() =>
                        setGuiConfig({
                          ...guiConfig,
                          active_conversation_indicator: 'left_bar',
                        })
                      }
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '12px',
                        padding: '12px 14px',
                        borderRadius: '8px',
                        border: `1px solid ${
                          guiConfig.active_conversation_indicator === 'left_bar' ? 'var(--primary)' : 'var(--border)'
                        }`,
                        background:
                          guiConfig.active_conversation_indicator === 'left_bar' ? 'var(--accent-subtle, #eff6ff)' : '#f8fafc',
                        cursor: 'pointer',
                        transition: 'all 0.15s',
                      }}
                    >
                      <div style={{ flex: 1 }}>
                        <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                          Left Accent Bar
                        </div>
                      </div>
                      <input
                        type="radio"
                        name="active_indicator"
                        checked={guiConfig.active_conversation_indicator === 'left_bar'}
                        onChange={() =>
                          setGuiConfig({
                            ...guiConfig,
                            active_conversation_indicator: 'left_bar',
                          })
                        }
                        style={{ accentColor: '#0b57d0', cursor: 'pointer', flexShrink: 0 }}
                      />
                    </div>
                  </div>
                </div>

                {/* Bold text option with unified divider line */}
                <div style={{ paddingTop: '10px', borderTop: '1px solid var(--border)' }}>
                  <label
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      gap: '10px',
                      cursor: 'pointer',
                      fontSize: '13px',
                      color: 'var(--text)',
                      fontWeight: 500,
                    }}
                  >
                    <span>Bold Text On Current Open Conversation Tab</span>
                    <ToggleSwitch
                      size="sm"
                      checked={guiConfig.active_conversation_bold ?? false}
                      onChange={(checked) =>
                        setGuiConfig({
                          ...guiConfig,
                          active_conversation_bold: checked,
                        })
                      }
                    />
                  </label>
                </div>

                {/* Opacity slider with unified divider line */}
                <div style={{ paddingTop: '10px', borderTop: '1px solid var(--border)' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
                    <label style={{ fontSize: '13px', fontWeight: 500, color: 'var(--text)' }}>
                      Conversation Tab Tint Opacity:
                    </label>
                    <span style={{ fontSize: '12px', fontWeight: 600, color: '#0b57d0' }}>
                      {Math.round((guiConfig.tint_opacity ?? 0.15) * 100)}%
                    </span>
                  </div>
                  <input
                    type="range"
                    min="5"
                    max="35"
                    value={Math.round((guiConfig.tint_opacity ?? 0.15) * 100)}
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

              {/* VERTICAL DIVIDER */}
              <div style={{ width: '1px', backgroundColor: 'var(--border)', alignSelf: 'stretch' }} />

              {/* Real-time Interactive Preview */}
              <div
                style={{
                  width: '360px',
                  boxSizing: 'border-box',
                  background: 'var(--canvas)',
                  border: '1px solid var(--border)',
                  borderRadius: '10px',
                  padding: '16px',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '12px',
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.5px', textTransform: 'uppercase' }}>
                    Sidebar Live Preview
                  </span>
                  <span style={{ fontSize: '11px', color: '#94a3b8' }}>Antigravity 2.0</span>
                </div>

                {/* Preview Sidebar Snippet */}
                <div
                  style={{
                    background: '#ffffff',
                    border: '1px solid var(--border)',
                    borderRadius: '10px',
                    padding: '10px',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '4px',
                    boxShadow: '0 2px 4px rgba(0,0,0,0.04)',
                    fontFamily: "system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
                  }}
                >
                  {/* Project Header */}
                  <div
                    style={{
                      background: previewProjColor,
                      color: projCardTextColor,
                      borderRadius: '8px',
                      height: '32px',
                      padding: '0 10px',
                      fontSize: '13px',
                      fontWeight: 600,
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      userSelect: 'none',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: 0, overflow: 'hidden' }}>
                      <Folder size={14} style={{ flexShrink: 0 }} />
                      <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {activeProjectName}
                      </span>
                    </div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexShrink: 0, opacity: 0.9 }}>
                      <Plus size={13} style={{ cursor: 'pointer' }} />
                      <MoreVertical size={13} style={{ cursor: 'pointer' }} />
                    </div>
                  </div>

                  {/* Active Open Conversation Tab */}
                  <div
                    style={{
                      height: '32px',
                      backgroundColor:
                        guiConfig.active_conversation_indicator === 'background'
                          ? `rgba(${parsedProjColor.r}, ${parsedProjColor.g}, ${parsedProjColor.b}, ${effectiveActiveOpacity})`
                          : `rgba(${parsedProjColor.r}, ${parsedProjColor.g}, ${parsedProjColor.b}, ${effectivePreviewOpacity})`,
                      border:
                        guiConfig.active_conversation_indicator === 'border'
                          ? `${guiConfig.active_conversation_border_width || '2px'} solid ${projAccentTextColor}`
                          : '1px solid transparent',
                      borderLeft:
                        guiConfig.active_conversation_indicator === 'left_bar'
                          ? `3px solid ${projAccentTextColor}`
                          : guiConfig.active_conversation_indicator === 'border'
                          ? `${guiConfig.active_conversation_border_width || '2px'} solid ${projAccentTextColor}`
                          : '1px solid transparent',
                      borderRadius: '8px',
                      padding: '0 10px',
                      fontSize: '13px',
                      fontWeight: (guiConfig.active_conversation_bold ?? false) ? 700 : 400,
                      color: 'var(--text)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      boxSizing: 'border-box',
                      transition: 'all 0.15s ease',
                      userSelect: 'none',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '7px', minWidth: 0, overflow: 'hidden' }}>
                      <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        Task Completion Check
                      </span>
                    </div>
                    <span style={{ fontSize: '11px', color: projAccentTextColor, fontWeight: 600, flexShrink: 0 }}>Just now</span>
                  </div>

                  {/* Ordinary Conversation Tab 1 */}
                  <div
                    style={{
                      height: '32px',
                      backgroundColor: `rgba(${parsedProjColor.r}, ${parsedProjColor.g}, ${parsedProjColor.b}, ${effectivePreviewOpacity})`,
                      border: '1px solid transparent',
                      borderRadius: '8px',
                      padding: '0 10px',
                      fontSize: '13px',
                      fontWeight: 400,
                      color: 'var(--text-muted)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      boxSizing: 'border-box',
                      userSelect: 'none',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '7px', minWidth: 0, overflow: 'hidden' }}>
                      <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        Matching Font and UI ...
                      </span>
                    </div>
                    <span style={{ fontSize: '11px', color: 'var(--text-muted)', flexShrink: 0 }}>6h</span>
                  </div>

                  {/* Ordinary Conversation Tab 2 */}
                  <div
                    style={{
                      height: '32px',
                      backgroundColor: `rgba(${parsedProjColor.r}, ${parsedProjColor.g}, ${parsedProjColor.b}, ${effectivePreviewOpacity})`,
                      border: '1px solid transparent',
                      borderRadius: '8px',
                      padding: '0 10px',
                      fontSize: '13px',
                      fontWeight: 400,
                      color: 'var(--text-muted)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      boxSizing: 'border-box',
                      userSelect: 'none',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '7px', minWidth: 0, overflow: 'hidden' }}>
                      <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        Antigravity Manager Pl...
                      </span>
                    </div>
                    <span style={{ fontSize: '11px', color: 'var(--text-muted)', flexShrink: 0 }}>1d</span>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Feature 6: Conversation Tabs Display & Expand/Contract Divider */}
      {guiConfig && (
        <div className="google-card">
          <div style={{ marginBottom: '20px' }}>
            <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
              Conversation Tabs Display
            </h2>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) 1px 360px', gap: '24px 12px', alignItems: 'stretch' }}>
            {/* Left column: Controls stacked vertically */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
              {/* Simplicity Replacement Zone */}
              <div
                style={{
                  padding: '14px 16px',
                  borderRadius: '8px',
                  border: '1px solid var(--border)',
                  background: 'var(--card-bg, #ffffff)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: '16px',
                }}
              >
                <div style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
                  Simplicity Replacement For "See All" / "See Less"
                </div>
                <ToggleSwitch
                  checked={guiConfig.replace_see_all_triangle ?? true}
                  onChange={(checked) =>
                    setGuiConfig({
                      ...guiConfig,
                      replace_see_all_triangle: checked,
                    })
                  }
                />
              </div>

              {/* Consistent Project Spacing */}
              <div
                style={{
                  padding: '14px 16px',
                  borderRadius: '8px',
                  border: '1px solid var(--border)',
                  background: 'var(--card-bg, #ffffff)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: '16px',
                }}
              >
                <div style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
                  Consistent Project Spacing
                </div>
                <ToggleSwitch
                  checked={(guiConfig.consistent_project_spacing ?? true) && (guiConfig.consistent_project_spacing_line ?? true)}
                  onChange={(checked) =>
                    setGuiConfig({
                      ...guiConfig,
                      consistent_project_spacing: checked,
                      consistent_project_spacing_line: checked,
                    })
                  }
                />
              </div>

              {/* Horizontal Divider Line between Simplicity Zone and Fixed Number Zone */}
              <div style={{ height: '1px', backgroundColor: 'var(--border)', margin: '2px 0' }} />

              {/* Header above Fixed vs Dynamic */}
              <div style={{ marginTop: '12px', marginBottom: '8px' }}>
                <h4 style={{ margin: 0, fontSize: '14px', fontWeight: 600, color: 'var(--text)' }}>
                  Visible Conversation Tabs
                </h4>
              </div>

              {/* Option 1: Fixed Limit */}
              <div
                onClick={() =>
                  setGuiConfig({
                    ...guiConfig,
                    conversation_tabs_mode: 'fixed',
                  })
                }
                style={{
                  padding: '16px',
                  borderRadius: '8px',
                  border: `1px solid ${guiConfig.conversation_tabs_mode === 'fixed' ? 'var(--primary)' : 'var(--border)'}`,
                  background: guiConfig.conversation_tabs_mode === 'fixed' ? 'var(--accent-subtle, #f0f7ff)' : 'var(--card-bg, #ffffff)',
                  cursor: 'pointer',
                  transition: 'border-color 0.15s, background 0.15s',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px' }}>
                  <input
                    type="radio"
                    name="convo_tabs_mode"
                    checked={guiConfig.conversation_tabs_mode === 'fixed'}
                    onChange={() => {}}
                    style={{ accentColor: '#0b57d0' }}
                  />
                  <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
                    Fixed Number
                  </span>
                </div>

                <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }} onClick={(e) => e.stopPropagation()}>
                  <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>Tabs Per Project:</label>
                  <select
                    value={guiConfig.conversation_tabs_fixed_limit || 6}
                    onChange={(e) =>
                      setGuiConfig({
                        ...guiConfig,
                        conversation_tabs_mode: 'fixed',
                        conversation_tabs_fixed_limit: parseInt(e.target.value, 10),
                      })
                    }
                    style={{
                      height: '32px',
                      padding: '0 10px',
                      borderRadius: '6px',
                      border: '1px solid var(--border)',
                      background: 'var(--surface, #ffffff)',
                      fontSize: '13px',
                      fontWeight: 600,
                      color: 'var(--text)',
                      cursor: 'pointer',
                    }}
                  >
                    {[1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map((num) => (
                      <option key={num} value={num}>
                        {num}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              {/* Option 2: Dynamic by Chat Age */}
              <div
                onClick={() =>
                  setGuiConfig({
                    ...guiConfig,
                    conversation_tabs_mode: 'dynamic',
                  })
                }
                style={{
                  padding: '16px',
                  borderRadius: '8px',
                  border: `1px solid ${guiConfig.conversation_tabs_mode !== 'fixed' ? 'var(--primary)' : 'var(--border)'}`,
                  background: guiConfig.conversation_tabs_mode !== 'fixed' ? 'var(--accent-subtle, #f0f7ff)' : 'var(--card-bg, #ffffff)',
                  cursor: 'pointer',
                  transition: 'border-color 0.15s, background 0.15s',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '12px' }}>
                  <input
                    type="radio"
                    name="convo_tabs_mode"
                    checked={guiConfig.conversation_tabs_mode !== 'fixed'}
                    onChange={() => {}}
                    style={{ accentColor: '#0b57d0' }}
                  />
                  <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>Dynamic (By Chat Recency)</span>
                </div>

                <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }} onClick={(e) => e.stopPropagation()}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '10px' }}>
                    <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>Active Within:</label>
                    <select
                      value={guiConfig.conversation_tabs_age_threshold || '14d'}
                      onChange={(e) =>
                        setGuiConfig({
                          ...guiConfig,
                          conversation_tabs_mode: 'dynamic',
                          conversation_tabs_age_threshold: e.target.value as any,
                        })
                      }
                      style={{
                        height: '32px',
                        padding: '0 10px',
                        borderRadius: '6px',
                        border: '1px solid var(--border)',
                        background: 'var(--surface, #ffffff)',
                        fontSize: '13px',
                        fontWeight: 600,
                        color: 'var(--text)',
                        cursor: 'pointer',
                      }}
                    >
                      <option value="1d">1 Day</option>
                      <option value="3d">3 Days</option>
                      <option value="7d">7 Days</option>
                      <option value="14d">14 Days</option>
                      <option value="30d">30 Days</option>
                    </select>
                  </div>

                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>Min Tabs:</label>
                      <select
                        value={guiConfig.conversation_tabs_min ?? 3}
                        onChange={(e) =>
                          setGuiConfig({
                            ...guiConfig,
                            conversation_tabs_mode: 'dynamic',
                            conversation_tabs_min: parseInt(e.target.value, 10),
                          })
                        }
                        style={{
                          height: '30px',
                          padding: '0 8px',
                          borderRadius: '6px',
                          border: '1px solid var(--border)',
                          background: 'var(--surface, #ffffff)',
                          fontSize: '12px',
                          fontWeight: 600,
                          color: 'var(--text)',
                          cursor: 'pointer',
                          width: '100%',
                        }}
                      >
                        {[1, 2, 3, 4, 5].map((n) => (
                          <option key={n} value={n}>
                            {n}
                          </option>
                        ))}
                      </select>
                    </div>

                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>Max Tabs:</label>
                      <select
                        value={guiConfig.conversation_tabs_max ?? 6}
                        onChange={(e) =>
                          setGuiConfig({
                            ...guiConfig,
                            conversation_tabs_mode: 'dynamic',
                            conversation_tabs_max: parseInt(e.target.value, 10),
                          })
                        }
                        style={{
                          height: '30px',
                          padding: '0 8px',
                          borderRadius: '6px',
                          border: '1px solid var(--border)',
                          background: 'var(--surface, #ffffff)',
                          fontSize: '12px',
                          fontWeight: 600,
                          color: 'var(--text)',
                          cursor: 'pointer',
                          width: '100%',
                        }}
                      >
                        {[4, 5, 6, 7, 8, 9, 10].map((n) => (
                          <option key={n} value={n}>
                            {n}
                          </option>
                        ))}
                      </select>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            {/* VERTICAL DIVIDER */}
            <div style={{ width: '1px', backgroundColor: 'var(--border)', alignSelf: 'stretch' }} />

            {/* Right column: Interactive Micro-Interaction Preview */}
            <div
              style={{
                width: '360px',
                boxSizing: 'border-box',
                borderRadius: '8px',
                border: '1px solid var(--border)',
                background: 'var(--canvas)',
                padding: '16px',
                display: 'flex',
                flexDirection: 'column',
                gap: '8px',
              }}
            >
              <div style={{ fontSize: '12px', fontWeight: 700, color: 'var(--text-muted)', marginBottom: '4px' }}>
                Divider Visual Design & Micro-Interaction Preview
              </div>

              {/* Project Card */}
              <div
                style={{
                  background: previewProjColor,
                  color: projCardTextColor,
                  padding: '6px 12px',
                  borderRadius: '6px',
                  fontSize: '13px',
                  fontWeight: 600,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                }}
              >
                <span>{activeProjectName}</span>
                <ChevronDown size={12} strokeWidth={2} style={{ opacity: 0.85 }} />
              </div>

              {/* Visible Sample Rows */}
              {['Frontend Component Architecture', 'API Endpoint Optimization', 'Design Token Synchronization'].map((title, i) => (
                <div
                  key={title}
                  style={{
                    padding: '6px 10px',
                    borderRadius: '6px',
                    fontSize: '12px',
                    color: 'var(--text)',
                    background: `rgba(${parsedProjColor.r}, ${parsedProjColor.g}, ${parsedProjColor.b}, 0.08)`,
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                  }}
                >
                  <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{title}</span>
                  <span style={{ fontSize: '10px', color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: '2px' }}>
                    {i + 1}h
                    <ChevronRight size={10} />
                  </span>
                </div>
              ))}

              {previewExpanded &&
                ['Telemetry Pipeline Refactor', 'State Synchronization Hook'].map((title, i) => (
                  <div
                    key={title}
                    style={{
                      padding: '6px 10px',
                      borderRadius: '6px',
                      fontSize: '12px',
                      color: 'var(--text)',
                      background: `rgba(${parsedProjColor.r}, ${parsedProjColor.g}, ${parsedProjColor.b}, 0.08)`,
                      display: 'flex',
                      justifyContent: 'space-between',
                    }}
                  >
                    <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{title}</span>
                    <span style={{ fontSize: '10px', color: 'var(--text-muted)' }}>{i + 4}h</span>
                  </div>
                ))}

              {/* Centered Divider with Solid Triangle Above Continuous Line */}
              {guiConfig.replace_see_all_triangle !== false ? (
                <div
                  onClick={() => setPreviewExpanded(!previewExpanded)}
                  title={previewExpanded ? 'Show fewer conversations' : 'Show all 5 conversations (2 hidden)'}
                  style={{
                    position: 'relative',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    width: '100%',
                    height: '24px',
                    marginTop: '1px',
                    cursor: 'pointer',
                    userSelect: 'none',
                    padding: '0',
                    boxSizing: 'border-box',
                  }}
                >
                  <div
                    style={{
                      position: 'absolute',
                      top: '50%',
                      left: 0,
                      right: 0,
                      width: '100%',
                      height: '1px',
                      transform: 'translateY(-50%)',
                      background: 'rgba(148, 163, 184, 0.35)',
                      zIndex: 1,
                    }}
                  />
                  <div
                    style={{
                      position: 'absolute',
                      bottom: '50%',
                      left: '50%',
                      transform: 'translateX(-50%)',
                      marginBottom: '1px',
                      zIndex: 2,
                      display: 'inline-flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      width: '16px',
                      height: '11px',
                      color: 'var(--text-muted)',
                      fontSize: '8px',
                      transition: 'all 0.18s ease',
                    }}
                  >
                    {previewExpanded ? (
                      <ChevronUp size={12} strokeWidth={2} />
                    ) : (
                      <ChevronDown size={12} strokeWidth={2} />
                    )}
                  </div>
                </div>
              ) : (
                <div
                  onClick={() => setPreviewExpanded(!previewExpanded)}
                  style={{
                    padding: '4px 0',
                    fontSize: '11px',
                    fontWeight: 600,
                    color: previewProjColor,
                    cursor: 'pointer',
                    textAlign: 'left',
                  }}
                >
                  {previewExpanded ? 'See less' : 'See all 5'}
                </div>
              )}

              {/* Project 2: Uncontracted project demonstrating consistent bottom spacing */}
              <div
                style={{
                  background: secondaryProjColor,
                  color: secondaryCardTextColor,
                  padding: '6px 12px',
                  borderRadius: '6px',
                  fontSize: '13px',
                  fontWeight: 600,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  marginTop: '16px',
                }}
              >
                <span>{secondaryProjectName}</span>
                <ChevronDown size={12} strokeWidth={2} style={{ opacity: 0.85 }} />
              </div>
              <div
                style={{
                  padding: '6px 10px',
                  borderRadius: '6px',
                  fontSize: '12px',
                  color: 'var(--text)',
                  background: `rgba(${parsedSecondaryColor.r}, ${parsedSecondaryColor.g}, ${parsedSecondaryColor.b}, 0.08)`,
                  display: 'flex',
                  justifyContent: 'space-between',
                }}
              >
                <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>Active Experiment Runner</span>
                <span style={{ fontSize: '10px', color: 'var(--text-muted)' }}>20m</span>
              </div>
              {guiConfig.consistent_project_spacing !== false && (
                <div
                  style={{
                    position: 'relative',
                    height: '0px',
                    width: '100%',
                    boxSizing: 'border-box',
                  }}
                  title={
                    (guiConfig.consistent_project_spacing_line ?? true)
                      ? 'Consistent horizontal divider line centered in the existing project gap'
                      : 'Natural project gap below uncontracted project'
                  }
                >
                  {(guiConfig.consistent_project_spacing_line ?? true) && (
                    <div
                      style={{
                        position: 'absolute',
                        top: '6px',
                        left: 0,
                        right: 0,
                        height: '1px',
                        background: 'rgba(148, 163, 184, 0.35)',
                      }}
                    />
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Feature 7: Auto-Archive Inactive Conversations */}
      {guiConfig && (
        <div className="google-card" style={{ marginTop: '20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '16px' }}>
            <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
              Auto-Archive Inactive Conversations
            </h2>

            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
              <select
                value={guiConfig.auto_archive_horizon || '14d'}
                onChange={(e) =>
                  setGuiConfig({
                    ...guiConfig,
                    auto_archive_horizon: e.target.value as any,
                  })
                }
                title="Inactivity horizon — conversations inactive longer than this are archived"
                style={{
                  height: '36px',
                  padding: '0 12px',
                  borderRadius: '6px',
                  border: '1px solid var(--border)',
                  background: 'var(--card-bg, #ffffff)',
                  fontSize: '13px',
                  fontWeight: 600,
                  color: 'var(--text)',
                  cursor: 'pointer',
                  minWidth: '140px',
                }}
              >
                <option value="3d">3 Days</option>
                <option value="7d">7 Days</option>
                <option value="14d">14 Days</option>
                <option value="30d">30 Days</option>
                <option value="60d">60 Days</option>
                <option value="90d">90 Days</option>
              </select>

              <button
                onClick={handleTriggerAutoArchive}
                disabled={archiving}
                className="btn-pill-tonal"
                style={{
                  height: '36px',
                  border: '1px solid var(--border)',
                  backgroundColor: 'var(--tonal)',
                  borderRadius: '20px',
                  padding: '0 14px',
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '6px',
                  cursor: archiving ? 'not-allowed' : 'pointer',
                  whiteSpace: 'nowrap',
                  fontSize: '13px',
                  fontWeight: 600,
                }}
              >
                <Archive size={14} />
                <span>{archiving ? 'Archiving...' : 'Manual Archive Now'}</span>
              </button>

              <ToggleSwitch
                checked={guiConfig.auto_archive_conversations ?? true}
                onChange={(checked) =>
                  setGuiConfig({
                    ...guiConfig,
                    auto_archive_conversations: checked,
                  })
                }
              />
            </div>
          </div>

          {archiveResult && (
            <div style={{ marginTop: '12px', fontSize: '12px', color: '#059669', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '4px' }}>
              <CheckCircle2 size={13} />
              <span>{archiveResult}</span>
            </div>
          )}
        </div>
      )}

      {/* Feature 4: Predefined Default Project for New Conversations */}
      <div className="google-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '16px' }}>
          <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
            Default Project For New Conversations
          </h2>

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
                border: '1px solid var(--border)',
                background: 'var(--canvas)',
                fontSize: '13px',
                fontWeight: 600,
                color: 'var(--text)',
                cursor: 'pointer',
              }}
            >
              <option value="">Auto (Follow Active Project)</option>
              {projects.map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </select>
          </div>
        </div>
      </div>
    </>
  )}

  {/* Category 3: Overview Panel */}
  {activeCategoryTab === 2 && (
    <>
      {/* Feature Card: Auxiliary Extension Tab Switchers Format */}
      <div className="google-card" style={{ marginBottom: '20px' }}>
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            marginBottom: '16px',
          }}
        >
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
                Extension Tab Switchers Format
              </h2>
            </div>
          </div>
        </div>

        {/* 2-Option Selector Grid */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: '16px' }}>
          {/* Option 1: Just an Icon (Compact) */}
          <div
            onClick={() => handleAuxTabsFormatChange('icon')}
            style={{
              display: 'flex',
              flexDirection: 'column',
              gap: '12px',
              padding: '16px',
              borderRadius: '10px',
              border: `1px solid ${(op.aux_tabs_format || 'icon') === 'icon' ? 'var(--primary)' : 'var(--border)'}`,
              backgroundColor: (op.aux_tabs_format || 'icon') === 'icon' ? 'var(--accent-subtle, #eff6ff)' : 'var(--surface, #ffffff)',
              cursor: 'pointer',
              transition: 'all 0.15s ease',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 600, fontSize: '13px', color: 'var(--text)' }}>
                <span>Just An Icon (Compact)</span>
              </div>
              <input
                type="radio"
                name="aux_tabs_format"
                checked={(op.aux_tabs_format || 'icon') === 'icon'}
                onChange={() => handleAuxTabsFormatChange('icon')}
                style={{ accentColor: '#0b57d0', cursor: 'pointer' }}
              />
            </div>

            {/* Visual Demo of Icon Only Tabs */}
            <div
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '4px',
                padding: '6px 10px',
                backgroundColor: '#f1f5f9',
                borderRadius: '6px',
                width: 'fit-content',
                border: '1px solid var(--border)',
              }}
            >

              <div
                style={{
                  width: '24px',
                  height: '24px',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  borderRadius: '4px',
                  backgroundColor: '#ffffff',
                  border: '1px solid var(--border)',
                  color: 'var(--text)',
                }}
                title="Files"
              >
                <Folder size={14} />
              </div>
              <div
                style={{
                  width: '24px',
                  height: '24px',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  borderRadius: '4px',
                  backgroundColor: '#ffffff',
                  border: '1px solid var(--border)',
                  color: 'var(--text)',
                }}
                title="Memos"
              >
                <FileText size={14} />
              </div>
              <div
                style={{
                  width: '24px',
                  height: '24px',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  borderRadius: '4px',
                  backgroundColor: '#ffffff',
                  border: '1px solid var(--border)',
                  color: 'var(--text)',
                }}
                title="GitHub"
              >
                <GithubIcon size={14} />
              </div>
            </div>
          </div>

          {/* Option 2: Icon and Name */}
          <div
            onClick={() => handleAuxTabsFormatChange('icon_and_name')}
            style={{
              display: 'flex',
              flexDirection: 'column',
              gap: '12px',
              padding: '16px',
              borderRadius: '10px',
              border: `1px solid ${op.aux_tabs_format === 'icon_and_name' ? 'var(--primary)' : 'var(--border)'}`,
              backgroundColor: op.aux_tabs_format === 'icon_and_name' ? 'var(--accent-subtle, #eff6ff)' : 'var(--surface, #ffffff)',
              cursor: 'pointer',
              transition: 'all 0.15s ease',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 600, fontSize: '13px', color: 'var(--text)' }}>
                <span>Icon And Name</span>
              </div>
              <input
                type="radio"
                name="aux_tabs_format"
                checked={op.aux_tabs_format === 'icon_and_name'}
                onChange={() => handleAuxTabsFormatChange('icon_and_name')}
                style={{ accentColor: '#0b57d0', cursor: 'pointer' }}
              />
            </div>

            {/* Visual Demo of Icon and Name Tabs */}
            <div
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '4px',
                padding: '6px 10px',
                backgroundColor: '#f1f5f9',
                borderRadius: '6px',
                width: 'fit-content',
                border: '1px solid var(--border)',
              }}
            >

              <div
                style={{
                  height: '24px',
                  padding: '0 8px',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '5px',
                  borderRadius: '4px',
                  backgroundColor: '#ffffff',
                  border: '1px solid var(--border)',
                  color: 'var(--text)',
                  fontSize: '11px',
                  fontWeight: 500,
                }}
              >
                <Folder size={13} />
                <span>Files</span>
              </div>
              <div
                style={{
                  height: '24px',
                  padding: '0 8px',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '5px',
                  borderRadius: '4px',
                  backgroundColor: '#ffffff',
                  border: '1px solid var(--border)',
                  color: 'var(--text)',
                  fontSize: '11px',
                  fontWeight: 500,
                }}
              >
                <FileText size={13} />
                <span>Memos</span>
              </div>
              <div
                style={{
                  height: '24px',
                  padding: '0 8px',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '5px',
                  borderRadius: '4px',
                  backgroundColor: '#ffffff',
                  border: '1px solid var(--border)',
                  color: 'var(--text)',
                  fontSize: '11px',
                  fontWeight: 500,
                }}
              >
                <GithubIcon size={13} />
                <span>GitHub</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="google-card">
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            marginBottom: '16px',
          }}
        >
          <div>
            <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
              Overview Panel Customization
            </h2>
          </div>
        </div>

        <div>
          {/* Section Division Modes & Detailed Controls */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                {/* Switch 1: Replace See all / See less with refined divider */}
                <div
                  style={{
                    background: 'var(--canvas)',
                    borderRadius: '10px',
                    border: '1px solid var(--border)',
                    padding: '14px 16px',
                  }}
                >
                  <label
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      gap: '12px',
                      fontSize: '13px',
                      color: 'var(--text)',
                      cursor: 'pointer',
                    }}
                  >
                    <span>Refined Expand/Collapse Indicator</span>
                    <ToggleSwitch
                      size="sm"
                      checked={op.replace_see_all_triangle}
                      onChange={(checked) =>
                        setConfig({
                          ...config,
                          overview_panel: { ...op, replace_see_all_triangle: checked },
                        })
                      }
                    />
                  </label>
                </div>

                {/* Section Division Style Selection (2 Visual Option Cards) */}
                <div
                  style={{
                    background: 'var(--canvas)',
                    borderRadius: '10px',
                    border: '1px solid var(--border)',
                    padding: '16px 20px',
                  }}
                >
                  <div style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)', marginBottom: '14px' }}>
                    Section Division Style
                  </div>

                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                    {/* Option 1: Horizontal Divider Line */}
                    <div
                      onClick={() =>
                        setConfig({
                          ...config,
                          overview_panel: { ...op, division_style: 'divider_line' },
                        })
                      }
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        padding: '14px',
                        borderRadius: '8px',
                        border: `1px solid ${op.division_style === 'divider_line' ? 'var(--primary)' : 'var(--border)'}`,
                        backgroundColor: op.division_style === 'divider_line' ? 'var(--accent-subtle, #eff6ff)' : 'var(--surface, #ffffff)',
                        cursor: 'pointer',
                        transition: 'all 0.15s ease',
                      }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 600, fontSize: '13px', color: 'var(--text)' }}>
                        <Split size={16} color={op.division_style === 'divider_line' ? '#0b57d0' : '#64748b'} />
                        <span>Horizontal Divider Line</span>
                      </div>
                      <input
                        type="radio"
                        name="division_style"
                        checked={op.division_style === 'divider_line'}
                        onChange={() =>
                          setConfig({
                            ...config,
                            overview_panel: { ...op, division_style: 'divider_line' },
                          })
                        }
                        style={{ accentColor: '#0b57d0', cursor: 'pointer' }}
                      />
                    </div>

                    {/* Option 2: Border Zone Grouping (Whiter Contrast) */}
                    <div
                      onClick={() =>
                        setConfig({
                          ...config,
                          overview_panel: { ...op, division_style: 'border_zone' },
                        })
                      }
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        padding: '14px',
                        borderRadius: '8px',
                        border: `1px solid ${op.division_style === 'border_zone' ? 'var(--primary)' : 'var(--border)'}`,
                        backgroundColor: op.division_style === 'border_zone' ? 'var(--accent-subtle, #eff6ff)' : 'var(--surface, #ffffff)',
                        cursor: 'pointer',
                        transition: 'all 0.15s ease',
                      }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 600, fontSize: '13px', color: 'var(--text)' }}>
                        <Layers size={16} color={op.division_style === 'border_zone' ? '#0b57d0' : '#64748b'} />
                        <span>Border Zone (Whiter BG)</span>
                      </div>
                      <input
                        type="radio"
                        name="division_style"
                        checked={op.division_style === 'border_zone'}
                        onChange={() =>
                          setConfig({
                            ...config,
                            overview_panel: { ...op, division_style: 'border_zone' },
                          })
                        }
                        style={{ accentColor: '#0b57d0', cursor: 'pointer' }}
                      />
                    </div>
                  </div>
                </div>

                {/* Detailed Controls based on selected mode */}
                {op.division_style === 'border_zone' && (
                  /* Option 2 Settings: Border Zone Grouping, Background Contrast, Radius, Padding & Spacing */
                  <div
                    style={{
                      background: 'var(--canvas)',
                      borderRadius: '10px',
                      border: '1px solid var(--border)',
                      padding: '16px 20px',
                      display: 'flex',
                      flexDirection: 'column',
                      gap: '16px',
                    }}
                  >
                    <div style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text)' }}>
                      Border Zone Appearance & Contrast
                    </div>

                    {/* Background Contrast Selector */}
                    <div>
                      <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '8px' }}>
                        Zone Background Tone:
                      </label>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                        {[
                          { id: 'whiter', label: 'Whiter Contrast (#ffffff)' },
                          { id: 'subtle', label: 'Subtle Card Tint (#fcfdfd)' },
                          { id: 'card', label: 'Elevated Surface Card' },
                        ].map((tone) => (
                          <div
                            key={tone.id}
                            onClick={() => setConfig({ ...config, overview_panel: { ...op, zone_background_contrast: tone.id as any } })}
                            style={{
                              display: 'flex',
                              alignItems: 'center',
                              gap: '12px',
                              padding: '10px 12px',
                              borderRadius: '8px',
                              border: `1px solid ${op.zone_background_contrast === tone.id ? 'var(--primary)' : 'var(--border)'}`,
                              background: op.zone_background_contrast === tone.id ? 'var(--accent-subtle, #eff6ff)' : 'var(--surface, #ffffff)',
                              cursor: 'pointer',
                            }}
                          >
                            <input
                              type="radio"
                              name="zone_tone"
                              checked={op.zone_background_contrast === tone.id}
                              onChange={() => setConfig({ ...config, overview_panel: { ...op, zone_background_contrast: tone.id as any } })}
                              style={{ accentColor: '#0b57d0' }}
                            />
                            <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>{tone.label}</span>
                          </div>
                        ))}
                      </div>
                    </div>

                    {/* Radius and Gap row with vertical breaker line */}
                    <div
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        flexWrap: 'wrap',
                        gap: '0px',
                        padding: '12px 14px',
                        background: 'var(--surface, #ffffff)',
                        borderRadius: '8px',
                        border: '1px solid var(--border)',
                      }}
                    >
                      {/* Border Radius */}
                      <div style={{ flex: 1, minWidth: '180px', paddingRight: '8px' }}>
                        <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '8px' }}>
                          Border Radius: <strong>{op.zone_border_radius}px</strong>
                        </label>
                        <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                          {[6, 8, 10, 14].map((r) => (
                            <button
                              key={r}
                              type="button"
                              onClick={() => setConfig({ ...config, overview_panel: { ...op, zone_border_radius: r } })}
                              style={{
                                flex: 1,
                                padding: '5px 0',
                                borderRadius: '6px',
                                fontSize: '12px',
                                fontWeight: op.zone_border_radius === r ? 700 : 500,
                                color: op.zone_border_radius === r ? '#ffffff' : '#334155',
                                backgroundColor: op.zone_border_radius === r ? '#0b57d0' : '#f1f5f9',
                                border: '1px solid',
                                borderColor: op.zone_border_radius === r ? 'var(--primary)' : 'var(--border)',
                                cursor: 'pointer',
                              }}
                            >
                              {r}px
                            </button>
                          ))}
                        </div>
                      </div>

                      {/* Vertical Grey Breaker */}
                      <div style={{ width: '1px', height: '40px', backgroundColor: 'var(--border)', margin: '0' }} />

                      {/* Gap Between Zones */}
                      <div style={{ flex: 1, minWidth: '180px', paddingLeft: '8px' }}>
                        <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '8px' }}>
                          Zone Spacing (Gap): <strong>{op.zone_gap}px</strong>
                        </label>
                        <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                          {[6, 8, 10, 14].map((g) => (
                            <button
                              key={g}
                              type="button"
                              onClick={() => setConfig({ ...config, overview_panel: { ...op, zone_gap: g } })}
                              style={{
                                flex: 1,
                                padding: '5px 0',
                                borderRadius: '6px',
                                fontSize: '12px',
                                fontWeight: op.zone_gap === g ? 700 : 500,
                                color: op.zone_gap === g ? '#ffffff' : '#334155',
                                backgroundColor: op.zone_gap === g ? '#0b57d0' : '#f1f5f9',
                                border: '1px solid',
                                borderColor: op.zone_gap === g ? 'var(--primary)' : 'var(--border)',
                                cursor: 'pointer',
                              }}
                            >
                              {g}px
                            </button>
                          ))}
                        </div>
                      </div>
                    </div>

                    {/* Internal Padding & Border Color */}
                    <div
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        flexWrap: 'wrap',
                        gap: '0px',
                        padding: '12px 14px',
                        background: 'var(--surface, #ffffff)',
                        borderRadius: '8px',
                        border: '1px solid var(--border)',
                      }}
                    >
                      {/* Internal Padding */}
                      <div style={{ flex: 1, minWidth: '180px', paddingRight: '8px' }}>
                        <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '8px' }}>
                          Internal Padding: <strong>{op.zone_padding}px</strong>
                        </label>
                        <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                          {[8, 10, 12, 16].map((p) => (
                            <button
                              key={p}
                              type="button"
                              onClick={() => setConfig({ ...config, overview_panel: { ...op, zone_padding: p } })}
                              style={{
                                flex: 1,
                                padding: '5px 0',
                                borderRadius: '6px',
                                fontSize: '12px',
                                fontWeight: op.zone_padding === p ? 700 : 500,
                                color: op.zone_padding === p ? '#ffffff' : '#334155',
                                backgroundColor: op.zone_padding === p ? '#0b57d0' : '#f1f5f9',
                                border: '1px solid',
                                borderColor: op.zone_padding === p ? 'var(--primary)' : 'var(--border)',
                                cursor: 'pointer',
                              }}
                            >
                              {p}px
                            </button>
                          ))}
                        </div>
                      </div>

                      {/* Vertical Grey Breaker */}
                      <div style={{ width: '1px', height: '40px', backgroundColor: 'var(--border)', margin: '0' }} />

                      {/* Border Color */}
                      <div style={{ flex: 1, minWidth: '180px', paddingLeft: '8px' }}>
                        <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '8px' }}>
                          Border Color:
                        </label>
                        <div style={{ display: 'flex', gap: '6px', alignItems: 'center', flexWrap: 'wrap' }}>
                          {[
                            { label: 'Subtle', hex: '#e2e8f0' },
                            { label: 'Slate', hex: '#cbd5e1' },
                            { label: 'Blue Tint', hex: 'rgba(11,87,208,0.22)' },
                          ].map((col) => (
                            <button
                              key={col.hex}
                              type="button"
                              onClick={() => setConfig({ ...config, overview_panel: { ...op, zone_border_color: col.hex } })}
                              style={{
                                padding: '4px 8px',
                                borderRadius: '6px',
                                border: `1px solid ${op.zone_border_color === col.hex ? 'var(--primary)' : 'var(--border)'}`,
                                background: op.zone_border_color === col.hex ? 'var(--accent-subtle, #eff6ff)' : 'var(--surface, #ffffff)',
                                fontSize: '11px',
                                fontWeight: op.zone_border_color === col.hex ? 700 : 500,
                                cursor: 'pointer',
                              }}
                            >
                              {col.label}
                            </button>
                          ))}
                        </div>
                      </div>
                    </div>
                  </div>
                )}
              </div>
          </div>
      </div>
    </>
  )}
    </div>
  )
}

