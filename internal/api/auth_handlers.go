package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"water-monitoring-system/internal/auth"
	"water-monitoring-system/internal/database"
	"water-monitoring-system/internal/models"
)

// AuthHandler bundles auth-related endpoints.
type AuthHandler struct {
	Users      *database.UserStore
	JWTSecret  string
	SessionTTL time.Duration
	BcryptCost int
}

// ─── Login / Logout / Me ─────────────────────────────────────────────────────

func (a *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method not allowed")
		return
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	body.Email = strings.ToLower(strings.TrimSpace(body.Email))
	if body.Email == "" || body.Password == "" {
		writeError(w, 400, "email and password required")
		return
	}

	u, err := a.Users.GetUserByEmail(body.Email)
	if err != nil {
		a.audit(models.ActivityLog{UserEmail: body.Email, Action: "login", Status: "failure", Detail: "no such user", IP: clientIP(r), UserAgent: r.UserAgent()})
		writeError(w, 401, "invalid email or password")
		return
	}
	if !u.IsActive {
		a.audit(models.ActivityLog{UserID: u.ID, UserEmail: u.Email, Action: "login", Status: "failure", Detail: "inactive", IP: clientIP(r), UserAgent: r.UserAgent()})
		writeError(w, 403, "account disabled")
		return
	}
	if u.LockedUntil != nil && u.LockedUntil.After(time.Now()) {
		writeError(w, 423, "account temporarily locked")
		return
	}

	if err := auth.VerifyPassword(u.PasswordHash, body.Password); err != nil {
		// Increment failure counter, lock after 5
		_, _ = a.Users.UpdateUser(u.ID, func(uu *models.User) error {
			uu.FailedAttempts++
			if uu.FailedAttempts >= 5 {
				t := time.Now().Add(15 * time.Minute)
				uu.LockedUntil = &t
				uu.FailedAttempts = 0
			}
			return nil
		})
		a.audit(models.ActivityLog{UserID: u.ID, UserEmail: u.Email, Action: "login", Status: "failure", Detail: "bad password", IP: clientIP(r), UserAgent: r.UserAgent()})
		writeError(w, 401, "invalid email or password")
		return
	}

	// Issue token + session
	now := time.Now()
	exp := now.Add(a.SessionTTL)
	claims := auth.Claims{
		SessionID: "",
		UserID:    u.ID,
		Role:      string(u.Role),
		Email:     u.Email,
		IssuedAt:  now.Unix(),
		ExpiresAt: exp.Unix(),
	}
	tok, err := auth.Sign(claims, a.JWTSecret)
	if err != nil {
		writeError(w, 500, "could not issue token")
		return
	}
	sess, err := a.Users.CreateSession(models.Session{
		UserID:    u.ID,
		TokenHash: auth.HashToken(tok),
		IP:        clientIP(r),
		UserAgent: r.UserAgent(),
		IssuedAt:  now,
		ExpiresAt: exp,
	})
	if err != nil {
		writeError(w, 500, "could not create session")
		return
	}

	_, _ = a.Users.UpdateUser(u.ID, func(uu *models.User) error {
		t := time.Now()
		uu.LastLoginAt = &t
		uu.FailedAttempts = 0
		uu.LockedUntil = nil
		return nil
	})

	a.audit(models.ActivityLog{UserID: u.ID, UserEmail: u.Email, Action: "login", Status: "success", IP: clientIP(r), UserAgent: r.UserAgent(), Resource: sess.ID})

	// Set HTTP-only session cookie for browser redirects
	http.SetCookie(w, &http.Cookie{
		Name:     "wms_session",
		Value:    tok,
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	respondJSON(w, 200, map[string]any{
		"token":      tok,
		"expires_at": exp,
		"user":       u.SafeUser(),
	})
}

func (a *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "wms_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	tok := extractToken(r)
	if tok == "" {
		respondJSON(w, 200, map[string]string{"status": "ok"})
		return
	}
	sess, err := a.Users.FindSessionByTokenHash(auth.HashToken(tok))
	if err == nil {
		_ = a.Users.RevokeSession(sess.ID)
		u, _ := CurrentUser(r)
		a.audit(models.ActivityLog{UserID: u.ID, UserEmail: u.Email, Action: "logout", Status: "success", IP: clientIP(r), UserAgent: r.UserAgent(), Resource: sess.ID})
	}
	respondJSON(w, 200, map[string]string{"status": "ok"})
}

func (a *AuthHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r)
	if !ok {
		writeError(w, 401, "authentication required")
		return
	}
	respondJSON(w, 200, u.SafeUser())
}

// HandleChangeMyPassword lets the logged-in user change their own password.
func (a *AuthHandler) HandleChangeMyPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method not allowed")
		return
	}
	u, ok := CurrentUser(r)
	if !ok {
		writeError(w, 401, "authentication required")
		return
	}
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	// Reload to get the hash.
	full, _ := a.Users.GetUser(u.ID)
	if err := auth.VerifyPassword(full.PasswordHash, body.CurrentPassword); err != nil {
		writeError(w, 401, "current password incorrect")
		return
	}
	if err := auth.ValidatePasswordStrength(body.NewPassword); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	hash, err := auth.HashPassword(body.NewPassword, a.BcryptCost)
	if err != nil {
		writeError(w, 500, "could not hash password")
		return
	}
	_, err = a.Users.UpdateUser(u.ID, func(uu *models.User) error {
		uu.PasswordHash = hash
		uu.MustChangePass = false
		return nil
	})
	if err != nil {
		writeError(w, 500, "could not update user")
		return
	}
	// Revoke other sessions for security.
	_ = a.Users.RevokeUserSessions(u.ID)
	a.audit(models.ActivityLog{UserID: u.ID, UserEmail: u.Email, Action: "change_password", Status: "success", IP: clientIP(r), UserAgent: r.UserAgent()})
	respondJSON(w, 200, map[string]string{"status": "password updated; please log in again"})
}

func (a *AuthHandler) audit(l models.ActivityLog) {
	a.Users.LogActivity(l)
}

func respondJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
