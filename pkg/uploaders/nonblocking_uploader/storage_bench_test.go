package nonblocking_uploader_test

import (
	"container/list"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyz")
var r1 *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func randSeq(n int) string {
	b := make([]rune, n)

	for i := range b {
		b[i] = letters[r1.Intn(len(letters))]
	}
	return string(b)
}

var holder []byte

var inputTable = []struct {
	payloadSize int
}{
	{payloadSize: 100},
	{payloadSize: 1000},
	{payloadSize: 74382},
	{payloadSize: 382399},
}

func exerciseListBack(l *list.List, payload string, loops int) {
	for i := 0; i < loops; i++ {
		l.PushBack([]byte(fmt.Sprintf("%s%d", payload, i)))
	}

	for i := 0; i < loops; i++ {
		e := l.Back()
		if e == nil {
			fmt.Println("something went wrong")
			panic("got a nil value")
		}
		holder = e.Value.([]byte)
		l.Remove(e)
	}
}

func exerciseListFront(l *list.List, payload string, loops int) {
	for i := 0; i < loops; i++ {
		l.PushFront([]byte(fmt.Sprintf("%s%d", payload, i)))
	}

	for i := 0; i < loops; i++ {
		e := l.Front()
		if e == nil {
			fmt.Println("something went wrong")
			panic("got a nil value")
		}
		holder = e.Value.([]byte)
		l.Remove(e)
	}
}

func exerciseSliceBack(s [][]byte, payload string, loops int) {
	for i := 0; i < loops; i++ {
		s = append(s, []byte(fmt.Sprintf("%s%d", payload, i)))
	}

	for i := 0; i < loops; i++ {
		size := len(s)

		elem := s[size-1]
		s = s[:size-1]

		if elem == nil {
			fmt.Println("something went wrong")
			panic("got a nil value")
		}
		holder = elem
	}
}

func exerciseSliceFront(s [][]byte, payload string, loops int) {
	for i := 0; i < loops; i++ {
		s = append(s, []byte(fmt.Sprintf("%s%d", payload, i)))
	}

	for i := 0; i < loops; i++ {

		elem := s[0]
		s = s[1:]

		if elem == nil {
			fmt.Println("something went wrong")
			panic("got a nil value")
		}
		holder = elem
	}
}

// func BenchmarkListBack(b *testing.B) {
// 	for _, tc := range inputTable {
// 		b.Run(fmt.Sprintf("input_size_%d", tc.payloadSize), func(b *testing.B) {
// 			for n := 0; n < b.N; n++ {
// 				payload := randSeq(tc.payloadSize)
// 				li := list.New()
// 				exerciseListBack(li, payload, 1000)
// 			}
// 		})
// 	}
// }

// func BenchmarkListFront(b *testing.B) {
// 	for _, tc := range inputTable {
// 		b.Run(fmt.Sprintf("input_size_%d", tc.payloadSize), func(b *testing.B) {
// 			for n := 0; n < b.N; n++ {
// 				payload := randSeq(tc.payloadSize)
// 				li := list.New()
// 				exerciseListFront(li, payload, 1000)
// 			}
// 		})
// 	}
// }

// func BenchmarkSliceBack(b *testing.B) {
// 	for _, tc := range inputTable {
// 		b.Run(fmt.Sprintf("input_size_%d", tc.payloadSize), func(b *testing.B) {
// 			for n := 0; n < b.N; n++ {
// 				payload := randSeq(tc.payloadSize)
// 				s := make([][]byte, 0, 1000)
// 				exerciseSliceBack(s, payload, 1000)
// 			}
// 		})
// 	}
// }

// func BenchmarkSliceFront(b *testing.B) {
// 	for _, tc := range inputTable {
// 		b.Run(fmt.Sprintf("input_size_%d", tc.payloadSize), func(b *testing.B) {
// 			for n := 0; n < b.N; n++ {
// 				payload := randSeq(tc.payloadSize)
// 				s := make([][]byte, 0, 1000)
// 				exerciseSliceFront(s, payload, 1000)
// 			}
// 		})
// 	}
// }

// func BenchmarkUploaderStorage(b *testing.B) {
// 	for _, tc := range inputTable {
// 		b.Run(fmt.Sprintf("input_size_%d", tc.payloadSize), func(b *testing.B) {
// 			for n := 0; n < b.N; n++ {
// 				payload := randSeq(tc.payloadSize)
// 				li := list.New()
// 				exerciseListFront(li, payload, 1000)
// 			}
// 		})
// 	}
// }

func BenchmarkUploaderStorage(b *testing.B) {
	for _, tc := range inputTable {
		b.Run(fmt.Sprintf("input_size_%d", tc.payloadSize), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				payload := randSeq(tc.payloadSize)
				s := make([][]byte, 0, 1000)
				exerciseSliceFront(s, payload, 1000)
			}
		})
	}
}
