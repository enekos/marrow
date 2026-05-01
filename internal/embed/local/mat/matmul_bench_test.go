package mat

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

func BenchmarkMatMulTransposeB(b *testing.B) {
	sizes := []struct{ M, K, N int }{
		{4, 384, 384},
		{8, 384, 384},
		{16, 384, 384},
		{32, 384, 384},
		{4, 384, 1536},
		{4, 1536, 384},
	}
	for _, sz := range sizes {
		a := make([]float32, sz.M*sz.K)
		bt := make([]float32, sz.N*sz.K)
		c := make([]float32, sz.M*sz.N)
		for i := range a { a[i] = rand.Float32() }
		for i := range bt { bt[i] = rand.Float32() }
		b.Run(fmt.Sprintf("%dx%d_%d", sz.M, sz.K, sz.N), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				MatMulTransposeB(a, bt, c, sz.M, sz.K, sz.N)
			}
		})
	}
}
