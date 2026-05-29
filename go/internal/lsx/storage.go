package lsx

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"lt2_reverse/lsx_server_go/internal/lsx/compat"
	"lt2_reverse/lsx_server_go/internal/lsxvalue"
	"lt2_reverse/lsx_server_go/internal/strutil"

	sq "github.com/Masterminds/squirrel"
	_ "github.com/ncruces/go-sqlite3/driver"
)

const sqliteDriver = "sqlite3"

var submissionInsertBaseColumns = []string{
	"received_at",
	"remote_addr",
	"host",
	"raw_query",
	"checksum_client",
	"checksum_computed",
	"checksum_present",
	"checksum_valid",
}

var submissionSelectBaseColumns = append([]string{"id"}, submissionInsertBaseColumns...)

type rowScanner interface {
	Scan(dest ...any) error
}

func NewServer(cfg Config) (*Server, error) {
	if cfg.DBPath == "" {
		cfg.DBPath = filepath.Join("data", "lsx.sqlite3")
	}
	adminPath, err := normalizeAdminPath(cfg.AdminPath)
	if err != nil {
		return nil, err
	}
	db, err := openSQLite(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	srv := &Server{
		db:             db,
		strictChecksum: cfg.StrictChecksum,
		eventSink:      cfg.EventSink,
		adminUser:      cfg.AdminUser,
		adminPassword:  cfg.AdminPassword,
		adminPath:      adminPath,
		sessionSecret:  []byte(strutil.FirstNonEmpty(cfg.SessionSecret, randomSecret())),
		openAPIYAML:    slices.Clone(cfg.OpenAPIYAML),
	}
	if err := srv.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return srv, nil
}

func randomSecret() string {
	return rand.Text()
}

func (s *Server) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func openSQLite(path string) (*sql.DB, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open(sqliteDriver, sqliteDSN(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := configureSQLite(db, path); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func sqliteDSN(path string) string {
	if path == ":memory:" {
		return ":memory:"
	}
	return path
}

func configureSQLite(db *sql.DB, path string) error {
	if _, err := db.Exec("PRAGMA busy_timeout = 10000"); err != nil {
		return err
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return err
	}
	if path != ":memory:" {
		if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) migrate() error {
	fieldColumns := make([]string, 0, len(compat.SyncFields))
	for _, name := range compat.SyncFields {
		fieldColumns = append(fieldColumns, name+" TEXT NOT NULL DEFAULT ''")
	}
	schema := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS submissions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	received_at TEXT NOT NULL,
	remote_addr TEXT NOT NULL,
	host TEXT NOT NULL,
	raw_query TEXT NOT NULL,
	checksum_client INTEGER NOT NULL,
	checksum_computed INTEGER NOT NULL,
	checksum_present BOOLEAN NOT NULL,
	checksum_valid BOOLEAN NOT NULL,
	%s
);
CREATE INDEX IF NOT EXISTS submissions_game_idx ON submissions(game);
CREATE INDEX IF NOT EXISTS submissions_market_idx ON submissions(cashassets, stockassets, standsassets, upgradesassets);
CREATE TABLE IF NOT EXISTS accounts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	received_at TEXT NOT NULL,
	remote_addr TEXT NOT NULL,
	host TEXT NOT NULL,
	username TEXT NOT NULL,
	password TEXT NOT NULL,
	raw_query TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS request_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	received_at TEXT NOT NULL,
	kind TEXT NOT NULL,
	method TEXT NOT NULL,
	path TEXT NOT NULL,
	remote_addr TEXT NOT NULL,
	status INTEGER NOT NULL,
	message TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS request_events_received_idx ON request_events(received_at);
CREATE INDEX IF NOT EXISTS request_events_kind_idx ON request_events(kind);`, strings.Join(fieldColumns, ",\n\t"))
	_, err := s.db.Exec(schema)
	return err
}

func (s *Server) appendSubmission(sub Submission) error {
	columns := submissionInsertColumns()
	_, err := s.sql().
		Insert("submissions").
		Columns(columns...).
		Values(submissionInsertArgs(sub)...).
		Exec()
	return err
}

func (s *Server) appendAccount(req AccountRequest) error {
	_, err := s.sql().
		Insert("accounts").
		Columns("received_at", "remote_addr", "host", "username", "password", "raw_query").
		Values(
			req.ReceivedAt.Format(time.RFC3339Nano),
			req.RemoteAddr,
			req.Host,
			req.Username,
			req.Password,
			req.RawQuery,
		).
		Exec()
	return err
}

func (s *Server) appendEvent(ev Event) error {
	_, err := s.sql().
		Insert("request_events").
		Columns("received_at", "kind", "method", "path", "remote_addr", "status", "message").
		Values(
			ev.Time.UTC().Format(time.RFC3339Nano),
			ev.Kind,
			ev.Method,
			ev.Path,
			ev.RemoteAddr,
			ev.Status,
			ev.Message,
		).
		Exec()
	return err
}

func (s *Server) RecentEvents(limit int) ([]Event, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := s.sql().
		Select("id", "received_at", "kind", "method", "path", "remote_addr", "status", "message").
		From("request_events").
		OrderBy("id DESC").
		Limit(uint64(limit)).
		Query()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var newestFirst []Event
	for rows.Next() {
		var ev Event
		var receivedAt string
		if err := rows.Scan(&ev.ID, &receivedAt, &ev.Kind, &ev.Method, &ev.Path, &ev.RemoteAddr, &ev.Status, &ev.Message); err != nil {
			return nil, err
		}
		ev.Time, _ = time.Parse(time.RFC3339Nano, receivedAt)
		newestFirst = append(newestFirst, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	slices.Reverse(newestFirst)
	return newestFirst, nil
}

func (s *Server) AdminStats() (AdminStats, error) {
	var stats AdminStats
	if err := s.countRows("submissions").Scan(&stats.Submissions); err != nil {
		return stats, err
	}
	if err := s.countRows("accounts").Scan(&stats.Accounts); err != nil {
		return stats, err
	}
	if err := s.countRows("request_events").Scan(&stats.Events); err != nil {
		return stats, err
	}
	if err := s.sql().
		Select("COALESCE(MAX(received_at), '')").
		From("submissions").
		QueryRow().
		Scan(&stats.LastSubmission); err != nil {
		return stats, err
	}
	return stats, nil
}

func (s *Server) AdminSubmissions(limit int) ([]Submission, error) {
	return s.loadSubmissionsLimit(limit)
}

func (s *Server) AdminAccounts(limit int) ([]AccountRequest, error) {
	rows, err := s.sql().
		Select("id", "received_at", "remote_addr", "host", "username", "password", "raw_query").
		From("accounts").
		OrderBy("id DESC").
		Limit(uint64(limit)).
		Query()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var accounts []AccountRequest
	for rows.Next() {
		var req AccountRequest
		var receivedAt string
		if err := rows.Scan(&req.ID, &receivedAt, &req.RemoteAddr, &req.Host, &req.Username, &req.Password, &req.RawQuery); err != nil {
			return nil, err
		}
		req.ReceivedAt, _ = time.Parse(time.RFC3339Nano, receivedAt)
		accounts = append(accounts, req)
	}
	return accounts, rows.Err()
}

func (s *Server) DeleteSubmission(id int64) error {
	return s.deleteByID("submissions", id)
}

func (s *Server) DeleteAccount(id int64) error {
	return s.deleteByID("accounts", id)
}

func (s *Server) DeleteEvent(id int64) error {
	return s.deleteByID("request_events", id)
}

func (s *Server) ClearEvents() error {
	_, err := s.sql().
		Delete("request_events").
		Exec()
	return err
}

func (s *Server) UpdateSubmissionFields(id int64, fields map[string]string) error {
	query := s.sql().Update("submissions")
	for _, name := range compat.SyncFields {
		query = query.Set(name, fields[name])
	}
	computed := compat.ComputeChecksum(fields)
	client, present := lsxvalue.ParseI32(fields[compat.FieldChecksumClient])
	_, err := query.
		Set("checksum_client", client).
		Set("checksum_computed", computed).
		Set("checksum_present", present).
		Set("checksum_valid", !present || client == computed).
		Where(sq.Eq{"id": id}).
		Exec()
	return err
}

func (s *Server) loadSubmissions() ([]Submission, error) {
	return s.loadSubmissionsLimit(0)
}

func (s *Server) loadSubmissionsLimit(limit int) ([]Submission, error) {
	columns := submissionSelectColumns()
	query := s.sql().
		Select(columns...).
		From("submissions").
		Where(sq.Or{
			sq.Eq{compat.FieldGame: "lemonade2"},
			sq.Eq{compat.FieldGame: ""},
		}).
		OrderBy("id DESC")
	if limit > 0 {
		query = query.Limit(uint64(limit))
	}
	rows, err := query.Query()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var submissions []Submission
	for rows.Next() {
		sub, err := scanSubmission(rows)
		if err != nil {
			return nil, err
		}
		submissions = append(submissions, sub)
	}
	return submissions, rows.Err()
}

func submissionInsertColumns() []string {
	return append(slices.Clone(submissionInsertBaseColumns), compat.SyncFields...)
}

func submissionSelectColumns() []string {
	return append(slices.Clone(submissionSelectBaseColumns), compat.SyncFields...)
}

func submissionInsertArgs(sub Submission) []any {
	args := make([]any, 0, len(submissionInsertBaseColumns)+len(compat.SyncFields))
	args = append(args,
		sub.ReceivedAt.Format(time.RFC3339Nano),
		sub.RemoteAddr,
		sub.Host,
		sub.RawQuery,
		sub.ChecksumClient,
		sub.ChecksumComputed,
		sub.ChecksumPresent,
		sub.ChecksumValid,
	)
	for _, name := range compat.SyncFields {
		args = append(args, sub.Fields[name])
	}
	return args
}

func scanSubmission(scan rowScanner) (Submission, error) {
	var sub Submission
	var receivedAt string
	fieldValues := make([]string, len(compat.SyncFields))
	dest := make([]any, 0, len(submissionSelectBaseColumns)+len(fieldValues))
	dest = append(dest,
		&sub.ID,
		&receivedAt,
		&sub.RemoteAddr,
		&sub.Host,
		&sub.RawQuery,
		&sub.ChecksumClient,
		&sub.ChecksumComputed,
		&sub.ChecksumPresent,
		&sub.ChecksumValid,
	)
	for i := range fieldValues {
		dest = append(dest, &fieldValues[i])
	}
	if err := scan.Scan(dest...); err != nil {
		return sub, err
	}
	sub.ReceivedAt, _ = time.Parse(time.RFC3339Nano, receivedAt)
	sub.Fields = make(map[string]string, len(compat.SyncFields))
	for i, name := range compat.SyncFields {
		sub.Fields[name] = fieldValues[i]
	}
	return sub, nil
}

func (s *Server) sql() sq.StatementBuilderType {
	return sq.StatementBuilder.RunWith(s.db)
}

func (s *Server) countRows(table string) sq.RowScanner {
	return s.sql().
		Select("COUNT(*)").
		From(table).
		QueryRow()
}

func (s *Server) deleteByID(table string, id int64) error {
	_, err := s.sql().
		Delete(table).
		Where(sq.Eq{"id": id}).
		Exec()
	return err
}
