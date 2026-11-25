-- Restore the /portal-prefixed values saved by the up migration.
DO $$
DECLARE
    app RECORD;
    saved JSONB;
    restored JSONB;
BEGIN
    FOR app IN SELECT id, metadata, lifecycle_status, enabled FROM ihp_systems
        WHERE metadata ? 'portalPathsBeforeRootMove'
    LOOP
        saved := app.metadata->'portalPathsBeforeRootMove';
        restored := app.metadata - 'portalPathsBeforeRootMove' - 'authenticatedLaunchUrl' - 'navigation';
        IF saved->'authenticatedLaunchUrl' IS NOT NULL AND jsonb_typeof(saved->'authenticatedLaunchUrl') <> 'null' THEN
            restored := restored || jsonb_build_object('authenticatedLaunchUrl', saved->'authenticatedLaunchUrl');
        END IF;
        IF saved->'navigation' IS NOT NULL AND jsonb_typeof(saved->'navigation') <> 'null' THEN
            restored := restored || jsonb_build_object('navigation', saved->'navigation');
        END IF;
        UPDATE ihp_systems SET launch_url = saved->>'launchUrl', metadata = restored, updated_at = now()
            WHERE id = app.id;
        UPDATE ihp_systems SET lifecycle_status = app.lifecycle_status, enabled = app.enabled
            WHERE id = app.id;
    END LOOP;
END;
$$;
