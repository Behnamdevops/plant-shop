package notification

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
)

// Handler provides HTTP handlers for the notification outbox.
// Currently only admin-facing endpoints for inspection.
type Handler struct {
	repo *Repository
	auth *auth.Handler
}

// NewHandler creates a new notification handler.
func NewHandler(repo *Repository, authHandler *auth.Handler) *Handler {
	return &Handler{repo: repo, auth: authHandler}
}

// adminAccess checks if the request has admin access.
func (h *Handler) adminAccess(w http.ResponseWriter, r *http.Request) bool {
	if _, err := h.auth.RequireAdmin(r); err != nil {
		if err == auth.ErrForbidden {
			http.Error(w, "forbidden", http.StatusForbidden)
		} else {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		return false
	}
	return true
}

// List returns all notifications, newest first. Admin-only.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if !h.adminAccess(w, r) {
		return
	}

	limit := int64(50)
	beforeID := int64(0)

	for name, target := range map[string]*int64{"limit": &limit, "before_id": &beforeID} {
		if raw, ok := r.URL.Query()[name]; ok {
			value, err := strconv.ParseInt(raw[0], 10, 64)
			if err != nil || value <= 0 || name == "limit" && value > 100 {
				http.Error(w, "invalid pagination", http.StatusBadRequest)
				return
			}
			*target = value
		}
	}

	notifications, err := h.repo.List(r.Context(), limit, beforeID)
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	for i := range notifications {
		redactRecovery(&notifications[i])
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notifications)
}

// GetByID returns a single notification by ID. Admin-only.
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	if !h.adminAccess(w, r) {
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	n, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if err.Error() == "notification not found" {
			http.Error(w, "notification not found", http.StatusNotFound)
		} else {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	redactRecovery(n)
	json.NewEncoder(w).Encode(n)
}

// Count returns the count of pending notifications. Admin-only.
func (h *Handler) Count(w http.ResponseWriter, r *http.Request) {
	if !h.adminAccess(w, r) {
		return
	}

	var count int
	if err := h.repo.db.QueryRow(r.Context(), "SELECT count(*) FROM notification_outbox WHERE status IN ('pending','processing')").Scan(&count); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"pending": count})
}

func redactRecovery(n *NotificationOutbox) {
	if n.EventType == "password_reset" {
		n.Payload = Payload{"reset_url": "[redacted]"}
	}
}
