package order

import (
	"context"
	"errors"
	"fmt"
	"os"
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

	_, err := env.orderRepo.CreateFromCart(context.Background(), userID)
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

	o, err := env.orderRepo.CreateFromCart(context.Background(), userID)
	if err != nil {
		t.Fatalf("CreateFromCart failed: %v", err)
	}
	if o.UserID != userID {
		t.Errorf("expected user id %d, got %d", userID, o.UserID)
	}
	if o.Status != StatusPending {
		t.Errorf("expected status %q, got %q", StatusPending, o.Status)
	}
	wantTotal := p1.Price*2 + p2.Price*3
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

	if _, err := env.orderRepo.CreateFromCart(context.Background(), userID); err != nil {
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

	if _, err := env.orderRepo.CreateFromCart(context.Background(), userID); err != nil {
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

	_, err = env.orderRepo.CreateFromCart(context.Background(), userID)
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
	if _, err := env.orderRepo.CreateFromCart(context.Background(), userA); err != nil {
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
	o, err := env.orderRepo.CreateFromCart(context.Background(), owner)
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
