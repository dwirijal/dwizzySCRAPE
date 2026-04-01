#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [[ -f "${PROJECT_DIR}/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${PROJECT_DIR}/.env"
  set +a
fi

DB_DSN="${DATABASE_URL:-${POSTGRES_URL:-${NEON_DATABASE_URL:-}}}"
if [[ -z "${DB_DSN}" ]]; then
  echo "[backfill-samehadaku] missing DB DSN (DATABASE_URL; POSTGRES_URL/NEON_DATABASE_URL are compatibility fallbacks)" >&2
  exit 1
fi

COMMAND="${1:-full}"
shift || true

if [[ "${DWIZZYSCRAPE_USE_GO_RUN:-1}" == "1" ]]; then
  DWIZZY_CMD=(go run ./cmd/samehadaku-backfill)
elif [[ -n "${SAMEHADAKU_BACKFILL_CMD:-}" ]]; then
  # shellcheck disable=SC2206
  DWIZZY_CMD=(${SAMEHADAKU_BACKFILL_CMD})
elif [[ -x "${PROJECT_DIR}/.bin/samehadaku-backfill" ]]; then
  DWIZZY_CMD=("${PROJECT_DIR}/.bin/samehadaku-backfill")
else
  DWIZZY_CMD=(go run ./cmd/samehadaku-backfill)
fi

cd "${PROJECT_DIR}"
exec "${DWIZZY_CMD[@]}" "${COMMAND}" "$@"
