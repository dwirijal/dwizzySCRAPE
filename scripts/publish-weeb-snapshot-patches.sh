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

TARGETS_FILE="${TARGETS_FILE:-}"
LOG_PREFIX="[publish-weeb-snapshot-patches]"
DRY_RUN="${DRY_RUN:-0}"
PUSH_AFTER_PATCH="${DWIZZYWEEB_PUSH_SNAPSHOT_BUNDLE:-0}"

now_utc() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

if [[ -n "${TARGETS_FILE}" ]]; then
  if [[ ! -f "${TARGETS_FILE}" ]]; then
    echo "${LOG_PREFIX} missing TARGETS_FILE=${TARGETS_FILE}" >&2
    exit 1
  fi
  mapfile -t targets <"${TARGETS_FILE}"
else
  mapfile -t targets < <("${SCRIPT_DIR}/list-snapshot-patch-targets.sh")
fi

if [[ "${#targets[@]}" -eq 0 ]]; then
  echo "${LOG_PREFIX} $(now_utc) no recent snapshot patch targets"
  exit 0
fi

patched=0
for target in "${targets[@]}"; do
  domain="$(printf '%s' "${target}" | cut -f1)"
  slug="$(printf '%s' "${target}" | cut -f2)"
  touched_at="$(printf '%s' "${target}" | cut -f3)"

  if [[ -z "${domain}" || -z "${slug}" ]]; then
    continue
  fi

  echo "${LOG_PREFIX} $(now_utc) patch domain=${domain} slug=${slug} touched_at=${touched_at}"
  if [[ "${DRY_RUN}" == "1" ]]; then
    patched=$((patched + 1))
    continue
  fi
  "${SCRIPT_DIR}/publish-weeb-snapshots.sh" patch "${domain}" "${slug}"
  patched=$((patched + 1))
done

echo "${LOG_PREFIX} $(now_utc) patched=${patched}"

if [[ "${patched}" -gt 0 && "${PUSH_AFTER_PATCH}" == "1" ]]; then
  "${SCRIPT_DIR}/push-weeb-snapshot-bundle.sh"
fi
