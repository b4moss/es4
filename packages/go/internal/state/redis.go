package state

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

// DefaultRedisHashKey is the Redis HASH name when state_redis_key_prefix is empty.
const DefaultRedisHashKey = "es4:state"

// Redis is a Redis-protocol State adapter (also used for Valkey).
// Entries live in a single HASH: field = logical key, value = JSON bytes.
type Redis struct {
	mu     sync.Mutex
	txCond *sync.Cond
	client redis.UniversalClient
	hash   string
	closed bool
	active *redisTx
	owns   bool // close client on Close when true
}

// RedisConfig opens a Redis/Valkey State adapter.
type RedisConfig struct {
	// URL is a redis:// or rediss:// connection string (required unless Client is set).
	URL string
	// KeyPrefix is the Redis HASH key. Empty uses DefaultRedisHashKey.
	KeyPrefix string
	// Client, when non-nil, is used instead of dialing URL (tests / injection).
	// When set, Close does not close the client unless OwnsClient is true.
	Client redis.UniversalClient
	// OwnsClient closes Client on Store.Close when true.
	OwnsClient bool
}

// OpenRedis dials URL and returns a Redis State Store.
func OpenRedis(ctx context.Context, url, keyPrefix string) (*Redis, error) {
	return OpenRedisConfig(ctx, RedisConfig{URL: url, KeyPrefix: keyPrefix})
}

// OpenRedisConfig opens a Redis State Store from cfg.
func OpenRedisConfig(ctx context.Context, cfg RedisConfig) (*Redis, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	hash := cfg.KeyPrefix
	if hash == "" {
		hash = DefaultRedisHashKey
	}

	client := cfg.Client
	owns := cfg.OwnsClient
	if client == nil {
		if cfg.URL == "" {
			return nil, fmt.Errorf("state: empty redis url")
		}
		opt, err := redis.ParseURL(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("state: parse redis url: %w", err)
		}
		rdb := redis.NewClient(opt)
		if err := rdb.Ping(ctx).Err(); err != nil {
			_ = rdb.Close()
			return nil, fmt.Errorf("state: redis ping: %w", err)
		}
		client = rdb
		owns = true
	}

	r := &Redis{client: client, hash: hash, owns: owns}
	r.txCond = sync.NewCond(&r.mu)
	return r, nil
}

// HashKey returns the Redis HASH key (tests / diagnostics).
func (r *Redis) HashKey() string { return r.hash }

func (r *Redis) waitNoActiveTxLocked() {
	for r.active != nil {
		r.txCond.Wait()
	}
}

func (r *Redis) Set(ctx context.Context, key string, value json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateKey(key); err != nil {
		return err
	}
	if err := ValidateValue(value); err != nil {
		return err
	}
	cp := append(json.RawMessage(nil), value...)

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrClosed
	}
	r.waitNoActiveTxLocked()
	if r.closed {
		return ErrClosed
	}
	if err := r.client.HSet(ctx, r.hash, key, string(cp)).Err(); err != nil {
		return fmt.Errorf("state: redis set: %w", err)
	}
	return nil
}

func (r *Redis) Get(ctx context.Context, key string) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ValidateKey(key); err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, ErrClosed
	}
	r.waitNoActiveTxLocked()
	if r.closed {
		return nil, ErrClosed
	}
	s, err := r.client.HGet(ctx, r.hash, key).Result()
	if err == redis.Nil {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("state: redis get: %w", err)
	}
	return append(json.RawMessage(nil), s...), nil
}

func (r *Redis) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateKey(key); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrClosed
	}
	r.waitNoActiveTxLocked()
	if r.closed {
		return ErrClosed
	}
	n, err := r.client.HDel(ctx, r.hash, key).Result()
	if err != nil {
		return fmt.Errorf("state: redis delete: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Redis) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := ValidateKey(key); err != nil {
		return false, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return false, ErrClosed
	}
	r.waitNoActiveTxLocked()
	if r.closed {
		return false, ErrClosed
	}
	ok, err := r.client.HExists(ctx, r.hash, key).Result()
	if err != nil {
		return false, fmt.Errorf("state: redis exists: %w", err)
	}
	return ok, nil
}

func (r *Redis) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrClosed
	}
	r.waitNoActiveTxLocked()
	if r.closed {
		return ErrClosed
	}
	if err := r.client.Del(ctx, r.hash).Err(); err != nil {
		return fmt.Errorf("state: redis clear: %w", err)
	}
	return nil
}

func (r *Redis) Export(ctx context.Context) (map[string]json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, ErrClosed
	}
	r.waitNoActiveTxLocked()
	if r.closed {
		return nil, ErrClosed
	}
	m, err := r.client.HGetAll(ctx, r.hash).Result()
	if err != nil {
		return nil, fmt.Errorf("state: redis export: %w", err)
	}
	out := make(map[string]json.RawMessage, len(m))
	for k, v := range m {
		out[k] = append(json.RawMessage(nil), v...)
	}
	return out, nil
}

