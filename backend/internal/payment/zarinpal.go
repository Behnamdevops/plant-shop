package payment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	baseURLProduction     = "https://payment.zarinpal.com/pg/v4/payment/"
	baseURLSandbox        = "https://sandbox.zarinpal.com/pg/v4/payment/"
	startPayURLProduction = "https://www.zarinpal.com/pg/StartPay/"
	startPayURLSandbox    = "https://sandbox.zarinpal.com/pg/StartPay/"
	requestPath           = "request.json"
	verifyPath            = "verify.json"
	codeSuccess           = 100
	codeAlreadyVerified   = 101
	maxResponseBytes      = 1 << 20
	minimumAmountIRR      = 10000
)

type RequestPaymentInput struct {
	Amount      int64
	Description string
	CallbackURL string
	Mobile      string
	Email       string
}

type RequestPaymentOutput struct {
	Authority   string
	RedirectURL string
	Code        int
	Message     string
}

type VerifyPaymentInput struct {
	Authority string
	Amount    int64
}

type VerifyPaymentOutput struct {
	Code            int
	Message         string
	RefID           int64
	AlreadyVerified bool
}

type Client interface {
	RequestPayment(ctx context.Context, in RequestPaymentInput) (RequestPaymentOutput, error)
	VerifyPayment(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error)
}

type ProviderError struct {
	Code int
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("zarinpal: provider rejected payment (code=%d)", e.Code)
}

var ErrProviderUnavailable = errors.New("zarinpal: provider unavailable")
var errInvalidResponse = errors.New("zarinpal: invalid provider response")

type ZarinPalClient struct {
	MerchantID string
	Sandbox    bool
	httpClient *http.Client
}

