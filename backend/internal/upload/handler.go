// Package upload provides the admin-only product image upload endpoint.
// It depends only on the storage.Store abstraction, never on filesystem or
// cloud-vendor specifics directly.
package upload

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/Behnamdevops/plant-shop/backend/internal/storage"
)

// authenticator mirrors the interface used by the product and category
// packages so this package stays decoupled from auth's concrete type.
type authenticator interface {
	RequireAdmin(r *http.Request) (int64, error)
}

// maxUploadBody caps the entire multipart request body, not just the file
// part: MaxImageBytes for the file plus generous headroom for multipart
// boundaries/headers and form fields.
const maxUploadBody = storage.MaxImageBytes + 64*1024

type Handler struct {
	store storage.Store
	auth  authenticator
}

func NewHandler(store storage.Store, authHandler *auth.Handler) *Handler {
	return &Handler{store: store, auth: authHandler}
}

type uploadResponse struct {
	URL string `json:"url"`
}

// UploadProductImage handles POST /api/v1/admin/uploads/products.
// Admin-only: unauthenticated requests get 401, authenticated non-admins
// get 403. The uploaded file's content is validated server-side (never
// trusting the client filename or Content-Type) before being persisted
// under a freshly generated, unpredictable name.
func (h *Handler) UploadProductImage(w http.ResponseWriter, r *http.Request) {
	if _, err := h.auth.RequireAdmin(r); err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			http.Error(w, "forbidden", http.StatusForbidden)
		} else {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBody)

	if err := r.ParseMultipartForm(maxUploadBody); err != nil {
		http.Error(w, "invalid or oversized multipart form", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `"file" field is required`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := storage.ReadLimited(file, storage.MaxImageBytes)
	if err != nil {
		if errors.Is(err, storage.ErrTooLarge) {
			http.Error(w, "file exceeds maximum allowed size", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "failed to read uploaded file", http.StatusBadRequest)
		return
	}

	validated, err := storage.ValidateImage(data)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrEmptyFile):
			http.Error(w, "uploaded file is empty", http.StatusBadRequest)
		case errors.Is(err, storage.ErrTooLarge):
			http.Error(w, "file exceeds maximum allowed size", http.StatusRequestEntityTooLarge)
		default:
			http.Error(w, "unsupported or invalid image file", http.StatusBadRequest)
		}
		return
	}

	key, err := h.store.Save(r.Context(), validated.Ext, bytes.NewReader(validated.Data))
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(uploadResponse{URL: h.store.PublicURL(key)})
}
