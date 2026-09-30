package order

import (
	"context"
	"github.com/Behnamdevops/plant-shop/backend/internal/coupon"
	"time"
)

func (r *Repository) ExpireReservations(ctx context.Context, cutoff time.Time) (int, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id,payment_status FROM orders WHERE status='pending' AND payment_status IN('pending','failed') AND created_at<$1 AND NOT EXISTS(SELECT 1 FROM payment_attempts a WHERE a.order_id=orders.id AND (a.status IN('pending','paid','reconciliation') OR (a.authority IS NOT NULL AND a.status<>'expired'))) ORDER BY id LIMIT 20 FOR UPDATE SKIP LOCKED`, cutoff)
	if err != nil {
		return 0, err
	}
	type entry struct {
		id     int64
		status string
	}
	items := []entry{}
	for rows.Next() {
		var i entry
		if err = rows.Scan(&i.id, &i.status); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, i := range items {
		blocked, err := paymentBlocksCancellation(ctx, tx, i.id, i.status)
		if err != nil {
			return 0, err
		}
		if blocked {
			continue
		}
		if err = restoreOrderStock(ctx, tx, i.id); err != nil {
			return 0, err
		}
		if _, err = coupon.ReleaseRedemptionForOrder(ctx, tx, i.id); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, "UPDATE orders SET status='cancelled',updated_at=NOW() WHERE id=$1", i.id); err != nil {
			return 0, err
		}
		if r.notifications != nil {
			if err = r.notifications.OrderEventTx(ctx, tx, i.id, "cancelled"); err != nil {
				return 0, err
			}
		}
		count++
	}
	return count, tx.Commit(ctx)
}
