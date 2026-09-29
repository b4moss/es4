package state_test

import (
	"strings"
	"testing"

	"github.com/b4moss/es4/packages/go/internal/state"
)

func TestEncodeFirestoreDocID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		key  string
		want string
	}{
		{"K-N1 single", "a", "a"},
		{"K-N2 hierarchy", "a/b/c", "a%2Fb%2Fc"},
		{"space", "a b", "a%20b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := state.EncodeFirestoreDocID(tc.key)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
			// K-N3 round-trip
			back, err := state.DecodeFirestoreDocID(got)
			if err != nil {
				t.Fatal(err)
			}
			if back != tc.key {
				t.Fatalf("round-trip: got %q want %q", back, tc.key)
			}
		})
	}
}

func TestEncodeFirestoreDocID_TooLong(t *testing.T) {
	t.Parallel()
	// Build a key whose PathEscape exceeds 1500 bytes.
	key := strings.Repeat("a/", 800) + "b" // many '/' → %2F
	_, err := state.EncodeFirestoreDocID(key)
	if err == nil {
		t.Fatal("want ErrFirestoreDocIDTooLong")
	}
	if !strings.Contains(err.Error(), "too long") {
		t.Fatalf("got %v", err)
	}
}
