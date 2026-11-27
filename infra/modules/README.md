# Pending backend module releases

`releases.json` lists the release each changed `backend/modules/<id>` module
will become. A pull request that changes a module's source adds an entry:

```json
{
  "releases": {
    "announcements": {
      "version": "v0.4.0",
      "notes": "Accept core.Platform instead of individual callbacks."
    }
  }
}
```

`infra/scripts/module_release.py check` (run by CI) requires the version to be
newer than the module's pin in `backend/go.mod` and not yet tagged. After the
pull request merges, the Module Release workflow tags
`backend/modules/<id>/<version>`, verifies the published artifact and opens a
pull request that pins it and removes the entry. See
[Release a module version](../../docs/modular-distribution.md#release-a-module-version).
