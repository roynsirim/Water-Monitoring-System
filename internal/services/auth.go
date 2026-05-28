// Package services contains business logic separated from HTTP handlers.
// Services encapsulate domain operations, making handlers thin and testable.
package services

import (
	"errors"
	"strings"
	"time"

	"water-monitoring-system/internal/auth"
	"water-monitoring-system/internal/database"
	"water-monitoring-system/internal/models"
)

// ─── Auth Service ─────────────────────────────────────────────────────────────

// AuthService handles authentication business logic.
type AuthService struct {
	Users      *database.UserStore
	JWTSecret  string
	SessionTTL time.Duration
	BcryptCost int
}

// AuthServiceConfig configures AuthService.
type AuthServiceConfig struct {
	Users      *database.UserStore
	JWTSecret  string
	SessionTTL time.Duration
	BcryptCost int
}

// NewAuthService creates a new AuthService.
func NewAuthService(cfg AuthServiceConfig) *AuthService {
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 12 * time.Hour
	}
	if cfg.BcryptCost == 0 {
		cfg.BcryptCost = 12
	}
	return &AuthService{
		Users:      cfg.Users,
		JWTSecret:  cfg.JWTSecret,
		SessionTTL: cfg.SessionTTL,
		BcryptCost: cfg.BcryptCost,
	}
}

// LoginResult contains successful login response.
type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	User      models.User
}

// ErrInvalidCredentials indicates login failed.
var ErrInvalidCredentials = errors.New("invalid email or password")

// ErrAccountLocked indicates too many failed attempts.
var ErrAccountLocked = errors.New("account locked due to too many failed attempts")

// ErrAccountDisabled indicates the user is deactivated.
var ErrAccountDisabled = errors.New("account is disabled")

// MaxFailedAttempts before lockout.
const MaxFailedAttempts = 5

// LockoutDuration is how long accounts stay locked.
const LockoutDuration = 15 * time.Minute

// Login authenticates a user and returns a session token.
func (s *AuthService) Login(email, password, ip, userAgent string) (*LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	user, err := s.Users.GetUserByEmail(email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// Check lockout
	if user.FailedAttempts >= MaxFailedAttempts {
		if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
			return nil, ErrAccountLocked
		}
		// Lockout expired, reset
		s.Users.UpdateUser(user.ID, func(u *models.User) error {
			u.FailedAttempts = 0
			u.LockedUntil = nil
			return nil
		})
		user.FailedAttempts = 0
	}

	// Check password
	if err := auth.VerifyPassword(user.PasswordHash, password); err != nil {
		// Increment failed attempts
		s.Users.UpdateUser(user.ID, func(u *models.User) error {
			u.FailedAttempts++
			if u.FailedAttempts >= MaxFailedAttempts {
				lockUntil := time.Now().Add(LockoutDuration)
				u.LockedUntil = &lockUntil
			}
			return nil
		})
		return nil, ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, ErrAccountDisabled
	}

	// Generate token
	now := time.Now()
	expiresAt := now.Add(s.SessionTTL)
	rawToken, err := auth.RandomToken(32)
	if err != nil {
		return nil, err
	}
	tokenHash := auth.HashToken(rawToken)

	claims := auth.Claims{
		UserID:    user.ID,
		Role:      string(user.Role),
		Email:     user.Email,
		IssuedAt:  now.Unix(),
		ExpiresAt: expiresAt.Unix(),
	}
	signedToken, err := auth.Sign(claims, s.JWTSecret)
	if err != nil {
		return nil, err
	}

	// Create session
	sess := models.Session{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		IP:        ip,
		UserAgent: userAgent,
	}
	sess, err = s.Users.CreateSession(sess)
	if err != nil {
		return nil, err
	}

	// Update login timestamp, reset failed attempts
	s.Users.UpdateUser(user.ID, func(u *models.User) error {
		u.LastLoginAt = &now
		u.FailedAttempts = 0
		u.LockedUntil = nil
		return nil
	})

	// Log activity
	s.Users.LogActivity(models.ActivityLog{
		UserID:    user.ID,
		UserEmail: user.Email,
		Action:    "login",
		Resource:  sess.ID,
		IP:        ip,
		UserAgent: userAgent,
		Status:    "success",
	})

	return &LoginResult{
		Token:     signedToken,
		ExpiresAt: expiresAt,
		User:      user.SafeUser(),
	}, nil
}

// Logout revokes a session.
func (s *AuthService) Logout(sessionID, userID, email, ip, userAgent string) error {
	if err := s.Users.RevokeSession(sessionID); err != nil {
		return err
	}
	s.Users.LogActivity(models.ActivityLog{
		UserID:    userID,
		UserEmail: email,
		Action:    "logout",
		IP:        ip,
		UserAgent: userAgent,
		Status:    "success",
	})
	return nil
}

// ChangePassword changes a user's password and revokes sessions.
func (s *AuthService) ChangePassword(userID, currentPassword, newPassword, ip, userAgent string) error {
	user, err := s.Users.GetUser(userID)
	if err != nil {
		return err
	}

	// Verify current password
	if err := auth.VerifyPassword(user.PasswordHash, currentPassword); err != nil {
		s.Users.LogActivity(models.ActivityLog{
			UserID:    userID,
			UserEmail: user.Email,
			Action:    "password_change",
			IP:        ip,
			UserAgent: userAgent,
			Status:    "failed",
			Detail:    "wrong current password",
		})
		return errors.New("current password is incorrect")
	}

	// Validate new password
	if err := auth.ValidatePasswordStrength(newPassword); err != nil {
		return err
	}

	// Hash new password
	newHash, err := auth.HashPassword(newPassword, s.BcryptCost)
	if err != nil {
		return err
	}

	// Update user
	_, err = s.Users.UpdateUser(userID, func(u *models.User) error {
		u.PasswordHash = newHash
		u.MustChangePass = false
		return nil
	})
	if err != nil {
		return err
	}

	// Revoke all sessions
	s.Users.RevokeUserSessions(userID)

	s.Users.LogActivity(models.ActivityLog{
		UserID:    userID,
		UserEmail: user.Email,
		Action:    "password_change",
		IP:        ip,
		UserAgent: userAgent,
		Status:    "success",
	})

	return nil
}

// ValidateSession checks if a token is valid and returns the user.
func (s *AuthService) ValidateSession(token string) (*models.User, *auth.Claims, error) {
	claims, err := auth.Verify(token, s.JWTSecret)
	if err != nil {
		return nil, nil, err
	}

	// Check session not revoked
	_, err = s.Users.FindSessionByTokenHash(auth.HashToken(token))
	if err != nil {
		return nil, nil, errors.New("session revoked")
	}

	user, err := s.Users.GetUser(claims.UserID)
	if err != nil {
		return nil, nil, err
	}

	if !user.IsActive {
		return nil, nil, ErrAccountDisabled
	}

	return &user, claims, nil
}
