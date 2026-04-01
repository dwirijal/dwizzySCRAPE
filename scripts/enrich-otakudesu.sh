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
  echo "[enrich-otakudesu] missing DB DSN (DATABASE_URL; POSTGRES_URL/NEON_DATABASE_URL are compatibility fallbacks)" >&2
  exit 1
fi

if [[ "${DWIZZYSCRAPE_USE_GO_RUN:-1}" == "1" ]]; then
  DWIZZY_CMD=(go run ./cmd/otakudesu-enrich)
elif [[ -n "${OTAKUDESU_ENRICH_CMD:-}" ]]; then
  # shellcheck disable=SC2206
  DWIZZY_CMD=(${OTAKUDESU_ENRICH_CMD})
elif [[ -x "${PROJECT_DIR}/.bin/otakudesu-enrich" ]]; then
  DWIZZY_CMD=("${PROJECT_DIR}/.bin/otakudesu-enrich")
else
  DWIZZY_CMD=(go run ./cmd/otakudesu-enrich)
fi

cd "${PROJECT_DIR}"
exec "${DWIZZY_CMD[@]}"
