import { useCallback, useEffect, useState } from 'react'
import {
  DEFAULT_ROUTE,
  parseHash,
  routesEqual,
  serializeRoute,
  type Route,
} from './routes'

export interface NavigateOptions {
  replace?: boolean
}

export type Navigate = (next: Route, options?: NavigateOptions) => void

function readRouteFromLocation(): Route {
  if (typeof window === 'undefined') return DEFAULT_ROUTE
  return parseHash(window.location.hash)
}

function replaceHash(hash: string): void {
  if (typeof window === 'undefined' || window.location.hash === hash) return
  window.history.replaceState(null, '', hash)
}

function writeHash(hash: string, replace: boolean): void {
  if (typeof window === 'undefined' || window.location.hash === hash) return
  if (replace) {
    window.history.replaceState(null, '', hash)
  } else {
    window.history.pushState(null, '', hash)
  }
}

export function useRouteNavigation(): { route: Route; navigate: Navigate } {
  const [route, setRoute] = useState<Route>(readRouteFromLocation)

  const navigate = useCallback<Navigate>((next, options = {}) => {
    writeHash(serializeRoute(next), options.replace === true)
    setRoute((current) => (routesEqual(current, next) ? current : next))
  }, [])

  useEffect(() => {
    const syncFromLocation = () => {
      const next = parseHash(window.location.hash)
      setRoute((current) => (routesEqual(current, next) ? current : next))
    }

    syncFromLocation()
    window.addEventListener('popstate', syncFromLocation)
    window.addEventListener('hashchange', syncFromLocation)
    return () => {
      window.removeEventListener('popstate', syncFromLocation)
      window.removeEventListener('hashchange', syncFromLocation)
    }
  }, [])

  useEffect(() => {
    replaceHash(serializeRoute(route))
  }, [route])

  return { route, navigate }
}
