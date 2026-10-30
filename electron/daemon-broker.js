'use strict';

const DAEMON_ORIGIN = 'http://127.0.0.1:8765';
const DEFAULT_TIMEOUT_MS = 15000;
const MAX_TIMEOUT_MS = 60000;
const DAEMON_SESSION_RESOURCE_TYPES = new Set([
  'mainFrame',
  'subFrame',
  'stylesheet',
  'script',
  'image',
  'font',
  'media',
]);

const GET_PATHS = Object.freeze([
  '/health',
  '/api/status',
  '/api/gui/config',
  '/api/gui/projects',
  '/api/gui/conversations/pruned',
  '/api/enhancements',
  '/api/custom_models',
  '/api/custom_models/active',
  '/api/files/list',
  '/api/files/read',
  '/api/files/ide/config',
  '/api/memos',
  '/api/memos/config',
  '/api/memos/export',
  '/api/tokens/chat-metrics',
  '/api/github/repo',
  '/api/github/issues',
  '/api/github/prs',
  '/api/github/agent-tasks',
  '/api/github/kanban',
  '/api/github/issues/detail',
]);

const POST_PATHS = Object.freeze([
  '/api/enhancements',
  '/api/gui/config',
  '/api/gui/color',
  '/api/gui/color/reset',
  '/api/gui/color/delete',
  '/api/gui/projects/archive',
  '/api/conversations/ack',
  '/api/memos/save',
  '/api/memos/delete',
  '/api/memos/extract',
  '/api/memos/export',
  '/api/memos/config',
  '/api/custom_models',
  '/api/custom_models/active',
  '/api/custom_models/thinking_level',
  '/api/github/agent-tasks/label',
  '/api/github/agent-tasks/bind',
  '/api/github/issues/update',
  '/api/github/issues/comment',
  '/api/github/kanban/move',
]);

const ALLOWED_ROUTES = Object.freeze({
  GET: new Set(GET_PATHS),
  POST: new Set(POST_PATHS),
});

const APPROVAL_BODY_PREVIEW_LIMIT = 400;
const APPROVAL_PREVIEW_MAX_DEPTH = 3;
const APPROVAL_PREVIEW_MAX_KEYS = 12;
const SENSITIVE_BODY_KEY = /(token|secret|password|passwd|authorization|api[_-]?key|credential|cookie)/i;
const OPAQUE_BODY_KEY = /^(content|body|data|text|value)$/i;
const SECRET_IN_TEXT = /(bearer\s+|token|secret|password|api[_-]?key)/i;

function isApprovalRoute(method, pathname) {
  return method === 'POST' && pathname.startsWith('/api/') && pathname.length > 5 && !ALLOWED_ROUTES.POST.has(pathname);
}

function previewBodyValue(value, depth = 0) {
  if (value === null || typeof value === 'number' || typeof value === 'boolean') return value;
  if (typeof value === 'string') {
    if (SECRET_IN_TEXT.test(value)) return '[REDACTED]';
    return value.length > 80 ? `${value.slice(0, 77)}...` : value;
  }
  if (Array.isArray(value)) {
    const preview = value.slice(0, 4).map((item) => previewBodyValue(item, depth + 1));
    if (value.length > 4) preview.push(`[${value.length - 4} more items]`);
    return preview;
  }
  if (typeof value === 'object') {
    if (depth >= APPROVAL_PREVIEW_MAX_DEPTH) return '[nested object]';
    const preview = Object.create(null);
    const entries = Object.entries(value);
    for (const [key, item] of entries.slice(0, APPROVAL_PREVIEW_MAX_KEYS)) {
      if (SENSITIVE_BODY_KEY.test(key)) {
        preview[key] = '[REDACTED]';
      } else if (OPAQUE_BODY_KEY.test(key)) {
        preview[key] = `[${String(item ?? '').length} chars]`;
      } else {
        preview[key] = previewBodyValue(item, depth + 1);
      }
    }
    if (entries.length > APPROVAL_PREVIEW_MAX_KEYS) {
      preview.more = `[${entries.length - APPROVAL_PREVIEW_MAX_KEYS} more fields]`;
    }
    return preview;
  }
  return '[unsupported]';
}

function buildApprovalBodyPreview(body) {
  if (body === undefined) return 'No request body';
  const bytes = Buffer.byteLength(body, 'utf8');
  let parsed;
  try {
    parsed = JSON.parse(body);
  } catch {
    return `Non-JSON body omitted (${bytes} bytes)`;
  }
  let summary;
  try {
    summary = JSON.stringify(previewBodyValue(parsed));
  } catch {
    return `JSON body omitted (${bytes} bytes)`;
  }
  const suffix = ` (${bytes} bytes)`;
  const maxSummary = APPROVAL_BODY_PREVIEW_LIMIT - suffix.length;
  const bounded = summary.length > maxSummary
    ? `${summary.slice(0, Math.max(0, maxSummary - 3))}...`
    : summary;
  return `${bounded}${suffix}`;
}

class DaemonBrokerError extends Error {
  constructor(code, message, status = 400) {
    super(message);
    this.name = 'DaemonBrokerError';
    this.code = code;
    this.status = status;
  }
}

function shouldAttachDaemonSessionHeader(details) {
  return Boolean(details && DAEMON_SESSION_RESOURCE_TYPES.has(details.resourceType));
}

function invalid(message) {
  throw new DaemonBrokerError('INVALID_REQUEST', message);
}

function validateRequest(request, defaultTimeoutMs) {
  if (!request || typeof request !== 'object' || Array.isArray(request)) {
    invalid('Daemon request must be an object');
  }

  const method = typeof request.method === 'string' ? request.method.toUpperCase() : '';
  if (method !== 'GET' && method !== 'POST') {
    throw new DaemonBrokerError('METHOD_NOT_ALLOWED', 'Only GET and POST daemon requests are allowed', 405);
  }

  const rawPath = request.path;
  if (typeof rawPath !== 'string' || !rawPath.startsWith('/') || rawPath.startsWith('//') || rawPath.includes('\\') || rawPath.includes('#')) {
    invalid('Daemon request path must be a relative loopback path');
  }

  let url;
  try {
    url = new URL(rawPath, DAEMON_ORIGIN);
  } catch {
    invalid('Daemon request path is invalid');
  }
  if (url.origin !== DAEMON_ORIGIN) {
    invalid('Daemon request must stay on the fixed loopback origin');
  }
  const requiresApproval = isApprovalRoute(method, url.pathname);
  if (!ALLOWED_ROUTES[method].has(url.pathname) && !requiresApproval) {
    throw new DaemonBrokerError('ROUTE_NOT_ALLOWED', 'Daemon route is not allowlisted', 403);
  }

  let body;
  if (request.body !== undefined) {
    if (typeof request.body !== 'string') {
      invalid('Daemon request body must be a string');
    }
    body = request.body;
  }
  if (method === 'GET' && body !== undefined) {
    invalid('GET daemon requests cannot include a body');
  }

  let timeoutMs = request.timeoutMs === undefined ? defaultTimeoutMs : request.timeoutMs;
  if (!Number.isInteger(timeoutMs) || timeoutMs < 1 || timeoutMs > MAX_TIMEOUT_MS) {
    invalid(`Daemon timeoutMs must be an integer between 1 and ${MAX_TIMEOUT_MS}`);
  }

  return { method, url, path: rawPath, body, timeoutMs, requiresApproval };
}

function createDaemonBroker(options = {}) {
  const fetchImpl = options.fetchImpl || globalThis.fetch;
  const getToken = options.getToken;
  const requestApproval = options.requestApproval;
  const defaultTimeoutMs = options.defaultTimeoutMs || DEFAULT_TIMEOUT_MS;
  if (typeof fetchImpl !== 'function') {
    throw new TypeError('A fetch implementation is required');
  }
  if (typeof getToken !== 'function') {
    throw new TypeError('A daemon token provider is required');
  }

  async function request(input) {
    const { method, url, path, body, timeoutMs, requiresApproval } = validateRequest(input, defaultTimeoutMs);
    const providedToken = getToken();
    const token = typeof providedToken === 'string' ? providedToken.trim() : '';
    if (!token) {
      throw new DaemonBrokerError('DAEMON_AUTH_UNAVAILABLE', 'Daemon authentication is unavailable', 503);
    }

    if (requiresApproval) {
      if (typeof requestApproval !== 'function') {
        throw new DaemonBrokerError('APPROVAL_UNAVAILABLE', 'Native approval is unavailable', 503);
      }
      let approved;
      try {
        approved = await requestApproval({
          method,
          path,
          bodyPreview: buildApprovalBodyPreview(body),
        });
      } catch {
        throw new DaemonBrokerError('APPROVAL_UNAVAILABLE', 'Native approval could not be shown', 503);
      }
      if (approved !== true) {
        throw new DaemonBrokerError('APPROVAL_DENIED', 'The user denied this machine-control request', 403);
      }
    }

    const headers = { Accept: 'application/json' };
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    headers['X-Session-Token'] = token;

    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);
    let response;
    try {
      response = await fetchImpl(url.toString(), {
        method,
        headers,
        body,
        redirect: 'manual',
        signal: controller.signal,
      });
    } catch (error) {
      if (controller.signal.aborted) {
        throw new DaemonBrokerError('DAEMON_TIMEOUT', `Daemon request timed out after ${timeoutMs}ms`, 504);
      }
      throw new DaemonBrokerError('DAEMON_UNAVAILABLE', error && error.message ? error.message : 'Daemon request failed', 502);
    } finally {
      clearTimeout(timer);
    }

    if (response.redirected || (response.status >= 300 && response.status < 400) || response.headers.has('location')) {
      throw new DaemonBrokerError('REDIRECT_REJECTED', 'Daemon redirects are not allowed', 502);
    }

    return {
      ok: response.ok,
      status: response.status,
      statusText: response.statusText || '',
      headers: Object.fromEntries(response.headers.entries()),
      body: await response.text(),
    };
  }

  return { request };
}

module.exports = {
  ALLOWED_ROUTES,
  APPROVAL_BODY_PREVIEW_LIMIT,
  DAEMON_ORIGIN,
  DaemonBrokerError,
  buildApprovalBodyPreview,
  createDaemonBroker,
  isApprovalRoute,
  shouldAttachDaemonSessionHeader,
  validateRequest,
};
