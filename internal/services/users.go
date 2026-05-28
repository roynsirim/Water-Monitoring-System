package services

import (
	"errors"
	"strings"

	"water-monitoring-system/internal/auth"
	"water-monitoring-system/internal/database"
	"water-monitoring-system/internal/models"
)

// ─── User Service ─────────────────────────────────────────────────────────────

// UserService handles user management business logic.
type UserService struct {
	Users      *database.UserStore
	BcryptCost int
}

// NewUserService creates a new UserService.
func NewUserService(users *database.UserStore, bcryptCost int) *UserService {
	if bcryptCost == 0 {
		bcryptCost = 12
	}
	return &UserService{
		Users:      users,
		BcryptCost: bcryptCost,
	}
}

// CreateUserInput contains data for creating a user.
type CreateUserInput struct {
	Email    string
	Name     string
	Password string
	Role     models.Role
}

// Validate checks input fields.
func (i *CreateUserInput) Validate() error {
	i.Email = strings.ToLower(strings.TrimSpace(i.Email))
	i.Name = strings.TrimSpace(i.Name)

	if i.Email == "" {
		return errors.New("email is required")
	}
	if !strings.Contains(i.Email, "@") {
		return errors.New("invalid email format")
	}
	if i.Name == "" {
		return errors.New("name is required")
	}
	if i.Password == "" {
		return errors.New("password is required")
	}
	if err := auth.ValidatePasswordStrength(i.Password); err != nil {
		return err
	}
	if i.Role == "" {
		i.Role = models.RoleUser
	}
	if !models.IsValidRole(i.Role) {
		return errors.New("invalid role")
	}
	return nil
}

// CreateUser creates a new user.
func (s *UserService) CreateUser(input CreateUserInput, actorID, actorEmail, ip, userAgent string) (models.User, error) {
	if err := input.Validate(); err != nil {
		return models.User{}, err
	}

	hash, err := auth.HashPassword(input.Password, s.BcryptCost)
	if err != nil {
		return models.User{}, err
	}

	user := models.User{
		Email:          input.Email,
		Name:           input.Name,
		PasswordHash:   hash,
		Role:           input.Role,
		IsActive:       true,
		MustChangePass: true,
	}

	created, err := s.Users.CreateUser(user)
	if err != nil {
		return models.User{}, err
	}

	s.Users.LogActivity(models.ActivityLog{
		UserID:    actorID,
		UserEmail: actorEmail,
		Action:    "user_create",
		Resource:  created.ID,
		IP:        ip,
		UserAgent: userAgent,
		Status:    "success",
		Detail:    "created user: " + input.Email,
	})

	return created, nil
}

// UpdateUserInput contains data for updating a user.
type UpdateUserInput struct {
	Name     *string
	Role     *models.Role
	IsActive *bool
}

// UpdateUser updates user fields.
func (s *UserService) UpdateUser(userID string, input UpdateUserInput, actorID, actorEmail, ip, userAgent string) (models.User, error) {
	updated, err := s.Users.UpdateUser(userID, func(u *models.User) error {
		if input.Name != nil {
			u.Name = strings.TrimSpace(*input.Name)
		}
		if input.Role != nil {
			if !models.IsValidRole(*input.Role) {
				return errors.New("invalid role")
			}
			u.Role = *input.Role
		}
		if input.IsActive != nil {
			u.IsActive = *input.IsActive
		}
		return nil
	})
	if err != nil {
		return models.User{}, err
	}

	s.Users.LogActivity(models.ActivityLog{
		UserID:    actorID,
		UserEmail: actorEmail,
		Action:    "user_update",
		Resource:  userID,
		IP:        ip,
		UserAgent: userAgent,
		Status:    "success",
	})

	return updated, nil
}

