package non_blocking_bucket

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyz")
var r1 *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

var holder []byte

var inputTable = []struct {
	payloadSize int
}{
	{payloadSize: 100},
	{payloadSize: 1000},
	{payloadSize: 74382},
	{payloadSize: 382399},
}

type dummyDataDropper struct{}

func (dd *dummyDataDropper) Drop([]byte) {
	// Do nothing
}

type dummyDataEnqueuerMock struct{}

func (w *dummyDataEnqueuerMock) Enqueue(data []byte) error {
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
		l := logger.New(&config.Config{Log: config.LogConfig{Level: "warn", Format: "json"}})
		acc := New(l, (10 * tc.payloadSize), []byte("_n_"), 50, &dummyDataDropper{}, &dummyDataEnqueuerMock{}, prometheus.NewRegistry())

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