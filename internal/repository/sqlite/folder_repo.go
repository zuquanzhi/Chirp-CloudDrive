package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

type folderRepository struct {
	db *sql.DB
}

func NewFolderRepository(db *sql.DB) domain.FolderRepository {
	return &folderRepository{db: db}
}

func (r *folderRepository) Create(ctx context.Context, f *domain.Folder) error {
	stmt := `INSERT INTO folders(owner_id, parent_id, name, created_at) VALUES(?,?,?,?)`
	now := time.Now()
	res, err := r.db.ExecContext(ctx, stmt, f.OwnerID, f.ParentID, f.Name, now.Format("2006-01-02 15:04:05"))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	f.ID = id
	f.CreatedAt = now
	return nil
}

func (r *folderRepository) GetByID(ctx context.Context, id int64) (*domain.Folder, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id,owner_id,parent_id,name,created_at,deleted_at FROM folders WHERE id = ?`, id)
	return scanFolder(row)
}

func (r *folderRepository) ListByParent(ctx context.Context, ownerID int64, parentID *int64) ([]domain.Folder, error) {
	var rows *sql.Rows
	var err error
	if parentID == nil {
		rows, err = r.db.QueryContext(ctx, `SELECT id,owner_id,parent_id,name,created_at,deleted_at FROM folders WHERE owner_id = ? AND parent_id IS NULL AND deleted_at IS NULL ORDER BY name`, ownerID)
	} else {
		rows, err = r.db.QueryContext(ctx, `SELECT id,owner_id,parent_id,name,created_at,deleted_at FROM folders WHERE owner_id = ? AND parent_id = ? AND deleted_at IS NULL ORDER BY name`, ownerID, *parentID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	folders := make([]domain.Folder, 0)
	for rows.Next() {
		f, err := scanFolder(rows)
		if err != nil {
			return nil, err
		}
		folders = append(folders, *f)
	}
	return folders, rows.Err()
}

func (r *folderRepository) Update(ctx context.Context, f *domain.Folder) error {
	_, err := r.db.ExecContext(ctx, `UPDATE folders SET name = ?, parent_id = ? WHERE id = ? AND owner_id = ?`, f.Name, f.ParentID, f.ID, f.OwnerID)
	return err
}

func (r *folderRepository) SoftDelete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE folders SET deleted_at = ? WHERE id = ?`, time.Now().Format("2006-01-02 15:04:05"), id)
	return err
}

func (r *folderRepository) ListDescendantIDs(ctx context.Context, id int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH RECURSIVE sub(id) AS (
			SELECT id FROM folders WHERE parent_id = ?
			UNION ALL
			SELECT f.id FROM folders f JOIN sub s ON f.parent_id = s.id
		) SELECT id FROM sub`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var fid int64
		if err := rows.Scan(&fid); err != nil {
			return nil, err
		}
		ids = append(ids, fid)
	}
	return ids, rows.Err()
}

func (r *folderRepository) ListDeleted(ctx context.Context, ownerID int64) ([]domain.Folder, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,owner_id,parent_id,name,created_at,deleted_at FROM folders WHERE owner_id = ? AND deleted_at IS NOT NULL ORDER BY deleted_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	folders := make([]domain.Folder, 0)
	for rows.Next() {
		f, err := scanFolder(rows)
		if err != nil {
			return nil, err
		}
		folders = append(folders, *f)
	}
	return folders, rows.Err()
}

func (r *folderRepository) Restore(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE folders SET deleted_at = NULL WHERE id = ?`, id)
	return err
}

func (r *folderRepository) HardDelete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM folders WHERE id = ?`, id)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

// parseDBTime handles values the sqlite driver may return as time.Time or string.
func parseDBTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case string:
		for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339Nano, time.RFC3339} {
			if parsed, err := time.Parse(layout, t); err == nil {
				return parsed, true
			}
		}
	}
	return time.Time{}, false
}

func scanFolder(s rowScanner) (*domain.Folder, error) {
	f := &domain.Folder{}
	var created, deleted any
	err := s.Scan(&f.ID, &f.OwnerID, &f.ParentID, &f.Name, &created, &deleted)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if t, ok := parseDBTime(created); ok {
		f.CreatedAt = t
	}
	if t, ok := parseDBTime(deleted); ok {
		f.DeletedAt = &t
	}
	return f, nil
}
