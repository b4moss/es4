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
		t.Fatal("restore_on_startup: want true")
	}
	if got.MemoryOnly {
		t.Fatal("memory_only: want false")
	}
	if got.RecoveryPath != "" {
		t.Fatalf("recovery_path: got %q want empty", got.RecoveryPath)
	}
	if got.StatePath != "" {
		t.Fatalf("state_path: got %q want empty", got.StatePath)
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

func TestLoad_YAMLOverrides(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "es4.yaml")
	body := `
snapshot_interval: 10s
restore_on_startup: false
memory_only: true
recovery_path: /tmp/recovery.bin
state_path: /tmp/state.db
`
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
		t.Fatal("restore_on_startup: want false")
	}
	if !got.MemoryOnly {
		t.Fatal("memory_only: want true")
	}
	if got.RecoveryPath != "/tmp/recovery.bin" {
		t.Fatalf("recovery_path: got %q", got.RecoveryPath)
	}
	if got.StatePath != "/tmp/state.db" {
		t.Fatalf("state_path: got %q", got.StatePath)
	}
}

func TestLoad_PartialYAMLKeepsDefaults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.yaml")
	if err := os.WriteFile(path, []byte("recovery_path: /data/r\n"), 0o644); err != nil {
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
		t.Fatal("restore_on_startup should stay default true")
	}
	if got.MemoryOnly {
		t.Fatal("memory_only should stay default false")
	}
	if got.RecoveryPath != "/data/r" {
		t.Fatalf("recovery_path: got %q", got.RecoveryPath)
	}
}

func TestLoad_EnvOverridesYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "es4.yaml")
	if err := os.WriteFile(path, []byte(`
snapshot_interval: 10s
restore_on_startup: false
memory_only: false
recovery_path: /from/file
state_path: /state/from/file
`), 0o644); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"ES4_SNAPSHOT_INTERVAL":  "45s",
		"ES4_RESTORE_ON_STARTUP": "true",
		"ES4_MEMORY_ONLY":        "true",
		"ES4_RECOVERY_PATH":      "/from/env",
		"ES4_STATE_PATH":         "/state/from/env",
	}
	got, err := options.Load(path, func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if got.SnapshotInterval != 45*time.Second {
		t.Fatalf("snapshot_interval: got %v want 45s", got.SnapshotInterval)
	}
	if !got.RestoreOnStartup {
		t.Fatal("restore_on_startup: want true from env")
	}
	if !got.MemoryOnly {
		t.Fatal("memory_only: want true from env")
	}
	if got.RecoveryPath != "/from/env" {
		t.Fatalf("recovery_path: got %q", got.RecoveryPath)
	}
	if got.StatePath != "/state/from/env" {
		t.Fatalf("state_path: got %q", got.StatePath)
	}
}

