import type { KanbanBoard, KanbanCard, KanbanColumn, AgentTaskSummary } from '../types.ts'

export const WIP_LABELS = ['in progress', 'in-progress', 'wip', 'doing', 'active', 'working']
export const REVIEW_LABELS = ['review', 'in review', 'in-review', 'needs review', 'under review', 'qa']

export function isWIPLabel(label: string): boolean {
  if (!label) return false
  return WIP_LABELS.includes(label.trim().toLowerCase())
}

export function isReviewLabel(label: string): boolean {
  if (!label) return false
  return REVIEW_LABELS.includes(label.trim().toLowerCase())
}

export function categorizeIssue(issue?: {
  state?: string
  assigned_agent?: AgentTaskSummary | null
  labels?: string[]
} | null): 'todo' | 'in_progress' | 'review' | 'done' {
  if (!issue) return 'todo'
  const state = (issue.state || 'open').trim().toLowerCase()
  if (state === 'closed') {
    return 'done'
  }
  if (issue.assigned_agent) {
    return 'in_progress'
  }
  const labels = issue.labels || []
  if (labels.some(isWIPLabel)) {
    return 'in_progress'
  }
  if (labels.some(isReviewLabel)) {
    return 'review'
  }
  return 'todo'
}

export function categorizePullRequest(pr?: {
  state?: string
  is_draft?: boolean
  labels?: string[]
} | null): 'todo' | 'in_progress' | 'review' | 'done' {
  if (!pr) return 'review'
  const state = (pr.state || 'open').trim().toLowerCase()
  if (state === 'closed' || state === 'merged') {
    return 'done'
  }
  if (pr.is_draft || (pr.labels || []).some(isWIPLabel)) {
    return 'in_progress'
  }
  return 'review'
}

export function synthesizeKanbanBoard(issues: any[] = [], prs: any[] = []): KanbanBoard {
  const colMap: Record<string, KanbanCard[]> = {
    todo: [],
    in_progress: [],
    review: [],
    done: [],
  }

  if (Array.isArray(issues)) {
    issues.forEach((iss) => {
      if (!iss) return
      const colId = categorizeIssue(iss)
      const st = (iss.state || 'open').toLowerCase()
      colMap[colId].push({
        id: `issue-${iss.number}`,
        type: 'issue',
        number: iss.number,
        title: iss.title || '',
        body: iss.body || '',
        state: st,
        column_id: colId,
        labels: iss.labels || [],
        author: iss.author || '',
        assigned_agent: iss.assigned_agent,
      })
    })
  }

  if (Array.isArray(prs)) {
    prs.forEach((pr) => {
      if (!pr) return
      const colId = categorizePullRequest(pr)
      const st = (pr.state || 'open').toLowerCase()
      colMap[colId].push({
        id: `pr-${pr.number}`,
        type: 'pr',
        number: pr.number,
        title: pr.title || '',
        body: pr.body || '',
        state: st,
        column_id: colId,
        labels: pr.labels || [],
        author: pr.author || '',
        assigned_agent: pr.assigned_agent,
      })
    })
  }

  return {
    is_synthesized: true,
    columns: [
      { id: 'todo', title: 'Todo', cards: colMap.todo },
      { id: 'in_progress', title: 'In Progress', cards: colMap.in_progress },
      { id: 'review', title: 'Review', cards: colMap.review },
      { id: 'done', title: 'Done', cards: colMap.done },
    ],
  }
}

export function filterColumnsByQuery(columns: KanbanColumn[], query?: string): KanbanColumn[] {
  if (!columns || !Array.isArray(columns)) {
    return []
  }

  if (!query || !query.trim()) {
    return columns
  }

  const q = query.trim().toLowerCase()
  return columns.map((col) => ({
    ...col,
    cards: (col.cards || []).filter((card) => {
      const title = (card.title || '').toLowerCase()
      const num = card.number !== undefined && card.number !== null ? card.number.toString() : ''
      const author = (card.author || '').toLowerCase()
      const agentLabel = (card.assigned_agent?.agent_label || '').toLowerCase()
      const agentName = (card.assigned_agent?.agent_name || '').toLowerCase()

      return (
        title.includes(q) ||
        num.includes(q) ||
        author.includes(q) ||
        agentLabel.includes(q) ||
        agentName.includes(q)
      )
    }),
  }))
}

export function getEffectiveKanbanColumns(
  board?: KanbanBoard | null,
  issues: any[] = [],
  prs: any[] = [],
  searchQuery?: string
): KanbanColumn[] {
  const baseColumns =
    board && board.columns && board.columns.length > 0
      ? board.columns
      : synthesizeKanbanBoard(issues, prs).columns

  return filterColumnsByQuery(baseColumns, searchQuery)
}
