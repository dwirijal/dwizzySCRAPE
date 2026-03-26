UPDATE public.samehadaku_episode_details
SET secondary_source_url = '',
    secondary_source_domain = '',
    effective_source_url = primary_source_url,
    effective_source_domain = primary_source_domain,
    effective_source_kind = 'primary',
    fetch_status = CASE
        WHEN fetch_status IN ('primary_fetched_secondary_fetched', 'primary_redirected_offsite_secondary_fetched')
            THEN 'primary_fetched'
        ELSE fetch_status
    END,
    fetch_error = CASE
        WHEN fetch_status IN ('primary_fetched_secondary_fetched', 'primary_redirected_offsite_secondary_fetched')
            THEN ''
        ELSE fetch_error
    END,
    source_meta_json = jsonb_set(
        jsonb_set(
            jsonb_set(source_meta_json, '{effective_source_kind}', to_jsonb('primary'::text), true),
            '{effective_source_url}',
            to_jsonb(primary_source_url),
            true
        ),
        '{parser_source_kind}',
        to_jsonb('primary'::text),
        true
    )
WHERE primary_source_url <> ''
  AND secondary_source_url = primary_source_url
  AND primary_source_domain = secondary_source_domain;
