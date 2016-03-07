package records

import (
	"math/rand"
	"strconv"
	"testing"
)

// BenchmarkInsertRR *only* tests insertRR, not the taskRecord funcs.
func BenchmarkInsertRR(b *testing.B) {
	const (
		clusterSize = 1000
		appCount    = 5
	)
	var (
		slaves = make([]string, clusterSize)
		apps   = make([]string, appCount)
		rg     = &RecordGenerator{
			As:   rrs{},
			SRVs: rrs{},
		}
	)
	for i := 0; i < clusterSize; i++ {
		slaves[i] = "slave" + strconv.Itoa(i)
	}
	for i := 0; i < appCount; i++ {
		apps[i] = "app" + strconv.Itoa(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var (
			si = rand.Int31n(clusterSize)
			ai = rand.Int31n(appCount)
		)
		rg.insertRR(apps[ai], slaves[si], "A")
	}
}
