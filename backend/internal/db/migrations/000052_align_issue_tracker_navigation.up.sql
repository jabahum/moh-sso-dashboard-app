-- Navigation-only alignment: keep each application's original lifecycle/access state.
-- Save the original item so rollback does not overwrite unrelated navigation changes.
DO $$
DECLARE
    app RECORD;
    navigation JSONB;
    aligned JSONB;
    original_item JSONB;
BEGIN
    FOR app IN SELECT id, metadata, lifecycle_status, enabled FROM ihp_systems
        WHERE metadata->>'navigation' IS NOT NULL
    LOOP
        BEGIN
            navigation := (app.metadata->>'navigation')::jsonb;
        EXCEPTION WHEN invalid_text_representation THEN
            CONTINUE;
        END;
        IF jsonb_typeof(navigation) <> 'array' THEN CONTINUE; END IF;
        SELECT item INTO original_item FROM jsonb_array_elements(navigation) AS item
            WHERE item->>'path' = '/apps/dwh/issue-tracker' LIMIT 1;
        IF original_item IS NULL THEN CONTINUE; END IF;
        SELECT jsonb_agg(CASE WHEN item->>'path' = '/apps/dwh/issue-tracker' THEN
            item || CASE WHEN item->>'label' = 'Issue Tracking'
                THEN '{"label":"Issue Tracker"}'::jsonb ELSE '{}'::jsonb END ||
            CASE WHEN jsonb_typeof(item->'children') = 'array' THEN
                jsonb_build_object('children', COALESCE((SELECT jsonb_agg(child || '{"displayInLauncher":false}'::jsonb ORDER BY child_order)
                    FROM jsonb_array_elements(item->'children') WITH ORDINALITY AS children(child, child_order)), '[]'::jsonb))
                ELSE '{}'::jsonb END
            ELSE item END ORDER BY item_order)
        INTO aligned FROM jsonb_array_elements(navigation) WITH ORDINALITY AS items(item, item_order);
        IF aligned = navigation THEN CONTINUE; END IF;
        UPDATE ihp_systems SET metadata = jsonb_set(metadata, '{navigation}', to_jsonb(aligned::text)) ||
            jsonb_build_object('issueTrackerNavigationBeforeAlignment', original_item) WHERE id = app.id;
        -- The configuration trigger intentionally invalidates any metadata edit. This
        -- audited, display-only migration restores precisely the prior state.
        UPDATE ihp_systems SET lifecycle_status = app.lifecycle_status, enabled = app.enabled WHERE id = app.id;
        INSERT INTO ihp_rbac_audit_events (action, resource_type, resource_id, details)
            VALUES ('system.navigation.aligned', 'system', app.id::text,
                jsonb_build_object('module', 'issue-tracker', 'lifecyclePreserved', app.lifecycle_status, 'enabledPreserved', app.enabled));
    END LOOP;
END;
$$;
