import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
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

describe('Adversarial & Empirical Privacy Stress Tests (Milestone 9)', () => {
  const frontendSrcDir = path.resolve(import.meta.dirname, '..')

  // Helper to recursively collect all production code files (.ts, .tsx, excluding test files)
  const getProductionFiles = (dir: string): string[] => {
    const entries = fs.readdirSync(dir, { withFileTypes: true })
    const files: string[] = []
    for (const entry of entries) {
      const fullPath = path.join(dir, entry.name)
      if (entry.isDirectory()) {
        files.push(...getProductionFiles(fullPath))
      } else if (
        (entry.name.endsWith('.ts') || entry.name.endsWith('.tsx')) &&
        !entry.name.includes('.test.') &&
        !entry.name.includes('.spec.')
      ) {
        files.push(fullPath)
      }
    }
    return files
  }

  describe('1. Global Production Codebase Privacy Leak Scan', () => {
    const prodFiles = getProductionFiles(frontendSrcDir)

    it('verifies non-empty collection of production files to scan', () => {
      assert.ok(prodFiles.length > 10, 'Expected multiple production files in frontend/src')
    })

    it('scans all production files for user prompts and slash command leaks', () => {
      const forbiddenPrompts = [
        '/teamwork-preview',
        '/wish-coding',
        'you need more then these 4 tickets',
        'add a new section',
      ]

      for (const filePath of prodFiles) {
        const content = fs.readFileSync(filePath, 'utf8')
        for (const forbidden of forbiddenPrompts) {
          assert.ok(
            !content.includes(forbidden),
            `Privacy leak detected in ${path.relative(frontendSrcDir, filePath)}: found forbidden prompt "${forbidden}"`
          )
        }
      }
    })

    it('scans all production files for private internal skill names', () => {
      const privateSkills = ['antigravity_guide', 'wish-coding']

      for (const filePath of prodFiles) {
        const content = fs.readFileSync(filePath, 'utf8')
        for (const skill of privateSkills) {
          assert.ok(
            !content.includes(skill),
            `Privacy leak detected in ${path.relative(frontendSrcDir, filePath)}: found internal skill "${skill}"`
          )
        }
      }
    })

    it('scans all production files for personal filesystem directories and developer paths', () => {
      const personalPaths = ['/mnt/Data/Projects', '/home/david']

      for (const filePath of prodFiles) {
        const content = fs.readFileSync(filePath, 'utf8')
        for (const p of personalPaths) {
          assert.ok(
            !content.includes(p),
            `Privacy leak detected in ${path.relative(frontendSrcDir, filePath)}: found personal path "${p}"`
          )
        }
      }
    })

    it('scans all production files for morning developer work session timestamps in mock data', () => {
      const morningTimestamps = ['6:59 AM', '6:57 AM', '6:55 AM', '6:54 AM', '6:53 AM']

      for (const filePath of prodFiles) {
        const content = fs.readFileSync(filePath, 'utf8')
        for (const ts of morningTimestamps) {
          assert.ok(
            !content.includes(ts),
            `Privacy leak detected in ${path.relative(frontendSrcDir, filePath)}: found morning session timestamp "${ts}"`
          )
        }
      }
    })
  })

  describe('2. Rigorous Invariant Assertions on Synthetic Mock Data Fixtures', () => {
    it('verifies header string satisfies exact contract and length bounds', () => {
      assert.strictEqual(OVERVIEW_PREVIEW_HEADER, 'Overview Panel Preview')
      assert.ok(OVERVIEW_PREVIEW_HEADER.length <= 30)
      assert.ok(!OVERVIEW_PREVIEW_HEADER.toLowerCase().includes('live preview'))
      assert.ok(!OVERVIEW_PREVIEW_HEADER.toLowerCase().includes('antigravity swiss'))
    })

    it('verifies badge labels are concise with zero explanatory parentheses', () => {
      assert.deepStrictEqual(OVERVIEW_DIVISION_BADGE_LABELS, {
        divider_line: 'Divider Lines',
        border_zone: 'Border Zones',
      })
      for (const label of Object.values(OVERVIEW_DIVISION_BADGE_LABELS)) {
        assert.ok(!label.includes('('), 'Label must not contain opening parenthesis')
        assert.ok(!label.includes(')'), 'Label must not contain closing parenthesis')
        assert.ok(!label.toLowerCase().includes('whiter'), 'Label must not contain "whiter"')
      }
    })

    it('verifies demo project name is generic', () => {
      assert.strictEqual(DEMO_PROJECT_NAME, 'Demo Project')
      assert.ok(!DEMO_PROJECT_NAME.includes('Antigravity'))
      assert.ok(!DEMO_PROJECT_NAME.includes('Swiss'))
    })

    it('validates synthetic files contain only generic enterprise paths', () => {
      assert.strictEqual(SYNTHETIC_FILES_COUNT, 18)
      assert.strictEqual(SYNTHETIC_FILES_TAG, 'Uncommitted')
      assert.strictEqual(SYNTHETIC_VISIBLE_FILES.length, 5)
      assert.strictEqual(SYNTHETIC_EXPANDED_FILES.length, 3)

      const allFiles = [...SYNTHETIC_VISIBLE_FILES, ...SYNTHETIC_EXPANDED_FILES]
      const fileNames = new Set<string>()

      for (const file of allFiles) {
        assert.ok(!file.name.includes('/'), `File name "${file.name}" must not contain slash`)
        assert.ok(!file.name.toLowerCase().includes('swiss'), `File name "${file.name}" must not contain swiss`)
        assert.ok(!file.name.toLowerCase().includes('enhancement'), `File name "${file.name}" must not contain enhancement`)
        if (file.directory) {
          assert.ok(!file.directory.startsWith('/'), `Directory "${file.directory}" must be relative`)
          assert.ok(!file.directory.toLowerCase().includes('swiss'), `Directory "${file.directory}" must not contain swiss`)
          assert.ok(!file.directory.toLowerCase().includes('enhancement'), `Directory "${file.directory}" must not contain enhancement`)
        }
        assert.ok(!fileNames.has(file.name), `Duplicate file name detected: ${file.name}`)
        fileNames.add(file.name)
      }
    })

    it('validates synthetic uploads timestamps and formats', () => {
      assert.strictEqual(SYNTHETIC_UPLOADS_COUNT, 8)
      assert.strictEqual(SYNTHETIC_VISIBLE_UPLOADS.length, 3)
      assert.strictEqual(SYNTHETIC_EXPANDED_UPLOADS.length, 2)

      const allUploads = [...SYNTHETIC_VISIBLE_UPLOADS, ...SYNTHETIC_EXPANDED_UPLOADS]
      for (const upload of allUploads) {
        assert.ok(upload.name.endsWith('.png'), `Upload "${upload.name}" must be a PNG asset`)
        assert.ok(
          /^Today (09|10):\d{2} AM$/.test(upload.timestamp),
          `Upload timestamp "${upload.timestamp}" must be a generic morning/midday mock timestamp`
        )
        assert.strictEqual(upload.label, `${upload.name} (${upload.timestamp})`)
      }
    })

    it('validates synthetic subagents and artifacts are non-personal', () => {
      assert.strictEqual(SYNTHETIC_SUBAGENTS_COUNT, 1)
      assert.strictEqual(SYNTHETIC_SUBAGENT.title, 'Codebase Architecture Reviewer (2 subagents)')
      assert.strictEqual(SYNTHETIC_SUBAGENT.duration, 'Worked for 12m')

      assert.strictEqual(SYNTHETIC_ARTIFACTS_COUNT, 1)
      assert.strictEqual(SYNTHETIC_ARTIFACT.title, 'Architecture RFC')
    })

    it('validates synthetic goals and skills are professional and generic', () => {
      assert.strictEqual(SYNTHETIC_GOALS_COUNT, 2)
      assert.strictEqual(SYNTHETIC_GOALS.length, 2)
      for (const goal of SYNTHETIC_GOALS) {
        assert.ok(goal.text.length > 10, 'Goal description must be descriptive')
        assert.ok(!goal.text.includes('/'), 'Goal must not contain slash commands')
        assert.ok(!goal.text.toLowerCase().includes('ticket'), 'Goal must not mention tickets')
      }

      assert.strictEqual(SYNTHETIC_SKILLS_COUNT, 2)
      assert.strictEqual(SYNTHETIC_SKILLS.length, 2)
      const skillNames = SYNTHETIC_SKILLS.map((s) => s.name)
      assert.deepStrictEqual(skillNames, ['code-review', 'tdd'])
      for (const skill of SYNTHETIC_SKILLS) {
        assert.ok(skill.path.startsWith('.../skills/'), `Skill path must use standard mask: ${skill.path}`)
      }
    })

    it('validates background tasks and terminals counts are zero', () => {
      assert.strictEqual(SYNTHETIC_TASKS_COUNT, 0)
      assert.strictEqual(SYNTHETIC_TERMINALS_COUNT, 0)
    })

    it('enforces David-Design zero-decorative-emoji invariant across all fixtures', () => {
      const serialized = JSON.stringify(SYNTHETIC_OVERVIEW_FIXTURES)
      const emojiRegex = /[\u{1F300}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u
      assert.ok(!emojiRegex.test(serialized), 'Synthetic fixtures must contain zero decorative emojis')
    })
  })

  describe('3. Adversarial Check of AppEnhancementsPage UI Integration', () => {
    const pagePath = path.join(frontendSrcDir, 'pages', 'AppEnhancementsPage.tsx')
    const pageContent = fs.readFileSync(pagePath, 'utf8')

    it('asserts 4px grid compliance on header container padding (paddingBottom: 8px)', () => {
      assert.ok(
        pageContent.includes("paddingBottom: '8px'"),
        'Header container must use 8px bottom padding (2 * 4px)'
      )
      assert.ok(
        !pageContent.includes("paddingBottom: '10px'"),
        'Header container must not use 10px bottom padding (non-4px grid)'
      )
    })

    it('asserts whiteSpace nowrap on preview header and badge', () => {
      assert.ok(
        pageContent.includes('{OVERVIEW_PREVIEW_HEADER}'),
        'Page must bind to OVERVIEW_PREVIEW_HEADER'
      )
      assert.ok(
        pageContent.includes('{OVERVIEW_DIVISION_BADGE_LABELS[op.division_style]}'),
        'Page must bind to OVERVIEW_DIVISION_BADGE_LABELS'
      )
      // Check that both header and badge use nowrap
      const headerSnippet = pageContent.slice(
        pageContent.indexOf('{OVERVIEW_PREVIEW_HEADER}') - 150,
        pageContent.indexOf('{OVERVIEW_PREVIEW_HEADER}') + 100
      )
      assert.ok(
        headerSnippet.includes("whiteSpace: 'nowrap'"),
        'Overview preview header text must specify whiteSpace: nowrap'
      )

      const badgeSnippet = pageContent.slice(
        pageContent.indexOf('{OVERVIEW_DIVISION_BADGE_LABELS[op.division_style]}') - 250,
        pageContent.indexOf('{OVERVIEW_DIVISION_BADGE_LABELS[op.division_style]}') + 100
      )
      assert.ok(
        badgeSnippet.includes("whiteSpace: 'nowrap'"),
        'Overview preview badge must specify whiteSpace: nowrap'
      )
    })

    it('asserts Tab 1 project preview displays DEMO_PROJECT_NAME', () => {
      assert.ok(
        pageContent.includes('{DEMO_PROJECT_NAME}'),
        'Tab 1 project folder item must render {DEMO_PROJECT_NAME}'
      )
    })
  })
})
