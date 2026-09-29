// Package recovery defines the Recovery Storage adapters.
//
// Snapshot envelopes are stored as opaque byte blobs (logical entries).
// Adapters are generation-aware: Save writes a new generation then prunes;
// Load returns the latest generation (ErrNotFound when none).
package recovery

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// ErrNotFound indicates no recovery generation is available.
var ErrNotFound = errors.New("recovery: not found")

// Store is the swappable Recovery adapter surface.
type Store interface {
	// Save persists data as a new generation, then prunes per TTL policy.
	Save(ctx context.Context, data []byte) error
	// Load reads the latest recovery generation. Returns ErrNotFound when missing.
	Load(ctx context.Context) ([]byte, error)
}

// genID formats a UTC timestamp as a zero-padded Unix-nano string (lexicographic order).
func genID(t time.Time) string {
	return fmt.Sprintf("%020d", t.UTC().UnixNano())
}

// parseGenID parses a generation id produced by genID.
func parseGenID(id string) (time.Time, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("recovery: bad generation id %q: %w", id, err)
	}
	return time.Unix(0, n).UTC(), nil
}

// keepGeneration reports whether a generation created at created should be kept
// given now and ttl. ttl<=0 keeps only the latest (caller deletes all but newest).
// ttl>0 keeps generations with age <= ttl.
func keepGeneration(created, now time.Time, ttl time.Duration) bool {
	if ttl <= 0 {
		return false
	}
	return !created.Before(now.Add(-ttl))
}
