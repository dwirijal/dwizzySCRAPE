#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
RAW_OUTPUT_DIR="${SNAPSHOT_OUTPUT_DIR:-${PROJECT_DIR}/.snapshots/raw}"
WEEB_DIR="${DWIZZYWEEB_DIR:-${PROJECT_DIR}/../dwizzyWEEB}"
WEEB_OUTPUT_DIR="${DWIZZYWEEB_SNAPSHOT_TARGET_DIR:-${WEEB_DIR}/public/snapshots/current}"
BUILD_BUNDLE="${DWIZZYWEEB_BUILD_BUNDLE:-1}"

if [[ -f "${PROJECT_DIR}/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${PROJECT_DIR}/.env"
  set +a
fi

if [[ "${DWIZZYSCRAPE_USE_GO_RUN:-0}" == "1" ]]; then
  DWIZZY_CMD=(go run ./cmd/dwizzyscrape)
elif [[ -n "${DWIZZYSCRAPE_CMD:-}" ]]; then
  # shellcheck disable=SC2206
  DWIZZY_CMD=(${DWIZZYSCRAPE_CMD})
elif [[ -x "${PROJECT_DIR}/.bin/dwizzyscrape" ]]; then
  DWIZZY_CMD=("${PROJECT_DIR}/.bin/dwizzyscrape")
else
  DWIZZY_CMD=(go run ./cmd/dwizzyscrape)
fi

MODE="${1:-build}"
DOMAIN="${2:-}"
SLUG="${3:-}"

cd "${PROJECT_DIR}"

case "${MODE}" in
  build)
    "${DWIZZY_CMD[@]}" snapshot-build "${RAW_OUTPUT_DIR}"
    ;;
  patch)
    if [[ -z "${DOMAIN}" || -z "${SLUG}" ]]; then
      echo "usage: publish-weeb-snapshots.sh patch <domain> <slug>" >&2
      exit 1
    fi
    "${DWIZZY_CMD[@]}" snapshot-patch "${DOMAIN}" "${SLUG}" "${RAW_OUTPUT_DIR}"
    ;;
  *)
    echo "usage: publish-weeb-snapshots.sh [build|patch <domain> <slug>]" >&2
    exit 1
    ;;
esac

if [[ "${BUILD_BUNDLE}" != "1" ]]; then
  echo "skipped snapshot bundle build because DWIZZYWEEB_BUILD_BUNDLE=${BUILD_BUNDLE}"
  exit 0
fi

if [[ ! -f "${WEEB_DIR}/scripts/build-snapshot-bundle.mjs" ]]; then
  echo "skipped snapshot bundle build because ${WEEB_DIR}/scripts/build-snapshot-bundle.mjs is unavailable"
  exit 0
fi

node "${WEEB_DIR}/scripts/build-snapshot-bundle.mjs" "${RAW_OUTPUT_DIR}" "${WEEB_OUTPUT_DIR}"
echo "published snapshot bundle to ${WEEB_OUTPUT_DIR}"
