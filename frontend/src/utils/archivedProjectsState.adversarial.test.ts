import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import type { ArchivedProjectItem } from '../types'

describe('Adversarial & State Machine Verification: ArchivedProjectsPage (Milestone 10)', () => {
  const pagePath = path.resolve(import.meta.dirname, '../pages/ArchivedProjectsPage.tsx')
  const pageSrc = fs.readFileSync(pagePath, 'utf8')

  // Sample fixture data for state simulation
  const sampleProjects: ArchivedProjectItem[] = [
    {
      id: 'proj-1',
      name: 'Alpha Project',
      color: '#3b82f6',
      conversation_count: 5,
      last_active_time: '2026-10-08T12:00:00Z',
      last_active_relative: '1 day ago',
      folder_uri: 'file:///mnt/Data/Projects/Alpha',
      archived_at: '2026-10-09T08:00:00Z',
    },
    {
      id: 'proj-2',
      name: 'Beta Project [Special+Chars]',
      color: '#10b981',
      conversation_count: 12,
      last_active_time: '2026-10-09T02:00:00Z',
      last_active_relative: '8 hours ago',
      folder_uri: 'file:///home/david/workspace/beta-special',
      archived_at: '2026-10-09T09:00:00Z',
    },
    {
      id: 'proj-3',
      name: 'Gamma Tool',
      color: '#8b5cf6',
      conversation_count: 0,
      last_active_time: '',
      last_active_relative: 'Never active',
      folder_uri: '',
      archived_at: '2026-10-09T10:00:00Z',
    },
  ]

  // Pure function reproducing ArchivedProjectsPage filter logic
  const filterProjects = (projects: ArchivedProjectItem[], searchQuery: string) => {
    return projects.filter((p) => {
      const q = searchQuery.toLowerCase().trim()
      if (!q) return true
      return (
        p.name.toLowerCase().includes(q) ||
        p.id.toLowerCase().includes(q) ||
        (p.folder_uri && p.folder_uri.toLowerCase().includes(q))
      )
    })
  }

  describe('1. Zero-Project Empty State (archived.length === 0)', () => {
    it('verifies search filter row is conditionally omitted when archived.length === 0', () => {
      // Must use conditional render {archived.length > 0 && (...)}
      assert.ok(
        pageSrc.includes('archived.length > 0 && ('),
        'Search filter row must only render when archived.length > 0'
      )
    })

    it('verifies empty state renders cleanly directly inside google-card without top-bar padding artifacts', () => {
      const googleCardMatch = pageSrc.match(/<div className="google-card" style={{ padding: 0, overflow: 'hidden' }}>([\s\S]*?)<\/div>\s*<\/div>/)
      assert.ok(googleCardMatch, 'google-card must declare padding: 0 and overflow: hidden')

      // When archived is empty, filteredProjects is also empty
      const emptyProjects: ArchivedProjectItem[] = []
      const filtered = filterProjects(emptyProjects, '')
      assert.equal(filtered.length, 0, 'filteredProjects is empty when archived is empty')

      // Empty state branch contains "No archived projects" and guidance text
      assert.ok(pageSrc.includes("'No archived projects'"), 'Default empty state title present')
      assert.ok(
        pageSrc.includes('To archive a project and remove it from your sidebar, click the project options (•••) button in Antigravity and select "Archive".'),
        'Comprehensive empty state guidance present'
      )
    })
  })

  describe('2. Search Filter Behavior & Non-Matching Results (filteredProjects.length === 0)', () => {
    it('filters accurately by project name (case-insensitive)', () => {
      const matches = filterProjects(sampleProjects, 'ALPHA')
      assert.equal(matches.length, 1)
      assert.equal(matches[0].id, 'proj-1')
    })

    it('filters accurately by project id', () => {
      const matches = filterProjects(sampleProjects, 'proj-2')
      assert.equal(matches.length, 1)
      assert.equal(matches[0].name, 'Beta Project [Special+Chars]')
    })

    it('filters accurately by folder_uri substring', () => {
      const matches = filterProjects(sampleProjects, 'workspace/beta')
      assert.equal(matches.length, 1)
      assert.equal(matches[0].id, 'proj-2')
    })

    it('returns all projects when query is empty or whitespace only', () => {
      const emptyMatches = filterProjects(sampleProjects, '')
      assert.equal(emptyMatches.length, 3)

      const whitespaceMatches = filterProjects(sampleProjects, '    ')
      assert.equal(whitespaceMatches.length, 3)
    })

    it('handles special regex characters safely without throwing SyntaxError', () => {
      assert.doesNotThrow(() => {
        const matches = filterProjects(sampleProjects, '[Special+Chars]')
        assert.equal(matches.length, 1)
        assert.equal(matches[0].id, 'proj-2')
      })

      assert.doesNotThrow(() => {
        const matches = filterProjects(sampleProjects, '(?=.*abc).*{2,5}')
        assert.equal(matches.length, 0)
      })
    })

    it('displays distinct filter-specific empty message when query yields 0 results while search bar stays rendered', () => {
      const matches = filterProjects(sampleProjects, 'nonexistent query')
      assert.equal(matches.length, 0)

      // In component: searchQuery ? 'No matching archived projects' : 'No archived projects'
      assert.ok(
        pageSrc.includes("searchQuery ? 'No matching archived projects' : 'No archived projects'"),
        'Dynamic title differentiates between 0 total vs 0 matched'
      )
      assert.ok(
        pageSrc.includes("searchQuery\n                ? 'Try a different keyword or clear your filter query.'"),
        'Dynamic guidance prompts user to clear filter on zero query matches'
      )
    })
  })

  describe('3. Mutation Lifecycles & List Refresh (handleRestore & handleDelete)', () => {
    it('verifies handleRestore triggers loadData to refresh the list', () => {
      const restoreFnMatch = pageSrc.match(/const handleRestore = async \([\s\S]*?^  }/m)
      assert.ok(restoreFnMatch, 'handleRestore function must exist')
      const restoreFn = restoreFnMatch[0]

      assert.ok(restoreFn.includes('setActionLoading(`restore-${project.id}`)'), 'Locks button state during restore')
      assert.ok(restoreFn.includes('await api.restoreProject(project.name || project.id)'), 'Calls API restoreProject with fallback')
      assert.ok(restoreFn.includes('await loadData()'), 'Must re-fetch archived projects on success')
      assert.ok(restoreFn.includes('setActionLoading(null)'), 'Releases lock in finally block')
    })

    it('verifies handleDelete triggers loadData and resets confirmation state', () => {
      const deleteFnMatch = pageSrc.match(/const handleDelete = async \(\) => {[\s\S]*?^  }/m)
      assert.ok(deleteFnMatch, 'handleDelete function must exist')
      const deleteFn = deleteFnMatch[0]

      assert.ok(deleteFn.includes('if (!deleteConfirmProject) return'), 'Guards against null target')
      assert.ok(deleteFn.includes('setActionLoading(`delete-${deleteConfirmProject.id}`)'), 'Locks button state during delete')
      assert.ok(deleteFn.includes('await api.deleteArchivedProject(deleteConfirmProject.name || deleteConfirmProject.id)'), 'Calls API deleteArchivedProject')
      assert.ok(deleteFn.includes('setDeleteConfirmProject(null)'), 'Closes modal on success')
      assert.ok(deleteFn.includes('await loadData()'), 'Must re-fetch archived projects on success')
      assert.ok(deleteFn.includes('setActionLoading(null)'), 'Releases lock in finally block')
    })

    it('verifies action buttons are disabled during active mutations to prevent race conditions', () => {
      const disabledCount = (pageSrc.match(/disabled={actionLoading !== null}/g) || []).length
      assert.ok(disabledCount >= 4, 'Settings, Restore, Delete, and Modal Delete buttons must disable when actionLoading is set')
    })
  })

  describe('4. Delete Confirmation Modal & Event Propagation Safety', () => {
    it('triggers modal state when Delete button in table is clicked', () => {
      assert.ok(
        pageSrc.includes('onClick={() => setDeleteConfirmProject(p)}'),
        'Table row Delete button sets deleteConfirmProject target'
      )
    })

    it('safely handles backdrop dismissal while stopping propagation on modal body clicks', () => {
      const modalBlockMatch = pageSrc.match(/{\/\* Delete Confirmation Modal \*\/}[\s\S]*?<\/div>\s*\)\}/)
      assert.ok(modalBlockMatch, 'Delete confirmation modal block must exist')
      const modalBlock = modalBlockMatch[0]

      // Backdrop dismissal
      assert.ok(
        modalBlock.includes('onClick={() => setDeleteConfirmProject(null)}'),
        'Clicking outer backdrop closes modal'
      )

      // Inner propagation stop
      assert.ok(
        modalBlock.includes('onClick={(e) => e.stopPropagation()}'),
        'Inner modal dialog must stop click propagation'
      )

      // Cancel button
      assert.ok(
        modalBlock.includes('onClick={() => setDeleteConfirmProject(null)}'),
        'Cancel button resets deleteConfirmProject'
      )
    })

    it('confirms permanent deletion via handleDelete with confirmation prompt', () => {
      assert.ok(pageSrc.includes('Confirm Project Deletion'), 'Modal title states deletion intent')
      assert.ok(pageSrc.includes('Permanently Delete'), 'Confirm button label is explicit')
      assert.ok(pageSrc.includes('onClick={handleDelete}'), 'Confirm button triggers handleDelete')
    })
  })

  describe('5. Error Handling & Toast Resilience', () => {
    it('catches loadData failure and renders error banner', () => {
      const loadDataMatch = pageSrc.match(/const loadData = async \(\) => {[\s\S]*?^  }/m)
      assert.ok(loadDataMatch, 'loadData function must exist')
      const loadData = loadDataMatch[0]

      assert.ok(loadData.includes('catch (err: any)'), 'loadData catches exceptions')
      assert.ok(loadData.includes("setStatusMsg({ text: 'Failed to load archived projects: ' + err.message, type: 'error' })"), 'Sets error status banner')
      assert.ok(loadData.includes('setLoading(false)'), 'Ensures loading terminates on failure')
    })

    it('handles restore failure gracefully with error toast', () => {
      assert.ok(
        pageSrc.includes("showToast('Failed to restore project: ' + err.message, 'error')"),
        'Restore failure triggers error toast'
      )
    })

    it('handles delete failure gracefully with error toast', () => {
      assert.ok(
        pageSrc.includes("showToast('Failed to delete project: ' + err.message, 'error')"),
        'Delete failure triggers error toast'
      )
    })
  })
})
