package lsx

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	webassets "lt2_reverse/lsx_server_go/assets"
	"lt2_reverse/lsx_server_go/internal/eventpath"
	"lt2_reverse/lsx_server_go/internal/lsx/compat"
	"lt2_reverse/lsx_server_go/internal/lsxvalue"
	"lt2_reverse/lsx_server_go/internal/strutil"
)

const (
	defaultAdminPath = "/admin"
	adminCookieName  = "lsx_admin"
)

//go:embed admin_templates/*.html.tmpl
var adminFS embed.FS

type adminPage struct {
	Title       string
	AdminPath   string
	Active      string
	User        string
	Stats       AdminStats
	Flash       string
	CSRF        string
	Submissions []adminSubmission
	Accounts    []AccountRequest
	Events      []adminEvent
	Fields      []string
}

type adminLoginPage struct {
	AdminPath string
	Error     string
	User      string
}

type adminSubmission struct {
	ID       int64
	When     string
	Remote   string
	Host     string
	Valid    bool
	Computed int32
	Client   int32
	Fields   map[string]string
	Row      LeaderboardRow
}

type adminEvent struct {
	Event
	Route   string
	Summary []detailField
	Raw     string
}

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	if !s.adminEnabled() {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, s.adminURL("asset")+"/") {
		s.handleAdminAsset(w, r)
		return
	}
	switch s.adminRouteSuffix(r.URL.Path) {
	case "/login":
		s.handleAdminLogin(w, r)
	case "/logout":
		if r.Method != http.MethodPost {
			http.Redirect(w, r, s.adminPath, http.StatusSeeOther)
			return
		}
		s.clearAdminSession(w)
		http.Redirect(w, r, s.adminURL("login"), http.StatusSeeOther)
	case "", "/submissions", "/accounts", "/events":
		if !s.requireAdmin(w, r) {
			return
		}
		s.renderAdmin(w, r)
	case "/action":
		if !s.requireAdmin(w, r) {
			return
		}
		s.handleAdminAction(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) adminEnabled() bool {
	return strings.TrimSpace(s.adminUser) != "" && strings.TrimSpace(s.adminPassword) != ""
}

func (s *Server) AdminEnabled() bool {
	return s != nil && s.adminEnabled()
}

func (s *Server) AdminPath() string {
	if s == nil {
		return ""
	}
	return s.adminPath
}

func normalizeAdminPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultAdminPath, nil
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	if strings.ContainsAny(raw, "?#\\") {
		return "", fmt.Errorf("admin path must be a URL path without query, fragment, or backslash: %q", raw)
	}
	clean := path.Clean(raw)
	if clean == "/" {
		return "", errors.New("admin path must not be /")
	}
	for part := range strings.SplitSeq(strings.TrimPrefix(clean, "/"), "/") {
		if part == "" || strings.TrimSpace(part) != part || strings.ContainsAny(part, " \t\r\n") {
			return "", fmt.Errorf("admin path contains an invalid segment: %q", raw)
		}
	}
	return clean, nil
}

func constantTimeStringEqual(got string, want string) bool {
	gotHash := sha256.Sum256([]byte(got))
	wantHash := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1
}

func (s *Server) isAdminRoute(requestPath string) bool {
	return requestPath == s.adminPath || strings.HasPrefix(requestPath, s.adminPath+"/")
}

func (s *Server) adminRouteSuffix(requestPath string) string {
	if requestPath == s.adminPath {
		return ""
	}
	return strings.TrimPrefix(requestPath, s.adminPath)
}

func (s *Server) adminURL(parts ...string) string {
	var out strings.Builder
	out.WriteString(s.adminPath)
	for _, part := range parts {
		part = strings.Trim(part, "/")
		if part == "" {
			continue
		}
		out.WriteByte('/')
		out.WriteString(part)
	}
	return out.String()
}

func (s *Server) handleAdminAsset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, s.adminURL("asset")+"/")
	if name == "" || name == "." || strings.Contains(name, "\\") || !fs.ValidPath(name) {
		http.NotFound(w, r)
		return
	}
	data, err := webassets.FS.ReadFile("admin/" + name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := assetContentType(name)
	if contentType == "" {
		http.NotFound(w, r)
		return
	}
	serveWebAsset(w, r, name, contentType, data)
}

func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if s.adminAuthenticated(r) {
		http.Redirect(w, r, s.adminPath, http.StatusSeeOther)
		return
	}
	page := adminLoginPage{AdminPath: s.adminPath, User: s.adminUser}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			page.Error = "Could not parse login request."
		} else if constantTimeStringEqual(r.Form.Get("username"), s.adminUser) &&
			constantTimeStringEqual(r.Form.Get("password"), s.adminPassword) {
			s.setAdminSession(w, r)
			http.Redirect(w, r, s.adminPath, http.StatusSeeOther)
			return
		} else {
			page.Error = "Invalid admin credentials."
		}
	}
	s.renderAdminLogin(w, page)
}

