package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/Behnamdevops/plant-shop/backend/internal/order"
)

func (e *testEnv) insertLegacyAttempt(t *testing.T, o order.Order, status string) Attempt {
	t.Helper()
	authority := "A" + uniqueSuffixPayment()
	a, err := scanAttempt(e.db.QueryRow(t.Context(), `
		INSERT INTO payment_attempts (order_id, amount, authority, status, environment, account_key)
		VALUES ($1, $2, $3, $4, 'test', 'test') RETURNING `+attemptColumns,
		o.ID, o.Total, authority, status))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func (e *testEnv) assertOrderStock(t *testing.T, orderID int64, status, paymentStatus string, stock int) {
	t.Helper()
	var gotStatus, gotPayment string
	var gotStock int
	err := e.db.QueryRow(t.Context(), `
		SELECT o.status, o.payment_status, p.stock
		FROM orders o JOIN order_items i ON i.order_id = o.id JOIN products p ON p.id = i.product_id
		WHERE o.id = $1`, orderID).Scan(&gotStatus, &gotPayment, &gotStock)
	if err != nil {
		t.Fatal(err)
	}
	if gotStatus != status || gotPayment != paymentStatus || gotStock != stock {
		t.Fatalf("order status=%s payment=%s stock=%d; want %s/%s/%d", gotStatus, gotPayment, gotStock, status, paymentStatus, stock)
	}
}

func TestRepositoryConcurrentAttemptCreationHasOneWinner(t *testing.T) {
	env := newTestEnv(t, succeedingClient())
	cookie, _ := env.registerAndLogin(t)
	o := env.createOrder(t, cookie)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	start := make(chan struct{})
	results := make(chan error, 12)
	for i := 0; i < cap(results); i++ {
		go func() {
			<-start
			_, err := env.repository.CreateAttemptForOrder(ctx, o.UserID, o.ID)
			results <- err
		}()
	}
	close(start)
	winners := 0
	for i := 0; i < cap(results); i++ {
		select {
		case err := <-results:
			if err == nil {
				winners++
			} else if !errors.Is(err, ErrPaymentInProgress) {
				t.Errorf("unexpected losing result: %v", err)
			}
		case <-ctx.Done():
			t.Fatal("attempt creation deadlock")
		}
	}
	attempts, err := env.repository.ListAttemptsForOrder(t.Context(), o.ID)
	if err != nil || winners != 1 || len(attempts) != 1 {
		t.Fatalf("winners=%d attempts=%+v err=%v", winners, attempts, err)
	}
	env.assertOrderStock(t, o.ID, order.StatusPending, order.PaymentStatusPending, 9)
}

func TestRepositoryCancellationRacesPayment(t *testing.T) {
	for _, admin := range []bool{false, true} {
		for _, phase := range []string{"creation", "verification"} {
			for _, schedule := range []string{"concurrent", "cancel_first", "payment_first"} {
				t.Run(fmt.Sprintf("admin=%t/%s/%s", admin, phase, schedule), func(t *testing.T) {
					env := newTestEnv(t, succeedingClient())
					cookie, _ := env.registerAndLogin(t)
					o := env.createOrder(t, cookie)
					var a Attempt
					if phase == "verification" {
						a = env.startPayment(t, cookie, o.ID)
					}
					ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
					defer cancel()
					orders := order.NewRepository(env.db)
					cancelOrder := func() error {
						if admin {
							_, err := orders.UpdateStatus(ctx, o.ID, order.StatusCancelled)
							return err
						}
						_, err := orders.CancelOwnOrder(ctx, o.UserID, o.ID)
						return err
					}
					pay := func() error {
						if phase == "creation" {
							_, err := env.repository.CreateAttemptForOrder(ctx, o.UserID, o.ID)
							return err
						}
						_, err := env.repository.FinalizeVerifiedPayment(ctx, *a.Authority, 123456, 100)
						return err
					}
					var cancelErr, payErr error
					switch schedule {
					case "cancel_first":
						cancelErr, payErr = cancelOrder(), pay()
					case "payment_first":
						payErr, cancelErr = pay(), cancelOrder()
					default:
						start := make(chan struct{})
						cancelDone, payDone := make(chan error, 1), make(chan error, 1)
						go func() { <-start; cancelDone <- cancelOrder() }()
						go func() { <-start; payDone <- pay() }()
						close(start)
						cancelErr, payErr = <-cancelDone, <-payDone
					}
					if phase == "creation" && cancelErr == nil {
						if !errors.Is(payErr, ErrOrderNotPayable) {
							t.Fatalf("cancel won but payment err=%v", payErr)
						}
						env.assertOrderStock(t, o.ID, order.StatusCancelled, order.PaymentStatusPending, 10)
						attempts, err := env.repository.ListAttemptsForOrder(t.Context(), o.ID)
						if err != nil || len(attempts) != 0 {
							t.Fatalf("cancelled order attempts=%+v err=%v", attempts, err)
						}
					} else {
						wantErr := order.ErrOrderNotEligibleForCancel
						if admin {
							wantErr = order.ErrInvalidTransition
						}
						if payErr != nil || !errors.Is(cancelErr, wantErr) {
							t.Fatalf("payment err=%v cancellation err=%v, want %v", payErr, cancelErr, wantErr)
						}
						paymentStatus := order.PaymentStatusPending
						if phase == "verification" {
							paymentStatus = order.PaymentStatusPaid
						}
						env.assertOrderStock(t, o.ID, order.StatusPending, paymentStatus, 9)
					}
					if err := cancelOrder(); err == nil {
						t.Fatal("repeated cancellation unexpectedly succeeded")
					}
					if cancelErr == nil {
						env.assertOrderStock(t, o.ID, order.StatusCancelled, order.PaymentStatusPending, 10)
					} else if phase == "verification" {
						env.assertOrderStock(t, o.ID, order.StatusPending, order.PaymentStatusPaid, 9)
					} else {
						env.assertOrderStock(t, o.ID, order.StatusPending, order.PaymentStatusPending, 9)
					}
				})
			}
		}
	}
}

type paymentEvent struct {
	ID       int64
	Kind     string
	Snapshot json.RawMessage
}

func (e *testEnv) paymentEvents(t *testing.T, attemptID int64) []paymentEvent {
	t.Helper()
	rows, err := e.db.Query(t.Context(), `SELECT id, event_type, snapshot FROM payment_attempt_events WHERE attempt_id = $1 ORDER BY id`, attemptID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var events []paymentEvent
	for rows.Next() {
		var event paymentEvent
		if err := rows.Scan(&event.ID, &event.Kind, &event.Snapshot); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func TestCallbackLegacyAttemptsSettleOnceAndPreserveHistory(t *testing.T) {
	client := succeedingClient()
	env := newTestEnv(t, client)
	cookie, _ := env.registerAndLogin(t)
	o := env.createOrder(t, cookie)
	first := env.insertLegacyAttempt(t, o, StatusFailed)
	second := env.insertLegacyAttempt(t, o, StatusPending)
	code := -51
	if _, err := env.repository.FinalizeFailedPayment(t.Context(), *first.Authority, &code); err != nil {
		t.Fatal(err)
	}
	if _, err := env.repository.FinalizeFailedPayment(t.Context(), *first.Authority, nil); err != nil {
		t.Fatal(err)
	}
	history := env.paymentEvents(t, first.ID)
	calls := 0
	client.verifyFunc = func(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error) {
		calls++
		if in.Authority == *first.Authority {
			return VerifyPaymentOutput{Code: 101, RefID: 123456, AlreadyVerified: true}, nil
		}
		return VerifyPaymentOutput{Code: 100, RefID: 123457}, nil
	}
	env.callback(t, *first.Authority, "OK", "success")
	env.callback(t, *second.Authority, "OK", "unknown")
	code = 101
	env.assertAttempt(t, *first.Authority, StatusPaid, 123456, &code)
	code = 100
	env.assertAttempt(t, *second.Authority, StatusReconciliation, 123457, &code)
	env.assertOrderStock(t, o.ID, order.StatusPending, order.PaymentStatusPaid, 9)
	var paid int
	if err := env.db.QueryRow(t.Context(), `SELECT COUNT(*) FROM payment_attempts WHERE order_id = $1 AND status = 'paid'`, o.ID).Scan(&paid); err != nil || paid != 1 {
		t.Fatalf("settlements=%d err=%v", paid, err)
	}
	for _, a := range []Attempt{first, second} {
		events := env.paymentEvents(t, a.ID)
		settlements := 0
		for _, event := range events {
			var snapshot Attempt
			if err := json.Unmarshal(event.Snapshot, &snapshot); err != nil {
				t.Fatal(err)
			}
			var binding struct {
				AccountKey string `json:"account_key"`
			}
			if err := json.Unmarshal(event.Snapshot, &binding); err != nil {
				t.Fatal(err)
			}
			if snapshot.OrderID != o.ID || snapshot.Amount != o.Total || snapshot.Authority == nil || *snapshot.Authority != *a.Authority || snapshot.Environment != "test" || binding.AccountKey != "test" {
				t.Fatalf("event lost binding: %s", event.Snapshot)
			}
			if event.Kind == StatusPaid || event.Kind == StatusReconciliation {
				settlements++
				wantRef, wantCode, wantStatus := int64(123456), 101, StatusPaid
				if a.ID == second.ID {
					wantRef, wantCode, wantStatus = 123457, 100, StatusReconciliation
				}
				if snapshot.Status != wantStatus || snapshot.RefID == nil || *snapshot.RefID != wantRef || snapshot.ProviderCode == nil || *snapshot.ProviderCode != wantCode || snapshot.VerifiedAt == nil {
					t.Fatalf("event lost settlement: %s", event.Snapshot)
				}
			}
		}
		if settlements != 1 {
			t.Fatalf("settlement events=%d", settlements)
		}
		if a.ID == first.ID {
			if len(events) <= len(history) {
				t.Fatal("missing settlement event")
			}
			for i, before := range history {
				if before.ID != events[i].ID || before.Kind != events[i].Kind || string(before.Snapshot) != string(events[i].Snapshot) {
					t.Fatal("verification overwrote event history")
				}
			}
			rejected, uncertain := false, false
			for _, event := range history {
				var snapshot Attempt
				if err := json.Unmarshal(event.Snapshot, &snapshot); err != nil {
					t.Fatal(err)
				}
				if event.Kind == "verify_rejected" || event.Kind == "verify_uncertain" {
					if snapshot.ProviderCode == nil || *snapshot.ProviderCode != -51 || snapshot.RefID != nil {
						t.Fatalf("lost rejection code: %s", event.Snapshot)
					}
					rejected = rejected || event.Kind == "verify_rejected"
					uncertain = uncertain || event.Kind == "verify_uncertain"
				}
			}
			if !rejected || !uncertain {
				t.Fatal("missing rejection/uncertain history")
			}
		}
		if _, err := env.db.Exec(t.Context(), `UPDATE payment_attempt_events SET event_type = 'updated' WHERE id = $1`, events[0].ID); err == nil {
			t.Fatal("event update accepted")
		}
		if _, err := env.db.Exec(t.Context(), `DELETE FROM payment_attempt_events WHERE id = $1`, events[0].ID); err == nil {
			t.Fatal("event deletion accepted")
		}
	}
	env.callback(t, *first.Authority, "NOK", "success")
	env.callback(t, *second.Authority, "OK", "unknown")
	if calls != 2 {
		t.Fatalf("terminal callbacks made additional provider calls: %d", calls)
	}
}

func TestCallbackCancelledLegacyAttemptQuarantinesWithoutStockChange(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(fmt.Sprintf("admin=%t", admin), func(t *testing.T) {
			env := newTestEnv(t, succeedingClient())
			cookie, _ := env.registerAndLogin(t)
			o := env.createOrder(t, cookie)
			orders := order.NewRepository(env.db)
			var err error
			if admin {
				_, err = orders.UpdateStatus(t.Context(), o.ID, order.StatusCancelled)
			} else {
				_, err = orders.CancelOwnOrder(t.Context(), o.UserID, o.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			a := env.insertLegacyAttempt(t, o, StatusFailed)
			env.assertOrderStock(t, o.ID, order.StatusCancelled, order.PaymentStatusPending, 10)
			env.callback(t, *a.Authority, "OK", "unknown")
			code := 100
			env.assertAttempt(t, *a.Authority, StatusReconciliation, 123456, &code)
			env.callback(t, *a.Authority, "OK", "unknown")
			env.assertOrderStock(t, o.ID, order.StatusCancelled, order.PaymentStatusPending, 10)
		})
	}
}

func TestCallbackTotalMismatchQuarantines(t *testing.T) {
	client := succeedingClient()
	var gotAmount int64
	client.verifyFunc = func(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error) {
		gotAmount = in.Amount
		return VerifyPaymentOutput{Code: 100, RefID: 123456}, nil
	}
	env := newTestEnv(t, client)
	cookie, _ := env.registerAndLogin(t)
	o := env.createOrder(t, cookie)
	a := env.startPayment(t, cookie, o.ID)
	if _, err := env.db.Exec(t.Context(), `UPDATE orders SET total = total + 10000 WHERE id = $1`, o.ID); err != nil {
		t.Fatal(err)
	}
	env.callback(t, *a.Authority, "OK", "unknown")
	code := 100
	env.assertAttempt(t, *a.Authority, StatusReconciliation, 123456, &code)
	if gotAmount != a.Amount {
		t.Fatalf("verified changed total: got %d want %d", gotAmount, a.Amount)
	}
	env.assertOrderStock(t, o.ID, order.StatusPending, order.PaymentStatusPending, 9)
}

func TestRepositoryAttemptBindingImmutable(t *testing.T) {
	env := newTestEnv(t, succeedingClient())
	cookie, _ := env.registerAndLogin(t)
	o := env.createOrder(t, cookie)
	other := env.createOrder(t, cookie)
	a := env.startPayment(t, cookie, o.ID)
	for _, mutation := range []struct {
		name, query string
		value       any
	}{
		{"order", `UPDATE payment_attempts SET order_id = $2 WHERE id = $1`, other.ID},
		{"amount", `UPDATE payment_attempts SET amount = $2 WHERE id = $1`, a.Amount + 10000},
		{"currency", `UPDATE payment_attempts SET currency = $2 WHERE id = $1`, "USD"},
		{"provider", `UPDATE payment_attempts SET provider = $2 WHERE id = $1`, "other"},
		{"environment", `UPDATE payment_attempts SET environment = $2 WHERE id = $1`, "sandbox"},
		{"merchant", `UPDATE payment_attempts SET account_key = $2 WHERE id = $1`, "other"},
		{"authority", `UPDATE payment_attempts SET authority = $2 WHERE id = $1`, "A" + uniqueSuffixPayment()},
		{"clear_authority", `UPDATE payment_attempts SET authority = $2 WHERE id = $1`, nil},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			if _, err := env.db.Exec(t.Context(), mutation.query, a.ID, mutation.value); err == nil {
				t.Fatal("binding mutation accepted")
			}
		})
	}
	if err := env.repository.SetAuthority(t.Context(), a.ID, *a.Authority); err != nil {
		t.Fatalf("same authority should be idempotent: %v", err)
	}
	if err := env.repository.SetAuthority(t.Context(), a.ID, "A"+uniqueSuffixPayment()); !errors.Is(err, ErrAlreadyProcessed) {
		t.Fatalf("authority reassignment err=%v", err)
	}
	got := env.assertAttempt(t, *a.Authority, StatusPending, 0, nil)
	if got.OrderID != a.OrderID || got.Amount != a.Amount || got.Currency != a.Currency || got.Environment != a.Environment || got.AccountKey != a.AccountKey || got.Provider != a.Provider {
		t.Fatalf("binding changed: %+v", got)
	}
	env.callback(t, *a.Authority, "OK", "success")
	env.assertOrderStock(t, other.ID, order.StatusPending, order.PaymentStatusPending, 9)
}

func TestRepositoryOrderDeletionRetainsImmutablePaymentEvents(t *testing.T) {
	for _, status := range []string{StatusPaid, StatusReconciliation} {
		t.Run(status, func(t *testing.T) {
			env := newTestEnv(t, succeedingClient())
			cookie, _ := env.registerAndLogin(t)
			o := env.createOrder(t, cookie)
			a := env.startPayment(t, cookie, o.ID)
			if status == StatusReconciliation {
				if _, err := env.db.Exec(t.Context(), `UPDATE orders SET total = total + 10000 WHERE id = $1`, o.ID); err != nil {
					t.Fatal(err)
				}
			}
			result, err := env.repository.FinalizeVerifiedPayment(t.Context(), *a.Authority, 123456, 100)
			if err != nil || result.FinalStatus != status {
				t.Fatalf("settlement=%+v err=%v", result, err)
			}
			history := env.paymentEvents(t, a.ID)
			if len(history) == 0 || history[len(history)-1].Kind != status {
				t.Fatal("missing settlement history")
			}
			tag, err := env.db.Exec(t.Context(), `DELETE FROM orders WHERE id = $1`, o.ID)
			if err != nil || tag.RowsAffected() != 1 {
				t.Fatalf("delete order: rows=%d err=%v", tag.RowsAffected(), err)
			}
			if _, err := env.repository.GetAttemptByAuthority(t.Context(), *a.Authority); !errors.Is(err, ErrAttemptNotFound) {
				t.Fatalf("parent attempt was not deleted: %v", err)
			}
			if got := env.paymentEvents(t, a.ID); !reflect.DeepEqual(got, history) {
				t.Fatal("order deletion changed payment history")
			}
			for _, mutation := range []struct {
				name, query string
				args        []any
			}{
				{"update", `UPDATE payment_attempt_events SET snapshot = '{}'::jsonb WHERE attempt_id = $1`, []any{a.ID}},
				{"delete", `DELETE FROM payment_attempt_events WHERE attempt_id = $1`, []any{a.ID}},
				{"empty_update", `UPDATE payment_attempt_events SET snapshot = '{}'::jsonb WHERE FALSE`, nil},
				{"empty_delete", `DELETE FROM payment_attempt_events WHERE FALSE`, nil},
				{"truncate", `TRUNCATE payment_attempt_events`, nil},
				{"truncate_cascade", `TRUNCATE payment_attempt_events CASCADE`, nil},
			} {
				t.Run(mutation.name, func(t *testing.T) {
					tx, err := env.db.Begin(t.Context())
					if err != nil {
						t.Fatal(err)
					}
					defer tx.Rollback(context.Background())
					if _, err := tx.Exec(t.Context(), mutation.query, mutation.args...); err == nil {
						t.Fatal("event mutation accepted after parent deletion")
					}
				})
			}
			if got := env.paymentEvents(t, a.ID); !reflect.DeepEqual(got, history) {
				t.Fatal("rejected mutations changed payment history")
			}
		})
	}
}

func TestRepositoryTerminalSettlementImmutable(t *testing.T) {
	for _, status := range []string{StatusPaid, StatusReconciliation} {
		t.Run(status, func(t *testing.T) {
			env := newTestEnv(t, succeedingClient())
			cookie, _ := env.registerAndLogin(t)
			o := env.createOrder(t, cookie)
			a := env.startPayment(t, cookie, o.ID)
			if status == StatusReconciliation {
				if _, err := env.db.Exec(t.Context(), `UPDATE orders SET total = total + 10000 WHERE id = $1`, o.ID); err != nil {
					t.Fatal(err)
				}
			}
			result, err := env.repository.FinalizeVerifiedPayment(t.Context(), *a.Authority, 123456, 100)
			if err != nil || result.FinalStatus != status {
				t.Fatalf("settlement=%+v err=%v", result, err)
			}
			code := 100
			before := env.assertAttempt(t, *a.Authority, status, 123456, &code)
			history := env.paymentEvents(t, a.ID)
			otherTerminal := StatusReconciliation
			if status == StatusReconciliation {
				otherTerminal = StatusPaid
			}
			for _, mutation := range []struct {
				name, query string
				value       any
			}{
				{"pending", `UPDATE payment_attempts SET status = $2 WHERE id = $1`, StatusPending},
				{"failed", `UPDATE payment_attempts SET status = $2 WHERE id = $1`, StatusFailed},
				{"other_terminal", `UPDATE payment_attempts SET status = $2 WHERE id = $1`, otherTerminal},
				{"reference", `UPDATE payment_attempts SET ref_id = $2 WHERE id = $1`, int64(123457)},
				{"clear_reference", `UPDATE payment_attempts SET ref_id = $2 WHERE id = $1`, nil},
				{"code", `UPDATE payment_attempts SET provider_code = $2 WHERE id = $1`, 101},
				{"clear_code", `UPDATE payment_attempts SET provider_code = $2 WHERE id = $1`, nil},
				{"verified_at", `UPDATE payment_attempts SET verified_at = $2 WHERE id = $1`, before.VerifiedAt.Add(time.Hour)},
				{"clear_verified_at", `UPDATE payment_attempts SET verified_at = $2 WHERE id = $1`, nil},
				{"reset", `UPDATE payment_attempts SET status = $2, ref_id = NULL, provider_code = NULL, verified_at = NULL WHERE id = $1`, StatusPending},
			} {
				t.Run(mutation.name, func(t *testing.T) {
					if _, err := env.db.Exec(t.Context(), mutation.query, a.ID, mutation.value); err == nil {
						t.Fatal("terminal settlement mutation accepted")
					}
					if got := env.assertAttempt(t, *a.Authority, status, 123456, &code); !reflect.DeepEqual(got, before) {
						t.Fatal("rejected mutation changed terminal attempt")
					}
				})
			}
			result, err = env.repository.FinalizeVerifiedPayment(t.Context(), *a.Authority, 123457, 101)
			if err != nil || !result.AlreadySettled || result.FinalStatus != status {
				t.Fatalf("terminal replay=%+v err=%v", result, err)
			}
			failureCode := -51
			result, err = env.repository.FinalizeFailedPayment(t.Context(), *a.Authority, &failureCode)
			if err != nil || !result.AlreadySettled || result.FinalStatus != status {
				t.Fatalf("terminal failure replay=%+v err=%v", result, err)
			}
			if got := env.assertAttempt(t, *a.Authority, status, 123456, &code); !reflect.DeepEqual(got, before) {
				t.Fatal("terminal replay changed settlement")
			}
			if got := env.paymentEvents(t, a.ID); !reflect.DeepEqual(got, history) {
				t.Fatal("terminal mutation or replay changed history")
			}
		})
	}
}

func TestRepositorySameReferenceAcrossOrdersQuarantines(t *testing.T) {
	for _, concurrent := range []bool{false, true} {
		t.Run(fmt.Sprintf("concurrent=%t", concurrent), func(t *testing.T) {
			env := newTestEnv(t, succeedingClient())
			cookie, _ := env.registerAndLogin(t)
			orders := []order.Order{env.createOrder(t, cookie), env.createOrder(t, cookie)}
			attempts := []Attempt{env.startPayment(t, cookie, orders[0].ID), env.startPayment(t, cookie, orders[1].ID)}
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			type result struct {
				value VerifyResult
				err   error
			}
			results := make(chan result, 2)
			start := make(chan struct{})
			for _, a := range attempts {
				finalize := func() {
					value, err := env.repository.FinalizeVerifiedPayment(ctx, *a.Authority, 123456, 100)
					results <- result{value, err}
				}
				if concurrent {
					go func() { <-start; finalize() }()
				} else {
					finalize()
				}
			}
			close(start)
			paid, quarantined := 0, 0
			for range attempts {
				got := <-results
				if got.err != nil {
					t.Fatal(got.err)
				}
				switch got.value.FinalStatus {
				case StatusPaid:
					paid++
					env.assertOrderStock(t, got.value.OrderID, order.StatusPending, order.PaymentStatusPaid, 9)
				case StatusReconciliation:
					quarantined++
					env.assertOrderStock(t, got.value.OrderID, order.StatusPending, order.PaymentStatusPending, 9)
				default:
					t.Fatalf("unexpected result: %+v", got.value)
				}
			}
			if paid != 1 || quarantined != 1 {
				t.Fatalf("paid=%d quarantined=%d", paid, quarantined)
			}
			for _, a := range attempts {
				got, err := env.repository.GetAttemptByAuthority(t.Context(), *a.Authority)
				if err != nil || got.RefID == nil || *got.RefID != 123456 || got.ProviderCode == nil || *got.ProviderCode != 100 || got.VerifiedAt == nil {
					t.Fatalf("lost verification evidence: %+v err=%v", got, err)
				}
			}
		})
	}
}
