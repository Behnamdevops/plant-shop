package product

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Behnamdevops/plant-shop/backend/internal/category"
)

// seedListingProduct creates a product with a unique name/slug for listing
// tests and registers cleanup.
func seedListingProduct(t *testing.T, repo *Repository, name, slugSuffix string, price int64, stock int, categoryID *int64) Product {
	t.Helper()
	slug := uniqueSlug(slugSuffix)
	p, err := repo.Create(context.Background(), CreateProductInput{
		Name:       name,
		Slug:       slug,
		Price:      price,
		Stock:      stock,
		CategoryID: categoryID,
	})
	if err != nil {
		t.Fatalf("seed product failed: %v", err)
	}
	t.Cleanup(func() {
		repo.db.Exec(context.Background(), "DELETE FROM products WHERE id = $1", p.ID)
	})
	return p
}

func TestRepositoryListDefault(t *testing.T) {
	repo := newTestRepository(t)
	suffix := uniqueSlug("listing-default")

	p1 := seedListingProduct(t, repo, "Listing Default A "+suffix, "listing-default-a-"+suffix, 100, 5, nil)
	p2 := seedListingProduct(t, repo, "Listing Default B "+suffix, "listing-default-b-"+suffix, 200, 3, nil)

	result, err := repo.List(context.Background(), ListFilters{Sort: DefaultSort, Page: 1, PageSize: DefaultPageSize})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	var foundA, foundB bool
	for _, item := range result.Items {
		if item.ID == p1.ID {
			foundA = true
		}
		if item.ID == p2.ID {
			foundB = true
		}
	}
	if !foundA || !foundB {
		t.Error("expected both seeded products to appear in default listing")
	}
	if result.Total < 2 {
		t.Errorf("expected total >= 2, got %d", result.Total)
	}
}

