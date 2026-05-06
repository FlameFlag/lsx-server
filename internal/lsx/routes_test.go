package lsx

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGameCriticalRoutes(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	tests := []struct {
		path string
		want string
	}{
		{"/createaccount.php?username=u&password=p", "ACCEPT\n"},
		{"/syncgame.php?game=lemonade2&username=u&password=p&companyname=Co&ceoname=CEO&gamemode=7&gamegoal=4&gamestartingdate=2&lifespan=3&stands=1&cupssold=6&cashassets=10&stockassets=0&standsassets=5&upgradesassets=9&retainedearnings=8&revenues=500&checksumclient=2485", "SUCCESS\n"},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rr := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", tc.path, rr.Code)
		}
		if rr.Body.String() != tc.want {
			t.Fatalf("%s body = %q, want %q", tc.path, rr.Body.String(), tc.want)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/lsx2.php", nil)
	rr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/lsx2.php status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Lemonade Stock Exchange") || !strings.Contains(body, "Co") {
		t.Fatalf("/lsx2.php body did not include expected local leaderboard content")
	}

	req = httptest.NewRequest(http.MethodGet, "/img/lsx2/connection.gif", nil)
	rr = httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("connection gif status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "image/gif" {
		t.Fatalf("connection gif content-type = %q, want image/gif", got)
	}
	if rr.Body.Len() == 0 {
		t.Fatal("connection gif body is empty")
	}
}

func TestSyncChecksumCompatibilityModes(t *testing.T) {
	path := "/syncgame.php?game=lemonade2&username=u&password=p&companyname=Co&ceoname=CEO&gamemode=0&gamegoal=0&gamestartingdate=0&lifespan=1&stands=1&cupssold=0&cashassets=49000&stockassets=0&standsassets=3000&upgradesassets=0&retainedearnings=0&revenues=0&checksumclient=0"

	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Body.String() != "SUCCESS\n" {
		t.Fatalf("default checksum mode returned status=%d body=%q, want 200 SUCCESS", rr.Code, rr.Body.String())
	}

	strictSrv, err := NewServer(Config{
		DBPath:         filepath.Join(t.TempDir(), "strict.sqlite3"),
		StrictChecksum: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = strictSrv.Close() }()

	req = httptest.NewRequest(http.MethodGet, path, nil)
	rr = httptest.NewRecorder()
	strictSrv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Body.String() != "FAIL\n" {
		t.Fatalf("strict checksum mode returned status=%d body=%q, want 200 FAIL", rr.Code, rr.Body.String())
	}
}

func TestModernBrowsersSeeProjectPage(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	req := httptest.NewRequest(http.MethodGet, "/lsx2.php", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 AppleWebKit/537.36 Chrome/124.0.0.0 Safari/537.36")
	rr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("modern /lsx2.php status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Open LSX Board") || strings.Contains(body, "Reverse engineering archive") || strings.Contains(body, `href="/lsx2.css"`) {
		t.Fatalf("modern /lsx2.php did not render project page")
	}
	if strings.Contains(body, "Main Street Citrus") || strings.Contains(body, "Central Park Squeeze") || !strings.Contains(body, "No LSX submissions have been received yet.") {
		t.Fatalf("modern project page rendered static leaderboard rows instead of empty database state")
	}
	if !strings.Contains(body, "<style>") || strings.Contains(body, `href="/project.css`) {
		t.Fatalf("project page did not inline CSS")
	}
	if strings.Contains(body, "/admin") {
		t.Fatalf("project page exposed admin link")
	}

	rr = getRoute(t, srv, "/syncgame.php?game=lemonade2&username=project-preview&password=p&companyname=LiveProjectCo&ceoname=CEO&gamemode=1&gamegoal=1&gamestartingdate=1&lifespan=3&stands=1&cupssold=6&cashassets=1000&stockassets=0&standsassets=500&upgradesassets=900&retainedearnings=0&revenues=500&checksumclient=0")
	if rr.Code != http.StatusOK || rr.Body.String() != "SUCCESS\n" {
		t.Fatalf("project preview sync status=%d body=%q, want 200 SUCCESS", rr.Code, rr.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/lsx2.php", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 AppleWebKit/537.36 Chrome/124.0.0.0 Safari/537.36")
	rr = httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	body = rr.Body.String()
	if !strings.Contains(body, "LiveProjectCo") || !strings.Contains(body, "1 LSX submission currently ranked.") {
		t.Fatalf("modern project page did not render leaderboard preview from database")
	}

	req = httptest.NewRequest(http.MethodGet, "/lsx2.php?legacy=1", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 AppleWebKit/537.36 Chrome/124.0.0.0 Safari/537.36")
	rr = httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), "Lemonade Stock Exchange") {
		t.Fatalf("legacy override did not render LSX board")
	}

	req = httptest.NewRequest(http.MethodGet, "/board", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 AppleWebKit/537.36 Chrome/124.0.0.0 Safari/537.36")
	rr = httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), "Lemonade Stock Exchange") {
		t.Fatalf("/board did not render LSX board for modern browser")
	}

	req = httptest.NewRequest(http.MethodGet, "/lsx2.php", nil)
	req.Header.Set("User-Agent", "Mozilla/4.0 (compatible; MSIE 6.0; Windows NT 5.1)")
	rr = httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), "Lemonade Stock Exchange") {
		t.Fatalf("old IE user agent did not render LSX board")
	}
}

