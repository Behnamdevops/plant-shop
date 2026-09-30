package cart

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/http"
	"sort"
	"strings"
)

type MergeInput struct {
	Key   string         `json:"merge_key"`
	Items []AddItemInput `json:"items"`
}
type MergeResult struct {
	Warnings []string `json:"warnings"`
}

func (r *Repository) Merge(ctx context.Context, userID int64, input MergeInput) (MergeResult, error) {
	result := MergeResult{Warnings: []string{}}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", userID); err != nil {
		return result, err
	}
	var previous []byte
	err = tx.QueryRow(ctx, "SELECT result FROM cart_merge_batches WHERE user_id=$1 AND merge_key=$2", userID, input.Key).Scan(&previous)
	if err == nil {
		err = json.Unmarshal(previous, &result)
		return result, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	quantities := map[int64]int{}
	for _, item := range input.Items {
		quantities[item.ProductID] += item.Quantity
		if quantities[item.ProductID] > 1000 {
			quantities[item.ProductID] = 1000
		}
	}
	ids := []int64{}
	for id := range quantities {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		var stock, current int
		var name string
		err = tx.QueryRow(ctx, "SELECT stock,name FROM products WHERE id=$1 FOR UPDATE", id).Scan(&stock, &name)
		if errors.Is(err, pgx.ErrNoRows) {
			result.Warnings = append(result.Warnings, "یکی از محصولات دیگر موجود نیست.")
			continue
		}
		if err != nil {
			return result, err
		}
		err = tx.QueryRow(ctx, "SELECT quantity FROM cart_items WHERE user_id=$1 AND product_id=$2", userID, id).Scan(&current)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return result, err
		}
		wanted := current + quantities[id]
		if wanted > stock {
			wanted = stock
			result.Warnings = append(result.Warnings, "تعداد «"+name+"» با موجودی فعلی هماهنگ شد.")
		}
		if wanted <= 0 {
			continue
		}
		if _, err = tx.Exec(ctx, `INSERT INTO cart_items(user_id,product_id,quantity) VALUES($1,$2,$3) ON CONFLICT(user_id,product_id) DO UPDATE SET quantity=EXCLUDED.quantity,updated_at=NOW()`, userID, id, wanted); err != nil {
			return result, err
		}
	}
	body, err := json.Marshal(result)
	if err != nil {
		return result, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO cart_merge_batches(user_id,merge_key,result) VALUES($1,$2,$3)", userID, input.Key, body); err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}
func (h *Handler) Merge(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", 401)
		return
	}
	var input MergeInput
	if json.NewDecoder(r.Body).Decode(&input) != nil || len(input.Items) > 100 || len(input.Key) < 16 || len(input.Key) > 128 || strings.TrimSpace(input.Key) != input.Key {
		http.Error(w, "invalid cart merge", 400)
		return
	}
	for _, i := range input.Items {
		if i.ProductID <= 0 || i.Quantity < 1 || i.Quantity > 1000 {
			http.Error(w, "invalid cart merge", 400)
			return
		}
	}
	result, err := h.repository.Merge(r.Context(), userID, input)
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
