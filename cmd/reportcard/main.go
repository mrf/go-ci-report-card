// Command reportcard generates a static Go project report card.
//
// Stage 1 of the Go rewrite: this binary parses the full flag set, loads and
// validates the configuration, and prints the resolved configuration. Checks,
// scoring, and site rendering arrive in later stages.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/mrf/go-ci-report-card/internal/config"
)

const (
	exitOK            = 0
	exitConfiguration = 2
)

// options holds the parsed command line.
type options struct {
	configPath   string
	repoRoot     string
	output       string
	githubOutput string
	enforce      bool
	overrides    config.Overrides
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}

// run is main without the process globals: getenv supplies environment
// variables so tests can run in parallel.
func run(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	opts, err := parseFlags(args, stderr, getenv)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}

		return exitConfiguration
	}

	repoRoot, err := filepath.Abs(opts.repoRoot)
	if err != nil {
		fmt.Fprintf(stderr, "configuration error: %v\n", err)

		return exitConfiguration
	}

	configPath := resolvePath(repoRoot, opts.configPath)
	output := resolvePath(repoRoot, opts.output)

	cfg, err := config.Load(configPath, opts.overrides)
	if err == nil {
		err = cfg.ValidateEnv(repoRoot, getenv)
	}

	if err != nil {
		fmt.Fprintf(stderr, "configuration error: %v\n", err)

		return exitConfiguration
	}

	fmt.Fprintf(stdout, "# repo_root = %q\n# source = %q\n# output = %q\n# enforce = %v\n# github_output = %q\n",
		repoRoot, cfg.SourcePath(repoRoot), output, opts.enforce, opts.githubOutput)

	if err := toml.NewEncoder(stdout).Encode(cfg); err != nil {
		fmt.Fprintf(stderr, "generation error: %v\n", err)

		return exitConfiguration
	}

	return exitOK
}

// resolvePath returns path unchanged when absolute or empty, else joined to
// repoRoot.
func resolvePath(repoRoot, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(repoRoot, path)
}

func parseFlags(args []string, stderr io.Writer, getenv func(string) string) (*options, error) {
	opts := &options{}
	fs := flag.NewFlagSet("reportcard", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		name           string
		sourceDir      string
		minScore       float64
		coverageTarget float64
	)

	fs.StringVar(&opts.configPath, "config", "", "TOML config path (defaults apply when absent)")
	fs.StringVar(&opts.repoRoot, "repo-root", ".", "repository root")
	fs.StringVar(&sourceDir, "source-dir", "", "overrides project.source_dir")
	fs.StringVar(&opts.output, "output", "_site", "output directory")
	fs.Float64Var(&minScore, "min-score", 0, "overrides quality.minimum_score")
	fs.Float64Var(&coverageTarget, "coverage-target", 0, "overrides checks.tests.coverage_target")
	fs.StringVar(&name, "name", "", "overrides project.name")
	fs.BoolVar(&opts.enforce, "enforce", false, "exit 1 when the quality gate fails")
	fs.StringVar(&opts.githubOutput, "github-output", getenv("GITHUB_OUTPUT"), "path of the GitHub Actions output file")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("parse flags: %w", err)
	}

	// Only flags the user actually passed become overrides, so a zero value
	// on the command line still wins over the file.
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "name":
			opts.overrides.Name = &name
		case "source-dir":
			opts.overrides.SourceDir = &sourceDir
		case "min-score":
			opts.overrides.MinimumScore = &minScore
		case "coverage-target":
			opts.overrides.CoverageTarget = &coverageTarget
		}
	})

	return opts, nil
}
