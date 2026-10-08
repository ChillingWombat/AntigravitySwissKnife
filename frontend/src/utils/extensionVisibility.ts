import type { SystemStatus } from '../types.ts'

/**
 * Checks whether extension navigation, pages, and tabs should be visible.
 * Extensions are strictly guarded by the active daemon connection.
 */
export function isExtensionNavigationVisible(status: SystemStatus | null | undefined): boolean {
  return Boolean(status?.daemon_running)
}

/**
 * Resolves the effective tool index. If the daemon is disconnected and the
 * requested tool is an extension tool (7: Extensions or 10: GitHub Workspace),
 * safely redirects to the default tool (0: Account Switcher).
 */
export function resolveEffectiveToolIndex(
  currentTool: number,
  status: SystemStatus | null | undefined
): number {
  if (!isExtensionNavigationVisible(status) && (currentTool === 7 || currentTool === 10)) {
    return 0
  }
  return currentTool
}
