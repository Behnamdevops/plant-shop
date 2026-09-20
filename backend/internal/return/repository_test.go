package returnpkg

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Behnamdevops/plant-shop/backend/internal/cart"
	"github.com/Behnamdevops/plant-shop/backend/internal/order"
	"github.com/Behnamdevops/plant-shop/backend/internal/payment"
	"github.com/Behnamdevops/plant-shop/backend/internal/product"
	"github.com/jackc/pgx/v5/pgxpool"
)

type testRepositoryEnv struct {
	repository  *Repository
	orderRepo   *order.Repository
	productRepo *product.Repository
	paymentRepo *payment.Repository
	cartRepo    *cart.Repository
	db          *pgxpool.Pool
}

func newTestRepositoryEnv(t *testing.T) *testRepositoryEnv {
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
	paymentRepo := payment.NewRepository(db)
	repository := NewRepository(db, orderRepo)

	return &testRepositoryEnv{
		repository:  repository,
		orderRepo:   orderRepo,
		productRepo: productRepo,
		cartRepo:    cartRepo,
		paymentRepo: paymentRepo,
		db:          db,
	}
}

func (e *testRepositoryEnv) cleanup(t *testing.T) {
	t.Helper()
	e.db.Exec(context.Background(), `DELETE FROM payment_attempts WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%test%'))`)
	e.db.Exec(context.Background(), `DELETE FROM return_requests WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%test%'))`)
	e.db.Exec(context.Background(), `DELETE FROM refunds WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%test%'))`)
	e.db.Exec(context.Background(), `DELETE FROM cart_items WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%test%')`)
	e.db.Exec(context.Background(), `DELETE FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%test%')`)
	e.db.Exec(context.Background(), `DELETE FROM users WHERE email LIKE '%test%'`)
}

func TestRepositoryCheckEligibility(t *testing.T) {
	env := newTestRepositoryEnv(t)
	defer env.cleanup(t)

	// Create a test user
	userID, err := createTestUser(env.db, "test1@test.com")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create a test product
	product, err := env.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Test Product",
		Slug:  "test-product-return-" + fmt.Sprintf("%d", time.Now().UnixNano()),
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

	// Test: order not delivered -> not eligible
	err = env.repository.CheckEligibility(context.Background(), order.ID, userID)
	if err == nil {
		t.Error("expected error for non-delivered order")
	}

	// Mark order as paid (for payment status check)
	_, err = env.db.Exec(context.Background(), `
		UPDATE orders SET payment_status = $1 WHERE id = $2
	`, "paid", order.ID)
	if err != nil {
		t.Fatalf("failed to update order payment status: %v", err)
	}

	// Test: order delivered but not paid -> not eligible
	err = env.repository.CheckEligibility(context.Background(), order.ID, userID)
	if err == nil {
		t.Error("expected error for paid but not delivered order")
	}

	// Mark order as delivered
	_, err = env.db.Exec(context.Background(), `
		UPDATE orders SET status = $1 WHERE id = $2
	`, "delivered", order.ID)
	if err != nil {
		t.Fatalf("failed to update order status: %v", err)
	}

	// Test: order delivered and paid -> eligible
	err = env.repository.CheckEligibility(context.Background(), order.ID, userID)
	if err != nil {
		t.Errorf("expected order to be eligible: %v", err)
	}
}

func TestRepositoryCreateRequest(t *testing.T) {
	env := newTestRepositoryEnv(t)
	defer env.cleanup(t)

	// Create a test user and order that is delivered and paid
	userID, err := createTestUser(env.db, "test2@test.com")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	product, err := env.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Test Product 2",
		Slug:  "test-product-return-2-" + fmt.Sprintf("%d", time.Now().UnixNano()),
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

	// Test: create return request
	input := RequestInput{
		Reason: "Defective product",
	}
	rr, err := env.repository.CreateRequest(context.Background(), order.ID, userID, input)
	if err != nil {
		t.Fatalf("failed to create return request: %v", err)
	}
	if rr.Status != StatusRequested {
		t.Errorf("expected status %q, got %q", StatusRequested, rr.Status)
	}
	if rr.Reason != "Defective product" {
		t.Errorf("expected reason %q, got %q", "Defective product", rr.Reason)
	}

	// Test: duplicate request -> ErrReturnAlreadyExists
	_, err = env.repository.CreateRequest(context.Background(), order.ID, userID, input)
	if err != ErrReturnAlreadyExists {
		t.Errorf("expected ErrReturnAlreadyExists, got %v", err)
	}
}

