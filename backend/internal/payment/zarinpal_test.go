package payment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

const contractAuthority = "A0000000000000000000000000000wwOGYpd"
const contractRequest = `{"data":{"code":100,"message":"Success","authority":"A0000000000000000000000000000wwOGYpd","fee_type":"Merchant","fee":100},"errors":[]}`
const contractVerify = `{"data":{"code":100,"message":"Verified","ref_id":201,"card_hash":"1EBE3EBEBE35D3A3EFC6E6E1CB338A7C30E5C88C0D83D608B64E894CDFD1513","card_pan":"502229******5998","fee_type":"Merchant","fee":0},"errors":[]}`
const sdkRejection = `{"data":[],"errors":{"code":-9,"message":"merchant-secret provider-text","validations":{"merchant_id":["merchant-secret"]}}}`

type contractTransport func(*http.Request) (*http.Response, error)

func (f contractTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func contractClient(t *testing.T, sandbox bool, handler http.HandlerFunc) *ZarinPalClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := NewZarinPalClient("merchant-secret", sandbox)
	transport := &http.Transport{Proxy: nil}
	t.Cleanup(transport.CloseIdleConnections)
	client.httpClient.Transport = contractTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != client.baseURL()+requestPath && r.URL.String() != client.baseURL()+verifyPath {
			return nil, errors.New("unexpected destination")
		}
		clone := r.Clone(r.Context())
		clone.URL.Scheme = target.Scheme
		clone.URL.Host = target.Host
		return transport.RoundTrip(clone)
	})
	return client
}

func responseClient(t *testing.T, body string, status int) *ZarinPalClient {
	t.Helper()
	return contractClient(t, true, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	})
}

func TestZarinPalOfficialRequestContract(t *testing.T) {
	for _, sandbox := range []bool{true, false} {
		for _, metadata := range []bool{true, false} {
			t.Run(fmt.Sprintf("sandbox=%t/metadata=%t", sandbox, metadata), func(t *testing.T) {
				client := contractClient(t, sandbox, func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodPost || r.URL.Path != "/pg/v4/payment/request.json" || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Accept") != "application/json" {
						t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					}
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body["merchant_id"] != "merchant-secret" || body["amount"] != float64(10000) || body["currency"] != "IRR" || body["description"] != "Order 1" || body["callback_url"] != "https://example.test/api/v1/payments/zarinpal/callback" {
						t.Error("incorrect request fields")
					}
					if metadata {
						m, ok := body["metadata"].(map[string]any)
						if !ok || m["mobile"] != "09120000000" || m["email"] != "test@example.test" {
							t.Error("incorrect metadata")
						}
					} else if _, exists := body["metadata"]; exists {
						t.Error("empty metadata must be omitted")
					}
					_, _ = io.WriteString(w, contractRequest)
				})
				input := RequestPaymentInput{Amount: 10000, Description: "Order 1", CallbackURL: "https://example.test/api/v1/payments/zarinpal/callback"}
				if metadata {
					input.Mobile, input.Email = "09120000000", "test@example.test"
				}
				out, err := client.RequestPayment(t.Context(), input)
				if err != nil || out.Code != 100 || out.Authority != contractAuthority || out.Message != "Success" || out.RedirectURL != client.startPayURL()+contractAuthority {
					t.Fatalf("out=%+v err=%v", out, err)
				}
			})
		}
	}
}

func TestZarinPalOfficialVerifyContract(t *testing.T) {
	for _, code := range []int{100, 101} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			client := contractClient(t, true, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/pg/v4/payment/verify.json" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["merchant_id"] != "merchant-secret" || body["amount"] != float64(123456) || len(body) != 3 || body["authority"] != contractAuthority {
					t.Error("incorrect verification fields")
				}
				_, _ = io.WriteString(w, strings.Replace(contractVerify, `"code":100`, fmt.Sprintf(`"code":%d`, code), 1))
			})
			out, err := client.VerifyPayment(t.Context(), VerifyPaymentInput{Amount: 123456, Authority: contractAuthority})
			if err != nil || out.Code != code || out.RefID != 201 || out.AlreadyVerified != (code == 101) || out.Message != "Verified" {
				t.Fatalf("out=%+v err=%v", out, err)
			}
		})
	}
}

