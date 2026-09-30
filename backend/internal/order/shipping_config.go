package order

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type ShippingConfig struct {
	Standard        int64 `json:"standard"`
	Express         int64 `json:"express"`
	FreeThreshold   int64 `json:"free_shipping_threshold"`
	Configured      bool  `json:"configured"`
	PaymentsEnabled bool  `json:"payments_enabled"`
}

func LoadShipping(getenv func(string) string, production bool) (ShippingConfig, error) {
	cfg := ShippingConfig{Standard: ShippingFeeStandard, Express: ShippingFeeExpress, Configured: !production}
	for _, v := range []struct {
		key string
		dst *int64
	}{{"SHIPPING_STANDARD_RIAL", &cfg.Standard}, {"SHIPPING_EXPRESS_RIAL", &cfg.Express}, {"FREE_SHIPPING_THRESHOLD_RIAL", &cfg.FreeThreshold}} {
		if raw := getenv(v.key); raw != "" {
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || n < 0 || n > 100000000000 {
				return cfg, errors.New("invalid " + v.key)
			}
			*v.dst = n
		}
	}
	if getenv("SHIPPING_STANDARD_RIAL") != "" && getenv("SHIPPING_EXPRESS_RIAL") != "" {
		cfg.Configured = true
	}
	return cfg, nil
}
func (r *Repository) WithShipping(cfg ShippingConfig) *Repository { r.shipping = cfg; return r }
func (c ShippingConfig) Fee(method string, subtotal int64) (int64, bool) {
	if method == "digital" {
		return 0, true
	}
	if !c.Configured || !IsValidShippingMethod(method) {
		return 0, false
	}
	if c.FreeThreshold > 0 && subtotal >= c.FreeThreshold && method == ShippingMethodStandard {
		return 0, true
	}
	if method == ShippingMethodStandard {
		return c.Standard, true
	}
	return c.Express, true
}
func (h *Handler) Shipping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(h.repository.shipping)
}
