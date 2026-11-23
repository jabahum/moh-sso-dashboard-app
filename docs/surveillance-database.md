# Observatory database for surveillance

This page describes the separate Observatory database used by the surveillance module: how it connects and recovers, how shared geography is owned and synchronized with the primary database, how to run it locally, import surveillance CSV data, and plan a cutover. It is for developers and operators working on surveillance data.

The surveillance module uses a separate Observatory PostgreSQL connection while authentication,
documents, processes, notifications and other features remain on the primary database.
Connection settings are still required and validated, but an unreachable database no
longer blocks the portal from starting. There is no primary-database fallback.

## Connection monitoring and availability

A background monitor checks Observatory every 15 seconds with a three-second ping
deadline. It initializes the dedicated migrations before enabling surveillance and
checks migrations again after reconnecting. Shared repository pools reconnect without
restarting the backend. Failed migrations keep surveillance unavailable; the monitor
never forces a dirty migration version or rewinds data. Credentials or endpoint
changes still require a backend redeployment to load the updated private environment.

During initialization or an outage, surveillance routes return `503` with code
`OBSERVATORY_UNAVAILABLE` and `Retry-After: 15`. Dashboard and disease pages show a
clear temporary-unavailability message and retry failed queries automatically.
Health contexts and their full geography hierarchy read the primary database directly.
No HTTP health-context operation connects to Observatory. Core readiness continues to
require its existing primary, remote, Redis and identity dependencies; Observatory
is reported separately as `unavailable` in aggregated health with status `degraded`.

## Relationship to the DQA connection

DQA uses `Config.DqaDbSource()` and the shared `db.InitDB` connection-pool
initializer. It targets the DWH host, port and database, optionally using its own
`DQA_DB_USERNAME`/`DQA_DB_PASSWORD`. When those are unset it uses DWH credentials.
`NewDQAStore` creates its `dqa` schema/tables idempotently on first use.

The Observatory connection uses the same pool settings and repository injection
pattern, with a nonblocking initializer and its own database. Development uses the
container described below; production uses the external `OBSERVATORY_DB_*` endpoint
without provisioning a database container. Versioned surveillance
migrations remain in place. Authentication and other primary stores are unaffected.
`OBSERVATORY_DB_*` is the only connection namespace. The PostgreSQL role remains
`surveillance` and the existing populated volume and versioned migrations are retained.
The local database container is `sso-observatory-db` on the main development network.

## Observatory configuration names

Settings are `OBSERVATORY_DB_HOST`, `OBSERVATORY_DB_PORT`,
`OBSERVATORY_DB_NAME`, `OBSERVATORY_DB_USER`, `OBSERVATORY_DB_PASSWORD`, and
`OBSERVATORY_DB_ENABLE_SSL`. The host and complete credentials are required.
There is no fallback to the primary database. An exact primary host/port/database
target is rejected; configure distinct database destinations even when host aliases
are used. Previous `SURVEILLANCE_DB_*`
settings are no longer read: migrate private environment and password-file keys
before restarting an existing installation with this version. The local host-process
example is `infra/env/observatory.env.example`; Compose sets its internal hostname
and port explicitly.

The standalone `cmd/surveillance-db` initializer uses the same Observatory settings.
Its command and the `internal/db/surveillance_migrations` directory retain their
module names; existing migration versions and data are preserved. Enable the import
integration test with `OBSERVATORY_DOCKER_TEST=1`. The loader's `--db-user` selects
a non-default role without renaming the role in an existing container.

## Local container

The `observatory-db` service lives in `infra/compose/docker-compose.dev.yml`, alongside
`remote-postgres`. The backend uses `observatory-db:5432` on the shared `sso_network`
and waits for its health check. Starting the main development stack starts both databases.
A fresh stack creates the `observatory_data` named volume. Development-only default
credentials can be overridden using `OBSERVATORY_DB_USER`, `OBSERVATORY_DB_NAME`
and `OBSERVATORY_DB_PASSWORD`.
The same values configure PostgreSQL and the backend.

```sh
docker compose -f infra/compose/docker-compose.dev.yml up -d --wait
```

The container is `sso-observatory-db`, also the CSV loader default. Host access is limited
to `127.0.0.1:55434`; `OBSERVATORY_DB_LOCAL_PORT` overrides that published port.
The database defaults to `observatory` and the existing role to `surveillance`.
A backend running directly on the host uses this loopback address/port instead.
Do not use `down -v`; it removes managed data volumes.

