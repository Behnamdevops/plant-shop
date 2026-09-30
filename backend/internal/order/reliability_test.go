package order

import (
	"github.com/Behnamdevops/plant-shop/backend/internal/notification"
	"testing"
	"time"
)

func TestExpiryRestoresStockExactlyOnceAndKeepsOutbox(t *testing.T) {
	e := newTestEnv(t)
	u := e.createUser(t)
	p := e.createProduct(t, 4)
	e.orderRepo.WithNotifications(notification.NewRepository(e.db))
	if _, err := e.cartRepo.AddItem(t.Context(), u, p.ID, 2); err != nil {
		t.Fatal(err)
	}
	o, err := e.orderRepo.CreateFromCart(t.Context(), u, validCheckoutInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.db.Exec(t.Context(), "UPDATE orders SET created_at=NOW()-INTERVAL '2 hours' WHERE id=$1", o.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = e.orderRepo.ExpireReservations(t.Context(), time.Now().Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	var stock int
	var status string
	e.db.QueryRow(t.Context(), "SELECT stock FROM products WHERE id=$1", p.ID).Scan(&stock)
	e.db.QueryRow(t.Context(), "SELECT status FROM orders WHERE id=$1", o.ID).Scan(&status)
	if stock != 4 || status != "cancelled" {
		t.Fatalf("stock=%d status=%s", stock, status)
	}
	var events int
	e.db.QueryRow(t.Context(), "SELECT count(*) FROM notification_outbox WHERE user_id=$1", u).Scan(&events)
	if events != 2 {
		t.Fatalf("outbox events=%d, want 2", events)
	}
}
func TestShippingUsesDiscountedSubtotal(t *testing.T) {
	c := ShippingConfig{Standard: 500, Express: 1500, FreeThreshold: 1000, Configured: true}
	fee, ok := c.Fee("standard", 900)
	if !ok || fee != 500 {
		t.Fatal("discounted order should pay shipping")
	}
	fee, ok = c.Fee("standard", 1000)
	if !ok || fee != 0 {
		t.Fatal("qualifying standard order should be free")
	}
	fee, _ = c.Fee("express", 1000)
	if fee != 1500 {
		t.Fatal("express fee changed")
	}
}
