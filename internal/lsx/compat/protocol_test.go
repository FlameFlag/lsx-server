package compat

import (
	"reflect"
	"strconv"
	"testing"
)

func TestProtocolMetadataContracts(t *testing.T) {
	wantSyncFields := []string{
		"game",
		"username",
		"password",
		"companyname",
		"ceoname",
		"gamemode",
		"gamegoal",
		"gamestartingdate",
		"lifespan",
		"stands",
		"cupssold",
		"cashassets",
		"stockassets",
		"standsassets",
		"upgradesassets",
		"retainedearnings",
		"revenues",
		"checksumclient",
	}
	if !reflect.DeepEqual(SyncFields, wantSyncFields) {
		t.Fatalf("SyncFields = %#v, want %#v", SyncFields, wantSyncFields)
	}

	wantDetailLabels := []string{
		"Company",
		"CEO",
		"Mode",
		"Goal",
		"Rank",
		"Total Entries",
		"Title",
		"Lifespan",
		"Stands",
		"Cups Sold",
		"Market Cap",
		"Revenues",
		"Retained Earnings",
		"Percent",
		"Cash",
		"Stock",
		"Stand Assets",
		"Upgrade Assets",
	}
	if len(DetailFieldSpecs) != len(wantDetailLabels) {
		t.Fatalf("DetailFieldSpecs length = %d, want %d", len(DetailFieldSpecs), len(wantDetailLabels))
	}
	for i, spec := range DetailFieldSpecs {
		if got, want := spec.Param, "d"+strconv.Itoa(i+1); got != want {
			t.Fatalf("DetailFieldSpecs[%d].Param = %q, want %q", i, got, want)
		}
		if got, want := spec.Label, wantDetailLabels[i]; got != want {
			t.Fatalf("DetailFieldSpecs[%d].Label = %q, want %q", i, got, want)
		}
	}
}
