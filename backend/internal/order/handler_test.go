package order

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/Behnamdevops/plant-shop/backend/internal/cart"
	"github.com/Behnamdevops/plant-shop/backend/internal/product"
	"github.com/jackc/pgx/v5/pgxpool"
)

type testHandlerEnv struct {
	handler     *Handler
	authHandler *auth.Handler
	productRepo *product.Repository
	cartHandler *cart.Handler
	db          *pgxpool.Pool
}

func newTestHandlerEnv(t *testing.T) *testHandlerEnv {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	db, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Skip("cannot connect to database; skipping integration test: ", err)
	}
	authHandler := auth.NewHandler(auth.NewRepository(db))
	orderHandler := NewHandler(NewRepository(db), authHandler)
	cartHandler := cart.NewHandler(cart.NewRepository(db), authHandler)
	return &testHandlerEnv{
		handler:     orderHandler,
		authHandler: authHandler,
		productRepo: product.NewRepository(db),
		cartHandler: cartHandler,
		db:          db,
	}
}

var handlerSeq int

func handlerUniqueSuffix() string {
	handlerSeq++
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), handlerSeq)
}

// registerAndLogin creates a new user via the real auth handler and returns
// the session cookie so order requests can authenticate the same way a real
// client would (never trusting a client-supplied user_id).
func (e *testHandlerEnv) registerAndLogin(t *testing.T) *http.Cookie {
	t.Helper()
	suffix := handlerUniqueSuffix()
	body, _ := json.Marshal(map[string]any{
		"name":     "orderuser-" + suffix,
		"email":    "orderuser-" + suffix + "@example.com",
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	e.authHandler.Register(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup registration failed: got %d", w.Code)
	}
	res := w.Result()
	for _, c := range res.Cookies() {
		if c.Name == "session_token" {
			return c
		}
	}
	t.Fatal("no session_token cookie set on registration")
	return nil
}

func (e *testHandlerEnv) createProduct(t *testing.T, stock int) product.Product {
	t.Helper()
	suffix := handlerUniqueSuffix()
	p, err := e.productRepo.Create(t.Context(), product.CreateProductInput{
		Name:  "Order Product " + suffix,
		Slug:  "order-product-" + suffix,
		Price: 200,
		Stock: stock,
	})
	if err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	return p
}

// addToCart adds quantity of productID to cookie's cart via the real cart
// handler, mirroring how a real checkout flow builds up a cart.
func (e *testHandlerEnv) addToCart(t *testing.T, cookie *http.Cookie, productID int64, quantity int) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"product_id": productID, "quantity": quantity})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	e.cartHandler.AddItem(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup addToCart failed: got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestHandlerCreateUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", nil)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerListUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	w := httptest.NewRecorder()
	env.handler.List(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerGetByIDUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/1", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	env.handler.GetByID(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerCreateEmptyCart(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandlerCreateSuccess(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	p := env.createProduct(t, 10)
	env.addToCart(t, cookie, p.ID, 3)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusCreated, w.Body.String())
	}

	var o Order
	if err := json.NewDecoder(w.Body).Decode(&o); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if o.Status != StatusPending {
		t.Errorf("expected status %q, got %q", StatusPending, o.Status)
	}
	wantTotal := p.Price * 3
	if o.Total != wantTotal {
		t.Errorf("expected total %d, got %d", wantTotal, o.Total)
	}

	// Cart should now be empty.
	getCartReq := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
	getCartReq.AddCookie(cookie)
	getCartW := httptest.NewRecorder()
	env.cartHandler.GetCart(getCartW, getCartReq)
	var c cart.Cart
	json.NewDecoder(getCartW.Body).Decode(&c)
	if len(c.Items) != 0 {
		t.Errorf("expected cart to be empty after order, got %d items", len(c.Items))
	}
}

