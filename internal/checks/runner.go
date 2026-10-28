// Package checks runs the built-in and custom quality checks and turns each
// into a report row. Commands are argument vectors executed directly, never
// through a shell.
package checks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Exit codes the Python generator synthesised for launch failures, matching
// the shell's conventions.
const (
	exitTimedOut      = 124
	exitNotExecutable = 126
	exitNotFound      = 127
	exitLaunchFailed  = 1

	// waitDelay bounds how long a killed command's orphaned children may hold
	// the output pipe open before it is closed under them.
	waitDelay = time.Second
)

// Runner executes check commands in a directory with a timeout and trims
// their output.
type Runner struct {
	// Dir is the working directory for every command.
	Dir string
	// Timeout is the per-command deadline.
	Timeout time.Duration
	// DetailsLimit caps the number of output lines kept.
	DetailsLimit int
}

// Output is the result of one command.
type Output struct {
	Code     int
	Lines    []string
	Duration time.Duration
}

// Run executes argv in r.Dir with NO_COLOR and TERM=dumb set, returning the
// exit code, trimmed combined output, and wall time. Launch failures and
// timeouts are reported as exit codes with an explanatory line, never as
// errors, so a broken command scores zero rather than aborting the report.
func (r *Runner) Run(ctx context.Context, argv ...string) Output {
	started := time.Now()

	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	// argv comes from repo-owned config or fixed check definitions;
	// executing it is the point.
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // see above
	cmd.Dir = r.Dir
	cmd.WaitDelay = waitDelay
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "NO_COLOR=1", "TERM=dumb")

	var combined bytes.Buffer

	cmd.Stdout = &combined
	cmd.Stderr = &combined

	err := cmd.Run()
	duration := time.Since(started)

	switch {
	case err == nil:
		return Output{Code: 0, Lines: trimLines(combined.String(), r.DetailsLimit), Duration: duration}
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		lines := trimLines(combined.String(), max(0, r.DetailsLimit-1))
		lines = append(lines, fmt.Sprintf("Timed out after %d seconds", int(r.Timeout/time.Second)))

		return Output{Code: exitTimedOut, Lines: lines, Duration: duration}
	case errors.Is(err, exec.ErrNotFound):
		return Output{Code: exitNotFound, Lines: []string{"Command not found: " + argv[0]}, Duration: duration}
	case errors.Is(err, fs.ErrPermission):
		return Output{Code: exitNotExecutable, Lines: []string{"Command is not executable: " + argv[0]}, Duration: duration}
	}

	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return Output{Code: exit.ExitCode(), Lines: trimLines(combined.String(), r.DetailsLimit), Duration: duration}
	}

	return Output{
		Code:     exitLaunchFailed,
		Lines:    []string{"Unable to run " + argv[0] + ": " + err.Error()},
		Duration: duration,
	}
}

// trimLines strips blank lines and surrounding whitespace, then caps the
// result at limit lines with a note counting the omitted remainder. The
// result is never nil so it serialises as [] rather than null.
func trimLines(value string, limit int) []string {
	lines := []string{}

	for line := range strings.SplitSeq(value, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}

	if len(lines) > limit {
		omitted := len(lines) - limit
		lines = append(lines[:limit], fmt.Sprintf("… %d more lines omitted", omitted))
	}

	return lines
}
