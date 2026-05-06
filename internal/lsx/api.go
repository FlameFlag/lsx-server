package lsx

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"lt2_reverse/lsx_server_go/internal/eventpath"
)

const (
	apiDefaultPageSize = 10
	apiMaxPageSize     = 100
)

type apiProblem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type apiLeaderboardResponse struct {
	Data       []apiLeaderboardRow `json:"data"`
	Pagination apiPagination       `json:"pagination"`
	Filters    apiFilters          `json:"filters"`
	Sort       string              `json:"sort"`
}

type apiPagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

type apiFilters struct {
	GameMode string `json:"gamemode,omitempty"`
	GameGoal string `json:"gamegoal,omitempty"`
	Username string `json:"username,omitempty"`
}

type apiLeaderboardRow struct {
	Rank          int    `json:"rank"`
	Company       string `json:"company"`
	CEO           string `json:"ceo"`
	Mode          string `json:"mode"`
	Goal          string `json:"goal"`
	Title         string `json:"title"`
	Lifespan      int64  `json:"lifespan"`
	MarketCents   int64  `json:"market_cents"`
	RevenueCents  int64  `json:"revenue_cents"`
	RetainedCents int64  `json:"retained_cents"`
	Stands        int64  `json:"stands"`
	CupsSold      int64  `json:"cups_sold"`
	Username      string `json:"username,omitempty"`
	DateScalar    string `json:"date_scalar,omitempty"`
	Source        string `json:"source"`
	ChecksumValid bool   `json:"checksum_valid"`
}

func (s *Server) handleAPILeaderboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		s.emit(r, "api_error", http.StatusMethodNotAllowed, "method not allowed")
		writeAPIProblem(w, r, http.StatusMethodNotAllowed, "Method Not Allowed", "use GET for this endpoint")
		return
	}

	query, err := parseAPILeaderboardQuery(r.URL.Query())
	if err != nil {
		s.emit(r, "api_error", http.StatusBadRequest, err.Error())
		writeAPIProblem(w, r, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}

	rows, err := s.leaderboardRows()
	if err != nil {
		s.emit(r, "api_error", http.StatusInternalServerError, err.Error())
		writeAPIProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "failed to load leaderboard")
		return
	}

	rows = filterRows(rows, query.filters)
	sortRows(rows, query.sort)

	totalItems := len(rows)
	totalPages := 1
	if totalItems > 0 {
		totalPages = (totalItems + query.pageSize - 1) / query.pageSize
	}
	if query.page > totalPages {
		query.page = totalPages
	}

	start := (query.page - 1) * query.pageSize
	end := start + query.pageSize
	if start > totalItems {
		start = totalItems
	}
	if end > totalItems {
		end = totalItems
	}

	data := make([]apiLeaderboardRow, 0, end-start)
	for i, row := range rows[start:end] {
		data = append(data, apiRowFromLeaderboard(row, start+i+1))
	}

	resp := apiLeaderboardResponse{
		Data: data,
		Pagination: apiPagination{
			Page:       query.page,
			PageSize:   query.pageSize,
			TotalItems: totalItems,
			TotalPages: totalPages,
		},
		Filters: apiFilters{
			GameMode: query.filters.Get("gamemode"),
			GameGoal: query.filters.Get("gamegoal"),
			Username: query.filters.Get("username"),
		},
		Sort: query.sort,
	}
	s.emit(r, "api_leaderboard", http.StatusOK, fmt.Sprintf("returned %d leaderboard rows", len(data)))
	writeJSON(w, r, http.StatusOK, resp)
}

type apiLeaderboardQuery struct {
	page     int
	pageSize int
	sort     string
	filters  url.Values
}

func parseAPILeaderboardQuery(q url.Values) (apiLeaderboardQuery, error) {
	page, err := parseBoundedPositiveInt(q.Get("page"), 1, 1, math.MaxInt)
	if err != nil {
		return apiLeaderboardQuery{}, errors.New("page must be a positive integer")
	}
	pageSize, err := parseBoundedPositiveInt(q.Get("page_size"), apiDefaultPageSize, 1, apiMaxPageSize)
	if err != nil {
		return apiLeaderboardQuery{}, fmt.Errorf("page_size must be an integer from 1 to %d", apiMaxPageSize)
	}

	sortParam := q.Get("sort")
	if sortParam == "" {
		sortParam = "market"
	}
	legacySort, ok := mapAPISort(sortParam)
	if !ok {
		return apiLeaderboardQuery{}, errors.New("sort must be one of market, company, ceo, lifespan")
	}

	filters := make(url.Values)
	for _, key := range []string{"gamemode", "gamegoal", "username"} {
		if value := q.Get(key); value != "" {
			filters.Set(key, value)
		}
	}
	return apiLeaderboardQuery{
		page:     page,
		pageSize: pageSize,
		sort:     sortParam,
		filters:  withLegacySort(filters, legacySort),
	}, nil
}

func withLegacySort(q url.Values, sortParam string) url.Values {
	next := eventpath.CloneValues(q)
	next.Set("sort", sortParam)
	return next
}

func parseBoundedPositiveInt(raw string, defaultValue int, minValue int, maxValue int) (int, error) {
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minValue || value > maxValue {
		return 0, errors.New("invalid positive integer")
	}
	return value, nil
}

func mapAPISort(sortParam string) (string, bool) {
	switch sortParam {
	case "market":
		return "4", true
	case "company":
		return "1", true
	case "ceo":
		return "2", true
	case "lifespan":
		return "3", true
	default:
		return "", false
	}
}

func apiRowFromLeaderboard(row LeaderboardRow, rank int) apiLeaderboardRow {
	return apiLeaderboardRow{
		Rank:          rank,
		Company:       row.Company,
		CEO:           row.CEO,
		Mode:          row.Mode,
		Goal:          row.Goal,
		Title:         row.Title,
		Lifespan:      row.Lifespan,
		MarketCents:   row.MarketCents,
		RevenueCents:  row.RevenueCents,
		RetainedCents: row.RetainedCents,
		Stands:        row.Stands,
		CupsSold:      row.CupsSold,
		Username:      row.Username,
		DateScalar:    row.DateScalar,
		Source:        row.Source,
		ChecksumValid: row.ChecksumValid,
	}
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func writeAPIProblem(w http.ResponseWriter, r *http.Request, status int, title string, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(apiProblem{
		Type:   "about:blank",
		Title:  title,
		Status: status,
		Detail: detail,
	})
}
