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
	if got.RecoveryBackend != "" {
		t.Fatalf("recovery_backend: got %q want empty", got.RecoveryBackend)
	}
	if got.RecoveryTTL != 0 {
		t.Fatalf("recovery_ttl: got %v want 0", got.RecoveryTTL)
	}
	if got.RecoveryLibSQLURL != "" || got.RecoveryLibSQLAuthToken != "" {
		t.Fatal("libsql keys want empty")
	}
	if got.RecoveryS3Bucket != "" || got.RecoveryS3Prefix != "" || got.RecoveryS3Region != "" || got.RecoveryS3Endpoint != "" {
		t.Fatal("s3 keys want empty")
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
		SnapshotInterval:        15 * time.Second,
		RestoreOnStartup:        true,
		MemoryOnly:              true,
		RecoveryPath:            "/should/ignore",
		RecoveryBackend:         "libsql",
		RecoveryTTL:             time.Hour,
		RecoveryLibSQLURL:       "libsql://x",
		RecoveryLibSQLAuthToken: "tok",
		RecoveryS3Bucket:        "b",
		RecoveryS3Prefix:        "p",
		RecoveryS3Region:        "r",
		RecoveryS3Endpoint:      "http://e",
		StatePath:               "/should/ignore-state",
		StateBackend:              "redis",
		StateRedisURL:             "redis://127.0.0.1:6379/0",
		StateRedisKeyPrefix:       "e2e/ignore/",
		StateFirestoreProjectID:    "proj",
		StateFirestoreDatabaseID:   "db",
		StateFirestoreCollection:   "col",
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
	if eff.RecoveryBackend != "" || eff.RecoveryTTL != 0 {
		t.Fatalf("recovery backend/ttl should be cleared: %#v", eff)
	}
	if eff.RecoveryLibSQLURL != "" || eff.RecoveryS3Bucket != "" {
		t.Fatal("libsql/s3 keys should be cleared")
	}
	if eff.StatePath != "" {
		t.Fatalf("state_path should be ignored, got %q", eff.StatePath)
	}
	if eff.StateBackend != "" || eff.StateRedisURL != "" || eff.StateRedisKeyPrefix != "" {
		t.Fatalf("state_backend/redis keys should be cleared: %#v", eff)
	}
	if eff.StateFirestoreProjectID != "" || eff.StateFirestoreCollection != "" {
		t.Fatalf("firestore keys should be cleared: %#v", eff)
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
	if raw.StateRedisURL != "redis://127.0.0.1:6379/0" {
		t.Fatal("Effective must not mutate state_redis_url on the receiver")
	}
}

func TestEffective_MemoryOnlyOff(t *testing.T) {
	t.Parallel()
	raw := options.Options{
		SnapshotInterval:         12 * time.Second,
		RestoreOnStartup:         false,
		MemoryOnly:               false,
		RecoveryPath:             "/keep",
		StatePath:                "/keep-state",
		StateFirestoreDatabaseID: "(default)",
	}
	if raw.Effective() != raw {
		t.Fatalf("got %#v want %#v", raw.Effective(), raw)
	}
	if !raw.UsesRecovery() {
		t.Fatal("UsesRecovery must be true when memory_only false")
	}
}

func TestEffective_FirestoreDatabaseDefault(t *testing.T) {
	t.Parallel()
	raw := options.Options{StateBackend: "firestore", StateFirestoreProjectID: "p", StateFirestoreCollection: "c"}
	eff := raw.Effective()
	if eff.StateFirestoreDatabaseID != options.DefaultFirestoreDatabaseID {
		t.Fatalf("got %q want %q", eff.StateFirestoreDatabaseID, options.DefaultFirestoreDatabaseID)
	}
	raw.StateFirestoreDatabaseID = "other-db"
	eff = raw.Effective()
	if eff.StateFirestoreDatabaseID != "other-db" {
		t.Fatalf("got %q", eff.StateFirestoreDatabaseID)
	}
}

func TestEnvName(t *testing.T) {
	t.Parallel()
	if got := options.EnvName("snapshot_interval"); got != "ES4_SNAPSHOT_INTERVAL" {
		t.Fatalf("got %q", got)
	}
}

func TestLoad_RecoveryExtensions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "rec.yaml")
	body := `
recovery_backend: libsql
recovery_ttl: 24h
recovery_libsql_url: libsql://db.example
recovery_libsql_auth_token: secret
recovery_s3_bucket: ignored-when-libsql
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := options.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.RecoveryBackend != "libsql" || got.RecoveryTTL != 24*time.Hour {
		t.Fatalf("got backend=%q ttl=%v", got.RecoveryBackend, got.RecoveryTTL)
	}
	if got.RecoveryLibSQLURL != "libsql://db.example" || got.RecoveryLibSQLAuthToken != "secret" {
		t.Fatalf("libsql fields: %#v", got)
	}
}

func TestLoad_RecoveryObjectKeys(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "obj.yaml")
	if err := os.WriteFile(path, []byte(`
