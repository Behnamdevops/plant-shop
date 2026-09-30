package product

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCareProductDetailsPersistAndFilter(t *testing.T) {
	e := newTestHandlerEnv(t)
	admin := e.registerAndLogin(t, "admin")
	input := CreateProductInput{Name: "کود آزمایشی", Slug: "care-" + handlerUniqueSuffix(), Price: 2980000, Stock: 5, Details: Details{Kind: "fertilizer", Brand: "برند آزمایشی", WeightVolume: "۱ لیتر", Usage: "مطابق برچسب", Warnings: "هشدار آزمایشی", ArticleSlug: "fertilizer-guide"}}
	body, _ := json.Marshal(input)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	e.handler.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var p Product
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	fetched, err := e.handler.repository.GetBySlug(t.Context(), p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if fetched.Details.Brand != input.Details.Brand || fetched.Details.Usage != input.Details.Usage {
		t.Fatalf("details lost: %+v", fetched.Details)
	}
	result, err := e.handler.repository.List(t.Context(), ListFilters{Kind: "fertilizer", Brand: input.Details.Brand, Guide: input.Details.ArticleSlug, Page: 1, PageSize: 20, Sort: "bestselling"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range result.Items {
		if item.ID == p.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("new product missing from kind/brand/guide filters")
	}
	wrong, err := e.handler.repository.List(t.Context(), ListFilters{Kind: "tool", Brand: input.Details.Brand, Page: 1, PageSize: 20, Sort: "newest"})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range wrong.Items {
		if item.ID == p.ID {
			t.Fatal("wrong product kind leaked into filter")
		}
	}
	if err := validateDetails(Details{ExpiryDate: "2026-99-99"}, nil); err == nil {
		t.Fatal("malformed expiry accepted")
	}
	if err := validateDetails(Details{RelatedIDs: []int64{1, 1}}, nil); err == nil {
		t.Fatal("duplicate related IDs accepted")
	}
}
