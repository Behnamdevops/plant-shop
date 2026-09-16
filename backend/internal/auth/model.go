package auth

import (
	"errors"
	"time"
)

type User struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// IsAdmin reports whether the user has the admin role. Role is always
// determined from the database row, never from client-supplied input.
func (u User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

type Session struct {
	ID        int64     `json:"-"`
	UserID    int64     `json:"-"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"-"`
	CreatedAt time.Time `json:"-"`
}

type RegisterInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

var (
	ErrDuplicateEmail     = errors.New("duplicate email")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrForbidden          = errors.New("forbidden")
)
