package rendererbroker

// FetchScript installs the Electron daemon broker in an injected renderer.
// It is idempotent and falls back to native fetch outside Electron.
const FetchScript = `(function installSwissDaemonFetchBroker() {
  if (typeof window === 'undefined' || window.__swissAuthFetchPatched) return;
  window.__swissAuthFetchPatched = true;
  const nativeFetch = typeof window.fetch === 'function' ? window.fetch.bind(window) : null;
  if (!nativeFetch) return;
  const daemonOrigin = 'http://127.0.0.1:8765';

  window.fetch = async function(input, init) {
    const options = init || {};
    const rawUrl = typeof input === 'string' ? input : (input && input.url);
    let url = null;
    try {
      url = new URL(rawUrl, (window.location && window.location.href) || daemonOrigin);
    } catch (_) {
      return nativeFetch(input, options);
    }
    if (url.origin !== daemonOrigin) return nativeFetch(input, options);

    const broker = window.swiss;
    if (!broker || typeof broker.request !== 'function') {
      // Browser/CDP contexts without the Electron preload keep native fetch.
      return nativeFetch(input, options);
    }

    const method = String(options.method || (input && input.method) || 'GET').toUpperCase();
    let body = typeof options.body === 'string' ? options.body : undefined;
    if (method !== 'GET' && body === undefined && input && typeof input.clone === 'function' && typeof input.text === 'function') {
      try { body = await input.clone().text(); } catch (_) {}
    }
    const result = await broker.request({ method, path: url.pathname + url.search, body });
    if (result && result.error) {
      const error = new Error(result.error.message || 'Swiss daemon broker rejected the request');
      error.code = result.error.code || 'DAEMON_BROKER_ERROR';
      throw error;
    }
    return new Response(result && typeof result.body === 'string' ? result.body : '', {
      status: result && Number.isInteger(result.status) ? result.status : 500,
      statusText: (result && result.statusText) || '',
      headers: (result && result.headers) || {},
    });
  };
})();`
