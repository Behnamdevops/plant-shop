package cart

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
	"github.com/Behnamdevops/plant-shop/backend/internal/product"
	"github.com/jackc/pgx/v5/pgxpool"
)

type testHandlerEnv struct {
	handler     *Handler
	authHandler *auth.Handler
	productRepo *product.Repository
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
	cartHandler := NewHandler(NewRepository(db), authHandler)
	return &testHandlerEnv{
		handler:     cartHandler,
		authHandler: authHandler,
		productRepo: product.NewRepository(db),
	}
}

var handlerSeq int

func handlerUniqueSuffix() string {
	handlerSeq++
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), handlerSeq)
}

// registerAndLogin creates a new user via the real auth handler and returns
// the session cookie so cart requests can authenticate the same way a real
// client would (never trusting a client-supplied user_id).
func (e *testHandlerEnv) registerAndLogin(t *testing.T) *http.Cookie {
	t.Helper()
	suffix := handlerUniqueSuffix()
	body, _ := json.Marshal(map[string]any{
		"name":     "cartuser-" + suffix,
		"email":    "cartuser-" + suffix + "@example.com",
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
		Name:  "Cart Product " + suffix,
		Slug:  "cart-product-" + suffix,
		Price: 150,
		Stock: stock,
	})
	if err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	return p
}

func TestHandlerGetCartUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
	w := httptest.NewRecorder()
	env.handler.GetCart(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerAddItemUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	body, _ := json.Marshal(map[string]any{"product_id": 1, "quantity": 1})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewReader(body))
	w := httptest.NewRecorder()
	env.handler.AddItem(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerUpdateItemUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	body, _ := json.Marshal(map[string]any{"quantity": 1})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/cart/items/1", bytes.NewReader(body))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	env.handler.UpdateItem(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerDeleteItemUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/cart/items/1", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	env.handler.DeleteItem(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerAddItemMissingProductID(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)

	body, _ := json.Marshal(map[string]any{"quantity": 1})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	env.handler.AddItem(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandlerAddItem(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	p := env.createProduct(t, 10)

	body, _ := json.Marshal(map[string]any{"product_id": p.ID, "quantity": 2})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	env.handler.AddItem(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusCreated, w.Body.String())
	}
	var item Item
	if err := json.NewDecoder(w.Body).Decode(&item); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if item.Quantity != 2 {
		t.Errorf("expected quantity 2, got %d", item.Quantity)
	}
}

func TestHandlerAddItemIncrementsExisting(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	p := env.createProduct(t, 10)

	body1, _ := json.Marshal(map[string]any{"product_id": p.ID, "quantity": 2})
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewReader(body1))
	req1.AddCookie(cookie)
	w1 := httptest.NewRecorder()
	env.handler.AddItem(w1, req1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first add: got status %d", w1.Code)
	}

	body2, _ := json.Marshal(map[string]any{"product_id": p.ID, "quantity": 3})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewReader(body2))
	req2.AddCookie(cookie)
	w2 := httptest.NewRecorder()
	env.handler.AddItem(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("second add: got status %d", w2.Code)
	}
	var item Item
	json.NewDecoder(w2.Body).Decode(&item)
	if item.Quantity != 5 {
		t.Errorf("expected quantity 5, got %d", item.Quantity)
	}
}

