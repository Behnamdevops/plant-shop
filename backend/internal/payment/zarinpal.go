package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ZarinPal v4 REST endpoints, confirmed against the current official
// documentation (zarinpal.com/docs/sdk/php/method/request,
// zarinpal.com/docs/sdk/php/method/verify) and ZarinPal-Lab's own sample
// code/OSS clients as of this integration. Amounts are always integers in
// Rials (IRR); ZarinPal enforces a minimum of 10,000 Rials per payment.
const (
	baseURLProduction = "https://payment.zarinpal.com/pg/v4/payment/"
	baseURLSandbox    = "https://sandbox.zarinpal.com/pg/v4/payment/"

	startPayURLProduction = "https://www.zarinpal.com/pg/StartPay/"
	startPayURLSandbox    = "https://sandbox.zarinpal.com/pg/StartPay/"

	requestPath = "request.json"
	verifyPath  = "verify.json"
)

// Success/idempotent response codes from ZarinPal's `data.code` field.
// 100 = the operation just succeeded (payment authorized, or verified for
// the first time). 101 = specifically returned by verify.json when the
// transaction was already verified previously — ZarinPal explicitly
// documents this as a success-equivalent code so a duplicate/retried
// verify call is not treated as an error. All other codes (in particular,
// every negative code) are failures.
const (
	codeSuccess         = 100
	codeAlreadyVerified = 101
)

// RequestPaymentInput is the information needed to start a new ZarinPal
// payment. Amount is in Rials.
type RequestPaymentInput struct {
	Amount      int64
	Description string
	CallbackURL string
	Mobile      string
	Email       string
}

// RequestPaymentOutput is ZarinPal's response to a successful payment
// request: the authority to persist and use for verification, and the
// full URL to redirect the browser to.
type RequestPaymentOutput struct {
	Authority   string
	RedirectURL string
	Code        int
	Message     string
}

// VerifyPaymentInput is the information needed to verify a completed
// payment. Amount MUST be the amount that was actually requested (read
// from our own persisted payment_attempts row), never a value supplied by
// the browser/callback.
type VerifyPaymentInput struct {
	Authority string
	Amount    int64
}

// VerifyPaymentOutput is ZarinPal's response to a verification call.
// AlreadyVerified is true when ZarinPal's response code was 101 (the
// transaction had already been verified in a previous call) — callers
// should treat this as success but must not re-run any paid-order side
// effects a second time.
type VerifyPaymentOutput struct {
	Code            int
	Message         string
	RefID           int64
	AlreadyVerified bool
}

// Client is the interface the payment package depends on for talking to
// ZarinPal, so tests can substitute a fake implementation instead of
// making real network calls (there is no live-gateway dependency in
// automated tests).
type Client interface {
	RequestPayment(ctx context.Context, in RequestPaymentInput) (RequestPaymentOutput, error)
	VerifyPayment(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error)
}

// ZarinPalClient is the real Client implementation, talking to ZarinPal's
// v4 REST API over stdlib net/http (no third-party ZarinPal SDK — see
// AGENTS.md/task instructions on avoiding unnecessary dependencies).
type ZarinPalClient struct {
	MerchantID string
	Sandbox    bool
	httpClient *http.Client
}

// NewZarinPalClient constructs a ZarinPalClient with a bounded-timeout
// HTTP client, so a slow/unresponsive ZarinPal never hangs a request
// indefinitely.
func NewZarinPalClient(merchantID string, sandbox bool) *ZarinPalClient {
	return &ZarinPalClient{
		MerchantID: merchantID,
		Sandbox:    sandbox,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *ZarinPalClient) baseURL() string {
	if c.Sandbox {
		return baseURLSandbox
	}
	return baseURLProduction
}

func (c *ZarinPalClient) startPayURL() string {
	if c.Sandbox {
		return startPayURLSandbox
	}
	return startPayURLProduction
}

// zarinpalMetadata carries optional contact fields under ZarinPal's
// "metadata" object, per the current request.json contract.
type zarinpalMetadata struct {
	Mobile string `json:"mobile,omitempty"`
	Email  string `json:"email,omitempty"`
}

type paymentRequestBody struct {
	MerchantID  string            `json:"merchant_id"`
	Amount      int64             `json:"amount"`
	Description string            `json:"description"`
	CallbackURL string            `json:"callback_url"`
	Metadata    *zarinpalMetadata `json:"metadata,omitempty"`
}

type paymentVerifyBody struct {
	MerchantID string `json:"merchant_id"`
	Amount     int64  `json:"amount"`
	Authority  string `json:"authority"`
}

// zarinpalErrors mirrors ZarinPal's top-level "errors" object, returned
// instead of "data" when the request itself is rejected (e.g. invalid
// merchant_id, validation failure) rather than merely unsuccessful.
type zarinpalErrors struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type paymentRequestResponse struct {
	Data struct {
		Code      int    `json:"code"`
		Message   string `json:"message"`
		Authority string `json:"authority"`
	} `json:"data"`
	Errors zarinpalErrors `json:"errors"`
}

type paymentVerifyResponse struct {
	Data struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		RefID   int64  `json:"ref_id"`
	} `json:"data"`
	Errors zarinpalErrors `json:"errors"`
}

