package compat

type QueryDefault struct {
	Name  string
	Value string
}

type LeaderboardColumn struct {
	Class string
	Sort  string
	Label string
}

type DetailFieldSpec struct {
	Param string
	Label string
}

const (
	FieldGame             = "game"
	FieldUsername         = "username"
	FieldPassword         = "password"
	FieldCompanyName      = "companyname"
	FieldCEOName          = "ceoname"
	FieldGameMode         = "gamemode"
	FieldGameGoal         = "gamegoal"
	FieldGameStartingDate = "gamestartingdate"
	FieldLifespan         = "lifespan"
	FieldStands           = "stands"
	FieldCupsSold         = "cupssold"
	FieldCashAssets       = "cashassets"
	FieldStockAssets      = "stockassets"
	FieldStandsAssets     = "standsassets"
	FieldUpgradesAssets   = "upgradesassets"
	FieldRetainedEarnings = "retainedearnings"
	FieldRevenues         = "revenues"
	FieldChecksumClient   = "checksumclient"
)

var SyncFields = []string{
	FieldGame,
	FieldUsername,
	FieldPassword,
	FieldCompanyName,
	FieldCEOName,
	FieldGameMode,
	FieldGameGoal,
	FieldGameStartingDate,
	FieldLifespan,
	FieldStands,
	FieldCupsSold,
	FieldCashAssets,
	FieldStockAssets,
	FieldStandsAssets,
	FieldUpgradesAssets,
	FieldRetainedEarnings,
	FieldRevenues,
	FieldChecksumClient,
}

var LeaderboardColumns = []LeaderboardColumn{
	{Class: "rank", Sort: "0", Label: "Rank"},
	{Class: "company", Sort: "1", Label: "Company Name"},
	{Class: "ceo", Sort: "2", Label: "CEO"},
	{Class: "life", Sort: "3", Label: "Lifespan"},
	{Class: "money", Sort: "4", Label: "Market Cap"},
}

var LeaderboardQueryDefaults = []QueryDefault{
	{Name: "pagenum", Value: "1"},
	{Name: "sort", Value: "0"},
	{Name: "gamemode", Value: "0"},
	{Name: "gamegoal", Value: "0"},
	{Name: "ranktype", Value: "0"},
	{Name: "username", Value: "0"},
}

var DetailFieldSpecs = []DetailFieldSpec{
	{Param: "d1", Label: "Company"},
	{Param: "d2", Label: "CEO"},
	{Param: "d3", Label: "Mode"},
	{Param: "d4", Label: "Goal"},
	{Param: "d5", Label: "Rank"},
	{Param: "d6", Label: "Total Entries"},
	{Param: "d7", Label: "Title"},
	{Param: "d8", Label: "Lifespan"},
	{Param: "d9", Label: "Stands"},
	{Param: "d10", Label: "Cups Sold"},
	{Param: "d11", Label: "Market Cap"},
	{Param: "d12", Label: "Revenues"},
	{Param: "d13", Label: "Retained Earnings"},
	{Param: "d14", Label: "Percent"},
	{Param: "d15", Label: "Cash"},
	{Param: "d16", Label: "Stock"},
	{Param: "d17", Label: "Stand Assets"},
	{Param: "d18", Label: "Upgrade Assets"},
}

var ModernBrowserTokens = []string{
	"firefox/",
	"chrome/",
	"chromium/",
	"crios/",
	"fxios/",
	"safari/",
	"edg/",
	"opr/",
}

var ProjectAssetPaths = map[string]string{
	"flameflag_lemon.png":     "static/project/flameflag_lemon.png",
	"lt2_asset_credits.avif":  "static/project/lt2_asset_credits.avif",
	"lt2_asset_github.avif":   "static/project/lt2_asset_github.avif",
	"lt2_green_pill.avif":     "static/project/lt2_green_pill.avif",
	"lt2_icon_findings.avif":  "static/project/lt2_icon_findings.avif",
	"lt2_icon_lsx.avif":       "static/project/lt2_icon_lsx.avif",
	"lt2_icon_play.avif":      "static/project/lt2_icon_play.avif",
	"lt2_lemon_pair.avif":     "static/project/lt2_lemon_pair.avif",
	"lt2_logo_text_only.avif": "static/project/lt2_logo_text_only.avif",
	"menu_map_backdrop.avif":  "static/project/menu_map_backdrop.avif",
	"pitcher.avif":            "static/admin/pitcher.avif",
	"timz_lemon.png":          "static/project/timz_lemon.png",
	"warning.avif":            "static/admin/warning.avif",
}
