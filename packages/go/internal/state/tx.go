package state

import (
	"context"
	"encoding/json"
	"errors"
)

// Tx is the transactional State surface (separate from the non-Tx Store API).
// Key / value rules match Store (including optional SetMeta expiry). Nested
// Begin is rejected by Store.BeginTx.
type Tx interface {
	Set(ctx context.Context, key string, value json.RawMessage, meta ...SetMeta) error
	Get(ctx context.Context, key string) (json.RawMessage, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Clear(ctx context.Context) error
	// Scan lists keys visible in this transaction (overlay + base unless cleared).
	// Expired keys are omitted.
	Scan(ctx context.Context, prefix string) ([]string, error)
	Commit() error
	Rollback() error
}

// Tx-related sentinel errors.
var (
	ErrTxDone   = errors.New("state: transaction finished")
	ErrNestedTx = errors.New("state: nested transaction not supported")
)
