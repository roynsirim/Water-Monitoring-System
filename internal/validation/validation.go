// Package validation provides input validation for API requests.
package validation

import (
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// ─── Validation Errors ────────────────────────────────────────────────────────

// ValidationError represents a field-level validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return ""
	}
	if len(ve) == 1 {
		return ve[0].Error()
	}
	var b strings.Builder
	for i, e := range ve {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(e.Error())
	}
	return b.String()
}

// HasErrors returns true if there are any errors.
func (ve ValidationErrors) HasErrors() bool {
	return len(ve) > 0
}

// Add appends a validation error.
func (ve *ValidationErrors) Add(field, message string) {
	*ve = append(*ve, ValidationError{Field: field, Message: message})
}

// ToError returns nil if no errors, otherwise returns self.
func (ve ValidationErrors) ToError() error {
	if len(ve) == 0 {
		return nil
	}
	return ve
}

// ─── Validator ────────────────────────────────────────────────────────────────

// Validator accumulates validation errors.
type Validator struct {
	Errors ValidationErrors
}

// New creates a new Validator.
func New() *Validator {
	return &Validator{}
}

// Check adds an error if condition is false.
func (v *Validator) Check(ok bool, field, message string) {
	if !ok {
		v.Errors.Add(field, message)
	}
}

// Valid returns true if no errors.
func (v *Validator) Valid() bool {
	return !v.Errors.HasErrors()
}

// Err returns the validation errors or nil.
func (v *Validator) Err() error {
	return v.Errors.ToError()
}

// ─── Common Validators ────────────────────────────────────────────────────────

// Required checks that a string is not empty.
func Required(v *Validator, value, field string) {
	v.Check(strings.TrimSpace(value) != "", field, "is required")
}

// RequiredPtr checks that a pointer is not nil.
func RequiredPtr[T any](v *Validator, ptr *T, field string) {
	v.Check(ptr != nil, field, "is required")
}

// MaxLength checks string doesn't exceed max length.
func MaxLength(v *Validator, value, field string, max int) {
	v.Check(utf8.RuneCountInString(value) <= max, field, fmt.Sprintf("must not exceed %d characters", max))
}

// MinLength checks string meets minimum length.
func MinLength(v *Validator, value, field string, min int) {
	v.Check(utf8.RuneCountInString(value) >= min, field, fmt.Sprintf("must be at least %d characters", min))
}

// Email validates email format.
func Email(v *Validator, value, field string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return // use Required for empty check
	}
	_, err := mail.ParseAddress(value)
	v.Check(err == nil, field, "must be a valid email address")
}

// InRange checks that a number is within bounds.
func InRange[T ~int | ~int64 | ~float64](v *Validator, value T, field string, min, max T) {
	v.Check(value >= min && value <= max, field, fmt.Sprintf("must be between %v and %v", min, max))
}

// PositiveInt checks that an integer is positive.
func PositiveInt(v *Validator, value int, field string) {
	v.Check(value > 0, field, "must be positive")
}

// NonNegativeInt checks that an integer is zero or positive.
func NonNegativeInt(v *Validator, value int, field string) {
	v.Check(value >= 0, field, "must be zero or positive")
}

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// UUID validates UUID format.
func UUID(v *Validator, value, field string) {
	if value == "" {
		return
	}
	v.Check(uuidRegex.MatchString(value), field, "must be a valid UUID")
}

// OneOf checks that value is one of the allowed values.
func OneOf[T comparable](v *Validator, value T, field string, allowed ...T) {
	for _, a := range allowed {
		if value == a {
			return
		}
	}
	v.Check(false, field, fmt.Sprintf("must be one of: %v", allowed))
}

// DateFormat validates date string format.
func DateFormat(v *Validator, value, field, layout string) {
	if value == "" {
		return
	}
	_, err := time.Parse(layout, value)
	v.Check(err == nil, field, fmt.Sprintf("must be in format %s", layout))
}

// URL validates URL format (basic check).
func URL(v *Validator, value, field string) {
	if value == "" {
		return
	}
	v.Check(strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://"),
		field, "must be a valid URL")
}

// ─── Convenience Functions ────────────────────────────────────────────────────

// ValidateEmail is a convenience function that returns an error if email is invalid.
func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return errors.New("email is required")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return errors.New("invalid email format")
	}
	return nil
}

// ValidatePassword checks password meets requirements.
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	var hasUpper, hasLower, hasDigit bool
	for _, r := range password {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return errors.New("password must contain uppercase, lowercase, and digit")
	}
	return nil
}

// ValidateUUID checks if string is a valid UUID.
func ValidateUUID(id string) error {
	if !uuidRegex.MatchString(id) {
		return errors.New("invalid UUID format")
	}
	return nil
}

// ─── Pagination Helpers ───────────────────────────────────────────────────────

// PaginationDefaults contains default pagination values.
type PaginationDefaults struct {
	DefaultPage     int
	DefaultPageSize int
	MaxPageSize     int
}

// DefaultPagination returns standard pagination defaults.
func DefaultPagination() PaginationDefaults {
	return PaginationDefaults{
		DefaultPage:     1,
		DefaultPageSize: 20,
		MaxPageSize:     100,
	}
}

// ValidatePagination normalizes and validates pagination parameters.
func ValidatePagination(page, pageSize int, defaults PaginationDefaults) (int, int) {
	if page < 1 {
		page = defaults.DefaultPage
	}
	if pageSize < 1 {
		pageSize = defaults.DefaultPageSize
	}
	if pageSize > defaults.MaxPageSize {
		pageSize = defaults.MaxPageSize
	}
	return page, pageSize
}
