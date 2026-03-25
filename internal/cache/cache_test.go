package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/evan/football-picks/internal/cache"
	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/models"
	"github.com/evan/football-picks/internal/testutil"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ── LeaderboardCache ────────────────────────────────────────────────────────

func TestLeaderboardCache_MissReturnsEmpty(t *testing.T) {
	c := cache.NewLeaderboardCache(nil)
	if _, ok := c.GetWeeklyScores(1); ok {
		t.Error("expected miss on empty cache")
	}
	if _, ok := c.GetSeasonStandings(2025); ok {
		t.Error("expected miss on empty season cache")
	}
}

func TestLeaderboardCache_SetAndGet(t *testing.T) {
	c := cache.NewLeaderboardCache(nil)

	scores := []queries.WeeklyScoreRow{{UserID: uuid.New(), DisplayName: "Alice", Correct: 3, Total: 5}}
	c.SetWeeklyScores(7, scores)

	got, ok := c.GetWeeklyScores(7)
	if !ok {
		t.Fatal("expected hit after set")
	}
	if len(got) != 1 || got[0].DisplayName != "Alice" {
		t.Errorf("unexpected cached value: %+v", got)
	}

	standings := []models.SeasonLeaderboardEntry{{Rank: 1, DisplayName: "Bob", Correct: 10, Total: 12}}
	c.SetSeasonStandings(2025, standings)
	gotS, ok := c.GetSeasonStandings(2025)
	if !ok {
		t.Fatal("expected season hit after set")
	}
	if gotS[0].DisplayName != "Bob" {
		t.Errorf("unexpected standing: %+v", gotS[0])
	}
}

func TestLeaderboardCache_Invalidate(t *testing.T) {
	c := cache.NewLeaderboardCache(nil)

	scores := []queries.WeeklyScoreRow{{UserID: uuid.New(), Correct: 1, Total: 1}}
	c.SetWeeklyScores(3, scores)
	c.SetSeasonStandings(2025, []models.SeasonLeaderboardEntry{{Rank: 1}})

	c.InvalidateScores()

	if _, ok := c.GetWeeklyScores(3); ok {
		t.Error("weekly scores should be gone after invalidation")
	}
	if _, ok := c.GetSeasonStandings(2025); ok {
		t.Error("season standings should be gone after invalidation")
	}
}

func TestLeaderboardCache_DifferentWeeks(t *testing.T) {
	c := cache.NewLeaderboardCache(nil)
	c.SetWeeklyScores(1, []queries.WeeklyScoreRow{{Correct: 1}})
	c.SetWeeklyScores(2, []queries.WeeklyScoreRow{{Correct: 2}})

	w1, ok1 := c.GetWeeklyScores(1)
	w2, ok2 := c.GetWeeklyScores(2)
	if !ok1 || !ok2 {
		t.Fatal("both week caches should be present")
	}
	if w1[0].Correct == w2[0].Correct {
		t.Error("different week IDs should have independent cache entries")
	}
}

// ── UserCache (in-process path) ────────────────────────────────────────────

func TestUserCache_SetAndGetFromCache(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	u := testutil.SeedUser(t, pool, "uid-uc-1", "CacheUser", "cache@test.com")

	c := cache.NewUserCache(pool, nil)
	c.Set(u.SupabaseUID, u)

	// Get should return from in-process cache without DB round-trip.
	got, err := c.Get(context.Background(), u.SupabaseUID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("got user %v, want %v", got.ID, u.ID)
	}
}

func TestUserCache_GetMissHitsDB(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	u := testutil.SeedUser(t, pool, "uid-uc-2", "DBUser", "db@test.com")
	c := cache.NewUserCache(pool, nil)

	// No prior Set — should fall through to DB.
	got, err := c.Get(context.Background(), u.SupabaseUID)
	if err != nil {
		t.Fatalf("Get from DB: %v", err)
	}
	if got.DisplayName != u.DisplayName {
		t.Errorf("display name: got %q, want %q", got.DisplayName, u.DisplayName)
	}
}