func (s *Server) renderAdminLogin(w http.ResponseWriter, page adminLoginPage) {
	var body bytes.Buffer
	if err := adminLoginTemplate.Execute(&body, page); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = body.WriteTo(w)
}

func (s *Server) renderAdmin(w http.ResponseWriter, r *http.Request) {
	active := s.adminActiveSection(r.URL.Path)
	page := adminPage{
		Title:     "LSX Admin",
		AdminPath: s.adminPath,
		Active:    active,
		User:      s.adminUser,
		CSRF:      s.adminCSRF(r),
		Fields:    compat.SyncFields,
		Flash:     s.flash(w, r),
	}
	stats, err := s.AdminStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	page.Stats = stats
	if err := s.loadAdminSection(&page); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var body bytes.Buffer
	if err := adminTemplate.Execute(&body, page); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = body.WriteTo(w)
}

func (s *Server) adminActiveSection(requestPath string) string {
	active := strings.TrimPrefix(requestPath, s.adminPath+"/")
	if active == "" || active == s.adminPath {
		return "submissions"
	}
	return active
}

func (s *Server) loadAdminSection(page *adminPage) error {
	switch page.Active {
	case "submissions":
		submissions, err := s.adminSubmissions(50)
		page.Submissions = submissions
		return err
	case "accounts":
		accounts, err := s.AdminAccounts(50)
		page.Accounts = accounts
		return err
	case "events":
		events, err := s.RecentEvents(80)
		page.Events = adminEvents(events)
		return err
	default:
		return fmt.Errorf("unknown admin section %q", page.Active)
	}
}

func adminEvents(events []Event) []adminEvent {
	out := make([]adminEvent, 0, len(events))
	for _, ev := range events {
		item := adminEvent{
			Event: ev,
			Route: eventpath.Route(ev.Path),
			Raw:   ev.Path,
		}
		values := eventpath.Values(ev.Path)
		if len(values) > 0 {
			switch ev.Kind {
			case "detail":
				item.Summary = detailFields(values)
			case "sync", "sync_rejected", "sync_error":
				item.Summary = []detailField{
					{Label: "Company", Value: values.Get(compat.FieldCompanyName)},
					{Label: "CEO", Value: values.Get(compat.FieldCEOName)},
					{Label: "User", Value: values.Get(compat.FieldUsername)},
					{Label: "Lifespan", Value: values.Get(compat.FieldLifespan)},
					{Label: "Stands", Value: values.Get(compat.FieldStands)},
					{Label: "Cash", Value: values.Get(compat.FieldCashAssets)},
					{Label: "Revenues", Value: values.Get(compat.FieldRevenues)},
					{Label: "Checksum", Value: values.Get(compat.FieldChecksumClient)},
				}
			case "account":
				item.Summary = []detailField{{Label: "Username", Value: values.Get(compat.FieldUsername)}}
			}
			item.Summary = compactDetailFields(item.Summary)
		}
		out = append(out, item)
	}
	return out
}

func compactDetailFields(fields []detailField) []detailField {
	out := make([]detailField, 0, len(fields))
	for _, field := range fields {
		if strings.TrimSpace(field.Value) != "" {
			out = append(out, field)
		}
	}
	return out
}

func (s *Server) handleAdminAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, s.adminPath, http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !s.validAdminCSRF(r, r.Form.Get("csrf")) {
		http.Error(w, "bad csrf token", http.StatusForbidden)
		return
	}
	action := r.Form.Get("action")
	back := s.safeAdminBack(strutil.FirstNonEmpty(r.Form.Get("back"), s.adminPath))
	var err error
	var message string
	switch action {
	case "seed":
		var inserted int
		inserted, err = s.SeedDemoData()
		message = fmt.Sprintf("Seeded %d demo row(s).", inserted)
	case "update_submission":
		var id int64
		id, err = strconv.ParseInt(r.Form.Get("id"), 10, 64)
		if err == nil {
			fields := make(map[string]string, len(compat.SyncFields))
			for _, name := range compat.SyncFields {
				fields[name] = r.Form.Get(name)
			}
			err = s.UpdateSubmissionFields(id, fields)
		}
		message = "Updated submission."
	case "delete_submission":
		err = s.DeleteSubmission(formID(r))
		message = "Deleted submission."
	case "delete_account":
		err = s.DeleteAccount(formID(r))
		message = "Deleted account request."
	case "delete_event":
		err = s.DeleteEvent(formID(r))
		message = "Deleted request log entry."
	case "clear_events":
		err = s.ClearEvents()
		message = "Cleared request logs."
	default:
		err = fmt.Errorf("unknown admin action %q", action)
	}
	if err != nil {
		message = "Error: " + err.Error()
	}
	s.setFlash(w, message)
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (s *Server) safeAdminBack(back string) string {
	if back == s.adminPath || strings.HasPrefix(back, s.adminPath+"/") {
		return back
	}
	return s.adminPath
}

