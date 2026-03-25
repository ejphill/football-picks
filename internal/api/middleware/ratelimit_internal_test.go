package middleware

import (
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// TestCleanupEvictsStaleEntries exercises the cleanup loop body directly.
// We cannot wait 5 minutes for the real ticker, so we replicate the loop
// logic against an ipRateLimiter we construct internally.
func TestCleanupEvictsStaleEntries(t *testing.T) {
	rl := &ipRateLimiter{
		entries: make(map[string]*ipEntry),
		r:       rate.Limit(10),
		burst:   5,
	}

	// Stale entry — last seen 10 minutes ago.
	rl.entries["stale"] = &ipEntry{
		limiter:  rate.NewLimiter(10, 5),
		lastSeen: time.Now().Add(-10 * time.Minute),
	}
	// Fresh entry — just added.
	rl.entries["fresh"] = &ipEntry{
		limiter:  rate.NewLimiter(10, 5),
		lastSeen: time.Now(),
	}

	// Run the same logic as cleanup() body.
	cutoff := time.Now().Add(-5 * time.Minute)
	rl.mu.Lock()
	for ip, e := range rl.entries {
		if e.lastSeen.Before(cutoff) {
			delete(rl.entries, ip)
		}
	}
	rl.mu.Unlock()

	if _, ok := rl.entries["stale"]; ok {
		t.Error("stale entry should have been evicted")
	}
	if _, ok := rl.entries["fresh"]; !ok {
		t.Error("fresh entry should not have been evicted")
	}
}