func TestUserCache_Invalidate(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	u := testutil.SeedUser(t, pool, "uid-uc-3", "InvUser", "inv@test.com")
	c := cache.NewUserCache(pool, nil)
	c.Set(u.SupabaseUID, u)

	c.Invalidate(u.SupabaseUID)

	// After invalidation, Get should re-query the DB (still succeeds since user exists).
	got, err := c.Get(context.Background(), u.SupabaseUID)
	if err != nil {
		t.Fatalf("Get after invalidate: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("got unexpected user after invalidate")
	}
}

// can't test expiry without exporting the TTL; just verify non-expired reads work
func TestUserCache_FreshEntryIsReturned(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)

	u := testutil.SeedUser(t, pool, "uid-uc-4", "Fresh", "fresh@test.com")
	c := cache.NewUserCache(pool, nil)
	c.Set(u.SupabaseUID, u)

	start := time.Now()
	got, err := c.Get(context.Background(), u.SupabaseUID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if time.Since(start) > 50*time.Millisecond {
		t.Error("in-process cache get should be near-instant")
	}
	if got.ID != u.ID {
		t.Errorf("unexpected user ID")
	}
}

// ── LeaderboardCache with Redis ─────────────────────────────────────────────

func newTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestLeaderboardCache_Redis_SetAndGet(t *testing.T) {
	rdb := newTestRedisClient(t)
	c := cache.NewLeaderboardCache(rdb)

	scores := []queries.WeeklyScoreRow{{UserID: uuid.New(), DisplayName: "RedisUser", Correct: 4, Total: 6}}
	c.SetWeeklyScores(7, scores)

	got, ok := c.GetWeeklyScores(7)
	if !ok {
		t.Fatal("expected Redis cache hit after set")
	}
	if len(got) != 1 || got[0].DisplayName != "RedisUser" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestLeaderboardCache_Redis_SeasonSetAndGet(t *testing.T) {
	rdb := newTestRedisClient(t)
	c := cache.NewLeaderboardCache(rdb)

	standings := []models.SeasonLeaderboardEntry{{Rank: 1, DisplayName: "RedisSeason", Correct: 10, Total: 12}}
	c.SetSeasonStandings(2025, standings)

	got, ok := c.GetSeasonStandings(2025)
	if !ok {
		t.Fatal("expected Redis season hit after set")
	}
	if got[0].DisplayName != "RedisSeason" {
		t.Errorf("unexpected standing: %+v", got[0])
	}
}

// ── UserCache with Redis ──────────────────────────────────────────────────────

func TestUserCache_Redis_SetAndGet(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)
	rdb := newTestRedisClient(t)

	u := testutil.SeedUser(t, pool, "uid-ucr-1", "RedisUser", "redis@test.com")
	c := cache.NewUserCache(pool, rdb)
	c.Set(u.SupabaseUID, u)

	got, err := c.Get(context.Background(), u.SupabaseUID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("user ID mismatch: got %v, want %v", got.ID, u.ID)
	}
}

func TestUserCache_Redis_Invalidate(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)
	rdb := newTestRedisClient(t)

	u := testutil.SeedUser(t, pool, "uid-ucr-2", "InvRedis", "invredis@test.com")
	c := cache.NewUserCache(pool, rdb)
	c.Set(u.SupabaseUID, u)

	c.Invalidate(u.SupabaseUID)

	// After invalidation, Get should re-query the DB (still succeeds since user exists).
	got, err := c.Get(context.Background(), u.SupabaseUID)
	if err != nil {
		t.Fatalf("Get after invalidate: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("unexpected user after invalidate")
	}
}

func TestUserCache_Redis_MissHitsDB(t *testing.T) {
	pool := testutil.NewTestDB(t)
	testutil.ResetDB(t, pool)
	rdb := newTestRedisClient(t)

	u := testutil.SeedUser(t, pool, "uid-ucr-3", "DBFallback", "dbfallback@test.com")
	c := cache.NewUserCache(pool, rdb)

	// No prior Set — should fall through to DB.
	got, err := c.Get(context.Background(), u.SupabaseUID)
	if err != nil {
		t.Fatalf("Get from DB: %v", err)
	}
	if got.DisplayName != u.DisplayName {
		t.Errorf("display name: got %q, want %q", got.DisplayName, u.DisplayName)
	}
}

func TestLeaderboardCache_Redis_Invalidate(t *testing.T) {
	rdb := newTestRedisClient(t)
	c := cache.NewLeaderboardCache(rdb)

	c.SetWeeklyScores(3, []queries.WeeklyScoreRow{{Correct: 1}})
	c.SetSeasonStandings(2025, []models.SeasonLeaderboardEntry{{Rank: 1}})

	c.InvalidateScores()

	if _, ok := c.GetWeeklyScores(3); ok {
		t.Error("Redis: weekly scores should be gone after invalidation")
	}
	if _, ok := c.GetSeasonStandings(2025); ok {
		t.Error("Redis: season standings should be gone after invalidation")
	}
}
