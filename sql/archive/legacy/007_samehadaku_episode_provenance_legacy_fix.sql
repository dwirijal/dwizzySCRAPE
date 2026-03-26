UPDATE public.samehadaku_episode_details
SET effective_source_url = secondary_source_url,
    effective_source_domain = secondary_source_domain,
    effective_source_kind = 'secondary',
    fetch_status = CASE
        WHEN fetch_status IN ('', 'pending') THEN 'legacy_secondary_only'
        ELSE fetch_status
    END
WHERE secondary_source_url <> ''
  AND primary_source_url <> ''
  AND effective_source_url = primary_source_url
  AND effective_source_kind = 'primary';