### Adopting the existing standalone volume

For an existing standalone container, back up the database first and inspect its
`/var/lib/postgresql/data` volume. Stop the backend and old database, then remove
only the old database container. Do not remove its volume. Use a private override
so the main project adopts the populated volume instead of creating an empty one:

```yaml
volumes:
  observatory_data:
    external: true
    name: moh-surveillance_surveillance_data
```

The current workspace keeps this override at `.local-sso/observatory-volume.override.json`
and credentials at `.local-sso/observatory.env`, both mode 0600. Its saved backend
environment remains private in `.local-sso/backend-before-surveillance.override.json`.
Start the adopted service and backend with:

```sh
docker compose --env-file .local-sso/observatory.env -f infra/compose/docker-compose.dev.yml -f .local-sso/backend-before-surveillance.override.json -f .local-sso/observatory-volume.override.json up -d --no-deps --wait observatory-db backend
```

Check readiness and compare all table counts before resuming writes. The main
project now manages both containers; the separate surveillance Compose files and
external network override are no longer needed. The existing data volume remains
external and is retained by Compose teardown. No production configuration is changed.

Keep the adopted-volume override on subsequent local Compose commands. Setting
an empty Observatory host is no longer a rollback mechanism; it prevents startup.

Load private variables into the environment without printing them, then, from
`backend/`, initialize the schema with `go run ./cmd/surveillance-db`. The backend
also applies the dedicated versioned migrations automatically. File-backed passwords
remain supported through `OBSERVATORY_DB_PASSWORD_FILE` in the backend entrypoint.

### Existing local container rename

The old container name was `sso-surveillance-db`. Stop the backend, then stop and
remove only that old container before starting `observatory-db` with the main Compose
file, private Observatory environment and external-volume override described above.
Compose creates `sso-observatory-db` using the same volume. Do not remove or recreate
the volume, PostgreSQL role, database, tables or migration history. Keep a private
backup and verify table counts and readiness after the change.

## Rename an existing local database

Changing `POSTGRES_DB` does not rename a database in an existing volume. For an
already populated container, take a backup, stop the local backend, and rename the
database while connected to `postgres`:

```sh
docker exec sso-observatory-db psql -X -v ON_ERROR_STOP=1 -U surveillance -d postgres -c 'ALTER DATABASE surveillance RENAME TO observatory;'
```

All connections to the old database must be closed first. This preserves tables,
IDs, ownership, migration history, staging and facts; it does not reimport data.
Set the private `OBSERVATORY_DB_NAME=observatory`, recreate the database container
without removing its volume to update its health check, then recreate the backend
using the updated database name. Compare table counts and verify readiness. Keep the
backup private. Rollback uses the inverse rename while the backend is stopped and
restores the prior connection name. Production database changes are separate from
this local procedure.

The CSV loader defaults to `observatory`; `--database surveillance` supports an
existing database before its rename, or an explicit alternate database during
verification. The PostgreSQL role remains `surveillance`.

## Table ownership

The isolated domain schema contains 13 tables (plus three geography-replica bookkeeping tables): `regions`, `districts`, `sub_counties`,
`facilities`, `diseases`, `indicators`, `disease_indicators`, `epi_weeks`,
`facility_weekly_metrics`, `weekly_status`, `alerts`, `surveillance_import_batches`
and `surveillance_import_raw_rows`. It also owns `risk_level` and `alert_status`.
Staging retains its batch lookup index without a JSON GIN index, keeping bulk status
updates efficient. Subject/calendar and geography reference seeds are included.
The schema has its own migration history. Existing primary migrations remain
unchanged so upgrades and existing IDs are not silently invalidated.

## Shared geography ownership

Primary migration `000060_primary_geography_registry` creates `geography.regions`,
`geography.districts`, `geography.sub_counties` and `geography.facilities`. The
missing-only historical reference snapshot supplies 17 regions, 146 districts,
177 legacy sub-county/reporting labels and 6,150 facilities even when Observatory
has never connected. Existing primary health-context aliases are adopted first,
preserving context UUIDs, assignments and disabled state. Registry row triggers
maintain context nodes and hierarchy in the same primary transaction.

### Observatory replicas