func TestEmbeddedIEBrowserFallsBackToLeaderboardEverywhere(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	const ieUA = "Mozilla/4.0 (compatible; MSIE 7.0; Windows NT 5.1; Trident/4.0)"
	tests := []string{
		"/findings",
		"/project/asset/lt2_icon_lsx.avif",
		"/admin",
		"/not-a-real-page",
	}
	for _, path := range tests {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("User-Agent", ieUA)
		rr := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s legacy IE status = %d, want 200", path, rr.Code)
		}
		body := rr.Body.String()
		if !strings.Contains(body, "Lemonade Stock Exchange") {
			t.Fatalf("%s legacy IE did not render LSX board", path)
		}
		if strings.Contains(body, `id="findings-blog-root"`) || strings.Contains(body, "Open LSX Board") {
			t.Fatalf("%s legacy IE rendered modern project content", path)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/syncgame.php?username=u&companyname=Co", nil)
	req.Header.Set("User-Agent", ieUA)
	rr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Body.String() != "SUCCESS\n" {
		t.Fatalf("legacy IE sync route status=%d body=%q, want 200 SUCCESS", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/createaccount.php?username=u&password=p", nil)
	req.Header.Set("User-Agent", ieUA)
	req.Header.Set("Accept", "text/plain")
	rr = httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Body.String() != "ACCEPT\n" {
		t.Fatalf("legacy IE account route status=%d body=%q, want 200 ACCEPT", rr.Code, rr.Body.String())
	}
}

func TestLegacyIENonDocumentRequestsDoNotFallbackToLeaderboard(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	const ieUA = "Mozilla/4.0 (compatible; MSIE 7.0; Windows NT 5.1; Trident/4.0)"
	tests := []struct {
		name string
		path string
		set  func(*http.Request)
	}{
		{
			name: "game style text request",
			path: "/unknown-protocol-call",
			set: func(req *http.Request) {
				req.Header.Set("Accept", "text/plain")
			},
		},
		{
			name: "fetch style request with sec fetch",
			path: "/unknown-fetch-call",
			set: func(req *http.Request) {
				req.Header.Set("Accept", "*/*")
				req.Header.Set("Sec-Fetch-Dest", "empty")
			},
		},
		{
			name: "wildcard accept request",
			path: "/unknown-wildcard-call",
			set: func(req *http.Request) {
				req.Header.Set("Accept", "*/*")
			},
		},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		req.Header.Set("User-Agent", ieUA)
		tc.set(req)
		rr := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", tc.name, rr.Code)
		}
		if strings.Contains(rr.Body.String(), "<table>") {
			t.Fatalf("%s was forced to the legacy leaderboard", tc.name)
		}
	}
}

func TestLegacyIEBoardAndDetailViewsReachEventLog(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	const ieUA = "Mozilla/4.0 (compatible; MSIE 8.0; Windows NT 5.1; Trident/4.0)"
	for _, path := range []string{
		"/board",
		"/lsx2_detail.php?d1=IECurlCo&d2=IEBoss&d5=1&d18=9.99",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("User-Agent", ieUA)
		req.Header.Set("Accept", "text/html")
		rr := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, rr.Code)
		}
	}

	events, err := srv.RecentEvents(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("RecentEvents returned %d rows, want board and detail", len(events))
	}
	if events[0].Kind != "leaderboard" || events[1].Kind != "detail" {
		t.Fatalf("RecentEvents = %#v, want leaderboard then detail", events)
	}
}

