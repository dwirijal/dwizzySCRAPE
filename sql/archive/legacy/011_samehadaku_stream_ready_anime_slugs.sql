create or replace view public.samehadaku_stream_ready_anime_slugs as
select distinct
  anime_slug
from public.samehadaku_episode_details
where anime_slug is not null
  and anime_slug <> ''
  and (
    coalesce(stream_links_json ->> 'primary', '') <> ''
    or coalesce(stream_links_json -> 'mirrors', '{}'::jsonb) <> '{}'::jsonb
  );
