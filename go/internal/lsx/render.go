package lsx

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lt2_reverse/lsx_server_go/internal/eventpath"
	"lt2_reverse/lsx_server_go/internal/lsx/compat"
	"lt2_reverse/lsx_server_go/internal/lsxvalue"
	webassets "lt2_reverse/lsx_server_go/web"
)

type leaderboardHeader struct {
	Class string
	Sort  string
	Label string
	URL   template.URL
}

type leaderboardViewRow struct {
	Rank      int
	RowClass  string
	DetailURL template.URL
	Company   string
	CEO       string
	Lifespan  string
	MarketCap string
}

type leaderboardPage struct {
	Headers []leaderboardHeader
	Rows    []leaderboardViewRow
	Page    int
	Pages   int
	HasPrev bool
	PrevURL template.URL
	HasNext bool
	NextURL template.URL
}

type projectLeaderboardRow struct {
	Rank      int
	Company   string
	MarketCap string
}

type detailField struct {
	Label string
	Value string
}

type svelteProjectPage struct {
	Title      string                  `json:"title"`
	Heading    string                  `json:"heading"`
	BoardRows  []projectLeaderboardRow `json:"boardRows,omitempty"`
	BoardTotal int                     `json:"boardTotal,omitempty"`
	Markdown   string                  `json:"markdown,omitempty"`
	ServerAddr string                  `json:"serverAddr,omitempty"`
}

type svelteAppPage struct {
	Title     string
	Page      string
	BodyClass string
	DataJSON  template.JS
	Styles    []string
	Scripts   []string
}

type svelteManifestEntry struct {
	File string   `json:"file"`
	CSS  []string `json:"css"`
}

