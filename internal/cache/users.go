package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/evan/football-picks/internal/db/queries"
	"github.com/evan/football-picks/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const userTTL = 60 * time.Second

type userEntry struct {
	user      *models.User
	expiresAt time.Time
}

// UserCache is a write-through TTL cache keyed by Supabase UID.
// Redis-backed when rdb is set (shared across instances); falls back to an
// in-process map for single-instance / dev use.
type UserCache struct {
	mu   sync.RWMutex
	pool *pgxpool.Pool
	data map[string]*userEntry
	rdb  *redis.Client // nil → in-process fallback
}

func NewUserCache(pool *pgxpool.Pool, rdb *redis.Client) *UserCache {
	return &UserCache{
		pool: pool,
		data: make(map[string]*userEntry),
		rdb:  rdb,
	}
}

func userKey(supabaseUID string) string {
	return fmt.Sprintf("fp:user:%s", supabaseUID)
}

func (c *UserCache) Get(ctx context.Context, supabaseUID string) (*models.User, error) {
	if c.rdb != nil {
		if user, ok := redisGet[*models.User](ctx, c.rdb, userKey(supabaseUID)); ok {
			return user, nil
		}
	} else {
		c.mu.RLock()
		e, ok := c.data[supabaseUID]
		c.mu.RUnlock()
		if ok && time.Now().Before(e.expiresAt) {
			return e.user, nil
		}
	}

	user, err := queries.GetUserBySupabaseUID(ctx, c.pool, supabaseUID)
	if err != nil {
		return nil, err
	}
	c.store(supabaseUID, user)
	return user, nil
}

func (c *UserCache) Set(supabaseUID string, user *models.User) {
	c.store(supabaseUID, user)
}

func (c *UserCache) Invalidate(supabaseUID string) {
	if c.rdb != nil {
		redisDel(context.Background(), c.rdb, userKey(supabaseUID))
		return
	}
	c.mu.Lock()
	delete(c.data, supabaseUID)
	c.mu.Unlock()
}

func (c *UserCache) store(supabaseUID string, user *models.User) {
	if c.rdb != nil {
		redisSet(context.Background(), c.rdb, userKey(supabaseUID), user, userTTL)
		return
	}
	c.mu.Lock()
	c.data[supabaseUID] = &userEntry{user: user, expiresAt: time.Now().Add(userTTL)}
	c.mu.Unlock()
}
