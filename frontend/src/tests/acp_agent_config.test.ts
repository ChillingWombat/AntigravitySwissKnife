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

  it('POSTs to the sync-registry endpoint', async () => {
    const calls = recordFetch()
    await api.syncAcpRegistry()
    assert.equal(calls[0].url, '/api/utilities/acp/sync-registry')
    assert.equal(calls[0].method, 'POST')
  })
})

describe('ACP Adapter page configurability', () => {
  const originalFetch = globalThis.fetch

  afterEach(() => {
    globalThis.fetch = originalFetch
    cancelAllInFlightRequests()
  })

  const mountPage = async (customFetch?: any, activeTab?: number) => {
    if (customFetch) {
      globalThis.fetch = customFetch
    } else {
      globalThis.fetch = (async () => ({
        ok: true,
        status: 200,
        json: async () => MESH_FIXTURE,
      })) as any
    }

    const rendered = await mount(React.createElement(ACPAgentMeshPage, { activeTab }))
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

  it('shows the daemon error instead of the empty state when the initial mesh read fails', async () => {
    const { container, unmount } = await mountPage((async () => {
      throw new Error('Daemon route is not allowlisted')
    }) as any)

    assert.match(container.textContent || '', /Daemon route is not allowlisted/)
    assert.doesNotMatch(container.textContent || '', /No active agent daemons/)
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
      !gadgetCards.some((card) => (card.textContent || '').includes('Handshake ping completed')),
      'Top handshake alert banner must not render'
    )
    assert.ok(
      gadgetCards.some((card) => (card.textContent || '').includes('Antigravity ACP Host Adapter')),
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
        const body = {
          success: true,
          count: 2,
          status: 'success',
          source: 'auto-detect',
          candidates: [
            {
              endpoint: '\\\\.\\pipe\\antigravity-acp',
              kind: 'pipe',
              reachable: true,
              note: 'OS default for this host',
              source: 'os_default',
              status: 'reachable',
            },
            {
              endpoint: 'unix:///run/user/1000/antigravity-acp.sock',
              kind: 'unix',
              reachable: false,
              note: 'Currently configured endpoint',
              source: 'configured',
              status: 'unreachable',
            },
          ],
          os_default: '\\\\.\\pipe\\antigravity-acp',
          pid: 0,
        }
        return {
          ok: true,
          status: 200,
          headers: new Headers({ 'Content-Type': 'application/json' }),
          text: async () => JSON.stringify(body),
          json: async () => body,
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
    const rendered = await mountPage(undefined, 1)

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
    assert.ok(!pageText.includes('ACP Handshake ping completed'), 'top alert banner must not render')
    assert.match(pageText, /installed_idle/)
    assert.match(pageText, /not_found/)

    rendered.unmount()
  })

  it('uses the mesh fallback when an older API client has no ping method', async () => {
    const rendered = await mountPage(undefined, 1)
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

      assert.ok(
        !(rendered.container.textContent || '').includes('ACP Handshake ping completed'),
        'top alert banner must not render'
      )
      assert.match(rendered.container.textContent || '', /MESH_STATUS_FALLBACK/)
    } finally {
      ;(api as any).pingAcpAgents = originalPing
      rendered.unmount()
    }
  })

  it('renders Sync Registry button to the left of Add Agent and handles click', async () => {
    const { querySelector, click, unmount } = await mountPage()
    const syncBtn = querySelector('#btnAcpSyncRegistry')
    assert.ok(syncBtn, 'Sync Registry button must exist')
    assert.ok(
      syncBtn.className.includes('btn-pill-tonal'),
      'Sync Registry button must use btn-pill-tonal class'
    )
    assert.equal((syncBtn as any).tagName?.toLowerCase(), 'button', 'Sync Registry must be a button element')
    const addBtn = querySelector('#btnAcpAddAgent')
    assert.ok(addBtn, 'Add Agent button must exist')

    let syncCalled = false
    globalThis.fetch = (async (url: any) => {
      if (String(url).includes('/api/utilities/acp/sync-registry')) {
        syncCalled = true
        return {
          ok: true,
          status: 200,
          json: async () => ({
            ...MESH_FIXTURE,
            synced: 1,
            summary: 'Successfully synced 1 preset ACP settings from official registry',
          }),
        } as any
      }
      return { ok: true, status: 200, json: async () => MESH_FIXTURE } as any
    }) as any

    await click(syncBtn)
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 20))
    })

    assert.equal(syncCalled, true, 'syncAcpRegistry must be called')
    unmount()
  })

  it('handles sync registry failure gracefully without crashing the UI', async () => {
    const { querySelector, click, unmount, container } = await mountPage()
    const syncBtn = querySelector('#btnAcpSyncRegistry')
    assert.ok(syncBtn, 'Sync Registry button must exist')

    globalThis.fetch = (async (url: any) => {
      if (String(url).includes('/api/utilities/acp/sync-registry')) {
        return {
          ok: false,
          status: 502,
          json: async () => ({
            success: false,
            error: 'failed to sync registry: connection refused',
          }),
        } as any
      }
      return { ok: true, status: 200, json: async () => MESH_FIXTURE } as any
    }) as any

    await click(syncBtn)
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 20))
    })

    assert.ok(syncBtn, 'Sync button remains intact and operable after failure')
    assert.ok(
      !container.textContent?.includes('Failed to sync ACP registry'),
      'top banner must not render on sync error'
    )
    unmount()
  })

  it('renders Antigravity ACP adapter as dedicated section at top with 1x3 sub-sections', async () => {
    const { querySelector, unmount } = await mountPage()
    const antigravityCard = querySelector('[data-acp-agent-id="agent-antigravity"]') as HTMLElement | null
    assert.ok(antigravityCard, 'Antigravity card must exist')
    assert.match(antigravityCard?.textContent || '', /Antigravity ACP Host Adapter/)
    assert.match(antigravityCard?.textContent || '', /Antigravity 2\.0/)
    assert.match(antigravityCard?.textContent || '', /Antigravity CLI/)
    assert.match(antigravityCard?.textContent || '', /Antigravity IDE Extension/)
    assert.ok(!antigravityCard?.textContent?.includes('Socket / Pipe:'), 'Socket / Pipe must be removed from sub-sections')
    assert.ok(!antigravityCard?.textContent?.includes('Integration:'), 'Integration must be removed from sub-sections')
    assert.match(antigravityCard?.textContent || '', /Handshake:/)
    unmount()
  })

  it('positions unified agent-antigravity-acp at the very top as a dedicated section', async () => {
    const MESH_WITH_ACP = {
      ...MESH_FIXTURE,
      agents: [
        {
          id: 'agent-devin',
          name: 'Devin',
          type: 'Autonomous Developer IDE',
          binary_path: '/usr/bin/devin-desktop',
          pid: 0,
          port_socket: 'unix:///run/user/1000/devin-acp.sock',
          acp_version: 'v1.2.0',
          status: 'unreachable',
          ping_latency_ms: 0,
          supported_tools: ['terminal'],
          last_handshake: 'Offline',
          custom: false,
          os_default: true,
          editable: true,
        },
        {
          id: 'agent-antigravity-acp',
          name: 'Google Antigravity (ACP)',
          type: 'Desktop IDE & Agent Core (ACP)',
          binary_path: '~/.local/bin/antigravity-acp',
          pid: 1234,
          port_socket: '',
          transport: 'stdio',
          acp_version: 'v1.2.0-draft',
          status: 'installed_idle',
          ping_latency_ms: null,
          supported_tools: ['read_file', 'terminal'],
          last_handshake: 'never',
          custom: false,
          os_default: true,
          editable: true,
        },
      ],
    }

    const { querySelector, querySelectorAll, unmount } = await mountPage(async () => ({
      ok: true,
      status: 200,
      json: async () => MESH_WITH_ACP,
    }))

    const allCards = querySelectorAll('[data-acp-agent-id]')
    assert.ok(allCards.length >= 2, 'Should render multiple agent cards')
    // First card in DOM must be agent-antigravity-acp
    assert.equal(allCards[0].getAttribute('data-acp-agent-id'), 'agent-antigravity-acp')

    const antigravityCard = querySelector('[data-acp-agent-id="agent-antigravity-acp"]') as HTMLElement | null
    assert.ok(antigravityCard, 'Antigravity ACP card must exist')
    assert.match(antigravityCard?.textContent || '', /Antigravity ACP Host Adapter/)
    assert.match(antigravityCard?.textContent || '', /stdio \(process pipe\)/)
    assert.match(antigravityCard?.textContent || '', /Antigravity 2\.0/)
    assert.match(antigravityCard?.textContent || '', /Antigravity CLI/)
    assert.match(antigravityCard?.textContent || '', /Antigravity IDE Extension/)

    // Verify no INSTALLED tag rendered in adapter card or sub-sections
    const spans = Array.from(antigravityCard?.querySelectorAll('span') || [])
    const spanTexts = spans.map((s) => s.textContent?.trim())
    assert.ok(!spanTexts.includes('INSTALLED'), 'INSTALLED tag must not be rendered in adapter card or sub-sections')

    // Verify sub-card headers do not contain icons
    const subCardHeaders = Array.from(antigravityCard?.querySelectorAll('div[style*="gridTemplateColumns"] > div > div:first-child') || [])
    for (const h of subCardHeaders) {
      const svgs = h.querySelectorAll('svg')
      assert.equal(svgs.length, 0, 'sub-card header must not contain an icon')
    }
    unmount()
  })

  it('removes header subtitle and relabels ping button to the far left on agent cards', async () => {
    const MESH_WITH_DEVIN = {
      ...MESH_FIXTURE,
      agents: [
        {
          id: 'agent-devin',
          name: 'Devin',
          type: 'Autonomous Developer IDE',
          binary_path: '/usr/bin/devin-desktop',
          pid: 0,
          port_socket: 'unix:///run/user/1000/devin-acp.sock',
          acp_version: 'v1.2.0',
          status: 'unreachable',
          ping_latency_ms: 0,
          supported_tools: ['terminal'],
          last_handshake: 'Offline',
          custom: false,
          os_default: true,
          editable: true,
        },
      ],
    }

    const { querySelector, unmount } = await mountPage(async () => ({
      ok: true,
      status: 200,
      json: async () => MESH_WITH_DEVIN,
    }))

    const devinCard = querySelector('[data-acp-agent-id="agent-devin"]') as HTMLElement | null
    assert.ok(devinCard, 'Devin card must exist')

    // 1. Subtitle removal check
    assert.ok(
      !devinCard?.textContent?.includes('Autonomous Developer IDE • PID'),
      'Card header subtitle must be removed'
    )

    // 2. Ping button text and position check
    const buttons = devinCard?.querySelectorAll('button') || []
    assert.ok(buttons.length >= 3, 'Action buttons must exist')
    const pingButton = buttons[0]
    assert.equal(pingButton?.textContent?.trim(), 'Ping', 'Ping button text must be strictly "Ping"')
    assert.equal(pingButton?.getAttribute('aria-label'), 'Ping Devin', 'Ping button must have aria-label')

    unmount()
  })

  it('separates Antigravity ACP adapter and ordinary agent nodes into dedicated google-card sections', async () => {
    const { querySelector, querySelectorAll, unmount } = await mountPage()

    const googleCards = querySelectorAll('.google-card')
    assert.ok(googleCards.length >= 2, 'Must have at least two separate google-card sections')

    const adapterCard = querySelector('[data-acp-agent-id="agent-antigravity"]')
    assert.ok(adapterCard, 'Antigravity adapter card must exist')

    const otherCard = querySelector('[data-acp-agent-id="custom-my-agent"]')
    assert.ok(otherCard, 'Ordinary agent card must exist')

    const adapterSection = adapterCard?.closest('.google-card')
    const otherSection = otherCard?.closest('.google-card')

    assert.ok(adapterSection, 'Adapter card must be inside a google-card section')
    assert.ok(otherSection, 'Ordinary agent card must be inside a google-card section')
    assert.notEqual(adapterSection, otherSection, 'Adapter and ordinary agent cards must reside in separate google-card sections')

    assert.match(adapterSection?.textContent || '', /Antigravity ACP Host Adapter/)
    assert.match(otherSection?.textContent || '', /OTHER AGENTS/)

    unmount()
  })

  it('renders ordinary ACP cards sorted by connected > installed > notfound, then alphabetically', async () => {
    const MESH_SORT_FIXTURE = {
      status: 'online',
      mesh_nodes: 8,
      protocol_version: 'v1.2.0-draft',
      agents: [
        {
          id: 'agent-antigravity-acp',
          name: 'Antigravity ACP Adapter',
          type: 'Host Adapter',
          binary_path: '',
          pid: 0,
          port_socket: 'stdio',
          acp_version: '1.2.0',
          status: 'installed_idle',
          ping_latency_ms: 0,
          supported_tools: [],
          last_handshake: 'never',
          custom: false,
          os_default: true,
          editable: true,
        },
        {
          id: 'agent-devin',
          name: 'Devin',
          type: 'IDE',
          binary_path: '',
          pid: 0,
          port_socket: '',
          acp_version: '1.0',
          status: 'not_found',
          ping_latency_ms: 0,
          supported_tools: [],
          last_handshake: 'never',
          custom: false,
          os_default: true,
          editable: true,
        },
        {
          id: 'agent-opencode',
          name: 'OpenCode',
          type: 'IDE',
          binary_path: '',
          pid: 0,
          port_socket: '',
          acp_version: '1.0',
          status: 'not_found',
          ping_latency_ms: 0,
          supported_tools: [],
          last_handshake: 'never',
          custom: false,
          os_default: true,
          editable: true,
        },
        {
          id: 'agent-deepseek-harness',
          name: 'DeepSeek Harness',
          type: 'IDE',
          binary_path: '',
          pid: 0,
          port_socket: '',
          acp_version: '1.0',
          status: 'not_found',
          ping_latency_ms: 0,
          supported_tools: [],
          last_handshake: 'never',
          custom: false,
          os_default: true,
          editable: true,
        },
        {
          id: 'agent-pi',
          name: 'Pi',
          type: 'Agent',
          binary_path: '',
          pid: 1234,
          port_socket: '127.0.0.1:41888',
          acp_version: '1.3.0',
          status: 'connected',
          ping_latency_ms: 233,
          supported_tools: ['read', 'bash'],
          last_handshake: '2026-10-30',
          custom: false,
          os_default: true,
          editable: true,
        },
        {
          id: 'agent-codex',
          name: 'Codex',
          type: 'CLI',
          binary_path: '/usr/bin/codex',
          pid: 0,
          port_socket: '127.0.0.1:48080',
          acp_version: '1.0',
          status: 'installed_idle',
          ping_latency_ms: 0,
          supported_tools: [],
          last_handshake: 'never',
          custom: false,
          os_default: true,
          editable: true,
        },
        {
          id: 'agent-claude-code',
          name: 'Claude Code CLI',
          type: 'CLI',
          binary_path: '',
          pid: 0,
          port_socket: '',
          acp_version: '1.0',
          status: 'not_found',
          ping_latency_ms: 0,
          supported_tools: [],
          last_handshake: 'never',
          custom: false,
          os_default: true,
          editable: true,
        },
        {
          id: 'agent-cursor',
          name: 'Cursor Editor Agent',
          type: 'IDE',
          binary_path: '',
          pid: 0,
          port_socket: '',
          acp_version: '1.0',
          status: 'not_found',
          ping_latency_ms: 0,
          supported_tools: [],
          last_handshake: 'never',
          custom: false,
          os_default: true,
          editable: true,
        },
      ],
    }

    const { querySelectorAll, unmount } = await mountPage(async () => ({
      ok: true,
      status: 200,
      json: async () => MESH_SORT_FIXTURE,
    }))

    const allCards = querySelectorAll('[data-acp-agent-id]')
    // First card is Antigravity host adapter
    assert.equal(allCards[0].getAttribute('data-acp-agent-id'), 'agent-antigravity-acp')

    // Remaining cards are the ordinary agent nodes in Section 2
    const ordinaryCardIds = Array.from(allCards).slice(1).map((el) => el.getAttribute('data-acp-agent-id'))

    assert.deepEqual(ordinaryCardIds, [
      'agent-pi',                 // Connected
      'agent-codex',              // Installed
      'agent-claude-code',        // Not Found (alphabetical)
      'agent-cursor',             // Not Found
      'agent-deepseek-harness',   // Not Found
      'agent-devin',              // Not Found
      'agent-opencode',           // Not Found
    ])

    unmount()
  })

  it('enforces consistent status tag and omits socket and integration fields across 3 agy apps', async () => {
    // 1. When online: all 3 show ACTIVE
    const MESH_ONLINE = {
      ...MESH_FIXTURE,
      agents: [
        {
          id: 'agent-antigravity-acp',
          name: 'Antigravity ACP Adapter',
          type: 'Host Adapter',
          binary_path: '',
          pid: 100,
          port_socket: 'stdio',
          acp_version: '1.2.0',
          status: 'connected',
          ping_latency_ms: 2,
          supported_tools: [],
          last_handshake: 'now',
          custom: false,
          os_default: true,
          editable: true,
        },
      ],
    }

    const { querySelectorAll: querySelectorAll1, unmount: unmount1 } = await mountPage(async () => ({
      ok: true,
      status: 200,
      json: async () => MESH_ONLINE,
    }))

    const subCardsOnline = querySelectorAll1('[data-sub-agent-id]')
    assert.equal(subCardsOnline.length, 3, 'Must render exactly 3 sub-cards')
    for (const card of subCardsOnline) {
      assert.ok(!card.textContent?.includes('Socket / Pipe:'), 'Must not render Socket / Pipe')
      assert.ok(!card.textContent?.includes('Integration:'), 'Must not render Integration')
      assert.match(card.textContent || '', /Handshake:/)
      const spans = Array.from(card.querySelectorAll('span'))
      const hasActive = spans.some((s) => s.textContent?.trim() === 'ACTIVE')
      assert.ok(hasActive, 'All 3 sub-sections must show consistent ACTIVE status when online')
    }
    unmount1()

    // 2. When idle/installed: all 3 show READY
    const MESH_IDLE = {
      ...MESH_FIXTURE,
      agents: [
        {
          id: 'agent-antigravity-acp',
          name: 'Antigravity ACP Adapter',
          type: 'Host Adapter',
          binary_path: '',
          pid: 0,
          port_socket: 'stdio',
          acp_version: '1.2.0',
          status: 'installed_idle',
          ping_latency_ms: 0,
          supported_tools: [],
          last_handshake: 'never',
          custom: false,
          os_default: true,
          editable: true,
        },
      ],
    }

    const { querySelectorAll: querySelectorAll2, unmount: unmount2 } = await mountPage(async () => ({
      ok: true,
      status: 200,
      json: async () => MESH_IDLE,
    }))

    const subCardsIdle = querySelectorAll2('[data-sub-agent-id]')
    assert.equal(subCardsIdle.length, 3, 'Must render exactly 3 sub-cards')
    for (const card of subCardsIdle) {
      assert.ok(!card.textContent?.includes('Socket / Pipe:'), 'Must not render Socket / Pipe')
      assert.ok(!card.textContent?.includes('Integration:'), 'Must not render Integration')
      assert.match(card.textContent || '', /Handshake:/)
      const spans = Array.from(card.querySelectorAll('span'))
      const hasReady = spans.some((s) => s.textContent?.trim() === 'READY')
      assert.ok(hasReady, 'All 3 sub-sections must show consistent READY status when idle')
    }
    unmount2()
  })

  it('does not render OS default subtitle below endpoints on agent cards', async () => {
    const MESH_WITH_AGENTS = {
      ...MESH_FIXTURE,
      agents: [
        {
          id: 'agent-antigravity-acp',
          name: 'Antigravity ACP Adapter',
          type: 'Host Adapter',
          binary_path: '',
          pid: 0,
          port_socket: 'stdio',
          acp_version: '1.2.0',
          status: 'connected',
          ping_latency_ms: 1,
          supported_tools: [],
          last_handshake: '1s ago',
          custom: false,
          os_default: true,
          editable: true,
        },
        {
          id: 'agent-codex',
          name: 'Codex',
          type: 'Coding Agent',
          binary_path: '/usr/local/bin/codex',
          pid: 0,
          port_socket: '127.0.0.1:48080',
          acp_version: '1.0',
          status: 'installed_idle',
          ping_latency_ms: 0,
          supported_tools: [],
          last_handshake: 'never',
          custom: false,
          os_default: true,
          editable: true,
        },
      ],
    }

    const { querySelectorAll, unmount } = await mountPage(async () => ({
      ok: true,
      status: 200,
      json: async () => MESH_WITH_AGENTS,
    }))

    const cards = querySelectorAll('[data-acp-agent-id]')
    assert.equal(cards.length, 2)
    for (const card of cards) {
      assert.ok(
        !card.textContent?.includes('OS default'),
        `Card ${card.getAttribute('data-acp-agent-id')} must not render "OS default" below endpoint`
      )
    }

    unmount()
  })

  it('switches between Agents and Logs tabs and relocates communication stream to Logs tab', async () => {
    const { querySelector, querySelectorAll, click, unmount } = await mountPage()

    // Tab 0 (Agents) by default
    assert.ok(querySelector('[data-acp-agent-id="agent-antigravity"]'), 'Agents tab must show Antigravity agent')
    assert.ok(querySelector('[data-acp-agent-id="custom-my-agent"]'), 'Agents tab must show Other Agents')
    assert.ok(querySelector('#btnAcpSyncRegistry'), 'Sync Registry must be in Other Agents gadget header')
    assert.ok(querySelector('#btnAcpAddAgent'), 'Add Agent must be in Other Agents gadget header')
    assert.equal(querySelector('select[aria-label="Target ACP Agent"]'), null, 'Logs stream must not be visible on Agents tab')

    // Switch to Logs tab
    const tabs = querySelectorAll('button').filter((b) => b.textContent?.trim() === 'Logs' || b.textContent?.trim() === 'Agents')
    const logsTab = tabs.find((b) => b.textContent?.trim() === 'Logs')
    assert.ok(logsTab, 'Logs tab button must exist')
    await click(logsTab)
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 20))
    })

    // Tab 1 (Logs)
    assert.equal(querySelector('[data-acp-agent-id="agent-antigravity"]'), null, 'Agents cards must not be visible on Logs tab')
    assert.ok(querySelector('select[aria-label="Target ACP Agent"]'), 'Target ACP Agent select must be visible on Logs tab')
    assert.ok(querySelector('input[aria-label="ACP Prompt Input"]'), 'Prompt input must be visible on Logs tab')
    assert.ok(querySelectorAll('button').some((b) => b.textContent?.includes('Ping All ACP Nodes')), 'Ping All ACP Nodes must be in Logs tab')

    unmount()
  })
})
