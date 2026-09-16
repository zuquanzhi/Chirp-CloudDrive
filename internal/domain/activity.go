package domain

import (
	"context"
	"time"
)

// Activity actions recorded in the activity log.
const (
	ActivityUpload        = "upload"
	ActivityInstantUpload = "instant_upload"
	ActivityChunkedUpload = "chunked_upload"
	ActivityRename        = "rename"
	ActivityMove          = "move"
	ActivityDelete        = "delete"      // soft delete (to trash)
	ActivityRestore       = "restore"     // from trash
	ActivityHardDelete    = "hard_delete" // permanent delete
	ActivityShareCreate   = "share_create"
	ActivityShareCancel   = "share_cancel"
	ActivityNewVersion    = "new_version"
	ActivityRestoreVersion = "restore_version"
	ActivityFolderCreate  = "folder_create"
	ActivityFolderRename  = "folder_rename"
	ActivityFolderMove    = "folder_move"
)

// Activity is one entry of a user's operation log.
type Activity struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	Action     string    `json:"action"`
	TargetKind string    `json:"target_kind"` // file | folder | share
	TargetID   int64     `json:"target_id"`
	TargetName string    `json:"target_name"`
	Detail     string    `json:"detail,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// ActivityRepository defines activity log persistence.
type ActivityRepository interface {
	Create(ctx context.Context, a *Activity) error
	ListByUser(ctx context.Context, userID int64, limit int) ([]Activity, error)
}
