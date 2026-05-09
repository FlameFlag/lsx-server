package lsx

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-chi/chi/v5"

	"lt2_reverse/lsx_server_go/internal/lsx/compat"
	"lt2_reverse/lsx_server_go/internal/lsxvalue"
)

type requestContextKey string

const forcedLegacyLeaderboardKey requestContextKey = "forced_legacy_leaderboard"

type routeHandler func(*Server, http.ResponseWriter, *http.Request)

func staticCSSRoute(name string) routeHandler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.handleStaticCSS(w, r, name)
	}
}

func (s *Server) Routes() http.Handler {
	router := chi.NewRouter()
	s.mountRoutes(router)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldForceLegacyLeaderboard(r) {
			r = r.WithContext(context.WithValue(r.Context(), forcedLegacyLeaderboardKey, true))
			s.handleLeaderboard(w, r)
			return
		}
		if s.isAdminRoute(r.URL.Path) {
			s.handleAdmin(w, r)
			return
		}
		router.ServeHTTP(w, r)
	})
}

func (s *Server) mountRoutes(router chi.Router) {
	router.HandleFunc("/api/v1/leaderboard", s.withServer((*Server).handleAPILeaderboard))
	router.HandleFunc("/project/asset/*", s.withServer((*Server).handleProjectAsset))
	router.HandleFunc("/board", s.withServer((*Server).handleLeaderboard))
	router.HandleFunc("/leaderboard", s.withServer((*Server).handleLeaderboard))
	router.HandleFunc("/findings", s.withServer((*Server).handleFindingsPage))
	router.HandleFunc("/docs", s.withServer((*Server).handleDocsPage))
	router.HandleFunc("/openapi.yaml", s.withServer((*Server).handleOpenAPIYAML))
	router.HandleFunc("/", s.withServer((*Server).handleEntryPage))
	router.HandleFunc("/lsx2", s.withServer((*Server).handleEntryPage))
	router.HandleFunc("/lsx2.php", s.withServer((*Server).handleEntryPage))
	router.HandleFunc("/lsx2_detail.php", s.withServer((*Server).handleDetail))
	router.HandleFunc("/lsx2_detail_blank.php", s.withServer((*Server).handleBlankDetail))
	router.HandleFunc("/lsx2.css", s.withServer(staticCSSRoute("legacy/leaderboard.css")))
	router.HandleFunc("/lsx2_detail.css", s.withServer(staticCSSRoute("legacy/detail.css")))
	router.HandleFunc("/project.css", s.withServer((*Server).handleProjectCSS))
	router.HandleFunc("/syncgame", s.withServer((*Server).handleSync))
	router.HandleFunc("/syncgame.php", s.withServer((*Server).handleSync))
	router.HandleFunc("/createaccount", s.withServer((*Server).handleCreateAccount))
	router.HandleFunc("/createaccount.php", s.withServer((*Server).handleCreateAccount))
	router.HandleFunc("/img/lsx2/connection.gif", s.withServer((*Server).handleConnectionGIF))
	router.HandleFunc("/healthz", s.withServer((*Server).handleHealthz))
	router.HandleFunc("/favicon.ico", s.withServer((*Server).handleFavicon))
	router.NotFound(s.handleNotFound)
}

func (s *Server) withServer(handler routeHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler(s, w, r)
	}
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	s.emit(r, "not_found", http.StatusNotFound, "unknown endpoint")
	http.NotFound(w, r)
}

func (s *Server) handleEntryPage(w http.ResponseWriter, r *http.Request) {
	if shouldServeProjectPage(r) {
		s.handleProjectPage(w, r)
		return
	}
	s.handleLeaderboard(w, r)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	s.emit(r, "health", http.StatusOK, "health check")
	writeText(w, http.StatusOK, "OK\n")
}

func (s *Server) handleFavicon(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func shouldServeProjectPage(r *http.Request) bool {
	if r.URL.Query().Get("legacy") == "1" {
		return false
	}
	userAgent := r.UserAgent()
	if userAgent == "" {
		return false
	}
	ua := strings.ToLower(userAgent)
	if isLegacyIEUserAgent(ua) {
		return false
	}
	for _, token := range compat.ModernBrowserTokens {
		if strings.Contains(ua, token) {
			return true
		}
	}
	return false
}

func shouldForceLegacyLeaderboard(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if !isLegacyIEUserAgent(r.UserAgent()) {
		return false
	}
	if !isDocumentNavigation(r) {
		return false
	}

	switch r.URL.Path {
	case "/", "/board", "/leaderboard", "/lsx2", "/lsx2.php",
		"/docs", "/openapi.yaml",
		"/lsx2_detail.php", "/lsx2_detail_blank.php",
		"/lsx2.css", "/lsx2_detail.css",
		"/syncgame", "/syncgame.php",
		"/createaccount", "/createaccount.php",
		"/img/lsx2/connection.gif",
		"/healthz", "/favicon.ico":
		return false
	}
	return true
}

func isDocumentNavigation(r *http.Request) bool {
	if dest := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Dest"))); dest != "" {
		return dest == "document" || dest == "iframe" || dest == "frame"
	}

	accept := strings.ToLower(r.Header.Get("Accept"))
	if accept == "" {
		return true
	}
	for part := range strings.SplitSeq(accept, ",") {
		mediaType, _, _ := strings.Cut(part, ";")
		mediaType = strings.TrimSpace(mediaType)
		switch mediaType {
		case "text/html", "application/xhtml+xml":
			return true
		}
	}
	return false
}

