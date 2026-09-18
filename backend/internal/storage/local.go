package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// keyPattern matches the exact shape LocalStore generates:
// <32 lowercase hex chars><extension>. Anything else is rejected before it
// ever touches the filesystem, which is what makes Delete and the public
// HTTP handler safe against directory traversal or arbitrary-path requests.
var validExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

// LocalStore is a filesystem-backed Store for local development and simple
// single-node production deployments. Files are written under Dir using
// randomly generated names — the original client filename is never used to
// build a path.
type LocalStore struct {
	// Dir is the absolute directory uploaded files are written to and
	// served from. It must already exist or be creatable.
	Dir string
	// URLPrefix is the public URL path prefix returned by PublicURL, e.g.
	// "/uploads/products". It must not contain "..".
	URLPrefix string
}

// NewLocalStore creates the upload directory (if missing) and returns a
// LocalStore rooted at it.
func NewLocalStore(dir, urlPrefix string) (*LocalStore, error) {
	if dir == "" {
		return nil, errors.New("storage: dir must not be empty")
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("storage: resolve dir: %w", err)
	}
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return nil, fmt.Errorf("storage: create upload dir: %w", err)
	}
	return &LocalStore{Dir: absDir, URLPrefix: strings.TrimSuffix(urlPrefix, "/")}, nil
}

// generateKey returns a random, unpredictable filename with the given
// extension. 16 random bytes (32 hex chars) makes collisions practically
// impossible without needing to check for existing files.
func generateKey(ext string) (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]) + ext, nil
}

// isSafeKey reports whether key is exactly the shape LocalStore generates:
// 32 lowercase hex characters followed by one of the allowed extensions.
// This is deliberately strict (an allow-list, not a deny-list) so no
// combination of "..", separators, or absolute-path segments can ever pass.
func isSafeKey(key string) bool {
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp"} {
		if strings.HasSuffix(key, ext) {
			name := strings.TrimSuffix(key, ext)
			if len(name) != 32 {
				return false
			}
			for _, c := range name {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					return false
				}
			}
			return true
		}
	}
	return false
}

// Save writes r's content to a newly generated file under Dir and returns
// its key. ext must be one of the allowed image extensions (validated by
// the caller before Save is invoked).
func (s *LocalStore) Save(ctx context.Context, ext string, r io.Reader) (string, error) {
	if !validExtensions[ext] {
		return "", fmt.Errorf("storage: unsupported extension %q", ext)
	}

	var key string
	for attempt := 0; attempt < 5; attempt++ {
		candidate, err := generateKey(ext)
		if err != nil {
			return "", err
		}
		path := filepath.Join(s.Dir, candidate)
		// O_EXCL guarantees we never silently overwrite an existing file,
		// even in the astronomically unlikely case of a collision.
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", fmt.Errorf("storage: create file: %w", err)
		}
		_, copyErr := io.Copy(f, r)
		closeErr := f.Close()
		if copyErr != nil {
			os.Remove(path)
			return "", fmt.Errorf("storage: write file: %w", copyErr)
		}
		if closeErr != nil {
			os.Remove(path)
			return "", fmt.Errorf("storage: close file: %w", closeErr)
		}
		key = candidate
		break
	}
	if key == "" {
		return "", errors.New("storage: failed to allocate a unique filename")
	}
	return key, nil
}

// Delete removes the file identified by key. It refuses to touch anything
// outside Dir: keys that don't match the exact generated-filename shape are
// rejected outright rather than being filepath.Clean-ed and hoped safe.
func (s *LocalStore) Delete(ctx context.Context, key string) error {
	if !isSafeKey(key) {
		return fmt.Errorf("storage: refusing to delete unsafe key %q", key)
	}
	path := filepath.Join(s.Dir, key)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// PublicURL returns the public path clients use to fetch the object. It
// never leaks Dir's absolute filesystem location.
func (s *LocalStore) PublicURL(key string) string {
	return s.URLPrefix + "/" + key
}

// KeyFromURL extracts the storage key from a URL previously returned by
// PublicURL, or "" if url does not point at a locally managed object (e.g.
// an external/legacy image URL). This is used for best-effort cleanup of
// the previous image when a product is updated or deleted — it must never
// be fooled into producing a key for anything outside Dir, which is why it
// re-validates with isSafeKey.
func (s *LocalStore) KeyFromURL(url string) (string, bool) {
	prefix := s.URLPrefix + "/"
	if !strings.HasPrefix(url, prefix) {
		return "", false
	}
	key := strings.TrimPrefix(url, prefix)
	if !isSafeKey(key) {
		return "", false
	}
	return key, true
}