func (s *Server) adminSubmissions(limit int) ([]adminSubmission, error) {
	subs, err := s.AdminSubmissions(limit)
	if err != nil {
		return nil, err
	}
	out := make([]adminSubmission, 0, len(subs))
	for _, sub := range subs {
		out = append(out, adminSubmission{
			ID:       sub.ID,
			When:     sub.ReceivedAt.Local().Format("2006-01-02 15:04:05"),
			Remote:   sub.RemoteAddr,
			Host:     sub.Host,
			Valid:    sub.ChecksumValid,
			Computed: sub.ChecksumComputed,
			Client:   sub.ChecksumClient,
			Fields:   sub.Fields,
			Row:      rowFromSubmission(sub),
		})
	}
	return out, nil
}

func formID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.Form.Get("id"), 10, 64)
	return id
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s.adminAuthenticated(r) {
		return true
	}
	http.Redirect(w, r, s.adminURL("login"), http.StatusSeeOther)
	return false
}

func (s *Server) adminAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie(adminCookieName)
	if err != nil {
		return false
	}
	user, expires, sig, ok := parseSessionCookie(cookie.Value)
	if !ok || user != s.adminUser || time.Now().Unix() > expires {
		return false
	}
	return hmac.Equal([]byte(sig), []byte(s.signSession(user, expires)))
}

func (s *Server) setAdminSession(w http.ResponseWriter, r *http.Request) {
	expires := time.Now().Add(12 * time.Hour).Unix()
	value := s.sessionValue(s.adminUser, expires)
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookieName,
		Value:    value,
		Path:     s.adminPath,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(expires, 0),
	})
}

func (s *Server) clearAdminSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookieName,
		Value:    "",
		Path:     s.adminPath,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (s *Server) sessionValue(user string, expires int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(user)) + "|" + strconv.FormatInt(expires, 10) + "|" + s.signSession(user, expires)
}

func (s *Server) signSession(user string, expires int64) string {
	mac := hmac.New(sha256.New, s.sessionSecret)
	_, _ = fmt.Fprintf(mac, "%s|%d", user, expires)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func parseSessionCookie(value string) (string, int64, string, bool) {
	userPart, rest, ok := strings.Cut(value, "|")
	if !ok {
		return "", 0, "", false
	}
	expiresPart, sig, ok := strings.Cut(rest, "|")
	if !ok || strings.Contains(sig, "|") {
		return "", 0, "", false
	}
	userBytes, err := base64.RawURLEncoding.DecodeString(userPart)
	if err != nil {
		return "", 0, "", false
	}
	expires, err := strconv.ParseInt(expiresPart, 10, 64)
	if err != nil {
		return "", 0, "", false
	}
	return string(userBytes), expires, sig, true
}

func (s *Server) adminCSRF(r *http.Request) string {
	cookie, err := r.Cookie(adminCookieName)
	if err != nil {
		return ""
	}
	mac := hmac.New(sha256.New, s.sessionSecret)
	_, _ = mac.Write([]byte("csrf|" + cookie.Value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Server) validAdminCSRF(r *http.Request, token string) bool {
	want := s.adminCSRF(r)
	return want != "" && hmac.Equal([]byte(token), []byte(want))
}

func (s *Server) setFlash(w http.ResponseWriter, message string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "lsx_admin_flash",
		Value:    url.QueryEscape(message),
		Path:     s.adminPath,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30,
	})
}

func (s *Server) flash(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie("lsx_admin_flash")
	if err != nil {
		return ""
	}
	http.SetCookie(w, &http.Cookie{Name: "lsx_admin_flash", Path: s.adminPath, MaxAge: -1})
	value, _ := url.QueryUnescape(cookie.Value)
	return value
}

var adminFuncMap = template.FuncMap{
	"field": func(fields map[string]string, name string) string { return fields[name] },
	"money": lsxvalue.FormatCents,
	"time":  adminTime,
}

var adminTemplates = template.Must(template.New("").
	Funcs(adminFuncMap).
	ParseFS(adminTemplateFS(), "admin_templates/*.html.tmpl"))
var adminLoginTemplate = adminTemplates.Lookup("login.html.tmpl")
var adminTemplate = adminTemplates.Lookup("admin.html.tmpl")

func adminTemplateFS() fs.FS {
	sub, err := fs.Sub(adminFS, ".")
	if err != nil {
		panic(err)
	}
	return sub
}

func adminTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04:05")
}
