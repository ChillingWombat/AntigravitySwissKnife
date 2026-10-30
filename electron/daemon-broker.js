'use strict';

const fs = require('node:fs');
const os = require('node:os');
const nodePath = require('node:path');

const DAEMON_ORIGIN = 'http://127.0.0.1:8765';
const DEFAULT_TIMEOUT_MS = 15000;
const MAX_TIMEOUT_MS = 60000;
const DAEMON_SESSION_RESOURCE_TYPES = new Set([
  'mainFrame',
  'stylesheet',
  'script',
  'image',
  'font',
  'media',
]);

const GET_PATHS = Object.freeze([
  '/health',
  '/api/status',
  '/api/auth/status',
  '/api/accounts',
  '/api/accounts/scan',
  '/api/quota',
  '/api/quota/fleet',
  '/api/rules',
  '/api/models/available',
  '/api/surfaces',
  '/api/oauth/google/url',
  '/api/settings/password',
  '/api/settings/storage',
  '/api/settings/privacy',
  '/api/settings/runtime-mode',
  '/api/settings/theme',
  '/api/fingerprint',
  '/api/cache/scan',
  '/api/cache/config',
  '/api/vault/status',
  '/api/system/installations',
  '/api/system/app_release',
  '/api/custom_models',
  '/api/custom_models/active',
  '/api/custom_models/integration',
  '/api/custom_models/cli_status',
  '/api/custom_models/presets',
  '/api/gui/config',
  '/api/gui/projects',
  '/api/gui/projects/archived',
  '/api/gui/conversations/pruned',
  '/api/gui/desktop/status',
  '/api/enhancements',
  '/api/extensions/community',
  '/api/files/ide/config',
  '/api/memos',
  '/api/memos/config',
  '/api/memos/export',
  '/api/utilities/acp',
  '/api/tokens/summary',
  '/api/tokens/chat-metrics',
  '/api/tokens/pricing',
  '/api/github/repo',
  '/api/github/issues',
  '/api/github/issues/detail',
  '/api/github/prs',
  '/api/github/prs/detail',
  '/api/github/agent-tasks',
  '/api/github/context',
  '/api/github/kanban',
  '/api/bridge/status',
  '/api/bridge/http-allowlist',
  '/api/bridge/remote/status',
  '/api/bridge/channels/status',
  '/api/bridge/channels/config',
]);

const POST_PATHS = Object.freeze([
  '/api/quota/refresh',
  '/api/rules',
  '/api/rules/auto_switch',
  '/api/cache/config',
  '/api/settings/runtime-mode',
  '/api/settings/theme',
  '/api/oauth/google/cancel',
  '/api/system/check_updates',
  '/api/system/check_app_release',
  '/api/custom_models/active',
  '/api/custom_models/thinking_level',
  '/api/enhancements',
  '/api/utilities/acp/detect',
  '/api/utilities/acp/ping',
  '/api/gui/config',
  '/api/gui/color',
  '/api/gui/color/reset',
  '/api/gui/color/delete',
  '/api/gui/projects/archive',
  '/api/gui/projects/restore',
  '/api/gui/conversations/auto-archive',
  '/api/conversations/ack',
  '/api/memos/save',
  '/api/memos/delete',
  '/api/memos/extract',
  '/api/memos/export',
  '/api/memos/config',
]);

