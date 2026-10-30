import { afterEach, describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { routeFromNavigationTarget, serializeRoute } from '../navigation/routes'
import { useRouteNavigation } from '../navigation/useRouteNavigation'
import { initDOM, mount, type RenderResult } from '../test-dom'

const RouteHost = () => {
  const { route, navigate } = useRouteNavigation()
  const navigateTarget = (target: unknown) => {
    const next = routeFromNavigationTarget(target)
    if (next) navigate(next)
  }

  return React.createElement(
    'div',
    null,
    React.createElement('span', { id: 'current-route' }, serializeRoute(route)),
    React.createElement('button', { id: 'system-settings', onClick: () => navigate({ tool: 'system-settings' }) }, 'System'),
    React.createElement('button', { id: 'tray-semantic', onClick: () => navigateTarget('utilities/archives') }, 'Tray semantic'),
    React.createElement('button', { id: 'tray-legacy', onClick: () => navigateTarget(2) }, 'Tray legacy'),
  )
}

const flush = async () => {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
}

describe('App URL routing seam', () => {
  let view: RenderResult | null = null

  afterEach(() => {
    view?.unmount()
    view = null
  })

  async function mountRouteHost() {
    const mounted = await mount(React.createElement(RouteHost))
    view = mounted
    await flush()
    return mounted
  }

  it('restores the rendered route from the hash on initial load and reload', async () => {
    initDOM()
    window.history.replaceState(null, '', '#/utilities/archives')
    let app = await mountRouteHost()
    assert.equal(window.location.hash, '#/utilities/archives')
    assert.equal(app.querySelector('#current-route')?.textContent, '#/utilities/archives')

    app.unmount()
    view = null
    app = await mountRouteHost()
    assert.equal(window.location.hash, '#/utilities/archives')
    assert.equal(app.querySelector('#current-route')?.textContent, '#/utilities/archives')
  })

  it('writes history once when navigating and synchronizes popstate changes', async () => {
    initDOM()
    window.history.replaceState(null, '', '#/switcher/dashboard')
    const app = await mountRouteHost()
    const history = window.history
    const originalPushState = history.pushState
    let pushes = 0
    history.pushState = function (data, unused, url) {
      pushes += 1
      originalPushState.call(history, data, unused, url)
    }

    await app.click(app.querySelector('#system-settings'))
    await flush()
    assert.equal(pushes, 1)
    assert.equal(window.location.hash, '#/system-settings')
    assert.equal(app.querySelector('#current-route')?.textContent, '#/system-settings')

    await act(async () => {
      originalPushState.call(history, null, '', '#/token-monitor')
      window.dispatchEvent(new Event('popstate'))
    })
    await flush()
    assert.equal(window.location.hash, '#/token-monitor')
    assert.equal(app.querySelector('#current-route')?.textContent, '#/token-monitor')
  })

  it('accepts semantic tray targets and legacy numeric targets', async () => {
    initDOM()
    window.history.replaceState(null, '', '#/switcher/dashboard')
    const app = await mountRouteHost()

    await app.click(app.querySelector('#tray-semantic'))
    await flush()
    assert.equal(window.location.hash, '#/utilities/archives')
    assert.equal(app.querySelector('#current-route')?.textContent, '#/utilities/archives')

    await app.click(app.querySelector('#tray-legacy'))
    await flush()
    assert.equal(window.location.hash, '#/system-settings')
    assert.equal(app.querySelector('#current-route')?.textContent, '#/system-settings')
  })
})
