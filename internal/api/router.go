package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/evan/football-picks/internal/api/handlers"
	"github.com/evan/football-picks/internal/api/middleware"
	"github.com/evan/football-picks/internal/cache"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"
)

func NewRouter(pool *pgxpool.Pool, jwks keyfunc.Keyfunc, corsOrigin string, lbCache *cache.LeaderboardCache, userCache *cache.UserCache, adminH *handlers.AdminHandler) http.Handler {
	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(middleware.RequestLogger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		// AllowOriginFunc lets any localhost port through (Vite may use 5173, 5174, etc.)
		// and still enforces the production origin exactly.
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			if strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://127.0.0.1:") {
				return true
			}
			return origin == corsOrigin
		},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	// Health check — no auth. Pings the DB to report degraded status.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"degraded"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	usersH := handlers.NewUsersHandler(pool, userCache)
	gamesH := handlers.NewGamesHandler(pool)
	picksH := handlers.NewPicksHandler(pool, userCache)
	lbH := handlers.NewLeaderboardHandler(pool, userCache, lbCache)
	announcementsH := handlers.NewAnnouncementsHandler(pool)

	// 30 req/min per IP, burst 10
	picksLimiter := middleware.NewRateLimiter(rate.Every(2*time.Second), 10)
	// 5 req/min per IP, burst 3
	registerLimiter := middleware.NewRateLimiter(rate.Every(12*time.Second), 3)

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Auth(jwks))
		r.Use(middleware.Timeout(5 * time.Second))

		// Auth / Users
		r.With(registerLimiter).Post("/auth/register", usersH.Register)
		r.Get("/users/me", usersH.GetMe)
		r.Patch("/users/me", usersH.UpdateMe)

		// Weeks
		r.Get("/weeks/active", gamesH.ActiveWeek)

		// Games
		r.Get("/games", gamesH.List)
		r.Get("/games/{gameId}", gamesH.Get)

		// Picks
		r.Get("/picks", picksH.List)
		r.With(picksLimiter).Post("/picks", picksH.Submit)
		r.With(picksLimiter).Delete("/picks/{gameId}", picksH.Delete)

		// Leaderboard
		r.With(middleware.ETag).Get("/leaderboard/weekly", lbH.Weekly)
		r.With(middleware.ETag).Get("/leaderboard/season", lbH.Season)

		// Announcements
		r.Get("/announcements", announcementsH.List)
		r.Get("/announcements/{id}", announcementsH.Get)

		// Admin-only routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.AdminOnly(userCache))
			// sync-games and draft-announcement call ESPN; give them a longer deadline.
			r.With(middleware.Timeout(30*time.Second)).Post("/admin/sync-games", adminH.SyncGames)
			r.With(middleware.Timeout(30*time.Second)).Get("/admin/draft-announcement", adminH.DraftAnnouncement)
			r.Post("/admin/score-week", adminH.ScoreWeek)
			r.Post("/admin/announcements", adminH.CreateAnnouncement)
			r.Get("/admin/games", adminH.ListGames)
			r.Patch("/admin/games/{gameId}", adminH.UpdateGame)
		})
	})

	return r
}
