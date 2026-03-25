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

	"github.com/evan/football-picks/internal/api"
	"github.com/evan/football-picks/internal/api/handlers"
	"github.com/evan/football-picks/internal/api/middleware"
	"github.com/evan/football-picks/internal/cache"
	appdb "github.com/evan/football-picks/internal/db"
	"github.com/evan/football-picks/internal/espn"
	"github.com/evan/football-picks/internal/notify"
	"github.com/evan/football-picks/internal/scheduler"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	ctx := context.Background()

	// Load .env if present (dev convenience; no-op in production where env vars are injected).
	_ = godotenv.Load()

	dbURL := mustEnv("DATABASE_URL")
	supabaseURL := mustEnv("SUPABASE_URL")
	port := getEnv("PORT", "8080")
	corsOrigin := getEnv("APP_BASE_URL", "http://localhost:5173")

	pool, err := appdb.New(ctx, dbURL)
	if err != nil {
		slog.Error("db connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Run migrations at startup using database/sql via the pgx stdlib adapter.
	sqlDB := stdlib.OpenDBFromPool(pool)
	if err := goose.SetDialect("postgres"); err != nil {
		slog.Error("goose dialect", "err", err)
		os.Exit(1)
	}
	if err := goose.Up(sqlDB, "./migrations"); err != nil {
		slog.Error("goose up", "err", err)
		os.Exit(1)
	}
	if err := sqlDB.Close(); err != nil {
		slog.Warn("close migration db", "err", err)
	}

	// Build JWKS keyfunc from Supabase's public JWKS endpoint.
	jwks, err := middleware.NewJWKS(ctx, supabaseURL)
	if err != nil {
		slog.Error("jwks init", "err", err)
		os.Exit(1)
	}

	var mailer notify.Mailer
	resendKey := getEnv("RESEND_API_KEY", "")
	if resendKey != "" {
		fromEmail := getEnv("RESEND_FROM_EMAIL", "picks@example.com")
		mailer = notify.NewResendMailer(resendKey, fromEmail, corsOrigin)
		slog.Info("notify: Resend mailer enabled")
	} else {
		mailer = notify.NoopMailer{}
		slog.Info("notify: RESEND_API_KEY not set, emails disabled")
	}

	// Connect to Redis if REDIS_URL is set; otherwise caches fall back to in-process.
	var rdb *redis.Client
	if redisURL := getEnv("REDIS_URL", ""); redisURL != "" {
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			slog.Error("redis: invalid URL", "err", err)
			os.Exit(1)
		}
		rdb = redis.NewClient(opt)
		if err := rdb.Ping(ctx).Err(); err != nil {
			slog.Error("redis: ping failed", "err", err)
			os.Exit(1)
		}
		defer rdb.Close()
		slog.Info("redis: connected")
	} else {
		slog.Info("redis: REDIS_URL not set, using in-process cache")
	}

	// ctx is cancelled on SIGINT/SIGTERM, which stops the scheduler loops.
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	lbCache := cache.NewLeaderboardCache(rdb)
	userCache := cache.NewUserCache(pool, rdb)

	syncer := espn.NewSyncer(pool)
	syncer.SetOnScored(lbCache.InvalidateScores)

	adminH := handlers.NewAdminHandler(pool, syncer, mailer, lbCache)

	sched := scheduler.New(pool, mailer, syncer)
	go sched.Start(ctx)

	router := api.NewRouter(pool, jwks, corsOrigin, lbCache, userCache, adminH)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("server listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Block until signal or server failure, then drain in-flight requests.
	select {
	case err := <-serverErr:
		slog.Error("server error", "err", err)
		return
	case <-ctx.Done():
	}
	stop() // release signal resources promptly

	slog.Info("server: shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server: forced shutdown", "err", err)
		os.Exit(1)
	}
	adminH.Shutdown() // wait for any in-flight notification sends
	slog.Info("server: stopped")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var is not set", "key", key)
		os.Exit(1)
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
