package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"water-monitoring-system/internal/auth"
	"water-monitoring-system/internal/database"
	"water-monitoring-system/internal/models"
)

// AdminHandler exposes user/account administration endpoints.
type AdminHandler struct {
	DB         database.Store
	Users      database.UserStoreInterface
	BcryptCost int
}

// ─── Users CRUD ──────────────────────────────────────────────────────────────

// HandleUsers serves /api/admin/users for GET (list) and POST (create).
func (h *AdminHandler) HandleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, 200, h.Users.ListUsers())
		return
	case http.MethodPost:
		h.createUser(w, r)
		return
	}
	writeError(w, 405, "method not allowed")
}

func (h *AdminHandler) createUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email          string `json:"email"`
		Name           string `json:"name"`
		Role           string `json:"role"`
		Password       string `json:"password"`
		IsActive       *bool  `json:"is_active"`
		MustChangePass *bool  `json:"must_change_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	body.Email = strings.ToLower(strings.TrimSpace(body.Email))
	if body.Email == "" || body.Name == "" || body.Password == "" {
		writeError(w, 400, "email, name and password are required")
		return
	}
	role := models.Role(body.Role)
	if role == "" {
		role = models.RoleUser
	}
	if !models.IsValidRole(role) {
		writeError(w, 400, "invalid role")
		return
	}
	if err := auth.ValidatePasswordStrength(body.Password); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	hash, err := auth.HashPassword(body.Password, h.BcryptCost)
	if err != nil {
		writeError(w, 500, "could not hash password")
		return
	}
	u := models.User{
		Email:          body.Email,
		Name:           body.Name,
		Role:           role,
		IsActive:       true,
		PasswordHash:   hash,
		MustChangePass: true,
	}
	if body.IsActive != nil {
		u.IsActive = *body.IsActive
	}
	if body.MustChangePass != nil {
		u.MustChangePass = *body.MustChangePass
	}
	created, err := h.Users.CreateUser(u)
	if err == database.ErrEmailExists {
		writeError(w, 409, "email already exists")
		return
	}
	if err != nil {
		writeError(w, 500, "could not create user")
		return
	}
	actor, _ := CurrentUser(r)
	h.Users.LogActivity(models.ActivityLog{
		UserID: actor.ID, UserEmail: actor.Email,
		Action: "create_user", Resource: created.ID, Status: "success",
		IP: clientIP(r), UserAgent: r.UserAgent(),
		Detail: "created " + created.Email,
	})
	respondJSON(w, 201, created)
}

// HandleUser serves /api/admin/users/{id} for GET / PUT / DELETE.
func (h *AdminHandler) HandleUser(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	id = strings.Trim(id, "/")
	if id == "" {
		writeError(w, 400, "user id required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		u, err := h.Users.GetUser(id)
		if err != nil {
			writeError(w, 404, "user not found")
			return
		}
		respondJSON(w, 200, u.SafeUser())
	case http.MethodPut, http.MethodPatch:
		h.updateUser(w, r, id)
	case http.MethodDelete:
		h.deleteUser(w, r, id)
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (h *AdminHandler) updateUser(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Email          *string `json:"email"`
		Name           *string `json:"name"`
		Role           *string `json:"role"`
		IsActive       *bool   `json:"is_active"`
		MustChangePass *bool   `json:"must_change_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	u, err := h.Users.UpdateUser(id, func(u *models.User) error {
		if body.Email != nil {
			u.Email = strings.ToLower(strings.TrimSpace(*body.Email))
		}
		if body.Name != nil {
			u.Name = *body.Name
		}
		if body.Role != nil {
			rr := models.Role(*body.Role)
			if !models.IsValidRole(rr) {
				return database.ErrNotFound // reuse to short-circuit
			}
			u.Role = rr
		}
		if body.IsActive != nil {
			u.IsActive = *body.IsActive
		}
		if body.MustChangePass != nil {
			u.MustChangePass = *body.MustChangePass
		}
		return nil
	})
	if err == database.ErrNotFound {
		writeError(w, 404, "user not found or invalid role")
		return
	}
	if err != nil {
		writeError(w, 500, "could not update user")
		return
	}
	actor, _ := CurrentUser(r)
	h.Users.LogActivity(models.ActivityLog{
		UserID: actor.ID, UserEmail: actor.Email,
		Action: "update_user", Resource: id, Status: "success",
		IP: clientIP(r), UserAgent: r.UserAgent(),
	})
	respondJSON(w, 200, u)
}

func (h *AdminHandler) deleteUser(w http.ResponseWriter, r *http.Request, id string) {
	actor, _ := CurrentUser(r)
	if actor.ID == id {
		writeError(w, 400, "you cannot delete your own account")
		return
	}
	if err := h.Users.DeleteUser(id); err != nil {
		writeError(w, 404, "user not found")
		return
	}
	h.Users.LogActivity(models.ActivityLog{
		UserID: actor.ID, UserEmail: actor.Email,
		Action: "delete_user", Resource: id, Status: "success",
		IP: clientIP(r), UserAgent: r.UserAgent(),
	})
	respondJSON(w, 200, map[string]string{"status": "deleted"})
}

