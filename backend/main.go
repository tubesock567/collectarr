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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	mediaPath := envOrDefault("MEDIA_PATH", "/media")
	dbPath := envOrDefault("DB_PATH", "/data/collectarr.db")
	port := envOrDefault("PORT", "8080")
	autoScan := envOrDefault("AUTO_SCAN", "true") == "true"

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
	api := NewAPI(store, scanner, logger)

	if autoScan {
		if report, err := scanner.ScanLibrary(context.Background()); err != nil {
			logger.Error("startup scan failed", "error", err, "media_path", mediaPath)
		} else {
			logger.Info("startup scan complete", "files_found", report.FilesFound, "inserted", report.Inserted, "updated", report.Updated, "skipped", report.Skipped)
			api.enqueueConfiguredPreviewAssets()
		}
	} else {
		logger.Info("auto scan disabled, skipping startup scan")
	}

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

	thumbsDone := make(chan struct{})
	go func() {
		api.thumbWg.Wait()
		close(thumbsDone)
	}()

	select {
	case <-thumbsDone:
		logger.Info("background thumbnail tasks complete")
	case <-ctx.Done():
		logger.Warn("timed out waiting for background thumbnail tasks")
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
