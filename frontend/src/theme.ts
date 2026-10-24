/**
 * theme.ts — UI theme mode resolution (Auto / Light / Dark).
 *
 * "auto" follows the operating system preference via matchMedia
 * ('(prefers-color-scheme: dark)'); "light" and "dark" force the mode.
 * The effective theme ('light' | 'dark') is written to
 * document.documentElement.dataset.theme, which index.css keys its
 * :root / :root[data-theme="dark"] custom properties off of.
 */

export type ThemeMode = 'auto' | 'light' | 'dark'
export type EffectiveTheme = 'light' | 'dark'

export const THEME_STORAGE_KEY = 'antigravity_theme'
export const DEFAULT_THEME_MODE: ThemeMode = 'auto'

/** Validates an arbitrary value into a supported theme mode (defaults to "auto"). */
export function normalizeThemeMode(value: unknown): ThemeMode {
  if (typeof value !== 'string') return DEFAULT_THEME_MODE
  switch (value.trim().toLowerCase()) {
    case 'light':
      return 'light'
    case 'dark':
      return 'dark'
    default:
      return DEFAULT_THEME_MODE
  }
}

/** True when the OS currently prefers a dark color scheme. */
export function systemPrefersDark(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

/** Resolves a theme mode into the effective theme ("auto" follows the OS). */
export function resolveThemeMode(mode: ThemeMode): EffectiveTheme {
  if (mode === 'auto') return systemPrefersDark() ? 'dark' : 'light'
  return mode
}

/** Reads the persisted theme mode from localStorage (defaults to "auto"). */
export function getStoredThemeMode(): ThemeMode {
  try {
    if (typeof window !== 'undefined' && window.localStorage) {
      return normalizeThemeMode(window.localStorage.getItem(THEME_STORAGE_KEY))
    }
  } catch {}
  return DEFAULT_THEME_MODE
}

/**
 * Applies a theme mode: persists it, resolves "auto" against the OS preference,
 * and writes the effective theme onto <html data-theme>. Returns the effective theme.
 */
export function applyTheme(mode: ThemeMode): EffectiveTheme {
  const normalized = normalizeThemeMode(mode)
  const resolved = resolveThemeMode(normalized)
  try {
    if (typeof window !== 'undefined' && window.localStorage) {
      window.localStorage.setItem(THEME_STORAGE_KEY, normalized)
    }
  } catch {}
  if (typeof document !== 'undefined') {
    document.documentElement.dataset.theme = resolved
  }
  return resolved
}

let systemThemeListenerBound = false

/**
 * Re-applies the theme when the OS color-scheme preference changes while the
 * user-selected mode is "auto". Idempotent; safe to call on every boot.
 */
export function watchSystemThemeChanges(): void {
  if (systemThemeListenerBound) return
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return
  systemThemeListenerBound = true
  const query = window.matchMedia('(prefers-color-scheme: dark)')
  const onChange = () => {
    if (getStoredThemeMode() === 'auto') applyTheme('auto')
  }
  if (typeof query.addEventListener === 'function') {
    query.addEventListener('change', onChange)
  } else if (typeof query.addListener === 'function') {
    query.addListener(onChange) // legacy Safari
  }
}

/**
 * Bootstraps the theme: applies the locally stored mode immediately (no flash),
 * reconciles with the persisted backend setting when available, and starts
 * following OS preference changes. `loadFromApi` lets callers plug in the
 * settings API fetch without coupling this module to the network layer.
 */
export async function initTheme(loadFromApi?: () => Promise<ThemeMode | null>): Promise<ThemeMode> {
  const stored = getStoredThemeMode()
  applyTheme(stored)
  watchSystemThemeChanges()
  if (!loadFromApi) return stored
  try {
    const remote = await loadFromApi()
    if (remote) {
      const mode = normalizeThemeMode(remote)
      applyTheme(mode)
      return mode
    }
  } catch {}
  return stored
}