func (s *Server) handleProjectPage(w http.ResponseWriter, r *http.Request) {
	rows, err := s.leaderboardRows()
	if err != nil {
		s.emit(r, "project_error", http.StatusInternalServerError, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	query := normalizedLeaderboardQuery(r.URL.Query())
	rows = filterRows(rows, query)
	sortRows(rows, query.Get("sort"))
	s.renderSvelteApp(w, r, "home", "Lemonade Tycoon 2 LSX Revival", "", svelteProjectPage{
		Title:      "Lemonade Tycoon 2 LSX Revival",
		Heading:    "LSX",
		BoardRows:  newProjectLeaderboardRows(rows, 3),
		BoardTotal: len(rows),
	})
}

func newProjectLeaderboardRows(rows []LeaderboardRow, limit int) []projectLeaderboardRow {
	if limit > len(rows) {
		limit = len(rows)
	}
	out := make([]projectLeaderboardRow, 0, limit)
	for i, row := range rows[:limit] {
		out = append(out, projectLeaderboardRow{
			Rank:      i + 1,
			Company:   row.Company,
			MarketCap: lsxvalue.FormatCents(row.MarketCents),
		})
	}
	return out
}

func (s *Server) handleFindingsPage(w http.ResponseWriter, r *http.Request) {
	findingsMarkdown, err := webassets.FS.ReadFile("static/project/findings/content.md")
	if err != nil {
		s.emit(r, "project_error", http.StatusInternalServerError, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderSvelteApp(w, r, "findings", "LSX Reverse Engineering Findings", "findings-page", svelteProjectPage{
		Title:    "LSX Reverse Engineering Findings",
		Heading:  "Findings",
		Markdown: string(findingsMarkdown),
	})
}

func (s *Server) handleDocsPage(w http.ResponseWriter, r *http.Request) {
	s.renderSvelteApp(w, r, "docs", "LSX API Documentation", "docs-page", svelteProjectPage{
		Title:   "LSX API Documentation",
		Heading: "Docs",
	})
}

func (s *Server) handleActivatePage(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if colon := strings.LastIndex(host, ":"); colon >= 0 {
		host = host[:colon]
	}
	s.renderSvelteApp(w, r, "activate", "LT2 Activation Guide", "activate-page", svelteProjectPage{
		Title:      "LT2 Activation Guide",
		Heading:    "Activation Guide",
		ServerAddr: host,
	})
}

func (s *Server) renderSvelteApp(w http.ResponseWriter, r *http.Request, page string, title string, bodyClass string, data any) {
	styles, scripts, err := svelteAppAssets()
	if err != nil {
		s.emit(r, "svelte_error", http.StatusInternalServerError, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	payload, err := json.Marshal(data)
	if err != nil {
		s.emit(r, "svelte_error", http.StatusInternalServerError, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var body bytes.Buffer
	err = svelteAppTemplate.ExecuteTemplate(&body, "app.html.tmpl", svelteAppPage{
		Title:     title,
		Page:      page,
		BodyClass: bodyClass,
		DataJSON:  template.JS(payload),
		Styles:    styles,
		Scripts:   scripts,
	})
	if err != nil {
		s.emit(r, "svelte_error", http.StatusInternalServerError, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	switch page {
	case "findings":
		s.emit(r, "findings", http.StatusOK, "rendered findings page")
	case "docs":
		s.emit(r, "docs", http.StatusOK, "rendered docs page")
	case "activate":
		s.emit(r, "activate", http.StatusOK, "rendered activate page")
	case "admin", "admin-login":
		s.emit(r, "admin", http.StatusOK, "rendered admin page")
	default:
		s.emit(r, "project", http.StatusOK, "rendered project page")
	}
	_, _ = body.WriteTo(w)
}

func svelteAppAssets() ([]string, []string, error) {
	data, err := fs.ReadFile(webassets.DistFS(), "manifest.json")
	if err != nil {
		return nil, nil, err
	}
	var manifest map[string]svelteManifestEntry
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, nil, err
	}
	entry, ok := manifest["src/main.ts"]
	if !ok {
		return nil, nil, os.ErrNotExist
	}
	styles := make([]string, 0, len(entry.CSS))
	for _, css := range entry.CSS {
		styles = append(styles, "/project/asset/svelte/"+css)
	}
	return styles, []string{"/project/asset/svelte/" + entry.File}, nil
}

func (s *Server) handleProjectAsset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/project/asset/")
	assetPath, ok := projectAssetPath(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	data, err := readProjectAsset(assetPath)
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

func (s *Server) handleOpenAPIYAML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := s.readOpenAPIYAML()
	if err != nil {
		s.emit(r, "openapi_error", http.StatusNotFound, err.Error())
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "openapi.yaml", time.Time{}, bytes.NewReader(data))
}

func (s *Server) readOpenAPIYAML() ([]byte, error) {
	if len(s.openAPIYAML) > 0 {
		return s.openAPIYAML, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for {
		name := filepath.Join(dir, "openapi.yaml")
		data, err := os.ReadFile(name)
		if err == nil {
			return data, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, err
		}
		dir = parent
	}
}

func projectAssetPath(name string) (string, bool) {
	if assetPath, ok := compat.ProjectAssetPaths[name]; ok {
		return assetPath, true
	}

	if !strings.HasPrefix(name, "svelte/") {
		return "", false
	}

	clean := path.Clean(name)
	if clean != name || strings.Contains(name, "..") {
		return "", false
	}

	return strings.TrimPrefix(name, "svelte/"), true
}

func readProjectAsset(assetPath string) ([]byte, error) {
	if strings.HasPrefix(assetPath, "static/") {
		return webassets.FS.ReadFile(assetPath)
	}
	return fs.ReadFile(webassets.DistFS(), assetPath)
}

func (s *Server) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	rows, err := s.leaderboardRows()
	if err != nil {
		s.emit(r, "leaderboard_error", http.StatusInternalServerError, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	query := normalizedLeaderboardQuery(r.URL.Query())
	rows = filterRows(rows, query)
	sortRows(rows, query.Get("sort"))
	pageRows, page, pages := paginateRows(rows, query)

	var body bytes.Buffer
	if err := leaderboardTemplate.Execute(&body, newLeaderboardPage(pageRows, page, pages, len(rows), query)); err != nil {
		s.emit(r, "leaderboard_error", http.StatusInternalServerError, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=ISO-8859-1")
	w.WriteHeader(http.StatusOK)
	s.emit(r, "leaderboard", http.StatusOK, "rendered leaderboard")
	_, _ = body.WriteTo(w)
}

func newLeaderboardPage(rows []LeaderboardRow, page int, pages int, totalRows int, q url.Values) leaderboardPage {
	headers := make([]leaderboardHeader, 0, len(compat.LeaderboardColumns))
	for _, column := range compat.LeaderboardColumns {
		headers = append(headers, leaderboardHeader{
			Class: column.Class,
			Sort:  column.Sort,
			Label: column.Label,
			URL:   template.URL(linkWith(q, "sort", column.Sort, "pagenum", "1")),
		})
	}

	offset := (page - 1) * pageSize
	viewRows := make([]leaderboardViewRow, 0, len(rows))
	for i, row := range rows {
		rank := offset + i + 1
		rowClass := ""
		if i%2 == 1 {
			rowClass = "alt"
		}
		viewRows = append(viewRows, leaderboardViewRow{
			Rank:      rank,
			RowClass:  rowClass,
			DetailURL: template.URL(detailURL(row, rank, totalRows)),
			Company:   row.Company,
			CEO:       row.CEO,
			Lifespan:  lsxvalue.FormatInt(row.Lifespan),
			MarketCap: lsxvalue.FormatCents(row.MarketCents),
		})
	}

	pageData := leaderboardPage{
		Headers: headers,
		Rows:    viewRows,
		Page:    page,
		Pages:   pages,
		HasPrev: page > 1,
		HasNext: page < pages,
	}
	if pageData.HasPrev {
		pageData.PrevURL = template.URL(linkWith(q, "pagenum", strconv.Itoa(page-1)))
	}
	if pageData.HasNext {
		pageData.NextURL = template.URL(linkWith(q, "pagenum", strconv.Itoa(page+1)))
	}
	return pageData
}

func normalizedLeaderboardQuery(q url.Values) url.Values {
	next := eventpath.CloneValues(q)
	for _, def := range compat.LeaderboardQueryDefaults {
		if next.Get(def.Name) == "" {
			next.Set(def.Name, def.Value)
		}
	}
	return next
}

func (s *Server) handleDetail(w http.ResponseWriter, r *http.Request) {
	var body bytes.Buffer
	if err := detailTemplate.Execute(&body, detailFields(r.URL.Query())); err != nil {
		s.emit(r, "detail_error", http.StatusInternalServerError, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=ISO-8859-1")
	w.WriteHeader(http.StatusOK)
	s.emit(r, "detail", http.StatusOK, "rendered detail panel")
	_, _ = body.WriteTo(w)
}

func detailFields(q url.Values) []detailField {
	fields := make([]detailField, 0, len(compat.DetailFieldSpecs))
	for _, spec := range compat.DetailFieldSpecs {
		value := q.Get(spec.Param)
		if value == "" {
			continue
		}
		fields = append(fields, detailField{Label: spec.Label, Value: value})
	}
	return fields
}

func (s *Server) handleBlankDetail(w http.ResponseWriter, r *http.Request) {
	var body bytes.Buffer
	if err := templates.ExecuteTemplate(&body, "blank_detail.html.tmpl", nil); err != nil {
		s.emit(r, "detail_error", http.StatusInternalServerError, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=ISO-8859-1")
	w.WriteHeader(http.StatusOK)
	s.emit(r, "detail", http.StatusOK, "rendered blank detail panel")
	_, _ = body.WriteTo(w)
}

func (s *Server) handleStaticCSS(w http.ResponseWriter, r *http.Request, name string) {
	data, err := webassets.FS.ReadFile(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}

//go:embed templates/*.html.tmpl
var templateFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.html.tmpl"))
var svelteAppTemplate = template.Must(template.ParseFS(webassets.FS, "templates/app.html.tmpl"))

var leaderboardTemplate = templates.Lookup("leaderboard.html.tmpl")

var detailTemplate = templates.Lookup("detail.html.tmpl")
