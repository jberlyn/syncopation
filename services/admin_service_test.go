package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jberlyn/syncopation/db"
	"github.com/jberlyn/syncopation/services"
	"github.com/jberlyn/syncopation/storage"
	"golang.org/x/crypto/bcrypt"
)

func TestAdminService_CheckSetupNeeded(t *testing.T) {
	ctx := context.Background()

	t.Run("setup needed", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			CountUsersFunc: func(ctx context.Context) (int64, error) {
				return 0, nil
			},
		}

		svc := services.NewAdminService(mockRepo, nil)
		needed, err := svc.CheckSetupNeeded(ctx)

		if err != nil {
			t.Fatal(err)
		}
		if !needed {
			t.Error("expected setup to be needed")
		}
	})

	t.Run("setup not needed", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			CountUsersFunc: func(ctx context.Context) (int64, error) {
				return 1, nil
			},
		}

		svc := services.NewAdminService(mockRepo, nil)
		needed, err := svc.CheckSetupNeeded(ctx)

		if err != nil {
			t.Fatal(err)
		}
		if needed {
			t.Error("expected setup not to be needed")
		}
	})
}

func TestAdminService_ValidateAdminSession(t *testing.T) {
	ctx := context.Background()

	t.Run("valid admin", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetSessionFunc: func(ctx context.Context, id string) (db.Session, error) {
				return db.Session{UserID: "admin1"}, nil
			},
			GetUserFunc: func(ctx context.Context, id string) (db.User, error) {
				if id == "admin1" {
					return db.User{ID: "admin1", IsAdmin: 1}, nil
				}
				return db.User{}, errors.New("not found")
			},
		}

		svc := services.NewAdminService(mockRepo, nil)
		user, err := svc.ValidateAdminSession(ctx, "session1")

		if err != nil {
			t.Fatal(err)
		}
		if user.ID != "admin1" {
			t.Errorf("expected admin1, got %s", user.ID)
		}
	})

	t.Run("valid session but not admin", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetSessionFunc: func(ctx context.Context, id string) (db.Session, error) {
				return db.Session{UserID: "user1"}, nil
			},
			GetUserFunc: func(ctx context.Context, id string) (db.User, error) {
				return db.User{ID: "user1", IsAdmin: 0}, nil
			},
		}

		svc := services.NewAdminService(mockRepo, nil)
		_, err := svc.ValidateAdminSession(ctx, "session1")

		if err != services.ErrForbidden {
			t.Errorf("expected ErrForbidden, got %v", err)
		}
	})
}

func TestAdminService_CreateUser(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
				return db.User{}, errors.New("not found") // Email available
			},
			CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
				if arg.Email != "new@example.com" {
					t.Errorf("expected email new@example.com, got %s", arg.Email)
				}
				if arg.IsAdmin != 0 {
					t.Errorf("expected IsAdmin 0, got %d", arg.IsAdmin)
				}
				return db.User{}, nil
			},
		}

		svc := services.NewAdminService(mockRepo, nil)
		err := svc.CreateUser(ctx, "new@example.com", "password123")

		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("user exists", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
				return db.User{ID: "existing"}, nil
			},
		}

		svc := services.NewAdminService(mockRepo, nil)
		err := svc.CreateUser(ctx, "existing@example.com", "password123")

		if err != services.ErrUserExists {
			t.Errorf("expected ErrUserExists, got %v", err)
		}
	})
}

