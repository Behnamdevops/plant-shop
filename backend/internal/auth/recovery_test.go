package auth

import (
	"errors"
	"testing"
	"time"
)

func TestResetTokenSingleUseInvalidatesAllSessions(t *testing.T) {
	r := newTestRepository(t)
	u, err := r.CreateUser(t.Context(), "Recovery", uniqueEmail("recovery"), "old-hash")
	if err != nil {
		t.Fatal(err)
	}
	cleanupTestUser(t, r, u.ID)
	token := sha256Hash(uniqueToken("reset"))
	if _, err = r.db.Exec(t.Context(), "INSERT INTO password_reset_tokens(token_hash,user_id,expires_at) VALUES($1,$2,$3)", token, u.ID, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CreateSession(t.Context(), u.ID, uniqueToken("session"), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = r.ResetPassword(t.Context(), token, "new-hash"); err != nil {
		t.Fatal(err)
	}
	if err = r.ResetPassword(t.Context(), token, "bad-hash"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("token reused: %v", err)
	}
	var count int
	r.db.QueryRow(t.Context(), "SELECT count(*) FROM sessions WHERE user_id=$1", u.ID).Scan(&count)
	if count != 0 {
		t.Fatalf("sessions remain: %d", count)
	}
	user, err := r.FindUserByID(t.Context(), u.ID)
	if err != nil || user.PasswordHash != "new-hash" {
		t.Fatalf("password result: %v", err)
	}
}
