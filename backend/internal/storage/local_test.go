package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) *LocalStore {
	t.Helper()
	dir := t.TempDir()
	store, err := NewLocalStore(dir, "/uploads/products")
	if err != nil {
		t.Fatalf("NewLocalStore failed: %v", err)
	}
	return store
}

func TestLocalStoreSaveWritesFileAndReturnsResolvableURL(t *testing.T) {
	store := newTestStore(t)

	content := []byte("fake image bytes")
	key, err := store.Save(context.Background(), ".jpg", bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if !strings.HasSuffix(key, ".jpg") {
		t.Errorf("expected key to end with .jpg, got %q", key)
	}

	// File actually persisted on disk.
	path := filepath.Join(store.Dir, key)
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file to exist at %q: %v", path, err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("stored content mismatch")
	}

	// Returned URL resolves back to the same key.
	url := store.PublicURL(key)
	if url != "/uploads/products/"+key {
		t.Errorf("unexpected public URL: %q", url)
	}
	resolvedKey, ok := store.KeyFromURL(url)
	if !ok || resolvedKey != key {
		t.Errorf("expected KeyFromURL to resolve back to %q, got %q (ok=%v)", key, resolvedKey, ok)
	}
}

func TestLocalStoreSaveRejectsUnsupportedExtension(t *testing.T) {
	store := newTestStore(t)
	_, err := store.Save(context.Background(), ".exe", bytes.NewReader([]byte("x")))
	if err == nil {
		t.Fatal("expected error for unsupported extension")
	}
}

func TestLocalStoreGeneratedNamesDoNotCollideForDuplicateOriginalNames(t *testing.T) {
	store := newTestStore(t)

	// Simulates two uploads of files that originally had the identical
	// client-supplied filename ("photo.jpg") — the generated storage key
	// must never depend on that name, so collisions can't happen.
	key1, err := store.Save(context.Background(), ".jpg", bytes.NewReader([]byte("first")))
	if err != nil {
		t.Fatalf("first Save failed: %v", err)
	}
	key2, err := store.Save(context.Background(), ".jpg", bytes.NewReader([]byte("second")))
	if err != nil {
		t.Fatalf("second Save failed: %v", err)
	}

	if key1 == key2 {
		t.Fatal("expected distinct generated keys for separate uploads")
	}

	content1, _ := os.ReadFile(filepath.Join(store.Dir, key1))
	content2, _ := os.ReadFile(filepath.Join(store.Dir, key2))
	if string(content1) != "first" || string(content2) != "second" {
		t.Fatal("upload contents were not preserved independently")
	}
}

func TestLocalStoreGeneratedKeyNeverExposesOriginalFilename(t *testing.T) {
	store := newTestStore(t)
	key, err := store.Save(context.Background(), ".jpg", bytes.NewReader([]byte("data")))
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if strings.Contains(strings.ToLower(key), "secret") || strings.Contains(key, " ") {
		t.Fatalf("generated key unexpectedly resembles a client-controlled name: %q", key)
	}
	if !isSafeKey(key) {
		t.Fatalf("generated key %q does not match the expected safe shape", key)
	}
}

func TestLocalStoreDeleteRemovesFile(t *testing.T) {
	store := newTestStore(t)
	key, err := store.Save(context.Background(), ".png", bytes.NewReader([]byte("data")))
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := store.Delete(context.Background(), key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(store.Dir, key)); !os.IsNotExist(err) {
		t.Fatal("expected file to be removed")
	}
}

func TestLocalStoreDeleteMissingFileReturnsErrNotFound(t *testing.T) {
	store := newTestStore(t)
	err := store.Delete(context.Background(), strings.Repeat("0", 32)+".jpg")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestLocalStoreDeleteRejectsTraversalKeys is the core security guarantee:
// no key shape other than LocalStore's own generated filenames may ever
// reach the filesystem, regardless of how it is constructed.
func TestLocalStoreDeleteRejectsTraversalKeys(t *testing.T) {
	store := newTestStore(t)

	// Plant a canary file outside the store's directory to prove it's
	// never touched.
	parentDir := filepath.Dir(store.Dir)
	canary := filepath.Join(parentDir, "canary.txt")
	if err := os.WriteFile(canary, []byte("do not delete"), 0o644); err != nil {
		t.Fatalf("failed to plant canary file: %v", err)
	}
	defer os.Remove(canary)

	traversalKeys := []string{
		"../canary.txt",
		"..\\canary.txt",
		"../../etc/passwd",
		"/etc/passwd",
		"a/../../canary.txt",
		"canary.txt", // no matching extension/shape
		"" + strings.Repeat("a", 32) + ".exe",
		strings.Repeat("a", 31) + ".jpg", // wrong length
		strings.Repeat("g", 32) + ".jpg", // non-hex characters
	}
	for _, key := range traversalKeys {
		t.Run(key, func(t *testing.T) {
			err := store.Delete(context.Background(), key)
			if err == nil {
				t.Fatalf("expected Delete to reject unsafe key %q", key)
			}
			if _, statErr := os.Stat(canary); statErr != nil {
				t.Fatalf("canary file outside store dir was affected: %v", statErr)
			}
		})
	}
}

func TestLocalStoreKeyFromURLRejectsExternalURLs(t *testing.T) {
	store := newTestStore(t)

	external := []string{
		"https://cdn.example.com/plants/rose.jpg",
		"/uploads/products/../../etc/passwd",
		"/uploads/other/" + strings.Repeat("a", 32) + ".jpg",
		"",
	}
	for _, url := range external {
		if key, ok := store.KeyFromURL(url); ok {
			t.Errorf("expected KeyFromURL(%q) to report not-locally-managed, got key=%q", url, key)
		}
	}
}
