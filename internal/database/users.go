package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"water-monitoring-system/internal/models"

	"github.com/google/uuid"
)

// UserStore persists users, sessions, activity logs, and password resets to a
// separate JSON file (next to the main water.json) so that user data has its
// own backup/rotation lifecycle.
type UserStore struct {
	mu       sync.RWMutex
	path     string
	Users    []models.User           `json:"users"`
	Sessions []models.Session        `json:"sessions"`
	Activity []models.ActivityLog    `json:"activity"`
	Resets   []models.PasswordReset  `json:"resets"`
}

// OpenUserStore loads (or creates) the user-store JSON file.
func OpenUserStore(mainDBPath string) (*UserStore, error) {
	dir := filepath.Dir(mainDBPath)
	path := filepath.Join(dir, "users.json")
	s := &UserStore{path: path}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, s); err != nil {
			return nil, fmt.Errorf("parsing users store: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if s.Users == nil {
		s.Users = []models.User{}
	}
	if s.Sessions == nil {
		s.Sessions = []models.Session{}
	}
	if s.Activity == nil {
		s.Activity = []models.ActivityLog{}
	}
	if s.Resets == nil {
		s.Resets = []models.PasswordReset{}
	}
	return s, nil
}

func (s *UserStore) save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	// atomic write
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// ─── Users ────────────────────────────────────────────────────────────────────

var ErrNotFound = errors.New("not found")
var ErrEmailExists = errors.New("email already exists")

// CreateUser inserts a new user. PasswordHash must already be set.
func (s *UserStore) CreateUser(u models.User) (models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))
	for _, ex := range s.Users {
		if strings.EqualFold(ex.Email, u.Email) {
			return models.User{}, ErrEmailExists
		}
	}
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now
	s.Users = append(s.Users, u)
	if err := s.save(); err != nil {
		return models.User{}, err
	}
	return u.SafeUser(), nil
}

// GetUser returns the user with the given ID.
func (s *UserStore) GetUser(id string) (models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.Users {
		if u.ID == id {
			return u, nil
		}
	}
	return models.User{}, ErrNotFound
}

// GetUserByEmail returns the user with the given email (case-insensitive).
func (s *UserStore) GetUserByEmail(email string) (models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	email = strings.ToLower(strings.TrimSpace(email))
	for _, u := range s.Users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return models.User{}, ErrNotFound
}

// ListUsers returns all users (sanitised).
func (s *UserStore) ListUsers() []models.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.User, 0, len(s.Users))
	for _, u := range s.Users {
		out = append(out, u.SafeUser())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Email < out[j].Email })
	return out
}

// UpdateUser modifies mutable fields. Use UpdatePassword for password changes.
func (s *UserStore) UpdateUser(id string, patch func(*models.User) error) (models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Users {
		if s.Users[i].ID == id {
			if err := patch(&s.Users[i]); err != nil {
				return models.User{}, err
			}
			s.Users[i].UpdatedAt = time.Now()
			if err := s.save(); err != nil {
				return models.User{}, err
			}
			return s.Users[i].SafeUser(), nil
		}
	}
	return models.User{}, ErrNotFound
}

// DeleteUser removes a user and revokes their sessions.
func (s *UserStore) DeleteUser(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, u := range s.Users {
		if u.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrNotFound
	}
	s.Users = append(s.Users[:idx], s.Users[idx+1:]...)
	now := time.Now()
	for i := range s.Sessions {
		if s.Sessions[i].UserID == id && s.Sessions[i].RevokedAt == nil {
			s.Sessions[i].RevokedAt = &now
		}
	}
	return s.save()
}

// ─── Sessions ─────────────────────────────────────────────────────────────────

func (s *UserStore) CreateSession(sess models.Session) (models.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess.ID == "" {
		sess.ID = uuid.New().String()
	}
	if sess.IssuedAt.IsZero() {
		sess.IssuedAt = time.Now()
	}
	s.Sessions = append(s.Sessions, sess)
	return sess, s.save()
}

// FindSessionByTokenHash returns a non-revoked session matching the SHA-256
// hash of the bearer token.
func (s *UserStore) FindSessionByTokenHash(hash string) (models.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	for _, sess := range s.Sessions {
		if sess.TokenHash == hash && sess.RevokedAt == nil && sess.ExpiresAt.After(now) {
			return sess, nil
		}
	}
	return models.Session{}, ErrNotFound
}

// RevokeSession marks a session as revoked.
func (s *UserStore) RevokeSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for i := range s.Sessions {
		if s.Sessions[i].ID == id && s.Sessions[i].RevokedAt == nil {
			s.Sessions[i].RevokedAt = &now
			return s.save()
		}
	}
	return ErrNotFound
}

// RevokeUserSessions revokes ALL active sessions for a user (e.g. on password
// reset).
func (s *UserStore) RevokeUserSessions(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for i := range s.Sessions {
		if s.Sessions[i].UserID == userID && s.Sessions[i].RevokedAt == nil {
			s.Sessions[i].RevokedAt = &now
		}
	}
	return s.save()
}

// PurgeExpiredSessions trims sessions older than 30d past expiry.
func (s *UserStore) PurgeExpiredSessions() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().AddDate(0, 0, -30)
	kept := s.Sessions[:0]
	for _, sess := range s.Sessions {
		if sess.ExpiresAt.After(cutoff) {
			kept = append(kept, sess)
		}
	}
	s.Sessions = kept
	return s.save()
}

// ─── Activity Log ─────────────────────────────────────────────────────────────

// LogActivity appends an audit entry. Errors are swallowed (logged by caller)
// because logging must never block the main flow.
func (s *UserStore) LogActivity(entry models.ActivityLog) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	s.Activity = append(s.Activity, entry)
	// Cap to last 10k entries to keep file size bounded.
	if len(s.Activity) > 10000 {
		s.Activity = s.Activity[len(s.Activity)-10000:]
	}
	_ = s.save()
}

// ListActivity returns activity log entries, optionally filtered by user.
func (s *UserStore) ListActivity(userID string, limit int) []models.ActivityLog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	out := make([]models.ActivityLog, 0, limit)
	// iterate newest-first
	for i := len(s.Activity) - 1; i >= 0 && len(out) < limit; i-- {
		if userID != "" && s.Activity[i].UserID != userID {
			continue
		}
		out = append(out, s.Activity[i])
	}
	return out
}

// ─── Password Resets ──────────────────────────────────────────────────────────

func (s *UserStore) CreatePasswordReset(pr models.PasswordReset) (models.PasswordReset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pr.ID == "" {
		pr.ID = uuid.New().String()
	}
	if pr.CreatedAt.IsZero() {
		pr.CreatedAt = time.Now()
	}
	s.Resets = append(s.Resets, pr)
	return pr, s.save()
}

func (s *UserStore) ConsumePasswordReset(tokenHash string) (models.PasswordReset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for i := range s.Resets {
		if s.Resets[i].TokenHash == tokenHash && s.Resets[i].UsedAt == nil && s.Resets[i].ExpiresAt.After(now) {
			s.Resets[i].UsedAt = &now
			if err := s.save(); err != nil {
				return models.PasswordReset{}, err
			}
			return s.Resets[i], nil
		}
	}
	return models.PasswordReset{}, ErrNotFound
}
