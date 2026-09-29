package options

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Load builds Options as Defaults → optional YAML config file → env overlay.
// If configPath is empty, the file step is skipped. getenv may be nil to use os.Getenv.
// Empty env values are treated as unset (skipped) and do not clear underlying values.
func Load(configPath string, getenv func(string) string) (Options, error) {
	opts := Defaults()
	if configPath != "" {
		fileOpts, err := loadFile(configPath)
		if err != nil {
			return Options{}, err
		}
		opts = merge(opts, fileOpts)
	}
	opts, err := ApplyEnv(opts, getenv)
	if err != nil {
		return Options{}, err
	}
	if err := opts.Validate(); err != nil {
		return Options{}, err
	}
	return opts, nil
}

// FromFile loads a YAML config file and merges it onto Defaults (no env overlay).
func FromFile(path string) (Options, error) {
	fileOpts, err := loadFile(path)
	if err != nil {
		return Options{}, err
	}
	opts := merge(Defaults(), fileOpts)
	if err := opts.Validate(); err != nil {
		return Options{}, err
	}
	return opts, nil
}

// ApplyEnv overlays ES4_* environment variables onto opts.
// Only variables that are set (non-empty) override. Empty means unset and is skipped.
// getenv may be nil to use os.Getenv.
func ApplyEnv(opts Options, getenv func(string) string) (Options, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	overlay := partial{}
	if v := getenv(EnvPrefix + "SNAPSHOT_INTERVAL"); v != "" {
		d, err := parseDuration(v)
		if err != nil {
			return Options{}, fmt.Errorf("options: %sSNAPSHOT_INTERVAL: %w", EnvPrefix, err)
		}
		overlay.snapshotInterval = &d
	}
	if v := getenv(EnvPrefix + "RESTORE_ON_STARTUP"); v != "" {
		b, err := parseStrictBool(v)
		if err != nil {
			return Options{}, fmt.Errorf("options: %sRESTORE_ON_STARTUP: %w", EnvPrefix, err)
		}
		overlay.restoreOnStartup = &b
	}
	if v := getenv(EnvPrefix + "MEMORY_ONLY"); v != "" {
		b, err := parseStrictBool(v)
		if err != nil {
			return Options{}, fmt.Errorf("options: %sMEMORY_ONLY: %w", EnvPrefix, err)
		}
		overlay.memoryOnly = &b
	}
	if v := getenv(EnvPrefix + "RECOVERY_PATH"); v != "" {
		overlay.recoveryPath = &v
	}
	if v := getenv(EnvPrefix + "RECOVERY_BACKEND"); v != "" {
		overlay.recoveryBackend = &v
	}
	if v := getenv(EnvPrefix + "RECOVERY_TTL"); v != "" {
		d, err := parseDuration(v)
		if err != nil {
			return Options{}, fmt.Errorf("options: %sRECOVERY_TTL: %w", EnvPrefix, err)
		}
		overlay.recoveryTTL = &d
	}
	if v := getenv(EnvPrefix + "RECOVERY_LIBSQL_URL"); v != "" {
		overlay.recoveryLibSQLURL = &v
	}
	if v := getenv(EnvPrefix + "RECOVERY_LIBSQL_AUTH_TOKEN"); v != "" {
		overlay.recoveryLibSQLAuthToken = &v
	}
	if v := getenv(EnvPrefix + "RECOVERY_S3_BUCKET"); v != "" {
		overlay.recoveryS3Bucket = &v
	}
	if v := getenv(EnvPrefix + "RECOVERY_S3_PREFIX"); v != "" {
		overlay.recoveryS3Prefix = &v
	}
	if v := getenv(EnvPrefix + "RECOVERY_S3_REGION"); v != "" {
		overlay.recoveryS3Region = &v
	}
	if v := getenv(EnvPrefix + "RECOVERY_S3_ENDPOINT"); v != "" {
		overlay.recoveryS3Endpoint = &v
	}
	if v := getenv(EnvPrefix + "STATE_PATH"); v != "" {
		overlay.statePath = &v
	}
	return merge(opts, overlay), nil
}

// EnvName returns the ES4_* environment variable name for a snake_case option key.
func EnvName(snakeKey string) string {
	return EnvPrefix + strings.ToUpper(snakeKey)
}

type partial struct {
	snapshotInterval        *time.Duration
	restoreOnStartup        *bool
	memoryOnly              *bool
	recoveryPath            *string
	recoveryBackend         *string
	recoveryTTL             *time.Duration
	recoveryLibSQLURL       *string
	recoveryLibSQLAuthToken *string
	recoveryS3Bucket        *string
	recoveryS3Prefix        *string
	recoveryS3Region        *string
	recoveryS3Endpoint      *string
	statePath               *string
}

