package order

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/Behnamdevops/plant-shop/backend/internal/cart"
	"github.com/Behnamdevops/plant-shop/backend/internal/product"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testEnv bundles the repositories needed to set up fixtures (users,
// products, cart items) for order repository tests.
type testEnv struct {
	orderRepo   *Repository
	cartRepo    *cart.Repository
	authRepo    *auth.Repository
	productRepo *product.Repository
	db          *pgxpool.Pool
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	db, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Skip("cannot connect to database; skipping integration test: ", err)
	}
	return &testEnv{
		orderRepo:   NewRepository(db),
		cartRepo:    cart.NewRepository(db),
		authRepo:    auth.NewRepository(db),
		productRepo: product.NewRepository(db),
		db:          db,
	}
}

var seq int

func uniqueSuffix() string {
	seq++
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), seq)
}

func (e *testEnv) createUser(t *testing.T) int64 {
	t.Helper()
	suffix := uniqueSuffix()
	u, err := e.authRepo.CreateUser(context.Background(), "user-"+suffix, "user-"+suffix+"@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	return u.ID
}

func (e *testEnv) createProduct(t *testing.T, stock int) product.Product {
	t.Helper()
	suffix := uniqueSuffix()
	p, err := e.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Product " + suffix,
		Slug:  "product-" + suffix,
		Price: 100,
		Stock: stock,
	})
	if err != nil {
		t.Fatalf("Create product failed: %v", err)
	}
	return p
}

// validCheckoutInput returns a CheckoutInput that passes Validate(), for
// tests that only care about the cart/stock/status behavior of
// CreateFromCart and not about delivery/shipping field validation itself.
func validCheckoutInput() CheckoutInput {
	return CheckoutInput{
		RecipientName:  "Test Recipient",
		Phone:          "+1 555 0100",
		AddressLine1:   "123 Greenhouse Ave",
		City:           "Plantville",
		PostalCode:     "12345",
		Country:        "Testland",
		ShippingMethod: ShippingMethodStandard,
	}
}

func (e *testEnv) getStock(t *testing.T, productID int64) int {
	t.Helper()
	var stock int
	err := e.db.QueryRow(context.Background(), `SELECT stock FROM products WHERE id = $1`, productID).Scan(&stock)
	if err != nil {
		t.Fatalf("query stock failed: %v", err)
	}
	return stock
}

func TestRepositoryCreateFromCartEmptyCart(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)

	_, err := env.orderRepo.CreateFromCart(context.Background(), userID, validCheckoutInput())
	if !errors.Is(err, ErrEmptyCart) {
		t.Fatalf("expected ErrEmptyCart, got %v", err)
	}
}

func TestRepositoryCreateFromCartSuccess(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	p1 := env.createProduct(t, 10)
	p2 := env.createProduct(t, 5)

	if _, err := env.cartRepo.AddItem(context.Background(), userID, p1.ID, 2); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	if _, err := env.cartRepo.AddItem(context.Background(), userID, p2.ID, 3); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	o, err := env.orderRepo.CreateFromCart(context.Background(), userID, validCheckoutInput())
	if err != nil {
		t.Fatalf("CreateFromCart failed: %v", err)
	}
	if o.UserID != userID {
		t.Errorf("expected user id %d, got %d", userID, o.UserID)
	}
	if o.Status != StatusPending {
		t.Errorf("expected status %q, got %q", StatusPending, o.Status)
	}
	wantSubtotal := p1.Price*2 + p2.Price*3
	wantTotal := wantSubtotal + ShippingFeeStandard
	if o.ItemsSubtotal != wantSubtotal {
		t.Errorf("expected items_subtotal %d, got %d", wantSubtotal, o.ItemsSubtotal)
	}
	if o.ShippingFee != ShippingFeeStandard {
		t.Errorf("expected shipping_fee %d, got %d", ShippingFeeStandard, o.ShippingFee)
	}
	if o.Total != wantTotal {
		t.Errorf("expected total %d, got %d", wantTotal, o.Total)
	}

	full, err := env.orderRepo.GetByIDForUser(context.Background(), userID, o.ID)
	if err != nil {
		t.Fatalf("GetByIDForUser failed: %v", err)
	}
	if len(full.Items) != 2 {
		t.Fatalf("expected 2 order items, got %d", len(full.Items))
	}
	for _, it := range full.Items {
		if it.Subtotal != it.UnitPrice*int64(it.Quantity) {
			t.Errorf("subtotal mismatch for item %d", it.ID)
		}
		if it.ProductName == "" || it.ProductSlug == "" {
			t.Errorf("expected snapshot name/slug to be populated for item %d", it.ID)
		}
	}
}

