package filepather

import (
	"fmt"

	"github.com/google/uuid"
)

type DateTimeProvider interface {
	Date() string
	Hour() string
}

type FilePather struct {
	dtProvider   DateTimeProvider
	randomPrefix string
}

func New(dtProvider DateTimeProvider) *FilePather {
	return &FilePather{
		dtProvider:   dtProvider,
		randomPrefix: uuid.New().String(),
	}
}

func (fp *FilePather) Filename() *string {
	filename := uuid.New().String()
	return &filename
}

//TODO: turn this into an []string so we have space to other file systems separators (other than "/")
func (fp *FilePather) Prefix() *string {
	prefix := fmt.Sprintf("%s/%s/%s/", fp.dtProvider.Date(), fp.dtProvider.Hour(), fp.randomPrefix)
	return &prefix
}
