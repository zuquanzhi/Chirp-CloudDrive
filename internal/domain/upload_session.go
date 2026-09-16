package domain

import (
	"context"
	"time"
)

// UploadSession tracks a chunked (resumable) upload in progress.
type UploadSession struct {
	ID           string    `json:"id"` // uuid
	OwnerID      int64     `json:"owner_id"`
	FolderID     *int64    `json:"folder_id"`
	Filename     string    `json:"filename"`
	Size         int64     `json:"size"`
	ChunkSize    int64     `json:"chunk_size"`
	TotalChunks  int       `json:"total_chunks"`
	FileHash     string    `json:"file_hash"`
	CreatedAt    time.Time `json:"created_at"`

	// UploadedChunks is filled from the chunk directory on read (not stored).
	UploadedChunks []int `json:"uploaded_chunks"`
}

// UploadSessionRepository defines upload session persistence.
type UploadSessionRepository interface {
	Create(ctx context.Context, s *UploadSession) error
	GetByID(ctx context.Context, id string) (*UploadSession, error)
	Delete(ctx context.Context, id string) error
}
