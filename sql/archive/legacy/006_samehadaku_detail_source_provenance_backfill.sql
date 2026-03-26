UPDATE public.samehadaku_anime_details
SET primary_source_url = CASE
        WHEN primary_source_url = '' THEN canonical_url
        ELSE primary_source_url
    END,
    primary_source_domain = CASE
        WHEN primary_source_domain = '' THEN split_part(replace(replace(canonical_url, 'https://', ''), 'http://', ''), '/', 1)
        ELSE primary_source_domain
    END,
    effective_source_url = CASE
        WHEN effective_source_url = '' THEN canonical_url
        ELSE effective_source_url
    END,
    effective_source_domain = CASE
        WHEN effective_source_domain = '' THEN split_part(replace(replace(canonical_url, 'https://', ''), 'http://', ''), '/', 1)
        ELSE effective_source_domain
    END,
    effective_source_kind = CASE
        WHEN effective_source_kind = '' THEN 'primary'
        ELSE effective_source_kind
    END;

UPDATE public.samehadaku_episode_details
SET primary_source_url = CASE
        WHEN primary_source_url = '' THEN 'https://v2.samehadaku.how/' || episode_slug || '/'
        ELSE primary_source_url
    END,
    primary_source_domain = CASE
        WHEN primary_source_domain = '' THEN 'v2.samehadaku.how'
        ELSE primary_source_domain
    END,
    secondary_source_url = CASE
        WHEN secondary_source_url = '' AND canonical_url NOT LIKE 'https://v2.samehadaku.how/%' THEN canonical_url
        ELSE secondary_source_url
    END,
    secondary_source_domain = CASE
        WHEN secondary_source_domain = '' AND canonical_url NOT LIKE 'https://v2.samehadaku.how/%' THEN split_part(replace(replace(canonical_url, 'https://', ''), 'http://', ''), '/', 1)
        ELSE secondary_source_domain
    END,
    effective_source_url = CASE
        WHEN effective_source_url = '' AND canonical_url NOT LIKE 'https://v2.samehadaku.how/%' THEN canonical_url
        WHEN effective_source_url = '' THEN 'https://v2.samehadaku.how/' || episode_slug || '/'
        ELSE effective_source_url
    END,
    effective_source_domain = CASE
        WHEN effective_source_domain = '' AND canonical_url NOT LIKE 'https://v2.samehadaku.how/%' THEN split_part(replace(replace(canonical_url, 'https://', ''), 'http://', ''), '/', 1)
        WHEN effective_source_domain = '' THEN 'v2.samehadaku.how'
        ELSE effective_source_domain
    END,
    effective_source_kind = CASE
        WHEN effective_source_kind = '' AND canonical_url NOT LIKE 'https://v2.samehadaku.how/%' THEN 'secondary'
        WHEN effective_source_kind = '' THEN 'primary'
        ELSE effective_source_kind
    END,
    fetch_status = CASE
        WHEN fetch_status = '' AND canonical_url NOT LIKE 'https://v2.samehadaku.how/%' THEN 'legacy_secondary_only'
        WHEN fetch_status = '' THEN 'pending'
        ELSE fetch_status
    END,
    canonical_url = CASE
        WHEN canonical_url NOT LIKE 'https://v2.samehadaku.how/%' THEN 'https://v2.samehadaku.how/' || episode_slug || '/'
        ELSE canonical_url
    END;
