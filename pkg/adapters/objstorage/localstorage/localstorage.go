package localstorage

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/jademcosta/jiboia/pkg/domain"
	"gopkg.in/yaml.v2"
)

const TYPE string = "localstorage"

type Config struct {
	Path string `yaml:"path"`
}

type LocalStorage struct {
	path string
	log  *slog.Logger
}

func New(l *slog.Logger, c *Config) (*LocalStorage, error) {
	path, err := validateAndFormatPath(c.Path)
	if err != nil {
		return nil, fmt.Errorf("error creating localstorage: %w", err)
	}

	return &LocalStorage{path: path, log: l}, nil
}

func ParseConfig(confData []byte) (*Config, error) {
	conf := &Config{}

	err := yaml.Unmarshal(confData, conf)
	if err != nil {
		return conf, fmt.Errorf("error parsing localstorage config: %w", err)
	}

	return conf, nil
}

func (storage *LocalStorage) Upload(workU *domain.WorkUnit) (*domain.UploadResult, error) {

	directoryPath := filepath.Join(storage.path, workU.Prefix)
	_, err := os.Stat(directoryPath)

	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("error getting directory info: %w", err)
	}

	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(directoryPath, os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("error creating directory: %w", err)
		}
	}

	fullFilePath := filepath.Join(storage.path, workU.Prefix, workU.Filename)

	err = os.WriteFile(fullFilePath, workU.Data, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("error writing data into file: %w", err)
	}

	return &domain.UploadResult{
		Bucket:      "localstorage",
		Path:        fullFilePath,
		URL:         fullFilePath,
		SizeInBytes: len(workU.Data),
	}, nil
}

func (storage *LocalStorage) Type() string {
	return "localstorage"
}

func (storage *LocalStorage) Name() string {
	return "localstorage"
}

func validateAndFormatPath(path string) (string, error) {
	pathInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("the directory for the path doesn't exist: %w", err)
		}
		return "", fmt.Errorf("error on the provided path: %w", err)
	}

	if !pathInfo.IsDir() {
		return "", fmt.Errorf("provided path is not a directory")
	}

	formattedPath := strings.TrimSuffix(path, "/")
	return formattedPath, nil
}
