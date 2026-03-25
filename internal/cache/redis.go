package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// returns (zero, false) on miss or unmarshal error
func redisGet[V any](ctx context.Context, rdb *redis.Client, key string) (V, bool) {
	var zero V
	b, err := rdb.Get(ctx, key).Bytes()
	if err != nil {
		return zero, false
	}
	var v V
	if err := json.Unmarshal(b, &v); err != nil {
		return zero, false
	}
	return v, true
}

// cache writes are best-effort; errors are dropped
func redisSet[V any](ctx context.Context, rdb *redis.Client, key string, val V, ttl time.Duration) {
	b, err := json.Marshal(val)
	if err != nil {
		return
	}
	_ = rdb.Set(ctx, key, b, ttl).Err()
}

func redisDel(ctx context.Context, rdb *redis.Client, keys ...string) {
	_ = rdb.Del(ctx, keys...).Err()
}

// uses SCAN to avoid blocking; fine for small key spaces like leaderboard entries
func redisScanDel(ctx context.Context, rdb *redis.Client, pattern string) {
	var cursor uint64
	for {
		keys, next, err := rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return
		}
		if len(keys) > 0 {
			_ = rdb.Del(ctx, keys...).Err()
		}
		cursor = next
		if cursor == 0 {
			return
		}
	}
}
