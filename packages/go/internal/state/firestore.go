package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"unicode/utf8"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Firestore document ID limit (bytes). Encoded logical keys that exceed this
// are rejected with ErrFirestoreDocIDTooLong.
const firestoreMaxDocIDBytes = 1500

// ErrFirestoreDocIDTooLong is returned when the path-percent-encoded document ID
// would exceed Firestore's 1500-byte limit.
var ErrFirestoreDocIDTooLong = errors.New("state: firestore document id too long")

// firestoreDoc is the document schema stored in the configured collection.
// Field "value" holds JSON text reconstitutable to json.RawMessage semantics.
type firestoreDoc struct {
	Value string `firestore:"value"`
}

// Firestore is a Firestore State adapter.
// One logical key maps to one document in a single configured collection.
// Document ID = path-percent-encode of the logical key ("/" → "%2F").
// Clear / empty Replace delete only documents inside that collection.
type Firestore struct {
	mu         sync.Mutex
	txCond     *sync.Cond
	client     *firestore.Client
	projectID  string
	databaseID string
	collection string
	closed     bool
	active     *firestoreTx
	owns       bool // close client on Close when true
}

// FirestoreConfig opens a Firestore State adapter.
type FirestoreConfig struct {
	ProjectID  string // required
	DatabaseID string // empty → "(default)"
	Collection string // required; no slash
	// Client, when non-nil, is used instead of dialing (tests / injection).
	// When set, Close does not close the client unless OwnsClient is true.
	Client *firestore.Client
	// OwnsClient closes Client on Store.Close when true.
	OwnsClient bool
}

// OpenFirestore dials Firestore (or the emulator via FIRESTORE_EMULATOR_HOST)
// and returns a Store bound to projectID / databaseID / collection.
func OpenFirestore(ctx context.Context, projectID, databaseID, collection string) (*Firestore, error) {
	return OpenFirestoreConfig(ctx, FirestoreConfig{
		ProjectID:  projectID,
		DatabaseID: databaseID,
		Collection: collection,
	})
}

// OpenFirestoreConfig opens a Firestore State Store from cfg.
func OpenFirestoreConfig(ctx context.Context, cfg FirestoreConfig) (*Firestore, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if cfg.ProjectID == "" {
		return nil, fmt.Errorf("state: empty firestore project id")
	}
	if err := validateFirestoreCollectionID(cfg.Collection); err != nil {
		return nil, err
	}
	dbID := cfg.DatabaseID
	if dbID == "" {
		dbID = "(default)"
	}

	client := cfg.Client
	owns := cfg.OwnsClient
	if client == nil {
		var err error
		client, err = firestore.NewClientWithDatabase(ctx, cfg.ProjectID, dbID)
		if err != nil {
			return nil, fmt.Errorf("state: open firestore: %w", err)
		}
		owns = true
	}

	f := &Firestore{
		client:     client,
		projectID:  cfg.ProjectID,
		databaseID: dbID,
		collection: cfg.Collection,
		owns:       owns,
	}
	f.txCond = sync.NewCond(&f.mu)
	return f, nil
}

func validateFirestoreCollectionID(id string) error {
	if id == "" {
		return fmt.Errorf("state: empty firestore collection")
	}
	if strings.Contains(id, "/") {
		return fmt.Errorf("state: firestore collection must not contain '/'")
	}
	if id == "." || id == ".." {
		return fmt.Errorf("state: invalid firestore collection %q", id)
	}
	return nil
}

// EncodeFirestoreDocID path-percent-encodes a logical key for use as a document ID.
// "/" becomes "%2F". Callers must ValidateKey first.
func EncodeFirestoreDocID(logicalKey string) (string, error) {
	id := url.PathEscape(logicalKey)
	if len(id) > firestoreMaxDocIDBytes {
		return "", fmt.Errorf("%w: %d bytes (max %d)", ErrFirestoreDocIDTooLong, len(id), firestoreMaxDocIDBytes)
	}
	if !utf8.ValidString(id) {
		return "", fmt.Errorf("%w: encoded id not valid utf-8", ErrInvalidKey)
	}
	return id, nil
}

// DecodeFirestoreDocID reverses EncodeFirestoreDocID.
func DecodeFirestoreDocID(docID string) (string, error) {
	key, err := url.PathUnescape(docID)
	if err != nil {
		return "", fmt.Errorf("state: decode firestore doc id: %w", err)
	}
	return key, nil
}

// Collection returns the configured collection ID (tests / diagnostics).
func (f *Firestore) Collection() string { return f.collection }

// ProjectID returns the configured project ID.
func (f *Firestore) ProjectID() string { return f.projectID }

// DatabaseID returns the effective database ID.
func (f *Firestore) DatabaseID() string { return f.databaseID }

