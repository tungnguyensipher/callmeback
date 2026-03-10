package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func parseSimpleDuration(value string) (time.Duration, error) {
	raw := strings.TrimSpace(strings.ToLower(value))
	if raw == "" {
		return 0, fmt.Errorf("duration is empty")
	}

	endDigits := 0
	for endDigits < len(raw) && raw[endDigits] >= '0' && raw[endDigits] <= '9' {
		endDigits++
	}
	if endDigits == 0 {
		return 0, fmt.Errorf("duration must start with digits")
	}

	amount, err := strconv.Atoi(raw[:endDigits])
	if err != nil {
		return 0, fmt.Errorf("parse amount: %w", err)
	}
	if amount <= 0 {
		return 0, fmt.Errorf("duration must be greater than zero")
	}

	unit := strings.TrimSpace(raw[endDigits:])
	switch unit {
	case "", "s", "sec", "secs", "second", "seconds":
		return time.Duration(amount) * time.Second, nil
	case "m", "min", "mins", "minute", "minutes":
		return time.Duration(amount) * time.Minute, nil
	case "h", "hr", "hrs", "hour", "hours":
		return time.Duration(amount) * time.Hour, nil
	case "d", "day", "days":
		return time.Duration(amount) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported duration unit %q", unit)
	}
}
