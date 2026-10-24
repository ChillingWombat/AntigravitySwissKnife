/**
 * acp_agent_config.test.ts — ACP agent configurability
 *
 * Covers the API client contract for the new ACP registry endpoints and the
 * rendered controls that let a user set up / edit / add agents from the page.
 */

import { describe, it, afterEach } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { api, cancelAllInFlightRequests } from '../api.ts'
import { mount } from '../test-dom'
import { ACPAgentMeshPage } from '../pages/ACPAgentMeshPage'

const MESH_FIXTURE = {
  status: 'online',
  mesh_nodes: 2,
  protocol_version: 'v1.2.0-draft',
  agents: [
    {
      id: 'agent-antigravity',
      name: 'Google Antigravity 2.0',
      type: 'Desktop IDE & Agent Core',
      binary_path: '/opt/Antigravity/antigravity',
      pid: 0,
      port_socket: 'unix:///run/user/1000/antigravity-acp.sock',
      acp_version: 'v1.2.0-draft',
      status: 'unreachable',
      ping_latency_ms: 0,
      supported_tools: ['read_file', 'write_file'],
      last_handshake: 'Offline',
      custom: false,
      os_default: true,
      editable: true,
    },
    {
      id: 'custom-my-agent',
      name: 'My Agent',
      type: 'Custom Agent',
      binary_path: '/usr/local/bin/my-agent',
      pid: 0,
      port_socket: '127.0.0.1:19001',
      acp_version: 'unknown',
      status: 'unreachable',
      ping_latency_ms: 0,
      supported_tools: [],
      last_handshake: 'Offline',
      custom: true,
      os_default: false,
      editable: true,
    },
  ],
}

describe('ACP agent registry API client', () => {
  const originalFetch = globalThis.fetch

  afterEach(() => {
    globalThis.fetch = originalFetch
    cancelAllInFlightRequests()
  })

  const recordFetch = () => {
    const calls: Array<{ url: string; method?: string; body?: string }> = []
    globalThis.fetch = (async (url: any, opts: any) => {
      calls.push({ url: String(url), method: opts?.method, body: opts?.body })
      return {
        ok: true,
        status: 200,
        json: async () => MESH_FIXTURE,
      } as any
    }) as any
    return calls
  }

  it('GETs the merged mesh from /api/utilities/acp', async () => {
    const calls = recordFetch()
    const res = await api.getAcpMesh()
    assert.equal(calls[0].url, '/api/utilities/acp')
    assert.equal(res.agents.length, 2)
    assert.equal(res.agents[1].custom, true)
    assert.equal(res.agents[0].os_default, true)
  })

  it('POSTs an agent override to /api/utilities/acp/agents', async () => {
    const calls = recordFetch()
    await api.saveAcpAgent({
      id: 'agent-antigravity',
      name: 'Google Antigravity 2.0',
      endpoint: '127.0.0.1:19000',
      tools: ['read_file'],
    })

    assert.equal(calls[0].url, '/api/utilities/acp/agents')
    assert.equal(calls[0].method, 'POST')
    const body = JSON.parse(calls[0].body || '{}')
    assert.equal(body.id, 'agent-antigravity')
    assert.equal(body.endpoint, '127.0.0.1:19000')
    assert.deepEqual(body.tools, ['read_file'])
  })

  it('DELETEs an agent by id with an encoded query parameter', async () => {
    const calls = recordFetch()
    await api.deleteAcpAgent('custom-my agent')
    assert.equal(calls[0].url, '/api/utilities/acp/agents?id=custom-my%20agent')
    assert.equal(calls[0].method, 'DELETE')
  })

  it('POSTs to the detect endpoint with the agent id', async () => {
    const calls = recordFetch()
    await api.detectAcpAgent('agent-antigravity')
    assert.equal(calls[0].url, '/api/utilities/acp/detect')
    assert.equal(calls[0].method, 'POST')
    assert.deepEqual(JSON.parse(calls[0].body || '{}'), { id: 'agent-antigravity' })
  })

  it('POSTs to the protocol ping endpoint with an optional agent id', async () => {
    const calls = recordFetch()
    await api.pingAcpAgents('agent-antigravity-acp')
    assert.equal(calls[0].url, '/api/utilities/acp/ping')
    assert.equal(calls[0].method, 'POST')
    assert.deepEqual(JSON.parse(calls[0].body || '{}'), { id: 'agent-antigravity-acp' })
  })

  it('POSTs a prompt and working directory to the prompt endpoint', async () => {
    const calls = recordFetch()
    await api.promptAcpAgent('agent-pi', 'Reply with pong', '/tmp/workspace')
    assert.equal(calls[0].url, '/api/utilities/acp/prompt')
    assert.equal(calls[0].method, 'POST')
    assert.deepEqual(JSON.parse(calls[0].body || '{}'), {
      id: 'agent-pi',
      prompt: 'Reply with pong',
      cwd: '/tmp/workspace',
    })
  })
})

