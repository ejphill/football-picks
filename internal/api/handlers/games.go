package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GamesHandler struct {
	pool *pgxpool.Pool
}

func NewGamesHandler(pool *pgxpool.Pool) *GamesHandler {
	return &GamesHandler{pool: pool}
}

// GET /api/v1/weeks/active
func (h *GamesHandler) ActiveWeek(w http.ResponseWriter, r *http.Request) {
	week, err := queries.GetActiveWeek(r.Context(), h.pool)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, http.StatusNotFound, "no active season")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, week)
}

// GET /api/v1/games?week=1&season=2025
func (h *GamesHandler) List(w http.ResponseWriter, r *http.Request) {
	week, ok := weekFromRequest(w, r, h.pool)
	if !ok {
		return
	}

	games, err := queries.GetGamesByWeek(r.Context(), h.pool, week.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if games == nil {
		games = []models.Game{}
	}
	respondJSON(w, http.StatusOK, games)
}

// GET /api/v1/games/{gameId}
func (h *GamesHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "gameId")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid game id")
		return
	}

	game, err := queries.GetGameByID(r.Context(), h.pool, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, http.StatusNotFound, "game not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, game)
}
