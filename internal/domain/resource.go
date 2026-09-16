package domain

import (
	"context"
	"time"
)

type ResourceStatus string

const (
	ResourceStatusPending  ResourceStatus = "PENDING"
	ResourceStatusApproved ResourceStatus = "APPROVED"
	ResourceStatusRejected ResourceStatus = "REJECTED"
)

// Resource represents an uploaded file metadata
type Resource struct {
	ID           int64          `json:"id"`
	OwnerID      *int64         `json:"owner_id"`  // Nullable for anonymous uploads
	FolderID     *int64         `json:"folder_id"` // NULL means drive root
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	Filename     string         `json:"filename"`      // stored file name on disk
	OriginalName string         `json:"original_name"` // original filename uploaded
	Size         int64          `json:"size"`
	FileHash     string         `json:"file_hash"` // SHA256 hash for duplicate check
	Status       ResourceStatus `json:"status"`    // PENDING, APPROVED, REJECTED
	CreatedAt    time.Time      `json:"created_at"`
	DeletedAt    *time.Time     `json:"deleted_at,omitempty"` // soft delete (trash)
	Subject      string         `json:"subject,omitempty"`
	Type         string         `json:"type,omitempty"`
	URL          string         `json:"url,omitempty"` // Public URL for the file

	// Version history: rows sharing VersionGroup are versions of one logical file.
	VersionGroup string `json:"version_group,omitempty"`
	Version      int    `json:"version"`
	IsLatest     bool   `json:"is_latest"`
}

// ResourceRepository defines methods for resource persistence
type ResourceRepository interface {
	Create(ctx context.Context, resource *Resource) error
	List(ctx context.Context, status ResourceStatus, search string) ([]Resource, error)
	GetByID(ctx context.Context, id int64) (*Resource, error)
	UpdateStatus(ctx context.Context, id int64, status ResourceStatus) error
	GetByHash(ctx context.Context, hash string) ([]Resource, error)

	// Drive operations
	ListByFolder(ctx context.Context, ownerID int64, folderID *int64, search string) ([]Resource, error)
	Update(ctx context.Context, resource *Resource) error // rename / move
	SoftDelete(ctx context.Context, id int64) error
	SoftDeleteByFolder(ctx context.Context, folderID int64) error
	ListDeleted(ctx context.Context, ownerID int64) ([]Resource, error)
	ListByFolderIncludingDeleted(ctx context.Context, folderID int64) ([]Resource, error)
	Restore(ctx context.Context, id int64, folderID *int64) error
	RestoreByFolder(ctx context.Context, folderID int64) error
	HardDelete(ctx context.Context, id int64) error
	HardDeleteByFolder(ctx context.Context, folderID int64) error

	// Deduplication / refcount
	// CountByFilename counts rows pointing at the same stored object (shared physical file).
	CountByFilename(ctx context.Context, filename string) (int, error)
	// FindByHash finds a non-deleted row with this hash whose content can be reused (instant upload).
	FindByHash(ctx context.Context, hash string) (*Resource, error)

	// Version history
	// GetLatestByName returns the latest non-deleted file with this name in the folder.
	GetLatestByName(ctx context.Context, ownerID int64, folderID *int64, name string) (*Resource, error)
	// SetLatest flips the is_latest flag of one row.
	SetLatest(ctx context.Context, id int64, latest bool) error
	// SetVersionGroup assigns a version group to a legacy row that has none.
	SetVersionGroup(ctx context.Context, id int64, group string) error
	// ListVersions returns every row of a version group, newest version first.
	ListVersions(ctx context.Context, group string) ([]Resource, error)
	SoftDeleteByGroup(ctx context.Context, group string) error
	RestoreByGroup(ctx context.Context, group string) error
	HardDeleteByGroup(ctx context.Context, group string) error
	// ListDeletedBefore returns soft-deleted rows (any owner) deleted before cutoff, for auto-cleanup.
	ListDeletedBefore(ctx context.Context, cutoff time.Time) ([]Resource, error)
}
