# dwizzySCRAPE

`dwizzySCRAPE` is the local scraper service for dwizzyOS media ingestion.

Current scope:

- source: `Samehadaku`
- catalog: `https://v2.samehadaku.how/daftar-anime-2/`
- storage model: current-state only
- destination: lean local Postgres canonical media tables
- stored fields: title, canonical URL, slug, poster URL, anime type, status, score, views, synopsis excerpt, genres, timestamps
Compact v2 additive read model:

- anime: `anime_list` + `anime_meta` + `anime_episodes`
- old tables stay as scraper staging until cutover is complete
- v2 prefers MAL/Jikan for anime, then falls back to scraped source values

## Commands

```bash
go run ./cmd/dwizzyscrape migrate
go run ./cmd/dwizzyscrape backfill
go run ./cmd/dwizzyscrape sync
go run ./cmd/dwizzyscrape detail-anime ao-no-orchestra-season-2
go run ./cmd/dwizzyscrape backfill-anime-details
go run ./cmd/dwizzyscrape detail-episodes ao-no-orchestra-season-2
go run ./cmd/dwizzyscrape backfill-episodes
go run ./cmd/dwizzyscrape refresh-anime-v2
go run ./cmd/dwizzyscrape refresh-media-v2
go run ./cmd/dwizzyscrape refresh-movie-v3
go run ./cmd/dwizzyscrape manhwa-catalog
go run ./cmd/dwizzyscrape manhwa-series solo-leveling
go run ./cmd/dwizzyscrape manhwa-chapter solo-leveling-chapter-100
go run ./cmd/dwizzyscrape sync-manhwa-catalog
go run ./cmd/dwizzyscrape sync-manhwa-series solo-leveling
go run ./cmd/dwizzyscrape sync-manhwa-chapter solo-leveling-chapter-100
go run ./cmd/dwizzyscrape backfill-manhwa-series 1 3
go run ./cmd/dwizzyscrape backfill-manhwa-chapters 1 3 3
go run ./cmd/dwizzyscrape komiku-catalog
go run ./cmd/dwizzyscrape komiku-series standard-of-reincarnation-id
go run ./cmd/dwizzyscrape komiku-chapter standard-of-reincarnation-id-chapter-173
go run ./cmd/dwizzyscrape sync-komiku-catalog
go run ./cmd/dwizzyscrape sync-komiku-series standard-of-reincarnation-id
go run ./cmd/dwizzyscrape sync-komiku-chapter standard-of-reincarnation-id-chapter-173
go run ./cmd/dwizzyscrape backfill-komiku-series 1 10
go run ./cmd/dwizzyscrape backfill-komiku-chapters 1 10 20
go run ./cmd/dwizzyscrape snapshot-build ./snapshots
go run ./cmd/dwizzyscrape snapshot-patch movie war-machine-2026-1265609 ./snapshots
go run ./cmd/dwizzyscrape snapshot-webhook build ./snapshots
go run ./cmd/dwizzyscrape snapshot-webhook patch anime ao-no-orchestra-season-2 ./snapshots
```

## Bootstrapping

```bash
docker compose -f docker-compose.local.yml up -d
cp .env.example .env
source .env
go run ./cmd/dwizzyscrape migrate
go run ./cmd/dwizzyscrape backfill
```

The local bootstrap stack exposes:

- Postgres on `127.0.0.1:5432`
- Valkey on `127.0.0.1:6379`

## Scheduling (Cron)

Use provided scheduler scripts:

```bash
./scripts/cron-media-latest.sh
./scripts/cron-media-maintenance.sh
./scripts/list-snapshot-patch-targets.sh
./scripts/publish-weeb-snapshots.sh build
./scripts/publish-weeb-snapshots.sh patch anime ao-no-orchestra-season-2
./scripts/publish-weeb-snapshot-patches.sh
./scripts/push-weeb-snapshot-bundle.sh
```

Recommended crontab:

