package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/evan/football-picks/internal/api/middleware"
	"github.com/evan/football-picks/internal/cache"
	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/draft"
	"github.com/evan/football-picks/internal/espn"
	"github.com/evan/football-picks/internal/models"
	"github.com/evan/football-picks/internal/notify"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminHandler struct {
	pool    *pgxpool.Pool
	syncer  *espn.Syncer
	mailer  notify.Mailer
	lbCache *cache.LeaderboardCache
	wg      sync.WaitGroup
}

func NewAdminHandler(pool *pgxpool.Pool, syncer *espn.Syncer, mailer notify.Mailer, lbCache *cache.LeaderboardCache) *AdminHandler {
	return &AdminHandler{pool: pool, syncer: syncer, mailer: mailer, lbCache: lbCache}
}

// Shutdown waits for all in-flight notification sends to complete.
func (h *AdminHandler) Shutdown() { h.wg.Wait() }

// POST /api/v1/admin/sync-games?week=1&season=2025&seasontype=2
func (h *AdminHandler) SyncGames(w http.ResponseWriter, r *http.Request) {
	seasonType := 2 // default regular season
	if st, err := strconv.Atoi(r.URL.Query().Get("seasontype")); err == nil {
		seasonType = st
	}

	week, ok := weekFromRequest(w, r, h.pool)
	if !ok {
		return
	}

	if err := h.syncer.SyncWeek(r.Context(), week, seasonType); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /api/v1/admin/score-week  (manual trigger; auto-scoring runs after each sync)
func (h *AdminHandler) ScoreWeek(w http.ResponseWriter, r *http.Request) {
	if err := queries.ScorePicks(r.Context(), h.pool); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.lbCache.InvalidateScores()
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /api/v1/admin/announcements
func (h *AdminHandler) CreateAnnouncement(w http.ResponseWriter, r *http.Request) {
	author := middleware.UserFromContext(r.Context())
	if author == nil {
		respondError(w, http.StatusUnauthorized, "user not found")
		return
	}

	var body struct {
		WeekNumber int    `json:"week_number"`
		SeasonYear int    `json:"season_year"`
		Intro      string `json:"intro"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(body.Intro) == "" {
		respondError(w, http.StatusBadRequest, "intro is required")
		return
	}
	if body.WeekNumber == 0 || body.SeasonYear == 0 {
		respondError(w, http.StatusBadRequest, "week_number and season_year are required")
		return
	}

	week, err := queries.GetWeekByNumberAndSeason(r.Context(), h.pool, body.WeekNumber, body.SeasonYear)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, http.StatusNotFound, "week not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	a, err := queries.CreateAnnouncement(r.Context(), h.pool, author.ID, week.ID, body.Intro)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Fire email notifications in the background so we don't block the response.
	h.wg.Go(func() { h.sendNotifications(a, week.ID) })

	respondJSON(w, http.StatusCreated, a)
}

// GET /api/v1/admin/draft-announcement?week=1&season=2025
func (h *AdminHandler) DraftAnnouncement(w http.ResponseWriter, r *http.Request) {
	week, ok := weekFromRequest(w, r, h.pool)
	if !ok {
		return
	}

	d, err := draft.BuildDraft(r.Context(), h.pool, week)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to build draft")
		return
	}
	respondJSON(w, http.StatusOK, d)
}

// GET /api/v1/admin/games?week=1&season=2025
func (h *AdminHandler) ListGames(w http.ResponseWriter, r *http.Request) {
	week, ok := weekFromRequest(w, r, h.pool)
	if !ok {
		return
	}

	games, err := queries.GetAllGamesByWeek(r.Context(), h.pool, week.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if games == nil {
		games = []models.Game{}
	}
	respondJSON(w, http.StatusOK, games)
}

// PATCH /api/v1/admin/games/{gameId}
func (h *AdminHandler) UpdateGame(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "gameId")
	gameID, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid game id")
		return
	}

	var body struct {
		IncludedInPicks *bool `json:"included_in_picks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.IncludedInPicks == nil {
		respondError(w, http.StatusBadRequest, "included_in_picks is required")
		return
	}

	game, err := queries.SetGameIncluded(r.Context(), h.pool, gameID, *body.IncludedInPicks)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not update game")
		return
	}
	respondJSON(w, http.StatusOK, game)
}

func (h *AdminHandler) sendNotifications(a *models.Announcement, weekID int) {
	ctx := context.Background()
	week, err := queries.GetWeekByID(ctx, h.pool, weekID)
	if err != nil {
		slog.Error("notify: get week", "err", err)
		return
	}
	users, err := queries.GetUsersForNotification(ctx, h.pool, weekID)
	if err != nil {
		slog.Error("notify: get users", "err", err)
		return
	}
	notify.SendAll(ctx, h.pool, h.mailer, users, a, week.WeekNumber, weekID)
}
