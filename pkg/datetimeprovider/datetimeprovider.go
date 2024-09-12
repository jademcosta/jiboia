package datetimeprovider

import (
	"strconv"
	"time"
)

type Provider struct{}

func New() *Provider {
	return &Provider{}
}

func (provider *Provider) Date() string {
	currentTime := time.Now()
	return currentTime.Format(time.DateOnly)
}

func (provider *Provider) Hour() string {
	currentTime := time.Now()
	return strconv.Itoa(currentTime.Hour())
}
