package es4

import (
	"context"
	"encoding/json"

	"github.com/b4moss/es4/packages/go/internal/state"
)

// Tx is the public transactional State surface (separate from the non-Tx API).
// Set accepts the same options as DB.Set (e.g. WithExpiresAt).
type Tx interface {
	Set(ctx context.Context, key string, value json.RawMessage, opts ...SetOption) error
	Get(ctx context.Context, key string) (json.RawMessage, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Clear(ctx context.Context) error
	Scan(ctx context.Context, prefix string) ([]string, error)
	Commit() error
	Rollback() error
}

type tx struct {
	inner state.Tx
}

func wrapTx(inner state.Tx) Tx {
	return &tx{inner: inner}
}

func (t *tx) Set(ctx context.Context, key string, value json.RawMessage, opts ...SetOption) error {
	return t.inner.Set(ctx, key, value, applySetOptions(opts))
}

func (t *tx) Get(ctx context.Context, key string) (json.RawMessage, error) {
	return t.inner.Get(ctx, key)
}

func (t *tx) Delete(ctx context.Context, key string) error {
	return t.inner.Delete(ctx, key)
}

func (t *tx) Exists(ctx context.Context, key string) (bool, error) {
	return t.inner.Exists(ctx, key)
}

func (t *tx) Clear(ctx context.Context) error {
	return t.inner.Clear(ctx)
}

func (t *tx) Scan(ctx context.Context, prefix string) ([]string, error) {
	return t.inner.Scan(ctx, prefix)
}

func (t *tx) Commit() error {
	return t.inner.Commit()
}

func (t *tx) Rollback() error {
	return t.inner.Rollback()
}
