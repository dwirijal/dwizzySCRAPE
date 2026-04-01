UPDATE public.media_items i
SET
    item_key = 'samehadaku:movie:' || i.slug,
    media_type = 'movie',
    updated_at = timezone('utc', now())
WHERE i.source = 'samehadaku'
  AND i.media_type = 'anime'
  AND lower(coalesce(i.detail->>'anime_type', '')) = 'movie'
  AND NOT EXISTS (
      SELECT 1
      FROM public.media_units u
      WHERE u.item_key = i.item_key
  )
  AND NOT EXISTS (
      SELECT 1
      FROM public.media_items x
      WHERE x.item_key = 'samehadaku:movie:' || i.slug
  );