func TestHandlerAddItemInvalidQuantity(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	p := env.createProduct(t, 10)

	tests := []struct {
		name     string
		quantity any
	}{
		{"zero quantity", 0},
		{"negative quantity", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{"product_id": p.ID, "quantity": tt.quantity})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewReader(body))
			req.AddCookie(cookie)
			w := httptest.NewRecorder()
			env.handler.AddItem(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestHandlerAddItemProductNotFound(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)

	body, _ := json.Marshal(map[string]any{"product_id": 999999, "quantity": 1})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	env.handler.AddItem(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandlerAddItemInsufficientStock(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	p := env.createProduct(t, 2)

	body, _ := json.Marshal(map[string]any{"product_id": p.ID, "quantity": 5})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	env.handler.AddItem(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestHandlerUpdateItem(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	p := env.createProduct(t, 10)

	addBody, _ := json.Marshal(map[string]any{"product_id": p.ID, "quantity": 2})
	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewReader(addBody))
	addReq.AddCookie(cookie)
	addW := httptest.NewRecorder()
	env.handler.AddItem(addW, addReq)
	var created Item
	json.NewDecoder(addW.Body).Decode(&created)

	updateBody, _ := json.Marshal(map[string]any{"quantity": 6})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/cart/items/"+strconv.FormatInt(created.ID, 10), bytes.NewReader(updateBody))
	updateReq.SetPathValue("id", strconv.FormatInt(created.ID, 10))
	updateReq.AddCookie(cookie)
	updateW := httptest.NewRecorder()
	env.handler.UpdateItem(updateW, updateReq)
	if updateW.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", updateW.Code, http.StatusOK, updateW.Body.String())
	}
	var updated Item
	json.NewDecoder(updateW.Body).Decode(&updated)
	if updated.Quantity != 6 {
		t.Errorf("expected quantity 6, got %d", updated.Quantity)
	}
}

func TestHandlerUpdateItemInvalidQuantity(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)

	updateBody, _ := json.Marshal(map[string]any{"quantity": 0})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/cart/items/1", bytes.NewReader(updateBody))
	updateReq.SetPathValue("id", "1")
	updateReq.AddCookie(cookie)
	updateW := httptest.NewRecorder()
	env.handler.UpdateItem(updateW, updateReq)
	if updateW.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", updateW.Code, http.StatusBadRequest)
	}
}

func TestHandlerUpdateItemInsufficientStock(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	p := env.createProduct(t, 5)

	addBody, _ := json.Marshal(map[string]any{"product_id": p.ID, "quantity": 2})
	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewReader(addBody))
	addReq.AddCookie(cookie)
	addW := httptest.NewRecorder()
	env.handler.AddItem(addW, addReq)
	var created Item
	json.NewDecoder(addW.Body).Decode(&created)

	updateBody, _ := json.Marshal(map[string]any{"quantity": 100})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/cart/items/"+strconv.FormatInt(created.ID, 10), bytes.NewReader(updateBody))
	updateReq.SetPathValue("id", strconv.FormatInt(created.ID, 10))
	updateReq.AddCookie(cookie)
	updateW := httptest.NewRecorder()
	env.handler.UpdateItem(updateW, updateReq)
	if updateW.Code != http.StatusConflict {
		t.Fatalf("got status %d, want %d", updateW.Code, http.StatusConflict)
	}
}

func TestHandlerUpdateItemNotFound(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)

	updateBody, _ := json.Marshal(map[string]any{"quantity": 2})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/cart/items/999999", bytes.NewReader(updateBody))
	updateReq.SetPathValue("id", "999999")
	updateReq.AddCookie(cookie)
	updateW := httptest.NewRecorder()
	env.handler.UpdateItem(updateW, updateReq)
	if updateW.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", updateW.Code, http.StatusNotFound)
	}
}

func TestHandlerUpdateItemCrossUserProtection(t *testing.T) {
	env := newTestHandlerEnv(t)
	ownerCookie := env.registerAndLogin(t)
	attackerCookie := env.registerAndLogin(t)
	p := env.createProduct(t, 10)

	addBody, _ := json.Marshal(map[string]any{"product_id": p.ID, "quantity": 2})
	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewReader(addBody))
	addReq.AddCookie(ownerCookie)
	addW := httptest.NewRecorder()
	env.handler.AddItem(addW, addReq)
	var created Item
	json.NewDecoder(addW.Body).Decode(&created)

	updateBody, _ := json.Marshal(map[string]any{"quantity": 9})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/cart/items/"+strconv.FormatInt(created.ID, 10), bytes.NewReader(updateBody))
	updateReq.SetPathValue("id", strconv.FormatInt(created.ID, 10))
	updateReq.AddCookie(attackerCookie)
	updateW := httptest.NewRecorder()
	env.handler.UpdateItem(updateW, updateReq)
	if updateW.Code != http.StatusNotFound {
		t.Fatalf("expected cross-user update to be rejected with 404, got %d", updateW.Code)
	}
}

