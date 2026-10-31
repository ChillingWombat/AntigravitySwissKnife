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

func TestConversationIdentity_ExplicitNavigationIsRouteOwned(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("node not available: %v", err)
	}
	driver := conversationIdentityHelpers(t) + `
const source = { pathname: "/c/29d106f9-d43c-4cc5-9f9f-971c885d4743", search: "?tab=browser", href: "/c/29d106f9-d43c-4cc5-9f9f-971c885d4743?tab=browser" };
const target = { pathname: "/c/281241ba-17ff-4904-8305-031db425f416", search: "?section=new", href: "/c/281241ba-17ff-4904-8305-031db425f416?section=new" };
function mockTarget(selectorMatch) {
  return { closest: (selector) => selectorMatch(selector) ? { setAttribute: () => {} } : null };
}
const anchor = { href: target.href, getAttribute: () => target.href };
const newChat = mockTarget(selector => selector.includes("new-conversation-button"));
const sidebar = { closest: (selector) => selector === "a[href]" ? anchor : (selector.includes("conversation-row-sidebar") ? { setAttribute: () => {} } : null) };
const ring = mockTarget(() => false);
if (!isExplicitConversationNavigationElement(newChat) || !isExplicitConversationNavigationElement(sidebar)) {
  throw new Error("new-chat and sidebar clicks are explicit navigation");
}
if (isExplicitConversationNavigationElement(ring)) {
  throw new Error("unrelated clicks must not fence host navigation");
}
const intent = createExplicitConversationNavigationIntent(sidebar, source, 5000);
if (intent.kind !== "conversation" || intent.targetConversationId !== "281241ba-17ff-4904-8305-031db425f416") {
  throw new Error("sidebar intent must retain the explicit target conversation: " + JSON.stringify(intent));
}
if (!bindExplicitConversationNavigationIntent(intent, target) || intent.pending || intent.route !== target.href) {
  throw new Error("sidebar intent must bind to the acknowledged target route: " + JSON.stringify(intent));
}
if (!isExplicitConversationNavigationActive(intent, target)) {
  throw new Error("route-owned intent must survive an error/cancel delay without expiring");
}
const unrelated = { pathname: "/c/aaaaaaaa-1111-2222-3333-444444444444", search: "", href: "/c/aaaaaaaa-1111-2222-3333-444444444444" };
if (isExplicitConversationNavigationActive(intent, unrelated)) {
  throw new Error("route-owned intent must not fence an unrelated later route");
}
const history = createExplicitHistoryNavigationIntent(target, 40000);
if (history.kind !== "history" || history.pending || !isExplicitConversationNavigationActive(history, target)) {
  throw new Error("trusted history traversal must bind immediately to the current route: " + JSON.stringify(history));
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
