package lsxvalue

import "testing"

func TestParseIntUsesRecoveredMoneyScale(t *testing.T) {
	tests := map[string]int64{
		"":              0,
		"1,234":         1234,
		"1,234.56":      123456,
		"-4,315,202.88": -431520288,
		"12.345":        1235,
		"not-a-number":  0,
	}

	for input, want := range tests {
		if got := ParseInt(input); got != want {
			t.Fatalf("ParseInt(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestFormatters(t *testing.T) {
	if got, want := FormatInt(-1234567), "-1,234,567"; got != want {
		t.Fatalf("FormatInt() = %q, want %q", got, want)
	}
	if got, want := FormatCents(-431520288), "-4,315,202.88"; got != want {
		t.Fatalf("FormatCents() = %q, want %q", got, want)
	}
}
