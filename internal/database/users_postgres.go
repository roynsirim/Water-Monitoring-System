package database

import (
	"database/sql"
	"sort"
	"strings"
	"time"

	"water-monitoring-system/internal/models"

	"github.com/google/uuid"
)

// PostgresUserStore implements user/session/activity storage in PostgreSQL
type PostgresUserStore struct {
	db *sql.DB
}

// OpenPostgresUserStore creates a new PostgreSQL-backed user store
func OpenPostgresUserStore(db *sql.DB) (*PostgresUserStore, error) {
	store := &PostgresUserStore{db: db}
	if err := store.initSchema(); err != nil {
		return nil, err
	}
	return store, nil
}

// initSchema creates user-related tables if they don't exist
func (s *PostgresUserStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id VARCHAR(64) PRIMARY KEY,
		email VARCHAR(256) NOT NULL UNIQUE,
		name VARCHAR(256) NOT NULL,
		role VARCHAR(32) NOT NULL DEFAULT 'user',
		is_active BOOLEAN NOT NULL DEFAULT true,
		password_hash TEXT NOT NULL,
		must_change_password BOOLEAN NOT NULL DEFAULT true,
		last_login_at TIMESTAMP WITH TIME ZONE,
		failed_attempts INTEGER NOT NULL DEFAULT 0,
		locked_until TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS sessions (
		id VARCHAR(64) PRIMARY KEY,
		user_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		token_hash VARCHAR(128) NOT NULL,
		ip VARCHAR(64),
		user_agent TEXT,
		issued_at TIMESTAMP WITH TIME ZONE NOT NULL,
		expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
		revoked_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS activity_log (
		id VARCHAR(64) PRIMARY KEY,
		user_id VARCHAR(64) NOT NULL,
		user_email VARCHAR(256) NOT NULL,
		action VARCHAR(64) NOT NULL,
		resource VARCHAR(256),
		ip VARCHAR(64),
		user_agent TEXT,
		status VARCHAR(32) NOT NULL,
		detail TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS password_resets (
		id VARCHAR(64) PRIMARY KEY,
		user_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		token_hash VARCHAR(128) NOT NULL,
		expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
		used_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);

	-- Create indexes for common queries
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);
	CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
	CREATE INDEX IF NOT EXISTS idx_activity_user_id ON activity_log(user_id);
	CREATE INDEX IF NOT EXISTS idx_activity_created_at ON activity_log(created_at);
	CREATE INDEX IF NOT EXISTS idx_password_resets_token_hash ON password_resets(token_hash);
	`

	_, err := s.db.Exec(schema)
	return err
}

// ─── Users ────────────────────────────────────────────────────────────────────

// CreateUser inserts a new user. PasswordHash must already be set.
func (s *PostgresUserStore) CreateUser(u models.User) (models.User, error) {
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))

	// Check if email already exists
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(email) = LOWER($1))", u.Email).Scan(&exists)
	if err != nil {
		return models.User{}, err
	}
	if exists {
		return models.User{}, ErrEmailExists
	}

	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now

	_, err = s.db.Exec(`
		INSERT INTO users (id, email, name, role, is_active, password_hash, must_change_password, 
			last_login_at, failed_attempts, locked_until, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, u.ID, u.Email, u.Name, u.Role, u.IsActive, u.PasswordHash, u.MustChangePass,
		u.LastLoginAt, u.FailedAttempts, u.LockedUntil, u.CreatedAt, u.UpdatedAt)

	if err != nil {
		return models.User{}, err
	}
	return u.SafeUser(), nil
}

// GetUser returns the user with the given ID.
func (s *PostgresUserStore) GetUser(id string) (models.User, error) {
	var u models.User
	var lastLoginAt, lockedUntil sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, email, name, role, is_active, password_hash, must_change_password,
			last_login_at, failed_attempts, locked_until, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.IsActive, &u.PasswordHash, &u.MustChangePass,
		&lastLoginAt, &u.FailedAttempts, &lockedUntil, &u.CreatedAt, &u.UpdatedAt)

	if err == sql.ErrNoRows {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, err
	}

	if lastLoginAt.Valid {
		u.LastLoginAt = &lastLoginAt.Time
	}
	if lockedUntil.Valid {
		u.LockedUntil = &lockedUntil.Time
	}

	return u, nil
}

