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

expect_behavioral_failure() {
  local name="$1"
  local assertion="$2"
  shift 2
  if "$@" > "$TMP/$name.log" 2>&1; then
    printf 'MUTANT %s unexpectedly passed\n' "$name"
    cat "$TMP/$name.log"
    return 1
  fi
  cat "$TMP/$name.log"
  if grep -Eqi 'syntax error|build failed|undefined:|cannot use|unknown field' "$TMP/$name.log"; then
    printf 'MUTANT %s failed outside behavioral assertions\n' "$name"
    return 1
  fi
  if ! grep -Fq "$assertion" "$TMP/$name.log"; then
    printf 'MUTANT %s did not reach assertion: %s\n' "$name" "$assertion"
    return 1
  fi
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
expect_behavioral_failure active-stream 'frames = ' \
  go -C "$ROOT" test ./pkg/custommodels -run '^TestProxyGCPActiveStreamResetsIdleGuard$' -count=1
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
expect_behavioral_failure hidden-anchor 'clicked target = "hidden"' \
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
