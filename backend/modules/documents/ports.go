package documents

import (
	"context"
	"io"

	"github.com/google/uuid"
)

// Location is the part of a storage location the module needs: which
// file-storage provider holds the objects written to it.
type Location struct {
	ID       uuid.UUID
	Provider string
	BaseURI  string
}

// StorageLocations resolves a configured storage location by ID. The host
// adapts the storage-locations module's Service.
type StorageLocations interface {
	GetByID(ctx context.Context, id string) (Location, error)
}

// FileStore reads and writes objects in one file-storage provider.
type FileStore interface {
	Upload(ctx context.Context, objectKey string, r io.Reader, size int64, contentType string) error
	Download(ctx context.Context, objectKey string) (io.ReadCloser, error)
	Delete(ctx context.Context, objectKey string) error
	GetObjectURL(ctx context.Context, objectKey string) (string, error)
	GetDownloadURL(ctx context.Context, objectKey string, filename string) (string, error)
	GetViewURL(ctx context.Context, objectKey string, filename string) (string, error)
}

// FileStores opens the file store for a storage location's provider (for
// example "local" or "s3"). The host adapts its storage factory.
type FileStores interface {
	Get(provider string) (FileStore, error)
}

// ProcessExecutor runs one claimed processing job to completion, recording
// the job and document outcome. The host supplies its import processors.
type ProcessExecutor interface {
	Execute(ctx context.Context, processID uuid.UUID) error
}