// GetUserByEmail returns the user with the given email (case-insensitive).
func (s *PostgresUserStore) GetUserByEmail(email string) (models.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var u models.User
	var lastLoginAt, lockedUntil sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, email, name, role, is_active, password_hash, must_change_password,
			last_login_at, failed_attempts, locked_until, created_at, updated_at
		FROM users WHERE LOWER(email) = LOWER($1)
	`, email).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.IsActive, &u.PasswordHash, &u.MustChangePass,
		&lastLoginAt, &u.FailedAttempts, &lockedUntil, &u.CreatedAt, &u.UpdatedAt)

	if err == sql.ErrNoRows {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, err
	}

	if lastLoginAt.Valid {
		u.LastLoginAt = &lastLoginAt.Time
	}
	if lockedUntil.Valid {
		u.LockedUntil = &lockedUntil.Time
	}

	return u, nil
}

// ListUsers returns all users (sanitised).
func (s *PostgresUserStore) ListUsers() []models.User {
	rows, err := s.db.Query(`
		SELECT id, email, name, role, is_active, must_change_password,
			last_login_at, failed_attempts, locked_until, created_at, updated_at
		FROM users ORDER BY email
	`)
	if err != nil {
		return []models.User{}
	}
	defer rows.Close()

	users := []models.User{}
	for rows.Next() {
		var u models.User
		var lastLoginAt, lockedUntil sql.NullTime

		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.IsActive, &u.MustChangePass,
			&lastLoginAt, &u.FailedAttempts, &lockedUntil, &u.CreatedAt, &u.UpdatedAt); err != nil {
			continue
		}

		if lastLoginAt.Valid {
			u.LastLoginAt = &lastLoginAt.Time
		}
		if lockedUntil.Valid {
			u.LockedUntil = &lockedUntil.Time
		}

		users = append(users, u.SafeUser())
	}

	if err := rows.Err(); err != nil {
		return []models.User{}
	}

	sort.Slice(users, func(i, j int) bool { return users[i].Email < users[j].Email })
	return users
}

// UpdateUser modifies mutable fields.
func (s *PostgresUserStore) UpdateUser(id string, patch func(*models.User) error) (models.User, error) {
	// Get current user
	u, err := s.GetUser(id)
	if err != nil {
		return models.User{}, err
	}

	// Apply patch
	if err := patch(&u); err != nil {
		return models.User{}, err
	}
	u.UpdatedAt = time.Now()

	// Update in database
	_, err = s.db.Exec(`
		UPDATE users SET
			email = $2, name = $3, role = $4, is_active = $5, password_hash = $6,
			must_change_password = $7, last_login_at = $8, failed_attempts = $9,
			locked_until = $10, updated_at = $11
		WHERE id = $1
	`, u.ID, u.Email, u.Name, u.Role, u.IsActive, u.PasswordHash, u.MustChangePass,
		u.LastLoginAt, u.FailedAttempts, u.LockedUntil, u.UpdatedAt)

	if err != nil {
		return models.User{}, err
	}

	return u.SafeUser(), nil
}

// DeleteUser removes a user and revokes their sessions.
func (s *PostgresUserStore) DeleteUser(id string) error {
	// Check user exists
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", id).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}

	// Sessions will be deleted by CASCADE, but let's revoke them first for audit
	now := time.Now()
	_, err = s.db.Exec(`
		UPDATE sessions SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL
	`, id, now)
	if err != nil {
		return err
	}

	// Delete user (cascades to sessions)
	_, err = s.db.Exec("DELETE FROM users WHERE id = $1", id)
	return err
}

// ─── Sessions ─────────────────────────────────────────────────────────────────

func (s *PostgresUserStore) CreateSession(sess models.Session) (models.Session, error) {
	if sess.ID == "" {
		sess.ID = uuid.New().String()
	}
	if sess.IssuedAt.IsZero() {
		sess.IssuedAt = time.Now()
	}

	_, err := s.db.Exec(`
		INSERT INTO sessions (id, user_id, token_hash, ip, user_agent, issued_at, expires_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, sess.ID, sess.UserID, sess.TokenHash, sess.IP, sess.UserAgent, sess.IssuedAt, sess.ExpiresAt, sess.RevokedAt)

	if err != nil {
		return models.Session{}, err
	}
	return sess, nil
}

// FindSessionByTokenHash returns a non-revoked session matching the SHA-256 hash.
func (s *PostgresUserStore) FindSessionByTokenHash(hash string) (models.Session, error) {
	var sess models.Session
	var revokedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, user_id, token_hash, ip, user_agent, issued_at, expires_at, revoked_at
		FROM sessions
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2
	`, hash, time.Now()).Scan(&sess.ID, &sess.UserID, &sess.TokenHash, &sess.IP, &sess.UserAgent,
		&sess.IssuedAt, &sess.ExpiresAt, &revokedAt)

	if err == sql.ErrNoRows {
		return models.Session{}, ErrNotFound
	}
	if err != nil {
		return models.Session{}, err
	}

	if revokedAt.Valid {
		sess.RevokedAt = &revokedAt.Time
	}

	return sess, nil
}

// RevokeSession marks a session as revoked.
func (s *PostgresUserStore) RevokeSession(id string) error {
	result, err := s.db.Exec(`
		UPDATE sessions SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL
	`, id, time.Now())
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeUserSessions revokes ALL active sessions for a user.
func (s *PostgresUserStore) RevokeUserSessions(userID string) error {
	_, err := s.db.Exec(`
		UPDATE sessions SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL
	`, userID, time.Now())
	return err
}

// PurgeExpiredSessions trims sessions older than 30d past expiry.
func (s *PostgresUserStore) PurgeExpiredSessions() error {
	cutoff := time.Now().AddDate(0, 0, -30)
	_, err := s.db.Exec("DELETE FROM sessions WHERE expires_at < $1", cutoff)
	return err
}

// ─── Activity Log ─────────────────────────────────────────────────────────────

// LogActivity appends an audit entry.
func (s *PostgresUserStore) LogActivity(entry models.ActivityLog) {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	_, _ = s.db.Exec(`
		INSERT INTO activity_log (id, user_id, user_email, action, resource, ip, user_agent, status, detail, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, entry.ID, entry.UserID, entry.UserEmail, entry.Action, entry.Resource, entry.IP, entry.UserAgent,
		entry.Status, entry.Detail, entry.CreatedAt)

	// Trim old entries to keep table bounded (keep last 10k)
	_, _ = s.db.Exec(`
		DELETE FROM activity_log WHERE id IN (
			SELECT id FROM activity_log ORDER BY created_at DESC OFFSET 10000
		)
	`)
}