func TestRepositoryUpdateStatus(t *testing.T) {
	env := newTestRepositoryEnv(t)
	defer env.cleanup(t)

	userID, err := createTestUser(env.db, "test3@test.com")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	product, err := env.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Test Product 3",
		Slug:  "test-product-return-3-" + fmt.Sprintf("%d", time.Now().UnixNano()),
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

	// Create return request
	input := RequestInput{Reason: "Defective"}
	rr, err := env.repository.CreateRequest(context.Background(), order.ID, userID, input)
	if err != nil {
		t.Fatalf("failed to create return request: %v", err)
	}

	// Test: valid transition (requested -> approved)
	rr, err = env.repository.UpdateStatus(context.Background(), rr.ID, StatusApproved, nil, nil)
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}
	if rr.Status != StatusApproved {
		t.Errorf("expected status %q, got %q", StatusApproved, rr.Status)
	}

	// Test: invalid transition (approved -> rejected - must go through received first)
	_, err = env.repository.UpdateStatus(context.Background(), rr.ID, StatusRejected, nil, nil)
	if err != ErrReturnInvalidTransition {
		t.Errorf("expected ErrReturnInvalidTransition, got %v", err)
	}

	// Test: valid transition (approved -> received)
	rr, err = env.repository.UpdateStatus(context.Background(), rr.ID, StatusReceived, nil, nil)
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}
	if rr.Status != StatusReceived {
		t.Errorf("expected status %q, got %q", StatusReceived, rr.Status)
	}
}

func TestRepositoryEligibilityValidation(t *testing.T) {
	env := newTestRepositoryEnv(t)
	defer env.cleanup(t)

	userID, err := createTestUser(env.db, "test4@test.com")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	product, err := env.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Test Product 4",
		Slug:  "test-product-return-4-" + fmt.Sprintf("%d", time.Now().UnixNano()),
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

	// Create first return request
	input := RequestInput{Reason: "First request"}
	_, err = env.repository.CreateRequest(context.Background(), order.ID, userID, input)
	if err != nil {
		t.Fatalf("failed to create return request: %v", err)
	}

	// Test: duplicate request -> ErrReturnAlreadyExists
	_, err = env.repository.CreateRequest(context.Background(), order.ID, userID, input)
	if err != ErrReturnAlreadyExists {
		t.Errorf("expected ErrReturnAlreadyExists for duplicate request, got %v", err)
	}
}

func TestRepositoryInvalidStatus(t *testing.T) {
	env := newTestRepositoryEnv(t)
	defer env.cleanup(t)

	userID, err := createTestUser(env.db, "test5@test.com")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	product, err := env.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Test Product 5",
		Slug:  "test-product-return-5-" + fmt.Sprintf("%d", time.Now().UnixNano()),
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

	// Create return request
	input := RequestInput{Reason: "Defective"}
	rr, err := env.repository.CreateRequest(context.Background(), order.ID, userID, input)
	if err != nil {
		t.Fatalf("failed to create return request: %v", err)
	}

	// Test: invalid status value. UpdateStatus's documented contract
	// (model.go, matched by handler.go's errors.Is checks) is the sentinel
	// ErrReturnInvalidStatus, not the *ErrValidation type used for
	// request-body field validation elsewhere in this package.
	_, err = env.repository.UpdateStatus(context.Background(), rr.ID, "invalid_status", nil, nil)
	if err != ErrReturnInvalidStatus {
		t.Errorf("expected ErrReturnInvalidStatus, got %v (%T)", err, err)
	}
}

func TestRepositoryTransitionValidation(t *testing.T) {
	env := newTestRepositoryEnv(t)
	defer env.cleanup(t)

	userID, err := createTestUser(env.db, "test6@test.com")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	product, err := env.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Test Product 6",
		Slug:  "test-product-return-6-" + fmt.Sprintf("%d", time.Now().UnixNano()),
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

	// Create return request
	input := RequestInput{Reason: "Defective"}
	rr, err := env.repository.CreateRequest(context.Background(), order.ID, userID, input)
	if err != nil {
		t.Fatalf("failed to create return request: %v", err)
	}

	// Test: valid transition chain
	states := []string{StatusApproved, StatusReceived, StatusRefundPending}
	for _, state := range states {
		rr, err = env.repository.UpdateStatus(context.Background(), rr.ID, state, nil, nil)
		if err != nil {
			t.Errorf("transition to %s failed: %v", state, err)
		}
	}

	// Test: can't go back from refund_pending
	_, err = env.repository.UpdateStatus(context.Background(), rr.ID, StatusApproved, nil, nil)
	if err != ErrReturnInvalidTransition {
		t.Errorf("expected ErrReturnInvalidTransition, got %v", err)
	}
}

func TestRepositoryNotFound(t *testing.T) {
	env := newTestRepositoryEnv(t)
	defer env.cleanup(t)

	// Test: get non-existent return request
	_, err := env.repository.GetByOrder(context.Background(), 999999)
	if err != ErrReturnRequestNotFound {
		t.Errorf("expected ErrReturnRequestNotFound, got %v", err)
	}

	// Test: update non-existent return request
	_, err = env.repository.UpdateStatus(context.Background(), 999999, StatusApproved, nil, nil)
	if err != ErrReturnRequestNotFound {
		t.Errorf("expected ErrReturnRequestNotFound, got %v", err)
	}
}

