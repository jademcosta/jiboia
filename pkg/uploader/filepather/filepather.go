package filepather

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jademcosta/jiboia/pkg/domain"
)

type DateTimeProvider interface {
	Date() string
	Hour() string
}

// prefixVariety means how many different prefixes this instance will return
func New(dtProvider DateTimeProvider, prefixVariety int, fileExtension string) domain.FilePathProvider {
	if prefixVariety < 1 {
		panic("filepather: prefixVariety cannot be less than 1")
	}

	if prefixVariety == 1 {
		return &SinglePrefixFilePather{
			dtProvider:    dtProvider,
			randomPrefix:  uuid.New().String(),
			fileExtension: fileExtension,
		}
	} else {
		prefixes := make([]string, 0, prefixVariety)
		for i := 0; i < prefixVariety; i++ {
			prefixes = append(prefixes, uuid.New().String())
		}

		s := rand.NewSource(time.Now().Unix())
		r := rand.New(s)

		return &MultiplePrefixesFilePather{
			dtProvider:      dtProvider,
			randomPrefixes:  prefixes,
			prefixCount:     len(prefixes),
			randomGenerator: r,
			fileExtension:   fileExtension,
		}
	}

}

type SinglePrefixFilePather struct {
	dtProvider    DateTimeProvider
	randomPrefix  string
	fileExtension string
}

func (fp *SinglePrefixFilePather) Filename() *string {
	filename := uuid.New().String()

	if fp.fileExtension != "" {
		filename = filename + "." + fp.fileExtension
	}
	return &filename
}

// TODO: turn this into an []string so we have space to other file systems separators (other than "/")
func (fp *SinglePrefixFilePather) Prefix() *string {
	prefix := fmt.Sprintf("date=%s/hour=%s/%s/", fp.dtProvider.Date(), fp.dtProvider.Hour(), fp.randomPrefix)
	return &prefix
}

type MultiplePrefixesFilePather struct {
	dtProvider      DateTimeProvider
	randomPrefixes  []string
	prefixCount     int
	randomGenerator *rand.Rand
	fileExtension   string
}

func (fp *MultiplePrefixesFilePather) Filename() *string {
	filename := uuid.New().String()

	if fp.fileExtension != "" {
		filename = filename + "." + fp.fileExtension
	}
	return &filename
}

// TODO: turn this into an []string so we have space to other file systems separators (other than "/")
func (fp *MultiplePrefixesFilePather) Prefix() *string {
	pickedPrefix := fp.randomPrefixes[fp.randomGenerator.Intn(fp.prefixCount)]
	prefix := fmt.Sprintf("date=%s/hour=%s/%s/", fp.dtProvider.Date(), fp.dtProvider.Hour(), pickedPrefix)
	return &prefix
}
