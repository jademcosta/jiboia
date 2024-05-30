package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const onlyBytesRegex = "^[0-9]+$"
const sizeWithUnitRegex string = "^[0-9]+(kb|mb|gb|tb|pb|eb|KB|MB|GB|PB|EB)$"

func ToBytes(sizeRep string) (int64, error) {
	if sizeRep == "" {
		return 0, nil
	}

	rex := regexp.MustCompile(onlyBytesRegex)
	numberExpressedInBytes := rex.MatchString(sizeRep)
	if numberExpressedInBytes {
		inBytes, err := strconv.ParseInt(sizeRep, 10, 64)
		if err != nil {
			return 0, err
		}

		return inBytes, nil
	}

	rex = regexp.MustCompile(sizeWithUnitRegex)

	matches := rex.FindStringSubmatch(sizeRep)
	noUnitFound := len(matches) <= 1
	if noUnitFound {
		return 0, fmt.Errorf("invalid data size unit: %s", sizeRep)
	}

	unit := matches[1]
	rawSize := strings.Replace(sizeRep, unit, "", 1)
	size, err := strconv.ParseInt(rawSize, 10, 64)
	if err != nil {
		return 0, err
	}

	exponential := 0
	switch unit {
	case "kb", "KB":
		exponential = 1
	case "mb", "MB":
		exponential = 2
	case "gb", "GB":
		exponential = 3
	case "pb", "PB":
		exponential = 4
	case "eb", "EB":
		exponential = 5
	}

	//Doing exponentiation with integers (instead of float64) to allow bigger numbers
	sizeInBytes := size
	for i := 0; i < exponential; i++ {
		sizeInBytes *= 1024
	}
	return sizeInBytes, nil
}