func (r *Redis) Replace(ctx context.Context, entries map[string]json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	next := make(map[string]string, len(entries))
	for k, v := range entries {
		if err := ValidateKey(k); err != nil {
			return err
		}
		if err := ValidateValue(v); err != nil {
			return err
		}
		next[k] = string(append(json.RawMessage(nil), v...))
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrClosed
	}
	r.waitNoActiveTxLocked()
	if r.closed {
		return ErrClosed
	}

	pipe := r.client.TxPipeline()
	pipe.Del(ctx, r.hash)
	if len(next) > 0 {
		fields := make([]interface{}, 0, len(next)*2)
		for k, v := range next {
			fields = append(fields, k, v)
		}
		pipe.HSet(ctx, r.hash, fields...)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("state: redis replace: %w", err)
	}
	return nil
}

func (r *Redis) BeginTx(ctx context.Context) (Tx, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, ErrClosed
	}
	if r.active != nil {
		return nil, ErrNestedTx
	}
	tx := &redisTx{
		r:       r,
		overlay: make(map[string]overlayEntry),
	}
	r.active = tx
	return tx, nil
}

func (r *Redis) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active != nil {
		r.active.done = true
		r.active = nil
		r.txCond.Broadcast()
	}
	r.closed = true
	if r.owns && r.client != nil {
		err := r.client.Close()
		r.client = nil
		return err
	}
	return nil
}

type redisTx struct {
	r       *Redis
	overlay map[string]overlayEntry
	cleared bool
	done    bool
}

func (t *redisTx) withLock(ctx context.Context, fn func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	t.r.mu.Lock()
	defer t.r.mu.Unlock()
	if t.done || t.r.active != t {
		return ErrTxDone
	}
	if t.r.closed {
		return ErrClosed
	}
	return fn()
}

func (t *redisTx) baseGet(ctx context.Context, key string) (json.RawMessage, error) {
	s, err := t.r.client.HGet(ctx, t.r.hash, key).Result()
	if err == redis.Nil {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("state: redis tx get: %w", err)
	}
	return append(json.RawMessage(nil), s...), nil
}

func (t *redisTx) baseExists(ctx context.Context, key string) (bool, error) {
	ok, err := t.r.client.HExists(ctx, t.r.hash, key).Result()
	if err != nil {
		return false, fmt.Errorf("state: redis tx exists: %w", err)
	}
	return ok, nil
}

func (t *redisTx) Set(ctx context.Context, key string, value json.RawMessage) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	if err := ValidateValue(value); err != nil {
		return err
	}
	cp := append(json.RawMessage(nil), value...)
	return t.withLock(ctx, func() error {
		t.overlay[key] = overlayEntry{value: cp}
		return nil
	})
}

func (t *redisTx) Get(ctx context.Context, key string) (json.RawMessage, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	var out json.RawMessage
	err := t.withLock(ctx, func() error {
		if e, ok := t.overlay[key]; ok {
			if e.deleted {
				return ErrNotFound
			}
			out = append(json.RawMessage(nil), e.value...)
			return nil
		}
		if t.cleared {
			return ErrNotFound
		}
		v, err := t.baseGet(ctx, key)
		if err != nil {
			return err
		}
		out = v
		return nil
	})
	return out, err
}

func (t *redisTx) Delete(ctx context.Context, key string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	return t.withLock(ctx, func() error {
		if e, ok := t.overlay[key]; ok {
			if e.deleted {
				return ErrNotFound
			}
			t.overlay[key] = overlayEntry{deleted: true}
			return nil
		}
		if t.cleared {
			return ErrNotFound
		}
		ok, err := t.baseExists(ctx, key)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		t.overlay[key] = overlayEntry{deleted: true}
		return nil
	})
}

func (t *redisTx) Exists(ctx context.Context, key string) (bool, error) {
	if err := ValidateKey(key); err != nil {
		return false, err
	}
	var ok bool
	err := t.withLock(ctx, func() error {
		if e, hit := t.overlay[key]; hit {
			ok = !e.deleted
			return nil
		}
		if t.cleared {
			ok = false
			return nil
		}
		var err error
		ok, err = t.baseExists(ctx, key)
		return err
	})
	return ok, err
}

func (t *redisTx) Clear(ctx context.Context) error {
	return t.withLock(ctx, func() error {
		t.cleared = true
		t.overlay = make(map[string]overlayEntry)
		return nil
	})
}

func (t *redisTx) Commit() error {
	t.r.mu.Lock()
	defer t.r.mu.Unlock()
	if t.done || t.r.active != t {
		return ErrTxDone
	}
	if t.r.closed {
		return ErrClosed
	}
	ctx := context.Background()
	pipe := t.r.client.TxPipeline()
	if t.cleared {
		pipe.Del(ctx, t.r.hash)
	}
	for k, e := range t.overlay {
		if e.deleted {
			pipe.HDel(ctx, t.r.hash, k)
			continue
		}
		pipe.HSet(ctx, t.r.hash, k, string(e.value))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("state: redis tx commit: %w", err)
	}
	t.done = true
	t.r.active = nil
	t.r.txCond.Broadcast()
	return nil
}

func (t *redisTx) Rollback() error {
	t.r.mu.Lock()
	defer t.r.mu.Unlock()
	if t.done || t.r.active != t {
		return ErrTxDone
	}
	t.done = true
	t.r.active = nil
	t.r.txCond.Broadcast()
	return nil
}
