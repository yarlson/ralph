package config

import (
	"os"

	"github.com/spf13/viper"
)

// Config holds all Ralph harness configuration
type Config struct {
	Repo         RepoConfig         `mapstructure:"repo"`
	Tasks        TasksConfig        `mapstructure:"tasks"`
	Memory       MemoryConfig       `mapstructure:"memory"`
	Claude       ClaudeConfig       `mapstructure:"claude"`
	Verification VerificationConfig `mapstructure:"verification"`
	Loop         LoopConfig         `mapstructure:"loop"`
	Safety       SafetyConfig       `mapstructure:"safety"`
}

// RepoConfig holds repository-related settings
type RepoConfig struct {
	Root         string `mapstructure:"root"`
	BranchPrefix string `mapstructure:"branch_prefix"`
}

// TasksConfig holds task store settings
type TasksConfig struct {
	Backend      string `mapstructure:"backend"`
	Path         string `mapstructure:"path"`
	ParentIDFile string `mapstructure:"parent_id_file"`
}

// MemoryConfig holds memory/progress file settings
type MemoryConfig struct {
	ProgressFile string `mapstructure:"progress_file"`
	ArchiveDir   string `mapstructure:"archive_dir"`
}

// ClaudeConfig holds Claude Code invocation settings
type ClaudeConfig struct {
	Command []string `mapstructure:"command"`
	Args    []string `mapstructure:"args"`
}

// VerificationConfig holds verification command settings
type VerificationConfig struct {
	Commands [][]string `mapstructure:"commands"`
}

// LoopConfig holds iteration loop settings
type LoopConfig struct {
	MaxIterations          int          `mapstructure:"max_iterations"`
	MaxMinutesPerIteration int          `mapstructure:"max_minutes_per_iteration"`
	MaxRetries             int          `mapstructure:"max_retries"`
	MaxVerificationRetries int          `mapstructure:"max_verification_retries"`
	Gutter                 GutterConfig `mapstructure:"gutter"`
}

// GutterConfig holds gutter detection settings
type GutterConfig struct {
	MaxSameFailure     int  `mapstructure:"max_same_failure"`
	MaxChurnCommits    int  `mapstructure:"max_churn_commits"`
	MaxOscillations    int  `mapstructure:"max_oscillations"`
	EnableContentHash  bool `mapstructure:"enable_content_hash"`
	MaxChurnIterations int  `mapstructure:"max_churn_iterations"`
	ChurnThreshold     int  `mapstructure:"churn_threshold"`
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
	return LoadConfig(workDir)
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
	// Repo defaults
	v.SetDefault("repo.root", ".")
	v.SetDefault("repo.branch_prefix", "ralph/")

	// Tasks defaults
	v.SetDefault("tasks.backend", "local")
	v.SetDefault("tasks.path", ".ralph/tasks")
	v.SetDefault("tasks.parent_id_file", ".ralph/parent-task-id")

	// Memory defaults
	v.SetDefault("memory.progress_file", ".ralph/progress.md")
	v.SetDefault("memory.archive_dir", ".ralph/archive")

	// Claude defaults
	v.SetDefault("claude.command", []string{"claude"})
	v.SetDefault("claude.args", []string{})

	// Verification defaults (empty by default)
	v.SetDefault("verification.commands", [][]string{})

	// Loop defaults
	v.SetDefault("loop.max_iterations", 50)
	v.SetDefault("loop.max_minutes_per_iteration", 20)
	v.SetDefault("loop.max_retries", 2)
	v.SetDefault("loop.max_verification_retries", 2)
	v.SetDefault("loop.gutter.max_same_failure", 3)
	v.SetDefault("loop.gutter.max_churn_commits", 2)
	v.SetDefault("loop.gutter.max_oscillations", 2)
	v.SetDefault("loop.gutter.enable_content_hash", true)
	v.SetDefault("loop.gutter.max_churn_iterations", 5)
	v.SetDefault("loop.gutter.churn_threshold", 3)

	// Safety defaults
	v.SetDefault("safety.sandbox", false)
	v.SetDefault("safety.allowed_commands", []string{"npm", "go", "git"})
}
