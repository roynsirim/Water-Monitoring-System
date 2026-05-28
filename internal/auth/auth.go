// Package auth provides password hashing, session token signing/verification,
// and helpers for password validation.
//
// Tokens are stateless HMAC-SHA256 signed JWS-style tokens with a compact
// payload: "<base64url(payload_json)>.<base64url(hmac)>". Server-side, an
// accompanying Session row tracks revocation; a SHA-256 hash of the token is
// stored, never the token itself.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"strings"
	"time"
	"unicode"
)

// ─── Password hashing (bcrypt via stdlib-only fallback) ───────────────────────
//
// To keep this build dep-free we implement a minimal PBKDF2-SHA256 password
// hash with per-password salt. The cost parameter is the iteration count
// (logarithmic-ish, scale 10–14 → 30k–500k iterations).

const (
	pbkdfSaltLen = 16
	pbkdfKeyLen  = 32
)

// HashPassword returns a PHC-style string: $pbkdf2-sha256$i=<iter>$<salt_b64>$<hash_b64>
func HashPassword(password string, cost int) (string, error) {
	if cost < 10 {
		cost = 10
	}
	if cost > 16 {
		cost = 16
	}
	iter := costToIter(cost)

	salt := make([]byte, pbkdfSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	dk := pbkdf2(sha256.New, []byte(password), salt, iter, pbkdfKeyLen)
	return fmt.Sprintf("$pbkdf2-sha256$i=%d$%s$%s",
		iter,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(dk),
	), nil
}

// VerifyPassword returns nil if the password matches the hash.
func VerifyPassword(hash, password string) error {
	parts := strings.Split(hash, "$")
	// "" "pbkdf2-sha256" "i=<n>" "<salt>" "<dk>"
	if len(parts) != 5 || parts[1] != "pbkdf2-sha256" {
		return errors.New("unsupported password hash format")
	}
	var iter int
	if _, err := fmt.Sscanf(parts[2], "i=%d", &iter); err != nil || iter < 1000 {
		return errors.New("invalid iteration count")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return errors.New("invalid salt")
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return errors.New("invalid hash")
	}
	got := pbkdf2(sha256.New, []byte(password), salt, iter, len(want))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return errors.New("invalid password")
	}
	return nil
}

func costToIter(cost int) int {
	// cost 10 -> ~30k, cost 12 -> ~120k, cost 14 -> ~480k
	return 30000 * (1 << (cost - 10))
}

// ValidatePasswordStrength enforces minimum complexity rules.
func ValidatePasswordStrength(pw string) error {
	if len(pw) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	var hasUpper, hasLower, hasDigit bool
	for _, r := range pw {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !(hasUpper && hasLower && hasDigit) {
		return errors.New("password must contain upper, lower, and digit characters")
	}
	return nil
}

// ─── Random tokens ────────────────────────────────────────────────────────────

// RandomToken returns a URL-safe random string of n bytes (base64url, no pad).
func RandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken returns a hex SHA-256 hash of the input — used to store
// non-reversible references to issued tokens.
func HashToken(tok string) string {
	h := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(h[:])
}

// ─── Session tokens (HMAC-signed payload) ─────────────────────────────────────

// Claims is the payload carried in a session token.
type Claims struct {
	SessionID string `json:"sid"`
	UserID    string `json:"uid"`
	Role      string `json:"rol"`
	Email     string `json:"em"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// Sign returns "payload.sig". Both parts are base64url-encoded.
func Sign(c Claims, secret string) (string, error) {
	if secret == "" {
		return "", errors.New("auth: JWT secret not configured")
	}
	body, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig, nil
}

// Verify checks the signature, expiry, and returns the claims.
func Verify(token, secret string) (*Claims, error) {
	if secret == "" {
		return nil, errors.New("auth: JWT secret not configured")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid token format")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0]))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(want), []byte(parts[1])) != 1 {
		return nil, errors.New("invalid token signature")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("invalid token payload")
	}
	var c Claims
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, errors.New("invalid token claims")
	}
	if time.Now().Unix() > c.ExpiresAt {
		return nil, errors.New("token expired")
	}
	return &c, nil
}

// ─── PBKDF2 (RFC 2898) ────────────────────────────────────────────────────────
//
// Inlined here to avoid pulling x/crypto. Equivalent to
// golang.org/x/crypto/pbkdf2 with HMAC-SHA256.

func pbkdf2(h func() hash.Hash, password, salt []byte, iter, keyLen int) []byte {
	prf := hmac.New(h, password)
	hashLen := prf.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen

	var buf [4]byte
	dk := make([]byte, 0, numBlocks*hashLen)
	U := make([]byte, hashLen)
	for block := 1; block <= numBlocks; block++ {
		prf.Reset()
		prf.Write(salt)
		buf[0] = byte(block >> 24)
		buf[1] = byte(block >> 16)
		buf[2] = byte(block >> 8)
		buf[3] = byte(block)
		prf.Write(buf[:])
		T := prf.Sum(nil)
		copy(U, T)
		for n := 2; n <= iter; n++ {
			prf.Reset()
			prf.Write(U)
			U = prf.Sum(U[:0])
			for x := range T {
				T[x] ^= U[x]
			}
		}
		dk = append(dk, T...)
	}
	return dk[:keyLen]
}
