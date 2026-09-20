package refund

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Behnamdevops/plant-shop/backend/internal/cart"
	"github.com/Behnamdevops/plant-shop/backend/internal/order"
	"github.com/Behnamdevops/plant-shop/backend/internal/product"
	"github.com/jackc/pgx/v5/pgxpool"
)

type testRefundRepositoryEnv struct {
	repository  *Repository
	orderRepo   *order.Repository
	productRepo *product.Repository
	cartRepo    *cart.Repository
	db          *pgxpool.Pool
}

func newTestRefundRepositoryEnv(t *testing.T) *testRefundRepositoryEnv {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL_TEST")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping test")
	}
	db, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Skip("cannot connect to database: ", err)
	}

	orderRepo := order.NewRepository(db)
	productRepo := product.NewRepository(db)
	cartRepo := cart.NewRepository(db)
	repository := NewRepository(db, orderRepo)

	return &testRefundRepositoryEnv{
		repository:  repository,
		orderRepo:   orderRepo,
		productRepo: productRepo,
		cartRepo:    cartRepo,
		db:          db,
	}
}

func (e *testRefundRepositoryEnv) cleanup(t *testing.T) {
	t.Helper()
	e.db.Exec(context.Background(), `DELETE FROM refunds WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%refund%'))`)
	e.db.Exec(context.Background(), `DELETE FROM return_requests WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%refund%'))`)
	e.db.Exec(context.Background(), `DELETE FROM payment_attempts WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%refund%'))`)
	e.db.Exec(context.Background(), `DELETE FROM cart_items WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%refund%')`)
	e.db.Exec(context.Background(), `DELETE FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%refund%')`)
	e.db.Exec(context.Background(), `DELETE FROM users WHERE email LIKE '%refund%'`)
}

func TestRepositoryCheckEligibility(t *testing.T) {
	env := newTestRefundRepositoryEnv(t)
	defer env.cleanup(t)

	// Create a test user using a direct query
	userID, err := createTestUser(env.db, "refund1@refund.com")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create a test product
	product, err := env.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Refund Test Product",
		Slug:  "refund-test-product-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Price: 10000,
		Stock: 100,
	})
	if err != nil {
		t.Fatalf("failed to create product: %v", err)
	}

	// Create a cart and checkout an order
	_, err = env.cartRepo.AddItem(context.Background(), userID, product.ID, 1)
	if err != nil {
		t.Fatalf("failed to add to cart: %v", err)
	}

	order, err := env.orderRepo.CreateFromCart(context.Background(), userID, order.CheckoutInput{
		RecipientName:  "Test Recipient",
		Phone:          "09123456789",
		AddressLine1:   "Test Address",
		City:           "Test City",
		PostalCode:     "1234567890",
		Country:        "Iran",
		ShippingMethod: order.ShippingMethodStandard,
	})
	if err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	// refund.Repository.CheckEligibility is deliberately narrower than
	// returnpkg's return-request eligibility check: it only enforces the
	// V1 financial contract ("only paid, non-cancelled orders may be
	// refunded"), not the return workflow (requested/approved/received).
	// The order does NOT need to be "delivered" at this repository layer —
	// that would conflate two different eligibility concerns. The
	// application-level requirement that a refund can only be triggered
	// once a return request has reached "received" or "refund_pending" is
	// enforced separately, at the HTTP handler layer (see
	// refund.Handler.loadRefundableReturn and its tests in
	// handler_test.go), which is the layer actually reachable over HTTP.

	// Test: order not yet paid -> not eligible
	err = env.repository.CheckEligibility(context.Background(), order.ID)
	if err == nil {
		t.Error("expected error for unpaid order")
	}

	// Mark order as paid (delivered/not-delivered is irrelevant to this
	// repository-level check by design).
	_, err = env.db.Exec(context.Background(), `
		UPDATE orders SET payment_status = $1 WHERE id = $2
	`, "paid", order.ID)
	if err != nil {
		t.Fatalf("failed to update order payment status: %v", err)
	}

	// Test: order paid and not cancelled -> eligible
	err = env.repository.CheckEligibility(context.Background(), order.ID)
	if err != nil {
		t.Errorf("expected order to be eligible: %v", err)
	}

	// Test: cancelled order -> not eligible, even if paid
	_, err = env.db.Exec(context.Background(), `
		UPDATE orders SET status = $1 WHERE id = $2
	`, "cancelled", order.ID)
	if err != nil {
		t.Fatalf("failed to update order status: %v", err)
	}
	err = env.repository.CheckEligibility(context.Background(), order.ID)
	if err == nil {
		t.Error("expected error for cancelled order")
	}
}

