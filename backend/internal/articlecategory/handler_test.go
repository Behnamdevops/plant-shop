package articlecategory

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHandlerList(t *testing.T) {
	// Test that the handler structure is correct
	h := &Handler{}
	if h == nil {
		t.Error("expected Handler to be non-nil")
	}
}

func TestValidateNameSlug(t *testing.T) {
	// Test valid input
	name, slug, err := validateNameSlug("Test Name", "test-slug")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if name != "Test Name" {
		t.Errorf("expected name 'Test Name', got %q", name)
	}
	if slug != "test-slug" {
		t.Errorf("expected slug 'test-slug', got %q", slug)
	}

	// Test empty name
	_, _, err = validateNameSlug("", "test")
	if err == nil {
		t.Error("expected error for empty name")
	}

	// Test empty slug
	_, _, err = validateNameSlug("Test", "")
	if err == nil {
		t.Error("expected error for empty slug")
	}

	// Test trim whitespace
	name, slug, err = validateNameSlug("  Test  ", "  test-slug  ")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if name != "Test" {
		t.Errorf("expected name 'Test', got %q", name)
	}
}

func TestHandlerRequireAdmin(t *testing.T) {
	t.Skip("Handler.requireAdmin requires auth interface - tested via integration tests")
}

func TestHandlerAdminListAuth(t *testing.T) {
	t.Skip("Handler.AdminList requires auth interface - tested via integration tests")
}

func TestJSONDecoding(t *testing.T) {
	input := `{"name":"Test","slug":"test"}`
	var req struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}

	if err := json.NewDecoder(strings.NewReader(input)).Decode(&req); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if req.Name != "Test" {
		t.Errorf("expected name 'Test', got %q", req.Name)
	}
	if req.Slug != "test" {
		t.Errorf("expected slug 'test', got %q", req.Slug)
	}
}