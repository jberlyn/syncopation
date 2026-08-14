package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jberlyn/syncopation/db"
	"github.com/jberlyn/syncopation/services"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthService_Login(t *testing.T) {
	ctx := context.Background()

	password := "my-secret-password"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)

	t.Run("success", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
				if email == "test@example.com" {
					return db.User{ID: "user1", PasswordHash: string(hash)}, nil
				}
				return db.User{}, errors.New("not found")
			},
			CreateSessionFunc: func(ctx context.Context, arg db.CreateSessionParams) (db.Session, error) {
				return db.Session{ID: arg.ID, UserID: arg.UserID}, nil
			},
		}

		svc := services.NewAuthService(mockRepo)
		session, err := svc.Login(ctx, "test@example.com", password)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if session.UserID != "user1" {
			t.Errorf("expected UserID user1, got %s", session.UserID)
		}
		if session.ID == "" {
			t.Errorf("expected a session ID to be generated")
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
				return db.User{}, errors.New("not found")
			},
		}

		svc := services.NewAuthService(mockRepo)
		_, err := svc.Login(ctx, "wrong@example.com", password)

		if err != services.ErrInvalidCredentials {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("invalid password", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
				return db.User{ID: "user1", PasswordHash: string(hash)}, nil
			},
		}

		svc := services.NewAuthService(mockRepo)
		_, err := svc.Login(ctx, "test@example.com", "wrong-password")

		if err != services.ErrInvalidCredentials {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("db error on session create", func(t *testing.T) {
		dbErr := errors.New("db error")
		mockRepo := &db.QuerierMock{
			GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
				return db.User{ID: "user1", PasswordHash: string(hash)}, nil
			},
			CreateSessionFunc: func(ctx context.Context, arg db.CreateSessionParams) (db.Session, error) {
				return db.Session{}, dbErr
			},
		}

		svc := services.NewAuthService(mockRepo)
		_, err := svc.Login(ctx, "test@example.com", password)

		if err != dbErr {
			t.Fatalf("expected dbErr, got %v", err)
		}
	})
}

func TestAuthService_Logout(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			DeleteSessionFunc: func(ctx context.Context, id string) error {
				if id != "session1" {
					t.Errorf("expected session1, got %s", id)
				}
				return nil
			},
		}

		svc := services.NewAuthService(mockRepo)
		err := svc.Logout(ctx, "session1")

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		dbErr := errors.New("db error")
		mockRepo := &db.QuerierMock{
			DeleteSessionFunc: func(ctx context.Context, id string) error {
				return dbErr
			},
		}

		svc := services.NewAuthService(mockRepo)
		err := svc.Logout(ctx, "session1")

		if err != dbErr {
			t.Fatalf("expected dbErr, got %v", err)
		}
	})
}

func TestAuthService_ValidateSession(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetSessionFunc: func(ctx context.Context, id string) (db.Session, error) {
				if id == "valid-token" {
					return db.Session{ID: "valid-token", UserID: "user1"}, nil
				}
				return db.Session{}, errors.New("not found")
			},
		}

		svc := services.NewAuthService(mockRepo)
		session, err := svc.ValidateSession(ctx, "valid-token")

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if session.UserID != "user1" {
			t.Errorf("expected UserID user1, got %s", session.UserID)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		dbErr := errors.New("not found")
		mockRepo := &db.QuerierMock{
			GetSessionFunc: func(ctx context.Context, id string) (db.Session, error) {
				return db.Session{}, dbErr
			},
		}

		svc := services.NewAuthService(mockRepo)
		_, err := svc.ValidateSession(ctx, "invalid-token")

		if err != dbErr {
			t.Fatalf("expected dbErr, got %v", err)
		}
	})
}
