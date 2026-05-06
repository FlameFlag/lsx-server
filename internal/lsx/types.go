package lsx

import (
	"database/sql"
	"time"
)

type Config struct {
	DBPath         string
	StrictChecksum bool
	EventSink      EventSink
	AdminUser      string
	AdminPassword  string
	AdminPath      string
	SessionSecret  string
	OpenAPIYAML    []byte
}

type Server struct {
	db             *sql.DB
	strictChecksum bool
	eventSink      EventSink
	adminUser      string
	adminPassword  string
	adminPath      string
	sessionSecret  []byte
	openAPIYAML    []byte
}

type EventSink func(Event)

type Event struct {
	ID         int64
	Time       time.Time
	Kind       string
	Method     string
	Path       string
	RemoteAddr string
	Status     int
	Message    string
}

type Submission struct {
	ID               int64             `json:"id"`
	ReceivedAt       time.Time         `json:"received_at"`
	RemoteAddr       string            `json:"remote_addr"`
	Host             string            `json:"host"`
	RawQuery         string            `json:"raw_query"`
	Fields           map[string]string `json:"fields"`
	ChecksumClient   int32             `json:"checksum_client"`
	ChecksumComputed int32             `json:"checksum_computed"`
	ChecksumPresent  bool              `json:"checksum_present"`
	ChecksumValid    bool              `json:"checksum_valid"`
}

type AccountRequest struct {
	ID         int64     `json:"id"`
	ReceivedAt time.Time `json:"received_at"`
	RemoteAddr string    `json:"remote_addr"`
	Host       string    `json:"host"`
	Username   string    `json:"username"`
	Password   string    `json:"password"`
	RawQuery   string    `json:"raw_query"`
}

type AdminStats struct {
	Submissions    int64
	Accounts       int64
	Events         int64
	LastSubmission string
}

type LeaderboardRow struct {
	Company       string
	CEO           string
	Mode          string
	Goal          string
	Title         string
	Lifespan      int64
	GameMode      int64
	GameGoal      int64
	MarketCents   int64
	RevenueCents  int64
	RetainedCents int64
	Stands        int64
	CupsSold      int64
	CashCents     int64
	StockCents    int64
	StandCents    int64
	UpgradeCents  int64
	Username      string
	DateScalar    string
	Source        string
	ChecksumValid bool
	Detail        map[string]string
}

var gif1x1 = []byte{
	'G', 'I', 'F', '8', '9', 'a', 0x01, 0x00, 0x01, 0x00, 0x80, 0x00,
	0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, '!', 0xf9, 0x04, 0x01,
	0x00, 0x00, 0x00, 0x00, ',', 0x00, 0x00, 0x00, 0x00, 0x01, 0x00,
	0x01, 0x00, 0x00, 0x02, 0x02, 'D', 0x01, 0x00, ';',
}
