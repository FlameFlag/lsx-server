package lsx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPILeaderboardReturnsRankedJSON(t *testing.T) {
	srv := newAPITestServer(t)
	insertAPISubmission(t, srv, "slow", "Slow Stand", "1000", "0", "0", "0", "100", "1", "1")
	insertAPISubmission(t, srv, "fast", "Fast Citrus", "5000", "0", "0", "0", "500", "2", "2")

	rr := getRoute(t, srv, "/api/v1/leaderboard")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}

	var resp apiLeaderboardResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v\n%s", err, rr.Body.String())
	}
	if resp.Pagination.TotalItems != 2 || resp.Pagination.PageSize != apiDefaultPageSize {
		t.Fatalf("pagination = %+v, want 2 items and default page size", resp.Pagination)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("data len = %d, want 2", len(resp.Data))
	}
	if resp.Data[0].Rank != 1 || resp.Data[0].Company != "Fast Citrus" || resp.Data[0].MarketCents != 5000 {
		t.Fatalf("top row = %+v, want Fast Citrus ranked first by market", resp.Data[0])
	}
	if resp.Sort != "market" {
		t.Fatalf("sort = %q, want market", resp.Sort)
	}
}

func TestAPILeaderboardFiltersSortsAndPaginates(t *testing.T) {
	srv := newAPITestServer(t)
	insertAPISubmission(t, srv, "ada", "Beta Lemon", "1000", "0", "0", "0", "100", "1", "1")
	insertAPISubmission(t, srv, "ada", "Alpha Lemon", "2000", "0", "0", "0", "200", "1", "1")
	insertAPISubmission(t, srv, "bea", "Zeta Lemon", "3000", "0", "0", "0", "300", "2", "1")

	rr := getRoute(t, srv, "/api/v1/leaderboard?username=ada&gamemode=1&sort=company&page=2&page_size=1")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}

	var resp apiLeaderboardResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Pagination.Page != 2 || resp.Pagination.PageSize != 1 || resp.Pagination.TotalItems != 2 || resp.Pagination.TotalPages != 2 {
		t.Fatalf("pagination = %+v, want page 2 of 2 with one item per page", resp.Pagination)
	}
	if resp.Filters.Username != "ada" || resp.Filters.GameMode != "1" {
		t.Fatalf("filters = %+v, want username and gamemode echoed", resp.Filters)
	}
	if len(resp.Data) != 1 || resp.Data[0].Company != "Beta Lemon" || resp.Data[0].Rank != 2 {
		t.Fatalf("data = %+v, want second company-sorted ada row", resp.Data)
	}
}

func TestAPILeaderboardRejectsBadQuery(t *testing.T) {
	srv := newAPITestServer(t)

	rr := getRoute(t, srv, "/api/v1/leaderboard?page_size=500")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("content-type = %q, want application/problem+json", got)
	}
	if !strings.Contains(rr.Body.String(), `"status":400`) || !strings.Contains(rr.Body.String(), "page_size") {
		t.Fatalf("problem body missing expected details: %s", rr.Body.String())
	}
}

func TestAPILeaderboardSupportsHead(t *testing.T) {
	srv := newAPITestServer(t)
	insertAPISubmission(t, srv, "fast", "Fast Citrus", "5000", "0", "0", "0", "500", "2", "2")

	req := httptest.NewRequest(http.MethodHead, "/api/v1/leaderboard", nil)
	rr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("HEAD body len = %d, want 0", rr.Body.Len())
	}
}

func TestAPILeaderboardRejectsUnsupportedMethod(t *testing.T) {
	srv := newAPITestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/leaderboard", nil)
	rr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
	if got := rr.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("allow = %q, want GET, HEAD", got)
	}
}

func newAPITestServer(t *testing.T) *Server {
	t.Helper()
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = srv.Close()
	})
	return srv
}

func insertAPISubmission(t *testing.T, srv *Server, username string, company string, cash string, stock string, standsAssets string, upgrades string, revenues string, gameMode string, gameGoal string) {
	t.Helper()
	q := url.Values{
		"game":             {"lemonade2"},
		"username":         {username},
		"password":         {"p"},
		"companyname":      {company},
		"ceoname":          {"CEO"},
		"gamemode":         {gameMode},
		"gamegoal":         {gameGoal},
		"gamestartingdate": {"1"},
		"lifespan":         {"3"},
		"stands":           {"1"},
		"cupssold":         {"6"},
		"cashassets":       {cash},
		"stockassets":      {stock},
		"standsassets":     {standsAssets},
		"upgradesassets":   {upgrades},
		"retainedearnings": {"0"},
		"revenues":         {revenues},
		"checksumclient":   {"0"},
	}
	rr := getRoute(t, srv, "/syncgame.php?"+q.Encode())
	if rr.Code != http.StatusOK || rr.Body.String() != "SUCCESS\n" {
		t.Fatalf("insert submission status=%d body=%q", rr.Code, rr.Body.String())
	}
}
