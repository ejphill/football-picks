package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/evan/football-picks/internal/api/middleware"
	"github.com/evan/football-picks/internal/cache"
	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LeaderboardHandler struct {
	pool  *pgxpool.Pool
	users *cache.UserCache
	lb    *cache.LeaderboardCache
}

func NewLeaderboardHandler(pool *pgxpool.Pool, users *cache.UserCache, lb *cache.LeaderboardCache) *LeaderboardHandler {
	return &LeaderboardHandler{pool: pool, users: users, lb: lb}
}

const (
	defaultLimit = 200
	maxLimit     = 500
)

func parsePagination(r *http.Request) (limit, offset int) {
	limit = defaultLimit
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = v
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v >= 0 {
		offset = v
	}
	return
}

// GET /api/v1/leaderboard/weekly?week=1&season=2025&limit=50&offset=0
func (h *LeaderboardHandler) Weekly(w http.ResponseWriter, r *http.Request) {
	uid := middleware.SupabaseUIDFromContext(r.Context())

	currentUser, err := h.users.Get(r.Context(), uid)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "user not found")
		return
	}

	limit, offset := parsePagination(r)

	week, ok := weekFromRequest(w, r, h.pool)
	if !ok {
		return
	}

	locked := time.Now().After(week.PicksLockAt)

	games, err := queries.GetGamesByWeek(r.Context(), h.pool, week.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// served from cache when available; full list cached, pagination in-memory
	scores, ok := h.lb.GetWeeklyScores(week.ID)
	if !ok {
		scores, err = queries.GetWeeklyLeaderboardScores(r.Context(), h.pool, week.ID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		h.lb.SetWeeklyScores(week.ID, scores)
	}

	total := len(scores)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	pageScores := scores[offset:end]

	// Fetch pick views only for the users on this page.
	pageUserIDs := make([]uuid.UUID, len(pageScores))
	for i, s := range pageScores {
		pageUserIDs[i] = s.UserID
	}
	pagePicks, err := queries.GetPicksForUsersAndWeek(r.Context(), h.pool, week.ID, pageUserIDs)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Build per-user pick views with visibility gating.
	type pickList = []models.PickView
	userPicks := map[uuid.UUID]pickList{}
	for _, row := range pagePicks {
		pv := models.PickView{GameID: row.GameID}
		if locked || row.UserID == currentUser.ID {
			pv.PickedTeam = row.PickedTeam
			pv.IsCorrect = row.IsCorrect
		}
		userPicks[row.UserID] = append(userPicks[row.UserID], pv)
	}

	entries := make([]models.WeeklyLeaderboardEntry, 0, len(pageScores))
	for _, s := range pageScores {
		picks := userPicks[s.UserID]
		if picks == nil {
			picks = []models.PickView{}
		}
		entries = append(entries, models.WeeklyLeaderboardEntry{
			UserID:      s.UserID,
			DisplayName: s.DisplayName,
			Picks:       picks,
			Correct:     s.Correct,
			Total:       s.Total,
		})
	}

	if games == nil {
		games = []models.Game{}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"locked":  locked,
		"games":   games,
		"entries": entries,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// GET /api/v1/leaderboard/season?season=2025&limit=50&offset=0
func (h *LeaderboardHandler) Season(w http.ResponseWriter, r *http.Request) {
	seasonYear, err := strconv.Atoi(r.URL.Query().Get("season"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "season query param required")
		return
	}
	limit, offset := parsePagination(r)

	standings, ok := h.lb.GetSeasonStandings(seasonYear)
	if !ok {
		standings, err = queries.GetSeasonStandings(r.Context(), h.pool, seasonYear)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		h.lb.SetSeasonStandings(seasonYear, standings)
	}

	total := len(standings)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := standings[offset:end]
	if page == nil {
		page = []models.SeasonLeaderboardEntry{}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"entries": page,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}
