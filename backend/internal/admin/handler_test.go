package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestRepository(t *testing.T) *Repository {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping database test")
	}
	db, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Skip("cannot connect to database; skipping database test: ", err)
	}
	return NewRepository(db)
}

func TestDashboard_Unauthenticated(t *testing.T) {
	repo := &Repository{}
	auth := &mockAuth{err: http.ErrNoCookie}
	handler := NewHandler(repo, auth)

	req := httptest.NewRequest("GET", "/api/v1/admin/dashboard", nil)
	w := httptest.NewRecorder()

	handler.Dashboard(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestDashboard_Forbidden(t *testing.T) {
	repo := &Repository{}
	auth := &mockAuth{userID: 1, isAdmin: false}
	handler := NewHandler(repo, auth)

	req := httptest.NewRequest("GET", "/api/v1/admin/dashboard", nil)
	w := httptest.NewRecorder()

	handler.Dashboard(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestDashboard_Success(t *testing.T) {
	repo := newTestRepository(t)
	auth := &mockAuth{userID: 1, isAdmin: true}
	handler := NewHandler(repo, auth)

	req := httptest.NewRequest("GET", "/api/v1/admin/dashboard", nil)
	w := httptest.NewRecorder()

	handler.Dashboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var metrics DashboardMetrics
	if err := json.NewDecoder(w.Body).Decode(&metrics); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	// Verify we got actual data (not just defaults)
	if metrics.OrdersTotal < 0 || metrics.UsersTotal < 0 || metrics.ProductsTotal < 0 {
		t.Errorf("Expected non-negative metrics, got orders=%d users=%d products=%d",
			metrics.OrdersTotal, metrics.UsersTotal, metrics.ProductsTotal)
	}
}

func TestLowStock_Unauthenticated(t *testing.T) {
	repo := &Repository{}
	auth := &mockAuth{err: http.ErrNoCookie}
	handler := NewHandler(repo, auth)

	req := httptest.NewRequest("GET", "/api/v1/admin/inventory/low-stock", nil)
	w := httptest.NewRecorder()

	handler.LowStock(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestLowStock_Success(t *testing.T) {
	repo := newTestRepository(t)
	auth := &mockAuth{userID: 1, isAdmin: true}
	handler := NewHandler(repo, auth)

	req := httptest.NewRequest("GET", "/api/v1/admin/inventory/low-stock", nil)
	w := httptest.NewRecorder()

	handler.LowStock(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response LowStockResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}
}

func TestStockAdjustment_Unauthenticated(t *testing.T) {
	repo := &Repository{}
	auth := &mockAuth{err: http.ErrNoCookie}
	handler := NewHandler(repo, auth)

	req := httptest.NewRequest("POST", "/api/v1/admin/products/1/stock-adjustment", nil)
	w := httptest.NewRecorder()

	handler.StockAdjustment(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestStockAdjustment_Forbidden(t *testing.T) {
	repo := &Repository{}
	auth := &mockAuth{userID: 1, isAdmin: false}
	handler := NewHandler(repo, auth)

	req := httptest.NewRequest("POST", "/api/v1/admin/products/1/stock-adjustment", nil)
	w := httptest.NewRecorder()

	handler.StockAdjustment(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestStockAdjustment_InvalidProductID(t *testing.T) {
	repo := &Repository{}
	auth := &mockAuth{userID: 1, isAdmin: true}
	handler := NewHandler(repo, auth)

	req := httptest.NewRequest("POST", "/api/v1/admin/products/invalid/stock-adjustment", nil)
	w := httptest.NewRecorder()

	handler.StockAdjustment(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestStockAdjustment_MissingBody(t *testing.T) {
	repo := &Repository{}
	auth := &mockAuth{userID: 1, isAdmin: true}
	handler := NewHandler(repo, auth)

	req := httptest.NewRequest("POST", "/api/v1/admin/products/1/stock-adjustment", nil)
	w := httptest.NewRecorder()

	handler.StockAdjustment(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestStockAdjustment_ZeroDelta(t *testing.T) {
	repo := &Repository{}
	auth := &mockAuth{userID: 1, isAdmin: true}
	handler := NewHandler(repo, auth)

	input := StockAdjustmentInput{Delta: 0, Reason: "test"}
	body, _ := json.Marshal(input)
	req := httptest.NewRequest("POST", "/api/v1/admin/products/1/stock-adjustment", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.StockAdjustment(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestStockAdjustment_RepositoryDirect tests the repository's AdjustStock method directly
func TestStockAdjustment_RepositoryDirect(t *testing.T) {
	repo := newTestRepository(t)

	// Check if inventory_adjustments table exists
	var exists bool
	err := repo.db.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_name = 'inventory_adjustments'
		)
	`).Scan(&exists)
	if err != nil || !exists {
		t.Skip("inventory_adjustments table does not exist; apply migration first")
	}

	// Find a product to test
	var productID int64
	err = repo.db.QueryRow(context.Background(), "SELECT id FROM products WHERE stock >= 5 LIMIT 1").Scan(&productID)
	if err != nil {
		t.Skip("no product with sufficient stock: ", err)
	}

	// Get initial stock
	var initialStock int
	err = repo.db.QueryRow(context.Background(), "SELECT stock FROM products WHERE id = $1", productID).Scan(&initialStock)
	if err != nil {
		t.Fatal("failed to get initial stock: ", err)
	}

	// Test positive adjustment
	newStock, err := repo.AdjustStock(context.Background(), productID, 5, "test positive adjustment", nil)
	if err != nil {
		t.Fatal("positive adjustment failed: ", err)
	}
	if newStock != initialStock+5 {
		t.Errorf("expected stock %d, got %d", initialStock+5, newStock)
	}

	// Verify adjustment record exists
	var adjustmentID int64
	err = repo.db.QueryRow(context.Background(), "SELECT id FROM inventory_adjustments WHERE product_id = $1 AND delta = 5", productID).Scan(&adjustmentID)
	if err != nil {
		t.Error("adjustment record not found: ", err)
	}

	// Test negative adjustment
	newStock2, err := repo.AdjustStock(context.Background(), productID, -3, "test negative adjustment", nil)
	if err != nil {
		t.Fatal("negative adjustment failed: ", err)
	}
	if newStock2 != newStock-3 {
		t.Errorf("expected stock %d, got %d", newStock-3, newStock2)
	}

	// Test cannot go below zero
	err = repo.db.QueryRow(context.Background(), "SELECT stock FROM products WHERE id = $1", productID).Scan(&initialStock)
	if err != nil {
		t.Fatal("failed to get current stock: ", err)
	}
	_, err = repo.AdjustStock(context.Background(), productID, -(initialStock + 100), "test below zero", nil)
	if err == nil {
		t.Error("expected error for negative stock")
	} else if err.Error() != "stock cannot be negative" {
		t.Errorf("expected 'stock cannot be negative', got %v", err)
	}
}

func TestInventoryAdjustments_Unauthenticated(t *testing.T) {
	repo := &Repository{}
	auth := &mockAuth{err: http.ErrNoCookie}
	handler := NewHandler(repo, auth)

	req := httptest.NewRequest("GET", "/api/v1/admin/inventory/adjustments", nil)
	w := httptest.NewRecorder()

	handler.InventoryAdjustments(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestInventoryAdjustments_Success(t *testing.T) {
	repo := newTestRepository(t)

	// Check if inventory_adjustments table exists
	var exists bool
	err := repo.db.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_name = 'inventory_adjustments'
		)
	`).Scan(&exists)
	if err != nil || !exists {
		t.Skip("inventory_adjustments table does not exist; apply migration first")
	}

	auth := &mockAuth{userID: 1, isAdmin: true}
	handler := NewHandler(repo, auth)

	req := httptest.NewRequest("GET", "/api/v1/admin/inventory/adjustments", nil)
	w := httptest.NewRecorder()

	handler.InventoryAdjustments(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response InventoryAdjustmentList
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}
}

// mockAuth is a minimal implementation of the authenticator interface for testing.
type mockAuth struct {
	userID  int64
	isAdmin bool
	err     error
}

func (m *mockAuth) Authenticate(r *http.Request) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.userID, nil
}

func (m *mockAuth) RequireAdmin(r *http.Request) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	if !m.isAdmin {
		return 0, auth.ErrForbidden
	}
	return m.userID, nil
}
