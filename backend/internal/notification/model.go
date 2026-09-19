package notification

import (
	"encoding/json"
	"fmt"
	"time"
)

// Status represents the delivery state of a notification.
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusSent       = "sent"
	StatusFailed     = "failed"
)

// Event types for notifications.
const (
	EventTypeOrderCreated     = "order_created"
	EventTypePaymentSucceeded = "payment_succeeded"
	EventTypePaymentFailed    = "payment_failed"
	EventTypeOrderProcessing  = "order_processing"
	EventTypeOrderShipped     = "order_shipped"
	EventTypeOrderDelivered   = "order_delivered"
	EventTypeOrderCancelled   = "order_cancelled"
)

// NotificationOutbox represents a row in the notification_outbox table.
type NotificationOutbox struct {
	ID             int64      `json:"id"`
	EventKey       string     `json:"event_key"`
	UserID         *int64     `json:"user_id,omitempty"`
	RecipientEmail string     `json:"recipient_email"`
	EventType      string     `json:"event_type"`
	Payload        Payload    `json:"payload"`
	Status         string     `json:"status"`
	Attempts       int        `json:"attempts"`
	NextAttemptAt  time.Time  `json:"next_attempt_at"`
	LastError      *string    `json:"last_error,omitempty"`
	SentAt         *time.Time `json:"sent_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Payload is a map for JSONB notification data.
type Payload map[string]interface{}

// MarshalJSON implements custom JSON marshaling for Payload.
func (p Payload) MarshalJSON() ([]byte, error) {
	if p == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(map[string]interface{}(p))
}

// UnmarshalJSON implements custom JSON unmarshaling for Payload.
func (p *Payload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*p = m
	return nil
}

// EventKey generates a unique, idempotent event key for an order event.
func EventKey(orderID int64, eventType string) string {
	return fmt.Sprintf("order:%d:%s", orderID, eventType)
}

// EventKeyForPayment generates an event key for a payment event.
func EventKeyForPayment(orderID int64, eventType string) string {
	return fmt.Sprintf("order:%d:%s", orderID, eventType)
}

// EventKeyForOrderStatus generates an event key for an order status transition.
func EventKeyForOrderStatus(orderID int64, status string) string {
	return fmt.Sprintf("order:%d:status:%s", orderID, status)
}

// OrderCreatedPayload creates the payload for an order_created event.
func OrderCreatedPayload(orderID int64, customerName, customerEmail string, total int64, items []OrderItem) Payload {
	return Payload{
		"order_id":       orderID,
		"customer_name":  customerName,
		"customer_email": customerEmail,
		"total":          total,
		"items":          items,
		"total_toman":    formatToman(total),
	}
}

// PaymentSucceededPayload creates the payload for a payment_succeeded event.
func PaymentSucceededPayload(orderID int64, customerName, customerEmail string, total int64) Payload {
	return Payload{
		"order_id":       orderID,
		"customer_name":  customerName,
		"customer_email": customerEmail,
		"total":          total,
		"total_toman":    formatToman(total),
		"message":        "پرداخت شما با موفقیت انجام شد",
	}
}

// PaymentFailedPayload creates the payload for a payment_failed event.
func PaymentFailedPayload(orderID int64, customerName, customerEmail string, total int64) Payload {
	return Payload{
		"order_id":       orderID,
		"customer_name":  customerName,
		"customer_email": customerEmail,
		"total":          total,
		"total_toman":    formatToman(total),
		"message":        "پرداخت شما انجام نشد. لطفاً مجدداً تلاش کنید.",
	}
}

// OrderStatusPayload creates the payload for order status events.
func OrderStatusPayload(orderID int64, customerName, customerEmail string, status string) Payload {
	return Payload{
		"order_id":       orderID,
		"customer_name":  customerName,
		"customer_email": customerEmail,
		"status":         status,
		"status_message": statusMessage(status),
	}
}

// OrderCancelledPayload creates the payload for an order_cancelled event.
func OrderCancelledPayload(orderID int64, customerName, customerEmail string, total int64) Payload {
	return Payload{
		"order_id":       orderID,
		"customer_name":  customerName,
		"customer_email": customerEmail,
		"total":          total,
		"total_toman":    formatToman(total),
		"message":        "سفارش شما لغو شد.",
	}
}

// OrderItem represents an item in an order for notification payloads.
type OrderItem struct {
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
	Quantity    int    `json:"quantity"`
	UnitPrice   int64  `json:"unit_price"`
	Subtotal    int64  `json:"subtotal"`
}

// formatToman converts IRR to Toman (divide by 10).
func formatToman(amount int64) string {
	if amount < 0 {
		return "0 تومان"
	}
	toman := amount / 10
	return fmt.Sprintf("%d تومان", toman)
}

// statusMessage returns a Persian message for the given order status.
func statusMessage(status string) string {
	switch status {
	case "processing":
		return "سفارش در حال پردازش است"
	case "shipped":
		return "سفارش ارسال شد"
	case "delivered":
		return "سفارش تحویل داده شد"
	case "cancelled":
		return "سفارش لغو شد"
	default:
		return "وضعیت سفارش تغییر کرد"
	}
}
