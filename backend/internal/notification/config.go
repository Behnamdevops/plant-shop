package notification

import (
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
)

// Config holds email notification configuration loaded from environment variables.
type Config struct {
	Enabled       bool
	SMTPHost      string
	SMTPPort      int
	SMTPUsername  string
	SMTPPassword  string
	SMTPFromEmail string
	SMTPFromName  string
	SMTPUseTLS    bool
}

// LoadConfig loads notification configuration from environment variables.
func LoadConfig(getenv func(string) string) (Config, error) {
	var cfg Config

	enabled := getenv("EMAIL_ENABLED")
	if enabled == "" || enabled == "false" {
		cfg.Enabled = false
		return cfg, nil
	}
	if enabled != "true" {
		return cfg, errors.New("EMAIL_ENABLED must be true or false")
	}
	cfg.Enabled = true

	cfg.SMTPHost = getenv("SMTP_HOST")
	if cfg.SMTPHost == "" {
		return cfg, errors.New("SMTP_HOST is required when EMAIL_ENABLED=true")
	}

	cfg.SMTPPort = 587
	if raw := getenv("SMTP_PORT"); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil || port < 1 || port > 65535 {
			return cfg, errors.New("invalid SMTP_PORT")
		}
		cfg.SMTPPort = port
	}
	cfg.SMTPUsername = getenv("SMTP_USERNAME")
	if cfg.SMTPUsername == "" {
		return cfg, errors.New("SMTP_USERNAME is required when EMAIL_ENABLED=true")
	}

	cfg.SMTPPassword = getenv("SMTP_PASSWORD")
	if cfg.SMTPPassword == "" {
		return cfg, errors.New("SMTP_PASSWORD is required when EMAIL_ENABLED=true")
	}

	cfg.SMTPFromEmail = getenv("SMTP_FROM_EMAIL")
	if cfg.SMTPFromEmail == "" {
		return cfg, errors.New("SMTP_FROM_EMAIL is required when EMAIL_ENABLED=true")
	}

	cfg.SMTPFromName = getenv("SMTP_FROM_NAME")
	if cfg.SMTPFromName == "" {
		cfg.SMTPFromName = "Plant Shop"
	}

	useTLS := getenv("SMTP_USE_TLS")
	if useTLS == "" || useTLS == "true" {
		cfg.SMTPUseTLS = true
	} else {
		cfg.SMTPUseTLS = false
	}

	address, err := mail.ParseAddress(cfg.SMTPFromEmail)
	if err != nil || address.Address != cfg.SMTPFromEmail {
		return cfg, errors.New("invalid SMTP_FROM_EMAIL")
	}
	if strings.ContainsAny(cfg.SMTPFromName, "\r\n") || strings.ContainsAny(cfg.SMTPHost, "\r\n") {
		return cfg, errors.New("invalid SMTP headers")
	}
	if useTLS != "" && useTLS != "true" && useTLS != "false" {
		return cfg, errors.New("SMTP_USE_TLS must be true or false")
	}
	return cfg, nil
}

// Validate checks that all required configuration is present.
func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.SMTPHost == "" {
		return errors.New("SMTP_HOST is required")
	}
	if c.SMTPPort <= 0 || c.SMTPPort > 65535 {
		return fmt.Errorf("invalid SMTP_PORT: %d", c.SMTPPort)
	}
	if c.SMTPUsername == "" {
		return errors.New("SMTP_USERNAME is required")
	}
	if c.SMTPPassword == "" {
		return errors.New("SMTP_PASSWORD is required")
	}
	if c.SMTPFromEmail == "" {
		return errors.New("SMTP_FROM_EMAIL is required")
	}

	return nil
}
