import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import {
  OVERVIEW_PREVIEW_HEADER,
  OVERVIEW_DIVISION_BADGE_LABELS,
  DEMO_PROJECT_NAME,
  SYNTHETIC_SUBAGENTS_COUNT,
  SYNTHETIC_SUBAGENT,
  SYNTHETIC_FILES_COUNT,
  SYNTHETIC_FILES_TAG,
  SYNTHETIC_VISIBLE_FILES,
  SYNTHETIC_EXPANDED_FILES,
  SYNTHETIC_ARTIFACTS_COUNT,
  SYNTHETIC_ARTIFACT,
  SYNTHETIC_UPLOADS_COUNT,
  SYNTHETIC_VISIBLE_UPLOADS,
  SYNTHETIC_EXPANDED_UPLOADS,
  SYNTHETIC_TASKS_COUNT,
  SYNTHETIC_TERMINALS_COUNT,
  SYNTHETIC_GOALS_COUNT,
  SYNTHETIC_GOALS,
  SYNTHETIC_SKILLS_COUNT,
  SYNTHETIC_SKILLS,
  SYNTHETIC_OVERVIEW_FIXTURES,
} from './overviewPreviewData.ts'

describe('Overview Panel Preview Mock Data & Header Contracts', () => {
  it('enforces clean, shortened header wording without throat-clearing prefix', () => {
    assert.equal(OVERVIEW_PREVIEW_HEADER, 'Overview Panel Preview')
    assert.ok(
      !OVERVIEW_PREVIEW_HEADER.includes('Live Preview:'),
      'Header must not contain redundant "Live Preview:" prefix'
    )
    assert.ok(
      !OVERVIEW_PREVIEW_HEADER.includes('Antigravity'),
      'Header must be concise and avoid redundant app name repetition'
    )
  })

  it('shortens division style badge label and eliminates parenthetical commentary', () => {
    assert.equal(OVERVIEW_DIVISION_BADGE_LABELS.divider_line, 'Divider Lines')
    assert.equal(OVERVIEW_DIVISION_BADGE_LABELS.border_zone, 'Border Zones')

    assert.ok(
      !OVERVIEW_DIVISION_BADGE_LABELS.border_zone.includes('('),
      'Badge label must not contain opening parenthesis'
    )
    assert.ok(
      !OVERVIEW_DIVISION_BADGE_LABELS.border_zone.includes(')'),
      'Badge label must not contain closing parenthesis'
    )
    assert.ok(
      !OVERVIEW_DIVISION_BADGE_LABELS.border_zone.includes('Whiter BG'),
      'Badge label must eliminate "(Whiter BG)" commentary'
    )
  })

  it('verifies synthetic demo project name for preview components', () => {
    assert.equal(DEMO_PROJECT_NAME, 'Demo Project')
    assert.ok(!DEMO_PROJECT_NAME.includes('Antigravity Swiss Knife'))
  })

  it('enforces privacy invariants: zero leaked personal developer data', () => {
    const serialized = JSON.stringify(SYNTHETIC_OVERVIEW_FIXTURES)

    // Developer prompts and slash commands
    assert.ok(!serialized.includes('/teamwork-preview'), 'Must not contain /teamwork-preview')
    assert.ok(!serialized.includes('/wish-coding'), 'Must not contain /wish-coding')
    assert.ok(!serialized.includes('you need more then these 4 tickets'), 'Must not contain raw developer prompt')

    // Internal repository paths
    assert.ok(!serialized.includes('cmd/swiss'), 'Must not contain cmd/swiss')
    assert.ok(!serialized.includes('pkg/enhancements'), 'Must not contain pkg/enhancements')
    assert.ok(!serialized.includes('AccountDetailModal.tsx'), 'Must not contain AccountDetailModal.tsx')

    // Developer morning session timestamps
    assert.ok(!serialized.includes('6:59 AM'), 'Must not contain 6:59 AM')
    assert.ok(!serialized.includes('6:57 AM'), 'Must not contain 6:57 AM')
    assert.ok(!serialized.includes('6:55 AM'), 'Must not contain 6:55 AM')
    assert.ok(!serialized.includes('6:54 AM'), 'Must not contain 6:54 AM')
    assert.ok(!serialized.includes('6:53 AM'), 'Must not contain 6:53 AM')

    // Private skills
    assert.ok(!serialized.includes('antigravity_guide'), 'Must not contain antigravity_guide')
    assert.ok(!serialized.includes('wish-coding'), 'Must not contain wish-coding')
  })

  it('enforces David-Design zero-decorative-emoji invariant across all fixtures', () => {
    const serialized = JSON.stringify(SYNTHETIC_OVERVIEW_FIXTURES)
    // Match common emojis or Unicode emoji presentation
    const emojiRegex = /[\u{1F300}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u
    assert.ok(!emojiRegex.test(serialized), 'Synthetic fixtures must contain zero decorative emojis')
  })

  it('provides realistic full-stack app files with correct counts and file extensions', () => {
    assert.equal(SYNTHETIC_FILES_COUNT, 18)
    assert.equal(SYNTHETIC_FILES_TAG, 'Uncommitted')
    assert.equal(SYNTHETIC_VISIBLE_FILES.length, 5)
    assert.equal(SYNTHETIC_EXPANDED_FILES.length, 3)

    const allFileNames = [
      ...SYNTHETIC_VISIBLE_FILES.map((f) => f.name),
      ...SYNTHETIC_EXPANDED_FILES.map((f) => f.name),
    ]

    assert.ok(allFileNames.includes('DashboardView.tsx'))
    assert.ok(allFileNames.includes('apiClient.ts'))
    assert.ok(allFileNames.includes('auth_service.go'))
    assert.ok(allFileNames.includes('UserSettingsModal.tsx'))
    assert.ok(allFileNames.includes('session_handler.go'))
    assert.ok(allFileNames.includes('models.go'))
    assert.ok(allFileNames.includes('schema.ts'))

    // Verify directories are generic enterprise paths
    const directories = [
      ...SYNTHETIC_VISIBLE_FILES.map((f) => f.directory),
      ...SYNTHETIC_EXPANDED_FILES.map((f) => f.directory),
    ].filter(Boolean)

    for (const dir of directories) {
      assert.ok(!dir?.includes('swiss'), `Directory ${dir} must not mention swiss`)
      assert.ok(!dir?.includes('enhancements'), `Directory ${dir} must not mention enhancements`)
    }
  })

  it('provides realistic design/architecture uploads with 8 total items', () => {
    assert.equal(SYNTHETIC_UPLOADS_COUNT, 8)
    assert.equal(SYNTHETIC_VISIBLE_UPLOADS.length, 3)
    assert.equal(SYNTHETIC_EXPANDED_UPLOADS.length, 2)

    const allUploads = [...SYNTHETIC_VISIBLE_UPLOADS, ...SYNTHETIC_EXPANDED_UPLOADS]
    for (const item of allUploads) {
      assert.ok(item.name.endsWith('.png'), `Upload item ${item.name} should be a png asset`)
      assert.ok(item.timestamp.startsWith('Today 10:') || item.timestamp.startsWith('Today 09:'))
      assert.ok(item.label.includes(item.name))
      assert.ok(item.label.includes(item.timestamp))
    }
  })

  it('provides realistic generic subagent and artifact fixtures', () => {
    assert.equal(SYNTHETIC_SUBAGENTS_COUNT, 1)
    assert.equal(SYNTHETIC_SUBAGENT.title, 'Codebase Architecture Reviewer (2 subagents)')
    assert.equal(SYNTHETIC_SUBAGENT.duration, 'Worked for 12m')

    assert.equal(SYNTHETIC_ARTIFACTS_COUNT, 1)
    assert.equal(SYNTHETIC_ARTIFACT.title, 'Architecture RFC')
  })

  it('provides generic engineering goals and standard development skills', () => {
    assert.equal(SYNTHETIC_GOALS_COUNT, 2)
    assert.equal(SYNTHETIC_GOALS.length, 2)
    assert.equal(
      SYNTHETIC_GOALS[0].text,
      'Implement responsive layout grid for dashboard cards'
    )
    assert.equal(
      SYNTHETIC_GOALS[1].text,
      'Add unit tests for auth token expiration handling'
    )

    assert.equal(SYNTHETIC_SKILLS_COUNT, 2)
    assert.equal(SYNTHETIC_SKILLS.length, 2)
    assert.equal(SYNTHETIC_SKILLS[0].name, 'code-review')
    assert.equal(SYNTHETIC_SKILLS[0].path, '.../skills/code-review')
    assert.equal(SYNTHETIC_SKILLS[1].name, 'tdd')
    assert.equal(SYNTHETIC_SKILLS[1].path, '.../skills/tdd')
  })

  it('verifies background tasks and terminals counts are zero', () => {
    assert.equal(SYNTHETIC_TASKS_COUNT, 0)
    assert.equal(SYNTHETIC_TERMINALS_COUNT, 0)
  })
})
