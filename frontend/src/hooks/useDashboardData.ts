import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api } from '../api'
import type { FleetQuotaSummary, RuleConfig, SystemStatus } from '../types'
import { toAccountState } from '../types'
import {
  DASHBOARD_ENDPOINTS,
  createInitialDashboardLoadState,
  dashboardReducer,
  failedDashboardEndpoints,
  type AccountInput,
  type DashboardDataMap,
  type DashboardEndpoint,
  type DashboardFailure,
  type DashboardLoadAction,
  type DashboardProject,
} from '../utils/dashboardLoadState'

export interface DashboardLoaders {
  status: () => Promise<SystemStatus>
  quota: () => Promise<FleetQuotaSummary>
  rules: () => Promise<RuleConfig>
  projects: () => Promise<DashboardProject[]>
  accounts: () => Promise<AccountInput[]>
}

export interface UseDashboardDataOptions {
  loaders?: DashboardLoaders
  autoLoad?: boolean
}

class InvalidDashboardResponse extends Error {}

function requireArray<T>(value: T[]): T[] {
  if (!Array.isArray(value)) throw new InvalidDashboardResponse()
  return value
}

export const defaultDashboardLoaders: DashboardLoaders = {
  status: () => api.getStatus(),
  quota: () => api.getFleetQuota(),
  rules: () => api.getRules(),
  projects: async () => requireArray(await api.getGUIProjects()),
  accounts: async () => requireArray(await api.getAccounts()),
}

function fulfilledAction<K extends DashboardEndpoint>(
  endpoint: K,
  data: DashboardDataMap[K],
): DashboardLoadAction {
  return { type: 'load/fulfilled', endpoint, data } as DashboardLoadAction
}

function typedFailure(error: unknown): DashboardFailure {
  return error instanceof InvalidDashboardResponse ? 'invalid_response' : 'request_failed'
}

export function useDashboardData({
  loaders = defaultDashboardLoaders,
  autoLoad = true,
}: UseDashboardDataOptions = {}) {
  const [state, setState] = useState(createInitialDashboardLoadState)
  const loadersRef = useRef(loaders)
  const epochRef = useRef(0)
  const inFlightRef = useRef<Promise<void> | null>(null)
  loadersRef.current = loaders

  const runLoad = useCallback((
    endpoints: readonly DashboardEndpoint[],
    supersede: boolean,
  ): Promise<void> => {
    if (!supersede && inFlightRef.current) return inFlightRef.current
    if (endpoints.length === 0) return Promise.resolve()

    const epoch = ++epochRef.current
    setState((current) => dashboardReducer(current, { type: 'load/start', endpoints }))

    const promise = Promise.all(endpoints.map(async (endpoint) => {
      try {
        const data = await loadersRef.current[endpoint]()
        if (epoch !== epochRef.current) return
        setState((current) => dashboardReducer(
          current,
          fulfilledAction(endpoint, data as DashboardDataMap[typeof endpoint]),
        ))
      } catch (error) {
        if (epoch !== epochRef.current) return
        setState((current) => dashboardReducer(current, {
          type: 'load/rejected',
          endpoints: [endpoint],
          error: typedFailure(error),
        }))
      }
    })).then(() => undefined).finally(() => {
      if (epoch === epochRef.current) inFlightRef.current = null
    })

    inFlightRef.current = promise
    return promise
  }, [])

  const load = useCallback(
    () => runLoad(DASHBOARD_ENDPOINTS, false),
    [runLoad],
  )

  const failedEndpoints = useMemo(() => failedDashboardEndpoints(state), [state])

  const retry = useCallback(
    (endpoints: readonly DashboardEndpoint[] = failedEndpoints) => runLoad(endpoints, true),
    [failedEndpoints, runLoad],
  )

  useEffect(() => {
    if (autoLoad) void runLoad(DASHBOARD_ENDPOINTS, false)
    return () => {
      epochRef.current += 1
      inFlightRef.current = null
    }
  }, [autoLoad, runLoad])

  const isLoading = useMemo(
    () => DASHBOARD_ENDPOINTS.some((endpoint) => state[endpoint].status === 'loading'),
    [state],
  )

  const directAccounts = state.accounts.source === 'direct' ? state.accounts.data ?? [] : []
  const effectiveAccounts = useMemo(() => {
    const fleetAccounts = state.quota.data?.accounts ?? []
    if (fleetAccounts.length > 0) return fleetAccounts
    const activeEmail = state.status.data?.active_account || state.quota.data?.active_account || ''
    return directAccounts.map((account) => toAccountState(
      account,
      activeEmail,
      state.rules.data?.allow_non_gemini_native_models,
    ))
  }, [
    directAccounts,
    state.quota.data,
    state.rules.data?.allow_non_gemini_native_models,
    state.status.data?.active_account,
  ])

  return {
    state,
    isLoading,
    failedEndpoints,
    effectiveAccounts,
    directAccounts,
    load,
    retry,
  }
}