func TestRepositoryCreateFromCartDecrementsStock(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	p := env.createProduct(t, 10)

	if _, err := env.cartRepo.AddItem(context.Background(), userID, p.ID, 4); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	if _, err := env.orderRepo.CreateFromCart(context.Background(), userID, validCheckoutInput()); err != nil {
		t.Fatalf("CreateFromCart failed: %v", err)
	}

	stock := env.getStock(t, p.ID)
	if stock != 6 {
		t.Errorf("expected stock 6 after checkout, got %d", stock)
	}
}

func TestRepositoryCreateFromCartClearsCart(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	p := env.createProduct(t, 10)

	if _, err := env.cartRepo.AddItem(context.Background(), userID, p.ID, 2); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	if _, err := env.orderRepo.CreateFromCart(context.Background(), userID, validCheckoutInput()); err != nil {
		t.Fatalf("CreateFromCart failed: %v", err)
	}

	c, err := env.cartRepo.GetCart(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetCart failed: %v", err)
	}
	if len(c.Items) != 0 {
		t.Fatalf("expected cart to be cleared, got %d items", len(c.Items))
	}
}

func TestRepositoryCreateFromCartInsufficientStockRollsBack(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	pOK := env.createProduct(t, 10)
	pLow := env.createProduct(t, 1)

	if _, err := env.cartRepo.AddItem(context.Background(), userID, pOK.ID, 2); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	// Add a second item and then reduce stock behind the cart's back to
	// simulate a race where the cart quantity now exceeds stock.
	if _, err := env.cartRepo.AddItem(context.Background(), userID, pLow.ID, 1); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	_, err := env.db.Exec(context.Background(), `UPDATE products SET stock = 0 WHERE id = $1`, pLow.ID)
	if err != nil {
		t.Fatalf("failed to force low stock: %v", err)
	}

	_, err = env.orderRepo.CreateFromCart(context.Background(), userID, validCheckoutInput())
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}

	// Nothing should have been created or changed.
	orders, err := env.orderRepo.ListByUser(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListByUser failed: %v", err)
	}
	if len(orders) != 0 {
		t.Fatalf("expected no orders to be created, got %d", len(orders))
	}

	stock := env.getStock(t, pOK.ID)
	if stock != 10 {
		t.Errorf("expected untouched stock 10, got %d", stock)
	}

	c, err := env.cartRepo.GetCart(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetCart failed: %v", err)
	}
	if len(c.Items) != 2 {
		t.Fatalf("expected cart to remain intact with 2 items, got %d", len(c.Items))
	}
}

func TestRepositoryListByUserIsolatedPerUser(t *testing.T) {
	env := newTestEnv(t)
	userA := env.createUser(t)
	userB := env.createUser(t)
	p := env.createProduct(t, 10)

	if _, err := env.cartRepo.AddItem(context.Background(), userA, p.ID, 1); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	if _, err := env.orderRepo.CreateFromCart(context.Background(), userA, validCheckoutInput()); err != nil {
		t.Fatalf("CreateFromCart failed: %v", err)
	}

	ordersB, err := env.orderRepo.ListByUser(context.Background(), userB)
	if err != nil {
		t.Fatalf("ListByUser failed: %v", err)
	}
	if len(ordersB) != 0 {
		t.Fatalf("expected userB to have 0 orders, got %d", len(ordersB))
	}

	ordersA, err := env.orderRepo.ListByUser(context.Background(), userA)
	if err != nil {
		t.Fatalf("ListByUser failed: %v", err)
	}
	if len(ordersA) != 1 {
		t.Fatalf("expected userA to have 1 order, got %d", len(ordersA))
	}
}