func (f *Firestore) coll() *firestore.CollectionRef {
	return f.client.Collection(f.collection)
}

func (f *Firestore) waitNoActiveTxLocked() {
	for f.active != nil {
		f.txCond.Wait()
	}
}

func (f *Firestore) Set(ctx context.Context, key string, value json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateKey(key); err != nil {
		return err
	}
	if err := ValidateValue(value); err != nil {
		return err
	}
	docID, err := EncodeFirestoreDocID(key)
	if err != nil {
		return err
	}
	cp := append(json.RawMessage(nil), value...)

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return ErrClosed
	}
	f.waitNoActiveTxLocked()
	if f.closed {
		return ErrClosed
	}
	_, err = f.coll().Doc(docID).Set(ctx, firestoreDoc{Value: string(cp)})
	if err != nil {
		return fmt.Errorf("state: firestore set: %w", err)
	}
	return nil
}

func (f *Firestore) Get(ctx context.Context, key string) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	docID, err := EncodeFirestoreDocID(key)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil, ErrClosed
	}
	f.waitNoActiveTxLocked()
	if f.closed {
		return nil, ErrClosed
	}
	snap, err := f.coll().Doc(docID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("state: firestore get: %w", err)
	}
	var doc firestoreDoc
	if err := snap.DataTo(&doc); err != nil {
		return nil, fmt.Errorf("state: firestore get decode: %w", err)
	}
	return append(json.RawMessage(nil), doc.Value...), nil
}

func (f *Firestore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateKey(key); err != nil {
		return err
	}
	docID, err := EncodeFirestoreDocID(key)
	if err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return ErrClosed
	}
	f.waitNoActiveTxLocked()
	if f.closed {
		return ErrClosed
	}
	// Exists check so missing keys return ErrNotFound (Firestore Delete is idempotent).
	_, err = f.coll().Doc(docID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return ErrNotFound
		}
		return fmt.Errorf("state: firestore delete get: %w", err)
	}
	_, err = f.coll().Doc(docID).Delete(ctx)
	if err != nil {
		return fmt.Errorf("state: firestore delete: %w", err)
	}
	return nil
}

func (f *Firestore) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := ValidateKey(key); err != nil {
		return false, err
	}
	docID, err := EncodeFirestoreDocID(key)
	if err != nil {
		return false, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return false, ErrClosed
	}
	f.waitNoActiveTxLocked()
	if f.closed {
		return false, ErrClosed
	}
	_, err = f.coll().Doc(docID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, fmt.Errorf("state: firestore exists: %w", err)
	}
	return true, nil
}

// Clear deletes every document in the configured collection only.
// It never wipes other collections, the project, or uses admin "delete all".
func (f *Firestore) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return ErrClosed
	}
	f.waitNoActiveTxLocked()
	if f.closed {
		return ErrClosed
	}
	return f.clearCollectionLocked(ctx)
}

func (f *Firestore) clearCollectionLocked(ctx context.Context) error {
	bw := f.client.BulkWriter(ctx)
	iter := f.coll().Documents(ctx)
	defer iter.Stop()
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			bw.End()
			return fmt.Errorf("state: firestore clear iterate: %w", err)
		}
		if _, err := bw.Delete(doc.Ref); err != nil {
			bw.End()
			return fmt.Errorf("state: firestore clear delete: %w", err)
		}
	}
	bw.End()
	return nil
}

func (f *Firestore) Export(ctx context.Context) (map[string]json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil, ErrClosed
	}
	f.waitNoActiveTxLocked()
	if f.closed {
		return nil, ErrClosed
	}

	out := make(map[string]json.RawMessage)
	iter := f.coll().Documents(ctx)
	defer iter.Stop()
	for {
		snap, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("state: firestore export: %w", err)
		}
		logical, err := DecodeFirestoreDocID(snap.Ref.ID)
		if err != nil {
			return nil, err
		}
		var doc firestoreDoc
		if err := snap.DataTo(&doc); err != nil {
			return nil, fmt.Errorf("state: firestore export decode: %w", err)
		}
		out[logical] = append(json.RawMessage(nil), doc.Value...)
	}
	return out, nil
}

func (f *Firestore) Replace(ctx context.Context, entries map[string]json.RawMessage) error {
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
		docID, err := EncodeFirestoreDocID(k)
		if err != nil {
			return err
		}
		next[docID] = string(append(json.RawMessage(nil), v...))
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return ErrClosed
	}
	f.waitNoActiveTxLocked()
	if f.closed {
		return ErrClosed
	}

	// Atomic under Store lock: clear configured collection, then write new set.
	// Scope is the single collection only (same as Clear).
	if err := f.clearCollectionLocked(ctx); err != nil {
		return err
	}
	bw := f.client.BulkWriter(ctx)
	for docID, val := range next {
		ref := f.coll().Doc(docID)
		if _, err := bw.Set(ref, firestoreDoc{Value: val}); err != nil {
			bw.End()
			return fmt.Errorf("state: firestore replace set: %w", err)
		}
	}
	bw.End()
	return nil
}

