package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID `json:"id"`
	SupabaseUID string    `json:"supabase_uid"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
	NotifyEmail bool      `json:"notify_email"`
	IsAdmin     bool      `json:"is_admin"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Season struct {
	ID        int       `json:"id"`
	Year      int       `json:"year"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// SeasonYear is 0 when Week is constructed without a season JOIN.
type Week struct {
	ID          int       `json:"id"`
	SeasonID    int       `json:"season_id"`
	SeasonYear  int       `json:"season_year"`
	WeekNumber  int       `json:"week_number"`
	PicksLockAt time.Time `json:"picks_lock_at"`
}

type Game struct {
	ID           uuid.UUID `json:"id"`
	WeekID       int       `json:"week_id"`
	ESPNGameID   string    `json:"espn_game_id"`
	HomeTeam     string    `json:"home_team"`
	AwayTeam     string    `json:"away_team"`
	HomeTeamName string    `json:"home_team_name"`
	AwayTeamName string    `json:"away_team_name"`
	Spread       *float64  `json:"spread,omitempty"`
	KickoffAt    time.Time `json:"kickoff_at"`
	HomeScore    *int      `json:"home_score,omitempty"`
	AwayScore    *int      `json:"away_score,omitempty"`
	Winner       *string   `json:"winner,omitempty"`
	Status          string    `json:"status"`
	IncludedInPicks bool      `json:"included_in_picks"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Pick struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	GameID     uuid.UUID `json:"game_id"`
	PickedTeam string    `json:"picked_team"`
	IsCorrect  *bool     `json:"is_correct,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Announcement struct {
	ID          uuid.UUID  `json:"id"`
	AuthorID    uuid.UUID  `json:"author_id"`
	WeekID      int        `json:"week_id"`
	Intro       string     `json:"intro"`
	BodyJSON    *string    `json:"body_json,omitempty"`
	PublishedAt time.Time  `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

type NotificationLog struct {
	ID       uuid.UUID `json:"id"`
	UserID   uuid.UUID `json:"user_id"`
	WeekID   int       `json:"week_id"`
	SentAt   time.Time `json:"sent_at"`
	Success  bool      `json:"success"`
	ErrorMsg *string   `json:"error_msg,omitempty"`
}

type WeeklyLeaderboardEntry struct {
	UserID      uuid.UUID  `json:"user_id"`
	DisplayName string     `json:"display_name"`
	Picks       []PickView `json:"picks"`
	Correct     int        `json:"correct"`
	Total       int        `json:"total"`
}

// PickView is a pick as seen on the leaderboard — picked_team is "" before lock time.
type PickView struct {
	GameID     uuid.UUID `json:"game_id"`
	PickedTeam string    `json:"picked_team"` // "" when hidden
	IsCorrect  *bool     `json:"is_correct,omitempty"`
}

type SeasonLeaderboardEntry struct {
	Rank        int       `json:"rank"`
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	Correct     int       `json:"correct"`
	Total       int       `json:"total"`
	WinPct      float64   `json:"win_pct"`
}
