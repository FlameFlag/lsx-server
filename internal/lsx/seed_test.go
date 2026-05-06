package lsx

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestWaybackSeedRowsUseRecoveredDetailFieldMap(t *testing.T) {
	row := waybackSeedRows[0]
	if row.GameGoal != 1 {
		t.Fatalf("seed GameGoal = %d, want 1 from goal label", row.GameGoal)
	}
	if row.Stands != 3 {
		t.Fatalf("seed Stands = %d, want d9 stands", row.Stands)
	}
	if row.CupsSold != -431520288 {
		t.Fatalf("seed CupsSold = %d, want d10 cups", row.CupsSold)
	}
	if row.DateScalar != "0" {
		t.Fatalf("seed DateScalar = %q, want 0 because Wayback d6 is total entries", row.DateScalar)
	}
	if got := row.Detail["d6"]; got != "78880" {
		t.Fatalf("seed detail d6 = %q, want total entries 78880", got)
	}
	if got := row.Detail["d14"]; got != "-395" {
		t.Fatalf("seed detail d14 = %q, want recovered percent field", got)
	}
}

func TestSeedDemoDataAndRequestEvents(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	inserted, err := srv.SeedDemoData()
	if err != nil {
		t.Fatal(err)
	}
	if inserted != len(waybackSeedRows) {
		t.Fatalf("SeedDemoData inserted %d rows, want %d", inserted, len(waybackSeedRows))
	}
	inserted, err = srv.SeedDemoData()
	if err != nil {
		t.Fatal(err)
	}
	if inserted != 0 {
		t.Fatalf("second SeedDemoData inserted %d rows, want 0", inserted)
	}

	req := httptest.NewRequest(http.MethodGet, "/lsx2.php", nil)
	rr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/lsx2.php status = %d, want 200", rr.Code)
	}

	events, err := srv.RecentEvents(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("RecentEvents returned %d rows, want 1", len(events))
	}
	if events[0].Kind != "leaderboard" || events[0].Path != "/lsx2.php" {
		t.Fatalf("RecentEvents returned %#v, want leaderboard /lsx2.php", events[0])
	}
}
