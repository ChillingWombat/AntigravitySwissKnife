import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import * as ts from 'typescript'

type DaemonMethod = 'GET' | 'POST' | 'DELETE'
type RoutePolicy = Record<DaemonMethod, Set<string>>

const repoRoot = path.resolve(import.meta.dirname, '..', '..', '..')
const require = createRequire(import.meta.url)
const {
  ALLOWED_ROUTES,
  PRIVILEGED_ROUTES,
  isApprovalRoute,
} = require(path.join(repoRoot, 'electron', 'daemon-broker.js')) as {
  ALLOWED_ROUTES: RoutePolicy
  PRIVILEGED_ROUTES: RoutePolicy
  isApprovalRoute: (method: DaemonMethod, pathOrUrl: string) => boolean
}

const apiSourcePath = path.join(repoRoot, 'frontend', 'src', 'api.ts')
const source = fs.readFileSync(apiSourcePath, 'utf8')
const sourceFile = ts.createSourceFile(apiSourcePath, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)

type StaticPaths = { paths: string[]; dynamic: boolean }

function staticRoutePaths(node: ts.Node | undefined): StaticPaths {
  if (!node) return { paths: [], dynamic: true }
  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
    return { paths: [node.text], dynamic: false }
  }
  if (ts.isTemplateExpression(node)) {
    const pathHead = node.head.text.split('?')[0]
    if (!node.head.text.includes('?') || !pathHead.startsWith('/api/')) {
      return { paths: [], dynamic: true }
    }
    let value = node.head.text
    for (const span of node.templateSpans) {
      value += '${}' + span.literal.text
    }
    return { paths: [value], dynamic: false }
  }
  if (ts.isConditionalExpression(node)) {
    const whenTrue = staticRoutePaths(node.whenTrue)
    const whenFalse = staticRoutePaths(node.whenFalse)
    return {
      paths: [...whenTrue.paths, ...whenFalse.paths],
      dynamic: whenTrue.dynamic || whenFalse.dynamic,
    }
  }
  if (ts.isParenthesizedExpression(node) || ts.isAsExpression(node)) {
    return staticRoutePaths(node.expression)
  }
  return { paths: [], dynamic: true }
}

function methodFromOptions(node: ts.Node | undefined): DaemonMethod | null {
  if (!node) return 'GET'
  if (!ts.isObjectLiteralExpression(node)) return null
  let methodProperty: ts.PropertyAssignment | null = null
  let methodIsFinal = false
  let hasUncertainProperty = false
  for (const property of node.properties) {
    if (ts.isSpreadAssignment(property)) {
      methodIsFinal = false
      hasUncertainProperty = true
      continue
    }
    const name = 'name' in property && property.name && (ts.isIdentifier(property.name) || ts.isStringLiteral(property.name))
      ? property.name.text
      : null
    if (!name) {
      methodIsFinal = false
      hasUncertainProperty = true
      continue
    }
    if (ts.isPropertyAssignment(property) && name === 'method') {
      methodProperty = property
      methodIsFinal = true
      continue
    }
    if (name === 'method') {
      methodProperty = null
      methodIsFinal = false
      hasUncertainProperty = true
    }
  }
  if (!methodProperty) return hasUncertainProperty ? null : 'GET'
  if (!methodIsFinal) return null
  if (!ts.isStringLiteral(methodProperty.initializer)) return null
  const method = methodProperty.initializer.text.toUpperCase()
  return method === 'GET' || method === 'POST' || method === 'DELETE' ? method : null
}

type FrontendRoute = {
  method: DaemonMethod
  path: string
  rawPaths: Set<string>
  line: number
}

function frontendDaemonRoutes(parsedSource: ts.SourceFile = sourceFile): { routes: FrontendRoute[]; unresolved: string[] } {
  const routes = new Map<string, FrontendRoute>()
  const unresolved: string[] = []

  const visit = (node: ts.Node) => {
    if (ts.isCallExpression(node)) {
      const line = parsedSource.getLineAndCharacterOfPosition(node.getStart()).line + 1
      const callee = ts.isIdentifier(node.expression) ? node.expression.text : null
      const isRequest = callee === 'request'
      const isFetch = callee === 'fetch'

      if (isRequest || isFetch) {
        const routeExpression = node.arguments[0] ? staticRoutePaths(node.arguments[0]) : { paths: [], dynamic: true }
        const daemonPaths = routeExpression.paths.filter((rawPath) => rawPath.startsWith('/api/'))
        if (isRequest && (daemonPaths.length === 0 || routeExpression.dynamic)) {
          unresolved.push(`L${line}: request route is not statically resolvable`)
        }
        if (isFetch && daemonPaths.length === 0) {
          ts.forEachChild(node, visit)
          return
        }
        if (isFetch && routeExpression.dynamic) {
          unresolved.push(`L${line}: fetch route has a dynamic branch`)
        }
        if (routeExpression.dynamic) {
          ts.forEachChild(node, visit)
          return
        }
        const method = methodFromOptions(node.arguments[1])
        for (const rawPath of daemonPaths) {
          if (!method) {
            unresolved.push(`L${line}: ${rawPath} has an unsupported or computed method`)
            continue
          }
          const routePath = rawPath.split('?')[0]
          const key = `${method} ${routePath}`
          const existing = routes.get(key)
          if (existing) {
            existing.rawPaths.add(rawPath)
          } else {
            routes.set(key, { method, path: routePath, rawPaths: new Set([rawPath]), line })
          }
        }
      } else if (!(
        ts.isPropertyAccessExpression(node.expression) &&
        ['includes', 'startsWith', 'endsWith', 'indexOf'].includes(node.expression.name.text)
      )) {
        for (const argument of node.arguments) {
          const routeExpression = staticRoutePaths(argument)
          if (routeExpression.dynamic && !routeExpression.paths.some((rawPath) => rawPath.startsWith('/api/'))) {
            continue
          }
          for (const rawPath of routeExpression.paths) {
            if (rawPath.startsWith('/api/')) {
              unresolved.push(`L${line}: ${rawPath} bypasses request() or fetch()`)
            }
          }
        }
      }
    }
    ts.forEachChild(node, visit)
  }

  visit(parsedSource)
  return { routes: [...routes.values()], unresolved }
}

