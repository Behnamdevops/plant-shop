package address

import (
	"errors"
	"strings"
	"time"
)

// Address represents a row in the user_addresses table.
type Address struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"-"`
	Label        string    `json:"label"`
	RecipientName string    `json:"recipient_name"`
	Phone        string    `json:"phone"`
	AddressLine1 string    `json:"address_line1"`
	AddressLine2 string    `json:"address_line2"`
	City         string    `json:"city"`
	PostalCode   string    `json:"postal_code"`
	Country      string    `json:"country"`
	IsDefault    bool      `json:"is_default"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateInput is the request payload for creating a new address.
type CreateInput struct {
	Label         string `json:"label"`
	RecipientName string `json:"recipient_name"`
	Phone         string `json:"phone"`
	AddressLine1  string `json:"address_line1"`
	AddressLine2  string `json:"address_line2"`
	City          string `json:"city"`
	PostalCode    string `json:"postal_code"`
	Country       string `json:"country"`
	IsDefault     bool   `json:"is_default"`
}

// UpdateInput is the request payload for updating an existing address.
type UpdateInput struct {
	Label         string `json:"label"`
	RecipientName string `json:"recipient_name"`
	Phone         string `json:"phone"`
	AddressLine1  string `json:"address_line1"`
	AddressLine2  string `json:"address_line2"`
	City          string `json:"city"`
	PostalCode    string `json:"postal_code"`
	Country       string `json:"country"`
	IsDefault     bool   `json:"is_default"`
}

// Field length limits matching the database column sizes
const (
	maxLabelLen         = 255
	maxRecipientNameLen = 255
	maxPhoneLen         = 20
	maxAddressLineLen   = 255
	maxCityLen          = 128
	maxPostalCodeLen    = 20
	maxCountryLen       = 128
)

var (
	ErrAddressNotFound = errors.New("address not found")
	ErrDuplicateDefault = errors.New("cannot have multiple default addresses")
)

// ErrValidation is returned by Validate methods when a field fails validation.
type ErrValidation struct {
	Field   string
	Message string
}

func (e *ErrValidation) Error() string {
	return e.Field + ": " + e.Message
}

// Validate checks that every required field is present (after trimming
// surrounding whitespace), within its maximum length.
func (in CreateInput) Validate() error {
	trimmed := in.Trimmed()

	required := []struct {
		field  string
		value  string
		maxLen int
	}{
		{"recipient_name", trimmed.RecipientName, maxRecipientNameLen},
		{"phone", trimmed.Phone, maxPhoneLen},
		{"address_line1", trimmed.AddressLine1, maxAddressLineLen},
		{"city", trimmed.City, maxCityLen},
		{"postal_code", trimmed.PostalCode, maxPostalCodeLen},
		{"country", trimmed.Country, maxCountryLen},
	}
	for _, f := range required {
		if f.value == "" {
			return &ErrValidation{Field: f.field, Message: "is required"}
		}
		if len(f.value) > f.maxLen {
			return &ErrValidation{Field: f.field, Message: "is too long"}
		}
	}

	// Optional fields still length-limited
	if len(trimmed.Label) > maxLabelLen {
		return &ErrValidation{Field: "label", Message: "is too long"}
	}
	if len(trimmed.AddressLine2) > maxAddressLineLen {
		return &ErrValidation{Field: "address_line2", Message: "is too long"}
	}

	return nil
}

// Validate checks that every required field is present (after trimming
// surrounding whitespace), within its maximum length.
func (in UpdateInput) Validate() error {
	trimmed := in.Trimmed()

	required := []struct {
		field  string
		value  string
		maxLen int
	}{
		{"recipient_name", trimmed.RecipientName, maxRecipientNameLen},
		{"phone", trimmed.Phone, maxPhoneLen},
		{"address_line1", trimmed.AddressLine1, maxAddressLineLen},
		{"city", trimmed.City, maxCityLen},
		{"postal_code", trimmed.PostalCode, maxPostalCodeLen},
		{"country", trimmed.Country, maxCountryLen},
	}
	for _, f := range required {
		if f.value == "" {
			return &ErrValidation{Field: f.field, Message: "is required"}
		}
		if len(f.value) > f.maxLen {
			return &ErrValidation{Field: f.field, Message: "is too long"}
		}
	}

	// Optional fields still length-limited
	if len(trimmed.Label) > maxLabelLen {
		return &ErrValidation{Field: "label", Message: "is too long"}
	}
	if len(trimmed.AddressLine2) > maxAddressLineLen {
		return &ErrValidation{Field: "address_line2", Message: "is too long"}
	}

	return nil
}

// Trimmed returns a copy of in with every string field's surrounding
// whitespace removed.
func (in CreateInput) Trimmed() CreateInput {
	return CreateInput{
		Label:         strings.TrimSpace(in.Label),
		RecipientName: strings.TrimSpace(in.RecipientName),
		Phone:         strings.TrimSpace(in.Phone),
		AddressLine1:  strings.TrimSpace(in.AddressLine1),
		AddressLine2:  strings.TrimSpace(in.AddressLine2),
		City:          strings.TrimSpace(in.City),
		PostalCode:    strings.TrimSpace(in.PostalCode),
		Country:       strings.TrimSpace(in.Country),
		IsDefault:     in.IsDefault,
	}
}

// Trimmed returns a copy of in with every string field's surrounding
// whitespace removed.
func (in UpdateInput) Trimmed() UpdateInput {
	return UpdateInput{
		Label:         strings.TrimSpace(in.Label),
		RecipientName: strings.TrimSpace(in.RecipientName),
		Phone:         strings.TrimSpace(in.Phone),
		AddressLine1:  strings.TrimSpace(in.AddressLine1),
		AddressLine2:  strings.TrimSpace(in.AddressLine2),
		City:          strings.TrimSpace(in.City),
		PostalCode:    strings.TrimSpace(in.PostalCode),
		Country:       strings.TrimSpace(in.Country),
		IsDefault:     in.IsDefault,
	}
}