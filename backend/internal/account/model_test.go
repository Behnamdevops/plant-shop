package account

import (
	"testing"
)

func TestUpdateInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   UpdateInput
		wantErr bool
	}{
		{
			name: "valid input",
			input: UpdateInput{
				Name:  "John Doe",
				Phone: "+1234567890",
			},
			wantErr: false,
		},
		{
			name: "valid input without phone",
			input: UpdateInput{
				Name:  "John Doe",
				Phone: "",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			input: UpdateInput{
				Name:  "",
				Phone: "+1234567890",
			},
			wantErr: true,
		},
		{
			name: "name too long",
			input: UpdateInput{
				Name:  stringOfLength(256),
				Phone: "+1234567890",
			},
			wantErr: true,
		},
		{
			name: "phone too long",
			input: UpdateInput{
				Name:  "John Doe",
				Phone: stringOfLength(21),
			},
			wantErr: true,
		},
		{
			name: "valid long phone",
			input: UpdateInput{
				Name:  "John Doe",
				Phone: stringOfLength(20),
			},
			wantErr: false,
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

func TestUpdateInput_Trimmed(t *testing.T) {
	input := UpdateInput{
		Name:  "  John Doe  ",
		Phone: "  +1234567890  ",
	}
	trimmed := input.Trimmed()

	if trimmed.Name != "John Doe" {
		t.Errorf("expected trimmed name 'John Doe', got %q", trimmed.Name)
	}
	if trimmed.Phone != "+1234567890" {
		t.Errorf("expected trimmed phone '+1234567890', got %q", trimmed.Phone)
	}
}

func stringOfLength(n int) string {
	s := make([]byte, n)
	for i := 0; i < n; i++ {
		s[i] = 'a'
	}
	return string(s)
}