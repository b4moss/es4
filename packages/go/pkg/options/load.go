package options

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Load builds Options as Defaults → optional JSON config file → env overlay.
// If configPath is empty, the file step is skipped. getenv may be nil to use os.Getenv.
func Load(configPath string, getenv func(string) string) (Options, error) {
	opts := Defaults()
	if configPath != "" {
		fileOpts, err := loadFile(configPath)
		if err != nil {
			return Options{}, err
		}
		opts = merge(opts, fileOpts)
	}
	return ApplyEnv(opts, getenv)
}

// FromFile loads a JSON config file and merges it onto Defaults (no env overlay).
func FromFile(path string) (Options, error) {
	fileOpts, err := loadFile(path)
	if err != nil {
		return Options{}, err
	}
	return merge(Defaults(), fileOpts), nil
}

// ApplyEnv overlays ES4_* environment variables onto opts.
// Only variables that are set (non-empty) override. getenv may be nil to use os.Getenv.
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
		b, err := parseBool(v)
		if err != nil {
			return Options{}, fmt.Errorf("options: %sRESTORE_ON_STARTUP: %w", EnvPrefix, err)
		}
		overlay.restoreOnStartup = &b
	}
	if v := getenv(EnvPrefix + "MEMORY_ONLY"); v != "" {
		b, err := parseBool(v)
		if err != nil {
			return Options{}, fmt.Errorf("options: %sMEMORY_ONLY: %w", EnvPrefix, err)
		}
		overlay.memoryOnly = &b
	}
	if v := getenv(EnvPrefix + "RECOVERY_PATH"); v != "" {
		overlay.recoveryPath = &v
	}
	return merge(opts, overlay), nil
}

// EnvName returns the ES4_* environment variable name for a snake_case option key.
func EnvName(snakeKey string) string {
	return EnvPrefix + strings.ToUpper(snakeKey)
}

type partial struct {
	snapshotInterval *time.Duration
	restoreOnStartup *bool
	memoryOnly       *bool
	recoveryPath     *string
}

type fileDTO struct {
	SnapshotInterval json.RawMessage `json:"snapshot_interval"`
	RestoreOnStartup json.RawMessage `json:"restore_on_startup"`
	MemoryOnly       json.RawMessage `json:"memory_only"`
	RecoveryPath     *string         `json:"recovery_path"`
}

func loadFile(path string) (partial, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return partial{}, fmt.Errorf("options: read config %q: %w", path, err)
	}
	var dto fileDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return partial{}, fmt.Errorf("options: parse config %q: %w", path, err)
	}
	out := partial{recoveryPath: dto.RecoveryPath}
	if len(dto.SnapshotInterval) > 0 {
		d, err := parseJSONDuration(dto.SnapshotInterval)
		if err != nil {
			return partial{}, fmt.Errorf("options: snapshot_interval: %w", err)
		}
		out.snapshotInterval = &d
	}
	if len(dto.RestoreOnStartup) > 0 {
		b, err := parseJSONBool(dto.RestoreOnStartup)
		if err != nil {
			return partial{}, fmt.Errorf("options: restore_on_startup: %w", err)
		}
		out.restoreOnStartup = &b
	}
	if len(dto.MemoryOnly) > 0 {
		b, err := parseJSONBool(dto.MemoryOnly)
		if err != nil {
			return partial{}, fmt.Errorf("options: memory_only: %w", err)
		}
		out.memoryOnly = &b
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
	return out
}

func parseJSONDuration(raw json.RawMessage) (time.Duration, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return parseDuration(s)
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return time.Duration(n * float64(time.Second)), nil
	}
	return 0, fmt.Errorf("invalid duration %s", string(raw))
}

func parseDuration(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	return d, nil
}

func parseJSONBool(raw json.RawMessage) (bool, error) {
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return parseBool(s)
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		switch n {
		case 0:
			return false, nil
		case 1:
			return true, nil
		}
	}
	return false, fmt.Errorf("invalid bool %s", string(raw))
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "on", "yes":
		return true, nil
	case "0", "false", "off", "no":
		return false, nil
	default:
		if _, err := strconv.ParseBool(s); err == nil {
			return strconv.ParseBool(s)
		}
		return false, fmt.Errorf("invalid bool %q", s)
	}
}
