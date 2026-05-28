package database

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"water-monitoring-system/internal/models"
)

// ─── Test Helpers ─────────────────────────────────────────────────────────────

func setupTestStore(t *testing.T) (*UserStore, func()) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "water.json")
	
	// Create empty water.json
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}

	store, err := OpenUserStore(path)
	if err != nil {
		t.Fatalf("OpenUserStore() error = %v", err)
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}
	return store, cleanup
}

// ─── User CRUD Tests ──────────────────────────────────────────────────────────

func TestCreateUser(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user := models.User{
		Email:        "test@example.com",
		Name:         "Test User",
		PasswordHash: "hashed-password",
		Role:         models.RoleUser,
		IsActive:     true,
	}

	created, err := store.CreateUser(user)
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if created.ID == "" {
		t.Error("CreateUser() should generate ID")
	}
	if created.Email != "test@example.com" {
		t.Errorf("CreateUser() email = %v, want test@example.com", created.Email)
	}
	if created.CreatedAt.IsZero() {
		t.Error("CreateUser() should set CreatedAt")
	}
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user := models.User{
		Email:        "test@example.com",
		PasswordHash: "hash",
		Role:         models.RoleUser,
	}

	_, err := store.CreateUser(user)
	if err != nil {
		t.Fatalf("first CreateUser() error = %v", err)
	}

	_, err = store.CreateUser(user)
	if err != ErrEmailExists {
		t.Errorf("second CreateUser() error = %v, want ErrEmailExists", err)
	}
}

func TestCreateUserEmailCaseInsensitive(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	_, err := store.CreateUser(models.User{
		Email: "Test@Example.com",
		Role:  models.RoleUser,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	_, err = store.CreateUser(models.User{
		Email: "test@example.com",
		Role:  models.RoleUser,
	})
	if err != ErrEmailExists {
		t.Error("CreateUser() should detect case-insensitive duplicate")
	}
}

func TestGetUser(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	created, _ := store.CreateUser(models.User{
		Email: "test@example.com",
		Role:  models.RoleUser,
	})

	got, err := store.GetUser(created.ID)
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if got.Email != "test@example.com" {
		t.Errorf("GetUser() email = %v, want test@example.com", got.Email)
	}
}

func TestGetUserNotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	_, err := store.GetUser("nonexistent-id")
	if err != ErrNotFound {
		t.Errorf("GetUser() error = %v, want ErrNotFound", err)
	}
}

func TestGetUserByEmail(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	store.CreateUser(models.User{
		Email: "Test@Example.COM",
		Name:  "Test User",
		Role:  models.RoleUser,
	})

	// Case-insensitive lookup
	got, err := store.GetUserByEmail("test@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail() error = %v", err)
	}
	if got.Name != "Test User" {
		t.Errorf("GetUserByEmail() name = %v, want Test User", got.Name)
	}
}

func TestListUsers(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	store.CreateUser(models.User{Email: "bob@example.com", Role: models.RoleUser})
	store.CreateUser(models.User{Email: "alice@example.com", Role: models.RoleAdmin})

	users := store.ListUsers()
	if len(users) != 2 {
		t.Fatalf("ListUsers() count = %d, want 2", len(users))
	}
	// Should be sorted by email
	if users[0].Email != "alice@example.com" {
		t.Error("ListUsers() should sort by email")
	}
}

func TestUpdateUser(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	created, _ := store.CreateUser(models.User{
		Email: "test@example.com",
		Name:  "Original Name",
		Role:  models.RoleUser,
	})

	updated, err := store.UpdateUser(created.ID, func(u *models.User) error {
		u.Name = "New Name"
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("UpdateUser() name = %v, want New Name", updated.Name)
	}
	if !updated.UpdatedAt.After(created.CreatedAt) {
		t.Error("UpdateUser() should update UpdatedAt")
	}
}

func TestDeleteUser(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	created, _ := store.CreateUser(models.User{
		Email: "test@example.com",
		Role:  models.RoleUser,
	})

	err := store.DeleteUser(created.ID)
	if err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	_, err = store.GetUser(created.ID)
	if err != ErrNotFound {
		t.Error("GetUser() should return ErrNotFound after delete")
	}
}

func TestDeleteUserNotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	err := store.DeleteUser("nonexistent-id")
	if err != ErrNotFound {
		t.Errorf("DeleteUser() error = %v, want ErrNotFound", err)
	}
}

// ─── Session Tests ────────────────────────────────────────────────────────────

func TestCreateSession(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	sess := models.Session{
		UserID:    "user-123",
		TokenHash: "token-hash-abc",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	created, err := store.CreateSession(sess)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if created.ID == "" {
		t.Error("CreateSession() should generate ID")
	}
	if created.IssuedAt.IsZero() {
		t.Error("CreateSession() should set IssuedAt")
	}
}

func TestFindSessionByTokenHash(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	tokenHash := "unique-token-hash"
	store.CreateSession(models.Session{
		UserID:    "user-123",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(time.Hour),
	})

	found, err := store.FindSessionByTokenHash(tokenHash)
	if err != nil {
		t.Fatalf("FindSessionByTokenHash() error = %v", err)
	}
	if found.UserID != "user-123" {
		t.Errorf("FindSessionByTokenHash() UserID = %v, want user-123", found.UserID)
	}
}

func TestFindSessionByTokenHashNotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	_, err := store.FindSessionByTokenHash("nonexistent")
	if err != ErrNotFound {
		t.Errorf("FindSessionByTokenHash() error = %v, want ErrNotFound", err)
	}
}

