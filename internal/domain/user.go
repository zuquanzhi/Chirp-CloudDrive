package domain

import (
	"context"
	"time"
)

type UserRole string

const (
	RoleUser  UserRole = "USER"
	RoleAdmin UserRole = "ADMIN"
)

// DefaultQuota is the per-user storage quota (1GB) assigned at signup.
const DefaultQuota int64 = 1 << 30

// User represents a registered user
type User struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Password    string    `json:"-"`
	Role        UserRole  `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
	PhoneNumber string    `json:"phone_number,omitempty"`
	School      string    `json:"school,omitempty"`
	StudentID   string    `json:"student_id,omitempty"`
	Birthdate   string    `json:"birthdate,omitempty"`
	Address     string    `json:"address,omitempty"`
	Gender      string    `json:"gender,omitempty"`
	Quota       int64     `json:"quota"` // total storage quota in bytes
	Used        int64     `json:"used"`  // used storage in bytes
}

// UserRepository defines methods for user persistence
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	UpdateProfile(ctx context.Context, user *User) error
	// AddUsed adjusts the user's used bytes by delta (negative to decrease)
	AddUsed(ctx context.Context, userID int64, delta int64) error
}
