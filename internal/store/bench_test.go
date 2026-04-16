package store_test

import (
	"path/filepath"
	"testing"

	"github.com/yjmrobert/txsim/internal/store"
)

// BenchmarkWithLock measures the cost of a load→mutate→save cycle under the
// file lock on a small state. Representative of a single `mockpay charge`.
func BenchmarkWithLock(b *testing.B) {
	dir := b.TempDir()
	s, err := store.Open(filepath.Join(dir, "state.json"))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := s.WithLock(func(st *store.State) error {
			st.Customers["cus_bench"] = store.Customer{ID: "cus_bench", Email: "a@b.com"}
			return nil
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
