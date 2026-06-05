#!/usr/bin/env bash
# PreToolUse(Bash) guard — backstop for the 2026-06-06 incident where the test
# harness, sharing the prod Compose project, destroyed the s4rciv_db_data volume.
#
# The test harness is now isolated under the `s4rciv-test` Compose project
# (Makefile + compose.test.yaml `name:`), so legitimate test teardowns are allowed.
# This hook refuses commands that could destroy the PRODUCTION local database:
#   - removing/pruning the s4rciv_db_data volume
#   - `docker compose ... down` with -v/--volumes NOT scoped to the test stack
#   - `docker system prune --volumes`
# Exit 2 blocks the tool call and shows this message to the model (Claude Code hooks).
set -euo pipefail

input=$(cat)
cmd=$(printf '%s' "$input" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("tool_input",{}).get("command",""))' 2>/dev/null || true)
[ -z "$cmd" ] && exit 0

block() {
  echo "BLOCKED by guard-prod-docker: $1" >&2
  echo "The production local DB is off-limits to automation. Run it yourself if truly intended (e.g. the ! prefix), or scope it to the s4rciv-test project." >&2
  exit 2
}

# Matching is intra-segment: [^|;&]* between tokens never crosses a shell
# separator (| ; & && ||), so coincidental token co-occurrence in a compound or
# echo command (e.g. `... | grep -v ...; echo down`) does NOT false-positive.

# 1) Direct removal of the production data volume.
if printf '%s' "$cmd" | grep -qE 'docker[[:space:]]+volume[[:space:]]+rm[^|;&]*s4rciv_db_data'; then
  block "removing the production volume s4rciv_db_data"
fi
if printf '%s' "$cmd" | grep -qE 'docker[[:space:]]+volume[[:space:]]+prune'; then
  block "docker volume prune (can remove the production volume)"
fi

# 2) `docker compose ... down` with -v/--volumes that is NOT scoped to the test stack.
if printf '%s' "$cmd" | grep -qE 'docker[ -]compose[^|;&]*down[^|;&]*(-v([[:space:]]|$)|--volumes)' \
   && ! printf '%s' "$cmd" | grep -qE 's4rciv-test|compose\.test\.ya?ml'; then
  block "'docker compose ... down -v' not scoped to s4rciv-test (would remove production volumes)"
fi

# 3) `docker system prune --volumes`.
if printf '%s' "$cmd" | grep -qE 'docker[[:space:]]+system[[:space:]]+prune[^|;&]*--volumes'; then
  block "docker system prune --volumes (can remove the production volume)"
fi

exit 0
