package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestRedis starts an in-process Redis server for use in tests.
func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestRedisGetSet(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	// Miss on empty cache.
	_, ok := redisGet[string](ctx, rdb, "fp:test:key")
	if ok {
		t.Error("expected miss on empty cache")
	}

	// Set then get.
	redisSet(ctx, rdb, "fp:test:key", "hello world", time.Minute)
	got, ok := redisGet[string](ctx, rdb, "fp:test:key")
	if !ok {
		t.Fatal("expected hit after set")
	}
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestRedisGetSet_Struct(t *testing.T) {
	type item struct {
		Name  string `json:"name"`
		Score int    `json:"score"`
	}
	rdb := newTestRedis(t)
	ctx := context.Background()

	redisSet(ctx, rdb, "fp:test:struct", item{Name: "Alice", Score: 5}, time.Minute)
	got, ok := redisGet[item](ctx, rdb, "fp:test:struct")
	if !ok {
		t.Fatal("expected hit")
	}
	if got.Name != "Alice" || got.Score != 5 {
		t.Errorf("got %+v", got)
	}
}

func TestRedisDel(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	redisSet(ctx, rdb, "fp:del:1", "val1", time.Minute)
	redisSet(ctx, rdb, "fp:del:2", "val2", time.Minute)

	redisDel(ctx, rdb, "fp:del:1", "fp:del:2")

	if _, ok := redisGet[string](ctx, rdb, "fp:del:1"); ok {
		t.Error("key 1 should be deleted")
	}
	if _, ok := redisGet[string](ctx, rdb, "fp:del:2"); ok {
		t.Error("key 2 should be deleted")
	}
}

func TestRedisScanDel(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	redisSet(ctx, rdb, "fp:lb:weekly:1", "data1", time.Minute)
	redisSet(ctx, rdb, "fp:lb:weekly:2", "data2", time.Minute)
	redisSet(ctx, rdb, "fp:other:key", "keep", time.Minute)

	redisScanDel(ctx, rdb, "fp:lb:*")

	if _, ok := redisGet[string](ctx, rdb, "fp:lb:weekly:1"); ok {
		t.Error("fp:lb:weekly:1 should be deleted")
	}
	if _, ok := redisGet[string](ctx, rdb, "fp:lb:weekly:2"); ok {
		t.Error("fp:lb:weekly:2 should be deleted")
	}
	if _, ok := redisGet[string](ctx, rdb, "fp:other:key"); !ok {
		t.Error("fp:other:key should not be deleted")
	}
}

func TestRedisSet_UnmarshalableValue(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()
	// A channel cannot be marshalled to JSON — redisSet should silently drop the error.
	redisSet(ctx, rdb, "fp:test:bad", make(chan int), time.Minute)
	// No panic, no crash.
}
