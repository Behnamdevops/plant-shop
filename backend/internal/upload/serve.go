package upload

import (
	"net/http"
	"path/filepath"
	"strings"
)

// FileServer returns a handler that safely serves uploaded files out of
// dir. It intentionally does not use http.FileServer/http.Dir directly:
// that would allow directory listing and any relative-path trickery that
// slips past net/http's own cleaning. Instead, the key extracted from the
// URL is validated against the exact generated-filename shape before ever
// touching the filesystem, and only a fixed allow-list of image content
// types is ever written.
func FileServer(dir string) http.Handler {
	root := http.Dir(dir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		key := r.PathValue("key")
		if !isSafeServeKey(key) {
			http.NotFound(w, r)
			return
		}

		f, err := root.Open(key)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil || stat.IsDir() {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", contentTypeForExt(filepath.Ext(key)))
		// Uploaded filenames are randomly generated and never reused, so
		// their content is immutable for the lifetime of the file — safe
		// to cache aggressively.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		http.ServeContent(w, r, key, stat.ModTime(), f)
	})
}

// isSafeServeKey re-validates the path value with the same strict
// allow-list LocalStore uses internally (32 lowercase hex chars + a
// supported extension), rejecting anything else outright — including any
// "..", "/", or empty segments — before it ever reaches the filesystem.
func isSafeServeKey(key string) bool {
	if key == "" || strings.ContainsAny(key, `/\`) {
		return false
	}
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

func contentTypeForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
