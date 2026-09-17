package payment

import (
	"strings"
	"testing"
)

const configMerchant = "98e53e51-8a80-4b4c-93d7-25ed5bf0d723"
const configCallback = "https://shop.example.test/api/v1/payments/zarinpal/callback"

func configValues() map[string]string {
	return map[string]string{
		"ZARINPAL_ENABLED":      "true",
		"ZARINPAL_SANDBOX":      "true",
		"ZARINPAL_MERCHANT_ID":  configMerchant,
		"ZARINPAL_CALLBACK_URL": configCallback,
	}
}

func configFromValues(values map[string]string) (Config, error) {
	return LoadConfig(func(key string) string { return values[key] })
}

func TestPaymentFrontendBaseURL(t *testing.T) {
	cases := []struct {
		name       string
		base       string
		production bool
		live       bool
		want       string
		invalid    bool
	}{
		{name: "unset same origin"},
		{name: "development", base: "http://localhost:5173", want: "http://localhost:5173"},
		{name: "trailing slash", base: "http://localhost:5173/", want: "http://localhost:5173"},
		{name: "localhost subdomain", base: "http://shop.localhost:5173", want: "http://shop.localhost:5173"},
		{name: "IPv4", base: "http://127.0.0.1:5173", want: "http://127.0.0.1:5173"},
		{name: "IPv6", base: "http://[::1]:5173", want: "http://[::1]:5173"},
		{name: "production HTTPS", base: "https://shop.example.com", production: true, want: "https://shop.example.com"},
		{name: "live HTTPS", base: "https://shop.example.com", live: true, want: "https://shop.example.com"},
		{name: "production unset", production: true},
		{name: "production HTTP", base: "http://localhost:5173", production: true, invalid: true},
		{name: "production local HTTPS", base: "https://localhost:5173", production: true, invalid: true},
		{name: "production private HTTPS", base: "https://192.168.1.1", production: true, invalid: true},
		{name: "production mapped loopback", base: "https://[::ffff:127.0.0.1]", production: true, invalid: true},
		{name: "production CGNAT", base: "https://100.64.0.1", production: true, invalid: true},
		{name: "live HTTP", base: "http://localhost:5173", live: true, invalid: true},
		{name: "public HTTP", base: "http://shop.example.com", invalid: true},
		{name: "private HTTP", base: "http://192.168.1.1", invalid: true},
		{name: "relative", base: "/payment/result", invalid: true},
		{name: "scheme relative", base: "//shop.example.com", invalid: true},
		{name: "opaque", base: "https:shop.example.com", invalid: true},
		{name: "other scheme", base: "javascript:alert(1)", invalid: true},
		{name: "missing host", base: "https:///", invalid: true},
		{name: "userinfo", base: "https://user:secret@shop.example.com", invalid: true},
		{name: "fragment", base: "https://shop.example.com/#fragment", invalid: true},
		{name: "empty fragment", base: "https://shop.example.com/#", invalid: true},
		{name: "query", base: "https://shop.example.com/?next=evil", invalid: true},
		{name: "empty query", base: "https://shop.example.com/?", invalid: true},
		{name: "path", base: "https://shop.example.com/store", invalid: true},
		{name: "encoded path", base: "https://shop.example.com/%2f", invalid: true},
		{name: "port zero", base: "http://localhost:0", invalid: true},
		{name: "port overflow", base: "http://localhost:65536", invalid: true},
		{name: "empty port", base: "http://localhost:", invalid: true},
		{name: "non numeric port", base: "http://localhost:abc", invalid: true},
		{name: "invalid hostname", base: "https://bad_host.example.com", invalid: true},
		{name: "invalid IPv6", base: "https://[localhost]", invalid: true},
		{name: "IPv6 zone", base: "https://[fe80::1%25eth0]", invalid: true},
		{name: "header injection", base: "https://shop.example.com\r\nLocation: https://evil.example", invalid: true},
	}
	for _, tc := range cases {
		for _, enabled := range []string{"true", "false"} {
			t.Run(tc.name+"/enabled="+enabled, func(t *testing.T) {
				values := configValues()
				values["ZARINPAL_ENABLED"] = enabled
				values["FRONTEND_BASE_URL"] = tc.base
				if tc.production {
					values["APP_ENV"] = "production"
				}
				if tc.live {
					values["ZARINPAL_SANDBOX"] = "false"
					values["ZARINPAL_LIVE_ENABLED"] = "true"
				}
				cfg, err := configFromValues(values)
				if (err != nil) != tc.invalid {
					t.Fatalf("invalid=%t err=%v", tc.invalid, err)
				}
				if tc.invalid {
					if cfg != (Config{}) {
						t.Fatalf("invalid frontend returned config: %+v", cfg)
					}
				} else if cfg.FrontendBaseURL != tc.want {
					t.Fatalf("frontend=%q want=%q", cfg.FrontendBaseURL, tc.want)
				}
			})
		}
	}
}

