package localstorage_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/adapters/objstorage/localstorage"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/stretchr/testify/assert"
)

const configYaml = `
path: /tmp/
`

var llog = logger.NewDummy()

func TestParseConfig(t *testing.T) {
	localStorageConfig, err := localstorage.ParseConfig([]byte(configYaml))
	assert.NoError(t, err, "should not return error when parsing localstorage config")

	assert.Equal(t, "/tmp/", localStorageConfig.Path, "path doesn't match")
}

func TestNewErrorsWhenPathDoesNotExist(t *testing.T) {
	conf := &localstorage.Config{Path: "/non_existant_dir_1373a98298"}
	_, err := localstorage.New(llog, conf)
	assert.Error(t, err, "returns error when dir doesn't exist")
}

func TestNewErrorsWhenPathIsNotADirectory(t *testing.T) {
	randomNumber := strconv.Itoa(int(time.Now().Unix()))
	filePath := "/tmp/" + randomNumber
	err := os.WriteFile(filePath, []byte("content!"), os.ModePerm)
	assert.NoError(t, err, "writing file should not err")

	conf := &localstorage.Config{Path: filePath}
	_, err = localstorage.New(llog, conf)
	assert.Error(t, err, "returns error when path is not a directory")
}

func TestUploadFailsIfDirectoryPathHasAFileInIt(t *testing.T) {
	randomNumber := strconv.Itoa(int(time.Now().Unix()))

	conf := &localstorage.Config{Path: "/tmp"}
	sut, err := localstorage.New(llog, conf)
	assert.NoError(t, err, "should not return an error")

	err = os.WriteFile(filepath.Join("/tmp", randomNumber), []byte("content!"), os.ModePerm)
	assert.NoError(t, err, "should not error on file writing")

	work := &domain.WorkUnit{
		Filename: "some_filename_here",
		Prefix:   fmt.Sprintf("%s/here/now", randomNumber),
		Data:     []byte("something"),
	}

	_, err = sut.Upload(work)
	assert.Error(t, err, "returns an error when there's a file in the path that should be a path of directories")
}

func TestUploadPutsTheContentOnFile(t *testing.T) {
	fixedPath := "/tmp"
	conf := &localstorage.Config{Path: fixedPath}
	sut, err := localstorage.New(llog, conf)

	assert.NoError(t, err, "shouldn't return an error")

	fileContent := "content of the file!"
	work := &domain.WorkUnit{
		Filename: "some_filename_here",
		Prefix:   "prefix/here/now",
		Data:     []byte(fileContent),
	}

	uploadResult, err := sut.Upload(work)
	assert.NoError(t, err, "shouldn't return an error")

	expectedFullPath := filepath.Join(fixedPath, work.Prefix, work.Filename)

	assert.Equal(t, "", uploadResult.Region, "region should be empty")
	assert.Equal(t, "localstorage", uploadResult.Bucket, "bucket var should have the fixed value of 'localstorage'")
	assert.Equal(t, len(fileContent), uploadResult.SizeInBytes, "size should be the content size")
	assert.Equal(t, expectedFullPath,
		uploadResult.URL, "url should be the full path")
	assert.Equal(t, uploadResult.Path, uploadResult.URL, "path should be equal to URL")

	content, err := os.ReadFile(expectedFullPath)
	assert.NoError(t, err, "shouldn't return an error")

	assert.Equal(t, fileContent, string(content), "the file content should be the one we sent to the storage")
}
