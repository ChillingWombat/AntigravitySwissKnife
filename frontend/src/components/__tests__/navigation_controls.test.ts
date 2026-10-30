import { afterEach, describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React from 'react'
import { NavRail } from '../NavRail'
import { TopRibbon } from '../TopRibbon'
import type { Route } from '../../navigation/routes'
import { mount, type RenderResult } from '../../test-dom'

describe('semantic navigation controls', () => {
  let view: RenderResult | null = null

  afterEach(() => {
    view?.unmount()
    view = null
  })

  it('emits semantic routes from the navigation rail', async () => {
    const calls: Route[] = []
    view = await mount(React.createElement(NavRail, {
      currentTool: 'switcher',
      navigate: (route) => calls.push(route),
      status: null,
    }))

    for (const id of ['btnNavCustomModels', 'btnNavUtilities', 'btnNavBridge', 'btnNavACPAgentMesh', 'btnNavSystemSettings']) {
      const button = view.querySelector<HTMLButtonElement>(`#${id}`)
      assert.ok(button, `${id} must render`)
      await view.click(button)
    }

    assert.deepEqual(calls, [
      { tool: 'custom-models' },
      { tool: 'utilities', tab: 'cache' },
      { tool: 'remote-connect', tab: 'remote' },
      { tool: 'acp-adapter' },
      { tool: 'system-settings', tab: 'general' },
    ])
  })

  it('emits semantic switcher routes from the top ribbon', async () => {
    const calls: Route[] = []
    view = await mount(React.createElement(TopRibbon, {
      currentTab: 'dashboard',
      navigate: (route) => calls.push(route),
      activeAccount: null,
    }))

    const buttons = view.querySelectorAll<HTMLButtonElement>('button')
    const fingerprints = buttons.find((button) => button.textContent === 'Fingerprints')
    const settings = buttons.find((button) => button.textContent === 'Settings')
    assert.ok(fingerprints)
    assert.ok(settings)
    await view.click(fingerprints)
    await view.click(settings)

    assert.deepEqual(calls, [
      { tool: 'switcher', tab: 'fingerprints' },
      { tool: 'switcher', tab: 'switcher-settings' },
    ])
  })
})
