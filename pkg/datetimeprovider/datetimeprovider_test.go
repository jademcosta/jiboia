package datetimeprovider_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/datetimeprovider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// There's a chance that this test will fail if the test is run at the exact second that the
// minute changes. The same might happen with the hour.
func TestHourAndMinuteAreCurrentHourAndMinute(t *testing.T) {
	expected := time.Now()
	expectedHour := expected.Hour()
	expectedMinute := expected.Minute()

	sut := datetimeprovider.New()
	hour, minute := sut.HourAndMinute()

	hourResult, err := strconv.Atoi(hour)
	require.NoError(t, err, "should return an integer as hour string")

	minuteResult, err := strconv.Atoi(minute)
	require.NoError(t, err, "should return an integer as minute string")

	assert.GreaterOrEqual(t, hourResult, 0, "should always be greater or equal to 0")
	assert.LessOrEqual(t, hourResult, 23, "should always be smaller or equal to 23")

	assert.Equal(t, expectedHour, hourResult, "hour should be the current hour")
	assert.Equal(t, expectedMinute, minuteResult, "minute should be the current minute")
}

func TestDateIsCurrentDate(t *testing.T) {
	sut := datetimeprovider.New()

	result := sut.Date()

	assert.Regexp(t, "[0-9]{4}-([10]{1}[0-9]{1})-([0123]{1}[0-9]{1})", result, "should return a date in the format YYYY-MM-DD")
	assert.Equal(t, time.Now().Format(time.DateOnly), result, "should be the current day")
	assert.Equal(t, "2006-01-02", time.DateOnly, "should obey the specific time format of YYY-MM-DD that Go requires")
}
