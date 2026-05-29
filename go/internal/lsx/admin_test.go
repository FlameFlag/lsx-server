package lsx

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdminDisabledUntilCredentialsConfigured(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	rr := getRoute(t, srv, "/admin")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("default /admin status = %d, want 404", rr.Code)
	}

	rr = getRoute(t, srv, "/admin/login")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("default /admin/login status = %d, want 404", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "Admin credentials are not configured") {
		t.Fatalf("default /admin/login exposed disabled admin message")
	}

	rr = getRoute(t, srv, "/admin/asset/login.css")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("default /admin/asset/login.css status = %d, want 404", rr.Code)
	}

	enabled, err := NewServer(Config{
		DBPath:        filepath.Join(t.TempDir(), "lsx.sqlite3"),
		AdminUser:     "admin",
		AdminPassword: "admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = enabled.Close() }()

	rr = getRoute(t, enabled, "/admin")
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("configured /admin status = %d, want redirect to login", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/admin/login" {
		t.Fatalf("configured /admin location = %q, want /admin/login", got)
	}
}

func TestAdminCustomPathHidesDefaultPath(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath:        filepath.Join(t.TempDir(), "lsx.sqlite3"),
		AdminUser:     "admin",
		AdminPassword: "secret",
		AdminPath:     "manage-8f3c2a",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	rr := getRoute(t, srv, "/admin")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("/admin status = %d, want 404 when custom admin path is configured", rr.Code)
	}

	rr = getRoute(t, srv, "/manage-8f3c2a")
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("custom admin root status = %d, want redirect to login", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/manage-8f3c2a/login" {
		t.Fatalf("custom admin root location = %q, want /manage-8f3c2a/login", got)
	}

	rr = getRoute(t, srv, "/manage-8f3c2a/login")
	if rr.Code != http.StatusOK {
		t.Fatalf("custom admin login status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `data-page="admin-login"`) ||
		!strings.Contains(body, `"AdminPath":"/manage-8f3c2a"`) ||
		!strings.Contains(body, `/project/asset/svelte/`) {
		t.Fatalf("custom admin login did not use expected asset paths:\n%s", body)
	}

	rr = getRoute(t, srv, "/manage-8f3c2a/asset/admin.css")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("custom admin legacy stylesheet status = %d, want 404", rr.Code)
	}

	rr = getRoute(t, srv, "/manage-8f3c2a/asset/warning.avif")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("custom admin legacy image status = %d, want 404", rr.Code)
	}
}

func TestAdminCustomPathScopesLoginCookie(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath:        filepath.Join(t.TempDir(), "lsx.sqlite3"),
		AdminUser:     "admin",
		AdminPassword: "secret",
		AdminPath:     "/manage-8f3c2a",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	form := url.Values{
		"username": {"admin"},
		"password": {"secret"},
	}
	req := httptest.NewRequest(http.MethodPost, "/manage-8f3c2a/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d, want redirect", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/manage-8f3c2a" {
		t.Fatalf("login location = %q, want /manage-8f3c2a", got)
	}
	cookies := rr.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("login set %d cookies, want 1", len(cookies))
	}
	if got := cookies[0].Path; got != "/manage-8f3c2a" {
		t.Fatalf("admin cookie path = %q, want /manage-8f3c2a", got)
	}
}

func TestAdminPathValidation(t *testing.T) {
	tests := []string{"/", "/admin?x=1", "/bad path", `/bad\path`}
	for _, adminPath := range tests {
		_, err := NewServer(Config{
			DBPath:    filepath.Join(t.TempDir(), "lsx.sqlite3"),
			AdminPath: adminPath,
		})
		if err == nil {
			t.Fatalf("NewServer(AdminPath %q) succeeded, want error", adminPath)
		}
	}
}
