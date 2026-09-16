package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	repository *Repository

	// SecureCookies controls the Secure attribute on the session cookie.
	// It defaults to false so local HTTP development keeps working without
	// extra configuration. main.go sets this to true in production
	// (APP_ENV=production), where the app is expected to be served over
	// HTTPS. HttpOnly and SameSite=Lax are always applied regardless.
	SecureCookies bool
}

func NewHandler(repository *Repository) *Handler { return &Handler{repository: repository} }
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var input RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(input.Name)
	email := strings.TrimSpace(strings.ToLower(input.Email))
	password := input.Password
	if name == "" || email == "" || password == "" {
		http.Error(w, "name, email, and password are required", http.StatusBadRequest)
		return
	}
	if len(password) < 8 {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	user, err := h.repository.CreateUser(r.Context(), name, email, string(passwordHash))
	if err != nil {
		if errors.Is(err, ErrDuplicateEmail) {
			http.Error(w, "email already exists", http.StatusConflict)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	token, err := generateSessionToken()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	tokenHash := sha256Hash(token)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err = h.repository.CreateSession(r.Context(), user.ID, tokenHash, expiresAt)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "session_token", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: h.SecureCookies, Expires: expiresAt})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var input LoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(strings.ToLower(input.Email))

	if email == "" || input.Password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return
	}

	user, err := h.repository.FindUserByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}

		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(input.Password),
	); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := generateSessionToken()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	tokenHash := sha256Hash(token)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	_, err = h.repository.CreateSession(
		r.Context(),
		user.ID,
		tokenHash,
		expiresAt,
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.SecureCookies,
		Expires:  expiresAt,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	tokenHash := sha256Hash(cookie.Value)
	session, err := h.repository.FindSessionByTokenHash(r.Context(), tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if err := h.repository.DeleteSessionByID(r.Context(), session.ID); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "session_token", Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: h.SecureCookies, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, err := h.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := h.repository.FindUserByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// Authenticate validates the request's session cookie and returns the
// authenticated user's ID. Other domains (e.g. cart) reuse this to enforce
// authentication without duplicating session/cookie logic.
func (h *Handler) Authenticate(r *http.Request) (int64, error) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		return 0, ErrUnauthenticated
	}
	tokenHash := sha256Hash(cookie.Value)
	session, err := h.repository.FindSessionByTokenHash(r.Context(), tokenHash)
	if err != nil {
		return 0, ErrUnauthenticated
	}
	if time.Now().After(session.ExpiresAt) {
		h.repository.DeleteSessionByID(r.Context(), session.ID)
		return 0, ErrUnauthenticated
	}
	return session.UserID, nil
}

// RequireAdmin validates the request's session cookie and ensures the
// authenticated user has the admin role. The role is always read fresh from
// the database (never trusted from the request), so a client cannot forge
// admin access via headers, body fields, or query parameters. Returns
// ErrUnauthenticated if there is no valid session, or ErrForbidden if the
// session is valid but the user is not an admin.
func (h *Handler) RequireAdmin(r *http.Request) (int64, error) {
	userID, err := h.Authenticate(r)
	if err != nil {
		return 0, err
	}
	user, err := h.repository.FindUserByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrUnauthenticated
		}
		return 0, err
	}
	if !user.IsAdmin() {
		return 0, ErrForbidden
	}
	return userID, nil
}

func generateSessionToken() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
func sha256Hash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
