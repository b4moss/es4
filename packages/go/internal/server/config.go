package server

import (
	"fmt"
	"os"
	"strconv"
	"unicode"

	"github.com/b4moss/es4/packages/go/pkg/options"
)

const (
	envConfigPath = "ES4_CONFIG_PATH"
	envListenAddr = "ES4_LISTEN_ADDR"
	envPort       = "PORT"
	defaultListen = ":8080"
)

// LoadOptions builds library Options: Defaults → optional ES4_CONFIG_PATH YAML → env overlay.
func LoadOptions(getenv func(string) string) (options.Options, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	return options.Load(getenv(envConfigPath), getenv)
}

// ListenAddr resolves the HTTP listen address.
// Precedence: PORT (Cloud Run) > ES4_LISTEN_ADDR > :8080.
// PORT must be digits only; the result is ":{PORT}".
func ListenAddr(getenv func(string) string) (string, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	if port := getenv(envPort); port != "" {
		if !isDigits(port) {
			return "", fmt.Errorf("server: invalid PORT %q: want digits only", port)
		}
		if _, err := strconv.Atoi(port); err != nil {
			return "", fmt.Errorf("server: invalid PORT %q: %w", port, err)
		}
		return ":" + port, nil
	}
	if addr := getenv(envListenAddr); addr != "" {
		return addr, nil
	}
	return defaultListen, nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
