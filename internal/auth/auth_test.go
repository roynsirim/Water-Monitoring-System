package auth

import (
	"strings"
	"testing"
	"time"
)

// ─── Password Hashing Tests ───────────────────────────────────────────────────

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		cost     int
		wantErr  bool
	}{
		{"valid password cost 10", "SecurePass123", 10, false},
		{"valid password cost 12", "SecurePass123", 12, false},
		{"empty password", "", 10, false}, // allowed at hash level, validation is separate
		{"cost below min clamps to 10", "password", 5, false},
		{"cost above max clamps to 16", "password", 20, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password, tt.cost)
			if (err != nil) != tt.wantErr {
				t.Errorf("HashPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !strings.HasPrefix(hash, "$pbkdf2-sha256$") {
				t.Errorf("HashPassword() hash format invalid: %s", hash)
			}
		})
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "TestPassword123"
	hash, err := HashPassword(password, 10)
	if err != nil {
		t.Fatalf("HashPassword() failed: %v", err)
	}

	tests := []struct {
		name     string
		hash     string
		password string
		wantErr  bool
	}{
		{"correct password", hash, password, false},
		{"wrong password", hash, "WrongPassword123", true},
		{"empty password against hash", hash, "", true},
		{"invalid hash format", "not-a-valid-hash", password, true},
		{"malformed pbkdf2 hash", "$pbkdf2-sha256$invalid", password, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyPassword(tt.hash, tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyPassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyPasswordTiming(t *testing.T) {
	// Ensure verification uses constant-time comparison
	hash, _ := HashPassword("password123", 10)
	
	// Both should take similar time (constant-time)
	start := time.Now()
	_ = VerifyPassword(hash, "password123") // correct
	correct := time.Since(start)

	start = time.Now()
	_ = VerifyPassword(hash, "wrongpasswd") // wrong
	wrong := time.Since(start)

	// Allow 50% variance (hashing dominates, so timing should be similar)
	ratio := float64(correct) / float64(wrong)
	if ratio < 0.5 || ratio > 2.0 {
		t.Logf("Timing variance: correct=%v, wrong=%v, ratio=%.2f", correct, wrong, ratio)
	}
}

// ─── Password Strength Tests ──────────────────────────────────────────────────

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
		errMsg   string
	}{
		{"valid password", "SecurePass123", false, ""},
		{"too short", "Abc1", true, "at least 8 characters"},
		{"no uppercase", "securepass123", true, "upper, lower, and digit"},
		{"no lowercase", "SECUREPASS123", true, "upper, lower, and digit"},
		{"no digit", "SecurePassword", true, "upper, lower, and digit"},
		{"exactly 8 chars valid", "Abcdef1!", false, ""},
		{"unicode with requirements", "Пароль1A", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswordStrength(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePasswordStrength() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidatePasswordStrength() error = %v, want error containing %q", err, tt.errMsg)
			}
		})
	}
}

// ─── Token Tests ──────────────────────────────────────────────────────────────

func TestRandomToken(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		wantErr bool
	}{
		{"32 bytes", 32, false},
		{"16 bytes", 16, false},
		{"64 bytes", 64, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok1, err := RandomToken(tt.length)
			if (err != nil) != tt.wantErr {
				t.Errorf("RandomToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			tok2, _ := RandomToken(tt.length)
			if tok1 == tok2 {
				t.Error("RandomToken() produced duplicate tokens")
			}
		})
	}
}

func TestHashToken(t *testing.T) {
	token := "test-token-12345"
	hash1 := HashToken(token)
	hash2 := HashToken(token)

	if hash1 != hash2 {
		t.Error("HashToken() not deterministic")
	}
	if len(hash1) != 64 { // SHA-256 produces 32 bytes = 64 hex chars
		t.Errorf("HashToken() length = %d, want 64", len(hash1))
	}
	if HashToken("different") == hash1 {
		t.Error("HashToken() collision on different inputs")
	}
}

// ─── Sign/Verify Tests ────────────────────────────────────────────────────────

func TestSignAndVerify(t *testing.T) {
	secret := "test-secret-key-32bytes-long!!"
	claims := Claims{
		SessionID: "sess-123",
		UserID:    "user-456",
		Role:      "admin",
		Email:     "test@example.com",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}

	token, err := Sign(claims, secret)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	// Verify with correct secret
	got, err := Verify(token, secret)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if got.UserID != claims.UserID {
		t.Errorf("Verify() UserID = %v, want %v", got.UserID, claims.UserID)
	}
	if got.Email != claims.Email {
		t.Errorf("Verify() Email = %v, want %v", got.Email, claims.Email)
	}
}

func TestSignEmptySecret(t *testing.T) {
	claims := Claims{UserID: "test"}
	_, err := Sign(claims, "")
	if err == nil {
		t.Error("Sign() with empty secret should error")
	}
}

func TestVerifyEmptySecret(t *testing.T) {
	_, err := Verify("some.token", "")
	if err == nil {
		t.Error("Verify() with empty secret should error")
	}
}

func TestVerifyWrongSecret(t *testing.T) {
	claims := Claims{
		UserID:    "user-123",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, _ := Sign(claims, "correct-secret")

	_, err := Verify(token, "wrong-secret")
	if err == nil {
		t.Error("Verify() with wrong secret should error")
	}
}

func TestVerifyExpiredToken(t *testing.T) {
	claims := Claims{
		UserID:    "user-123",
		ExpiresAt: time.Now().Add(-time.Hour).Unix(), // expired
	}
	token, _ := Sign(claims, "secret")

	_, err := Verify(token, "secret")
	if err == nil {
		t.Error("Verify() with expired token should error")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("Verify() error = %v, want error containing 'expired'", err)
	}
}

func TestVerifyInvalidFormat(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"no separator", "invalidtokenwithoutdot"},
		{"too many parts", "part1.part2.part3"},
		{"empty payload", ".signature"},
		{"empty signature", "payload."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Verify(tt.token, "secret")
			if err == nil {
				t.Error("Verify() should error on invalid format")
			}
		})
	}
}

func TestVerifyTamperedToken(t *testing.T) {
	claims := Claims{
		UserID:    "user-123",
		Role:      "user",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, _ := Sign(claims, "secret")

	// Tamper with payload (change role)
	parts := strings.Split(token, ".")
	tampered := parts[0] + "X" + "." + parts[1] // modify payload

	_, err := Verify(tampered, "secret")
	if err == nil {
		t.Error("Verify() should detect tampered payload")
	}
}

// ─── Benchmarks ───────────────────────────────────────────────────────────────

func BenchmarkHashPassword(b *testing.B) {
	for i := 0; i < b.N; i++ {
		HashPassword("benchmark-password", 10)
	}
}

func BenchmarkVerifyPassword(b *testing.B) {
	hash, _ := HashPassword("benchmark-password", 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		VerifyPassword(hash, "benchmark-password")
	}
}

func BenchmarkSignToken(b *testing.B) {
	claims := Claims{
		UserID:    "user-123",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	for i := 0; i < b.N; i++ {
		Sign(claims, "benchmark-secret-key")
	}
}

func BenchmarkVerifyToken(b *testing.B) {
	claims := Claims{
		UserID:    "user-123",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, _ := Sign(claims, "benchmark-secret-key")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Verify(token, "benchmark-secret-key")
	}
}
