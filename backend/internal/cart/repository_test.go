package cart

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/Behnamdevops/plant-shop/backend/internal/product"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testEnv bundles the repositories needed to set up fixtures (users,
// products) for cart repository tests.
type testEnv struct {
	cartRepo    *Repository
	authRepo    *auth.Repository
	productRepo *product.Repository
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
		cartRepo:    NewRepository(db),
		authRepo:    auth.NewRepository(db),
		productRepo: product.NewRepository(db),
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

func (e *testEnv) createProduct(t *testing.T, stock int) int64 {
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
	return p.ID
}

func TestRepositoryAddItem(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	productID := env.createProduct(t, 10)

	item, err := env.cartRepo.AddItem(context.Background(), userID, productID, 2)
	if err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	if item.Quantity != 2 {
		t.Errorf("expected quantity 2, got %d", item.Quantity)
	}
	if item.UserID != userID {
		t.Errorf("expected user id %d, got %d", userID, item.UserID)
	}
}

func TestRepositoryAddItemIncrementsExisting(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	productID := env.createProduct(t, 10)

	first, err := env.cartRepo.AddItem(context.Background(), userID, productID, 2)
	if err != nil {
		t.Fatalf("first AddItem failed: %v", err)
	}
	second, err := env.cartRepo.AddItem(context.Background(), userID, productID, 3)
	if err != nil {
		t.Fatalf("second AddItem failed: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("expected same cart item id, got %d and %d", first.ID, second.ID)
	}
	if second.Quantity != 5 {
		t.Errorf("expected quantity 5, got %d", second.Quantity)
	}
}

func TestRepositoryAddItemProductNotFound(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)

	_, err := env.cartRepo.AddItem(context.Background(), userID, 999999, 1)
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("expected ErrProductNotFound, got %v", err)
	}
}

func TestRepositoryAddItemInsufficientStock(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	productID := env.createProduct(t, 2)

	_, err := env.cartRepo.AddItem(context.Background(), userID, productID, 5)
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}
}

func TestRepositoryAddItemIncrementExceedsStock(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	productID := env.createProduct(t, 3)

	_, err := env.cartRepo.AddItem(context.Background(), userID, productID, 2)
	if err != nil {
		t.Fatalf("first AddItem failed: %v", err)
	}
	_, err = env.cartRepo.AddItem(context.Background(), userID, productID, 2)
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}
}

func TestRepositoryUpdateItemQuantity(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	productID := env.createProduct(t, 10)

	item, err := env.cartRepo.AddItem(context.Background(), userID, productID, 2)
	if err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	updated, err := env.cartRepo.UpdateItemQuantity(context.Background(), userID, item.ID, 7)
	if err != nil {
		t.Fatalf("UpdateItemQuantity failed: %v", err)
	}
	if updated.Quantity != 7 {
		t.Errorf("expected quantity 7, got %d", updated.Quantity)
	}
}

func TestRepositoryUpdateItemQuantityInsufficientStock(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	productID := env.createProduct(t, 5)

	item, err := env.cartRepo.AddItem(context.Background(), userID, productID, 2)
	if err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	_, err = env.cartRepo.UpdateItemQuantity(context.Background(), userID, item.ID, 100)
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}
}

func TestRepositoryUpdateItemQuantityNotFound(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)

	_, err := env.cartRepo.UpdateItemQuantity(context.Background(), userID, 999999, 1)
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}
}

func TestRepositoryUpdateItemQuantityCrossUserProtection(t *testing.T) {
	env := newTestEnv(t)
	ownerID := env.createUser(t)
	attackerID := env.createUser(t)
	productID := env.createProduct(t, 10)

	item, err := env.cartRepo.AddItem(context.Background(), ownerID, productID, 2)
	if err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	_, err = env.cartRepo.UpdateItemQuantity(context.Background(), attackerID, item.ID, 5)
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound for cross-user update, got %v", err)
	}
}

func TestRepositoryDeleteItem(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	productID := env.createProduct(t, 10)

	item, err := env.cartRepo.AddItem(context.Background(), userID, productID, 2)
	if err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	if err := env.cartRepo.DeleteItem(context.Background(), userID, item.ID); err != nil {
		t.Fatalf("DeleteItem failed: %v", err)
	}

	_, err = env.cartRepo.UpdateItemQuantity(context.Background(), userID, item.ID, 1)
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected item to be gone after delete, got %v", err)
	}
}

func TestRepositoryDeleteItemNotFound(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)

	err := env.cartRepo.DeleteItem(context.Background(), userID, 999999)
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}
}

func TestRepositoryDeleteItemCrossUserProtection(t *testing.T) {
	env := newTestEnv(t)
	ownerID := env.createUser(t)
	attackerID := env.createUser(t)
	productID := env.createProduct(t, 10)

	item, err := env.cartRepo.AddItem(context.Background(), ownerID, productID, 2)
	if err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	err = env.cartRepo.DeleteItem(context.Background(), attackerID, item.ID)
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound for cross-user delete, got %v", err)
	}

	// Item must still exist for the real owner.
	_, err = env.cartRepo.UpdateItemQuantity(context.Background(), ownerID, item.ID, 3)
	if err != nil {
		t.Fatalf("expected item to still exist for owner, got %v", err)
	}
}

func TestRepositoryGetCart(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	productA := env.createProduct(t, 10)
	productB := env.createProduct(t, 10)

	if _, err := env.cartRepo.AddItem(context.Background(), userID, productA, 2); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	if _, err := env.cartRepo.AddItem(context.Background(), userID, productB, 3); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	c, err := env.cartRepo.GetCart(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetCart failed: %v", err)
	}
	if len(c.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(c.Items))
	}
	wantTotal := int64(2*100 + 3*100)
	if c.Total != wantTotal {
		t.Errorf("expected total %d, got %d", wantTotal, c.Total)
	}
	for _, item := range c.Items {
		if item.Subtotal != item.Price*int64(item.Quantity) {
			t.Errorf("subtotal mismatch for item %d", item.ID)
		}
	}
}

func TestRepositoryGetCartEmpty(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)

	c, err := env.cartRepo.GetCart(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetCart failed: %v", err)
	}
	if len(c.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(c.Items))
	}
	if c.Total != 0 {
		t.Fatalf("expected total 0, got %d", c.Total)
	}
}

func TestRepositoryGetCartIsolatedPerUser(t *testing.T) {
	env := newTestEnv(t)
	userA := env.createUser(t)
	userB := env.createUser(t)
	productID := env.createProduct(t, 10)

	if _, err := env.cartRepo.AddItem(context.Background(), userA, productID, 2); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	cartB, err := env.cartRepo.GetCart(context.Background(), userB)
	if err != nil {
		t.Fatalf("GetCart failed: %v", err)
	}
	if len(cartB.Items) != 0 {
		t.Fatalf("expected userB cart to be empty, got %d items", len(cartB.Items))
	}
}
