package domain

import (
	"context"
	"time"
)

// Share represents a public download link for a single drive file.
type Share struct {
	ID         int64      `json:"id"`
	Token      string     `json:"token"`
	ResourceID int64      `json:"resource_id"`
	OwnerID    int64      `json:"owner_id"`
	Password   string     `json:"-"`                       // extraction code, never serialized
	HasPassword bool       `json:"has_password"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`    // nil = never expires
	Downloads  int64      `json:"downloads"`
	CreatedAt  time.Time  `json:"created_at"`

	// Joined fields for display (not stored on the shares row).
	FileName string `json:"file_name,omitempty"`
	FileSize int64  `json:"file_size,omitempty"`
}

// ShareRepository defines share persistence.
type ShareRepository interface {
	Create(ctx context.Context, share *Share) error
	GetByToken(ctx context.Context, token string) (*Share, error)
	ListByOwner(ctx context.Context, ownerID int64) ([]Share, error)
	Delete(ctx context.Context, id int64, ownerID int64) error
	IncrementDownloads(ctx context.Context, id int64) error
}
