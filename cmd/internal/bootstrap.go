// Package internal provides shared utilities for Ralph CLI commands.
package internal

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// FileType represents the detected type of an input file.
type FileType int

// File type constants.
const (
	FileTypeUnknown FileType = iota
	FileTypePRD
	FileTypeTasks
)

// String returns the string representation of the FileType.
func (ft FileType) String() string {
	switch ft {
	case FileTypePRD:
		return "prd"
	case FileTypeTasks:
		return "tasks"
	default:
		return "unknown"
	}
}

// prdPatterns are regex patterns that indicate PRD/spec content.
var prdPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)##\s*objectives`),
	regexp.MustCompile(`(?i)##\s*requirements`),
	regexp.MustCompile(`(?i)##\s*user\s+stories`),
	regexp.MustCompile(`(?i)##\s*acceptance\s+criteria`),
	regexp.MustCompile(`(?i)##\s*overview`),
}

// taskPatterns are patterns that indicate task YAML content.
var taskPatterns = []string{
	"id:",
	"tasks:",
	"status:",
	"depends_on:",
}

// DetectFileType analyzes content and returns the detected file type.
// It checks for PRD markers first, then task YAML patterns.
func DetectFileType(content string) FileType {
	// Check for PRD patterns (markdown sections)
	for _, pattern := range prdPatterns {
		if pattern.MatchString(content) {
			return FileTypePRD
		}
	}

	// Check for task YAML patterns
	for _, pattern := range taskPatterns {
		if strings.Contains(content, pattern) {
			return FileTypeTasks
		}
	}

	return FileTypeUnknown
}

// DecomposeResultInfo holds information about decomposition results.
type DecomposeResultInfo struct {
	TaskCount int
	YAMLPath  string
}

// ImportResultInfo holds information about import results.
type ImportResultInfo struct {
	TaskCount int
}

// PipelineDecomposer is the interface for PRD decomposition.
type PipelineDecomposer interface {
	Decompose(ctx context.Context) (*DecomposeResultInfo, error)
}

// PipelineImporter is the interface for task importing.
type PipelineImporter interface {
	Import() (*ImportResultInfo, error)
}

// PipelineInitializer is the interface for ralph initialization.
type PipelineInitializer interface {
	Init() error
}

// PipelineRunner is the interface for running the ralph loop.
type PipelineRunner interface {
	Run(ctx context.Context) error
}

// PRDPipeline orchestrates the decompose→import→init→run pipeline.
type PRDPipeline struct {
	decomposer  PipelineDecomposer
	importer    PipelineImporter
	initializer PipelineInitializer
	runner      PipelineRunner
	output      io.Writer
}

// NewPRDPipeline creates a new PRD pipeline with the given dependencies.
func NewPRDPipeline(d PipelineDecomposer, i PipelineImporter, init PipelineInitializer, r PipelineRunner, out io.Writer) *PRDPipeline {
	return &PRDPipeline{
		decomposer:  d,
		importer:    i,
		initializer: init,
		runner:      r,
		output:      out,
	}
}

// Execute runs the full PRD bootstrap pipeline.
func (p *PRDPipeline) Execute(ctx context.Context, prdPath string) error {
	// Show analyzing message
	_, _ = fmt.Fprintf(p.output, "Analyzing PRD: %s\n", prdPath)

	// Step 1: Decompose
	result, err := p.decomposer.Decompose(ctx)
	if err != nil {
		return err
	}

	// Show task count summary
	_, _ = fmt.Fprintf(p.output, "Generated %d tasks\n", result.TaskCount)

	// Step 2: Import
	_, err = p.importer.Import()
	if err != nil {
		return err
	}

	// Step 3: Init
	err = p.initializer.Init()
	if err != nil {
		return err
	}

	// Step 4: Run
	return p.runner.Run(ctx)
}

// BootstrapOptions contains configuration for the bootstrap pipeline.
type BootstrapOptions struct {
	Once          bool
	MaxIterations int
	Parent        string
	Branch        string
}

// BootstrapFromPRD orchestrates the decompose→import→init→run pipeline for .md files.
// It takes a PRD file path and runs the full bootstrap sequence.
func BootstrapFromPRD(ctx context.Context, prdPath string, out io.Writer, opts *BootstrapOptions) error {
	// This is a placeholder that will be called from root.go
	// The actual implementation with real dependencies will be in root.go
	// This function signature is here to define the contract
	return fmt.Errorf("PRD file not found: %s", prdPath)
}
