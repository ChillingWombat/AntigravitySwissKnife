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

function stringPaths(node: ts.Node | undefined): string[] {
  const paths: string[] = []

  const walk = (candidate: ts.Node | undefined) => {
    if (!candidate) return
    if (ts.isStringLiteral(candidate) || ts.isNoSubstitutionTemplateLiteral(candidate)) {
      paths.push(candidate.text)
      return
    }
    if (ts.isTemplateExpression(candidate)) {
      let value = candidate.head.text
      for (const span of candidate.templateSpans) {
        value += '${}' + span.literal.text
      }
      paths.push(value)
      return
    }
    if (ts.isParenthesizedExpression(candidate) || ts.isAsExpression(candidate)) {
      walk(candidate.expression)
      return
    }
    ts.forEachChild(candidate, walk)
  }

  walk(node)
  return paths
}

function methodFromOptions(node: ts.Node | undefined): DaemonMethod | null {
  if (!node) return 'GET'
  if (!ts.isObjectLiteralExpression(node)) return null
  let methodProperty: ts.PropertyAssignment | null = null
  for (const property of node.properties) {
    if (!ts.isPropertyAssignment(property)) continue
    const name = ts.isIdentifier(property.name) || ts.isStringLiteral(property.name)
      ? property.name.text
      : null
    if (name === 'method') methodProperty = property
  }
  if (!methodProperty) return 'GET'
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

function frontendDaemonRoutes(): { routes: FrontendRoute[]; unresolved: string[] } {
  const routes = new Map<string, FrontendRoute>()
  const unresolved: string[] = []

  const visit = (node: ts.Node) => {
    if (ts.isCallExpression(node)) {
      const line = sourceFile.getLineAndCharacterOfPosition(node.getStart()).line + 1
      const callee = ts.isIdentifier(node.expression) ? node.expression.text : null
      const isRequest = callee === 'request'
      const isFetch = callee === 'fetch'

      if (isRequest || isFetch) {
        const paths = node.arguments[0] ? stringPaths(node.arguments[0]) : []
        const daemonPaths = paths.filter((rawPath) => rawPath.startsWith('/api/'))
        if (isRequest && daemonPaths.length === 0) {
          unresolved.push(`L${line}: request route is not statically resolvable`)
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
          for (const rawPath of stringPaths(argument)) {
            if (rawPath.startsWith('/api/')) {
              unresolved.push(`L${line}: ${rawPath} bypasses request() or fetch()`)
            }
          }
        }
      }
    }
    ts.forEachChild(node, visit)
  }

  visit(sourceFile)
  return { routes: [...routes.values()], unresolved }
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
