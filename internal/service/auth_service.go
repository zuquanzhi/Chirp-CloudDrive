package service

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/zuquanzhi/Chirp/backend/internal/domain"
	"github.com/zuquanzhi/Chirp/backend/pkg/util"
)

type AuthService struct {
	userRepo  domain.UserRepository
	jwtSecret string
}

func NewAuthService(userRepo domain.UserRepository, jwtSecret string) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
	}
}

func (s *AuthService) Signup(ctx context.Context, name, email, password string) (*domain.User, error) {
	existing, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("email already used")
	}

	hash, err := util.HashPassword(password)
	if err != nil {
		return nil, err
	}

	u := &domain.User{
		Name:     name,
		Email:    email,
		Password: hash,
	}

	if err := s.userRepo.Create(ctx, u); err != nil {
		return nil, err
	}

	return u, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	u, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	if u == nil {
		return "", errors.New("invalid credentials")
	}

	if err := util.CheckPassword(u.Password, password); err != nil {
		return "", errors.New("invalid credentials")
	}

	// Generate Token
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   u.ID,
		"email": u.Email,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	})

	return t.SignedString([]byte(s.jwtSecret))
}

func (s *AuthService) GetUserByID(ctx context.Context, id int64) (*domain.User, error) {
	return s.userRepo.GetByID(ctx, id)
}

func (s *AuthService) UpdateProfile(ctx context.Context, u *domain.User) (*domain.User, error) {
	// Ensure user exists
	existing, err := s.userRepo.GetByID(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("user not found")
	}

	// Apply updates (allow empty string to clear)
	existing.Name = u.Name
	existing.School = u.School
	existing.StudentID = u.StudentID
	existing.Birthdate = u.Birthdate
	existing.Address = u.Address
	existing.Gender = u.Gender

	if err := s.userRepo.UpdateProfile(ctx, existing); err != nil {
		return nil, err
	}

	return existing, nil
}