func isLegacyIEUserAgent(userAgent string) bool {
	// TeneonIERelease.dll is an MFC CWebBrowser/IEBrowserContainer wrapper,
	// which presents as the installed Internet Explorer engine.
	ua := strings.ToLower(userAgent)
	return strings.Contains(ua, "msie") || strings.Contains(ua, "trident/")
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.emit(r, "sync_error", http.StatusBadRequest, "bad query form")
		writeText(w, http.StatusBadRequest, "BADREQUEST\n")
		return
	}

	fields := make(map[string]string, len(compat.SyncFields))
	for _, name := range compat.SyncFields {
		fields[name] = r.Form.Get(name)
	}
	computed := compat.ComputeChecksum(fields)
	client, present := lsxvalue.ParseI32(fields[compat.FieldChecksumClient])
	valid := !present || client == computed

	sub := Submission{
		ReceivedAt:       time.Now().UTC(),
		RemoteAddr:       remoteIP(r),
		Host:             r.Host,
		RawQuery:         r.URL.RawQuery,
		Fields:           fields,
		ChecksumClient:   client,
		ChecksumComputed: computed,
		ChecksumPresent:  present,
		ChecksumValid:    valid,
	}

	if err := s.appendSubmission(sub); err != nil {
		log.Error("store submission", "err", err)
		s.emit(r, "sync_error", http.StatusInternalServerError, err.Error())
		writeText(w, http.StatusInternalServerError, "ERROR\n")
		return
	}
	if s.strictChecksum && present && !valid {
		s.emit(r, "sync_rejected", http.StatusOK, syncMessage(fields, valid, computed, client))
		writeText(w, http.StatusOK, "FAIL\n")
		return
	}
	s.emit(r, "sync", http.StatusOK, syncMessage(fields, valid, computed, client))
	writeText(w, http.StatusOK, "SUCCESS\n")
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err == nil {
		req := AccountRequest{
			ReceivedAt: time.Now().UTC(),
			RemoteAddr: remoteIP(r),
			Host:       r.Host,
			Username:   r.Form.Get(compat.FieldUsername),
			Password:   r.Form.Get(compat.FieldPassword),
			RawQuery:   r.URL.RawQuery,
		}
		if err := s.appendAccount(req); err != nil {
			log.Error("store account request", "err", err)
			s.emit(r, "account_error", http.StatusOK, err.Error())
		} else {
			s.emit(r, "account", http.StatusOK, fmt.Sprintf("accepted username=%q", req.Username))
		}
	} else {
		s.emit(r, "account", http.StatusOK, "accepted account request")
	}
	writeText(w, http.StatusOK, "ACCEPT\n")
}

func (s *Server) handleConnectionGIF(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	s.emit(r, "connection", http.StatusOK, "served connection gif")
	_, _ = w.Write(gif1x1)
}

func (s *Server) emit(r *http.Request, kind string, status int, message string) {
	if shouldSuppressRequestEvent(r, kind) {
		return
	}
	ev := Event{
		Time:       time.Now(),
		Kind:       kind,
		Method:     r.Method,
		Path:       r.URL.RequestURI(),
		RemoteAddr: remoteIP(r),
		Status:     status,
		Message:    message,
	}
	if err := s.appendEvent(ev); err != nil {
		log.Error("store request event", "err", err)
	}
	if s.eventSink != nil {
		s.eventSink(ev)
	}
}

func shouldSuppressRequestEvent(r *http.Request, kind string) bool {
	if kind == "not_found" && strings.HasPrefix(r.URL.Path, "/.well-known/appspecific/") {
		return true
	}
	if isLegacyIEUserAgent(r.UserAgent()) {
		if kind == "connection" {
			return true
		}
		if kind == "detail" && r.URL.Path == "/lsx2_detail_blank.php" {
			return true
		}
		if kind == "leaderboard" && r.Context().Value(forcedLegacyLeaderboardKey) == true {
			return true
		}
	}
	return false
}

func syncMessage(fields map[string]string, valid bool, computed int32, client int32) string {
	state := "checksum missing"
	if fields[compat.FieldChecksumClient] != "" {
		state = "checksum ok"
		if !valid {
			state = fmt.Sprintf("checksum mismatch client=%d computed=%d", client, computed)
		}
	}
	return fmt.Sprintf("company=%q ceo=%q user=%q %s", fields[compat.FieldCompanyName], fields[compat.FieldCEOName], fields[compat.FieldUsername], state)
}

func writeText(w http.ResponseWriter, code int, body string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(body))
}

func remoteIP(r *http.Request) string {
	if forwarded := forwardedForIP(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		return forwarded
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func forwardedForIP(header string) string {
	for part := range strings.SplitSeq(header, ",") {
		ip := strings.TrimSpace(part)
		if ip != "" {
			return ip
		}
	}
	return ""
}

func LogRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/favicon.ico" {
			log.Info("request", "remote", remoteIP(r), "method", r.Method, "path", r.URL.RequestURI())
		}
		next.ServeHTTP(w, r)
	})
}
