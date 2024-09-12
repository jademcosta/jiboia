package httpstorage_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jademcosta/jiboia/pkg/adapters/objstorage/httpstorage"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/stretchr/testify/assert"
)

var llog = logger.NewDummy()

func TestURLFormat(t *testing.T) {
	type testCase struct {
		url         string
		shouldError bool
	}

	testCases := []testCase{
		{"without-http-start.com/something", true},
		{"without-http-start", true},
		{"http://with-correct-start/%s%s", true},
		{"http://with-correct-start/%s%s/", true},
		{"http://with-correct-start/%s/%s/", true},
		{"http://with-correct-start%s", true},
		{"http://with-correct-start.com/something", false},
		{"http://with-correct-start.com/", false},
		{"http://with-correct-start.com", false},
		{"https://with-correct-start.com/", false},
		{"http://with-correct-start", false},
		{"http://with-correct-start/%s", false},
		{"http://with-correct-start/%s/", false},
		{"http://with-correct-start/something/%s/", false},
		{"http://with-correct-start/something/%s", false},
	}

	for _, tc := range testCases {
		_, err := httpstorage.NewHTTPStorage(llog, &httpstorage.Config{URL: tc.url})
		if tc.shouldError {
			assert.Errorf(t, err,
				"should return error when URL %s is used", tc.url)
		} else {
			assert.NoErrorf(t, err,
				"should NOT return error when URL %s is used", tc.url)
		}
	}
}

func TestURLFormatWhenUploading(t *testing.T) {

	uploadedData := make([][]byte, 0)
	paths := make([]string, 0)
	methods := make([]string, 0)
	externalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			panic("Body is nil when it shouldn't")
		}

		uploadedData = append(uploadedData, b)
		paths = append(paths, r.URL.Path)
		methods = append(methods, r.Method)

		w.WriteHeader(http.StatusOK)
	}))
	defer externalServer.Close()

	//server url is like http://127.0.0.1:57439
	sut, err := httpstorage.NewHTTPStorage(llog, &httpstorage.Config{
		URL: fmt.Sprintf("%s/something-to-upload", externalServer.URL)})
	assert.NoError(t, err, "should not error on storage creation")

	_, err = sut.Upload(&domain.WorkUnit{
		Filename: "some-filename",
		Prefix:   "some-prefix",
		Data:     []byte("some-data")})

	assert.Equalf(t, []byte("some-data"), uploadedData[0],
		"the data should be sent to the external server")
	assert.Equalf(t, "/something-to-upload", paths[0],
		"the path should NOT have any variation when the URL doesn't have %s")
	assert.Equalf(t, http.MethodPost, methods[0],
		"the method used to send should be POST")
	assert.NoError(t, err, "the upload should return no error")

	//server url is like http://127.0.0.1:57439
	sut, err = httpstorage.NewHTTPStorage(llog, &httpstorage.Config{
		URL: fmt.Sprintf("%s/something-to-upload/%s", externalServer.URL, "%s")})
	assert.NoError(t, err, "should not error on storage creation")

	_, err = sut.Upload(&domain.WorkUnit{
		Filename: "some-filename2",
		Prefix:   "some-prefix2",
		Data:     []byte("some-data2")})

	assert.Equalf(t, []byte("some-data2"), uploadedData[1],
		"the data should be sent to the external server")
	assert.Equalf(t, "/something-to-upload/some-prefix2/some-filename2", paths[1],
		"the path should have the prefix and filaname when URL have %s")
	assert.Equalf(t, http.MethodPost, methods[1],
		"the method used to send should be POST")
	assert.NoError(t, err, "the upload should return no error")
}

func TestUploadSuccess(t *testing.T) {

	externalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer externalServer.Close()

	//server url is like http://127.0.0.1:57439
	sut, err := httpstorage.NewHTTPStorage(llog, &httpstorage.Config{
		URL: fmt.Sprintf("%s/something-to-upload/%%s", externalServer.URL)})
	assert.NoError(t, err, "should not error on storage creation")

	uploadResult, err := sut.Upload(&domain.WorkUnit{
		Filename: "some-filename",
		Prefix:   "some-prefix",
		Data:     []byte("some-data")})

	assert.NoError(t, err, "the upload should return no error")
	assert.Equal(t, "some-prefix/some-filename",
		uploadResult.Path, "the upload result should contain the path of the data")
	assert.Equal(t, fmt.Sprintf("%s/something-to-upload/some-prefix/some-filename", externalServer.URL),
		uploadResult.URL, "the upload result should contain the full URL")
	assert.Equal(t, len("some-data"),
		uploadResult.SizeInBytes, "the upload result should contain the size in bytes")
}

func TestUploadErrors(t *testing.T) {

	type testCase struct {
		statusCode  int
		shouldError bool
	}

	testCases := []testCase{
		{200, false},
		{201, false},
		{202, false},
		{204, false},
		{300, true},
		{301, true},
		{302, true},
		{303, true},
		{304, true},
		{400, true},
		{401, true},
		{404, true},
		{500, true},
		{501, true},
		{502, true},
		{503, true},
		{504, true},
	}

	for _, tc := range testCases {
		externalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.statusCode)
		}))

		sut, err := httpstorage.NewHTTPStorage(llog, &httpstorage.Config{
			URL: fmt.Sprintf("%s/something-to-upload/%%s", externalServer.URL)})
		assert.NoError(t, err, "should not error on storage creation")

		_, err = sut.Upload(&domain.WorkUnit{
			Filename: "some-filename",
			Prefix:   "some-prefix",
			Data:     []byte("some-data")})

		if tc.shouldError {
			assert.Errorf(t, err,
				"upload should return error when status code from external server is %d", tc.statusCode)
		} else {
			assert.NoErrorf(t, err,
				"upload should NOT return error when status code from external server is %d", tc.statusCode)
		}

		externalServer.Close()
	}
}
