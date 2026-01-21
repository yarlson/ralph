package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all Ralph harness configuration
type Config struct {
	Provider string         `mapstructure:"provider"`
	Claude   ClaudeConfig   `mapstructure:"claude"`
	OpenCode OpenCodeConfig `mapstructure:"opencode"`
	Safety   SafetyConfig   `mapstructure:"safety"`
}

// ClaudeConfig holds Claude Code invocation settings
type ClaudeConfig struct {
	Command []string `mapstructure:"command"`
	Args    []string `mapstructure:"args"`
}

// OpenCodeConfig holds OpenCode invocation settings
type OpenCodeConfig struct {
	Command []string `mapstructure:"command"`
	Args    []string `mapstructure:"args"`
}


// SafetyConfig holds safety and sandbox settings
type SafetyConfig struct {
	Sandbox         bool     `mapstructure:"sandbox"`
	AllowedCommands []string `mapstructure:"allowed_commands"`
}

// LoadConfigWithFile loads configuration from a specific file if provided,
// otherwise falls back to LoadConfig with the working directory.
func LoadConfigWithFile(workDir, configFile string) (*Config, error) {
	if configFile != "" {
		return LoadConfigFromPath(configFile)
	}

	localPath := filepath.Join(workDir, "ralph.yaml")
	if _, err := os.Stat(localPath); err == nil {
		return LoadConfig(workDir)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	globalPath, err := GlobalConfigPath()
	if err != nil {
		return nil, err
	}

	return LoadConfigFromPath(globalPath)
}

// LoadConfig loads configuration from ralph.yaml in the given directory.
// If no config file exists, sensible defaults are returned.
func LoadConfig(dir string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Configure viper
	v.SetConfigName("ralph")
	v.SetConfigType("yaml")
	v.AddConfigPath(dir)

	// Read config file (ignore not found errors)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	// Unmarshal into Config struct
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadConfigFromPath loads configuration from a specific file path
func LoadConfigFromPath(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Check if file exists
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return defaults
			cfg := &Config{}
			if err := v.Unmarshal(cfg); err != nil {
				return nil, err
			}
			return cfg, nil
		}
		return nil, err
	}

	// Configure viper to read from specific file
	v.SetConfigFile(configPath)

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	// Unmarshal into Config struct
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// setDefaults sets all default values for configuration
func setDefaults(v *viper.Viper) {
	// Claude defaults
	v.SetDefault("claude.command", []string{"claude"})
	v.SetDefault("claude.args", []string{})

	// OpenCode defaults
	v.SetDefault("opencode.command", []string{"opencode", "run"})
	v.SetDefault("opencode.args", []string{})

	// Provider defaults
	v.SetDefault("provider", "claude")

	// Safety defaults
	v.SetDefault("safety.sandbox", false)
	v.SetDefault("safety.allowed_commands", []string{"npm", "go", "git"})
}
