package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/evan/football-picks/internal/api/middleware"
	"github.com/evan/football-picks/internal/cache"
	"github.com/evan/football-picks/internal/db/queries"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UsersHandler struct {
	pool  *pgxpool.Pool
	users *cache.UserCache
}

func NewUsersHandler(pool *pgxpool.Pool, users *cache.UserCache) *UsersHandler {
	return &UsersHandler{pool: pool, users: users}
}

// POST /api/v1/auth/register
func (h *UsersHandler) Register(w http.ResponseWriter, r *http.Request) {
	uid := middleware.SupabaseUIDFromContext(r.Context())
	if uid == "" {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.DisplayName == "" || body.Email == "" {
		respondError(w, http.StatusBadRequest, "display_name and email are required")
		return
	}

	// Idempotent: if user already exists, return existing row.
	existing, err := h.users.Get(r.Context(), uid)
	if err == nil {
		respondJSON(w, http.StatusOK, existing)
		return
	}

	user, err := queries.CreateUser(r.Context(), h.pool, uid, body.DisplayName, body.Email)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not create user")
		return
	}
	h.users.Set(uid, user)
	respondJSON(w, http.StatusCreated, user)
}

// GET /api/v1/users/me
func (h *UsersHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	uid := middleware.SupabaseUIDFromContext(r.Context())
	user, err := h.users.Get(r.Context(), uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, http.StatusNotFound, "user not found; complete registration")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, user)
}

// PATCH /api/v1/users/me
func (h *UsersHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	uid := middleware.SupabaseUIDFromContext(r.Context())

	existing, err := h.users.Get(r.Context(), uid)
	if err != nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	var body struct {
		DisplayName *string `json:"display_name"`
		NotifyEmail *bool   `json:"notify_email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	displayName := existing.DisplayName
	notifyEmail := existing.NotifyEmail
	if body.DisplayName != nil {
		displayName = *body.DisplayName
	}
	if body.NotifyEmail != nil {
		notifyEmail = *body.NotifyEmail
	}

	updated, err := queries.UpdateUser(r.Context(), h.pool, existing.ID, displayName, notifyEmail)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not update user")
		return
	}
	h.users.Set(uid, updated)
	respondJSON(w, http.StatusOK, updated)
}
