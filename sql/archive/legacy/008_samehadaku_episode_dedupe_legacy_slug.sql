DELETE FROM public.samehadaku_episode_details legacy
USING public.samehadaku_episode_details canonical
WHERE legacy.anime_slug = canonical.anime_slug
  AND legacy.episode_number = canonical.episode_number
  AND legacy.episode_slug LIKE '%subtitle-indonesia%'
  AND canonical.episode_slug NOT LIKE '%subtitle-indonesia%';
