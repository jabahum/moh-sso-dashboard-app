// Package storagelocations is the Storage Locations application module: the
// catalogue of file-storage destinations (provider and base URI) that document
// uploads are written to.
//
// It owns its HTTP API, OpenAPI contract and persistence. The platform host
// supplies verified access, notifications and the database through the
// core/modulekit contracts; the storage_locations table stays in the host's
// primary-schema migrations.
package storagelocations
