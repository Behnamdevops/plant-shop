package coupon

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/Behnamdevops/plant-shop/backend/internal/cart"
)

// authenticator mirrors the interface used by other packages (order,
// category) so this package stays decoupled from auth's concrete type.
type authenticator interface {
	Authenticate(r *http.Request) (int64, error)
	RequireAdmin(r *http.Request) (int64, error)
}

type Handler struct {
	repository *Repository
	cartRepo   *cart.Repository
	auth       authenticator
}

func NewHandler(repository *Repository, cartRepo *cart.Repository, authHandler *auth.Handler) *Handler {
	return &Handler{repository: repository, cartRepo: cartRepo, auth: authHandler}
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	_, err := h.auth.RequireAdmin(r)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			http.Error(w, "forbidden", http.StatusForbidden)
		} else {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		return false
	}
	return true
}

// previewRequest is the request body for POST /api/v1/coupons/preview.
type previewRequest struct {
	Code string `json:"code"`
}

// Preview loads the authenticated user's current cart, validates the
// supplied coupon code against it, and returns the resulting discount
// preview. This is advisory only: it never persists a redemption, and
// order creation always revalidates/recalculates everything from scratch
// inside its own transaction. Never trust this response as authoritative.
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body previewRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	c, err := h.cartRepo.GetCart(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if len(c.Items) == 0 {
		http.Error(w, "cart is empty", http.StatusBadRequest)
		return
	}

	coup, discount, err := h.repository.EligiblePreview(r.Context(), body.Code, userID, c.Total, time.Now())
	if err != nil {
		var eerr *EligibilityError
		if errors.As(err, &eerr) {
			http.Error(w, eerr.Message, http.StatusBadRequest)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := Preview{
		Code:                    coup.Code,
		DiscountType:            coup.DiscountType,
		ItemsSubtotal:           c.Total,
		DiscountAmount:          discount,
		DiscountedItemsSubtotal: c.Total - discount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// AdminList returns every coupon for admin management. Admin-only.
func (h *Handler) AdminList(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	coupons, err := h.repository.ListAll(r.Context())
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(coupons)
}

// AdminGetByID returns a single coupon for admin management. Admin-only.
func (h *Handler) AdminGetByID(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	id, ok := parseID(w, r)
	if !ok {
		return
	}

	c, err := h.repository.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "coupon not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}

// AdminCreate creates a new coupon. Admin-only.
func (h *Handler) AdminCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	var input CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	validated, err := ValidateCreateInput(input)
	if err != nil {
		writeValidationError(w, err)
		return
	}

	c, err := h.repository.Create(r.Context(), validated)
	if err != nil {
		if errors.Is(err, ErrDuplicateCode) {
			http.Error(w, "coupon code already exists", http.StatusConflict)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(c)
}

// AdminUpdate updates an existing coupon. Admin-only. For V1, prefer
// setting is_active=false to disabling a coupon rather than deleting it —
// there is no delete endpoint, since a used coupon's row must remain
// resolvable (coupon_id is best-effort; orders.coupon_code/discount_amount
// are the authoritative historical snapshot regardless).
func (h *Handler) AdminUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	id, ok := parseID(w, r)
	if !ok {
		return
	}

	var raw rawUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	input := raw.toUpdateInput()

	existing, err := h.repository.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "coupon not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	updated, err := ApplyUpdate(existing.Coupon, input)
	if err != nil {
		writeValidationError(w, err)
		return
	}

	c, err := h.repository.Update(r.Context(), id, updated)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "coupon not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrDuplicateCode) {
			http.Error(w, "coupon code already exists", http.StatusConflict)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}

// rawUpdateRequest is decoded first (using json.RawMessage for the
// optional nullable fields) so the handler can tell "field omitted" apart
// from "field explicitly set to null" — needed for usage_limit,
// per_user_limit, starts_at, and ends_at, which are all meaningfully
// nullable (NULL = unlimited / no bound).
type rawUpdateRequest struct {
	Code           *string         `json:"code"`
	DiscountType   *string         `json:"discount_type"`
	Value          *int64          `json:"value"`
	MinOrderAmount *int64          `json:"min_order_amount"`
	UsageLimit     json.RawMessage `json:"usage_limit"`
	PerUserLimit   json.RawMessage `json:"per_user_limit"`
	StartsAt       json.RawMessage `json:"starts_at"`
	EndsAt         json.RawMessage `json:"ends_at"`
	IsActive       *bool           `json:"is_active"`
}

func (raw rawUpdateRequest) toUpdateInput() UpdateInput {
	in := UpdateInput{
		Code:           raw.Code,
		DiscountType:   raw.DiscountType,
		Value:          raw.Value,
		MinOrderAmount: raw.MinOrderAmount,
		IsActive:       raw.IsActive,
	}
	if len(raw.UsageLimit) > 0 {
		in.HasUsageLimit = true
		var v *int
		json.Unmarshal(raw.UsageLimit, &v)
		in.UsageLimit = v
	}
	if len(raw.PerUserLimit) > 0 {
		in.HasPerUserLimit = true
		var v *int
		json.Unmarshal(raw.PerUserLimit, &v)
		in.PerUserLimit = v
	}
	if len(raw.StartsAt) > 0 {
		in.HasStartsAt = true
		var v *time.Time
		json.Unmarshal(raw.StartsAt, &v)
		in.StartsAt = v
	}
	if len(raw.EndsAt) > 0 {
		in.HasEndsAt = true
		var v *time.Time
		json.Unmarshal(raw.EndsAt, &v)
		in.EndsAt = v
	}
	return in
}

func writeValidationError(w http.ResponseWriter, err error) {
	var verr *ValidationError
	if errors.As(err, &verr) {
		http.Error(w, verr.Error(), http.StatusBadRequest)
		return
	}
	http.Error(w, "invalid request", http.StatusBadRequest)
}

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}
