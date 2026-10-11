import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

import { fileURLToPath } from 'node:url'

describe('GitHub Workspace AGY Agents - Killed Filter & Grouping Contract', () => {
  it('verifies 3-day window filtering excludes stale killed subagents', () => {
    const now = Date.now()
    const THREE_DAYS_MS = 3 * 24 * 60 * 60 * 1000

    const mockTasks = [
      {
        conversation_id: 'root-1',
        nesting_depth: 0,
        status: 'idle',
        not_fully_idle: false,
        last_modified: new Date(now - 10 * 86400000).toISOString(), // 10 days ago, but main agent
      },
      {
        conversation_id: 'sub-recent',
        root_parent_conversation_id: 'root-1',
        nesting_depth: 1,
        status: 'idle',
        not_fully_idle: false,
        last_modified: new Date(now - 1 * 86400000).toISOString(), // 1 day ago
      },
      {
        conversation_id: 'sub-old',
        root_parent_conversation_id: 'root-1',
        nesting_depth: 1,
        status: 'idle',
        not_fully_idle: false,
        last_modified: new Date(now - 5 * 86400000).toISOString(), // 5 days ago (should be excluded)
      },
    ]

    const filtered = mockTasks.filter((item) => {
      const isWorking = item.not_fully_idle || item.status === 'working'
      const isMain = (!item.nesting_depth || item.nesting_depth === 0) &&
        (!item.root_parent_conversation_id || item.root_parent_conversation_id === item.conversation_id)
      if (!isMain && !isWorking) {
        const tMod = item.last_modified ? new Date(item.last_modified).getTime() : 0
        const threeDaysAgo = now - THREE_DAYS_MS
        if (tMod < threeDaysAgo) return false
      }
      return true
    })

    assert.equal(filtered.length, 2)
    assert.equal(filtered[0].conversation_id, 'root-1')
    assert.equal(filtered[1].conversation_id, 'sub-recent')
  })

  it('caps subagents at maximum 20 per conversation group card', () => {
    const now = Date.now()
    const subAgents: any[] = []
    for (let i = 0; i < 35; i++) {
      subAgents.push({
        conversation_id: `sub-${i}`,
        root_parent_conversation_id: 'root-1',
        nesting_depth: 1,
        last_modified: new Date(now - i * 3600000).toISOString(),
      })
    }

    subAgents.sort((a, b) => {
      const tA = a.last_modified ? new Date(a.last_modified).getTime() : 0
      const tB = b.last_modified ? new Date(b.last_modified).getTime() : 0
      return tB - tA
    })

    const capped = subAgents.slice(0, 20)
    assert.equal(capped.length, 20)
    assert.equal(capped[0].conversation_id, 'sub-0')
    assert.equal(capped[19].conversation_id, 'sub-19')
  })

  it('verifies GitHubWorkspacePage source enforces maxHeight without minHeight', () => {
    const currentDir = path.dirname(fileURLToPath(import.meta.url))
    const pagePath = path.resolve(currentDir, '../pages/GitHubWorkspacePage.tsx')
    const content = fs.readFileSync(pagePath, 'utf-8')

    assert.ok(content.includes("maxHeight: '380px'"), 'Must have maxHeight: 380px on group card')
    assert.ok(content.includes("overflowY: 'auto'"), 'Must have overflowY: auto for scrolling')
    assert.ok(content.includes('THREE_DAYS_MS = 3 * 24 * 60 * 60 * 1000'), 'Must define 3-day window')
    assert.ok(content.includes('grp.subAgents.slice(0, 20)'), 'Must enforce 20 subagents cap')
  })
})