func NewZarinPalClient(merchantID string, sandbox bool) *ZarinPalClient {
	return &ZarinPalClient{
		MerchantID: strings.ToLower(merchantID),
		Sandbox:    sandbox,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *ZarinPalClient) Environment() string {
	if c.Sandbox {
		return "sandbox"
	}
	return "live"
}

func (c *ZarinPalClient) Identity() string {
	sum := sha256.Sum256([]byte(strings.ToLower(c.MerchantID) + "\n" + c.Environment()))
	return hex.EncodeToString(sum[:])
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

type zarinpalMetadata struct {
	Mobile string `json:"mobile,omitempty"`
	Email  string `json:"email,omitempty"`
}

type paymentRequestBody struct {
	MerchantID  string            `json:"merchant_id"`
	Amount      int64             `json:"amount"`
	Currency    string            `json:"currency"`
	Description string            `json:"description"`
	CallbackURL string            `json:"callback_url"`
	Metadata    *zarinpalMetadata `json:"metadata,omitempty"`
}

type paymentVerifyBody struct {
	MerchantID string `json:"merchant_id"`
	Amount     int64  `json:"amount"`
	Authority  string `json:"authority"`
}

func (c *ZarinPalClient) RequestPayment(ctx context.Context, in RequestPaymentInput) (RequestPaymentOutput, error) {
	if in.Amount < minimumAmountIRR {
		return RequestPaymentOutput{}, errors.New("zarinpal: amount must be at least 10000 IRR")
	}
	var metadata *zarinpalMetadata
	if in.Mobile != "" || in.Email != "" {
		metadata = &zarinpalMetadata{Mobile: in.Mobile, Email: in.Email}
	}
	resp, err := c.post(ctx, requestPath, paymentRequestBody{
		MerchantID:  c.MerchantID,
		Amount:      in.Amount,
		Currency:    "IRR",
		Description: in.Description,
		CallbackURL: in.CallbackURL,
		Metadata:    metadata,
	})
	if err != nil {
		return RequestPaymentOutput{}, err
	}
	if resp.code < 0 {
		return RequestPaymentOutput{}, &ProviderError{Code: resp.code}
	}
	authority, ok := requiredString(resp.data, "authority")
	if resp.code != codeSuccess || !ok || !validAuthority(authority) {
		return RequestPaymentOutput{}, errInvalidResponse
	}
	return RequestPaymentOutput{
		Authority:   authority,
		RedirectURL: c.startPayURL() + authority,
		Code:        resp.code,
		Message:     resp.message,
	}, nil
}

func (c *ZarinPalClient) VerifyPayment(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error) {
	if in.Amount < minimumAmountIRR {
		return VerifyPaymentOutput{}, errors.New("zarinpal: amount must be at least 10000 IRR")
	}
	if !validAuthority(in.Authority) {
		return VerifyPaymentOutput{}, errors.New("zarinpal: invalid authority")
	}
	resp, err := c.post(ctx, verifyPath, paymentVerifyBody{
		MerchantID: c.MerchantID,
		Amount:     in.Amount,
		Authority:  in.Authority,
	})
	if err != nil {
		return VerifyPaymentOutput{}, err
	}
	if resp.code < 0 {
		return VerifyPaymentOutput{Code: resp.code}, nil
	}
	refID, ok := requiredInteger(resp.data, "ref_id")
	if !ok || refID <= 0 || resp.code != codeSuccess && resp.code != codeAlreadyVerified {
		return VerifyPaymentOutput{}, errInvalidResponse
	}
	return VerifyPaymentOutput{
		Code:            resp.code,
		Message:         resp.message,
		RefID:           refID,
		AlreadyVerified: resp.code == codeAlreadyVerified,
	}, nil
}

func validAuthority(authority string) bool {
	if len(authority) == 0 || len(authority) > 255 {
		return false
	}
	for _, ch := range authority {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9') {
			return false
		}
	}
	return true
}

type providerResponse struct {
	code    int
	message string
	data    map[string]any
}

func (c *ZarinPalClient) post(ctx context.Context, path string, body any) (providerResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return providerResponse{}, errors.New("zarinpal: cannot encode request")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL()+path, bytes.NewReader(payload))
	if err != nil {
		return providerResponse{}, errors.New("zarinpal: cannot build request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return providerResponse{}, ErrProviderUnavailable
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return providerResponse{}, ErrProviderUnavailable
	}
	if len(respBody) > maxResponseBytes {
		return providerResponse{}, errInvalidResponse
	}
	decoded, err := decodeProviderResponse(respBody)
	if err != nil {
		return providerResponse{}, err
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 || (resp.StatusCode < 200 || resp.StatusCode >= 300) && decoded.code >= 0 {
		return providerResponse{}, errors.New("zarinpal: unexpected HTTP status")
	}
	return decoded, nil
}

func decodeProviderResponse(body []byte) (providerResponse, error) {
	if !utf8.Valid(body) {
		return providerResponse{}, errInvalidResponse
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	value, err := decodeUniqueJSON(decoder, 0)
	if err != nil {
		return providerResponse{}, errInvalidResponse
	}
	if _, err := decoder.Token(); err != io.EOF {
		return providerResponse{}, errInvalidResponse
	}
	envelope, ok := value.(map[string]any)
	if !ok {
		return providerResponse{}, errInvalidResponse
	}
	dataValue, hasData := envelope["data"]
	errorsValue, hasErrors := envelope["errors"]
	if !hasData || !hasErrors || dataValue == nil || errorsValue == nil {
		return providerResponse{}, errInvalidResponse
	}
	if rejection, ok := errorsValue.(map[string]any); ok {
		data, empty := dataValue.([]any)
		code, message, valid := responseCode(rejection)
		if !empty || len(data) != 0 || !valid || code >= 0 || !validResponseFields(rejection) {
			return providerResponse{}, errInvalidResponse
		}
		if _, present := rejection["authority"]; present {
			return providerResponse{}, errInvalidResponse
		}
		if _, present := rejection["ref_id"]; present {
			return providerResponse{}, errInvalidResponse
		}
		return providerResponse{code: code, message: message}, nil
	}
	errorsArray, ok := errorsValue.([]any)
	if !ok || len(errorsArray) != 0 {
		return providerResponse{}, errInvalidResponse
	}
	data, ok := dataValue.(map[string]any)
	if !ok || !validResponseFields(data) {
		return providerResponse{}, errInvalidResponse
	}
	code, message, ok := responseCode(data)
	if !ok || code == 0 || code > 0 && code != codeSuccess && code != codeAlreadyVerified {
		return providerResponse{}, errInvalidResponse
	}
	if code < 0 {
		if _, present := data["authority"]; present {
			return providerResponse{}, errInvalidResponse
		}
		if _, present := data["ref_id"]; present {
			return providerResponse{}, errInvalidResponse
		}
	}
	return providerResponse{code: code, message: message, data: data}, nil
}

func responseCode(data map[string]any) (int, string, bool) {
	code, ok := requiredInteger(data, "code")
	message, valid := requiredString(data, "message")
	return int(code), message, ok && valid && int64(int(code)) == code
}

func requiredString(data map[string]any, key string) (string, bool) {
	value, ok := data[key].(string)
	return value, ok && strings.TrimSpace(value) != ""
}

func requiredInteger(data map[string]any, key string) (int64, bool) {
	value, ok := data[key].(json.Number)
	if !ok {
		return 0, false
	}
	number, err := strconv.ParseInt(string(value), 10, 64)
	return number, err == nil
}

func validResponseFields(data map[string]any) bool {
	for _, key := range []string{"authority", "fee_type", "card_hash", "card_pan"} {
		if _, exists := data[key]; exists {
			if _, ok := requiredString(data, key); !ok {
				return false
			}
		}
	}
	for _, key := range []string{"fee", "ref_id"} {
		if _, exists := data[key]; exists {
			if value, ok := requiredInteger(data, key); !ok || value < 0 {
				return false
			}
		}
	}
	if validations, exists := data["validations"]; exists {
		switch validations.(type) {
		case map[string]any, []any:
		default:
			return false
		}
	}
	return true
}

func decodeUniqueJSON(decoder *json.Decoder, depth int) (any, error) {
	if depth > 64 {
		return nil, errInvalidResponse
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch token {
	case json.Delim('{'):
		object := make(map[string]any)
		seen := make(map[string]bool)
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := token.(string)
			if !ok || seen[strings.ToLower(key)] {
				return nil, errInvalidResponse
			}
			lowerKey := strings.ToLower(key)
			switch lowerKey {
			case "data", "errors", "code", "message", "authority", "ref_id", "fee", "fee_type", "card_hash", "card_pan", "validations":
				if key != lowerKey {
					return nil, errInvalidResponse
				}
			}
			seen[lowerKey] = true
			value, err := decodeUniqueJSON(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
			return nil, errInvalidResponse
		}
		return object, nil
	case json.Delim('['):
		array := make([]any, 0)
		for decoder.More() {
			value, err := decodeUniqueJSON(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		if end, err := decoder.Token(); err != nil || end != json.Delim(']') {
			return nil, errInvalidResponse
		}
		return array, nil
	default:
		if _, ok := token.(json.Delim); ok {
			return nil, errInvalidResponse
		}
		return token, nil
	}
}