func TestRepositoryCreate(t *testing.T) {
	env := newTestRefundRepositoryEnv(t)
	defer env.cleanup(t)

	userID, err := createTestUser(env.db, "refund2@refund.com")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	product, err := env.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Refund Test Product 2",
		Slug:  "refund-test-product-2-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Price: 10000,
		Stock: 100,
	})
	if err != nil {
		t.Fatalf("failed to create product: %v", err)
	}

	_, err = env.cartRepo.AddItem(context.Background(), userID, product.ID, 1)
	if err != nil {
		t.Fatalf("failed to add to cart: %v", err)
	}

	order, err := env.orderRepo.CreateFromCart(context.Background(), userID, order.CheckoutInput{
		RecipientName:  "Test Recipient",
		Phone:          "09123456789",
		AddressLine1:   "Test Address",
		City:           "Test City",
		PostalCode:     "1234567890",
		Country:        "Iran",
		ShippingMethod: order.ShippingMethodStandard,
	})
	if err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	// Mark order as delivered and paid
	_, err = env.db.Exec(context.Background(), `
		UPDATE orders SET status = $1, payment_status = $2 WHERE id = $3
	`, "delivered", "paid", order.ID)
	if err != nil {
		t.Fatalf("failed to update order: %v", err)
	}

	// Test: create refund
	refund, err := env.repository.Create(context.Background(), order.ID, nil, order.Total, "manual", nil)
	if err != nil {
		t.Fatalf("failed to create refund: %v", err)
	}
	if refund.Status != StatusPending {
		t.Errorf("expected status %q, got %q", StatusPending, refund.Status)
	}
	if refund.Amount != order.Total {
		t.Errorf("expected amount %d, got %d", order.Total, refund.Amount)
	}
	if refund.PaymentMethod != "manual" {
		t.Errorf("expected payment_method %q, got %q", "manual", refund.PaymentMethod)
	}
}

func TestRepositoryAlreadyRefunded(t *testing.T) {
	env := newTestRefundRepositoryEnv(t)
	defer env.cleanup(t)

	userID, err := createTestUser(env.db, "refund3@refund.com")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	product, err := env.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Refund Test Product 3",
		Slug:  "refund-test-product-3-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Price: 10000,
		Stock: 100,
	})
	if err != nil {
		t.Fatalf("failed to create product: %v", err)
	}

	_, err = env.cartRepo.AddItem(context.Background(), userID, product.ID, 1)
	if err != nil {
		t.Fatalf("failed to add to cart: %v", err)
	}

	order, err := env.orderRepo.CreateFromCart(context.Background(), userID, order.CheckoutInput{
		RecipientName:  "Test Recipient",
		Phone:          "09123456789",
		AddressLine1:   "Test Address",
		City:           "Test City",
		PostalCode:     "1234567890",
		Country:        "Iran",
		ShippingMethod: order.ShippingMethodStandard,
	})
	if err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	// Mark order as delivered and paid
	_, err = env.db.Exec(context.Background(), `
		UPDATE orders SET status = $1, payment_status = $2 WHERE id = $3
	`, "delivered", "paid", order.ID)
	if err != nil {
		t.Fatalf("failed to update order: %v", err)
	}

	// Create first refund
	refund, err := env.repository.Create(context.Background(), order.ID, nil, order.Total, "manual", nil)
	if err != nil {
		t.Fatalf("failed to create refund: %v", err)
	}

	// Drive the refund through its real state machine: pending -> processing
	// -> succeeded. Skipping "processing" is intentionally rejected by
	// CanTransition (see model.go), so the test must follow the same path
	// production code takes.
	_, err = env.repository.UpdateStatus(context.Background(), refund.ID, StatusProcessing, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("failed to mark refund as processing: %v", err)
	}
	_, err = env.repository.UpdateStatus(context.Background(), refund.ID, StatusSucceeded, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("failed to mark refund as succeeded: %v", err)
	}

	// Test: attempt to refund again -> ErrRefundAlreadyRefunded
	_, err = env.repository.Create(context.Background(), order.ID, nil, order.Total, "manual", nil)
	if err != ErrRefundAlreadyRefunded {
		t.Errorf("expected ErrRefundAlreadyRefunded, got %v", err)
	}

	// Verify order payment_status is still 'refunded'
	var paymentStatus string
	err = env.db.QueryRow(context.Background(), `SELECT payment_status FROM orders WHERE id = $1`, order.ID).Scan(&paymentStatus)
	if err != nil {
		t.Fatalf("failed to query payment_status: %v", err)
	}
	if paymentStatus != "refunded" {
		t.Errorf("expected payment_status %q, got %q", "refunded", paymentStatus)
	}
}

