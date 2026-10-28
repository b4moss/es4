package state

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

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

// metaHash returns the companion HASH for expiry metadata (non-public).
func (r *Redis) metaHash() string { return r.hash + ":meta" }

func (r *Redis) waitNoActiveTxLocked() {
	for r.active != nil {
		r.txCond.Wait()
	}
}

func (r *Redis) Set(ctx context.Context, key string, value json.RawMessage, meta ...SetMeta) error {
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
	exp := CloneTimePtr(FirstMeta(meta).ExpiresAt)

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
	pipe.HSet(ctx, r.hash, key, string(cp))
	if exp == nil {
		pipe.HDel(ctx, r.metaHash(), key)
	} else {
		pipe.HSet(ctx, r.metaHash(), key, FormatExpiresAt(*exp))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("state: redis set: %w", err)
	}
	return nil
}

func (r *Redis) redisExpires(ctx context.Context, key string) (*time.Time, error) {
	s, err := r.client.HGet(ctx, r.metaHash(), key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("state: redis meta get: %w", err)
	}
	return ParseExpiresAt(s)
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
	exp, err := r.redisExpires(ctx, key)
	if err != nil {
		return nil, err
	}
	if IsExpired(exp) {
		pipe := r.client.TxPipeline()
		pipe.HDel(ctx, r.hash, key)
		pipe.HDel(ctx, r.metaHash(), key)
		_, _ = pipe.Exec(ctx)
		return nil, ErrNotFound
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
	ok, err := r.client.HExists(ctx, r.hash, key).Result()
	if err != nil {
		return fmt.Errorf("state: redis delete exists: %w", err)
	}
	if !ok {
		return ErrNotFound
	}
	exp, err := r.redisExpires(ctx, key)
	if err != nil {
		return err
	}
	if IsExpired(exp) {
		pipe := r.client.TxPipeline()
		pipe.HDel(ctx, r.hash, key)
		pipe.HDel(ctx, r.metaHash(), key)
		_, _ = pipe.Exec(ctx)
		return ErrNotFound
	}
	pipe := r.client.TxPipeline()
	pipe.HDel(ctx, r.hash, key)
	pipe.HDel(ctx, r.metaHash(), key)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("state: redis delete: %w", err)
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
	if !ok {
		return false, nil
	}
	exp, err := r.redisExpires(ctx, key)
	if err != nil {
		return false, err
	}
	if IsExpired(exp) {
		pipe := r.client.TxPipeline()
		pipe.HDel(ctx, r.hash, key)
		pipe.HDel(ctx, r.metaHash(), key)
		_, _ = pipe.Exec(ctx)
		return false, nil
	}
	return true, nil
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
	if err := r.client.Del(ctx, r.hash, r.metaHash()).Err(); err != nil {
		return fmt.Errorf("state: redis clear: %w", err)
	}
	return nil
}

func (r *Redis) Scan(ctx context.Context, prefix string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ValidateScanPrefix(prefix); err != nil {
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
	fields, err := r.client.HKeys(ctx, r.hash).Result()
	if err != nil {
		return nil, fmt.Errorf("state: redis scan: %w", err)
	}
	meta, err := r.client.HGetAll(ctx, r.metaHash()).Result()
	if err != nil {
		return nil, fmt.Errorf("state: redis scan meta: %w", err)
	}
	out := make([]string, 0)
	for _, k := range KeysWithPrefix(fields, prefix) {
		if s, ok := meta[k]; ok {
			exp, err := ParseExpiresAt(s)
			if err != nil {
				return nil, err
			}
			if IsExpired(exp) {
				continue
			}
		}
		out = append(out, k)
	}
	return out, nil
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
	meta, err := r.client.HGetAll(ctx, r.metaHash()).Result()
	if err != nil {
		return nil, fmt.Errorf("state: redis export meta: %w", err)
	}
	out := make(map[string]json.RawMessage, len(m))
	for k, v := range m {
		if s, ok := meta[k]; ok {
			exp, err := ParseExpiresAt(s)
			if err != nil {
				return nil, err
			}
			if IsExpired(exp) {
				continue
			}
		}
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
	pipe.Del(ctx, r.hash, r.metaHash())
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
	exp, err := t.r.redisExpires(ctx, key)
	if err != nil {
		return nil, err
	}
	if IsExpired(exp) {
		return nil, ErrNotFound
	}
	return append(json.RawMessage(nil), s...), nil
}

func (t *redisTx) baseExists(ctx context.Context, key string) (bool, error) {
	ok, err := t.r.client.HExists(ctx, t.r.hash, key).Result()
	if err != nil {
		return false, fmt.Errorf("state: redis tx exists: %w", err)
	}
	if !ok {
		return false, nil
	}
	exp, err := t.r.redisExpires(ctx, key)
	if err != nil {
		return false, err
	}
	if IsExpired(exp) {
		return false, nil
	}
	return true, nil
}

func (t *redisTx) Set(ctx context.Context, key string, value json.RawMessage, meta ...SetMeta) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	if err := ValidateValue(value); err != nil {
		return err
	}
	cp := append(json.RawMessage(nil), value...)
	exp := CloneTimePtr(FirstMeta(meta).ExpiresAt)
	return t.withLock(ctx, func() error {
		t.overlay[key] = overlayEntry{value: cp, expiresAt: exp}
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
			if e.deleted || IsExpired(e.expiresAt) {
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
			if e.deleted || IsExpired(e.expiresAt) {
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
			ok = !e.deleted && !IsExpired(e.expiresAt)
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

func (t *redisTx) Scan(ctx context.Context, prefix string) ([]string, error) {
	if err := ValidateScanPrefix(prefix); err != nil {
		return nil, err
	}
	var out []string
	err := t.withLock(ctx, func() error {
		seen := make(map[string]struct{})
		if !t.cleared {
			fields, err := t.r.client.HKeys(ctx, t.r.hash).Result()
			if err != nil {
				return fmt.Errorf("state: redis tx scan: %w", err)
			}
			meta, err := t.r.client.HGetAll(ctx, t.r.metaHash()).Result()
			if err != nil {
				return fmt.Errorf("state: redis tx scan meta: %w", err)
			}
			for _, k := range fields {
				if !strings.HasPrefix(k, prefix) {
					continue
				}
				if e, ok := t.overlay[k]; ok {
					if e.deleted || IsExpired(e.expiresAt) {
						continue
					}
					seen[k] = struct{}{}
					continue
				}
				if s, ok := meta[k]; ok {
					exp, err := ParseExpiresAt(s)
					if err != nil {
						return err
					}
					if IsExpired(exp) {
						continue
					}
				}
				seen[k] = struct{}{}
			}
		}
		for k, e := range t.overlay {
			if e.deleted || IsExpired(e.expiresAt) || !strings.HasPrefix(k, prefix) {
				continue
			}
			seen[k] = struct{}{}
		}
		out = make([]string, 0, len(seen))
		for k := range seen {
			out = append(out, k)
		}
		return nil
	})
	return out, err
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
		pipe.Del(ctx, t.r.hash, t.r.metaHash())
	}
	for k, e := range t.overlay {
		if e.deleted {
			pipe.HDel(ctx, t.r.hash, k)
			pipe.HDel(ctx, t.r.metaHash(), k)
			continue
		}
		pipe.HSet(ctx, t.r.hash, k, string(e.value))
		if e.expiresAt == nil {
			pipe.HDel(ctx, t.r.metaHash(), k)
		} else {
			pipe.HSet(ctx, t.r.metaHash(), k, FormatExpiresAt(*e.expiresAt))
		}
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
