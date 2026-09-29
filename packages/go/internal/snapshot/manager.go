// Package snapshot implements the internal Snapshot envelope and manager.
// Snapshot is not part of the public API surface.
package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/b4moss/es4/packages/go/internal/recovery"
	"github.com/b4moss/es4/packages/go/internal/state"
)

// CurrentVersion is the Phase 1 envelope version. v0 has no backward-compat promise.
const CurrentVersion = 1

// Envelope is the versioned common wrapper around a backend-specific payload.
type Envelope struct {
	Version   int             `json:"version"`
	CreatedAt time.Time       `json:"created_at"`
	Payload   json.RawMessage `json:"payload"`
}

// MemoryPayload is the logical entries payload used by Memory and SQLite
// State adapters (`{ "entries": { ... } }`). Snapshot does not copy DB files.
type MemoryPayload struct {
	Entries map[string]json.RawMessage `json:"entries"`
}

// Manager takes Snapshots from a State Store and persists them via Recovery.
type Manager struct {
	state    state.Store
	recovery recovery.Store // may be nil when recovery writes are disabled
	interval time.Duration

	mu      sync.Mutex
	ticker  *time.Ticker
	stopCh  chan struct{}
	doneCh  chan struct{}
	running bool
	clock   func() time.Time
}

// Config configures a Snapshot Manager.
type Config struct {
	State    state.Store
	Recovery recovery.Store // nil disables persistence writes
	Interval time.Duration  // <=0 disables periodic snapshots
	Clock    func() time.Time
}

// NewManager builds a Snapshot Manager. Call Start to begin periodic snapshots.
func NewManager(cfg Config) *Manager {
	clock := cfg.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Manager{
		state:    cfg.State,
		recovery: cfg.Recovery,
		interval: cfg.Interval,
		clock:    clock,
	}
}

// Start begins the periodic snapshot loop when Interval > 0.
// Idempotent if already running.
func (m *Manager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running || m.interval <= 0 {
		return
	}
	m.stopCh = make(chan struct{})
	m.doneCh = make(chan struct{})
	m.ticker = time.NewTicker(m.interval)
	m.running = true
	go m.loop()
}

func (m *Manager) loop() {
	defer close(m.doneCh)
	for {
		select {
		case <-m.stopCh:
			return
		case <-m.ticker.C:
			_ = m.Take(context.Background())
		}
	}
}

// Stop ends the periodic loop and waits for the goroutine to exit.
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	close(m.stopCh)
	if m.ticker != nil {
		m.ticker.Stop()
	}
	m.running = false
	done := m.doneCh
	m.mu.Unlock()
	<-done
}

// Take creates an explicit Snapshot, writes it via Recovery (when configured),
// and resets the periodic interval timer when the loop is running.
func (m *Manager) Take(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	entries, err := m.state.Export(ctx)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(MemoryPayload{Entries: entries})
	if err != nil {
		return fmt.Errorf("snapshot: marshal payload: %w", err)
	}
	env := Envelope{
		Version:   CurrentVersion,
		CreatedAt: m.clock().UTC(),
		Payload:   payload,
	}
	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("snapshot: marshal envelope: %w", err)
	}

	if m.recovery != nil {
		if err := m.recovery.Save(ctx, data); err != nil {
			return err
		}
	}

	m.resetTicker()
	return nil
}

func (m *Manager) resetTicker() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ticker != nil && m.running && m.interval > 0 {
		m.ticker.Reset(m.interval)
	}
}

// DecodeEnvelope parses a Snapshot envelope from bytes.
func DecodeEnvelope(data []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return Envelope{}, fmt.Errorf("snapshot: decode envelope: %w", err)
	}
	return env, nil
}

// DecodeMemoryPayload parses the in-memory backend payload.
func DecodeMemoryPayload(payload json.RawMessage) (MemoryPayload, error) {
	var p MemoryPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return MemoryPayload{}, fmt.Errorf("snapshot: decode memory payload: %w", err)
	}
	if p.Entries == nil {
		p.Entries = make(map[string]json.RawMessage)
	}
	return p, nil
}

// RestoreInto loads Recovery data (if any) into State. Missing/unreadable
// recovery is treated as empty by the caller; this function applies a decoded
// envelope's payload into state.
func RestoreInto(ctx context.Context, st state.Store, data []byte) error {
	env, err := DecodeEnvelope(data)
	if err != nil {
		return err
	}
	payload, err := DecodeMemoryPayload(env.Payload)
	if err != nil {
		return err
	}
	return st.Replace(ctx, payload.Entries)
}
