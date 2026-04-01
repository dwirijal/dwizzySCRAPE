#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

BEGIN_MARKER="# BEGIN DWIZZYSCRAPE_MEDIA_CRON"
END_MARKER="# END DWIZZYSCRAPE_MEDIA_CRON"

CRON_BLOCK=$(cat <<EOF
${BEGIN_MARKER}
0 */4 * * * cd ${PROJECT_DIR} && /usr/bin/env bash ./scripts/cron-release-anime.sh
20 */4 * * * cd ${PROJECT_DIR} && /usr/bin/env bash ./scripts/cron-release-comics.sh
15 */6 * * * cd ${PROJECT_DIR} && /usr/bin/env bash ./scripts/cron-media-maintenance.sh
${END_MARKER}
EOF
)

TMP_FILE="$(mktemp)"
cleanup() {
  rm -f "${TMP_FILE}"
}
trap cleanup EXIT

if crontab -l >"${TMP_FILE}" 2>/dev/null; then
  :
else
  : >"${TMP_FILE}"
fi

awk -v begin="${BEGIN_MARKER}" -v end="${END_MARKER}" '
  $0 == begin {skip=1; next}
  $0 == end {skip=0; next}
  !skip {print}
' "${TMP_FILE}" >"${TMP_FILE}.next"

printf '%s\n' "${CRON_BLOCK}" >>"${TMP_FILE}.next"
crontab "${TMP_FILE}.next"

echo "installed dwizzyscrape cron block"
crontab -l
