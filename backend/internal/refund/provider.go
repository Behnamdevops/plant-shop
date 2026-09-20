package refund

import (
	"context"
	"encoding/json"
	"sync"
)

// RefundProvider is the interface for refund processing.
// Implementations handle the actual call to the payment provider's refund API.
type RefundProvider interface {
	// Refund attempts to process a refund for the given order.
	// It returns the provider result and any error encountered.
	// The caller is responsible for persisting the result and updating
	// the refund status atomically.
	Refund(ctx context.Context, input RefundInput) (RefundResult, error)
}

// RefundInput contains the data needed to process a refund.
type RefundInput struct {
	OrderID          int64
	Amount           int64
	PaymentMethod    string
	ProviderRefundID *string
}

// RefundResult contains the result of a refund attempt.
type RefundResult struct {
	Success          bool
	ProviderRefundID *string
	ErrorMessage     string
	ProviderResponse json.RawMessage
}

// ZarinPalRefundProvider implements refund processing for ZarinPal.
// IMPORTANT: As of current knowledge, ZarinPal does NOT provide a refund API endpoint.
// This implementation uses a manual_review workflow where refunds must be processed
// manually through the ZarinPal dashboard or bank transfer.
//
// If ZarinPal adds official refund API support, this provider should be updated
// to use that endpoint with proper idempotency handling.
type ZarinPalRefundProvider struct {
	// MerchantID is the ZarinPal merchant ID (for future use if API changes)
	MerchantID string
}

// NewZarinPalRefundProvider creates a new ZarinPal refund provider.
func NewZarinPalRefundProvider(merchantID string) *ZarinPalRefundProvider {
	return &ZarinPalRefundProvider{MerchantID: merchantID}
}

// Refund processes a refund for ZarinPal payments.
// Since ZarinPal does not provide a refund API, this implementation:
// 1. Returns Success=false with an error message indicating manual review is required
// 2. The refund status will be set to 'manual_review' for admin action
func (p *ZarinPalRefundProvider) Refund(ctx context.Context, input RefundInput) (RefundResult, error) {
	// ZarinPal does not provide a refund API endpoint as of current documentation.
	// Refunds must be processed manually through the ZarinPal dashboard or bank transfer.
	// This provider returns a result that marks the refund for manual review.

	providerResponse := map[string]interface{}{
		"order_id":       input.OrderID,
		"amount":         input.Amount,
		"payment_method": input.PaymentMethod,
		"refund_status":  "requires_manual_review",
		"note":           "ZarinPal does not provide a refund API. Process refund manually through ZarinPal dashboard or bank transfer.",
	}

	responseBytes, _ := json.Marshal(providerResponse)

	return RefundResult{
		Success:          false,
		ErrorMessage:     "ZarinPal does not provide a refund API. Manual review required.",
		ProviderResponse: responseBytes,
	}, nil
}

// ManualRefundProvider is a provider for manual payment methods.
// It immediately succeeds since there's no external API call needed.
type ManualRefundProvider struct{}

// NewManualRefundProvider creates a new manual refund provider.
func NewManualRefundProvider() *ManualRefundProvider {
	return &ManualRefundProvider{}
}

// Refund processes a refund for manual payment methods.
// Since the payment was made manually (cash, bank transfer, etc.), the refund
// is processed by the admin and this provider just confirms completion.
func (p *ManualRefundProvider) Refund(ctx context.Context, input RefundInput) (RefundResult, error) {
	// For manual payments, there's no external API call needed.
	// The refund is processed by the admin (cash refund, bank transfer, etc.)
	// and this provider confirms completion.

	providerResponse := map[string]interface{}{
		"order_id":       input.OrderID,
		"amount":         input.Amount,
		"payment_method": input.PaymentMethod,
		"refund_status":  "manual_refund_processed",
		"note":           "Refund processed manually by admin.",
	}

	responseBytes, _ := json.Marshal(providerResponse)

	return RefundResult{
		Success:          true,
		ProviderResponse: responseBytes,
	}, nil
}

// FakeRefundProvider is a test implementation that simulates refund behavior.
// It never makes a real network call; it is used exclusively in tests to
// stand in for ZarinPal or any other real provider.
type FakeRefundProvider struct {
	ShouldSucceed bool
	Delay         int // milliseconds to delay (for testing)

	mu        sync.Mutex
	callCount int
}

// NewFakeRefundProvider creates a fake refund provider for testing.
func NewFakeRefundProvider(shouldSucceed bool) *FakeRefundProvider {
	return &FakeRefundProvider{
		ShouldSucceed: shouldSucceed,
	}
}

// Refund simulates a refund for testing purposes.
func (p *FakeRefundProvider) Refund(ctx context.Context, input RefundInput) (RefundResult, error) {
	p.mu.Lock()
	p.callCount++
	p.mu.Unlock()

	providerResponse := map[string]interface{}{
		"order_id":       input.OrderID,
		"amount":         input.Amount,
		"payment_method": input.PaymentMethod,
		"refund_status":  "success",
	}

	responseBytes, _ := json.Marshal(providerResponse)

	if p.ShouldSucceed {
		return RefundResult{
			Success:          true,
			ProviderResponse: responseBytes,
		}, nil
	}

	return RefundResult{
		Success:          false,
		ErrorMessage:     "refund failed (simulated)",
		ProviderResponse: responseBytes,
	}, nil
}

// calls returns the number of times Refund has been invoked. Intended for
// test assertions about provider-call exclusivity.
func (p *FakeRefundProvider) calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.callCount
}
