package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Behnamdevops/plant-shop/backend/internal/payment"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Environment      string
	Port             string
	Pool             *pgxpool.Config
	Payment          payment.Config
	UploadDir        string
	ArticleUploadDir string
}

func Load(getenv func(string) string) (Config, error) {
	var cfg Config
	cfg.Environment = getenv("APP_ENV")
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}
	if cfg.Environment != "development" && cfg.Environment != "production" {
		return Config{}, errors.New("APP_ENV must be development or production")
	}
	cfg.Port = getenv("PORT")
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	port, err := strconv.Atoi(cfg.Port)
	if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != cfg.Port {
		return Config{}, errors.New("PORT must be an integer from 1 to 65535")
	}
	cfg.Pool, err = PoolConfig(getenv)
	if err != nil {
		return Config{}, err
	}
	for _, key := range []string{"ZARINPAL_ENABLED", "ZARINPAL_SANDBOX", "ZARINPAL_LIVE_ENABLED"} {
		value := getenv(key)
		if value != "" && value != "true" && value != "false" {
			return Config{}, fmt.Errorf("%s must be true or false", key)
		}
	}
	if getenv("ZARINPAL_SANDBOX") == "true" && getenv("ZARINPAL_LIVE_ENABLED") == "true" {
		return Config{}, errors.New("ZARINPAL_SANDBOX and ZARINPAL_LIVE_ENABLED conflict")
	}
	strict := cfg.Environment == "production" || getenv("ZARINPAL_SANDBOX") == "false"
	for _, key := range []string{"FRONTEND_BASE_URL", "ZARINPAL_CALLBACK_URL"} {
		raw := getenv(key)
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			return Config{}, fmt.Errorf("invalid %s", key)
		}
		if strict && placeholderHost(u.Hostname()) {
			return Config{}, fmt.Errorf("%s must not use a placeholder host", key)
		}
	}
	cfg.Payment, err = payment.LoadConfig(getenv)
	if err != nil {
		return Config{}, err
	}
	cfg.UploadDir = getenv("UPLOAD_DIR")
	if cfg.UploadDir == "" {
		cfg.UploadDir = "data/uploads/products"
	}
	cfg.ArticleUploadDir = getenv("ARTICLE_UPLOAD_DIR")
	if cfg.ArticleUploadDir == "" {
		cfg.ArticleUploadDir = "data/uploads/articles"
	}
	callback := getenv("ZARINPAL_CALLBACK_URL")
	if callback != "" {
		u, err := url.Parse(callback)
		if err != nil || u.User != nil || u.RawQuery != "" || u.ForceQuery || strings.Contains(callback, "#") || u.EscapedPath() != "/api/v1/payments/zarinpal/callback" {
			return Config{}, errors.New("invalid ZARINPAL_CALLBACK_URL")
		}
		if strict && !(u.Scheme == "https" && !placeholderHost(u.Hostname())) {
			return Config{}, errors.New("ZARINPAL_CALLBACK_URL must use a public HTTPS host in production; localhost HTTP is development-only")
		}
		if u.Port() != "" {
			port, err := strconv.Atoi(u.Port())
			if err != nil || port < 1 || port > 65535 {
				return Config{}, errors.New("invalid ZARINPAL_CALLBACK_URL port")
			}
		}
	}
	return cfg, nil
}

func placeholderHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for _, suffix := range []string{"example.com", "example.org", "example.net", "example", "test", "invalid", "local", "internal"} {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

func PoolConfig(getenv func(string) string) (*pgxpool.Config, error) {
	raw := getenv("DATABASE_URL")
	if raw == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" || u.Path == "" || u.Path == "/" || u.Fragment != "" {
		return nil, errors.New("DATABASE_URL must be a valid PostgreSQL URL with a host and database")
	}
	pool, err := pgxpool.ParseConfig(raw)
	if err != nil {
		return nil, errors.New("invalid DATABASE_URL")
	}
	if getenv("APP_ENV") == "production" {
		password := strings.ToLower(pool.ConnConfig.Password)
		if password == "" || password == "plantshop" || strings.Contains(password, "change-me") || strings.Contains(password, "changeme") || strings.Contains(password, "placeholder") {
			return nil, errors.New("DATABASE_URL requires a nonplaceholder database password in production")
		}
	}
	pool.ConnConfig.ConnectTimeout = 5 * time.Second
	pool.ConnConfig.RuntimeParams["search_path"] = "public"
	pool.MaxConns = 10
	pool.MinConns = 2
	pool.MinIdleConns = 0
	for key, target := range map[string]*int32{"DB_MAX_CONNS": &pool.MaxConns, "DB_MIN_CONNS": &pool.MinConns, "DB_MIN_IDLE_CONNS": &pool.MinIdleConns} {
		if raw := getenv(key); raw != "" {
			value, err := strconv.ParseInt(raw, 10, 32)
			if err != nil || value < 0 || value > 1000 {
				return nil, fmt.Errorf("%s must be an integer between 0 and 1000", key)
			}
			*target = int32(value)
		}
	}
	if pool.MaxConns < 1 || pool.MinConns > pool.MaxConns || pool.MinIdleConns > pool.MaxConns {
		return nil, errors.New("DB_MAX_CONNS must be positive and not below DB_MIN_CONNS or DB_MIN_IDLE_CONNS")
	}
	pool.MaxConnLifetime = time.Hour
	pool.MaxConnIdleTime = 15 * time.Minute
	pool.HealthCheckPeriod = time.Minute
	for key, target := range map[string]*time.Duration{"DB_MAX_CONN_LIFETIME": &pool.MaxConnLifetime, "DB_MAX_CONN_IDLE_TIME": &pool.MaxConnIdleTime, "DB_HEALTH_CHECK_PERIOD": &pool.HealthCheckPeriod} {
		if raw := getenv(key); raw != "" {
			value, err := time.ParseDuration(raw)
			if err != nil || value < time.Second || value > 24*time.Hour {
				return nil, fmt.Errorf("%s must be a duration between 1s and 24h", key)
			}
			*target = value
		}
	}
	pool.MaxConnLifetimeJitter = pool.MaxConnLifetime / 10
	return pool, nil
}