func TestAdminService_DeleteUser(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetUserFunc: func(ctx context.Context, id string) (db.User, error) {
				return db.User{ID: "u1", IsAdmin: 0}, nil
			},
			InsertShareTombstonesForDeletedUserFunc: func(ctx context.Context, arg db.InsertShareTombstonesForDeletedUserParams) error {
				return nil
			},
			DeleteUserFunc: func(ctx context.Context, id string) error {
				return nil
			},
		}

		mockStorage := &storage.StorageMock{
			DeleteUserFunc: func(ctx context.Context, userID string) error {
				return nil
			},
		}

		svc := services.NewAdminService(mockRepo, mockStorage)
		err := svc.DeleteUser(ctx, "u1")

		if err != nil {
			t.Fatal(err)
		}
		if len(mockRepo.DeleteUserCalls()) != 1 {
			t.Error("expected DeleteUser to be called")
		}
		if len(mockStorage.DeleteUserCalls()) != 1 {
			t.Error("expected storage DeleteUser to be called")
		}
	})

	t.Run("cannot delete admin", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetUserFunc: func(ctx context.Context, id string) (db.User, error) {
				return db.User{ID: "u1", IsAdmin: 1}, nil
			},
		}

		svc := services.NewAdminService(mockRepo, nil)
		err := svc.DeleteUser(ctx, "u1")

		if err == nil || err.Error() != "cannot delete admin users" {
			t.Errorf("expected cannot delete admin users, got %v", err)
		}
	})
}

func TestAdminService_AdminLogin(t *testing.T) {
	ctx := context.Background()

	password := "admin-pass"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)

	t.Run("success", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
				return db.User{ID: "admin1", IsAdmin: 1, PasswordHash: string(hash)}, nil
			},
			CreateSessionFunc: func(ctx context.Context, arg db.CreateSessionParams) (db.Session, error) {
				return db.Session{ID: arg.ID, UserID: arg.UserID}, nil
			},
		}

		svc := services.NewAdminService(mockRepo, nil)
		sessionID, err := svc.AdminLogin(ctx, "admin@example.com", password)

		if err != nil {
			t.Fatal(err)
		}
		if sessionID == "" {
			t.Error("expected sessionID")
		}
	})

	t.Run("not an admin", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetUserByEmailFunc: func(ctx context.Context, email string) (db.User, error) {
				return db.User{ID: "user1", IsAdmin: 0, PasswordHash: string(hash)}, nil
			},
		}

		svc := services.NewAdminService(mockRepo, nil)
		_, err := svc.AdminLogin(ctx, "user@example.com", password)

		if err != services.ErrInvalidCredentials {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})
}

func TestAdminService_SetupFirstAdmin(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
				if arg.IsAdmin != 1 {
					t.Errorf("expected IsAdmin 1, got %d", arg.IsAdmin)
				}
				return db.User{ID: "admin1"}, nil
			},
			CreateSessionFunc: func(ctx context.Context, arg db.CreateSessionParams) (db.Session, error) {
				return db.Session{ID: arg.ID}, nil
			},
		}

		svc := services.NewAdminService(mockRepo, nil)
		sessionID, err := svc.SetupFirstAdmin(ctx, "admin@example.com", "password123")

		if err != nil {
			t.Fatal(err)
		}
		if sessionID == "" {
			t.Error("expected sessionID")
		}
	})
}

func TestAdminService_AdminLogout(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			DeleteSessionFunc: func(ctx context.Context, id string) error {
				return nil
			},
		}

		svc := services.NewAdminService(mockRepo, nil)
		err := svc.AdminLogout(ctx, "session1")

		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestAdminService_GetDashboardStats(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetInstanceStatsFunc: func(ctx context.Context) (db.GetInstanceStatsRow, error) {
				return db.GetInstanceStatsRow{TotalUsers: 5}, nil
			},
			GetUserStatsFunc: func(ctx context.Context) ([]db.GetUserStatsRow, error) {
				return []db.GetUserStatsRow{{Email: "admin@example.com"}}, nil
			},
		}

		svc := services.NewAdminService(mockRepo, nil)
		stats, userStats, err := svc.GetDashboardStats(ctx)

		if err != nil {
			t.Fatal(err)
		}
		if stats.TotalUsers != 5 {
			t.Errorf("expected 5 total users")
		}
		if len(userStats) != 1 {
			t.Errorf("expected 1 user stat")
		}
	})
}

