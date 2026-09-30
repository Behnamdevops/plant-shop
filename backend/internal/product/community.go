package product

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (h *Handler) Wishlist(w http.ResponseWriter, r *http.Request) {
	u, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", 401)
		return
	}
	ids := []int64{}
	rows, err := h.repository.db.Query(r.Context(), "SELECT product_id FROM wishlists WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100", u)
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			http.Error(w, "internal server error", 500)
			return
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	products := []Product{}
	if len(ids) > 0 {
		rows, err = h.repository.db.Query(r.Context(), "SELECT "+productColumns+" FROM products WHERE id=ANY($1)", ids)
		if err != nil {
			http.Error(w, "internal server error", 500)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var p Product
			if err = scanProduct(rows, &p); err != nil {
				http.Error(w, "internal server error", 500)
				return
			}
			products = append(products, p)
		}
		if rows.Err() != nil {
			http.Error(w, "internal server error", 500)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}
func (h *Handler) MergeWishlist(w http.ResponseWriter, r *http.Request) {
	u, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", 401)
		return
	}
	var input struct {
		IDs []int64 `json:"product_ids"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || len(input.IDs) > 100 {
		http.Error(w, "invalid wishlist", 400)
		return
	}
	for _, id := range input.IDs {
		if id < 1 {
			http.Error(w, "invalid wishlist", 400)
			return
		}
	}
	tx, err := h.repository.db.Begin(r.Context())
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), "SELECT id FROM users WHERE id=$1 FOR UPDATE", u); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	// Union is idempotent and capped; no client-supplied product data is persisted.
	if _, err = tx.Exec(r.Context(), `INSERT INTO wishlists(user_id,product_id) SELECT $1,id FROM products WHERE id=ANY($2) AND NOT EXISTS(SELECT 1 FROM wishlists WHERE user_id=$1 AND product_id=products.id) ORDER BY id LIMIT GREATEST(0,100-(SELECT count(*) FROM wishlists WHERE user_id=$1)) ON CONFLICT DO NOTHING`, u, input.IDs); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	if tx.Commit(r.Context()) != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) DeleteWishlist(w http.ResponseWriter, r *http.Request) {
	u, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", 401)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.Error(w, "invalid id", 400)
		return
	}
	if _, err = h.repository.db.Exec(r.Context(), "DELETE FROM wishlists WHERE user_id=$1 AND product_id=$2", u, id); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	w.WriteHeader(204)
}

type Review struct {
	ID        int64     `json:"id"`
	ProductID int64     `json:"product_id"`
	Name      string    `json:"customer_name"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *Handler) Reviews(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.Error(w, "invalid id", 400)
		return
	}
	rows, err := h.repository.db.Query(r.Context(), `SELECT r.id,r.product_id,u.name,r.rating,r.comment,r.status,r.created_at FROM product_reviews r JOIN users u ON u.id=r.user_id WHERE r.product_id=$1 AND r.status='approved' ORDER BY r.id DESC LIMIT 100`, id)
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	defer rows.Close()
	items := []Review{}
	for rows.Next() {
		var i Review
		if err = rows.Scan(&i.ID, &i.ProductID, &i.Name, &i.Rating, &i.Comment, &i.Status, &i.CreatedAt); err != nil {
			http.Error(w, "internal server error", 500)
			return
		}
		items = append(items, i)
	}
	if rows.Err() != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	var count int
	var average float64
	if err = h.repository.db.QueryRow(r.Context(), "SELECT count(*),COALESCE(avg(rating),0)::float8 FROM product_reviews WHERE product_id=$1 AND status='approved'", id).Scan(&count, &average); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"items": items, "count": count, "average": average})
}
func (h *Handler) SubmitReview(w http.ResponseWriter, r *http.Request) {
	u, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", 401)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.Error(w, "invalid id", 400)
		return
	}
	var input struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		http.Error(w, "invalid review", 400)
		return
	}
	input.Comment = strings.TrimSpace(input.Comment)
	if input.Rating < 1 || input.Rating > 5 || len([]rune(input.Comment)) < 3 || len([]rune(input.Comment)) > 2000 {
		http.Error(w, "invalid review", 400)
		return
	}
	// Eligibility and the write use one statement, so only delivered paid buyers can review.
	tag, err := h.repository.db.Exec(r.Context(), `INSERT INTO product_reviews(user_id,product_id,rating,comment) SELECT $1,$2,$3,$4 WHERE EXISTS(SELECT 1 FROM order_items i JOIN orders o ON o.id=i.order_id WHERE i.product_id=$2 AND o.user_id=$1 AND o.status='delivered' AND o.payment_status='paid') ON CONFLICT(user_id,product_id) DO UPDATE SET rating=EXCLUDED.rating,comment=EXCLUDED.comment,status='pending',updated_at=NOW()`, u, id, input.Rating, input.Comment)
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, "only verified buyers can review", 403)
		return
	}
	w.WriteHeader(202)
}
func (h *Handler) AdminReviews(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	rows, err := h.repository.db.Query(r.Context(), `SELECT r.id,r.product_id,u.name,r.rating,r.comment,r.status,r.created_at FROM product_reviews r JOIN users u ON u.id=r.user_id ORDER BY r.id DESC LIMIT 200`)
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	defer rows.Close()
	items := []Review{}
	for rows.Next() {
		var i Review
		if err = rows.Scan(&i.ID, &i.ProductID, &i.Name, &i.Rating, &i.Comment, &i.Status, &i.CreatedAt); err != nil {
			http.Error(w, "internal server error", 500)
			return
		}
		items = append(items, i)
	}
	if rows.Err() != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}
func (h *Handler) ModerateReview(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var input struct {
		Status string `json:"status"`
	}
	if err != nil || id < 1 || json.NewDecoder(r.Body).Decode(&input) != nil || (input.Status != "approved" && input.Status != "rejected") {
		http.Error(w, "invalid review status", 400)
		return
	}
	tag, err := h.repository.db.Exec(r.Context(), "UPDATE product_reviews SET status=$1,updated_at=NOW() WHERE id=$2", input.Status, id)
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, "review not found", 404)
		return
	}
	w.WriteHeader(204)
}
