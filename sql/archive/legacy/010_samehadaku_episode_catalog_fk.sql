DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'samehadaku_episode_details_anime_slug_fkey'
          AND conrelid = 'public.samehadaku_episode_details'::regclass
    ) THEN
        ALTER TABLE public.samehadaku_episode_details
            DROP CONSTRAINT samehadaku_episode_details_anime_slug_fkey;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'samehadaku_episode_details_anime_slug_fkey'
          AND conrelid = 'public.samehadaku_episode_details'::regclass
          AND confrelid = 'public.samehadaku_anime_catalog'::regclass
    ) THEN
        ALTER TABLE public.samehadaku_episode_details
            ADD CONSTRAINT samehadaku_episode_details_anime_slug_fkey
            FOREIGN KEY (anime_slug)
            REFERENCES public.samehadaku_anime_catalog(slug)
            ON DELETE CASCADE;
    END IF;
END $$;
