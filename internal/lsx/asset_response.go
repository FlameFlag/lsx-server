package lsx

import (
	"bytes"
	"net/http"
	"strings"
	"time"
)

func serveWebAsset(w http.ResponseWriter, r *http.Request, name, contentType string, data []byte) {
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}

func assetContentType(name string) string {
	switch {
	case strings.HasPrefix(name, "findings/vendor/shiki/"):
		return "text/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".mjs"):
		return "text/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(name, ".avif"):
		return "image/avif"
	case strings.HasSuffix(name, ".png"):
		return "image/png"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".ttf"):
		return "font/ttf"
	case strings.HasSuffix(name, ".woff2"):
		return "font/woff2"
	default:
		return ""
	}
}