function productionSourceFiles(directory: string): string[] {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const entryPath = path.join(directory, entry.name)
    if (entry.isDirectory()) {
      if (entry.name === 'tests') return []
      return productionSourceFiles(entryPath)
    }
    if (entry.name.includes('.test.')) return []
    return entry.name.endsWith('.ts') || entry.name.endsWith('.tsx') ? [entryPath] : []
  })
}

describe('daemon broker route policy coverage', () => {
  const { routes: frontendRoutes, unresolved } = frontendDaemonRoutes()

  it('classifies every frontend daemon request as allowed or approval-gated', () => {
    assert.deepEqual(unresolved, [])
    assert.equal(frontendRoutes.length, 159)
    const missing = frontendRoutes.filter(({ method, path: routePath }) => {
      return !ALLOWED_ROUTES[method].has(routePath) && !PRIVILEGED_ROUTES[method].has(routePath)
    })
    assert.deepEqual(missing, [])
  })

  it('fails closed on computed methods and partially dynamic routes', () => {
    const fixturePath = path.join(repoRoot, 'frontend', 'src', 'route-policy-fixture.ts')
    const fixture = ts.createSourceFile(
      fixturePath,
      `
        request('/api/custom_models', { ...options })
        request(flag ? '/api/status' : route)
        request('/api/status', { method: 'PUT' })
        request('/api/status', { [name]: 'POST' })
        request('/api/status', { method })
      `,
      ts.ScriptTarget.Latest,
      true,
      ts.ScriptKind.TS,
    )

    const result = frontendDaemonRoutes(fixture)
    assert.deepEqual(result.routes, [])
    assert.equal(result.unresolved.length, 5)
  })

  it('keeps direct daemon calls centralized in api.ts', () => {
    const violations: string[] = []
    for (const file of productionSourceFiles(path.join(repoRoot, 'frontend', 'src'))) {
      if (file === apiSourcePath) continue
      const parsed = ts.createSourceFile(file, fs.readFileSync(file, 'utf8'), ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX)
      const visit = (node: ts.Node) => {
        if (ts.isCallExpression(node)) {
          for (const argument of node.arguments) {
            const routeExpression = staticRoutePaths(argument)
            if (routeExpression.paths.some((rawPath) => rawPath.startsWith('/api/'))) {
              const line = parsed.getLineAndCharacterOfPosition(node.getStart()).line + 1
              violations.push(`${path.relative(repoRoot, file)}:${line}`)
            }
          }
        }
        ts.forEachChild(node, visit)
      }
      visit(parsed)
    }
    assert.deepEqual(violations, [])
  })

  it('preserves approval semantics for every query variant', () => {
    for (const route of frontendRoutes) {
      const privileged = PRIVILEGED_ROUTES[route.method].has(route.path)
      for (const rawPath of route.rawPaths) {
        if (privileged) {
          assert.equal(isApprovalRoute(route.method, rawPath), true, rawPath)
        } else if (!rawPath.includes('unredacted=true')) {
          assert.equal(isApprovalRoute(route.method, rawPath), false, rawPath)
        }
      }
    }
    assert.equal(isApprovalRoute('GET', '/api/bridge/channels/config?unredacted=true'), true)
  })

  it('does not place a route in both the allowed and approval-gated sets', () => {
    for (const method of ['GET', 'POST', 'DELETE'] as const) {
      const overlap = [...ALLOWED_ROUTES[method]].filter((routePath) => PRIVILEGED_ROUTES[method].has(routePath))
      assert.deepEqual(overlap, [], `${method} route policy overlap`)
    }
  })

  it('only references backend paths registered by the daemon', () => {
    const serverSource = fs.readFileSync(
      path.join(repoRoot, 'pkg', 'webgui', 'server.go'),
      'utf8',
    )
    const backendRoutes = new Set(
      [...serverSource.matchAll(/mux\.HandleFunc\("([^"]+)"/g)].map((match) => match[1]),
    )

    for (const policy of [ALLOWED_ROUTES, PRIVILEGED_ROUTES]) {
      for (const method of ['GET', 'POST', 'DELETE'] as const) {
        for (const routePath of policy[method]) {
          assert.ok(backendRoutes.has(routePath), `${method} ${routePath} is not registered by the daemon`)
        }
      }
    }
  })
})