func TestZarinPalRejections(t *testing.T) {
	for _, status := range []int{200, 400, 422, 500} {
		for _, body := range []string{sdkRejection, `{"data":{"code":-51,"message":"merchant-secret"},"errors":[]}`} {
			t.Run(fmt.Sprintf("%d/%s", status, body), func(t *testing.T) {
				client := responseClient(t, body, status)
				code := -9
				if strings.Contains(body, "-51") {
					code = -51
				}
				request, err := client.RequestPayment(t.Context(), RequestPaymentInput{Amount: 10000})
				var providerError *ProviderError
				if !errors.As(err, &providerError) || providerError.Code != code || request != (RequestPaymentOutput{}) {
					t.Fatalf("out=%+v err=%v", request, err)
				}
				if err.Error() != fmt.Sprintf("zarinpal: provider rejected payment (code=%d)", code) {
					t.Fatal("provider error must only expose code")
				}
				verify, err := client.VerifyPayment(t.Context(), VerifyPaymentInput{Amount: 10000, Authority: contractAuthority})
				if err != nil || verify != (VerifyPaymentOutput{Code: code}) {
					t.Fatalf("out=%+v err=%v", verify, err)
				}
			})
		}
	}
}

func TestZarinPalMalformedEnvelopes(t *testing.T) {
	bodies := map[string]string{
		"empty":                   "",
		"null":                    "null",
		"array":                   "[]",
		"object":                  "{}",
		"missing data":            `{"errors":[]}`,
		"missing errors":          `{"data":{"code":100,"message":"Success"}}`,
		"null data":               `{"data":null,"errors":[]}`,
		"null errors":             `{"data":{},"errors":null}`,
		"empty data array":        `{"data":[],"errors":[]}`,
		"string data":             `{"data":"merchant-secret","errors":[]}`,
		"string errors":           `{"data":{},"errors":"merchant-secret"}`,
		"nonempty errors":         `{"data":{},"errors":[{"code":-9,"message":"rejected"}]}`,
		"empty error object":      `{"data":[],"errors":{}}`,
		"error missing code":      `{"data":[],"errors":{"message":"rejected"}}`,
		"error missing message":   `{"data":[],"errors":{"code":-9}}`,
		"error null code":         `{"data":[],"errors":{"code":null,"message":"rejected"}}`,
		"error string code":       `{"data":[],"errors":{"code":"-9","message":"rejected"}}`,
		"error zero code":         `{"data":[],"errors":{"code":0,"message":"rejected"}}`,
		"error positive code":     `{"data":[],"errors":{"code":100,"message":"rejected"}}`,
		"error data missing":      `{"errors":{"code":-9,"message":"rejected"}}`,
		"error data null":         `{"data":null,"errors":{"code":-9,"message":"rejected"}}`,
		"error data object":       `{"data":{},"errors":{"code":-9,"message":"rejected"}}`,
		"error data populated":    `{"data":[1],"errors":{"code":-9,"message":"rejected"}}`,
		"error validations type":  `{"data":[],"errors":{"code":-9,"message":"rejected","validations":"invalid"}}`,
		"contradiction":           strings.Replace(contractRequest, `"errors":[]`, `"errors":{"code":-9,"message":"rejected"}`, 1),
		"trailing junk":           contractRequest + "merchant-secret",
		"trailing document":       contractRequest + "{}",
		"truncated":               contractRequest[:len(contractRequest)-1],
		"oversized":               contractRequest + strings.Repeat(" ", maxResponseBytes),
		"invalid utf8":            strings.Replace(contractRequest, "Success", string([]byte{0xff}), 1),
		"duplicate envelope":      strings.Replace(contractRequest, `"errors":[]`, `"errors":[],"errors":[]`, 1),
		"case envelope":           strings.Replace(contractRequest, `"errors":[]`, `"errors":[],"Errors":[]`, 1),
		"case data":               strings.Replace(contractRequest, `"data":`, `"Data":`, 1),
		"duplicate code":          strings.Replace(contractRequest, `"code":100`, `"code":-9,"code":100`, 1),
		"case code":               strings.Replace(contractRequest, `"code":100`, `"code":100,"Code":-9`, 1),
		"escaped code":            strings.Replace(contractRequest, `"code":100`, `"code":100,"\u0063ode":-9`, 1),
		"duplicate unknown":       strings.Replace(contractRequest, `"errors":[]`, `"errors":[],"extra":{"a":1,"a":2}`, 1),
		"deep unknown":            strings.Replace(contractRequest, `"errors":[]`, `"errors":[],"extra":`+strings.Repeat("[", 70)+"0"+strings.Repeat("]", 70), 1),
		"negative with authority": strings.Replace(contractRequest, `"code":100`, `"code":-9`, 1),
		"negative with ref":       strings.Replace(contractVerify, `"code":100`, `"code":-51`, 1),
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			client := responseClient(t, body, 200)
			out, err := client.RequestPayment(t.Context(), RequestPaymentInput{Amount: 10000})
			if err == nil || out != (RequestPaymentOutput{}) {
				t.Fatalf("request out=%+v err=%v", out, err)
			}
			var rejection *ProviderError
			if errors.As(err, &rejection) || strings.Contains(err.Error(), "merchant-secret") {
				t.Fatal("uncertain response must not be a rejection or expose provider text")
			}
			verify, err := client.VerifyPayment(t.Context(), VerifyPaymentInput{Amount: 10000, Authority: contractAuthority})
			if err == nil || verify != (VerifyPaymentOutput{}) {
				t.Fatalf("verify out=%+v err=%v", verify, err)
			}
		})
	}
}

