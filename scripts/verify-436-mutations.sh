#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="${TMPDIR:-/tmp}/worker-436-mutation-$$"
mkdir -p "$TMP"
VERTEX="pkg/custommodels/vertex.go"
INJECTOR="pkg/gui/injector.go"
cp "$ROOT/$VERTEX" "$TMP/vertex.go"
cp "$ROOT/$INJECTOR" "$TMP/injector.go"

restore_mutant() {
  cp "$TMP/vertex.go" "$ROOT/$VERTEX"
  cp "$TMP/injector.go" "$ROOT/$INJECTOR"
}
trap restore_mutant EXIT

run_expected_failure() {
  local name="$1"
  shift
  if "$@" > "$TMP/$name.log" 2>&1; then
    printf 'MUTANT %s unexpectedly passed\n' "$name"
    cat "$TMP/$name.log"
    return 1
  fi
  cat "$TMP/$name.log"
  if grep -Eqi 'syntax error|SyntaxError|unexpected token|build failed|undefined:|cannot use|unknown field' "$TMP/$name.log"; then
    printf 'MUTANT %s failed outside behavioral assertions\n' "$name"
    return 1
  fi
}

require_log() {
  local name="$1"
  local assertion="$2"
  if ! grep -Fq "$assertion" "$TMP/$name.log"; then
    printf 'MUTANT %s did not reach assertion: %s\n' "$name" "$assertion"
    return 1
  fi
}

expect_behavioral_failure() {
  local name="$1"
  local assertion="$2"
  shift 2
  run_expected_failure "$name" "$@"
  require_log "$name" "$assertion"
  printf 'MUTANT %s rejected behaviorally: %s\n' "$name" "$assertion"
}

python3 - "$ROOT/$VERTEX" <<'PY'
from pathlib import Path
import sys
path = Path(sys.argv[1])
source = path.read_text()
needle = '\tidleTimer.Reset(idleTimeout)\n'
assert source.count(needle) == 1, 'post-header idle reset pattern missing'
path.write_text(source.replace(needle, '\t// mutation 436: post-header idle timer reset removed\n'))
PY
expect_behavioral_failure active-stream 'frames = 1, want five active frames plus terminal STOP' \
  go -C "$ROOT" test ./pkg/custommodels -run '^TestProxyGCPActiveStreamResetsIdleGuard$' -count=1
restore_mutant

python3 - "$ROOT/$VERTEX" <<'PY'
from pathlib import Path
import sys
path = Path(sys.argv[1])
source = path.read_text()
needle = '\tidleTimer.Reset(idleTimeout)\n'
assert source.count(needle) == 1, 'post-header idle reset pattern missing'
path.write_text(source.replace(needle, '\tidleTimer.Stop()\n', 1))
PY
run_expected_failure post-header-stop \
  go -C "$ROOT" test ./pkg/custommodels -run '^TestProxyGCPStreamFailuresEndWithTerminalFrame$/^stalled$' -count=1
require_log post-header-stop 'want "Vertex stream made no progress for 50ms"'
actual_error="$(sed -n 's/.*error message = "\([^"]*\)", want .*/\1/p' "$TMP/post-header-stop.log" | head -n 1)"
if [[ -z "$actual_error" ]]; then
  printf 'MUTANT post-header-stop did not report an actual error line\n'
  exit 1
fi
if [[ "$actual_error" != *'context deadline exceeded'* && "$actual_error" != *'Client.Timeout or context cancellation'* ]]; then
  printf 'MUTANT post-header-stop actual error was not an accepted timeout: %s\n' "$actual_error"
  exit 1
fi
if [[ "$actual_error" == *'Vertex stream made no progress for 50ms'* ]]; then
  printf 'MUTANT post-header-stop survived with the idle error: %s\n' "$actual_error"
  exit 1
fi
printf 'MUTANT post-header-stop rejected behaviorally: actual=%s\n' "$actual_error"
restore_mutant

python3 - "$ROOT/$INJECTOR" <<'PY'
from pathlib import Path
import sys
path = Path(sys.argv[1])
source = path.read_text()
needle = 'element.getClientRects().length === 0) return false;'
assert source.count(needle) == 1, 'hidden-anchor visibility pattern missing'
path.write_text(source.replace(needle, 'element.getClientRects().length === 0) return true;', 1))
PY
expect_behavioral_failure hidden-anchor 'clicked target = "hidden", want visible (hidden duplicate must never receive the click)' \
  go -C "$ROOT" test ./pkg/gui -run '^TestRestoreConversationPathScript_SelectsOnlyVisibleMatchingSidebarAnchor$' -count=1
restore_mutant

python3 - "$ROOT/$INJECTOR" <<'PY'
from pathlib import Path
import sys
path = Path(sys.argv[1])
source = path.read_text()
view_needle = 'elementConversationId(view) === targetId'
input_needle = 'if (inputId) return inputId === targetId;'
assert source.count(view_needle) == 1, 'mounted-view identity pattern missing'
assert source.count(input_needle) == 1, 'mounted-input identity pattern missing'
source = source.replace(view_needle, 'elementConversationId(view) !== targetId', 1)
path.write_text(source.replace(input_needle, 'if (inputId) return inputId !== targetId;', 1))
PY
expect_behavioral_failure wrong-cascade 'a stale mounted view must not be ready' \
  go -C "$ROOT" test ./pkg/gui -run '^TestRestoreConversationPathScript_RejectsStaleMountedConversationView$' -count=1
restore_mutant

if ! cmp -s "$ROOT/$VERTEX" "$TMP/vertex.go" || ! cmp -s "$ROOT/$INJECTOR" "$TMP/injector.go"; then
  printf 'mutant restoration did not match the pre-probe baseline\n'
  exit 1
fi
printf 'All 436 mutants were restored; production files match the pre-probe baseline.\n' 
