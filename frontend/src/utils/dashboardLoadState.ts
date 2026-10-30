import type { AccountState, FleetQuotaSummary, RuleConfig, SystemStatus } from '../types'

export const DASHBOARD_ENDPOINTS = ['status', 'quota', 'rules', 'projects', 'accounts'] as const

export type DashboardEndpoint = typeof DASHBOARD_ENDPOINTS[number]
export type EndpointStatus = 'initial' | 'loading' | 'ready' | 'degraded' | 'error'
export type EndpointSource = 'direct' | 'fleet'
export type DashboardFailure = 'request_failed' | 'invalid_response'

export interface EndpointState<T> {
  status: EndpointStatus
  data: T | null
  error: DashboardFailure | null
  updatedAt: number | null
  source: EndpointSource
}

export interface DashboardProject {
  name: string
  color: string
  order: number
  is_archived: boolean
  folder_path?: string
}

export type AccountInput = AccountState | Record<string, unknown>

export interface DashboardDataMap {
  status: SystemStatus
  quota: FleetQuotaSummary
  rules: RuleConfig
  projects: DashboardProject[]
  accounts: AccountInput[]
}

export type DashboardLoadState = {
  [K in DashboardEndpoint]: EndpointState<DashboardDataMap[K]>
}

type DashboardFulfilledAction = {
  [K in DashboardEndpoint]: {
    type: 'load/fulfilled'
    endpoint: K
    data: DashboardDataMap[K]
  }
}[DashboardEndpoint]

export type DashboardLoadAction =
  | { type: 'load/start'; endpoints: readonly DashboardEndpoint[] }
  | { type: 'load/rejected'; endpoints: readonly DashboardEndpoint[]; error: DashboardFailure }
  | DashboardFulfilledAction

const EMPTY_ENDPOINT = {
  status: 'initial',
  data: null,
  error: null,
  updatedAt: null,
  source: 'direct',
} as const

export function createInitialDashboardLoadState(): DashboardLoadState {
  return {
    status: { ...EMPTY_ENDPOINT },
    quota: { ...EMPTY_ENDPOINT },
    rules: { ...EMPTY_ENDPOINT },
    projects: { ...EMPTY_ENDPOINT },
    accounts: { ...EMPTY_ENDPOINT },
  }
}

export function hasUsableData<T>(data: T | null): data is T {
  return data !== null
}

function beginLoad<T>(endpoint: EndpointState<T>): EndpointState<T> {
  return {
    ...endpoint,
    status: 'loading',
    error: null,
  }
}

function finishLoad<T>(
  endpoint: EndpointState<T>,
  data: T,
  source: EndpointSource,
): EndpointState<T> {
  return {
    ...endpoint,
    status: 'ready',
    data,
    error: null,
    updatedAt: Date.now(),
    source,
  }
}

function rejectLoad<T>(endpoint: EndpointState<T>, error: DashboardFailure): EndpointState<T> {
  return {
    ...endpoint,
    status: hasUsableData(endpoint.data) ? 'degraded' : 'error',
    error,
  }
}

function replaceEndpoint<K extends DashboardEndpoint>(
  state: DashboardLoadState,
  endpoint: K,
  next: DashboardLoadState[K],
): DashboardLoadState {
  return { ...state, [endpoint]: next }
}

export function dashboardReducer(
  state: DashboardLoadState,
  action: DashboardLoadAction,
): DashboardLoadState {
  if (action.type === 'load/start') {
    return action.endpoints.reduce(
      (next, endpoint) => ({
        ...next,
        [endpoint]: beginLoad(next[endpoint] as EndpointState<unknown>),
      } as DashboardLoadState),
      state,
    )
  }

  if (action.type === 'load/rejected') {
    return action.endpoints.reduce(
      (next, endpoint) => ({
        ...next,
        [endpoint]: rejectLoad(next[endpoint] as EndpointState<unknown>, action.error),
      } as DashboardLoadState),
      state,
    )
  }

  if (action.endpoint === 'quota') {
    const next = replaceEndpoint(
      state,
      'quota',
      finishLoad(state.quota, action.data, 'direct'),
    )
    if (action.data.accounts.length === 0) return next
    return replaceEndpoint(
      next,
      'accounts',
      finishLoad(next.accounts, action.data.accounts, 'fleet'),
    )
  }

  if (action.endpoint === 'accounts') {
    const fleetAccounts = state.quota.data?.accounts
    if (fleetAccounts && fleetAccounts.length > 0) {
      return replaceEndpoint(
        state,
        'accounts',
        finishLoad(state.accounts, fleetAccounts, 'fleet'),
      )
    }
    return replaceEndpoint(
      state,
      'accounts',
      finishLoad(state.accounts, action.data, 'direct'),
    )
  }

  if (action.endpoint === 'status') {
    return replaceEndpoint(
      state,
      'status',
      finishLoad(state.status, action.data, 'direct'),
    )
  }
  if (action.endpoint === 'rules') {
    return replaceEndpoint(
      state,
      'rules',
      finishLoad(state.rules, action.data, 'direct'),
    )
  }
  return replaceEndpoint(
    state,
    'projects',
    finishLoad(state.projects, action.data, 'direct'),
  )
}

export function failedDashboardEndpoints(
  state: DashboardLoadState,
): DashboardEndpoint[] {
  return DASHBOARD_ENDPOINTS.filter((endpoint) => {
    const status = state[endpoint].status
    return status === 'degraded' || status === 'error'
  })
}

export function dashboardIsOffline(state: DashboardLoadState): boolean {
  return DASHBOARD_ENDPOINTS.every((endpoint) => state[endpoint].status === 'error')
}

export const SAFE_DASHBOARD_ERROR_MESSAGES: Record<DashboardFailure, string> = {
  request_failed: 'Request failed',
  invalid_response: 'Invalid response',
}

export function safeDashboardErrorMessage(error: DashboardFailure): string {
  return SAFE_DASHBOARD_ERROR_MESSAGES[error]
}
