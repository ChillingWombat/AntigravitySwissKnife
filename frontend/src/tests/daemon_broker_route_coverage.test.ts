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
} = require(path.join(repoRoot, 'electron', 'daemon-broker.js')) as {
  ALLOWED_ROUTES: RoutePolicy
  PRIVILEGED_ROUTES: RoutePolicy
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

function methodFromOptions(node: ts.Node | undefined): DaemonMethod {
  if (!node || !ts.isObjectLiteralExpression(node)) return 'GET'
  for (const property of node.properties) {
    if (!ts.isPropertyAssignment(property)) continue
    const name = ts.isIdentifier(property.name) || ts.isStringLiteral(property.name)
      ? property.name.text
      : null
    if (name === 'method' && ts.isStringLiteral(property.initializer)) {
      const method = property.initializer.text.toUpperCase()
      if (method === 'GET' || method === 'POST' || method === 'DELETE') return method
    }
  }
  return 'GET'
}

function frontendDaemonRoutes(): Array<{ method: DaemonMethod; path: string }> {
  const routes = new Map<string, { method: DaemonMethod; path: string }>()

  const visit = (node: ts.Node) => {
    if (ts.isCallExpression(node) && ts.isIdentifier(node.expression)) {
      const isRequest = node.expression.text === 'request'
      const isFetch = node.expression.text === 'fetch'
      if ((isRequest || isFetch) && node.arguments[0]) {
        const method = methodFromOptions(node.arguments[1])
        for (const rawPath of stringPaths(node.arguments[0])) {
          const routePath = rawPath.split('?')[0]
          if (!routePath.startsWith('/api/')) continue
          routes.set(`${method} ${routePath}`, { method, path: routePath })
        }
      }
    }
    ts.forEachChild(node, visit)
  }

  visit(sourceFile)
  return [...routes.values()]
}

describe('daemon broker route policy coverage', () => {
  const frontendRoutes = frontendDaemonRoutes()

  it('classifies every frontend daemon request as allowed or approval-gated', () => {
    assert.ok(frontendRoutes.length >= 150)
    const missing = frontendRoutes.filter(({ method, path: routePath }) => {
      return !ALLOWED_ROUTES[method].has(routePath) && !PRIVILEGED_ROUTES[method].has(routePath)
    })
    assert.deepEqual(missing, [])
  })

  it('does not place a route in both the allowed and approval-gated sets', () => {
    for (const method of ['GET', 'POST', 'DELETE'] as const) {
      const overlap = [...ALLOWED_ROUTES[method]].filter((routePath) => PRIVILEGED_ROUTES[method].has(routePath))
      assert.deepEqual(overlap, [], `${method} route policy overlap`)
    }
  })

  it('only references backend routes registered by the daemon', () => {
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