func TestRepositoryUpdateStatus(t *testing.T) {
	env := newTestRefundRepositoryEnv(t)
	defer env.cleanup(t)

	userID, err := createTestUser(env.db, "refund4@refund.com")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	product, err := env.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Refund Test Product 4",
		Slug:  "refund-test-product-4-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Price: 10000,
		Stock: 100,
	})
	if err != nil {
		t.Fatalf("failed to create product: %v", err)
	}

	_, err = env.cartRepo.AddItem(context.Background(), userID, product.ID, 1)
	if err != nil {
		t.Fatalf("failed to add to cart: %v", err)
	}

	order, err := env.orderRepo.CreateFromCart(context.Background(), userID, order.CheckoutInput{
		RecipientName:  "Test Recipient",
		Phone:          "09123456789",
		AddressLine1:   "Test Address",
		City:           "Test City",
		PostalCode:     "1234567890",
		Country:        "Iran",
		ShippingMethod: order.ShippingMethodStandard,
	})
	if err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	// Mark order as delivered and paid
	_, err = env.db.Exec(context.Background(), `
		UPDATE orders SET status = $1, payment_status = $2 WHERE id = $3
	`, "delivered", "paid", order.ID)
	if err != nil {
		t.Fatalf("failed to update order: %v", err)
	}

	// Create refund
	refund, err := env.repository.Create(context.Background(), order.ID, nil, order.Total, "manual", nil)
	if err != nil {
		t.Fatalf("failed to create refund: %v", err)
	}

	// Test: invalid status transition
	_, err = env.repository.UpdateStatus(context.Background(), refund.ID, StatusSucceeded, nil, nil, nil, nil)
	if err != ErrRefundInvalidTransition {
		t.Errorf("expected ErrRefundInvalidTransition (pending -> succeeded is invalid, must go through processing first), got %v", err)
	}

	// Test: valid transition chain
	_, err = env.repository.UpdateStatus(context.Background(), refund.ID, StatusProcessing, nil, nil, nil, nil)
	if err != nil {
		t.Errorf("failed to transition to processing: %v", err)
	}

	var refundResponse json.RawMessage
	json.Unmarshal([]byte(`{"test": "response"}`), &refundResponse)

	_, err = env.repository.UpdateStatus(context.Background(), refund.ID, StatusSucceeded, nil, refundResponse, nil, nil)
	if err != nil {
		t.Errorf("failed to transition to succeeded: %v", err)
	}

	// Verify order payment_status changed to refunded
	var paymentStatus string
	err = env.db.QueryRow(context.Background(), `SELECT payment_status FROM orders WHERE id = $1`, order.ID).Scan(&paymentStatus)
	if err != nil {
		t.Fatalf("failed to query payment_status: %v", err)
	}
	if paymentStatus != "refunded" {
		t.Errorf("expected payment_status %q, got %q", "refunded", paymentStatus)
	}
}

