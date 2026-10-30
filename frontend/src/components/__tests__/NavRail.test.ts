/**
 * NavRail.test.ts — Component DOM Behavior Tests for NavRail (<NavRail />)
 *
 * Verifies genuine React component mounting, David-Design branding elements,
 * navigation tab selection, Lucide SVG icon rendering, conditional daemon button visibility,
 * and user click interaction event handling.
 */

import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { initDOM, mount } from '../../test-dom'
import { NavRail } from '../NavRail'
import type { SystemStatus } from '../../types'
import { parseHash, type Route } from '../../navigation/routes'
import { useRouteNavigation } from '../../navigation/useRouteNavigation'

const RouteNavRail: React.FC<{ status: SystemStatus | null }> = ({ status }) => {
  const { route, navigate } = useRouteNavigation()
  return React.createElement(NavRail, {
    currentTool: route.tool,
    navigate,
    status,
  })
}

const flushNavigation = async () => {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
}

describe('Component DOM: NavRail (<NavRail />)', () => {
  it('renders brand header typography with Antigravity title and SWISS KNIFE subtitle', async () => {
    const { querySelector, unmount } = await mount(
      React.createElement(NavRail, {
        currentTool: 'switcher',
        navigate: () => {},
        status: null,
      })
    )

    const title = querySelector('#brandTitle')
    assert.ok(title, 'Brand title element must exist')
    assert.equal(title.textContent?.trim(), 'Antigravity')

    const subtitle = querySelector('#brandSubtitle')
    assert.ok(subtitle, 'Brand subtitle element must exist')
    assert.ok(subtitle.textContent?.replace(/\u00a0/g, ' ').includes('SWISS KNIFE'))

    unmount()
  })

  it('renders active tool highlight on selected tab and navigates to semantic routes on click', async () => {
    initDOM()
    window.history.replaceState(null, '', '#/switcher/dashboard')
    const { querySelector, click, unmount } = await mount(
      React.createElement(RouteNavRail, { status: null })
    )

    const btnAccount = querySelector<HTMLButtonElement>('#btnNavAccountSwitcher')
    assert.ok(btnAccount, 'Account Switcher button must exist')
    assert.equal(btnAccount.style.fontWeight, '600', 'Selected tool must render its active highlight')

    const btnCustomModels = querySelector<HTMLButtonElement>('#btnNavCustomModels')
    assert.ok(btnCustomModels, 'btnNavCustomModels button must exist')
    await click(btnCustomModels)
    await flushNavigation()
    assert.equal(window.location.hash, '#/custom-models')
    assert.deepEqual(parseHash(window.location.hash), { tool: 'custom-models' })
    assert.equal(btnCustomModels.style.fontWeight, '600', 'Custom Models must become the active tool')

    const btnBridge = querySelector<HTMLButtonElement>('#btnNavBridge')
    assert.ok(btnBridge, 'btnNavBridge button must exist')
    await click(btnBridge)
    await flushNavigation()
    assert.equal(window.location.hash, '#/remote-connect')
    assert.deepEqual(parseHash(window.location.hash), { tool: 'remote-connect', tab: 'remote' })
    assert.equal(btnBridge.style.fontWeight, '600', 'Remote Connect must become the active tool')

    const btnTokens = querySelector<HTMLButtonElement>('#btnNavTokenMonitor')
    assert.ok(btnTokens, 'btnNavTokenMonitor button must exist')
    await click(btnTokens)
    await flushNavigation()
    assert.equal(window.location.hash, '#/token-monitor')
    assert.deepEqual(parseHash(window.location.hash), { tool: 'token-monitor', tab: 'overview' })
    assert.equal(btnTokens.style.fontWeight, '600', 'Token Monitor must become the active tool')

    unmount()
  })

  it('conditionally hides Extensions button when daemon is offline', async () => {
    const offlineStatus: SystemStatus = {
      daemon_running: false,
      daemon_version: '0.2.0',
      active_account: null,
      total_accounts: 0,
    } as unknown as SystemStatus

    const { querySelector, unmount } = await mount(
      React.createElement(NavRail, {
        currentTool: 'switcher',
        navigate: () => {},
        status: offlineStatus,
      })
    )

    const btnExtensions = querySelector('#btnNavExtensions')
    assert.equal(btnExtensions, null, 'Extensions button must NOT render when daemon is offline')

    unmount()
  })

  it('renders Extensions button when daemon is online and navigates to extensions', async () => {
    const onlineStatus: SystemStatus = {
      daemon_running: true,
      daemon_version: '0.2.0',
      active_account: null,
      total_accounts: 1,
    } as unknown as SystemStatus
    initDOM()
    window.history.replaceState(null, '', '#/switcher/dashboard')

    const { querySelector, click, unmount } = await mount(
      React.createElement(RouteNavRail, { status: onlineStatus })
    )

    const btnExtensions = querySelector<HTMLButtonElement>('#btnNavExtensions')
    assert.ok(btnExtensions, 'Extensions button must render when daemon is online')

    await click(btnExtensions)
    await flushNavigation()
    assert.equal(window.location.hash, '#/extensions')
    assert.deepEqual(parseHash(window.location.hash), { tool: 'extensions', tab: 'built-in' })
    assert.equal(btnExtensions.style.fontWeight, '600', 'Extensions must become the active tool')

    unmount()
  })

  it('enforces white-space: nowrap and renders Lucide SVG icons on navigation buttons', async () => {
    const { querySelectorAll, unmount } = await mount(
      React.createElement(NavRail, {
        currentTool: 'switcher',
        navigate: () => {},
        status: null,
      })
    )

    const buttons = querySelectorAll<HTMLButtonElement>('aside button')
    assert.ok(buttons.length >= 6, 'Nav rail must contain multiple nav buttons')

    for (const btn of buttons) {
      assert.equal(btn.style.whiteSpace, 'nowrap', 'Each nav button must enforce white-space: nowrap invariant')
      const svg = btn.querySelector('svg')
      assert.ok(svg, 'Each nav button must render a Lucide SVG icon')
    }

    unmount()
  })

  it('orders primary navigation exactly and keeps Archived Projects out of the rail', async () => {
    const onlineStatus = {
      daemon_running: true,
      daemon_version: '0.2.0',
      active_account: null,
      total_accounts: 0,
    } as unknown as SystemStatus
    const selectedTools: Route[] = []
    const { querySelector, querySelectorAll, click, unmount } = await mount(
      React.createElement(NavRail, {
        currentTool: 'switcher',
        navigate: (route: Route) => selectedTools.push(route),
        status: onlineStatus,
      })
    )

    const expected = [
      'Account Switcher',
      'Custom Models',
      'Token Monitor',
      'Remote Connect',
      'ACP Adapter',
      'Extensions',
      'UI Enhancements',
      'Utilities',
    ]
    const labels = querySelectorAll<HTMLButtonElement>('aside button')
      .map((button) => button.textContent?.trim() || '')
      .filter((label) => expected.includes(label))
    assert.deepEqual(labels, expected)
    assert.equal(querySelector('#btnNavArchivedProjects'), null)

    const acpButton = querySelector<HTMLButtonElement>('#btnNavACPAgentMesh')
    assert.ok(acpButton, 'ACP Adapter button must exist')
    await click(acpButton)
    assert.deepEqual(selectedTools[selectedTools.length - 1], { tool: 'acp-adapter' })

    unmount()
  })
})