func TestHandlerCreateInsufficientStock(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	p := env.createProduct(t, 5)
	env.addToCart(t, cookie, p.ID, 3)

	// Simulate a race: stock drops below the cart's requested quantity
	// after the item was added (e.g. another checkout consumed it).
	_, err := env.db.Exec(t.Context(), `UPDATE products SET stock = 1 WHERE id = $1`, p.ID)
	if err != nil {
		t.Fatalf("failed to force low stock: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusConflict, w.Body.String())
	}

	// Nothing should have been created, and the cart should remain intact.
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	listReq.AddCookie(cookie)
	listW := httptest.NewRecorder()
	env.handler.List(listW, listReq)
	var orders []Order
	json.NewDecoder(listW.Body).Decode(&orders)
	if len(orders) != 0 {
		t.Errorf("expected no orders to be created, got %d", len(orders))
	}

	getCartReq := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
	getCartReq.AddCookie(cookie)
	getCartW := httptest.NewRecorder()
	env.cartHandler.GetCart(getCartW, getCartReq)
	var c cart.Cart
	json.NewDecoder(getCartW.Body).Decode(&c)
	if len(c.Items) != 1 {
		t.Errorf("expected cart to still hold 1 item after failed checkout, got %d", len(c.Items))
	}
}

func TestHandlerListOnlyCurrentUser(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookieA := env.registerAndLogin(t)
	cookieB := env.registerAndLogin(t)
	p := env.createProduct(t, 10)

	env.addToCart(t, cookieA, p.ID, 1)
	createReqA := httptest.NewRequest(http.MethodPost, "/api/v1/orders", nil)
	createReqA.AddCookie(cookieA)
	createWA := httptest.NewRecorder()
	env.handler.Create(createWA, createReqA)
	if createWA.Code != http.StatusCreated {
		t.Fatalf("setup order for user A failed: got %d", createWA.Code)
	}

	listReqB := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	listReqB.AddCookie(cookieB)
	listWB := httptest.NewRecorder()
	env.handler.List(listWB, listReqB)
	if listWB.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", listWB.Code, http.StatusOK)
	}
	var ordersB []Order
	json.NewDecoder(listWB.Body).Decode(&ordersB)
	if len(ordersB) != 0 {
		t.Fatalf("expected user B to see 0 orders, got %d", len(ordersB))
	}

	listReqA := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	listReqA.AddCookie(cookieA)
	listWA := httptest.NewRecorder()
	env.handler.List(listWA, listReqA)
	var ordersA []Order
	json.NewDecoder(listWA.Body).Decode(&ordersA)
	if len(ordersA) != 1 {
		t.Fatalf("expected user A to see 1 order, got %d", len(ordersA))
	}
}

func TestHandlerGetByIDCrossUserReturns404(t *testing.T) {
	env := newTestHandlerEnv(t)
	owner := env.registerAndLogin(t)
	attacker := env.registerAndLogin(t)
	p := env.createProduct(t, 10)

	env.addToCart(t, owner, p.ID, 1)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/orders", nil)
	createReq.AddCookie(owner)
	createW := httptest.NewRecorder()
	env.handler.Create(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("setup order failed: got %d", createW.Code)
	}
	var created Order
	json.NewDecoder(createW.Body).Decode(&created)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+strconv.FormatInt(created.ID, 10), nil)
	getReq.SetPathValue("id", strconv.FormatInt(created.ID, 10))
	getReq.AddCookie(attacker)
	getW := httptest.NewRecorder()
	env.handler.GetByID(getW, getReq)
	if getW.Code != http.StatusNotFound {
		t.Fatalf("expected cross-user get to be rejected with 404, got %d", getW.Code)
	}

	// Confirm the owner can still fetch it, including its items.
	getReq2 := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+strconv.FormatInt(created.ID, 10), nil)
	getReq2.SetPathValue("id", strconv.FormatInt(created.ID, 10))
	getReq2.AddCookie(owner)
	getW2 := httptest.NewRecorder()
	env.handler.GetByID(getW2, getReq2)
	if getW2.Code != http.StatusOK {
		t.Fatalf("expected owner get to succeed, got %d", getW2.Code)
	}
	var full OrderWithItems
	json.NewDecoder(getW2.Body).Decode(&full)
	if len(full.Items) != 1 {
		t.Fatalf("expected 1 order item, got %d", len(full.Items))
	}
}

func TestHandlerGetByIDNotFound(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/999999", nil)
	req.SetPathValue("id", "999999")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	env.handler.GetByID(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}
