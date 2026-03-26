#!/usr/bin/env bash

set -euo pipefail

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 1
fi

SANKA_BASE="${SANKA_BASE:-https://www.sankavollerei.com}"

fetch_json() {
  local url="$1"
  local code
  code=$(curl -sS -o /tmp/sanka_discover.json -w '%{http_code}' "$url")
  if [[ "$code" != "200" ]]; then
    echo "status=$code for $url" >&2
    return 1
  fi
}

print_domains() {
  jq -r '..|strings|select(test("^https?://"))' /tmp/sanka_discover.json \
    | sed 's#^https\?://##' \
    | cut -d/ -f1 \
    | sort -u
}

print_upstream_urls() {
  jq -r '
    paths(strings) as $p
    | (getpath($p)) as $v
    | ($p[-1] // "") as $k
    | select($v | test("^https?://"))
    | select(($k|type) == "string")
    | select(($k|test("url$"; "i")) or ($k|test("href$"; "i")))
    | $k + "=" + $v
  ' /tmp/sanka_discover.json | sort -u
}

resolve_dynamic_endpoints() {
  local otaku_slug anoboy_ep oploverz_ep animasu_slug animasu_ep donghua_ep samehadaku_ep

  fetch_json "$SANKA_BASE/anime/home" || true
  otaku_slug="$(jq -r '.data.ongoing.animeList[0].animeId // empty' /tmp/sanka_discover.json)"

  fetch_json "$SANKA_BASE/anime/anoboy/home?page=1" || true
  anoboy_ep="$(jq -r '.anime_list[0].slug // empty' /tmp/sanka_discover.json)"

  fetch_json "$SANKA_BASE/anime/oploverz/home?page=1" || true
  oploverz_ep="$(jq -r '.anime_list[0].slug // empty' /tmp/sanka_discover.json)"

  fetch_json "$SANKA_BASE/anime/animasu/home" || true
  animasu_slug="$(jq -r '.ongoing[0].slug // empty' /tmp/sanka_discover.json)"

  if [[ -n "$animasu_slug" ]]; then
    fetch_json "$SANKA_BASE/anime/animasu/detail/$animasu_slug" || true
    animasu_ep="$(jq -r '.detail.episodes[0].slug // empty' /tmp/sanka_discover.json)"
  fi

  fetch_json "$SANKA_BASE/anime/donghua/home/1" || true
  donghua_ep="$(jq -r '.latest_release[0].slug // empty' /tmp/sanka_discover.json | sed 's#/$##')"

  fetch_json "$SANKA_BASE/anime/samehadaku/home" || true
  samehadaku_ep="$(jq -r '.data.recent.animeList[0].animeId // empty' /tmp/sanka_discover.json)"
  if [[ -n "$samehadaku_ep" ]]; then
    fetch_json "$SANKA_BASE/anime/samehadaku/anime/$samehadaku_ep" || true
    samehadaku_ep="$(jq -r '.data.episodeList[0].episodeId // empty' /tmp/sanka_discover.json)"
  fi

  cat <<EOF
$SANKA_BASE/anime/home
$SANKA_BASE/anime/samehadaku/home
$SANKA_BASE/anime/samehadaku/recent?page=1
$SANKA_BASE/anime/donghua/home/1
$SANKA_BASE/anime/anoboy/home?page=1
$SANKA_BASE/anime/oploverz/home?page=1
$SANKA_BASE/anime/animasu/home
EOF

  [[ -n "$otaku_slug" ]] && echo "$SANKA_BASE/anime/anime/$otaku_slug"
  [[ -n "$samehadaku_ep" ]] && echo "$SANKA_BASE/anime/samehadaku/episode/$samehadaku_ep"
  [[ -n "$donghua_ep" ]] && echo "$SANKA_BASE/anime/donghua/episode/$donghua_ep"
  [[ -n "$anoboy_ep" ]] && echo "$SANKA_BASE/anime/anoboy/episode/$anoboy_ep"
  [[ -n "$oploverz_ep" ]] && echo "$SANKA_BASE/anime/oploverz/episode/$oploverz_ep"
  [[ -n "$animasu_ep" ]] && echo "$SANKA_BASE/anime/animasu/episode/$animasu_ep"
}

main() {
  mapfile -t endpoints < <(resolve_dynamic_endpoints | awk 'NF' | sort -u)

  echo "== Sankavollerei Probe =="
  for ep in "${endpoints[@]}"; do
    echo
    echo "### $ep"
    if fetch_json "$ep"; then
      echo "upstream_url_fields:"
      print_upstream_urls | sed 's/^/  - /' || true
      echo "domains:"
      print_domains | sed 's/^/  - /' || true
    else
      echo "  - failed to fetch endpoint"
    fi
  done
}

main "$@"
