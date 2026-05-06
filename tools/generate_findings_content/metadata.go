package main

import (
	"slices"
	"strings"
)

func sectionWordCount(sec section) int {
	return wordCount(
		sec.Title,
		sec.Summary,
		sec.Takeaway,
		strings.Join(sec.Body, " "),
		strings.Join(sec.Findings, " "),
	)
}

func paddedRow(row []string, length int) []string {
	if len(row) >= length {
		return row
	}
	next := slices.Clone(row)
	for len(next) < length {
		next = append(next, "")
	}
	return next
}

func wordCount(parts ...string) int {
	count := 0
	for _, part := range parts {
		count += len(strings.Fields(part))
	}
	return count
}

func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}