func TestNoisyBrowserProbesAndForcedLegacyFallbacksDoNotFillEventLog(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	const ieUA = "Mozilla/4.0 (compatible; MSIE 7.0; Windows NT 5.1; Trident/4.0)"
	paths := []string{
		"/.well-known/appspecific/com.chrome.devtools.json",
		"/lsx2_detail_blank.php",
		"/findings",
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if path != "/.well-known/appspecific/com.chrome.devtools.json" {
			req.Header.Set("User-Agent", ieUA)
		}
		rr := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rr, req)
	}

	events, err := srv.RecentEvents(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("RecentEvents returned noisy browser events: %#v", events)
	}

	req := httptest.NewRequest(http.MethodGet, "/syncgame.php?username=u&companyname=Co", nil)
	req.Header.Set("User-Agent", ieUA)
	rr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)

	events, err = srv.RecentEvents(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Kind != "sync" {
		t.Fatalf("RecentEvents after sync = %#v, want one sync event", events)
	}
}

func TestModernBrowserRoutesRemainNormal(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	const modernUA = "Mozilla/5.0 AppleWebKit/537.36 Chrome/124.0.0.0 Safari/537.36"

	req := httptest.NewRequest(http.MethodGet, "/project/asset/lt2_icon_lsx.avif", nil)
	req.Header.Set("User-Agent", modernUA)
	rr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("modern project asset status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "image/avif" {
		t.Fatalf("modern project asset content-type = %q, want image/avif", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("User-Agent", modernUA)
	rr = httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("modern /admin status=%d, want 404 when admin credentials are unset", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/not-a-real-page", nil)
	req.Header.Set("User-Agent", modernUA)
	rr = httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("modern unknown route status = %d, want 404", rr.Code)
	}
}

func TestFindingsBlogPage(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	req := httptest.NewRequest(http.MethodGet, "/findings", nil)
	rr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/findings status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `id="findings-blog-root"`) ||
		!strings.Contains(body, "findings/entry.js") ||
		!strings.Contains(body, "Lemonade2.rb is a resource container") ||
		!strings.Contains(body, "Decode one bitmap record") {
		t.Fatalf("/findings did not render the generated findings article")
	}
	if strings.Contains(body, "/admin") {
		t.Fatalf("findings page exposed admin link")
	}
}

func TestDocsPage(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	rr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/docs status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `id="openapi-docs-root"`) ||
		!strings.Contains(body, "docs/entry.js") ||
		!strings.Contains(body, "/openapi.yaml") ||
		!strings.Contains(body, "LSX Server API") {
		t.Fatalf("/docs did not render the OpenAPI docs shell")
	}
	if strings.Contains(body, "/admin") {
		t.Fatalf("docs page exposed admin link")
	}
}

func TestOpenAPIYAMLRoute(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	rr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/openapi.yaml status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/yaml") {
		t.Fatalf("/openapi.yaml content-type = %q, want application/yaml", got)
	}
	if body := rr.Body.String(); !strings.Contains(body, "openapi: 3.1.0") ||
		!strings.Contains(body, "/api/v1/leaderboard") {
		t.Fatalf("/openapi.yaml did not serve the OpenAPI document")
	}
}

func TestOpenAPIYAMLRouteUsesConfiguredContract(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(Config{
		DBPath:      filepath.Join(t.TempDir(), "lsx.sqlite3"),
		OpenAPIYAML: []byte("openapi: 3.1.0\ninfo:\n  title: embedded test\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	rr := getRoute(t, srv, "/openapi.yaml")
	if rr.Code != http.StatusOK {
		t.Fatalf("/openapi.yaml status = %d, want 200", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "title: embedded test") {
		t.Fatalf("/openapi.yaml body = %q, want configured contract", body)
	}
}
