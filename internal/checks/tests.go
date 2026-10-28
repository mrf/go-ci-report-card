package checks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mrf/go-ci-report-card/internal/report"
)

// totalPattern matches the summary line of `go tool cover -func`.
var totalPattern = regexp.MustCompile(`total:\s+\(statements\)\s+([0-9.]+)%`)

// Tests runs the test suite with coverage and scores the total coverage
// against target: 100 when target is not positive, else coverage/target
// capped at 100. Status is passed at or above target, warning above zero,
// failed at zero.
func Tests(ctx context.Context, runner *Runner, weight float64, target float64) report.Check {
	label, description := Info(IDTests)

	fail := func(observed string, details []string, duration float64) report.Check {
		return report.NewCheck(IDTests, label, description, weight, 0, observed, details, duration, "")
	}

	tempDir, err := os.MkdirTemp("", "go-ci-report-card-")
	if err != nil {
		return fail("Coverage unavailable", []string{err.Error()}, 0)
	}

	defer os.RemoveAll(tempDir) //nolint:errcheck // best-effort cleanup of a temp dir

	profile := filepath.Join(tempDir, "coverage.out")

	run := runner.Run(ctx, "go", "test", "-covermode=atomic", "-coverprofile="+profile, "./...")
	if run.Code != 0 {
		return fail("Tests failed", run.Lines, run.Duration.Seconds())
	}

	cover := runner.Run(ctx, "go", "tool", "cover", "-func="+profile)
	duration := (run.Duration + cover.Duration).Seconds()

	if cover.Code != 0 {
		return fail("Coverage unavailable", cover.Lines, duration)
	}

	coverage, ok := ParseCoverage(cover.Lines)
	if !ok {
		return fail("Coverage unavailable", cover.Lines, duration)
	}

	score := report.MaxScore
	if target > 0 {
		score = min(report.MaxScore, coverage/target*report.MaxScore)
	}

	status := report.StatusFailed
	details := []string{}

	switch {
	case coverage >= target:
		status = report.StatusPassed
	case coverage > 0:
		status = report.StatusWarning
	}

	if status != report.StatusPassed {
		details = append(details, fmt.Sprintf("Coverage is %.1f points below target.", target-coverage))
	}

	observed := fmt.Sprintf("%.1f%% / %s%% target", coverage, formatG(target))

	return report.NewCheck(IDTests, label, description, weight, score, observed, details, duration, status)
}

// ParseCoverage extracts the total statement coverage percentage from
// `go tool cover -func` output. The bool is false when no total line is
// present or the number does not parse.
func ParseCoverage(lines []string) (float64, bool) {
	for _, line := range lines {
		if !strings.HasPrefix(line, "total:") {
			continue
		}

		match := totalPattern.FindStringSubmatch(line)
		if match == nil {
			return 0, false
		}

		value, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return 0, false
		}

		return value, true
	}

	return 0, false
}

// formatG formats like Python's "%g": six significant digits, trailing
// zeros removed.
func formatG(value float64) string {
	const significant = 6

	return strconv.FormatFloat(value, 'g', significant, 64)
}
