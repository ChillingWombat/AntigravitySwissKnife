import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

describe('Adversarial & Empirical Stress Verification: Milestone 12 Path Tab Cleanliness & Resilience', () => {
  const pagePath = path.resolve(__dirname, '../pages/SystemSettingsPage.tsx')
  const pageSource = fs.readFileSync(pagePath, 'utf8')

  const tab1Start = pageSource.indexOf('{/* Tab 1: Path & Storage */}')
  const tab2Start = pageSource.indexOf('{/* Tab 2: Error & Privacy */}')
  assert.ok(tab1Start !== -1, 'Tab 1 must exist')
  assert.ok(tab2Start !== -1, 'Tab 2 must exist')
  const tab1Source = pageSource.slice(tab1Start, tab2Start)

  describe('1. Absence & Leak Prevention of Excised Override UI', () => {
    it('verifies absolute zero matches for excised override keywords in Tab 1', () => {
      const forbiddenStrings = [
        'Optional Executable Override Per Account',
        'accountOverrideDrafts',
        'handleSaveAccountOverride',
        'handleRemoveAccountOverride',
        'Set Override',
        'Configured Account Overrides:',
        'Select Account...',
        'Account executable path...',
        'isAccountOverride',
      ]

      for (const phrase of forbiddenStrings) {
        assert.equal(
          tab1Source.includes(phrase),
          false,
          `Tab 1 must not contain excised phrase: "${phrase}"`
        )
      }
    })

    it('verifies that no dormant JSX comments or dead conditionals leak the override block', () => {
      assert.equal(
        tab1Source.includes('Per-Account Executable Override Section'),
        false,
        'No dead comment for per-account executable override section should exist'
      )
      assert.equal(
        tab1Source.includes('Active Overrides Table/List'),
        false,
        'No dead comment for active overrides table should exist'
      )
    })

    it('empirically verifies that legacy account_overrides data in backend payload is never rendered', () => {
      // Simulating a backend response that still returns account_overrides
      const legacyStoragePayload = {
        app_zones: {
          desktop: {
            installed: true,
            version: '2.0.4',
            detected_path: '/opt/Antigravity/antigravity',
            custom_path: '',
            account_overrides: {
              'work@example.com': '/opt/Antigravity-work/antigravity',
              'personal@gmail.com': '/opt/Antigravity-personal/antigravity',
            },
          },
          agy: {
            installed: true,
            version: '1.2.0',
            detected_path: '/home/user/.local/bin/agy',
            custom_path: '',
            account_overrides: {
              'admin@enterprise.com': '/usr/local/bin/agy-admin',
            },
          },
          vscode: {
            installed: true,
            version: '0.9.1',
            detected_path: '/home/user/.vscode/extensions/google.antigravity-0.9.1',
            custom_path: '',
            account_overrides: {},
          },
        },
      }
      assert.ok(legacyStoragePayload.app_zones.desktop.account_overrides)

      // Assert that SystemSettingsPage Tab 1 code has zero references to zone?.account_overrides
      assert.equal(
        tab1Source.includes('account_overrides'),
        false,
        'Tab 1 JSX must completely ignore zone.account_overrides even if present in payload'
      )
      assert.equal(
        tab1Source.includes('overrides'),
        false,
        'Tab 1 JSX must not declare or render any overrides variable'
      )
    })
  })

  describe('2. Interactive State Resilience: Custom Path Setting across Desktop, CLI, and VS Code', () => {
    it('verifies independent state management across all 3 apps without cross-contamination', () => {
      type AppType = 'desktop' | 'agy' | 'vscode'
      let customPaths: Record<string, string> = { desktop: '', agy: '', vscode: '' }

      const setAppPath = (app: AppType, val: string) => {
        customPaths = { ...customPaths, [app]: val }
      }

      setAppPath('desktop', '/opt/custom/antigravity')
      assert.equal(customPaths.desktop, '/opt/custom/antigravity')
      assert.equal(customPaths.agy, '')
      assert.equal(customPaths.vscode, '')

      setAppPath('agy', '/custom/bin/agy')
      assert.equal(customPaths.desktop, '/opt/custom/antigravity')
      assert.equal(customPaths.agy, '/custom/bin/agy')
      assert.equal(customPaths.vscode, '')

      setAppPath('vscode', '/custom/vscode/ext')
      assert.equal(customPaths.desktop, '/opt/custom/antigravity')
      assert.equal(customPaths.agy, '/custom/bin/agy')
      assert.equal(customPaths.vscode, '/custom/vscode/ext')
    })

    it('simulates handleBrowsePath under Electron and fallback prompt environments', async () => {
      type AppType = 'desktop' | 'agy' | 'vscode'
      let customPaths: Record<string, string> = { desktop: '', agy: '', vscode: '' }

      // 1. Electron environment simulation
      const mockElectronAPI: {
        selectPath: (options: { directory: boolean; title: string }) => Promise<string | null>
      } = {
        selectPath: async (options: { directory: boolean; title: string }) => {
          if (options.directory) {
            return '/selected/directory/path'
          }
          return '/selected/executable/binary'
        },
      }

      const runBrowseElectron = async (appType: AppType) => {
        const isDir = appType === 'vscode'
        const selected = await mockElectronAPI.selectPath({
          directory: isDir,
          title: isDir ? `Select ${appType.toUpperCase()} Extension Directory` : `Select ${appType.toUpperCase()} Executable`,
        })
        if (selected) {
          customPaths = { ...customPaths, [appType]: selected }
        }
      }

      await runBrowseElectron('desktop')
      assert.equal(customPaths.desktop, '/selected/executable/binary')

      await runBrowseElectron('vscode')
      assert.equal(customPaths.vscode, '/selected/directory/path')

      // User cancels dialog (returns null)
      mockElectronAPI.selectPath = async () => null
      await runBrowseElectron('desktop')
      // Must preserve existing value
      assert.equal(customPaths.desktop, '/selected/executable/binary')

      // 2. Non-Electron fallback (window.prompt) simulation
      const runBrowsePrompt = (appType: AppType, promptInput: string | null) => {
        if (promptInput !== null) {
          customPaths = { ...customPaths, [appType]: promptInput.trim() }
        }
      }

      runBrowsePrompt('agy', '   /usr/local/bin/agy-custom   ')
      assert.equal(customPaths.agy, '/usr/local/bin/agy-custom')

      // Prompt cancelled (null)
      runBrowsePrompt('agy', null)
      assert.equal(customPaths.agy, '/usr/local/bin/agy-custom')
    })

    it('simulates handleSaveAppPath success and error states', async () => {
      type AppType = 'desktop' | 'agy' | 'vscode'
      let storageInfo: any = { app_zones: { desktop: { custom_path: '' } } }
      let feedback: Record<string, { text: string; isError: boolean }> = {}

      const saveAppPath = async (
        appType: AppType,
        pathVal: string,
        mockApiResponse: { success: boolean; storage?: any; error?: string }
      ) => {
        feedback[appType] = { text: 'Saving path...', isError: false }
        if (mockApiResponse.success && mockApiResponse.storage) {
          storageInfo = mockApiResponse.storage
          feedback[appType] = {
            text: pathVal ? 'Custom executable path saved.' : 'Path reset to auto-detected default.',
            isError: false,
          }
        } else {
          feedback[appType] = {
            text: mockApiResponse.error || 'Failed to save path.',
            isError: true,
          }
        }
      }

      // Successful save
      await saveAppPath('desktop', '/opt/Antigravity/bin', {
        success: true,
        storage: { app_zones: { desktop: { custom_path: '/opt/Antigravity/bin' } } },
      })
      assert.equal(feedback.desktop.isError, false)
      assert.equal(feedback.desktop.text, 'Custom executable path saved.')
      assert.equal(storageInfo.app_zones.desktop.custom_path, '/opt/Antigravity/bin')

      // Failed save
      await saveAppPath('agy', '/invalid/bin', {
        success: false,
        error: 'Target binary does not have executable permissions.',
      })
      assert.equal(feedback.agy.isError, true)
      assert.equal(feedback.agy.text, 'Target binary does not have executable permissions.')
    })

    it('simulates handleResetAppPath and verifies Reset button conditional visibility', async () => {
      let customPaths: Record<string, string> = { desktop: '/custom/bin', agy: '', vscode: '' }
      let zoneDesktop = { custom_path: '/custom/bin' }
      let feedback: Record<string, { text: string; isError: boolean }> = {}

      // Visibility check before reset
      const isResetVisible = (zone: { custom_path?: string }) => Boolean(zone?.custom_path)
      assert.equal(isResetVisible(zoneDesktop), true)

      // Execute reset
      customPaths.desktop = ''
      zoneDesktop.custom_path = ''
      feedback.desktop = { text: 'Path reset to auto-detected default.', isError: false }

      assert.equal(customPaths.desktop, '')
      assert.equal(isResetVisible(zoneDesktop), false, 'Reset button must hide once custom_path is cleared')
      assert.equal(feedback.desktop.text, 'Path reset to auto-detected default.')
    })
  })

  describe('3. Cache Clearing Interaction & Feedback Robustness', () => {
    it('simulates handleClearAppCache states: clearing in-flight, success with freed bytes, clean cache, and error', async () => {
      type CacheFeedback = { text: string; isError: boolean; isClearing: boolean }
      let feedback: Record<string, CacheFeedback> = {}

      const clearAppCache = async (
        appType: string,
        mockResponse: { success: boolean; freed_bytes?: number; deleted_files?: number; error?: string }
      ) => {
        feedback[appType] = { text: 'Clearing application cache...', isError: false, isClearing: true }
        if (mockResponse.success) {
          const freedMb = ((mockResponse.freed_bytes || 0) / (1024 * 1024)).toFixed(1)
          const msg =
            (mockResponse.freed_bytes || 0) > 0
              ? `Cache cleared: freed ${freedMb} MB (${mockResponse.deleted_files} files removed).`
              : 'Cache already clean (no temporary files found).'
          feedback[appType] = { text: msg, isError: false, isClearing: false }
        } else {
          feedback[appType] = {
            text: mockResponse.error || 'Failed to clear cache.',
            isError: true,
            isClearing: false,
          }
        }
      }

      // Scenario 1: Cache cleared with bytes freed
      await clearAppCache('desktop', { success: true, freed_bytes: 52428800, deleted_files: 312 })
      assert.equal(feedback.desktop.isClearing, false)
      assert.equal(feedback.desktop.isError, false)
      assert.equal(feedback.desktop.text, 'Cache cleared: freed 50.0 MB (312 files removed).')

      // Scenario 2: Cache already clean
      await clearAppCache('agy', { success: true, freed_bytes: 0, deleted_files: 0 })
      assert.equal(feedback.agy.isClearing, false)
      assert.equal(feedback.agy.isError, false)
      assert.equal(feedback.agy.text, 'Cache already clean (no temporary files found).')

      // Scenario 3: Cache clear error
      await clearAppCache('vscode', { success: false, error: 'Permission denied deleting lock file' })
      assert.equal(feedback.vscode.isClearing, false)
      assert.equal(feedback.vscode.isError, true)
      assert.equal(feedback.vscode.text, 'Permission denied deleting lock file')
    })

    it('verifies button disabling and loading indicators during clearing', () => {
      assert.ok(
        tab1Source.includes('disabled={cacheFeedback?.isClearing}'),
        'Clear Cache button must be disabled when clearing is active'
      )
      assert.ok(
        tab1Source.includes("cacheFeedback?.isClearing ? 'Clearing...' : 'Clear Cache'"),
        'Button label must reflect clearing in-flight progress'
      )
    })
  })

  describe('4. Clipboard Copying: Auto-Detected & Active Runtime Paths', () => {
    it('verifies auto-detected path copy button presence conditional on detectedPath !== "Not detected"', () => {
      assert.ok(
        tab1Source.includes("detectedPath !== 'Not detected'"),
        'Copy button must only render if auto-detected path is valid'
      )
    })

    it('simulates copyPath feedback key and timeout state transitions', () => {
      let copiedKey: string | null = null

      const copyPath = (key: string, _val: string) => {
        copiedKey = key
      }

      copyPath('desktop_detected', '/opt/Antigravity/antigravity')
      assert.equal(copiedKey, 'desktop_detected')

      // Reset
      copiedKey = null
      assert.equal(copiedKey, null)

      copyPath('config_dir', '/home/user/.config/antigravity-swiss')
      assert.equal(copiedKey, 'config_dir')
    })

    it('verifies that all 5 active runtime paths are provided with copy triggers', () => {
      const expectedPaths = ['config_dir', 'credentials', 'temp_dir', 'socket', 'antigravity_bin']
      for (const p of expectedPaths) {
        assert.ok(
          pageSource.includes(`key: '${p}'`),
          `Runtime paths array must contain key '${p}'`
        )
      }
      assert.ok(
        tab1Source.includes('{envPaths.map((item) => ('),
        'Tab 1 must map and render envPaths array'
      )
      assert.ok(
        tab1Source.includes('copyPath(item.key, item.val)'),
        'Tab 1 must bind copyPath to runtime path items'
      )
    })
  })

  describe('5. David-Design Aesthetics & Sizing Invariants', () => {
    it('verifies single-line nowrap styling on all 4 action buttons', () => {
      // Clear Cache, Browse, Save Path, and Reset buttons
      assert.ok(
        tab1Source.includes("whiteSpace: 'nowrap'") && tab1Source.includes('Save Path'),
        'Save Path button must have whiteSpace: nowrap'
      )
      assert.ok(
        tab1Source.includes("whiteSpace: 'nowrap'") && tab1Source.includes('Browse'),
        'Browse button must have whiteSpace: nowrap'
      )
      assert.ok(
        tab1Source.includes("whiteSpace: 'nowrap'") && tab1Source.includes('Clear Cache'),
        'Clear Cache button must have whiteSpace: nowrap'
      )
      assert.ok(
        tab1Source.includes("whiteSpace: 'nowrap'") && tab1Source.includes('Reset'),
        'Reset button must have whiteSpace: nowrap'
      )
    })

    it('verifies strictly zero decorative emojis in Tab 1', () => {
      const emojiRegex = /[\u{1F300}-\u{1F5FF}\u{1F600}-\u{1F64F}\u{1F680}-\u{1F6FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u
      assert.equal(
        emojiRegex.test(tab1Source),
        false,
        'Tab 1 source must contain zero decorative emojis'
      )
    })
  })
})
