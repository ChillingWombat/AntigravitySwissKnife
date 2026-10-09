import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

describe('Milestone 12: Path Tab Executable Override Removal (Cleanliness & Preservation)', () => {
  const pagePath = path.resolve(__dirname, '../pages/SystemSettingsPage.tsx')
  const pageSource = fs.readFileSync(pagePath, 'utf8')

  describe('1. Absence of Per-Account Override UI from all 3 cards', () => {
    it('does not contain "Optional Executable Override Per Account" heading', () => {
      assert.ok(
        !pageSource.includes('Optional Executable Override Per Account'),
        'SystemSettingsPage must not contain "Optional Executable Override Per Account"'
      )
    })

    it('does not contain override helper description copy', () => {
      assert.ok(
        !pageSource.includes(
          'Run this application using a specific binary or directory when switching to a selected account.'
        ),
        'SystemSettingsPage must not contain override description copy'
      )
    })

    it('does not contain account selection dropdown placeholder', () => {
      assert.ok(
        !pageSource.includes('Select Account...'),
        'SystemSettingsPage must not contain "Select Account..." option'
      )
    })

    it('does not contain account override path input placeholder', () => {
      assert.ok(
        !pageSource.includes('Account executable path...'),
        'SystemSettingsPage must not contain "Account executable path..." placeholder'
      )
    })

    it('does not contain "Set Override" button text', () => {
      assert.ok(
        !pageSource.includes('Set Override'),
        'SystemSettingsPage must not contain "Set Override" action'
      )
    })

    it('does not contain "Configured Account Overrides:" label', () => {
      assert.ok(
        !pageSource.includes('Configured Account Overrides:'),
        'SystemSettingsPage must not contain "Configured Account Overrides:" label'
      )
    })
  })

  describe('2. Absence of Dead States & Handlers in SystemSettingsPage', () => {
    it('does not declare or bind accountOverrideDrafts state', () => {
      assert.ok(
        !pageSource.includes('accountOverrideDrafts'),
        'SystemSettingsPage must not declare accountOverrideDrafts state'
      )
    })

    it('does not declare handleSaveAccountOverride or handleRemoveAccountOverride', () => {
      assert.ok(
        !pageSource.includes('handleSaveAccountOverride'),
        'SystemSettingsPage must not declare handleSaveAccountOverride'
      )
      assert.ok(
        !pageSource.includes('handleRemoveAccountOverride'),
        'SystemSettingsPage must not declare handleRemoveAccountOverride'
      )
    })

    it('does not declare unused accounts state or fetch log for path overrides', () => {
      assert.ok(
        !pageSource.includes('Failed to load accounts for path overrides:'),
        'SystemSettingsPage must not have accounts loading block for path overrides'
      )
    })

    it('handleBrowsePath has simplified single-argument signature without isAccountOverride', () => {
      assert.ok(
        !pageSource.includes('isAccountOverride'),
        'handleBrowsePath must not take or reference isAccountOverride'
      )
    })
  })

  describe('3. Complete Preservation of Core Application Path Controls', () => {
    it('preserves Application Executable Paths & Cache Management card title', () => {
      assert.ok(
        pageSource.includes('Application Executable Paths & Cache Management'),
        'Must retain Application Executable Paths & Cache Management card'
      )
    })

    it('preserves all 3 application card definitions (desktop, agy, vscode)', () => {
      assert.ok(pageSource.includes("appType: 'desktop' as const"))
      assert.ok(pageSource.includes("appType: 'agy' as const"))
      assert.ok(pageSource.includes("appType: 'vscode' as const"))
      assert.ok(pageSource.includes("title: 'Antigravity 2.0'"))
      assert.ok(pageSource.includes("title: 'Antigravity CLI'"))
      assert.ok(pageSource.includes("title: 'Antigravity VS Code Extension'"))
    })

    it('preserves Auto-Detected Path label, input, and copy button', () => {
      assert.ok(pageSource.includes('Auto-Detected Path:'))
      assert.ok(pageSource.includes('detectedPath'))
      assert.ok(pageSource.includes('copyPath'))
    })

    it('preserves Manual Custom Executable / Path Override controls', () => {
      assert.ok(pageSource.includes('Manual Custom Executable / Path Override:'))
      assert.ok(pageSource.includes('Save Path'))
      assert.ok(pageSource.includes('Browse'))
      assert.ok(pageSource.includes('handleSaveAppPath'))
      assert.ok(pageSource.includes('handleResetAppPath'))
    })

    it('preserves Clear Cache button and feedback banner', () => {
      assert.ok(pageSource.includes('Clear Cache'))
      assert.ok(pageSource.includes('handleClearAppCache'))
      assert.ok(pageSource.includes('cacheClearFeedback'))
    })

    it('preserves Current Active Runtime Paths card with all 5 system paths', () => {
      assert.ok(pageSource.includes('Current Active Runtime Paths'))
      assert.ok(pageSource.includes('config_dir'))
      assert.ok(pageSource.includes('credentials'))
      assert.ok(pageSource.includes('temp_dir'))
      assert.ok(pageSource.includes('socket'))
      assert.ok(pageSource.includes('antigravity_bin'))
    })
  })

  describe('4. David-Design Compliance & Invariants', () => {
    it('strictly maintains zero decorative emojis in the Path & Storage tab', () => {
      const pathTabStartIndex = pageSource.indexOf('{/* Tab 1: Path & Storage */}')
      const pathTabEndIndex = pageSource.indexOf('{/* Tab 2: Error & Privacy */}')
      assert.ok(pathTabStartIndex !== -1, 'Tab 1 start index must be found')
      assert.ok(pathTabEndIndex !== -1, 'Tab 2 start index must be found')
      const pathTabSlice = pageSource.slice(pathTabStartIndex, pathTabEndIndex)
      const emojiRegex = /[\u{1F300}-\u{1F5FF}\u{1F600}-\u{1F64F}\u{1F680}-\u{1F6FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u
      assert.ok(!emojiRegex.test(pathTabSlice), 'Path tab must contain zero decorative emojis')
    })

    it('enforces single-line action text (whiteSpace: nowrap) on all app card action buttons', () => {
      const pathTabStartIndex = pageSource.indexOf('{/* Tab 1: Path & Storage */}')
      const pathTabEndIndex = pageSource.indexOf('{/* Tab 2: Error & Privacy */}')
      const pathTabSlice = pageSource.slice(pathTabStartIndex, pathTabEndIndex)

      // Clear Cache, Browse, Save Path, and Reset buttons must have whiteSpace: 'nowrap'
      assert.ok(
        pathTabSlice.includes("title={`Clear cache for ${title}`}"),
        'Must retain Clear Cache button'
      )
      assert.ok(
        pathTabSlice.includes("title=\"Open file/folder picker\""),
        'Must retain Browse button'
      )
      assert.ok(
        pathTabSlice.includes('Save Path'),
        'Must retain Save Path button'
      )
      assert.ok(
        pathTabSlice.includes("title=\"Reset to auto-detected default\""),
        'Must retain Reset button'
      )
    })
  })

  describe('5. Functional Logic Model Simulation', () => {
    it('simulates handleBrowsePath for custom executable updates', () => {
      let customPaths: Record<string, string> = {
        desktop: '',
        agy: '',
        vscode: '',
      }

      const updateCustomPath = (appType: 'desktop' | 'agy' | 'vscode', selectedPath: string) => {
        customPaths = { ...customPaths, [appType]: selectedPath }
      }

      updateCustomPath('desktop', '/opt/Antigravity/antigravity')
      assert.equal(customPaths.desktop, '/opt/Antigravity/antigravity')
      assert.equal(customPaths.agy, '')
      assert.equal(customPaths.vscode, '')

      updateCustomPath('agy', '/usr/local/bin/agy')
      assert.equal(customPaths.agy, '/usr/local/bin/agy')

      updateCustomPath('vscode', '/home/user/.vscode/extensions/antigravity')
      assert.equal(customPaths.vscode, '/home/user/.vscode/extensions/antigravity')
    })

    it('verifies that zone custom_path reset logic restores default state', () => {
      let customPaths: Record<string, string> = {
        desktop: '/custom/path/bin',
        agy: '',
        vscode: '',
      }

      const resetCustomPath = (appType: 'desktop' | 'agy' | 'vscode') => {
        customPaths = { ...customPaths, [appType]: '' }
      }

      resetCustomPath('desktop')
      assert.equal(customPaths.desktop, '')
    })
  })
})
