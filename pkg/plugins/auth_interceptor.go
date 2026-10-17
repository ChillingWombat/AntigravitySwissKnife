package plugins

// SwissAuthInterceptorScript patches window.fetch so every daemon-origin
// request carries the current session token. Idempotent; scoped to the daemon
// origin only, so host-app requests are untouched.
const SwissAuthInterceptorScript = `(function installSwissAuthInterceptor() {
  try {
    if (window.__swissAuthFetchPatched) return;
    window.__swissAuthFetchPatched = true;
    var apiOrigin = "http://127.0.0.1:8765";
    var getToken = function() { return (typeof window !== "undefined" && window.__SWISS_SESSION_TOKEN__) || (typeof sessionStorage !== "undefined" && sessionStorage.getItem("antigravity_swiss_session_token")) || ""; };
    if (!window.fetch) return;
    var origFetch = window.fetch.bind(window);
    window.fetch = function(input, init) {
      try {
        var url = "";
        if (typeof input === "string") url = input;
        else if (input && typeof input.url === "string") url = input.url;
        if (url.indexOf(apiOrigin) === 0) {
          var token = getToken();
          if (token) {
            var merged = Object.assign({}, init || {});
            var headers = new Headers(merged.headers || undefined);
            if (!headers.has("X-Session-Token")) headers.set("X-Session-Token", token);
            merged.headers = headers;
            return origFetch(input, merged);
          }
        }
      } catch (_) {}
      return origFetch.apply(window, arguments);
    };
  } catch (_) {}
})();`
