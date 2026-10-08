import React, { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import {
  Zap,
  Save,
  Archive,
  Folder,
  MessageSquare,
  Plus,
  MoreVertical,
  Layers,
  Split,
  CheckCircle2,
  FileText,
  Image,
  Sliders,
  Globe,
  PanelLeft,
  LayoutGrid,
  Layout,
  PocketKnife,
  GitBranch,
  Maximize2,
  X,
} from 'lucide-react'
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
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [applying, setApplying] = useState(false)
  const [statusMsg, setStatusMsg] = useState<{ text: string; type: 'success' | 'error' } | null>(null)
  const [previewHover, setPreviewHover] = useState<number | null>(null)
  const [projects, setProjects] = useState<string[]>([])
  const [archiving, setArchiving] = useState(false)
  const [archiveResult, setArchiveResult] = useState<string | null>(null)
  const [previewExpanded, setPreviewExpanded] = useState(false)
  const [overviewFilesExpanded, setOverviewFilesExpanded] = useState(false)
  const [overviewUploadsExpanded, setOverviewUploadsExpanded] = useState(false)
  const [collapsedSections, setCollapsedSections] = useState<Record<string, boolean>>({
    tasks: true,
    terminals: true,
  })

  const baseTintOpacity = guiConfig?.tint_opacity ?? 0.15
  const previewProjColor = guiConfig?.project_colors?.['Antigravity Swiss Knife'] || '#0b57d0'
  const parsedProjColor = parseColorWithAlpha(previewProjColor)
  const projColorLum = (0.2126 * parsedProjColor.r + 0.7152 * parsedProjColor.g + 0.0722 * parsedProjColor.b) / 255.0
  const projCardTextColor = projColorLum > 0.6 ? '#0f172a' : '#ffffff'
  const projAccentTextColor = projColorLum > 0.6 ? '#334155' : previewProjColor
  const projColorMult = calculateColorMultiplier(parsedProjColor.r, parsedProjColor.g, parsedProjColor.b, parsedProjColor.alpha)
  const effectivePreviewOpacity = Number((baseTintOpacity * projColorMult).toFixed(3))
  const effectiveActiveOpacity = Number((Math.min(1.0, (baseTintOpacity + 0.16) * projColorMult)).toFixed(3))

  const toggleSection = (key: string) => {
    setCollapsedSections((prev) => ({
      ...prev,
      [key]: !prev[key],
    }))
  }
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
        setStatusMsg({ text: res.message, type: 'success' })
      } else {
        setArchiveResult(res.message || 'No stale conversations found')
      }
    } catch (err: any) {
      setStatusMsg({ text: 'Auto-archive failed: ' + err.message, type: 'error' })
    } finally {
      setArchiving(false)
    }
  }

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

  const handleLeftPanelEnabledChange = async (enabled: boolean) => {
    if (!config) return
    const updatedConfig: EnhancementsConfig = {
      ...config,
      left_panel_extensions_enabled: enabled,
    }
    setConfig(updatedConfig)
    try {
      localStorage.setItem('antigravity_swiss_left_panel_enabled', enabled ? 'true' : 'false')
      window.dispatchEvent(
        new CustomEvent('swiss-left-nav-config-updated', {
          detail: {
            enabled,
            mode: config.left_panel_extensions_mode || 'single',
            main_section_enabled: config.main_section_extensions_enabled !== false,
          },
        })
      )
      if (typeof (window as any).setupLeftNavTabs === 'function') {
        (window as any).setupLeftNavTabs()
      }
    } catch {}
    try {
      await api.updateEnhancements(updatedConfig)
    } catch (err: any) {
      console.error('Failed to auto-save left_panel_extensions_enabled:', err)
    }
  }

  const handleLeftPanelModeChange = async (mode: 'single' | 'individual') => {
    if (!config) return
    const updatedConfig: EnhancementsConfig = {
      ...config,
      left_panel_extensions_mode: mode,
    }
    setConfig(updatedConfig)
    try {
      localStorage.setItem('antigravity_swiss_left_panel_mode', mode)
      window.dispatchEvent(
        new CustomEvent('swiss-left-nav-config-updated', {
          detail: {
            enabled: config.left_panel_extensions_enabled !== false,
            mode,
            main_section_enabled: config.main_section_extensions_enabled !== false,
          },
        })
      )
      if (typeof (window as any).setupLeftNavTabs === 'function') {
        (window as any).setupLeftNavTabs()
      }
    } catch {}
    try {
      await api.updateEnhancements(updatedConfig)
    } catch (err: any) {
      console.error('Failed to auto-save left_panel_extensions_mode:', err)
    }
  }

  const handleMainSectionEnabledChange = async (enabled: boolean) => {
    if (!config) return
    const updatedConfig: EnhancementsConfig = {
      ...config,
      main_section_extensions_enabled: enabled,
    }
    setConfig(updatedConfig)
    try {
      localStorage.setItem('antigravity_swiss_main_section_enabled', enabled ? 'true' : 'false')
      window.dispatchEvent(
        new CustomEvent('swiss-left-nav-config-updated', {
          detail: {
            enabled: config.left_panel_extensions_enabled !== false,
            mode: config.left_panel_extensions_mode || 'single',
            main_section_enabled: enabled,
          },
        })
      )
      if (typeof (window as any).setupLeftNavTabs === 'function') {
        (window as any).setupLeftNavTabs()
      }
    } catch {}
    try {
      await api.updateEnhancements(updatedConfig)
    } catch (err: any) {
      console.error('Failed to auto-save main_section_extensions_enabled:', err)
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
  const leftPanelEnabled = config.left_panel_extensions_enabled !== false
  const leftPanelMode = config.left_panel_extensions_mode || 'single'
  const mainSectionEnabled = config.main_section_extensions_enabled !== false

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {portalTarget &&
        createPortal(
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
            background: statusMsg.type === 'success' ? 'var(--green-bg)' : '#fce8e6',
            color: statusMsg.type === 'success' ? 'var(--green)' : '#b3261e',
            border: `1px solid ${statusMsg.type === 'success' ? '#bbf7d0' : '#fecaca'}`,
          }}
        >
          {statusMsg.text}
        </div>
      )}

      {/* Category 1: Chat View */}
      {activeCategoryTab === 0 && (
        <>
          {/* Feature 1: Quick Prompt Jump Bar */}
          <div className="google-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
          <div>
            <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: '#1e293b' }}>
              Quick Prompt Jump Bar
            </h2>
            <p style={{ margin: '4px 0 0', fontSize: '13px', color: '#64748b' }}>
              Jump directly to any prompt turn from the margin. Automatically highlights the current turn while scrolling.
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
          <div style={{ borderTop: '1px solid var(--border, #e2e8f0)', paddingTop: '18px' }}>
            {/* Optional interaction setting: Pulse Highlight Target Prompt Card on Jump */}
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                background: '#f8fafc',
                borderRadius: '10px',
                border: '1px solid #e2e8f0',
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
                  color: '#334155',
                }}
              >
                <div>
                  <span style={{ fontWeight: 500 }}>Pulse Highlight Target Prompt Card on Jump</span>
                  <p style={{ margin: '2px 0 0', fontSize: '12px', color: '#64748b' }}>
                    Briefly highlights the target prompt card with an accent focus outline for 1.2s when jumping.
                  </p>
                </div>
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

            {/* 2-Column Layout: Settings on Left, Fixed Vertical Divider, Interactive Gutter Preview on Right */}
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'minmax(0, 1fr) 1px 360px',
                gap: '24px 12px',
                alignItems: 'stretch',
              }}
            >
              {/* LEFT COLUMN: Line Width, Line Thickness, and Color Mode Settings */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                {/* Line Dimensions Settings (Width & Thickness) */}
                <div
                  style={{
                    background: '#f8fafc',
                    borderRadius: '10px',
                    border: '1px solid #e2e8f0',
                    padding: '16px 20px',
                  }}
                >
                  <div style={{ fontSize: '13px', fontWeight: 700, color: '#1e293b', marginBottom: '8px' }}>
                    Line Geometry & Dimensions
                  </div>
                  <p style={{ margin: '0 0 14px', fontSize: '12px', color: '#64748b' }}>
                    Fine-tune dash line length and thickness for active and inactive prompts:
                  </p>

                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '16px' }}>
                    {/* Line Width */}
                    <div>
                      <label style={{ fontSize: '12px', fontWeight: 600, color: '#334155', display: 'block', marginBottom: '6px' }}>
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
                            border: '1px solid #cbd5e1',
                            fontSize: '12.5px',
                          }}
                        />
                        <span style={{ fontSize: '12px', color: '#94a3b8' }}>px (default: 14)</span>
                      </div>
                    </div>

                    {/* Active Line Thickness */}
                    <div>
                      <label style={{ fontSize: '12px', fontWeight: 600, color: '#334155', display: 'block', marginBottom: '6px' }}>
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
                            border: '1px solid #cbd5e1',
                            fontSize: '12.5px',
                          }}
                        />
                        <span style={{ fontSize: '12px', color: '#94a3b8' }}>px (default: 2.5)</span>
                      </div>
                    </div>

                    {/* Inactive Line Thickness */}
                    <div>
                      <label style={{ fontSize: '12px', fontWeight: 600, color: '#334155', display: 'block', marginBottom: '6px' }}>
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
                            border: '1px solid #cbd5e1',
                            fontSize: '12.5px',
                          }}
                        />
                        <span style={{ fontSize: '12px', color: '#94a3b8' }}>px (default: 1.5)</span>
                      </div>
                    </div>
                  </div>
                </div>

                {/* Color Mode Selection (User Request: removed default tag) */}
                <div
                  style={{
                    background: '#f8fafc',
                    borderRadius: '10px',
                    border: '1px solid #e2e8f0',
                    padding: '16px 20px',
                  }}
                >
                  <div style={{ fontSize: '13px', fontWeight: 700, color: '#1e293b', marginBottom: '8px' }}>
                    Active & Hover Line Color
                  </div>
                  <p style={{ margin: '0 0 12px', fontSize: '12px', color: '#64748b' }}>
                    Inactive lines remain subtle grey ({jb.inactive_thickness || 1.5}px). Choose the accent color for active indicator ({jb.dash_thickness || 2.5}px) and hover:
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
                      <span>Slate Grey</span>
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
                      <span>Match Project Color</span>
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
                      <span>Custom Color</span>
                    </label>
                  </div>

                  {/* Custom Color Palette */}
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
                      <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '12px', flexWrap: 'wrap' }}>
                        <span style={{ fontSize: '11px', fontWeight: 700, color: '#64748b', textTransform: 'uppercase' }}>
                          Presets:
                        </span>
                        <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
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
                                  border: isSel ? '2px solid #0f172a' : '2px solid transparent',
                                  boxShadow: isSel && p.hex === '#0f172a' ? 'inset 0 0 0 1.5px #ffffff' : 'none',
                                  transform: isSel ? 'scale(1.15)' : 'scale(1)',
                                  transition: 'transform 0.12s',
                                }}
                              />
                            )
                          })}
                        </div>

                        <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '8px' }}>
                          <span style={{ fontSize: '12px', color: '#64748b' }}>Hex:</span>
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
                              width: '80px',
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
                            gridTemplateColumns: 'repeat(10, 18px)',
                            gap: '3px',
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
                                  title={hex}
                                  style={{
                                    width: '18px',
                                    height: '18px',
                                    borderRadius: '2px',
                                    boxSizing: 'border-box',
                                    background: cellHsl,
                                    cursor: 'pointer',
                                    border: isSelected ? '2px solid #0f172a' : '1px solid rgba(0,0,0,0.08)',
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
              </div>

              {/* VERTICAL DIVIDER */}
              <div style={{ width: '1px', backgroundColor: 'var(--border, #e2e8f0)', alignSelf: 'stretch' }} />

              {/* RIGHT COLUMN: Interactive Gutter Live Preview Component */}
              <div
                style={{
                  width: '360px',
                  boxSizing: 'border-box',
                  background: '#f8fafc',
                  borderRadius: '12px',
                  border: '1px dashed #cbd5e1',
                  padding: '20px',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '16px',
                  position: 'sticky',
                  top: '20px',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <div style={{ fontSize: '13px', fontWeight: 700, color: '#1e293b' }}>Interactive Gutter Preview</div>
                  <span style={{ fontSize: '11px', color: '#64748b', backgroundColor: '#e2e8f0', padding: '2px 8px', borderRadius: '10px' }}>
                    Live Sandbox
                  </span>
                </div>

                <div
                  style={{
                    backgroundColor: '#ffffff',
                    borderRadius: '8px',
                    border: '1px solid #e2e8f0',
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
                            transition: 'height 0.15s, background 0.15s, transform 0.1s',
                            transform: isHovered ? 'scaleX(1.15)' : 'none',
                          }}
                        />
                      )
                    })}
                  </div>

                  <div style={{ fontSize: '12px', color: '#64748b', lineHeight: 1.5 }}>
                    Line #3 is active (<strong>{jb.dash_width || 14}px &times; {jb.dash_thickness || 2.5}px</strong> in{' '}
                    <span style={{ color: activeColor, fontWeight: 700 }}>{activeColor}</span>).
                    <br />
                    Hover lines to test accent hover. Inactive lines stay at <strong>{jb.inactive_thickness || 1.5}px</strong> grey.
                  </div>
                </div>
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
            Compact intermediate tool runs, commands, and thinking steps to keep chat answers clean and readable.
          </p>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '14px' }}>
          {[
            {
              id: 'muted',
              title: 'Greyed Out / Muted',
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
              Add a subtle horizontal divider above each user prompt to clearly separate turns.
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
    </>
  )}

  {/* Category 2: Project Panel */}
  {activeCategoryTab === 1 && (
    <>
      {/* Feature: Left Sidebar Extension Navigation */}
      <div className="google-card">
        {/* Toggle 1: Include in Left Sidebar */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '16px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <PanelLeft size={18} color="#0b57d0" />
              <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
                Left Sidebar Extension Navigation
              </h2>
            </div>
            <p style={{ margin: '4px 0 0', fontSize: '13px', color: 'var(--text-muted)', maxWidth: '640px', lineHeight: 1.5 }}>
              Show Swiss Knife extension tabs in Antigravity's left sidebar as a unified tab or individual extension buttons.
            </p>
          </div>

          <ToggleSwitch
            checked={leftPanelEnabled}
            onChange={handleLeftPanelEnabledChange}
          />
        </div>

        {/* Toggle 2: Open in Main Section */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '16px', marginTop: '16px', paddingTop: '16px', borderTop: '1px solid var(--border, #e2e8f0)' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Layout size={18} color="#0b57d0" />
              <h3 style={{ margin: 0, fontSize: '14.5px', fontWeight: 600, color: 'var(--text)' }}>
                Open Extension in Main Section
              </h3>
            </div>
            <p style={{ margin: '4px 0 0', fontSize: '13px', color: 'var(--text-muted)', maxWidth: '640px', lineHeight: 1.5 }}>
              Open extensions in the main chat stage rather than the right auxiliary panel.
            </p>
          </div>

          <ToggleSwitch
            checked={mainSectionEnabled}
            onChange={handleMainSectionEnabledChange}
          />
        </div>

        {leftPanelEnabled && (
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'minmax(0, 1fr) 1px 320px',
              gap: '24px 12px',
              alignItems: 'stretch',
              marginTop: '18px',
            }}
          >
            {/* Left Column: Mode Selection Options */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
              <label style={{ fontSize: '13px', fontWeight: 600, color: '#1e293b' }}>
                Sidebar Navigation Mode
              </label>

              {/* Option 1: Single Tab Button */}
              <div
                onClick={() => handleLeftPanelModeChange('single')}
                style={{
                  display: 'flex',
                  alignItems: 'flex-start',
                  gap: '12px',
                  padding: '14px 16px',
                  borderRadius: '8px',
                  border: `1.5px solid ${leftPanelMode === 'single' ? '#0b57d0' : '#e2e8f0'}`,
                  backgroundColor: leftPanelMode === 'single' ? '#eff6ff' : '#ffffff',
                  cursor: 'pointer',
                  transition: 'all 0.15s ease',
                }}
              >
                <div
                  style={{
                    width: '18px',
                    height: '18px',
                    borderRadius: '50%',
                    border: `2px solid ${leftPanelMode === 'single' ? '#0b57d0' : '#94a3b8'}`,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    marginTop: '2px',
                    flexShrink: 0,
                  }}
                >
                  {leftPanelMode === 'single' && (
                    <div style={{ width: '8px', height: '8px', borderRadius: '50%', backgroundColor: '#0b57d0' }} />
                  )}
                </div>
                <div style={{ flex: 1 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '2px' }}>
                    <PocketKnife size={14} color={leftPanelMode === 'single' ? '#0b57d0' : '#64748b'} />
                    <span style={{ fontSize: '13.5px', fontWeight: 600, color: leftPanelMode === 'single' ? '#1e3a8a' : '#1e293b' }}>
                      Single Tab Button (Swiss Knife)
                    </span>
                    <span
                      style={{
                        fontSize: '11px',
                        padding: '1px 6px',
                        borderRadius: '4px',
                        backgroundColor: leftPanelMode === 'single' ? '#dbeafe' : '#f1f5f9',
                        color: leftPanelMode === 'single' ? '#1d4ed8' : '#64748b',
                        fontWeight: 500,
                      }}
                    >
                      Compact
                    </span>
                  </div>
                  <p style={{ margin: 0, fontSize: '12.5px', color: '#64748b', lineHeight: 1.4 }}>
                    Single compact Swiss Knife tab in the sidebar. Keeps navigation minimal.
                  </p>
                </div>
              </div>

              {/* Option 2: Individual Extension Tabs */}
              <div
                onClick={() => handleLeftPanelModeChange('individual')}
                style={{
                  display: 'flex',
                  alignItems: 'flex-start',
                  gap: '12px',
                  padding: '14px 16px',
                  borderRadius: '8px',
                  border: `1.5px solid ${leftPanelMode === 'individual' ? '#0b57d0' : '#e2e8f0'}`,
                  backgroundColor: leftPanelMode === 'individual' ? '#eff6ff' : '#ffffff',
                  cursor: 'pointer',
                  transition: 'all 0.15s ease',
                }}
              >
                <div
                  style={{
                    width: '18px',
                    height: '18px',
                    borderRadius: '50%',
                    border: `2px solid ${leftPanelMode === 'individual' ? '#0b57d0' : '#94a3b8'}`,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    marginTop: '2px',
                    flexShrink: 0,
                  }}
                >
                  {leftPanelMode === 'individual' && (
                    <div style={{ width: '8px', height: '8px', borderRadius: '50%', backgroundColor: '#0b57d0' }} />
                  )}
                </div>
                <div style={{ flex: 1 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '2px' }}>
                    <LayoutGrid size={14} color={leftPanelMode === 'individual' ? '#0b57d0' : '#64748b'} />
                    <span style={{ fontSize: '13.5px', fontWeight: 600, color: leftPanelMode === 'individual' ? '#1e3a8a' : '#1e293b' }}>
                      Individual Extension Tabs
                    </span>
                    <span
                      style={{
                        fontSize: '11px',
                        padding: '1px 6px',
                        borderRadius: '4px',
                        backgroundColor: leftPanelMode === 'individual' ? '#dbeafe' : '#f1f5f9',
                        color: leftPanelMode === 'individual' ? '#1d4ed8' : '#64748b',
                        fontWeight: 500,
                      }}
                    >
                      Direct Access
                    </span>
                  </div>
                  <p style={{ margin: 0, fontSize: '12.5px', color: '#64748b', lineHeight: 1.4 }}>
                    Individual tabs for Browser, Files, Memos, and GitHub matching Antigravity native sidebar items.
                  </p>
                </div>
              </div>

              {/* Informational Note */}
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '8px',
                  padding: '8px 12px',
                  borderRadius: '6px',
                  backgroundColor: '#f8fafc',
                  border: '1px solid #e2e8f0',
                  fontSize: '12px',
                  color: '#64748b',
                }}
              >
                <span>Proportions: Tab buttons match factory buttons (16px optical icon symbol, 13-14px font size, weight 400, 32px height). Universal Breaker Line Rule: Breaker lines do not expand the gap between tab buttons or sections (identical distance as if no breaker line was added).</span>
              </div>
            </div>

            {/* Vertical Breaker */}
            <div style={{ width: '1px', backgroundColor: 'var(--border, #e2e8f0)', alignSelf: 'stretch', margin: '0' }} />

            {/* Right Column: Sidebar Live Preview */}
            <div
              style={{
                width: '320px',
                boxSizing: 'border-box',
                background: '#f8fafc',
                border: '1px solid #e2e8f0',
                borderRadius: '8px',
                padding: '14px',
                display: 'flex',
                flexDirection: 'column',
                gap: '8px',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '4px' }}>
                <span style={{ fontSize: '11px', fontWeight: 700, color: '#64748b', letterSpacing: '0.5px', textTransform: 'uppercase' }}>
                  Left Sidebar Live Preview
                </span>
                <span style={{ fontSize: '11px', color: '#94a3b8' }}>Antigravity 2.21</span>
              </div>

              {/* Simulated Left Sidebar Container */}
              <div
                style={{
                  backgroundColor: '#ffffff',
                  border: '1px solid #cbd5e1',
                  borderRadius: '6px',
                  padding: '8px',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '6px',
                }}
              >
                {/* Factory Button: New Conversation */}
                <div
                  style={{
                    height: '32px',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '6px',
                    padding: '0 8px',
                    borderRadius: '8px',
                    backgroundColor: 'rgba(0,0,0,0.03)',
                    color: '#334155',
                    fontSize: '13px',
                    fontWeight: 400,
                  }}
                >
                  <Plus size={16} strokeWidth={1.6} color="#64748b" />
                  <span>New conversation</span>
                </div>

                {/* Factory Button: Automations */}
                <div
                  style={{
                    height: '32px',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '6px',
                    padding: '0 8px',
                    borderRadius: '8px',
                    color: '#475569',
                    fontSize: '13px',
                    fontWeight: 400,
                  }}
                >
                  <Zap size={16} strokeWidth={1.6} color="#64748b" />
                  <span>Automations</span>
                </div>

                {/* Breaker Line - Zero gap expansion (margin: -2.5px 0 preserves exact 6px button gap) */}
                <div style={{ height: '1px', backgroundColor: '#e2e8f0', margin: '-2.5px 0' }} />

                {/* Injected Swiss Nav Tabs */}
                {leftPanelMode === 'single' ? (
                  <div
                    style={{
                      height: '32px',
                      display: 'flex',
                      alignItems: 'center',
                      gap: '6px',
                      padding: '0 8px',
                      borderRadius: '8px',
                      backgroundColor: '#eff6ff',
                      color: '#0b57d0',
                      fontSize: '13px',
                      fontWeight: 500,
                      border: '1px solid #bfdbfe',
                    }}
                  >
                    <PocketKnife size={16} strokeWidth={1.6} color="#0b57d0" />
                    <span style={{ lineHeight: 1.2 }}>Swiss Knife</span>
                    <span style={{ marginLeft: 'auto', fontSize: '10px', color: '#2563eb', backgroundColor: '#dbeafe', padding: '1px 5px', borderRadius: '3px', fontWeight: 500 }}>
                      {mainSectionEnabled ? 'Main Section' : 'Aux Panel'}
                    </span>
                  </div>
                ) : (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                    <div
                      style={{
                        height: '32px',
                        display: 'flex',
                        alignItems: 'center',
                        gap: '6px',
                        padding: '0 8px',
                        borderRadius: '8px',
                        backgroundColor: '#eff6ff',
                        color: '#0b57d0',
                        fontSize: '13px',
                        fontWeight: 500,
                      }}
                    >
                      <Globe size={16} strokeWidth={1.6} color="#0b57d0" />
                      <span style={{ lineHeight: 1.2 }}>Preview Browser</span>
                    </div>
                    <div
                      style={{
                        height: '32px',
                        display: 'flex',
                        alignItems: 'center',
                        gap: '6px',
                        padding: '0 8px',
                        borderRadius: '8px',
                        color: '#475569',
                        fontSize: '13px',
                        fontWeight: 400,
                      }}
                    >
                      <Folder size={16} strokeWidth={1.6} color="#64748b" />
                      <span style={{ lineHeight: 1.2 }}>File Explorer</span>
                    </div>
                    <div
                      style={{
                        height: '32px',
                        display: 'flex',
                        alignItems: 'center',
                        gap: '6px',
                        padding: '0 8px',
                        borderRadius: '8px',
                        color: '#475569',
                        fontSize: '13px',
                        fontWeight: 400,
                      }}
                    >
                      <FileText size={16} strokeWidth={1.6} color="#64748b" />
                      <span style={{ lineHeight: 1.2 }}>Quick Memos</span>
                    </div>
                    <div
                      style={{
                        height: '32px',
                        display: 'flex',
                        alignItems: 'center',
                        gap: '6px',
                        padding: '0 8px',
                        borderRadius: '8px',
                        color: '#475569',
                        fontSize: '13px',
                        fontWeight: 400,
                      }}
                    >
                      <GitBranch size={16} strokeWidth={1.6} color="#64748b" />
                      <span style={{ lineHeight: 1.2 }}>GitHub Workspace</span>
                    </div>
                  </div>
                )}

                {/* Breaker Line - Zero gap expansion (margin: -2.5px 0 preserves exact 6px gap) */}
                <div style={{ height: '1px', backgroundColor: '#e2e8f0', margin: '-2.5px 0' }} />

                {/* Simulated Project Header */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '2px 6px', fontSize: '11px', color: '#94a3b8' }}>
                  <span>Projects</span>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '3px 8px', fontSize: '12px', color: '#475569' }}>
                  <span>▾ Antigravity Swiss Knife</span>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Feature 4: Predefined Default Project for New Conversations */}
      <div className="google-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '16px' }}>
          <div>
            <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
              Default Project for New Conversations
            </h2>
            <p style={{ margin: '4px 0 0', fontSize: '13px', color: 'var(--text-muted)', maxWidth: '600px', lineHeight: 1.5 }}>
              Set a default project for new chats (Ctrl+N / Cmd+N) instead of the current active project.
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

      {/* Feature 5: Project Colors & Active Conversation Tab Indicator */}
      {guiConfig && (
        <div className="google-card">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '20px' }}>
            <div>
              <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
                Project Colors & Active Conversation Indicator
              </h2>
              <p style={{ margin: '4px 0 0', fontSize: '13px', color: 'var(--text-muted)' }}>
                Set project accent colors and sidebar conversation highlight styles.
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
            <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) 1px 360px', gap: '24px 12px', alignItems: 'stretch', marginTop: '16px' }}>
              {/* Settings Controls */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: '18px' }}>
                {/* Mode Selection */}
                <div>
                  <label style={{ display: 'block', fontSize: '13px', fontWeight: 600, color: '#1e293b', marginBottom: '8px' }}>
                    Open Conversation Highlight Mode:
                  </label>
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
                        alignItems: 'flex-start',
                        gap: '12px',
                        padding: '12px 14px',
                        borderRadius: '8px',
                        border: `1.5px solid ${
                          guiConfig.active_conversation_indicator === 'background' ? '#0b57d0' : '#e2e8f0'
                        }`,
                        background:
                          guiConfig.active_conversation_indicator === 'background' ? '#eff6ff' : '#f8fafc',
                        cursor: 'pointer',
                        transition: 'all 0.15s',
                      }}
                    >
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
                        style={{ marginTop: '2px', accentColor: '#0b57d0', cursor: 'pointer' }}
                      />
                      <div style={{ flex: 1 }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                          <span style={{ fontSize: '13px', fontWeight: 600, color: '#1e293b' }}>
                            Accent Background / Fill
                          </span>
                        </div>
                        <div style={{ fontSize: '12px', color: '#64748b', marginTop: '2px', lineHeight: 1.4 }}>
                          Fills the active conversation tab with a subtle project accent background tint.
                        </div>
                      </div>
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
                      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '8px' }}>
                        <div>
                          <div style={{ fontSize: '13px', fontWeight: 600, color: '#1e293b' }}>
                            Border Outline
                          </div>
                          <div style={{ fontSize: '12px', color: '#64748b', marginTop: '2px', lineHeight: 1.4 }}>
                            Outlines the active tab with an accent border while preserving the normal tab background.
                          </div>
                        </div>
                        {guiConfig.active_conversation_indicator === 'border' && (
                          <div
                            style={{ display: 'flex', alignItems: 'center', gap: '6px' }}
                            onClick={(e) => e.stopPropagation()}
                          >
                            <span style={{ fontSize: '12px', color: '#475569', fontWeight: 500 }}>Width:</span>
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
                                border: '1.5px solid #cbd5e1',
                                background: '#ffffff',
                                fontSize: '12px',
                                fontWeight: 600,
                                color: '#1e293b',
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
                        alignItems: 'flex-start',
                        gap: '12px',
                        padding: '12px 14px',
                        borderRadius: '8px',
                        border: `1.5px solid ${
                          guiConfig.active_conversation_indicator === 'left_bar' ? '#0b57d0' : '#e2e8f0'
                        }`,
                        background:
                          guiConfig.active_conversation_indicator === 'left_bar' ? '#eff6ff' : '#f8fafc',
                        cursor: 'pointer',
                        transition: 'all 0.15s',
                      }}
                    >
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
                        style={{ marginTop: '2px', accentColor: '#0b57d0', cursor: 'pointer' }}
                      />
                      <div style={{ flex: 1 }}>
                        <div style={{ fontSize: '13px', fontWeight: 600, color: '#1e293b' }}>
                          Left Accent Bar
                        </div>
                        <div style={{ fontSize: '12px', color: '#64748b', marginTop: '2px', lineHeight: 1.4 }}>
                          Highlights the active conversation tab with a distinct 3px colored bar along its left edge.
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                {/* Bold text option with unified divider line */}
                <div style={{ paddingTop: '10px', borderTop: '1px solid var(--border, #e2e8f0)' }}>
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
                      checked={guiConfig.active_conversation_bold ?? false}
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

                {/* Opacity slider with unified divider line */}
                <div style={{ paddingTop: '10px', borderTop: '1px solid var(--border, #e2e8f0)' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
                    <label style={{ fontSize: '13px', fontWeight: 500, color: '#1e293b' }}>
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
              <div style={{ width: '1px', backgroundColor: 'var(--border, #e2e8f0)', alignSelf: 'stretch' }} />

              {/* Real-time Interactive Preview */}
              <div
                style={{
                  width: '360px',
                  boxSizing: 'border-box',
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
                        Antigravity Swiss Knife
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
                      color: '#0f172a',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      boxSizing: 'border-box',
                      transition: 'all 0.15s ease',
                      userSelect: 'none',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '7px', minWidth: 0, overflow: 'hidden' }}>
                      <MessageSquare size={13} style={{ color: projAccentTextColor, flexShrink: 0 }} />
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
                      color: '#475569',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      boxSizing: 'border-box',
                      userSelect: 'none',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '7px', minWidth: 0, overflow: 'hidden' }}>
                      <MessageSquare size={13} style={{ color: '#94a3b8', flexShrink: 0 }} />
                      <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        Matching Font and UI ...
                      </span>
                    </div>
                    <span style={{ fontSize: '11px', color: '#64748b', flexShrink: 0 }}>6h</span>
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
                      color: '#475569',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      boxSizing: 'border-box',
                      userSelect: 'none',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '7px', minWidth: 0, overflow: 'hidden' }}>
                      <MessageSquare size={13} style={{ color: '#94a3b8', flexShrink: 0 }} />
                      <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        Antigravity Manager Pl...
                      </span>
                    </div>
                    <span style={{ fontSize: '11px', color: '#64748b', flexShrink: 0 }}>1d</span>
                  </div>
                </div>

                <div style={{ fontSize: '11px', color: '#64748b', textAlign: 'center', lineHeight: 1.4 }}>
                  {guiConfig.active_conversation_indicator === 'border' ? (
                    <span>
                      Active tab outlined with <strong>{guiConfig.active_conversation_border_width || '2px'} accent border</strong>
                    </span>
                  ) : guiConfig.active_conversation_indicator === 'left_bar' ? (
                    <span>
                      Active tab marked with <strong>3px left accent bar</strong>
                    </span>
                  ) : (
                    <span>
                      Active tab highlighted with <strong>accent background fill</strong>
                    </span>
                  )}
                  {(guiConfig.active_conversation_bold ?? false) ? ' (bold title)' : ' (regular title)'}
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
            <p style={{ margin: '4px 0 0', fontSize: '13px', color: 'var(--text-muted)' }}>
              Limit visible conversation tabs per project and replace text buttons with minimal dividers.
            </p>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) 1px 360px', gap: '24px 12px', alignItems: 'stretch' }}>
            {/* Left column: Controls stacked vertically */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
              {/* Simplicity Replacement Zone */}
              <div
                style={{
                  padding: '14px 16px',
                  borderRadius: '8px',
                  border: '1px solid var(--border, #e2e8f0)',
                  background: 'var(--card-bg, #ffffff)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: '16px',
                }}
              >
                <div>
                  <div style={{ fontSize: '14px', fontWeight: 700, color: '#1e293b' }}>
                    Simplicity Replacement for "See all" / "See less"
                  </div>
                  <p style={{ margin: '3px 0 0', fontSize: '12px', color: '#64748b', lineHeight: 1.4 }}>
                    Replace raw text buttons with a sleek 1px divider and centered solid triangle (▾ / ▴).
                  </p>
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

              {/* Divider Separation Below All Projects Zone */}
              <div
                style={{
                  padding: '14px 16px',
                  borderRadius: '8px',
                  border: '1px solid var(--border, #e2e8f0)',
                  background: 'var(--card-bg, #ffffff)',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '12px',
                }}
              >
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    gap: '16px',
                  }}
                >
                  <div>
                    <div style={{ fontSize: '14px', fontWeight: 700, color: '#1e293b' }}>
                      Divider Separation Below All Projects
                    </div>
                    <p style={{ margin: '3px 0 0', fontSize: '12px', color: '#64748b', lineHeight: 1.4 }}>
                      Add horizontal divider lines in the natural gap below projects without contracted conversation tabs for balanced, consistent project separation.
                    </p>
                  </div>
                  <ToggleSwitch
                    checked={guiConfig.consistent_project_spacing ?? true}
                    onChange={(checked) =>
                      setGuiConfig({
                        ...guiConfig,
                        consistent_project_spacing: checked,
                      })
                    }
                  />
                </div>

                {/* Sub-option: Horizontal Line at Middle of Project Gap */}
                {(guiConfig.consistent_project_spacing ?? true) && (
                  <div
                    style={{
                      paddingTop: '10px',
                      borderTop: '1px solid var(--border, #f1f5f9)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      gap: '16px',
                    }}
                  >
                    <div>
                      <div style={{ fontSize: '13px', fontWeight: 600, color: '#334155' }}>
                        Horizontal Line at Middle of Project Gap
                      </div>
                      <p style={{ margin: '2px 0 0', fontSize: '12px', color: '#64748b', lineHeight: 1.4 }}>
                        Add a centered 1px horizontal divider line in the middle of the existing project gap.
                      </p>
                    </div>
                    <ToggleSwitch
                      size="sm"
                      checked={guiConfig.consistent_project_spacing_line ?? true}
                      onChange={(checked) =>
                        setGuiConfig({
                          ...guiConfig,
                          consistent_project_spacing_line: checked,
                        })
                      }
                    />
                  </div>
                )}
              </div>

              {/* Horizontal Divider Line between Simplicity Zone and Fixed Number Zone */}
              <div style={{ height: '1px', backgroundColor: 'var(--border, #e2e8f0)', margin: '2px 0' }} />

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
                  border: `1.5px solid ${guiConfig.conversation_tabs_mode === 'fixed' ? '#0b57d0' : '#e2e8f0'}`,
                  background: guiConfig.conversation_tabs_mode === 'fixed' ? '#f0f7ff' : 'var(--card-bg, #ffffff)',
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
                  <span style={{ fontSize: '14px', fontWeight: 700, color: '#1e293b' }}>
                    Fixed Number
                  </span>
                </div>
                <p style={{ margin: '0 0 12px', fontSize: '12px', color: '#64748b', lineHeight: 1.4 }}>
                  Show a constant number of conversation tabs under each project before showing the expand divider.
                </p>

                <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }} onClick={(e) => e.stopPropagation()}>
                  <label style={{ fontSize: '12px', fontWeight: 600, color: '#334155' }}>Tabs per project:</label>
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
                      border: '1.5px solid #cbd5e1',
                      background: '#ffffff',
                      fontSize: '13px',
                      fontWeight: 600,
                      color: '#0f172a',
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
                  border: `1.5px solid ${guiConfig.conversation_tabs_mode !== 'fixed' ? '#0b57d0' : '#e2e8f0'}`,
                  background: guiConfig.conversation_tabs_mode !== 'fixed' ? '#f0f7ff' : 'var(--card-bg, #ffffff)',
                  cursor: 'pointer',
                  transition: 'border-color 0.15s, background 0.15s',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px' }}>
                  <input
                    type="radio"
                    name="convo_tabs_mode"
                    checked={guiConfig.conversation_tabs_mode !== 'fixed'}
                    onChange={() => {}}
                    style={{ accentColor: '#0b57d0' }}
                  />
                  <span style={{ fontSize: '14px', fontWeight: 700, color: '#1e293b' }}>Dynamic (By Chat Recency)</span>
                </div>
                <p style={{ margin: '0 0 12px', fontSize: '12px', color: '#64748b', lineHeight: 1.4 }}>
                  Dynamically adjust visible tabs per project based on last active timestamp, bounded between min and max.
                </p>

                <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }} onClick={(e) => e.stopPropagation()}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '10px' }}>
                    <label style={{ fontSize: '12px', fontWeight: 600, color: '#334155' }}>Active within:</label>
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
                        border: '1.5px solid #cbd5e1',
                        background: '#ffffff',
                        fontSize: '13px',
                        fontWeight: 600,
                        color: '#0f172a',
                        cursor: 'pointer',
                      }}
                    >
                      <option value="1d">1 day</option>
                      <option value="3d">3 days</option>
                      <option value="7d">7 days</option>
                      <option value="14d">14 days</option>
                      <option value="30d">30 days</option>
                    </select>
                  </div>

                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <label style={{ fontSize: '12px', fontWeight: 600, color: '#334155' }}>Min tabs:</label>
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
                          border: '1.5px solid #cbd5e1',
                          background: '#ffffff',
                          fontSize: '12px',
                          fontWeight: 600,
                          color: '#0f172a',
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
                      <label style={{ fontSize: '12px', fontWeight: 600, color: '#334155' }}>Max tabs:</label>
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
                          border: '1.5px solid #cbd5e1',
                          background: '#ffffff',
                          fontSize: '12px',
                          fontWeight: 600,
                          color: '#0f172a',
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
            <div style={{ width: '1px', backgroundColor: 'var(--border, #e2e8f0)', alignSelf: 'stretch' }} />

            {/* Right column: Interactive Micro-Interaction Preview */}
            <div
              style={{
                width: '360px',
                boxSizing: 'border-box',
                borderRadius: '8px',
                border: '1px solid var(--border, #e2e8f0)',
                background: '#f8fafc',
                padding: '16px',
                display: 'flex',
                flexDirection: 'column',
                gap: '8px',
              }}
            >
              <div style={{ fontSize: '12px', fontWeight: 700, color: '#475569', marginBottom: '4px' }}>
                Divider Visual Design & Micro-Interaction Preview
              </div>

              {/* Project Card */}
              <div
                style={{
                  background: '#2563eb',
                  color: '#ffffff',
                  padding: '6px 12px',
                  borderRadius: '6px',
                  fontSize: '13px',
                  fontWeight: 600,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                }}
              >
                <span>Demo Workspace</span>
                <span style={{ fontSize: '11px', opacity: 0.85 }}>▾</span>
              </div>

              {/* Visible Sample Rows */}
              {['Frontend Component Architecture', 'API Endpoint Optimization', 'Design Token Synchronization'].map((title, i) => (
                <div
                  key={title}
                  style={{
                    padding: '6px 10px',
                    borderRadius: '6px',
                    fontSize: '12px',
                    color: '#334155',
                    background: 'rgba(37, 99, 235, 0.08)',
                    display: 'flex',
                    justifyContent: 'space-between',
                  }}
                >
                  <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{title}</span>
                  <span style={{ fontSize: '10px', color: '#64748b' }}>{i + 1}h</span>
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
                      color: '#334155',
                      background: 'rgba(37, 99, 235, 0.08)',
                      display: 'flex',
                      justifyContent: 'space-between',
                    }}
                  >
                    <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{title}</span>
                    <span style={{ fontSize: '10px', color: '#64748b' }}>{i + 4}h</span>
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
                      color: '#64748b',
                      fontSize: '8px',
                      transition: 'all 0.18s ease',
                    }}
                  >
                    <span
                      style={{
                        display: 'inline-block',
                        transform: previewExpanded ? 'rotate(180deg)' : 'rotate(0deg)',
                        transition: 'transform 0.2s cubic-bezier(0.4, 0, 0.2, 1)',
                      }}
                    >
                      ▼
                    </span>
                  </div>
                </div>
              ) : (
                <div
                  onClick={() => setPreviewExpanded(!previewExpanded)}
                  style={{
                    padding: '4px 0',
                    fontSize: '11px',
                    fontWeight: 600,
                    color: '#2563eb',
                    cursor: 'pointer',
                    textAlign: 'left',
                  }}
                >
                  {previewExpanded ? 'See less' : 'See all 5'}
                </div>
              )}

              <div style={{ fontSize: '11px', color: '#64748b', textAlign: 'center' }}>
                {previewExpanded ? '▲ Expanded (click to collapse)' : '▼ Collapsed: 2 hidden tabs (click to expand)'}
              </div>

              {/* Project 2: Uncontracted project demonstrating consistent bottom spacing */}
              <div
                style={{
                  background: '#059669',
                  color: '#ffffff',
                  padding: '6px 12px',
                  borderRadius: '6px',
                  fontSize: '13px',
                  fontWeight: 600,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  marginTop: '14px',
                }}
              >
                <span>Secondary Project (Uncontracted)</span>
                <span style={{ fontSize: '11px', opacity: 0.85 }}>▾</span>
              </div>
              <div
                style={{
                  padding: '6px 10px',
                  borderRadius: '6px',
                  fontSize: '12px',
                  color: '#334155',
                  background: 'rgba(5, 150, 105, 0.08)',
                  display: 'flex',
                  justifyContent: 'space-between',
                }}
              >
                <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>Active Experiment Runner</span>
                <span style={{ fontSize: '10px', color: '#64748b' }}>20m</span>
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
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '16px' }}>
            <div>
              <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
                Auto-Archive Inactive Conversations
              </h2>
              <p style={{ margin: '4px 0 0', fontSize: '13px', color: 'var(--text-muted)', maxWidth: '600px' }}>
                Archive inactive project conversations to history after a set duration.
              </p>
            </div>

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

          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '12px' }}>
              <div>
                <label style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                  Inactivity Time Horizon Cutoff
                </label>
                <p style={{ margin: '2px 0 0', fontSize: '12px', color: 'var(--text-muted)' }}>
                  Conversations inactive for longer than this duration will be archived.
                </p>
              </div>

              <select
                value={guiConfig.auto_archive_horizon || '14d'}
                onChange={(e) =>
                  setGuiConfig({
                    ...guiConfig,
                    auto_archive_horizon: e.target.value as any,
                  })
                }
                style={{
                  height: '36px',
                  padding: '0 12px',
                  borderRadius: '6px',
                  border: '1.5px solid #cbd5e1',
                  background: 'var(--card-bg, #ffffff)',
                  fontSize: '13px',
                  fontWeight: 600,
                  color: 'var(--text)',
                  cursor: 'pointer',
                  minWidth: '180px',
                }}
              >
                <option value="3d">3 days</option>
                <option value="7d">7 days</option>
                <option value="14d">14 days</option>
                <option value="30d">30 days</option>
                <option value="60d">60 days</option>
                <option value="90d">90 days</option>
              </select>
            </div>

            <div style={{ borderTop: '1px solid #f1f5f9', paddingTop: '16px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '12px' }}>
              <div>
                <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                  Manual Archival Trigger
                </div>
                <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                  Scan conversation database now and archive conversations older than {guiConfig.auto_archive_horizon || '14d'}.
                </div>
                {archiveResult && (
                  <div style={{ marginTop: '4px', fontSize: '12px', color: '#059669', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '4px' }}>
                    <CheckCircle2 size={13} />
                    <span>{archiveResult}</span>
                  </div>
                )}
              </div>

              <button
                onClick={handleTriggerAutoArchive}
                disabled={archiving}
                className="google-button google-button-secondary"
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '8px',
                  height: '36px',
                  padding: '0 16px',
                  fontSize: '13px',
                  fontWeight: 600,
                  cursor: archiving ? 'not-allowed' : 'pointer',
                }}
              >
                <Archive size={16} />
                <span>{archiving ? 'Scanning & Archiving...' : 'Archive Inactive Conversations Now'}</span>
              </button>
            </div>
          </div>
        </div>
      )}
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
            alignItems: 'flex-start',
            marginBottom: '16px',
          }}
        >
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Sliders size={18} color="var(--primary)" />
              <h2 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
                Extension Tab Switchers Format
              </h2>
              <span
                style={{
                  fontSize: '11px',
                  fontWeight: 600,
                  padding: '2px 8px',
                  borderRadius: '12px',
                  backgroundColor: '#e8f0fe',
                  color: '#0b57d0',
                }}
              >
                In-App UI
              </span>
            </div>
            <p style={{ margin: '6px 0 0', fontSize: '13px', color: 'var(--text-muted)' }}>
              Choose between icon-only or icon with label for auxiliary panel extension tabs.
            </p>
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
              border: `1.5px solid ${(op.aux_tabs_format || 'icon') === 'icon' ? '#0b57d0' : '#e2e8f0'}`,
              backgroundColor: (op.aux_tabs_format || 'icon') === 'icon' ? '#eff6ff' : '#ffffff',
              cursor: 'pointer',
              transition: 'all 0.15s ease',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 600, fontSize: '13px', color: '#1e293b' }}>
                <span>Just an Icon (Compact)</span>
                <span
                  style={{
                    fontSize: '10px',
                    fontWeight: 600,
                    padding: '2px 6px',
                    borderRadius: '10px',
                    backgroundColor: '#dcfce7',
                    color: '#15803d',
                  }}
                >
                  Matches Native Tabs
                </span>
              </div>
              <input
                type="radio"
                name="aux_tabs_format"
                checked={(op.aux_tabs_format || 'icon') === 'icon'}
                onChange={() => handleAuxTabsFormatChange('icon')}
                style={{ accentColor: '#0b57d0', cursor: 'pointer' }}
              />
            </div>

            <div style={{ fontSize: '12px', color: '#64748b', lineHeight: 1.4 }}>
              Displays clean monochrome stroke icons matching Antigravity's native tabs without text. Hovering displays the tab name tooltip.
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
                border: '1px solid #e2e8f0',
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
                  border: '1px solid #cbd5e1',
                  color: '#1e293b',
                }}
                title="Preview Browser"
              >
                <Globe size={14} />
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
                  border: '1px solid #cbd5e1',
                  color: '#1e293b',
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
                  border: '1px solid #cbd5e1',
                  color: '#1e293b',
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
                  border: '1px solid #cbd5e1',
                  color: '#1e293b',
                }}
                title="GitHub"
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22" />
                </svg>
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
              border: `1.5px solid ${op.aux_tabs_format === 'icon_and_name' ? '#0b57d0' : '#e2e8f0'}`,
              backgroundColor: op.aux_tabs_format === 'icon_and_name' ? '#eff6ff' : '#ffffff',
              cursor: 'pointer',
              transition: 'all 0.15s ease',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 600, fontSize: '13px', color: '#1e293b' }}>
                <span>Icon and Name</span>
              </div>
              <input
                type="radio"
                name="aux_tabs_format"
                checked={op.aux_tabs_format === 'icon_and_name'}
                onChange={() => handleAuxTabsFormatChange('icon_and_name')}
                style={{ accentColor: '#0b57d0', cursor: 'pointer' }}
              />
            </div>

            <div style={{ fontSize: '12px', color: '#64748b', lineHeight: 1.4 }}>
              Displays clean monochrome stroke icons accompanied by full tab names for immediate text navigation.
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
                border: '1px solid #e2e8f0',
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
                  border: '1px solid #cbd5e1',
                  color: '#1e293b',
                  fontSize: '11px',
                  fontWeight: 500,
                }}
              >
                <Globe size={13} />
                <span>Preview Browser</span>
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
                  border: '1px solid #cbd5e1',
                  color: '#1e293b',
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
                  border: '1px solid #cbd5e1',
                  color: '#1e293b',
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
                  border: '1px solid #cbd5e1',
                  color: '#1e293b',
                  fontSize: '11px',
                  fontWeight: 500,
                }}
              >
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22" />
                </svg>
                <span>GitHub</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="google-card">
        {/* Header with Master Toggle on the Right */}
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
              Overview Panel Section Division
            </h2>
            <p style={{ margin: '4px 0 0', fontSize: '13px', color: 'var(--text-muted)' }}>
              Add clean borders, subtle dividers, and zebra striping to Antigravity's Overview panel sections.
            </p>
          </div>

          <ToggleSwitch
            checked={op.enabled}
            onChange={(checked) =>
              setConfig({
                ...config,
                overview_panel: { ...op, enabled: checked },
              })
            }
          />
        </div>

        {op.enabled && (
          <div style={{ borderTop: '1px solid #f1f5f9', paddingTop: '18px' }}>
            {/* Top Quick Settings Row: Toggles on the right with vertical grey breaker lines */}
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                flexWrap: 'wrap',
                background: '#f8fafc',
                borderRadius: '10px',
                border: '1px solid #e2e8f0',
                padding: '12px 16px',
                marginBottom: '20px',
              }}
            >
              {/* Switch 1: Replace See all / See less with refined divider */}
              <label
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: '12px',
                  fontSize: '13px',
                  color: '#334155',
                  cursor: 'pointer',
                  flex: 1,
                  minWidth: '280px',
                  paddingRight: '16px',
                }}
              >
                <span>Refined Expand/Contract Triangle (▾ / ▴)</span>
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

            {/* 2-Column Layout: Settings on Left, Fixed Vertical Divider, Interactive Overview Preview on Right */}
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'minmax(0, 1fr) 1px 360px',
                gap: '24px 12px',
                alignItems: 'stretch',
              }}
            >
              {/* LEFT COLUMN: Section Division Modes & Detailed Controls */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                {/* Section Division Style Selection (2 Visual Option Cards) */}
                <div
                  style={{
                    background: '#f8fafc',
                    borderRadius: '10px',
                    border: '1px solid #e2e8f0',
                    padding: '16px 20px',
                  }}
                >
                  <div style={{ fontSize: '13px', fontWeight: 700, color: '#1e293b', marginBottom: '6px' }}>
                    Section Division Style
                  </div>
                  <p style={{ margin: '0 0 14px', fontSize: '12px', color: '#64748b' }}>
                    Choose how the sections in the overview panel are visually separated:
                  </p>

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
                        flexDirection: 'column',
                        gap: '8px',
                        padding: '14px',
                        borderRadius: '8px',
                        border: `1.5px solid ${op.division_style === 'divider_line' ? '#0b57d0' : '#e2e8f0'}`,
                        backgroundColor: op.division_style === 'divider_line' ? '#eff6ff' : '#ffffff',
                        cursor: 'pointer',
                        transition: 'all 0.15s ease',
                      }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 600, fontSize: '13px', color: '#1e293b' }}>
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
                      <div style={{ fontSize: '12px', color: '#64748b', lineHeight: 1.4 }}>
                        Adds a clean horizontal divider line between each section in the panel.
                      </div>
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
                        flexDirection: 'column',
                        gap: '8px',
                        padding: '14px',
                        borderRadius: '8px',
                        border: `1.5px solid ${op.division_style === 'border_zone' ? '#0b57d0' : '#e2e8f0'}`,
                        backgroundColor: op.division_style === 'border_zone' ? '#eff6ff' : '#ffffff',
                        cursor: 'pointer',
                        transition: 'all 0.15s ease',
                      }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 600, fontSize: '13px', color: '#1e293b' }}>
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
                      <div style={{ fontSize: '12px', color: '#64748b', lineHeight: 1.4 }}>
                        Groups each section in a bounded box with slightly whiter background like the chat input box.
                      </div>
                    </div>
                  </div>
                </div>

                {/* Detailed Controls based on selected mode */}
                {op.division_style === 'border_zone' && (
                  /* Option 2 Settings: Border Zone Grouping, Background Contrast, Radius, Padding & Spacing */
                  <div
                    style={{
                      background: '#f8fafc',
                      borderRadius: '10px',
                      border: '1px solid #e2e8f0',
                      padding: '16px 20px',
                      display: 'flex',
                      flexDirection: 'column',
                      gap: '16px',
                    }}
                  >
                    <div style={{ fontSize: '13px', fontWeight: 700, color: '#1e293b' }}>
                      Border Zone Appearance & Contrast
                    </div>

                    {/* Background Contrast Selector */}
                    <div>
                      <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: '#475569', marginBottom: '8px' }}>
                        Zone Background Tone:
                      </label>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                        {[
                          { id: 'whiter', label: 'Whiter Contrast (#ffffff)', desc: 'Like the chat input box vs canvas background — crisp and bright' },
                          { id: 'subtle', label: 'Subtle Card Tint (#fcfdfd)', desc: 'Ultra-gentle contrast with crisp 1px border' },
                          { id: 'card', label: 'Elevated Surface Card', desc: 'Slightly elevated container with soft shadow' },
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
                              border: `1.5px solid ${op.zone_background_contrast === tone.id ? '#0b57d0' : '#e2e8f0'}`,
                              background: op.zone_background_contrast === tone.id ? '#eff6ff' : '#ffffff',
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
                            <div>
                              <div style={{ fontSize: '12px', fontWeight: 600, color: '#1e293b' }}>{tone.label}</div>
                              <div style={{ fontSize: '11px', color: '#64748b' }}>{tone.desc}</div>
                            </div>
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
                        background: '#ffffff',
                        borderRadius: '8px',
                        border: '1px solid #e2e8f0',
                      }}
                    >
                      {/* Border Radius */}
                      <div style={{ flex: 1, minWidth: '180px', paddingRight: '8px' }}>
                        <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: '#475569', marginBottom: '8px' }}>
                          Border Radius: <strong>{op.zone_border_radius}px</strong>
                        </label>
                        <div style={{ display: 'flex', gap: '6px' }}>
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
                                borderColor: op.zone_border_radius === r ? '#0b57d0' : '#e2e8f0',
                                cursor: 'pointer',
                              }}
                            >
                              {r}px
                            </button>
                          ))}
                        </div>
                      </div>

                      {/* Vertical Grey Breaker */}
                      <div style={{ width: '1px', height: '40px', backgroundColor: 'var(--border, #e2e8f0)', margin: '0' }} />

                      {/* Gap Between Zones */}
                      <div style={{ flex: 1, minWidth: '180px', paddingLeft: '8px' }}>
                        <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: '#475569', marginBottom: '8px' }}>
                          Zone Spacing (Gap): <strong>{op.zone_gap}px</strong>
                        </label>
                        <div style={{ display: 'flex', gap: '6px' }}>
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
                                borderColor: op.zone_gap === g ? '#0b57d0' : '#e2e8f0',
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
                        background: '#ffffff',
                        borderRadius: '8px',
                        border: '1px solid #e2e8f0',
                      }}
                    >
                      {/* Internal Padding */}
                      <div style={{ flex: 1, minWidth: '180px', paddingRight: '8px' }}>
                        <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: '#475569', marginBottom: '8px' }}>
                          Internal Padding: <strong>{op.zone_padding}px</strong>
                        </label>
                        <div style={{ display: 'flex', gap: '6px' }}>
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
                                borderColor: op.zone_padding === p ? '#0b57d0' : '#e2e8f0',
                                cursor: 'pointer',
                              }}
                            >
                              {p}px
                            </button>
                          ))}
                        </div>
                      </div>

                      {/* Vertical Grey Breaker */}
                      <div style={{ width: '1px', height: '40px', backgroundColor: 'var(--border, #e2e8f0)', margin: '0' }} />

                      {/* Border Color */}
                      <div style={{ flex: 1, minWidth: '180px', paddingLeft: '8px' }}>
                        <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: '#475569', marginBottom: '8px' }}>
                          Border Color:
                        </label>
                        <div style={{ display: 'flex', gap: '6px', alignItems: 'center' }}>
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
                                border: `1px solid ${op.zone_border_color === col.hex ? '#0b57d0' : '#e2e8f0'}`,
                                background: op.zone_border_color === col.hex ? '#eff6ff' : '#ffffff',
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

              {/* VERTICAL DIVIDER */}
              <div style={{ width: '1px', backgroundColor: 'var(--border, #e2e8f0)', alignSelf: 'stretch' }} />

              {/* RIGHT COLUMN: Interactive Live Preview of Antigravity's Overview Panel */}
              <div
                style={{
                  width: '360px',
                  boxSizing: 'border-box',
                  background: '#f8fafc',
                  borderRadius: '12px',
                  border: '1px solid var(--border, #e2e8f0)',
                  padding: '16px',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '12px',
                  position: 'sticky',
                  top: '20px',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid var(--border, #e2e8f0)', paddingBottom: '10px' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <Sliders size={16} color="#0b57d0" />
                    <span style={{ fontSize: '13px', fontWeight: 700, color: '#1e293b' }}>
                      Live Preview: Antigravity Overview Panel
                    </span>
                  </div>
                  <span
                    style={{
                      fontSize: '11px',
                      fontWeight: 600,
                      padding: '2px 8px',
                      borderRadius: '12px',
                      backgroundColor: op.division_style === 'divider_line' ? '#e0f2fe' : '#dcfce7',
                      color: op.division_style === 'divider_line' ? '#0369a1' : '#15803d',
                    }}
                  >
                    {op.division_style === 'divider_line' ? 'Divider Lines' : 'Border Zones (Whiter BG)'}
                  </span>
                </div>

                {/* Simulating the Antigravity right-side panel container */}
                <div
                  style={{
                    backgroundColor: op.division_style === 'border_zone' ? '#f1f5f9' : '#ffffff',
                    borderRadius: '10px',
                    border: '1px solid #cbd5e1',
                    padding: '14px',
                    maxHeight: '640px',
                    overflowY: 'auto',
                    boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.02)',
                  }}
                >
                  {/* Top Auxiliary Header Tab Strip Preview */}
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '14px', borderBottom: '1px solid #e2e8f0', paddingBottom: '8px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '3px' }}>
                      {/* Native Tab 1: Overview (Active) */}
                      <div
                        style={{
                          width: '24px',
                          height: '24px',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          borderRadius: '4px',
                          backgroundColor: 'rgba(0,0,0,0.08)',
                          color: '#1e293b',
                        }}
                        title="Overview"
                      >
                        <FileText size={14} />
                      </div>
                      {/* Native Tab 2: Review */}
                      <div
                        style={{
                          width: '24px',
                          height: '24px',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          borderRadius: '4px',
                          color: '#64748b',
                        }}
                        title="Review"
                      >
                        <Split size={14} />
                      </div>
                      {/* Native Tab 3: Terminal */}
                      <div
                        style={{
                          width: '24px',
                          height: '24px',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          borderRadius: '4px',
                          color: '#64748b',
                          fontSize: '11px',
                          fontFamily: 'monospace',
                          fontWeight: 700,
                        }}
                        title="Terminal"
                      >
                        {'>_'}
                      </div>
                      {/* Native Plus */}
                      <div
                        style={{
                          width: '20px',
                          height: '20px',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          borderRadius: '4px',
                          color: '#94a3b8',
                        }}
                      >
                        <Plus size={13} />
                      </div>

                      {/* Left Divider */}
                      <div style={{ width: '1px', minWidth: '1px', maxWidth: '1px', height: '16px', backgroundColor: 'var(--border, #e2e8f0)', margin: '0 0.5px', flexShrink: 0, opacity: 0.7 }} />

                      {/* Injected Swiss Tabs in chosen format */}
                      {(op.aux_tabs_format || 'icon') === 'icon' ? (
                        <div style={{ display: 'inline-flex', alignItems: 'center', gap: '1px', flexShrink: 0 }}>
                          <div
                            style={{
                              width: '21px',
                              minWidth: '20px',
                              height: '24px',
                              display: 'flex',
                              alignItems: 'center',
                              justifyContent: 'center',
                              borderRadius: '4px',
                              color: '#64748b',
                              backgroundColor: 'transparent',
                              flexShrink: 0,
                            }}
                            title="Preview Browser"
                          >
                            <Globe size={13.5} />
                          </div>
                          <div
                            style={{
                              width: '21px',
                              minWidth: '20px',
                              height: '24px',
                              display: 'flex',
                              alignItems: 'center',
                              justifyContent: 'center',
                              borderRadius: '4px',
                              color: '#64748b',
                              backgroundColor: 'transparent',
                              flexShrink: 0,
                            }}
                            title="Files"
                          >
                            <Folder size={13.5} />
                          </div>
                          <div
                            style={{
                              width: '21px',
                              minWidth: '20px',
                              height: '24px',
                              display: 'flex',
                              alignItems: 'center',
                              justifyContent: 'center',
                              borderRadius: '4px',
                              color: '#64748b',
                              backgroundColor: 'transparent',
                              flexShrink: 0,
                            }}
                            title="Memos"
                          >
                            <FileText size={13.5} />
                          </div>
                          <div
                            style={{
                              width: '21px',
                              minWidth: '20px',
                              height: '24px',
                              display: 'flex',
                              alignItems: 'center',
                              justifyContent: 'center',
                              borderRadius: '4px',
                              color: '#64748b',
                              backgroundColor: 'transparent',
                              flexShrink: 0,
                            }}
                            title="GitHub"
                          >
                            <svg width="13.5" height="13.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                              <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22" />
                            </svg>
                          </div>
                        </div>
                      ) : (
                        <div style={{ display: 'inline-flex', alignItems: 'center', gap: '2px', flexShrink: 0 }}>
                          <div
                            style={{
                              height: '24px',
                              padding: '0 6px',
                              display: 'flex',
                              alignItems: 'center',
                              gap: '4px',
                              borderRadius: '4px',
                              color: '#64748b',
                              fontSize: '10.5px',
                              fontWeight: 500,
                              whiteSpace: 'nowrap',
                              flexShrink: 0,
                            }}
                          >
                            <Globe size={13} />
                            <span>Preview Browser</span>
                          </div>
                          <div
                            style={{
                              height: '24px',
                              padding: '0 6px',
                              display: 'flex',
                              alignItems: 'center',
                              gap: '4px',
                              borderRadius: '4px',
                              color: '#64748b',
                              fontSize: '10.5px',
                              fontWeight: 500,
                              whiteSpace: 'nowrap',
                              flexShrink: 0,
                            }}
                          >
                            <Folder size={13} />
                            <span>Files</span>
                          </div>
                          <div
                            style={{
                              height: '24px',
                              padding: '0 6px',
                              display: 'flex',
                              alignItems: 'center',
                              gap: '4px',
                              borderRadius: '4px',
                              color: '#64748b',
                              fontSize: '10.5px',
                              fontWeight: 500,
                              whiteSpace: 'nowrap',
                              flexShrink: 0,
                            }}
                          >
                            <FileText size={13} />
                            <span>Memos</span>
                          </div>
                          <div
                            style={{
                              height: '24px',
                              padding: '0 6px',
                              display: 'flex',
                              alignItems: 'center',
                              gap: '4px',
                              borderRadius: '4px',
                              color: '#64748b',
                              fontSize: '10.5px',
                              fontWeight: 500,
                              whiteSpace: 'nowrap',
                              flexShrink: 0,
                            }}
                          >
                            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                              <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22" />
                            </svg>
                            <span>GitHub</span>
                          </div>
                        </div>
                      )}

                      {/* Right Divider */}
                      <div style={{ width: '1px', minWidth: '1px', maxWidth: '1px', height: '16px', backgroundColor: 'var(--border, #e2e8f0)', margin: '0 0.5px', flexShrink: 0, opacity: 0.7 }} />
                    </div>

                    <div style={{ display: 'flex', gap: '8px', alignItems: 'center', color: '#94a3b8' }}>
                      <Maximize2 size={12} style={{ cursor: 'pointer' }} />
                      <X size={12} style={{ cursor: 'pointer' }} />
                    </div>
                  </div>

                  {/* Render 8 Sections accurately matching host screenshot */}
                  {(() => {
                    const sections = [
                      {
                        id: 'subagents',
                        title: 'Subagents',
                        count: 1,
                        hasChevron: true,
                        renderContent: () => (
                          <div style={{ marginTop: '6px' }}>
                            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: '12px', fontWeight: 500, color: '#1e293b' }}>
                              <span>Comprehensive Requirements Investigator (2 subagents)</span>
                              <span style={{ color: '#94a3b8' }}>›</span>
                            </div>
                            <div style={{ display: 'flex', alignItems: 'center', gap: '4px', fontSize: '11px', color: '#64748b', marginTop: '2px' }}>
                              <CheckCircle2 size={12} color="#16a34a" />
                              <span>Worked for 17m</span>
                            </div>
                          </div>
                        ),
                      },
                      {
                        id: 'files',
                        title: 'Files Changed',
                        count: 33,
                        tag: 'Uncommitted',
                        hasChevron: true,
                        renderContent: () => (
                          <div style={{ marginTop: '6px' }}>
                            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '12px', color: '#334155' }}>
                              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                                <span style={{ color: '#0284c7', fontSize: '11px', fontWeight: 700 }}>M↓</span>
                                <span style={{ fontWeight: 500 }}>README.md</span>
                              </div>
                              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                                <span style={{ color: '#059669', fontSize: '11px', fontWeight: 700 }}>Go</span>
                                <span>main_test.go <span style={{ color: '#94a3b8', fontSize: '11px' }}>cmd/swiss</span></span>
                              </div>
                              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                                <span style={{ color: '#6366f1', fontSize: '11px', fontWeight: 700 }}>TSX</span>
                                <span>App.tsx <span style={{ color: '#94a3b8', fontSize: '11px' }}>frontend/src</span></span>
                              </div>
                              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                                <span style={{ color: '#3b82f6', fontSize: '11px', fontWeight: 700 }}>TS</span>
                                <span>api.ts <span style={{ color: '#94a3b8', fontSize: '11px' }}>frontend/src</span></span>
                              </div>
                              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                                <span style={{ color: '#6366f1', fontSize: '11px', fontWeight: 700 }}>TSX</span>
                                <span>AccountDetailModal.tsx <span style={{ color: '#94a3b8', fontSize: '11px' }}>frontend/src/components</span></span>
                              </div>

                              {overviewFilesExpanded && (
                                <>
                                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                                    <span style={{ color: '#059669', fontSize: '11px', fontWeight: 700 }}>Go</span>
                                    <span>models.go <span style={{ color: '#94a3b8', fontSize: '11px' }}>pkg/enhancements</span></span>
                                  </div>
                                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                                    <span style={{ color: '#059669', fontSize: '11px', fontWeight: 700 }}>Go</span>
                                    <span>script.go <span style={{ color: '#94a3b8', fontSize: '11px' }}>pkg/enhancements</span></span>
                                  </div>
                                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                                    <span style={{ color: '#3b82f6', fontSize: '11px', fontWeight: 700 }}>TS</span>
                                    <span>types.ts <span style={{ color: '#94a3b8', fontSize: '11px' }}>frontend/src</span></span>
                                  </div>
                                </>
                              )}
                            </div>

                            {/* "See all" vs Refined Triangle Replacement */}
                            <div style={{ marginTop: '2px' }}>
                              {op.replace_see_all_triangle ? (
                                <div
                                  onClick={() => setOverviewFilesExpanded(!overviewFilesExpanded)}
                                  style={{
                                    display: 'flex',
                                    alignItems: 'center',
                                    justifyContent: 'center',
                                    width: '100%',
                                    height: '8px',
                                    marginTop: '1px',
                                    cursor: 'pointer',
                                    userSelect: 'none',
                                    boxSizing: 'border-box',
                                  }}
                                  title={overviewFilesExpanded ? 'Collapse files list' : 'Expand all 33 files'}
                                >
                                  <div
                                    style={{
                                      display: 'inline-flex',
                                      alignItems: 'center',
                                      justifyContent: 'center',
                                      width: '14px',
                                      height: '6px',
                                      color: '#64748b',
                                      fontSize: '8px',
                                      transition: 'all 0.18s ease',
                                    }}
                                  >
                                    <span style={{ fontSize: '8px', lineHeight: 1 }}>
                                      {overviewFilesExpanded ? '▴' : '▾'}
                                    </span>
                                  </div>
                                </div>
                              ) : (
                                <button
                                  type="button"
                                  onClick={() => setOverviewFilesExpanded(!overviewFilesExpanded)}
                                  style={{
                                    fontSize: '12px',
                                    color: '#0b57d0',
                                    background: 'none',
                                    border: 'none',
                                    cursor: 'pointer',
                                    padding: '2px 0',
                                    textAlign: 'left',
                                  }}
                                >
                                  {overviewFilesExpanded ? 'See less' : 'See all (33)'}
                                </button>
                              )}
                            </div>
                          </div>
                        ),
                      },
                      {
                        id: 'artifacts',
                        title: 'Artifacts',
                        count: 1,
                        hasChevron: true,
                        renderContent: () => (
                          <div style={{ marginTop: '6px', fontSize: '12px', color: '#334155', display: 'flex', alignItems: 'center', gap: '6px' }}>
                            <FileText size={13} color="#64748b" />
                            <span>Prompt Draft</span>
                          </div>
                        ),
                      },
                      {
                        id: 'uploads',
                        title: 'Uploads',
                        count: 14,
                        hasChevron: true,
                        renderContent: () => (
                          <div style={{ marginTop: '6px' }}>
                            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '12px', color: '#334155' }}>
                              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                                <Image size={13} color="#64748b" />
                                <span>Media (Today 6:59 AM)</span>
                              </div>
                              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                                <Image size={13} color="#64748b" />
                                <span>Media (Today 6:57 AM)</span>
                              </div>
                              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                                <Image size={13} color="#64748b" />
                                <span>Media (Today 6:55 AM)</span>
                              </div>
                              {overviewUploadsExpanded && (
                                <>
                                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                                    <Image size={13} color="#64748b" />
                                    <span>Media (Today 6:54 AM)</span>
                                  </div>
                                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                                    <Image size={13} color="#64748b" />
                                    <span>Media (Today 6:53 AM)</span>
                                  </div>
                                </>
                              )}
                            </div>

                            <div style={{ marginTop: '2px' }}>
                              {op.replace_see_all_triangle ? (
                                <div
                                  onClick={() => setOverviewUploadsExpanded(!overviewUploadsExpanded)}
                                  style={{
                                    display: 'flex',
                                    alignItems: 'center',
                                    justifyContent: 'center',
                                    width: '100%',
                                    height: '8px',
                                    marginTop: '1px',
                                    cursor: 'pointer',
                                    userSelect: 'none',
                                    boxSizing: 'border-box',
                                  }}
                                  title={overviewUploadsExpanded ? 'Collapse uploads' : 'Expand all 14 uploads'}
                                >
                                  <div
                                    style={{
                                      display: 'inline-flex',
                                      alignItems: 'center',
                                      justifyContent: 'center',
                                      width: '14px',
                                      height: '6px',
                                      color: '#64748b',
                                      fontSize: '8px',
                                      transition: 'all 0.18s ease',
                                    }}
                                  >
                                    <span style={{ fontSize: '8px', lineHeight: 1 }}>
                                      {overviewUploadsExpanded ? '▴' : '▾'}
                                    </span>
                                  </div>
                                </div>
                              ) : (
                                <button
                                  type="button"
                                  onClick={() => setOverviewUploadsExpanded(!overviewUploadsExpanded)}
                                  style={{
                                    fontSize: '12px',
                                    color: '#0b57d0',
                                    background: 'none',
                                    border: 'none',
                                    cursor: 'pointer',
                                    padding: '2px 0',
                                    textAlign: 'left',
                                  }}
                                >
                                  {overviewUploadsExpanded ? 'See less' : 'See all (14)'}
                                </button>
                              )}
                            </div>
                          </div>
                        ),
                      },
                      {
                        id: 'tasks',
                        title: 'Background Tasks',
                        count: 0,
                        hasChevron: false,
                        renderContent: () => null,
                      },
                      {
                        id: 'terminals',
                        title: 'Terminals',
                        count: 0,
                        hasChevron: false,
                        renderContent: () => null,
                      },
                      {
                        id: 'goals',
                        title: 'Goals',
                        count: 2,
                        hasChevron: true,
                        renderContent: () => (
                          <div style={{ marginTop: '6px', display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '12px', color: '#334155' }}>
                            <div style={{ display: 'flex', alignItems: 'flex-start', gap: '6px' }}>
                              <CheckCircle2 size={13} color="#16a34a" style={{ marginTop: '2px', flexShrink: 0 }} />
                              <span style={{ lineHeight: 1.3 }}>/teamwork-preview /wish-coding you need more then these 4 tickets...</span>
                            </div>
                            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                              <CheckCircle2 size={13} color="#16a34a" style={{ marginTop: '2px', flexShrink: 0 }} />
                              <span style={{ lineHeight: 1.3 }}>/teamwork-preview add a new section, to be below the UI Enhancement sectio...</span>
                            </div>
                          </div>
                        ),
                      },
                      {
                        id: 'skills',
                        title: 'Skills Used',
                        count: 2,
                        hasChevron: true,
                        renderContent: () => (
                          <div style={{ marginTop: '6px', display: 'flex', flexDirection: 'column', gap: '4px', fontSize: '12px', color: '#334155' }}>
                            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                              <FileText size={13} color="#64748b" />
                              <span>antigravity-guide <span style={{ color: '#94a3b8', fontSize: '11px' }}>.../skills/antigravity_guide</span></span>
                            </div>
                            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                              <FileText size={13} color="#64748b" />
                              <span>wish-coding <span style={{ color: '#94a3b8', fontSize: '11px' }}>.../skills/wish-coding</span></span>
                            </div>
                          </div>
                        ),
                      },
                    ]

                    return sections.map((sec, idx) => {
                      const isCollapsed = collapsedSections[sec.id]
                      const isZone = op.division_style === 'border_zone'
                      const zoneBg =
                        op.zone_background_contrast === 'whiter'
                          ? '#ffffff'
                          : op.zone_background_contrast === 'subtle'
                          ? '#fcfdfd'
                          : '#ffffff'

                      return (
                        <React.Fragment key={sec.id}>
                          <div
                            style={
                              isZone
                                ? {
                                    backgroundColor: zoneBg,
                                    border: `1px solid ${op.zone_border_color || '#e2e8f0'}`,
                                    borderRadius: `${op.zone_border_radius || 8}px`,
                                    padding: `${op.zone_padding || 10}px`,
                                    marginBottom: `${op.zone_gap || 10}px`,
                                    boxShadow:
                                      op.zone_background_contrast === 'card'
                                        ? '0 1px 3px rgba(0,0,0,0.04)'
                                        : '0 1px 2px rgba(0,0,0,0.02)',
                                    transition: 'all 0.15s ease',
                                  }
                                : {
                                    padding: '0',
                                    marginBottom: idx < sections.length - 1 ? (op.division_style === 'divider_line' ? '5px' : '10px') : 0,
                                  }
                            }
                          >
                            {/* Section Header */}
                            <div
                              onClick={() => toggleSection(sec.id)}
                              style={{
                                display: 'flex',
                                alignItems: 'center',
                                justifyContent: 'space-between',
                                cursor: 'pointer',
                                userSelect: 'none',
                              }}
                            >
                              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                                <span style={{ fontSize: '13px', fontWeight: 600, color: '#334155' }}>
                                  {sec.title}
                                </span>
                                <span
                                  style={{
                                    fontSize: '11px',
                                    fontWeight: 700,
                                    color: '#64748b',
                                  }}
                                >
                                  {sec.count}
                                </span>
                                {sec.hasChevron ? (
                                  <span style={{ fontSize: '10px', color: '#94a3b8', transform: isCollapsed ? 'rotate(-90deg)' : 'none', transition: 'transform 0.15s' }}>
                                    ▼
                                  </span>
                                ) : (
                                  <span style={{ fontSize: '11px', color: '#94a3b8' }}>›</span>
                                )}
                              </div>

                              {sec.tag && (
                                <span
                                  style={{
                                    fontSize: '10px',
                                    fontWeight: 600,
                                    padding: '1px 6px',
                                    borderRadius: '4px',
                                    background: '#f1f5f9',
                                    color: '#475569',
                                    border: '1px solid #e2e8f0',
                                  }}
                                >
                                  {sec.tag} ▾
                                </span>
                              )}
                            </div>

                            {/* Section Content */}
                            {!isCollapsed && sec.renderContent()}

                          </div>

                          {/* Horizontal Divider Line between sections (when in divider_line mode) */}
                          {!isZone && op.division_style === 'divider_line' && idx < sections.length - 1 && (
                            <div
                              style={{
                                height: '0px',
                                width: 'calc(100% - 12px)',
                                border: 'none',
                                borderTop: '1px solid #e2e8f0',
                                margin: '0 auto 5px auto',
                                boxSizing: 'border-box',
                              }}
                            />
                          )}
                        </React.Fragment>
                      )
                    })
                  })()}
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </>
  )}
    </div>
  )
}