func TestRepositoryNotFound(t *testing.T) {
	env := newTestRefundRepositoryEnv(t)
	defer env.cleanup(t)

	// Test: get non-existent refund
	_, err := env.repository.GetByOrder(context.Background(), 999999)
	if err != ErrRefundNotFound {
		t.Errorf("expected ErrRefundNotFound, got %v", err)
	}

	// Test: update non-existent refund
	_, err = env.repository.UpdateStatus(context.Background(), 999999, StatusProcessing, nil, nil, nil, nil)
	if err != ErrRefundNotFound {
		t.Errorf("expected ErrRefundNotFound, got %v", err)
	}
}

func TestRepositoryIsOrderRefunded(t *testing.T) {
	env := newTestRefundRepositoryEnv(t)
	defer env.cleanup(t)

	userID, err := createTestUser(env.db, "refund5@refund.com")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	product, err := env.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Refund Test Product 5",
		Slug:  "refund-test-product-5-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Price: 10000,
		Stock: 100,
	})
	if err != nil {
		t.Fatalf("failed to create product: %v", err)
	}

	_, err = env.cartRepo.AddItem(context.Background(), userID, product.ID, 1)
	if err != nil {
		t.Fatalf("failed to add to cart: %v", err)
	}

	order, err := env.orderRepo.CreateFromCart(context.Background(), userID, order.CheckoutInput{
		RecipientName:  "Test Recipient",
		Phone:          "09123456789",
		AddressLine1:   "Test Address",
		City:           "Test City",
		PostalCode:     "1234567890",
		Country:        "Iran",
		ShippingMethod: order.ShippingMethodStandard,
	})
	if err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	// Mark order as delivered and paid
	_, err = env.db.Exec(context.Background(), `
		UPDATE orders SET status = $1, payment_status = $2 WHERE id = $3
	`, "delivered", "paid", order.ID)
	if err != nil {
		t.Fatalf("failed to update order: %v", err)
	}

	// Test: order not refunded yet
	refunded, err := env.repository.IsOrderRefunded(context.Background(), order.ID)
	if err != nil {
		t.Fatalf("failed to check if refunded: %v", err)
	}
	if refunded {
		t.Error("expected order to not be refunded")
	}

	// Create and process refund
	refund, err := env.repository.Create(context.Background(), order.ID, nil, order.Total, "manual", nil)
	if err != nil {
		t.Fatalf("failed to create refund: %v", err)
	}

	_, err = env.repository.UpdateStatus(context.Background(), refund.ID, StatusProcessing, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("failed to transition to processing: %v", err)
	}

	_, err = env.repository.UpdateStatus(context.Background(), refund.ID, StatusSucceeded, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("failed to transition to succeeded: %v", err)
	}

	// Test: order refunded
	refunded, err = env.repository.IsOrderRefunded(context.Background(), order.ID)
	if err != nil {
		t.Fatalf("failed to check if refunded: %v", err)
	}
	if !refunded {
		t.Error("expected order to be refunded")
	}
}