```bash
*/15 * * * * cd /home/dwizzy/workspace/projects/dwizzyOS/dwizzySCRAPE && /usr/bin/env bash ./scripts/cron-media-latest.sh && /usr/bin/env bash ./scripts/cron-media-maintenance.sh && /usr/bin/env bash ./scripts/publish-weeb-snapshot-patches.sh
15 */6 * * * cd /home/dwizzy/workspace/projects/dwizzyOS/dwizzySCRAPE && /usr/bin/env bash ./scripts/publish-weeb-snapshots.sh build
```

Notes:

- `cron-media-latest.sh` does incremental fetch from Samehadaku, Manhwaindo, Komiku, and Kanata movie home.
- It uses lock file (`/tmp/dwizzyscrape-cron-media-latest.lock`) to avoid overlap and can skip when backfill job is running.
- `cron-media-maintenance.sh` refreshes SQL read models (`refresh-anime-v2`, `refresh-media-v2`, `refresh-movie-v3`).
- `list-snapshot-patch-targets.sh` queries the read model for titles updated recently and emits `domain slug touched_at` rows.
- `publish-weeb-snapshot-patches.sh` loops over recent patch targets and refreshes only the affected snapshot docs.
- `publish-weeb-snapshots.sh` bridges the raw snapshot pack into `dwizzyWEEB/public/snapshots/current` by calling `dwizzyWEEB/scripts/build-snapshot-bundle.mjs`.
- `push-weeb-snapshot-bundle.sh` stages only `public/snapshots/current` in a checked-out `dwizzyWEEB` repo, commits it, and can push it to trigger Vercel deploy.
- Tune limits and behavior with env vars (examples): `ANIME_RECENT_LIMIT`, `MANHWA_CATALOG_PAGES`, `KOMIKU_CATALOG_PAGES`, `MOVIE_HOME_LIMIT`, `RESPECT_BACKFILL`.
- Set `DWIZZYSCRAPE_USE_GO_RUN=1` when you want scripts to ignore a stale `.bin/dwizzyscrape` and execute the current source tree instead.

## GitHub Actions

The repo now supports free CI scheduling with two workflows:

- `media-latest.yml`: every 15 minutes, runs incremental scrape + read-model refresh + snapshot patch publish
- `snapshot-full-rebuild.yml`: every 6 hours, rebuilds the full hot-media snapshot pack

Expected GitHub secrets for CI:

- `POSTGRES_URL` preferred when CI uses a reachable Postgres
- `NEON_DATABASE_URL`
- `SAMEHADAKU_COOKIE` when needed
- `TMDB_READ_TOKEN` or `TMDB_API_KEY` when movie enrichment is enabled
- `DWIZZYWEEB_PUSH_TOKEN` if the workflow should push updated snapshot bundles into `dwirijal/dwizzyWEEB`

CI flow:

1. Checkout `dwizzySCRAPE`
2. Optionally checkout `dwizzyWEEB`
3. Run scrape/refresh jobs
4. Build or patch snapshots
5. Commit only `public/snapshots/current` in `dwizzyWEEB`
6. Push to `main` to trigger Vercel auto-deploy

## Environment

Runtime ownership:

- `Supabase` is for auth/account flows only
- `dwizzySCRAPE` writes media directly to `Postgres`
- public apps should read anime/movie content through `api.dwizzy.my.id`, not from Neon or Supabase directly

