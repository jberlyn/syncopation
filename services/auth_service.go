package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jberlyn/syncopation/db"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type AuthService struct {
	repo db.Querier
}

func NewAuthService(repo db.Querier) *AuthService {
	return &AuthService{
		repo: repo,
	}
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*db.Session, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Generate 32-character hex UUID for session ID
	sessionID := strings.ReplaceAll(uuid.New().String(), "-", "")
	now := time.Now().UnixMilli()

	session, err := s.repo.CreateSession(ctx, db.CreateSessionParams{
		ID:        sessionID,
		UserID:    user.ID,
		AuthCode:  "",
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return nil, err
	}

	return &session, nil
}

func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	return s.repo.DeleteSession(ctx, sessionID)
}

func (s *AuthService) ValidateSession(ctx context.Context, sessionID string) (*db.Session, error) {
	session, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return &session, nil
}
