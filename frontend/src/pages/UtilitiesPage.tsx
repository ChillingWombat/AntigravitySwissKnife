import React, { useState, useEffect } from 'react'
import {
  DownloadCloud,
  RefreshCw,
  CheckCircle2,
  Eye,
  Activity,
  X,
  Monitor,
  Crosshair,
} from 'lucide-react'
import type {
  ChatImportSource,
  ProjectMatchOption,
  ImportCandidate,
  ImportHistoryItem,
  ComputerUseStatus,
  OSComputerUseSettings,
  CalibrationResult,
} from '../types'
import { api } from '../api'
import { ToggleSwitch } from '../components/ToggleSwitch'
import { BrainCachePage } from './BrainCachePage'
import { ACPAgentMeshPage } from './ACPAgentMeshPage'

interface UtilitiesPageProps {
  initialTab?: number
  activeTab?: number
  onTabChange?: (tab: number) => void
}

export const UtilitiesPage: React.FC<UtilitiesPageProps> = ({
  initialTab = 0,
  activeTab: controlledActiveTab,
  onTabChange: _onTabChange,
}) => {
  const [internalActiveTab] = useState<number>(initialTab)
  const activeTab = controlledActiveTab !== undefined ? controlledActiveTab : internalActiveTab

  // --- Computer Use Enhancer State ---
  const [compUseStatus, setCompUseStatus] = useState<ComputerUseStatus | null>(null)
  const [compUseLoading, setCompUseLoading] = useState(false)
  const [selectedPlatformOS, setSelectedPlatformOS] = useState<'linux' | 'windows' | 'darwin'>('linux')
  const [compSettings, setCompSettings] = useState<OSComputerUseSettings>({
    wayland_pipewire: true,
    accessibility_grounding: true,
    linux_dpi_normalizer: true,
    per_monitor_v2_dpi: true,
    windows_graphics_capture: true,
    ui_automation_grounding: true,
    screen_capture_kit: true,
    quartz_retina_normalizer: true,
    ax_accessibility_grounding: true,
  })
  const [calibX, setCalibX] = useState<number>(960)
  const [calibY, setCalibY] = useState<number>(540)
  const [isCalibrating, setIsCalibrating] = useState(false)
  const [calibResult, setCalibResult] = useState<CalibrationResult | null>(null)

  // --- 1. Chat Import State ---
  const [selectedSource, setSelectedSource] = useState<ChatImportSource>('opencode')
  const [projectMatchMode, setProjectMatchMode] = useState<ProjectMatchOption>('auto')
  const [importSyncMode, setImportSyncMode] = useState<'manual' | 'scheduled' | 'watch'>('manual')
  const [isScanning, setIsScanning] = useState(false)
  const [isImporting, setIsImporting] = useState(false)
  const [importProgress, setImportProgress] = useState(0)
  const [importFeedback, setImportFeedback] = useState<string | null>(null)
  const [previewCandidate, setPreviewCandidate] = useState<ImportCandidate | null>(null)

  // Real Discovered Candidates from Source Agent
  const [candidates, setCandidates] = useState<ImportCandidate[]>([])

  // Past Import Logs (persisted in localStorage)
  const [history, setHistory] = useState<ImportHistoryItem[]>(() => {
    try {
      const saved = localStorage.getItem('antigravity_import_history')
      return saved ? JSON.parse(saved) : []
    } catch {
      return []
    }
  })

  const loadCandidates = async (src: string) => {
    setIsScanning(true)
    try {
      const res = await api.scanImportCandidates(src)
      if (res && res.candidates) {
        setCandidates(res.candidates)
      } else {
        setCandidates([])
      }
    } catch (e) {
      console.error('Error scanning candidates:', e)
      setCandidates([])
    } finally {
      setIsScanning(false)
    }
  }

  useEffect(() => {
    loadCandidates(selectedSource)
  }, [selectedSource])

  const loadComputerUseStatus = async () => {
    setCompUseLoading(true)
    try {
      const res = await api.getComputerUseStatus()
      if (res && res.data) {
        setCompUseStatus(res.data)
        if (res.data.settings) {
          setCompSettings(res.data.settings)
        }
        if (res.data.current_os) {
          const detected = res.data.current_os.toLowerCase()
          if (detected === 'windows') setSelectedPlatformOS('windows')
          else if (detected === 'darwin') setSelectedPlatformOS('darwin')
          else setSelectedPlatformOS('linux')
        }
      }
    } catch (e) {
      console.error('Error fetching computer use status:', e)
    } finally {
      setCompUseLoading(false)
    }
  }

  useEffect(() => {
    if (activeTab === 3) {
      loadComputerUseStatus()
    }
  }, [activeTab])

  const handleToggleCompSetting = async (key: keyof OSComputerUseSettings, val: boolean) => {
    const updated = { ...compSettings, [key]: val }
    setCompSettings(updated)
    try {
      await api.saveComputerUseSettings(updated)
    } catch (e) {
      console.error('Error saving computer use settings:', e)
    }
  }

  const handleRunCalibration = async () => {
    setIsCalibrating(true)
    try {
      const res = await api.calibrateComputerUse(selectedPlatformOS, calibX, calibY)
      if (res && res.data) {
        setCalibResult(res.data)
      }
    } catch (e) {
      console.error('Error calibrating coordinates:', e)
    } finally {
      setIsCalibrating(false)
    }
  }

  // Trigger Real Import Execution
  const handleExecuteImport = async () => {
    const selectedCands = candidates.filter((c) => c.selected)
    const selectedIds = selectedCands.map((c) => c.id)
    if (selectedIds.length === 0) return

    setIsImporting(true)
    setImportProgress(25)
    try {
      const res = await api.executeImport(selectedIds, selectedSource, projectMatchMode)
      setImportProgress(100)
      setIsImporting(false)
      const msg = res.message || `Successfully imported ${selectedIds.length} conversations into Antigravity!`
      setImportFeedback(msg)

      const newHistItem: ImportHistoryItem = {
        id: `hist-${Date.now()}`,
        timestamp: new Date().toLocaleString(),
        source: selectedSource,
        conversation_count: selectedIds.length,
        target_project: selectedCands[0]?.target_antigravity_project || 'Antigravity Workspace',
        status: 'completed',
        duration_ms: 380,
      }
      const updated = [newHistItem, ...history]
      setHistory(updated)
      try {
        localStorage.setItem('antigravity_import_history', JSON.stringify(updated))
      } catch {}
      setTimeout(() => setImportFeedback(null), 5000)
    } catch (err: any) {
      setIsImporting(false)
      setImportFeedback(`Import failed: ${err?.message || 'Unknown error'}`)
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* ============================================================ */}
      {/* TAB 0: CACHE MANAGER & CONVERSATION VAULT */}
      {/* ============================================================ */}
      {activeTab === 0 && (
        <BrainCachePage />
      )}

      {/* ============================================================ */}
      {/* TAB 2: AGENT CHAT & PROJECT IMPORTER */}
      {/* ============================================================ */}
      {activeTab === 2 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          {importFeedback && (
            <div
              style={{
                backgroundColor: '#e6f4ea',
                color: '#137333',
                border: '1px solid #ceead6',
                borderRadius: '10px',
                padding: '10px 16px',
                fontSize: '13px',
                display: 'flex',
                alignItems: 'center',
                gap: '8px',
              }}
            >
              <CheckCircle2 size={16} />
              {importFeedback}
            </div>
          )}

          {/* Import Setup Wizard Card */}
          <div
            style={{
              backgroundColor: '#ffffff',
              border: '1px solid var(--border)',
              borderRadius: '10px',
              padding: '20px',
            }}
          >
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
                gap: '16px',
                marginBottom: '20px',
              }}
            >
              {/* Source Selector */}
              <div>
                <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '6px' }}>
                  Source Agent Application
                </label>
                <select
                  value={selectedSource}
                  onChange={(e) => setSelectedSource(e.target.value as ChatImportSource)}
                  style={{
                    width: '100%',
                    padding: '9px 12px',
                    borderRadius: '8px',
                    border: '1px solid var(--border)',
                    fontSize: '14px',
                    fontWeight: 600,
                    backgroundColor: '#ffffff',
                    color: 'var(--text)',
                  }}
                >
                  <option value="opencode">OpenCode Interpreter / Go (~/.local/share/opencode/opencode.db)</option>
                  <option value="dsh">DeepSeek Harness / DSH (~/.dsh/sessions/)</option>
                  <option value="devin">Devin CLI & Desktop (~/.local/share/devin/cli/sessions.db)</option>
                  <option value="pi">Pi Agent (~/.pi/agent)</option>
                  <option value="cursor">Cursor Composer / Agent (state.vscdb)</option>
                  <option value="chatgpt">ChatGPT Data Export (conversations.json)</option>
                  <option value="copilot">GitHub Copilot Workspace</option>
                  <option value="openwebui">Open-WebUI / Ollama Chat Export</option>
                  <option value="custom-file">Custom File / Folder (JSON, JSONL, DB, ZIP)</option>
                  <option value="claude-code">Claude Code (~/.claude/transcripts)</option>
                  <option value="raw-json">Raw JSON / Markdown Clipboard Paste</option>
                </select>
              </div>

              {/* Project Destination Strategy */}
              <div>
                <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '6px' }}>
                  Unmatched Project Strategy
                </label>
                <select
                  value={projectMatchMode}
                  onChange={(e) => setProjectMatchMode(e.target.value as ProjectMatchOption)}
                  style={{
                    width: '100%',
                    padding: '9px 12px',
                    borderRadius: '8px',
                    border: '1px solid var(--border)',
                    fontSize: '14px',
                    fontWeight: 600,
                    backgroundColor: '#ffffff',
                    color: 'var(--text)',
                  }}
                >
                  <option value="auto">Auto-Match Workspace URI & Git Origin</option>
                  <option value="create-new">Re-create New Projects for each detected path</option>
                  <option value="standalone">Import as Standalone Chats (No Project association)</option>
                  <option value="single-dedicated">Group all into '[Imported] Shared Archive'</option>
                  <option value="source-dedicated">Group into Source Folders (e.g. '[Imported] OpenCode')</option>
                </select>
              </div>

              {/* Sync Schedule Mode */}
              <div>
                <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '6px' }}>
                  Execution & Background Sync
                </label>
                <div style={{ display: 'flex', gap: '8px' }}>
                  {(['manual', 'scheduled', 'watch'] as const).map((mode) => (
                    <button
                      key={mode}
                      onClick={() => setImportSyncMode(mode)}
                      style={{
                        flex: 1,
                        padding: '8px 10px',
                        borderRadius: '8px',
                        fontSize: '13px',
                        fontWeight: importSyncMode === mode ? 700 : 600,
                        backgroundColor: importSyncMode === mode ? '#e8f0fe' : '#ffffff',
                        color: importSyncMode === mode ? 'var(--primary)' : 'var(--text-muted)',
                        border: `1px solid ${importSyncMode === mode ? 'var(--primary)' : 'var(--border)'}`,
                        cursor: 'pointer',
                        textTransform: 'capitalize',
                      }}
                    >
                      {mode}
                    </button>
                  ))}
                </div>
              </div>
            </div>

            {/* Sync Mode Details Note */}
            {importSyncMode === 'watch' && (
              <div
                style={{
                  backgroundColor: '#fef7e0',
                  border: '1px solid #feefc3',
                  borderRadius: '10px',
                  padding: '10px 14px',
                  fontSize: '12px',
                  color: '#b06000',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '8px',
                  marginBottom: '16px',
                }}
              >
                <Activity size={15} />
                <span>
                  <b>Continuous Watch Enabled:</b> Swiss Knife daemon will listen to filesystem events (inotify/WAL changes) on <code>{selectedSource}</code> and automatically mirror new conversations into your Antigravity project as they are completed.
                </span>
              </div>
            )}

            {/* Candidate Review Table */}
            <div style={{ borderTop: '1px solid var(--border)', paddingTop: '16px' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
                <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                  Detected Conversations ({candidates.length} found)
                </span>
                <div style={{ display: 'flex', gap: '8px' }}>
                  <button
                    onClick={() => loadCandidates(selectedSource)}
                    disabled={isScanning}
                    style={{
                      backgroundColor: 'transparent',
                      border: '1px solid var(--border)',
                      borderRadius: '6px',
                      padding: '4px 12px',
                      fontSize: '12px',
                      color: 'var(--text)',
                      cursor: isScanning ? 'not-allowed' : 'pointer',
                      display: 'flex',
                      alignItems: 'center',
                      gap: '4px',
                    }}
                  >
                    <RefreshCw size={12} className={isScanning ? 'animate-spin' : ''} />
                    {isScanning ? 'Scanning...' : 'Rescan'}
                  </button>
                  <button
                    onClick={() => setCandidates(candidates.map((c) => ({ ...c, selected: !candidates.every((x) => x.selected) })))}
                    style={{
                      backgroundColor: 'transparent',
                      border: '1px solid var(--border)',
                      borderRadius: '6px',
                      padding: '4px 12px',
                      fontSize: '12px',
                      color: 'var(--text)',
                      cursor: 'pointer',
                    }}
                  >
                    {candidates.every((x) => x.selected) ? 'Deselect All' : 'Select All'}
                  </button>
                </div>
              </div>

              <div
                style={{
                  overflowX: 'auto',
                  ...(candidates.length > 0 ? { height: '420px', overflowY: 'auto' } : {}),
                }}
              >
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '13px' }}>
                  <thead>
                    <tr style={{ borderBottom: '1px solid var(--border)', textAlign: 'left', color: 'var(--text-muted)' }}>
                      <th style={{ padding: '8px 10px', width: '30px', position: 'sticky', top: 0, backgroundColor: '#ffffff', zIndex: 1 }}>
                        <input
                          type="checkbox"
                          checked={candidates.length > 0 && candidates.every((c) => c.selected)}
                          onChange={(e) => setCandidates(candidates.map((c) => ({ ...c, selected: e.target.checked })))}
                        />
                      </th>
                      <th style={{ padding: '8px 10px', position: 'sticky', top: 0, backgroundColor: '#ffffff', zIndex: 1 }}>Conversation Title</th>
                      <th style={{ padding: '8px 10px', position: 'sticky', top: 0, backgroundColor: '#ffffff', zIndex: 1 }}>Source</th>
                      <th style={{ padding: '8px 10px', position: 'sticky', top: 0, backgroundColor: '#ffffff', zIndex: 1 }}>Turns & Tools</th>
                      <th style={{ padding: '8px 10px', position: 'sticky', top: 0, backgroundColor: '#ffffff', zIndex: 1 }}>Target Destination</th>
                      <th style={{ padding: '8px 10px', position: 'sticky', top: 0, backgroundColor: '#ffffff', zIndex: 1 }}>Match Status</th>
                      <th style={{ padding: '8px 10px', textAlign: 'right', position: 'sticky', top: 0, backgroundColor: '#ffffff', zIndex: 1 }}>Preview</th>
                    </tr>
                  </thead>
                  <tbody>
                    {candidates.length === 0 ? (
                      <tr>
                        <td colSpan={7} style={{ padding: '32px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '13px' }}>
                          {isScanning
                            ? 'Scanning local agent directories for conversations...'
                            : `No conversations found in ${selectedSource}. Click Rescan or select another source agent.`}
                        </td>
                      </tr>
                    ) : (
                      candidates.map((cand) => (
                      <tr key={cand.id} style={{ borderBottom: '1px solid #f1f3f4' }}>
                        <td style={{ padding: '10px 10px' }}>
                          <input
                            type="checkbox"
                            checked={cand.selected}
                            onChange={(e) =>
                              setCandidates(
                                candidates.map((c) => (c.id === cand.id ? { ...c, selected: e.target.checked } : c))
                              )
                            }
                          />
                        </td>
                        <td style={{ padding: '10px 10px', fontWeight: 600, color: 'var(--text)' }}>
                          {cand.title}
                        </td>
                        <td style={{ padding: '10px 10px', textTransform: 'capitalize' }}>
                          <span
                            style={{
                              padding: '2px 8px',
                              borderRadius: '10px',
                              fontSize: '11px',
                              fontWeight: 600,
                              backgroundColor: cand.source === 'claude-code' ? '#fef3c7' : '#e8f0fe',
                              color: cand.source === 'claude-code' ? '#b45309' : 'var(--primary)',
                            }}
                          >
                            {cand.source}
                          </span>
                        </td>
                        <td style={{ padding: '10px 10px', color: 'var(--text-muted)' }}>
                          {cand.message_count} msgs • {cand.tool_calls_count} tools ({Math.round(cand.token_estimate / 1000)}k tok)
                        </td>
                        <td style={{ padding: '10px 10px', fontWeight: 500, color: 'var(--text)' }}>
                          {cand.target_antigravity_project}
                        </td>
                        <td style={{ padding: '10px 10px' }}>
                          <span
                            style={{
                              fontSize: '11px',
                              padding: '2px 8px',
                              borderRadius: '10px',
                              fontWeight: 600,
                              backgroundColor:
                                cand.match_status === 'exact'
                                  ? '#e6f4ea'
                                  : cand.match_status === 'new'
                                  ? '#e8f0fe'
                                  : '#f1f3f4',
                              color:
                                cand.match_status === 'exact'
                                  ? '#137333'
                                  : cand.match_status === 'new'
                                  ? 'var(--primary)'
                                  : '#5f6368',
                            }}
                          >
                            {cand.match_status === 'exact'
                              ? 'Exact Workspace Match'
                              : cand.match_status === 'new'
                              ? '+ Auto-Create Project'
                              : 'Standalone Chat'}
                          </span>
                        </td>
                        <td style={{ padding: '10px 10px', textAlign: 'right' }}>
                          <button
                            onClick={() => setPreviewCandidate(cand)}
                            style={{
                              backgroundColor: 'transparent',
                              border: 'none',
                              color: 'var(--primary)',
                              fontSize: '12px',
                              cursor: 'pointer',
                              display: 'inline-flex',
                              alignItems: 'center',
                              gap: '4px',
                            }}
                          >
                            <Eye size={13} />
                            Inspect
                          </button>
                        </td>
                      </tr>
                    ))
                  )}
                  </tbody>
                </table>
              </div>

              {/* Action Buttons */}
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  marginTop: '16px',
                  paddingTop: '14px',
                  borderTop: '1px solid var(--border)',
                }}
              >
                <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                  Selected {candidates.filter((c) => c.selected).length} of {candidates.length} conversations to import
                </span>

                <button
                  onClick={handleExecuteImport}
                  disabled={isImporting || candidates.filter((c) => c.selected).length === 0}
                  style={{
                    backgroundColor: 'var(--primary)',
                    color: '#ffffff',
                    border: 'none',
                    borderRadius: '6px',
                    padding: '8px 20px',
                    fontSize: '13px',
                    fontWeight: 600,
                    cursor: isImporting ? 'not-allowed' : 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                    boxShadow: '0 1px 3px rgba(0,0,0,0.12)',
                  }}
                >
                  <DownloadCloud size={16} />
                  {isImporting ? `Importing (${importProgress}%)...` : 'Start Import into Antigravity'}
                </button>
              </div>
            </div>
          </div>

          {/* Import History Table */}
          <div
            style={{
              backgroundColor: '#ffffff',
              border: '1px solid var(--border)',
              borderRadius: '10px',
              padding: '20px',
            }}
          >
            <h3 style={{ fontSize: '15px', fontWeight: 700, margin: '0 0 12px 0', color: 'var(--text)' }}>
              Recent Migration History
            </h3>
            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '12px' }}>
                <thead>
                  <tr style={{ borderBottom: '1px solid var(--border)', textAlign: 'left', color: 'var(--text-muted)' }}>
                    <th style={{ padding: '8px 10px' }}>Date</th>
                    <th style={{ padding: '8px 10px' }}>Source Agent</th>
                    <th style={{ padding: '8px 10px' }}>Conversations</th>
                    <th style={{ padding: '8px 10px' }}>Target Project</th>
                    <th style={{ padding: '8px 10px' }}>Duration</th>
                    <th style={{ padding: '8px 10px' }}>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {history.length === 0 ? (
                    <tr>
                      <td colSpan={6} style={{ padding: '24px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '12px' }}>
                        No previous migration runs recorded. Run an import above to migrate chats.
                      </td>
                    </tr>
                  ) : (
                    history.map((h) => (
                      <tr key={h.id} style={{ borderBottom: '1px solid #f1f3f4' }}>
                        <td style={{ padding: '8px 10px', color: 'var(--text-muted)' }}>{h.timestamp}</td>
                        <td style={{ padding: '8px 10px', fontWeight: 600, color: 'var(--text)' }}>{h.source}</td>
                        <td style={{ padding: '8px 10px' }}>{h.conversation_count} conversations</td>
                        <td style={{ padding: '8px 10px' }}>{h.target_project}</td>
                        <td style={{ padding: '8px 10px', color: 'var(--text-muted)' }}>{h.duration_ms}ms</td>
                        <td style={{ padding: '8px 10px' }}>
                          <span
                            style={{
                              backgroundColor: '#e6f4ea',
                              color: '#137333',
                              padding: '2px 8px',
                              borderRadius: '10px',
                              fontSize: '11px',
                              fontWeight: 600,
                            }}
                          >
                            Completed
                          </span>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>

          {/* Inspect Candidate Modal */}
          {previewCandidate && (
            <div
              style={{
                position: 'fixed',
                top: 0,
                left: 0,
                right: 0,
                bottom: 0,
                backgroundColor: 'rgba(0,0,0,0.45)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                zIndex: 1000,
              }}
            >
              <div
                style={{
                  backgroundColor: '#ffffff',
                  borderRadius: '10px',
                  padding: '24px',
                  width: '540px',
                  maxHeight: '80vh',
                  overflowY: 'auto',
                  boxShadow: '0 8px 30px rgba(0,0,0,0.18)',
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px' }}>
                  <h3 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
                    Conversation Preview & Metadata
                  </h3>
                  <button
                    onClick={() => setPreviewCandidate(null)}
                    style={{
                      background: 'transparent',
                      border: 'none',
                      cursor: 'pointer',
                      color: 'var(--text-muted)',
                      width: '26px',
                      height: '26px',
                      borderRadius: '6px',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      padding: 0,
                    }}
                    title="Close"
                  >
                    <X size={16} />
                  </button>
                </div>

                <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', fontSize: '13px' }}>
                  <div>
                    <span style={{ fontWeight: 600, color: 'var(--text-muted)' }}>Title:</span>
                    <div style={{ fontWeight: 600, color: 'var(--text)', marginTop: '2px' }}>{previewCandidate.title}</div>
                  </div>

                  <div>
                    <span style={{ fontWeight: 600, color: 'var(--text-muted)' }}>Detected Project Path:</span>
                    <div style={{ fontFamily: 'monospace', fontSize: '12px', color: 'var(--text)', marginTop: '2px' }}>
                      {previewCandidate.detected_project_path || 'None (General discussion)'}
                    </div>
                  </div>

                  <div>
                    <span style={{ fontWeight: 600, color: 'var(--text-muted)' }}>Target Antigravity Mapping:</span>
                    <div style={{ color: 'var(--primary)', fontWeight: 600, marginTop: '2px' }}>
                      {previewCandidate.target_antigravity_project}
                    </div>
                  </div>

                  <div
                    style={{
                      backgroundColor: '#f8f9fa',
                      border: '1px solid var(--border)',
                      borderRadius: '8px',
                      padding: '12px',
                      fontFamily: 'monospace',
                      fontSize: '12px',
                      maxHeight: '180px',
                      overflowY: 'auto',
                    }}
                  >
                    <div><b>Session ID:</b> {previewCandidate.id}</div>
                    <div style={{ marginTop: '6px' }}><b>Session Title:</b> {previewCandidate.title}</div>
                    <div style={{ marginTop: '6px' }}><b>Turns / Activity:</b> {previewCandidate.message_count} messages, {previewCandidate.tool_calls_count} tool calls</div>
                    <div style={{ marginTop: '6px' }}><b>Token Estimate:</b> ~{previewCandidate.token_estimate.toLocaleString()} tokens</div>
                    <div style={{ marginTop: '6px' }}><b>Format:</b> {previewCandidate.source} ({previewCandidate.match_status})</div>
                  </div>
                </div>

                <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '20px' }}>
                  <button
                    onClick={() => setPreviewCandidate(null)}
                    style={{
                      backgroundColor: 'var(--primary)',
                      border: 'none',
                      borderRadius: '6px',
                      padding: '7px 18px',
                      fontSize: '13px',
                      fontWeight: 600,
                      color: '#ffffff',
                      cursor: 'pointer',
                    }}
                  >
                    Done
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* ============================================================ */}
      {/* TAB 1: ACP (AGENT CLIENT PROTOCOL) AGENT MESH */}
      {/* ============================================================ */}
      {activeTab === 1 && (
        <ACPAgentMeshPage />
      )}

      {/* 4. Computer Use Tab */}
      {activeTab === 3 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '18px' }}>
          {/* Main Card */}
          <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: '16px', padding: '20px 24px' }}>
            <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px', flexWrap: 'wrap' }}>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <Monitor size={18} color="var(--primary)" />
                  <h3 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
                    Computer Use Enhancer
                  </h3>
                </div>
                <p style={{ margin: '2px 0 0', fontSize: '13px', color: 'var(--text-muted)', lineHeight: 1.5, maxWidth: '820px' }}>
                  Cross-platform execution enhancer optimizing Antigravity computer use with display coordinate scaling normalization, native screen capture pipelines, and token-saving accessibility tree grounding across Linux, Windows, and macOS.
                </p>
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '6px',
                    padding: '4px 10px',
                    borderRadius: '20px',
                    fontSize: '12px',
                    fontWeight: 600,
                    backgroundColor: '#e6f4ea',
                    color: '#137333',
                    border: '1px solid #ceead6',
                    whiteSpace: 'nowrap',
                  }}
                >
                  <span
                    style={{
                      width: '7px',
                      height: '7px',
                      borderRadius: '50%',
                      backgroundColor: '#137333',
                      display: 'inline-block',
                    }}
                  />
                  <span>Host: {compUseStatus?.current_os ? compUseStatus.current_os.toUpperCase() : 'LINUX'}</span>
                </div>

                <button
                  onClick={loadComputerUseStatus}
                  disabled={compUseLoading}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '6px',
                    padding: '6px 12px',
                    borderRadius: '6px',
                    border: '1px solid var(--border)',
                    backgroundColor: '#ffffff',
                    color: 'var(--text)',
                    fontSize: '12px',
                    fontWeight: 600,
                    cursor: compUseLoading ? 'not-allowed' : 'pointer',
                    whiteSpace: 'nowrap',
                  }}
                  title="Reload live OS diagnostics"
                >
                  <RefreshCw size={13} style={{ animation: compUseLoading ? 'spin 1s linear infinite' : 'none' }} />
                  <span>Refresh</span>
                </button>
              </div>
            </div>

            {/* Operating System Selector Tabs */}
            <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', borderBottom: '1px solid var(--border)', paddingBottom: '12px' }}>
              {(
                [
                  { id: 'linux', label: 'Linux (Wayland & X11)' },
                  { id: 'windows', label: 'Windows 11 / 10 (DWM)' },
                  { id: 'darwin', label: 'macOS (Retina & SCK)' },
                ] as const
              ).map((tab) => {
                const isSelected = selectedPlatformOS === tab.id
                const isHost = compUseStatus?.current_os === tab.id
                return (
                  <button
                    key={tab.id}
                    onClick={() => setSelectedPlatformOS(tab.id)}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '8px',
                      padding: '8px 16px',
                      borderRadius: '8px',
                      border: isSelected ? '1px solid var(--primary)' : '1px solid var(--border)',
                      backgroundColor: isSelected ? 'rgba(26, 115, 232, 0.08)' : 'var(--canvas)',
                      color: isSelected ? 'var(--primary)' : 'var(--text)',
                      fontSize: '13px',
                      fontWeight: isSelected ? 700 : 500,
                      cursor: 'pointer',
                      whiteSpace: 'nowrap',
                      transition: 'all 0.15s ease',
                    }}
                  >
                    <span>{tab.label}</span>
                    {isHost && (
                      <span
                        style={{
                          fontSize: '11px',
                          fontWeight: 600,
                          padding: '1px 6px',
                          borderRadius: '10px',
                          backgroundColor: '#e6f4ea',
                          color: '#137333',
                        }}
                      >
                        Active Host
                      </span>
                    )}
                  </button>
                )
              })}
            </div>

            {/* Diagnostic Badges for Selected Platform */}
            {(() => {
              const spec = compUseStatus?.profiles?.[selectedPlatformOS] || compUseStatus?.host_spec
              const pipeline = spec?.display_pipeline || (selectedPlatformOS === 'linux' ? 'Wayland Compositor (Xwayland Rootless)' : selectedPlatformOS === 'windows' ? 'Desktop Window Manager (DWM) Per-Monitor V2' : 'Quartz Display Services with Retina 2.0x')
              const capture = spec?.screen_capture_backend || (selectedPlatformOS === 'linux' ? 'xdg-desktop-portal / PipeWire Stream' : selectedPlatformOS === 'windows' ? 'Windows Graphics Capture (WGC) & DXGI' : 'ScreenCaptureKit (SCK) Zero-Copy Stream')
              const grounding = spec?.grounding_backend || (selectedPlatformOS === 'linux' ? 'AT-SPI2 D-Bus Accessibility Tree' : selectedPlatformOS === 'windows' ? 'Windows UI Automation (UIA) COM Patterns' : 'macOS AXUIElement Accessibility Hierarchy')
              const scale = spec?.dpi_scaling_factor || (selectedPlatformOS === 'darwin' ? 2.0 : selectedPlatformOS === 'windows' ? 1.25 : 1.0)
              const resolution = spec?.display_resolution || (selectedPlatformOS === 'darwin' ? '2880x1800 (Retina Display)' : selectedPlatformOS === 'windows' ? '2560x1440 (Primary High-DPI)' : '1920x1080 (Primary)')

              return (
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap', padding: '10px 14px', backgroundColor: 'var(--canvas)', borderRadius: '8px', border: '1px solid var(--border)' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', color: 'var(--text-muted)' }}>
                    <CheckCircle2 size={13} color="var(--green)" />
                    <span>Display Pipeline: <strong>{pipeline}</strong></span>
                  </div>
                  <span style={{ color: 'var(--border)' }}>•</span>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', color: 'var(--text-muted)' }}>
                    <CheckCircle2 size={13} color="var(--green)" />
                    <span>Screen Capture: <strong>{capture}</strong></span>
                  </div>
                  <span style={{ color: 'var(--border)' }}>•</span>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', color: 'var(--text-muted)' }}>
                    <CheckCircle2 size={13} color="var(--green)" />
                    <span>Grounding: <strong>{grounding}</strong></span>
                  </div>
                  <span style={{ color: 'var(--border)' }}>•</span>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', color: 'var(--text-muted)' }}>
                    <CheckCircle2 size={13} color="var(--green)" />
                    <span>Resolution: <strong>{resolution} ({scale}x Scale)</strong></span>
                  </div>
                </div>
              )
            })()}

            {/* Setting Cards for Selected Platform */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '14px' }}>
              {selectedPlatformOS === 'linux' && (
                <>
                  <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 16px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--canvas)', cursor: 'pointer' }}>
                    <div style={{ paddingRight: '12px' }}>
                      <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>HiDPI Fractional Normalizer</div>
                      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                        Calibrate fractional scaling offsets (125%/150%) so click and drag coordinates target exact pixel boundaries.
                      </div>
                    </div>
                    <ToggleSwitch
                      size="sm"
                      checked={compSettings.linux_dpi_normalizer}
                      onChange={(val) => handleToggleCompSetting('linux_dpi_normalizer', val)}
                      ariaLabel="Toggle HiDPI Fractional Normalizer"
                    />
                  </label>

                  <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 16px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--canvas)', cursor: 'pointer' }}>
                    <div style={{ paddingRight: '12px' }}>
                      <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>Wayland PipeWire Stream</div>
                      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                        Capture frames directly via xdg-desktop-portal PipeWire streams under modern Wayland compositors.
                      </div>
                    </div>
                    <ToggleSwitch
                      size="sm"
                      checked={compSettings.wayland_pipewire}
                      onChange={(val) => handleToggleCompSetting('wayland_pipewire', val)}
                      ariaLabel="Toggle Wayland PipeWire Stream"
                    />
                  </label>

                  <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 16px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--canvas)', cursor: 'pointer' }}>
                    <div style={{ paddingRight: '12px' }}>
                      <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>AT-SPI2 Accessibility Grounding</div>
                      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                        Query OS accessibility tree via D-Bus to locate UI elements deterministically without burning vision tokens.
                      </div>
                    </div>
                    <ToggleSwitch
                      size="sm"
                      checked={compSettings.accessibility_grounding}
                      onChange={(val) => handleToggleCompSetting('accessibility_grounding', val)}
                      ariaLabel="Toggle Accessibility Grounding"
                    />
                  </label>
                </>
              )}

              {selectedPlatformOS === 'windows' && (
                <>
                  <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 16px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--canvas)', cursor: 'pointer' }}>
                    <div style={{ paddingRight: '12px' }}>
                      <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>Per-Monitor V2 DPI Awareness</div>
                      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                        Enforce Per-Monitor V2 scaling context so input injection coordinates scale correctly across mixed-DPI displays.
                      </div>
                    </div>
                    <ToggleSwitch
                      size="sm"
                      checked={compSettings.per_monitor_v2_dpi}
                      onChange={(val) => handleToggleCompSetting('per_monitor_v2_dpi', val)}
                      ariaLabel="Toggle Per-Monitor V2 DPI Awareness"
                    />
                  </label>

                  <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 16px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--canvas)', cursor: 'pointer' }}>
                    <div style={{ paddingRight: '12px' }}>
                      <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>Windows Graphics Capture (WGC)</div>
                      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                        Capture hardware-accelerated desktop buffers via modern Windows.Graphics.Capture API instead of legacy BitBlt.
                      </div>
                    </div>
                    <ToggleSwitch
                      size="sm"
                      checked={compSettings.windows_graphics_capture}
                      onChange={(val) => handleToggleCompSetting('windows_graphics_capture', val)}
                      ariaLabel="Toggle Windows Graphics Capture"
                    />
                  </label>

                  <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 16px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--canvas)', cursor: 'pointer' }}>
                    <div style={{ paddingRight: '12px' }}>
                      <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>UI Automation COM Grounding</div>
                      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                        Inspect UIAutomationCore COM element tree to identify clickable controls by AutomationId and Name without vision cost.
                      </div>
                    </div>
                    <ToggleSwitch
                      size="sm"
                      checked={compSettings.ui_automation_grounding}
                      onChange={(val) => handleToggleCompSetting('ui_automation_grounding', val)}
                      ariaLabel="Toggle UI Automation COM Grounding"
                    />
                  </label>
                </>
              )}

              {selectedPlatformOS === 'darwin' && (
                <>
                  <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 16px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--canvas)', cursor: 'pointer' }}>
                    <div style={{ paddingRight: '12px' }}>
                      <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>Quartz Retina Coordinate Normalizer</div>
                      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                        Normalize 2.0x Retina points to hardware raster pixels with inverted Cocoa bottom-left coordinate matrix transform.
                      </div>
                    </div>
                    <ToggleSwitch
                      size="sm"
                      checked={compSettings.quartz_retina_normalizer}
                      onChange={(val) => handleToggleCompSetting('quartz_retina_normalizer', val)}
                      ariaLabel="Toggle Quartz Retina Coordinate Normalizer"
                    />
                  </label>

                  <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 16px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--canvas)', cursor: 'pointer' }}>
                    <div style={{ paddingRight: '12px' }}>
                      <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>ScreenCaptureKit Zero-Copy Stream</div>
                      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                        Stream display frames via macOS ScreenCaptureKit (SCK) with zero memory copy and Metal hardware optimization.
                      </div>
                    </div>
                    <ToggleSwitch
                      size="sm"
                      checked={compSettings.screen_capture_kit}
                      onChange={(val) => handleToggleCompSetting('screen_capture_kit', val)}
                      ariaLabel="Toggle ScreenCaptureKit Stream"
                    />
                  </label>

                  <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 16px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--canvas)', cursor: 'pointer' }}>
                    <div style={{ paddingRight: '12px' }}>
                      <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>AXUIElement Accessibility Grounding</div>
                      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                        Query macOS Accessibility framework hierarchy to locate interactive buttons and textfields by AXRole and AXTitle.
                      </div>
                    </div>
                    <ToggleSwitch
                      size="sm"
                      checked={compSettings.ax_accessibility_grounding}
                      onChange={(val) => handleToggleCompSetting('ax_accessibility_grounding', val)}
                      ariaLabel="Toggle AXUIElement Accessibility Grounding"
                    />
                  </label>
                </>
              )}
            </div>
          </div>

          {/* Real Interactive Coordinate Calibration Card */}
          <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: '14px', padding: '18px 22px' }}>
            <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px', flexWrap: 'wrap' }}>
              <div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <Crosshair size={16} color="var(--primary)" />
                  <h4 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                    Display Coordinate Normalization Probe
                  </h4>
                </div>
                <p style={{ margin: '4px 0 0', fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
                  Verify logical-to-physical coordinate transformation on {selectedPlatformOS.toUpperCase()}. Ensures open-computer-use click and drag actions target exact pixel coordinates without scaling drift.
                </p>
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>Target X:</span>
                  <input
                    type="number"
                    value={calibX}
                    onChange={(e) => setCalibX(parseInt(e.target.value) || 0)}
                    style={{
                      width: '75px',
                      padding: '5px 8px',
                      borderRadius: '6px',
                      border: '1px solid var(--border)',
                      fontSize: '12px',
                      backgroundColor: '#ffffff',
                      color: 'var(--text)',
                    }}
                  />
                </div>

                <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)' }}>Target Y:</span>
                  <input
                    type="number"
                    value={calibY}
                    onChange={(e) => setCalibY(parseInt(e.target.value) || 0)}
                    style={{
                      width: '75px',
                      padding: '5px 8px',
                      borderRadius: '6px',
                      border: '1px solid var(--border)',
                      fontSize: '12px',
                      backgroundColor: '#ffffff',
                      color: 'var(--text)',
                    }}
                  />
                </div>

                <button
                  onClick={handleRunCalibration}
                  disabled={isCalibrating}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '6px',
                    padding: '7px 14px',
                    borderRadius: '6px',
                    border: 'none',
                    backgroundColor: 'var(--primary)',
                    color: '#ffffff',
                    fontSize: '12px',
                    fontWeight: 600,
                    cursor: isCalibrating ? 'not-allowed' : 'pointer',
                    whiteSpace: 'nowrap',
                  }}
                >
                  <Crosshair size={13} />
                  <span>{isCalibrating ? 'Calibrating...' : 'Calibrate Coordinates'}</span>
                </button>
              </div>
            </div>

            {calibResult && (
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
                  gap: '10px',
                  padding: '12px 14px',
                  backgroundColor: 'var(--canvas)',
                  borderRadius: '8px',
                  border: '1px solid var(--border)',
                  fontSize: '12px',
                }}
              >
                <div>
                  <span style={{ color: 'var(--text-muted)', display: 'block', marginBottom: '2px' }}>Platform Profile</span>
                  <strong style={{ color: 'var(--text)' }}>{calibResult.os.toUpperCase()}</strong>
                </div>
                <div>
                  <span style={{ color: 'var(--text-muted)', display: 'block', marginBottom: '2px' }}>Scaling Factor</span>
                  <strong style={{ color: 'var(--primary)' }}>{calibResult.scale_factor}x</strong>
                </div>
                <div>
                  <span style={{ color: 'var(--text-muted)', display: 'block', marginBottom: '2px' }}>Logical Viewport</span>
                  <strong style={{ color: 'var(--text)' }}>{calibResult.logical_width} x {calibResult.logical_height}</strong>
                </div>
                <div>
                  <span style={{ color: 'var(--text-muted)', display: 'block', marginBottom: '2px' }}>Physical Hardware</span>
                  <strong style={{ color: 'var(--text)' }}>{calibResult.physical_width} x {calibResult.physical_height}</strong>
                </div>
                <div>
                  <span style={{ color: 'var(--text-muted)', display: 'block', marginBottom: '2px' }}>Input Coordinates</span>
                  <strong style={{ color: 'var(--text)' }}>({calibResult.offset_target_x}, {calibResult.offset_target_y})</strong>
                </div>
                <div>
                  <span style={{ color: 'var(--text-muted)', display: 'block', marginBottom: '2px' }}>Corrected Hardware Target</span>
                  <strong style={{ color: '#137333' }}>({calibResult.corrected_target_x}, {calibResult.corrected_target_y})</strong>
                </div>
              </div>
            )}
          </div>

          {/* Architecture Explanation Card */}
          <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: '12px', padding: '18px 22px' }}>
            <h4 style={{ margin: 0, fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
              Architecture: MCP Tools, Skills & Native System Framework
            </h4>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: '12px', fontSize: '12px', lineHeight: 1.5, color: 'var(--text-muted)' }}>
              <div style={{ padding: '12px', borderRadius: '6px', backgroundColor: 'var(--canvas)', border: '1px solid var(--border)' }}>
                <div style={{ fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>1. Agent MCP Tools</div>
                Antigravity agents invoke the <code>open-computer-use</code> MCP server tools (<code>click</code>, <code>drag</code>, <code>press_key</code>, <code>type_text</code>, <code>get_app_state</code>) to express user-level intent.
              </div>
              <div style={{ padding: '12px', borderRadius: '6px', backgroundColor: 'var(--canvas)', border: '1px solid var(--border)' }}>
                <div style={{ fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>2. Coded OS Integration</div>
                Swiss Knife executes the actual system-level bridge: intercepting coordinate frames, scaling HiDPI displays, and capturing Wayland buffers through PipeWire.
              </div>
              <div style={{ padding: '12px', borderRadius: '6px', backgroundColor: 'var(--canvas)', border: '1px solid var(--border)' }}>
                <div style={{ fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>3. Accessibility Tree Grounding</div>
                Bypasses costly full-screen multimodal vision passes by pulling semantic node hierarchies from the Linux AT-SPI D-Bus daemon for token-efficient element localization.
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default UtilitiesPage
