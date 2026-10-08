import React, { useState } from 'react'
import {
  Globe,
  Folder,
  StickyNote,
  Tablet,
  Crosshair,
  ExternalLink,
  Terminal,
  CheckCircle2,
  ArrowLeft,
} from 'lucide-react'
import { ToggleSwitch } from '../components/ToggleSwitch'
import { api } from '../api'
import { GitHubWorkspacePage } from './GitHubWorkspacePage'

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
  // --- 1. Extension Active Toggles (persisted in localStorage) ---
  const [githubEnabled, setGithubEnabled] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_ext_github_enabled') !== 'false'
  })
  const [browserEnabled, setBrowserEnabled] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_ext_browser_enabled') !== 'false'
  })
  const [filesEnabled, setFilesEnabled] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_ext_files_enabled') !== 'false'
  })
  const [memosEnabled, setMemosEnabled] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_ext_memos_enabled') !== 'false'
  })
  const [mobileEnabled, setMobileEnabled] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_ext_mobile_enabled') !== 'false'
  })
  const [computerUseEnabled, setComputerUseEnabled] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_ext_computer_use_enabled') !== 'false'
  })

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
  const [browserTarget, setBrowserTarget] = useState<'auxiliary' | 'main'>(() => {
    return (localStorage.getItem('antigravity_browser_target') as 'auxiliary' | 'main') || 'auxiliary'
  })

  // File Explorer
  const [preferredIDE, setPreferredIDE] = useState<string>(() => {
    return localStorage.getItem('antigravity_preferred_ide') || 'code'
  })

  // Quick Memos
  const [memoStorageLocation, setMemoStorageLocation] = useState<'global' | 'project'>(() => {
    return (localStorage.getItem('antigravity_memo_storage_location') as 'global' | 'project') || 'global'
  })
  const [memoSearchScope, setMemoSearchScope] = useState<'text' | 'all'>(() => {
    return (localStorage.getItem('antigravity_memo_search_scope') as 'text' | 'all') || 'text'
  })
  const [memoViewScope, setMemoViewScope] = useState<'all' | 'current'>(() => {
    return (localStorage.getItem('antigravity_memo_view_scope') as 'all' | 'current') || 'all'
  })

  // Mobile Simulator
  const [selectedDevice, setSelectedDevice] = useState<'iphone16' | 'pixel9' | 'ipad'>(() => {
    return (localStorage.getItem('antigravity_mobile_default_device') as any) || 'iphone16'
  })
  const [showBezel, setShowBezel] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_mobile_show_bezel') !== 'false'
  })
  const [isLandscape, setIsLandscape] = useState<boolean>(false)

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

  const handleToggleExtension = (key: string, currentVal: boolean, setter: (v: boolean) => void) => {
    const nextVal = !currentVal
    setter(nextVal)
    localStorage.setItem(key, String(nextVal))
    showFeedback(`Extension status updated: ${nextVal ? 'Enabled' : 'Disabled'}`)
  }

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
                <div style={{ padding: '6px', borderRadius: '8px', backgroundColor: 'var(--primary-container)', color: 'var(--primary)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22" />
                  </svg>
                </div>
                <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                  GitHub Workspace
                </h3>
                <span style={{ fontSize: '10.5px', fontWeight: 600, padding: '2px 7px', borderRadius: '10px', backgroundColor: githubEnabled ? '#dcfce7' : 'var(--tonal)', color: githubEnabled ? '#15803d' : 'var(--text-muted)' }}>
                  {githubEnabled ? 'Enabled' : 'Disabled'}
                </span>
              </div>
              <p style={{ margin: '2px 0 0', fontSize: '12.5px', color: 'var(--text-muted)', lineHeight: 1.45, maxWidth: '780px' }}>
                Manage repository issues, pull requests, agent tasks, and Kanban boards with one-click direct jump into Antigravity conversations.
              </p>
            </div>
            <ToggleSwitch
              checked={githubEnabled}
              onChange={() => handleToggleExtension('antigravity_ext_github_enabled', githubEnabled, setGithubEnabled)}
            />
          </div>

          {/* Extension Settings & Actions */}
          {githubEnabled && (
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
                <div style={{ padding: '6px', borderRadius: '8px', backgroundColor: 'rgba(26, 115, 232, 0.1)', color: 'var(--primary)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <Globe size={16} />
                </div>
                <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                  Preview Browser
                </h3>
                <span style={{ fontSize: '10.5px', fontWeight: 600, padding: '2px 7px', borderRadius: '10px', backgroundColor: browserEnabled ? '#dcfce7' : 'var(--tonal)', color: browserEnabled ? '#15803d' : 'var(--text-muted)' }}>
                  {browserEnabled ? 'Enabled' : 'Disabled'}
                </span>
              </div>
              <p style={{ margin: '2px 0 0', fontSize: '12.5px', color: 'var(--text-muted)', lineHeight: 1.45, maxWidth: '780px' }}>
                Embeds a lightweight development browser inside Antigravity's auxiliary panel with port shortcuts (:5173, :3000, :8080) and live visual annotation.
              </p>
            </div>
            <ToggleSwitch
              checked={browserEnabled}
              onChange={() => handleToggleExtension('antigravity_ext_browser_enabled', browserEnabled, setBrowserEnabled)}
            />
          </div>

          {/* Extension Settings */}
          {browserEnabled && (
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

              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
                <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                  Open Target:
                </span>
                <div style={{ display: 'flex', gap: '4px', backgroundColor: 'var(--tonal)', padding: '2px', borderRadius: '6px' }}>
                  <button
                    onClick={() => {
                      setBrowserTarget('auxiliary')
                      localStorage.setItem('antigravity_browser_target', 'auxiliary')
                    }}
                    style={{
                      border: 'none',
                      padding: '4px 10px',
                      borderRadius: '5px',
                      fontSize: '11.5px',
                      fontWeight: browserTarget === 'auxiliary' ? 600 : 500,
                      backgroundColor: browserTarget === 'auxiliary' ? '#ffffff' : 'transparent',
                      color: browserTarget === 'auxiliary' ? 'var(--primary)' : 'var(--text-muted)',
                      boxShadow: browserTarget === 'auxiliary' ? '0 1px 2px rgba(0,0,0,0.06)' : 'none',
                      cursor: 'pointer',
                    }}
                  >
                    Right Auxiliary Panel
                  </button>
                  <button
                    onClick={() => {
                      setBrowserTarget('main')
                      localStorage.setItem('antigravity_browser_target', 'main')
                    }}
                    style={{
                      border: 'none',
                      padding: '4px 10px',
                      borderRadius: '5px',
                      fontSize: '11.5px',
                      fontWeight: browserTarget === 'main' ? 600 : 500,
                      backgroundColor: browserTarget === 'main' ? '#ffffff' : 'transparent',
                      color: browserTarget === 'main' ? 'var(--primary)' : 'var(--text-muted)',
                      boxShadow: browserTarget === 'main' ? '0 1px 2px rgba(0,0,0,0.06)' : 'none',
                      cursor: 'pointer',
                    }}
                  >
                    Main Chat Stage
                  </button>
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
                <div style={{ padding: '6px', borderRadius: '8px', backgroundColor: 'rgba(234, 134, 0, 0.1)', color: '#d97706', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <Folder size={16} />
                </div>
                <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                  Auxiliary File Explorer
                </h3>
                <span style={{ fontSize: '10.5px', fontWeight: 600, padding: '2px 7px', borderRadius: '10px', backgroundColor: filesEnabled ? '#dcfce7' : 'var(--tonal)', color: filesEnabled ? '#15803d' : 'var(--text-muted)' }}>
                  {filesEnabled ? 'Enabled' : 'Disabled'}
                </span>
              </div>
              <p style={{ margin: '2px 0 0', fontSize: '12.5px', color: 'var(--text-muted)', lineHeight: 1.45, maxWidth: '780px' }}>
                Lightweight in-panel file browser and editor allowing instant workspace navigation, file reveal, terminal launch, and code inspection.
              </p>
            </div>
            <ToggleSwitch
              checked={filesEnabled}
              onChange={() => handleToggleExtension('antigravity_ext_files_enabled', filesEnabled, setFilesEnabled)}
            />
          </div>

          {/* Extension Settings */}
          {filesEnabled && (
            <div style={{ marginTop: '4px', paddingTop: '12px', borderTop: '1px solid var(--border-subtle)', display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '12px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
                <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
                  Preferred IDE:
                </span>
                <select
                  value={preferredIDE}
                  onChange={(e) => {
                    const next = e.target.value
                    setPreferredIDE(next)
                    localStorage.setItem('antigravity_preferred_ide', next)
                    api.setPreferredIDE(next).catch(() => {})
                    showFeedback(`Default IDE set to ${getIDEName(next)}`)
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
                  <option value="code">VS Code (code)</option>
                  <option value="cursor">Cursor (cursor)</option>
                  <option value="windsurf">Windsurf (windsurf)</option>
                  <option value="codium">VSCodium (codium)</option>
                  <option value="zed">Zed (zed)</option>
                </select>
              </div>

              <div style={{ display: 'flex', gap: '8px' }}>
                <button
                  onClick={async () => {
                    try {
                      await api.revealFile('.')
                      showFeedback('Project directory opened in system file manager')
                    } catch (e: any) {
                      console.error(e)
                    }
                  }}
                  className="btn-pill-tonal"
                  style={{ padding: '6px 12px', fontSize: '11.5px', display: 'inline-flex', alignItems: 'center', gap: '5px' }}
                >
                  <Folder size={13} />
                  <span>Reveal Folder</span>
                </button>
                <button
                  onClick={async () => {
                    try {
                      await api.openTerminal('.')
                      showFeedback('Terminal launched at project root')
                    } catch (e: any) {
                      console.error(e)
                    }
                  }}
                  className="btn-pill-tonal"
                  style={{ padding: '6px 12px', fontSize: '11.5px', display: 'inline-flex', alignItems: 'center', gap: '5px' }}
                >
                  <Terminal size={13} />
                  <span>Terminal</span>
                </button>
              </div>
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
                <div style={{ padding: '6px', borderRadius: '8px', backgroundColor: 'rgba(5, 150, 105, 0.1)', color: '#059669', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <StickyNote size={16} />
                </div>
                <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                  Quick Memos
                </h3>
                <span style={{ fontSize: '10.5px', fontWeight: 600, padding: '2px 7px', borderRadius: '10px', backgroundColor: memosEnabled ? '#dcfce7' : 'var(--tonal)', color: memosEnabled ? '#15803d' : 'var(--text-muted)' }}>
                  {memosEnabled ? 'Enabled' : 'Disabled'}
                </span>
              </div>
              <p style={{ margin: '2px 0 0', fontSize: '12.5px', color: 'var(--text-muted)', lineHeight: 1.45, maxWidth: '780px' }}>
                Rapid notepad for ephemeral text and speech-to-text audio memos with instant drag-and-drop into active Antigravity agent conversations.
              </p>
            </div>
            <ToggleSwitch
              checked={memosEnabled}
              onChange={() => handleToggleExtension('antigravity_ext_memos_enabled', memosEnabled, setMemosEnabled)}
            />
          </div>

          {/* Extension Settings */}
          {memosEnabled && (
            <div style={{ marginTop: '4px', paddingTop: '12px', borderTop: '1px solid var(--border-subtle)', display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '12px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                  Storage:
                </span>
                <select
                  value={memoStorageLocation}
                  onChange={(e) => {
                    const next = e.target.value as 'global' | 'project'
                    setMemoStorageLocation(next)
                    localStorage.setItem('antigravity_memo_storage_location', next)
                    showFeedback(`Storage set to ${next === 'global' ? 'Global (~/.gemini)' : 'Current Project'}`)
                  }}
                  style={{
                    flex: 1,
                    fontSize: '12px',
                    padding: '5px 10px',
                    borderRadius: '6px',
                    border: '1px solid var(--border)',
                    backgroundColor: 'var(--canvas)',
                  }}
                >
                  <option value="global">Global (~/.gemini/antigravity/memos.json)</option>
                  <option value="project">Per-Project (.antigravity/memos.json)</option>
                </select>
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                  Search Scope:
                </span>
                <select
                  value={memoSearchScope}
                  onChange={(e) => {
                    const next = e.target.value as 'text' | 'all'
                    setMemoSearchScope(next)
                    localStorage.setItem('antigravity_memo_search_scope', next)
                    showFeedback(`Search scope updated to ${next === 'text' ? 'Text Only' : 'Text + Voice'}`)
                  }}
                  style={{
                    flex: 1,
                    fontSize: '12px',
                    padding: '5px 10px',
                    borderRadius: '6px',
                    border: '1px solid var(--border)',
                    backgroundColor: 'var(--canvas)',
                  }}
                >
                  <option value="text">Text Only</option>
                  <option value="all">Text + Voice Transcription</option>
                </select>
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                  View Scope:
                </span>
                <select
                  value={memoViewScope}
                  onChange={(e) => {
                    const next = e.target.value as 'all' | 'current'
                    setMemoViewScope(next)
                    localStorage.setItem('antigravity_memo_view_scope', next)
                    showFeedback(`View scope updated to ${next === 'all' ? 'All Projects' : 'Current Project Only'}`)
                  }}
                  style={{
                    flex: 1,
                    fontSize: '12px',
                    padding: '5px 10px',
                    borderRadius: '6px',
                    border: '1px solid var(--border)',
                    backgroundColor: 'var(--canvas)',
                  }}
                >
                  <option value="all">All Projects</option>
                  <option value="current">Current Project Only</option>
                </select>
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
                <div style={{ padding: '6px', borderRadius: '8px', backgroundColor: 'rgba(124, 58, 237, 0.1)', color: '#7c3aed', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <Tablet size={16} />
                </div>
                <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                  Mobile Viewport Simulator
                </h3>
                <span style={{ fontSize: '10.5px', fontWeight: 600, padding: '2px 7px', borderRadius: '10px', backgroundColor: mobileEnabled ? '#dcfce7' : 'var(--tonal)', color: mobileEnabled ? '#15803d' : 'var(--text-muted)' }}>
                  {mobileEnabled ? 'Enabled' : 'Disabled'}
                </span>
              </div>
              <p style={{ margin: '2px 0 0', fontSize: '12.5px', color: 'var(--text-muted)', lineHeight: 1.45, maxWidth: '780px' }}>
                Virtual mobile viewport emulation for testing responsive web designs, touch events, and mobile screen ratios directly within Antigravity.
              </p>
            </div>
            <ToggleSwitch
              checked={mobileEnabled}
              onChange={() => handleToggleExtension('antigravity_ext_mobile_enabled', mobileEnabled, setMobileEnabled)}
            />
          </div>

          {/* Extension Settings */}
          {mobileEnabled && (
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

                <button
                  onClick={() => setIsLandscape(!isLandscape)}
                  className="btn-pill-tonal"
                  style={{ padding: '4px 10px', fontSize: '11.5px' }}
                >
                  {isLandscape ? 'Portrait' : 'Landscape'}
                </button>
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
                <div style={{ padding: '6px', borderRadius: '8px', backgroundColor: 'rgba(220, 38, 38, 0.1)', color: '#dc2626', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <Crosshair size={16} />
                </div>
                <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                  Computer Use Enhancer
                </h3>
                <span style={{ fontSize: '10.5px', fontWeight: 600, padding: '2px 7px', borderRadius: '10px', backgroundColor: computerUseEnabled ? '#dcfce7' : 'var(--tonal)', color: computerUseEnabled ? '#15803d' : 'var(--text-muted)' }}>
                  {computerUseEnabled ? 'Enabled' : 'Disabled'}
                </span>
              </div>
              <p style={{ margin: '2px 0 0', fontSize: '12.5px', color: 'var(--text-muted)', lineHeight: 1.45, maxWidth: '780px' }}>
                OS-level execution enhancer optimizing Antigravity computer use with display coordinate scaling normalization, Wayland PipeWire capture, and accessibility grounding.
              </p>
            </div>
            <ToggleSwitch
              checked={computerUseEnabled}
              onChange={() => handleToggleExtension('antigravity_ext_computer_use_enabled', computerUseEnabled, setComputerUseEnabled)}
            />
          </div>

          {/* Extension Settings */}
          {computerUseEnabled && (
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
