package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/fang"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	configureLogger()
	if err := loadDotEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to load .env: %v\n", err)
		os.Exit(1)
	}
	cmd, err := newRootCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid environment configuration: %v\n", err)
		os.Exit(1)
	}
	displayVersion := envString("LSX_VERSION", version)
	if err := fang.Execute(context.Background(), cmd, fang.WithVersion(displayVersion)); err != nil {
		os.Exit(1)
	}
}

func loadDotEnv() error {
	err := godotenv.Load()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func newRootCommand() (*cobra.Command, error) {
	opts, err := serveOptionsFromEnv()
	if err != nil {
		return nil, err
	}

	cmd := &cobra.Command{
		Use:   "lsx-server",
		Short: "Run a Lemonade Tycoon 2 LSX compatibility server",
		Long: "Run a compatibility server for the recovered Lemonade Tycoon 2 " +
			"Stock Exchange HTTP endpoints.",
		Example: "  lsx-server --addr 127.0.0.1:8080\n" +
			"  lsx-server --data ./data/lsx.sqlite3\n" +
			"  lsx-server --seed --addr 127.0.0.1:8080\n" +
			"  lsx-server --admin-user admin --admin-password secret --admin-path /manage-secret\n" +
			"  lsx-server --discord-webhook https://discord.com/api/webhooks/...\n" +
			"  lsx-server --plain",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.plain {
				return runPlainServer(cmd.Context(), opts)
			}
			return runTUI(cmd.Context(), opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.addr, "addr", opts.addr, "address to listen on; use :80 when pointing the game at this server; can also be set with LSX_ADDR or LSX_HTTP_PORT")
	flags.StringVar(&opts.dbPath, "data", opts.dbPath, "SQLite database path; can also be set with LSX_DATA")
	flags.BoolVar(&opts.seed, "seed", opts.seed, "insert recovered Wayback leaderboard rows into SQLite for local testing; can also be set with LSX_SEED")
	flags.BoolVar(&opts.strictChecksum, "strict-checksum", opts.strictChecksum, "return failure when checksumclient is present but does not match the recovered client formula; can also be set with LSX_STRICT_CHECKSUM")
	flags.BoolVar(&opts.plain, "plain", opts.plain, "disable the Bubble Tea monitor and print plain request logs; can also be set with LSX_PLAIN")
	flags.StringVar(&opts.adminUser, "admin-user", opts.adminUser, "admin username; enables the admin console with --admin-password; can also be set with LSX_ADMIN_USER")
	flags.StringVar(&opts.adminPassword, "admin-password", opts.adminPassword, "admin password; enables the admin console with --admin-user; can also be set with LSX_ADMIN_PASSWORD")
	flags.StringVar(&opts.adminPath, "admin-path", opts.adminPath, "custom admin console URL path; defaults to /admin and can also be set with LSX_ADMIN_PATH")
	flags.StringVar(&opts.discordWebhook, "discord-webhook", opts.discordWebhook, "Discord webhook URL for sync/account/error notifications; can also be set with LSX_DISCORD_WEBHOOK")
	flags.StringVar(&opts.discordEvents, "discord-events", opts.discordEvents, "comma-separated event kinds sent to Discord; can also be set with LSX_DISCORD_EVENTS")
	flags.StringVar(&opts.discordIcon, "discord-icon", opts.discordIcon, "image file uploaded as the Discord embed thumbnail; use embedded, a file path, or empty to disable it; can also be set with LSX_DISCORD_ICON")
	flags.DurationVar(&opts.discordTimeout, "discord-timeout", opts.discordTimeout, "timeout for each Discord webhook POST; can also be set with LSX_DISCORD_TIMEOUT")

	return cmd, nil
}

func serveOptionsFromEnv() (serveOptions, error) {
	opts := serveOptions{
		addr:           envString("LSX_ADDR", ":80"),
		dbPath:         envString("LSX_DATA", "data/lsx.sqlite3"),
		adminUser:      os.Getenv("LSX_ADMIN_USER"),
		adminPassword:  os.Getenv("LSX_ADMIN_PASSWORD"),
		adminPath:      os.Getenv("LSX_ADMIN_PATH"),
		discordWebhook: envString("LSX_DISCORD_WEBHOOK", ""),
		discordEvents:  envString("LSX_DISCORD_EVENTS", "sync,sync_rejected,sync_error,account,account_error"),
		discordIcon:    envString("LSX_DISCORD_ICON", "embedded"),
		discordTimeout: 5 * time.Second,
	}

	if _, ok := os.LookupEnv("LSX_ADDR"); !ok {
		opts.addr = envString("LSX_HTTP_PORT", opts.addr)
	}
	var err error
	if opts.seed, err = envBool("LSX_SEED", false); err != nil {
		return serveOptions{}, err
	}
	if opts.strictChecksum, err = envBool("LSX_STRICT_CHECKSUM", false); err != nil {
		return serveOptions{}, err
	}
	if opts.plain, err = envBool("LSX_PLAIN", false); err != nil {
		return serveOptions{}, err
	}
	if opts.discordTimeout, err = envDuration("LSX_DISCORD_TIMEOUT", opts.discordTimeout); err != nil {
		return serveOptions{}, err
	}

	return opts, nil
}

func envString(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}

func envBool(name string, fallback bool) (bool, error) {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", name, err)
	}
	return parsed, nil
}

func envDuration(name string, fallback time.Duration) (time.Duration, error) {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration like 5s or 1m: %w", name, err)
	}
	return parsed, nil
}

type serveOptions struct {
	addr           string
	dbPath         string
	strictChecksum bool
	plain          bool
	seed           bool
	adminUser      string
	adminPassword  string
	adminPath      string
	discordWebhook string
	discordEvents  string
	discordIcon    string
	discordTimeout time.Duration
}