recovery_backend: object
recovery_s3_bucket: my-bkt
recovery_s3_prefix: pref/
recovery_s3_region: us-east-1
recovery_s3_endpoint: https://storage.googleapis.com
`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := options.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.RecoveryBackend != "object" || got.RecoveryS3Bucket != "my-bkt" {
		t.Fatalf("%#v", got)
	}
	if got.RecoveryS3Prefix != "pref/" || got.RecoveryS3Region != "us-east-1" || got.RecoveryS3Endpoint != "https://storage.googleapis.com" {
		t.Fatalf("%#v", got)
	}
}

func TestLoad_EmptyLibSQLAuthTokenSkipped(t *testing.T) {
	t.Parallel()
	base := options.Defaults()
	base.RecoveryBackend = "libsql"
	base.RecoveryLibSQLURL = "file:/tmp/x.db"
	base.RecoveryLibSQLAuthToken = "keep"
	got, err := options.ApplyEnv(base, func(k string) string {
		if k == "ES4_RECOVERY_LIBSQL_AUTH_TOKEN" {
			return ""
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.RecoveryLibSQLAuthToken != "keep" {
		t.Fatalf("empty env must not clear token, got %q", got.RecoveryLibSQLAuthToken)
	}
}

func TestLoad_InvalidRecoveryBackend(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("recovery_backend: litestream\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := options.FromFile(path); err == nil {
		t.Fatal("want error for invalid backend")
	}
}

func TestLoad_NegativeRecoveryTTL(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ttl.yaml")
	if err := os.WriteFile(path, []byte("recovery_ttl: -5s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := options.FromFile(path); err == nil {
		t.Fatal("want error for negative ttl")
	}
}

func TestValidate_BackendRequiredKeys(t *testing.T) {
	t.Parallel()
	cases := []options.Options{
		{RecoveryBackend: "file"},
		{RecoveryBackend: "libsql"},
		{RecoveryBackend: "object"},
	}
	for _, c := range cases {
		if err := c.Validate(); err == nil {
			t.Fatalf("want error for %#v", c)
		}
	}
	ok := options.Options{RecoveryPath: "/r"} // empty backend → file
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	// memory_only skips required keys
	mo := options.Options{MemoryOnly: true, RecoveryBackend: "libsql"}
	if err := mo.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidate_StateBackend(t *testing.T) {
	t.Parallel()
	if err := (options.Options{StateBackend: "redis"}).Validate(); err == nil {
		t.Fatal("want error: redis without url")
	}
	if err := (options.Options{StateBackend: "valkey"}).Validate(); err == nil {
		t.Fatal("want error: valkey without url")
	}
	if err := (options.Options{StateBackend: "sqlite"}).Validate(); err == nil {
		t.Fatal("want error: sqlite without state_path")
	}
	if err := (options.Options{StateBackend: "mongo"}).Validate(); err == nil {
		t.Fatal("want error: unknown state_backend")
	}
	if err := (options.Options{StateBackend: "Firestore"}).Validate(); err == nil {
		t.Fatal("want error: mixed case")
	}
	if err := (options.Options{StateBackend: "firestore"}).Validate(); err == nil {
		t.Fatal("want error: firestore without project/collection")
	}
	if err := (options.Options{
		StateBackend:             "firestore",
		StateFirestoreProjectID:  "p",
		StateFirestoreCollection: "a/b",
	}).Validate(); err == nil {
		t.Fatal("want error: slash in collection")
	}
	ok := options.Options{StateBackend: "redis", StateRedisURL: "redis://127.0.0.1:6379/0"}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	okFS := options.Options{
		StateBackend:             "firestore",
		StateFirestoreProjectID:  "demo-es4",
		StateFirestoreCollection: "es4_state",
	}
	if err := okFS.Validate(); err != nil {
		t.Fatal(err)
	}
	okMem := options.Options{StateBackend: "memory", StatePath: "/ignored.db"}
	if err := okMem.Validate(); err != nil {
		t.Fatal(err)
	}
	mo := options.Options{MemoryOnly: true, StateBackend: "redis"}
	if err := mo.Validate(); err != nil {
		t.Fatal(err)
	}
	moFS := options.Options{MemoryOnly: true, StateBackend: "firestore"}
	if err := moFS.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_StateFirestoreKeys(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "fs.yaml")
	if err := os.WriteFile(path, []byte(`
state_backend: firestore
state_firestore_project_id: demo-es4
state_firestore_database_id: custom-db
state_firestore_collection: es4_state
`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := options.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.StateBackend != "firestore" || got.StateFirestoreProjectID != "demo-es4" {
		t.Fatalf("%#v", got)
	}
	if got.StateFirestoreDatabaseID != "custom-db" || got.StateFirestoreCollection != "es4_state" {
		t.Fatalf("%#v", got)
	}
}

func TestLoad_StateRedisKeys(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "redis.yaml")
	if err := os.WriteFile(path, []byte(`
state_backend: redis
state_redis_url: redis://127.0.0.1:6379/1
state_redis_key_prefix: es4/test/
`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := options.FromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.StateBackend != "redis" || got.StateRedisURL != "redis://127.0.0.1:6379/1" {
		t.Fatalf("%#v", got)
	}
	if got.StateRedisKeyPrefix != "es4/test/" {
		t.Fatalf("prefix: %q", got.StateRedisKeyPrefix)
	}

	envGot, err := options.ApplyEnv(options.Defaults(), func(k string) string {
		switch k {
		case "ES4_STATE_BACKEND":
			return "valkey"
		case "ES4_STATE_REDIS_URL":
			return "redis://127.0.0.1:6380/0"
		case "ES4_STATE_REDIS_KEY_PREFIX":
			return "env/"
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if envGot.StateBackend != "valkey" || envGot.StateRedisURL != "redis://127.0.0.1:6380/0" || envGot.StateRedisKeyPrefix != "env/" {
		t.Fatalf("%#v", envGot)
	}
}

func TestResolvedRecoveryBackend(t *testing.T) {
	t.Parallel()
	if (options.Options{}).ResolvedRecoveryBackend() != "" {
		t.Fatal("want empty")
	}
	if (options.Options{RecoveryPath: "/x"}).ResolvedRecoveryBackend() != "file" {
		t.Fatal("want file from path")
	}
	if (options.Options{RecoveryBackend: "object", RecoveryPath: "/x"}).ResolvedRecoveryBackend() != "object" {
		t.Fatal("explicit backend wins")
	}
}
