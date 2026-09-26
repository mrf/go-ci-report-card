package checks

import (
	"context"
	"strconv"
	"time"

	"github.com/mrf/go-ci-report-card/internal/config"
	"github.com/mrf/go-ci-report-card/internal/report"
)

// Built-in check ids, in report order.
const (
	IDFormat  = "format"
	IDVet     = "vet"
	IDBuild   = "build"
	IDTests   = "tests"
	IDModules = "modules"
)

// info is a built-in check's label and description.
type info struct {
	label       string
	description string
}

// builtinInfo is the Python CHECK_INFO table.
//
//nolint:gochecknoglobals // immutable lookup table
var builtinInfo = map[string]info{
	IDFormat:  {"Formatting", "Go source files match gofmt's canonical format."},
	IDVet:     {"Go vet", "The standard Go analyzer found no suspicious constructs."},
	IDBuild:   {"Build", "Every package builds successfully with the project toolchain."},
	IDTests:   {"Tests & coverage", "The test suite passes and reaches the configured coverage target."},
	IDModules: {"Module integrity", "Downloaded module content matches the hashes in go.sum."},
}

// builtinCommands are the single-command pass/fail checks.
//
//nolint:gochecknoglobals // immutable lookup table
var builtinCommands = map[string][]string{
	IDVet:     {"go", "vet", "./..."},
	IDBuild:   {"go", "build", "./..."},
	IDModules: {"go", "mod", "verify"},
}

// Info returns the label and description of a built-in check id.
func Info(id string) (string, string) {
	entry := builtinInfo[id]

	return entry.label, entry.description
}

// NewRunner builds the shared runner from validated configuration.
func NewRunner(cfg *config.Config, source string) *Runner {
	return &Runner{
		Dir:          source,
		Timeout:      time.Duration(cfg.Quality.CommandTimeoutSeconds) * time.Second,
		DetailsLimit: cfg.Quality.DetailsLimit,
	}
}

// RunAll executes every enabled built-in check in report order, then every
// enabled custom check, and returns their rows.
func RunAll(ctx context.Context, cfg *config.Config, source string) []report.Check {
	runner := NewRunner(cfg, source)
	results := make([]report.Check, 0, len(cfg.EnabledChecks())+len(cfg.CustomChecks))

	for _, check := range cfg.EnabledChecks() {
		switch check.ID {
		case IDFormat:
			results = append(results, Format(ctx, runner, check.Weight, cfg.Project.Exclude))
		case IDTests:
			results = append(results, Tests(ctx, runner, check.Weight, cfg.Checks.Tests.CoverageTarget))
		default:
			results = append(results, Command(ctx, runner, check.ID, check.Weight))
		}
	}

	for _, custom := range cfg.EnabledCustomChecks() {
		results = append(results, Custom(ctx, runner, &custom))
	}

	return results
}

// passFail scores a command by exit code: 100 and "Clean" on success, else 0
// and "Exited N" with the captured output as details.
func passFail(checkID, label, description string, weight float64, out Output) report.Check {
	if out.Code == 0 {
		return report.NewCheck(checkID, label, description, weight, report.MaxScore, "Clean", nil, out.Duration.Seconds(), "")
	}

	observed := "Exited " + strconv.Itoa(out.Code)

	return report.NewCheck(checkID, label, description, weight, 0, observed, out.Lines, out.Duration.Seconds(), "")
}

// Command runs a single-command built-in check (vet, build, modules).
func Command(ctx context.Context, runner *Runner, id string, weight float64) report.Check {
	label, description := Info(id)

	return passFail(id, label, description, weight, runner.Run(ctx, builtinCommands[id]...))
}

// Custom runs one [[custom_checks]] entry.
func Custom(ctx context.Context, runner *Runner, check *config.CustomCheck) report.Check {
	return passFail(check.ID, check.Label, check.Description, check.Weight, runner.Run(ctx, check.Command...))
}
