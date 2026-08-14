package api

import (
	"context"
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jberlyn/syncopation/db"
	"github.com/jberlyn/syncopation/services"
)

//go:embed templates/*
var templatesFS embed.FS

var templates = map[string]*template.Template{}

func init() {
	layout := template.Must(template.ParseFS(templatesFS, "templates/layout.html"))
	templates["setup.html"] = template.Must(template.Must(layout.Clone()).ParseFS(templatesFS, "templates/setup.html"))
	templates["login.html"] = template.Must(template.Must(layout.Clone()).ParseFS(templatesFS, "templates/login.html"))
	templates["dashboard.html"] = template.Must(template.Must(layout.Clone()).ParseFS(templatesFS, "templates/dashboard.html", "templates/user_list.html", "templates/add_user_form.html", "templates/stats.html"))
	templates["user_list.html"] = template.Must(template.ParseFS(templatesFS, "templates/user_list.html"))
	templates["add_user_form.html"] = template.Must(template.ParseFS(templatesFS, "templates/add_user_form.html"))
	templates["stats.html"] = template.Must(template.ParseFS(templatesFS, "templates/stats.html"))
}

type AdminHandler struct {
	AdminService *services.AdminService
	Version      string
}

type UserContextKey string

const AdminUserKey UserContextKey = "admin_user"

func (h *AdminHandler) AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check if zero-user onboarding is needed
		needed, err := h.AdminService.CheckSetupNeeded(r.Context())
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if needed {
			if r.URL.Path != "/setup" {
				http.Redirect(w, r, "/setup", http.StatusFound)
				return
			}
			next.ServeHTTP(w, r)
			return
		} else {
			if r.URL.Path == "/setup" {
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
		}

		// Allow login page to bypass auth check
		if r.URL.Path == "/login" {
			next.ServeHTTP(w, r)
			return
		}

		// 2. Check for session cookie
		cookie, err := r.Cookie("admin_session")
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// 3. Validate session
		user, err := h.AdminService.ValidateAdminSession(r.Context(), cookie.Value)
		if err != nil {
			if err == services.ErrForbidden {
				http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
			} else {
				http.SetCookie(w, &http.Cookie{Name: "admin_session", Value: "", MaxAge: -1, Path: "/"})
				http.Redirect(w, r, "/login", http.StatusFound)
			}
			return
		}

		ctx := context.WithValue(r.Context(), AdminUserKey, *user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *AdminHandler) HandleSetupGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = templates["setup.html"].ExecuteTemplate(w, "base", map[string]interface{}{"Version": h.Version})
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
		_ = templates["setup.html"].ExecuteTemplate(w, "base", map[string]interface{}{"Error": "Email and password are required", "Version": h.Version})
		return
	}

	sessionID, err := h.AdminService.SetupFirstAdmin(r.Context(), email, password)
	if err != nil {
		slog.Error("Failed to create admin user during setup", "error", err)
		_ = templates["setup.html"].ExecuteTemplate(w, "base", map[string]interface{}{"Error": "Failed to create account", "Version": h.Version})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *AdminHandler) HandleLoginGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	toastMsg := ""
	if r.URL.Query().Get("logged_out") == "1" {
		toastMsg = "You have been signed out"
	}
	_ = templates["login.html"].ExecuteTemplate(w, "base", map[string]interface{}{
		"Version":      h.Version,
		"ToastMessage": toastMsg,
	})
}

func (h *AdminHandler) HandleLoginPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_ = r.ParseForm()
	email := r.FormValue("email")
	password := r.FormValue("password")

	sessionID, err := h.AdminService.AdminLogin(r.Context(), email, password)
	if err != nil {
		if err == services.ErrInvalidCredentials {
			_ = templates["login.html"].ExecuteTemplate(w, "base", map[string]interface{}{"Error": "Invalid credentials or not an admin", "Version": h.Version})
			return
		}
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

	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *AdminHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("admin_session")
	if err == nil && cookie.Value != "" {
		_ = h.AdminService.AdminLogout(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		HttpOnly: true,
	})
	http.Redirect(w, r, "/login?logged_out=1", http.StatusFound)
}

