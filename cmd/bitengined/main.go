package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const version = "0.1.0-mvp"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	slog.Info("starting bitengined", "version", version)

	// TODO: Task 1 — config + DB + Redis + health check
	// TODO: Task 2 — auth + setup wizard
	// TODO: Task 3 — Ollama client + intent engine
	// TODO: Task 4 — code generator + cloud API
	// TODO: Task 5 — Docker runtime + build + deploy
	// TODO: Task 6 — SSE streaming + end-to-end pipeline
	// TODO: Task 7 — React frontend
	// TODO: Task 8 — templates + install script

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/system/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","version":"%s"}`, version)
	})

	srv := &http.Server{Addr: ":9000", Handler: mux, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()
	<-ctx.Done()
	slog.Info("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}
