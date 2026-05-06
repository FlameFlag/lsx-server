package tui

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"

	"lt2_reverse/lsx_server_go/internal/lsx"
)

type eventEntry struct {
	row        table.Row
	historical bool
	detail     string
	status     int
	kind       string
	message    string
	path       string
	when       time.Time
	repeats    int
	samples    []string
}

func startupEntry(bound string) eventEntry {
	now := time.Now()
	message := "ready for game requests at " + localAccessURL(bound)
	return eventEntry{
		row: table.Row{
			now.Format(timeFormat),
			"startup",
			"-",
			"-",
			message,
		},
		detail:  message,
		status:  http.StatusOK,
		kind:    "startup",
		message: message,
		path:    "-",
		when:    now,
		repeats: 1,
	}
}

func serverErrorEntry(err error) eventEntry {
	if err == nil {
		err = errors.New("unknown server error")
	}
	now := time.Now()
	return eventEntry{
		row: table.Row{
			now.Format(timeFormat),
			"server_error",
			"-",
			"-",
			err.Error(),
		},
		detail:  err.Error(),
		status:  http.StatusInternalServerError,
		kind:    "server_error",
		message: err.Error(),
		when:    now,
		repeats: 1,
	}
}

func entryFromEvent(ev lsx.Event, historical bool) eventEntry {
	return eventEntry{
		row: table.Row{
			ev.Time.Format(timeFormat),
			ev.Kind,
			ev.RemoteAddr,
			truncate(displayPath(ev), maxPathPreviewWidth),
			statusPrefix(ev) + ev.Message,
		},
		historical: historical,
		status:     ev.Status,
		kind:       ev.Kind,
		message:    ev.Message,
		path:       ev.Path,
		detail:     ev.Message,
		when:       ev.Time,
		repeats:    1,
		samples:    activitySamples(ev.Kind, ev.Path),
	}
}

func rowsFromEntries(entries []eventEntry) []table.Row {
	rows := make([]table.Row, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, displayRow(entry.row))
	}
	return rows
}

func displayRow(row table.Row) table.Row {
	if len(row) < rawColumnCount {
		return row
	}
	return table.Row{
		row[0],
		row[1],
		columnSeparator,
		row[2],
		columnSeparator,
		row[3],
		columnSeparator,
		row[4],
	}
}

func coalesceEntries(entries []eventEntry) []eventEntry {
	out := make([]eventEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.repeats == 0 {
			entry.repeats = 1
		}
		if len(out) == 0 || coalesceKey(out[len(out)-1]) != coalesceKey(entry) {
			out = append(out, entry)
			continue
		}
		last := &out[len(out)-1]
		last.repeats += entry.repeats
		last.when = entry.when
		last.samples = appendSamples(last.samples, entry.samples, maxSampleCount)
		last.row = coalescedRow(*last)
	}
	return out
}

func coalescedRow(entry eventEntry) table.Row {
	row := append(table.Row(nil), entry.row...)
	if entry.repeats <= 1 {
		return row
	}
	row[0] = entry.when.Format(timeFormat)
	row[1] = fmt.Sprintf("%s x%d", entry.kind, entry.repeats)
	row[3] = routeOnly(entry.path)
	return row
}

func coalesceKey(entry eventEntry) string {
	return strings.Join([]string{
		entry.kind,
		entry.row[2],
		routeOnly(entry.path),
		entry.message,
		strconv.Itoa(entry.status),
	}, "\x00")
}

func appendSamples(existing []string, next []string, limit int) []string {
	out := slices.Clone(existing)
	for _, sample := range next {
		sample = strings.TrimSpace(sample)
		if sample == "" || slices.Contains(out, sample) {
			continue
		}
		if len(out) >= limit {
			if out[len(out)-1] != overflowMarker {
				out[len(out)-1] = overflowMarker
			}
			continue
		}
		out = append(out, sample)
	}
	return out
}