const PRIVILEGED_POST_PATHS = Object.freeze([
  '/api/accounts/import',
  '/api/accounts/batch-import',
  '/api/accounts/update',
  '/api/accounts/delete',
  '/api/switch',
  '/api/desktop/relaunch',
  '/api/desktop/launch',
  '/api/oauth/google/start',
  '/api/oauth/google/exchange',
  '/api/auth/unlock',
  '/api/settings/password',
  '/api/settings/storage',
  '/api/settings/app_path',
  '/api/settings/account_override',
  '/api/settings/cache_clear',
  '/api/settings/privacy',
  '/api/settings/diagnose-issue',
  '/api/system/factory_reset',
  '/api/system/app_release/settings',
  '/api/system/app_release/upgrade',
  '/api/totp',
  '/api/fingerprint',
  '/api/cache/prune',
  '/api/vault/sync',
  '/api/vault/toggle',
  '/api/custom_models',
  '/api/custom_models/integration',
  '/api/custom_models/launch',
  '/api/custom_models/cli_setup',
  '/api/custom_models/test',
  '/api/custom_models/fetch_quota',
  '/api/custom_models/refresh_quotas',
  '/api/custom_models/fetch_models',
  '/api/custom_models/security-audit',
  '/api/enhancements/apply',
  '/api/extensions/community',
  '/api/files/write',
  '/api/files/create',
  '/api/files/delete',
  '/api/files/rename',
  '/api/files/copy',
  '/api/files/move',
  '/api/files/reveal',
  '/api/files/terminal',
  '/api/files/open_ide',
  '/api/files/ide/config',
  '/api/gui/projects/delete',
  '/api/gui/projects/open_settings',
  '/api/gui/apply',
  '/api/gui/desktop/persistence',
  '/api/daemon/start',
  '/api/utilities/import',
  '/api/utilities/acp/sync-registry',
  '/api/utilities/acp/agents',
  '/api/utilities/acp/prompt',
  '/api/tokens/pricing',
  '/api/github/issues/create',
  '/api/github/issues/update',
  '/api/github/issues/comment',
  '/api/github/kanban/move',
  '/api/github/agent-tasks/label',
  '/api/github/agent-tasks/bind',
  '/api/bridge/http-allowlist',
  '/api/bridge/config',
  '/api/bridge/dispatch',
  '/api/bridge/test',
  '/api/bridge/tunnel/start',
  '/api/bridge/tunnel/stop',
  '/api/bridge/token/rotate',
  '/api/bridge/agent',
  '/api/bridge/mode',
  '/api/bridge/remote/config',
  '/api/bridge/remote/pin/rotate',
  '/api/bridge/remote/tunnel/start',
  '/api/bridge/remote/tunnel/stop',
  '/api/bridge/channels/config',
  '/api/bridge/channels/test',
]);

const PRIVILEGED_GET_PATHS = Object.freeze([
  '/api/accounts/export',
  '/api/totp',
  '/api/custom_models/key',
  '/api/files/list',
  '/api/files/read',
  '/api/utilities/import/scan',
  '/api/bridge/remote/config',
]);

const DELETE_PATHS = Object.freeze([]);

const PRIVILEGED_DELETE_PATHS = Object.freeze([
  '/api/custom_models',
  '/api/extensions/community',
  '/api/utilities/acp/agents',
  '/api/tokens/pricing',
]);

const ALLOWED_ROUTES = Object.freeze({
  GET: new Set(GET_PATHS),
  POST: new Set(POST_PATHS),
  DELETE: new Set(DELETE_PATHS),
});
const PRIVILEGED_ROUTES = Object.freeze({
  GET: new Set(PRIVILEGED_GET_PATHS),
  POST: new Set(PRIVILEGED_POST_PATHS),
  DELETE: new Set(PRIVILEGED_DELETE_PATHS),
});
const PRIVILEGED_QUERY_ROUTES = new Map([
  ['GET /api/bridge/channels/config', (params) => params.get('unredacted') === 'true'],
]);
const FILE_ACCESS_PATHS = new Set(['/api/files/list', '/api/files/read']);
const SENSITIVE_FILE_NAME = /^(?:session\.token|session\.pid|credentials(?:\..+)?)$/i;

const APPROVAL_BODY_PREVIEW_LIMIT = 400;
const APPROVAL_PREVIEW_MAX_DEPTH = 3;
const APPROVAL_PREVIEW_MAX_KEYS = 12;
const SENSITIVE_BODY_KEY = /(token|secret|password|passwd|authorization|api[_-]?key|credential|cookie)/i;
const OPAQUE_BODY_KEY = /^(content|body|data|text|value)$/i;
const SECRET_IN_TEXT = /(bearer\s+|token|secret|password|api[_-]?key)/i;

