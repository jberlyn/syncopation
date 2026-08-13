package api

import (
	"context"
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jberlyn/joplin-sync/db"
	"golang.org/x/crypto/bcrypt"
)

//go:embed templates/*
var templatesFS embed.FS

var templates = map[string]*template.Template{}

func init() {
	layout := template.Must(template.ParseFS(templatesFS, "templates/layout.html"))
	templates["setup.html"] = template.Must(template.Must(layout.Clone()).ParseFS(templatesFS, "templates/setup.html"))
	templates["login.html"] = template.Must(template.Must(layout.Clone()).ParseFS(templatesFS, "templates/login.html"))
	templates["dashboard.html"] = template.Must(template.Must(layout.Clone()).ParseFS(templatesFS, "templates/dashboard.html"))
}

type AdminHandler struct {
	Queries *db.Queries
}

type UserContextKey string

const AdminUserKey UserContextKey = "admin_user"

func (h *AdminHandler) AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check if zero-user onboarding is needed
		count, err := h.Queries.CountUsers(r.Context())
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if count == 0 {
			if r.URL.Path != "/admin/setup" {
				http.Redirect(w, r, "/admin/setup", http.StatusFound)
				return
			}
			next.ServeHTTP(w, r)
			return
		} else {
			if r.URL.Path == "/admin/setup" {
				http.Redirect(w, r, "/admin", http.StatusFound)
				return
			}
		}

		// Allow login page to bypass auth check
		if r.URL.Path == "/admin/login" {
			next.ServeHTTP(w, r)
			return
		}

		// 2. Check for session cookie
		cookie, err := r.Cookie("admin_session")
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}

		// 3. Validate session
		session, err := h.Queries.GetSession(r.Context(), cookie.Value)
		if err != nil {
			http.SetCookie(w, &http.Cookie{Name: "admin_session", Value: "", MaxAge: -1, Path: "/"})
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}

		// 4. Validate user is admin
		user, err := h.Queries.GetUser(r.Context(), session.UserID)
		if err != nil || user.IsAdmin != 1 {
			http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), AdminUserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *AdminHandler) HandleSetupGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = templates["setup.html"].ExecuteTemplate(w, "base", nil)
}

func (h *AdminHandler) HandleSetupPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	if email == "" || password == "" {
		_ = templates["setup.html"].ExecuteTemplate(w, "base", map[string]string{"Error": "Email and password are required"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	id := uuid.New().String()
	now := time.Now().UnixMilli()

	user, err := h.Queries.CreateUser(r.Context(), db.CreateUserParams{
		ID:          id,
		Email:       email,
		Password:    string(hashedPassword),
		IsAdmin:     1,
		CreatedTime: now,
		UpdatedTime: now,
	})
	if err != nil {
		slog.Error("Failed to create admin user during setup", "error", err)
		_ = templates["setup.html"].ExecuteTemplate(w, "base", map[string]string{"Error": "Failed to create account"})
		return
	}

	// Auto-login the user after setup
	sessionID := strings.ReplaceAll(uuid.New().String(), "-", "")
	_, err = h.Queries.CreateSession(r.Context(), db.CreateSessionParams{
		ID:          sessionID,
		UserID:      user.ID,
		AuthCode:    "",
		CreatedTime: now,
		UpdatedTime: now,
	})
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/admin", http.StatusFound)
}

func (h *AdminHandler) HandleLoginGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = templates["login.html"].ExecuteTemplate(w, "base", nil)
}

func (h *AdminHandler) HandleLoginPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_ = r.ParseForm()
	email := r.FormValue("email")
	password := r.FormValue("password")

	user, err := h.Queries.GetUserByEmail(r.Context(), email)
	if err != nil || user.IsAdmin != 1 {
		_ = templates["login.html"].ExecuteTemplate(w, "base", map[string]string{"Error": "Invalid credentials or not an admin"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		_ = templates["login.html"].ExecuteTemplate(w, "base", map[string]string{"Error": "Invalid credentials or not an admin"})
		return
	}

	sessionID := strings.ReplaceAll(uuid.New().String(), "-", "")
	now := time.Now().UnixMilli()
	_, err = h.Queries.CreateSession(r.Context(), db.CreateSessionParams{
		ID:          sessionID,
		UserID:      user.ID,
		AuthCode:    "",
		CreatedTime: now,
		UpdatedTime: now,
	})
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/admin", http.StatusFound)
}

func (h *AdminHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("admin_session")
	if err == nil && cookie.Value != "" {
		_ = h.Queries.DeleteSession(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		HttpOnly: true,
	})
	http.Redirect(w, r, "/admin/login", http.StatusFound)
}

func (h *AdminHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := r.Context().Value(AdminUserKey).(db.User)
	_ = templates["dashboard.html"].ExecuteTemplate(w, "base", map[string]interface{}{
		"User": user,
	})
}
