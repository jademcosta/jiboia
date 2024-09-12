package accumulator

// This test is to be used in case we need to improve the algorithm of the accumulator
// internal storage, used to accumulate data.

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/circuitbreaker"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyz")
var r1 = rand.New(rand.NewSource(time.Now().UnixNano()))

var inputTable = []struct {
	payloadSize int
}{
	{payloadSize: 100},
	{payloadSize: 1000},
	{payloadSize: 74382},
	{payloadSize: 382399},
}

type dummyDataEnqueuerMock struct{}

func (w *dummyDataEnqueuerMock) Enqueue(_ []byte) error {
	//Do nothing
	return nil
}

func randSeq(n int) string {
	b := make([]rune, n)

	for i := range b {
		b[i] = letters[r1.Intn(len(letters))]
	}
	return string(b)
}

func BenchmarkAccumulatorAppend(b *testing.B) {
	for _, tc := range inputTable {
		l := logger.NewDummy()
		acc := New(
			"someflow", l, (10 * tc.payloadSize), []byte("_n_"), 50,
			&dummyDataEnqueuerMock{}, circuitbreaker.NewDummyCircuitBreaker(), prometheus.NewRegistry(),
		)

		b.Run(fmt.Sprintf("input_size_%d", tc.payloadSize), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				payload := randSeq(tc.payloadSize)
				for i := 0; i < 10000; i++ {
					acc.append([]byte(payload))
				}
			}
		})
	}
}
