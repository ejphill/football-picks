package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/evan/football-picks/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SeedUser(t *testing.T, pool *pgxpool.Pool, supabaseUID, displayName, email string) *models.User {
	t.Helper()
	u := &models.User{}
	err := pool.QueryRow(context.Background(), `
		INSERT INTO users (supabase_uid, display_name, email)
		VALUES ($1, $2, $3)
		RETURNING id, supabase_uid, display_name, email, notify_email, is_admin, created_at, updated_at
	`, supabaseUID, displayName, email).Scan(
		&u.ID, &u.SupabaseUID, &u.DisplayName, &u.Email,
		&u.NotifyEmail, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		t.Fatalf("testutil: seed user: %v", err)
	}
	return u
}

func SeedSeason(t *testing.T, pool *pgxpool.Pool, year int, active bool) *models.Season {
	t.Helper()
	s := &models.Season{}
	err := pool.QueryRow(context.Background(), `
		INSERT INTO seasons (year, is_active) VALUES ($1, $2)
		RETURNING id, year, is_active, created_at
	`, year, active).Scan(&s.ID, &s.Year, &s.IsActive, &s.CreatedAt)
	if err != nil {
		t.Fatalf("testutil: seed season: %v", err)
	}
	return s
}

func SeedWeek(t *testing.T, pool *pgxpool.Pool, seasonID, weekNumber int, picksLockAt time.Time) *models.Week {
	t.Helper()
	w := &models.Week{}
	err := pool.QueryRow(context.Background(), `
		INSERT INTO weeks (season_id, week_number, picks_lock_at) VALUES ($1, $2, $3)
		RETURNING id, season_id, week_number, picks_lock_at
	`, seasonID, weekNumber, picksLockAt).Scan(&w.ID, &w.SeasonID, &w.WeekNumber, &w.PicksLockAt)
	if err != nil {
		t.Fatalf("testutil: seed week: %v", err)
	}
	return w
}

// SeedGame seeds a game with kickoff one hour in the future.
func SeedGame(t *testing.T, pool *pgxpool.Pool, weekID int, espnID, homeTeam, awayTeam, status string, winner *string) *models.Game {
	t.Helper()
	return SeedGameAt(t, pool, weekID, espnID, homeTeam, awayTeam, status, winner, time.Now().Add(time.Hour))
}

// SeedGameAt seeds a game at a specific kickoff time.
func SeedGameAt(t *testing.T, pool *pgxpool.Pool, weekID int, espnID, homeTeam, awayTeam, status string, winner *string, kickoffAt time.Time) *models.Game {
	t.Helper()
	g := &models.Game{}
	err := pool.QueryRow(context.Background(), `
		INSERT INTO games (week_id, espn_game_id, home_team, away_team, home_team_name, away_team_name,
		                   kickoff_at, status, winner)
		VALUES ($1, $2, $3, $4, $3||' Name', $4||' Name', $5, $6, $7)
		RETURNING id, week_id, espn_game_id, home_team, away_team, home_team_name, away_team_name,
		          spread, kickoff_at, home_score, away_score, winner, status, created_at, updated_at
	`, weekID, espnID, homeTeam, awayTeam, kickoffAt, status, winner).Scan(
		&g.ID, &g.WeekID, &g.ESPNGameID, &g.HomeTeam, &g.AwayTeam,
		&g.HomeTeamName, &g.AwayTeamName, &g.Spread, &g.KickoffAt,
		&g.HomeScore, &g.AwayScore, &g.Winner, &g.Status,
		&g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		t.Fatalf("testutil: seed game at: %v", err)
	}
	return g
}

func SeedAnnouncement(t *testing.T, pool *pgxpool.Pool, authorID uuid.UUID, weekID int, intro string) *models.Announcement {
	t.Helper()
	a := &models.Announcement{}
	err := pool.QueryRow(context.Background(), `
		INSERT INTO announcements (author_id, week_id, intro)
		VALUES ($1, $2, $3)
		RETURNING id, author_id, week_id, intro, body_json, published_at, created_at
	`, authorID, weekID, intro).Scan(
		&a.ID, &a.AuthorID, &a.WeekID, &a.Intro, &a.BodyJSON, &a.PublishedAt, &a.CreatedAt,
	)
	if err != nil {
		t.Fatalf("testutil: seed announcement: %v", err)
	}
	return a
}

func SeedPick(t *testing.T, pool *pgxpool.Pool, userID, gameID uuid.UUID, pickedTeam string) *models.Pick {
	t.Helper()
	p := &models.Pick{}
	err := pool.QueryRow(context.Background(), `
		INSERT INTO picks (user_id, game_id, picked_team)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, game_id, picked_team, is_correct, created_at, updated_at
	`, userID, gameID, pickedTeam).Scan(
		&p.ID, &p.UserID, &p.GameID, &p.PickedTeam, &p.IsCorrect, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		t.Fatalf("testutil: seed pick: %v", err)
	}
	return p
}
