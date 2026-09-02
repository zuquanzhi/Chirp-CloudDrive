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
}

// Resource represents an uploaded file metadata
type Resource struct {
	ID           int64          `json:"id"`
	OwnerID      *int64         `json:"owner_id"` // Nullable for anonymous uploads
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	Filename     string         `json:"filename"`      // stored file name on disk
	OriginalName string         `json:"original_name"` // original filename uploaded
	Size         int64          `json:"size"`
	FileHash     string         `json:"file_hash"` // SHA256 hash for duplicate check
	Status       ResourceStatus `json:"status"`    // PENDING, APPROVED, REJECTED
	CreatedAt    time.Time      `json:"created_at"`
	Subject      string         `json:"subject,omitempty"`
	Type         string         `json:"type,omitempty"`
	URL          string         `json:"url,omitempty"` // Public URL for the file
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
}

// ResourceRepository defines methods for resource persistence
type ResourceRepository interface {
	Create(ctx context.Context, resource *Resource) error
	List(ctx context.Context, status ResourceStatus, search string) ([]Resource, error)
	GetByID(ctx context.Context, id int64) (*Resource, error)
	UpdateStatus(ctx context.Context, id int64, status ResourceStatus) error
	GetByHash(ctx context.Context, hash string) ([]Resource, error)
}

// NotificationRepository defines methods for notifications
type NotificationRepository interface {
	Create(ctx context.Context, notif *Notification) error
	List(ctx context.Context, userID *int64) ([]Notification, error)
}
