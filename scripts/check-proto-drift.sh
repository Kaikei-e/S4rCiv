#!/usr/bin/env bash
# CDC guard: the differ vendors a MANUAL copy of diff.proto (its Docker build
# context is ./services/differ, so the proto must live inside the crate). If the
# two trees drift, every contract test can pass while the real Go↔Rust boundary
# breaks. This fails CI when the two schemas diverge.
#
# Comparison ignores comments and blank lines: what must match is the schema that
# codegen sees (package, messages, fields, enums) — not the vendored-copy banner.
set -euo pipefail
cd "$(dirname "$0")/.."

API=services/api/proto/s4rciv/diff/v1/diff.proto
DIFFER=services/differ/proto/s4rciv/diff/v1/diff.proto

for f in "$API" "$DIFFER"; do
  [ -f "$f" ] || { echo "✗ missing proto: $f" >&2; exit 2; }
done

# Strip // comments to EOL, trailing whitespace, and blank lines.
norm() { sed -E 's://.*$::' "$1" | sed -E 's/[[:space:]]+$//' | grep -vE '^[[:space:]]*$'; }

if diff <(norm "$API") <(norm "$DIFFER") >/dev/null; then
  echo "✓ diff.proto is in sync between api and differ (schema-identical)."
  exit 0
fi

echo "✗ proto drift between the api and differ copies of diff.proto." >&2
echo "  api:    $API" >&2
echo "  differ: $DIFFER" >&2
echo "  Re-sync the vendored copy (they must be schema-identical). Schema diff:" >&2
diff <(norm "$API") <(norm "$DIFFER") >&2 || true
exit 1
