package main

import (
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/shengli/prism/internal/collector"
	"github.com/shengli/prism/internal/storage"
	prismpb "github.com/shengli/prism/proto/gen"
	"google.golang.org/grpc"
)

func main() {
	var (
		listenAddr  = flag.String("listen", ":24317", "gRPC listen address")
		metricsAddr = flag.String("metrics", ":24318", "Prometheus metrics HTTP address")
		chAddrs     = flag.String("clickhouse", "localhost:29000", "ClickHouse address")
		chDB        = flag.String("ch-db", "prism", "ClickHouse database")
		chUser      = flag.String("ch-user", "default", "ClickHouse username")
		chPass      = flag.String("ch-pass", "", "ClickHouse password")
		redisAddr   = flag.String("redis", "localhost:26379", "Redis address")
		flushSize   = flag.Int("flush-size", 5000, "Batch flush size")
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

	// Setup Redis
	rdb := redis.NewClient(&redis.Options{Addr: *redisAddr})
	depTracker := collector.NewDependencyTracker(rdb)

	// Create collector
	coll := collector.New(collector.Config{
		FlushSize:  *flushSize,
		Store:      store,
		DepTracker: depTracker,
	})

	// Start metrics + HTTP ingest server
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		})
		mux.Handle("/api/v1/spans", coll.HTTPIngestHandler())
		slog.Info("collector metrics+ingest server starting", "addr", *metricsAddr)
		if err := http.ListenAndServe(*metricsAddr, mux); err != nil {
			slog.Error("metrics server error", "error", err)
		}
	}()

	// Start gRPC server
	lis, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		slog.Error("failed to listen", "error", err, "addr", *listenAddr)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	prismpb.RegisterCollectorServiceServer(grpcServer, coll)

	go func() {
		slog.Info("collector gRPC server starting", "addr", *listenAddr)
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("gRPC server error", "error", err)
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	slog.Info("shutting down collector...")
	grpcServer.GracefulStop()
	coll.Shutdown()
	rdb.Close()
	slog.Info("collector shutdown complete")
}
