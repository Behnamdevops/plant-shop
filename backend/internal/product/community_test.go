package product

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReviewRequiresDeliveredPaidPurchaseAndModeration(t *testing.T) {
	e := newTestHandlerEnv(t)
	buyer := e.registerAndLogin(t, "user")
	admin := e.registerAndLogin(t, "admin")
	p := e.createProduct(t, handlerUniqueSuffix())
	request := func(method string, cookie *http.Cookie, body string, handler http.HandlerFunc) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/", bytes.NewBufferString(body))
		r.SetPathValue("id", jsonNumber(p.ID))
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler(w, r)
		return w
	}
	if w := request("POST", buyer, `{"rating":5,"comment":"نظر آزمایشی"}`, e.handler.SubmitReview); w.Code != 403 {
		t.Fatalf("nonbuyer: %d %s", w.Code, w.Body.String())
	}
	var userID, orderID int64
	if err := e.db.QueryRow(t.Context(), "SELECT user_id FROM sessions WHERE token_hash=$1", sha256HashForTest(buyer.Value)).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := e.db.QueryRow(t.Context(), "INSERT INTO orders(user_id,status,total,payment_status) VALUES($1,'delivered',100,'paid') RETURNING id", userID).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(t.Context(), "INSERT INTO order_items(order_id,product_id,product_name,product_slug,unit_price,quantity,subtotal) VALUES($1,$2,$3,$4,100,1,100)", orderID, p.ID, p.Name, p.Slug); err != nil {
		t.Fatal(err)
	}
	if w := request("POST", buyer, `{"rating":5,"comment":"نظر آزمایشی"}`, e.handler.SubmitReview); w.Code != 202 {
		t.Fatalf("buyer: %d %s", w.Code, w.Body.String())
	}
	w := request("GET", nil, "", e.handler.Reviews)
	var data struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if data.Count != 0 {
		t.Fatal("pending review leaked")
	}
	var reviewID int64
	e.db.QueryRow(t.Context(), "SELECT id FROM product_reviews WHERE product_id=$1 AND user_id=$2", p.ID, userID).Scan(&reviewID)
	r := httptest.NewRequest("PATCH", "/", bytes.NewBufferString(`{"status":"approved"}`))
	r.SetPathValue("id", jsonNumber(reviewID))
	r.AddCookie(admin)
	w = httptest.NewRecorder()
	e.handler.ModerateReview(w, r)
	if w.Code != 204 {
		t.Fatalf("moderate: %d %s", w.Code, w.Body.String())
	}
	w = request("GET", nil, "", e.handler.Reviews)
	json.Unmarshal(w.Body.Bytes(), &data)
	if data.Count != 1 {
		t.Fatal("approved review missing")
	}
}
func jsonNumber(v int64) string { b, _ := json.Marshal(v); return string(b) }
