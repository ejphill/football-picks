package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/models"
	"github.com/redis/go-redis/v9"
)

const (
	// safety net only — InvalidateScores fires after every ScorePicks run so
	// this rarely triggers; 1h ensures stale data doesn't persist if invalidation is missed
	weeklyScoresTTL = 1 * time.Hour

	// same rationale as weeklyScoresTTL
	seasonStandingsTTL = 1 * time.Hour
)

// LeaderboardCache wraps the two most expensive leaderboard queries.
// Redis-backed when rdb is set; in-process TTL otherwise.
type LeaderboardCache struct {
	weekly *ttlCache[int, []queries.WeeklyScoreRow]
	season *ttlCache[int, []models.SeasonLeaderboardEntry]
	rdb    *redis.Client // nil → in-process fallback
}

func NewLeaderboardCache(rdb *redis.Client) *LeaderboardCache {
	return &LeaderboardCache{
		weekly: newTTLCache[int, []queries.WeeklyScoreRow](weeklyScoresTTL),
		season: newTTLCache[int, []models.SeasonLeaderboardEntry](seasonStandingsTTL),
		rdb:    rdb,
	}
}

func weeklyKey(weekID int) string {
	return fmt.Sprintf("fp:lb:weekly:%d", weekID)
}

func seasonKey(year int) string {
	return fmt.Sprintf("fp:lb:season:%d", year)
}

func (c *LeaderboardCache) GetWeeklyScores(weekID int) ([]queries.WeeklyScoreRow, bool) {
	if c.rdb != nil {
		return redisGet[[]queries.WeeklyScoreRow](context.Background(), c.rdb, weeklyKey(weekID))
	}
	return c.weekly.get(weekID)
}

func (c *LeaderboardCache) SetWeeklyScores(weekID int, scores []queries.WeeklyScoreRow) {
	if c.rdb != nil {
		redisSet(context.Background(), c.rdb, weeklyKey(weekID), scores, weeklyScoresTTL)
		return
	}
	c.weekly.set(weekID, scores)
}

func (c *LeaderboardCache) GetSeasonStandings(year int) ([]models.SeasonLeaderboardEntry, bool) {
	if c.rdb != nil {
		return redisGet[[]models.SeasonLeaderboardEntry](context.Background(), c.rdb, seasonKey(year))
	}
	return c.season.get(year)
}

func (c *LeaderboardCache) SetSeasonStandings(year int, standings []models.SeasonLeaderboardEntry) {
	if c.rdb != nil {
		redisSet(context.Background(), c.rdb, seasonKey(year), standings, seasonStandingsTTL)
		return
	}
	c.season.set(year, standings)
}

// InvalidateScores clears all entries so the next request picks up fresh scores.
func (c *LeaderboardCache) InvalidateScores() {
	if c.rdb != nil {
		redisScanDel(context.Background(), c.rdb, "fp:lb:*")
		return
	}
	c.weekly.clear()
	c.season.clear()
}
