package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/bitEngine-AI/bitengine/api"
	"github.com/bitEngine-AI/bitengine/internal/ai"
	"github.com/bitEngine-AI/bitengine/internal/apps"
	"github.com/bitEngine-AI/bitengine/internal/caddy"
	"github.com/bitEngine-AI/bitengine/internal/config"
	"github.com/bitEngine-AI/bitengine/internal/runtime"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	slog.Info("starting bitengined", "version", api.Version)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(ctx)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	db, err := sqlx.ConnectContext(ctx, "postgres", cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("database connected")

	if err := runMigrations(ctx, db); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Error("failed to parse redis url", "error", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	slog.Info("redis connected")

	ollama := ai.NewOllamaClient(cfg.OllamaURL)
	if ollama.IsAvailable(ctx) {
		slog.Info("ollama connected", "url", cfg.OllamaURL)
	} else {
		slog.Warn("ollama not available, AI features will be limited", "url", cfg.OllamaURL)
	}

	codegen := ai.NewCodeGen(cfg.AnthropicKey, cfg.DeepSeekKey, ollama)
	slog.Info("code generator ready", "mode", codegen.Mode())

	containerMgr, err := runtime.NewContainerManager()
	if err != nil {
		slog.Warn("docker not available, app deployment disabled", "error", err)
	} else {
		defer containerMgr.Close()
		slog.Info("docker connected")
	}

	caddyMgr := caddy.NewManager(cfg.CaddyAdminURL, cfg.BaseDomain)

	var builder *runtime.ImageBuilder
	if containerMgr != nil {
		builder = runtime.NewImageBuilderFromManager(containerMgr)
	}

	gen := apps.NewAppGenerator(
		ai.NewIntentEngine(ollama),
		codegen,
		ai.NewCodeReviewer(ollama),
		builder,
		containerMgr,
		caddyMgr,
		db,
	)
	svc := apps.NewAppService(db, containerMgr)
	tplSvc := apps.NewTemplateService("templates", builder, containerMgr, caddyMgr, db)

	router := api.NewRouter(db, rdb, cfg.JWTSecret, ollama, codegen, gen, svc, tplSvc)

	srv := &http.Server{
		Addr:        cfg.ListenAddr,
		Handler:     router,
		ReadTimeout: 15 * time.Second,
		// WriteTimeout disabled: SSE streams (app generation) can run for minutes
	}

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

func runMigrations(ctx context.Context, db *sqlx.DB) error {
	files, err := filepath.Glob("migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		slog.Info("running migration", "file", f)
		content, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, string(content)); err != nil {
			slog.Warn("migration warning (may already be applied)", "file", f, "error", err)
		}
	}
	return nil
}
