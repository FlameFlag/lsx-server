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
		"/admin/asset/admin.css",
		"/admin/asset/login.css",
		"/admin/asset/dashboard/foundation.css",
		"/admin/asset/login/foundation.css",
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
		case strings.HasSuffix(path, "admin.css"):
			if !strings.Contains(body, `@import url("dashboard/foundation.css");`) {
				t.Fatalf("%s stylesheet body did not include expected imports", path)
			}
		case strings.HasSuffix(path, "login.css"):
			if !strings.Contains(body, `@import url("login/foundation.css");`) {
				t.Fatalf("%s stylesheet body did not include expected imports", path)
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
		"/admin/asset/upload_icon.avif",
		"/admin/asset/pitcher.avif",
		"/admin/asset/warning.avif",
		"/admin/asset/lt2_green_pill.avif",
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

func TestProjectFontAssetRoutes(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	tests := []string{
		"/project/asset/fonts/NotoSans-Bold.ttf",
		"/project/asset/fonts/NotoSans-Bold.woff2",
		"/project/asset/fonts/RobotoCondensed-Bold.ttf",
		"/project/asset/fonts/RobotoCondensed-Bold.woff2",
	}
	for _, path := range tests {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, rr.Code)
		}
		wantType := "font/ttf"
		wantHeader := "\x00\x01\x00\x00"
		if strings.HasSuffix(path, ".woff2") {
			wantType = "font/woff2"
			wantHeader = "wOF2"
		}
		if got := rr.Header().Get("Content-Type"); got != wantType {
			t.Fatalf("%s content-type = %q, want %s", path, got, wantType)
		}
		if !strings.HasPrefix(rr.Body.String(), wantHeader) {
			t.Fatalf("%s body did not start with expected font header", path)
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

	tests := map[string]string{
		"/project/asset/docs/entry.js":     "openapi-docs-root",
		"/project/asset/findings/code.js":  "export const highlightCodeBlocks",
		"/project/asset/findings/entry.js": "highlightCodeBlocks",
	}
	for path, want := range tests {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, rr.Code)
		}
		if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/javascript") {
			t.Fatalf("%s content-type = %q, want text/javascript", path, got)
		}
		if !strings.Contains(rr.Body.String(), want) {
			t.Fatalf("%s body did not include %q", path, want)
		}
	}
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
