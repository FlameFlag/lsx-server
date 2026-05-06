package tui

import (
	"strings"
	"testing"
)

func TestLocalAccessURLUsesLocalhostForWildcardBind(t *testing.T) {
	tests := map[string]string{
		"[::]:80":        "http://localhost:80/",
		"0.0.0.0:8080":   "http://localhost:8080/",
		"127.0.0.1:8080": "http://127.0.0.1:8080/",
	}

	for bound, want := range tests {
		if got := localAccessURL(bound); got != want {
			t.Fatalf("localAccessURL(%q) = %q, want %q", bound, got, want)
		}
	}
}

func TestLocalAccessURLForPathAddsAdminEndpoint(t *testing.T) {
	tests := map[string]string{
		"[::]:80|/manage-secret":       "http://localhost:80/manage-secret",
		"127.0.0.1:8080|manage-secret": "http://127.0.0.1:8080/manage-secret",
	}

	for input, want := range tests {
		parts := strings.Split(input, "|")
		if got := localAccessURLForPath(parts[0], parts[1]); got != want {
			t.Fatalf("localAccessURLForPath(%q, %q) = %q, want %q", parts[0], parts[1], got, want)
		}
	}
}

func TestBindDescriptionExplainsScope(t *testing.T) {
	tests := map[string]string{
		"[::]:80":        "local + LAN, port 80",
		"0.0.0.0:8080":   "local + LAN, port 8080",
		"127.0.0.1:8080": "this machine only, port 8080",
	}

	for bound, want := range tests {
		if got := bindDescription(bound); got != want {
			t.Fatalf("bindDescription(%q) = %q, want %q", bound, got, want)
		}
	}
}

func TestTruncateUsesTerminalCells(t *testing.T) {
	if got, want := truncate("company-日本-dashboard", 12), "company-..."; got != want {
		t.Fatalf("truncate() = %q, want %q", got, want)
	}

	if got, want := truncate("éclair", 4), "é..."; got != want {
		t.Fatalf("truncate() = %q, want %q", got, want)
	}
}
