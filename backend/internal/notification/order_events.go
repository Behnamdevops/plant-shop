package notification

import (
	"context"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) OrderEventTx(ctx context.Context, tx pgx.Tx, orderID int64, kind string) error {
	var user, total int64
	var name, email string
	if err := tx.QueryRow(ctx, "SELECT o.user_id,o.total,u.name,u.email FROM orders o JOIN users u ON u.id=o.user_id WHERE o.id=$1", orderID).Scan(&user, &total, &name, &email); err != nil {
		return err
	}
	var payload Payload
	event := ""
	key := EventKey(orderID, kind)
	switch kind {
	case "created":
		event = EventTypeOrderCreated
		rows, err := tx.Query(ctx, "SELECT product_id,product_name,unit_price,quantity,subtotal FROM order_items WHERE order_id=$1 ORDER BY id", orderID)
		if err != nil {
			return err
		}
		items := []OrderItem{}
		for rows.Next() {
			var i OrderItem
			if err := rows.Scan(&i.ProductID, &i.ProductName, &i.UnitPrice, &i.Quantity, &i.Subtotal); err != nil {
				rows.Close()
				return err
			}
			items = append(items, i)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		payload = OrderCreatedPayload(orderID, name, email, total, items)
	case "cancelled":
		event = EventTypeOrderCancelled
		payload = OrderCancelledPayload(orderID, name, email, total)
	case "payment_succeeded":
		event = EventTypePaymentSucceeded
		payload = PaymentSucceededPayload(orderID, name, email, total)
	case "payment_failed":
		event = EventTypePaymentFailed
		payload = PaymentFailedPayload(orderID, name, email, total)
	case "processing", "shipped", "delivered":
		event = "order_" + kind
		key = EventKeyForOrderStatus(orderID, kind)
		payload = OrderStatusPayload(orderID, name, email, kind)
	default:
		return nil
	}
	return r.EnqueueTx(ctx, tx, key, user, email, event, payload)
}
func (r *Repository) QueuePasswordReset(ctx context.Context, tx pgx.Tx, userID int64, email, name, resetURL, tokenHash string) error {
	return r.EnqueueTx(ctx, tx, "password-reset:"+tokenHash, userID, email, "password_reset", Payload{"customer_name": name, "reset_url": resetURL})
}
