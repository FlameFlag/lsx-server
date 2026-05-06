package lsx

import (
	"cmp"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"lt2_reverse/lsx_server_go/internal/eventpath"
	"lt2_reverse/lsx_server_go/internal/lsx/compat"
	"lt2_reverse/lsx_server_go/internal/lsxvalue"
	"lt2_reverse/lsx_server_go/internal/strutil"
)

const pageSize = 10

func (s *Server) leaderboardRows() ([]LeaderboardRow, error) {
	subs, err := s.loadSubmissions()
	if err != nil {
		return nil, err
	}

	rows := make([]LeaderboardRow, 0, len(subs))
	for _, sub := range subs {
		rows = append(rows, rowFromSubmission(sub))
	}
	return rows, nil
}

func rowFromSubmission(sub Submission) LeaderboardRow {
	f := sub.Fields
	cash := lsxvalue.ParseInt(f[compat.FieldCashAssets])
	stock := lsxvalue.ParseInt(f[compat.FieldStockAssets])
	standsAssets := lsxvalue.ParseInt(f[compat.FieldStandsAssets])
	upgrades := lsxvalue.ParseInt(f[compat.FieldUpgradesAssets])
	retained := lsxvalue.ParseInt(f[compat.FieldRetainedEarnings])
	revenues := lsxvalue.ParseInt(f[compat.FieldRevenues])
	market := cash + stock + standsAssets + upgrades
	if market == 0 {
		market = retained
	}

	return LeaderboardRow{
		Company:       strutil.FirstNonEmpty(f[compat.FieldCompanyName], "(unnamed company)"),
		CEO:           strutil.FirstNonEmpty(f[compat.FieldCEOName], "(unknown)"),
		Mode:          modeLabel(lsxvalue.ParseInt(f[compat.FieldGameMode])),
		Goal:          goalLabel(lsxvalue.ParseInt(f[compat.FieldGameGoal])),
		Title:         titleForMarket(market),
		Lifespan:      lsxvalue.ParseInt(f[compat.FieldLifespan]),
		GameMode:      lsxvalue.ParseInt(f[compat.FieldGameMode]),
		GameGoal:      lsxvalue.ParseInt(f[compat.FieldGameGoal]),
		MarketCents:   market,
		RevenueCents:  revenues,
		RetainedCents: retained,
		Stands:        lsxvalue.ParseInt(f[compat.FieldStands]),
		CupsSold:      lsxvalue.ParseInt(f[compat.FieldCupsSold]),
		CashCents:     cash,
		StockCents:    stock,
		StandCents:    standsAssets,
		UpgradeCents:  upgrades,
		Username:      f[compat.FieldUsername],
		DateScalar:    f[compat.FieldGameStartingDate],
		Source:        fmt.Sprintf("local #%d", sub.ID),
		ChecksumValid: sub.ChecksumValid,
	}
}

func filterRows(rows []LeaderboardRow, q url.Values) []LeaderboardRow {
	gamemode := q.Get("gamemode")
	gamegoal := q.Get("gamegoal")
	username := q.Get("username")
	out := rows[:0]
	for _, row := range rows {
		if gamemode != "" && gamemode != "0" && strconv.FormatInt(row.GameMode, 10) != gamemode {
			continue
		}
		if gamegoal != "" && gamegoal != "0" && strconv.FormatInt(row.GameGoal, 10) != gamegoal {
			continue
		}
		if username != "" && username != "0" && !strings.EqualFold(row.Username, username) {
			continue
		}
		out = append(out, row)
	}
	return out
}

func sortRows(rows []LeaderboardRow, sortParam string) {
	slices.SortStableFunc(rows, func(a, b LeaderboardRow) int {
		switch sortParam {
		case "1":
			return cmp.Compare(strings.ToLower(a.Company), strings.ToLower(b.Company))
		case "2":
			return cmp.Compare(strings.ToLower(a.CEO), strings.ToLower(b.CEO))
		case "3":
			if byLifespan := cmp.Compare(b.Lifespan, a.Lifespan); byLifespan != 0 {
				return byLifespan
			}
			return cmp.Compare(b.MarketCents, a.MarketCents)
		case "4", "14":
			return cmp.Compare(b.MarketCents, a.MarketCents)
		default:
			return cmp.Compare(b.MarketCents, a.MarketCents)
		}
	})
}

func paginateRows(rows []LeaderboardRow, q url.Values) ([]LeaderboardRow, int, int) {
	page := lsxvalue.ParsePositive(q.Get("pagenum"), 1)
	pages := int64(1)
	if len(rows) > 0 {
		pages = int64((len(rows) + pageSize - 1) / pageSize)
	}
	if page > pages {
		page = pages
	}

	start := int((page - 1) * pageSize)
	end := start + pageSize
	if start > len(rows) {
		start = len(rows)
	}
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end], int(page), int(pages)
}

func detailURL(row LeaderboardRow, rank int, totalRows int) string {
	if row.Detail != nil {
		q := make(url.Values)
		for k, v := range row.Detail {
			q.Set(k, v)
		}
		q.Set("d5", strconv.Itoa(rank))
		if q.Get("d6") == "" {
			q.Set("d6", strconv.Itoa(totalRows))
		}
		return "lsx2_detail.php?" + q.Encode()
	}
	q := url.Values{}
	q.Set("d1", row.Company)
	q.Set("d2", row.CEO)
	q.Set("d3", row.Mode)
	q.Set("d4", row.Goal)
	q.Set("d5", strconv.Itoa(rank))
	q.Set("d6", strconv.Itoa(totalRows))
	q.Set("d7", row.Title)
	q.Set("d8", strconv.FormatInt(row.Lifespan, 10))
	q.Set("d9", strconv.FormatInt(row.Stands, 10))
	q.Set("d10", strconv.FormatInt(row.CupsSold, 10))
	q.Set("d11", lsxvalue.FormatCents(row.MarketCents))
	q.Set("d12", lsxvalue.FormatCents(row.RevenueCents))
	q.Set("d13", lsxvalue.FormatCents(row.RetainedCents))
	q.Set("d14", "0")
	q.Set("d15", lsxvalue.FormatCents(row.CashCents))
	q.Set("d16", lsxvalue.FormatCents(row.StockCents))
	q.Set("d17", lsxvalue.FormatCents(row.StandCents))
	q.Set("d18", lsxvalue.FormatCents(row.UpgradeCents))
	return "lsx2_detail.php?" + q.Encode()
}

func linkWith(q url.Values, pairs ...string) string {
	next := eventpath.CloneValues(q)
	for i := 0; i+1 < len(pairs); i += 2 {
		next.Set(pairs[i], pairs[i+1])
	}
	return "?" + next.Encode()
}

func modeLabel(mode int64) string {
	switch mode {
	case 1:
		return "Career"
	case 2:
		return "Challenge"
	default:
		return "Career"
	}
}

func goalLabel(goal int64) string {
	switch goal {
	case 1:
		return "Sky is the limit!"
	case 2:
		return "Cash challenge"
	case 3:
		return "Expansion"
	default:
		return "Sky is the limit!"
	}
}

func titleForMarket(cents int64) string {
	switch {
	case cents >= 100_000_000_00:
		return "Tycoon"
	case cents >= 25_000_000_00:
		return "Magnate"
	case cents >= 5_000_000_00:
		return "Entrepreneur"
	default:
		return "Vendor"
	}
}
