'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const vm = require('node:vm');
const {
  ALLOWED_ROUTES,
  APPROVAL_BODY_PREVIEW_LIMIT,
  DAEMON_ORIGIN,
  LEGACY_SESSION_TOKEN_STORAGE_KEY,
  PRIVILEGED_ROUTES,
  buildApprovalBodyPreview,
  createDaemonBroker,
  installLegacySessionTokenCleanup,
  isApprovalRoute,
  purgeLegacySessionToken,
  shouldAttachDaemonSessionHeader,
  shouldCancelDaemonSubFrameNavigation,
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

test('broker allows read-only dashboard account, fleet, and rules routes', async () => {
  for (const path of ['/api/accounts', '/api/quota/fleet', '/api/rules']) {
    assert.ok(ALLOWED_ROUTES.GET.has(path), `${path} must be in the GET allowlist`);
  }

  const calls = [];
  const broker = createDaemonBroker({
    getToken: () => 'main-process-secret',
    fetchImpl: async (url) => {
      calls.push(url);
      return fakeResponse(200, '{}', { 'Content-Type': 'application/json' });
    },
  });

  for (const path of ['/api/accounts', '/api/quota/fleet', '/api/rules']) {
    const result = await broker.request({ method: 'GET', path });
    assert.equal(result.status, 200);
  }
  assert.deepEqual(calls, [
    'http://127.0.0.1:8765/api/accounts',
    'http://127.0.0.1:8765/api/quota/fleet',
    'http://127.0.0.1:8765/api/rules',
  ]);
});

test('broker allows the ACP mesh read used by the agent registry', async () => {
  const path = '/api/utilities/acp';
  assert.ok(ALLOWED_ROUTES.GET.has(path), `${path} must be in the GET allowlist`);

  const calls = [];
  const broker = createDaemonBroker({
    getToken: () => 'main-process-secret',
    fetchImpl: async (url) => {
      calls.push(url);
      return fakeResponse(200, '{"agents":[]}', { 'Content-Type': 'application/json' });
    },
  });

  const result = await broker.request({ method: 'GET', path });
  assert.equal(result.status, 200);
  assert.deepEqual(calls, ['http://127.0.0.1:8765/api/utilities/acp']);
});

test('broker rejects unknown methods, paths, bodies, and redirects with typed errors', async () => {
  let daemonCalls = 0;
  const broker = createDaemonBroker({
    getToken: () => 'token',
    fetchImpl: async () => {
      daemonCalls += 1;
      return fakeResponse(302, '', { Location: 'http://example.com/' });
    },
  });

  await assert.rejects(() => broker.request({ method: 'DELETE', path: '/api/status' }), { code: 'METHOD_NOT_ALLOWED' });
  await assert.rejects(() => broker.request({ method: 'GET', path: '/api/bridge/tools' }), { code: 'ROUTE_NOT_ALLOWED' });
  await assert.rejects(() => broker.request({ method: 'GET', path: '/api/files/unknown?path=/tmp' }), { code: 'ROUTE_NOT_ALLOWED' });
  await assert.rejects(() => broker.request({ method: 'POST', path: '/api/not-a-real-route' }), { code: 'ROUTE_NOT_ALLOWED' });
  assert.equal(daemonCalls, 0);
  await assert.rejects(() => broker.request({ method: 'GET', path: '//example.com/api/status' }), { code: 'INVALID_REQUEST' });
  await assert.rejects(() => broker.request({ method: 'GET', path: '/api/status', body: '{}' }), { code: 'INVALID_REQUEST' });
  await assert.rejects(() => broker.request({ method: 'GET', path: '/api/status' }), { code: 'REDIRECT_REJECTED' });
});

test('broker allowlist contains approved reads and exact mutation routes', () => {
  for (const route of ['/health', '/api/status', '/api/github/issues/detail', '/api/memos/export']) {
    assert.equal(ALLOWED_ROUTES.GET.has(route), true, route);
  }
  for (const route of ['/api/enhancements', '/api/gui/color', '/api/conversations/ack', '/api/memos/save', '/api/memos/extract', '/api/github/issues/comment']) {
    assert.equal(ALLOWED_ROUTES.POST.has(route), true, route);
  }
  assert.equal(ALLOWED_ROUTES.GET.has('/api/files/write'), false);
  assert.equal(ALLOWED_ROUTES.GET.has('/api/files/list'), false);
  assert.equal(ALLOWED_ROUTES.GET.has('/api/files/read'), false);
  assert.equal(ALLOWED_ROUTES.POST.has('/api/files/write'), false);
  assert.equal(PRIVILEGED_ROUTES.GET.has('/api/files/list'), true);
  assert.equal(PRIVILEGED_ROUTES.GET.has('/api/files/read'), true);
  assert.equal(PRIVILEGED_ROUTES.POST.has('/api/files/write'), true);
  assert.equal(PRIVILEGED_ROUTES.POST.has('/api/not-a-real-route'), false);
});

test('file reads and listings require approval and approve the resolved workspace path', async (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'swiss-broker-files-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const workspace = path.join(root, 'workspace');
  const file = path.join(workspace, 'report.txt');
  fs.mkdirSync(workspace);
  fs.writeFileSync(file, 'report');
  const configDir = path.join(root, 'config');
  fs.mkdirSync(configDir);
  const approvals = [];
  let tokenReads = 0;
  let fetches = 0;
  const broker = createDaemonBroker({
    configDir,
    getToken: () => {
      tokenReads += 1;
      return 'token';
    },
    requestApproval: async (request) => {
      approvals.push(request);
      return true;
    },
    fetchImpl: async () => {
      fetches += 1;
      return fakeResponse(200, '{"success":true}');
    },
  });

  await broker.request({ method: 'GET', path: `/api/files/read?path=${encodeURIComponent(file)}` });
  await broker.request({ method: 'GET', path: `/api/files/list?path=${encodeURIComponent(workspace)}` });

  assert.equal(approvals.length, 2);
  assert.equal(approvals[0].resolvedPath, fs.realpathSync(file));
  assert.equal(approvals[1].resolvedPath, fs.realpathSync(workspace));
  assert.equal(tokenReads, 2);
  assert.equal(fetches, 2);
});

test('protected daemon files are denied before token read or daemon call', async (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'swiss-broker-denied-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const configDir = path.join(root, 'config');
  fs.mkdirSync(configDir);
  const tokenPath = path.join(configDir, 'session.token');
  fs.writeFileSync(tokenPath, 'token');
  let tokenReads = 0;
  let fetches = 0;
  let denial;
  const broker = createDaemonBroker({
    configDir,
    getToken: () => {
      tokenReads += 1;
      return 'token';
    },
    requestApproval: async (request) => {
      denial = request;
      return true;
    },
    fetchImpl: async () => {
      fetches += 1;
      return fakeResponse(200, '{}');
    },
  });

  await assert.rejects(
    () => broker.request({ method: 'GET', path: `/api/files/read?path=${encodeURIComponent(tokenPath)}` }),
    (error) => {
      assert.equal(error.code, 'PATH_DENIED');
      assert.equal(error.status, 403);
      assert.match(error.message, /configuration directory/);
      return true;
    },
  );
  assert.equal(denial.resolvedPath, fs.realpathSync(tokenPath));
  assert.match(denial.denialReason, /configuration directory/);
  assert.equal(tokenReads, 0);
  assert.equal(fetches, 0);
});

test('direct and encoded traversal paths return PATH_DENIED before token read', async () => {
  let tokenReads = 0;
  let fetches = 0;
  const broker = createDaemonBroker({
    getToken: () => {
      tokenReads += 1;
      return 'token';
    },
    fetchImpl: async () => {
      fetches += 1;
      return fakeResponse(200, '{}');
    },
  });

  for (const requestPath of [
    '/api/files/read?path=%2Ftmp%2F..%2Fetc%2Fpasswd',
    '/api/files/list?path=%252Ftmp%252F%252E%252E%252Fetc',
    '/api/files/read?path=%2Ftmp%2F%00secret',
  ]) {
    await assert.rejects(() => broker.request({ method: 'GET', path: requestPath }), { code: 'PATH_DENIED' });
  }
  assert.equal(tokenReads, 0);
  assert.equal(fetches, 0);
});

test('five-, six-, and ten-layer encoded traversal paths return PATH_DENIED before approval or I/O', async () => {
  let tokenReads = 0;
  let daemonCalls = 0;
  let approvals = 0;
  const broker = createDaemonBroker({
    getToken: () => {
      tokenReads += 1;
      return 'token';
    },
    requestApproval: async () => {
      approvals += 1;
      return true;
    },
    fetchImpl: async () => {
      daemonCalls += 1;
      return fakeResponse(200, '{}');
    },
  });

  const encodeTraversal = (layers) => {
    let segment = '%2E%2E';
    for (let pass = 1; pass < layers; pass += 1) {
      segment = encodeURIComponent(segment);
    }
    return `/tmp/${segment}/etc/passwd`;
  };

  for (const layers of [5, 6, 10]) {
    await assert.rejects(
      () => broker.request({ method: 'GET', path: `/api/files/read?path=${encodeTraversal(layers)}` }),
      (error) => {
        assert.equal(error.code, 'PATH_DENIED', `${layers}-layer traversal`);
        assert.equal(error.status, 403, `${layers}-layer traversal status`);
        return true;
      },
    );
  }
  assert.equal(approvals, 0);
  assert.equal(tokenReads, 0);
  assert.equal(daemonCalls, 0);
});

test('session auth headers are limited to top-level daemon document and static loads', () => {
  assert.equal(shouldAttachDaemonSessionHeader({ resourceType: 'mainFrame', url: `${DAEMON_ORIGIN}/` }), true);
  assert.equal(shouldAttachDaemonSessionHeader({ resourceType: 'script', url: `${DAEMON_ORIGIN}/assets/app.js` }), true);
  assert.equal(shouldAttachDaemonSessionHeader({ resourceType: 'subFrame', url: `${DAEMON_ORIGIN}/frame` }), false);
  assert.equal(shouldAttachDaemonSessionHeader({ resourceType: 'subFrame', url: `${DAEMON_ORIGIN}/api/status` }), false);
  assert.equal(shouldAttachDaemonSessionHeader({ resourceType: 'xhr', url: `${DAEMON_ORIGIN}/health` }), false);
  assert.equal(shouldAttachDaemonSessionHeader({ resourceType: 'fetch', url: `${DAEMON_ORIGIN}/health` }), false);
  assert.equal(shouldAttachDaemonSessionHeader({ resourceType: 'mainFrame', url: `${DAEMON_ORIGIN}/api/status` }), false);
  assert.equal(shouldAttachDaemonSessionHeader({ resourceType: 'mainFrame', url: 'https://example.com/' }), false);
  assert.equal(shouldAttachDaemonSessionHeader({
    resourceType: 'script',
    url: `${DAEMON_ORIGIN}/assets/app.js`,
    frame: { parent: {} },
  }), false);
  assert.equal(shouldAttachDaemonSessionHeader({}), false);
});

test('daemon API subframe navigations are cancelled while ordinary frames are not', () => {
  assert.equal(shouldCancelDaemonSubFrameNavigation({
    resourceType: 'subFrame',
    url: `${DAEMON_ORIGIN}/api/status`,
  }), true);
  assert.equal(shouldCancelDaemonSubFrameNavigation({
    resourceType: 'subFrame',
    url: `${DAEMON_ORIGIN}/frame`,
  }), false);
  assert.equal(shouldCancelDaemonSubFrameNavigation({
    resourceType: 'mainFrame',
    url: `${DAEMON_ORIGIN}/api/status`,
  }), false);
  assert.equal(shouldCancelDaemonSubFrameNavigation({
    resourceType: 'subFrame',
    url: `${DAEMON_ORIGIN}/apiary`,
  }), false);
});

test('startup cleanup script removes the legacy token from both web storages', () => {
  const localStorage = new Map([[LEGACY_SESSION_TOKEN_STORAGE_KEY, 'legacy-local']]);
  const sessionStorage = new Map([[LEGACY_SESSION_TOKEN_STORAGE_KEY, 'legacy-session']]);
  const window = {
    localStorage: { removeItem: (key) => localStorage.delete(key) },
    sessionStorage: { removeItem: (key) => sessionStorage.delete(key) },
  };
  let cleanupScript = '';
  const webContents = {
    executeJavaScript: async (script) => {
      cleanupScript = script;
      vm.runInNewContext(script, { window });
    },
  };

  assert.equal(purgeLegacySessionToken(webContents), true);
  assert.equal(localStorage.has(LEGACY_SESSION_TOKEN_STORAGE_KEY), false);
  assert.equal(sessionStorage.has(LEGACY_SESSION_TOKEN_STORAGE_KEY), false);
  assert.doesNotMatch(cleanupScript, /console\./);
});

test('startup cleanup is registered for every webContents at did-finish-load', () => {
  const localStorage = new Map([[LEGACY_SESSION_TOKEN_STORAGE_KEY, 'legacy-local']]);
  const sessionStorage = new Map([[LEGACY_SESSION_TOKEN_STORAGE_KEY, 'legacy-session']]);
  const window = {
    localStorage: { removeItem: (key) => localStorage.delete(key) },
    sessionStorage: { removeItem: (key) => sessionStorage.delete(key) },
  };
  let createdListener;
  let finishListener;
  const webContents = {
    isLoading: () => true,
    once: (event, listener) => {
      assert.equal(event, 'did-finish-load');
      finishListener = listener;
    },
    executeJavaScript: async (script) => {
      vm.runInNewContext(script, { window });
    },
  };

  assert.equal(installLegacySessionTokenCleanup({
    on: (event, listener) => {
      assert.equal(event, 'web-contents-created');
      createdListener = listener;
    },
  }), true);
  createdListener({}, webContents);
  finishListener();
  assert.equal(localStorage.has(LEGACY_SESSION_TOKEN_STORAGE_KEY), false);
  assert.equal(sessionStorage.has(LEGACY_SESSION_TOKEN_STORAGE_KEY), false);
});

test('preload exposes swiss.request and no raw-token getter', () => {
  const preload = fs.readFileSync(path.join(__dirname, 'preload.js'), 'utf8');
  assert.match(preload, /desktop:swiss-daemon-request/);
  assert.doesNotMatch(preload, /getSessionToken|desktop:get-session-token/);
  const main = fs.readFileSync(path.join(__dirname, 'main.js'), 'utf8');
  assert.doesNotMatch(main, /desktop:get-session-token/);
  assert.match(main, /installLegacySessionTokenCleanup\(app\)/);
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
  assert.equal(isApprovalRoute('GET', '/api/files/read'), true);
  assert.equal(isApprovalRoute('GET', '/api/files/list'), true);
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