func TestPaymentConfigDisabled(t *testing.T) {
	for _, enabled := range []string{"", "false"} {
		values := map[string]string{
			"ZARINPAL_ENABLED":      enabled,
			"ZARINPAL_SANDBOX":      "false",
			"ZARINPAL_LIVE_ENABLED": "true",
			"ZARINPAL_MERCHANT_ID":  "merchant-secret",
			"ZARINPAL_CALLBACK_URL": "invalid-secret",
		}
		cfg, err := configFromValues(values)
		if err != nil || cfg != (Config{}) {
			t.Fatalf("disabled out=%+v err=%v", cfg, err)
		}
	}
}

func TestPaymentConfigSandboxAndLive(t *testing.T) {
	for _, sandbox := range []string{"true", "false"} {
		values := configValues()
		values["ZARINPAL_SANDBOX"] = sandbox
		values["ZARINPAL_LIVE_ENABLED"] = "true"
		values["ZARINPAL_MERCHANT_ID"] = strings.ToUpper(configMerchant)
		cfg, err := configFromValues(values)
		if err != nil || !cfg.Enabled || cfg.Sandbox != (sandbox == "true") || cfg.MerchantID != configMerchant || cfg.CallbackURL != configCallback {
			t.Fatalf("out=%+v err=%v", cfg, err)
		}
	}
}

func TestPaymentConfigExplicitFlags(t *testing.T) {
	for _, key := range []string{"ZARINPAL_ENABLED", "ZARINPAL_SANDBOX", "ZARINPAL_LIVE_ENABLED"} {
		invalid := []string{"TRUE", "False", "1", "0", "yes", " true", "true ", "merchant-secret"}
		if key != "ZARINPAL_ENABLED" {
			invalid = append(invalid, "")
		}
		if key == "ZARINPAL_LIVE_ENABLED" {
			invalid = append(invalid, "false")
		}
		for _, value := range invalid {
			t.Run(key+"/"+value, func(t *testing.T) {
				values := configValues()
				if key == "ZARINPAL_LIVE_ENABLED" {
					values["ZARINPAL_SANDBOX"] = "false"
				}
				values[key] = value
				cfg, err := configFromValues(values)
				if err == nil || cfg != (Config{}) || strings.Contains(err.Error(), "merchant-secret") {
					t.Fatalf("out=%+v err=%v", cfg, err)
				}
			})
		}
	}
	for _, appEnv := range []string{"", "development", "production"} {
		values := configValues()
		values["APP_ENV"] = appEnv
		values["ZARINPAL_SANDBOX"] = "false"
		if _, err := configFromValues(values); err == nil {
			t.Errorf("live allowed without opt-in in %q", appEnv)
		}
	}
}

func TestPaymentConfigMerchant(t *testing.T) {
	for _, merchant := range []string{"", "merchant-secret", "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", "00000000-0000-0000-0000-000000000000", "11111111-1111-1111-1111-111111111111", "ffffffff-ffff-ffff-ffff-ffffffffffff", "123e4567-e89b-12d3-a456-426614174000", "550e8400-e29b-41d4-a716-446655440000", " " + configMerchant, configMerchant + " ", strings.ReplaceAll(configMerchant, "-", ""), "{" + configMerchant + "}"} {
		t.Run(merchant, func(t *testing.T) {
			values := configValues()
			values["ZARINPAL_MERCHANT_ID"] = merchant
			cfg, err := configFromValues(values)
			if err == nil || cfg != (Config{}) || strings.Contains(err.Error(), "merchant-secret") {
				t.Fatalf("out=%+v err=%v", cfg, err)
			}
		})
	}
}

func TestPaymentConfigCallback(t *testing.T) {
	for _, callback := range []string{
		"", "merchant-secret", "/api/v1/payments/zarinpal/callback",
		strings.Replace(configCallback, "https:", "http:", 1),
		strings.Replace(configCallback, "https:", "ftp:", 1),
		strings.Replace(configCallback, "https://", "https://user:merchant-secret@", 1),
		configCallback + "?token=merchant-secret", configCallback + "?", configCallback + "#fragment", configCallback + "#",
		configCallback + "/", strings.Replace(configCallback, "/callback", "/other", 1),
		strings.Replace(configCallback, "/callback", "/%63allback", 1),
		strings.Replace(configCallback, "shop.example.test", "", 1),
		strings.Replace(configCallback, "shop.example.test", "shop.example.test:0", 1),
		strings.Replace(configCallback, "shop.example.test", "shop.example.test:65536", 1),
		strings.Replace(configCallback, "shop.example.test", "shop.example.test:", 1),
		strings.Replace(configCallback, "shop.example.test", "shop.example.test:abc", 1),
		strings.Replace(configCallback, "shop.example.test", "bad_host.example", 1),
		strings.Replace(configCallback, "shop.example.test", "bad..example", 1),
		strings.Replace(configCallback, "shop.example.test", "-bad.example", 1),
		strings.Replace(configCallback, "shop.example.test", "[not-ip]", 1),
		strings.Replace(configCallback, "shop.example.test", "[fe80::1%25eth0]", 1),
	} {
		t.Run(callback, func(t *testing.T) {
			values := configValues()
			values["ZARINPAL_CALLBACK_URL"] = callback
			cfg, err := configFromValues(values)
			if err == nil || cfg != (Config{}) || strings.Contains(err.Error(), "merchant-secret") {
				t.Fatalf("out=%+v err=%v", cfg, err)
			}
		})
	}
}

