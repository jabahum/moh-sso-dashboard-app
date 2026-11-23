// Package visualiser is the Data Visualiser application module: read-only
// queries over the data warehouse (datasets, data elements and aggregated data
// values) used by the data-visualizer frontend.
//
// It owns its HTTP API, OpenAPI contract and warehouse SQL. The platform host
// supplies verified access, the warehouse database and the health-context
// directory through the core/modulekit contracts. Data-value queries are
// constrained to the organisation units aliased to the caller's selected
// health context. The warehouse schema is external and is not migrated here.
package visualiser
