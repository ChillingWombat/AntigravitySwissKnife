'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { ALLOWED_ROUTES, createDaemonBroker, shouldAttachDaemonSessionHeader } = require('./daemon-broker');

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