func TestPaymentConfigCallbackSchemes(t *testing.T) {
	cases := []struct {
		name    string
		sandbox bool
		origin  string
		suffix  string
		allowed bool
	}{
		{"sandbox localhost", true, "http://localhost:8080", "", true},
		{"sandbox localhost subdomain", true, "http://shop.localhost", "", true},
		{"sandbox IPv4", true, "http://127.0.0.1:8080", "", true},
		{"sandbox IPv6", true, "http://[::1]:8080", "", true},
		{"sandbox HTTPS", true, "https://shop.example.test", "", true},
		{"sandbox public HTTP", true, "http://shop.example.test", "", false},
		{"sandbox private HTTP", true, "http://192.168.1.1", "", false},
		{"sandbox other loopback", true, "http://127.0.0.2", "", false},
		{"sandbox mapped loopback", true, "http://[::ffff:127.0.0.1]", "", false},
		{"sandbox misleading hostname", true, "http://localhost.example.test", "", false},
		{"sandbox invalid hostname", true, "http://bad_host.localhost", "", false},
		{"sandbox invalid port", true, "http://localhost:65536", "", false},
		{"sandbox userinfo", true, "http://user@localhost", "", false},
		{"sandbox query", true, "http://localhost", "?x=1", false},
		{"sandbox empty query", true, "http://localhost", "?", false},
		{"sandbox fragment", true, "http://localhost", "#fragment", false},
		{"sandbox empty fragment", true, "http://localhost", "#", false},
		{"sandbox wrong path", true, "http://localhost", "/other", false},
		{"sandbox opaque URL", true, "http:localhost", "", false},
		{"live HTTP localhost", false, "http://localhost", "", false},
		{"live HTTPS localhost", false, "https://localhost", "", false},
		{"live HTTP public", false, "http://shop.example.test", "", false},
		{"live HTTPS public", false, "https://shop.example.test", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			values := configValues()
			if !tc.sandbox {
				values["ZARINPAL_SANDBOX"] = "false"
			}
			values["ZARINPAL_LIVE_ENABLED"] = "true"
			values["ZARINPAL_CALLBACK_URL"] = tc.origin + "/api/v1/payments/zarinpal/callback" + tc.suffix
			cfg, err := configFromValues(values)
			if (err == nil) != tc.allowed {
				t.Fatalf("allowed=%t err=%v", tc.allowed, err)
			}
			if tc.allowed && (cfg.CallbackURL != values["ZARINPAL_CALLBACK_URL"] || cfg.Sandbox != tc.sandbox) {
				t.Fatalf("unexpected config: %+v", cfg)
			}
			if !tc.allowed && cfg != (Config{}) {
				t.Fatalf("invalid callback returned enabled config: %+v", cfg)
			}
		})
	}
}

func TestPaymentConfigLivePrivateHosts(t *testing.T) {
	for _, host := range []string{"localhost", "LOCALHOST.", "shop.localhost", "shop.local", "backend", "127.0.0.1", "127.1", "2130706433", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.169.254", "0.0.0.0", "224.0.0.1", "100.64.0.1", "[::1]", "[::]", "[fc00::1]", "[fe80::1]", "[ff02::1]", "[::ffff:127.0.0.1]", "[::ffff:192.168.0.1]"} {
		t.Run(host, func(t *testing.T) {
			values := configValues()
			values["ZARINPAL_SANDBOX"] = "false"
			values["ZARINPAL_LIVE_ENABLED"] = "true"
			values["ZARINPAL_CALLBACK_URL"] = "https://" + host + "/api/v1/payments/zarinpal/callback"
			cfg, err := configFromValues(values)
			if err == nil || cfg != (Config{}) {
				t.Fatalf("out=%+v err=%v", cfg, err)
			}
		})
	}
}

func TestPaymentConfigValidCallbackHosts(t *testing.T) {
	for _, host := range []string{"shop.example.test", "shop.example.test:8443", "8.8.8.8", "[2606:4700:4700::1111]"} {
		values := configValues()
		values["ZARINPAL_SANDBOX"] = "false"
		values["ZARINPAL_LIVE_ENABLED"] = "true"
		values["ZARINPAL_CALLBACK_URL"] = "https://" + host + "/api/v1/payments/zarinpal/callback"
		if _, err := configFromValues(values); err != nil {
			t.Errorf("host=%s err=%v", host, err)
		}
	}
	values := configValues()
	values["ZARINPAL_CALLBACK_URL"] = "https://localhost:8080/api/v1/payments/zarinpal/callback"
	if _, err := configFromValues(values); err != nil {
		t.Fatal(err)
	}
}