function isApprovalRoute(method, pathOrUrl) {
  const url = typeof pathOrUrl === 'string' ? new URL(pathOrUrl, DAEMON_ORIGIN) : pathOrUrl;
  if (PRIVILEGED_ROUTES[method] && PRIVILEGED_ROUTES[method].has(url.pathname)) {
    return true;
  }
  const queryRule = PRIVILEGED_QUERY_ROUTES.get(`${method} ${url.pathname}`);
  return Boolean(queryRule && queryRule(url.searchParams));
}

function getSwissConfigDir(platform = process.platform, env = process.env, home = os.homedir()) {
  if (env.ANTIGRAVITY_SWISS_CONFIG_DIR) return env.ANTIGRAVITY_SWISS_CONFIG_DIR;
  if (platform === 'win32') {
    return nodePath.join(env.APPDATA || nodePath.join(home, 'AppData', 'Roaming'), 'antigravity-swiss');
  }
  if (platform === 'darwin') {
    return nodePath.join(home, 'Library', 'Application Support', 'antigravity-swiss');
  }
  return nodePath.join(env.XDG_CONFIG_HOME || nodePath.join(home, '.config'), 'antigravity-swiss');
}

const MAX_PATH_DECODE_PASSES = 10;

function decodePathValue(value) {
  let decoded = value;
  for (let pass = 0; pass < MAX_PATH_DECODE_PASSES; pass += 1) {
    let next;
    try {
      next = decodeURIComponent(decoded);
    } catch {
      throw new DaemonBrokerError('PATH_DENIED', 'File path contains malformed percent encoding', 403);
    }
    if (next === decoded) return decoded;
    decoded = next;
  }
  throw new DaemonBrokerError(
    'PATH_DENIED',
    `File path remains encoded after ${MAX_PATH_DECODE_PASSES} decoding passes`,
    403,
  );
}

function containsPathTraversal(value) {
  return value.replace(/\\/g, '/').split('/').includes('..');
}

function assertSafeFileQueryValue(value) {
  const decoded = decodePathValue(value);
  if (value.includes('\0') || decoded.includes('\0')) {
    throw new DaemonBrokerError('PATH_DENIED', 'File paths containing NUL bytes are blocked', 403);
  }
  if (containsPathTraversal(value) || containsPathTraversal(decoded)) {
    throw new DaemonBrokerError('PATH_DENIED', 'File paths containing traversal are blocked', 403);
  }
  return decoded;
}

function expandHomePath(value, home = os.homedir()) {
  if (value === '~') return home;
  if (value.startsWith('~/') || value.startsWith('~\\')) return nodePath.join(home, value.slice(2));
  return value;
}

function resolveFileSystemPath(value) {
  const lexical = nodePath.resolve(expandHomePath(value));
  try {
    const resolved = typeof fs.realpathSync.native === 'function'
      ? fs.realpathSync.native(lexical)
      : fs.realpathSync(lexical);
    return { lexical, resolved };
  } catch {
    return { lexical, resolved: lexical };
  }
}

function isPathWithin(candidate, root) {
  const relative = nodePath.relative(root, candidate);
  return relative === '' ||
    (relative !== '..' && !relative.startsWith(`..${nodePath.sep}`) && !nodePath.isAbsolute(relative));
}

function getSensitivePathReason(candidates, configDir) {
  const configPaths = resolveFileSystemPath(configDir);
  const configRoots = new Set([configPaths.lexical, configPaths.resolved]);
  for (const candidate of candidates) {
    for (const root of configRoots) {
      if (isPathWithin(candidate, root)) {
        return `Path is inside the Swiss daemon configuration directory (${configPaths.resolved})`;
      }
    }
    if (SENSITIVE_FILE_NAME.test(nodePath.basename(candidate))) {
      return `Path targets a protected Swiss daemon credential file (${nodePath.basename(candidate)})`;
    }
  }
  return '';
}

