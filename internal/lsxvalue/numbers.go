package lsxvalue

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

func ParseInt(value string) int64 {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	if value == "" {
		return 0
	}
	if strings.Contains(value, ".") {
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0
		}
		return int64(math.Round(f * 100))
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func ParseI32(value string) (int32, bool) {
	if strings.TrimSpace(value) == "" {
		return 0, false
	}
	return int32(ParseInt(value)), true
}

func ParsePositive(value string, fallback int64) int64 {
	n := ParseInt(value)
	if n <= 0 {
		return fallback
	}
	return n
}

func FormatCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	whole := cents / 100
	frac := cents % 100
	return fmt.Sprintf("%s%s.%02d", sign, FormatInt(whole), frac)
}

func FormatInt(n int64) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var parts []string
	for len(s) > 3 {
		parts = append(parts, s[len(s)-3:])
		s = s[:len(s)-3]
	}
	parts = append(parts, s)
	slices.Reverse(parts)
	return sign + strings.Join(parts, ",")
}