// HandleResetUserPassword sets a new password for the target user (admin
// only). Optionally generates a random temporary password if "password" is
// omitted; that temp password is returned in the response and must be
// communicated to the user out-of-band.
func (h *AdminHandler) HandleResetUserPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method not allowed")
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/admin/users/"), "/reset-password")
	id = strings.Trim(id, "/")
	if id == "" {
		writeError(w, 400, "user id required")
		return
	}
	var body struct {
		NewPassword string `json:"new_password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	temp := ""
	if body.NewPassword == "" {
		t, err := auth.RandomToken(9) // ~12 chars base64url
		if err != nil {
			writeError(w, 500, "could not generate password")
			return
		}
		body.NewPassword = "Tmp!" + t
		temp = body.NewPassword
	}
	if err := auth.ValidatePasswordStrength(body.NewPassword); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	hash, err := auth.HashPassword(body.NewPassword, h.BcryptCost)
	if err != nil {
		writeError(w, 500, "could not hash password")
		return
	}
	_, err = h.Users.UpdateUser(id, func(u *models.User) error {
		u.PasswordHash = hash
		u.MustChangePass = true
		u.FailedAttempts = 0
		u.LockedUntil = nil
		return nil
	})
	if err != nil {
		writeError(w, 404, "user not found")
		return
	}
	_ = h.Users.RevokeUserSessions(id)
	actor, _ := CurrentUser(r)
	h.Users.LogActivity(models.ActivityLog{
		UserID: actor.ID, UserEmail: actor.Email,
		Action: "admin_reset_password", Resource: id, Status: "success",
		IP: clientIP(r), UserAgent: r.UserAgent(),
	})
	resp := map[string]string{"status": "password reset"}
	if temp != "" {
		resp["temporary_password"] = temp
	}
	respondJSON(w, 200, resp)
}

// ─── Activity log ────────────────────────────────────────────────────────────

func (h *AdminHandler) HandleActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method not allowed")
		return
	}
	userID := r.URL.Query().Get("user_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	respondJSON(w, 200, h.Users.ListActivity(userID, limit))
}

// ─── Median fill ─────────────────────────────────────────────────────────────

// HandleMedianFill — admin-only endpoint to fill missing data using the
// median of historical readings. Body: { meter_id?, site_id?, target_date?,
// lookback_days?, freshness_days? }
// If meter_id is given, fills that single meter; otherwise fills every active
// stale meter under the optional site_id filter.
func (h *AdminHandler) HandleMedianFill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method not allowed")
		return
	}
	var body struct {
		MeterID       string `json:"meter_id"`
		SiteID        string `json:"site_id"`
		TargetDate    string `json:"target_date"`
		LookbackDays  int    `json:"lookback_days"`
		FreshnessDays int    `json:"freshness_days"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	target := time.Now()
	if body.TargetDate != "" {
		target = parseDate(body.TargetDate)
	}

	actor, _ := CurrentUser(r)

	if body.MeterID != "" {
		est := h.DB.MedianFillMissingData(body.MeterID, target, body.LookbackDays)
		if est == nil {
			writeError(w, 404, "no historical data to compute median")
			return
		}
		if err := h.DB.AddReading(*est); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		h.Users.LogActivity(models.ActivityLog{
			UserID: actor.ID, UserEmail: actor.Email,
			Action: "median_fill", Resource: body.MeterID, Status: "success",
			IP: clientIP(r), UserAgent: r.UserAgent(),
			Detail: "single meter fill",
		})
		respondJSON(w, 201, map[string]any{"status": "filled", "reading": est})
		return
	}

	n, err := h.DB.MedianFillAll(body.SiteID, target, body.FreshnessDays, body.LookbackDays)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	h.Users.LogActivity(models.ActivityLog{
		UserID: actor.ID, UserEmail: actor.Email,
		Action: "median_fill_all", Resource: body.SiteID, Status: "success",
		IP: clientIP(r), UserAgent: r.UserAgent(),
		Detail: "bulk median fill",
	})
	respondJSON(w, 200, map[string]any{"status": "completed", "filled": n})
}

// ─── Data Management ─────────────────────────────────────────────────────────

