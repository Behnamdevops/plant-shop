package cart

import (
	"sync"
	"testing"
)

func TestMergeRetriesAreIdempotentAndStockBounded(t *testing.T) {
	e := newTestEnv(t)
	u := e.createUser(t)
	p := e.createProduct(t, 3)
	input := MergeInput{Key: uniqueSuffix(), Items: []AddItemInput{{ProductID: p, Quantity: 5}}}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := e.cartRepo.Merge(t.Context(), u, input); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var quantity, batches int
	if err := e.cartRepo.db.QueryRow(t.Context(), "SELECT quantity FROM cart_items WHERE user_id=$1 AND product_id=$2", u, p).Scan(&quantity); err != nil {
		t.Fatal(err)
	}
	if quantity != 3 {
		t.Fatalf("quantity %d, want 3", quantity)
	}
	e.cartRepo.db.QueryRow(t.Context(), "SELECT count(*) FROM cart_merge_batches WHERE user_id=$1", u).Scan(&batches)
	if batches != 1 {
		t.Fatalf("batches %d", batches)
	}
}
