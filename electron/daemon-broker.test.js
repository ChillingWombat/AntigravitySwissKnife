'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {
  ALLOWED_ROUTES,
  APPROVAL_BODY_PREVIEW_LIMIT,
  buildApprovalBodyPreview,
  createDaemonBroker,
  isApprovalRoute,
  shouldAttachDaemonSessionHeader,
} = require('./daemon-broker');

function fakeResponse(status, body, headers = {}) {
  const responseHeaders = new Headers(headers);
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: status === 200 ? 'OK' : '',
    headers: responseHeaders,
    redirected: false,
    text: async () => body,
  };
}

test('broker pins loopback, ignores caller auth, and attaches the main-process token', async () => {
  const calls = [];
  const broker = createDaemonBroker({
    getToken: () => 'main-process-secret',
    fetchImpl: async (url, options) => {
      calls.push({ url, options });
      return fakeResponse(200, '{"status":"ok"}', { 'Content-Type': 'application/json' });
    },
  });

  const result = await broker.request({
    method: 'GET',
    path: '/api/status?scope=current',
    headers: { Authorization: 'Bearer renderer-supplied', 'X-Session-Token': 'renderer-token' },
  });

  assert.equal(result.status, 200);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].url, 'http://127.0.0.1:8765/api/status?scope=current');
  assert.equal(calls[0].options.headers['X-Session-Token'], 'main-process-secret');
  assert.equal(calls[0].options.headers.Authorization, undefined);
  assert.equal(calls[0].options.redirect, 'manual');
});

test('broker rejects unknown methods, paths, bodies, and redirects with typed errors', async () => {
  const broker = createDaemonBroker({
    getToken: () => 'token',
    fetchImpl: async () => fakeResponse(302, '', { Location: 'http://example.com/' }),
  });

  await assert.rejects(() => broker.request({ method: 'DELETE', path: '/api/status' }), { code: 'METHOD_NOT_ALLOWED' });
  await assert.rejects(() => broker.request({ method: 'GET', path: '/api/bridge/tools' }), { code: 'ROUTE_NOT_ALLOWED' });
  await assert.rejects(() => broker.request({ method: 'GET', path: '//example.com/api/status' }), { code: 'INVALID_REQUEST' });
  await assert.rejects(() => broker.request({ method: 'GET', path: '/api/status', body: '{}' }), { code: 'INVALID_REQUEST' });
  await assert.rejects(() => broker.request({ method: 'GET', path: '/api/status' }), { code: 'REDIRECT_REJECTED' });
});

test('broker allowlist contains approved reads and exact mutation routes', () => {
  for (const route of ['/health', '/api/status', '/api/files/list', '/api/files/read', '/api/github/issues/detail', '/api/memos/export']) {
    assert.equal(ALLOWED_ROUTES.GET.has(route), true, route);
  }
  for (const route of ['/api/enhancements', '/api/gui/color', '/api/conversations/ack', '/api/memos/save', '/api/memos/extract', '/api/github/issues/comment']) {
    assert.equal(ALLOWED_ROUTES.POST.has(route), true, route);
  }
  assert.equal(ALLOWED_ROUTES.GET.has('/api/files/write'), false);
  assert.equal(ALLOWED_ROUTES.POST.has('/api/files/write'), false);
});

test('session auth headers are limited to document and static resource loads', () => {
  assert.equal(shouldAttachDaemonSessionHeader({ resourceType: 'mainFrame' }), true);
  assert.equal(shouldAttachDaemonSessionHeader({ resourceType: 'script' }), true);
  assert.equal(shouldAttachDaemonSessionHeader({ resourceType: 'xhr' }), false);
  assert.equal(shouldAttachDaemonSessionHeader({ resourceType: 'fetch' }), false);
  assert.equal(shouldAttachDaemonSessionHeader({}), false);
});

test('preload exposes swiss.request and no raw-token getter', () => {
  const preload = fs.readFileSync(path.join(__dirname, 'preload.js'), 'utf8');
  assert.match(preload, /desktop:swiss-daemon-request/);
  assert.doesNotMatch(preload, /getSessionToken|desktop:get-session-token/);
  const main = fs.readFileSync(path.join(__dirname, 'main.js'), 'utf8');
  assert.doesNotMatch(main, /desktop:get-session-token/);
});


