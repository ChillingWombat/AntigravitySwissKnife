import React, { useEffect, useState } from 'react'
import {
  Globe,
  Folder,
  FileText,
  Smartphone,
  Monitor,
  ExternalLink,
  CheckCircle2,
  AlertTriangle,
  ArrowLeft,
} from 'lucide-react'
import { ToggleSwitch } from '../components/ToggleSwitch'
import { GithubIcon } from '../components/GithubIcon'
import { api } from '../api'
import { GitHubWorkspacePage } from './GitHubWorkspacePage'
import type { EnhancementsConfig } from '../types'

const IDE_PRESETS = ['code', 'cursor', 'windsurf', 'codium', 'zed']

type ExtensionVisibility = { aux_panel: boolean; main_page: boolean }
const DEFAULT_EXT_VIS: ExtensionVisibility = { aux_panel: true, main_page: true }

export interface ExtensionsPageProps {
  activeTab?: number
  onTabChange?: (tab: number) => void
  scope?: string
  onScopeChange?: (scope: string) => void
  fallbackProject?: string
}

export const ExtensionsPage: React.FC<ExtensionsPageProps> = ({
  scope,
  fallbackProject,
}) => {
  // --- 1. Extension Visibility Switches (daemon-persisted EnhancementsConfig) ---
  // aux_panel  = available in the IDE's right auxiliary panel
  // main_page  = openable on the main stage via a left-sidebar tab button
  const [enhConfig, setEnhConfig] = useState<EnhancementsConfig | null>(null)
  const [extVisLocal, setExtVisLocal] = useState<Record<string, ExtensionVisibility>>({})
  const [enhError, setEnhError] = useState<string | null>(null)

  // --- 2. Extension Specific Settings State ---
  // GitHub Workspace
  const [showWorkspaceModal, setShowWorkspaceModal] = useState<boolean>(false)
  const [workspaceDefaultView, setWorkspaceDefaultView] = useState<'kanban' | 'list'>(() => {
    return (localStorage.getItem('antigravity_workspace_default_view') as 'kanban' | 'list') || 'kanban'
  })

  // Preview Browser
  const [previewUrl, setPreviewUrl] = useState<string>(() => {
    return localStorage.getItem('antigravity_browser_default_url') || 'http://localhost:5173'
  })

  // File Explorer
  const [preferredIDE, setPreferredIDE] = useState<string>(() => {
    return (
      localStorage.getItem('antigravity_swiss_preferred_ide') ||
      localStorage.getItem('antigravity_preferred_ide') ||
      'code'
    )
  })
  const [ideCustomMode, setIdeCustomMode] = useState<boolean>(false)
  const [customIDE, setCustomIDE] = useState<string>(() =>
    IDE_PRESETS.includes(preferredIDE.toLowerCase()) ? '' : preferredIDE
  )
  const ideIsCustom = ideCustomMode || !IDE_PRESETS.includes(preferredIDE.toLowerCase())

  // Quick Memos (daemon-persisted; localStorage seeds the first paint)
  const [memoStorageLocation, setMemoStorageLocation] = useState<'global' | 'project'>(() => {
    return (localStorage.getItem('antigravity_memo_storage_location') as 'global' | 'project') || 'global'
  })
  const [memoViewScope, setMemoViewScope] = useState<'all' | 'current'>(() => {
    return (localStorage.getItem('antigravity_memo_view_scope') as 'all' | 'current') || 'all'
  })
  const [memoSearchScope, setMemoSearchScope] = useState<'text' | 'all'>(() => {
    return (localStorage.getItem('antigravity_memo_search_scope') as 'text' | 'all') || 'text'
  })

  // Mobile Simulator
  const [selectedDevice, setSelectedDevice] = useState<'iphone16' | 'pixel9' | 'ipad'>(() => {
    return (localStorage.getItem('antigravity_mobile_default_device') as any) || 'iphone16'
  })
  const [showBezel, setShowBezel] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_mobile_show_bezel') !== 'false'
  })

  // Computer Use Enhancer
  const [dpiNormalization, setDpiNormalization] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_comp_dpi_norm') !== 'false'
  })
  const [waylandPipeWire, setWaylandPipeWire] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_comp_wayland_pipewire') !== 'false'
  })
  const [accessibilityGrounding, setAccessibilityGrounding] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_comp_accessibility_grounding') !== 'false'
  })

  // Feedback Notification
  const [feedback, setFeedback] = useState<string | null>(null)

  const showFeedback = (msg: string) => {
    setFeedback(msg)
    setTimeout(() => setFeedback(null), 3000)
  }

  // Load daemon-persisted settings (extension visibility, IDE + Quick Memos);
  // keep localStorage fallbacks if the daemon is unreachable.
  useEffect(() => {
    api
      .getEnhancements()
      .then((cfg) => {
        setEnhConfig(cfg)
        setEnhError(null)
      })
      .catch(() =>
        setEnhError('Enhancements daemon unreachable — visibility switches show defaults and will not persist')
      )

    api
      .getPreferredIDE()
      .then((res) => {
        if (res?.success && res.preferred_ide) {
          setPreferredIDE(res.preferred_ide)
          if (!IDE_PRESETS.includes(res.preferred_ide.toLowerCase())) {
            setCustomIDE(res.preferred_ide)
          }
          localStorage.setItem('antigravity_swiss_preferred_ide', res.preferred_ide)
          localStorage.setItem('antigravity_preferred_ide', res.preferred_ide)
        }
      })
      .catch(() => {})

    api
      .getMemoConfig()
      .then((res) => {
        const cfg = res?.config
        if (!cfg) return
        if (cfg.storage_location === 'global' || cfg.storage_location === 'project') {
          setMemoStorageLocation(cfg.storage_location)
          localStorage.setItem('antigravity_memo_storage_location', cfg.storage_location)
        }
        if (cfg.view_scope === 'all' || cfg.view_scope === 'current') {
          setMemoViewScope(cfg.view_scope)
          localStorage.setItem('antigravity_memo_view_scope', cfg.view_scope)
        }
        if (cfg.search_scope === 'text' || cfg.search_scope === 'all') {
          setMemoSearchScope(cfg.search_scope)
          localStorage.setItem('antigravity_memo_search_scope', cfg.search_scope)
        }
      })
      .catch(() => {})
  }, [])

  const handleSaveIDE = async (next: string) => {
    const ide = next.trim()
    if (!ide) return
    setPreferredIDE(ide)
    localStorage.setItem('antigravity_swiss_preferred_ide', ide)
    // Legacy key still read first by the injected extension tab — keep both in sync.
    localStorage.setItem('antigravity_preferred_ide', ide)
    try {
      await api.setPreferredIDE(ide)
      showFeedback(`Preferred IDE set to ${getIDEName(ide)}`)
    } catch {
      showFeedback('Saved preferred IDE locally (daemon unreachable)')
    }
  }

  const handleUpdateMemoConfig = (
    patch: Partial<{
      storage_location: 'global' | 'project'
      view_scope: 'all' | 'current'
      search_scope: 'text' | 'all'
    }>
  ) => {
    const next = {
      storage_location: patch.storage_location ?? memoStorageLocation,
      view_scope: patch.view_scope ?? memoViewScope,
      search_scope: patch.search_scope ?? memoSearchScope,
    }
    setMemoStorageLocation(next.storage_location)
    setMemoViewScope(next.view_scope)
    setMemoSearchScope(next.search_scope)
    localStorage.setItem('antigravity_memo_storage_location', next.storage_location)
    localStorage.setItem('antigravity_memo_view_scope', next.view_scope)
    localStorage.setItem('antigravity_memo_search_scope', next.search_scope)
    api
      .updateMemoConfig(next)
      .then((res) => showFeedback(res?.success ? 'Quick Memos settings saved' : 'Quick Memos settings saved locally'))
      .catch(() => showFeedback('Quick Memos settings saved locally (daemon unreachable)'))
  }

  const segBtnStyle = (active: boolean): React.CSSProperties => ({
    border: 'none',
    padding: '4px 10px',
    borderRadius: '5px',
    fontSize: '11.5px',
    fontWeight: active ? 600 : 500,
    backgroundColor: active ? '#ffffff' : 'transparent',
    color: active ? 'var(--primary)' : 'var(--text-muted)',
    boxShadow: active ? '0 1px 2px rgba(0,0,0,0.06)' : 'none',
    cursor: 'pointer',
  })

  // Per-extension visibility: the EnhancementsConfig.extensions map is the
  // source of truth (dashboard localStorage has no effect on the IDE).
  const visFor = (id: string): ExtensionVisibility => {
    const local = extVisLocal[id]
    if (local) return local
    const v = enhConfig?.extensions?.[id]
    if (v) return { aux_panel: v.aux_panel !== false, main_page: v.main_page === true }
    return DEFAULT_EXT_VIS
  }

  const handleExtVisChange = async (id: string, field: 'aux_panel' | 'main_page', value: boolean) => {
    const optimistic: ExtensionVisibility = { ...visFor(id), [field]: value }
    setExtVisLocal((prev) => {
      const cfgVis = enhConfig?.extensions?.[id]
      const baseVis =
        prev[id] ||
        (cfgVis
          ? { aux_panel: cfgVis.aux_panel !== false, main_page: cfgVis.main_page === true }
          : DEFAULT_EXT_VIS)
      return { ...prev, [id]: { ...baseVis, [field]: value } }
    })
    try {
      let base = enhConfig
      if (!base) {
        base = await api.getEnhancements()
        setEnhConfig(base)
      }
      const cur = base.extensions?.[id]
      const next: ExtensionVisibility = {
        aux_panel: field === 'aux_panel' ? value : cur ? cur.aux_panel !== false : optimistic.aux_panel,
        main_page: field === 'main_page' ? value : cur ? cur.main_page === true : optimistic.main_page,
      }
      const updated: EnhancementsConfig = {
        ...base,
        extensions: { ...(base.extensions || {}), [id]: next },
      }
      const saved = await api.updateEnhancements(updated)
      setEnhConfig(saved)
      setEnhError(null)
      setExtVisLocal((prev) => {
        const copy = { ...prev }
        delete copy[id]
        return copy
      })
      showFeedback('Extension visibility updated')
    } catch {
      setEnhError('Daemon unreachable — extension visibility change was not saved')
    }
  }

  const extSwitchLabelStyle: React.CSSProperties = {
    display: 'inline-flex',
    alignItems: 'center',
    gap: '6px',
    fontSize: '11.5px',
    fontWeight: 600,
    color: 'var(--text-muted)',
  }

  const renderExtSwitches = (id: string, name: string) => {
    const vis = visFor(id)
    return (
      <div style={{ display: 'flex', alignItems: 'center', gap: '16px', flexShrink: 0 }}>
        <span style={extSwitchLabelStyle} title="Available in the IDE's right auxiliary panel">
          <ToggleSwitch
            size="sm"
            checked={vis.aux_panel}
            onChange={(v) => handleExtVisChange(id, 'aux_panel', v)}
            ariaLabel={`${name} enabled`}
          />
          Enabled
        </span>
        <span style={extSwitchLabelStyle} title="Openable on the main stage via a left-sidebar tab button">
          <ToggleSwitch
            size="sm"
            checked={vis.main_page}
            onChange={(v) => handleExtVisChange(id, 'main_page', v)}
            ariaLabel={`${name} main page`}
          />
          Main Page
        </span>
      </div>
    )
  }

  const githubVis = visFor('github')
  const browserVis = visFor('browser')
  const filesVis = visFor('files')
  const memosVis = visFor('memos')
  const mobileVis = visFor('mobile')
  const computerUseVis = visFor('computer_use')

  const getIDEName = (id: string) => {
    const map: Record<string, string> = {
      code: 'VS Code',
      vscode: 'VS Code',
      cursor: 'Cursor',
      windsurf: 'Windsurf',
      codium: 'VSCodium',
      vscodium: 'VSCodium',
      zed: 'Zed',
    }
    return map[id.toLowerCase()] || id || 'VS Code'
  }

  // --- Fullscreen View for GitHub Workspace if requested ---
  if (showWorkspaceModal) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', height: '100%', gap: '12px' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 4px' }}>
          <button
            onClick={() => setShowWorkspaceModal(false)}
            className="btn-pill-tonal"
            style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '6px 14px', fontSize: '12px', fontWeight: 600 }}
          >
            <ArrowLeft size={14} />
            <span>Back to Extensions</span>
          </button>
          <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text-muted)' }}>
            GitHub Workspace Active View
          </div>
        </div>
        <div
          style={{
            flex: 1,
            backgroundColor: 'var(--card, #ffffff)',
            borderRadius: '10px',
            border: '1px solid var(--border)',
            overflow: 'hidden',
            minHeight: '680px',
            height: 'calc(100vh - 140px)',
          }}
        >
          <GitHubWorkspacePage scope={scope} fallbackProject={fallbackProject} />
        </div>
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px', maxWidth: '1200px', margin: '0 auto', width: '100%' }}>

      {feedback && (
        <div
          style={{
            padding: '10px 16px',
            borderRadius: '8px',
            backgroundColor: 'var(--green-bg)',
            color: 'var(--green)',
            fontSize: '12px',
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            fontWeight: 600,
          }}
        >
          <CheckCircle2 size={15} />
          <span>{feedback}</span>
        </div>
      )}

      {enhError && (
        <div
          style={{
            padding: '8px 14px',
            borderRadius: '8px',
            backgroundColor: 'rgba(220, 38, 38, 0.08)',
            border: '1px solid rgba(220, 38, 38, 0.25)',
            color: '#dc2626',
            fontSize: '11.5px',
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            fontWeight: 500,
          }}
        >
          <AlertTriangle size={14} />
          <span>{enhError}</span>
        </div>
      )}

      {/* Extension Gadgets Grid */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>

        {/* ========================================================================= */}
        {/* GADGET 1: GitHub Workspace                                                */}
        {/* ========================================================================= */}
        <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: '14px', padding: '18px 20px' }}>
          {/* Header & Toggle Row */}
          <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <GithubIcon size={16} color="var(--text-muted)" />
                <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                  GitHub Workspace
                </h3>
              </div>
              <p style={{ margin: '2px 0 0', fontSize: '12.5px', color: 'var(--text-muted)', lineHeight: 1.45, maxWidth: '780px' }}>
                Manage repository issues, pull requests, agent tasks, and Kanban boards with one-click direct jump into Antigravity conversations.
              </p>
            </div>
            {renderExtSwitches('github', 'GitHub Workspace')}
          </div>

          {/* Extension Settings & Actions */}
          {(githubVis.aux_panel || githubVis.main_page) && (
            <div style={{ marginTop: '4px', paddingTop: '12px', borderTop: '1px solid var(--border-subtle)', display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '12px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
                <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
                  Default View:
                </span>
                <div style={{ display: 'flex', gap: '4px', backgroundColor: 'var(--tonal)', padding: '2px', borderRadius: '6px' }}>
                  <button
                    onClick={() => {
                      setWorkspaceDefaultView('kanban')
                      localStorage.setItem('antigravity_workspace_default_view', 'kanban')
                    }}
                    style={{
                      border: 'none',
                      padding: '4px 10px',
                      borderRadius: '5px',
                      fontSize: '11.5px',
                      fontWeight: workspaceDefaultView === 'kanban' ? 600 : 500,
                      backgroundColor: workspaceDefaultView === 'kanban' ? '#ffffff' : 'transparent',
                      color: workspaceDefaultView === 'kanban' ? 'var(--primary)' : 'var(--text-muted)',
                      boxShadow: workspaceDefaultView === 'kanban' ? '0 1px 2px rgba(0,0,0,0.06)' : 'none',
                      cursor: 'pointer',
                    }}
                  >
                    Kanban Board
                  </button>
                  <button
                    onClick={() => {
                      setWorkspaceDefaultView('list')
                      localStorage.setItem('antigravity_workspace_default_view', 'list')
                    }}
                    style={{
                      border: 'none',
                      padding: '4px 10px',
                      borderRadius: '5px',
                      fontSize: '11.5px',
                      fontWeight: workspaceDefaultView === 'list' ? 600 : 500,
                      backgroundColor: workspaceDefaultView === 'list' ? '#ffffff' : 'transparent',
                      color: workspaceDefaultView === 'list' ? 'var(--primary)' : 'var(--text-muted)',
                      boxShadow: workspaceDefaultView === 'list' ? '0 1px 2px rgba(0,0,0,0.06)' : 'none',
                      cursor: 'pointer',
                    }}
                  >
                    List View
                  </button>
                </div>
              </div>

              <button
                onClick={() => setShowWorkspaceModal(true)}
                className="btn-pill-primary"
                style={{ padding: '6px 14px', fontSize: '12px', display: 'inline-flex', alignItems: 'center', gap: '6px' }}
              >
                <ExternalLink size={13} />
                <span>Open Full Workspace</span>
              </button>
            </div>
          )}
        </div>

        {/* ========================================================================= */}
        {/* GADGET 2: Preview Browser                                                 */}
        {/* ========================================================================= */}
        <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: '14px', padding: '18px 20px' }}>
          {/* Header & Toggle Row */}
          <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <Globe size={16} color="var(--text-muted)" />
                <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                  Preview Browser
                </h3>
              </div>
              <p style={{ margin: '2px 0 0', fontSize: '12.5px', color: 'var(--text-muted)', lineHeight: 1.45, maxWidth: '780px' }}>
                Embeds a lightweight development browser inside Antigravity's auxiliary panel with port shortcuts (:5173, :3000, :8080) and live visual annotation.
              </p>
            </div>
            {renderExtSwitches('browser', 'Preview Browser')}
          </div>

          {/* Extension Settings */}
          {(browserVis.aux_panel || browserVis.main_page) && (
            <div style={{ marginTop: '4px', paddingTop: '12px', borderTop: '1px solid var(--border-subtle)', display: 'flex', flexDirection: 'column', gap: '10px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
                <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                  Default URL:
                </span>
                <input
                  type="text"
                  value={previewUrl}
                  onChange={(e) => {
                    setPreviewUrl(e.target.value)
                    localStorage.setItem('antigravity_browser_default_url', e.target.value)
                  }}
                  style={{
                    flex: 1,
                    minWidth: '220px',
                    fontSize: '12px',
                    padding: '5px 10px',
                    fontFamily: 'monospace',
                    borderRadius: '6px',
                    border: '1px solid var(--border)',
                    backgroundColor: 'var(--canvas)',
                  }}
                  placeholder="http://localhost:5173"
                />
                <div style={{ display: 'flex', gap: '4px' }}>
                  {['5173', '3000', '8080'].map((port) => (
                    <button
                      key={port}
                      onClick={() => {
                        const url = `http://localhost:${port}`
                        setPreviewUrl(url)
                        localStorage.setItem('antigravity_browser_default_url', url)
                      }}
                      className="btn-pill-tonal"
                      style={{ padding: '3px 8px', fontSize: '11px', fontWeight: 600 }}
                    >
                      :{port}
                    </button>
                  ))}
                </div>
              </div>
            </div>
          )}
        </div>

        {/* ========================================================================= */}
        {/* GADGET 3: Auxiliary File Explorer                                         */}
        {/* ========================================================================= */}
        <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: '14px', padding: '18px 20px' }}>
          {/* Header & Toggle Row */}
          <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <Folder size={16} color="var(--text-muted)" />
                <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                  Auxiliary File Explorer
                </h3>
              </div>
              <p style={{ margin: '2px 0 0', fontSize: '12.5px', color: 'var(--text-muted)', lineHeight: 1.45, maxWidth: '780px' }}>
                Lightweight in-panel file browser and editor allowing instant workspace navigation, file reveal, terminal launch, and code inspection.
              </p>
            </div>
            {renderExtSwitches('files', 'Auxiliary File Explorer')}
          </div>

          {/* Extension Settings */}
          {(filesVis.aux_panel || filesVis.main_page) && (
            <div style={{ marginTop: '4px', paddingTop: '12px', borderTop: '1px solid var(--border-subtle)', display: 'flex', flexDirection: 'column', gap: '10px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
                <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                  Preferred IDE:
                </span>
                <select
                  value={ideIsCustom ? 'custom' : preferredIDE.toLowerCase()}
                  onChange={(e) => {
                    if (e.target.value === 'custom') {
                      setIdeCustomMode(true)
                      if (!IDE_PRESETS.includes(preferredIDE.toLowerCase())) {
                        setCustomIDE(preferredIDE)
                      }
                    } else {
                      setIdeCustomMode(false)
                      handleSaveIDE(e.target.value)
                    }
                  }}
                  style={{
                    fontSize: '12px',
                    padding: '5px 12px',
                    borderRadius: '6px',
                    border: '1px solid var(--border)',
                    backgroundColor: 'var(--canvas)',
                    color: 'var(--text)',
                  }}
                >
                  <option value="code">VS Code (`code`)</option>
                  <option value="cursor">Cursor (`cursor`)</option>
                  <option value="windsurf">Windsurf (`windsurf`)</option>
                  <option value="codium">VSCodium (`codium`)</option>
                  <option value="zed">Zed (`zed`)</option>
                  <option value="custom">Custom Command / Binary</option>
                </select>
              </div>

              {ideIsCustom && (
                <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
                  <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                    Command:
                  </span>
                  <input
                    type="text"
                    value={customIDE}
                    onChange={(e) => setCustomIDE(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') handleSaveIDE(customIDE)
                    }}
                    placeholder="e.g. code-insiders, fleet"
                    style={{
                      flex: 1,
                      minWidth: '180px',
                      maxWidth: '320px',
                      fontSize: '12px',
                      padding: '5px 10px',
                      fontFamily: 'monospace',
                      borderRadius: '6px',
                      border: '1px solid var(--border)',
                      backgroundColor: 'var(--canvas)',
                    }}
                  />
                  <button
                    onClick={() => handleSaveIDE(customIDE)}
                    className="btn-pill-primary"
                    style={{ padding: '5px 12px', fontSize: '11.5px' }}
                  >
                    Save IDE
                  </button>
                </div>
              )}
            </div>
          )}
        </div>

        {/* ========================================================================= */}
        {/* GADGET 4: Quick Memos                                                     */}
        {/* ========================================================================= */}
        <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: '14px', padding: '18px 20px' }}>
          {/* Header & Toggle Row */}
          <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <FileText size={16} color="var(--text-muted)" />
                <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                  Quick Memos
                </h3>
              </div>
              <p style={{ margin: '2px 0 0', fontSize: '12.5px', color: 'var(--text-muted)', lineHeight: 1.45, maxWidth: '780px' }}>
                Rapid notepad for ephemeral text and speech-to-text audio memos with instant drag-and-drop into active Antigravity agent conversations.
              </p>
            </div>
            {renderExtSwitches('memos', 'Quick Memos')}
          </div>

          {/* Extension Settings (persisted via daemon /api/memos/config) */}
          {(memosVis.aux_panel || memosVis.main_page) && (
            <div style={{ marginTop: '4px', paddingTop: '12px', borderTop: '1px solid var(--border-subtle)', display: 'flex', flexDirection: 'column', gap: '10px' }}>
              {/* Row 1: Storage Location */}
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
                <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                  Storage Location:
                </span>
                <div style={{ display: 'flex', gap: '4px', backgroundColor: 'var(--tonal)', padding: '2px', borderRadius: '6px' }}>
                  <button
                    onClick={() => handleUpdateMemoConfig({ storage_location: 'global' })}
                    style={segBtnStyle(memoStorageLocation === 'global')}
                  >
                    Global
                  </button>
                  <button
                    onClick={() => handleUpdateMemoConfig({ storage_location: 'project' })}
                    style={segBtnStyle(memoStorageLocation === 'project')}
                  >
                    Project
                  </button>
                </div>
              </div>

              {/* Row 2: View Scope + Search Scope side by side */}
              <div style={{ display: 'flex', alignItems: 'center', gap: '24px', flexWrap: 'wrap' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                    View Scope:
                  </span>
                  <div style={{ display: 'flex', gap: '4px', backgroundColor: 'var(--tonal)', padding: '2px', borderRadius: '6px' }}>
                    <button
                      onClick={() => handleUpdateMemoConfig({ view_scope: 'all' })}
                      style={segBtnStyle(memoViewScope === 'all')}
                    >
                      All Projects
                    </button>
                    <button
                      onClick={() => handleUpdateMemoConfig({ view_scope: 'current' })}
                      style={segBtnStyle(memoViewScope === 'current')}
                    >
                      Current Project
                    </button>
                  </div>
                </div>

                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                    Search Scope:
                  </span>
                  <div style={{ display: 'flex', gap: '4px', backgroundColor: 'var(--tonal)', padding: '2px', borderRadius: '6px' }}>
                    <button
                      onClick={() => handleUpdateMemoConfig({ search_scope: 'text' })}
                      style={segBtnStyle(memoSearchScope === 'text')}
                    >
                      Text
                    </button>
                    <button
                      onClick={() => handleUpdateMemoConfig({ search_scope: 'all' })}
                      style={segBtnStyle(memoSearchScope === 'all')}
                    >
                      Semantic
                    </button>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* ========================================================================= */}
        {/* GADGET 5: Mobile Viewport Simulator                                       */}
        {/* ========================================================================= */}
        <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: '14px', padding: '18px 20px' }}>
          {/* Header & Toggle Row */}
          <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <Smartphone size={16} color="var(--text-muted)" />
                <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                  Mobile Viewport Simulator
                </h3>
              </div>
              <p style={{ margin: '2px 0 0', fontSize: '12.5px', color: 'var(--text-muted)', lineHeight: 1.45, maxWidth: '780px' }}>
                Virtual mobile viewport emulation for testing responsive web designs, touch events, and mobile screen ratios directly within Antigravity.
              </p>
            </div>
            {renderExtSwitches('mobile', 'Mobile Viewport Simulator')}
          </div>

          {/* Extension Settings */}
          {(mobileVis.aux_panel || mobileVis.main_page) && (
            <div style={{ marginTop: '4px', paddingTop: '12px', borderTop: '1px solid var(--border-subtle)', display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '12px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
                <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
                  Default Device:
                </span>
                <select
                  value={selectedDevice}
                  onChange={(e) => {
                    const dev = e.target.value as any
                    setSelectedDevice(dev)
                    localStorage.setItem('antigravity_mobile_default_device', dev)
                  }}
                  style={{
                    fontSize: '12px',
                    padding: '5px 10px',
                    borderRadius: '6px',
                    border: '1px solid var(--border)',
                    backgroundColor: 'var(--canvas)',
                  }}
                >
                  <option value="iphone16">iPhone 16 Pro (393 × 852)</option>
                  <option value="pixel9">Google Pixel 9 (412 × 924)</option>
                  <option value="ipad">iPad Air (820 × 1180)</option>
                </select>
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
                <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', cursor: 'pointer' }}>
                  <ToggleSwitch
                    size="sm"
                    checked={showBezel}
                    onChange={(val) => {
                      setShowBezel(val)
                      localStorage.setItem('antigravity_mobile_show_bezel', String(val))
                    }}
                  />
                  <span>Show Hardware Bezel</span>
                </label>
              </div>
            </div>
          )}
        </div>

        {/* ========================================================================= */}
        {/* GADGET 6: Computer Use Enhancer                                           */}
        {/* ========================================================================= */}
        <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: '14px', padding: '18px 20px' }}>
          {/* Header & Toggle Row */}
          <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <Monitor size={16} color="var(--text-muted)" />
                <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                  Computer Use Enhancer
                </h3>
              </div>
              <p style={{ margin: '2px 0 0', fontSize: '12.5px', color: 'var(--text-muted)', lineHeight: 1.45, maxWidth: '780px' }}>
                OS-level execution enhancer optimizing Antigravity computer use with display coordinate scaling normalization, Wayland PipeWire capture, and accessibility grounding.
              </p>
            </div>
            {renderExtSwitches('computer_use', 'Computer Use Enhancer')}
          </div>

          {/* Extension Settings */}
          {(computerUseVis.aux_panel || computerUseVis.main_page) && (
            <div style={{ marginTop: '4px', paddingTop: '12px', borderTop: '1px solid var(--border-subtle)', display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '12px' }}>
              <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 12px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--canvas)', cursor: 'pointer' }}>
                <div>
                  <div style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>DPI Normalizer</div>
                  <div style={{ fontSize: '10.5px', color: 'var(--text-muted)' }}>Calibrate HiDPI 125%/150% scaling offsets</div>
                </div>
                <ToggleSwitch
                  size="sm"
                  checked={dpiNormalization}
                  onChange={(val) => {
                    setDpiNormalization(val)
                    localStorage.setItem('antigravity_comp_dpi_norm', String(val))
                  }}
                />
              </label>

              <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 12px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--canvas)', cursor: 'pointer' }}>
                <div>
                  <div style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>Wayland PipeWire Stream</div>
                  <div style={{ fontSize: '10.5px', color: 'var(--text-muted)' }}>Capture via desktop portal PipeWire</div>
                </div>
                <ToggleSwitch
                  size="sm"
                  checked={waylandPipeWire}
                  onChange={(val) => {
                    setWaylandPipeWire(val)
                    localStorage.setItem('antigravity_comp_wayland_pipewire', String(val))
                  }}
                />
              </label>

              <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 12px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--canvas)', cursor: 'pointer' }}>
                <div>
                  <div style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>Accessibility Grounding</div>
                  <div style={{ fontSize: '10.5px', color: 'var(--text-muted)' }}>Query OS accessibility tree to save tokens</div>
                </div>
                <ToggleSwitch
                  size="sm"
                  checked={accessibilityGrounding}
                  onChange={(val) => {
                    setAccessibilityGrounding(val)
                    localStorage.setItem('antigravity_comp_accessibility_grounding', String(val))
                  }}
                />
              </label>
            </div>
          )}
        </div>

      </div>
    </div>
  )
}

export { ExtensionsPage as FeaturePluginsPage }