- `POSTGRES_URL` recommended as primary runtime database DSN
- `DATABASE_URL` supported as compatibility alias
- `NEON_DATABASE_URL` supported only as compatibility fallback while local Postgres is being rolled out
- `SAMEHADAKU_CATALOG_URL` optional, defaults to `https://v2.samehadaku.how/daftar-anime-2/`
- `SAMEHADAKU_COOKIE` optional, only needed when Cloudflare challenge blocks anonymous requests
- `SAMEHADAKU_USER_AGENT` optional
- `SAMEHADAKU_HTTP_TIMEOUT` optional, defaults to `30s`
- `KANATA_MOVIETUBE_BASE_URL` optional, defaults to `https://api.kanata.web.id/movietube`
- `MANHWAINDO_BASE_URL` optional, defaults to `https://www.manhwaindo.my`
- `MANHWAINDO_USER_AGENT` optional, defaults to the same browser UA used for Samehadaku
- `MANHWAINDO_COOKIE` optional, only needed when source protection blocks anonymous requests
- `KOMIKU_BASE_URL` optional, defaults to `https://komiku.org`
- `KOMIKU_USER_AGENT` optional, defaults to the same browser UA used for Samehadaku
- `KOMIKU_COOKIE` optional, only needed when source protection blocks anonymous requests
- `JIKAN_BASE_URL` optional, defaults to `https://api.jikan.moe/v4`
- `TMDB_BASE_URL` optional, defaults to `https://api.themoviedb.org/3`
- `TMDB_READ_TOKEN` optional, recommended for movie enrichment
- `TMDB_API_KEY` optional fallback if you do not want to use a bearer token
- `SNAPSHOT_OUTPUT_DIR` optional, defaults to `snapshots`
- `SNAPSHOT_HOT_LIMIT` optional, defaults to `8`
- `SNAPSHOT_CATALOG_PAGE` optional, defaults to `1`
- `SNAPSHOT_MOVIE_GENRES` optional CSV for movie catalog snapshot seeds, defaults to `action,drama`
- `SNAPSHOT_MOVIE_SEARCH_QUERIES` optional CSV for movie search snapshot seeds; if empty, top home titles are used

## Cloudflare note

Samehadaku currently serves a Cloudflare challenge to anonymous machine requests.
In current verification, the catalog sync still worked without a cookie.
Keep `SAMEHADAKU_COOKIE` as a fallback for days when Cloudflare starts challenging automated requests again.

## Storage note

Active anime/movie/media sync writes directly to `POSTGRES_URL` (or `DATABASE_URL` alias). `NEON_DATABASE_URL` is kept only as a compatibility fallback during migration.
No Supabase management API path is required for media ingestion in this service.

Only root-level files in `sql/*.sql` are replayed during migrate/refresh.
Historical migrations are archived under `sql/archive/` and intentionally excluded
from runtime replay to keep schema application lean.
## Compact v2 code map

- anime source: `s = samehadaku`
- anime metadata source: `m = MAL/Jikan`, `s = scrape`
- anime type: `t = TV`, `m = movie`, `o = OVA`, `n = ONA`, `p = special`, `u = unknown`
- anime status: `a = airing`, `f = finished`, `u = upcoming`, `x = unknown`
- anime season: `w = winter`, `p = spring`, `s = summer`, `f = fall`, `x = unknown`
- episode fetch: `p = primary`, `s = secondary`, `b = blocked`, `x = unknown`

## Public Read API

`dwizzyBRAIN` is the public media gateway. Current public route families include:

```text
GET /v1/anime/home?limit=6
GET /v1/anime/{slug}
GET /v1/anime/episodes/{slug}
GET /v1/film/home?limit=6
GET /v1/film/{slug}
GET /v1/film/watch/{slug}
```

## Snapshot Pack

Snapshot packs are filesystem outputs intended to be consumed by another repo as build input.

Output layout:

```text
<output>/
  manifest.json
  anime/catalog/*.json
  anime/title/*.json
  anime/playback/*.json
  movie/home/*.json
  movie/catalog/*.json
  movie/search/*.json
  movie/title/*.json
  movie/playback/*.json
  manhwaindo/catalog/*.json
  manhwaindo/title/*.json
  manhwaindo/playback/*.json
  komiku/catalog/*.json
  komiku/title/*.json
  komiku/playback/*.json
```

Notes:

- `snapshot-build` writes a full hot-media pack and rebuilds `manifest.json`.
- `snapshot-patch <domain> <slug>` refreshes only the changed title/playback pair for one domain and then rebuilds the manifest.
- `snapshot-webhook` is a thin CLI entry for automation. It accepts either positional args or env vars:
  - `SNAPSHOT_ACTION=build`
  - `SNAPSHOT_ACTION=patch`
  - `SNAPSHOT_DOMAIN=<domain>`
  - `SNAPSHOT_SLUG=<slug>`
