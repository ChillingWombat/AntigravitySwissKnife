'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const vm = require('node:vm');
const PROJECT_ROUTE_PATHS = require('./daemon-route-allowlist');
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

test('every registered project method-and-path pair is exact-path allowlisted', () => {
  const routeCount = PROJECT_ROUTE_PATHS.GET.length + PROJECT_ROUTE_PATHS.POST.length + PROJECT_ROUTE_PATHS.DELETE.length;
  assert.ok(routeCount >= 200, `expected the full route surface, got ${routeCount}`);
  for (const method of ['GET', 'POST', 'DELETE']) {
    for (const route of PROJECT_ROUTE_PATHS[method]) {
      assert.equal(ALLOWED_ROUTES[method].has(route), true, `${method} ${route}`);
      assert.equal(PRIVILEGED_ROUTES[method].has(route), false, `${method} ${route}`);
      assert.equal(isApprovalRoute(method, route), false, `${method} ${route}`);
    }
    assert.equal(PRIVILEGED_ROUTES[method].size, 0, `${method} approval set must be empty`);
  }
  assert.equal(ALLOWED_ROUTES.GET.has('/api/not-a-real-route'), false);
});

test('all registered project routes execute without invoking native approval', async (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'swiss-broker-routes-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const file = path.join(root, 'report.txt');
  fs.writeFileSync(file, 'report');
  const fileQuery = `?path=${encodeURIComponent(file)}`;
  let fetches = 0;
  const broker = createDaemonBroker({
    getToken: () => 'main-process-secret',
    requestApproval: async () => {
      throw new Error('native approval must not be requested for registered project routes');
    },
    fetchImpl: async () => {
      fetches += 1;
      return fakeResponse(200, '{}', { 'Content-Type': 'application/json' });
    },
  });

  for (const method of ['GET', 'POST', 'DELETE']) {
    for (const route of PROJECT_ROUTE_PATHS[method]) {
      const requestPath = route === '/api/files/list' || route === '/api/files/read'
        ? `${route}${fileQuery}`
        : route;
      const result = await broker.request({
        method,
        path: requestPath,
        body: method === 'POST' ? '{}' : undefined,
      });
      assert.equal(result.status, 200, `${method} ${requestPath}`);
    }
  }
  assert.equal(
    fetches,
    PROJECT_ROUTE_PATHS.GET.length + PROJECT_ROUTE_PATHS.POST.length + PROJECT_ROUTE_PATHS.DELETE.length,
  );
});

test('file read and list routes remain direct while path jailing stays enforced', async (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'swiss-broker-files-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const workspace = path.join(root, 'workspace');
  const file = path.join(workspace, 'report.txt');
  const configDir = path.join(root, 'config');
  fs.mkdirSync(workspace);
  fs.mkdirSync(configDir);
  fs.writeFileSync(file, 'report');
  let tokenReads = 0;
  let fetches = 0;
  const broker = createDaemonBroker({
    configDir,
    getToken: () => {
      tokenReads += 1;
      return 'token';
    },
    requestApproval: async () => {
      throw new Error('file routes must not request native approval');
    },
    fetchImpl: async () => {
      fetches += 1;
      return fakeResponse(200, '{"success":true}');
    },
  });

  await broker.request({ method: 'GET', path: `/api/files/read?path=${encodeURIComponent(file)}` });
  await broker.request({ method: 'GET', path: `/api/files/list?path=${encodeURIComponent(workspace)}` });
  assert.equal(tokenReads, 2);
  assert.equal(fetches, 2);
});

test('protected daemon files and traversal paths are denied before token read or daemon call', async (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'swiss-broker-denied-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const configDir = path.join(root, 'config');
  fs.mkdirSync(configDir);
  const tokenPath = path.join(configDir, 'session.token');
  fs.writeFileSync(tokenPath, 'token');
  let tokenReads = 0;
  let fetches = 0;
  const broker = createDaemonBroker({
    configDir,
    getToken: () => {
      tokenReads += 1;
      return 'token';
    },
    requestApproval: async () => {
      throw new Error('path checks must run before approval');
    },
    fetchImpl: async () => {
      fetches += 1;
      return fakeResponse(200, '{}');
    },
  });

  for (const requestPath of [
    `/api/files/read?path=${encodeURIComponent(tokenPath)}`,
    '/api/files/read?path=%2Ftmp%2F..%2Fetc%2Fpasswd',
    '/api/files/list?path=%252Ftmp%252F%252E%252E%252Fetc',
    '/api/files/read?path=%2Ftmp%2F%00secret',
  ]) {
    await assert.rejects(() => broker.request({ method: 'GET', path: requestPath }), (error) => {
      assert.equal(error.code, 'PATH_DENIED');
      assert.equal(error.status, 403);
      return true;
    });
  }
  assert.equal(tokenReads, 0);
  assert.equal(fetches, 0);
});

test('repeatedly encoded traversal paths are denied before token read or I/O', async () => {
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
  assert.equal(tokenReads, 0);
  assert.equal(fetches, 0);
});

