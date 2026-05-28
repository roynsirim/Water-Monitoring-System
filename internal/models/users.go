package models

import "time"

// ─── Roles ────────────────────────────────────────────────────────────────────

type Role string

const (
	RoleAdmin   Role = "admin"
	RoleManager Role = "manager"
	RoleUser    Role = "user"
	RoleViewer  Role = "viewer"
)

// IsValidRole returns true if r is a known role.
func IsValidRole(r Role) bool {
	switch r {
	case RoleAdmin, RoleManager, RoleUser, RoleViewer:
		return true
	}
	return false
}

// ─── User ─────────────────────────────────────────────────────────────────────

// User represents an application user. PasswordHash is never serialized to
// API responses (see SafeUser / MarshalJSON).
type User struct {
	ID             string     `json:"id"`
	Email          string     `json:"email"`
	Name           string     `json:"name"`
	Role           Role       `json:"role"`
	IsActive       bool       `json:"is_active"`
	PasswordHash   string     `json:"password_hash,omitempty"`
	MustChangePass bool       `json:"must_change_password"`
	LastLoginAt    *time.Time `json:"last_login_at,omitempty"`
	FailedAttempts int        `json:"failed_attempts"`
	LockedUntil    *time.Time `json:"locked_until,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// SafeUser is a user value sanitised for API output.
func (u User) SafeUser() User {
	u.PasswordHash = ""
	return u
}

// ─── Session ──────────────────────────────────────────────────────────────────

type Session struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	TokenHash string     `json:"token_hash,omitempty"` // SHA-256 of issued token
	IP        string     `json:"ip"`
	UserAgent string     `json:"user_agent"`
	IssuedAt  time.Time  `json:"issued_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

// ─── Activity Log ─────────────────────────────────────────────────────────────

type ActivityLog struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	UserEmail string    `json:"user_email"`
	Action    string    `json:"action"`   // login, logout, create_user, update_user, ...
	Resource  string    `json:"resource"` // affected resource id (optional)
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Status    string    `json:"status"` // success | failure
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ─── Password Reset ───────────────────────────────────────────────────────────

type PasswordReset struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	TokenHash string     `json:"token_hash,omitempty"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}
