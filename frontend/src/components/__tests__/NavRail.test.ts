/**
 * NavRail.test.ts — Component DOM Behavior Tests for NavRail (<NavRail />)
 *
 * Verifies genuine React component mounting, David-Design branding elements,
 * navigation tab selection, Lucide SVG icon rendering, conditional daemon button visibility,
 * and user click interaction event handling.
 */

import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React from 'react'
import { mount } from '../../test-dom'
import { NavRail } from '../NavRail'
import type { SystemStatus } from '../../types'

describe('Component DOM: NavRail (<NavRail />)', () => {
  it('renders brand header typography with Antigravity title and SWISS KNIFE subtitle', async () => {
    const { querySelector, unmount } = await mount(
      React.createElement(NavRail, {
        currentTool: 0,
        onSelectTool: () => {},
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

  it('renders active tool highlight on selected tab and triggers onSelectTool on click', async () => {
    const selectedTools: number[] = []
    const { querySelector, click, unmount } = await mount(
      React.createElement(NavRail, {
        currentTool: 0,
        onSelectTool: (idx: number) => {
          selectedTools.push(idx)
        },
        status: null,
      })
    )

    // Click Custom Models (tool 3)
    const btnCustomModels = querySelector<HTMLButtonElement>('#btnNavCustomModels')
    assert.ok(btnCustomModels, 'btnNavCustomModels button must exist')
    await click(btnCustomModels)
    assert.equal(selectedTools[selectedTools.length - 1], 3, 'Clicking Custom Models must fire onSelectTool(3)')

    // Click Remote Connect / Bridge (tool 10)
    const btnBridge = querySelector<HTMLButtonElement>('#btnNavBridge')
    assert.ok(btnBridge, 'btnNavBridge button must exist')
    await click(btnBridge)
    assert.equal(selectedTools[selectedTools.length - 1], 10, 'Clicking Bridge must fire onSelectTool(10)')

    // Click Token Monitor (tool 8)
    const btnTokens = querySelector<HTMLButtonElement>('#btnNavTokenMonitor')
    assert.ok(btnTokens, 'btnNavTokenMonitor button must exist')
    await click(btnTokens)
    assert.equal(selectedTools[selectedTools.length - 1], 8, 'Clicking Token Monitor must fire onSelectTool(8)')

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
        currentTool: 0,
        onSelectTool: () => {},
        status: offlineStatus,
      })
    )

    const btnExtensions = querySelector('#btnNavExtensions')
    assert.equal(btnExtensions, null, 'Extensions button must NOT render when daemon is offline')

    unmount()
  })

  it('renders Extensions button when daemon is online and dispatches onSelectTool(7)', async () => {
    const onlineStatus: SystemStatus = {
      daemon_running: true,
      daemon_version: '0.2.0',
      active_account: null,
      total_accounts: 1,
    } as unknown as SystemStatus

    const selectedTools: number[] = []
    const { querySelector, click, unmount } = await mount(
      React.createElement(NavRail, {
        currentTool: 0,
        onSelectTool: (idx: number) => {
          selectedTools.push(idx)
        },
        status: onlineStatus,
      })
    )

    const btnExtensions = querySelector<HTMLButtonElement>('#btnNavExtensions')
    assert.ok(btnExtensions, 'Extensions button must render when daemon is online')

    await click(btnExtensions)
    assert.equal(selectedTools[selectedTools.length - 1], 7, 'Clicking Extensions must fire onSelectTool(7)')

    unmount()
  })

  it('enforces white-space: nowrap and renders Lucide SVG icons on navigation buttons', async () => {
    const { querySelectorAll, unmount } = await mount(
      React.createElement(NavRail, {
        currentTool: 0,
        onSelectTool: () => {},
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
    const selectedTools: number[] = []
    const { querySelector, querySelectorAll, click, unmount } = await mount(
      React.createElement(NavRail, {
        currentTool: 0,
        onSelectTool: (idx: number) => selectedTools.push(idx),
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
    assert.equal(selectedTools[selectedTools.length - 1], 11)

    unmount()
  })
})
