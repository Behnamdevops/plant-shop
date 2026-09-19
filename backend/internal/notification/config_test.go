package notification

import (
	"testing"
)

func TestLoadConfigDisabled(t *testing.T) {
	// Test with EMAIL_ENABLED=false (or empty)
	getenv := func(key string) string {
		switch key {
		case "EMAIL_ENABLED":
			return "false"
		case "SMTP_HOST":
			return ""
		case "SMTP_PORT":
			return ""
		case "SMTP_USERNAME":
			return ""
		case "SMTP_PASSWORD":
			return ""
		case "SMTP_FROM_EMAIL":
			return ""
		case "SMTP_FROM_NAME":
			return ""
		case "SMTP_USE_TLS":
			return ""
		}
		return ""
	}

	cfg, err := LoadConfig(getenv)
	if err != nil {
		t.Fatalf("LoadConfig with disabled email should succeed: %v", err)
	}
	if cfg.Enabled {
		t.Error("expected email to be disabled")
	}
}

func TestLoadConfigEnabled(t *testing.T) {
	getenv := func(key string) string {
		switch key {
		case "EMAIL_ENABLED":
			return "true"
		case "SMTP_HOST":
			return "smtp.example.com"
		case "SMTP_PORT":
			return "587"
		case "SMTP_USERNAME":
			return "user@example.com"
		case "SMTP_PASSWORD":
			return "password123"
		case "SMTP_FROM_EMAIL":
			return "noreply@example.com"
		case "SMTP_FROM_NAME":
			return "Test Shop"
		case "SMTP_USE_TLS":
			return "true"
		}
		return ""
	}

	cfg, err := LoadConfig(getenv)
	if err != nil {
		t.Fatalf("LoadConfig with enabled email should succeed: %v", err)
	}
	if !cfg.Enabled {
		t.Error("expected email to be enabled")
	}
	if cfg.SMTPHost != "smtp.example.com" {
		t.Errorf("expected SMTP_HOST 'smtp.example.com', got '%s'", cfg.SMTPHost)
	}
	if cfg.SMTPPort != 587 {
		t.Errorf("expected SMTP_PORT 587, got %d", cfg.SMTPPort)
	}
	if cfg.SMTPUsername != "user@example.com" {
		t.Errorf("expected SMTP_USERNAME 'user@example.com', got '%s'", cfg.SMTPUsername)
	}
	if cfg.SMTPPassword != "password123" {
		t.Errorf("expected SMTP_PASSWORD 'password123', got '%s'", cfg.SMTPPassword)
	}
	if cfg.SMTPFromEmail != "noreply@example.com" {
		t.Errorf("expected SMTP_FROM_EMAIL 'noreply@example.com', got '%s'", cfg.SMTPFromEmail)
	}
	if cfg.SMTPFromName != "Test Shop" {
		t.Errorf("expected SMTP_FROM_NAME 'Test Shop', got '%s'", cfg.SMTPFromName)
	}
	if !cfg.SMTPUseTLS {
		t.Error("expected SMTP_USE_TLS to be true")
	}
}

func TestLoadConfigMissingRequired(t *testing.T) {
	cases := []struct {
		name     string
		getenv   func(string) string
		expected string
	}{
		{
			name: "missing SMTP_HOST",
			getenv: func(key string) string {
				if key == "EMAIL_ENABLED" {
					return "true"
				}
				if key == "SMTP_HOST" {
					return ""
				}
				return "placeholder"
			},
			expected: "SMTP_HOST is required when EMAIL_ENABLED=true",
		},
		{
			name: "missing SMTP_USERNAME",
			getenv: func(key string) string {
				if key == "EMAIL_ENABLED" {
					return "true"
				}
				if key == "SMTP_USERNAME" {
					return ""
				}
				return "placeholder"
			},
			expected: "SMTP_USERNAME is required when EMAIL_ENABLED=true",
		},
		{
			name: "missing SMTP_PASSWORD",
			getenv: func(key string) string {
				if key == "EMAIL_ENABLED" {
					return "true"
				}
				if key == "SMTP_PASSWORD" {
					return ""
				}
				return "placeholder"
			},
			expected: "SMTP_PASSWORD is required when EMAIL_ENABLED=true",
		},
		{
			name: "missing SMTP_FROM_EMAIL",
			getenv: func(key string) string {
				if key == "EMAIL_ENABLED" {
					return "true"
				}
				if key == "SMTP_FROM_EMAIL" {
					return ""
				}
				return "placeholder"
			},
			expected: "SMTP_FROM_EMAIL is required when EMAIL_ENABLED=true",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadConfig(tc.getenv)
			if err == nil {
				t.Error("expected error for missing required config")
			} else if err.Error() != tc.expected {
				t.Errorf("expected error '%s', got '%s'", tc.expected, err.Error())
			}
		})
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name  string
		cfg   Config
		valid bool
	}{
		{
			name:  "disabled is always valid",
			cfg:   Config{Enabled: false},
			valid: true,
		},
		{
			name: "enabled with valid config",
			cfg: Config{
				Enabled:       true,
				SMTPHost:      "smtp.example.com",
				SMTPPort:      587,
				SMTPUsername:  "user@example.com",
				SMTPPassword:  "password",
				SMTPFromEmail: "noreply@example.com",
				SMTPUseTLS:    true,
			},
			valid: true,
		},
		{
			name: "enabled with empty host",
			cfg: Config{
				Enabled:       true,
				SMTPHost:      "",
				SMTPPort:      587,
				SMTPUsername:  "user@example.com",
				SMTPPassword:  "password",
				SMTPFromEmail: "noreply@example.com",
			},
			valid: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.valid && err != nil {
				t.Errorf("expected valid config, got error: %v", err)
			}
			if !tc.valid && err == nil {
				t.Error("expected invalid config to fail validation")
			}
		})
	}
}
