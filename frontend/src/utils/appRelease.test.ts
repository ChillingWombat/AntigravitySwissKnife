import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { compareVersions, formatReleaseStatus } from './appRelease.ts'
import type { AppReleaseInfo } from '../types.ts'

describe('appRelease utility', () => {
  describe('compareVersions', () => {
    it('compares identical versions', () => {
      assert.equal(compareVersions('2.0.0', '2.0.0'), 0)
      assert.equal(compareVersions('v2.0.0', '2.0.0'), 0)
      assert.equal(compareVersions('2.0', '2.0.0'), 0)
    })

    it('returns 1 when v1 is greater than v2', () => {
      assert.equal(compareVersions('2.1.0', '2.0.0'), 1)
      assert.equal(compareVersions('v2.0.1', 'v2.0.0'), 1)
      assert.equal(compareVersions('3.0.0', '2.9.9'), 1)
    })

    it('returns -1 when v1 is less than v2', () => {
      assert.equal(compareVersions('1.9.0', '2.0.0'), -1)
      assert.equal(compareVersions('2.0.0', '2.0.1'), -1)
      assert.equal(compareVersions('2.0.0', '3.0.0'), -1)
    })

    it('handles empty, malformed, or prefix variations safely', () => {
      assert.equal(compareVersions('', '2.0.0'), -1)
      assert.equal(compareVersions('v2.0.0', ''), 1)
      assert.equal(compareVersions('', ''), 0)
    })
  })

  describe('formatReleaseStatus', () => {
    it('returns checking status for null release', () => {
      const res = formatReleaseStatus(null)
      assert.equal(res.isUpdateAvailable, false)
      assert.equal(res.badgeText, 'Checking...')
    })

    it('returns up to date when has_update is false', () => {
      const release: AppReleaseInfo = {
        current_version: '2.0.0',
        latest_version: '2.0.0',
        has_update: false,
        release_name: 'v2.0.0',
        release_notes: '',
        published_at: '',
        html_url: '',
        download_url: '',
        platform: 'linux',
        arch: 'amd64',
        auto_check: true,
        auto_upgrade: false,
        last_checked: '2026-10-08 20:00:00',
        status_message: 'Application is up to date.',
      }
      const res = formatReleaseStatus(release)
      assert.equal(res.isUpdateAvailable, false)
      assert.equal(res.badgeText, 'Up to Date')
      assert.equal(res.badgeClass, 'badge-green')
    })

    it('returns update available when has_update is true', () => {
      const release: AppReleaseInfo = {
        current_version: '2.0.0',
        latest_version: '2.1.0',
        has_update: true,
        release_name: 'v2.1.0 Performance',
        release_notes: 'New release notes',
        published_at: '2026-10-08 12:00:00',
        html_url: 'https://github.com/releases/v2.1.0',
        download_url: 'https://github.com/releases/download/v2.1.0/app.AppImage',
        platform: 'linux',
        arch: 'amd64',
        auto_check: true,
        auto_upgrade: true,
        last_checked: '2026-10-08 20:00:00',
        status_message: 'A newer release (v2.1.0) is available!',
      }
      const res = formatReleaseStatus(release)
      assert.equal(res.isUpdateAvailable, true)
      assert.equal(res.badgeText, 'Update Available')
      assert.equal(res.badgeClass, 'badge-yellow')
    })
  })
})
