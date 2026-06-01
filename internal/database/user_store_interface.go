package database

import (
	"database/sql"

	"water-monitoring-system/internal/models"
)

// UserStoreInterface defines the interface for user/session/activity storage
// Both JSON-based UserStore and PostgresUserStore implement this interface
type UserStoreInterface interface {
	// Users
	CreateUser(u models.User) (models.User, error)
	GetUser(id string) (models.User, error)
	GetUserByEmail(email string) (models.User, error)
	ListUsers() []models.User
	UpdateUser(id string, patch func(*models.User) error) (models.User, error)
	DeleteUser(id string) error

	// Sessions
	CreateSession(sess models.Session) (models.Session, error)
	FindSessionByTokenHash(hash string) (models.Session, error)
	RevokeSession(id string) error
	RevokeUserSessions(userID string) error
	PurgeExpiredSessions() error

	// Activity Log
	LogActivity(entry models.ActivityLog)
	ListActivity(userID string, limit int) []models.ActivityLog

	// Password Resets
	CreatePasswordReset(pr models.PasswordReset) (models.PasswordReset, error)
	ConsumePasswordReset(tokenHash string) (models.PasswordReset, error)
}

// Ensure both implementations satisfy the interface
var _ UserStoreInterface = (*UserStore)(nil)
var _ UserStoreInterface = (*PostgresUserStore)(nil)

// OpenUserStoreForDriver opens the appropriate user store based on driver type
func OpenUserStoreForDriver(driver, jsonPath string, db *sql.DB) (UserStoreInterface, error) {
	switch driver {
	case "postgres":
		return OpenPostgresUserStore(db)
	default:
		// Default to JSON store
		return OpenUserStore(jsonPath)
	}
}
