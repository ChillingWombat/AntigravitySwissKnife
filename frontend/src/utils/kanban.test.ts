import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import {
  isWIPLabel,
  isReviewLabel,
  categorizeIssue,
  categorizePullRequest,
  synthesizeKanbanBoard,
  filterColumnsByQuery,
  getEffectiveKanbanColumns,
  findItemColumn,
} from './kanban.ts'
import type { KanbanBoard, KanbanColumn } from '../types.ts'

describe('kanban utility', () => {
  describe('isWIPLabel', () => {
    it('identifies standard WIP labels regardless of case and trimming', () => {
      assert.equal(isWIPLabel('wip'), true)
      assert.equal(isWIPLabel('WIP'), true)
      assert.equal(isWIPLabel('In Progress'), true)
      assert.equal(isWIPLabel('in-progress'), true)
      assert.equal(isWIPLabel('  doing  '), true)
      assert.equal(isWIPLabel('active'), true)
      assert.equal(isWIPLabel('working'), true)
    })

    it('rejects non-WIP labels and empty strings', () => {
      assert.equal(isWIPLabel(''), false)
      assert.equal(isWIPLabel('bug'), false)
      assert.equal(isWIPLabel('enhancement'), false)
      assert.equal(isWIPLabel('documentation'), false)
    })
  })

  describe('isReviewLabel', () => {
    it('identifies standard review labels regardless of case and trimming', () => {
      assert.equal(isReviewLabel('review'), true)
      assert.equal(isReviewLabel('In Review'), true)
      assert.equal(isReviewLabel('in-review'), true)
      assert.equal(isReviewLabel('needs review'), true)
      assert.equal(isReviewLabel('under review'), true)
      assert.equal(isReviewLabel('QA'), true)
      assert.equal(isReviewLabel('  qa  '), true)
    })

    it('rejects non-review labels and empty strings', () => {
      assert.equal(isReviewLabel(''), false)
      assert.equal(isReviewLabel('triage'), false)
      assert.equal(isReviewLabel('feature'), false)
    })
  })

  describe('categorizeIssue', () => {
    it('places closed issues in done column', () => {
      const col = categorizeIssue({ state: 'closed', labels: ['wip'] })
      assert.equal(col, 'done')
    })

    it('places closed issues in done even if assigned to an agent', () => {
      const col = categorizeIssue({
        state: 'closed',
        assigned_agent: {
          conversation_id: 'c-1',
          conversation_title: 'Agent task',
          agent_name: 'Lead',
          agent_label: 'Lead',
          status: 'working',
          not_fully_idle: true,
        },
      })
      assert.equal(col, 'done')
    })

    it('places open issues with an assigned agent in in_progress', () => {
      const col = categorizeIssue({
        state: 'open',
        assigned_agent: {
          conversation_id: 'c-1',
          conversation_title: 'Agent task',
          agent_name: 'Lead',
          agent_label: 'Lead',
          status: 'working',
          not_fully_idle: true,
        },
      })
      assert.equal(col, 'in_progress')
    })

    it('places open issues with WIP labels in in_progress', () => {
      const col = categorizeIssue({
        state: 'open',
        labels: ['bug', 'in-progress'],
      })
      assert.equal(col, 'in_progress')
    })

    it('places open issues with review labels in review', () => {
      const col = categorizeIssue({
        state: 'open',
        labels: ['needs review'],
      })
      assert.equal(col, 'review')
    })

    it('defaults open issues with other or no labels to todo', () => {
      const col1 = categorizeIssue({ state: 'open', labels: ['documentation'] })
      assert.equal(col1, 'todo')

      const col2 = categorizeIssue({ state: 'open' })
      assert.equal(col2, 'todo')
    })
  })

  describe('categorizePullRequest', () => {
    it('places closed or merged pull requests in done', () => {
      assert.equal(categorizePullRequest({ state: 'closed' }), 'done')
      assert.equal(categorizePullRequest({ state: 'merged' }), 'done')
    })

    it('places draft pull requests in in_progress', () => {
      assert.equal(categorizePullRequest({ state: 'open', is_draft: true }), 'in_progress')
    })

    it('places pull requests with WIP labels in in_progress', () => {
      assert.equal(
        categorizePullRequest({ state: 'open', is_draft: false, labels: ['wip'] }),
        'in_progress'
      )
    })

    it('defaults normal open pull requests to review', () => {
      assert.equal(
        categorizePullRequest({ state: 'open', is_draft: false, labels: ['frontend'] }),
        'review'
      )
    })
  })

  describe('synthesizeKanbanBoard', () => {
    it('returns 4 empty columns for empty inputs', () => {
      const board = synthesizeKanbanBoard([], [])
      assert.equal(board.is_synthesized, true)
      assert.equal(board.columns.length, 4)
      assert.deepEqual(
        board.columns.map((c) => c.id),
        ['todo', 'in_progress', 'review', 'done']
      )
      assert.equal(board.columns.every((c) => c.cards.length === 0), true)
    })

    it('synthesizes issues and PRs into appropriate columns', () => {
      const issues = [
        { number: 1, title: 'Open Task', state: 'open', author: 'alice' },
        {
          number: 2,
          title: 'Working Task',
          state: 'open',
          labels: ['wip'],
          author: 'bob',
          assigned_agent: {
            conversation_id: 'conv-123',
            conversation_title: 'Agent run',
            agent_name: 'Builder',
            agent_label: 'Agent Builder',
            status: 'working',
            not_fully_idle: true,
          },
        },
        { number: 3, title: 'Finished Task', state: 'closed', author: 'charlie' },
      ]
      const prs = [
        { number: 101, title: 'Draft Feature', state: 'open', is_draft: true, author: 'alice' },
        { number: 102, title: 'Ready for Review', state: 'open', is_draft: false, author: 'bob' },
        { number: 103, title: 'Merged Feature', state: 'merged', author: 'charlie' },
      ]

      const board = synthesizeKanbanBoard(issues, prs)
      const todoCol = board.columns.find((c) => c.id === 'todo')!
      const inProgressCol = board.columns.find((c) => c.id === 'in_progress')!
      const reviewCol = board.columns.find((c) => c.id === 'review')!
      const doneCol = board.columns.find((c) => c.id === 'done')!

      assert.equal(todoCol.cards.length, 1)
      assert.equal(todoCol.cards[0].id, 'issue-1')

      assert.equal(inProgressCol.cards.length, 2)
      assert.equal(inProgressCol.cards[0].id, 'issue-2')
      assert.equal(inProgressCol.cards[0].assigned_agent?.agent_label, 'Agent Builder')
      assert.equal(inProgressCol.cards[1].id, 'pr-101')

      assert.equal(reviewCol.cards.length, 1)
      assert.equal(reviewCol.cards[0].id, 'pr-102')

      assert.equal(doneCol.cards.length, 2)
      assert.equal(doneCol.cards[0].id, 'issue-3')
      assert.equal(doneCol.cards[1].id, 'pr-103')
    })
  })

  describe('filterColumnsByQuery', () => {
    const sampleColumns: KanbanColumn[] = [
      {
        id: 'todo',
        title: 'Todo',
        cards: [
          {
            id: 'issue-10',
            type: 'issue',
            number: 10,
            title: 'Refactor database indexing',
            state: 'open',
            column_id: 'todo',
            author: 'david',
          },
        ],
      },
      {
        id: 'in_progress',
        title: 'In Progress',
        cards: [
          {
            id: 'issue-11',
            type: 'issue',
            number: 11,
            title: 'Build Kanban board interface',
            state: 'open',
            column_id: 'in_progress',
            author: 'sarah',
            assigned_agent: {
              conversation_id: 'conv-55',
              conversation_title: 'Task 55',
              agent_name: 'Orchestrator',
              agent_label: 'Senior Lead',
              status: 'working',
              not_fully_idle: true,
            },
          },
        ],
      },
    ]

    it('returns unmodified columns when query is empty or whitespace', () => {
      const resEmpty = filterColumnsByQuery(sampleColumns, '')
      assert.equal(resEmpty[0].cards.length, 1)
      assert.equal(resEmpty[1].cards.length, 1)

      const resSpace = filterColumnsByQuery(sampleColumns, '   ')
      assert.equal(resSpace[0].cards.length, 1)
      assert.equal(resSpace[1].cards.length, 1)
    })

    it('filters cards by title case-insensitively', () => {
      const res = filterColumnsByQuery(sampleColumns, 'KANBAN')
      assert.equal(res[0].cards.length, 0)
      assert.equal(res[1].cards.length, 1)
      assert.equal(res[1].cards[0].number, 11)
    })

    it('filters cards by issue number', () => {
      const res = filterColumnsByQuery(sampleColumns, '10')
      assert.equal(res[0].cards.length, 1)
      assert.equal(res[1].cards.length, 0)
    })

    it('filters cards by author', () => {
      const res = filterColumnsByQuery(sampleColumns, 'sarah')
      assert.equal(res[0].cards.length, 0)
      assert.equal(res[1].cards.length, 1)
    })

    it('filters cards by assigned agent name or label', () => {
      const resByAgentName = filterColumnsByQuery(sampleColumns, 'orchestrator')
      assert.equal(resByAgentName[1].cards.length, 1)

      const resByAgentLabel = filterColumnsByQuery(sampleColumns, 'senior lead')
      assert.equal(resByAgentLabel[1].cards.length, 1)
    })
  })

  describe('getEffectiveKanbanColumns', () => {
    it('uses server board columns when available', () => {
      const serverBoard: KanbanBoard = {
        is_synthesized: false,
        columns: [
          {
            id: 'col-custom',
            title: 'Custom Col',
            cards: [
              {
                id: 'card-1',
                type: 'issue',
                number: 99,
                title: 'Server Provided Card',
                state: 'open',
                column_id: 'col-custom',
              },
            ],
          },
        ],
      }

      const effective = getEffectiveKanbanColumns(serverBoard, [], [])
      assert.equal(effective.length, 1)
      assert.equal(effective[0].id, 'col-custom')
      assert.equal(effective[0].cards[0].title, 'Server Provided Card')
    })

    it('falls back to synthesized board when server board is null or has no columns', () => {
      const issues = [{ number: 5, title: 'Fallback Issue', state: 'open' }]
      const effectiveNull = getEffectiveKanbanColumns(null, issues, [])
      assert.equal(effectiveNull.length, 4)
      assert.equal(effectiveNull[0].cards[0].title, 'Fallback Issue')

      const emptyBoard: KanbanBoard = { is_synthesized: false, columns: [] }
      const effectiveEmpty = getEffectiveKanbanColumns(emptyBoard, issues, [])
      assert.equal(effectiveEmpty.length, 4)
      assert.equal(effectiveEmpty[0].cards[0].title, 'Fallback Issue')
    })
  })

  describe('findItemColumn', () => {
    const mockColumns = [
      { id: 'todo', title: 'Todo', cards: [{ id: 'issue-10', number: 10, type: 'issue' }] },
      { id: 'in_progress', title: 'In Progress', cards: [{ id: 'issue-20', number: 20, type: 'issue' }] },
      { id: 'review', title: 'Review', cards: [{ id: 'pr-30', number: 30, type: 'pr' }] },
      { id: 'done', title: 'Done', cards: [{ id: 'issue-40', number: 40, type: 'issue' }] },
    ] as any

    it('locates column by number and type', () => {
      assert.equal(findItemColumn(mockColumns, 10, 'issue'), 'todo')
      assert.equal(findItemColumn(mockColumns, 20, 'issue'), 'in_progress')
      assert.equal(findItemColumn(mockColumns, 30, 'pr'), 'review')
      assert.equal(findItemColumn(mockColumns, 40, 'issue'), 'done')
    })

    it('falls back to todo if card number is not found', () => {
      assert.equal(findItemColumn(mockColumns, 999, 'issue'), 'todo')
    })

    it('handles null, undefined, or empty column lists safely', () => {
      assert.equal(findItemColumn(null as any, 10), 'todo')
      assert.equal(findItemColumn(undefined as any, 10), 'todo')
      assert.equal(findItemColumn([], 10), 'todo')
    })
  })

  describe('Null, undefined, and malformed edge cases', () => {
    it('handles null and undefined issues and PRs in categorization safely', () => {
      assert.equal(categorizeIssue(null), 'todo')
      assert.equal(categorizeIssue(undefined), 'todo')
      assert.equal(categorizePullRequest(null), 'review')
      assert.equal(categorizePullRequest(undefined), 'review')
    })

    it('handles arrays with null/undefined elements in synthesizeKanbanBoard', () => {
      const board = synthesizeKanbanBoard([null, undefined, { number: 1, title: 'Valid' }] as any, [null] as any)
      assert.equal(board.is_synthesized, true)
      assert.equal(board.columns[0].cards.length, 1)
      assert.equal(board.columns[0].cards[0].number, 1)
    })

    it('handles null, undefined, or malformed columns in filterColumnsByQuery', () => {
      assert.deepEqual(filterColumnsByQuery(null as any, 'query'), [])
      assert.deepEqual(filterColumnsByQuery(undefined as any, 'query'), [])
      assert.deepEqual(filterColumnsByQuery([] as any, 'query'), [])
    })
  })
})


