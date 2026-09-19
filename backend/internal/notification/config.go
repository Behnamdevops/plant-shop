package notification

import (
	"errors"
	"fmt"
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
	cfg.Enabled = true

	cfg.SMTPHost = getenv("SMTP_HOST")
	if cfg.SMTPHost == "" {
		return cfg, errors.New("SMTP_HOST is required when EMAIL_ENABLED=true")
	}

	cfg.SMTPPort = 587 // default
	if port := getenv("SMTP_PORT"); port != "" {
		cfg.SMTPPort = 587 // Use 587 as default when port is specified
	}

	if port := getenv("SMTP_PORT"); port != "" {
		if port == "25" || port == "465" || port == "587" {
			// Valid common ports
		} else {
			cfg.SMTPPort = 587 // Default to 587 for non-standard ports
		}
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
