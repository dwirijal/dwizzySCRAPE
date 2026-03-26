Archived migrations live here for historical reference only.

The bootstrap loader replays only `sql/*.sql` from the root `sql/` directory.
Anything moved under `sql/archive/` is intentionally excluded from runtime
replay to keep Supabase schema application lean.

Layout:
- `legacy/`: superseded Samehadaku staging/public-object migrations
- `redundant/`: migrations whose effects were folded into the active baseline