// Helper function to create a test user
func createTestUser(db *pgxpool.Pool, email string) (int64, error) {
	var userID int64
	err := db.QueryRow(context.Background(), `
		INSERT INTO users (name, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, "refund user", email, "password_hash", "user").Scan(&userID)
	return userID, err
}

// cleanupForTest cleans up test data for the given email pattern
func (e *testRefundRepositoryEnv) cleanupForTest(emailPattern string) {
	e.db.Exec(context.Background(), `DELETE FROM refunds WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE $1))`, emailPattern)
	e.db.Exec(context.Background(), `DELETE FROM return_requests WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE $1))`, emailPattern)
	e.db.Exec(context.Background(), `DELETE FROM payment_attempts WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE $1))`, emailPattern)
	e.db.Exec(context.Background(), `DELETE FROM cart_items WHERE user_id IN (SELECT id FROM users WHERE email LIKE $1)`, emailPattern)
	e.db.Exec(context.Background(), `DELETE FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE $1)`, emailPattern)
	e.db.Exec(context.Background(), `DELETE FROM users WHERE email LIKE $1`, emailPattern)
}

// TestConcurrentProcessingClaimIsExclusive simulates two admins concurrently
// attempting to process the same refund. It reproduces the claim step used
// by Handler.AdminProcessRefund: transition pending -> processing via
// Repository.UpdateStatus (which takes a `FOR UPDATE` lock on the refund
// row), and only calls the refund provider if that claim succeeds.
//
// Invariant under test: for a single refund, only one concurrent execution
// may successfully claim the "processing" transition, so only one call to
// RefundProvider.Refund() is ever made. A losing request must fail with
// ErrRefundInvalidTransition and must NOT touch the provider.
func TestConcurrentProcessingClaimIsExclusive(t *testing.T) {
	env := newTestRefundRepositoryEnv(t)
	defer env.cleanup(t)

	userID, err := createTestUser(env.db, "refund6@refund.com")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	product, err := env.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Refund Test Product 6",
		Slug:  "refund-test-product-6-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Price: 10000,
		Stock: 100,
	})
	if err != nil {
		t.Fatalf("failed to create product: %v", err)
	}

	_, err = env.cartRepo.AddItem(context.Background(), userID, product.ID, 1)
	if err != nil {
		t.Fatalf("failed to add to cart: %v", err)
	}

	ord, err := env.orderRepo.CreateFromCart(context.Background(), userID, order.CheckoutInput{
		RecipientName:  "Test Recipient",
		Phone:          "09123456789",
		AddressLine1:   "Test Address",
		City:           "Test City",
		PostalCode:     "1234567890",
		Country:        "Iran",
		ShippingMethod: order.ShippingMethodStandard,
	})
	if err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	// Mark order as delivered and paid
	_, err = env.db.Exec(context.Background(), `
		UPDATE orders SET status = $1, payment_status = $2 WHERE id = $3
	`, "delivered", "paid", ord.ID)
	if err != nil {
		t.Fatalf("failed to update order: %v", err)
	}

	refund, err := env.repository.Create(context.Background(), ord.ID, nil, ord.Total, "manual", nil)
	if err != nil {
		t.Fatalf("failed to create refund: %v", err)
	}

	provider := NewFakeRefundProvider(true)

	const concurrency = 8
	var providerCalls int32
	var claimWins int32
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()

			// Step 1: attempt to claim "processing" (mirrors
			// Handler.AdminProcessRefund's claim transition).
			_, err := env.repository.UpdateStatus(context.Background(), refund.ID, StatusProcessing, nil, nil, nil, nil)
			if err != nil {
				// Losing requests must fail the claim and never call the provider.
				if err != ErrRefundInvalidTransition {
					t.Errorf("unexpected error claiming processing: %v", err)
				}
				return
			}

			atomic.AddInt32(&claimWins, 1)

			// Step 2: only the winner reaches the provider call, and it
			// happens outside of any DB transaction/lock.
			result, err := provider.Refund(context.Background(), RefundInput{
				OrderID:       ord.ID,
				Amount:        ord.Total,
				PaymentMethod: refund.PaymentMethod,
			})
			if err != nil {
				t.Errorf("provider call failed: %v", err)
				return
			}
			atomic.AddInt32(&providerCalls, 1)

			if result.Success {
				now := time.Now()
				if _, err := env.repository.UpdateStatus(context.Background(), refund.ID, StatusSucceeded, nil, nil, nil, &now); err != nil {
					t.Errorf("failed to finalize refund: %v", err)
				}
			}
		}()
	}

	wg.Wait()

	if claimWins != 1 {
		t.Errorf("expected exactly 1 goroutine to win the processing claim, got %d", claimWins)
	}
	if providerCalls != 1 {
		t.Errorf("expected exactly 1 call to RefundProvider.Refund(), got %d", providerCalls)
	}

	// Final state must reflect exactly one successful refund.
	finalRefund, err := env.repository.GetByID(context.Background(), refund.ID)
	if err != nil {
		t.Fatalf("failed to get final refund state: %v", err)
	}
	if finalRefund.Status != StatusSucceeded {
		t.Errorf("expected final refund status %q, got %q", StatusSucceeded, finalRefund.Status)
	}

	refunded, err := env.repository.IsOrderRefunded(context.Background(), ord.ID)
	if err != nil {
		t.Fatalf("failed to check IsOrderRefunded: %v", err)
	}
	if !refunded {
		t.Error("expected order to be marked refunded exactly once")
	}
}
