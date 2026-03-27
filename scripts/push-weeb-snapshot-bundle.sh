#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
WEEB_DIR="${DWIZZYWEEB_DIR:-${PROJECT_DIR}/../dwizzyWEEB}"
WEEB_BRANCH="${DWIZZYWEEB_BRANCH:-main}"
COMMIT_MESSAGE="${DWIZZYWEEB_GIT_COMMIT_MESSAGE:-chore: update media snapshots}"
PUSH_CHANGES="${DWIZZYWEEB_PUSH:-0}"
AUTHOR_NAME="${GIT_AUTHOR_NAME:-github-actions[bot]}"
AUTHOR_EMAIL="${GIT_AUTHOR_EMAIL:-41898282+github-actions[bot]@users.noreply.github.com}"

if [[ ! -d "${WEEB_DIR}/.git" ]]; then
  echo "[push-weeb-snapshot-bundle] expected git checkout at ${WEEB_DIR}" >&2
  exit 1
fi

git -C "${WEEB_DIR}" add public/snapshots/current

if git -C "${WEEB_DIR}" diff --cached --quiet -- public/snapshots/current; then
  echo "[push-weeb-snapshot-bundle] no snapshot bundle changes to commit"
  exit 0
fi

git -C "${WEEB_DIR}" config user.name "${AUTHOR_NAME}"
git -C "${WEEB_DIR}" config user.email "${AUTHOR_EMAIL}"
git -C "${WEEB_DIR}" commit -m "${COMMIT_MESSAGE}"

if [[ "${PUSH_CHANGES}" == "1" ]]; then
  git -C "${WEEB_DIR}" push origin "HEAD:${WEEB_BRANCH}"
fi
