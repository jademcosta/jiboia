package datetimeprovider

import (
	"strconv"
	"time"
)

const TIME_FORMAT string = "2006-01-02"

type Provider struct{}

func New() *Provider {
	return &Provider{}
}

func (provider *Provider) Date() string {
	currentTime := time.Now()
	return currentTime.Format(TIME_FORMAT)
}

func (provider *Provider) Hour() string {
	currentTime := time.Now()
	return strconv.Itoa(currentTime.Hour())
}