func TestHandlerDeleteItem(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	p := env.createProduct(t, 10)

	addBody, _ := json.Marshal(map[string]any{"product_id": p.ID, "quantity": 2})
	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewReader(addBody))
	addReq.AddCookie(cookie)
	addW := httptest.NewRecorder()
	env.handler.AddItem(addW, addReq)
	var created Item
	json.NewDecoder(addW.Body).Decode(&created)

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/cart/items/"+strconv.FormatInt(created.ID, 10), nil)
	delReq.SetPathValue("id", strconv.FormatInt(created.ID, 10))
	delReq.AddCookie(cookie)
	delW := httptest.NewRecorder()
	env.handler.DeleteItem(delW, delReq)
	if delW.Code != http.StatusNoContent {
		t.Fatalf("got status %d, want %d", delW.Code, http.StatusNoContent)
	}
}

func TestHandlerDeleteItemCrossUserProtection(t *testing.T) {
	env := newTestHandlerEnv(t)
	ownerCookie := env.registerAndLogin(t)
	attackerCookie := env.registerAndLogin(t)
	p := env.createProduct(t, 10)

	addBody, _ := json.Marshal(map[string]any{"product_id": p.ID, "quantity": 2})
	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewReader(addBody))
	addReq.AddCookie(ownerCookie)
	addW := httptest.NewRecorder()
	env.handler.AddItem(addW, addReq)
	var created Item
	json.NewDecoder(addW.Body).Decode(&created)

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/cart/items/"+strconv.FormatInt(created.ID, 10), nil)
	delReq.SetPathValue("id", strconv.FormatInt(created.ID, 10))
	delReq.AddCookie(attackerCookie)
	delW := httptest.NewRecorder()
	env.handler.DeleteItem(delW, delReq)
	if delW.Code != http.StatusNotFound {
		t.Fatalf("expected cross-user delete to be rejected with 404, got %d", delW.Code)
	}

	// Confirm the owner can still delete it.
	delReq2 := httptest.NewRequest(http.MethodDelete, "/api/v1/cart/items/"+strconv.FormatInt(created.ID, 10), nil)
	delReq2.SetPathValue("id", strconv.FormatInt(created.ID, 10))
	delReq2.AddCookie(ownerCookie)
	delW2 := httptest.NewRecorder()
	env.handler.DeleteItem(delW2, delReq2)
	if delW2.Code != http.StatusNoContent {
		t.Fatalf("expected owner delete to succeed, got %d", delW2.Code)
	}
}

func TestHandlerGetCart(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	p := env.createProduct(t, 10)

	addBody, _ := json.Marshal(map[string]any{"product_id": p.ID, "quantity": 2})
	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewReader(addBody))
	addReq.AddCookie(cookie)
	addW := httptest.NewRecorder()
	env.handler.AddItem(addW, addReq)
	if addW.Code != http.StatusCreated {
		t.Fatalf("setup add item failed: got %d", addW.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
	getReq.AddCookie(cookie)
	getW := httptest.NewRecorder()
	env.handler.GetCart(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", getW.Code, http.StatusOK)
	}
	var c Cart
	if err := json.NewDecoder(getW.Body).Decode(&c); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(c.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(c.Items))
	}
	if c.Items[0].ProductID != p.ID {
		t.Errorf("expected product id %d, got %d", p.ID, c.Items[0].ProductID)
	}
	wantSubtotal := p.Price * 2
	if c.Items[0].Subtotal != wantSubtotal {
		t.Errorf("expected subtotal %d, got %d", wantSubtotal, c.Items[0].Subtotal)
	}
	if c.Total != wantSubtotal {
		t.Errorf("expected total %d, got %d", wantSubtotal, c.Total)
	}
}
