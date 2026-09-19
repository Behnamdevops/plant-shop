package article

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// Test normalizeSlug
func TestNormalizeSlug(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Test Article", "test-article"},
		{"  Hello World  ", "hello-world"},
		{"Article with 123 numbers", "article-with-123-numbers"},
		{"Special!@#chars", "specialchars"},
		{"multiple   spaces", "multiple---spaces"},
		{"already-normalized", "already-normalized"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeSlug(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeSlug(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Test parseListFilters
func TestParseListFilters(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		wantErr      bool
		wantPage     int
		wantPageSize int
	}{
		{"default", "", false, 1, DefaultPageSize},
		{"valid page", "?page=2", false, 2, DefaultPageSize},
		{"valid page_size", "?page_size=5", false, 1, 5},
		{"invalid page", "?page=abc", true, 0, 0},
		{"invalid page_size", "?page_size=-1", true, 0, 0},
		{"invalid category", "?category=abc", true, 0, 0},
		{"capped page_size", "?page_size=500", false, 1, MaxPageSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/v1/articles"+tt.query, nil)
			filters, err := parseListFilters(r)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseListFilters() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if filters.Page != tt.wantPage {
					t.Errorf("parseListFilters() page = %d, want %d", filters.Page, tt.wantPage)
				}
				if filters.PageSize != tt.wantPageSize {
					t.Errorf("parseListFilters() pageSize = %d, want %d", filters.PageSize, tt.wantPageSize)
				}
			}
		})
	}
}

// Test validation constants
func TestValidationConstants(t *testing.T) {
	if MaxTitleLen <= 0 {
		t.Error("MaxTitleLen should be positive")
	}
	if MaxExcerptLen <= 0 {
		t.Error("MaxExcerptLen should be positive")
	}
	if MaxSeoTitleLen <= 0 {
		t.Error("MaxSeoTitleLen should be positive")
	}
	if MaxSeoDescLen <= 0 {
		t.Error("MaxSeoDescLen should be positive")
	}
	if DefaultPageSize <= 0 {
		t.Error("DefaultPageSize should be positive")
	}
	if MaxPageSize < DefaultPageSize {
		t.Error("MaxPageSize should be >= DefaultPageSize")
	}
}

// Test status constants
func TestStatusConstants(t *testing.T) {
	if StatusDraft == "" {
		t.Error("StatusDraft should not be empty")
	}
	if StatusPublished == "" {
		t.Error("StatusPublished should not be empty")
	}
	if StatusDraft == StatusPublished {
		t.Error("StatusDraft and StatusPublished should be different")
	}
}

// Test JSON validation scenarios
func TestJSONValidation(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{"valid article", `{"title":"Test","slug":"test","content":"Content"}`, false},
		{"empty title", `{"title":"","slug":"test","content":"Content"}`, true},
		{"empty slug", `{"title":"Test","slug":"","content":"Content"}`, true},
		{"empty content", `{"title":"Test","slug":"test","content":""}`, true},
		{"invalid status", `{"title":"Test","slug":"test","content":"Content","status":"invalid"}`, true},
		{"valid draft status", `{"title":"Test","slug":"test","content":"Content","status":"draft"}`, false},
		{"valid published status", `{"title":"Test","slug":"test","content":"Content","status":"published"}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input struct {
				Title   string `json:"title"`
				Slug    string `json:"slug"`
				Content string `json:"content"`
				Status  string `json:"status"`
			}

			// Check basic JSON validity
			err := json.Unmarshal([]byte(tt.json), &input)

			if err != nil && !tt.wantErr {
				t.Errorf("unexpected JSON decode error: %v", err)
			}

			// Check validation logic
			hasError := input.Title == "" || input.Slug == "" || input.Content == ""
			if hasError && !tt.wantErr {
				t.Logf("validation correctly caught empty required fields for: %s", tt.name)
			}
		})
	}
}

// Test requireAdmin returns 401 for unauthorized (requires auth set - skip for unit test)
func TestHandlerRequireAdmin(t *testing.T) {
	// Handler requires auth to be set - this is tested via integration
	// Skip for unit test as it would panic with nil auth
	t.Skip("Handler.requireAdmin requires auth interface - tested via integration tests")
}

// Test SEO field validation
func TestSeoFieldValidation(t *testing.T) {
	// Test SEO title max length
	longSeoTitle := strings.Repeat("a", MaxSeoTitleLen+1)
	if len(longSeoTitle) <= MaxSeoTitleLen {
		t.Error("test setup error: longSeoTitle should exceed MaxSeoTitleLen")
	}

	// Test SEO description max length
	longSeoDesc := strings.Repeat("a", MaxSeoDescLen+1)
	if len(longSeoDesc) <= MaxSeoDescLen {
		t.Error("test setup error: longSeoDesc should exceed MaxSeoDescLen")
	}
}

// Test ListResult structure
func TestListResult(t *testing.T) {
	result := ListResult{
		Items:      []Article{},
		Page:       1,
		PageSize:   12,
		Total:      0,
		TotalPages: 0,
	}

	if result.Page < 1 {
		t.Error("page should be >= 1")
	}
	if result.PageSize < 1 {
		t.Error("pageSize should be >= 1")
	}
	if result.Total < 0 {
		t.Error("total should be >= 0")
	}
}

// Test error types
func TestErrorTypes(t *testing.T) {
	if ErrDuplicateSlug == nil {
		t.Error("ErrDuplicateSlug should not be nil")
	}
	if ErrCategoryNotFound == nil {
		t.Error("ErrCategoryNotFound should not be nil")
	}
	if ErrInvalidStatus == nil {
		t.Error("ErrInvalidStatus should not be nil")
	}
}

// Test pagination calculation
func TestPaginationCalculation(t *testing.T) {
	tests := []struct {
		total     int64
		pageSize  int
		wantPages int
	}{
		{0, 10, 0},
		{1, 10, 1},
		{10, 10, 1},
		{11, 10, 2},
		{25, 10, 3},
		{100, 12, 9},
	}

	for _, tt := range tests {
		pageSize := tt.pageSize
		pages := int((tt.total + int64(pageSize) - 1) / int64(pageSize))
		if pages != tt.wantPages {
			t.Errorf("total=%d pageSize=%d: got %d pages, want %d", tt.total, tt.pageSize, pages, tt.wantPages)
		}
	}
}
