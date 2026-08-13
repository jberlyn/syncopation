package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jberlyn/joplin-sync/db"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	Queries *db.Queries
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.Queries.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized) // 401 is typically for bad login
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Generate 32-character hex UUID for session ID
	sessionID := strings.ReplaceAll(uuid.New().String(), "-", "")
	now := time.Now().UnixMilli()

	session, err := h.Queries.CreateSession(r.Context(), db.CreateSessionParams{
		ID:          sessionID,
		UserID:      user.ID,
		AuthCode:    "",
		CreatedTime: now,
		UpdatedTime: now,
	})
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	resp := LoginResponse{
		ID:     session.ID,
		UserID: session.UserID,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Missing session ID", http.StatusBadRequest)
		return
	}

	err := h.Queries.DeleteSession(r.Context(), id)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

type contextKey string

const UserIDKey contextKey = "user_id"

// RequireAuth is a middleware that validates the X-API-AUTH header
func (h *AuthHandler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-API-AUTH")
		if token == "" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		session, err := h.Queries.GetSession(r.Context(), token)
		if err != nil {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, session.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
