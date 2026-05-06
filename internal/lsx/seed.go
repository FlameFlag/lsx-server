package lsx

import (
	"net/url"
	"strconv"
	"time"

	"lt2_reverse/lsx_server_go/internal/lsx/compat"
	"lt2_reverse/lsx_server_go/internal/lsxvalue"

	sq "github.com/Masterminds/squirrel"
)

var waybackSeedRows = []LeaderboardRow{
	seedRow("tm", "tyler", "Career", "Sky is the limit!", "Tycoon", 2, "3", "-431520288", "476,884,102.21", "17,059,278.48", "-4,315,202.88", "-395", "0.00", "-17,259,708.72", "-4,315,022.88", "-4,315,102.88", "78880"),
	seedRow("DarKMon", "DarKAvengeR", "Career", "Sky is the limit!", "Tycoon", 3, "2", "100", "456,589,836.48", "21,473,904.52", "185.00", "11607516", "21,473,656.77", "2.75", "180.00", "625.00", "78880"),
	seedRow("Niunia", "Giedrius", "Career", "Sky is the limit!", "Tycoon", 5, "2", "1556291054", "162,157,049.09", "7,478,601.89", "15,563,053.36", "48", "367.74", "19,302,417.59", "15,563,089.76", "15,562,959.76", "78880"),
	seedRow("LEMONY GOODNESS", "LIAM PATRICK", "Career", "Sky is the limit!", "Tycoon", 11, "1", "-1977890271", "82,214,087.82", "16,389,741.61", "-19,778,357.73", "-83", "82.91", "1.60", "500.00", "100.00", "15206"),
}

func seedRow(company, ceo, mode, goal, title string, lifespan int64, stands, cupsSold, market, revenues, retained, percent, cash, stock, standsAssets, upgrades, totalEntries string) LeaderboardRow {
	row := LeaderboardRow{
		Company:       company,
		CEO:           ceo,
		Mode:          mode,
		Goal:          goal,
		Title:         title,
		Lifespan:      lifespan,
		GameMode:      1,
		GameGoal:      goalCode(goal),
		MarketCents:   lsxvalue.ParseInt(market),
		RevenueCents:  lsxvalue.ParseInt(revenues),
		RetainedCents: lsxvalue.ParseInt(retained),
		Stands:        lsxvalue.ParseInt(stands),
		CupsSold:      lsxvalue.ParseInt(cupsSold),
		CashCents:     lsxvalue.ParseInt(cash),
		StockCents:    lsxvalue.ParseInt(stock),
		StandCents:    lsxvalue.ParseInt(standsAssets),
		UpgradeCents:  lsxvalue.ParseInt(upgrades),
		DateScalar:    "0",
		Source:        "Wayback lsx2.php capture",
		ChecksumValid: true,
	}
	row.Detail = map[string]string{
		"d1":  company,
		"d2":  ceo,
		"d3":  mode,
		"d4":  goal,
		"d6":  totalEntries,
		"d7":  title,
		"d8":  strconv.FormatInt(lifespan, 10),
		"d9":  stands,
		"d10": cupsSold,
		"d11": market,
		"d12": revenues,
		"d13": retained,
		"d14": percent,
		"d15": cash,
		"d16": stock,
		"d17": standsAssets,
		"d18": upgrades,
	}
	return row
}

func goalCode(goal string) int64 {
	switch goal {
	case "Cash challenge":
		return 2
	case "Expansion":
		return 3
	default:
		return 1
	}
}

func (s *Server) SeedDemoData() (int, error) {
	inserted := 0
	for i, row := range waybackSeedRows {
		fields := seedFields(row, i)
		exists, err := s.seedSubmissionExists(fields)
		if err != nil {
			return inserted, err
		}
		if exists {
			continue
		}
		checksum := compat.ComputeChecksum(fields)
		fields["checksumclient"] = strconv.FormatInt(int64(checksum), 10)
		if err := s.appendSubmission(Submission{
			ReceivedAt:       time.Now().UTC().Add(time.Duration(-len(waybackSeedRows)+i) * time.Minute),
			RemoteAddr:       "seed",
			Host:             "seed.local",
			RawQuery:         encodeSeedQuery(fields),
			Fields:           fields,
			ChecksumClient:   checksum,
			ChecksumComputed: checksum,
			ChecksumPresent:  true,
			ChecksumValid:    true,
		}); err != nil {
			return inserted, err
		}
		inserted++
	}
	return inserted, nil
}

func (s *Server) seedSubmissionExists(fields map[string]string) (bool, error) {
	var count int
	err := s.sql().
		Select("COUNT(*)").
		From("submissions").
		Where(sq.Eq{
			compat.FieldCompanyName:      fields[compat.FieldCompanyName],
			compat.FieldCEOName:          fields[compat.FieldCEOName],
			compat.FieldUsername:         fields[compat.FieldUsername],
			compat.FieldGameStartingDate: fields[compat.FieldGameStartingDate],
		}).
		QueryRow().
		Scan(&count)
	return count > 0, err
}

func seedFields(row LeaderboardRow, index int) map[string]string {
	fields := map[string]string{
		compat.FieldGame:             "lemonade2",
		compat.FieldUsername:         "wayback_seed_" + strconv.Itoa(index+1),
		compat.FieldPassword:         "",
		compat.FieldCompanyName:      row.Company,
		compat.FieldCEOName:          row.CEO,
		compat.FieldGameMode:         strconv.FormatInt(row.GameMode, 10),
		compat.FieldGameGoal:         strconv.FormatInt(row.GameGoal, 10),
		compat.FieldGameStartingDate: row.DateScalar,
		compat.FieldLifespan:         strconv.FormatInt(row.Lifespan, 10),
		compat.FieldStands:           strconv.FormatInt(row.Stands, 10),
		compat.FieldCupsSold:         strconv.FormatInt(row.CupsSold, 10),
		compat.FieldCashAssets:       strconv.FormatInt(row.CashCents, 10),
		compat.FieldStockAssets:      strconv.FormatInt(row.StockCents, 10),
		compat.FieldStandsAssets:     strconv.FormatInt(row.StandCents, 10),
		compat.FieldUpgradesAssets:   strconv.FormatInt(row.UpgradeCents, 10),
		compat.FieldRetainedEarnings: strconv.FormatInt(row.RetainedCents, 10),
		compat.FieldRevenues:         strconv.FormatInt(row.RevenueCents, 10),
		compat.FieldChecksumClient:   "",
	}
	return fields
}

func encodeSeedQuery(fields map[string]string) string {
	values := url.Values{}
	for _, name := range compat.SyncFields {
		values.Set(name, fields[name])
	}
	return values.Encode()
}