Observatory retains local geography tables for its fact foreign keys. Its new
replica metadata maps canonical primary UUIDs to existing local UUIDs; synchronization
never deletes reference rows, changes existing Observatory UUIDs or rewrites facts.
`geography-*` context aliases identify canonical rows, while `surveillance-*` aliases
identify the corresponding Observatory rows. The identifiers can differ safely.

### Adoption and synchronization

The first successful connection adopts existing Observatory geography and metadata
into primary once, including custom locations. Its adoption marker commits in primary
before replica publication, so an interrupted destination update cannot overwrite
later primary edits by adopting again. Subsequent updates flow only **primary ->
Observatory**, approximately once per minute and immediately after location writes.
The 15-second connection monitor retries failures. Surveillance becomes available
only after migrations, initial synchronization and destination aliases succeed.

### Location APIs and conflicts

Location APIs and transactional metric-import reference creation use a primary-aware
store. Parent IDs supplied by surveillance are resolved to canonical IDs; responses
continue to use Observatory IDs. Facts, diseases, indicators, weeks, alerts and import
staging remain in Observatory. Unknown/ambiguous identities or a replica already
bound to another primary registry stop synchronization instead of guessing or
merging unrelated areas. Operators must resolve those conflicts before surveillance
can be enabled; other portal features remain available.

### Deletion

Shared geography cannot be physically deleted through surveillance APIs: sub-county
delete returns `409 GEOGRAPHY_DELETE_RESTRICTED`. Disable the associated health
context to withdraw access while preserving historical records. Assigned or
registry-backed context nodes cannot be deleted either. This implementation does
not propagate deletions from either database.

### Backup, restore and rollback

Back up **both databases** before this cutover. Keep primary and Observatory UUIDs,
replica identity tables and adoption markers together during restores. A primary
replacement must restore its registry identity; do not manually clear replica origin
or adoption markers to bypass an ownership conflict. Both new down migrations retain
data, and reapplying them preserves IDs. Existing applied migrations are unchanged. An older backend image must be supplied
with these retained migration files before binary rollback; otherwise its migration
runner cannot recognize the newer database version. Do not force migration history
backwards or drop geography to make an old image start.
New geography corrections should be primary migrations, not edits to the historic
Observatory seed. The historical reference labels are not a verified current
administrative or facility registry.

## Automatic geography reference data

Observatory migration `000003_reference_geography` supplies 17 regions, 146 districts,
177 legacy sub-county/reporting labels, and 6,150 facilities. The backend applies this
migration before serving surveillance requests, on fresh databases and when upgrading an existing
Observatory database. Production images already include the complete migration directory;
no CSV import, primary-database connection, or network download is needed to populate these lists.

This versioned snapshot restores the geography originally bundled in primary migrations
7–10 at repository commit `34e056ce1df7514a639841971bfbcc108364bf00`, before database
isolation removed those seeds. Blank and `nan`/`null`/`none` placeholder names are excluded.
It includes legacy reporting labels such as `DLG`; these are not a replacement for
the bundled UBOS map boundaries or a claim that the historical facility registry is current.

Inserts match existing location names without case/outer-whitespace differences, and
facilities by external identifier or name within a district. Existing IDs, spelling,
metadata, parent links, locally added records, and reports are preserved. Missing facilities
inherit their resolved district's current region. A populated database may therefore have
more reference rows than the baseline. Restarting containers preserves the database volume;
startup does not reset the registry or reinsert rows intentionally removed after migration.
Rolling this migration back also retains its reference rows so dependent reports are safe.

Maintain future shared-geography additions or verified corrections in a **new primary migration**;
do not edit an already applied snapshot migration. Use external identifiers and existing
natural keys to preserve identity. Update the baseline assertions in
`backend/internal/migrate/migrations_integration_test.go` alongside a new version, and run
`make migrations-check`. Corrections that overwrite existing metadata should target specific
verified records explicitly. Test observations, weekly status flags and alerts remain separate
imports and are never seeded by this geography migration.

`surveillance_import_batches.document_id` is an opaque ID into the primary documents
service, not a foreign key across databases. Documents/processes stay on the primary
connection while the import repository uses the surveillance connection.

## Primary migration ownership

Primary migration versions 7–19 are now explicit no-ops in both directions. Their
numbers are retained so existing migration histories can advance without replaying
schema creation or deleting old tables during rollback. Fresh primary installs no
longer create the 13 surveillance tables, their enums or reference seeds. Their
schema/seeds are maintained only in `internal/db/surveillance_migrations`; existing
Observatory migration versions and data are unchanged.

