package report

import (
	"fmt"
	"os"
	"strings"
)

// WriteCIMetadata appends the gate outputs (passed, score, grade) to the
// GitHub Actions output file and a markdown summary table to the step
// summary file. Either path may be empty, in which case that file is skipped.
func WriteCIMetadata(r *Report, githubOutput, stepSummary string) error {
	if githubOutput != "" {
		outputs := fmt.Sprintf("passed=%t\nscore=%s\ngrade=%s\n", r.Passed, r.Score, r.Grade)
		if err := appendFile(githubOutput, outputs); err != nil {
			return err
		}
	}

	if stepSummary != "" {
		if err := appendFile(stepSummary, Summary(r)); err != nil {
			return err
		}
	}

	return nil
}

// Summary renders the markdown step summary in the Python generator's format.
func Summary(r *Report) string {
	var sb strings.Builder

	outcome := "Needs attention"
	if r.Passed {
		outcome = "Passed"
	}

	fmt.Fprintf(&sb, "## Go CI Report Card: %s (%.1f)\n\n", r.Grade, float64(r.Score))
	fmt.Fprintf(&sb, "**Quality gate:** %s (minimum %s)\n\n", outcome, r.MinimumScore)
	sb.WriteString("| Check | Result | Score |\n|---|---:|---:|\n")

	for _, check := range r.Checks {
		fmt.Fprintf(&sb, "| %s | %s | %.1f |\n", check.Label, check.Observed, float64(check.Score))
	}

	return sb.String()
}

func appendFile(path, content string) error {
	// The path is $GITHUB_OUTPUT or $GITHUB_STEP_SUMMARY; appending to it is
	// the point.
	handle, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, filePerm) //nolint:gosec // see above
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}

	_, writeErr := handle.WriteString(content)

	if err := handle.Close(); err != nil && writeErr == nil {
		writeErr = err
	}

	if writeErr != nil {
		return fmt.Errorf("write %s: %w", path, writeErr)
	}

	return nil
}
