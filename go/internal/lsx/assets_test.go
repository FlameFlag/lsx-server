package lsx

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestStylesheetRoutes(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath:        filepath.Join(t.TempDir(), "lsx.sqlite3"),
		AdminUser:     "admin",
		AdminPassword: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	tests := []string{
		"/lsx2.css",
		"/lsx2_detail.css",
	}
	for _, path := range tests {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, rr.Code)
		}
		if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/css") {
			t.Fatalf("%s content-type = %q, want text/css", path, got)
		}
		body := rr.Body.String()
		switch {
		case strings.HasPrefix(path, "/lsx2"):
			if !strings.Contains(body, "TeneonIERelease.dll") {
				t.Fatalf("%s legacy stylesheet is missing the embedded IE warning", path)
			}
			if strings.Contains(body, ":nth-child") {
				t.Fatalf("%s legacy stylesheet contains a modern selector", path)
			}
		default:
			if !strings.Contains(body, "body") {
				t.Fatalf("%s stylesheet body did not include expected CSS", path)
			}
		}
	}
}

func TestImageAssetRoutes(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath:        filepath.Join(t.TempDir(), "lsx.sqlite3"),
		AdminUser:     "admin",
		AdminPassword: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	tests := []string{
		"/project/asset/lt2_asset_credits.avif",
		"/project/asset/lt2_asset_github.avif",
		"/project/asset/lt2_green_pill.avif",
		"/project/asset/lt2_icon_findings.avif",
		"/project/asset/lt2_icon_lsx.avif",
		"/project/asset/lt2_icon_play.avif",
		"/project/asset/lt2_lemon_pair.avif",
		"/project/asset/lt2_logo_text_only.avif",
		"/project/asset/menu_map_backdrop.avif",
		"/project/asset/pitcher.avif",
		"/project/asset/warning.avif",
	}
	for _, path := range tests {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, rr.Code)
		}
		if got := rr.Header().Get("Content-Type"); got != "image/avif" {
			t.Fatalf("%s content-type = %q, want image/avif", path, got)
		}
		if rr.Body.Len() == 0 {
			t.Fatalf("%s image body is empty", path)
		}
	}
}

func TestPNGImageAssetRoutes(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	tests := []string{
		"/project/asset/flameflag_lemon.png",
		"/project/asset/timz_lemon.png",
	}
	for _, path := range tests {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, rr.Code)
		}
		if got := rr.Header().Get("Content-Type"); got != "image/png" {
			t.Fatalf("%s content-type = %q, want image/png", path, got)
		}
		if rr.Body.Len() == 0 {
			t.Fatalf("%s image body is empty", path)
		}
	}
}

func TestPNGProjectAssetRoutesAreGoneForModernBrowsers(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	tests := []string{
		"/admin/asset/pitcher.png",
		"/project/asset/menu_map_backdrop.png",
	}
	for _, path := range tests {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 AppleWebKit/537.36 Chrome/124.0.0.0 Safari/537.36")
		rr := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", path, rr.Code)
		}
	}
}

func TestProjectScriptAssetRoute(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	pageReq := httptest.NewRequest(http.MethodGet, "/findings", nil)
	pageReq.Header.Set("User-Agent", "Mozilla/5.0 AppleWebKit/537.36 Chrome/124.0.0.0 Safari/537.36")
	pageRR := httptest.NewRecorder()
	srv.Routes().ServeHTTP(pageRR, pageReq)
	if pageRR.Code != http.StatusOK {
		t.Fatalf("/findings status = %d, want 200", pageRR.Code)
	}
	body := pageRR.Body.String()
	if !strings.Contains(body, `data-page="findings"`) ||
		!strings.Contains(body, `/project/asset/svelte/`) {
		t.Fatalf("/findings did not render the Svelte app shell:\n%s", body)
	}

	assetPaths := svelteAssetPaths(body)
	if len(assetPaths) == 0 {
		t.Fatalf("/findings did not include Svelte asset paths:\n%s", body)
	}
	for _, path := range assetPaths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, rr.Code)
		}
		contentType := rr.Header().Get("Content-Type")
		if strings.HasSuffix(path, ".js") && !strings.HasPrefix(contentType, "text/javascript") {
			t.Fatalf("%s content-type = %q, want text/javascript", path, contentType)
		}
		if strings.HasSuffix(path, ".css") && !strings.HasPrefix(contentType, "text/css") {
			t.Fatalf("%s content-type = %q, want text/css", path, contentType)
		}
	}
}

func svelteAssetPaths(body string) []string {
	var paths []string
	for _, token := range strings.FieldsFunc(body, func(r rune) bool {
		return r == '"' || r == '\''
	}) {
		if strings.HasPrefix(token, "/project/asset/svelte/") {
			paths = append(paths, token)
		}
	}
	return paths
}

func TestGeneratedFindingsContentIsNotServedAsAnAsset(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	for _, path := range []string{
		"/project/asset/findings/content.json",
		"/project/asset/findings/content.html.tmpl",
		"/project/asset/findings/dom.js",
		"/project/asset/findings/render.js",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", path, rr.Code)
		}
	}
}
