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
)

func main() {
	logBuffer := NewLogBuffer(defaultLogEntryLimit)
	logger := slog.New(NewBufferedHandler(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		logBuffer,
	))

	mediaPath := envOrDefault("MEDIA_PATH", "/media")
	dbPath := envOrDefault("DB_PATH", "/data/collectarr.db")
	port := envOrDefault("PORT", "8893")

	store, err := NewStore(dbPath, logger)
	if err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.CreateDefaultUser(); err != nil {
		logger.Error("default user initialization failed", "error", err)
		os.Exit(1)
	}

	scanner := NewScanner(mediaPath, store, logger)
	api := NewAPI(store, scanner, logger, logBuffer)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           api.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("backend listening", "port", port, "media_path", mediaPath, "db_path", dbPath)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-shutdownCtx.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("shutting down backend")
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown failed", "error", err)
	}

	backgroundDone := make(chan struct{})
	go func() {
		api.thumbWg.Wait()
		api.scanWg.Wait()
		close(backgroundDone)
	}()

	select {
	case <-backgroundDone:
		logger.Info("background scan and thumbnail tasks complete")
	case <-ctx.Done():
		logger.Warn("timed out waiting for background scan and thumbnail tasks")
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
