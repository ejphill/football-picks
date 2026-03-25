package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/evan/football-picks/internal/api/handlers"
	"github.com/evan/football-picks/internal/models"
	"github.com/evan/football-picks/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAnnouncementsList(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("returns announcements for season", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		author := testutil.SeedUser(t, pool, "uid-al-1", "Author", "author@test.com")
		testutil.SeedAnnouncement(t, pool, author.ID, week.ID, "Week 1 is here!")

		rr := doAnnouncementsList(t, pool, 2025)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
		var list []models.Announcement
		json.Unmarshal(rr.Body.Bytes(), &list)
		if len(list) != 1 {
			t.Errorf("expected 1 announcement, got %d", len(list))
		}
		if list[0].Intro != "Week 1 is here!" {
			t.Errorf("intro: got %q", list[0].Intro)
		}
	})

	t.Run("returns empty array when none exist", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doAnnouncementsList(t, pool, 2025)
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d", rr.Code)
		}
		var list []models.Announcement
		json.Unmarshal(rr.Body.Bytes(), &list)
		if len(list) != 0 {
			t.Errorf("expected empty list, got %d", len(list))
		}
	})

	t.Run("400 for missing season param", func(t *testing.T) {
		rr := doAnnouncementsListRaw(t, pool, "")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})
}

func TestAnnouncementsGet(t *testing.T) {
	pool := testutil.NewTestDB(t)

	t.Run("returns announcement by id", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		season := testutil.SeedSeason(t, pool, 2025, true)
		week := testutil.SeedWeek(t, pool, season.ID, 1, time.Now().Add(time.Hour))
		author := testutil.SeedUser(t, pool, "uid-ag-1", "Author2", "author2@test.com")
		a := testutil.SeedAnnouncement(t, pool, author.ID, week.ID, "Hello!")

		rr := doAnnouncementsGet(t, pool, a.ID.String())
		if rr.Code != http.StatusOK {
			t.Fatalf("status: got %d — body: %s", rr.Code, rr.Body.String())
		}
		var got models.Announcement
		json.Unmarshal(rr.Body.Bytes(), &got)
		if got.Intro != "Hello!" {
			t.Errorf("intro: got %q, want Hello!", got.Intro)
		}
	})

	t.Run("404 for unknown id", func(t *testing.T) {
		testutil.ResetDB(t, pool)
		rr := doAnnouncementsGet(t, pool, "00000000-0000-0000-0000-000000000000")
		if rr.Code != http.StatusNotFound {
			t.Errorf("status: got %d, want 404", rr.Code)
		}
	})

	t.Run("400 for invalid uuid", func(t *testing.T) {
		rr := doAnnouncementsGet(t, pool, "not-a-uuid")
		if rr.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rr.Code)
		}
	})
}

// helpers

func doAnnouncementsList(t *testing.T, pool *pgxpool.Pool, season int) *httptest.ResponseRecorder {
	t.Helper()
	return doAnnouncementsListRaw(t, pool, fmt.Sprintf("?season=%d", season))
}

func doAnnouncementsListRaw(t *testing.T, pool *pgxpool.Pool, query string) *httptest.ResponseRecorder {
	t.Helper()
	ah := handlers.NewAnnouncementsHandler(pool)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/announcements"+query, nil)
	rr := httptest.NewRecorder()
	ah.List(rr, r)
	return rr
}

func doAnnouncementsGet(t *testing.T, pool *pgxpool.Pool, id string) *httptest.ResponseRecorder {
	t.Helper()
	ah := handlers.NewAnnouncementsHandler(pool)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/announcements/"+id, nil)
	rr := httptest.NewRecorder()
	router := chi.NewRouter()
	router.Get("/api/v1/announcements/{id}", ah.Get)
	router.ServeHTTP(rr, r)
	return rr
}
