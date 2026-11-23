# Host adapters for modules

No published module currently needs a transitional adapter here; every module
under `backend/modules` is self-contained. Host services reach modules through
the `core/modulekit` contracts, adapted in `internal/adapters/modulekit`
(notifications, platform details), `internal/features/health_context`
(`ModuleDirectory`) and `internal/modulehttp` (request state, rate limits).

The catalogue and architecture checks still recognise
`internal/adapters/modules/<feature>` for a module with `integration: adapter`,
should one be added. See `docs/module-development.md`.
