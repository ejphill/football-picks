package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/draft"
	"github.com/evan/football-picks/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AnnouncementsHandler struct {
	pool *pgxpool.Pool
}

func NewAnnouncementsHandler(pool *pgxpool.Pool) *AnnouncementsHandler {
	return &AnnouncementsHandler{pool: pool}
}

// GET /api/v1/announcements?season=2025
func (h *AnnouncementsHandler) List(w http.ResponseWriter, r *http.Request) {
	seasonYear, err := strconv.Atoi(r.URL.Query().Get("season"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "season query param required")
		return
	}

	list, err := queries.GetAnnouncementsBySeason(r.Context(), h.pool, seasonYear)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if list == nil {
		list = make([]models.Announcement, 0)
	}
	respondJSON(w, http.StatusOK, list)
}

// GET /api/v1/announcements/draft?week=1&season=2025
// Public (non-admin) draft preview — used by the Home page as a fallback so
// there's always something to show even if nobody's posted a real
// announcement for the active week yet. Unlike the real thing, this is
// never persisted or emailed, just generated on the fly from current data.
func (h *AnnouncementsHandler) Draft(w http.ResponseWriter, r *http.Request) {
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

// GET /api/v1/announcements/{id}
func (h *AnnouncementsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	a, err := queries.GetAnnouncementByID(r.Context(), h.pool, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, http.StatusNotFound, "announcement not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, a)
}
