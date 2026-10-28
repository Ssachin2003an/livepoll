package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"strings"
	"livepoll/backend/internal/config"
	"livepoll/backend/internal/httpapi"
	"livepoll/backend/internal/live"
	"livepoll/backend/internal/store"
	"github.com/joho/godotenv"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := godotenv.Load("../.env"); err != nil {
    	slog.Info("No root .env file found; using environment variables")
	}

cfg := config.Load()
	slog.Info("MongoDB configuration",
	"database", cfg.MongoDB,
	"uriConfigured", cfg.MongoURI != "",
	"usesAtlas", strings.HasPrefix(cfg.MongoURI, "mongodb+srv://"),
)

	startup, cancel := context.WithTimeout(ctx, 20*time.Second)
	database, err := store.Connect(startup, cfg.MongoURI, cfg.MongoDB)
	cancel()
	if err != nil {
		slog.Error("MongoDB connection failed; check service availability and MONGO_URI")
		os.Exit(1)
	}
	defer func() {
		closeCtx, done := context.WithTimeout(context.Background(), 8*time.Second)
		defer done()
		if err := database.Close(closeCtx); err != nil {
			slog.Warn("MongoDB disconnect failed")
		}
	}()

	realtime, err := live.Connect(cfg.RedisURL)
	if err != nil {
		slog.Error("Redis connection failed; check service availability and REDIS_URL")
		os.Exit(1)
	}
	defer func() {
		if err := realtime.Close(); err != nil {
			slog.Warn("Redis disconnect failed")
		}
	}()

	api := httpapi.New(database, realtime, cfg)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           api.Router(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go reconcile(ctx, api)
	serverErr := make(chan error, 1)
	go func() { slog.Info("Live Poll API listening", "port", port); serverErr <- server.ListenAndServe() }()

	select {
	case <-ctx.Done():
		slog.Info("shutdown requested")
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server stopped unexpectedly")
			os.Exit(1)
		}
	}

	shutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdown); err != nil {
		slog.Warn("graceful shutdown timed out")
	}
}

func reconcile(ctx context.Context, api *httpapi.API) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			work, cancel := context.WithTimeout(ctx, 4*time.Second)
			api.ReconcilePending(work)
			cancel()
		}
	}
}
