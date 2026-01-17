// Package bootstrap provides pipeline orchestration for Ralph initialization.
package bootstrap

import (
	"context"
	"fmt"
	"io"
)

// DecomposeResultInfo holds information about decomposition results.
type DecomposeResultInfo struct {
	TaskCount int
	YAMLPath  string
}

// ImportResultInfo holds information about import results.
type ImportResultInfo struct {
	TaskCount int
}

// Decomposer is the interface for PRD decomposition.
type Decomposer interface {
	Decompose(ctx context.Context) (*DecomposeResultInfo, error)
}

// Importer is the interface for task importing.
type Importer interface {
	Import() (*ImportResultInfo, error)
}

// Initializer is the interface for ralph initialization.
type Initializer interface {
	Init() error
}

// Runner is the interface for running the ralph loop.
type Runner interface {
	Run(ctx context.Context) error
}

// Options contains configuration for the bootstrap pipeline.
type Options struct {
	Once          bool
	MaxIterations int
	Parent        string
	Branch        string
}

// PRDPipeline orchestrates the decompose→import→init→run pipeline.
type PRDPipeline struct {
	decomposer  Decomposer
	importer    Importer
	initializer Initializer
	runner      Runner
	output      io.Writer
}

// NewPRDPipeline creates a new PRD pipeline with the given dependencies.
func NewPRDPipeline(d Decomposer, i Importer, init Initializer, r Runner, out io.Writer) *PRDPipeline {
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

// YAMLPipeline orchestrates the import→init→run pipeline for .yaml/.yml files.
// It skips the decomposition step entirely since tasks are already defined.
type YAMLPipeline struct {
	importer    Importer
	initializer Initializer
	runner      Runner
	output      io.Writer
}

// NewYAMLPipeline creates a new YAML pipeline with the given dependencies.
func NewYAMLPipeline(i Importer, init Initializer, r Runner, out io.Writer) *YAMLPipeline {
	return &YAMLPipeline{
		importer:    i,
		initializer: init,
		runner:      r,
		output:      out,
	}
}

// Execute runs the YAML bootstrap pipeline (import→init→run).
func (p *YAMLPipeline) Execute(ctx context.Context, yamlPath string) error {
	// Show initializing message
	_, _ = fmt.Fprintf(p.output, "Initializing from YAML: %s\n", yamlPath)

	// Step 1: Import
	result, err := p.importer.Import()
	if err != nil {
		return err
	}

	// Show task count summary
	_, _ = fmt.Fprintf(p.output, "Imported %d tasks\n", result.TaskCount)

	// Step 2: Init
	err = p.initializer.Init()
	if err != nil {
		return err
	}

	// Step 3: Run
	return p.runner.Run(ctx)
}
