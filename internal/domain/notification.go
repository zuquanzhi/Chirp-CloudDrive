package domain

import (
	"context"
	"time"
)

// Notification represents a system message
type Notification struct {
	ID        int64     `json:"id"`
	UserID    *int64    `json:"user_id"` // Null for system-wide, or specific user
	Content   string    `json:"content"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

// NotificationRepository defines methods for notifications
type NotificationRepository interface {
	Create(ctx context.Context, notif *Notification) error
	List(ctx context.Context, userID *int64) ([]Notification, error)
}
