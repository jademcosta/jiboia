package datetimeprovider_test

import (
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/datetimeprovider"
	"github.com/stretchr/testify/assert"
)

func TestHourIsCurrentHour(t *testing.T) {
	sut := datetimeprovider.New()

	result, err := strconv.Atoi(sut.Hour())
	assert.Nil(t, err, "should return an integer as string")

	assert.GreaterOrEqual(t, result, 0, "should always be greater or equal to 0")
	assert.LessOrEqual(t, result, 23, "should always be smaller or equal to 23")

	expected := time.Now().Hour()

	if expected == 0 { // There's a chance the resul;t is at the 23 hour
		assert.Condition(t, func() bool { return result == 0 || result == 23 }, "hour should be the current hour")
	} else {
		assert.Equal(t, expected, result, "hour should be the current hour")
	}
}

func TestDateIsCurrentDate(t *testing.T) {
	sut := datetimeprovider.New()

	result := sut.Date()

	assert.Regexp(t, regexp.MustCompile("[0-9]{4}-([10]{1}[0-9]{1})-([0123]{1}[0-9]{1})"), result, "should return a date in the format YYYY-MM-DD")
	assert.Equal(t, time.Now().Format(time.DateOnly), result, "should be the current day")
	assert.Equal(t, "2006-01-02", time.DateOnly, "should obey the specific time format of YYY-MM-DD that Go requires")
}
