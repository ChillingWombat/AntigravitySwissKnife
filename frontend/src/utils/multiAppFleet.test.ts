import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import type {
  InstalledAppsStatus,
  MultiAppSyncMode,
  TargetApp,
} from '../types.ts'

// --- Multi-App Fleet Logic Models (mirroring QuotaDashboardPage & SwitcherSettingsPage) ---

interface InstalledAppItem {
  key: 'desktop' | 'agy' | 'vscode'
  target: TargetApp
  label: string
  shortLabel: string
}

function resolveInstalledAppsList(installed?: InstalledAppsStatus): InstalledAppItem[] {
  const current = installed || { desktop: true, agy: true, vscode: true }
  const list: InstalledAppItem[] = []
  if (current.desktop) {
    list.push({
      key: 'desktop',
      target: 'desktop',
      label: 'AGY 2.0',
      shortLabel: '2.0',
    })
  }
  if (current.agy) {
    list.push({
      key: 'agy',
      target: 'agy',
      label: 'AGY CLI',
      shortLabel: 'CLI',
    })
  }
  if (current.vscode) {
    list.push({
      key: 'vscode',
      target: 'vscode',
      label: 'AGY EXT',
      shortLabel: 'EXT',
    })
  }
  if (list.length === 0) {
    list.push({
      key: 'desktop',
      target: 'desktop',
      label: 'AGY 2.0',
      shortLabel: '2.0',
    })
  }
  return list
}

function getInstalledCount(installed?: InstalledAppsStatus): number {
  if (!installed) return 0
  return [installed.desktop, installed.agy, installed.vscode].filter(Boolean).length
}

function isSingleAppOrLessRule(installed?: InstalledAppsStatus): boolean {
  return getInstalledCount(installed) <= 1
}

function resolveActiveBadgeText(params: {
  isIndividualMode: boolean
  acc: { email: string; active_apps?: Array<'desktop' | 'agy' | 'vscode'> }
  installedAppsList: InstalledAppItem[]
  activeAppMap: Record<string, string>
  activeAccount: string
}): { isRowActive: boolean; activeBadgeText: string; usingApps: InstalledAppItem[] } {
  const { isIndividualMode, acc, installedAppsList, activeAppMap, activeAccount } = params
  const accEmailLower = acc.email.toLowerCase()

  const usingApps = isIndividualMode
    ? installedAppsList.filter((app) => {
        if (activeAppMap[app.key]) {
          return activeAppMap[app.key].toLowerCase() === accEmailLower
        }
        if (acc.active_apps && acc.active_apps.length > 0) {
          return acc.active_apps.includes(app.key)
        }
        return accEmailLower === activeAccount.toLowerCase()
      })
    : []

  const isRowActive = isIndividualMode
    ? usingApps.length > 0
    : (activeAccount ? accEmailLower === activeAccount.toLowerCase() : false)

  let activeBadgeText = 'Active'
  if (isIndividualMode && usingApps.length > 0) {
    if (usingApps.length === installedAppsList.length) {
      activeBadgeText = 'All'
    } else if (usingApps.length === 1) {
      activeBadgeText = usingApps[0].label
    } else if (usingApps.length === 2) {
      activeBadgeText = `${usingApps[0].shortLabel} & ${usingApps[1].shortLabel}`
    } else {
      activeBadgeText = usingApps.map((u) => u.shortLabel).join(' & ')
    }
  }

  return { isRowActive, activeBadgeText, usingApps }
}

function getSwitchDropdownOptions(installedAppsList: InstalledAppItem[]): Array<{ label: string; target: TargetApp }> {
  return [
    { label: 'All', target: 'all' },
    ...installedAppsList.map((app) => ({ label: app.label, target: app.target })),
  ]
}

