package upload

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newServeMux(t *testing.T, dir string) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /uploads/products/{key}", FileServer(dir))
	return mux
}

func TestFileServerServesExistingFile(t *testing.T) {
	dir := t.TempDir()
	key := strings.Repeat("a", 32) + ".jpg"
	if err := os.WriteFile(filepath.Join(dir, key), []byte("fake jpeg bytes"), 0o644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	mux := newServeMux(t, dir)
	req := httptest.NewRequest(http.MethodGet, "/uploads/products/"+key, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("expected Content-Type image/jpeg, got %q", got)
	}
	if w.Body.String() != "fake jpeg bytes" {
		t.Errorf("unexpected body: %q", w.Body.String())
	}
}

func TestFileServerReturns404ForMissingFile(t *testing.T) {
	dir := t.TempDir()
	mux := newServeMux(t, dir)

	key := strings.Repeat("b", 32) + ".png"
	req := httptest.NewRequest(http.MethodGet, "/uploads/products/"+key, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// TestFileServerRejectsArbitraryPaths is the core security guarantee for
// the public serving route: no path other than the exact generated-key
// shape may read anything from the filesystem, so directory traversal and
// arbitrary file reads (e.g. reaching outside the upload directory, or
// reading directories) must always be rejected.
func TestFileServerRejectsArbitraryPaths(t *testing.T) {
	dir := t.TempDir()

	// A secret file that must never be reachable through this route, one
	// directory above the served root.
	parent := filepath.Dir(dir)
	secretPath := filepath.Join(parent, "secret.txt")
	if err := os.WriteFile(secretPath, []byte("top secret"), 0o644); err != nil {
		t.Fatalf("failed to write secret fixture: %v", err)
	}
	defer os.Remove(secretPath)

	mux := newServeMux(t, dir)

	requestPaths := []string{
		"/uploads/products/..%2f..%2fsecret.txt",
		"/uploads/products/..%5c..%5csecret.txt",
		"/uploads/products/notes.txt",
		"/uploads/products/" + strings.Repeat("a", 32),          // no extension
		"/uploads/products/" + strings.Repeat("a", 31) + ".jpg", // wrong length
		"/uploads/products/" + strings.Repeat("Z", 32) + ".jpg", // non-hex, uppercase
	}
	for _, path := range requestPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				t.Fatalf("expected request to be rejected, got 200 with body %q", w.Body.String())
			}
			if strings.Contains(w.Body.String(), "top secret") {
				t.Fatal("secret file content leaked through the file server")
			}
		})
	}
}

func TestFileServerRejectsDirectoryListing(t *testing.T) {
	dir := t.TempDir()
	key := strings.Repeat("c", 32) + ".webp"
	os.WriteFile(filepath.Join(dir, key), []byte("data"), 0o644)

	mux := newServeMux(t, dir)
	req := httptest.NewRequest(http.MethodGet, "/uploads/products/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// The mux pattern requires a {key} segment; a bare trailing slash
	// either 404s at the router level or is rejected by isSafeServeKey.
	// Either way, the file list must never be exposed.
	if w.Code == http.StatusOK && strings.Contains(w.Body.String(), key) {
		t.Fatal("directory listing was exposed")
	}
}

func TestFileServerSetsImmutableCacheHeaders(t *testing.T) {
	dir := t.TempDir()
	key := strings.Repeat("d", 32) + ".png"
	os.WriteFile(filepath.Join(dir, key), []byte("data"), 0o644)

	mux := newServeMux(t, dir)
	req := httptest.NewRequest(http.MethodGet, "/uploads/products/"+key, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	cc := w.Header().Get("Cache-Control")
	if !strings.Contains(cc, "immutable") {
		t.Errorf("expected immutable cache-control, got %q", cc)
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("expected nosniff, got %q", got)
	}
}

func TestFileServerRejectsNonGetMethods(t *testing.T) {
	dir := t.TempDir()
	key := strings.Repeat("e", 32) + ".jpg"
	os.WriteFile(filepath.Join(dir, key), []byte("data"), 0o644)

	handler := FileServer(dir)
	req := httptest.NewRequest(http.MethodPost, "/uploads/products/"+key, nil)
	req.SetPathValue("key", key)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}
