package notification

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"sync"
	"time"
)

// Worker processes notification outbox events.
type Worker struct {
	repo     *Repository
	sender   Sender
	interval time.Duration
	maxRetry int

	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	stopped    bool
}

// WorkerConfig holds worker configuration.
type WorkerConfig struct {
	Interval  time.Duration
	MaxRetry  int
	BatchSize int
}

// DefaultWorkerConfig returns default worker configuration.
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		Interval:  10 * time.Second,
		MaxRetry:  5,
		BatchSize: 10,
	}
}

// NewWorker creates a new notification worker.
func NewWorker(repo *Repository, sender Sender, cfg WorkerConfig) *Worker {
	if cfg.Interval <= 0 {
		cfg.Interval = 10 * time.Second
	}
	if cfg.MaxRetry <= 0 {
		cfg.MaxRetry = 5
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 10
	}

	return &Worker{
		repo:     repo,
		sender:   sender,
		interval: cfg.Interval,
		maxRetry: cfg.MaxRetry,
	}
}

// Start starts the worker in the background.
func (w *Worker) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	w.cancelFunc = cancel

	w.wg.Add(1)
	go w.run(ctx)
}

// Stop gracefully stops the worker.
func (w *Worker) Stop(ctx context.Context) {
	if w.stopped {
		return
	}
	w.stopped = true

	if w.cancelFunc != nil {
		w.cancelFunc()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.wg.Wait()
	}()

	select {
	case <-done:
	case <-ctx.Done():
		// Timeout waiting for worker to stop
	}
}

func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *Worker) processBatch(ctx context.Context) {
	// Fetch pending notifications
	notifications, err := w.repo.GetPendingDue(ctx)
	if err != nil {
		slog.Error("notification worker: failed to fetch pending notifications", "error", err)
		return
	}

	if len(notifications) == 0 {
		return
	}

	slog.Info("notification worker: processing batch", "count", len(notifications))

	for _, n := range notifications {
		if ctx.Err() != nil {
			return
		}

		w.processOne(ctx, &n)
	}
}

func (w *Worker) processOne(ctx context.Context, n *NotificationOutbox) {
	// Build email message
	message := w.buildMessage(n)

	// Send email
	err := w.sender.Send(ctx, message)
	if err != nil {
		w.handleSendFailure(ctx, n, err)
		return
	}

	// Mark as sent
	sentAt := time.Now()
	if err := w.repo.MarkSent(ctx, n.ID, sentAt); err != nil {
		slog.Error("notification worker: failed to mark notification as sent",
			"id", n.ID, "event_key", n.EventKey, "error", err)
	}
}

func (w *Worker) handleSendFailure(ctx context.Context, n *NotificationOutbox, err error) {
	lastError := err.Error()
	slog.Error("notification worker: failed to send notification",
		"id", n.ID, "event_key", n.EventKey, "attempts", n.Attempts, "error", err)

	// Check if we should mark as permanently failed
	if n.Attempts >= w.maxRetry {
		if err := w.repo.MarkPermanentFailure(ctx, n.ID, lastError); err != nil {
			slog.Error("notification worker: failed to mark notification as permanently failed",
				"id", n.ID, "error", err)
		}
		return
	}

	// Calculate backoff: exponential with base 2, capped at 1 hour
	// Attempts 1-4: backoff increases, attempt 5+ uses max backoff
	backoffSeconds := 1 << n.Attempts // 2, 4, 8, 16, 32...
	if backoffSeconds > 3600 {
		backoffSeconds = 3600
	}

	nextAttemptAt := time.Now().Add(time.Duration(backoffSeconds) * time.Second)

	if err := w.repo.MarkFailed(ctx, n.ID, lastError, nextAttemptAt); err != nil {
		slog.Error("notification worker: failed to mark notification as failed for retry",
			"id", n.ID, "error", err)
	}
}

func (w *Worker) buildMessage(n *NotificationOutbox) Message {
	customerName := "مشتری"
	if val, ok := n.Payload["customer_name"].(string); ok {
		customerName = val
	}

	orderID := int64(0)
	if val, ok := n.Payload["order_id"].(float64); ok {
		orderID = int64(val)
	} else if val, ok := n.Payload["order_id"].(int64); ok {
		orderID = val
	}

	subject := "اعلان - درخت‌فروشی"
	html := "<p>اعلان جدید</p>"
	plain := "اعلان جدید"

	switch n.EventType {
	case EventTypeOrderCreated:
		subject = "سفارش جدید ثبت شد - درخت‌فروشی"
		html = orderCreatedHTML
		plain = orderCreatedPlain

	case EventTypePaymentSucceeded:
		subject = "پرداخت موفق - درخت‌فروشی"
		html = paymentSucceededHTML
		plain = paymentSucceededPlain

	case EventTypePaymentFailed:
		subject = "پرداخت ناموفق - درخت‌فروشی"
		html = paymentFailedHTML
		plain = paymentFailedPlain

	case EventTypeOrderProcessing:
		subject = "وضعیت سفارش: در حال پردازش - درخت‌فروشی"
		html = orderStatusHTML
		plain = orderStatusPlain

	case EventTypeOrderShipped:
		subject = "وضعیت سفارش: ارسال شد - درخت‌فروشی"
		html = orderStatusHTML
		plain = orderStatusPlain

	case EventTypeOrderDelivered:
		subject = "وضعیت سفارش: تحویل داده شد - درخت‌فروشی"
		html = orderStatusHTML
		plain = orderStatusPlain

	case EventTypeOrderCancelled:
		subject = "سفارش لغو شد - درخت‌فروشی"
		html = orderCancelledHTML
		plain = orderCancelledPlain

	default:
		subject = "اعلان - درخت‌فروشی"
		html = fmt.Sprintf("<p>نوع اعلان: %s</p>", n.EventType)
		plain = fmt.Sprintf("نوع اعلان: %s", n.EventType)
	}

	// Create template data
	data := EmailTemplateData{
		RecipientName: customerName,
		CustomerName:  customerName,
		OrderID:       orderID,
	}

	// Parse and execute template
	tmpl, err := template.New("email").Parse(html)
	if err == nil {
		htmlBuf := &bytes.Buffer{}
		if err := tmpl.Execute(htmlBuf, data); err == nil {
			html = htmlBuf.String()
		}
	}

	// Parse plain text template
	tmplPlain, err := template.New("plain").Parse(plain)
	if err == nil {
		plainBuf := &bytes.Buffer{}
		if err := tmplPlain.Execute(plainBuf, data); err == nil {
			plain = plainBuf.String()
		}
	}

	return Message{
		To:      n.RecipientEmail,
		Subject: subject,
		HTML:    html,
		Plain:   plain,
	}
}
