export const TOOLS = [
  'switcher',
  'system-settings',
  'custom-models',
  'enhancements',
  'extensions',
  'token-monitor',
  'utilities',
  'remote-connect',
  'acp-adapter',
] as const

export type Tool = (typeof TOOLS)[number]

export const SWITCHER_TABS = ['dashboard', 'fingerprints', 'switcher-settings'] as const
export type SwitcherTab = (typeof SWITCHER_TABS)[number]

export const UTILITIES_TABS = ['cache', 'archives', 'import'] as const
export type UtilitiesTab = (typeof UTILITIES_TABS)[number]

type SimpleTool = Exclude<Tool, 'switcher' | 'utilities'>

export type Route =
  | { tool: 'switcher'; tab: SwitcherTab }
  | { tool: 'utilities'; tab: UtilitiesTab }
  | { tool: SimpleTool }

export const DEFAULT_ROUTE = { tool: 'switcher', tab: 'dashboard' } as const satisfies Route

const SIMPLE_TOOLS = TOOLS.filter((tool): tool is SimpleTool => tool !== 'switcher' && tool !== 'utilities')

export function routeToTool(route: Route): Tool {
  return route.tool
}

export function serializeRoute(route: Route): string {
  if (route.tool === 'switcher') return `#/switcher/${route.tab}`
  if (route.tool === 'utilities') return `#/utilities/${route.tab}`
  return `#/${route.tool}`
}

export function parseHash(hash: string): Route {
  const path = hash.startsWith('#') ? hash.slice(1) : hash
  if (!path.startsWith('/')) return DEFAULT_ROUTE

  const segments = path.slice(1).split('/')
  if (segments.length === 1) {
    const tool = segments[0]
    if (SIMPLE_TOOLS.includes(tool as SimpleTool)) return { tool: tool as SimpleTool }
    return DEFAULT_ROUTE
  }

  if (segments.length !== 2 || !segments[1]) return DEFAULT_ROUTE
  const [tool, tab] = segments
  if (tool === 'switcher' && SWITCHER_TABS.includes(tab as SwitcherTab)) {
    return { tool, tab: tab as SwitcherTab }
  }
  if (tool === 'utilities' && UTILITIES_TABS.includes(tab as UtilitiesTab)) {
    return { tool, tab: tab as UtilitiesTab }
  }
  return DEFAULT_ROUTE
}

export function routesEqual(left: Route, right: Route): boolean {
  return serializeRoute(left) === serializeRoute(right)
}

const LEGACY_TOOL_ROUTES: Readonly<Record<number, Route>> = {
  0: DEFAULT_ROUTE,
  1: { tool: 'utilities', tab: 'archives' },
  2: { tool: 'system-settings' },
  3: { tool: 'custom-models' },
  4: { tool: 'enhancements' },
  5: DEFAULT_ROUTE,
  6: { tool: 'utilities', tab: 'archives' },
  7: { tool: 'extensions' },
  8: { tool: 'token-monitor' },
  9: { tool: 'utilities', tab: 'cache' },
  10: { tool: 'remote-connect' },
  11: { tool: 'acp-adapter' },
}

export function routeFromNavigationTarget(target: unknown): Route | null {
  if (typeof target === 'number') return LEGACY_TOOL_ROUTES[target] ?? null
  if (typeof target !== 'string') return null

  const targetPath = target.trim().replace(/^#?\/+/, '')
  if (!targetPath) return null
  const hash = `#/${targetPath}`
  const route = parseHash(hash)
  return serializeRoute(route) === hash ? route : null
}
