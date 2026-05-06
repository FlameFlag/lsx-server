package discord

import (
	"strings"
	"testing"
	"time"

	"lt2_reverse/lsx_server_go/internal/lsx"
)

func TestRenderSyncLayout(t *testing.T) {
	ev := lsx.Event{
		Kind:    "sync",
		Status:  200,
		Path:    "/sync?companyname=HQ&ceoname=Bud&username=player&cashassets=1200&stockassets=300&standsassets=500&upgradesassets=100&revenues=4567&retainedearnings=1234&stands=4&cupssold=9876&gamemode=Classic&gamegoal=Rich&lifespan=5y",
		Message: "checksum ok",
		Time:    time.Unix(1_700_000_000, 0),
	}

	rendered := render(ev)

	if rendered.title != "Score uploaded · HQ" {
		t.Fatalf("title = %q", rendered.title)
	}
	if !strings.Contains(rendered.description, "**HQ** led by **Bud**") {
		t.Fatalf("description missing company/CEO: %q", rendered.description)
	}
	if !strings.Contains(rendered.description, "**Checksum:** ok") {
		t.Fatalf("description missing checksum: %q", rendered.description)
	}
	if got, want := fieldNames(rendered.fields), "Run,Financials,Operations,Server"; got != want {
		t.Fatalf("field names = %q, want %q", got, want)
	}
}

func TestRenderUnknownEventFallsBackToServerField(t *testing.T) {
	rendered := render(lsx.Event{
		Kind:    "custom",
		Status:  204,
		Path:    "/custom?x=1",
		Message: "kept as-is",
	})

	if rendered.title != "LSX custom" {
		t.Fatalf("title = %q", rendered.title)
	}
	if rendered.description != "kept as-is" {
		t.Fatalf("description = %q", rendered.description)
	}
	if got, want := fieldNames(rendered.fields), "Server"; got != want {
		t.Fatalf("field names = %q, want %q", got, want)
	}
}

func fieldNames(fields []field) string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}
	return strings.Join(names, ",")
}
