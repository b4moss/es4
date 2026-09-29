package server

import (
	"sync"

	"github.com/b4moss/es4/packages/go/pkg/es4"
	"github.com/google/uuid"
)

type txEntry struct {
	tx   es4.Tx
	done bool
}

// txRegistry tracks active and finished transactions by id.
type txRegistry struct {
	mu   sync.Mutex
	byID map[string]*txEntry
}

func newTxRegistry() *txRegistry {
	return &txRegistry{byID: make(map[string]*txEntry)}
}

func (r *txRegistry) begin(tx es4.Tx) string {
	id := uuid.NewString()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[id] = &txEntry{tx: tx}
	return id
}

// lookup returns the live Tx, or conflict/notFound signals.
func (r *txRegistry) lookup(id string) (tx es4.Tx, conflict bool, notFound bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok {
		return nil, false, true
	}
	if e.done {
		return nil, true, false
	}
	return e.tx, false, false
}

func (r *txRegistry) markDone(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.byID[id]; ok {
		e.done = true
		e.tx = nil
	}
}

func (r *txRegistry) rollbackAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, e := range r.byID {
		if e.done || e.tx == nil {
			continue
		}
		_ = e.tx.Rollback()
		e.done = true
		e.tx = nil
		_ = id
	}
}
