import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import type { StorageInfo, StoragePaths, AppZonesInfo } from '../types.ts'

describe('StorageContract & SystemSettingsPage Tab 1 Data Mapping', () => {
  const minimalValidStorageInfo: StorageInfo = {
    storage_mode: 'system_default',
    current_paths: {
      config_dir: '/home/user/.config/antigravity-swiss',
      credentials_path: '/home/user/.config/antigravity-swiss/accounts.json',
      temp_dir: '/tmp/antigravity-swiss',
      socket_path: '/tmp/antigravity-swiss/swiss.sock',
    },
    system_default_paths: {
      config_dir: '/home/user/.config/antigravity-swiss',
      credentials_path: '/home/user/.config/antigravity-swiss/accounts.json',
      temp_dir: '/tmp/antigravity-swiss',
      socket_path: '/tmp/antigravity-swiss/swiss.sock',
    },
    app_execution_type: 'standard_binary',
    app_execution_detail: 'Standard OS binary distribution',
    can_migrate: false,
    app_zones: {
      desktop: {
        app_type: 'desktop',
        display_name: 'Antigravity 2.0 Desktop App',
        detected_path: '/opt/Antigravity/antigravity',
        custom_path: '',
        active_path: '/opt/Antigravity/antigravity',
        installed: true,
        version: '2.0.0',
        account_overrides: {},
      },
      agy: {
        app_type: 'agy',
        display_name: 'agy CLI',
        detected_path: '/usr/local/bin/agy',
        custom_path: '',
        active_path: '/usr/local/bin/agy',
        installed: true,
        version: '1.4.0',
        account_overrides: {},
      },
      vscode: {
        app_type: 'vscode',
        display_name: 'VS Code Extension',
        detected_path: '',
        custom_path: '',
        active_path: '',
        installed: false,
        version: '',
        account_overrides: {},
      },
    },
  }

  // Simulates the Tab 1 envPaths derivation in SystemSettingsPage.tsx
  function deriveEnvPaths(
    storageInfo: StorageInfo | null | undefined,
    desktopAppPath?: string
  ) {
    return [
      {
        key: 'config_dir',
        label: 'Configuration Directory:',
        val: storageInfo?.current_paths?.config_dir || '—',
      },
      {
        key: 'credentials',
        label: 'Credentials / Accounts File:',
        val: storageInfo?.current_paths?.credentials_path || '—',
      },
      {
        key: 'temp_dir',
        label: 'Temp & Cache Directory:',
        val: storageInfo?.current_paths?.temp_dir || '—',
      },
      {
        key: 'socket',
        label: 'Daemon IPC Socket:',
        val: storageInfo?.current_paths?.socket_path || '—',
      },
      {
        key: 'antigravity_bin',
        label: 'Antigravity Binary:',
        val: desktopAppPath || '—',
      },
    ]
  }

  // Simulates the Tab 1 App Zone resolution in SystemSettingsPage.tsx
  function resolveZoneData(
    storageInfo: StorageInfo | null | undefined,
    appType: 'desktop' | 'agy' | 'vscode',
    customPaths: Record<string, string>
  ) {
    const zone = storageInfo?.app_zones?.[appType]
    const detectedPath = zone?.detected_path || 'Not detected'
    const isInstalled = zone?.installed ?? false
    const version = zone?.version || ''
    const currentVal = customPaths[appType] ?? (zone?.custom_path || '')
    const overrides = zone?.account_overrides || {}
    return { detectedPath, isInstalled, version, currentVal, overrides }
  }

  describe('Type safety without app_portable_paths', () => {
    it('accepts StorageInfo completely devoid of app_portable_paths', () => {
      const info: StorageInfo = minimalValidStorageInfo
      assert.equal('app_portable_paths' in info, false)
      assert.equal(info.app_portable_paths, undefined)
      assert.equal(info.storage_mode, 'system_default')
      assert.equal(info.can_migrate, false)
    })

    it('allows app_portable_paths to be optional if present', () => {
      const withPortable: StorageInfo = {
        ...minimalValidStorageInfo,
        app_portable_paths: minimalValidStorageInfo.system_default_paths,
      }
      assert.ok(withPortable.app_portable_paths)
      assert.equal(withPortable.app_portable_paths.config_dir, '/home/user/.config/antigravity-swiss')
    })
  })

  describe('Tab 1 rendering resilience across payload states', () => {
    it('correctly maps envPaths with full system_default storage metadata', () => {
      const paths = deriveEnvPaths(minimalValidStorageInfo, '/opt/Antigravity/antigravity')
      assert.equal(paths.length, 5)
      assert.equal(paths[0].val, '/home/user/.config/antigravity-swiss')
      assert.equal(paths[1].val, '/home/user/.config/antigravity-swiss/accounts.json')
      assert.equal(paths[2].val, '/tmp/antigravity-swiss')
      assert.equal(paths[3].val, '/tmp/antigravity-swiss/swiss.sock')
      assert.equal(paths[4].val, '/opt/Antigravity/antigravity')
    })

    it('falls back gracefully to em-dash when storageInfo is null (loading state)', () => {
      const paths = deriveEnvPaths(null, undefined)
      assert.equal(paths.length, 5)
      for (const p of paths) {
        assert.equal(p.val, '—')
      }
    })

    it('falls back gracefully when current_paths is partially missing', () => {
      const incompleteInfo = {
        ...minimalValidStorageInfo,
        current_paths: {} as StoragePaths,
      }
      const paths = deriveEnvPaths(incompleteInfo, undefined)
      assert.equal(paths[0].val, '—')
      assert.equal(paths[1].val, '—')
      assert.equal(paths[2].val, '—')
      assert.equal(paths[3].val, '—')
    })

    it('resolves application zones cleanly with installed desktop and agy', () => {
      const custom = { desktop: '/custom/desktop' }
      const desktop = resolveZoneData(minimalValidStorageInfo, 'desktop', custom)
      assert.equal(desktop.detectedPath, '/opt/Antigravity/antigravity')
      assert.equal(desktop.isInstalled, true)
      assert.equal(desktop.version, '2.0.0')
      assert.equal(desktop.currentVal, '/custom/desktop')

      const vscode = resolveZoneData(minimalValidStorageInfo, 'vscode', custom)
      assert.equal(vscode.detectedPath, 'Not detected')
      assert.equal(vscode.isInstalled, false)
      assert.equal(vscode.version, '')
      assert.equal(vscode.currentVal, '')
      assert.deepEqual(vscode.overrides, {})
    })

    it('safely handles missing app_zones object without runtime exceptions', () => {
      const noZones = {
        ...minimalValidStorageInfo,
        app_zones: undefined as unknown as AppZonesInfo,
      }
      const desktop = resolveZoneData(noZones, 'desktop', {})
      assert.equal(desktop.detectedPath, 'Not detected')
      assert.equal(desktop.isInstalled, false)
      assert.equal(desktop.version, '')
      assert.equal(desktop.currentVal, '')
      assert.deepEqual(desktop.overrides, {})
    })
  })
})
