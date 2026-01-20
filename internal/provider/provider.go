package provider

import (
	"fmt"
	"strings"
)

const (
	Claude   = "claude"
	OpenCode = "opencode"
)

func Normalize(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return Claude, nil
	}

	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case Claude, OpenCode:
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported provider: %s", value)
	}
}

func Resolve(cliValue, configValue string) (string, error) {
	if strings.TrimSpace(cliValue) != "" {
		return Normalize(cliValue)
	}
	return Normalize(configValue)
}
