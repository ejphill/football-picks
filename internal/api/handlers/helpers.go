package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// weekFromRequest parses ?week=&season= and fetches the week row.
// On error it writes the HTTP response and returns nil, false.
func weekFromRequest(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) (*models.Week, bool) {
	weekNum, err := strconv.Atoi(r.URL.Query().Get("week"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "week query param required")
		return nil, false
	}
	seasonYear, err := strconv.Atoi(r.URL.Query().Get("season"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "season query param required")
		return nil, false
	}
	week, err := queries.GetWeekByNumberAndSeason(r.Context(), pool, weekNum, seasonYear)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, http.StatusNotFound, "week not found")
		} else {
			respondError(w, http.StatusInternalServerError, "internal error")
		}
		return nil, false
	}
	return week, true
}
