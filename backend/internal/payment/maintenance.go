package payment

import (
	"context"
	"time"
)

// An issued authority is never released on a timeout or an ambiguous response.
func (h *Handler) ExpireAbandoned(ctx context.Context, cutoff time.Time) error {
	rows, err := h.repository.db.Query(ctx, `SELECT id FROM payment_attempts WHERE (status='pending' OR(status='failed' AND authority IS NOT NULL)) AND created_at<$1 AND environment=$2 AND account_key=$3 ORDER BY updated_at,id LIMIT 20`, cutoff, h.repository.environment, h.repository.identity)
	if err != nil {
		return err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		item, err := h.repository.GetReconciliation(ctx, id)
		if err != nil {
			return err
		}
		if item.Authority == nil {
			if item.Status == StatusPending {
				_ = h.repository.MarkAttemptFailed(ctx, id, nil)
			}
			continue
		}
		if h.client == nil || !item.Retryable {
			continue
		}
		checked, err := h.ReconcilePayment(ctx, id)
		if err != nil {
			continue
		}
		if (checked.Status == StatusPending || checked.Status == StatusFailed) && checked.LastOutcome == "definitive_rejection" && checked.ProviderCode != nil && *checked.ProviderCode == -51 {
			_ = h.repository.expireRejected(ctx, id, cutoff)
		}
	}
	return nil
}
func (r *Repository) expireRejected(ctx context.Context, id int64, cutoff time.Time) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	o, a, err := lockPaymentAttempt(ctx, tx, id)
	if err != nil {
		return err
	}
	if !r.matchesProvider(a) || a.CreatedAt.After(cutoff) || (a.Status != StatusPending && a.Status != StatusFailed) || a.ProviderCode == nil || *a.ProviderCode != -51 || o.Status != "pending" || o.PaymentStatus == "paid" {
		return ErrAlreadyProcessed
	}
	if _, err = tx.Exec(ctx, "UPDATE payment_attempts SET status='expired',updated_at=NOW() WHERE id=$1", id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
