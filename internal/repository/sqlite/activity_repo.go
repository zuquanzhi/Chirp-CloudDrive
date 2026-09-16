package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

type activityRepository struct {
	db *sql.DB
}

func NewActivityRepository(db *sql.DB) domain.ActivityRepository {
	return &activityRepository{db: db}
}

func (r *activityRepository) Create(ctx context.Context, a *domain.Activity) error {
	now := time.Now()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO activities(user_id,action,target_kind,target_id,target_name,detail,created_at) VALUES(?,?,?,?,?,?,?)`,
		a.UserID, a.Action, a.TargetKind, a.TargetID, a.TargetName, a.Detail, now.Format("2006-01-02 15:04:05"))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	a.ID = id
	a.CreatedAt = now
	return nil
}

func (r *activityRepository) ListByUser(ctx context.Context, userID int64, limit int) ([]domain.Activity, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,user_id,action,target_kind,COALESCE(target_id,0),COALESCE(target_name,''),COALESCE(detail,''),created_at
		 FROM activities WHERE user_id = ? ORDER BY id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]domain.Activity, 0)
	for rows.Next() {
		var a domain.Activity
		var created any
		if err := rows.Scan(&a.ID, &a.UserID, &a.Action, &a.TargetKind, &a.TargetID, &a.TargetName, &a.Detail, &created); err != nil {
			return nil, err
		}
		if t, ok := parseDBTime(created); ok {
			a.CreatedAt = t
		}
		list = append(list, a)
	}
	return list, rows.Err()
}
