package config

import (
	"fmt"
	"os"
	"path/filepath"
)

var (
	getEnv      = os.Getenv
	userHomeDir = os.UserHomeDir
)

// GlobalConfigPath resolves the global config file path using XDG conventions.
func GlobalConfigPath() (string, error) {
	if xdgHome := getEnv("XDG_CONFIG_HOME"); xdgHome != "" {
		return filepath.Join(xdgHome, "ralph", "config.yaml"), nil
	}

	homeDir, err := userHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}

	return filepath.Join(homeDir, ".config", "ralph", "config.yaml"), nil
}