test('unknown routes, invalid paths, invalid bodies, and redirects fail closed', async () => {
  let unknownRouteFetches = 0;
  const broker = createDaemonBroker({
    getToken: () => 'token',
    requestApproval: async () => {
      throw new Error('unknown routes must never reach approval');
    },
    fetchImpl: async (url) => {
      if (url.endsWith('/api/status')) {
        return fakeResponse(302, '', { Location: 'http://example.com/' });
      }
      unknownRouteFetches += 1;
      return fakeResponse(200, '{}');
    },
  });

  await assert.rejects(() => broker.request({ method: 'PUT', path: '/api/status' }), { code: 'METHOD_NOT_ALLOWED' });
  for (const method of ['GET', 'POST', 'DELETE']) {
    await assert.rejects(
      () => broker.request({ method, path: '/api/not-a-real-route', body: method === 'POST' ? '{}' : undefined }),
      { code: 'ROUTE_NOT_ALLOWED' },
    );
  }
  await assert.rejects(() => broker.request({ method: 'GET', path: '/api/status/extra' }), { code: 'ROUTE_NOT_ALLOWED' });
  await assert.rejects(() => broker.request({ method: 'GET', path: '//example.com/api/status' }), { code: 'INVALID_REQUEST' });
  await assert.rejects(() => broker.request({ method: 'GET', path: '/api/status\\extra' }), { code: 'INVALID_REQUEST' });
  await assert.rejects(() => broker.request({ method: 'GET', path: '/api/status#fragment' }), { code: 'INVALID_REQUEST' });
  await assert.rejects(() => broker.request({ method: 'GET', path: '/api/status', body: '{}' }), { code: 'INVALID_REQUEST' });
  await assert.rejects(() => broker.request({ method: 'DELETE', path: '/api/custom_models?id=model-1', body: '{}' }), { code: 'INVALID_REQUEST' });
  await assert.rejects(() => broker.request({ method: 'GET', path: '/api/status' }), { code: 'REDIRECT_REJECTED' });
  assert.equal(unknownRouteFetches, 0);
});

test('sensitive reads and formerly privileged mutations are direct exact routes', async () => {
  const paths = [
    ['GET', '/api/accounts/export?email=person@example.com'],
    ['GET', '/api/totp?email=person@example.com'],
    ['GET', '/api/custom_models/key?id=model-1'],
    ['GET', '/api/bridge/remote/config'],
    ['GET', '/api/bridge/channels/config?unredacted=true'],
    ['POST', '/api/custom_models'],
    ['POST', '/api/custom_models/test'],
    ['POST', '/api/custom_models/fetch_quota'],
    ['POST', '/api/custom_models/refresh_quotas'],
    ['POST', '/api/custom_models/fetch_models'],
    ['POST', '/api/custom_models/security-audit'],
    ['POST', '/api/gui/projects/delete'],
    ['POST', '/api/system/factory_reset'],
    ['POST', '/api/files/write'],
    ['DELETE', '/api/custom_models?id=model-1'],
  ];
  const calls = [];
  const broker = createDaemonBroker({
    getToken: () => 'token',
    requestApproval: async () => {
      throw new Error('registered project actions must be direct');
    },
    fetchImpl: async (url, options) => {
      calls.push({ url, method: options.method });
      return fakeResponse(200, '{}');
    },
  });

  for (const [method, requestPath] of paths) {
    const result = await broker.request({
      method,
      path: requestPath,
      body: method === 'POST' ? '{}' : undefined,
    });
    assert.equal(result.status, 200, `${method} ${requestPath}`);
    assert.equal(isApprovalRoute(method, requestPath), false, `${method} ${requestPath}`);
  }
  assert.equal(calls.length, paths.length);
});

test('approval body preview still redacts sensitive values for future reuse', () => {
  const fileBody = 'file-body-secret';
  const body = JSON.stringify({
    path: '/tmp/report.txt',
    content: fileBody,
    token: 'token-must-not-appear',
    note: 'x'.repeat(900),
  });
  const preview = buildApprovalBodyPreview(body);

  assert.match(preview, /\/tmp\/report\.txt/);
  assert.match(preview, /\[REDACTED\]/);
  assert.match(preview, /chars\]/);
  assert.equal(preview.includes(fileBody), false);
  assert.equal(preview.includes('token-must-not-appear'), false);
  assert.ok(preview.length <= 400, preview);
  assert.equal(APPROVAL_BODY_PREVIEW_LIMIT, 400);
  assert.equal(buildApprovalBodyPreview('not-json-secret').includes('not-json-secret'), false);
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
  assert.equal(shouldCancelDaemonSubFrameNavigation({ resourceType: 'subFrame', url: `${DAEMON_ORIGIN}/api/status` }), true);
  assert.equal(shouldCancelDaemonSubFrameNavigation({ resourceType: 'subFrame', url: `${DAEMON_ORIGIN}/frame` }), false);
  assert.equal(shouldCancelDaemonSubFrameNavigation({ resourceType: 'mainFrame', url: `${DAEMON_ORIGIN}/api/status` }), false);
  assert.equal(shouldCancelDaemonSubFrameNavigation({ resourceType: 'subFrame', url: `${DAEMON_ORIGIN}/apiary` }), false);
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
});