describe('Milestone 5 Frontend Contracts: Multi-App Sync & Account Fleet', () => {
  describe('1. Single-App Disable Rule in SwitcherSettingsPage', () => {
    it('disables individual mode when 0 apps are installed', () => {
      const installed: InstalledAppsStatus = { desktop: false, agy: false, vscode: false }
      const count = getInstalledCount(installed)
      assert.equal(count, 0)
      assert.equal(isSingleAppOrLessRule(installed), true)

      // Expected tooltip when disabled
      const tooltip = `Disabled: At least 2 Antigravity applications must be installed to use individual mode (currently detected: ${count}).`
      assert.match(tooltip, /currently detected: 0/)

      // Expected error message banner
      const banner = `Requires at least 2 installed Antigravity apps (${count} detected).`
      assert.match(banner, /0 detected/)
    })

    it('disables individual mode when exactly 1 app is installed (desktop only)', () => {
      const installed: InstalledAppsStatus = { desktop: true, agy: false, vscode: false }
      const count = getInstalledCount(installed)
      assert.equal(count, 1)
      assert.equal(isSingleAppOrLessRule(installed), true)

      const tooltip = `Disabled: At least 2 Antigravity applications must be installed to use individual mode (currently detected: ${count}).`
      assert.match(tooltip, /currently detected: 1/)
      const banner = `Requires at least 2 installed Antigravity apps (${count} detected).`
      assert.match(banner, /1 detected/)
    })

    it('disables individual mode when exactly 1 app is installed (agy CLI only)', () => {
      const installed: InstalledAppsStatus = { desktop: false, agy: true, vscode: false }
      const count = getInstalledCount(installed)
      assert.equal(count, 1)
      assert.equal(isSingleAppOrLessRule(installed), true)
    })

    it('disables individual mode when exactly 1 app is installed (vscode extension only)', () => {
      const installed: InstalledAppsStatus = { desktop: false, agy: false, vscode: true }
      const count = getInstalledCount(installed)
      assert.equal(count, 1)
      assert.equal(isSingleAppOrLessRule(installed), true)
    })

    it('enables toggle between shared and individual mode when 2 apps are installed', () => {
      const installed: InstalledAppsStatus = { desktop: true, agy: true, vscode: false }
      const count = getInstalledCount(installed)
      assert.equal(count, 2)
      assert.equal(isSingleAppOrLessRule(installed), false)
    })

    it('enables toggle between shared and individual mode when all 3 apps are installed', () => {
      const installed: InstalledAppsStatus = { desktop: true, agy: true, vscode: true }
      const count = getInstalledCount(installed)
      assert.equal(count, 3)
      assert.equal(isSingleAppOrLessRule(installed), false)
    })

    it('payload forcibly falls back to shared mode if single app or less', () => {
      const installed: InstalledAppsStatus = { desktop: true, agy: false, vscode: false }
      const userSelectedMode: MultiAppSyncMode = 'individual'
      const effectiveMode: MultiAppSyncMode = isSingleAppOrLessRule(installed) ? 'shared' : userSelectedMode
      assert.equal(effectiveMode, 'shared')
    })
  })

  describe('2. QuotaDashboardPage Switch Dropdown Options Filtering', () => {
    it('provides All, AGY 2.0, AGY CLI, AGY EXT when all 3 apps are installed', () => {
      const installed: InstalledAppsStatus = { desktop: true, agy: true, vscode: true }
      const list = resolveInstalledAppsList(installed)
      const options = getSwitchDropdownOptions(list)

      assert.deepEqual(
        options.map((o) => o.label),
        ['All', 'AGY 2.0', 'AGY CLI', 'AGY EXT']
      )
      assert.deepEqual(
        options.map((o) => o.target),
        ['all', 'desktop', 'agy', 'vscode']
      )
    })

    it('restricts dropdown options when vscode extension is not installed', () => {
      const installed: InstalledAppsStatus = { desktop: true, agy: true, vscode: false }
      const list = resolveInstalledAppsList(installed)
      const options = getSwitchDropdownOptions(list)

      assert.deepEqual(
        options.map((o) => o.label),
        ['All', 'AGY 2.0', 'AGY CLI']
      )
      assert.deepEqual(
        options.map((o) => o.target),
        ['all', 'desktop', 'agy']
      )
      assert.equal(options.some((o) => o.label === 'AGY EXT'), false)
    })

    it('restricts dropdown options when agy CLI is not installed', () => {
      const installed: InstalledAppsStatus = { desktop: true, agy: false, vscode: true }
      const list = resolveInstalledAppsList(installed)
      const options = getSwitchDropdownOptions(list)

      assert.deepEqual(
        options.map((o) => o.label),
        ['All', 'AGY 2.0', 'AGY EXT']
      )
      assert.deepEqual(
        options.map((o) => o.target),
        ['all', 'desktop', 'vscode']
      )
      assert.equal(options.some((o) => o.label === 'AGY CLI'), false)
    })

    it('restricts dropdown options when desktop IDE is not installed (CLI + EXT)', () => {
      const installed: InstalledAppsStatus = { desktop: false, agy: true, vscode: true }
      const list = resolveInstalledAppsList(installed)
      const options = getSwitchDropdownOptions(list)

      assert.deepEqual(
        options.map((o) => o.label),
        ['All', 'AGY CLI', 'AGY EXT']
      )
      assert.deepEqual(
        options.map((o) => o.target),
        ['all', 'agy', 'vscode']
      )
      assert.equal(options.some((o) => o.label === 'AGY 2.0'), false)
    })
  })

  describe('3. Dynamic Active Badges Formatting Matrix', () => {
    const installedAll: InstalledAppsStatus = { desktop: true, agy: true, vscode: true }
    const listAll = resolveInstalledAppsList(installedAll)

    it('displays "Active" in shared mode regardless of active account', () => {
      const result = resolveActiveBadgeText({
        isIndividualMode: false,
        acc: { email: 'user@gmail.com' },
        installedAppsList: listAll,
        activeAppMap: {},
        activeAccount: 'user@gmail.com',
      })
      assert.equal(result.isRowActive, true)
      assert.equal(result.activeBadgeText, 'Active')
    })

    it('displays "All" when account is active on all 3 installed apps', () => {
      const result = resolveActiveBadgeText({
        isIndividualMode: true,
        acc: { email: 'user@gmail.com' },
        installedAppsList: listAll,
        activeAppMap: {
          desktop: 'user@gmail.com',
          agy: 'user@gmail.com',
          vscode: 'user@gmail.com',
        },
        activeAccount: 'user@gmail.com',
      })
      assert.equal(result.isRowActive, true)
      assert.equal(result.activeBadgeText, 'All')
      assert.equal(result.usingApps.length, 3)
    })

    it('displays "AGY 2.0" when active only on Desktop', () => {
      const result = resolveActiveBadgeText({
        isIndividualMode: true,
        acc: { email: 'desktop.user@gmail.com' },
        installedAppsList: listAll,
        activeAppMap: {
          desktop: 'desktop.user@gmail.com',
          agy: 'other@gmail.com',
          vscode: 'another@gmail.com',
        },
        activeAccount: 'desktop.user@gmail.com',
      })
      assert.equal(result.isRowActive, true)
      assert.equal(result.activeBadgeText, 'AGY 2.0')
    })

    it('displays "AGY CLI" when active only on CLI', () => {
      const result = resolveActiveBadgeText({
        isIndividualMode: true,
        acc: { email: 'cli.user@gmail.com' },
        installedAppsList: listAll,
        activeAppMap: {
          desktop: 'other@gmail.com',
          agy: 'cli.user@gmail.com',
          vscode: 'another@gmail.com',
        },
        activeAccount: 'other@gmail.com',
      })
      assert.equal(result.isRowActive, true)
      assert.equal(result.activeBadgeText, 'AGY CLI')
    })

    it('displays "AGY EXT" when active only on VSCode Extension', () => {
      const result = resolveActiveBadgeText({
        isIndividualMode: true,
        acc: { email: 'ext.user@gmail.com' },
        installedAppsList: listAll,
        activeAppMap: {
          desktop: 'other@gmail.com',
          agy: 'cli@gmail.com',
          vscode: 'ext.user@gmail.com',
        },
        activeAccount: 'other@gmail.com',
      })
      assert.equal(result.isRowActive, true)
      assert.equal(result.activeBadgeText, 'AGY EXT')
    })

    it('displays "2.0 & CLI" when active on Desktop and CLI', () => {
      const result = resolveActiveBadgeText({
        isIndividualMode: true,
        acc: { email: 'combo@gmail.com' },
        installedAppsList: listAll,
        activeAppMap: {
          desktop: 'combo@gmail.com',
          agy: 'combo@gmail.com',
          vscode: 'other@gmail.com',
        },
        activeAccount: 'combo@gmail.com',
      })
      assert.equal(result.isRowActive, true)
      assert.equal(result.activeBadgeText, '2.0 & CLI')
    })

    it('displays "2.0 & EXT" when active on Desktop and VSCode Extension', () => {
      const result = resolveActiveBadgeText({
        isIndividualMode: true,
        acc: { email: 'combo@gmail.com' },
        installedAppsList: listAll,
        activeAppMap: {
          desktop: 'combo@gmail.com',
          agy: 'other@gmail.com',
          vscode: 'combo@gmail.com',
        },
        activeAccount: 'combo@gmail.com',
      })
      assert.equal(result.isRowActive, true)
      assert.equal(result.activeBadgeText, '2.0 & EXT')
    })

    it('displays "CLI & EXT" when active on CLI and VSCode Extension', () => {
      const result = resolveActiveBadgeText({
        isIndividualMode: true,
        acc: { email: 'combo@gmail.com' },
        installedAppsList: listAll,
        activeAppMap: {
          desktop: 'other@gmail.com',
          agy: 'combo@gmail.com',
          vscode: 'combo@gmail.com',
        },
        activeAccount: 'other@gmail.com',
      })
      assert.equal(result.isRowActive, true)
      assert.equal(result.activeBadgeText, 'CLI & EXT')
    })

    it('displays "All" when only 2 apps installed and active on both', () => {
      const installedTwo: InstalledAppsStatus = { desktop: true, agy: true, vscode: false }
      const listTwo = resolveInstalledAppsList(installedTwo)
      const result = resolveActiveBadgeText({
        isIndividualMode: true,
        acc: { email: 'dual@gmail.com' },
        installedAppsList: listTwo,
        activeAppMap: {
          desktop: 'dual@gmail.com',
          agy: 'dual@gmail.com',
        },
        activeAccount: 'dual@gmail.com',
      })
      assert.equal(result.isRowActive, true)
      // Because all installed apps are using it, it shows "All"
      assert.equal(result.activeBadgeText, 'All')
    })

    it('handles case-insensitive email comparisons in activeAppMap', () => {
      const result = resolveActiveBadgeText({
        isIndividualMode: true,
        acc: { email: 'Alice.Smith@Example.COM' },
        installedAppsList: listAll,
        activeAppMap: {
          desktop: 'alice.smith@example.com',
        },
        activeAccount: '',
      })
      assert.equal(result.isRowActive, true)
      assert.equal(result.activeBadgeText, 'AGY 2.0')
    })

    it('marks row inactive when account is not used by any application', () => {
      const result = resolveActiveBadgeText({
        isIndividualMode: true,
        acc: { email: 'standby@gmail.com' },
        installedAppsList: listAll,
        activeAppMap: {
          desktop: 'other1@gmail.com',
          agy: 'other2@gmail.com',
          vscode: 'other3@gmail.com',
        },
        activeAccount: 'other1@gmail.com',
      })
      assert.equal(result.isRowActive, false)
      assert.equal(result.usingApps.length, 0)
    })
  })
})