func TestLoad_EmptyEnvSkipped(t *testing.T) {
	t.Parallel()
	base := options.Defaults()
	base.RecoveryPath = "/keep"
	base.StatePath = "/keep-state"
	base.SnapshotInterval = 12 * time.Second
	got, err := options.ApplyEnv(base, func(k string) string {
		switch k {
		case "ES4_SNAPSHOT_INTERVAL":
			return "5s"
		case "ES4_RESTORE_ON_STARTUP", "ES4_MEMORY_ONLY", "ES4_RECOVERY_PATH", "ES4_STATE_PATH":
			return "" // empty = unset; must not clear
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.SnapshotInterval != 5*time.Second {
		t.Fatalf("snapshot_interval: got %v", got.SnapshotInterval)
	}
	if got.RecoveryPath != "/keep" {
		t.Fatalf("empty env must not clear recovery_path, got %q", got.RecoveryPath)
	}
	if got.StatePath != "/keep-state" {
		t.Fatalf("empty env must not clear state_path, got %q", got.StatePath)
	}
	if !got.RestoreOnStartup || got.MemoryOnly {
		t.Fatal("empty/unset env must not change other fields")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := options.Load(filepath.Join(t.TempDir(), "missing.yaml"), func(string) string { return "" })
	if err == nil {
		t.Fatal("want error for missing config file")
	}
}

func TestLoad_BadYAML(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte(":\n  - broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := options.FromFile(path)
	if err == nil {
		t.Fatal("want error for bad YAML")
	}
}

func TestLoad_NonDurationSnapshotInterval(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	t.Run("numeric_yaml", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(dir, "num.yaml")
		if err := os.WriteFile(path, []byte("snapshot_interval: 30\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := options.FromFile(path); err == nil {
			t.Fatal("want error for numeric snapshot_interval")
		}
	})

	t.Run("numeric_string_yaml", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(dir, "numstr.yaml")
		if err := os.WriteFile(path, []byte("snapshot_interval: \"30\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := options.FromFile(path); err == nil {
			t.Fatal("want error for numeric-string snapshot_interval")
		}
	})

	t.Run("invalid_duration_string", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(dir, "bad-dur.yaml")
		if err := os.WriteFile(path, []byte("snapshot_interval: nope\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := options.FromFile(path); err == nil {
			t.Fatal("want error for invalid duration")
		}
	})

	t.Run("numeric_env", func(t *testing.T) {
		t.Parallel()
		_, err := options.ApplyEnv(options.Defaults(), func(k string) string {
			if k == "ES4_SNAPSHOT_INTERVAL" {
				return "30"
			}
			return ""
		})
		if err == nil {
			t.Fatal("want error for numeric env snapshot_interval")
		}
	})
}

func TestLoad_BoolNotLowercaseTrueFalse(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rejected := []string{"True", "TRUE", "False", "FALSE", "on", "off", "1", "0", "yes", "no"}

	for _, in := range rejected {
		in := in
		t.Run("yaml_"+in, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(dir, "bool-"+in+".yaml")
			body := "memory_only: " + in + "\n"
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := options.FromFile(path); err == nil {
				t.Fatalf("want error for bool %q", in)
			}
		})
		t.Run("env_"+in, func(t *testing.T) {
			t.Parallel()
			_, err := options.ApplyEnv(options.Defaults(), func(k string) string {
				if k == "ES4_MEMORY_ONLY" {
					return in
				}
				return ""
			})
			if err == nil {
				t.Fatalf("want error for env bool %q", in)
			}
		})
	}

	t.Run("yaml_lowercase_ok", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(dir, "ok.yaml")
		if err := os.WriteFile(path, []byte("memory_only: true\nrestore_on_startup: false\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := options.FromFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !got.MemoryOnly || got.RestoreOnStartup {
			t.Fatalf("got memory_only=%v restore_on_startup=%v", got.MemoryOnly, got.RestoreOnStartup)
		}
	})
}

func TestEffective_MemoryOnlyOn(t *testing.T) {
	t.Parallel()
	raw := options.Options{
		SnapshotInterval: 15 * time.Second,
		RestoreOnStartup: true,
		MemoryOnly:       true,
		RecoveryPath:     "/should/ignore",
		StatePath:        "/should/ignore-state",
	}
	eff := raw.Effective()
	if eff.SnapshotInterval != 0 {
		t.Fatalf("snapshot_interval should be ignored, got %v", eff.SnapshotInterval)
	}
	if eff.RestoreOnStartup {
		t.Fatal("restore_on_startup should be ignored (false)")
	}
	if eff.RecoveryPath != "" {
		t.Fatalf("recovery_path should be ignored, got %q", eff.RecoveryPath)
	}
	if eff.StatePath != "" {
		t.Fatalf("state_path should be ignored, got %q", eff.StatePath)
	}
	if !eff.MemoryOnly {
		t.Fatal("memory_only must remain true")
	}
	if raw.UsesRecovery() {
		t.Fatal("UsesRecovery must be false when memory_only")
	}
	if raw.RecoveryPath != "/should/ignore" {
		t.Fatal("Effective must not mutate the receiver")
	}
	if raw.StatePath != "/should/ignore-state" {
		t.Fatal("Effective must not mutate state_path on the receiver")
	}
}

func TestEffective_MemoryOnlyOff(t *testing.T) {
	t.Parallel()
	raw := options.Options{
		SnapshotInterval: 12 * time.Second,
		RestoreOnStartup: false,
		MemoryOnly:       false,
		RecoveryPath:     "/keep",
		StatePath:        "/keep-state",
	}
	if raw.Effective() != raw {
		t.Fatalf("got %#v want %#v", raw.Effective(), raw)
	}
	if !raw.UsesRecovery() {
		t.Fatal("UsesRecovery must be true when memory_only false")
	}
}

func TestEnvName(t *testing.T) {
	t.Parallel()
	if got := options.EnvName("snapshot_interval"); got != "ES4_SNAPSHOT_INTERVAL" {
		t.Fatalf("got %q", got)
	}
}
