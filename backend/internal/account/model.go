package account

import (
	"errors"
	"strings"
	"time"
)

// Profile represents safe user profile data for reading and updating.
type Profile struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     *string   `json:"phone"` // Nullable
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpdateInput is the request payload for updating user profile.
// Email is read-only in V2 (requires verification), role is server-controlled.
type UpdateInput struct {
	Name  string `json:"name"`
	Phone string `json:"phone"` // Can be empty string to clear phone
}

// Field length limits
const (
	maxNameLen  = 255
	maxPhoneLen = 20
)

var (
	ErrProfileNotFound = errors.New("profile not found")
)

// ErrValidation is returned by Validate methods when a field fails validation.
type ErrValidation struct {
	Field   string
	Message string
}

func (e *ErrValidation) Error() string {
	return e.Field + ": " + e.Message
}

// Validate checks that required fields are present and within length limits.
func (in UpdateInput) Validate() error {
	trimmed := in.Trimmed()

	if trimmed.Name == "" {
		return &ErrValidation{Field: "name", Message: "is required"}
	}
	if len(trimmed.Name) > maxNameLen {
		return &ErrValidation{Field: "name", Message: "is too long"}
	}

	if trimmed.Phone != "" && len(trimmed.Phone) > maxPhoneLen {
		return &ErrValidation{Field: "phone", Message: "is too long"}
	}

	return nil
}

// Trimmed returns a copy of in with string fields trimmed.
func (in UpdateInput) Trimmed() UpdateInput {
	return UpdateInput{
		Name:  strings.TrimSpace(in.Name),
		Phone: strings.TrimSpace(in.Phone),
	}
}