# AniList Fallback Enrichment Design

## Summary

Add `AniList` as a fallback metadata enricher in `dwizzySCRAPE`. It will not replace `TMDB` or `Jikan` as the primary resolvers. It will only run for items that are still `non-matched` or `low-confidence`, and it may promote canonical fields when `AniList` match confidence is high enough.

The design keeps Aiven Postgres lean by storing only compact item-level enrichment payloads in `public.media_item_enrichments`. No raw GraphQL response blobs and no unit-level enrichment are allowed.

## Goals

- Improve metadata coverage for items that `TMDB` and `Jikan` fail to match cleanly
- Allow canonical promotion from `AniList` for ambiguous items
- Keep provider locks intact for strongly classified sources
- Reuse the existing enrichment table and backfill patterns
- Avoid request-heavy enrichment across already-stable rows

## Non-Goals

- Replacing `TMDB` as the primary movie or live-action enricher
- Replacing `Jikan` as the primary anime resolver
- Storing raw AniList responses
- Enriching `media_units`
- Reworking `dwizzyWEEB` UI in this tranche

## Scope

### Included

- New AniList client in `dwizzySCRAPE`
- Fallback candidate selection for `non-matched + low-confidence` items
- Compact `provider='anilist'` writes into `public.media_item_enrichments`
- Canonical promotion rules from AniList payloads
- Video first, then comic in the same pipeline model

### Excluded

- UI-level editorial changes for `variety`
- Any `dwizzyBRAIN` integration
- Raw payload archival outside Postgres

## Resolver Order

Primary resolution remains unchanged:

1. `TMDB` for `movie` and `live_action series`
2. `Jikan` for anime-leaning items
3. `AniList` only when the item is still unresolved enough to justify fallback

AniList runs only when at least one of these is true:

- no `matched` enrichment exists from the preferred primary provider
- canonical confidence is still low
- item belongs to a domain with weak primary coverage but is not provider-locked

## Provider Locks

AniList promotion must not override strong source truths.

Strong locks:

- `anichin -> origin_type='donghua'`
- `samehadaku anime -> origin_type='anime'`
- `drakorid movie -> surface_type='movie', presentation_type='live_action', origin_type='movie'`
- `drakorid variety -> origin_type='variety'`

If a strong lock applies, AniList may still enrich metadata fields like aliases, score, synopsis, cover, and tags, but canonical type-level fields remain locked.

## Promotion Rules

AniList may promote these canonical fields:

- `origin_type`
- `presentation_type`
- `release_country`
- `genre_names`
- `is_nsfw`
- supporting metadata such as aliases, score, synopsis, year, cover, and banner

Promotion threshold:

- `match_score >= 70`

Promotion is allowed only when:

- `match_status = 'matched'`
- no strong provider lock blocks the field
- AniList payload provides a stronger or missing canonical signal

## Storage Rules

AniList payload must stay compact and item-level.

Allowed fields in `payload`:

- `id`
- `idMal`
- title aliases
- `format`
- `status`
- `countryOfOrigin`
- `genres`
- selected tags
- `averageScore`
- `seasonYear`
- `coverImage`
- `bannerImage`
- `isAdult`

Not allowed:

- raw GraphQL response blobs
- full character lists
- episode/chapter payloads

## Candidate Strategy

AniList fallback candidates are:

- items with no `matched` primary enrichment
- items with low canonical confidence
- ambiguous catalog rows where source heuristics are weak

AniList must not be run for:

- rows with strong provider locks and already-good enrichment
- rows whose only remaining mismatch belongs to known non-scripted `variety` handling

## Execution Order

### Phase 1

- Add AniList config and GraphQL client
- Add unit tests for query building and compact payload mapping

### Phase 2

- Add AniList candidate reader and backfill service
- Write `provider='anilist'` rows into `media_item_enrichments`

### Phase 3

- Add canonical promotion rules from AniList payload
- Respect provider locks and promotion threshold

### Phase 4

- Run backfill for `non-matched + low-confidence` video items
- Verify coverage

### Phase 5

- Extend same fallback model to comic items

## Verification

Required evidence before claiming success:

- Go tests for AniList client, candidate selection, promotion logic, and backfill runner
- live backfill run completes without looping
- enrichment counts show new AniList coverage
- canonical counts remain stable for locked providers
- `dwizzyWEEB` smoke routes still resolve against canonical data

## Risks

- AniList matching may be overly aggressive at `70`, especially for ambiguous romanized titles
- GraphQL rate limiting requires a conservative request cadence
- promotion bugs could corrupt taxonomy if provider locks are not enforced correctly

## Recommendation

Implement AniList as a narrow fallback first:

- only `non-matched + low-confidence`
- item-level compact payloads only
- promotion allowed, but field-level lock rules must win over AniList

This gives broader coverage without bloating storage or destabilizing the canonical taxonomy already established for `movie`, `series`, `donghua`, and `variety`.
