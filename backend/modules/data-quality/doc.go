// Package dataquality is the Data Quality application module: the issue
// tracker (issues, assignments, resolution transactions and their emails) and
// the DQA rule engine (table registration, declarative rules compiled to SQL by
// the dqa subpackage, scans, scheduled scans and completeness runs).
//
// It owns its HTTP API, OpenAPI contract, persistence and the scheduled-run
// worker. The platform host supplies verified access, the selected health
// context, the databases, the identity-provider directory and email delivery
// through narrow ports; the primary-schema tables stay in the host's
// migrations, while the warehouse-side dqa schema self-migrates at startup as
// it did before extraction.
package dataquality
