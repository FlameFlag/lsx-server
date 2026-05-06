package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNormalizeListenAddr(t *testing.T) {
	tests := map[string]string{
		"80":             ":80",
		":80":            ":80",
		"127.0.0.1:8080": "127.0.0.1:8080",
		"localhost:8080": "localhost:8080",
	}

	for input, want := range tests {
		if got := normalizeListenAddr(input); got != want {
			t.Fatalf("normalizeListenAddr(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLoadDotEnv(t *testing.T) {
	restoreEnv(t, "LSX_ADMIN_USER", "LSX_ADMIN_PASSWORD")
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("LSX_ADMIN_USER=dotenv\nLSX_ADMIN_PASSWORD=secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := loadDotEnv(); err != nil {
		t.Fatalf("loadDotEnv() error = %v", err)
	}
	if got := os.Getenv("LSX_ADMIN_USER"); got != "dotenv" {
		t.Fatalf("LSX_ADMIN_USER = %q, want dotenv", got)
	}
	if got := os.Getenv("LSX_ADMIN_PASSWORD"); got != "secret" {
		t.Fatalf("LSX_ADMIN_PASSWORD = %q, want secret", got)
	}
}

func TestServeOptionsFromEnv(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("LSX_HTTP_PORT", "8080")
	t.Setenv("LSX_DATA", "/tmp/lsx.sqlite3")
	t.Setenv("LSX_SEED", "true")
	t.Setenv("LSX_STRICT_CHECKSUM", "1")
	t.Setenv("LSX_PLAIN", "true")
	t.Setenv("LSX_ADMIN_USER", "admin")
	t.Setenv("LSX_ADMIN_PASSWORD", "secret")
	t.Setenv("LSX_ADMIN_PATH", "/manage")
	t.Setenv("LSX_DISCORD_WEBHOOK", "https://discord.com/api/webhooks/example")
	t.Setenv("LSX_DISCORD_EVENTS", "sync")
	t.Setenv("LSX_DISCORD_ICON", "")
	t.Setenv("LSX_DISCORD_TIMEOUT", "10s")

	opts, err := serveOptionsFromEnv()
	if err != nil {
		t.Fatalf("serveOptionsFromEnv() error = %v", err)
	}
	if opts.addr != "8080" {
		t.Fatalf("addr = %q, want 8080", opts.addr)
	}
	if opts.dbPath != "/tmp/lsx.sqlite3" {
		t.Fatalf("dbPath = %q, want /tmp/lsx.sqlite3", opts.dbPath)
	}
	if !opts.seed || !opts.strictChecksum || !opts.plain {
		t.Fatalf("boolean envs not applied: seed=%v strictChecksum=%v plain=%v", opts.seed, opts.strictChecksum, opts.plain)
	}
	if opts.adminUser != "admin" || opts.adminPassword != "secret" || opts.adminPath != "/manage" {
		t.Fatalf("admin envs not applied: %#v", opts)
	}
	if opts.discordWebhook != "https://discord.com/api/webhooks/example" || opts.discordEvents != "sync" || opts.discordIcon != "" {
		t.Fatalf("discord string envs not applied: %#v", opts)
	}
	if opts.discordTimeout != 10*time.Second {
		t.Fatalf("discordTimeout = %v, want 10s", opts.discordTimeout)
	}
}

func TestLSXAddrOverridesHTTPPort(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("LSX_ADDR", "127.0.0.1:9090")
	t.Setenv("LSX_HTTP_PORT", "8080")

	opts, err := serveOptionsFromEnv()
	if err != nil {
		t.Fatalf("serveOptionsFromEnv() error = %v", err)
	}
	if opts.addr != "127.0.0.1:9090" {
		t.Fatalf("addr = %q, want 127.0.0.1:9090", opts.addr)
	}
}

func TestServeOptionsFromEnvRejectsInvalidBool(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("LSX_SEED", "sometimes")

	if _, err := serveOptionsFromEnv(); err == nil {
		t.Fatal("serveOptionsFromEnv() error = nil, want error")
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	clearEnv(t,
		"LSX_ADDR",
		"LSX_HTTP_PORT",
		"LSX_DATA",
		"LSX_SEED",
		"LSX_STRICT_CHECKSUM",
		"LSX_PLAIN",
		"LSX_ADMIN_USER",
		"LSX_ADMIN_PASSWORD",
		"LSX_ADMIN_PATH",
		"LSX_DISCORD_WEBHOOK",
		"LSX_DISCORD_EVENTS",
		"LSX_DISCORD_ICON",
		"LSX_DISCORD_TIMEOUT",
	)
}

func restoreEnv(t *testing.T, names ...string) {
	t.Helper()
	original := make(map[string]*string, len(names))
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok {
			value := value
			original[name] = &value
		} else {
			original[name] = nil
		}
	}
	t.Cleanup(func() {
		for name, value := range original {
			if value == nil {
				_ = os.Unsetenv(name)
				continue
			}
			_ = os.Setenv(name, *value)
		}
	})
}

func clearEnv(t *testing.T, names ...string) {
	t.Helper()
	restoreEnv(t, names...)
	for _, name := range names {
		_ = os.Unsetenv(name)
	}
}
