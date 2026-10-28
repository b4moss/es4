package es4

import (
	"time"

	"github.com/b4moss/es4/packages/go/internal/state"
)

// SetOption configures an optional absolute expiry for Set.
type SetOption func(*setConfig)

type setConfig struct {
	expiresAt *time.Time
}

// WithExpiresAt sets an absolute UTC expiry for the written entry.
// Past times are stored successfully but treated as absent on read.
func WithExpiresAt(t time.Time) SetOption {
	return func(c *setConfig) {
		u := t.UTC()
		c.expiresAt = &u
	}
}

func applySetOptions(opts []SetOption) state.SetMeta {
	var c setConfig
	for _, o := range opts {
		if o != nil {
			o(&c)
		}
	}
	return state.SetMeta{ExpiresAt: c.expiresAt}
}