func TestRepositoryGetByIDForUserCrossUserProtection(t *testing.T) {
	env := newTestEnv(t)
	owner := env.createUser(t)
	attacker := env.createUser(t)
	p := env.createProduct(t, 10)

	if _, err := env.cartRepo.AddItem(context.Background(), owner, p.ID, 1); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	o, err := env.orderRepo.CreateFromCart(context.Background(), owner, validCheckoutInput())
	if err != nil {
		t.Fatalf("CreateFromCart failed: %v", err)
	}

	_, err = env.orderRepo.GetByIDForUser(context.Background(), attacker, o.ID)
	if !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("expected ErrOrderNotFound for cross-user access, got %v", err)
	}
}

func TestRepositoryGetByIDForUserNotFound(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)

	_, err := env.orderRepo.GetByIDForUser(context.Background(), userID, 999999)
	if !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("expected ErrOrderNotFound, got %v", err)
	}
}

func TestRepositoryCreateFromCartStoresDeliverySnapshot(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	p := env.createProduct(t, 10)
	if _, err := env.cartRepo.AddItem(context.Background(), userID, p.ID, 1); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	input := CheckoutInput{
		RecipientName:  "Jane Doe",
		Phone:          "+1 555 0199",
		AddressLine1:   "42 Fern Street",
		AddressLine2:   "Unit 7",
		City:           "Rootford",
		PostalCode:     "98765",
		Country:        "Leafland",
		ShippingMethod: ShippingMethodExpress,
	}

	o, err := env.orderRepo.CreateFromCart(context.Background(), userID, input)
	if err != nil {
		t.Fatalf("CreateFromCart failed: %v", err)
	}

	check := func(name string, got *string, want string) {
		if got == nil || *got != want {
			t.Errorf("expected %s %q, got %v", name, want, got)
		}
	}
	check("recipient_name", o.RecipientName, input.RecipientName)
	check("phone", o.Phone, input.Phone)
	check("address_line1", o.AddressLine1, input.AddressLine1)
	check("address_line2", o.AddressLine2, input.AddressLine2)
	check("city", o.City, input.City)
	check("postal_code", o.PostalCode, input.PostalCode)
	check("country", o.Country, input.Country)

	if o.ShippingMethod != ShippingMethodExpress {
		t.Errorf("expected shipping_method %q, got %q", ShippingMethodExpress, o.ShippingMethod)
	}
	if o.ShippingFee != ShippingFeeExpress {
		t.Errorf("expected shipping_fee %d, got %d", ShippingFeeExpress, o.ShippingFee)
	}
	if o.PaymentStatus != PaymentStatusPending {
		t.Errorf("expected payment_status %q, got %q", PaymentStatusPending, o.PaymentStatus)
	}
	if o.PaymentMethod != PaymentMethodManual {
		t.Errorf("expected payment_method %q, got %q", PaymentMethodManual, o.PaymentMethod)
	}
}

func TestRepositoryCreateFromCartOptionalAddressLine2Absent(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	p := env.createProduct(t, 10)
	if _, err := env.cartRepo.AddItem(context.Background(), userID, p.ID, 1); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	input := validCheckoutInput()
	input.AddressLine2 = ""

	o, err := env.orderRepo.CreateFromCart(context.Background(), userID, input)
	if err != nil {
		t.Fatalf("CreateFromCart failed: %v", err)
	}
	if o.AddressLine2 != nil {
		t.Errorf("expected address_line2 to be nil when not provided, got %v", *o.AddressLine2)
	}
}