func TestZarinPalMalformedSuccessFields(t *testing.T) {
	for _, verify := range []bool{false, true} {
		base := contractRequest
		fields := map[string]string{"code": "100", "message": `"Success"`, "authority": `"` + contractAuthority + `"`}
		if verify {
			base = contractVerify
			fields = map[string]string{"code": "100", "message": `"Verified"`, "ref_id": "201"}
		}
		for key, original := range fields {
			values := []string{"missing", "null", "true", "[]", "{}", `""`, `" "`}
			if key == "code" || key == "ref_id" {
				values = append(values, `"100"`, "100.0", "1e2", "0", "9223372036854775808")
			} else {
				values = append(values, "123")
			}
			if key == "ref_id" {
				values = append(values, "-1")
			}
			if key == "authority" {
				values = append(values, `"../redirect"`, `"A?secret"`, `"A#secret"`, `"A\nsecret"`)
			}
			for _, replacement := range values {
				t.Run(fmt.Sprintf("verify=%t/%s/%s", verify, key, replacement), func(t *testing.T) {
					body := strings.Replace(base, `"`+key+`":`+original, `"`+key+`":`+replacement, 1)
					if replacement == "missing" {
						body = strings.Replace(base, `"`+key+`":`+original+",", "", 1)
					}
					client := responseClient(t, body, 200)
					if verify {
						out, err := client.VerifyPayment(t.Context(), VerifyPaymentInput{Amount: 10000, Authority: contractAuthority})
						if err == nil || out != (VerifyPaymentOutput{}) {
							t.Fatalf("out=%+v err=%v", out, err)
						}
					} else {
						out, err := client.RequestPayment(t.Context(), RequestPaymentInput{Amount: 10000})
						if err == nil || out != (RequestPaymentOutput{}) {
							t.Fatalf("out=%+v err=%v", out, err)
						}
					}
				})
			}
		}
	}
	for _, code := range []string{"101", "102", "1"} {
		client := responseClient(t, strings.Replace(contractRequest, `"code":100`, `"code":`+code, 1), 200)
		if _, err := client.RequestPayment(t.Context(), RequestPaymentInput{Amount: 10000}); err == nil {
			t.Errorf("request accepted code %s", code)
		}
	}
	for _, ref := range []string{"null", "0", "-1", `"201"`} {
		body := strings.ReplaceAll(strings.Replace(contractVerify, `"code":100`, `"code":101`, 1), `"ref_id":201`, `"ref_id":`+ref)
		client := responseClient(t, body, 200)
		if out, err := client.VerifyPayment(t.Context(), VerifyPaymentInput{Amount: 10000, Authority: contractAuthority}); err == nil || out.AlreadyVerified {
			t.Errorf("101 without valid reference: out=%+v err=%v", out, err)
		}
	}
}

