package cache

import (
	"strings"
	"testing"
)

func TestWeeklyKey(t *testing.T) {
	k := weeklyKey(7)
	if !strings.Contains(k, "7") {
		t.Errorf("weeklyKey(7) = %q, expected to contain week ID", k)
	}
}

func TestSeasonKey(t *testing.T) {
	k := seasonKey(2025)
	if !strings.Contains(k, "2025") {
		t.Errorf("seasonKey(2025) = %q, expected to contain year", k)
	}
}

func TestUserKey(t *testing.T) {
	k := userKey("uid-abc-123")
	if !strings.Contains(k, "uid-abc-123") {
		t.Errorf("userKey(%q) = %q, expected to contain uid", "uid-abc-123", k)
	}
}
