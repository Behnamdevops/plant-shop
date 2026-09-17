package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestProductionFrontendHealthcheckUsesIPv4Listener(t *testing.T) {
	compose, err := os.ReadFile("../../../docker-compose.prod.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, frontend, ok := strings.Cut(string(compose), "\n  frontend:")
	if !ok {
		t.Fatal("frontend service missing")
	}
	frontend, _, _ = strings.Cut(frontend, "\nnetworks:")
	if !strings.Contains(frontend, `test: ["CMD", "wget", "-qO-", "http://127.0.0.1:80"]`) {
		t.Fatal("frontend healthcheck must target the nginx IPv4 listener explicitly")
	}
}

func values() map[string]string {
	return map[string]string{"DATABASE_URL": "postgres://user:unit-only-password@localhost/shop_test", "APP_ENV": "production", "FRONTEND_BASE_URL": "https://shop.plants.tld"}
}

func TestProductionValidation(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"APP_ENV", "prod"}, {"PORT", "0"}, {"PORT", "65536"}, {"PORT", "abc"},
		{"DATABASE_URL", ""}, {"DATABASE_URL", "postgres://user:change-me@localhost/shop"},
		{"DATABASE_URL", "postgres://user:unit-secret@%/shop"},
		{"ZARINPAL_ENABLED", "yes"}, {"ZARINPAL_SANDBOX", "TRUE"}, {"ZARINPAL_LIVE_ENABLED", "1"},
		{"FRONTEND_BASE_URL", "http://localhost:5173"}, {"FRONTEND_BASE_URL", "https://shop.example.com"},
		{"FRONTEND_BASE_URL", "https://192.168.1.2"}, {"FRONTEND_BASE_URL", "https://shop.plants.tld/path"},
		{"ZARINPAL_CALLBACK_URL", "http://localhost:8080/api/v1/payments/zarinpal/callback"},
		{"ZARINPAL_CALLBACK_URL", "https://shop.example.com/api/v1/payments/zarinpal/callback"},
		{"ZARINPAL_CALLBACK_URL", "https://shop.plants.tld/wrong"},
		{"ZARINPAL_CALLBACK_URL", "https://shop.plants.tld:0/api/v1/payments/zarinpal/callback"},
		{"DB_MAX_CONNS", "0"}, {"DB_MAX_CONNS", "1001"}, {"DB_MIN_CONNS", "11"},
		{"DB_MIN_IDLE_CONNS", "11"}, {"DB_MIN_CONNS", "-1"}, {"DB_MAX_CONN_LIFETIME", "bad"},
		{"DB_MAX_CONN_IDLE_TIME", "0s"}, {"DB_HEALTH_CHECK_PERIOD", "25h"},
	} {
		t.Run(tc.key+"/"+tc.value, func(t *testing.T) {
			env := values()
			env[tc.key] = tc.value
			_, err := Load(func(key string) string { return env[key] })
			if err == nil {
				t.Fatal("unsafe configuration accepted")
			}
			if strings.Contains(err.Error(), "unit-secret") {
				t.Fatal("secret leaked")
			}
		})
	}
}

func TestPaymentModes(t *testing.T) {
	for _, tc := range []struct {
		enabled, sandbox, live, merchant string
		valid                            bool
	}{
		{"false", "true", "false", "", true}, {"true", "true", "false", "98e53e51-8a80-4b4c-93d7-25ed5bf0d723", true},
		{"true", "false", "true", "98e53e51-8a80-4b4c-93d7-25ed5bf0d723", true},
		{"true", "true", "true", "98e53e51-8a80-4b4c-93d7-25ed5bf0d723", false},
		{"false", "true", "true", "", false}, {"true", "false", "false", "", false},
		{"true", "", "false", "", false}, {"true", "true", "false", "", false},
		{"true", "true", "false", "123e4567-e89b-12d3-a456-426614174000", false},
	} {
		t.Run(tc.enabled+tc.sandbox+tc.live+tc.merchant, func(t *testing.T) {
			env := values()
			env["ZARINPAL_ENABLED"], env["ZARINPAL_SANDBOX"], env["ZARINPAL_LIVE_ENABLED"], env["ZARINPAL_MERCHANT_ID"] = tc.enabled, tc.sandbox, tc.live, tc.merchant
			env["ZARINPAL_CALLBACK_URL"] = "https://shop.plants.tld/api/v1/payments/zarinpal/callback"
			_, err := Load(func(key string) string { return env[key] })
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}

func TestDefaultsAndOverrides(t *testing.T) {
	env := map[string]string{"DATABASE_URL": "postgres://plantshop:plantshop@localhost/plantshop"}
	cfg, err := Load(func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Environment != "development" || cfg.Port != "8080" || cfg.Pool.MaxConns != 10 || cfg.Pool.MinConns != 2 || cfg.Pool.MaxConnLifetime != time.Hour || cfg.Pool.MaxConnIdleTime != 15*time.Minute || cfg.Pool.HealthCheckPeriod != time.Minute {
		t.Fatal("incorrect defaults")
	}
	env["DB_MAX_CONNS"], env["DB_MIN_CONNS"], env["DB_MIN_IDLE_CONNS"] = "20", "0", "3"
	env["DB_MAX_CONN_LIFETIME"], env["DB_MAX_CONN_IDLE_TIME"], env["DB_HEALTH_CHECK_PERIOD"] = "30m", "5m", "10s"
	env["ZARINPAL_CALLBACK_URL"] = "http://localhost:8080/api/v1/payments/zarinpal/callback"
	cfg, err = Load(func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Pool.MaxConns != 20 || cfg.Pool.MinConns != 0 || cfg.Pool.MinIdleConns != 3 || cfg.Pool.MaxConnLifetime != 30*time.Minute || cfg.Pool.MaxConnIdleTime != 5*time.Minute || cfg.Pool.HealthCheckPeriod != 10*time.Second {
		t.Fatal("overrides not applied")
	}
}
