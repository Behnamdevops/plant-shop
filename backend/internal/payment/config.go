package payment

import (
	"errors"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type Config struct {
	Enabled         bool
	Sandbox         bool
	MerchantID      string
	CallbackURL     string
	FrontendBaseURL string
}

var merchantUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func LoadConfig(getenv func(string) string) (Config, error) {
	var cfg Config
	frontend, err := validateFrontendBaseURL(getenv("FRONTEND_BASE_URL"), getenv("APP_ENV") == "production" || getenv("ZARINPAL_SANDBOX") == "false")
	if err != nil {
		return Config{}, err
	}
	cfg.FrontendBaseURL = frontend
	switch getenv("ZARINPAL_ENABLED") {
	case "", "false":
		return cfg, nil
	case "true":
		cfg.Enabled = true
	default:
		return Config{}, errors.New("payment: ZARINPAL_ENABLED must be true or false")
	}
	switch getenv("ZARINPAL_SANDBOX") {
	case "true":
		cfg.Sandbox = true
	case "false":
	default:
		return Config{}, errors.New("payment: ZARINPAL_SANDBOX must be explicitly true or false")
	}
	if !cfg.Sandbox && getenv("ZARINPAL_LIVE_ENABLED") != "true" {
		return Config{}, errors.New("payment: live payments require ZARINPAL_LIVE_ENABLED=true")
	}
	cfg.MerchantID = getenv("ZARINPAL_MERCHANT_ID")
	if !merchantUUID.MatchString(cfg.MerchantID) || strings.EqualFold(cfg.MerchantID, "123e4567-e89b-12d3-a456-426614174000") || strings.EqualFold(cfg.MerchantID, "550e8400-e29b-41d4-a716-446655440000") {
		return Config{}, errors.New("payment: valid nonplaceholder ZARINPAL_MERCHANT_ID required")
	}
	cfg.MerchantID = strings.ToLower(cfg.MerchantID)
	cfg.CallbackURL = getenv("ZARINPAL_CALLBACK_URL")
	u, err := url.Parse(cfg.CallbackURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || strings.Contains(cfg.CallbackURL, "#") || u.Opaque != "" || u.EscapedPath() != "/api/v1/payments/zarinpal/callback" {
		return Config{}, errors.New("payment: invalid ZARINPAL_CALLBACK_URL")
	}
	if strings.HasSuffix(u.Host, ":") {
		return Config{}, errors.New("payment: invalid callback port")
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return Config{}, errors.New("payment: invalid callback port")
		}
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if ip, err := netip.ParseAddr(host); err != nil {
		if strings.ContainsAny(u.Host, "[]") || !validCallbackHostname(host) {
			return Config{}, errors.New("payment: invalid callback host")
		}
	} else if ip.Zone() != "" {
		return Config{}, errors.New("payment: invalid callback host")
	}
	if u.Scheme == "http" {
		local := host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "127.0.0.1" || host == "::1"
		if !cfg.Sandbox || !local {
			return Config{}, errors.New("payment: HTTP callback requires sandbox and a localhost host")
		}
	}
	if !cfg.Sandbox {
		if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || !strings.Contains(host, ".") && !strings.Contains(host, ":") {
			return Config{}, errors.New("payment: live callback requires a public host")
		}
		if ip, err := netip.ParseAddr(host); err == nil {
			ip = ip.Unmap()
			if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.Zone() != "" || netip.MustParsePrefix("100.64.0.0/10").Contains(ip) {
				return Config{}, errors.New("payment: live callback requires a public host")
			}
		}
	}
	return cfg, nil
}

func validateFrontendBaseURL(raw string, production bool) (string, error) {
	if raw == "" {
		return "", nil
	}
	invalid := errors.New("payment: invalid FRONTEND_BASE_URL")
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || strings.Contains(raw, "#") || (u.EscapedPath() != "" && u.EscapedPath() != "/") {
		return "", invalid
	}
	if strings.HasSuffix(u.Host, ":") {
		return "", invalid
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return "", invalid
		}
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	ip, ipErr := netip.ParseAddr(host)
	if ipErr != nil {
		if strings.ContainsAny(u.Host, "[]") || !validCallbackHostname(host) {
			return "", invalid
		}
	} else if ip.Zone() != "" {
		return "", invalid
	}
	local := host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "127.0.0.1" || host == "::1"
	if u.Scheme == "http" && (production || !local) {
		return "", invalid
	}
	if production {
		if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || !strings.ContainsAny(host, ".:") {
			return "", invalid
		}
		if ipErr == nil {
			ip = ip.Unmap()
			if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || netip.MustParsePrefix("100.64.0.0/10").Contains(ip) {
				return "", invalid
			}
		}
	}
	u.Path = ""
	return u.String(), nil
}

func validCallbackHostname(host string) bool {
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	onlyNumeric := true
	for _, ch := range host {
		if ch != '.' && (ch < '0' || ch > '9') {
			onlyNumeric = false
		}
	}
	if onlyNumeric {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, ch := range label {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
				return false
			}
		}
	}
	return true
}
