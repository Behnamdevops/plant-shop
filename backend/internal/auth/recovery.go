package auth

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

type recoveryNotifier interface {
	QueuePasswordReset(context.Context, pgx.Tx, int64, string, string, string, string) error
}

func (h *Handler) Recovery(notifier recoveryNotifier, base string, enabled bool) *Handler {
	h.notifier = notifier
	h.resetBase = strings.TrimSuffix(base, "/")
	h.resetEnabled = enabled
	return h
}
func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		http.Error(w, "invalid request body", 400)
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	a, err := mail.ParseAddress(email)
	if err != nil || a.Address != email || len(email) > 254 {
		http.Error(w, "invalid email", 400)
		return
	}
	if !h.resetEnabled || h.resetBase == "" || h.notifier == nil {
		http.Error(w, "password recovery is unavailable; contact support", 503)
		return
	}
	u, err := h.repository.FindUserByEmail(r.Context(), email)
	if errors.Is(err, pgx.ErrNoRows) {
		w.WriteHeader(202)
		return
	}
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	token, err := generateSessionToken()
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	ctx := r.Context()
	tx, err := h.repository.db.Begin(ctx)
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", u.ID); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	if _, err = tx.Exec(ctx, "DELETE FROM password_reset_tokens WHERE user_id=$1", u.ID); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	hash := sha256Hash(token)
	if _, err = tx.Exec(ctx, "INSERT INTO password_reset_tokens(token_hash,user_id,expires_at) VALUES($1,$2,$3)", hash, u.ID, time.Now().Add(30*time.Minute)); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	if err = h.notifier.QueuePasswordReset(ctx, tx, u.ID, u.Email, u.Name, h.resetBase+"/reset-password?token="+token, hash); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	if tx.Commit(ctx) != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	w.WriteHeader(202)
}
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || len(input.Token) != 64 || len(input.Password) < 8 || len(input.Password) > 72 {
		http.Error(w, "invalid password reset", 400)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	err = h.repository.ResetPassword(r.Context(), sha256Hash(input.Token), string(hash))
	if err != nil {
		if errors.Is(err, ErrUnauthenticated) {
			http.Error(w, "password reset link is invalid or expired", 400)
		} else {
			http.Error(w, "internal server error", 500)
		}
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "session_token", Value: "", Path: "/", HttpOnly: true, Secure: h.SecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.WriteHeader(204)
}
func (r *Repository) ResetPassword(ctx context.Context, token, password string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var user int64
	err = tx.QueryRow(ctx, `SELECT u.id FROM users u JOIN password_reset_tokens t ON t.user_id=u.id WHERE t.token_hash=$1 AND t.used_at IS NULL AND t.expires_at>NOW() FOR UPDATE OF u`, token).Scan(&user)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUnauthenticated
	}
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, "UPDATE password_reset_tokens SET used_at=NOW() WHERE token_hash=$1 AND used_at IS NULL AND expires_at>NOW()", token)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrUnauthenticated
	}
	if _, err = tx.Exec(ctx, "UPDATE users SET password_hash=$1,updated_at=NOW() WHERE id=$2", password, user); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM sessions WHERE user_id=$1", user); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *Repository) CleanupExpired(ctx context.Context) error {
	if _, err := r.db.Exec(ctx, "DELETE FROM sessions WHERE expires_at<NOW()"); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, "DELETE FROM password_reset_tokens WHERE used_at IS NOT NULL OR expires_at<NOW()")
	return err
}
