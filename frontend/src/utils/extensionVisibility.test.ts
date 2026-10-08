import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import {
  isExtensionNavigationVisible,
  resolveEffectiveToolIndex,
} from './extensionVisibility.ts'
import type { SystemStatus } from '../types.ts'

describe('extensionVisibility utility', () => {
  const onlineStatus: SystemStatus = {
    daemon_running: true,
    daemon_pid: 1234,
    active_account: 'test@example.com',
    auth_enabled: false,
    auth_configured: false,
    token_valid: true,
    token_age_seconds: 60,
    accounts_count: 2,
    active_index: 0,
    extension_counts: {
      github_workspaces: 1,
      memos: 0,
      prompts: 0,
      skills: 0,
    },
  }

  const offlineStatus: SystemStatus = {
    ...onlineStatus,
    daemon_running: false,
    daemon_pid: 0,
  }

  describe('isExtensionNavigationVisible', () => {
    it('returns true when daemon is running', () => {
      assert.equal(isExtensionNavigationVisible(onlineStatus), true)
    })

    it('returns false when daemon is not running', () => {
      assert.equal(isExtensionNavigationVisible(offlineStatus), false)
    })

    it('returns false when status is null or undefined', () => {
      assert.equal(isExtensionNavigationVisible(null), false)
      assert.equal(isExtensionNavigationVisible(undefined), false)
    })

    it('returns false when daemon_running is missing or falsey', () => {
      const emptyStatus = {} as SystemStatus
      assert.equal(isExtensionNavigationVisible(emptyStatus), false)
    })
  })

  describe('resolveEffectiveToolIndex', () => {
    it('preserves extension tools (7 and 10) when daemon is running', () => {
      assert.equal(resolveEffectiveToolIndex(7, onlineStatus), 7)
      assert.equal(resolveEffectiveToolIndex(10, onlineStatus), 10)
    })

    it('redirects extension tools (7 and 10) to 0 when daemon is offline', () => {
      assert.equal(resolveEffectiveToolIndex(7, offlineStatus), 0)
      assert.equal(resolveEffectiveToolIndex(10, offlineStatus), 0)
    })

    it('redirects extension tools to 0 when status is null or undefined', () => {
      assert.equal(resolveEffectiveToolIndex(7, null), 0)
      assert.equal(resolveEffectiveToolIndex(10, null), 0)
      assert.equal(resolveEffectiveToolIndex(7, undefined), 0)
      assert.equal(resolveEffectiveToolIndex(10, undefined), 0)
    })

    it('preserves non-extension tools regardless of daemon status', () => {
      const nonExtensionTools = [0, 1, 2, 3, 4, 5, 6, 8, 9]
      for (const tool of nonExtensionTools) {
        assert.equal(resolveEffectiveToolIndex(tool, onlineStatus), tool)
        assert.equal(resolveEffectiveToolIndex(tool, offlineStatus), tool)
        assert.equal(resolveEffectiveToolIndex(tool, null), tool)
      }
    })
  })
})
