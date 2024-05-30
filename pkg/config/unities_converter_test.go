package config_test

import (
	"testing"

	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestToBytes(t *testing.T) {
	testCases := []struct {
		size        string
		sizeInBytes int64
		shouldError bool
	}{
		{size: "0", sizeInBytes: 0},
		{size: "", sizeInBytes: 0},
		{size: "1", sizeInBytes: 1},
		{size: "9", sizeInBytes: 9},
		{size: "99", sizeInBytes: 99},
		{size: "278465", sizeInBytes: 278465},
		{size: "1111111111", sizeInBytes: 1111111111},
		{size: "187349873947", sizeInBytes: 187349873947},

		{size: "1kb", sizeInBytes: 1024},
		{size: "1KB", sizeInBytes: 1024},
		{size: "2KB", sizeInBytes: 2048},
		{size: "1024KB", sizeInBytes: 1048576},
		{size: "5000kb", sizeInBytes: 5120000},

		{size: "1mb", sizeInBytes: 1048576},
		{size: "2mb", sizeInBytes: 2 * 1048576},
		{size: "2MB", sizeInBytes: 2 * 1048576},
		{size: "110MB", sizeInBytes: 110 * 1048576},

		{size: "1gb", sizeInBytes: 1073741824},
		{size: "1GB", sizeInBytes: 1073741824},
		{size: "2gb", sizeInBytes: 2 * 1073741824},
		{size: "20gb", sizeInBytes: 20 * 1073741824},

		{size: "1pb", sizeInBytes: 1099511627776},
		{size: "1PB", sizeInBytes: 1099511627776},
		{size: "2PB", sizeInBytes: 2 * 1099511627776},
		{size: "23PB", sizeInBytes: 23 * 1099511627776},
		{size: "530PB", sizeInBytes: 530 * 1099511627776},

		{size: "1eb", sizeInBytes: 1125899906842624},
		{size: "1EB", sizeInBytes: 1125899906842624},
		{size: "2EB", sizeInBytes: 2 * 1125899906842624},
		{size: "15EB", sizeInBytes: 15 * 1125899906842624},

		{size: "a", shouldError: true},
		{size: "abcdefc", shouldError: true},
		{size: "1a", shouldError: true},
		{size: "1a", shouldError: true},
		{size: "128736kbkb", shouldError: true},
		{size: "128736kbk", shouldError: true},
		{size: "128736kbb", shouldError: true},
		{size: "one", shouldError: true},
		{size: "128.736", shouldError: true},
		{size: "128_736", shouldError: true},
	}

	for _, tc := range testCases {
		sizeInBytes, err := config.ToBytes(tc.size)

		if tc.shouldError {
			assert.Errorf(t, err, "size %s should return error", tc.size)
		} else {
			assert.Equalf(t, tc.sizeInBytes, sizeInBytes,
				"size %s should generate size in bytes %d", tc.size, tc.sizeInBytes)
			assert.NoErrorf(t, err, "size %s should return no error", tc.size)
		}
	}
}
