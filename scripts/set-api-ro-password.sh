#!/usr/bin/env bash
# Enables login for the api_ro role (migration 20260610000018) using the
# password in secrets/api_db_password.txt. Run once after the migration has
# been applied, and again whenever the password is rotated:
#
#   ./scripts/set-api-ro-password.sh
#
# To (re)generate the secret first:
#   openssl rand -base64 32 > secrets/api_db_password.txt
#   docker run --rm -v "$PWD/secrets:/s" postgres:18.4-trixie \
#     sh -c 'chown 65532:65532 /s/api_db_password.txt && chmod 600 /s/api_db_password.txt'
#
# The SQL travels over psql stdin (not argv), so the password never appears in
# `ps` output or shell history on either side.
set -euo pipefail
cd "$(dirname "$0")/.."

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi

SECRET_FILE="secrets/api_db_password.txt"
# The secret is owned by uid 65532 (the api container user) with mode 600, so
# read it through a root-in-namespace container instead of the host user.
PW="$(docker run --rm -v "$PWD/secrets:/s:ro" \
  postgres:18.4-trixie@sha256:8ff36f3c66371cba71d20ceedccfc3de9669a68737607888c4ef0af93abe8e39 \
  sh -c "tr -d '\n' < /s/$(basename "$SECRET_FILE")")"
if [ -z "$PW" ]; then
  echo "error: $SECRET_FILE is missing or empty" >&2
  exit 1
fi
# Escape single quotes for the SQL literal (base64 output has none, but be safe).
PW_ESCAPED="${PW//\'/\'\'}"

docker compose exec -T db \
  psql -U "${POSTGRES_USER:-s4rciv}" -d "${POSTGRES_DB:-s4rciv}" -v ON_ERROR_STOP=1 <<SQL
ALTER ROLE api_ro LOGIN PASSWORD '${PW_ESCAPED}';
SQL

echo "api_ro login enabled / password updated."
