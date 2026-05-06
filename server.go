package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/charmbracelet/log"

	"lt2_reverse/lsx_server_go/internal/lsx"
	"lt2_reverse/lsx_server_go/internal/tui"
)

func runTUI(ctx context.Context, opts serveOptions) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	events := make(chan lsx.Event, 256)
	discordSink := newDiscordNotifier(opts).Sink()
	srv, err := lsx.NewServer(lsx.Config{
		DBPath:         opts.dbPath,
		StrictChecksum: opts.strictChecksum,
		AdminUser:      opts.adminUser,
		AdminPassword:  opts.adminPassword,
		AdminPath:      opts.adminPath,
		EventSink: func(ev lsx.Event) {
			select {
			case events <- ev:
			default:
			}
			if discordSink != nil {
				discordSink(ev)
			}
		},
	})
	if err != nil {
		return err
	}
	defer func() { _ = srv.Close() }()
	if err := seedServer(srv, opts); err != nil {
		return err
	}
	history, err := srv.RecentEvents(200)
	if err != nil {
		return err
	}
	adminPath := ""
	if srv.AdminEnabled() {
		adminPath = srv.AdminPath()
	}

	_, listener, errCh, err := startHTTP(ctx, opts.addr, srv.Routes())
	if err != nil {
		return err
	}
	return tui.Run(ctx, tui.Config{
		Addr:         opts.addr,
		AdminPath:    adminPath,
		Bound:        listener.Addr().String(),
		DBPath:       opts.dbPath,
		Cancel:       cancel,
		Events:       events,
		ServerErrors: errCh,
		History:      history,
	})
}

func runPlainServer(ctx context.Context, opts serveOptions) error {
	notifier := newDiscordNotifier(opts)
	srv, err := lsx.NewServer(lsx.Config{
		DBPath:         opts.dbPath,
		StrictChecksum: opts.strictChecksum,
		EventSink:      notifier.Sink(),
		AdminUser:      opts.adminUser,
		AdminPassword:  opts.adminPassword,
		AdminPath:      opts.adminPath,
	})
	if err != nil {
		return err
	}
	defer func() { _ = srv.Close() }()
	if err := seedServer(srv, opts); err != nil {
		return err
	}

	_, listener, errCh, err := startHTTP(ctx, opts.addr, lsx.LogRequests(srv.Routes()))
	if err != nil {
		return err
	}

	log.Info("LT2 LSX compatibility server listening", "addr", listener.Addr(), "url", "http://"+listener.Addr().String()+"/")
	log.Info("SQLite storage", "path", opts.dbPath)
	return <-errCh
}

func seedServer(srv *lsx.Server, opts serveOptions) error {
	if !opts.seed {
		return nil
	}
	inserted, err := srv.SeedDemoData()
	if err != nil {
		return err
	}
	log.Info("Seeded recovered leaderboard rows into SQLite", "rows", inserted)
	return nil
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func startHTTP(ctx context.Context, addr string, handler http.Handler) (*http.Server, net.Listener, <-chan error, error) {
	addr = normalizeListenAddr(addr)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, nil, err
	}
	server := newHTTPServer(addr, handler)
	return server, listener, serveHTTP(ctx, server, listener), nil
}

func normalizeListenAddr(addr string) string {
	if _, err := strconv.Atoi(addr); err == nil {
		return ":" + addr
	}
	return addr
}

func serveHTTP(ctx context.Context, server *http.Server, listener net.Listener) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
		close(errCh)
	}()
	return errCh
}