Already deployed primary tables remain intact for an explicit, backed-up data
transfer and later removal. This code change does not transfer or delete production
data. The SQLC schema input includes both migration directories so query bindings
still describe both stores; runtime repository wiring keeps the connections separate.

## Import the root folder

```sh
python3 infra/scripts/load-surveillance.py --flag-year 2026
```

### Batches and staging

Each file gets a batch and each CSV record is retained as JSON in raw staging. The
loader uses a disk spool and PostgreSQL COPY, not one database request per source row.
The filename, SHA-256 checksum and explicit flag year identify a batch, preventing an
unchanged file from being staged again. `--stage-only` stops before transformation.
`--reprocess` transactionally rebuilds only the selected loader-owned source batches;
all source rows remain available for audit. Ordinary retries are idempotent.
Bulk SQL imports resolve existing replicated geography only. Unknown locations stay
in failed staging with an explicit message; register them in primary and synchronize
before retrying. The loader never creates an Observatory-only geography registry.

### Source files

- `data.csv`: location references and facility disease/indicator metrics. Original
  years are preserved. Exact known indicator names remain indicators; the suffix
  ` Cases` maps case labels to disease names. The loader does not sum malaria testing
  indicators into disease cases or invent epidemiological definitions.
- `eidsr.csv`: alerts, matching district names case-insensitively and deriving the
  year from the created date. Missing district/date/week/subject/narrative is held.
  Identical alert identities map to one alert while all source rows remain staged.
- `flags.csv`, `region_flags.csv`, `district_flags.csv`: national, regional and district
  weekly status. Each subject in a color list becomes a status. Lists are parsed with
  `ast.literal_eval`, never evaluated as executable code. `Maternal Death` is mapped
  to the seeded `Maternal Deaths` indicator; source text stays in raw JSON.

### Flag year and weeks

The agreed flag year is **2026**. Weeks **1–52 remain unchanged**. Source week **0**
is marked FAILED with a review reason and is never folded into week 1.

### Duplicates, conflicts and batch status

Identical facility/subject/week values collapse to one fact with a deterministic
source record, while each identical source row is counted as processed. If the values
conflict, **every member of that group is held for review**. Missing numeric values are
not treated as zero. Source row counts therefore need not equal normalized fact counts.
A batch can be COMPLETED with failed rows; inspect `success_rows`, `failed_rows`, and
raw-row error messages rather than treating batch status as full data acceptance.

### Limitations

This loader is an operational folder-import path. It does not repair the existing
surveillance UI's metadata-only upload action or its incomplete async CSV dispatch.
Those paths must be addressed separately before promising the same behavior for UI uploads.

## Existing database cutover

The local CSV import does not copy or delete existing production database records.
Before a real migration, inventory and back up all 13 tables, preserve their UUIDs,
restore them together into the new schema with constraints intact, and reconcile row
counts, foreign keys, batch outcomes and dashboard results. Seeded reference IDs in a
new database must not be mixed with old fact IDs. Pause surveillance writes during the
final copy/cutover, change only its connection, and retain the original tables for
rollback. Removal of legacy tables requires a separate reviewed migration.

## Verification

```sh
python3 infra/scripts/test-load-surveillance.py
OBSERVATORY_DOCKER_TEST=1 python3 infra/scripts/test-surveillance-database.py
cd backend && go test ./...
```

The Docker test uses a disposable PostgreSQL container and synthetic data; it checks
schema independence from documents, all five file mappings, identical/conflicting
duplicates, zero-week quarantine, and repeat/reprocess behavior. It runs in PR CI.

SQLC schema input includes both database migration directories. Enum declarations
are top-level DDL so SQLC and PostgreSQL resolve the same types. From `backend/`,
run `sqlc compile` and `sqlc generate` with SQLC v1.30.0. CI verifies compilation
and that generated query bindings match the checked-in files.

## Verified local import (2026 flag year)

The initial import retained 796,427 source rows and produced 648,111 facility
metrics, 14,693 weekly status records and 579 alerts. 789,000 source rows were
processed; 7,427 were held for review: 6,771 conflicting metrics, 440 invalid
metrics, 51 alerts and 165 source-week-zero flag rows. There were no pending rows.
Identical observations and alert identities explain why processed source counts
are larger than normalized record counts. These are local fixture/import results,
not evidence of a production migration or cutover.
