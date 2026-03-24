# api.dwizzy.my.id Content Platform Design

## Goal

Define the future public API shape for `api.dwizzy.my.id` so all `dwizzyOS` apps read media content through one stable gateway while:

- `auth.dwizzy.my.id` remains the identity authority
- `dwizzySCRAPE` remains the scrape and sync engine
- `dwizzyBRAIN` becomes the policy, aggregation, and entitlement layer
- storage stays lean by keeping only canonical content and access state

## Final Direction

The platform is split into three clear responsibilities:

- `auth.dwizzy.my.id`
  - Supabase Auth authority
  - login, logout, refresh, session
- `api.dwizzy.my.id`
  - canonical public API
  - content aggregation
  - premium gating
  - API key issuance and validation
  - billing and subscription handling
- `dwizzySCRAPE`
  - source-specific scraping
  - catalog sync
  - chapter and episode manifest refresh
  - internal write path only

## Data Ownership

### Supabase

Supabase remains the source of truth for identity and paid access state:

- auth users
- public profiles
- billing customer mapping
- subscriptions
- entitlements
- API keys
- usage rollups
- webhook audit records

Supabase is not used for canonical content storage.

### Neon

Neon becomes the canonical content store for thin media records:

- content title rows
- slug and media type
- source mapping and canonical URLs
- external IDs such as `mal_id` and `tmdb_id`
- chapter and episode indexes
- ordered chapter page manifest cache when needed
- sync run history

Neon does not store heavyweight upstream metadata, raw HTML, or binary media.

### Cache Layer

The edge cache layer is used for hot-path reads and access snapshots:

- Cloudflare Cache API for short-lived response caching
- Workers KV for cached manifests, entitlement snapshots, API key snapshots, and stale-safe read payloads

Cache is never the source of truth.

## Why Thin Canonical Storage

The platform should not persist every field from Jikan, TMDB, IRAG, or the source websites.

Only store data that is expensive or unsafe to recompute on every request:

- canonical slug
- canonical source URL
- minimal content identity
- external lookup IDs
- update timestamps
- list and unit indexes

Rich detail is composed at read time from:

- Neon canonical content
- Jikan for anime enrichment
- TMDB for movie enrichment
- IRAG for federated fallback or route-specific enrichment
- live source resolution or cached source manifests for manga and manhwa units

This keeps storage cheaper, schema churn lower, and sync jobs simpler.

## Public API Namespaces

To avoid collisions with existing IRAG route groups, canonical content uses `/v1/content/*`.

Public routes:

- `GET /v1/content/{type}`
- `GET /v1/content/{type}/{slug}`
- `GET /v1/content/{type}/{slug}/units`
- `GET /v1/content/units/{unitSlug}`
- `GET /v1/content/search`
- `GET /v1/account/me`
- `GET /v1/account/entitlement`
- `GET /v1/account/api-keys`
- `POST /v1/account/api-keys`
- `DELETE /v1/account/api-keys/{id}`
- `GET /v1/billing/plans`
- `POST /v1/billing/checkout`
- `GET /v1/billing/subscription`
- `POST /v1/billing/subscription/cancel`
- `POST /v1/billing/webhooks/xendit`

Internal routes:

- `POST /v1/internal/sync/{source}/catalog`
- `POST /v1/internal/sync/{source}/title/{slug}`
- `POST /v1/internal/sync/{source}/unit/{slug}`
- `POST /v1/internal/revalidate/{type}/{slug}`

Federated routes that belong to IRAG stay separate under their own gateway surface and are not the canonical content source.

## Route Behavior Matrix

### Content List

- source of truth: Neon
- enrichment: none
- fallback: stale cache only
- auth: public

List and pagination must remain canonical and stable. They must not be rebuilt live from Jikan, TMDB, or IRAG at request time.

### Content Search

- source of truth: Neon
- optional enrichment: IRAG hints or source-specific search assist
- fallback: stale cache
- auth: public

Search may blend provider hints later, but the ordered result set should still be resolved back into canonical content rows where possible.

### Content Detail

Anime detail:

- base: Neon
- enrich: Jikan
- fallback: IRAG anime surface, then DB-only partial response

Movie detail:

- base: Neon
- enrich: TMDB
- fallback: IRAG film surface, then DB-only partial response

Manga and manhwa detail:

- base: Neon
- enrich: source resolver when needed
- fallback: IRAG manga surface, then DB-only partial response

### Unit Detail

For anime episodes and movies:

- base: Neon unit record
- enrich: source resolver or stream resolver
- fallback: stale asset manifest

For manga and manhwa chapters:

- base: Neon unit record
- enrich: cached page manifest or live scrape manifest
- fallback: stale manifest

### Account and Billing

- source of truth: Supabase
- auth: token or API key depending on route
- fallback: no provider fallback

