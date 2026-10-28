package state_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/b4moss/es4/packages/go/internal/state"
)

func TestStoreContract_ExpiresAt(t *testing.T) {
	// Not parallel: SetNowFunc is process-global.
	for backend, open := range storeFactories(t) {
		t.Run(backend+"/expires", func(t *testing.T) {
			st := open(t)
			ctx := context.Background()
			fixed := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)
			state.SetNowFunc(func() time.Time { return fixed })
			t.Cleanup(func() { state.SetNowFunc(nil) })

			future := fixed.Add(time.Hour)
			past := fixed.Add(-time.Hour)
			val := json.RawMessage(`{"v":1}`)

			if err := st.Set(ctx, "ttl:future", val, state.SetMeta{ExpiresAt: &future}); err != nil {
				t.Fatal(err)
			}
			got, err := st.Get(ctx, "ttl:future")
			if err != nil || string(got) != string(val) {
				t.Fatalf("future get: %s err=%v", got, err)
			}
			ok, err := st.Exists(ctx, "ttl:future")
			if err != nil || !ok {
				t.Fatalf("future exists: %v %v", ok, err)
			}

			if err := st.Set(ctx, "ttl:past", val, state.SetMeta{ExpiresAt: &past}); err != nil {
				t.Fatal(err)
			}
			_, err = st.Get(ctx, "ttl:past")
			if !errors.Is(err, state.ErrNotFound) {
				t.Fatalf("past get: %v", err)
			}
			ok, err = st.Exists(ctx, "ttl:past")
			if err != nil || ok {
				t.Fatalf("past exists: %v %v", ok, err)
			}
			keys, err := st.Scan(ctx, "ttl")
			if err != nil {
				t.Fatal(err)
			}
			foundFuture := false
			for _, k := range keys {
				if k == "ttl:past" {
					t.Fatal("scan included past key")
				}
				if k == "ttl:future" {
					foundFuture = true
				}
			}
			if !foundFuture {
				t.Fatal("scan missing future key")
			}

			if err := st.Set(ctx, "ttl:future", json.RawMessage(`{"v":2}`)); err != nil {
				t.Fatal(err)
			}
			state.SetNowFunc(func() time.Time { return future.Add(time.Hour) })
			got, err = st.Get(ctx, "ttl:future")
			if err != nil || string(got) != `{"v":2}` {
				t.Fatalf("cleared expiry get: %s err=%v", got, err)
			}
			state.SetNowFunc(func() time.Time { return fixed })

			later := fixed.Add(2 * time.Hour)
			if err := st.Set(ctx, "ttl:re", val, state.SetMeta{ExpiresAt: &future}); err != nil {
				t.Fatal(err)
			}
			if err := st.Set(ctx, "ttl:re", json.RawMessage(`{"v":3}`), state.SetMeta{ExpiresAt: &later}); err != nil {
				t.Fatal(err)
			}
			state.SetNowFunc(func() time.Time { return future.Add(time.Minute) })
			got, err = st.Get(ctx, "ttl:re")
			if err != nil || string(got) != `{"v":3}` {
				t.Fatalf("new expiry still valid: %s err=%v", got, err)
			}
			state.SetNowFunc(func() time.Time { return later.Add(time.Second) })
			_, err = st.Get(ctx, "ttl:re")
			if !errors.Is(err, state.ErrNotFound) {
				t.Fatalf("after new expiry: %v", err)
			}
			state.SetNowFunc(func() time.Time { return fixed })

			_ = st.Clear(ctx)
			if err := st.Set(ctx, "exp:a", val, state.SetMeta{ExpiresAt: &future}); err != nil {
				t.Fatal(err)
			}
			if err := st.Set(ctx, "exp:b", val, state.SetMeta{ExpiresAt: &past}); err != nil {
				t.Fatal(err)
			}
			exp, err := st.Export(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := exp["exp:a"]; !ok {
				t.Fatal("export missing non-expired value")
			}
			if _, ok := exp["exp:b"]; ok {
				t.Fatal("export included expired key")
			}
			if err := st.Clear(ctx); err != nil {
				t.Fatal(err)
			}
			ok, _ = st.Exists(ctx, "exp:a")
			if ok {
				t.Fatal("clear left expiry key")
			}
		})
	}
}

func TestStoreContract_ExpiresAt_Tx(t *testing.T) {
	// Not parallel: SetNowFunc is process-global.
	for backend, open := range storeFactories(t) {
		t.Run(backend+"/expires_tx", func(t *testing.T) {
			st := open(t)
			ctx := context.Background()
			fixed := time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC)
			state.SetNowFunc(func() time.Time { return fixed })
			t.Cleanup(func() { state.SetNowFunc(nil) })

			future := fixed.Add(time.Hour)
			past := fixed.Add(-time.Minute)
			val := json.RawMessage(`{"tx":1}`)

			tx, err := st.BeginTx(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err := tx.Set(ctx, "tx:ttl", val, state.SetMeta{ExpiresAt: &future}); err != nil {
				t.Fatal(err)
			}
			got, err := tx.Get(ctx, "tx:ttl")
			if err != nil || string(got) != string(val) {
				t.Fatalf("tx get before commit: %s err=%v", got, err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			got, err = st.Get(ctx, "tx:ttl")
			if err != nil || string(got) != string(val) {
				t.Fatalf("after commit: %s err=%v", got, err)
			}

			tx2, err := st.BeginTx(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err := tx2.Set(ctx, "tx:roll", val, state.SetMeta{ExpiresAt: &future}); err != nil {
				t.Fatal(err)
			}
			if err := tx2.Rollback(); err != nil {
				t.Fatal(err)
			}
			ok, err := st.Exists(ctx, "tx:roll")
			if err != nil || ok {
				t.Fatalf("after rollback: %v %v", ok, err)
			}

			tx3, err := st.BeginTx(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err := tx3.Set(ctx, "tx:past", val, state.SetMeta{ExpiresAt: &past}); err != nil {
				t.Fatal(err)
			}
			_, err = tx3.Get(ctx, "tx:past")
			if !errors.Is(err, state.ErrNotFound) {
				t.Fatalf("tx past get: %v", err)
			}
			_ = tx3.Rollback()
		})
	}
}
