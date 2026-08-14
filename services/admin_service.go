package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jberlyn/syncopation/db"
	"github.com/jberlyn/syncopation/storage"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrForbidden     = errors.New("forbidden")
	ErrUserExists    = errors.New("user already exists")
	ErrAdminRequired = errors.New("admin access required")
)

type AdminService struct {
	repo    db.Querier
	storage storage.Storage
}

func NewAdminService(repo db.Querier, storage storage.Storage) *AdminService {
	return &AdminService{
		repo:    repo,
		storage: storage,
	}
}

func (s *AdminService) CheckSetupNeeded(ctx context.Context) (bool, error) {
	count, err := s.repo.CountUsers(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (s *AdminService) ValidateAdminSession(ctx context.Context, sessionID string) (*db.User, error) {
	session, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, ErrForbidden
	}

	user, err := s.repo.GetUser(ctx, session.UserID)
	if err != nil || user.IsAdmin != 1 {
		return nil, ErrForbidden
	}

	return &user, nil
}

func (s *AdminService) SetupFirstAdmin(ctx context.Context, email, password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	id := uuid.New().String()
	now := time.Now().UnixMilli()

	user, err := s.repo.CreateUser(ctx, db.CreateUserParams{
		ID:           id,
		Email:        email,
		PasswordHash: string(hashedPassword),
		IsAdmin:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return "", err
	}

	sessionID := strings.ReplaceAll(uuid.New().String(), "-", "")
	_, err = s.repo.CreateSession(ctx, db.CreateSessionParams{
		ID:        sessionID,
		UserID:    user.ID,
		AuthCode:  "",
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return "", err
	}

	return sessionID, nil
}

func (s *AdminService) AdminLogin(ctx context.Context, email, password string) (string, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil || user.IsAdmin != 1 {
		return "", ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}

	sessionID := strings.ReplaceAll(uuid.New().String(), "-", "")
	now := time.Now().UnixMilli()
	_, err = s.repo.CreateSession(ctx, db.CreateSessionParams{
		ID:        sessionID,
		UserID:    user.ID,
		AuthCode:  "",
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return "", err
	}

	return sessionID, nil
}

func (s *AdminService) AdminLogout(ctx context.Context, sessionID string) error {
	return s.repo.DeleteSession(ctx, sessionID)
}

func (s *AdminService) GetDashboardStats(ctx context.Context) (db.GetInstanceStatsRow, []db.GetUserStatsRow, error) {
	stats, err := s.repo.GetInstanceStats(ctx)
	if err != nil {
		return db.GetInstanceStatsRow{}, nil, err
	}
	userStats, err := s.repo.GetUserStats(ctx)
	if err != nil {
		return stats, nil, err
	}
	return stats, userStats, nil
}

func (s *AdminService) CreateUser(ctx context.Context, email, password string) error {
	_, err := s.repo.GetUserByEmail(ctx, email)
	if err == nil {
		return ErrUserExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	_, err = s.repo.CreateUser(ctx, db.CreateUserParams{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: string(hashedPassword),
		IsAdmin:      0,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	return err
}

func (s *AdminService) DeleteUser(ctx context.Context, userID string) error {
	user, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	if user.IsAdmin != 0 {
		return errors.New("cannot delete admin users")
	}

	now := time.Now().UnixMilli()
	err = s.repo.InsertShareTombstonesForDeletedUser(ctx, db.InsertShareTombstonesForDeletedUserParams{
		CreatedAt: now,
		UpdatedAt: now,
		OwnerID:   userID,
	})
	if err != nil {
		return err // Or log and continue, but typically service should return error
	}

	err = s.repo.DeleteUser(ctx, userID)
	if err != nil {
		return err
	}

	if s.storage != nil {
		return s.storage.DeleteUser(ctx, userID)
	}

	return nil
}
