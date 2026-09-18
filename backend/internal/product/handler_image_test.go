package product

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/Behnamdevops/plant-shop/backend/internal/storage"
)

// fakeImageCleaner records Delete calls so tests can assert exactly which
// keys were (or were not) cleaned up, without needing a real filesystem.
type fakeImageCleaner struct {
	mu       sync.Mutex
	prefix   string
	deleted  []string
	failNext bool
}

func (f *fakeImageCleaner) KeyFromURL(url string) (string, bool) {
	if len(url) > len(f.prefix) && url[:len(f.prefix)] == f.prefix {
		return url[len(f.prefix):], true
	}
	return "", false
}

func (f *fakeImageCleaner) Delete(ctx context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, key)
	return nil
}

func (f *fakeImageCleaner) deletedKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.deleted))
	copy(out, f.deleted)
	return out
}

func newTestHandlerEnvWithImages(t *testing.T) (*testHandlerEnv, *fakeImageCleaner) {
	t.Helper()
	env := newTestHandlerEnv(t)
	cleaner := &fakeImageCleaner{prefix: "/uploads/products/"}
	env.handler = NewHandler(env.handler.repository, env.authHandler, cleaner)
	return env, cleaner
}

func updateRequest(t *testing.T, id int64, payload map[string]any) *http.Request {
	t.Helper()
	body, _ := json.Marshal(payload)
	idStr := strconv.FormatInt(id, 10)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/products/"+idStr, bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// TestUpdateReplacesLocalImageCleansUpOldOne covers: replacing a locally
// managed image on update deletes the previous one after the DB write
// succeeds.
func TestUpdateReplacesLocalImageCleansUpOldOne(t *testing.T) {
	env, cleaner := newTestHandlerEnvWithImages(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)

	oldURL := "/uploads/products/" + oldKeyForTest()
	p := env.createProduct(t, "img-replace-"+handlerUniqueSuffix())
	if _, err := env.handler.repository.Update(t.Context(), p.ID, UpdateProductInput{
		Name: &p.Name, Slug: &p.Slug, Description: strPtr(""), Price: &p.Price, Stock: &p.Stock, ImageURL: &oldURL,
	}); err != nil {
		t.Fatalf("setup: failed to seed old image url: %v", err)
	}

	newURL := "/uploads/products/" + newKeyForTest()
	req := updateRequest(t, p.ID, map[string]any{
		"name": p.Name, "slug": p.Slug, "description": "d", "price": p.Price, "stock": p.Stock, "image_url": newURL,
	})
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	deleted := cleaner.deletedKeys()
	if len(deleted) != 1 || deleted[0] != oldKeyForTest() {
		t.Fatalf("expected old image key %q to be cleaned up exactly once, got %v", oldKeyForTest(), deleted)
	}
}

// TestUpdateKeepingSameImageDoesNotDeleteIt covers: saving a product
// without changing its image must never delete the file it still needs.
func TestUpdateKeepingSameImageDoesNotDeleteIt(t *testing.T) {
	env, cleaner := newTestHandlerEnvWithImages(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)

	sameURL := "/uploads/products/" + oldKeyForTest()
	p := env.createProduct(t, "img-keep-"+handlerUniqueSuffix())
	if _, err := env.handler.repository.Update(t.Context(), p.ID, UpdateProductInput{
		Name: &p.Name, Slug: &p.Slug, Description: strPtr(""), Price: &p.Price, Stock: &p.Stock, ImageURL: &sameURL,
	}); err != nil {
		t.Fatalf("setup: failed to seed image url: %v", err)
	}

	req := updateRequest(t, p.ID, map[string]any{
		"name": "Renamed", "slug": p.Slug, "description": "d", "price": p.Price, "stock": p.Stock, "image_url": sameURL,
	})
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if deleted := cleaner.deletedKeys(); len(deleted) != 0 {
		t.Fatalf("expected no cleanup when image is unchanged, got %v", deleted)
	}
}

// TestUpdateExternalImageURLNeverDeleted covers: an externally hosted
// image_url must never be passed to Delete, even when replaced.
func TestUpdateExternalImageURLNeverDeleted(t *testing.T) {
	env, cleaner := newTestHandlerEnvWithImages(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)

	externalURL := "https://cdn.example.com/plants/rose.jpg"
	p := env.createProduct(t, "img-external-"+handlerUniqueSuffix())
	if _, err := env.handler.repository.Update(t.Context(), p.ID, UpdateProductInput{
		Name: &p.Name, Slug: &p.Slug, Description: strPtr(""), Price: &p.Price, Stock: &p.Stock, ImageURL: &externalURL,
	}); err != nil {
		t.Fatalf("setup: failed to seed external image url: %v", err)
	}

	newURL := "/uploads/products/" + newKeyForTest()
	req := updateRequest(t, p.ID, map[string]any{
		"name": p.Name, "slug": p.Slug, "description": "d", "price": p.Price, "stock": p.Stock, "image_url": newURL,
	})
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if deleted := cleaner.deletedKeys(); len(deleted) != 0 {
		t.Fatalf("expected external URL to never be deleted, got %v", deleted)
	}
}

// TestUpdateFailureDoesNotDeleteImage covers: if the update fails (e.g.
// duplicate slug), the previous image must remain intact — cleanup must
// never run before the database outcome is known.
func TestUpdateFailureDoesNotDeleteImage(t *testing.T) {
	env, cleaner := newTestHandlerEnvWithImages(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)

	imageURL := "/uploads/products/" + oldKeyForTest()
	p1 := env.createProduct(t, "img-fail-a-"+handlerUniqueSuffix())
	p2 := env.createProduct(t, "img-fail-b-"+handlerUniqueSuffix())
	if _, err := env.handler.repository.Update(t.Context(), p1.ID, UpdateProductInput{
		Name: &p1.Name, Slug: &p1.Slug, Description: strPtr(""), Price: &p1.Price, Stock: &p1.Stock, ImageURL: &imageURL,
	}); err != nil {
		t.Fatalf("setup: failed to seed image url: %v", err)
	}

	// Attempt to rename p1 to p2's slug — must fail with a conflict, and
	// p1's image must remain untouched.
	req := updateRequest(t, p1.ID, map[string]any{
		"name": p1.Name, "slug": p2.Slug, "description": "d", "price": p1.Price, "stock": p1.Stock, "image_url": "/uploads/products/" + newKeyForTest(),
	})
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Update(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate slug, got %d: %s", w.Code, w.Body.String())
	}
	if deleted := cleaner.deletedKeys(); len(deleted) != 0 {
		t.Fatalf("expected no cleanup on failed update, got %v", deleted)
	}
}

// TestDeleteProductCleansUpLocalImage covers: successful product deletion
// best-effort cleans up its local image.
func TestDeleteProductCleansUpLocalImage(t *testing.T) {
	env, cleaner := newTestHandlerEnvWithImages(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)

	imageURL := "/uploads/products/" + oldKeyForTest()
	p := env.createProduct(t, "img-del-"+handlerUniqueSuffix())
	if _, err := env.handler.repository.Update(t.Context(), p.ID, UpdateProductInput{
		Name: &p.Name, Slug: &p.Slug, Description: strPtr(""), Price: &p.Price, Stock: &p.Stock, ImageURL: &imageURL,
	}); err != nil {
		t.Fatalf("setup: failed to seed image url: %v", err)
	}

	idStr := strconv.FormatInt(p.ID, 10)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/products/"+idStr, nil)
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	deleted := cleaner.deletedKeys()
	if len(deleted) != 1 || deleted[0] != oldKeyForTest() {
		t.Fatalf("expected image to be cleaned up after successful delete, got %v", deleted)
	}
}

// TestDeleteBlockedByOrderReferenceKeepsImage covers: when deletion is
// blocked because the product is referenced by existing orders, the image
// must remain intact (Delete must never be called on the cleaner).
func TestDeleteBlockedByOrderReferenceKeepsImage(t *testing.T) {
	env, cleaner := newTestHandlerEnvWithImages(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)
	buyer := env.registerAndLogin(t, auth.RoleUser)

	imageURL := "/uploads/products/" + oldKeyForTest()
	p := env.createProduct(t, "img-del-blocked-"+handlerUniqueSuffix())
	if _, err := env.handler.repository.Update(t.Context(), p.ID, UpdateProductInput{
		Name: &p.Name, Slug: &p.Slug, Description: strPtr(""), Price: &p.Price, Stock: &p.Stock, ImageURL: &imageURL,
	}); err != nil {
		t.Fatalf("setup: failed to seed image url: %v", err)
	}

	buyerTokenHash := sha256HashForTest(buyer.Value)
	var userID int64
	if err := env.db.QueryRow(t.Context(), `SELECT user_id FROM sessions WHERE token_hash = $1`, buyerTokenHash).Scan(&userID); err != nil {
		t.Fatalf("failed to look up buyer id: %v", err)
	}
	var orderID int64
	if err := env.db.QueryRow(t.Context(), `
		INSERT INTO orders (user_id, status, total) VALUES ($1, 'pending', $2) RETURNING id
	`, userID, p.Price).Scan(&orderID); err != nil {
		t.Fatalf("failed to seed order: %v", err)
	}
	t.Cleanup(func() {
		env.db.Exec(context.Background(), `DELETE FROM orders WHERE id = $1`, orderID)
	})
	if _, err := env.db.Exec(t.Context(), `
		INSERT INTO order_items (order_id, product_id, product_name, product_slug, unit_price, quantity, subtotal)
		VALUES ($1, $2, $3, $4, $5, 1, $5)
	`, orderID, p.ID, p.Name, p.Slug, p.Price); err != nil {
		t.Fatalf("failed to seed order item: %v", err)
	}

	idStr := strconv.FormatInt(p.ID, 10)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/products/"+idStr, nil)
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Delete(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
	if deleted := cleaner.deletedKeys(); len(deleted) != 0 {
		t.Fatalf("expected image to remain intact when delete is blocked, got %v", deleted)
	}
}

// TestCreateProductWithUploadedImageURL and companions cover product/image
// compatibility: an uploaded URL, an external legacy URL, and no image at
// all must all be storable and round-trip correctly.
func TestCreateProductWithUploadedImageURL(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)

	slug := "img-create-uploaded-" + handlerUniqueSuffix()
	uploadedURL := "/uploads/products/" + newKeyForTest()
	body, _ := json.Marshal(map[string]any{
		"name": "Uploaded Image Plant", "slug": slug, "price": 100, "stock": 1, "image_url": uploadedURL,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created Product
	json.Unmarshal(w.Body.Bytes(), &created)
	t.Cleanup(func() { env.db.Exec(context.Background(), "DELETE FROM products WHERE id = $1", created.ID) })

	if created.ImageURL == nil || *created.ImageURL != uploadedURL {
		t.Fatalf("expected image_url %q, got %v", uploadedURL, created.ImageURL)
	}
}

func TestCreateProductWithExternalLegacyImageURL(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)

	slug := "img-create-external-" + handlerUniqueSuffix()
	externalURL := "https://cdn.example.com/legacy/plant.jpg"
	body, _ := json.Marshal(map[string]any{
		"name": "External Image Plant", "slug": slug, "price": 100, "stock": 1, "image_url": externalURL,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created Product
	json.Unmarshal(w.Body.Bytes(), &created)
	t.Cleanup(func() { env.db.Exec(context.Background(), "DELETE FROM products WHERE id = $1", created.ID) })

	if created.ImageURL == nil || *created.ImageURL != externalURL {
		t.Fatalf("expected external image_url %q, got %v", externalURL, created.ImageURL)
	}
}

func TestCreateProductWithNoImage(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)

	slug := "img-create-none-" + handlerUniqueSuffix()
	body, _ := json.Marshal(map[string]any{
		"name": "No Image Plant", "slug": slug, "price": 100, "stock": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created Product
	json.Unmarshal(w.Body.Bytes(), &created)
	t.Cleanup(func() { env.db.Exec(context.Background(), "DELETE FROM products WHERE id = $1", created.ID) })

	if created.ImageURL != nil {
		t.Fatalf("expected nil image_url, got %v", *created.ImageURL)
	}
}

// TestProductHandlerWithRealLocalStore is a small end-to-end sanity check
// that the production imageCleaner (storage.LocalStore) satisfies the
// handler's interface and genuinely deletes files on disk, using a
// temporary directory rather than the real dev uploads directory.
func TestProductHandlerWithRealLocalStore(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)

	store, err := storage.NewLocalStore(t.TempDir(), "/uploads/products")
	if err != nil {
		t.Fatalf("NewLocalStore failed: %v", err)
	}
	env.handler = NewHandler(env.handler.repository, env.authHandler, store)

	key, err := store.Save(t.Context(), ".jpg", bytes.NewReader([]byte("fake image data")))
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	oldURL := store.PublicURL(key)

	p := env.createProduct(t, "img-real-store-"+handlerUniqueSuffix())
	if _, err := env.handler.repository.Update(t.Context(), p.ID, UpdateProductInput{
		Name: &p.Name, Slug: &p.Slug, Description: strPtr(""), Price: &p.Price, Stock: &p.Stock, ImageURL: &oldURL,
	}); err != nil {
		t.Fatalf("setup: failed to seed image url: %v", err)
	}

	idStr := strconv.FormatInt(p.ID, 10)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/products/"+idStr, nil)
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	if err := store.Delete(t.Context(), key); err == nil {
		t.Fatal("expected file to already be deleted by product handler cleanup")
	}
}

func oldKeyForTest() string { return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg" }
func newKeyForTest() string { return "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.jpg" }
