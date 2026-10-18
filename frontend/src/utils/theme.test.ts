/**
 * theme.test.ts — Theme resolution logic tests (Auto / Light / Dark)
 *
 * Verifies mode normalization, OS preference resolution ("auto"),
 * <html data-theme> application, localStorage persistence, live OS
 * preference changes, and API/localStorage bootstrap reconciliation.
 */

import { describe, it, beforeEach } from 'node:test'
import assert from 'node:assert/strict'
import { initDOM } from '../test-dom'
import {
  DEFAULT_THEME_MODE,
  THEME_STORAGE_KEY,
  applyTheme,
  getStoredThemeMode,
  initTheme,
  normalizeThemeMode,
  resolveThemeMode,
  systemPrefersDark,
  watchSystemThemeChanges,
  type ThemeMode,
} from '../theme'

/** Stubs window.matchMedia and captures registered change listeners. */
function stubSystemPreference(prefersDark: boolean): { listeners: Array<() => void>; set: (v: boolean) => void } {
  const listeners: Array<() => void> = []
  let current = prefersDark
  ;(window as any).matchMedia = (query: string) => ({
    matches: query.includes('dark') ? current : false,
    addEventListener: (_type: string, cb: () => void) => listeners.push(cb),
    removeEventListener: () => {},
    addListener: (cb: () => void) => listeners.push(cb),
    removeListener: () => {},
  })
  return {
    listeners,
    set: (v: boolean) => {
      current = v
    },
  }
}

describe('theme resolution (src/theme.ts)', () => {
  beforeEach(() => {
    const { window } = initDOM()
    window.localStorage.clear()
    stubSystemPreference(false)
  })

  it('watchSystemThemeChanges re-resolves the theme while the mode is auto', () => {
    const stub = stubSystemPreference(false)
    applyTheme('auto')
    assert.equal(document.documentElement.dataset.theme, 'light')
    assert.equal(systemPrefersDark(), false)

    watchSystemThemeChanges()
    assert.equal(stub.listeners.length, 1, 'exactly one change listener must be registered')

    // OS switches to dark while the user mode is "auto" -> dark theme applies.
    stub.set(true)
    stub.listeners[0]()
    assert.equal(document.documentElement.dataset.theme, 'dark', 'auto must follow the OS preference change')

    // An explicit user choice must NOT be overridden by OS changes.
    applyTheme('light')
    stub.set(true)
    stub.listeners[0]()
    assert.equal(document.documentElement.dataset.theme, 'light', 'explicit light mode must win over OS changes')
  })

  it('normalizeThemeMode accepts only auto/light/dark and defaults to auto', () => {
    assert.equal(normalizeThemeMode('auto'), 'auto')
    assert.equal(normalizeThemeMode('light'), 'light')
    assert.equal(normalizeThemeMode('dark'), 'dark')
    assert.equal(normalizeThemeMode(' DARK '), 'dark')
    assert.equal(normalizeThemeMode('Light'), 'light')
    assert.equal(normalizeThemeMode(''), DEFAULT_THEME_MODE)
    assert.equal(normalizeThemeMode('neon'), DEFAULT_THEME_MODE)
    assert.equal(normalizeThemeMode(null), DEFAULT_THEME_MODE)
    assert.equal(normalizeThemeMode(42), DEFAULT_THEME_MODE)
  })

  it('resolveThemeMode passes light/dark through and resolves auto from the OS', () => {
    assert.equal(resolveThemeMode('light'), 'light')
    assert.equal(resolveThemeMode('dark'), 'dark')

    const stub = stubSystemPreference(false)
    assert.equal(resolveThemeMode('auto'), 'light')
    stub.set(true)
    assert.equal(resolveThemeMode('auto'), 'dark')
  })

  it('applyTheme writes the effective theme to <html data-theme> and persists the mode', () => {
    assert.equal(applyTheme('dark'), 'dark')
    assert.equal(document.documentElement.dataset.theme, 'dark')
    assert.equal(window.localStorage.getItem(THEME_STORAGE_KEY), 'dark')

    assert.equal(applyTheme('light'), 'light')
    assert.equal(document.documentElement.dataset.theme, 'light')
    assert.equal(window.localStorage.getItem(THEME_STORAGE_KEY), 'light')

    // "auto" persists the mode but resolves the effective theme from the OS.
    stubSystemPreference(true)
    assert.equal(applyTheme('auto'), 'dark')
    assert.equal(window.localStorage.getItem(THEME_STORAGE_KEY), 'auto')
    assert.equal(document.documentElement.dataset.theme, 'dark')
  })

  it('getStoredThemeMode reads the persisted mode and falls back to auto', () => {
    assert.equal(getStoredThemeMode(), DEFAULT_THEME_MODE)
    window.localStorage.setItem(THEME_STORAGE_KEY, 'dark')
    assert.equal(getStoredThemeMode(), 'dark')
    window.localStorage.setItem(THEME_STORAGE_KEY, 'garbage')
    assert.equal(getStoredThemeMode(), DEFAULT_THEME_MODE)
  })

  it('initTheme applies the stored mode, then reconciles with the API value', async () => {
    window.localStorage.setItem(THEME_STORAGE_KEY, 'light')

    // API wins when it responds with a valid mode.
    const mode = await initTheme(async () => 'dark' as ThemeMode)
    assert.equal(mode, 'dark')
    assert.equal(document.documentElement.dataset.theme, 'dark')
    assert.equal(window.localStorage.getItem(THEME_STORAGE_KEY), 'dark')

    // API failures fall back to the locally stored mode.
    window.localStorage.setItem(THEME_STORAGE_KEY, 'light')
    const fallback = await initTheme(async () => {
      throw new Error('offline')
    })
    assert.equal(fallback, 'light')
    assert.equal(document.documentElement.dataset.theme, 'light')

    // Without a loader, the stored mode is returned as-is.
    const stored = await initTheme()
    assert.equal(stored, 'light')
  })
})
