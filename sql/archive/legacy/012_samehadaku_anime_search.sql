create or replace function public.search_samehadaku_anime(
  search_query text,
  match_limit integer default 8
)
returns table (
  slug text,
  title text,
  poster_url text,
  status text,
  anime_type text,
  canonical_url text
)
language sql
stable
as $$
  with normalized as (
    select
      nullif(btrim(search_query), '') as query,
      regexp_replace(lower(nullif(btrim(search_query), '')), '\s+', '-', 'g') as slug_query,
      least(greatest(coalesce(match_limit, 8), 1), 20) as row_limit
  ),
  matches as (
    select
      c.slug,
      c.title,
      c.poster_url,
      c.status,
      c.anime_type,
      c.canonical_url,
      case
        when lower(c.title) = lower(n.query) then 0
        when lower(c.slug) = n.slug_query then 1
        when lower(c.title) like lower(n.query) || '%' then 2
        when lower(c.slug) like n.slug_query || '%' then 3
        when lower(c.title) like '% ' || lower(n.query) || '%' then 4
        when lower(c.title) like '%' || lower(n.query) || '%' then 5
        when lower(c.slug) like '%' || n.slug_query || '%' then 6
        else 9
      end as rank_bucket,
      position(lower(n.query) in lower(c.title)) as title_position
    from public.samehadaku_anime_catalog c
    cross join normalized n
    where n.query is not null
      and char_length(n.query) >= 2
      and (
        lower(c.title) like '%' || lower(n.query) || '%'
        or lower(c.slug) like '%' || n.slug_query || '%'
      )
  )
  select
    m.slug,
    m.title,
    m.poster_url,
    m.status,
    m.anime_type,
    m.canonical_url
  from matches m
  cross join normalized n
  order by
    m.rank_bucket asc,
    case when m.title_position > 0 then m.title_position else 9999 end asc,
    m.title asc
  limit (select row_limit from normalized);
$$;
