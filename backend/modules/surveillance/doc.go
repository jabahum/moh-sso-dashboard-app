// Package surveillance is the Surveillance application module: diseases,
// epidemiological weeks, the surveillance geography (regions, districts,
// sub-counties, facilities), facility weekly metrics, weekly risk statuses,
// alerts and the CSV import pipeline that feeds them.
//
// It owns its HTTP API, OpenAPI contract, persistence (the Observatory
// database) and the surveillance import processors. The platform host supplies
// verified access, the health-context hierarchy, notifications, Observatory
// readiness, geography replication and its document pipeline through ports
// and the core/modulekit contracts. The Observatory schema migrations stay
// with the host.
package surveillance
