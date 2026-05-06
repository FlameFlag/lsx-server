package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	mirrorDir    = "project/findings/vendor/shiki/unpkg"
	upstreamHost = "unpkg.com"
	urlTimeout   = 30 * time.Second
)

var roots = []string{
	"https://unpkg.com/shiki@4.0.2/core?module",
	"https://unpkg.com/shiki@4.0.2/engine/oniguruma?module",
	"https://unpkg.com/shiki@4.0.2/wasm?module",
	"https://unpkg.com/@shikijs/themes@4.0.2/github-light?module",
	"https://unpkg.com/@shikijs/langs@4.0.2/c?module",
	"https://unpkg.com/@shikijs/langs@4.0.2/http?module",
	"https://unpkg.com/@shikijs/langs@4.0.2/python?module",
}

var staticImport = regexp.MustCompile(`(?m)(^|[;\n])\s*(?:import|export)\b(?:[^"'` + "`" + `]*?\bfrom\s*)?\s*["'][^"']+["']`)

type downloader struct {
	client *http.Client
	seen   map[string]bool
}

func main() {
	d := downloader{
		client: &http.Client{Timeout: urlTimeout},
		seen:   make(map[string]bool),
	}

	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		fatal(err)
	}

	for _, root := range roots {
		if err := d.download(root); err != nil {
			fatal(err)
		}
	}
}

func (d downloader) download(rawURL string) error {
	sourceURL, err := parseURL(rawURL)
	if err != nil {
		return err
	}
	if !isUpstream(sourceURL) || d.seen[sourceURL.String()] {
		return nil
	}
	d.seen[sourceURL.String()] = true

	body, finalURL, err := d.fetch(sourceURL)
	if err != nil {
		return err
	}

	name, err := localFile(finalURL)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(name, body, 0o644); err != nil {
		return err
	}

	for _, spec := range importSpecifiers(string(body)) {
		dependency, ok, err := resolveImport(finalURL, spec)
		if err != nil {
			return err
		}
		if ok {
			if err := d.download(dependency.String()); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d downloader) fetch(sourceURL *url.URL) ([]byte, *url.URL, error) {
	resp, err := d.client.Get(sourceURL.String())
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("download %s: %s", sourceURL, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	finalURL := *resp.Request.URL
	finalURL.Fragment = ""
	return body, &finalURL, nil
}

func importSpecifiers(source string) []string {
	matches := staticImport.FindAllString(source, -1)
	specs := make([]string, 0, len(matches))
	for _, match := range matches {
		if spec := quotedValue(match); spec != "" {
			specs = append(specs, spec)
		}
	}
	return specs
}

func quotedValue(text string) string {
	for _, quote := range []string{`"`, `'`} {
		start := strings.Index(text, quote)
		end := strings.LastIndex(text, quote)
		if start >= 0 && end > start {
			return text[start+1 : end]
		}
	}
	return ""
}

func resolveImport(baseURL *url.URL, spec string) (*url.URL, bool, error) {
	if strings.HasPrefix(spec, "data:") || strings.HasPrefix(spec, "node:") {
		return nil, false, nil
	}

	ref, err := url.Parse(spec)
	if err != nil {
		return nil, false, err
	}
	ref = baseURL.ResolveReference(ref)
	ref.Fragment = ""

	return ref, isUpstream(ref), nil
}

func localFile(sourceURL *url.URL) (string, error) {
	rel := strings.TrimPrefix(sourceURL.EscapedPath(), "/")
	local, err := filepath.Localize(rel)
	if err != nil {
		return "", err
	}
	return filepath.Join(mirrorDir, local), nil
}

func parseURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	parsed.Fragment = ""
	return parsed, nil
}

func isUpstream(sourceURL *url.URL) bool {
	return sourceURL.Scheme == "https" && sourceURL.Host == upstreamHost
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
