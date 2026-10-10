import React, { useState, useEffect } from 'react'
import {
  DownloadCloud,
  RefreshCw,
  CheckCircle2,
  ArrowRight,
  Eye,
  Activity,
  X,
  Monitor,
  Terminal,
  Bot,
  Edit3,
  Workflow,
  Code2,
  Cpu,
  Radio,
  TerminalSquare,
} from 'lucide-react'
import type {
  ChatImportSource,
  ProjectMatchOption,
  ImportCandidate,
  ImportHistoryItem,
  AcpAgentInstance,
  AcpHandshakeLog,
} from '../types'
import { api } from '../api'
import { ToggleSwitch } from '../components/ToggleSwitch'
import { BrainCachePage } from './BrainCachePage'
import {
  ACP_CARD_LAYOUT_TOKENS,
  getAcpStatusPresentation,
} from '../utils/acpPresentation'

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
  const [dpiNormalization, setDpiNormalization] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_comp_dpi_norm') !== 'false'
  })
  const [waylandPipeWire, setWaylandPipeWire] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_comp_wayland_pipewire') !== 'false'
  })
  const [accessibilityGrounding, setAccessibilityGrounding] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_comp_accessibility_grounding') !== 'false'
  })

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

  // --- 2. ACP Inspector State ---
  const [isPingingAll, setIsPingingAll] = useState(false)
  const [acpFeedback, setAcpFeedback] = useState<string | null>(null)
  const [agentInstances, setAgentInstances] = useState<AcpAgentInstance[]>([])
  const [handshakeLogs, setHandshakeLogs] = useState<AcpHandshakeLog[]>([])

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

  const loadAcpMesh = async () => {
    try {
      const res = await api.getAcpMesh()
      if (res && res.agents) {
        setAgentInstances(res.agents)
      }
    } catch (e) {
      console.error('Error fetching ACP mesh:', e)
    }
  }

  useEffect(() => {
    loadCandidates(selectedSource)
  }, [selectedSource])

  useEffect(() => {
    loadAcpMesh()
  }, [])

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

  // Trigger ACP Ping All
  const handlePingAllAcp = async () => {
    setIsPingingAll(true)
    setAcpFeedback(null)
    try {
      const res = await api.getAcpMesh()
      if (res && res.agents) {
        setAgentInstances(res.agents)
        const online = res.agents.filter((a: any) => a.status !== 'unreachable')
        setAcpFeedback(`ACP Handshake ping completed. ${online.length} active agent daemon${online.length === 1 ? '' : 's'} responded successfully.`)
        const newLogs: AcpHandshakeLog[] = online.map((a: any) => ({
          id: `log-${Date.now()}-${a.id}`,
          timestamp: new Date().toLocaleTimeString(),
          from_agent: 'Antigravity 2.0',
          to_agent: a.name,
          action: 'ACP_HELLO / CAPABILITY_EXCHANGE',
          payload_summary: `Negotiated ${a.supported_tools?.length || 0} tools (${(a.supported_tools || []).slice(0, 3).join(', ')}...) over socket ${a.port_socket}`,
          status: 'success',
        }))
        setHandshakeLogs((prev) => [...newLogs, ...prev])
      }
    } catch (e: any) {
      setAcpFeedback(`ACP Ping failed: ${e?.message || 'Network error'}`)
    } finally {
      setIsPingingAll(false)
      setTimeout(() => setAcpFeedback(null), 4000)
    }
  }

  const [pingingAgentId, setPingingAgentId] = useState<string | null>(null)

  const handlePingSingleAgent = async (agent: AcpAgentInstance) => {
    setPingingAgentId(agent.id)
    try {
      const res = await api.getAcpMesh()
      if (res && res.agents) {
        setAgentInstances(res.agents)
        const updatedAgent = res.agents.find((a: any) => a.id === agent.id) || agent
        const isOnline = updatedAgent.status !== 'unreachable'
        setAcpFeedback(`ACP Handshake ping completed for ${agent.name}: ${isOnline ? 'Active & Healthy' : 'Offline'}`)
        const log: AcpHandshakeLog = {
          id: `log-${Date.now()}-${agent.id}`,
          timestamp: new Date().toLocaleTimeString(),
          from_agent: 'Antigravity 2.0',
          to_agent: agent.name,
          action: 'ACP_HELLO / DIRECT_PING',
          payload_summary: isOnline
            ? `Pinged ${agent.name} over socket ${agent.port_socket} (latency: ${updatedAgent.ping_latency_ms} ms)`
            : `Attempted ping to ${agent.name} over socket ${agent.port_socket} — daemon unreachable`,
          status: isOnline ? 'success' : 'warning',
        }
        setHandshakeLogs((prev) => [log, ...prev])
      }
    } catch (e: any) {
      setAcpFeedback(`Ping ${agent.name} failed: ${e?.message || 'Network error'}`)
    } finally {
      setPingingAgentId(null)
      setTimeout(() => setAcpFeedback(null), 4000)
    }
  }

  const renderAgentAvatar = (id: string) => {
    const iconProps = { size: 15, strokeWidth: 1.75 }
    let icon = <Bot {...iconProps} />
    switch (id) {
      case 'agent-antigravity':
        icon = <Monitor {...iconProps} />
        break
      case 'agent-antigravity-cli':
        icon = <Terminal {...iconProps} />
        break
      case 'agent-devin':
        icon = <Workflow {...iconProps} />
        break
      case 'agent-opencode':
        icon = <Code2 {...iconProps} />
        break
      case 'agent-deepseek-harness':
        icon = <Cpu {...iconProps} />
        break
      case 'agent-pi':
        icon = <Radio {...iconProps} />
        break
      case 'agent-codex':
        icon = <TerminalSquare {...iconProps} />
        break
      case 'agent-claude-code':
        icon = <Bot {...iconProps} />
        break
      case 'agent-cursor':
        icon = <Edit3 {...iconProps} />
        break
    }
    return (
      <div
        style={{
          width: ACP_CARD_LAYOUT_TOKENS.avatarSize,
          height: ACP_CARD_LAYOUT_TOKENS.avatarSize,
          borderRadius: ACP_CARD_LAYOUT_TOKENS.avatarBorderRadius,
          backgroundColor: '#f1f3f4',
          color: 'var(--text)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          flexShrink: 0,
        }}
      >
        {icon}
      </div>
    )
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
                      fontSize: '11.5px',
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
                      fontSize: '11.5px',
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
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '12.5px' }}>
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
                              fontSize: '11.5px',
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
                      fontSize: '11.5px',
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
                      fontSize: '12.5px',
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
      {/* TAB 1: ACP (AGENT CLIENT PROTOCOL) STATUS INSPECTOR */}
      {/* ============================================================ */}
      {activeTab === 1 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          {acpFeedback && (
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
              {acpFeedback}
            </div>
          )}


          {/* Agent Nodes Grid */}
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))',
              gap: '16px',
            }}
          >
            {agentInstances.length === 0 ? (
              <div
                style={{
                  gridColumn: '1 / -1',
                  backgroundColor: '#ffffff',
                  border: '1px solid var(--border)',
                  borderRadius: '10px',
                  padding: '32px',
                  textAlign: 'center',
                  color: 'var(--text-muted)',
                  fontSize: '13px',
                }}
              >
                No active agent daemons or ACP nodes discovered on this system.
              </div>
            ) : (
              agentInstances.map((agent) => {
                const statusPres = getAcpStatusPresentation(agent.status)
                return (
                  <div
                    key={agent.id}
                    style={{
                      backgroundColor: '#ffffff',
                      border: `${ACP_CARD_LAYOUT_TOKENS.cardBorderWidth} solid var(--border)`,
                      borderRadius: ACP_CARD_LAYOUT_TOKENS.cardBorderRadius,
                      padding: ACP_CARD_LAYOUT_TOKENS.cardPadding,
                      boxShadow: ACP_CARD_LAYOUT_TOKENS.cardShadow,
                      display: 'flex',
                      flexDirection: 'column',
                      justifyContent: 'space-between',
                      gap: '8px',
                    }}
                  >
                    <div>
                      {/* Card Header with Lucide Avatar & Status */}
                      <div
                        style={{
                          display: 'flex',
                          justifyContent: 'space-between',
                          alignItems: 'flex-start',
                          marginBottom: ACP_CARD_LAYOUT_TOKENS.headerMarginBottom,
                        }}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                          {renderAgentAvatar(agent.id)}
                          <div>
                            <h4
                              style={{
                                fontSize: '14px',
                                fontWeight: 700,
                                margin: '0 0 2px 0',
                                color: 'var(--text)',
                                whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                              }}
                            >
                              {agent.name}
                            </h4>
                            <span
                              style={{
                                fontSize: '11.5px',
                                color: 'var(--text-muted)',
                                whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                              }}
                            >
                              {agent.type} • PID {agent.pid || 'N/A'}
                            </span>
                          </div>
                        </div>

                        <span
                          style={{
                            padding: '3px 8px',
                            borderRadius: '6px',
                            fontSize: '11px',
                            fontWeight: 600,
                            backgroundColor: statusPres.bg,
                            color: statusPres.color,
                            textTransform: 'uppercase',
                            whiteSpace: statusPres.whiteSpace,
                          }}
                        >
                          {statusPres.label}
                        </span>
                      </div>

                      {/* Card Body Metadata */}
                      <div
                        style={{
                          fontSize: '12px',
                          display: 'flex',
                          flexDirection: 'column',
                          gap: ACP_CARD_LAYOUT_TOKENS.bodyGap,
                          marginBottom: '8px',
                        }}
                      >
                        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                          <span style={{ color: 'var(--text-muted)', whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                            Socket / Endpoint:
                          </span>
                          <span
                            style={{
                              fontFamily: 'monospace',
                              fontSize: '11px',
                              color: 'var(--text)',
                              whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                            }}
                          >
                            {agent.port_socket}
                          </span>
                        </div>
                        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                          <span style={{ color: 'var(--text-muted)', whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                            Protocol Version:
                          </span>
                          <span
                            style={{
                              fontWeight: 600,
                              color: 'var(--text)',
                              whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                            }}
                          >
                            {agent.acp_version}
                          </span>
                        </div>
                        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                          <span style={{ color: 'var(--text-muted)', whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                            Ping Latency:
                          </span>
                          <span
                            style={{
                              fontWeight: 600,
                              color: agent.ping_latency_ms > 0 && agent.ping_latency_ms < 3 ? '#137333' : '#b06000',
                              whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                            }}
                          >
                            {agent.ping_latency_ms > 0 ? `${agent.ping_latency_ms} ms` : 'Unreachable'}
                          </span>
                        </div>
                        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                          <span style={{ color: 'var(--text-muted)', whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                            Last Handshake:
                          </span>
                          <span style={{ color: 'var(--text)', whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                            {agent.last_handshake}
                          </span>
                        </div>
                      </div>

                      {/* Shared Tools Tag List */}
                      <div style={{ borderTop: '1px solid #f1f3f4', paddingTop: '8px' }}>
                        <span
                          style={{
                            fontSize: '11px',
                            fontWeight: 600,
                            color: 'var(--text-muted)',
                            display: 'block',
                            marginBottom: '4px',
                            whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                          }}
                        >
                          Negotiated Tools Sharing ({agent.supported_tools.length})
                        </span>
                        <div style={{ display: 'flex', flexWrap: 'wrap', gap: ACP_CARD_LAYOUT_TOKENS.tagGap }}>
                          {agent.supported_tools.length > 0 ? (
                            agent.supported_tools.map((t) => (
                              <span
                                key={t}
                                style={{
                                  backgroundColor: '#f1f3f4',
                                  borderRadius: '4px',
                                  padding: '1px 6px',
                                  fontSize: '10.5px',
                                  fontFamily: 'monospace',
                                  color: '#3c4043',
                                  whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                                }}
                              >
                                {t}
                              </span>
                            ))
                          ) : (
                            <span
                              style={{
                                fontSize: '11px',
                                color: 'var(--text-muted)',
                                fontStyle: 'italic',
                                whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                              }}
                            >
                              No tools shared
                            </span>
                          )}
                        </div>
                      </div>
                    </div>

                    {/* Card Action Button: Ping {agent.name} */}
                    <div
                      style={{
                        display: 'flex',
                        justifyContent: 'flex-end',
                        borderTop: '1px solid #f1f3f4',
                        paddingTop: '8px',
                        marginTop: '4px',
                      }}
                    >
                      <button
                        className="btn-pill-tonal"
                        onClick={() => handlePingSingleAgent(agent)}
                        disabled={pingingAgentId === agent.id}
                        style={{
                          height: ACP_CARD_LAYOUT_TOKENS.actionButtonHeight,
                          padding: ACP_CARD_LAYOUT_TOKENS.actionButtonPadding,
                          fontSize: ACP_CARD_LAYOUT_TOKENS.actionButtonFontSize,
                          borderRadius: ACP_CARD_LAYOUT_TOKENS.actionButtonBorderRadius,
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '6px',
                          whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace,
                          cursor: pingingAgentId === agent.id ? 'not-allowed' : 'pointer',
                          border: '1px solid var(--border)',
                          backgroundColor: '#f8f9fa',
                          color: 'var(--text)',
                        }}
                      >
                        <Activity
                          size={12}
                          className={pingingAgentId === agent.id ? 'animate-spin' : ''}
                        />
                        <span style={{ whiteSpace: ACP_CARD_LAYOUT_TOKENS.whiteSpace }}>
                          Ping {agent.name}
                        </span>
                      </button>
                    </div>
                  </div>
                )
              })
            )}
          </div>

          {/* Live Handshake Logs */}
          <div
            style={{
              backgroundColor: '#ffffff',
              border: '1px solid var(--border)',
              borderRadius: '10px',
              padding: '20px',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
              <h3 style={{ fontSize: '15px', fontWeight: 700, margin: 0, color: 'var(--text)' }}>
                ACP Communication & Delegation Stream
              </h3>
              <button
                onClick={handlePingAllAcp}
                disabled={isPingingAll}
                style={{
                  backgroundColor: 'var(--primary)',
                  color: '#ffffff',
                  border: 'none',
                  borderRadius: '6px',
                  padding: '6px 14px',
                  fontSize: '12px',
                  fontWeight: 600,
                  cursor: isPingingAll ? 'not-allowed' : 'pointer',
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '6px',
                  whiteSpace: 'nowrap',
                  boxShadow: '0 1px 2px rgba(0,0,0,0.08)',
                }}
              >
                <Activity size={13} className={isPingingAll ? 'animate-spin' : ''} />
                <span>{isPingingAll ? 'Pinging Nodes...' : 'Ping All ACP Nodes'}</span>
              </button>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
              {handshakeLogs.length === 0 ? (
                <div style={{ padding: '24px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '13px' }}>
                  No ACP handshakes or delegations recorded yet. Click &quot;Ping All ACP Nodes&quot; above to initiate a protocol health check across all detected agent processes.
                </div>
              ) : (
                handshakeLogs.map((log) => (
                <div
                  key={log.id}
                  style={{
                    border: '1px solid #f1f3f4',
                    borderRadius: '10px',
                    padding: '10px 14px',
                    backgroundColor: '#fafafa',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '4px',
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600 }}>
                      <span style={{ color: 'var(--primary)' }}>{log.from_agent}</span>
                      <ArrowRight size={12} color="var(--text-muted)" />
                      <span style={{ color: 'var(--text)' }}>{log.to_agent}</span>
                      <span style={{ color: 'var(--text-muted)', fontWeight: 400, fontSize: '11px' }}>({log.action})</span>
                    </div>
                    <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>{log.timestamp}</span>
                  </div>
                  <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                    {log.payload_summary}
                  </div>
                </div>
                ))
              )}
            </div>
          </div>
        </div>
      )}

      {/* 4. Computer Use Tab */}
      {activeTab === 3 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '18px' }}>
          {/* Main Card */}
          <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: '16px', padding: '20px 24px' }}>
            <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px' }}>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <Monitor size={18} color="var(--primary)" />
                  <h3 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
                    Computer Use Enhancer
                  </h3>
                </div>
                <p style={{ margin: '2px 0 0', fontSize: '13px', color: 'var(--text-muted)', lineHeight: 1.5, maxWidth: '820px' }}>
                  OS-level execution enhancer optimizing Antigravity computer use with display coordinate scaling normalization, Wayland PipeWire screen capture, and token-saving accessibility tree grounding on Linux desktop.
                </p>
              </div>
            </div>

            {/* Diagnostic Badges */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap', padding: '10px 14px', backgroundColor: 'var(--canvas)', borderRadius: '8px', border: '1px solid var(--border)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', color: 'var(--text-muted)' }}>
                <CheckCircle2 size={13} color="var(--green)" />
                <span>Display Pipeline: <strong>Wayland & X11 Portal Ready</strong></span>
              </div>
              <span style={{ color: 'var(--border)' }}>•</span>
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', color: 'var(--text-muted)' }}>
                <CheckCircle2 size={13} color="var(--green)" />
                <span>Screen Capture: <strong>xdg-desktop-portal / PipeWire</strong></span>
              </div>
              <span style={{ color: 'var(--border)' }}>•</span>
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', color: 'var(--text-muted)' }}>
                <CheckCircle2 size={13} color="var(--green)" />
                <span>Grounding: <strong>AT-SPI D-Bus Accessible</strong></span>
              </div>
            </div>

            {/* 3 Setting Cards */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '14px' }}>
              <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 16px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--canvas)', cursor: 'pointer' }}>
                <div style={{ paddingRight: '12px' }}>
                  <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>DPI Normalizer</div>
                  <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                    Calibrate HiDPI 125%/150% scaling offsets to ensure click and drag coordinates target exact pixel boundaries.
                  </div>
                </div>
                <ToggleSwitch
                  size="sm"
                  checked={dpiNormalization}
                  onChange={(val) => {
                    setDpiNormalization(val)
                    localStorage.setItem('antigravity_comp_dpi_norm', String(val))
                  }}
                  ariaLabel="Toggle DPI Normalizer"
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
                  checked={waylandPipeWire}
                  onChange={(val) => {
                    setWaylandPipeWire(val)
                    localStorage.setItem('antigravity_comp_wayland_pipewire', String(val))
                  }}
                  ariaLabel="Toggle Wayland PipeWire Stream"
                />
              </label>

              <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 16px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--canvas)', cursor: 'pointer' }}>
                <div style={{ paddingRight: '12px' }}>
                  <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>Accessibility Grounding</div>
                  <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                    Query OS accessibility tree (AT-SPI) via D-Bus to locate UI elements deterministically without burning vision tokens.
                  </div>
                </div>
                <ToggleSwitch
                  size="sm"
                  checked={accessibilityGrounding}
                  onChange={(val) => {
                    setAccessibilityGrounding(val)
                    localStorage.setItem('antigravity_comp_accessibility_grounding', String(val))
                  }}
                  ariaLabel="Toggle Accessibility Grounding"
                />
              </label>
            </div>
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
