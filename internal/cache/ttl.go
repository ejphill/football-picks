package cache

import (
	"sync"
	"time"
)

// ttlCache is a generic, concurrency-safe in-process TTL cache.
type ttlCache[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]*ttlEntry[V]
	ttl  time.Duration
}

type ttlEntry[V any] struct {
	value     V
	expiresAt time.Time
}

func newTTLCache[K comparable, V any](ttl time.Duration) *ttlCache[K, V] {
	return &ttlCache[K, V]{data: make(map[K]*ttlEntry[V]), ttl: ttl}
}

func (c *ttlCache[K, V]) get(key K) (V, bool) {
	c.mu.RLock()
	e, ok := c.data[key]
	c.mu.RUnlock()
	if !ok {
		var zero V
		return zero, false
	}
	if time.Now().After(e.expiresAt) {
		// Delete on stale read so old-week entries don't linger after TTL.
		c.mu.Lock()
		delete(c.data, key)
		c.mu.Unlock()
		var zero V
		return zero, false
	}
	return e.value, true
}

func (c *ttlCache[K, V]) set(key K, val V) {
	c.mu.Lock()
	c.data[key] = &ttlEntry[V]{value: val, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *ttlCache[K, V]) clear() {
	c.mu.Lock()
	c.data = make(map[K]*ttlEntry[V])
	c.mu.Unlock()
}

// purgeExpired removes entries whose TTL has elapsed without clearing live ones.
func (c *ttlCache[K, V]) purgeExpired() {
	now := time.Now()
	c.mu.Lock()
	for k, e := range c.data {
		if now.After(e.expiresAt) {
			delete(c.data, k)
		}
	}
	c.mu.Unlock()
}
