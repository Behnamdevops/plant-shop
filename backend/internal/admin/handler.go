package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
)

type authenticator interface {
	Authenticate(r *http.Request) (int64, error)
	RequireAdmin(r *http.Request) (int64, error)
}

type Handler struct {
	repository *Repository
	auth       authenticator
}

func NewHandler(repository *Repository, auth authenticator) *Handler {
	return &Handler{
		repository: repository,
		auth:       auth,
	}
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	metrics, err := h.repository.DashboardMetrics(r.Context())
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (h *Handler) LowStock(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	page := 1
	pageSize := 50

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	response, err := h.repository.LowStockProducts(r.Context(), page, pageSize)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

type stockAdjustmentRequest struct {
	Delta  int    `json:"delta"`
	Reason string `json:"reason"`
}

func (h *Handler) StockAdjustment(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	productID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || productID <= 0 {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}

	var input stockAdjustmentRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if input.Delta == 0 {
		http.Error(w, "delta cannot be zero", http.StatusBadRequest)
		return
	}

	// Validate delta bounds (prevent absurdly large adjustments)
	if input.Delta < -100000 || input.Delta > 100000 {
		http.Error(w, "delta out of reasonable bounds", http.StatusBadRequest)
		return
	}

	// Validate reason length
	if len(input.Reason) > 255 {
		http.Error(w, "reason too long", http.StatusBadRequest)
		return
	}

	if input.Reason == "" {
		http.Error(w, "reason is required", http.StatusBadRequest)
		return
	}

	// Get admin user ID from authenticated session
	adminUserID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Perform stock adjustment
	newStock, err := h.repository.AdjustStock(r.Context(), productID, input.Delta, input.Reason, &adminUserID)
	if err != nil {
		if errors.Is(err, fmt.Errorf("product not found")) {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, fmt.Errorf("stock cannot be negative")) {
			http.Error(w, "stock cannot be negative", http.StatusConflict)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Fetch product name for response
	productName, _, _, err := h.repository.GetProductByID(r.Context(), productID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Get the adjustment record for timestamp
	adjustments, err := h.repository.InventoryAdjustments(r.Context(), &productID, 1, 1)
	if err != nil || len(adjustments.Items) == 0 {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	response := StockAdjustmentResponse{
		ProductID:   productID,
		ProductName: productName,
		StockBefore: newStock - input.Delta,
		StockAfter:  newStock,
		Delta:       input.Delta,
		Reason:      input.Reason,
		CreatedAt:   adjustments.Items[0].CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) InventoryAdjustments(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	productIDStr := r.URL.Query().Get("product_id")
	var productID *int64
	if productIDStr != "" {
		if pid, err := strconv.ParseInt(productIDStr, 10, 64); err == nil && pid > 0 {
			productID = &pid
		}
	}

	page := 1
	pageSize := 20

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	response, err := h.repository.InventoryAdjustments(r.Context(), productID, page, pageSize)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
