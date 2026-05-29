package lsx

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestLeaderboardWaybackControlsAndDetailRoutes(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	submitPaths := []string{
		"/syncgame.php?game=lemonade2&username=u1&password=p&companyname=Overall&ceoname=Boss&gamemode=1&gamegoal=1&gamestartingdate=1&lifespan=5&stands=1&cupssold=10&cashassets=5000&stockassets=0&standsassets=0&upgradesassets=0&retainedearnings=0&revenues=10000&checksumclient=0",
		"/syncgame.php?game=lemonade2&username=u2&password=p&companyname=GoalTwo&ceoname=Boss&gamemode=1&gamegoal=2&gamestartingdate=1&lifespan=8&stands=1&cupssold=10&cashassets=2500&stockassets=0&standsassets=0&upgradesassets=0&retainedearnings=0&revenues=10000&checksumclient=0",
		"/syncgame.php?game=lemonade2&username=u3&password=p&companyname=Challenge&ceoname=Boss&gamemode=2&gamegoal=1&gamestartingdate=1&lifespan=3&stands=1&cupssold=10&cashassets=9000&stockassets=0&standsassets=0&upgradesassets=0&retainedearnings=0&revenues=10000&checksumclient=0",
	}
	for _, path := range submitPaths {
		rr := getRoute(t, srv, path)
		if rr.Code != http.StatusOK || rr.Body.String() != "SUCCESS\n" {
			t.Fatalf("%s returned status=%d body=%q, want 200 SUCCESS", path, rr.Code, rr.Body.String())
		}
	}

	rr := getRoute(t, srv, "/lsx2.php?pagenum=1&sort=3&gamemode=1&gamegoal=2&ranktype=0&username=0")
	if rr.Code != http.StatusOK {
		t.Fatalf("leaderboard controls status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "GoalTwo") || strings.Contains(body, "Overall") || strings.Contains(body, "Challenge") {
		t.Fatalf("leaderboard filter body did not match gamemode/gamegoal controls")
	}
	if !strings.Contains(body, "ranktype=0") || !strings.Contains(body, "gamemode=1") || !strings.Contains(body, "gamegoal=2") {
		t.Fatalf("leaderboard links did not preserve Wayback-observed controls")
	}

	rr = getRoute(t, srv, "/lsx2.php")
	body = rr.Body.String()
	if !strings.Contains(body, "ranktype=0") || !strings.Contains(body, "sort=4") {
		t.Fatalf("default leaderboard links did not include normalized Wayback controls")
	}
	if !strings.Contains(body, "d6=3") || !strings.Contains(body, "d9=1") || !strings.Contains(body, "d10=10") {
		t.Fatalf("default leaderboard detail links did not include recovered d6/d9/d10 mapping")
	}

	rr = getRoute(t, srv, "/lsx2_detail.php?d1=GoalTwo&d2=Boss&d5=1&d18=25.00")
	if rr.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("detail content-type = %q, want text/html", got)
	}
	if detail := rr.Body.String(); !strings.Contains(detail, "GoalTwo") || !strings.Contains(detail, "Upgrade Assets") {
		t.Fatalf("detail body did not include expected d-field rendering")
	}

	rr = getRoute(t, srv, "/lsx2_detail_blank.php")
	if rr.Code != http.StatusOK {
		t.Fatalf("blank detail status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "#DEFFA5") {
		t.Fatalf("blank detail body did not include recovered background color")
	}
}

func TestEmptyDatabaseDoesNotShowWaybackRows(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	rr := getRoute(t, srv, "/lsx2.php")
	if rr.Code != http.StatusOK {
		t.Fatalf("empty leaderboard status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, "DarKMon") || strings.Contains(body, "LEMONY GOODNESS") || strings.Contains(body, "Wayback") {
		t.Fatalf("empty database leaderboard rendered recovered seed rows")
	}
	if !strings.Contains(body, "No LSX submissions match this view.") {
		t.Fatalf("empty database leaderboard did not render empty state")
	}
}
