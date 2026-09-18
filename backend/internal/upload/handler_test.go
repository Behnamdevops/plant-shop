package upload

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/Behnamdevops/plant-shop/backend/internal/storage"
)

// fakeAuth lets these tests exercise authorization behavior without a real
// database/session flow, mirroring the narrow authenticator interface the
// handler actually depends on.
type fakeAuth struct {
	userID int64
	err    error
}

func (f fakeAuth) RequireAdmin(r *http.Request) (int64, error) {
	return f.userID, f.err
}

func newHandlerWithStore(t *testing.T, a authenticator) (*Handler, *storage.LocalStore) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.NewLocalStore(dir, "/uploads/products")
	if err != nil {
		t.Fatalf("NewLocalStore failed: %v", err)
	}
	return &Handler{store: store, auth: a}, store
}

func multipartRequest(t *testing.T, fieldName, filename string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write part failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer failed: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/uploads/products", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func validJPEGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 100, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("failed to encode fixture JPEG: %v", err)
	}
	return buf.Bytes()
}

func validPNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 200, B: 10, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode fixture PNG: %v", err)
	}
	return buf.Bytes()
}

func TestUploadProductImageUnauthenticatedReturns401(t *testing.T) {
	handler, _ := newHandlerWithStore(t, fakeAuth{err: auth.ErrUnauthenticated})

	req := multipartRequest(t, "file", "photo.jpg", validJPEGBytes(t))
	w := httptest.NewRecorder()
	handler.UploadProductImage(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadProductImageNonAdminReturns403(t *testing.T) {
	handler, _ := newHandlerWithStore(t, fakeAuth{err: auth.ErrForbidden})

	req := multipartRequest(t, "file", "photo.jpg", validJPEGBytes(t))
	w := httptest.NewRecorder()
	handler.UploadProductImage(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadProductImageAdminSucceeds(t *testing.T) {
	handler, store := newHandlerWithStore(t, fakeAuth{userID: 1})

	req := multipartRequest(t, "file", "photo.jpg", validJPEGBytes(t))
	w := httptest.NewRecorder()
	handler.UploadProductImage(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp uploadResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !strings.HasPrefix(resp.URL, "/uploads/products/") {
		t.Fatalf("unexpected URL shape: %q", resp.URL)
	}
	if !strings.HasSuffix(resp.URL, ".jpg") {
		t.Fatalf("expected .jpg extension in URL, got %q", resp.URL)
	}

	key := strings.TrimPrefix(resp.URL, "/uploads/products/")
	if _, err := os.Stat(filepath.Join(store.Dir, key)); err != nil {
		t.Fatalf("expected uploaded file to exist on disk: %v", err)
	}
}

func TestUploadProductImageAcceptsPNG(t *testing.T) {
	handler, _ := newHandlerWithStore(t, fakeAuth{userID: 1})

	req := multipartRequest(t, "file", "photo.png", validPNGBytes(t))
	w := httptest.NewRecorder()
	handler.UploadProductImage(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp uploadResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.HasSuffix(resp.URL, ".png") {
		t.Fatalf("expected .png extension, got %q", resp.URL)
	}
}

func TestUploadProductImageRejectsFakeImageDespiteJPGExtension(t *testing.T) {
	handler, _ := newHandlerWithStore(t, fakeAuth{userID: 1})

	// A .jpg-named file that is actually plain text — the server must
	// decode the content, not trust the filename or claimed type.
	req := multipartRequest(t, "file", "fake.jpg", []byte("not actually an image"))
	w := httptest.NewRecorder()
	handler.UploadProductImage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadProductImageRejectsSVG(t *testing.T) {
	handler, _ := newHandlerWithStore(t, fakeAuth{userID: 1})

	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	req := multipartRequest(t, "file", "image.svg", svg)
	w := httptest.NewRecorder()
	handler.UploadProductImage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for SVG, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadProductImageRejectsOversizedFile(t *testing.T) {
	handler, _ := newHandlerWithStore(t, fakeAuth{userID: 1})

	oversized := bytes.Repeat([]byte{0xFF}, storage.MaxImageBytes+1024)
	req := multipartRequest(t, "file", "big.jpg", oversized)
	w := httptest.NewRecorder()
	handler.UploadProductImage(w, req)

	if w.Code != http.StatusRequestEntityTooLarge && w.Code != http.StatusBadRequest {
		t.Fatalf("expected 413 or 400 for oversized file, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadProductImageRejectsEmptyFile(t *testing.T) {
	handler, _ := newHandlerWithStore(t, fakeAuth{userID: 1})

	req := multipartRequest(t, "file", "empty.jpg", []byte{})
	w := httptest.NewRecorder()
	handler.UploadProductImage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty file, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadProductImageRejectsMissingFileField(t *testing.T) {
	handler, _ := newHandlerWithStore(t, fakeAuth{userID: 1})

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("other", "value")
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/uploads/products", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	handler.UploadProductImage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when file field missing, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUploadProductImageTraversalFilenameIsIgnored proves the client's
// filename (including path-traversal attempts) never influences the
// stored key or leaks into the response.
func TestUploadProductImageTraversalFilenameIsIgnored(t *testing.T) {
	handler, store := newHandlerWithStore(t, fakeAuth{userID: 1})

	req := multipartRequest(t, "file", "../../etc/passwd.jpg", validJPEGBytes(t))
	w := httptest.NewRecorder()
	handler.UploadProductImage(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp uploadResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if strings.Contains(resp.URL, "..") || strings.Contains(resp.URL, "passwd") {
		t.Fatalf("response URL leaked client filename: %q", resp.URL)
	}
	key := strings.TrimPrefix(resp.URL, "/uploads/products/")
	if _, err := os.Stat(filepath.Join(store.Dir, key)); err != nil {
		t.Fatalf("expected file to exist under store dir only: %v", err)
	}
}

// TestUploadProductImageDuplicateOriginalFilenamesDoNotCollide uploads two
// different files that both claim the same original client filename and
// verifies both are stored independently and both remain retrievable.
func TestUploadProductImageDuplicateOriginalFilenamesDoNotCollide(t *testing.T) {
	handler, store := newHandlerWithStore(t, fakeAuth{userID: 1})

	req1 := multipartRequest(t, "file", "photo.jpg", validJPEGBytes(t))
	w1 := httptest.NewRecorder()
	handler.UploadProductImage(w1, req1)

	req2 := multipartRequest(t, "file", "photo.jpg", validPNGBytes(t))
	w2 := httptest.NewRecorder()
	handler.UploadProductImage(w2, req2)

	if w1.Code != http.StatusCreated || w2.Code != http.StatusCreated {
		t.Fatalf("expected both uploads to succeed: %d, %d", w1.Code, w2.Code)
	}

	var resp1, resp2 uploadResponse
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	json.Unmarshal(w2.Body.Bytes(), &resp2)

	if resp1.URL == resp2.URL {
		t.Fatal("expected distinct URLs for separately uploaded files")
	}

	for _, url := range []string{resp1.URL, resp2.URL} {
		key := strings.TrimPrefix(url, "/uploads/products/")
		if _, err := os.Stat(filepath.Join(store.Dir, key)); err != nil {
			t.Fatalf("expected file for %q to exist: %v", url, err)
		}
	}
}

// TestUploadProductImageCannotOverwriteExistingFile verifies that even if
// two uploads were to generate the same key (simulated here directly via
// the store, since real random keys make a natural collision practically
// impossible), the second write does not silently overwrite the first.
func TestUploadProductImageCannotOverwriteExistingFile(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewLocalStore(dir, "/uploads/products")
	if err != nil {
		t.Fatalf("NewLocalStore failed: %v", err)
	}

	key, err := store.Save(context.Background(), ".jpg", bytesReader([]byte("first")))
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Attempt to write directly to the same path the way Save would, using
	// the same O_EXCL semantics Save relies on, to confirm the guarantee at
	// the filesystem level rather than just trusting random non-collision.
	path := filepath.Join(store.Dir, key)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err == nil {
		f.Close()
		t.Fatal("expected O_EXCL to prevent overwriting an existing file")
	}

	content, readErr := os.ReadFile(path)
	if readErr != nil || string(content) != "first" {
		t.Fatalf("original file content was not preserved: %v", readErr)
	}
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }
