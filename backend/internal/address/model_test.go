package address

import (
	"testing"
)

func TestCreateInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   CreateInput
		wantErr bool
	}{
		{
			name: "valid input with all fields",
			input: CreateInput{
				Label:         "Home",
				RecipientName: "John Doe",
				Phone:         "+1234567890",
				AddressLine1:  "123 Main St",
				AddressLine2:  "Apt 4",
				City:          "Tehran",
				PostalCode:    "12345",
				Country:       "Iran",
				IsDefault:     true,
			},
			wantErr: false,
		},
		{
			name: "valid input without optional fields",
			input: CreateInput{
				Label:         "",
				RecipientName: "John Doe",
				Phone:         "+1234567890",
				AddressLine1:  "123 Main St",
				AddressLine2:  "",
				City:          "Tehran",
				PostalCode:    "12345",
				Country:       "Iran",
				IsDefault:     false,
			},
			wantErr: false,
		},
		{
			name: "missing recipient name",
			input: CreateInput{
				RecipientName: "",
				Phone:         "+1234567890",
				AddressLine1:  "123 Main St",
				City:          "Tehran",
				PostalCode:    "12345",
				Country:       "Iran",
			},
			wantErr: true,
		},
		{
			name: "missing phone",
			input: CreateInput{
				RecipientName: "John Doe",
				Phone:         "",
				AddressLine1:  "123 Main St",
				City:          "Tehran",
				PostalCode:    "12345",
				Country:       "Iran",
			},
			wantErr: true,
		},
		{
			name: "missing address line 1",
			input: CreateInput{
				RecipientName: "John Doe",
				Phone:         "+1234567890",
				AddressLine1:  "",
				City:          "Tehran",
				PostalCode:    "12345",
				Country:       "Iran",
			},
			wantErr: true,
		},
		{
			name: "missing city",
			input: CreateInput{
				RecipientName: "John Doe",
				Phone:         "+1234567890",
				AddressLine1:  "123 Main St",
				City:          "",
				PostalCode:    "12345",
				Country:       "Iran",
			},
			wantErr: true,
		},
		{
			name: "missing postal code",
			input: CreateInput{
				RecipientName: "John Doe",
				Phone:         "+1234567890",
				AddressLine1:  "123 Main St",
				City:          "Tehran",
				PostalCode:    "",
				Country:       "Iran",
			},
			wantErr: true,
		},
		{
			name: "missing country",
			input: CreateInput{
				RecipientName: "John Doe",
				Phone:         "+1234567890",
				AddressLine1:  "123 Main St",
				City:          "Tehran",
				PostalCode:    "12345",
				Country:       "",
			},
			wantErr: true,
		},
		{
			name: "label too long",
			input: CreateInput{
				Label:         stringOfLength(256),
				RecipientName: "John Doe",
				Phone:         "+1234567890",
				AddressLine1:  "123 Main St",
				City:          "Tehran",
				PostalCode:    "12345",
				Country:       "Iran",
			},
			wantErr: true,
		},
		{
			name: "recipient name too long",
			input: CreateInput{
				RecipientName: stringOfLength(256),
				Phone:         "+1234567890",
				AddressLine1:  "123 Main St",
				City:          "Tehran",
				PostalCode:    "12345",
				Country:       "Iran",
			},
			wantErr: true,
		},
		{
			name: "phone too long",
			input: CreateInput{
				RecipientName: "John Doe",
				Phone:         stringOfLength(21),
				AddressLine1:  "123 Main St",
				City:          "Tehran",
				PostalCode:    "12345",
				Country:       "Iran",
			},
			wantErr: true,
		},
		{
			name: "address line 1 too long",
			input: CreateInput{
				RecipientName: "John Doe",
				Phone:         "+1234567890",
				AddressLine1:  stringOfLength(256),
				City:          "Tehran",
				PostalCode:    "12345",
				Country:       "Iran",
			},
			wantErr: true,
		},
		{
			name: "address line 2 too long",
			input: CreateInput{
				RecipientName: "John Doe",
				Phone:         "+1234567890",
				AddressLine1:  "123 Main St",
				AddressLine2:  stringOfLength(256),
				City:          "Tehran",
				PostalCode:    "12345",
				Country:       "Iran",
			},
			wantErr: true,
		},
		{
			name: "city too long",
			input: CreateInput{
				RecipientName: "John Doe",
				Phone:         "+1234567890",
				AddressLine1:  "123 Main St",
				City:          stringOfLength(129),
				PostalCode:    "12345",
				Country:       "Iran",
			},
			wantErr: true,
		},
		{
			name: "postal code too long",
			input: CreateInput{
				RecipientName: "John Doe",
				Phone:         "+1234567890",
				AddressLine1:  "123 Main St",
				City:          "Tehran",
				PostalCode:    stringOfLength(21),
				Country:       "Iran",
			},
			wantErr: true,
		},
		{
			name: "country too long",
			input: CreateInput{
				RecipientName: "John Doe",
				Phone:         "+1234567890",
				AddressLine1:  "123 Main St",
				City:          "Tehran",
				PostalCode:    "12345",
				Country:       stringOfLength(129),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trimmed := tt.input.Trimmed()
			err := trimmed.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   UpdateInput
		wantErr bool
	}{
		{
			name: "valid input",
			input: UpdateInput{
				Label:         "Work",
				RecipientName: "Jane Doe",
				Phone:         "+0987654321",
				AddressLine1:  "456 Oak St",
				AddressLine2:  "",
				City:          "Mashhad",
				PostalCode:    "67890",
				Country:       "Iran",
				IsDefault:     true,
			},
			wantErr: false,
		},
		{
			name: "invalid input - missing fields",
			input: UpdateInput{
				RecipientName: "",
				Phone:         "",
				AddressLine1:  "",
				City:          "",
				PostalCode:    "",
				Country:       "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trimmed := tt.input.Trimmed()
			err := trimmed.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateInput_Trimmed(t *testing.T) {
	input := CreateInput{
		Label:         "  Home  ",
		RecipientName: "  John Doe  ",
		Phone:         "  +1234567890  ",
		AddressLine1:  "  123 Main St  ",
		AddressLine2:  "  Apt 4  ",
		City:          "  Tehran  ",
		PostalCode:    "  12345  ",
		Country:       "  Iran  ",
		IsDefault:     true,
	}
	trimmed := input.Trimmed()

	if trimmed.Label != "Home" {
		t.Errorf("expected trimmed label 'Home', got %q", trimmed.Label)
	}
	if trimmed.RecipientName != "John Doe" {
		t.Errorf("expected trimmed recipient name 'John Doe', got %q", trimmed.RecipientName)
	}
	if trimmed.Phone != "+1234567890" {
		t.Errorf("expected trimmed phone '+1234567890', got %q", trimmed.Phone)
	}
	if trimmed.AddressLine1 != "123 Main St" {
		t.Errorf("expected trimmed address line 1 '123 Main St', got %q", trimmed.AddressLine1)
	}
	if trimmed.AddressLine2 != "Apt 4" {
		t.Errorf("expected trimmed address line 2 'Apt 4', got %q", trimmed.AddressLine2)
	}
	if trimmed.City != "Tehran" {
		t.Errorf("expected trimmed city 'Tehran', got %q", trimmed.City)
	}
	if trimmed.PostalCode != "12345" {
		t.Errorf("expected trimmed postal code '12345', got %q", trimmed.PostalCode)
	}
	if trimmed.Country != "Iran" {
		t.Errorf("expected trimmed country 'Iran', got %q", trimmed.Country)
	}
	if !trimmed.IsDefault {
		t.Error("expected IsDefault to remain true")
	}
}

func TestUpdateInput_Trimmed(t *testing.T) {
	input := UpdateInput{
		Label:         "  Work  ",
		RecipientName: "  Jane Doe  ",
		Phone:         "  +0987654321  ",
		AddressLine1:  "  456 Oak St  ",
		AddressLine2:  "  Suite 100  ",
		City:          "  Mashhad  ",
		PostalCode:    "  67890  ",
		Country:       "  Iran  ",
		IsDefault:     false,
	}
	trimmed := input.Trimmed()

	if trimmed.Label != "Work" {
		t.Errorf("expected trimmed label 'Work', got %q", trimmed.Label)
	}
	if trimmed.RecipientName != "Jane Doe" {
		t.Errorf("expected trimmed recipient name 'Jane Doe', got %q", trimmed.RecipientName)
	}
	if trimmed.Phone != "+0987654321" {
		t.Errorf("expected trimmed phone '+0987654321', got %q", trimmed.Phone)
	}
	if trimmed.AddressLine1 != "456 Oak St" {
		t.Errorf("expected trimmed address line 1 '456 Oak St', got %q", trimmed.AddressLine1)
	}
	if trimmed.AddressLine2 != "Suite 100" {
		t.Errorf("expected trimmed address line 2 'Suite 100', got %q", trimmed.AddressLine2)
	}
	if trimmed.City != "Mashhad" {
		t.Errorf("expected trimmed city 'Mashhad', got %q", trimmed.City)
	}
	if trimmed.PostalCode != "67890" {
		t.Errorf("expected trimmed postal code '67890', got %q", trimmed.PostalCode)
	}
	if trimmed.Country != "Iran" {
		t.Errorf("expected trimmed country 'Iran', got %q", trimmed.Country)
	}
	if trimmed.IsDefault {
		t.Error("expected IsDefault to remain false")
	}
}

func stringOfLength(n int) string {
	s := make([]byte, n)
	for i := 0; i < n; i++ {
		s[i] = 'a'
	}
	return string(s)
}