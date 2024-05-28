package config

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const ONLY_BYTES_REGEX string = "^[0-9]+$"
const SIZE_WITH_UNIT_REGEX string = "^[0-9]+(kb|mb|gb|tb|pb|eb|KB|MB|GB|PB|EB)$"

type ApiConfig struct {
	Port             int    `yaml:"port"`
	PayloadSizeLimit string `yaml:"payload_size_limit"`
}

func (apiConf ApiConfig) fillDefaults() ApiConfig {
	return apiConf
}

func (apiConf ApiConfig) validate() error {

	err := apiConf.validateSizeLimit()
	if err != nil {
		return err
	}
	return nil
}

func (apiconf ApiConfig) PayloadSizeLimitInBytes() (int, error) {
	if apiconf.PayloadSizeLimit == "" {
		return 0, nil
	}

	rex, err := regexp.Compile(ONLY_BYTES_REGEX)
	if err != nil {
		return 0, err
	}

	numberExpressedInBytes := rex.MatchString(apiconf.PayloadSizeLimit)
	if numberExpressedInBytes {
		inBytes, err := strconv.Atoi(apiconf.PayloadSizeLimit)
		if err != nil {
			return 0, err
		}

		return inBytes, nil
	}

	rex, err = regexp.Compile(SIZE_WITH_UNIT_REGEX)
	if err != nil {
		return 0, err
	}

	matches := rex.FindStringSubmatch(apiconf.PayloadSizeLimit)
	noUnitFound := len(matches) <= 1
	if noUnitFound {
		return 0, fmt.Errorf("invalid data size unit: %s", apiconf.PayloadSizeLimit)
	}

	unit := matches[1]
	rawSize := strings.Replace(apiconf.PayloadSizeLimit, unit, "", 1)
	size, err := strconv.Atoi(rawSize)
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

	//Doing exponentiation with integers to allow bigger numbers
	multiplier := 1
	for i := 0; i < exponential; i++ {
		multiplier *= 1024
	}
	sizeInBytes := size * multiplier
	return sizeInBytes, nil
}

func (apiconf ApiConfig) validateSizeLimit() error {

	if apiconf.PayloadSizeLimit == "" {
		return nil
	}

	bytesRex, err := regexp.Compile(ONLY_BYTES_REGEX)
	if err != nil {
		return err
	}

	unitiesRex, err := regexp.Compile(SIZE_WITH_UNIT_REGEX)
	if err != nil {
		return err
	}

	numberExpressedInBytes := bytesRex.MatchString(apiconf.PayloadSizeLimit)
	numberExpressedWithUnit := unitiesRex.MatchString(apiconf.PayloadSizeLimit)

	if !numberExpressedInBytes && !numberExpressedWithUnit {
		return errors.New("invalid format on api payload size limit")
	}

	return nil
}
