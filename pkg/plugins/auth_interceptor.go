package plugins

// SwissAuthInterceptorScript patches window.fetch so every daemon-origin
// request routes through window.swiss.request (when available).
// Idempotent; scoped to the daemon origin only.
const SwissAuthInterceptorScript = `(function installSwissAuthInterceptor() {
  try {
    if (window.__swissAuthFetchPatched) return;
    window.__swissAuthFetchPatched = true;
    var apiOrigin = "http://127.0.0.1:8765";
    if (!window.fetch) return;
    var origFetch = window.fetch.bind(window);
    window.fetch = async function(input, init) {
      try {
        var urlStr = "";
        if (typeof input === "string") urlStr = input;
        else if (input && typeof input.url === "string") urlStr = input.url;
        if (urlStr.indexOf(apiOrigin) === 0) {
          if (window.swiss && typeof window.swiss.request === "function") {
            var opts = init || {};
            var method = String(opts.method || (input && input.method) || "GET").toUpperCase();
            var parsed = new URL(urlStr);
            var reqPath = parsed.pathname + parsed.search;
            var body = typeof opts.body === "string" ? opts.body : undefined;
            var res = await window.swiss.request({ method: method, path: reqPath, body: body });
            if (res && res.error) {
              var err = new Error(res.error.message || "Broker error");
              err.code = res.error.code;
              throw err;
            }
            return new Response(res && typeof res.body === "string" ? res.body : "", {
              status: res && Number.isInteger(res.status) ? res.status : 200,
              statusText: (res && res.statusText) || "OK",
              headers: (res && res.headers) || { "Content-Type": "application/json" }
            });
          }
        }
      } catch (_) {}
      return origFetch.apply(window, arguments);
    };
  } catch (_) {}
})();`