func loadFile(path string) (partial, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return partial{}, fmt.Errorf("options: read config %q: %w", path, err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return partial{}, fmt.Errorf("options: parse config %q: %w", path, err)
	}
	doc := &root
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return partial{}, nil
		}
		doc = root.Content[0]
	}
	if doc.Kind == yaml.ScalarNode && (doc.Tag == "!!null" || doc.Value == "") {
		return partial{}, nil
	}
	if doc.Kind != yaml.MappingNode {
		return partial{}, fmt.Errorf("options: parse config %q: root must be a mapping", path)
	}

	out := partial{}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		keyNode := doc.Content[i]
		valNode := doc.Content[i+1]
		switch keyNode.Value {
		case "snapshot_interval":
			d, err := parseDurationNode(valNode)
			if err != nil {
				return partial{}, fmt.Errorf("options: snapshot_interval: %w", err)
			}
			out.snapshotInterval = &d
		case "restore_on_startup":
			b, err := parseStrictBoolNode(valNode)
			if err != nil {
				return partial{}, fmt.Errorf("options: restore_on_startup: %w", err)
			}
			out.restoreOnStartup = &b
		case "memory_only":
			b, err := parseStrictBoolNode(valNode)
			if err != nil {
				return partial{}, fmt.Errorf("options: memory_only: %w", err)
			}
			out.memoryOnly = &b
		case "recovery_path":
			s, err := parseStringNode(valNode)
			if err != nil {
				return partial{}, fmt.Errorf("options: recovery_path: %w", err)
			}
			out.recoveryPath = &s
		case "recovery_backend":
			s, err := parseStringNode(valNode)
			if err != nil {
				return partial{}, fmt.Errorf("options: recovery_backend: %w", err)
			}
			out.recoveryBackend = &s
		case "recovery_ttl":
			d, err := parseDurationNode(valNode)
			if err != nil {
				return partial{}, fmt.Errorf("options: recovery_ttl: %w", err)
			}
			out.recoveryTTL = &d
		case "recovery_libsql_url":
			s, err := parseStringNode(valNode)
			if err != nil {
				return partial{}, fmt.Errorf("options: recovery_libsql_url: %w", err)
			}
			out.recoveryLibSQLURL = &s
		case "recovery_libsql_auth_token":
			s, err := parseStringNode(valNode)
			if err != nil {
				return partial{}, fmt.Errorf("options: recovery_libsql_auth_token: %w", err)
			}
			out.recoveryLibSQLAuthToken = &s
		case "recovery_s3_bucket":
			s, err := parseStringNode(valNode)
			if err != nil {
				return partial{}, fmt.Errorf("options: recovery_s3_bucket: %w", err)
			}
			out.recoveryS3Bucket = &s
		case "recovery_s3_prefix":
			s, err := parseStringNode(valNode)
			if err != nil {
				return partial{}, fmt.Errorf("options: recovery_s3_prefix: %w", err)
			}
			out.recoveryS3Prefix = &s
		case "recovery_s3_region":
			s, err := parseStringNode(valNode)
			if err != nil {
				return partial{}, fmt.Errorf("options: recovery_s3_region: %w", err)
			}
			out.recoveryS3Region = &s
		case "recovery_s3_endpoint":
			s, err := parseStringNode(valNode)
			if err != nil {
				return partial{}, fmt.Errorf("options: recovery_s3_endpoint: %w", err)
			}
			out.recoveryS3Endpoint = &s
		case "state_path":
			s, err := parseStringNode(valNode)
			if err != nil {
				return partial{}, fmt.Errorf("options: state_path: %w", err)
			}
			out.statePath = &s
		}
	}
	return out, nil
}

func merge(base Options, over partial) Options {
	out := base
	if over.snapshotInterval != nil {
		out.SnapshotInterval = *over.snapshotInterval
	}
	if over.restoreOnStartup != nil {
		out.RestoreOnStartup = *over.restoreOnStartup
	}
	if over.memoryOnly != nil {
		out.MemoryOnly = *over.memoryOnly
	}
	if over.recoveryPath != nil {
		out.RecoveryPath = *over.recoveryPath
	}
	if over.recoveryBackend != nil {
		out.RecoveryBackend = *over.recoveryBackend
	}
	if over.recoveryTTL != nil {
		out.RecoveryTTL = *over.recoveryTTL
	}
	if over.recoveryLibSQLURL != nil {
		out.RecoveryLibSQLURL = *over.recoveryLibSQLURL
	}
	if over.recoveryLibSQLAuthToken != nil {
		out.RecoveryLibSQLAuthToken = *over.recoveryLibSQLAuthToken
	}
	if over.recoveryS3Bucket != nil {
		out.RecoveryS3Bucket = *over.recoveryS3Bucket
	}
	if over.recoveryS3Prefix != nil {
		out.RecoveryS3Prefix = *over.recoveryS3Prefix
	}
	if over.recoveryS3Region != nil {
		out.RecoveryS3Region = *over.recoveryS3Region
	}
	if over.recoveryS3Endpoint != nil {
		out.RecoveryS3Endpoint = *over.recoveryS3Endpoint
	}
	if over.statePath != nil {
		out.StatePath = *over.statePath
	}
	return out
}

func parseDurationNode(n *yaml.Node) (time.Duration, error) {
	if n == nil || n.Kind != yaml.ScalarNode {
		return 0, fmt.Errorf("want Go duration string")
	}
	switch n.Tag {
	case "!!int", "!!float":
		return 0, fmt.Errorf("numeric seconds rejected; use Go duration string (e.g. \"30s\"), got %s", n.Value)
	}
	// Reject bare numeric strings even when tagged as !!str.
	if _, err := strconv.ParseFloat(n.Value, 64); err == nil {
		return 0, fmt.Errorf("numeric seconds rejected; use Go duration string (e.g. \"30s\"), got %q", n.Value)
	}
	return parseDuration(n.Value)
}

func parseDuration(s string) (time.Duration, error) {
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return 0, fmt.Errorf("numeric seconds rejected; use Go duration string (e.g. \"30s\"), got %q", s)
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: want Go duration string (e.g. \"30s\")", s)
	}
	return d, nil
}

func parseStrictBoolNode(n *yaml.Node) (bool, error) {
	if n == nil || n.Kind != yaml.ScalarNode {
		return false, fmt.Errorf("want lowercase true or false")
	}
	return parseStrictBool(n.Value)
}

func parseStrictBool(s string) (bool, error) {
	switch s {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool %q: want lowercase true or false", s)
	}
}

func parseStringNode(n *yaml.Node) (string, error) {
	if n == nil || n.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("want string")
	}
	if n.Tag == "!!null" {
		return "", nil
	}
	return n.Value, nil
}
