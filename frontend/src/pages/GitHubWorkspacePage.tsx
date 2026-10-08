import React, { useState, useEffect } from 'react'
import {
  GitPullRequest,
  AlertCircle,
  CheckCircle2,
  GitBranch,
  RefreshCw,
  Search,
  Bot,
  ExternalLink,
  Plus,
  Send,
  Save,
  Sparkles,
  Kanban,
  GripVertical,
  Check,
  CornerDownRight,
} from 'lucide-react'
import { api } from '../api'
import type { KanbanBoard, KanbanCard, KanbanColumn } from '../types'
import { getEffectiveKanbanColumns, findItemColumn } from '../utils/kanban'

export interface GitHubWorkspacePageProps {
  scope?: string
  fallbackProject?: string
}

export const GitHubWorkspacePage: React.FC<GitHubWorkspacePageProps> = ({ scope, fallbackProject }) => {
  const [repo, setRepo] = useState<any>(null)
  const [issues, setIssues] = useState<any[]>([])
  const [prs, setPRs] = useState<any[]>([])
  const [agentTasks, setAgentTasks] = useState<any[]>([])
  const [loading, setLoading] = useState<boolean>(true)
  const [viewMode, setViewMode] = useState<'list' | 'kanban'>('kanban')
  const [kanbanBoard, setKanbanBoard] = useState<KanbanBoard | null>(null)
  const [draggedCard, setDraggedCard] = useState<KanbanCard | null>(null)
  const [dragOverCol, setDragOverCol] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState<'issues' | 'prs' | 'tasks'>('issues')
  const [searchQuery, setSearchQuery] = useState<string>('')
  const [stateFilter, setStateFilter] = useState<'all' | 'open' | 'closed'>('all')
  const [selectedItem, setSelectedItem] = useState<any | null>(null)
  const [commentText, setCommentText] = useState<string>('')
  const [editTitle, setEditTitle] = useState<string>('')
  const [editBody, setEditBody] = useState<string>('')
  const [toastMsg, setToastMsg] = useState<string | null>(null)
  const [isCreatingIssue, setIsCreatingIssue] = useState<boolean>(false)
  const [newTitle, setNewTitle] = useState<string>('')
  const [newBody, setNewBody] = useState<string>('')
  const [contextMenu, setContextMenu] = useState<{
    x: number
    y: number
    item: any
    itemType: 'issue' | 'pr'
    currentColId: string
  } | null>(null)

  useEffect(() => {
    const handleGlobalClick = () => setContextMenu(null)
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setContextMenu(null)
    }
    window.addEventListener('click', handleGlobalClick)
    window.addEventListener('keydown', handleKeyDown)
    return () => {
      window.removeEventListener('click', handleGlobalClick)
      window.removeEventListener('keydown', handleKeyDown)
    }
  }, [])


  const showToast = (msg: string) => {
    setToastMsg(msg)
    setTimeout(() => setToastMsg(null), 3000)
  }

  const navigateToConversation = (convId: string, rootParentId?: string, isPruned?: boolean) => {
    const targetId = rootParentId || convId
    if (isPruned || (targetId && ((window as any).__swissPrunedConversations?.has?.(targetId) || (window.parent as any)?.__swissPrunedConversations?.has?.(targetId)))) {
      showToast('Conversation database was pruned or archived (local file not found)')
      return
    }
    if (!targetId) return

    // 1. If running inside Antigravity / Electron with main stage, close stage
    if (typeof (window as any).closeMainStage === 'function') {
      ;(window as any).closeMainStage()
    }
    const stageContainer = document.getElementById('swiss-main-stage-container')
    if (stageContainer) {
      stageContainer.style.display = 'none'
    }

    try {
      if (window.parent && window.parent !== window) {
        if (typeof (window.parent as any).closeMainStage === 'function') {
          ;(window.parent as any).closeMainStage()
        }
        const parentStage = window.parent.document.getElementById('swiss-main-stage-container')
        if (parentStage) {
          parentStage.style.display = 'none'
        }
      }
    } catch (_) {}

    // 2. Try clicking matching sidebar conversation element in DOM
    const targetSelector = `a[href*="/c/${targetId}"], [data-testid="conversation-row-sidebar"][data-id="${targetId}"], [data-conversation-id="${targetId}"]`
    let sidebarLink = document.querySelector(targetSelector) as HTMLElement | null
    if (!sidebarLink) {
      try {
        if (window.parent && window.parent !== window) {
          sidebarLink = window.parent.document.querySelector(targetSelector) as HTMLElement | null
        }
      } catch (_) {}
    }
    if (sidebarLink) {
      sidebarLink.click()
      return
    }

    const allLinks = document.querySelectorAll('a[href*="/c/"]')
    for (let i = 0; i < allLinks.length; i++) {
      const a = allLinks[i] as HTMLAnchorElement
      if (a.getAttribute('href')?.includes(targetId)) {
        a.click()
        return
      }
    }

    try {
      if (window.parent && window.parent !== window) {
        const parentLinks = window.parent.document.querySelectorAll('a[href*="/c/"]')
        for (let i = 0; i < parentLinks.length; i++) {
          const a = parentLinks[i] as HTMLAnchorElement
          if (a.getAttribute('href')?.includes(targetId)) {
            a.click()
            return
          }
        }
      }
    } catch (_) {}

    // 3. In-memory navigation via pushState preserving ?section= to prevent 404s in Web GUI and cold reloads in Electron
    try {
      const curUrl = new URL(window.location.href)
      const section = curUrl.searchParams.get('section')
      const query = section ? `?section=${encodeURIComponent(section)}` : ''
      window.history.pushState({}, '', `/c/${targetId}${query}`)
      window.dispatchEvent(new PopStateEvent('popstate'))
      if (window.parent && window.parent !== window) {
        window.parent.history.pushState({}, '', `/c/${targetId}${query}`)
        window.parent.dispatchEvent(new PopStateEvent('popstate'))
      }
    } catch (_) {
      // ignore
    }
  }

  const jumpToConversation = navigateToConversation
  ;(window as any).navigateToConversation = navigateToConversation
  ;(window as any).jumpToConversation = jumpToConversation

  const getEffectiveWorkspacePath = () => {
    if (scope && scope !== 'GLOBAL') {
      return scope
    }
    const lastActive = localStorage.getItem('antigravity_last_active_project')
    if (lastActive && lastActive !== 'GLOBAL') {
      return lastActive
    }
    if (fallbackProject && fallbackProject !== 'GLOBAL') {
      return fallbackProject
    }
    return scope === 'GLOBAL' ? 'GLOBAL' : '.'
  }

  const loadData = async () => {
    setLoading(true)
    const wsPath = getEffectiveWorkspacePath()
    try {
      const repoRes = await api.getGitHubRepo(wsPath)
      if (repoRes.success && repoRes.repo) {
        setRepo(repoRes.repo)
      } else {
        setRepo(null)
      }
      const issuesRes = await api.getGitHubIssues(wsPath, stateFilter)
      if (issuesRes.success && issuesRes.issues) {
        setIssues(issuesRes.issues)
        if (!selectedItem && issuesRes.issues.length > 0) {
          setSelectedItem(issuesRes.issues[0])
          setEditTitle(issuesRes.issues[0].title)
          setEditBody(issuesRes.issues[0].body || '')
        }
      } else {
        setIssues([])
      }
      const prsRes = await api.getGitHubPRs(wsPath)
      if (prsRes.success && prsRes.prs) {
        setPRs(prsRes.prs)
      } else {
        setPRs([])
      }
      const tasksRes = await api.getGitHubAgentTasks(wsPath)
      if (tasksRes.success && tasksRes.tasks) {
        setAgentTasks(tasksRes.tasks)
      } else {
        setAgentTasks([])
      }
      try {
        const kbRes = await api.getGitHubKanbanBoard(wsPath)
        if (kbRes.success && kbRes.board) {
          setKanbanBoard(kbRes.board)
        } else {
          setKanbanBoard(null)
        }
      } catch (_) {
        setKanbanBoard(null)
      }
    } catch (err: any) {
      showToast('Error loading GitHub data: ' + err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleMoveCard = async (targetColumnId: string) => {
    if (!draggedCard || draggedCard.column_id === targetColumnId) {
      setDraggedCard(null)
      setDragOverCol(null)
      return
    }

    const sourceColId = draggedCard.column_id
    const cardNum = draggedCard.number
    const cardType = draggedCard.type || (draggedCard.id?.startsWith('pr-') ? 'pr' : 'issue')

    // Optimistic UI update
    const currentCols = getEffectiveColumns()
    const nextCols = currentCols.map((col: KanbanColumn) => {
      if (col.id === sourceColId) {
        return { ...col, cards: col.cards.filter((c) => c.id !== draggedCard.id) }
      }
      if (col.id === targetColumnId) {
        return { ...col, cards: [...col.cards, { ...draggedCard, column_id: targetColumnId }] }
      }
      return col
    })
    setKanbanBoard({
      project_id: kanbanBoard?.project_id,
      project_title: kanbanBoard?.project_title,
      is_synthesized: kanbanBoard ? kanbanBoard.is_synthesized : true,
      columns: nextCols,
    })


    try {
      const res = await api.moveGitHubKanbanCard({
        workspace_path: getEffectiveWorkspacePath(),
        number: cardNum,
        card_id: draggedCard.id || `${cardType}-${cardNum}`,
        card_type: cardType,
        source_column: sourceColId,
        target_column: targetColumnId,
      })
      if (res.success) {
        showToast(`Card #${cardNum} moved to ${targetColumnId.replace('_', ' ').toUpperCase()}`)
        loadData()
      } else {
        showToast(res.error || 'Failed to move card')
        loadData()
      }
    } catch (err: any) {
      showToast('Move failed: ' + err.message)
      loadData()
    } finally {
      setDraggedCard(null)
      setDragOverCol(null)
    }
  }

  const getEffectiveColumns = () =>
    getEffectiveKanbanColumns(kanbanBoard, issues, prs, searchQuery)

  const handleDirectMove = async (item: any, itemType: string, sourceColId: string, targetColId: string) => {
    if (sourceColId === targetColId) return
    const cardNum = item.number
    const cardType = itemType || (item.pull_request ? 'pr' : 'issue')
    const cardId = item.id ? String(item.id) : `${cardType}-${cardNum}`

    // Optimistic UI update
    const currentCols = getEffectiveColumns()
    const nextCols = currentCols.map((col: KanbanColumn) => {
      if (col.id === sourceColId) {
        return { ...col, cards: col.cards.filter((c) => c.number !== cardNum) }
      }
      if (col.id === targetColId) {
        return { ...col, cards: [...col.cards, { ...item, column_id: targetColId }] }
      }
      return col
    })
    setKanbanBoard({
      project_id: kanbanBoard?.project_id,
      project_title: kanbanBoard?.project_title,
      is_synthesized: kanbanBoard ? kanbanBoard.is_synthesized : true,
      columns: nextCols,
    })

    try {
      const res = await api.moveGitHubKanbanCard({
        workspace_path: getEffectiveWorkspacePath(),
        number: cardNum,
        card_id: cardId,
        card_type: cardType,
        source_column: sourceColId,
        target_column: targetColId,
      })
      if (res.success) {
        showToast(`Card #${cardNum} moved to ${targetColId.replace('_', ' ').toUpperCase()}`)
        loadData()
      } else {
        showToast(res.error || 'Failed to move card')
        loadData()
      }
    } catch (err: any) {
      showToast('Move failed: ' + err.message)
      loadData()
    }
  }



  useEffect(() => {
    loadData()
  }, [stateFilter, scope, fallbackProject])

  const selectItemForEditing = (item: any) => {
    setSelectedItem(item)
    setEditTitle(item.title)
    setEditBody(item.body || '')
  }

  const handleSaveIssue = async () => {
    if (!selectedItem) return
    try {
      const res = await api.updateGitHubIssue({
        workspace_path: getEffectiveWorkspacePath(),
        number: selectedItem.number,
        title: editTitle,
        body: editBody,
      })
      if (res.success) {
        showToast(`Issue #${selectedItem.number} updated`)
        loadData()
      }
    } catch (err: any) {
      showToast('Update failed: ' + err.message)
    }
  }

  const handleToggleState = async () => {
    if (!selectedItem) return
    const isOpen = (selectedItem.state || 'open').toLowerCase() === 'open'
    const nextState = isOpen ? 'closed' : 'open'
    try {
      const res = await api.updateGitHubIssue({
        workspace_path: getEffectiveWorkspacePath(),
        number: selectedItem.number,
        state: nextState,
      })
      if (res.success) {
        showToast(`Issue #${selectedItem.number} ${nextState}`)
        loadData()
      }
    } catch (err: any) {
      showToast('State change failed: ' + err.message)
    }
  }

  const handleAddComment = async () => {
    if (!selectedItem || !commentText.trim()) return
    try {
      const res = await api.addGitHubComment({
        workspace_path: getEffectiveWorkspacePath(),
        number: selectedItem.number,
        comment: commentText.trim(),
      })
      if (res.success) {
        showToast('Comment posted')
        setCommentText('')
        const detail = await api.getGitHubIssueDetail(selectedItem.number, getEffectiveWorkspacePath())
        if (detail.success) {
          setSelectedItem(detail.issue)
        }
      }
    } catch (err: any) {
      showToast('Comment failed: ' + err.message)
    }
  }

  const handleCreateIssue = async () => {
    if (!newTitle.trim()) return
    try {
      const res = await api.createGitHubIssue({
        workspace_path: getEffectiveWorkspacePath(),
        title: newTitle.trim(),
        body: newBody.trim(),
      })
      if (res.success) {
        showToast('Created issue #' + res.issue.number)
        setIsCreatingIssue(false)
        setNewTitle('')
        setNewBody('')
        loadData()
      }
    } catch (err: any) {
      showToast('Creation failed: ' + err.message)
    }
  }

  const handleCopyPromptContext = async () => {
    if (!selectedItem) return
    try {
      const type = activeTab === 'prs' ? 'pr' : 'issue'
      const res = await api.getGitHubContext(selectedItem.number, type, getEffectiveWorkspacePath())
      if (res.success && res.context) {
        await navigator.clipboard.writeText(res.context)
        showToast('Copied issue context to clipboard (ready to paste to agent)!')
      }
    } catch (err: any) {
      showToast('Copy failed: ' + err.message)
    }
  }

  const handleLabelAgent = async (convId: string, currentLabel: string) => {
    const label = prompt('Enter custom role / label for this agent:', currentLabel || '')
    if (label !== null) {
      await api.setGitHubAgentLabel({
        conversation_id: convId,
        agent_label: label,
      })
      showToast('Agent labeled')
      loadData()
    }
  }

  const handleBindTask = async (convId: string) => {
    const numStr = prompt('Enter GitHub Issue # to bind to this conversation (or 0 to unbind):')
    if (numStr !== null) {
      const num = parseInt(numStr, 10) || 0
      await api.bindGitHubAgentTask({
        conversation_id: convId,
        issue_number: num,
      })
      showToast('Task association updated')
      loadData()
    }
  }

  const filteredItems = (activeTab === 'issues' ? issues : activeTab === 'prs' ? prs : agentTasks).filter(
    (item) => {
      const title = (item.title || item.conversation_title || '').toLowerCase()
      const num = (item.number || item.bound_issue_number || '').toString()
      const agent = (item.agent_label || item.agent_name || '').toLowerCase()
      const q = searchQuery.toLowerCase()
      return title.includes(q) || num.includes(q) || agent.includes(q)
    }
  )

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', overflow: 'hidden' }}>
      {/* Toast Notification */}
      {toastMsg && (
        <div
          style={{
            position: 'fixed',
            bottom: '24px',
            right: '24px',
            zIndex: 1000,
            backgroundColor: '#1a73e8',
            color: '#fff',
            padding: '10px 16px',
            borderRadius: '8px',
            fontSize: '13px',
            fontWeight: 500,
            boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
          }}
        >
          {toastMsg}
        </div>
      )}

      {/* Top Header Bar */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '8px 24px',
          borderBottom: '1px solid var(--border)',
          backgroundColor: 'var(--surface)',
          flexShrink: 0,
          gap: '12px',
          flexWrap: 'wrap',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          {repo && (
            <span
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '4px',
                padding: '4px 10px',
                borderRadius: '6px',
                fontSize: '12px',
                fontWeight: 500,
                backgroundColor: 'rgba(0,0,0,0.05)',
                color: 'var(--text-muted)',
              }}
            >
              <GitBranch size={13} />
              {repo.current_branch}
            </span>
          )}

          <div
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              padding: '2px',
              borderRadius: '6px',
              backgroundColor: 'var(--surface-variant)',
              border: '1px solid var(--border)',
              gap: '2px',
            }}
          >
            <button
              onClick={() => setViewMode('kanban')}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '5px',
                padding: '5px 10px',
                borderRadius: '4px',
                fontSize: '12px',
                fontWeight: viewMode === 'kanban' ? 600 : 500,
                backgroundColor: viewMode === 'kanban' ? 'var(--surface)' : 'transparent',
                color: viewMode === 'kanban' ? '#1a73e8' : 'var(--text-muted)',
                border: 'none',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
                boxShadow: viewMode === 'kanban' ? '0 1px 2px rgba(0,0,0,0.08)' : 'none',
              }}
            >
              <Kanban size={13} /> Board
            </button>
            <button
              onClick={() => {
                setViewMode('list')
                setActiveTab('issues')
              }}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '5px',
                padding: '5px 10px',
                borderRadius: '4px',
                fontSize: '12px',
                fontWeight: viewMode === 'list' && activeTab === 'issues' ? 600 : 500,
                backgroundColor: viewMode === 'list' && activeTab === 'issues' ? 'var(--surface)' : 'transparent',
                color: viewMode === 'list' && activeTab === 'issues' ? '#1a73e8' : 'var(--text-muted)',
                border: 'none',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
                boxShadow: viewMode === 'list' && activeTab === 'issues' ? '0 1px 2px rgba(0,0,0,0.08)' : 'none',
              }}
            >
              Issues ({issues.length})
            </button>
            <button
              onClick={() => {
                setViewMode('list')
                setActiveTab('prs')
              }}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '5px',
                padding: '5px 10px',
                borderRadius: '4px',
                fontSize: '12px',
                fontWeight: viewMode === 'list' && activeTab === 'prs' ? 600 : 500,
                backgroundColor: viewMode === 'list' && activeTab === 'prs' ? 'var(--surface)' : 'transparent',
                color: viewMode === 'list' && activeTab === 'prs' ? '#1a73e8' : 'var(--text-muted)',
                border: 'none',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
                boxShadow: viewMode === 'list' && activeTab === 'prs' ? '0 1px 2px rgba(0,0,0,0.08)' : 'none',
              }}
            >
              Pull Requests ({prs.length})
            </button>
            <button
              onClick={() => {
                setViewMode('list')
                setActiveTab('tasks')
              }}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '5px',
                padding: '5px 10px',
                borderRadius: '4px',
                fontSize: '12px',
                fontWeight: viewMode === 'list' && activeTab === 'tasks' ? 600 : 500,
                backgroundColor: viewMode === 'list' && activeTab === 'tasks' ? 'var(--surface)' : 'transparent',
                color: viewMode === 'list' && activeTab === 'tasks' ? '#1a73e8' : 'var(--text-muted)',
                border: 'none',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
                boxShadow: viewMode === 'list' && activeTab === 'tasks' ? '0 1px 2px rgba(0,0,0,0.08)' : 'none',
              }}
            >
              Agent Tasks ({agentTasks.length})
            </button>
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          {viewMode === 'list' && activeTab !== 'tasks' && (
            <div style={{ display: 'flex', gap: '6px' }}>
              {(['all', 'open', 'closed'] as const).map((filter) => (
                <button
                  key={filter}
                  onClick={() => setStateFilter(filter)}
                  style={{
                    padding: '4px 10px',
                    borderRadius: '6px',
                    fontSize: '11px',
                    fontWeight: stateFilter === filter ? 600 : 500,
                    backgroundColor: stateFilter === filter ? '#1a73e8' : 'transparent',
                    color: stateFilter === filter ? '#fff' : 'var(--text-muted)',
                    border: '1px solid var(--border)',
                    cursor: 'pointer',
                    textTransform: 'capitalize',
                  }}
                >
                  {filter}
                </button>
              ))}
            </div>
          )}

          <div style={{ position: 'relative' }}>
            <Search
              size={14}
              style={{ position: 'absolute', left: '10px', top: '10px', color: 'var(--text-muted)' }}
            />
            <input
              type="text"
              placeholder="Search issues, PRs, agents..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              style={{
                padding: '6px 12px 6px 30px',
                fontSize: '13px',
                borderRadius: '6px',
                border: '1px solid var(--border)',
                backgroundColor: 'var(--surface-variant)',
                color: 'var(--text)',
                width: '220px',
                outline: 'none',
              }}
            />
          </div>

          <button
            onClick={() => setIsCreatingIssue(true)}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: '6px',
              padding: '6px 12px',
              borderRadius: '6px',
              fontSize: '12px',
              fontWeight: 600,
              backgroundColor: '#1a73e8',
              color: '#fff',
              border: 'none',
              cursor: 'pointer',
            }}
          >
            <Plus size={14} /> New Issue
          </button>

          <button
            onClick={loadData}
            style={{
              padding: '6px 10px',
              borderRadius: '6px',
              border: '1px solid var(--border)',
              backgroundColor: 'transparent',
              color: 'var(--text-muted)',
              cursor: 'pointer',
            }}
            title="Refresh"
          >
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          </button>
        </div>
      </div>

      {viewMode === 'kanban' ? (
        <div
          style={{
            display: 'flex',
            flex: 1,
            minHeight: 0,
            padding: '16px 24px',
            gap: '16px',
            overflowX: 'auto',
            backgroundColor: 'var(--canvas-subtle, rgba(0,0,0,0.01))',
            boxSizing: 'border-box',
          }}
        >
          {getEffectiveColumns().map((col: any) => {
            const isDragOver = dragOverCol === col.id
            const cards = col.cards || []

            return (
              <div
                key={col.id}
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  flex: '1 1 240px',
                  minWidth: '260px',
                  maxWidth: '380px',
                  height: '100%',
                  borderRadius: '8px',
                  border: isDragOver ? '2px dashed #1a73e8' : '1px solid var(--border)',
                  backgroundColor: isDragOver ? 'rgba(26, 115, 232, 0.04)' : 'var(--surface)',
                  overflow: 'hidden',
                  boxSizing: 'border-box',
                  transition: 'background-color 0.15s ease, border-color 0.15s ease',
                }}
                onDragOver={(e) => {
                  e.preventDefault()
                  e.dataTransfer.dropEffect = 'move'
                  if (dragOverCol !== col.id) setDragOverCol(col.id)
                }}
                onDragLeave={(e) => {
                  if (!e.currentTarget.contains(e.relatedTarget as Node)) {
                    setDragOverCol(null)
                  }
                }}
                onDrop={(e) => {
                  e.preventDefault()
                  handleMoveCard(col.id)
                }}
              >
                {/* Column Header */}
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    padding: '10px 14px',
                    borderBottom: '1px solid var(--border)',
                    backgroundColor: 'rgba(0,0,0,0.02)',
                    userSelect: 'none',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                      {col.title}
                    </span>
                    <span
                      style={{
                        fontSize: '11px',
                        fontWeight: 600,
                        padding: '1px 7px',
                        borderRadius: '10px',
                        backgroundColor: 'rgba(0,0,0,0.06)',
                        color: 'var(--text-muted)',
                      }}
                    >
                      {cards.length}
                    </span>
                  </div>
                  {col.id === 'todo' && (
                    <button
                      onClick={() => setIsCreatingIssue(true)}
                      title="New Issue"
                      style={{
                        padding: '2px 6px',
                        fontSize: '12px',
                        fontWeight: 600,
                        borderRadius: '4px',
                        border: '1px solid var(--border)',
                        backgroundColor: 'transparent',
                        color: 'var(--text-muted)',
                        cursor: 'pointer',
                      }}
                    >
                      +
                    </button>
                  )}
                </div>

                {/* Cards Container */}
                <div
                  style={{
                    display: 'flex',
                    flexDirection: 'column',
                    flex: 1,
                    padding: '10px',
                    gap: '10px',
                    overflowY: 'auto',
                    minHeight: '80px',
                  }}
                >
                  {cards.length === 0 ? (
                    <div
                      style={{
                        padding: '24px 12px',
                        textAlign: 'center',
                        fontSize: '12px',
                        color: 'var(--text-muted)',
                        margin: 'auto',
                      }}
                    >
                      No items
                    </div>
                  ) : (
                    cards.map((card: any) => {
                      const isPR = card.type === 'pr' || (card.id && card.id.startsWith('pr-'))
                      const num = card.number
                      const title = card.title || ''
                      const state = (card.state || 'open').toLowerCase()
                      const assigned = card.assigned_agent
                      const labels = card.labels || []

                      let badgeBg = 'rgba(34, 197, 94, 0.12)'
                      let badgeColor = '#16a34a'
                      if (state === 'closed') {
                        badgeBg = 'rgba(100, 116, 139, 0.12)'
                        badgeColor = '#64748b'
                      } else if (isPR) {
                        badgeBg = 'rgba(168, 85, 247, 0.12)'
                        badgeColor = '#9333ea'
                      }

                      return (
                        <div
                          key={card.id || `${card.type}-${num}`}
                          draggable={true}
                          onContextMenu={(e) => {
                            e.preventDefault()
                            e.stopPropagation()
                            setContextMenu({
                              x: e.clientX,
                              y: e.clientY,
                              item: card,
                              itemType: isPR ? 'pr' : 'issue',
                              currentColId: col.id,
                            })
                          }}
                          onDragStart={(e) => {
                            setDraggedCard(card)
                            const markdownContext = `### GitHub ${isPR ? 'Pull Request' : 'Issue'} #${num}: ${title}\n- **Repository:** ${repo ? repo.full_name : ''}\n- **State:** ${state}\n\n${card.body || ''}`
                            e.dataTransfer.setData('text/plain', markdownContext)
                            e.dataTransfer.effectAllowed = 'move'
                          }}
                          onDragEnd={() => {
                            setDraggedCard(null)
                            setDragOverCol(null)
                          }}
                          style={{
                            display: 'flex',
                            flexDirection: 'column',
                            gap: '6px',
                            padding: '10px 12px',
                            borderRadius: '6px',
                            border: '1px solid var(--border)',
                            backgroundColor: 'var(--card, #ffffff)',
                            cursor: 'grab',
                            transition: 'all 0.12s ease',
                            userSelect: 'none',
                            boxShadow: '0 1px 3px rgba(0,0,0,0.04)',
                          }}
                        >
                          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '4px' }}>
                            <div style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
                              <GripVertical size={12} style={{ color: 'var(--text-muted)', opacity: 0.6 }} />
                              <span
                                style={{
                                  fontSize: '10.5px',
                                  fontWeight: 700,
                                  padding: '1px 5px',
                                  borderRadius: '4px',
                                  backgroundColor: badgeBg,
                                  color: badgeColor,
                                }}
                              >
                                {isPR ? 'PR #' : '#'}{num} {state.toUpperCase()}
                              </span>
                            </div>

                            {/* Live Agent Task Pulse Indicator */}
                            {assigned && (
                              <span
                                onClick={(e) => {
                                  if (assigned.conversation_id) {
                                    e.stopPropagation()
                                    navigateToConversation(assigned.conversation_id, assigned.root_parent_conversation_id, assigned.is_pruned)
                                  }
                                }}
                                style={{
                                  display: 'inline-flex',
                                  alignItems: 'center',
                                  gap: '5px',
                                  padding: '2px 6px',
                                  borderRadius: '10px',
                                  fontSize: '10px',
                                  fontWeight: 600,
                                  cursor: assigned.conversation_id && !assigned.is_pruned ? 'pointer' : 'default',
                                  backgroundColor: assigned.is_pruned ? 'rgba(148, 163, 184, 0.15)' : assigned.not_fully_idle ? 'rgba(34, 197, 94, 0.12)' : 'rgba(100, 116, 139, 0.12)',
                                  color: assigned.is_pruned ? '#94a3b8' : assigned.not_fully_idle ? '#15803d' : '#64748b',
                                  border: assigned.is_pruned ? '1px dashed #94a3b8' : assigned.not_fully_idle ? '1px solid rgba(34, 197, 94, 0.3)' : '1px solid transparent',
                                  opacity: assigned.is_pruned ? 0.75 : 1,
                                }}
                                title={assigned.is_pruned
                                  ? `Agent: ${assigned.agent_label || assigned.agent_name || 'Agent'} [Archived / Pruned]\nConversation database file is missing`
                                  : `Agent: ${assigned.agent_label || assigned.agent_name || 'Agent'} (${assigned.conversation_id ? assigned.conversation_id.slice(0, 8) : ''})${assigned.parent_conversation_id ? ` · Subagent of ${assigned.parent_conversation_id.slice(0, 8)}` : ''}\nClick to jump to conversation`}
                              >
                                {assigned.not_fully_idle && !assigned.is_pruned && (
                                  <span
                                    style={{
                                      width: '6px',
                                      height: '6px',
                                      borderRadius: '50%',
                                      backgroundColor: '#16a34a',
                                      boxShadow: '0 0 0 2px rgba(34, 197, 94, 0.4)',
                                    }}
                                  />
                                )}
                                {assigned.is_pruned ? 'Archived' : assigned.not_fully_idle ? 'Working' : 'Idle'}: {assigned.agent_label || 'Agent'}
                                {assigned.conversation_id && (
                                  <span style={{ opacity: 0.75, fontFamily: 'monospace', fontSize: '9px' }}>
                                    [#{assigned.conversation_id.slice(0, 6)}]
                                  </span>
                                )}
                              </span>
                            )}
                          </div>

                          <div
                            style={{
                              fontSize: '12.5px',
                              fontWeight: 500,
                              color: 'var(--text)',
                              lineHeight: 1.35,
                              wordBreak: 'break-word',
                            }}
                          >
                            {title}
                          </div>

                          {labels && labels.length > 0 && (
                            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px', marginTop: '2px' }}>
                              {labels.slice(0, 3).map((lbl: string, lIdx: number) => (
                                <span
                                  key={lIdx}
                                  style={{
                                    fontSize: '9.5px',
                                    padding: '1px 6px',
                                    borderRadius: '10px',
                                    backgroundColor: 'rgba(0,0,0,0.05)',
                                    color: 'var(--text-muted)',
                                    lineHeight: 1.2,
                                  }}
                                >
                                  {lbl}
                                </span>
                              ))}
                            </div>
                          )}

                          <div
                            style={{
                              display: 'flex',
                              alignItems: 'center',
                              justifyContent: 'space-between',
                              paddingTop: '6px',
                              borderTop: '1px solid var(--border)',
                              marginTop: '2px',
                            }}
                          >
                            <span style={{ fontSize: '10.5px', color: 'var(--text-muted)' }}>
                              @{card.author || 'user'}
                            </span>
                            <div style={{ display: 'flex', gap: '4px' }}>
                              <button
                                onClick={async () => {
                                  const text = `### GitHub ${isPR ? 'Pull Request' : 'Issue'} #${num}: ${title}\n- **Repository:** ${repo ? repo.full_name : ''}\n- **State:** ${state}\n\n${card.body || ''}`
                                  await navigator.clipboard.writeText(text)
                                  showToast(`Copied #${num} markdown context to clipboard`)
                                }}
                                style={{
                                  padding: '3px 6px',
                                  fontSize: '10px',
                                  fontWeight: 500,
                                  borderRadius: '4px',
                                  border: '1px solid var(--border)',
                                  backgroundColor: 'transparent',
                                  color: 'var(--text-muted)',
                                  cursor: 'pointer',
                                  display: 'inline-flex',
                                  alignItems: 'center',
                                  gap: '3px',
                                  whiteSpace: 'nowrap',
                                }}
                                title="Copy context for agent prompt"
                              >
                                <Send size={9} /> Copy Prompt
                              </button>
                              <button
                                onClick={() => {
                                  selectItemForEditing(card)
                                  setViewMode('list')
                                }}
                                style={{
                                  padding: '3px 6px',
                                  fontSize: '10px',
                                  fontWeight: 500,
                                  borderRadius: '4px',
                                  border: '1px solid var(--border)',
                                  backgroundColor: 'transparent',
                                  color: 'var(--text-muted)',
                                  cursor: 'pointer',
                                  display: 'inline-flex',
                                  alignItems: 'center',
                                  gap: '3px',
                                  whiteSpace: 'nowrap',
                                }}
                                title="Edit details"
                              >
                                Edit
                              </button>
                            </div>
                          </div>
                        </div>
                      )
                    })
                  )}
                </div>
              </div>
            )
          })}
        </div>
      ) : (
        /* 3-Column Workspace */
        <div style={{ display: 'flex', flex: 1, minHeight: 0, overflow: 'hidden' }}>
        {/* Column 1: Items List */}
        <div
          style={{
            width: '360px',
            borderRight: '1px solid var(--border)',
            display: 'flex',
            flexDirection: 'column',
            overflowY: 'auto',
            backgroundColor: 'var(--surface-variant)',
          }}
        >
          {filteredItems.map((item, idx) => {
            const isSelected = selectedItem && (selectedItem.number === item.number || selectedItem.conversation_id === item.conversation_id)
            if (activeTab === 'tasks') {
              const isWorking = item.not_fully_idle
              return (
                <div
                  key={item.conversation_id || idx}
                  onClick={() => setSelectedItem(item)}
                  style={{
                    padding: '12px 16px',
                    borderBottom: '1px solid var(--border)',
                    backgroundColor: isSelected ? 'rgba(26, 115, 232, 0.08)' : 'transparent',
                    cursor: 'pointer',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <span
                      style={{
                        display: 'inline-flex',
                        alignItems: 'center',
                        gap: '6px',
                        padding: '2px 8px',
                        borderRadius: '6px',
                        fontSize: '11px',
                        fontWeight: 600,
                        backgroundColor: isWorking ? 'rgba(34, 197, 94, 0.15)' : 'rgba(148, 163, 184, 0.15)',
                        color: isWorking ? '#15803d' : '#64748b',
                      }}
                    >
                      <Bot size={12} />
                      {isWorking ? 'Working' : 'Idle'}: {item.agent_label || 'Agent'}
                    </span>
                    <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>{item.step_count} steps</span>
                  </div>
                  <div style={{ fontSize: '13px', fontWeight: 600, marginTop: '6px', color: 'var(--text)' }}>
                    {item.conversation_title || 'Active Conversation'}
                  </div>
                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      marginTop: '8px',
                      fontSize: '11px',
                      color: 'var(--text-muted)',
                    }}
                  >
                    <span>{item.bound_issue_number ? `Bound to #${item.bound_issue_number}` : 'Unbound'}</span>
                    <button
                      onClick={(e) => {
                        e.stopPropagation()
                        handleLabelAgent(item.conversation_id, item.agent_label)
                      }}
                      style={{
                        border: 'none',
                        background: 'transparent',
                        color: '#1a73e8',
                        cursor: 'pointer',
                        fontSize: '11px',
                        fontWeight: 600,
                      }}
                    >
                      Label Agent
                    </button>
                  </div>
                </div>
              )
            }

            const isOpen = (item.state || 'open').toLowerCase() === 'open'
            const assigned = item.assigned_agent

            return (
              <div
                key={item.number || idx}
                onClick={() => selectItemForEditing(item)}
                onContextMenu={(e) => {
                  e.preventDefault()
                  e.stopPropagation()
                  const colId = findItemColumn(getEffectiveColumns(), item.number, activeTab === 'prs' ? 'pr' : 'issue')
                  setContextMenu({
                    x: e.clientX,
                    y: e.clientY,
                    item,
                    itemType: activeTab === 'prs' ? 'pr' : 'issue',
                    currentColId: colId,
                  })
                }}
                style={{
                  padding: '12px 16px',
                  borderBottom: '1px solid var(--border)',
                  backgroundColor: isSelected ? 'rgba(26, 115, 232, 0.08)' : 'transparent',
                  cursor: 'pointer',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '6px' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                    {activeTab === 'prs' ? (
                      <GitPullRequest size={14} color="#9333ea" />
                    ) : isOpen ? (
                      <AlertCircle size={14} color="#16a34a" />
                    ) : (
                      <CheckCircle2 size={14} color="#64748b" />
                    )}
                    <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)' }}>
                      #{item.number}
                    </span>
                  </div>
                  {assigned && (
                    <span
                      onClick={(e) => {
                        if (assigned.conversation_id) {
                          e.stopPropagation()
                          navigateToConversation(assigned.conversation_id, assigned.root_parent_conversation_id, assigned.is_pruned)
                        }
                      }}
                      style={{
                        display: 'inline-flex',
                        alignItems: 'center',
                        gap: '4px',
                        padding: '1px 6px',
                        borderRadius: '10px',
                        fontSize: '10px',
                        fontWeight: 600,
                        cursor: assigned.conversation_id && !assigned.is_pruned ? 'pointer' : 'default',
                        backgroundColor: assigned.is_pruned ? 'rgba(148, 163, 184, 0.15)' : assigned.not_fully_idle ? 'rgba(34, 197, 94, 0.15)' : 'rgba(148, 163, 184, 0.15)',
                        color: assigned.is_pruned ? '#94a3b8' : assigned.not_fully_idle ? '#15803d' : '#64748b',
                        border: assigned.is_pruned ? '1px dashed #94a3b8' : 'none',
                        opacity: assigned.is_pruned ? 0.75 : 1,
                      }}
                      title={assigned.is_pruned
                        ? `Agent: ${assigned.agent_label || assigned.agent_name || 'Agent'} [Archived / Pruned]\nConversation database file is missing`
                        : `Agent: ${assigned.agent_label || assigned.agent_name || 'Agent'} (${assigned.conversation_id ? assigned.conversation_id.slice(0, 8) : ''})${assigned.parent_conversation_id ? ` · Subagent of ${assigned.parent_conversation_id.slice(0, 8)}` : ''}\nClick to jump to conversation`}
                    >
                      <Bot size={10} />
                      {assigned.is_pruned ? `${assigned.agent_label || 'Agent'} [Archived]` : (assigned.agent_label || 'Agent')}
                      {assigned.conversation_id && (
                        <span style={{ opacity: 0.75, fontFamily: 'monospace', fontSize: '9px' }}>
                          [#{assigned.conversation_id.slice(0, 6)}]
                        </span>
                      )}
                    </span>
                  )}
                </div>

                <div style={{ fontSize: '13px', fontWeight: 600, marginTop: '4px', color: 'var(--text)', lineHeight: 1.3 }}>
                  {item.title}
                </div>

                {item.labels && item.labels.length > 0 && (
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px', marginTop: '6px' }}>
                    {item.labels.slice(0, 3).map((l: string, i: number) => (
                      <span
                        key={i}
                        style={{
                          fontSize: '10px',
                          padding: '1px 6px',
                          borderRadius: '8px',
                          backgroundColor: 'rgba(0,0,0,0.05)',
                          color: 'var(--text-muted)',
                        }}
                      >
                        {l}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            )
          })}
        </div>

        {/* Column 2: In-Place Detail Viewer & Editor */}
        <div
          style={{
            flex: 1,
            display: 'flex',
            flexDirection: 'column',
            overflowY: 'auto',
            padding: '24px 32px',
            backgroundColor: 'var(--surface)',
          }}
        >
          {selectedItem ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                  <span
                    style={{
                      padding: '4px 10px',
                      borderRadius: '6px',
                      fontSize: '12px',
                      fontWeight: 700,
                      backgroundColor:
                        (selectedItem.state || 'open').toLowerCase() === 'open'
                          ? 'rgba(34, 197, 94, 0.15)'
                          : 'rgba(148, 163, 184, 0.15)',
                      color:
                        (selectedItem.state || 'open').toLowerCase() === 'open' ? '#16a34a' : '#64748b',
                    }}
                  >
                    #{selectedItem.number} {(selectedItem.state || 'open').toUpperCase()}
                  </span>
                  {selectedItem.url && (
                    <a
                      href={selectedItem.url}
                      target="_blank"
                      rel="noreferrer"
                      style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', fontSize: '12px', color: '#1a73e8' }}
                    >
                      View on GitHub <ExternalLink size={12} />
                    </a>
                  )}
                </div>

                <div style={{ display: 'flex', gap: '8px' }}>
                  <button
                    onClick={handleCopyPromptContext}
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: '6px',
                      padding: '6px 12px',
                      borderRadius: '6px',
                      fontSize: '12px',
                      fontWeight: 600,
                      backgroundColor: 'rgba(26, 115, 232, 0.1)',
                      color: '#1a73e8',
                      border: 'none',
                      cursor: 'pointer',
                    }}
                  >
                    <Sparkles size={14} /> Copy Context for Agent
                  </button>
                  <button
                    onClick={handleToggleState}
                    style={{
                      padding: '6px 12px',
                      borderRadius: '6px',
                      fontSize: '12px',
                      fontWeight: 600,
                      backgroundColor: 'var(--surface-variant)',
                      border: '1px solid var(--border)',
                      color: 'var(--text)',
                      cursor: 'pointer',
                    }}
                  >
                    {(selectedItem.state || 'open').toLowerCase() === 'open' ? 'Close Issue' : 'Reopen Issue'}
                  </button>
                </div>
              </div>

              <div>
                <label style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)' }}>TITLE</label>
                <input
                  type="text"
                  value={editTitle}
                  onChange={(e) => setEditTitle(e.target.value)}
                  style={{
                    width: '100%',
                    fontSize: '18px',
                    fontWeight: 600,
                    padding: '8px 12px',
                    borderRadius: '6px',
                    border: '1px solid var(--border)',
                    backgroundColor: 'var(--surface-variant)',
                    color: 'var(--text)',
                    marginTop: '4px',
                    boxSizing: 'border-box',
                  }}
                />
              </div>

              <div>
                <label style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)' }}>DESCRIPTION</label>
                <textarea
                  value={editBody}
                  onChange={(e) => setEditBody(e.target.value)}
                  style={{
                    width: '100%',
                    minHeight: '220px',
                    fontSize: '13px',
                    fontFamily: 'inherit',
                    padding: '10px 12px',
                    borderRadius: '6px',
                    border: '1px solid var(--border)',
                    backgroundColor: 'var(--surface-variant)',
                    color: 'var(--text)',
                    marginTop: '4px',
                    resize: 'vertical',
                    boxSizing: 'border-box',
                  }}
                />
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
                <button
                  onClick={handleSaveIssue}
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: '6px',
                    padding: '8px 16px',
                    borderRadius: '6px',
                    fontSize: '13px',
                    fontWeight: 600,
                    backgroundColor: '#1a73e8',
                    color: '#fff',
                    border: 'none',
                    cursor: 'pointer',
                  }}
                >
                  <Save size={14} /> Save Changes
                </button>
              </div>

              {/* Comments Thread */}
              <div style={{ marginTop: '16px', borderTop: '1px solid var(--border)', paddingTop: '16px' }}>
                <div style={{ fontSize: '14px', fontWeight: 600, marginBottom: '12px' }}>
                  Comments ({selectedItem.comments ? selectedItem.comments.length : 0})
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', marginBottom: '16px' }}>
                  {(selectedItem.comments || []).map((c: any, i: number) => (
                    <div
                      key={i}
                      style={{
                        padding: '12px',
                        borderRadius: '6px',
                        backgroundColor: 'var(--surface-variant)',
                        border: '1px solid var(--border)',
                      }}
                    >
                      <div style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
                        @{c.author}
                      </div>
                      <div style={{ fontSize: '13px', color: 'var(--text-muted)', whiteSpace: 'pre-wrap' }}>
                        {c.body}
                      </div>
                    </div>
                  ))}
                </div>

                <div style={{ display: 'flex', gap: '8px' }}>
                  <input
                    type="text"
                    placeholder="Write a comment..."
                    value={commentText}
                    onChange={(e) => setCommentText(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') handleAddComment()
                    }}
                    style={{
                      flex: 1,
                      padding: '8px 12px',
                      fontSize: '13px',
                      borderRadius: '6px',
                      border: '1px solid var(--border)',
                      backgroundColor: 'var(--surface-variant)',
                      color: 'var(--text)',
                      outline: 'none',
                    }}
                  />
                  <button
                    onClick={handleAddComment}
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: '4px',
                      padding: '8px 16px',
                      borderRadius: '6px',
                      fontSize: '13px',
                      fontWeight: 600,
                      backgroundColor: '#1a73e8',
                      color: '#fff',
                      border: 'none',
                      cursor: 'pointer',
                    }}
                  >
                    <Send size={14} /> Comment
                  </button>
                </div>
              </div>
            </div>
          ) : (
            <div style={{ margin: 'auto', textAlign: 'center', color: 'var(--text-muted)', fontSize: '14px' }}>
              Select an item to view and edit details
            </div>
          )}
        </div>

        {/* Column 3: Multi-Agent & Conversation Task Tracker */}
        <div
          style={{
            width: '320px',
            borderLeft: '1px solid var(--border)',
            display: 'flex',
            flexDirection: 'column',
            backgroundColor: 'var(--surface-variant)',
          }}
        >
          <div
            style={{
              padding: '12px 16px',
              borderBottom: '1px solid var(--border)',
              fontSize: '13px',
              fontWeight: 700,
              color: 'var(--text)',
            }}
          >
            Agent Conversations ({agentTasks.length})
          </div>
          <div style={{ overflowY: 'auto', flex: 1 }}>
            {agentTasks.map((t) => {
              const isWorking = t.not_fully_idle
              return (
                <div
                  key={t.conversation_id}
                  style={{
                    padding: '12px 16px',
                    borderBottom: '1px solid var(--border)',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '4px',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <span
                      style={{
                        display: 'inline-flex',
                        alignItems: 'center',
                        gap: '6px',
                        padding: '2px 8px',
                        borderRadius: '6px',
                        fontSize: '11px',
                        fontWeight: 600,
                        backgroundColor: isWorking ? 'rgba(34, 197, 94, 0.15)' : 'rgba(148, 163, 184, 0.15)',
                        color: isWorking ? '#15803d' : '#64748b',
                      }}
                    >
                      <Bot size={12} />
                      {isWorking ? 'Working' : 'Idle'}: {t.agent_label || 'Agent'}
                    </span>
                    <button
                      onClick={() => handleLabelAgent(t.conversation_id, t.agent_label)}
                      style={{
                        border: 'none',
                        background: 'transparent',
                        color: '#1a73e8',
                        cursor: 'pointer',
                        fontSize: '11px',
                        fontWeight: 600,
                      }}
                    >
                      Label
                    </button>
                  </div>

                  <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                    {t.conversation_title || 'Conversation'}
                  </div>

                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginTop: '2px' }}>
                    <span style={{ fontSize: '10px', fontFamily: 'monospace', color: 'var(--text-muted)' }}>
                      ID: #{t.conversation_id.slice(0, 8)}
                    </span>
                    {t.parent_conversation_id && (
                      <span style={{ fontSize: '10px', color: '#1a73e8', display: 'flex', alignItems: 'center', gap: '2px' }} title={`Subagent of ${t.parent_conversation_id}`}>
                        <CornerDownRight size={10} />
                        <span>Subagent</span>
                      </span>
                    )}
                    {t.is_pruned && (
                      <span style={{ fontSize: '9.5px', color: '#94a3b8', background: 'rgba(148, 163, 184, 0.2)', padding: '1px 4px', borderRadius: '3px' }} title="Database file missing">
                        Archived
                      </span>
                    )}
                    <button
                      onClick={() => { navigateToConversation(t.conversation_id, t.root_parent_conversation_id, t.is_pruned) }}
                      style={{
                        border: 'none',
                        background: 'transparent',
                        color: t.is_pruned ? '#94a3b8' : '#1a73e8',
                        cursor: 'pointer',
                        fontSize: '10px',
                        fontWeight: 600,
                        padding: '0 2px',
                        marginLeft: 'auto',
                      }}
                      title={t.is_pruned ? "Conversation database was pruned or archived" : "Focus conversation"}
                    >
                      {t.is_pruned ? 'Archived' : 'Focus'}
                    </button>
                  </div>

                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      marginTop: '6px',
                      fontSize: '11px',
                      color: 'var(--text-muted)',
                    }}
                  >
                    <span>{t.bound_issue_number ? `Task: #${t.bound_issue_number}` : 'No Issue Bound'}</span>
                    <button
                      onClick={() => handleBindTask(t.conversation_id)}
                      style={{
                        border: 'none',
                        background: 'transparent',
                        color: '#1a73e8',
                        cursor: 'pointer',
                        fontSize: '11px',
                        fontWeight: 600,
                      }}
                    >
                      Bind Task
                    </button>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      </div>
      )}

      {/* New Issue Modal */}
      {isCreatingIssue && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            backgroundColor: 'rgba(0,0,0,0.5)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
          }}
        >
          <div
            style={{
              width: '560px',
              backgroundColor: 'var(--surface)',
              borderRadius: '10px',
              padding: '24px',
              display: 'flex',
              flexDirection: 'column',
              gap: '16px',
              boxShadow: '0 8px 30px rgba(0,0,0,0.2)',
            }}
          >
            <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>Create New Issue</div>
            <div>
              <label style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)' }}>TITLE</label>
              <input
                type="text"
                placeholder="Issue title"
                value={newTitle}
                onChange={(e) => setNewTitle(e.target.value)}
                style={{
                  width: '100%',
                  fontSize: '14px',
                  padding: '8px 12px',
                  borderRadius: '6px',
                  border: '1px solid var(--border)',
                  backgroundColor: 'var(--surface-variant)',
                  color: 'var(--text)',
                  marginTop: '4px',
                  boxSizing: 'border-box',
                }}
              />
            </div>
            <div>
              <label style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)' }}>DESCRIPTION</label>
              <textarea
                placeholder="Describe the issue or feature request..."
                value={newBody}
                onChange={(e) => setNewBody(e.target.value)}
                style={{
                  width: '100%',
                  minHeight: '160px',
                  fontSize: '13px',
                  padding: '8px 12px',
                  borderRadius: '6px',
                  border: '1px solid var(--border)',
                  backgroundColor: 'var(--surface-variant)',
                  color: 'var(--text)',
                  marginTop: '4px',
                  boxSizing: 'border-box',
                }}
              />
            </div>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
              <button
                onClick={() => setIsCreatingIssue(false)}
                style={{
                  padding: '8px 16px',
                  borderRadius: '6px',
                  fontSize: '13px',
                  backgroundColor: 'transparent',
                  border: '1px solid var(--border)',
                  color: 'var(--text)',
                  cursor: 'pointer',
                }}
              >
                Cancel
              </button>
              <button
                onClick={handleCreateIssue}
                style={{
                  padding: '8px 16px',
                  borderRadius: '6px',
                  fontSize: '13px',
                  fontWeight: 600,
                  backgroundColor: '#1a73e8',
                  color: '#fff',
                  border: 'none',
                  cursor: 'pointer',
                }}
              >
                Create Issue
              </button>
            </div>
          </div>
        </div>
      )}

      {contextMenu && (
        <div
          style={{
            position: 'fixed',
            left: Math.min(contextMenu.x, window.innerWidth - 200),
            top: Math.min(contextMenu.y, window.innerHeight - 240),
            zIndex: 99999,
            minWidth: '180px',
            backgroundColor: 'var(--surface)',
            border: '1px solid var(--border)',
            borderRadius: '10px',
            boxShadow: '0 4px 16px rgba(0,0,0,0.12), 0 1px 3px rgba(0,0,0,0.08)',
            padding: '4px',
            userSelect: 'none',
          }}
          onClick={(e) => e.stopPropagation()}
        >
          <div
            style={{
              fontSize: '10px',
              fontWeight: 700,
              textTransform: 'uppercase',
              letterSpacing: '0.5px',
              color: 'var(--text-muted)',
              padding: '6px 10px 4px',
            }}
          >
            Move to Board
          </div>
          {[
            { id: 'todo', label: 'Todo' },
            { id: 'in_progress', label: 'In Progress' },
            { id: 'review', label: 'Review' },
            { id: 'done', label: 'Done' },
          ].map((col) => {
            const isCurrent = contextMenu.currentColId === col.id
            return (
              <button
                key={col.id}
                onClick={async () => {
                  const targetCol = col.id
                  const card = contextMenu.item
                  const itemType = contextMenu.itemType
                  const currentCol = contextMenu.currentColId
                  setContextMenu(null)
                  await handleDirectMove(card, itemType, currentCol, targetCol)
                }}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  width: '100%',
                  padding: '6px 10px',
                  borderRadius: '6px',
                  fontSize: '12px',
                  color: isCurrent ? '#1a73e8' : 'var(--text)',
                  fontWeight: isCurrent ? 600 : 400,
                  background: 'transparent',
                  border: 'none',
                  cursor: 'pointer',
                  textAlign: 'left',
                  boxSizing: 'border-box',
                }}
                onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'rgba(26,115,232,0.08)')}
                onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
              >
                <span>{col.label}</span>
                {isCurrent && <Check size={13} color="#1a73e8" />}
              </button>
            )
          })}
          <div style={{ height: '1px', backgroundColor: 'var(--border)', margin: '4px 0', opacity: 0.6 }} />
          <button
            onClick={async () => {
              const card = contextMenu.item
              setContextMenu(null)
              if (card.assigned_agent && card.assigned_agent.conversation_id) {
                navigateToConversation(
                  card.assigned_agent.conversation_id,
                  card.assigned_agent.root_parent_conversation_id,
                  card.assigned_agent.is_pruned
                )
              } else {
                const isPR = contextMenu.itemType === 'pr'
                const text = `### GitHub ${isPR ? 'Pull Request' : 'Issue'} #${card.number}: ${card.title}\n- **Repository:** ${repo ? repo.full_name : ''}\n- **State:** ${card.state || 'open'}\n\n${card.body || ''}`
                await navigator.clipboard.writeText(text)
                showToast(`Copied #${card.number} markdown context to clipboard`)
              }
            }}
            style={{
              display: 'flex',
              alignItems: 'center',
              width: '100%',
              padding: '6px 10px',
              borderRadius: '6px',
              fontSize: '12px',
              color: 'var(--text)',
              background: 'transparent',
              border: 'none',
              cursor: 'pointer',
              textAlign: 'left',
              boxSizing: 'border-box',
            }}
            onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'rgba(26,115,232,0.08)')}
            onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
          >
            {contextMenu.item.assigned_agent && contextMenu.item.assigned_agent.conversation_id
              ? `Jump to ${contextMenu.item.assigned_agent.agent_label || 'Agent'} [#{contextMenu.item.assigned_agent.conversation_id.slice(0, 6)}]${contextMenu.item.assigned_agent.is_pruned ? ' (Archived)' : ''}`
              : 'Chat with Agent'}
          </button>
          <button
            onClick={() => {
              selectItemForEditing(contextMenu.item)
              setViewMode('list')
              setContextMenu(null)
            }}
            style={{
              display: 'flex',
              alignItems: 'center',
              width: '100%',
              padding: '6px 10px',
              borderRadius: '6px',
              fontSize: '12px',
              color: 'var(--text)',
              background: 'transparent',
              border: 'none',
              cursor: 'pointer',
              textAlign: 'left',
              boxSizing: 'border-box',
            }}
            onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'rgba(26,115,232,0.08)')}
            onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
          >
            View & Edit Details
          </button>
          {contextMenu.item.url && (
            <button
              onClick={() => {
                const url = contextMenu.item.url
                setContextMenu(null)
                window.open(url, '_blank')
              }}
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                width: '100%',
                padding: '6px 10px',
                borderRadius: '6px',
                fontSize: '12px',
                color: 'var(--text)',
                background: 'transparent',
                border: 'none',
                cursor: 'pointer',
                textAlign: 'left',
                boxSizing: 'border-box',
              }}
              onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'rgba(26,115,232,0.08)')}
              onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
            >
              <span>Open on GitHub</span>
              <ExternalLink size={12} style={{ opacity: 0.7 }} />
            </button>
          )}
        </div>
      )}
    </div>
  )
}