// DeleteUser removes a user.
func (s *UserService) DeleteUser(userID, actorID, actorEmail, ip, userAgent string) error {
	user, err := s.Users.GetUser(userID)
	if err != nil {
		return err
	}

	if err := s.Users.DeleteUser(userID); err != nil {
		return err
	}

	s.Users.LogActivity(models.ActivityLog{
		UserID:    actorID,
		UserEmail: actorEmail,
		Action:    "user_delete",
		Resource:  userID,
		IP:        ip,
		UserAgent: userAgent,
		Status:    "success",
		Detail:    "deleted user: " + user.Email,
	})

	return nil
}

// ResetPasswordResult contains the temporary password.
type ResetPasswordResult struct {
	TemporaryPassword string
}

// ResetPassword generates a new temporary password.
func (s *UserService) ResetPassword(userID, actorID, actorEmail, ip, userAgent string) (*ResetPasswordResult, error) {
	user, err := s.Users.GetUser(userID)
	if err != nil {
		return nil, err
	}

	// Generate temporary password
	tmpPw, err := generateTemporaryPassword()
	if err != nil {
		return nil, err
	}

	hash, err := auth.HashPassword(tmpPw, s.BcryptCost)
	if err != nil {
		return nil, err
	}

	_, err = s.Users.UpdateUser(userID, func(u *models.User) error {
		u.PasswordHash = hash
		u.MustChangePass = true
		u.FailedAttempts = 0
		u.LockedUntil = nil
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Revoke all sessions
	s.Users.RevokeUserSessions(userID)

	s.Users.LogActivity(models.ActivityLog{
		UserID:    actorID,
		UserEmail: actorEmail,
		Action:    "password_reset",
		Resource:  userID,
		IP:        ip,
		UserAgent: userAgent,
		Status:    "success",
		Detail:    "reset password for: " + user.Email,
	})

	return &ResetPasswordResult{TemporaryPassword: tmpPw}, nil
}

// generateTemporaryPassword creates a secure random password.
func generateTemporaryPassword() (string, error) {
	tok, err := auth.RandomToken(12)
	if err != nil {
		return "", err
	}
	// Add prefix to ensure requirements met
	return "Tmp!" + tok, nil
}

// ListUsersInput contains pagination/filter parameters.
type ListUsersInput struct {
	Page     int
	PageSize int
	Role     models.Role // filter by role
	Search   string      // search email/name
}

// ListUsersResult contains paginated user list.
type ListUsersResult struct {
	Users      []models.User
	TotalCount int
	Page       int
	PageSize   int
	TotalPages int
}

// ListUsers returns paginated users.
func (s *UserService) ListUsers(input ListUsersInput) ListUsersResult {
	if input.Page < 1 {
		input.Page = 1
	}
	if input.PageSize < 1 || input.PageSize > 100 {
		input.PageSize = 20
	}

	allUsers := s.Users.ListUsers()

	// Filter
	var filtered []models.User
	search := strings.ToLower(strings.TrimSpace(input.Search))
	for _, u := range allUsers {
		if input.Role != "" && u.Role != input.Role {
			continue
		}
		if search != "" {
			if !strings.Contains(strings.ToLower(u.Email), search) &&
				!strings.Contains(strings.ToLower(u.Name), search) {
				continue
			}
		}
		filtered = append(filtered, u)
	}

	total := len(filtered)
	totalPages := (total + input.PageSize - 1) / input.PageSize
	if totalPages < 1 {
		totalPages = 1
	}

	// Paginate
	start := (input.Page - 1) * input.PageSize
	end := start + input.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return ListUsersResult{
		Users:      filtered[start:end],
		TotalCount: total,
		Page:       input.Page,
		PageSize:   input.PageSize,
		TotalPages: totalPages,
	}
}

// ListActivityInput contains activity log query parameters.
type ListActivityInput struct {
	UserID string
	Limit  int
}

// ListActivity returns filtered activity logs.
func (s *UserService) ListActivity(input ListActivityInput) []models.ActivityLog {
	return s.Users.ListActivity(input.UserID, input.Limit)
}
