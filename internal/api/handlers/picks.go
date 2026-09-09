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

// PickResult is one item's outcome from a batch submit — exactly one of
// Pick/Error is set, letting some picks in a batch succeed while others
// (e.g. a game that just locked) fail independently.
type PickResult struct {
	GameID string       `json:"game_id"`
	Pick   *models.Pick `json:"pick,omitempty"`
	Error  string       `json:"error,omitempty"`
}

// POST /api/v1/picks
// Body: [{ "game_id": "<uuid>", "picked_team": "home" | "away" }, ...]
// Each pick is validated and locked independently, so one locked/invalid
// game in the batch doesn't block the others from saving.
func (h *PicksHandler) Submit(w http.ResponseWriter, r *http.Request) {
	uid := middleware.SupabaseUIDFromContext(r.Context())

	user, err := h.users.Get(r.Context(), uid)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "user not found")
		return
	}

	var body []struct {
		GameID     string `json:"game_id"`
		PickedTeam string `json:"picked_team"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(body) == 0 {
		respondError(w, http.StatusBadRequest, "at least one pick is required")
		return
	}

	results := make([]PickResult, len(body))
	for i, item := range body {
		results[i].GameID = item.GameID

		gameID, err := uuid.Parse(item.GameID)
		if err != nil {
			results[i].Error = "invalid game_id"
			continue
		}
		if item.PickedTeam != "home" && item.PickedTeam != "away" {
			results[i].Error = "picked_team must be 'home' or 'away'"
			continue
		}

		pick, _, err := queries.UpsertPick(r.Context(), h.pool, user.ID, gameID, item.PickedTeam)
		if err != nil {
			switch {
			case errors.Is(err, pgx.ErrNoRows):
				results[i].Error = "game not found"
			case errors.Is(err, queries.ErrPickLocked):
				results[i].Error = "picks are locked for this game"
			case errors.Is(err, queries.ErrPickNotIncluded):
				results[i].Error = "this game is not part of picks this week"
			default:
				results[i].Error = "could not save pick"
			}
			continue
		}
		results[i].Pick = pick
	}

	respondJSON(w, http.StatusOK, results)
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
