// Package storage defines a small object-storage abstraction used for
// admin-uploaded product images. Product handlers depend only on the Store
// interface, never on filesystem or cloud-vendor details, so a future
// S3-compatible implementation can be swapped in without touching product
// logic.
package storage

import (
	"context"
	"errors"
	"io"
)

// ErrNotFound is returned by Delete when the referenced object does not
// exist. Callers that perform best-effort cleanup should treat this as a
// non-error.
var ErrNotFound = errors.New("storage: object not found")

// Store is the minimal object-storage abstraction the application depends
// on. Implementations must generate their own storage keys; callers never
// pass client-controlled filenames straight through.
type Store interface {
	// Save persists the content read from r under a newly generated key
	// and returns that key. ext is a normalized, validated file extension
	// (e.g. ".jpg") without a leading dot ambiguity — implementations
	// decide how to fold it into the key.
	Save(ctx context.Context, ext string, r io.Reader) (key string, err error)

	// Delete removes the object identified by key. It returns ErrNotFound
	// if the object does not exist. Implementations should make Delete
	// safe to call on attacker-influenced-looking keys without escaping
	// their storage root.
	Delete(ctx context.Context, key string) error

	// PublicURL returns the URL clients should use to fetch the object
	// identified by key. It never exposes local absolute filesystem paths.
	PublicURL(key string) string
}
