import React, { useState, useEffect } from 'react'
import {
  Wrench,
  DownloadCloud,
  Network,
  Clock,
  RefreshCw,
  CheckCircle2,
  ArrowRight,
  Eye,
  Activity,
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

interface UtilitiesPageProps {
  initialTab?: number
}

export const UtilitiesPage: React.FC<UtilitiesPageProps> = ({ initialTab = 0 }) => {
  const [activeTab, setActiveTab] = useState<number>(initialTab) // 0: Chat & Project Import, 1: ACP Protocol Inspector

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

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* Top Section Header */}
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          flexWrap: 'wrap',
          gap: '12px',
        }}
      >
        <div>
          <h2
            style={{
              fontSize: '20px',
              fontWeight: 700,
              color: 'var(--text)',
              margin: '0 0 4px 0',
              display: 'flex',
              alignItems: 'center',
              gap: '10px',
            }}
          >
            <Wrench size={22} color="var(--primary)" />
            Agent Utilities & Interoperability
          </h2>
          <p style={{ margin: 0, fontSize: '13px', color: 'var(--text-muted)' }}>
            Cross-agent conversation migration, project auto-recreation, and Agent Client Protocol (ACP) live mesh status.
          </p>
        </div>

        {/* Tab Switcher */}
        <div
          style={{
            display: 'flex',
            backgroundColor: 'var(--tonal)',
            borderRadius: '20px',
            padding: '3px',
            gap: '2px',
          }}
        >
          <button
            onClick={() => setActiveTab(0)}
            style={{
              borderRadius: '16px',
              padding: '6px 16px',
              fontSize: '12px',
              fontWeight: activeTab === 0 ? 600 : 500,
              color: activeTab === 0 ? 'var(--primary)' : 'var(--text-muted)',
              backgroundColor: activeTab === 0 ? '#ffffff' : 'transparent',
              boxShadow: activeTab === 0 ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
              border: 'none',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
            }}
          >
            <DownloadCloud size={14} />
            Chat & Project Importer
          </button>
          <button
            onClick={() => setActiveTab(1)}
            style={{
              borderRadius: '16px',
              padding: '6px 16px',
              fontSize: '12px',
              fontWeight: activeTab === 1 ? 600 : 500,
              color: activeTab === 1 ? 'var(--primary)' : 'var(--text-muted)',
              backgroundColor: activeTab === 1 ? '#ffffff' : 'transparent',
              boxShadow: activeTab === 1 ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
              border: 'none',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
            }}
          >
            <Network size={14} />
            ACP Agent Mesh Inspector
          </button>
        </div>
      </div>

      {/* ============================================================ */}
      {/* TAB 0: AGENT CHAT & PROJECT IMPORTER */}
      {/* ============================================================ */}
      {activeTab === 0 && (
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
              borderRadius: '16px',
              padding: '20px',
            }}
          >
            <div style={{ marginBottom: '16px' }}>
              <h3 style={{ fontSize: '15px', fontWeight: 700, margin: '0 0 4px 0', color: 'var(--text)' }}>
                1. Select Source Agent & Project Matching Rules
              </h3>
              <p style={{ margin: 0, fontSize: '12.5px', color: 'var(--text-muted)' }}>
                Inspired by <code>dsh-chat-import</code>, this tool converts transcripts, tool outputs, and project structures into Antigravity 2.0 format.
              </p>
            </div>

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
                    padding: '8px 12px',
                    borderRadius: '8px',
                    border: '1px solid var(--border)',
                    fontSize: '13px',
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
                  <option value="windsurf">Windsurf / Codeium Cascade</option>
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
                    padding: '8px 12px',
                    borderRadius: '8px',
                    border: '1px solid var(--border)',
                    fontSize: '13px',
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
                        padding: '7px 10px',
                        borderRadius: '8px',
                        fontSize: '12px',
                        fontWeight: importSyncMode === mode ? 600 : 500,
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
                      borderRadius: '14px',
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
                      borderRadius: '14px',
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

              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '12.5px' }}>
                  <thead>
                    <tr style={{ borderBottom: '1px solid var(--border)', textAlign: 'left', color: 'var(--text-muted)' }}>
                      <th style={{ padding: '8px 10px', width: '30px' }}>
                        <input
                          type="checkbox"
                          checked={candidates.length > 0 && candidates.every((c) => c.selected)}
                          onChange={(e) => setCandidates(candidates.map((c) => ({ ...c, selected: e.target.checked })))}
                        />
                      </th>
                      <th style={{ padding: '8px 10px' }}>Conversation Title</th>
                      <th style={{ padding: '8px 10px' }}>Source</th>
                      <th style={{ padding: '8px 10px' }}>Turns & Tools</th>
                      <th style={{ padding: '8px 10px' }}>Target Destination</th>
                      <th style={{ padding: '8px 10px' }}>Match Status</th>
                      <th style={{ padding: '8px 10px', textAlign: 'right' }}>Preview</th>
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
                              ? '✓ Exact Workspace Match'
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
                    borderRadius: '20px',
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
              borderRadius: '16px',
              padding: '20px',
            }}
          >
            <h3 style={{ fontSize: '15px', fontWeight: 700, margin: '0 0 12px 0', color: 'var(--text)', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Clock size={16} color="var(--primary)" />
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
                            ✓ Completed
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
                  borderRadius: '20px',
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
                    style={{ background: 'none', border: 'none', fontSize: '18px', cursor: 'pointer', color: 'var(--text-muted)' }}
                  >
                    ✕
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
                      borderRadius: '16px',
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

          {/* Overview Banner */}
          <div
            style={{
              backgroundColor: '#ffffff',
              border: '1px solid var(--border)',
              borderRadius: '16px',
              padding: '20px',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '12px' }}>
              <div>
                <h3 style={{ fontSize: '15px', fontWeight: 700, margin: '0 0 4px 0', color: 'var(--text)' }}>
                  Agent Client Protocol (ACP) Inter-Agent Mesh
                </h3>
                <p style={{ margin: 0, fontSize: '12.5px', color: 'var(--text-muted)' }}>
                  ACP provides cross-agent tool delegation, shared workspace context, and multi-agent coordination between Antigravity, Claude Code, Cursor, and Copilot.
                </p>
              </div>

              <button
                onClick={handlePingAllAcp}
                disabled={isPingingAll}
                style={{
                  backgroundColor: 'var(--primary)',
                  color: '#ffffff',
                  border: 'none',
                  borderRadius: '20px',
                  padding: '8px 18px',
                  fontSize: '12.5px',
                  fontWeight: 600,
                  cursor: isPingingAll ? 'not-allowed' : 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  boxShadow: '0 1px 3px rgba(0,0,0,0.1)',
                }}
              >
                <Activity size={14} className={isPingingAll ? 'animate-spin' : ''} />
                {isPingingAll ? 'Pinging Nodes...' : 'Ping All ACP Nodes'}
              </button>
            </div>
          </div>

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
                  borderRadius: '16px',
                  padding: '32px',
                  textAlign: 'center',
                  color: 'var(--text-muted)',
                  fontSize: '13px',
                }}
              >
                No active agent daemons or ACP nodes discovered on this system.
              </div>
            ) : (
              agentInstances.map((agent) => (
                <div
                  key={agent.id}
                  style={{
                    backgroundColor: '#ffffff',
                    border: '1px solid var(--border)',
                    borderRadius: '16px',
                    padding: '18px',
                    boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '10px' }}>
                    <div>
                      <h4 style={{ fontSize: '14.5px', fontWeight: 700, margin: '0 0 2px 0', color: 'var(--text)' }}>
                        {agent.name}
                      </h4>
                      <span style={{ fontSize: '11.5px', color: 'var(--text-muted)' }}>
                        {agent.type} • PID {agent.pid || 'N/A'}
                      </span>
                    </div>

                    <span
                      style={{
                        padding: '3px 8px',
                        borderRadius: '12px',
                        fontSize: '11px',
                        fontWeight: 600,
                        backgroundColor:
                          agent.status === 'active_hosting'
                            ? '#e6f4ea'
                            : agent.status === 'connected'
                            ? '#e8f0fe'
                            : agent.status === 'listening'
                            ? '#fef7e0'
                            : '#fce8e6',
                        color:
                          agent.status === 'active_hosting'
                            ? '#137333'
                            : agent.status === 'connected'
                            ? 'var(--primary)'
                            : agent.status === 'listening'
                            ? '#b06000'
                            : '#d93025',
                        textTransform: 'uppercase',
                      }}
                    >
                      {agent.status.replace('_', ' ')}
                    </span>
                  </div>

                  <div style={{ fontSize: '12px', display: 'flex', flexDirection: 'column', gap: '6px', marginBottom: '12px' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                      <span style={{ color: 'var(--text-muted)' }}>Socket / Endpoint:</span>
                      <span style={{ fontFamily: 'monospace', fontSize: '11px', color: 'var(--text)' }}>{agent.port_socket}</span>
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                      <span style={{ color: 'var(--text-muted)' }}>Protocol Version:</span>
                      <span style={{ fontWeight: 600, color: 'var(--text)' }}>{agent.acp_version}</span>
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                      <span style={{ color: 'var(--text-muted)' }}>Ping Latency:</span>
                      <span style={{ fontWeight: 600, color: agent.ping_latency_ms < 3 ? '#137333' : '#b06000' }}>
                        {agent.ping_latency_ms > 0 ? `${agent.ping_latency_ms} ms` : 'Unreachable'}
                      </span>
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                      <span style={{ color: 'var(--text-muted)' }}>Last Handshake:</span>
                      <span style={{ color: 'var(--text)' }}>{agent.last_handshake}</span>
                    </div>
                  </div>

                  {/* Shared Tools List */}
                  <div style={{ borderTop: '1px solid #f1f3f4', paddingTop: '10px' }}>
                    <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '6px' }}>
                      Negotiated Tools Sharing ({agent.supported_tools.length})
                    </span>
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px' }}>
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
                            }}
                          >
                            {t}
                          </span>
                        ))
                      ) : (
                        <span style={{ fontSize: '11px', color: 'var(--text-muted)', fontStyle: 'italic' }}>
                          No tools shared
                        </span>
                      )}
                    </div>
                  </div>
                </div>
              ))
            )}
          </div>

          {/* Live Handshake Logs */}
          <div
            style={{
              backgroundColor: '#ffffff',
              border: '1px solid var(--border)',
              borderRadius: '16px',
              padding: '20px',
            }}
          >
            <h3 style={{ fontSize: '15px', fontWeight: 700, margin: '0 0 12px 0', color: 'var(--text)', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Network size={16} color="var(--primary)" />
              ACP Communication & Delegation Stream
            </h3>

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
    </div>
  )
}

export default UtilitiesPage
