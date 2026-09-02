package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

const userColumns = `id,name,email,password,created_at,COALESCE(phone_number,''),COALESCE(school,''),COALESCE(student_id,''),COALESCE(birthdate,''),COALESCE(address,''),COALESCE(gender,''),COALESCE(role,'USER'),COALESCE(quota,1073741824),COALESCE(used,0)`

type userRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) domain.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, u *domain.User) error {
	if u.Role == "" {
		u.Role = domain.RoleUser
	}
	if u.Quota == 0 {
		u.Quota = domain.DefaultQuota
	}
	stmt := `INSERT INTO users(name,email,password,created_at,phone_number,school,student_id,birthdate,address,gender,role,quota,used) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`
	res, err := r.db.ExecContext(ctx, stmt, u.Name, u.Email, u.Password, time.Now(), u.PhoneNumber, u.School, u.StudentID, u.Birthdate, u.Address, u.Gender, u.Role, u.Quota, u.Used)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	u.ID = id
	return nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.scanOne(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE email = ?`, email))
}

func (r *userRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	return r.scanOne(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

func (r *userRepository) UpdateProfile(ctx context.Context, u *domain.User) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET name=?, school=?, student_id=?, birthdate=?, address=?, gender=? WHERE id=?`,
		u.Name, u.School, u.StudentID, u.Birthdate, u.Address, u.Gender, u.ID)
	return err
}

func (r *userRepository) AddUsed(ctx context.Context, userID int64, delta int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET used = MAX(0, used + ?) WHERE id = ?`, delta, userID)
	return err
}

func (r *userRepository) scanOne(row *sql.Row) (*domain.User, error) {
	u := &domain.User{}
	var created any
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &created, &u.PhoneNumber, &u.School, &u.StudentID, &u.Birthdate, &u.Address, &u.Gender, &u.Role, &u.Quota, &u.Used)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if t, ok := parseDBTime(created); ok {
		u.CreatedAt = t
	}
	return u, nil
}