func (f *Firestore) BeginTx(ctx context.Context) (Tx, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil, ErrClosed
	}
	if f.active != nil {
		return nil, ErrNestedTx
	}
	tx := &firestoreTx{
		f:       f,
		overlay: make(map[string]overlayEntry),
	}
	f.active = tx
	return tx, nil
}

func (f *Firestore) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.active != nil {
		f.active.done = true
		f.active = nil
		f.txCond.Broadcast()
	}
	f.closed = true
	if f.owns && f.client != nil {
		err := f.client.Close()
		f.client = nil
		return err
	}
	return nil
}

// firestoreTx uses an in-memory overlay committed via BulkWriter (same contract
// as Memory overlays). Nested Begin is rejected by the parent Store.
type firestoreTx struct {
	f       *Firestore
	overlay map[string]overlayEntry
	cleared bool
	done    bool
}

func (t *firestoreTx) withLock(ctx context.Context, fn func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	t.f.mu.Lock()
	defer t.f.mu.Unlock()
	if t.done || t.f.active != t {
		return ErrTxDone
	}
	if t.f.closed {
		return ErrClosed
	}
	return fn()
}

func (t *firestoreTx) baseGet(ctx context.Context, key string) (json.RawMessage, error) {
	docID, err := EncodeFirestoreDocID(key)
	if err != nil {
		return nil, err
	}
	snap, err := t.f.coll().Doc(docID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("state: firestore tx get: %w", err)
	}
	var doc firestoreDoc
	if err := snap.DataTo(&doc); err != nil {
		return nil, fmt.Errorf("state: firestore tx get decode: %w", err)
	}
	return append(json.RawMessage(nil), doc.Value...), nil
}

func (t *firestoreTx) baseExists(ctx context.Context, key string) (bool, error) {
	docID, err := EncodeFirestoreDocID(key)
	if err != nil {
		return false, err
	}
	_, err = t.f.coll().Doc(docID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, fmt.Errorf("state: firestore tx exists: %w", err)
	}
	return true, nil
}

func (t *firestoreTx) Set(ctx context.Context, key string, value json.RawMessage) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	if err := ValidateValue(value); err != nil {
		return err
	}
	if _, err := EncodeFirestoreDocID(key); err != nil {
		return err
	}
	cp := append(json.RawMessage(nil), value...)
	return t.withLock(ctx, func() error {
		t.overlay[key] = overlayEntry{value: cp}
		return nil
	})
}

func (t *firestoreTx) Get(ctx context.Context, key string) (json.RawMessage, error) {
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

func (t *firestoreTx) Delete(ctx context.Context, key string) error {
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

func (t *firestoreTx) Exists(ctx context.Context, key string) (bool, error) {
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

func (t *firestoreTx) Clear(ctx context.Context) error {
	return t.withLock(ctx, func() error {
		t.cleared = true
		t.overlay = make(map[string]overlayEntry)
		return nil
	})
}

func (t *firestoreTx) Commit() error {
	t.f.mu.Lock()
	defer t.f.mu.Unlock()
	if t.done || t.f.active != t {
		return ErrTxDone
	}
	if t.f.closed {
		return ErrClosed
	}
	ctx := context.Background()
	if t.cleared {
		if err := t.f.clearCollectionLocked(ctx); err != nil {
			return err
		}
	}
	bw := t.f.client.BulkWriter(ctx)
	for k, e := range t.overlay {
		docID, err := EncodeFirestoreDocID(k)
		if err != nil {
			bw.End()
			return err
		}
		ref := t.f.coll().Doc(docID)
		if e.deleted {
			if _, err := bw.Delete(ref); err != nil {
				bw.End()
				return fmt.Errorf("state: firestore tx commit delete: %w", err)
			}
			continue
		}
		if _, err := bw.Set(ref, firestoreDoc{Value: string(e.value)}); err != nil {
			bw.End()
			return fmt.Errorf("state: firestore tx commit set: %w", err)
		}
	}
	bw.End()
	t.done = true
	t.f.active = nil
	t.f.txCond.Broadcast()
	return nil
}

func (t *firestoreTx) Rollback() error {
	t.f.mu.Lock()
	defer t.f.mu.Unlock()
	if t.done || t.f.active != t {
		return ErrTxDone
	}
	t.done = true
	t.f.active = nil
	t.f.txCond.Broadcast()
	return nil
}
