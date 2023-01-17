package filepather_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/jademcosta/jiboia/pkg/uploaders/filepather"
	"github.com/stretchr/testify/assert"
)

type mockDateTimeProvider struct {
	date string
	hour string
}

func (mock *mockDateTimeProvider) Date() string {
	return mock.date
}

func (mock *mockDateTimeProvider) Hour() string {
	return mock.hour
}

func TestRandomFilename(t *testing.T) {

	sut := filepather.New(&mockDateTimeProvider{})

	results := make([]string, 0, 10)

	for i := 0; i < 10; i++ {
		results = append(results, *sut.Filename())
	}

	for index, current := range results {
		for insideIndex, testAgainst := range results {
			if index != insideIndex {
				assert.NotEqual(t, current, testAgainst, "should not generate the same path string")
			}
		}
	}

	for _, current := range results {
		assert.Equal(t, 36, len(current), "should always generate the same length of strings")
	}

	// String format is f1f9ef42-324b-40c2-bb1e-eddadfeef330
	for _, current := range results {
		assert.Regexp(
			t,
			regexp.MustCompile("[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}"),
			current,
			"should return a string with a UUID format",
		)
	}
}

func TestPrefixContainsDateAndHour(t *testing.T) {

	sut := filepather.New(&mockDateTimeProvider{date: "2022-02-20", hour: "23"})

	prefix := *sut.Prefix()

	// Last chunck format is f1f9ef42-324b-40c2-bb1e-eddadfeef330
	assert.Regexp(t, regexp.MustCompile("date=2022-02-20/hour=23/[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}"),
		prefix, "prefix should contain datime information")
}

func TestPrefixLastChunkIsFixed(t *testing.T) {

	sut := filepather.New(&mockDateTimeProvider{date: "2022-02-20", hour: "23"})

	prefixes := make([]string, 10)

	for i := 0; i < 10; i++ {
		prefixes[i] = *sut.Prefix()
	}

	splitPrefix := strings.Split(*sut.Prefix(), "/")
	lastChunk := splitPrefix[len(splitPrefix)-2] // -2 because it ends with "/"
	// Last chunck format is f1f9ef42-324b-40c2-bb1e-eddadfeef330

	for _, prefix := range prefixes {
		assert.Equal(t, fmt.Sprintf("date=2022-02-20/hour=23/%s/", lastChunk),
			prefix, "prefix should contain always the same random UUID as last chunk")
	}
}