func TestFindSessionByTokenHashExpired(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	store.CreateSession(models.Session{
		UserID:    "user-123",
		TokenHash: "expired-token",
		ExpiresAt: time.Now().Add(-time.Hour), // expired
	})

	_, err := store.FindSessionByTokenHash("expired-token")
	if err != ErrNotFound {
		t.Error("FindSessionByTokenHash() should not return expired session")
	}
}

func TestFindSessionByTokenHashRevoked(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	created, _ := store.CreateSession(models.Session{
		UserID:    "user-123",
		TokenHash: "revoked-token",
		ExpiresAt: time.Now().Add(time.Hour),
	})

	store.RevokeSession(created.ID)

	_, err := store.FindSessionByTokenHash("revoked-token")
	if err != ErrNotFound {
		t.Error("FindSessionByTokenHash() should not return revoked session")
	}
}

func TestRevokeSession(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	created, _ := store.CreateSession(models.Session{
		UserID:    "user-123",
		TokenHash: "token",
		ExpiresAt: time.Now().Add(time.Hour),
	})

	err := store.RevokeSession(created.ID)
	if err != nil {
		t.Fatalf("RevokeSession() error = %v", err)
	}
}

func TestRevokeSessionNotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	err := store.RevokeSession("nonexistent")
	if err != ErrNotFound {
		t.Errorf("RevokeSession() error = %v, want ErrNotFound", err)
	}
}

func TestRevokeUserSessions(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	userID := "user-123"
	store.CreateSession(models.Session{
		UserID:    userID,
		TokenHash: "token-1",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	store.CreateSession(models.Session{
		UserID:    userID,
		TokenHash: "token-2",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	store.CreateSession(models.Session{
		UserID:    "other-user",
		TokenHash: "token-3",
		ExpiresAt: time.Now().Add(time.Hour),
	})

	err := store.RevokeUserSessions(userID)
	if err != nil {
		t.Fatalf("RevokeUserSessions() error = %v", err)
	}

	// User sessions should be revoked
	_, err = store.FindSessionByTokenHash("token-1")
	if err != ErrNotFound {
		t.Error("session 1 should be revoked")
	}
	_, err = store.FindSessionByTokenHash("token-2")
	if err != ErrNotFound {
		t.Error("session 2 should be revoked")
	}

	// Other user's session should remain
	_, err = store.FindSessionByTokenHash("token-3")
	if err != nil {
		t.Error("other user's session should not be revoked")
	}
}

func TestPurgeExpiredSessions(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Add old expired session
	store.CreateSession(models.Session{
		UserID:    "user-1",
		TokenHash: "old-token",
		ExpiresAt: time.Now().AddDate(0, 0, -60), // 60 days ago
	})
	// Add recent session
	store.CreateSession(models.Session{
		UserID:    "user-2",
		TokenHash: "recent-token",
		ExpiresAt: time.Now().Add(time.Hour),
	})

	err := store.PurgeExpiredSessions()
	if err != nil {
		t.Fatalf("PurgeExpiredSessions() error = %v", err)
	}

	// Old session should be purged
	store.mu.RLock()
	count := len(store.Sessions)
	store.mu.RUnlock()
	
	if count != 1 {
		t.Errorf("PurgeExpiredSessions() kept %d sessions, want 1", count)
	}
}

func TestDeleteUserRevokesSessions(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser(models.User{
		Email: "test@example.com",
		Role:  models.RoleUser,
	})
	store.CreateSession(models.Session{
		UserID:    user.ID,
		TokenHash: "user-token",
		ExpiresAt: time.Now().Add(time.Hour),
	})

	store.DeleteUser(user.ID)

	_, err := store.FindSessionByTokenHash("user-token")
	if err != ErrNotFound {
		t.Error("DeleteUser() should revoke user's sessions")
	}
}

// ─── Persistence Tests ────────────────────────────────────────────────────────

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "water.json")
	os.WriteFile(path, []byte("{}"), 0644)

	// Create store and add data
	store1, _ := OpenUserStore(path)
	store1.CreateUser(models.User{
		Email: "persistent@example.com",
		Role:  models.RoleAdmin,
	})

	// Open new store from same file
	store2, err := OpenUserStore(path)
	if err != nil {
		t.Fatalf("OpenUserStore() reload error = %v", err)
	}

	users := store2.ListUsers()
	if len(users) != 1 {
		t.Fatalf("persistence: got %d users, want 1", len(users))
	}
	if users[0].Email != "persistent@example.com" {
		t.Error("persistence: user data not preserved")
	}
}

// ─── Concurrency Tests ────────────────────────────────────────────────────────

func TestConcurrentReads(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	store.CreateUser(models.User{
		Email: "test@example.com",
		Role:  models.RoleUser,
	})

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				store.ListUsers()
				store.GetUserByEmail("test@example.com")
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestConcurrentWrites(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func(n int) {
			for j := 0; j < 10; j++ {
				store.CreateSession(models.Session{
					UserID:    "user-123",
					TokenHash: time.Now().String(),
					ExpiresAt: time.Now().Add(time.Hour),
				})
			}
			done <- true
		}(i)
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}
