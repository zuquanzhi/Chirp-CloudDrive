package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zuquanzhi/Chirp/backend/internal/domain"
)

type userRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) domain.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, u *domain.User) error {
	stmt := `INSERT INTO users(name,email,password,created_at,phone_number,school,student_id,birthdate,address,gender) VALUES(?,?,?,?,?,?,?,?,?,?)`
	res, err := r.db.ExecContext(ctx, stmt, u.Name, u.Email, u.Password, time.Now(), u.PhoneNumber, u.School, u.StudentID, u.Birthdate, u.Address, u.Gender)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	u.ID = id
	return nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id,name,email,password,created_at,COALESCE(phone_number,''),COALESCE(school,''),COALESCE(student_id,''),COALESCE(birthdate,''),COALESCE(address,''),COALESCE(gender,'') FROM users WHERE email = ?`, email)
	u := &domain.User{}
	var created string
	if err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &created, &u.PhoneNumber, &u.School, &u.StudentID, &u.Birthdate, &u.Address, &u.Gender); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
	return u, nil
}

func (r *userRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id,name,email,password,created_at,COALESCE(phone_number,''),COALESCE(school,''),COALESCE(student_id,''),COALESCE(birthdate,''),COALESCE(address,''),COALESCE(gender,'') FROM users WHERE id = ?`, id)
	u := &domain.User{}
	var created string
	if err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &created, &u.PhoneNumber, &u.School, &u.StudentID, &u.Birthdate, &u.Address, &u.Gender); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
	return u, nil
}

func (r *userRepository) UpdateProfile(ctx context.Context, u *domain.User) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET name=?, school=?, student_id=?, birthdate=?, address=?, gender=? WHERE id=?`,
		u.Name, u.School, u.StudentID, u.Birthdate, u.Address, u.Gender, u.ID)
	return err
}