// ListActivity returns activity log entries, optionally filtered by user.
func (s *PostgresUserStore) ListActivity(userID string, limit int) []models.ActivityLog {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}

	var rows *sql.Rows
	var err error

	if userID != "" {
		rows, err = s.db.Query(`
			SELECT id, user_id, user_email, action, COALESCE(resource, ''), ip, COALESCE(user_agent, ''), 
				status, COALESCE(detail, ''), created_at
			FROM activity_log
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT $2
		`, userID, limit)
	} else {
		rows, err = s.db.Query(`
			SELECT id, user_id, user_email, action, COALESCE(resource, ''), ip, COALESCE(user_agent, ''), 
				status, COALESCE(detail, ''), created_at
			FROM activity_log
			ORDER BY created_at DESC
			LIMIT $1
		`, limit)
	}

	if err != nil {
		return []models.ActivityLog{}
	}
	defer rows.Close()

	entries := []models.ActivityLog{}
	for rows.Next() {
		var entry models.ActivityLog
		if err := rows.Scan(&entry.ID, &entry.UserID, &entry.UserEmail, &entry.Action, &entry.Resource,
			&entry.IP, &entry.UserAgent, &entry.Status, &entry.Detail, &entry.CreatedAt); err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return []models.ActivityLog{}
	}

	return entries
}

// ─── Password Resets ──────────────────────────────────────────────────────────

func (s *PostgresUserStore) CreatePasswordReset(pr models.PasswordReset) (models.PasswordReset, error) {
	if pr.ID == "" {
		pr.ID = uuid.New().String()
	}
	if pr.CreatedAt.IsZero() {
		pr.CreatedAt = time.Now()
	}

	_, err := s.db.Exec(`
		INSERT INTO password_resets (id, user_id, token_hash, expires_at, used_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, pr.ID, pr.UserID, pr.TokenHash, pr.ExpiresAt, pr.UsedAt, pr.CreatedAt)

	if err != nil {
		return models.PasswordReset{}, err
	}
	return pr, nil
}

func (s *PostgresUserStore) ConsumePasswordReset(tokenHash string) (models.PasswordReset, error) {
	var pr models.PasswordReset
	var usedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, user_id, token_hash, expires_at, used_at, created_at
		FROM password_resets
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2
	`, tokenHash, time.Now()).Scan(&pr.ID, &pr.UserID, &pr.TokenHash, &pr.ExpiresAt, &usedAt, &pr.CreatedAt)

	if err == sql.ErrNoRows {
		return models.PasswordReset{}, ErrNotFound
	}
	if err != nil {
		return models.PasswordReset{}, err
	}

	// Mark as used
	now := time.Now()
	_, err = s.db.Exec("UPDATE password_resets SET used_at = $2 WHERE id = $1", pr.ID, now)
	if err != nil {
		return models.PasswordReset{}, err
	}

	pr.UsedAt = &now
	return pr, nil
}

// ─── Ensure interface compatibility ───────────────────────────────────────────

// Compile-time check that PostgresUserStore implements the same methods as UserStore
var _ interface {
	CreateUser(u models.User) (models.User, error)
	GetUser(id string) (models.User, error)
	GetUserByEmail(email string) (models.User, error)
	ListUsers() []models.User
	UpdateUser(id string, patch func(*models.User) error) (models.User, error)
	DeleteUser(id string) error
	CreateSession(sess models.Session) (models.Session, error)
	FindSessionByTokenHash(hash string) (models.Session, error)
	RevokeSession(id string) error
	RevokeUserSessions(userID string) error
	PurgeExpiredSessions() error
	LogActivity(entry models.ActivityLog)
	ListActivity(userID string, limit int) []models.ActivityLog
	CreatePasswordReset(pr models.PasswordReset) (models.PasswordReset, error)
	ConsumePasswordReset(tokenHash string) (models.PasswordReset, error)
} = (*PostgresUserStore)(nil)