func (h *AdminHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := r.Context().Value(AdminUserKey).(db.User)

	stats, userStats, _ := h.AdminService.GetDashboardStats(r.Context())

	_ = templates["dashboard.html"].ExecuteTemplate(w, "base", map[string]interface{}{
		"User":      user,
		"Stats":     stats,
		"UserStats": userStats,
		"Version":   h.Version,
	})
}

func (h *AdminHandler) HandleUsersPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = r.ParseForm()
	email := r.FormValue("email")
	password := r.FormValue("password")

	var errMsg string
	var success bool
	if email == "" || password == "" {
		errMsg = "Email and password are required"
	} else {
		err := h.AdminService.CreateUser(r.Context(), email, password)
		if err != nil {
			if err == services.ErrUserExists {
				errMsg = "User with this email already exists"
			} else {
				errMsg = "Failed to create user"
			}
		} else {
			success = true
		}
	}

	if success {
		w.Header().Set("HX-Trigger", "closeAddUserModal")
		_ = templates["add_user_form.html"].ExecuteTemplate(w, "add-user-form", map[string]interface{}{})
		stats, userStats, _ := h.AdminService.GetDashboardStats(r.Context())
		_ = templates["user_list.html"].ExecuteTemplate(w, "user-list", map[string]interface{}{
			"User":      r.Context().Value(AdminUserKey).(db.User),
			"UserStats": userStats,
			"OOB":       true,
		})
		_ = templates["stats.html"].ExecuteTemplate(w, "stats", map[string]interface{}{
			"Stats": stats,
			"OOB":   true,
		})
		w.Write([]byte(`<div id="toast-container" hx-swap-oob="beforeend"><div class="toast">User created successfully</div></div>`))
	} else {
		_ = templates["add_user_form.html"].ExecuteTemplate(w, "add-user-form", map[string]interface{}{
			"Error": errMsg,
			"Email": email,
		})
	}
}

func (h *AdminHandler) HandleUsersDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) > 0 {
			id = parts[len(parts)-1]
		}
	}
	if id == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	err := h.AdminService.DeleteUser(r.Context(), id)
	if err != nil {
		slog.Error("Failed to delete user", "error", err)
	}

	stats, userStats, _ := h.AdminService.GetDashboardStats(r.Context())
	_ = templates["user_list.html"].ExecuteTemplate(w, "user-list", map[string]interface{}{
		"User":      r.Context().Value(AdminUserKey).(db.User),
		"UserStats": userStats,
	})

	_ = templates["stats.html"].ExecuteTemplate(w, "stats", map[string]interface{}{
		"Stats": stats,
		"OOB":   true,
	})
	w.Write([]byte(`<div id="toast-container" hx-swap-oob="beforeend"><div class="toast" style="border-left-color: var(--error);">User deleted successfully</div></div>`))
}

func (h *AdminHandler) HandleUsersPasswordPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) > 1 {
			id = parts[len(parts)-2] // /users/{id}/password
		}
	}
	if id == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	_ = r.ParseForm()
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	var errMsg string
	if newPassword == "" || confirmPassword == "" {
		errMsg = "Both password fields are required"
	} else if newPassword != confirmPassword {
		errMsg = "Passwords do not match"
	} else {
		err := h.AdminService.ChangeUserPassword(r.Context(), id, newPassword)
		if err != nil {
			slog.Error("Failed to update password", "error", err)
			errMsg = "Failed to update password"
		}
	}

	if errMsg != "" {
		w.Header().Set("HX-Retarget", "#change-password-error")
		w.Header().Set("HX-Reswap", "outerHTML")
		// Output the error inside a styled div
		w.Write([]byte(`<div id="change-password-error" class="error-message" style="margin-top: 0.5rem; padding: 0.5rem; display: block;">` + errMsg + `</div>`))
	} else {
		w.Header().Set("HX-Trigger", "closeChangePasswordModal")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<div id="toast-container" hx-swap-oob="beforeend"><div class="toast">Password changed successfully</div></div>`))
	}
}
