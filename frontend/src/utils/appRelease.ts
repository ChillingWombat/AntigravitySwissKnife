import type { AppReleaseInfo } from '../types'

/**
 * Compares two semantic version strings (e.g., "2.1.0" vs "2.0.0").
 * Returns 1 if v1 > v2, -1 if v1 < v2, and 0 if v1 === v2.
 */
export function compareVersions(v1: string, v2: string): number {
  const clean1 = (v1 || '').replace(/^v/, '').trim()
  const clean2 = (v2 || '').replace(/^v/, '').trim()

  const parts1 = clean1.split('.').map(p => parseInt(p, 10) || 0)
  const parts2 = clean2.split('.').map(p => parseInt(p, 10) || 0)
  const maxLen = Math.max(parts1.length, parts2.length)

  for (let i = 0; i < maxLen; i++) {
    const num1 = parts1[i] ?? 0
    const num2 = parts2[i] ?? 0
    if (num1 > num2) return 1
    if (num1 < num2) return -1
  }
  return 0
}

/**
 * Formats human-readable release status badge and description.
 */
export function formatReleaseStatus(release: AppReleaseInfo | null): {
  badgeText: string
  badgeClass: string
  description: string
  isUpdateAvailable: boolean
} {
  if (!release) {
    return {
      badgeText: 'Checking...',
      badgeClass: 'badge-neutral',
      description: 'Checking release server...',
      isUpdateAvailable: false,
    }
  }

  if (release.has_update) {
    return {
      badgeText: 'Update Available',
      badgeClass: 'badge-yellow',
      description: `New version v${release.latest_version} available for download`,
      isUpdateAvailable: true,
    }
  }

  return {
    badgeText: 'Up to Date',
    badgeClass: 'badge-green',
    description: `You are running the latest version (v${release.current_version})`,
    isUpdateAvailable: false,
  }
}
