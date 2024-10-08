package filepather_test

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/uploader/filepather"
	"github.com/stretchr/testify/assert"
)

const noFileExtension = ""

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
	sutSinglePrefix := filepather.New(&mockDateTimeProvider{}, 1, noFileExtension)
	sutMultiPrefix := filepather.New(&mockDateTimeProvider{}, randomIntForPrefixCount(), noFileExtension)

	suts := []domain.FilePathProvider{sutSinglePrefix, sutMultiPrefix}

	for _, sut := range suts {
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
}

func TestContainsFileTypeIfProvided(t *testing.T) {

	testCases := []struct{ fileType string }{
		{fileType: "abacaxi"},
		{fileType: "gzip"},
		{fileType: "snappy"},
		{fileType: "arj"},
	}

	for _, tc := range testCases {
		sutSinglePrefix := filepather.New(&mockDateTimeProvider{}, 1, tc.fileType)
		sutMultiPrefix := filepather.New(&mockDateTimeProvider{}, randomIntForPrefixCount(), tc.fileType)

		results := make([]string, 0, 10)
		for i := 0; i < 10; i++ {
			results = append(results, *sutSinglePrefix.Filename())
		}

		for _, filename := range results {
			assert.Truef(t, strings.HasSuffix(filename, fmt.Sprintf(".%s", tc.fileType)),
				"for singleFilePrefix, filename (%s) should end with provided file type (.%s)",
				filename, tc.fileType)
		}

		results = make([]string, 0, 10)
		for i := 0; i < 10; i++ {
			results = append(results, *sutMultiPrefix.Filename())
		}

		for _, filename := range results {
			assert.Truef(t, strings.HasSuffix(filename, fmt.Sprintf(".%s", tc.fileType)),
				"for singleFilePrefix, filename (%s) should end with provided file type (.%s)",
				filename, tc.fileType)
		}
	}
}

func TestPrefixContainsDateAndHour(t *testing.T) {

	sutSinglePrefix := filepather.New(&mockDateTimeProvider{date: "2022-02-20", hour: "23"}, 1, noFileExtension)
	sutMultiPrefix := filepather.New(&mockDateTimeProvider{date: "2022-02-20", hour: "23"},
		randomIntForPrefixCount(), noFileExtension)

	suts := []domain.FilePathProvider{sutSinglePrefix, sutMultiPrefix}

	for _, sut := range suts {
		pathPrefix := *sut.Prefix()

		// Last chunck format is f1f9ef42-324b-40c2-bb1e-eddadfeef330
		assert.Regexp(t, regexp.MustCompile("date=2022-02-20/hour=23/[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}"),
			pathPrefix, "prefix should contain datime information")
	}
}

func TestSingleFilePatherPrefixLastChunkIsFixed(t *testing.T) {

	sut := filepather.New(&mockDateTimeProvider{date: "2022-02-20", hour: "23"}, 1, noFileExtension)

	pathPrefixes := make([]string, 10)

	for i := 0; i < 10; i++ {
		pathPrefixes[i] = *sut.Prefix()
	}

	splitPrefix := strings.Split(*sut.Prefix(), "/")
	lastChunk := splitPrefix[len(splitPrefix)-2] // -2 because it ends with "/"
	// Last chunck format is f1f9ef42-324b-40c2-bb1e-eddadfeef330

	for _, prefix := range pathPrefixes {
		assert.Equal(t, fmt.Sprintf("date=2022-02-20/hour=23/%s/", lastChunk),
			prefix, "prefix should contain always the same random UUID as last chunk")
	}
}

func TestMultiPrefixesAreGenerated(t *testing.T) {
	prefixCount := randomIntForPrefixCount()
	sutMultiPrefix :=
		filepather.New(&mockDateTimeProvider{date: "2022-02-20", hour: "23"}, prefixCount, noFileExtension)

	prefixPossibilities := make(map[string]bool)

	var safetyCounter int
	for {
		splitPrefix := strings.Split(*sutMultiPrefix.Prefix(), "/")
		lastChunk := splitPrefix[len(splitPrefix)-2] // -2 because it ends with "/"
		// Last chunck format is f1f9ef42-324b-40c2-bb1e-eddadfeef330

		prefixPossibilities[lastChunk] = true

		if len(prefixPossibilities) == prefixCount {
			break
		}

		if safetyCounter > 10000*prefixCount { // This still can be a false negative, but :shrug:
			assert.Fail(
				t,
				"there seems to be too few prefixes generated by the filepatherMultiplePrefix."+
					"Desired variety: %d, variety found: %d",
				prefixCount,
				len(prefixPossibilities),
			)
			break
		}
	}

	for i := 0; i < 1000; i++ {
		splitPrefix := strings.Split(*sutMultiPrefix.Prefix(), "/")
		lastChunk := splitPrefix[len(splitPrefix)-2] // -2 because it ends with "/"
		// Last chunck format is f1f9ef42-324b-40c2-bb1e-eddadfeef330

		_, present := prefixPossibilities[lastChunk]
		if !present {
			assert.Fail(t, "the following prefix should have been seen before, but it wasn't."+
				"this might mean it is generating more prefixes than desired. "+
				"generated prefix: %d, prefixes seen before: %v", lastChunk, prefixPossibilities)
		}
	}
}

func TestPanicsIfPrefixCountLessThanOne(t *testing.T) {
	assert.Panics(t, func() { filepather.New(&mockDateTimeProvider{}, 0, noFileExtension) },
		"should panic if prefixCount < 1")
}

func randomIntForPrefixCount() int {
	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)

	minimum := 2
	maximum := 12
	return r.Intn(maximum-minimum) + minimum
}
