package s3

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrefixMergerDoesNotDuplicateSeparators(t *testing.T) {
	type testCase struct {
		fixedPrefix   string
		dynamicPrefix string
		key           string
		expected      string
	}

	testCases := []testCase{
		{"", "", "something", "something"},
		{"", "", "/something/", "something"},
		{"abc", "", "something", "abc/something"},
		{"/abc/", "", "/something", "abc/something"},
		{"abc/", "", "something", "abc/something"},
		{"", "abc", "something", "abc/something"},
		{"", "abc/", "something", "abc/something"},
		{"/abc/", "", "something", "abc/something"},
		{"abc/", "def/", "something", "abc/def/something"},
		{"abc", "def", "something", "abc/def/something"},
		{"abc/", "def", "something", "abc/def/something"},
		{"abc/", "def", "/something", "abc/def/something"},
		{"abc", "def/", "something", "abc/def/something"},
		{"/abc/", "/def/", "/something", "abc/def/something"},
		{"/abc/", "/def/", "/something/", "abc/def/something"},
	}

	for _, testCase := range testCases {
		result := mergeParts(testCase.fixedPrefix, testCase.dynamicPrefix, testCase.key)
		assert.Equal(t, testCase.expected, result, "merged string doesn't match")
	}
}
