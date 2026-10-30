import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import {
  DEFAULT_ROUTE,
  parseHash,
  routeFromNavigationTarget,
  routeToTool,
  serializeRoute,
  type Route,
} from './routes'

const ALL_ROUTES: Route[] = [
  { tool: 'switcher', tab: 'dashboard' },
  { tool: 'switcher', tab: 'fingerprints' },
  { tool: 'switcher', tab: 'switcher-settings' },
  { tool: 'system-settings' },
  { tool: 'custom-models' },
  { tool: 'enhancements' },
  { tool: 'extensions' },
  { tool: 'token-monitor' },
  { tool: 'utilities', tab: 'cache' },
  { tool: 'utilities', tab: 'archives' },
  { tool: 'utilities', tab: 'import' },
  { tool: 'remote-connect' },
  { tool: 'acp-adapter' },
]

describe('navigation routes', () => {
  it('round-trips every legal route through its canonical hash', () => {
    for (const route of ALL_ROUTES) {
      const hash = serializeRoute(route)
      assert.deepEqual(parseHash(hash), route)
      assert.equal(hash, serializeRoute(parseHash(hash)))
    }
  })

  it('falls back to dashboard for unknown and illegal hashes', () => {
    for (const hash of ['', '#', '#/', '#/unknown', '#/switcher', '#/switcher/unknown', '#/utilities/cache/extra', '#/system-settings/extra']) {
      assert.deepEqual(parseHash(hash), DEFAULT_ROUTE)
    }
  })

  it('maps canonical external targets and legacy numeric indices', () => {
    assert.deepEqual(routeFromNavigationTarget('utilities/archives'), { tool: 'utilities', tab: 'archives' })
    assert.deepEqual(routeFromNavigationTarget('#/switcher/fingerprints'), { tool: 'switcher', tab: 'fingerprints' })
    assert.deepEqual(routeFromNavigationTarget(2), { tool: 'system-settings' })
    assert.deepEqual(routeFromNavigationTarget(6), { tool: 'utilities', tab: 'archives' })
    assert.equal(routeFromNavigationTarget('not-a-route'), null)
    assert.equal(routeFromNavigationTarget(12), null)
  })

  it('keeps the typed tool projection stable', () => {
    assert.equal(routeToTool({ tool: 'custom-models' }), 'custom-models')
    assert.equal(routeToTool({ tool: 'utilities', tab: 'import' }), 'utilities')
  })
})
