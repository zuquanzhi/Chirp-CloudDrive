package domain

import (
	"context"
	"time"
)

// Folder represents a directory in a user's drive
type Folder struct {
	ID        int64      `json:"id"`
	OwnerID   int64      `json:"owner_id"`
	ParentID  *int64     `json:"parent_id"` // NULL means drive root
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"` // soft delete (trash)
}

// FolderRepository defines methods for folder persistence
type FolderRepository interface {
	Create(ctx context.Context, folder *Folder) error
	GetByID(ctx context.Context, id int64) (*Folder, error)
	ListByParent(ctx context.Context, ownerID int64, parentID *int64) ([]Folder, error)
	Update(ctx context.Context, folder *Folder) error
	SoftDelete(ctx context.Context, id int64) error
	// ListDescendantIDs returns ids of all folders under id (exclusive), for trash cascade and move-cycle checks
	ListDescendantIDs(ctx context.Context, id int64) ([]int64, error)
	// ListDeleted returns all soft-deleted folders of a user (trash)
	ListDeleted(ctx context.Context, ownerID int64) ([]Folder, error)
	// Restore clears the soft-delete mark
	Restore(ctx context.Context, id int64) error
	// HardDelete permanently removes the folder row
	HardDelete(ctx context.Context, id int64) error
}
