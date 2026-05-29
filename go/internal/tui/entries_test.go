package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/table"
)

func TestCoalesceEntriesCollapsesConsecutiveMatchingRequests(t *testing.T) {
	base := time.Date(2026, 5, 5, 1, 0, 0, 0, time.UTC)
	entries := []eventEntry{
		testEventEntry(base, "detail", "127.0.0.1", "/lsx2_detail.php?d1=LEMONY", "rendered detail panel"),
		testEventEntry(base.Add(time.Second), "detail", "127.0.0.1", "/lsx2_detail.php?d1=LEMONY", "rendered detail panel"),
		testEventEntry(base.Add(2*time.Second), "detail", "127.0.0.1", "/lsx2_detail.php?d1=Niumia", "rendered detail panel"),
		testEventEntry(base.Add(3*time.Second), "detail", "127.0.0.1", "/lsx2_detail.php?d1=Niumia", "rendered detail panel"),
	}

	got := coalesceEntries(entries)
	if len(got) != 1 {
		t.Fatalf("coalesceEntries length = %d, want 1", len(got))
	}
	if got[0].repeats != 4 || got[0].row[1] != "detail x4" {
		t.Fatalf("group = repeats %d kind %q, want repeats 4 kind detail x4", got[0].repeats, got[0].row[1])
	}
	if got[0].row[3] != "/lsx2_detail.php" {
		t.Fatalf("group path = %q, want route only", got[0].row[3])
	}
	if len(got[0].samples) != 2 || got[0].samples[0] != "LEMONY" || got[0].samples[1] != "Niumia" {
		t.Fatalf("group samples = %#v, want LEMONY and Niumia", got[0].samples)
	}
}

func TestCoalesceEntriesDoesNotCollapseSeparatedRequests(t *testing.T) {
	base := time.Date(2026, 5, 5, 1, 0, 0, 0, time.UTC)
	entries := []eventEntry{
		testEventEntry(base, "project", "127.0.0.1", "/", "rendered project page"),
		testEventEntry(base.Add(time.Second), "leaderboard", "127.0.0.1", "/board", "rendered leaderboard"),
		testEventEntry(base.Add(2*time.Second), "project", "127.0.0.1", "/", "rendered project page"),
	}

	got := coalesceEntries(entries)
	if len(got) != 3 {
		t.Fatalf("coalesceEntries length = %d, want 3", len(got))
	}
}

func TestRowsFromEntriesKeepsTableCellsRaw(t *testing.T) {
	base := time.Date(2026, 5, 5, 1, 0, 0, 0, time.UTC)
	rows := rowsFromEntries([]eventEntry{
		testEventEntry(base, "detail", "127.0.0.1", "/lsx2_detail.php?d1=LEMONY", "rendered detail panel"),
	})
	if got := strings.Join(rows[0], ""); strings.Contains(got, "\x1b[") {
		t.Fatalf("row contains ANSI escapes: %q", got)
	}
}

func testEventEntry(when time.Time, kind, remote, path, message string) eventEntry {
	return eventEntry{
		row: table.Row{
			when.Format(timeFormat),
			kind,
			remote,
			path,
			"200 " + message,
		},
		status:  200,
		kind:    kind,
		message: message,
		path:    path,
		when:    when,
		repeats: 1,
		samples: activitySamples(kind, path),
	}
}
