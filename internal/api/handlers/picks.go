package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/evan/football-picks/internal/api/middleware"
	"github.com/evan/football-picks/internal/cache"
	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PicksHandler struct {
	pool  *pgxpool.Pool
	users *cache.UserCache
}

func NewPicksHandler(pool *pgxpool.Pool, users *cache.UserCache) *PicksHandler {
	return &PicksHandler{pool: pool, users: users}
}

// GET /api/v1/picks?week=1&season=2025
func (h *PicksHandler) List(w http.ResponseWriter, r *http.Request) {
	uid := middleware.SupabaseUIDFromContext(r.Context())

	user, err := h.users.Get(r.Context(), uid)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "user not found")
		return
	}

	week, ok := weekFromRequest(w, r, h.pool)
	if !ok {
		return
	}

	picks, err := queries.GetPicksByUserAndWeek(r.Context(), h.pool, user.ID, week.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if picks == nil {
		picks = []models.Pick{}
	}
	respondJSON(w, http.StatusOK, picks)
}

// POST /api/v1/picks
// Body: { "game_id": "<uuid>", "picked_team": "home" | "away" }
func (h *PicksHandler) Submit(w http.ResponseWriter, r *http.Request) {
	uid := middleware.SupabaseUIDFromContext(r.Context())

	user, err := h.users.Get(r.Context(), uid)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "user not found")
		return
	}

	var body struct {
		GameID     string `json:"game_id"`
		PickedTeam string `json:"picked_team"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	gameID, err := uuid.Parse(body.GameID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid game_id")
		return
	}
	if body.PickedTeam != "home" && body.PickedTeam != "away" {
		respondError(w, http.StatusBadRequest, "picked_team must be 'home' or 'away'")
		return
	}

	pick, created, err := queries.UpsertPick(r.Context(), h.pool, user.ID, gameID, body.PickedTeam)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, http.StatusNotFound, "game not found")
			return
		}
		if errors.Is(err, queries.ErrPickLocked) {
			respondError(w, http.StatusLocked, "picks are locked for this game")
			return
		}
		respondError(w, http.StatusInternalServerError, "could not save pick")
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	respondJSON(w, status, pick)
}

// DELETE /api/v1/picks/{gameId}
func (h *PicksHandler) Delete(w http.ResponseWriter, r *http.Request) {
	uid := middleware.SupabaseUIDFromContext(r.Context())

	user, err := h.users.Get(r.Context(), uid)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "user not found")
		return
	}

	idStr := chi.URLParam(r, "gameId")
	gameID, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid game id")
		return
	}

	if err := queries.DeletePick(r.Context(), h.pool, user.ID, gameID); err != nil {
		if errors.Is(err, queries.ErrPickNotFound) {
			respondError(w, http.StatusNotFound, "pick not found")
			return
		}
		if errors.Is(err, queries.ErrPickLocked) {
			respondError(w, http.StatusLocked, "picks are locked for this game")
			return
		}
		respondError(w, http.StatusInternalServerError, "could not delete pick")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
