package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/shengli/prism/internal/query"
	"github.com/shengli/prism/internal/storage"
)

func main() {
	var (
		listenAddr  = flag.String("listen", ":28080", "HTTP listen address")
		chAddrs     = flag.String("clickhouse", "localhost:29000", "ClickHouse address")
		chDB        = flag.String("ch-db", "prism", "ClickHouse database")
		chUser      = flag.String("ch-user", "default", "ClickHouse username")
		chPass      = flag.String("ch-pass", "", "ClickHouse password")
		queryToken  = flag.String("query-token", "", "Optional bearer token for /api/v1/* auth (empty = no auth)")
		corsOrigins = flag.String("cors-origins", "*", "Allowed CORS origin for query API (Access-Control-Allow-Origin)")
	)
	flag.Parse()

	// Setup ClickHouse storage
	store, err := storage.NewClickHouseStorage(storage.ClickHouseConfig{
		Addrs:    []string{*chAddrs},
		Database: *chDB,
		Username: *chUser,
		Password: *chPass,
	})
	if err != nil {
		slog.Error("failed to connect to ClickHouse", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	// Create and start query server
	srv := query.NewServer(store,
		query.WithToken(*queryToken),
		query.WithCORSOrigins(*corsOrigins),
	)

	go func() {
		if err := srv.ListenAndServe(*listenAddr); err != nil {
			slog.Error("query server error", "error", err)
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	slog.Info("query server shutdown complete")
}
