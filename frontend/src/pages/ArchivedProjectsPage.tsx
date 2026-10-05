import React, { useEffect, useState } from 'react'
import {
  Archive,
  RotateCcw,
  Settings,
  Trash2,
  RefreshCw,
  Search,
  MessageSquare,
  Clock,
  Folder,
  AlertTriangle,
  CheckCircle2,
  ChevronDown,
} from 'lucide-react'
import { api } from '../api'
import type { ArchivedProjectItem } from '../types'

export const ArchivedProjectsPage: React.FC = () => {
  const [archived, setArchived] = useState<ArchivedProjectItem[]>([])
  const [activeProjects, setActiveProjects] = useState<Array<{ name: string; color: string; is_archived: boolean }>>([])
  const [loading, setLoading] = useState(true)
  const [searchQuery, setSearchQuery] = useState('')
  const [statusMsg, setStatusMsg] = useState<{ text: string; type: 'success' | 'error' } | null>(null)
  const [actionLoading, setActionLoading] = useState<string | null>(null)
  const [deleteConfirmProject, setDeleteConfirmProject] = useState<ArchivedProjectItem | null>(null)
  const [selectedToArchive, setSelectedToArchive] = useState('')

  const loadData = async () => {
    try {
      setLoading(true)
      const [archivedList, allProjects] = await Promise.all([
        api.getArchivedProjects(),
        api.getGUIProjects().catch(() => []),
      ])
      setArchived(archivedList || [])
      setActiveProjects((allProjects || []).filter((p) => !p.is_archived))
    } catch (err: any) {
      setStatusMsg({ text: 'Failed to load archived projects: ' + err.message, type: 'error' })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  const showToast = (text: string, type: 'success' | 'error' = 'success') => {
    setStatusMsg({ text, type })
    setTimeout(() => setStatusMsg(null), 4000)
  }

  const handleOpenSettings = async (project: ArchivedProjectItem) => {
    try {
      setActionLoading(`settings-${project.id}`)
      await api.openProjectSettings(project.name || project.id)
      showToast(`Triggered native project settings for "${project.name}" in Antigravity.`)
    } catch (err: any) {
      showToast('Failed to open project settings: ' + err.message, 'error')
    } finally {
      setActionLoading(null)
    }
  }

  const handleRestore = async (project: ArchivedProjectItem) => {
    try {
      setActionLoading(`restore-${project.id}`)
      await api.restoreProject(project.name || project.id)
      showToast(`Restored "${project.name}" back to the active Antigravity sidebar!`)
      await loadData()
    } catch (err: any) {
      showToast('Failed to restore project: ' + err.message, 'error')
    } finally {
      setActionLoading(null)
    }
  }

  const handleDelete = async () => {
    if (!deleteConfirmProject) return
    try {
      setActionLoading(`delete-${deleteConfirmProject.id}`)
      await api.deleteArchivedProject(deleteConfirmProject.name || deleteConfirmProject.id)
      showToast(`Project "${deleteConfirmProject.name}" permanently deleted.`)
      setDeleteConfirmProject(null)
      await loadData()
    } catch (err: any) {
      showToast('Failed to delete project: ' + err.message, 'error')
    } finally {
      setActionLoading(null)
    }
  }

  const handleArchiveSelect = async (name: string) => {
    if (!name) return
    try {
      setActionLoading(`archive-new`)
      await api.archiveProject(name)
      showToast(`Archived project "${name}". It is now hidden from the Antigravity sidebar.`)
      setSelectedToArchive('')
      await loadData()
    } catch (err: any) {
      showToast('Failed to archive project: ' + err.message, 'error')
    } finally {
      setActionLoading(null)
    }
  }

  const filteredProjects = archived.filter((p) => {
    const q = searchQuery.toLowerCase().trim()
    if (!q) return true
    return (
      p.name.toLowerCase().includes(q) ||
      p.id.toLowerCase().includes(q) ||
      (p.folder_uri && p.folder_uri.toLowerCase().includes(q))
    )
  })

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* Header Info Card */}
      <div className="google-card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '16px' }}>
          <div>
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
              Archived Projects
            </div>
            <div style={{ fontSize: '13px', color: 'var(--text)', marginTop: '4px' }}>
              Manage hidden and archived projects in Antigravity 2.0. Projects here are hidden from the sidebar to keep your workspace clutter-free.
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            {/* Archive Active Project Dropdown */}
            {activeProjects.length > 0 && (
              <div style={{ position: 'relative' }}>
                <select
                  value={selectedToArchive}
                  onChange={(e) => {
                    setSelectedToArchive(e.target.value)
                    handleArchiveSelect(e.target.value)
                  }}
                  disabled={actionLoading !== null}
                  style={{
                    height: '36px',
                    padding: '0 32px 0 12px',
                    borderRadius: '20px',
                    border: '1px solid var(--border)',
                    background: 'var(--surface)',
                    fontSize: '12px',
                    fontWeight: 500,
                    color: 'var(--text)',
                    cursor: 'pointer',
                    appearance: 'none',
                  }}
                >
                  <option value="">+ Archive an active project...</option>
                  {activeProjects.map((p) => (
                    <option key={p.name} value={p.name}>
                      {p.name}
                    </option>
                  ))}
                </select>
                <ChevronDown
                  size={14}
                  color="var(--text-muted)"
                  style={{ position: 'absolute', right: '12px', top: '11px', pointerEvents: 'none' }}
                />
              </div>
            )}

            <button
              onClick={loadData}
              disabled={loading}
              title="Refresh archived projects list"
              className="btn-pill-tonal"
              style={{ fontSize: '12px', padding: '7px 14px' }}
            >
              <RefreshCw size={13} className={loading ? 'spin' : ''} />
              <span>Refresh</span>
            </button>
          </div>
        </div>

        {statusMsg && (
          <div
            style={{
              marginTop: '16px',
              padding: '10px 14px',
              borderRadius: '8px',
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              fontSize: '13px',
              background: statusMsg.type === 'success' ? '#f0fdf4' : '#fef2f2',
              color: statusMsg.type === 'success' ? '#166534' : '#991b1b',
              border: `1px solid ${statusMsg.type === 'success' ? '#bbf7d0' : '#fecaca'}`,
            }}
          >
            {statusMsg.type === 'success' ? <CheckCircle2 size={16} /> : <AlertTriangle size={16} />}
            <span>{statusMsg.text}</span>
          </div>
        )}
      </div>

      {/* Search Bar */}
      {archived.length > 0 && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <div
            style={{
              position: 'relative',
              flex: 1,
              maxWidth: '380px',
            }}
          >
            <Search
              size={15}
              color="#94a3b8"
              style={{ position: 'absolute', left: '12px', top: '11px', pointerEvents: 'none' }}
            />
            <input
              type="text"
              placeholder="Filter archived projects by name or path..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              style={{
                width: '100%',
                height: '38px',
                padding: '0 12px 0 36px',
                borderRadius: '8px',
                border: '1px solid #e2e8f0',
                fontSize: '13px',
                color: '#1e293b',
                background: '#ffffff',
                boxSizing: 'border-box',
              }}
            />
          </div>
        </div>
      )}

      {/* Table Container */}
      <div className="google-card" style={{ padding: 0, overflow: 'hidden' }}>
        {loading ? (
          <div style={{ padding: '48px', textAlign: 'center', color: '#64748b' }}>
            <RefreshCw size={24} className="spin" style={{ margin: '0 auto 12px' }} />
            <div style={{ fontSize: '14px', fontWeight: 500 }}>Loading archived projects...</div>
          </div>
        ) : filteredProjects.length === 0 ? (
          <div style={{ padding: '48px 24px', textAlign: 'center' }}>
            <div
              style={{
                width: '48px',
                height: '48px',
                borderRadius: '50%',
                background: '#f8fafc',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                margin: '0 auto 12px',
                border: '1px solid #e2e8f0',
              }}
            >
              <Archive size={24} color="#94a3b8" />
            </div>
            <h3 style={{ margin: '0 0 6px', fontSize: '15px', fontWeight: 600, color: '#334155' }}>
              {searchQuery ? 'No matching archived projects' : 'No archived projects'}
            </h3>
            <p style={{ margin: 0, fontSize: '13px', color: '#64748b', maxWidth: '480px', marginInline: 'auto' }}>
              {searchQuery
                ? 'Try a different keyword or clear your filter query.'
                : 'To archive a project and hide it from your Antigravity sidebar, right-click on its project header in Antigravity and select "📦 Hide / Archive Project".'}
            </p>
          </div>
        ) : (
          <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left' }}>
            <thead>
              <tr style={{ background: '#f8fafc', borderBottom: '1px solid #e2e8f0' }}>
                <th style={{ padding: '12px 18px', fontSize: '11px', fontWeight: 700, color: '#64748b', textTransform: 'uppercase', letterSpacing: '0.6px' }}>
                  Project Name
                </th>
                <th style={{ padding: '12px 18px', fontSize: '11px', fontWeight: 700, color: '#64748b', textTransform: 'uppercase', letterSpacing: '0.6px' }}>
                  Last Conversation
                </th>
                <th style={{ padding: '12px 18px', fontSize: '11px', fontWeight: 700, color: '#64748b', textTransform: 'uppercase', letterSpacing: '0.6px' }}>
                  Conversations
                </th>
                <th style={{ padding: '12px 18px', fontSize: '11px', fontWeight: 700, color: '#64748b', textTransform: 'uppercase', letterSpacing: '0.6px', textAlign: 'right' }}>
                  Actions
                </th>
              </tr>
            </thead>
            <tbody>
              {filteredProjects.map((p, idx) => (
                <tr
                  key={p.id || idx}
                  style={{
                    borderBottom: idx < filteredProjects.length - 1 ? '1px solid #f1f5f9' : 'none',
                    transition: 'background 0.1s',
                  }}
                  onMouseEnter={(e) => (e.currentTarget.style.background = '#fbfcfe')}
                  onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
                >
                  {/* Project Name & Meta */}
                  <td style={{ padding: '16px 18px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                      <div
                        style={{
                          width: '12px',
                          height: '12px',
                          borderRadius: '50%',
                          background: p.color || '#64748b',
                          flexShrink: 0,
                          boxShadow: '0 1px 3px rgba(0,0,0,0.15)',
                        }}
                      />
                      <div>
                        <div style={{ fontSize: '14px', fontWeight: 600, color: '#1e293b' }}>
                          {p.name}
                        </div>
                        {p.folder_uri && (
                          <div
                            style={{
                              fontSize: '11px',
                              color: '#64748b',
                              marginTop: '2px',
                              display: 'flex',
                              alignItems: 'center',
                              gap: '4px',
                            }}
                          >
                            <Folder size={11} />
                            <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '340px' }}>
                              {p.folder_uri.replace('file://', '')}
                            </span>
                          </div>
                        )}
                      </div>
                    </div>
                  </td>

                  {/* Relative Last Active Time (Automatically promoted unit) */}
                  <td style={{ padding: '16px 18px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <Clock size={13} color="#64748b" />
                      <span
                        style={{
                          fontSize: '13px',
                          fontWeight: 500,
                          color: '#334155',
                        }}
                        title={p.last_active_time ? new Date(p.last_active_time).toLocaleString() : 'No activity recorded'}
                      >
                        {p.last_active_relative || 'Never active'}
                      </span>
                    </div>
                  </td>

                  {/* Conversation Count */}
                  <td style={{ padding: '16px 18px' }}>
                    <div style={{ display: 'inline-flex', alignItems: 'center', gap: '5px', padding: '3px 9px', borderRadius: '12px', background: '#f1f5f9', color: '#475569', fontSize: '12px', fontWeight: 600 }}>
                      <MessageSquare size={12} />
                      <span>{p.conversation_count}</span>
                    </div>
                  </td>

                  {/* Actions */}
                  <td style={{ padding: '16px 18px', textAlign: 'right' }}>
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: '8px' }}>
                      {/* Open Native Project Settings */}
                      <button
                        onClick={() => handleOpenSettings(p)}
                        disabled={actionLoading !== null}
                        title="Open native Antigravity Project Settings"
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: '5px',
                          padding: '6px 10px',
                          borderRadius: '6px',
                          border: '1px solid #d1d5db',
                          background: '#ffffff',
                          fontSize: '12px',
                          fontWeight: 500,
                          color: '#374151',
                          cursor: 'pointer',
                          transition: 'background 0.12s',
                        }}
                        onMouseEnter={(e) => (e.currentTarget.style.background = '#f3f4f6')}
                        onMouseLeave={(e) => (e.currentTarget.style.background = '#ffffff')}
                      >
                        <Settings size={13} />
                        <span>Settings</span>
                      </button>

                      {/* Unhide / Restore */}
                      <button
                        onClick={() => handleRestore(p)}
                        disabled={actionLoading !== null}
                        title="Unhide and restore to Antigravity sidebar"
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: '5px',
                          padding: '6px 10px',
                          borderRadius: '6px',
                          border: '1px solid #bbf7d0',
                          background: '#f0fdf4',
                          fontSize: '12px',
                          fontWeight: 600,
                          color: '#15803d',
                          cursor: 'pointer',
                          transition: 'background 0.12s',
                        }}
                        onMouseEnter={(e) => (e.currentTarget.style.background = '#dcfce7')}
                        onMouseLeave={(e) => (e.currentTarget.style.background = '#f0fdf4')}
                      >
                        <RotateCcw size={13} />
                        <span>Unhide</span>
                      </button>

                      {/* Delete */}
                      <button
                        onClick={() => setDeleteConfirmProject(p)}
                        disabled={actionLoading !== null}
                        title="Permanently delete project"
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: '5px',
                          padding: '6px 10px',
                          borderRadius: '6px',
                          border: '1px solid #fecaca',
                          background: '#fef2f2',
                          fontSize: '12px',
                          fontWeight: 500,
                          color: '#b91c1c',
                          cursor: 'pointer',
                          transition: 'background 0.12s',
                        }}
                        onMouseEnter={(e) => (e.currentTarget.style.background = '#fee2e2')}
                        onMouseLeave={(e) => (e.currentTarget.style.background = '#fef2f2')}
                      >
                        <Trash2 size={13} />
                        <span>Delete</span>
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Delete Confirmation Modal */}
      {deleteConfirmProject && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.45)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 999999,
          }}
          onClick={() => setDeleteConfirmProject(null)}
        >
          <div
            style={{
              background: '#ffffff',
              borderRadius: '12px',
              padding: '24px',
              maxWidth: '440px',
              width: '90%',
              boxShadow: '0 20px 40px rgba(0,0,0,0.2)',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', color: '#b91c1c', marginBottom: '12px' }}>
              <AlertTriangle size={24} />
              <h3 style={{ margin: 0, fontSize: '16px', fontWeight: 700 }}>Confirm Project Deletion</h3>
            </div>
            <p style={{ margin: '0 0 16px', fontSize: '13px', color: '#4b5563', lineHeight: 1.5 }}>
              Are you sure you want to permanently delete project <strong>"{deleteConfirmProject.name}"</strong>?
              This removes the project record and its styling configurations.
            </p>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px' }}>
              <button
                onClick={() => setDeleteConfirmProject(null)}
                style={{
                  padding: '8px 14px',
                  borderRadius: '6px',
                  border: '1px solid #d1d5db',
                  background: '#ffffff',
                  fontSize: '13px',
                  fontWeight: 500,
                  color: '#374151',
                  cursor: 'pointer',
                }}
              >
                Cancel
              </button>
              <button
                onClick={handleDelete}
                disabled={actionLoading !== null}
                style={{
                  padding: '8px 16px',
                  borderRadius: '6px',
                  border: 'none',
                  background: '#dc2626',
                  color: '#ffffff',
                  fontSize: '13px',
                  fontWeight: 600,
                  cursor: 'pointer',
                }}
              >
                Permanently Delete
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