function validateFileAccessPath(url, configDir) {
  if (!FILE_ACCESS_PATHS.has(url.pathname)) return null;

  const decodedPaths = [];
  for (const [name, value] of url.searchParams.entries()) {
    const decoded = assertSafeFileQueryValue(value);
    if (name === 'path') decodedPaths.push(decoded);
  }
  if (decodedPaths.length !== 1 || !decodedPaths[0]) {
    throw new DaemonBrokerError('PATH_DENIED', 'Exactly one non-empty path query parameter is required', 403);
  }
  if (!nodePath.isAbsolute(expandHomePath(decodedPaths[0]))) {
    throw new DaemonBrokerError('PATH_DENIED', 'File paths must be absolute', 403);
  }

  const pathInfo = resolveFileSystemPath(decodedPaths[0]);
  const denialReason = getSensitivePathReason(
    [pathInfo.lexical, pathInfo.resolved],
    configDir || getSwissConfigDir(),
  );
  return { resolvedPath: pathInfo.resolved, denialReason };
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

function isDaemonApiUrl(rawUrl) {
  try {
    const url = new URL(rawUrl);
    return url.origin === DAEMON_ORIGIN && (url.pathname === '/api' || url.pathname.startsWith('/api/'));
  } catch {
    return false;
  }
}

function shouldAttachDaemonSessionHeader(details) {
  if (!details || !DAEMON_SESSION_RESOURCE_TYPES.has(details.resourceType)) return false;
  if (details.frame && details.frame.parent) return false;
  try {
    const url = new URL(details.url);
    return url.origin === DAEMON_ORIGIN && !isDaemonApiUrl(details.url);
  } catch {
    return false;
  }
}

function shouldCancelDaemonSubFrameNavigation(details) {
  return Boolean(details && details.resourceType === 'subFrame' && isDaemonApiUrl(details.url));
}

const LEGACY_SESSION_TOKEN_STORAGE_KEY = 'swiss_session_token';
const LEGACY_SESSION_TOKEN_CLEANUP_SCRIPT = `(() => {
  for (const storageName of ['localStorage', 'sessionStorage']) {
    try {
      window[storageName].removeItem(${JSON.stringify(LEGACY_SESSION_TOKEN_STORAGE_KEY)});
    } catch {}
  }
})();`;

function purgeLegacySessionToken(webContents) {
  if (!webContents || typeof webContents.executeJavaScript !== 'function') return false;
  try {
    const pending = webContents.executeJavaScript(LEGACY_SESSION_TOKEN_CLEANUP_SCRIPT, true);
    if (pending && typeof pending.catch === 'function') pending.catch(() => {});
    return true;
  } catch {
    return false;
  }
}

function registerLegacySessionTokenCleanup(webContents) {
  if (!webContents || typeof webContents.once !== 'function') return false;
  if (typeof webContents.isDestroyed === 'function' && webContents.isDestroyed()) return false;

  const cleanup = () => {
    purgeLegacySessionToken(webContents);
  };
  webContents.once('did-finish-load', cleanup);
  if (typeof webContents.isLoading === 'function' && !webContents.isLoading()) cleanup();
  return true;
}

function installLegacySessionTokenCleanup(app) {
  if (!app || typeof app.on !== 'function') return false;
  app.on('web-contents-created', (_event, webContents) => {
    registerLegacySessionTokenCleanup(webContents);
  });
  return true;
}

function invalid(message) {
  throw new DaemonBrokerError('INVALID_REQUEST', message);
}

function validateRequest(request, defaultTimeoutMs, configDir) {
  if (!request || typeof request !== 'object' || Array.isArray(request)) {
    invalid('Daemon request must be an object');
  }

  const method = typeof request.method === 'string' ? request.method.toUpperCase() : '';
  if (method !== 'GET' && method !== 'POST' && method !== 'DELETE') {
    throw new DaemonBrokerError('METHOD_NOT_ALLOWED', 'Only GET, POST, and DELETE daemon requests are allowed', 405);
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
  const requiresApproval = isApprovalRoute(method, url);
  const routeIsAllowed = ALLOWED_ROUTES[method].has(url.pathname) ||
    (PRIVILEGED_ROUTES[method] && PRIVILEGED_ROUTES[method].has(url.pathname));
  if (!routeIsAllowed) {
    throw new DaemonBrokerError('ROUTE_NOT_ALLOWED', 'Daemon route is not allowlisted', 403);
  }
  const fileAccess = validateFileAccessPath(url, configDir);

  let body;
  if (request.body !== undefined) {
    if (typeof request.body !== 'string') {
      invalid('Daemon request body must be a string');
    }
    body = request.body;
  }
  if (method !== 'POST' && body !== undefined) {
    invalid(`${method} daemon requests cannot include a body`);
  }

  let timeoutMs = request.timeoutMs === undefined ? defaultTimeoutMs : request.timeoutMs;
  if (!Number.isInteger(timeoutMs) || timeoutMs < 1 || timeoutMs > MAX_TIMEOUT_MS) {
    invalid(`Daemon timeoutMs must be an integer between 1 and ${MAX_TIMEOUT_MS}`);
  }

  return { method, url, path: rawPath, body, timeoutMs, requiresApproval, fileAccess };
}

function createDaemonBroker(options = {}) {
  const fetchImpl = options.fetchImpl || globalThis.fetch;
  const getToken = options.getToken;
  const requestApproval = options.requestApproval;
  const defaultTimeoutMs = options.defaultTimeoutMs || DEFAULT_TIMEOUT_MS;
  const configDir = options.configDir;
  if (typeof fetchImpl !== 'function') {
    throw new TypeError('A fetch implementation is required');
  }
  if (typeof getToken !== 'function') {
    throw new TypeError('A daemon token provider is required');
  }

  async function request(input) {
    const {
      method,
      url,
      path,
      body,
      timeoutMs,
      requiresApproval,
      fileAccess,
    } = validateRequest(input, defaultTimeoutMs, configDir);

    if (fileAccess && fileAccess.denialReason) {
      if (typeof requestApproval === 'function') {
        try {
          await requestApproval({
            method,
            path,
            bodyPreview: buildApprovalBodyPreview(body),
            resolvedPath: fileAccess.resolvedPath,
            denialReason: fileAccess.denialReason,
          });
        } catch {
          // PATH_DENIED remains authoritative even if the denial dialog cannot be shown.
        }
      }
      throw new DaemonBrokerError('PATH_DENIED', fileAccess.denialReason, 403);
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
          resolvedPath: fileAccess ? fileAccess.resolvedPath : undefined,
        });
      } catch {
        throw new DaemonBrokerError('APPROVAL_UNAVAILABLE', 'Native approval could not be shown', 503);
      }
      if (approved !== true) {
        throw new DaemonBrokerError('APPROVAL_DENIED', 'The user denied this machine-control request', 403);
      }
    }

    const providedToken = getToken();
    const token = typeof providedToken === 'string' ? providedToken.trim() : '';
    if (!token) {
      throw new DaemonBrokerError('DAEMON_AUTH_UNAVAILABLE', 'Daemon authentication is unavailable', 503);
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
  LEGACY_SESSION_TOKEN_STORAGE_KEY,
  PRIVILEGED_ROUTES,
  buildApprovalBodyPreview,
  createDaemonBroker,
  getSwissConfigDir,
  installLegacySessionTokenCleanup,
  isApprovalRoute,
  isDaemonApiUrl,
  purgeLegacySessionToken,
  registerLegacySessionTokenCleanup,
  shouldAttachDaemonSessionHeader,
  shouldCancelDaemonSubFrameNavigation,
  validateRequest,
};
