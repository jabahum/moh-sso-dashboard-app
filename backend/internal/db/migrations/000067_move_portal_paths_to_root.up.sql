-- The portal moved from /portal/ to the host root. Rewrite stored portal routes
-- (/portal -> /, /portal/x -> /x) in launch URLs, authenticated-launch returnTo
-- targets and side navigation. The originals are saved so rollback is exact.
DO $$
DECLARE
    app RECORD;
    new_launch TEXT;
    new_auth TEXT;
    new_navigation TEXT;
    new_metadata JSONB;
BEGIN
    FOR app IN SELECT id, launch_url, metadata, lifecycle_status, enabled FROM ihp_systems LOOP
        new_launch := CASE
            WHEN app.launch_url IN ('/portal', '/portal/') THEN '/'
            WHEN app.launch_url LIKE '/portal/%' THEN substr(app.launch_url, 8)
            ELSE app.launch_url END;
        new_auth := regexp_replace(
            regexp_replace(app.metadata->>'authenticatedLaunchUrl',
                '([?&]returnTo=)%2Fportal%2F', '\1%2F', 'gi'),
            '([?&]returnTo=)%2Fportal(&|#|$)', '\1%2F\2', 'gi');
        new_navigation := regexp_replace(
            regexp_replace(app.metadata->>'navigation',
                '("path"[[:space:]]*:[[:space:]]*")/portal/', '\1/', 'g'),
            '("path"[[:space:]]*:[[:space:]]*")/portal"', '\1/"', 'g');

        IF new_launch IS NOT DISTINCT FROM app.launch_url
           AND new_auth IS NOT DISTINCT FROM app.metadata->>'authenticatedLaunchUrl'
           AND new_navigation IS NOT DISTINCT FROM app.metadata->>'navigation' THEN
            CONTINUE;
        END IF;

        new_metadata := app.metadata || jsonb_build_object('portalPathsBeforeRootMove', jsonb_build_object(
            'launchUrl', app.launch_url,
            'authenticatedLaunchUrl', app.metadata->'authenticatedLaunchUrl',
            'navigation', app.metadata->'navigation'));
        IF new_auth IS NOT NULL THEN
            new_metadata := jsonb_set(new_metadata, '{authenticatedLaunchUrl}', to_jsonb(new_auth));
        END IF;
        IF new_navigation IS NOT NULL THEN
            new_metadata := jsonb_set(new_metadata, '{navigation}', to_jsonb(new_navigation));
        END IF;

        UPDATE ihp_systems SET launch_url = new_launch, metadata = new_metadata, updated_at = now()
            WHERE id = app.id;
        -- The lifecycle trigger demotes any configuration edit. This path rewrite
        -- does not change what the application is, so restore the prior state.
        UPDATE ihp_systems SET lifecycle_status = app.lifecycle_status, enabled = app.enabled
            WHERE id = app.id;
        INSERT INTO ihp_rbac_audit_events (action, resource_type, resource_id, details)
            VALUES ('system.portal_paths.moved_to_root', 'system', app.id::text,
                jsonb_build_object('launchUrl', new_launch,
                    'lifecyclePreserved', app.lifecycle_status, 'enabledPreserved', app.enabled));
    END LOOP;
END;
$$;
