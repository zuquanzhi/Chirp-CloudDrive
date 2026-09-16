package service

import (
	"context"
	"log"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

// ActivityRecorder writes best-effort entries to the user's activity log.
// Failures are logged but never block the business operation.
type ActivityRecorder struct {
	repo domain.ActivityRepository
}

func NewActivityRecorder(repo domain.ActivityRepository) *ActivityRecorder {
	return &ActivityRecorder{repo: repo}
}

// Log records one activity entry. kind is "file" | "folder" | "share".
func (r *ActivityRecorder) Log(ctx context.Context, userID int64, action, kind string, targetID int64, targetName, detail string) {
	if r == nil || r.repo == nil {
		return
	}
	a := &domain.Activity{
		UserID:     userID,
		Action:     action,
		TargetKind: kind,
		TargetID:   targetID,
		TargetName: targetName,
		Detail:     detail,
	}
	if err := r.repo.Create(ctx, a); err != nil {
		log.Printf("activity log failed: %v", err)
	}
}

// List returns the most recent activities of a user.
func (r *ActivityRecorder) List(ctx context.Context, userID int64, limit int) ([]domain.Activity, error) {
	return r.repo.ListByUser(ctx, userID, limit)
}