// HandleReadingsCRUD handles /api/admin/readings/{id} for GET / PUT / DELETE
func (h *AdminHandler) HandleReadingsCRUD(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/admin/readings/")
	id = strings.Trim(id, "/")

	switch r.Method {
	case http.MethodGet:
		if id == "" {
			// List recent readings with pagination
			readings := h.DB.GetReadings("", "", time.Time{}, time.Time{})
			respondJSON(w, 200, readings)
			return
		}
		rd := h.DB.GetReading(id)
		if rd == nil {
			writeError(w, 404, "reading not found")
			return
		}
		respondJSON(w, 200, rd)
	case http.MethodPut, http.MethodPatch:
		if id == "" {
			writeError(w, 400, "reading id required")
			return
		}
		h.updateReading(w, r, id)
	case http.MethodDelete:
		if id == "" {
			writeError(w, 400, "reading id required")
			return
		}
		h.deleteReading(w, r, id)
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (h *AdminHandler) updateReading(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Value float64 `json:"value"`
		Usage float64 `json:"usage"`
		Date  string  `json:"date"`
		Notes string  `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}

	// Validate: value should not be negative
	if body.Value < 0 {
		writeError(w, 400, "value must not be negative")
		return
	}

	// Check if reading exists first
	existing := h.DB.GetReading(id)
	if existing == nil {
		writeError(w, 404, "reading not found")
		return
	}

	updates := models.Reading{
		Value: body.Value,
		Usage: body.Usage,
		Notes: body.Notes,
	}
	if body.Date != "" {
		updates.Date = parseDate(body.Date)
	}

	if err := h.DB.UpdateReading(id, updates); err != nil {
		writeError(w, 500, err.Error())
		return
	}

	actor, _ := CurrentUser(r)
	h.Users.LogActivity(models.ActivityLog{
		UserID: actor.ID, UserEmail: actor.Email,
		Action: "update_reading", Resource: id, Status: "success",
		IP: clientIP(r), UserAgent: r.UserAgent(),
	})
	respondJSON(w, 200, map[string]string{"status": "updated"})
}

func (h *AdminHandler) deleteReading(w http.ResponseWriter, r *http.Request, id string) {
	// Check if reading exists first
	existing := h.DB.GetReading(id)
	if existing == nil {
		writeError(w, 404, "reading not found")
		return
	}

	if err := h.DB.DeleteReading(id); err != nil {
		writeError(w, 500, err.Error())
		return
	}

	actor, _ := CurrentUser(r)
	h.Users.LogActivity(models.ActivityLog{
		UserID: actor.ID, UserEmail: actor.Email,
		Action: "delete_reading", Resource: id, Status: "success",
		IP: clientIP(r), UserAgent: r.UserAgent(),
	})
	respondJSON(w, 200, map[string]string{"status": "deleted"})
}

// HandleTonnesCRUD handles /api/admin/tonnes/{id} for GET / PUT / DELETE
func (h *AdminHandler) HandleTonnesCRUD(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/admin/tonnes/")
	id = strings.Trim(id, "/")

	switch r.Method {
	case http.MethodGet:
		if id == "" {
			// List recent tonnes entries
			tonnes := h.DB.GetTonnes("", time.Time{}, time.Time{})
			respondJSON(w, 200, tonnes)
			return
		}
		t := h.DB.GetTonnesEntry(id)
		if t == nil {
			writeError(w, 404, "tonnes entry not found")
			return
		}
		respondJSON(w, 200, t)
	case http.MethodPut, http.MethodPatch:
		if id == "" {
			writeError(w, 400, "tonnes entry id required")
			return
		}
		h.updateTonnes(w, r, id)
	case http.MethodDelete:
		if id == "" {
			writeError(w, 400, "tonnes entry id required")
			return
		}
		h.deleteTonnes(w, r, id)
	default:
		writeError(w, 405, "method not allowed")
	}
}

func (h *AdminHandler) updateTonnes(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		SiteID     string  `json:"site_id"`
		Department string  `json:"department"`
		Tonnes     float64 `json:"tonnes"`
		Date       string  `json:"date"`
		Notes      string  `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}

	// Validate: tonnes should not be negative
	if body.Tonnes < 0 {
		writeError(w, 400, "tonnes must not be negative")
		return
	}

	// Check if entry exists first
	existing := h.DB.GetTonnesEntry(id)
	if existing == nil {
		writeError(w, 404, "tonnes entry not found")
		return
	}

	updates := models.TonnesEntry{
		SiteID:     body.SiteID,
		Department: body.Department,
		Tonnes:     body.Tonnes,
		Notes:      body.Notes,
	}
	if body.Date != "" {
		updates.Date = parseDate(body.Date)
	}

	if err := h.DB.UpdateTonnes(id, updates); err != nil {
		writeError(w, 500, err.Error())
		return
	}

	actor, _ := CurrentUser(r)
	h.Users.LogActivity(models.ActivityLog{
		UserID: actor.ID, UserEmail: actor.Email,
		Action: "update_tonnes", Resource: id, Status: "success",
		IP: clientIP(r), UserAgent: r.UserAgent(),
	})
	respondJSON(w, 200, map[string]string{"status": "updated"})
}

func (h *AdminHandler) deleteTonnes(w http.ResponseWriter, r *http.Request, id string) {
	// Check if entry exists first
	existing := h.DB.GetTonnesEntry(id)
	if existing == nil {
		writeError(w, 404, "tonnes entry not found")
		return
	}

	if err := h.DB.DeleteTonnes(id); err != nil {
		writeError(w, 500, err.Error())
		return
	}

	actor, _ := CurrentUser(r)
	h.Users.LogActivity(models.ActivityLog{
		UserID: actor.ID, UserEmail: actor.Email,
		Action: "delete_tonnes", Resource: id, Status: "success",
		IP: clientIP(r), UserAgent: r.UserAgent(),
	})
	respondJSON(w, 200, map[string]string{"status": "deleted"})
}