func TestCheckoutInputValidateRequiredFields(t *testing.T) {
	base := validCheckoutInput()

	cases := []struct {
		name   string
		mutate func(in *CheckoutInput)
	}{
		{"empty recipient_name", func(in *CheckoutInput) { in.RecipientName = "" }},
		{"whitespace recipient_name", func(in *CheckoutInput) { in.RecipientName = "   " }},
		{"empty phone", func(in *CheckoutInput) { in.Phone = "" }},
		{"empty address_line1", func(in *CheckoutInput) { in.AddressLine1 = "" }},
		{"empty city", func(in *CheckoutInput) { in.City = "" }},
		{"empty postal_code", func(in *CheckoutInput) { in.PostalCode = "" }},
		{"empty country", func(in *CheckoutInput) { in.Country = "" }},
		{"empty shipping_method", func(in *CheckoutInput) { in.ShippingMethod = "" }},
		{"invalid shipping_method", func(in *CheckoutInput) { in.ShippingMethod = "teleport" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := base
			tc.mutate(&in)
			if err := in.Validate(); err == nil {
				t.Errorf("expected validation error for case %q", tc.name)
			}
		})
	}
}

func TestCheckoutInputValidateAcceptsValidInput(t *testing.T) {
	if err := validCheckoutInput().Validate(); err != nil {
		t.Errorf("expected valid input to pass validation, got %v", err)
	}
}

func TestRepositoryPaymentCancellation(t *testing.T) {
	env := newTestEnv(t)
	defer env.db.Close()

	cases := []struct {
		name          string
		paymentStatus string
		attemptStatus string
		authority     bool
		blocked       bool
	}{
		{"no attempts", PaymentStatusPending, "", false, false},
		{"paid order", PaymentStatusPaid, "", false, true},
		{"refunded order", PaymentStatusRefunded, "", false, true},
		{"pending request", PaymentStatusPending, "pending", false, true},
		{"pending authority", PaymentStatusPending, "pending", true, true},
		{"paid attempt", PaymentStatusPending, "paid", true, true},
		{"reconciliation authority", PaymentStatusPending, "reconciliation", true, true},
		{"failed legacy authority", PaymentStatusFailed, "failed", true, true},
		{"failed request", PaymentStatusFailed, "failed", false, false},
	}

	for _, admin := range []bool{false, true} {
		for _, status := range []string{StatusPending, StatusProcessing} {
			for _, method := range []string{PaymentMethodManual, PaymentMethodZarinPal} {
				for _, tc := range cases {
					t.Run(fmt.Sprintf("admin=%t/%s/%s/%s", admin, status, method, tc.name), func(t *testing.T) {
						userID := env.createUser(t)
						p := env.createProduct(t, 10)
						if _, err := env.cartRepo.AddItem(t.Context(), userID, p.ID, 3); err != nil {
							t.Fatalf("AddItem failed: %v", err)
						}
						o, err := env.orderRepo.CreateFromCart(t.Context(), userID, validCheckoutInput())
						if err != nil {
							t.Fatalf("CreateFromCart failed: %v", err)
						}
						if _, err := env.db.Exec(t.Context(), `
							UPDATE orders SET status = $2, payment_status = $3, payment_method = $4 WHERE id = $1
						`, o.ID, status, tc.paymentStatus, method); err != nil {
							t.Fatalf("set order payment state: %v", err)
						}
						if tc.attemptStatus != "" {
							var authority *string
							var refID *int64
							var code *int
							if tc.authority {
								value := "authority-" + uniqueSuffix()
								authority = &value
							}
							if tc.attemptStatus == "paid" || tc.attemptStatus == "reconciliation" {
								id := o.ID
								refID = &id
								code = func() *int { v := 100; return &v }()
							}
							if _, err := env.db.Exec(t.Context(), `
								INSERT INTO payment_attempts (order_id, amount, status, authority, ref_id, provider_code, verified_at)
								VALUES ($1, $2, $3, $4, $5, $6,
									CASE WHEN $5::bigint IS NULL THEN NULL ELSE NOW() END)
							`, o.ID, o.Total, tc.attemptStatus, authority, refID, code); err != nil {
								t.Fatalf("insert payment attempt: %v", err)
							}
							if _, err := env.db.Exec(t.Context(), `
								INSERT INTO payment_attempts (order_id, amount, status) VALUES ($1, $2, 'failed')
							`, o.ID, o.Total); err != nil {
								t.Fatalf("insert later failed request: %v", err)
							}
						}

						cancel := func() error {
							if admin {
								_, err := env.orderRepo.UpdateStatus(t.Context(), o.ID, StatusCancelled)
								return err
							}
							_, err := env.orderRepo.CancelOwnOrder(t.Context(), userID, o.ID)
							return err
						}
						var wantErr error
						wantStatus, wantStock := StatusCancelled, 10
						if tc.blocked {
							wantErr = ErrOrderNotEligibleForCancel
							if admin {
								wantErr = ErrInvalidTransition
							}
							wantStatus, wantStock = status, 7
						}
						for attempt := 0; attempt < 2; attempt++ {
							err := cancel()
							if !errors.Is(err, wantErr) {
								t.Fatalf("cancel error = %v, want %v", err, wantErr)
							}
							wantRefundRequired := admin && tc.paymentStatus == PaymentStatusPaid
							if errors.Is(err, ErrRefundRequired) != wantRefundRequired {
								t.Fatalf("cancel error = %v, want refund required = %t", err, wantRefundRequired)
							}
							if wantRefundRequired && !strings.Contains(err.Error(), "refund or manual reconciliation required") {
								t.Fatalf("missing refund guidance: %v", err)
							}
							if !tc.blocked {
								wantErr = ErrInvalidTransition
							}
						}
						got, err := env.orderRepo.GetByIDForUser(t.Context(), userID, o.ID)
						if err != nil {
							t.Fatalf("GetByIDForUser failed: %v", err)
						}
						if got.Status != wantStatus || got.PaymentStatus != tc.paymentStatus || got.PaymentMethod != method {
							t.Fatalf("unexpected order state: status=%s payment_status=%s payment_method=%s", got.Status, got.PaymentStatus, got.PaymentMethod)
						}
						if stock := env.getStock(t, p.ID); stock != wantStock {
							t.Fatalf("stock = %d, want %d", stock, wantStock)
						}
					})
				}
			}
		}
	}
}

