package options_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/b4moss/es4/packages/go/pkg/options"
)

func TestDefaults(t *testing.T) {
	t.Parallel()
	got := options.Defaults()
	if got.SnapshotInterval != 30*time.Second {
		t.Fatalf("snapshot_interval: got %v want 30s", got.SnapshotInterval)
	}
	if !got.RestoreOnStartup {
		t.Fatal("restore_on_startup: want on/true")
	}
	if got.MemoryOnly {
		t.Fatal("memory_only: want off/false")
	}
	if got.RecoveryPath != "" {
		t.Fatalf("recovery_path: got %q want empty", got.RecoveryPath)
	}
}

func TestLoad_DefaultsOnly(t *testing.T) {
	t.Parallel()
	got, err := options.Load("", func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	want := options.Defaults()
	if got != want {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestFromFile_OverridesDefaults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "es4.json")
	body := `{
  "snapshot_interval": "10s",
  "restore_on_startup": "off",
  "memory_only": true,
  "recovery_path": "/tmp/recovery.bin"
}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := options.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.SnapshotInterval != 10*time.Second {
		t.Fatalf("snapshot_interval: got %v", got.SnapshotInterval)
	}
	if got.RestoreOnStartup {
		t.Fatal("restore_on_startup: want off")
	}
	if !got.MemoryOnly {
		t.Fatal("memory_only: want on")
	}
	if got.RecoveryPath != "/tmp/recovery.bin" {
		t.Fatalf("recovery_path: got %q", got.RecoveryPath)
	}
}

func TestFromFile_PartialKeepsDefaults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.json")
	if err := os.WriteFile(path, []byte(`{"recovery_path":"/data/r"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := options.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.SnapshotInterval != 30*time.Second {
		t.Fatalf("snapshot_interval should stay default, got %v", got.SnapshotInterval)
	}
	if !got.RestoreOnStartup {
		t.Fatal("restore_on_startup should stay default on")
	}
	if got.MemoryOnly {
		t.Fatal("memory_only should stay default off")
	}
	if got.RecoveryPath != "/data/r" {
		t.Fatalf("recovery_path: got %q", got.RecoveryPath)
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "es4.json")
	if err := os.WriteFile(path, []byte(`{
  "snapshot_interval": "10s",
  "restore_on_startup": false,
  "memory_only": false,
  "recovery_path": "/from/file"
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"ES4_SNAPSHOT_INTERVAL":  "45s",
		"ES4_RESTORE_ON_STARTUP": "on",
		"ES4_MEMORY_ONLY":        "1",
		"ES4_RECOVERY_PATH":      "/from/env",
	}
	got, err := options.Load(path, func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if got.SnapshotInterval != 45*time.Second {
		t.Fatalf("snapshot_interval: got %v want 45s", got.SnapshotInterval)
	}
	if !got.RestoreOnStartup {
		t.Fatal("restore_on_startup: want on from env")
	}
	if !got.MemoryOnly {
		t.Fatal("memory_only: want on from env")
	}
	if got.RecoveryPath != "/from/env" {
		t.Fatalf("recovery_path: got %q", got.RecoveryPath)
	}
}

func TestApplyEnv_OnlySetKeys(t *testing.T) {
	t.Parallel()
	base := options.Defaults()
	base.RecoveryPath = "/keep"
	got, err := options.ApplyEnv(base, func(k string) string {
		if k == "ES4_SNAPSHOT_INTERVAL" {
			return "5s"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.SnapshotInterval != 5*time.Second {
		t.Fatalf("snapshot_interval: got %v", got.SnapshotInterval)
	}
	if got.RecoveryPath != "/keep" {
		t.Fatalf("recovery_path should be unchanged, got %q", got.RecoveryPath)
	}
	if !got.RestoreOnStartup || got.MemoryOnly {
		t.Fatal("unset env must not change other fields")
	}
}

func TestApplyEnv_BoolForms(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want bool
	}{
		{"on", true},
		{"OFF", false},
		{"true", true},
		{"False", false},
		{"1", true},
		{"0", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got, err := options.ApplyEnv(options.Defaults(), func(k string) string {
				if k == "ES4_MEMORY_ONLY" {
					return tc.in
				}
				return ""
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.MemoryOnly != tc.want {
				t.Fatalf("got %v want %v", got.MemoryOnly, tc.want)
			}
		})
	}
}

func TestLoad_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := options.Load(filepath.Join(t.TempDir(), "missing.json"), func(string) string { return "" })
	if err == nil {
		t.Fatal("want error for missing config file")
	}
}

func TestFromFile_InvalidJSON(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := options.FromFile(path)
	if err == nil {
		t.Fatal("want error for invalid JSON")
	}
}

func TestFromFile_InvalidDuration(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "bad-dur.json")
	if err := os.WriteFile(path, []byte(`{"snapshot_interval":"nope"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := options.FromFile(path)
	if err == nil {
		t.Fatal("want error for invalid duration")
	}
}

func TestApplyEnv_InvalidBool(t *testing.T) {
	t.Parallel()
	_, err := options.ApplyEnv(options.Defaults(), func(k string) string {
		if k == "ES4_RESTORE_ON_STARTUP" {
			return "maybe"
		}
		return ""
	})
	if err == nil {
		t.Fatal("want error for invalid bool")
	}
}

func TestEffective_MemoryOnlyIgnoresRecovery(t *testing.T) {
	t.Parallel()
	raw := options.Options{
		SnapshotInterval: 15 * time.Second,
		RestoreOnStartup: true,
		MemoryOnly:       true,
		RecoveryPath:     "/should/ignore",
	}
	eff := raw.Effective()
	if eff.SnapshotInterval != 0 {
		t.Fatalf("snapshot_interval should be ignored, got %v", eff.SnapshotInterval)
	}
	if eff.RestoreOnStartup {
		t.Fatal("restore_on_startup should be ignored (off)")
	}
	if eff.RecoveryPath != "" {
		t.Fatalf("recovery_path should be ignored, got %q", eff.RecoveryPath)
	}
	if !eff.MemoryOnly {
		t.Fatal("memory_only must remain on")
	}
	if raw.UsesRecovery() {
		t.Fatal("UsesRecovery must be false when memory_only")
	}
	// Raw values remain; ignoring must not be an error condition.
	if raw.RecoveryPath != "/should/ignore" {
		t.Fatal("Effective must not mutate the receiver's meaning via shared state")
	}
}

func TestEffective_MemoryOnlyOffPassthrough(t *testing.T) {
	t.Parallel()
	raw := options.Options{
		SnapshotInterval: 12 * time.Second,
		RestoreOnStartup: false,
		MemoryOnly:       false,
		RecoveryPath:     "/keep",
	}
	if raw.Effective() != raw {
		t.Fatalf("got %#v want %#v", raw.Effective(), raw)
	}
	if !raw.UsesRecovery() {
		t.Fatal("UsesRecovery must be true when memory_only off")
	}
}

func TestEnvName(t *testing.T) {
	t.Parallel()
	if got := options.EnvName("snapshot_interval"); got != "ES4_SNAPSHOT_INTERVAL" {
		t.Fatalf("got %q", got)
	}
}
