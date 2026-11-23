// Package documents is the Documents application module: uploaded files, their
// metadata and storage, spreadsheet structure detection, imported-data preview
// and the queue of processing jobs created for each upload.
//
// It owns its HTTP API, OpenAPI contract, persistence and the document
// processing worker loop. The platform host supplies verified access, the
// selected health context, notifications, rate limiting, the primary and upload
// databases, storage-location lookup, file-storage providers and the import
// processors that execute claimed jobs. The documents, processes and import
// tables stay in the host's migrations.
package documents