// RequestPayment calls ZarinPal's payment/request.json to obtain an
// authority and redirect URL for a new payment. It never treats a
// malformed/partial response as success: a missing authority alongside a
// success code, or an HTTP-level failure, is always returned as an error.
func (c *ZarinPalClient) RequestPayment(ctx context.Context, in RequestPaymentInput) (RequestPaymentOutput, error) {
	var metadata *zarinpalMetadata
	if in.Mobile != "" || in.Email != "" {
		metadata = &zarinpalMetadata{Mobile: in.Mobile, Email: in.Email}
	}

	body := paymentRequestBody{
		MerchantID:  c.MerchantID,
		Amount:      in.Amount,
		Description: in.Description,
		CallbackURL: in.CallbackURL,
		Metadata:    metadata,
	}

	var resp paymentRequestResponse
	if err := c.post(ctx, requestPath, body, &resp); err != nil {
		return RequestPaymentOutput{}, err
	}

	if resp.Data.Code != codeSuccess || resp.Data.Authority == "" {
		msg := resp.Data.Message
		if msg == "" {
			msg = resp.Errors.Message
		}
		return RequestPaymentOutput{}, fmt.Errorf("zarinpal payment request rejected (code=%d): %s", resp.Data.Code, msg)
	}

	return RequestPaymentOutput{
		Authority:   resp.Data.Authority,
		RedirectURL: c.startPayURL() + resp.Data.Authority,
		Code:        resp.Data.Code,
		Message:     resp.Data.Message,
	}, nil
}

// VerifyPayment calls ZarinPal's payment/verify.json to confirm a
// completed payment server-to-server. amount must be the amount originally
// requested for this authority (read from our own database), never a
// value supplied by the browser. A response code of 101 ("already
// verified") is reported as success via AlreadyVerified rather than an
// error, per ZarinPal's documented semantics.
func (c *ZarinPalClient) VerifyPayment(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error) {
	body := paymentVerifyBody{
		MerchantID: c.MerchantID,
		Amount:     in.Amount,
		Authority:  in.Authority,
	}

	var resp paymentVerifyResponse
	if err := c.post(ctx, verifyPath, body, &resp); err != nil {
		return VerifyPaymentOutput{}, err
	}

	switch resp.Data.Code {
	case codeSuccess:
		return VerifyPaymentOutput{Code: resp.Data.Code, Message: resp.Data.Message, RefID: resp.Data.RefID}, nil
	case codeAlreadyVerified:
		return VerifyPaymentOutput{Code: resp.Data.Code, Message: resp.Data.Message, RefID: resp.Data.RefID, AlreadyVerified: true}, nil
	default:
		msg := resp.Data.Message
		if msg == "" {
			msg = resp.Errors.Message
		}
		return VerifyPaymentOutput{Code: resp.Data.Code, Message: msg}, nil
	}
}

// post sends a JSON POST request to ZarinPal and decodes the response into
// out. It never logs the request body (which contains the Merchant ID) and
// treats any non-2xx status or malformed JSON body as an error rather than
// guessing at success.
func (c *ZarinPalClient) post(ctx context.Context, path string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("zarinpal: failed to encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL()+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("zarinpal: failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("zarinpal: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("zarinpal: failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("zarinpal: unexpected HTTP status %d", resp.StatusCode)
	}

	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("zarinpal: malformed response body: %w", err)
	}

	return nil
}

// ErrProviderUnavailable is a sentinel wrapper callers may use to detect
// "we could not reach ZarinPal at all" versus "ZarinPal responded with a
// rejection", though V1 callers currently treat both the same way (fail
// the attempt, never mark paid).
var ErrProviderUnavailable = errors.New("zarinpal: provider unavailable")
