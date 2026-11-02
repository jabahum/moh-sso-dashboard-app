DO $$
DECLARE
    app RECORD;
    restored JSONB;
BEGIN
    FOR app IN SELECT id, metadata, lifecycle_status, enabled FROM ihp_systems
        WHERE metadata ? 'issueTrackerNavigationBeforeAlignment'
    LOOP
        SELECT jsonb_agg(CASE WHEN item->>'path' = '/apps/dwh/issue-tracker'
            THEN app.metadata->'issueTrackerNavigationBeforeAlignment' ELSE item END ORDER BY item_order)
        INTO restored FROM jsonb_array_elements((app.metadata->>'navigation')::jsonb) WITH ORDINALITY AS items(item, item_order);
        UPDATE ihp_systems SET metadata = jsonb_set(metadata - 'issueTrackerNavigationBeforeAlignment',
            '{navigation}', to_jsonb(restored::text)) WHERE id = app.id;
        UPDATE ihp_systems SET lifecycle_status = app.lifecycle_status, enabled = app.enabled WHERE id = app.id;
    END LOOP;
END;
$$;