test('privileged mutations require a native approval once per request', async () => {
  let approvals = 0;
  let fetches = 0;
  const broker = createDaemonBroker({
    getToken: () => 'token',
    requestApproval: async ({ method, path, bodyPreview }) => {
      approvals += 1;
      assert.equal(method, 'POST');
      assert.equal(path, '/api/files/terminal');
      assert.match(bodyPreview, /\/tmp\/project/);
      return true;
    },
    fetchImpl: async () => {
      fetches += 1;
      return fakeResponse(200, '{"success":true}');
    },
  });

  await broker.request({ method: 'POST', path: '/api/files/terminal', body: '{"path":"/tmp/project"}' });
  await broker.request({ method: 'POST', path: '/api/files/terminal', body: '{"path":"/tmp/project"}' });
  assert.equal(approvals, 2, 'approval must not be cached across requests');
  assert.equal(fetches, 2);
});

test('denied native approval returns APPROVAL_DENIED without contacting the daemon', async () => {
  let fetches = 0;
  const broker = createDaemonBroker({
    getToken: () => 'token',
    requestApproval: async () => false,
    fetchImpl: async () => {
      fetches += 1;
      return fakeResponse(200, '{}');
    },
  });

  await assert.rejects(
    () => broker.request({ method: 'POST', path: '/api/files/delete', body: '{"path":"/tmp/project"}' }),
    { code: 'APPROVAL_DENIED', status: 403 },
  );
  assert.equal(fetches, 0);
});

test('approval arguments bind the exact request and bound/redact body values', async () => {
  let seen;
  const broker = createDaemonBroker({
    getToken: () => 'token',
    requestApproval: async (request) => {
      seen = request;
      return true;
    },
    fetchImpl: async () => fakeResponse(200, '{}'),
  });
  const fileBody = 'file-body-secret';
  const body = JSON.stringify({
    path: '/tmp/report.txt',
    content: fileBody,
    token: 'token-must-not-appear',
    note: 'x'.repeat(900),
  });

  await broker.request({ method: 'POST', path: '/api/files/write?source=ui', body });
  assert.equal(seen.method, 'POST');
  assert.equal(seen.path, '/api/files/write?source=ui');
  assert.match(seen.bodyPreview, /\/tmp\/report\.txt/);
  assert.match(seen.bodyPreview, /\[REDACTED\]/);
  assert.match(seen.bodyPreview, /chars\]/);
  assert.equal(seen.bodyPreview.includes(fileBody), false);
  assert.equal(seen.bodyPreview.includes('token-must-not-appear'), false);
  assert.ok(seen.bodyPreview.length <= 400, seen.bodyPreview);
  assert.equal(APPROVAL_BODY_PREVIEW_LIMIT, 400);
  assert.equal(buildApprovalBodyPreview('not-json-secret').includes('not-json-secret'), false);
});

test('approval routing covers machine-control and other non-allowlisted API mutations', () => {
  for (const route of [
    '/api/files/write',
    '/api/files/create',
    '/api/files/delete',
    '/api/files/rename',
    '/api/files/copy',
    '/api/files/move',
    '/api/files/reveal',
    '/api/files/terminal',
    '/api/files/open_ide',
    '/api/settings/factory-reset',
  ]) {
    assert.equal(isApprovalRoute('POST', route), true, route);
  }
  assert.equal(isApprovalRoute('GET', '/api/files/write'), false);
  assert.equal(isApprovalRoute('POST', '/api/enhancements'), false);
  assert.equal(isApprovalRoute('POST', '/not-daemon'), false);
});

test('production renderer and broker sources contain no daemon token leaks', () => {
  const files = [
    'electron/main.js',
    'electron/preload.js',
    'electron/daemon-broker.js',
    'frontend/src/api.ts',
    'frontend/src/App.tsx',
    'pkg/gui/styler.go',
    'pkg/custommodels/script.go',
    'pkg/enhancements/script.go',
    'pkg/plugins/auxiliary.go',
    'pkg/plugins/github_extension_script.go',
  ];
  const checks = [
    ['window global', /window\.__SWISS_SESSION_TOKEN__/],
    ['sessionStorage token write', /sessionStorage\.setItem\([^\n]*(?:__SWISS_SESSION_TOKEN__|swiss_session_token|antigravity_swiss_session_token|sessionToken)/i],
    ['token URL', /[?&](?:token|session_token)=/i],
    ['token log', /console\.(?:log|warn|error|debug|info)\([^\n]*(?:sessionToken|session_token|X-Session-Token)/i],
  ];
  for (const relative of files) {
    const source = fs.readFileSync(path.join(__dirname, '..', relative), 'utf8');
    for (const [label, pattern] of checks) {
      assert.doesNotMatch(source, pattern, `${relative}: ${label}`);
    }
  }
  const main = fs.readFileSync(path.join(__dirname, 'main.js'), 'utf8');
  assert.match(main, /dialog\.showMessageBox/);
  assert.match(main, /requestApproval:/);
});