// Helper function to create a test user
func createTestUser(db *pgxpool.Pool, email string) (int64, error) {
	var userID int64
	err := db.QueryRow(context.Background(), `
		INSERT INTO users (name, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, "test user", email, "password_hash", "user").Scan(&userID)
	return userID, err
}

// cleanupForTest cleans up test data for the given email pattern
func (e *testRepositoryEnv) cleanupForTest(emailPattern string) {
	e.db.Exec(context.Background(), `DELETE FROM payment_attempts WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE $1))`, emailPattern)
	e.db.Exec(context.Background(), `DELETE FROM return_requests WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE $1))`, emailPattern)
	e.db.Exec(context.Background(), `DELETE FROM refunds WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE $1))`, emailPattern)
	e.db.Exec(context.Background(), `DELETE FROM cart_items WHERE user_id IN (SELECT id FROM users WHERE email LIKE $1)`, emailPattern)
	e.db.Exec(context.Background(), `DELETE FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE $1)`, emailPattern)
	e.db.Exec(context.Background(), `DELETE FROM users WHERE email LIKE $1`, emailPattern)
}

// TestCannotMarkRefundedWithoutSucceededRefund is a regression test for a
// financial-integrity gap: UpdateStatus previously allowed a return request
// to move refund_pending -> refunded purely via the return_requests state
// machine, with no check against the refunds table. That let a return be
// marked "refunded" (customer-facing, terminal) with no refund ever having
// succeeded and no change to orders.payment_status. UpdateStatus must now
// require an authoritative succeeded refund row for the order before
// allowing the "refunded" transition.
func TestCannotMarkRefundedWithoutSucceededRefund(t *testing.T) {
	env := newTestRepositoryEnv(t)
	defer env.cleanup(t)

	userID, err := createTestUser(env.db, "test7@test.com")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	product, err := env.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Test Product 7",
		Slug:  "test-product-return-7-" + fmt.Sprintf("%d", time.Now().UnixNano()),
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

	input := RequestInput{Reason: "Defective"}
	rr, err := env.repository.CreateRequest(context.Background(), ord.ID, userID, input)
	if err != nil {
		t.Fatalf("failed to create return request: %v", err)
	}

	// Drive the return request to refund_pending without ever creating a
	// refund row.
	for _, state := range []string{StatusApproved, StatusReceived, StatusRefundPending} {
		rr, err = env.repository.UpdateStatus(context.Background(), rr.ID, state, nil, nil)
		if err != nil {
			t.Fatalf("transition to %s failed: %v", state, err)
		}
	}

	// No refund row exists yet: marking the return "refunded" must be rejected.
	_, err = env.repository.UpdateStatus(context.Background(), rr.ID, StatusRefunded, nil, nil)
	if err != ErrReturnInvalidTransition {
		t.Fatalf("expected ErrReturnInvalidTransition without a succeeded refund, got %v", err)
	}

	// Order payment_status must remain untouched.
	var paymentStatus string
	if err := env.db.QueryRow(context.Background(), `SELECT payment_status FROM orders WHERE id = $1`, ord.ID).Scan(&paymentStatus); err != nil {
		t.Fatalf("failed to query payment_status: %v", err)
	}
	if paymentStatus != "paid" {
		t.Errorf("expected payment_status to remain %q, got %q", "paid", paymentStatus)
	}

	// Create a refund row but leave it pending (not yet succeeded): still rejected.
	var refundID int64
	if err := env.db.QueryRow(context.Background(), `
		INSERT INTO refunds (order_id, return_request_id, amount, payment_method, status)
		VALUES ($1, $2, $3, 'manual', 'pending')
		RETURNING id
	`, ord.ID, rr.ID, ord.Total).Scan(&refundID); err != nil {
		t.Fatalf("failed to insert refund: %v", err)
	}

	_, err = env.repository.UpdateStatus(context.Background(), rr.ID, StatusRefunded, nil, nil)
	if err != ErrReturnInvalidTransition {
		t.Fatalf("expected ErrReturnInvalidTransition with only a pending refund, got %v", err)
	}

	// Now mark the refund as succeeded (simulating the refund package's
	// authoritative completion) and confirm the transition is now allowed.
	if _, err := env.db.Exec(context.Background(), `UPDATE refunds SET status = 'succeeded' WHERE id = $1`, refundID); err != nil {
		t.Fatalf("failed to mark refund succeeded: %v", err)
	}

	rr, err = env.repository.UpdateStatus(context.Background(), rr.ID, StatusRefunded, nil, nil)
	if err != nil {
		t.Fatalf("expected transition to refunded to succeed once refund succeeded, got %v", err)
	}
	if rr.Status != StatusRefunded {
		t.Errorf("expected status %q, got %q", StatusRefunded, rr.Status)
	}
}
