package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

type shareRepository struct {
	db *sql.DB
}

func NewShareRepository(db *sql.DB) domain.ShareRepository {
	return &shareRepository{db: db}
}

const shareColumns = `id,token,resource_id,owner_id,COALESCE(password,''),expires_at,downloads,created_at`

func (r *shareRepository) Create(ctx context.Context, s *domain.Share) error {
	now := time.Now()
	var expires any
	if s.ExpiresAt != nil {
		expires = s.ExpiresAt.Format("2006-01-02 15:04:05")
	}
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO shares(token,resource_id,owner_id,password,expires_at,downloads,created_at) VALUES(?,?,?,?,?,0,?)`,
		s.Token, s.ResourceID, s.OwnerID, s.Password, expires, now.Format("2006-01-02 15:04:05"))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	s.ID = id
	s.CreatedAt = now
	return nil
}

func (r *shareRepository) GetByToken(ctx context.Context, token string) (*domain.Share, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+shareColumns+` FROM shares WHERE token = ?`, token)
	return scanShare(row)
}

func (r *shareRepository) ListByOwner(ctx context.Context, ownerID int64) ([]domain.Share, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT s.id,s.token,s.resource_id,s.owner_id,COALESCE(s.password,''),s.expires_at,s.downloads,s.created_at, COALESCE(res.original_name,''), COALESCE(res.size,0)
		 FROM shares s LEFT JOIN resources res ON res.id = s.resource_id
		 WHERE s.owner_id = ? ORDER BY s.created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]domain.Share, 0)
	for rows.Next() {
		var s domain.Share
		var expires any
		var created any
		if err := rows.Scan(&s.ID, &s.Token, &s.ResourceID, &s.OwnerID, &s.Password, &expires, &s.Downloads, &created, &s.FileName, &s.FileSize); err != nil {
			return nil, err
		}
		if t, ok := parseDBTime(expires); ok {
			s.ExpiresAt = &t
		}
		if t, ok := parseDBTime(created); ok {
			s.CreatedAt = t
		}
		s.HasPassword = s.Password != ""
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *shareRepository) Delete(ctx context.Context, id int64, ownerID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM shares WHERE id = ? AND owner_id = ?`, id, ownerID)
	return err
}

func (r *shareRepository) IncrementDownloads(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE shares SET downloads = downloads + 1 WHERE id = ?`, id)
	return err
}

func scanShare(s rowScanner) (*domain.Share, error) {
	sh := &domain.Share{}
	var expires, created any
	err := s.Scan(&sh.ID, &sh.Token, &sh.ResourceID, &sh.OwnerID, &sh.Password, &expires, &sh.Downloads, &created)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if t, ok := parseDBTime(expires); ok {
		sh.ExpiresAt = &t
	}
	if t, ok := parseDBTime(created); ok {
		sh.CreatedAt = t
	}
	sh.HasPassword = sh.Password != ""
	return sh, nil
}
