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
  { tool: 'system-settings', tab: 'general' },
  { tool: 'system-settings', tab: 'storage' },
  { tool: 'system-settings', tab: 'privacy' },
  { tool: 'system-settings', tab: 'about' },
  { tool: 'custom-models' },
  { tool: 'enhancements', tab: 'chat-view' },
  { tool: 'enhancements', tab: 'project-panel' },
  { tool: 'enhancements', tab: 'auxiliary-panel' },
  { tool: 'extensions', tab: 'built-in' },
  { tool: 'extensions', tab: 'community' },
  { tool: 'token-monitor', tab: 'overview' },
  { tool: 'token-monitor', tab: 'telemetry' },
  { tool: 'token-monitor', tab: 'token-price' },
  { tool: 'utilities', tab: 'cache' },
  { tool: 'utilities', tab: 'archives' },
  { tool: 'utilities', tab: 'import' },
  { tool: 'remote-connect', tab: 'remote' },
  { tool: 'remote-connect', tab: 'gemini' },
  { tool: 'remote-connect', tab: 'whatsapp' },
  { tool: 'remote-connect', tab: 'telegram' },
  { tool: 'remote-connect', tab: 'discord' },
  { tool: 'acp-adapter' },
]

describe('navigation routes', () => {
  it('round-trips every visible tool and secondary tab through its canonical hash', () => {
    for (const route of ALL_ROUTES) {
      const hash = serializeRoute(route)
      assert.deepEqual(parseHash(hash), route, hash)
      assert.equal(serializeRoute(parseHash(hash)), hash, hash)
    }
  })

  it('serializes canonical defaults and every non-default tab', () => {
    const defaultTabs: Partial<Record<Route['tool'], string>> = {
      'system-settings': 'general',
      'enhancements': 'chat-view',
      'extensions': 'built-in',
      'token-monitor': 'overview',
      'remote-connect': 'remote',
    }

    for (const route of ALL_ROUTES) {
      const hash = serializeRoute(route)
      if ('tab' in route) {
        const expected = defaultTabs[route.tool] === route.tab
          ? `#/${route.tool}`
          : `#/${route.tool}/${route.tab}`
        assert.equal(hash, expected, hash)
      }
    }

    assert.equal(serializeRoute({ tool: 'system-settings', tab: 'storage' }), '#/system-settings/storage')
    assert.equal(serializeRoute({ tool: 'utilities', tab: 'cache' }), '#/utilities/cache')
  })

  it('falls back to dashboard for unknown and illegal hashes', () => {
    for (const hash of [
      '',
      '#',
      '#/',
      '#/unknown',
      '#/switcher',
      '#/switcher/unknown',
      '#/utilities/cache/extra',
      '#/system-settings/extra',
      '#/extensions/unknown',
      '#/remote-connect/unknown',
    ]) {
      assert.deepEqual(parseHash(hash), DEFAULT_ROUTE)
    }
  })

  it('maps canonical external targets and legacy numeric indices', () => {
    assert.deepEqual(routeFromNavigationTarget('utilities/archives'), { tool: 'utilities', tab: 'archives' })
    assert.deepEqual(routeFromNavigationTarget('#/switcher/fingerprints'), { tool: 'switcher', tab: 'fingerprints' })
    assert.deepEqual(routeFromNavigationTarget('extensions/community'), { tool: 'extensions', tab: 'community' })
    assert.deepEqual(routeFromNavigationTarget('remote-connect/discord'), { tool: 'remote-connect', tab: 'discord' })
    assert.deepEqual(routeFromNavigationTarget(2), { tool: 'system-settings', tab: 'general' })
    assert.deepEqual(routeFromNavigationTarget(7), { tool: 'extensions', tab: 'built-in' })
    assert.deepEqual(routeFromNavigationTarget(10), { tool: 'remote-connect', tab: 'remote' })
    assert.equal(routeFromNavigationTarget('not-a-route'), null)
    assert.equal(routeFromNavigationTarget(12), null)
  })

  it('keeps the typed tool projection stable', () => {
    assert.equal(routeToTool({ tool: 'custom-models' }), 'custom-models')
    assert.equal(routeToTool({ tool: 'utilities', tab: 'import' }), 'utilities')
    assert.equal(routeToTool({ tool: 'system-settings', tab: 'privacy' }), 'system-settings')
  })
})
