package lsx

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	webassets "lt2_reverse/lsx_server_go/assets"
	"lt2_reverse/lsx_server_go/internal/eventpath"
	"lt2_reverse/lsx_server_go/internal/lsx/compat"
	"lt2_reverse/lsx_server_go/internal/lsxvalue"
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

type projectPage struct {
	CSS          template.CSS
	Title        string
	Heading      string
	BoardActive  bool
	HelpActive   bool
	DocsActive   bool
	Findings     bool
	Docs         bool
	FindingsHTML template.HTML
	BoardRows    []projectLeaderboardRow
	BoardTotal   int
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
	s.handleProjectShell(w, r, projectPage{
		Title:       "Lemonade Tycoon 2 LSX Revival",
		Heading:     "LSX",
		BoardActive: true,
		BoardRows:   newProjectLeaderboardRows(rows, 3),
		BoardTotal:  len(rows),
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
	findingsHTML, err := webassets.FS.ReadFile("project/findings/content.html.tmpl")
	if err != nil {
		s.emit(r, "project_error", http.StatusInternalServerError, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.handleProjectShellTemplate(w, r, "project/findings/index.html.tmpl", projectPage{
		Title:        "LSX Reverse Engineering Findings",
		Heading:      "Findings",
		HelpActive:   true,
		Findings:     true,
		FindingsHTML: template.HTML(findingsHTML),
	})
}

func (s *Server) handleDocsPage(w http.ResponseWriter, r *http.Request) {
	s.handleProjectShellTemplate(w, r, "project/docs/index.html.tmpl", projectPage{
		Title:      "LSX API Documentation",
		Heading:    "Docs",
		DocsActive: true,
		Docs:       true,
	})
}

func (s *Server) handleProjectShell(w http.ResponseWriter, r *http.Request, page projectPage) {
	s.handleProjectShellTemplate(w, r, "project.html.tmpl", page)
}

func (s *Server) handleProjectShellTemplate(w http.ResponseWriter, r *http.Request, templateName string, page projectPage) {
	css, err := projectCSS()
	if err != nil {
		s.emit(r, "project_error", http.StatusInternalServerError, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var body bytes.Buffer
	page.CSS = template.CSS(css)
	if err := executeProjectTemplate(&body, templateName, page); err != nil {
		s.emit(r, "project_error", http.StatusInternalServerError, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	switch {
	case page.Findings:
		s.emit(r, "findings", http.StatusOK, "rendered findings page")
	case page.Docs:
		s.emit(r, "docs", http.StatusOK, "rendered docs page")
	default:
		s.emit(r, "project", http.StatusOK, "rendered project page")
	}
	_, _ = body.WriteTo(w)
}

func executeProjectTemplate(body *bytes.Buffer, templateName string, page projectPage) error {
	if templateName == "project.html.tmpl" {
		return templates.ExecuteTemplate(body, templateName, page)
	}
	data, err := webassets.FS.ReadFile(templateName)
	if err != nil {
		return err
	}
	tmpl, err := template.New(templateName).Parse(string(data))
	if err != nil {
		return err
	}
	return tmpl.Execute(body, page)
}

func (s *Server) handleProjectAsset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/project/asset/")
	assetPath, ok := projectAssetPath(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	data, err := webassets.FS.ReadFile(assetPath)
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

	if !strings.HasPrefix(name, "findings/vendor/shiki/") {
		return "", false
	}

	clean := path.Clean(name)
	if clean != name || strings.Contains(name, "..") {
		return "", false
	}

	return "project/" + name, true
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

func (s *Server) handleProjectCSS(w http.ResponseWriter, r *http.Request) {
	data, err := projectCSS()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "project.css", time.Time{}, bytes.NewReader(data))
}

var projectCSSFiles = []string{
	"project/css/base.css",
	"project/css/shell.css",
	"project/css/home.css",
	"project/css/findings.css",
	"project/css/docs.css",
	"project/css/responsive.css",
}

func projectCSS() ([]byte, error) {
	var css bytes.Buffer
	for _, name := range projectCSSFiles {
		data, err := webassets.FS.ReadFile(name)
		if err != nil {
			return nil, err
		}
		_, _ = css.Write(data)
	}
	return css.Bytes(), nil
}

//go:embed templates/*.html.tmpl
var templateFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.html.tmpl"))

var leaderboardTemplate = templates.Lookup("leaderboard.html.tmpl")

var detailTemplate = templates.Lookup("detail.html.tmpl")
