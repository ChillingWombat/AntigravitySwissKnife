package plugins

// SwissOnboardingAutoAcceptScript dismisses Antigravity's blocking "Security
// Notice & Data Use" dialog. The acceptance state is not persisted locally
// (not in localStorage, IndexedDB or app_storage.json), so the dialog comes
// back on every start and needs one click before the app is usable.
//
// It clicks "Accept and continue" only. The optional "help improve Antigravity
// by allowing Google to collect interaction data" checkbox is deliberately left
// untouched, so nothing is opted into on the user's behalf; they can enable it
// in settings if they want to.
const SwissOnboardingAutoAcceptScript = `(function swissAutoAcceptSecurityNotice() {
  try {
    var seen = 0;
    function tryAccept() {
      try {
        var buttons = document.querySelectorAll("button");
        for (var i = 0; i < buttons.length; i++) {
          var b = buttons[i];
          var text = (b.innerText || b.textContent || "").trim();
          if (!/^accept and continue$/i.test(text)) continue;
          if (b.disabled) continue;
          b.click();
          seen++;
          return true;
        }
      } catch (_) {}
      return false;
    }
    if (tryAccept()) return;
    var attempts = 0;
    var lastTry = Date.now();
    var throttled = function() {
      var now = Date.now();
      if (now - lastTry < 500) return false;
      lastTry = now;
      return tryAccept();
    };
    var timer = setInterval(function() {
      attempts++;
      if (tryAccept() || attempts > 240) clearInterval(timer);
    }, 500);
    try {
      var obs = new MutationObserver(function() { throttled(); });
      obs.observe(document.documentElement, { childList: true, subtree: true });
      setTimeout(function() { try { obs.disconnect(); } catch (_) {} }, 180000);
    } catch (_) {}
  } catch (_) {}
})();`
