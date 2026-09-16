package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

type uploadSessionRepository struct {
	db *sql.DB
}

func NewUploadSessionRepository(db *sql.DB) domain.UploadSessionRepository {
	return &uploadSessionRepository{db: db}
}

func (r *uploadSessionRepository) Create(ctx context.Context, s *domain.UploadSession) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO upload_sessions(id,owner_id,folder_id,filename,size,chunk_size,total_chunks,file_hash,created_at) VALUES(?,?,?,?,?,?,?,?,?)`,
		s.ID, s.OwnerID, s.FolderID, s.Filename, s.Size, s.ChunkSize, s.TotalChunks, s.FileHash, now.Format("2006-01-02 15:04:05"))
	if err != nil {
		return err
	}
	s.CreatedAt = now
	return nil
}

func (r *uploadSessionRepository) GetByID(ctx context.Context, id string) (*domain.UploadSession, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id,owner_id,folder_id,filename,size,chunk_size,total_chunks,COALESCE(file_hash,''),created_at FROM upload_sessions WHERE id = ?`, id)
	s := &domain.UploadSession{}
	var created any
	err := row.Scan(&s.ID, &s.OwnerID, &s.FolderID, &s.Filename, &s.Size, &s.ChunkSize, &s.TotalChunks, &s.FileHash, &created)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if t, ok := parseDBTime(created); ok {
		s.CreatedAt = t
	}
	return s, nil
}

func (r *uploadSessionRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM upload_sessions WHERE id = ?`, id)
	return err
}
