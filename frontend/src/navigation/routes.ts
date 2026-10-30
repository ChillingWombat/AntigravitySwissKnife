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

export const SYSTEM_SETTINGS_TABS = ['general', 'storage', 'privacy', 'about'] as const
export type SystemSettingsTab = (typeof SYSTEM_SETTINGS_TABS)[number]

export const ENHANCEMENT_TABS = ['chat-view', 'project-panel', 'auxiliary-panel'] as const
export type EnhancementTab = (typeof ENHANCEMENT_TABS)[number]

export const EXTENSIONS_TABS = ['built-in', 'community'] as const
export type ExtensionsTab = (typeof EXTENSIONS_TABS)[number]

export const TOKEN_MONITOR_TABS = ['overview', 'telemetry', 'token-price'] as const
export type TokenMonitorTab = (typeof TOKEN_MONITOR_TABS)[number]

export const BRIDGE_TABS = ['remote', 'gemini', 'whatsapp', 'telegram', 'discord'] as const
export type BridgeTab = (typeof BRIDGE_TABS)[number]

type SimpleTool = Exclude<
  Tool,
  'switcher' | 'utilities' | 'system-settings' | 'enhancements' | 'extensions' | 'token-monitor' | 'remote-connect'
>

export type Route =
  | { tool: 'switcher'; tab: SwitcherTab }
  | { tool: 'utilities'; tab: UtilitiesTab }
  | { tool: 'system-settings'; tab: SystemSettingsTab }
  | { tool: 'enhancements'; tab: EnhancementTab }
  | { tool: 'extensions'; tab: ExtensionsTab }
  | { tool: 'token-monitor'; tab: TokenMonitorTab }
  | { tool: 'remote-connect'; tab: BridgeTab }
  | { tool: SimpleTool }

export const DEFAULT_ROUTE = { tool: 'switcher', tab: 'dashboard' } as const satisfies Route

const DEFAULT_TAB_ROUTES = {
  'system-settings': { tool: 'system-settings', tab: 'general' },
  'enhancements': { tool: 'enhancements', tab: 'chat-view' },
  'extensions': { tool: 'extensions', tab: 'built-in' },
  'token-monitor': { tool: 'token-monitor', tab: 'overview' },
  'remote-connect': { tool: 'remote-connect', tab: 'remote' },
} as const satisfies Record<
  'system-settings' | 'enhancements' | 'extensions' | 'token-monitor' | 'remote-connect',
  Route
>

const SIMPLE_TOOLS = ['custom-models', 'acp-adapter'] as const satisfies readonly SimpleTool[]

export function routeToTool(route: Route): Tool {
  return route.tool
}

export function serializeRoute(route: Route): string {
  if (route.tool === 'switcher') return `#/switcher/${route.tab}`
  if (route.tool === 'utilities') return `#/utilities/${route.tab}`
  if (route.tool === 'system-settings') {
    return route.tab === 'general' ? '#/system-settings' : `#/system-settings/${route.tab}`
  }
  if (route.tool === 'enhancements') {
    return route.tab === 'chat-view' ? '#/enhancements' : `#/enhancements/${route.tab}`
  }
  if (route.tool === 'extensions') {
    return route.tab === 'built-in' ? '#/extensions' : `#/extensions/${route.tab}`
  }
  if (route.tool === 'token-monitor') {
    return route.tab === 'overview' ? '#/token-monitor' : `#/token-monitor/${route.tab}`
  }
  if (route.tool === 'remote-connect') {
    return route.tab === 'remote' ? '#/remote-connect' : `#/remote-connect/${route.tab}`
  }
  return `#/${route.tool}`
}

function parseRoutePath(path: string): Route | null {
  if (!path.startsWith('/')) return null

  const segments = path.slice(1).split('/')
  if (segments.length === 1) {
    const tool = segments[0]
    if ((SIMPLE_TOOLS as readonly string[]).includes(tool)) return { tool: tool as SimpleTool }
    return DEFAULT_TAB_ROUTES[tool as keyof typeof DEFAULT_TAB_ROUTES] ?? null
  }

  if (segments.length !== 2 || !segments[1]) return null
  const [tool, tab] = segments
  if (tool === 'switcher' && SWITCHER_TABS.includes(tab as SwitcherTab)) {
    return { tool, tab: tab as SwitcherTab }
  }
  if (tool === 'utilities' && UTILITIES_TABS.includes(tab as UtilitiesTab)) {
    return { tool, tab: tab as UtilitiesTab }
  }
  if (tool === 'system-settings' && SYSTEM_SETTINGS_TABS.includes(tab as SystemSettingsTab)) {
    return { tool, tab: tab as SystemSettingsTab }
  }
  if (tool === 'enhancements' && ENHANCEMENT_TABS.includes(tab as EnhancementTab)) {
    return { tool, tab: tab as EnhancementTab }
  }
  if (tool === 'extensions' && EXTENSIONS_TABS.includes(tab as ExtensionsTab)) {
    return { tool, tab: tab as ExtensionsTab }
  }
  if (tool === 'token-monitor' && TOKEN_MONITOR_TABS.includes(tab as TokenMonitorTab)) {
    return { tool, tab: tab as TokenMonitorTab }
  }
  if (tool === 'remote-connect' && BRIDGE_TABS.includes(tab as BridgeTab)) {
    return { tool, tab: tab as BridgeTab }
  }
  return null
}

export function parseHash(hash: string): Route {
  const path = hash.startsWith('#') ? hash.slice(1) : hash
  return parseRoutePath(path) ?? DEFAULT_ROUTE
}

export function routesEqual(left: Route, right: Route): boolean {
  return serializeRoute(left) === serializeRoute(right)
}

const LEGACY_TOOL_ROUTES: Readonly<Record<number, Route>> = {
  0: DEFAULT_ROUTE,
  1: { tool: 'utilities', tab: 'archives' },
  2: { tool: 'system-settings', tab: 'general' },
  3: { tool: 'custom-models' },
  4: { tool: 'enhancements', tab: 'chat-view' },
  5: DEFAULT_ROUTE,
  6: { tool: 'utilities', tab: 'archives' },
  7: { tool: 'extensions', tab: 'built-in' },
  8: { tool: 'token-monitor', tab: 'overview' },
  9: { tool: 'utilities', tab: 'cache' },
  10: { tool: 'remote-connect', tab: 'remote' },
  11: { tool: 'acp-adapter' },
}

export function routeFromNavigationTarget(target: unknown): Route | null {
  if (typeof target === 'number') return LEGACY_TOOL_ROUTES[target] ?? null
  if (typeof target !== 'string') return null

  const targetPath = target.trim().replace(/^#?\/+/, '')
  if (!targetPath) return null
  return parseRoutePath(`/${targetPath}`)
}
