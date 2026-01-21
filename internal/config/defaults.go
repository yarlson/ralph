package config

// Repo defaults
const (
	DefaultRepoRoot     = "."
	DefaultBranchPrefix = "ralph/"
)

// Tasks defaults
const (
	DefaultTasksBackend = "local"
	DefaultTasksPath    = ".ralph/tasks"
	DefaultParentIDFile = ".ralph/parent-task-id"
	DefaultTasksFile    = ".ralph/tasks/tasks.yaml"
)

// Memory defaults
const (
	DefaultProgressFile        = ".ralph/progress.md"
	DefaultArchiveDir          = ".ralph/archive"
	DefaultMaxProgressBytes    = 1048576
	DefaultMaxRecentIterations = 20
)

// Loop defaults
const (
	DefaultMaxIterations          = 50
	DefaultMaxMinutesPerIteration = 20
	DefaultMaxRetries             = 2
	DefaultMaxVerificationRetries = 2
)

// Gutter detection defaults
const (
	DefaultMaxSameFailure     = 3
	DefaultMaxChurnCommits    = 2
	DefaultMaxOscillations    = 2
	DefaultEnableContentHash  = true
	DefaultMaxChurnIterations = 5
	DefaultChurnThreshold     = 3
)