Billing reads and writes must never depend on enrichment providers.

## Response Contract

Every route should use a consistent envelope:

```json
{
  "data": {},
  "meta": {
    "providers_used": ["neon", "jikan"],
    "partial": false,
    "cache_status": "hit",
    "fallback_used": false
  },
  "error": null
}
```

If canonical data exists but enrichment fails, the API should still return `200` with:

- `meta.partial = true`
- `meta.fallback_used = true`

If canonical data does not exist and all providers fail, the API returns a terminal error.

## Authentication and Authorization

### Identity Authority

`auth.dwizzy.my.id` is the only identity authority consumed by public apps.

It owns:

- session creation
- session refresh
- login and logout
- Supabase-backed identity lifecycle

`api.dwizzy.my.id` trusts access tokens issued through this authority and performs its own authorization decisions.

### API Authorization

`api.dwizzy.my.id` decides:

- whether the caller is anonymous, free, premium, or internal
- whether a route is public or protected
- what rate limit profile applies
- whether API key creation is allowed

Auth and billing must remain separate concerns:

- auth answers who the caller is
- billing and entitlements answer what the caller may do

## Premium Subscription Model

Premium is subscription-based and local-Indonesia-first.

Recommended provider for v1:

- Xendit subscriptions

Reason:

- recurring-focused feature set
- local payment coverage
- cleaner path for Indonesian subscription flows than a card-only subscription-first provider

Subscription lifecycle:

- user logs in through `auth.dwizzy.my.id`
- user starts checkout through `api.dwizzy.my.id`
- payment provider calls webhook
- billing service updates subscription state in Supabase
- entitlement snapshot is updated
- future API reads use the premium access profile

Initial subscription states:

- `pending`
- `active`
- `past_due`
- `canceled`
- `expired`

## API Key Policy

API keys are created and managed by `api.dwizzy.my.id`, not by the auth app.

Rules:

- only premium accounts may create API keys
- plaintext key is shown once
- stored value is hash-only
- API keys are scoped
- API keys are used for programmatic access, not browser session auth

Initial policy:

- max `5` active keys per premium account
- `X-API-Key` header
- content and approved read scopes only
- no auth or billing write access via API key

## Rate Limit Policy

Initial access tiers:

- anonymous
  - public cached content only
- authenticated free
  - account routes
  - standard content routes
- premium session
  - higher request limits
  - premium read features
  - API key eligibility
- premium API key
  - server-to-server access with scoped limits
- internal service key
  - internal sync and revalidation routes only

Hot-path limiter state should live in edge storage, not in Supabase tables on every request.

Recommended runtime model:

- enforce by IP for anonymous callers
- enforce by `user_id` for bearer tokens
- enforce by `api_key_id` for API key access
- roll usage up into Supabase asynchronously

## Fallback Rules

The default fallback order is:

```text
cache -> canonical DB -> enrichment/live provider -> stale cache -> error
```

Provider failures:

- timeout, `429`, `5xx`, and parse failures may fall through to the next provider
- `404` is terminal for that provider branch
- provider health should be cached briefly to avoid hammering a failing upstream

IRAG remains a federated fallback layer, not the source of truth for canonical content rows.

## Deployment Topology

Recommended deployment:

- `dwizzyAUTH` on Vercel
- `api.dwizzy.my.id` on Cloudflare Workers as the edge API
- `dwizzyBRAIN` origin services on homelab or the current backend origin
- `dwizzySCRAPE` on homelab or cron-driven runtime
- Supabase for auth and access-state storage
- Neon for canonical content storage
- Cloudflare Cache API and KV for hot-path cache and snapshots

Heavy scraping and large backfills should not run inside Workers free-tier CPU limits.

## Cost Direction

The target is near-zero or low-cost operation, not infinite free usage.

This architecture stays lean because:

- content storage is minimal
- heavyweight metadata is fetched on demand
- billing and auth do not require a separate custom auth stack
- scraping stays off the edge runtime
- cache absorbs repeat reads

If usage grows, the first expected scale step is paid Cloudflare runtime or cache capacity, not a redesign.

## Non-Goals For V1

These are intentionally excluded from the first implementation:

- storing full raw provider payloads
- storing binary images in databases
- making IRAG the canonical content source
- per-request live assembly for list pagination
- enterprise RBAC
- organization billing
- multiple premium plan families beyond a simple subscription tier

## Locked Decisions

- `auth.dwizzy.my.id` stays the only identity authority
- `api.dwizzy.my.id` becomes the public policy and aggregation layer
- Supabase owns account, billing, entitlement, API key, and usage truth
- Neon owns thin canonical content truth
- Jikan, TMDB, IRAG, and live source resolvers are enrichment or fallback providers
- canonical content lives under `/v1/content/*`
- premium access is subscription-based
- API keys are premium-only and issued by `api.dwizzy.my.id`