func TestRepositoryAdminCancellationTerminalStates(t *testing.T) {
	env := newTestEnv(t)
	defer env.db.Close()
	for _, status := range []string{StatusShipped, StatusDelivered, StatusCancelled} {
		for _, paymentStatus := range []string{PaymentStatusPending, PaymentStatusPaid} {
			t.Run(status+"/"+paymentStatus, func(t *testing.T) {
				userID := env.createUser(t)
				p := env.createProduct(t, 10)
				if _, err := env.cartRepo.AddItem(t.Context(), userID, p.ID, 3); err != nil {
					t.Fatal(err)
				}
				o, err := env.orderRepo.CreateFromCart(t.Context(), userID, validCheckoutInput())
				if err != nil {
					t.Fatal(err)
				}
				wantStock := 7
				if status == StatusCancelled {
					if _, err := env.orderRepo.UpdateStatus(t.Context(), o.ID, StatusCancelled); err != nil {
						t.Fatal(err)
					}
					wantStock = 10
				}
				if _, err := env.db.Exec(t.Context(), `UPDATE orders SET status = $2, payment_status = $3 WHERE id = $1`, o.ID, status, paymentStatus); err != nil {
					t.Fatal(err)
				}
				for attempt := 0; attempt < 2; attempt++ {
					_, err := env.orderRepo.UpdateStatus(t.Context(), o.ID, StatusCancelled)
					if !errors.Is(err, ErrInvalidTransition) || errors.Is(err, ErrRefundRequired) {
						t.Fatalf("terminal cancellation error = %v, want only ErrInvalidTransition", err)
					}
					got, err := env.orderRepo.GetByID(t.Context(), o.ID)
					if err != nil {
						t.Fatal(err)
					}
					if got.Status != status || got.PaymentStatus != paymentStatus {
						t.Fatalf("terminal order changed: %s/%s", got.Status, got.PaymentStatus)
					}
					if stock := env.getStock(t, p.ID); stock != wantStock {
						t.Fatalf("stock = %d, want %d", stock, wantStock)
					}
				}
			})
		}
	}
}

func TestCheckoutInputValidateRejectsFieldTooLong(t *testing.T) {
	in := validCheckoutInput()
	in.RecipientName = strings.Repeat("a", maxRecipientNameLen+1)
	if err := in.Validate(); err == nil {
		t.Error("expected validation error for overlong recipient_name")
	}
}
