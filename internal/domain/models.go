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

type UserRole string

const (
	RoleUser  UserRole = "USER"
	RoleAdmin UserRole = "ADMIN"
)

// DefaultQuota is the per-user storage quota (1GB) assigned at signup.
const DefaultQuota int64 = 1 << 30

// User represents a registered user
type User struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Password    string    `json:"-"`
	Role        UserRole  `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
	PhoneNumber string    `json:"phone_number,omitempty"`
	School      string    `json:"school,omitempty"`
	StudentID   string    `json:"student_id,omitempty"`
	Birthdate   string    `json:"birthdate,omitempty"`
	Address     string    `json:"address,omitempty"`
	Gender      string    `json:"gender,omitempty"`
	Quota       int64     `json:"quota"` // total storage quota in bytes
	Used        int64     `json:"used"`  // used storage in bytes
}

// Resource represents an uploaded file metadata
type Resource struct {
	ID           int64          `json:"id"`
	OwnerID      *int64         `json:"owner_id"` // Nullable for anonymous uploads
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
}

// Folder represents a directory in a user's drive
type Folder struct {
	ID        int64      `json:"id"`
	OwnerID   int64      `json:"owner_id"`
	ParentID  *int64     `json:"parent_id"` // NULL means drive root
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"` // soft delete (trash)
}

// Notification represents a system message
type Notification struct {
	ID        int64     `json:"id"`
	UserID    *int64    `json:"user_id"` // Null for system-wide, or specific user
	Content   string    `json:"content"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

// UserRepository defines methods for user persistence
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	UpdateProfile(ctx context.Context, user *User) error
	// AddUsed adjusts the user's used bytes by delta (negative to decrease)
	AddUsed(ctx context.Context, userID int64, delta int64) error
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

// NotificationRepository defines methods for notifications
type NotificationRepository interface {
	Create(ctx context.Context, notif *Notification) error
	List(ctx context.Context, userID *int64) ([]Notification, error)
}
