package enhancements

import (
	"os/exec"
	"strings"
	"testing"
)

func conversationIdentityHelpers(t *testing.T) string {
	t.Helper()
	script := GenerateEnhancementsScript(DefaultConfig())
	const start = "/* __SWISS_CONVERSATION_IDENTITY_START__ */"
	const end = "/* __SWISS_CONVERSATION_IDENTITY_END__ */"
	startAt := strings.Index(script, start)
	endAt := strings.Index(script, end)
	if startAt < 0 || endAt <= startAt {
		t.Fatalf("conversation identity helper block missing from generated script")
	}
	return script[startAt+len(start) : endAt]
}

func TestConversationIdentity_VisibleConversationWinsOverRoute(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("node not available: %v", err)
	}
	driver := conversationIdentityHelpers(t) + `
const staleRouteId = "29d106f9-d43c-4cc5-9f9f-971c885d4743";
const visibleId = "281241ba-17ff-4904-8305-031db425f416";
if (resolveConversationIdentity(visibleId, "/c/" + staleRouteId) !== visibleId) {
  throw new Error("visible conversation must win over the stale route");
}
if (resolveConversationIdentity(null, "/c/" + staleRouteId) !== staleRouteId) {
  throw new Error("route is the fallback only when no visible identity exists");
}
console.log("identity mismatch behavior OK");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(driver)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("identity helper driver failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "identity mismatch behavior OK") {
		t.Fatalf("driver did not reach assertions:\n%s", string(out))
	}
}

func TestConversationIdentity_SupersededMetricsAndFailedCleanup(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("node not available: %v", err)
	}
	driver := conversationIdentityHelpers(t) + `
const oldId = "29d106f9-d43c-4cc5-9f9f-971c885d4743";
const visibleId = "281241ba-17ff-4904-8305-031db425f416";
const requests = {};
const identity = { conversationId: null };
activateConversationIdentity(identity, requests, oldId);
const oldRequest = beginConversationMetricRequest(requests, oldId);
activateConversationIdentity(identity, requests, visibleId);
if (isCurrentConversationMetricRequest(requests, oldId, oldRequest)) {
  throw new Error("a response from the superseded identity must be rejected");
}
const currentRequest = beginConversationMetricRequest(requests, visibleId);
if (!isCurrentConversationMetricRequest(requests, visibleId, currentRequest)) {
  throw new Error("the current request generation must remain valid");
}
const tokens = {}; const windows = {};
tokens[visibleId] = 222465;
windows[visibleId] = { used_tokens: 222465, max_tokens: 256000 };
clearConversationMetricCache(tokens, windows, visibleId);
if (Object.prototype.hasOwnProperty.call(tokens, visibleId) || Object.prototype.hasOwnProperty.call(windows, visibleId)) {
  throw new Error("failed lookup must clear both metric caches for that identity");
}
console.log("metric generation behavior OK");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(driver)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("metric helper driver failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "metric generation behavior OK") {
		t.Fatalf("driver did not reach assertions:\n%s", string(out))
	}
}

func TestConversationIdentity_ExplicitNavigationSurvivesErrorDelay(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("node not available: %v", err)
	}
	driver := conversationIdentityHelpers(t) + `
function target(selectorMatch) {
  return { closest: (selector) => selectorMatch(selector) ? { setAttribute: () => {} } : null };
}
const newChat = target(selector => selector.includes("new-conversation-button"));
const sidebar = target(selector => selector.includes("conversation-row-sidebar"));
const ring = target(() => false);
if (!isExplicitConversationNavigationElement(newChat) || !isExplicitConversationNavigationElement(sidebar)) {
  throw new Error("new-chat and sidebar clicks are explicit navigation");
}
if (isExplicitConversationNavigationElement(ring)) {
  throw new Error("unrelated clicks must not fence host navigation");
}
const markedAt = 5000;
const intent = createExplicitConversationNavigationIntent(sidebar, "/c/old", markedAt);
if (!hasRecentExplicitConversationNavigation(intent, markedAt + 8000)) {
  throw new Error("explicit navigation must survive an error/cancel delay");
}
if (hasRecentExplicitConversationNavigation(intent, markedAt + 30001)) {
  throw new Error("stale navigation intent must not suppress revival forever");
}
console.log("explicit navigation behavior OK");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(driver)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("navigation helper driver failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "explicit navigation behavior OK") {
		t.Fatalf("driver did not reach assertions:\n%s", string(out))
	}
}