func TestZarinPalUnknownFieldsAndWhitespace(t *testing.T) {
	for _, fixture := range []string{contractRequest, contractVerify} {
		body := " \n" + strings.Replace(fixture, `"errors":[]`, `"errors":[],"extra":{"future":[null,true,1,"value"]}`, 1) + " \n"
		client := responseClient(t, body, 200)
		if fixture == contractRequest {
			if _, err := client.RequestPayment(t.Context(), RequestPaymentInput{Amount: 10000}); err != nil {
				t.Fatal(err)
			}
		} else if _, err := client.VerifyPayment(t.Context(), VerifyPaymentInput{Amount: 10000, Authority: contractAuthority}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestZarinPalHTTPFailuresAndRedirects(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308, 400, 422, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			client := contractClient(t, true, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "/redirected")
				w.WriteHeader(status)
				if strings.HasSuffix(r.URL.Path, requestPath) {
					_, _ = io.WriteString(w, contractRequest)
				} else {
					_, _ = io.WriteString(w, contractVerify)
				}
			})
			if out, err := client.RequestPayment(t.Context(), RequestPaymentInput{Amount: 10000}); err == nil || out != (RequestPaymentOutput{}) {
				t.Fatalf("out=%+v err=%v", out, err)
			}
			if out, err := client.VerifyPayment(t.Context(), VerifyPaymentInput{Amount: 10000, Authority: contractAuthority}); err == nil || out != (VerifyPaymentOutput{}) {
				t.Fatalf("out=%+v err=%v", out, err)
			}
			if calls.Load() != 2 {
				t.Fatal("redirect followed")
			}
			if err := client.httpClient.CheckRedirect(nil, nil); err != http.ErrUseLastResponse {
				t.Fatal("redirect policy changed")
			}
		})
	}
}

func TestZarinPalInputAndTransportFailures(t *testing.T) {
	client := NewZarinPalClient("merchant-secret", true)
	calls := 0
	client.httpClient.Transport = contractTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("merchant-secret provider-text")
	})
	for _, amount := range []int64{-1, 0, 9999} {
		if _, err := client.RequestPayment(t.Context(), RequestPaymentInput{Amount: amount}); err == nil {
			t.Errorf("request accepted amount %d", amount)
		}
		if _, err := client.VerifyPayment(t.Context(), VerifyPaymentInput{Amount: amount, Authority: contractAuthority}); err == nil {
			t.Errorf("verify accepted amount %d", amount)
		}
	}
	if calls != 0 {
		t.Fatal("invalid amounts sent to provider")
	}
	for _, ctx := range []context.Context{t.Context(), cancelledContext()} {
		if _, err := client.RequestPayment(ctx, RequestPaymentInput{Amount: 10000}); !errors.Is(err, ErrProviderUnavailable) || strings.Contains(err.Error(), "merchant-secret") {
			t.Fatalf("unsafe request error: %v", err)
		}
		if out, err := client.VerifyPayment(ctx, VerifyPaymentInput{Amount: 10000, Authority: contractAuthority}); !errors.Is(err, ErrProviderUnavailable) || out != (VerifyPaymentOutput{}) {
			t.Fatalf("out=%+v err=%v", out, err)
		}
	}
}

func cancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestZarinPalAdditionalAmbiguousFields(t *testing.T) {
	for _, fixture := range []string{contractRequest, contractVerify} {
		mutations := []string{
			strings.Replace(fixture, `"message":`, `"Message":`, 1),
			strings.Replace(fixture, `"message":`, `"message":"first","message":`, 1),
			strings.Replace(fixture, `"fee_type":"Merchant"`, `"fee_type":null`, 1),
			strings.Replace(fixture, `"fee":`, `"fee":null,"fee":`, 1),
			strings.Replace(fixture, `"errors":[]`, `"errors":[],"DATA":{}`, 1),
		}
		if fixture == contractRequest {
			mutations = append(mutations,
				strings.Replace(fixture, `"authority":`, `"authority":"A","Authority":`, 1),
				strings.Replace(fixture, `"authority":`, `"authority":"A","authority":`, 1),
			)
		} else {
			mutations = append(mutations,
				strings.Replace(fixture, `"ref_id":201`, `"ref_id":201,"Ref_ID":202`, 1),
				strings.Replace(fixture, `"ref_id":201`, `"ref_id":201,"ref_id":202`, 1),
				strings.Replace(strings.Replace(fixture, `"code":100`, `"code":101`, 1), `"ref_id":201,`, "", 1),
				strings.Replace(fixture, `"code":100`, `"code":102`, 1),
				strings.Replace(fixture, `"card_pan":"502229******5998"`, `"card_pan":[]`, 1),
			)
		}
		for i, body := range mutations {
			t.Run(fmt.Sprintf("%t/%d", fixture == contractVerify, i), func(t *testing.T) {
				client := responseClient(t, body, 200)
				if fixture == contractRequest {
					if out, err := client.RequestPayment(t.Context(), RequestPaymentInput{Amount: 10000}); err == nil || out != (RequestPaymentOutput{}) {
						t.Fatalf("out=%+v err=%v", out, err)
					}
				} else if out, err := client.VerifyPayment(t.Context(), VerifyPaymentInput{Amount: 10000, Authority: contractAuthority}); err == nil || out != (VerifyPaymentOutput{}) {
					t.Fatalf("out=%+v err=%v", out, err)
				}
			})
		}
	}
	for _, extra := range []string{`"code":-9,`, `"Code":-9,`, `"authority":"A",`, `"ref_id":201,`} {
		body := strings.Replace(sdkRejection, `"code":-9,`, extra+`"code":-9,`, 1)
		client := responseClient(t, body, 422)
		if out, err := client.VerifyPayment(t.Context(), VerifyPaymentInput{Amount: 10000, Authority: contractAuthority}); err == nil || out != (VerifyPaymentOutput{}) {
			t.Fatalf("out=%+v err=%v", out, err)
		}
	}
}

type contractReadFailure struct{}

func (contractReadFailure) Read([]byte) (int, error) {
	return 0, errors.New("merchant-secret")
}

func (contractReadFailure) Close() error {
	return nil
}

func TestZarinPalResponseReadFailure(t *testing.T) {
	client := NewZarinPalClient("merchant-secret", true)
	client.httpClient.Transport = contractTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: contractReadFailure{}, Header: make(http.Header), Request: r}, nil
	})
	if out, err := client.RequestPayment(t.Context(), RequestPaymentInput{Amount: 10000}); !errors.Is(err, ErrProviderUnavailable) || out != (RequestPaymentOutput{}) {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if out, err := client.VerifyPayment(t.Context(), VerifyPaymentInput{Amount: 10000, Authority: contractAuthority}); !errors.Is(err, ErrProviderUnavailable) || out != (VerifyPaymentOutput{}) {
		t.Fatalf("out=%+v err=%v", out, err)
	}
}

func TestZarinPalIdentity(t *testing.T) {
	client := NewZarinPalClient("Merchant-ID", true)
	sum := sha256.Sum256([]byte("merchant-id\nsandbox"))
	if client.Environment() != "sandbox" || client.Identity() != hex.EncodeToString(sum[:]) {
		t.Fatal("unexpected sandbox identity")
	}
	if client.Identity() != NewZarinPalClient("merchant-id", true).Identity() || client.Identity() == NewZarinPalClient("merchant-id", false).Identity() || client.Identity() == NewZarinPalClient("other", true).Identity() {
		t.Fatal("identity must bind canonical merchant and environment")
	}
	if NewZarinPalClient("merchant-id", false).Environment() != "live" {
		t.Fatal("unexpected live environment")
	}
}
