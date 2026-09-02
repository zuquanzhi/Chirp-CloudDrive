package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

const resourceColumns = `id,owner_id,folder_id,title,description,filename,original_name,size,file_hash,status,created_at,deleted_at,COALESCE(subject,''),COALESCE(type,'')`

type resourceRepository struct {
	db *sql.DB
}

func NewResourceRepository(db *sql.DB) domain.ResourceRepository {
	return &resourceRepository{db: db}
}

func (r *resourceRepository) Create(ctx context.Context, res *domain.Resource) error {
	stmt := `INSERT INTO resources(owner_id,folder_id,title,description,filename,original_name,size,file_hash,status,created_at,subject,type) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`
	now := time.Now()
	result, err := r.db.ExecContext(ctx, stmt, res.OwnerID, res.FolderID, res.Title, res.Description, res.Filename, res.OriginalName, res.Size, res.FileHash, res.Status, now.Format("2006-01-02 15:04:05"), res.Subject, res.Type)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	res.ID = id
	res.CreatedAt = now
	return nil
}

func (r *resourceRepository) List(ctx context.Context, status domain.ResourceStatus, search string) ([]domain.Resource, error) {
	query := `SELECT ` + resourceColumns + ` FROM resources WHERE deleted_at IS NULL`
	args := []interface{}{}

	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	if search != "" {
		query += ` AND (title LIKE ? OR description LIKE ?)`
		args = append(args, "%"+search+"%", "%"+search+"%")
	}
	query += ` ORDER BY created_at DESC`

	return r.query(ctx, query, args...)
}

func (r *resourceRepository) GetByID(ctx context.Context, id int64) (*domain.Resource, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+resourceColumns+` FROM resources WHERE id = ?`, id)
	res, err := scanResource(row)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (r *resourceRepository) UpdateStatus(ctx context.Context, id int64, status domain.ResourceStatus) error {
	_, err := r.db.ExecContext(ctx, `UPDATE resources SET status = ? WHERE id = ?`, status, id)
	return err
}

func (r *resourceRepository) GetByHash(ctx context.Context, hash string) ([]domain.Resource, error) {
	return r.query(ctx, `SELECT `+resourceColumns+` FROM resources WHERE file_hash = ?`, hash)
}

// ---- Drive operations ----

func (r *resourceRepository) ListByFolder(ctx context.Context, ownerID int64, folderID *int64, search string) ([]domain.Resource, error) {
	query := `SELECT ` + resourceColumns + ` FROM resources WHERE owner_id = ? AND deleted_at IS NULL`
	args := []interface{}{ownerID}

	if folderID == nil {
		query += ` AND folder_id IS NULL`
	} else {
		query += ` AND folder_id = ?`
		args = append(args, *folderID)
	}
	if search != "" {
		query += ` AND original_name LIKE ?`
		args = append(args, "%"+search+"%")
	}
	query += ` ORDER BY original_name`

	return r.query(ctx, query, args...)
}

func (r *resourceRepository) Update(ctx context.Context, res *domain.Resource) error {
	_, err := r.db.ExecContext(ctx, `UPDATE resources SET original_name = ?, title = ?, folder_id = ? WHERE id = ? AND owner_id = ?`,
		res.OriginalName, res.Title, res.FolderID, res.ID, res.OwnerID)
	return err
}

func (r *resourceRepository) SoftDelete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE resources SET deleted_at = ? WHERE id = ?`, time.Now().Format("2006-01-02 15:04:05"), id)
	return err
}

func (r *resourceRepository) SoftDeleteByFolder(ctx context.Context, folderID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE resources SET deleted_at = ? WHERE folder_id = ? AND deleted_at IS NULL`, time.Now().Format("2006-01-02 15:04:05"), folderID)
	return err
}

func (r *resourceRepository) ListDeleted(ctx context.Context, ownerID int64) ([]domain.Resource, error) {
	return r.query(ctx, `SELECT `+resourceColumns+` FROM resources WHERE owner_id = ? AND deleted_at IS NOT NULL ORDER BY deleted_at DESC`, ownerID)
}

func (r *resourceRepository) ListByFolderIncludingDeleted(ctx context.Context, folderID int64) ([]domain.Resource, error) {
	return r.query(ctx, `SELECT `+resourceColumns+` FROM resources WHERE folder_id = ?`, folderID)
}

func (r *resourceRepository) Restore(ctx context.Context, id int64, folderID *int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE resources SET deleted_at = NULL, folder_id = ? WHERE id = ?`, folderID, id)
	return err
}

func (r *resourceRepository) RestoreByFolder(ctx context.Context, folderID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE resources SET deleted_at = NULL WHERE folder_id = ?`, folderID)
	return err
}

func (r *resourceRepository) HardDelete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM resources WHERE id = ?`, id)
	return err
}

func (r *resourceRepository) HardDeleteByFolder(ctx context.Context, folderID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM resources WHERE folder_id = ?`, folderID)
	return err
}

// ---- helpers ----

func (r *resourceRepository) query(ctx context.Context, query string, args ...interface{}) ([]domain.Resource, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Resource
	for rows.Next() {
		res, err := scanResource(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *res)
	}
	return list, rows.Err()
}

func scanResource(s rowScanner) (*domain.Resource, error) {
	var res domain.Resource
	var created, deleted any
	err := s.Scan(&res.ID, &res.OwnerID, &res.FolderID, &res.Title, &res.Description, &res.Filename, &res.OriginalName, &res.Size, &res.FileHash, &res.Status, &created, &deleted, &res.Subject, &res.Type)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if t, ok := parseDBTime(created); ok {
		res.CreatedAt = t
	}
	if t, ok := parseDBTime(deleted); ok {
		res.DeletedAt = &t
	}
	return &res, nil
}
