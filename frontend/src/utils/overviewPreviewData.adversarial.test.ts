import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import * as overviewModule from './overviewPreviewData.ts'
import {
  OVERVIEW_PREVIEW_HEADER,
  OVERVIEW_DIVISION_BADGE_LABELS,
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
  })

  describe('2. Anti-Facade Invariants: Synthetic Constants Elimination', () => {
    it('verifies zero synthetic mock fixtures exported from overviewPreviewData', () => {
      assert.strictEqual((overviewModule as any).DEMO_PROJECT_NAME, undefined, 'DEMO_PROJECT_NAME must be eliminated')
      assert.strictEqual((overviewModule as any).SYNTHETIC_SUBAGENTS_COUNT, undefined, 'SYNTHETIC_SUBAGENTS_COUNT must be eliminated')
      assert.strictEqual((overviewModule as any).SYNTHETIC_FILES_COUNT, undefined, 'SYNTHETIC_FILES_COUNT must be eliminated')
      assert.strictEqual((overviewModule as any).SYNTHETIC_ARTIFACTS_COUNT, undefined, 'SYNTHETIC_ARTIFACTS_COUNT must be eliminated')
      assert.strictEqual((overviewModule as any).SYNTHETIC_UPLOADS_COUNT, undefined, 'SYNTHETIC_UPLOADS_COUNT must be eliminated')
      assert.strictEqual((overviewModule as any).SYNTHETIC_OVERVIEW_FIXTURES, undefined, 'SYNTHETIC_OVERVIEW_FIXTURES must be eliminated')
    })

    it('verifies clean badge labels and header', () => {
      assert.strictEqual(OVERVIEW_PREVIEW_HEADER, 'Overview Panel Preview')
      assert.strictEqual(OVERVIEW_DIVISION_BADGE_LABELS.divider_line, 'Divider Lines')
      assert.strictEqual(OVERVIEW_DIVISION_BADGE_LABELS.border_zone, 'Border Zones')
    })
  })

  describe('3. Adversarial Check of AppEnhancementsPage UI Integration', () => {
    const pagePath = path.join(frontendSrcDir, 'pages', 'AppEnhancementsPage.tsx')
    const pageContent = fs.readFileSync(pagePath, 'utf8')

    it('asserts complete elimination of DEMO_PROJECT_NAME facade in AppEnhancementsPage', () => {
      assert.ok(
        !pageContent.includes('DEMO_PROJECT_NAME'),
        'AppEnhancementsPage must not reference DEMO_PROJECT_NAME'
      )
    })

    it('asserts complete elimination of fake overview preview right column', () => {
      assert.ok(
        !pageContent.includes('Interactive Live Preview of Antigravity\'s Overview Panel'),
        'AppEnhancementsPage must not contain fake overview panel preview right column'
      )
      assert.ok(
        !pageContent.includes('SYNTHETIC_'),
        'AppEnhancementsPage must not contain any SYNTHETIC_ fixtures'
      )
    })

    it('asserts Tab 1 project preview binds to real local project state', () => {
      assert.ok(
        pageContent.includes('{activeProjectName}'),
        'Tab 1 project folder item must bind to activeProjectName'
      )
    })
  })
})