describe('ACP Agent Mesh page configurability', () => {
  const originalFetch = globalThis.fetch

  afterEach(() => {
    globalThis.fetch = originalFetch
    cancelAllInFlightRequests()
  })

  const mountPage = async () => {
    globalThis.fetch = (async () => ({
      ok: true,
      status: 200,
      json: async () => MESH_FIXTURE,
    })) as any

    const rendered = await mount(React.createElement(ACPAgentMeshPage))
    // Let the initial useEffect fetch resolve.
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 20))
    })
    return rendered
  }

  it('renders Add Agent without the removed OS setup action', async () => {
    const { querySelector, unmount } = await mountPage()

    const addButton = querySelector('#btnAcpAddAgent')
    assert.ok(addButton, 'Add Agent button must exist as the inline fallback')
    assert.match(addButton?.textContent || '', /Add Agent/)
    assert.equal(querySelector('#btnAcpSetupForOS'), null)

    unmount()
  })

  it('renders agent cards without the removed discovered-daemons banner', async () => {
    const { querySelectorAll, unmount } = await mountPage()

    const gadgetCards = querySelectorAll('.google-card')
    assert.ok(gadgetCards.length > 0, 'ACP mesh gadget must render')
    assert.ok(
      !gadgetCards.some((card) => (card.textContent || '').includes('Discovered Daemons')),
      'Discovered Daemons banner must not render'
    )
    assert.ok(
      gadgetCards.some((card) => (card.textContent || '').includes('Google Antigravity 2.0')),
      'agent cards must still render'
    )

    unmount()
  })

  it('renders per-agent Edit / Auto-detect actions and a Custom badge', async () => {
    const { querySelector, querySelectorAll, unmount } = await mountPage()

    const cards = querySelectorAll('[data-acp-agent-id]')
    assert.equal(cards.length, 2, 'both agents must render')

    const customCard = querySelector('[data-acp-agent-id="custom-my-agent"]')
    assert.ok(customCard, 'custom agent card must render')
    assert.match(customCard?.textContent || '', /Custom/, 'custom agent must show a Custom badge')
    assert.match(customCard?.textContent || '', /Delete/, 'custom agent must offer Delete')

    const labels = querySelectorAll('[aria-label^="Edit "]')
    assert.equal(labels.length, 2, 'every agent must be editable')

    const detectLabels = querySelectorAll('[aria-label^="Auto-detect endpoint"]')
    assert.equal(detectLabels.length, 2, 'every agent must offer auto-detect')

    unmount()
  })

  it('opens the edit dialog prefilled with the current endpoint', async () => {
    const { querySelector, querySelectorAll, click, unmount } = await mountPage()

    const editButton = querySelectorAll('[aria-label^="Edit "]')[0]
    await click(editButton)

    const dialog = querySelector('[role="dialog"]')
    assert.ok(dialog, 'edit dialog must open')

    const endpointInput = querySelector<HTMLInputElement>('#acp-agent-endpoint')
    assert.ok(endpointInput, 'endpoint field must exist')
    assert.equal(endpointInput?.value, 'unix:///run/user/1000/antigravity-acp.sock')

    const nameInput = querySelector<HTMLInputElement>('#acp-agent-name')
    assert.equal(nameInput?.value, 'Google Antigravity 2.0')

    unmount()
  })

  it('opens an empty editor for a brand-new custom agent', async () => {
    const { querySelector, click, unmount } = await mountPage()

    await click(querySelector('#btnAcpAddAgent'))

    const dialog = querySelector('[role="dialog"]')
    assert.ok(dialog, 'add dialog must open')
    assert.match(dialog?.textContent || '', /Add ACP Agent/)

    const nameInput = querySelector<HTMLInputElement>('#acp-agent-name')
    assert.equal(nameInput?.value, '')
    const endpointInput = querySelector<HTMLInputElement>('#acp-agent-endpoint')
    assert.equal(endpointInput?.value, '')

    unmount()
  })

  it('shows detected candidates when auto-detect is clicked', async () => {
    const { querySelector, querySelectorAll, click, unmount } = await mountPage()

    globalThis.fetch = (async (url: any) => {
      if (String(url).includes('/api/utilities/acp/detect')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({
            candidates: [
              {
                endpoint: '\\\\.\\pipe\\antigravity-acp',
                kind: 'pipe',
                reachable: true,
                note: 'OS default for this host',
              },
              {
                endpoint: 'unix:///run/user/1000/antigravity-acp.sock',
                kind: 'unix',
                reachable: false,
                note: 'Currently configured endpoint',
              },
            ],
            os_default: '\\\\.\\pipe\\antigravity-acp',
            pid: 0,
          }),
        } as any
      }
      return { ok: true, status: 200, json: async () => MESH_FIXTURE } as any
    }) as any

    await click(querySelectorAll('[aria-label^="Auto-detect endpoint"]')[0])
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 20))
    })

    const dialog = querySelector('[role="dialog"]')
    assert.ok(dialog, 'detect dialog must open')
    const text = dialog?.textContent || ''
    assert.match(text, /OS default for this host/, 'candidate note must be shown')
    assert.match(text, /Reachable/, 'reachable candidates must be labelled')
    assert.match(text, /No answer/, 'unreachable candidates must be labelled')

    unmount()
  })

  it('falls back to the mesh and excludes idle/not-found nodes from ping success', async () => {
    const rendered = await mountPage()

    globalThis.fetch = (async (url: any) => {
      const target = String(url)
      if (target.includes('/api/utilities/acp/ping')) {
        // Older test mocks and daemons answer with the plain mesh shape.
        return { ok: true, status: 200, json: async () => MESH_FIXTURE } as any
      }
      return {
        ok: true,
        status: 200,
        json: async () => ({
          ...MESH_FIXTURE,
          agents: [
            { ...MESH_FIXTURE.agents[0], status: 'connected', ping_latency_ms: 1.2 },
            { ...MESH_FIXTURE.agents[1], status: 'installed_idle', ping_latency_ms: null },
            {
              ...MESH_FIXTURE.agents[1],
              id: 'custom-not-found',
              name: 'Absent Agent',
              status: 'not_found',
              ping_latency_ms: null,
            },
          ],
        }),
      } as any
    }) as any

    const pingAll = rendered
      .querySelectorAll('button')
      .find((button) => (button.textContent || '').includes('Ping All ACP Nodes'))
    await rendered.click(pingAll)
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 20))
    })

    const pageText = rendered.container.textContent || ''
    assert.match(pageText, /1 active agent daemon responded successfully/)
    assert.match(pageText, /installed_idle/)
    assert.match(pageText, /not_found/)

    rendered.unmount()
  })

  it('uses the mesh fallback when an older API client has no ping method', async () => {
    const rendered = await mountPage()
    const originalPing = (api as any).pingAcpAgents
    ;(api as any).pingAcpAgents = undefined

    try {
      globalThis.fetch = (async () => ({
        ok: true,
        status: 200,
        json: async () => ({
          ...MESH_FIXTURE,
          agents: [{ ...MESH_FIXTURE.agents[0], status: 'connected', ping_latency_ms: 0.8 }],
        }),
      })) as any

      const pingAll = rendered
        .querySelectorAll('button')
        .find((button) => (button.textContent || '').includes('Ping All ACP Nodes'))
      await rendered.click(pingAll)
      await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 20))
      })

      assert.match(rendered.container.textContent || '', /1 active agent daemon responded successfully/)
      assert.match(rendered.container.textContent || '', /MESH_STATUS_FALLBACK/)
    } finally {
      ;(api as any).pingAcpAgents = originalPing
      rendered.unmount()
    }
  })
})