func TestRepositoryListTextSearch(t *testing.T) {
	repo := newTestRepository(t)
	suffix := uniqueSlug("search")

	match := seedListingProduct(t, repo, "UniqueFicusLyrata"+suffix, "search-match-"+suffix, 100, 5, nil)
	seedListingProduct(t, repo, "Unrelated Plant "+suffix, "search-nomatch-"+suffix, 100, 5, nil)

	result, err := repo.List(context.Background(), ListFilters{
		Query: "UniqueFicusLyrata" + suffix, Sort: DefaultSort, Page: 1, PageSize: DefaultPageSize,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != match.ID {
		t.Fatalf("expected exactly the matching product, got %+v", result.Items)
	}
}

func TestRepositoryListCategoryFilter(t *testing.T) {
	repo := newTestRepository(t)
	catRepo := category.NewRepository(repo.db)
	suffix := uniqueSlug("catfilter")

	cat, err := catRepo.Create(context.Background(), category.CreateCategoryInput{Name: "Filter Cat " + suffix, Slug: "filter-cat-" + suffix})
	if err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	t.Cleanup(func() { repo.db.Exec(context.Background(), "DELETE FROM categories WHERE id = $1", cat.ID) })

	inCategory := seedListingProduct(t, repo, "In Category "+suffix, "in-category-"+suffix, 100, 5, &cat.ID)
	seedListingProduct(t, repo, "Uncategorized "+suffix, "uncategorized-"+suffix, 100, 5, nil)

	result, err := repo.List(context.Background(), ListFilters{
		CategoryID: &cat.ID, Sort: DefaultSort, Page: 1, PageSize: DefaultPageSize,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != inCategory.ID {
		t.Fatalf("expected exactly the categorized product, got %+v", result.Items)
	}
}

func TestRepositoryListInStockFilter(t *testing.T) {
	repo := newTestRepository(t)
	suffix := uniqueSlug("instock")

	inStock := seedListingProduct(t, repo, "In Stock "+suffix, "in-stock-"+suffix, 100, 5, nil)
	seedListingProduct(t, repo, "Out Of Stock "+suffix, "out-of-stock-"+suffix, 100, 0, nil)

	result, err := repo.List(context.Background(), ListFilters{
		InStock: true, Query: suffix, Sort: DefaultSort, Page: 1, PageSize: DefaultPageSize,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != inStock.ID {
		t.Fatalf("expected exactly the in-stock product, got %+v", result.Items)
	}
}

func TestRepositoryListPriceRange(t *testing.T) {
	repo := newTestRepository(t)
	suffix := uniqueSlug("pricerange")

	seedListingProduct(t, repo, "Cheap "+suffix, "cheap-"+suffix, 50, 5, nil)
	mid := seedListingProduct(t, repo, "Mid "+suffix, "mid-"+suffix, 150, 5, nil)
	seedListingProduct(t, repo, "Expensive "+suffix, "expensive-"+suffix, 500, 5, nil)

	minPrice := int64(100)
	maxPrice := int64(200)
	result, err := repo.List(context.Background(), ListFilters{
		Query: suffix, MinPrice: &minPrice, MaxPrice: &maxPrice, Sort: DefaultSort, Page: 1, PageSize: DefaultPageSize,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != mid.ID {
		t.Fatalf("expected exactly the mid-priced product, got %+v", result.Items)
	}
}

func TestRepositoryListSortModes(t *testing.T) {
	repo := newTestRepository(t)
	suffix := uniqueSlug("sortmodes")

	low := seedListingProduct(t, repo, "AAA Sort "+suffix, "aaa-sort-"+suffix, 10, 5, nil)
	high := seedListingProduct(t, repo, "ZZZ Sort "+suffix, "zzz-sort-"+suffix, 999, 5, nil)

	tests := []struct {
		sort      string
		wantFirst int64
	}{
		{SortPriceAsc, low.ID},
		{SortPriceDesc, high.ID},
		{SortNameAsc, low.ID}, // "AAA..." < "ZZZ..."
	}

	for _, tt := range tests {
		t.Run(tt.sort, func(t *testing.T) {
			result, err := repo.List(context.Background(), ListFilters{
				Query: suffix, Sort: tt.sort, Page: 1, PageSize: DefaultPageSize,
			})
			if err != nil {
				t.Fatalf("List failed: %v", err)
			}
			if len(result.Items) == 0 {
				t.Fatal("expected results")
			}
			if result.Items[0].ID != tt.wantFirst {
				t.Errorf("sort %s: expected first item ID %d, got %d", tt.sort, tt.wantFirst, result.Items[0].ID)
			}
		})
	}
}

func TestRepositoryListPagination(t *testing.T) {
	repo := newTestRepository(t)
	suffix := uniqueSlug("pagination")

	for i := 0; i < 5; i++ {
		seedListingProduct(t, repo, "Page Item "+suffix, "page-item-"+suffix+"-"+uniqueSlug("i"), 100, 5, nil)
	}

	result, err := repo.List(context.Background(), ListFilters{
		Query: suffix, Sort: DefaultSort, Page: 1, PageSize: 2,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected page size 2, got %d items", len(result.Items))
	}
	if result.Total != 5 {
		t.Fatalf("expected total 5, got %d", result.Total)
	}
	if result.TotalPages != 3 {
		t.Fatalf("expected 3 total pages, got %d", result.TotalPages)
	}

	page2, err := repo.List(context.Background(), ListFilters{
		Query: suffix, Sort: DefaultSort, Page: 2, PageSize: 2,
	})
	if err != nil {
		t.Fatalf("List page 2 failed: %v", err)
	}
	if len(page2.Items) != 2 {
		t.Fatalf("expected page 2 size 2, got %d items", len(page2.Items))
	}
	// Pages must not overlap.
	for _, a := range result.Items {
		for _, b := range page2.Items {
			if a.ID == b.ID {
				t.Errorf("page 1 and page 2 overlap on product %d", a.ID)
			}
		}
	}
}

func TestRepositoryListCombinedFilters(t *testing.T) {
	repo := newTestRepository(t)
	catRepo := category.NewRepository(repo.db)
	suffix := uniqueSlug("combined")

	cat, err := catRepo.Create(context.Background(), category.CreateCategoryInput{Name: "Combined Cat " + suffix, Slug: "combined-cat-" + suffix})
	if err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	t.Cleanup(func() { repo.db.Exec(context.Background(), "DELETE FROM categories WHERE id = $1", cat.ID) })

	match := seedListingProduct(t, repo, "Combined Match "+suffix, "combined-match-"+suffix, 150, 5, &cat.ID)
	seedListingProduct(t, repo, "Combined WrongPrice "+suffix, "combined-wrongprice-"+suffix, 999, 5, &cat.ID)
	seedListingProduct(t, repo, "Combined WrongStock "+suffix, "combined-wrongstock-"+suffix, 150, 0, &cat.ID)
	seedListingProduct(t, repo, "Combined WrongCategory "+suffix, "combined-wrongcategory-"+suffix, 150, 5, nil)

	minPrice := int64(100)
	maxPrice := int64(200)
	result, err := repo.List(context.Background(), ListFilters{
		Query: suffix, CategoryID: &cat.ID, InStock: true, MinPrice: &minPrice, MaxPrice: &maxPrice,
		Sort: DefaultSort, Page: 1, PageSize: DefaultPageSize,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != match.ID {
		t.Fatalf("expected exactly the fully-matching product, got %+v", result.Items)
	}
}

func TestRepositoryListNoSQLInjectionThroughQuery(t *testing.T) {
	repo := newTestRepository(t)

	// A classic injection payload passed as the free-text search query.
	// Parameterized queries treat this as a literal string; it must not
	// error out or return unexpected rows (e.g. by dropping a table or
	// bypassing filters).
	malicious := "'; DROP TABLE products; --"
	result, err := repo.List(context.Background(), ListFilters{
		Query: malicious, Sort: DefaultSort, Page: 1, PageSize: DefaultPageSize,
	})
	if err != nil {
		t.Fatalf("List with malicious query should not error, got: %v", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("expected no matches for nonsense query, got %d", len(result.Items))
	}

	// Confirm the products table still exists and is queryable.
	if _, err := repo.List(context.Background(), ListFilters{Sort: DefaultSort, Page: 1, PageSize: 1}); err != nil {
		t.Fatalf("products table appears to have been affected: %v", err)
	}
}

func TestRepositoryListUncategorizedProductsStillWork(t *testing.T) {
	repo := newTestRepository(t)
	suffix := uniqueSlug("uncategorized")

	p := seedListingProduct(t, repo, "Legacy Uncategorized "+suffix, "legacy-uncategorized-"+suffix, 100, 5, nil)

	got, err := repo.GetByID(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.CategoryID != nil {
		t.Errorf("expected nil CategoryID for uncategorized product, got %v", *got.CategoryID)
	}

	result, err := repo.List(context.Background(), ListFilters{
		Query: suffix, Sort: DefaultSort, Page: 1, PageSize: DefaultPageSize,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != p.ID {
		t.Fatalf("expected uncategorized product to appear in listing, got %+v", result.Items)
	}
}

func TestRepositoryCreateWithInvalidCategory(t *testing.T) {
	repo := newTestRepository(t)

	invalidCategoryID := int64(999999999)
	_, err := repo.Create(context.Background(), CreateProductInput{
		Name:       "Bad Category Product",
		Slug:       uniqueSlug("bad-category"),
		Price:      100,
		Stock:      1,
		CategoryID: &invalidCategoryID,
	})
	if err != ErrCategoryNotFound {
		t.Fatalf("expected ErrCategoryNotFound, got %v", err)
	}
}

func TestRepositoryCreateAndUpdateWithCategory(t *testing.T) {
	repo := newTestRepository(t)
	catRepo := category.NewRepository(repo.db)
	suffix := uniqueSlug("create-update-cat")

	cat, err := catRepo.Create(context.Background(), category.CreateCategoryInput{Name: "CU Cat " + suffix, Slug: "cu-cat-" + suffix})
	if err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	t.Cleanup(func() { repo.db.Exec(context.Background(), "DELETE FROM categories WHERE id = $1", cat.ID) })

	p, err := repo.Create(context.Background(), CreateProductInput{
		Name: "CU Product " + suffix, Slug: uniqueSlug("cu-product"), Price: 100, Stock: 1, CategoryID: &cat.ID,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	t.Cleanup(func() { repo.db.Exec(context.Background(), "DELETE FROM products WHERE id = $1", p.ID) })

	if p.CategoryID == nil || *p.CategoryID != cat.ID {
		t.Fatalf("expected category_id %d, got %v", cat.ID, p.CategoryID)
	}

	// Clear category via update.
	name, slug, desc := p.Name, p.Slug, p.Description
	price, stock := p.Price, p.Stock
	updated, err := repo.Update(context.Background(), p.ID, UpdateProductInput{
		Name: &name, Slug: &slug, Description: &desc, Price: &price, Stock: &stock, ClearCategory: true,
	})
	if err != nil {
		t.Fatalf("Update (clear category) failed: %v", err)
	}
	if updated.CategoryID != nil {
		t.Errorf("expected category_id to be cleared, got %v", *updated.CategoryID)
	}
}

// ---------- Handler-level validation tests ----------

func TestHandlerListInvalidSort(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products?sort=not_a_real_sort", nil)
	w := httptest.NewRecorder()
	env.handler.List(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandlerListMaxPageSizeClamped(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products?page_size=1000", nil)
	w := httptest.NewRecorder()
	env.handler.List(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	var result ListResult
	if err := decodeJSON(w, &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.PageSize != MaxPageSize {
		t.Errorf("expected page_size clamped to %d, got %d", MaxPageSize, result.PageSize)
	}
}

func TestHandlerListInvalidPage(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products?page=0", nil)
	w := httptest.NewRecorder()
	env.handler.List(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandlerListInvalidPriceRange(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products?min_price=500&max_price=100", nil)
	w := httptest.NewRecorder()
	env.handler.List(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandlerListDefaultResponseShape(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	w := httptest.NewRecorder()
	env.handler.List(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}
	var result ListResult
	if err := decodeJSON(w, &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Page != 1 {
		t.Errorf("expected default page 1, got %d", result.Page)
	}
	if result.PageSize != DefaultPageSize {
		t.Errorf("expected default page_size %d, got %d", DefaultPageSize, result.PageSize)
	}
	if result.Items == nil {
		t.Error("expected items to be a non-nil (possibly empty) slice")
	}
}

func decodeJSON(w *httptest.ResponseRecorder, target any) error {
	return json.Unmarshal(w.Body.Bytes(), target)
}
