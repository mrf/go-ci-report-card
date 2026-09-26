package checks_test

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mrf/go-ci-report-card/internal/checks"
)

// helperArg marks an invocation of this test binary as a helper process.
// TestMain dispatches on it before the testing framework parses flags, so
// runner tests get a portable command with controllable exit code, output,
// and duration.
const helperArg = "reportcard-helper"

func TestMain(m *testing.M) {
	if len(os.Args) >= 3 && os.Args[1] == helperArg {
		os.Exit(helper(os.Args[2], os.Args[3:]))
	}

	os.Exit(m.Run())
}

// helper implements the helper commands: "exit N", "lines N" (prints N
// numbered lines with blanks between), and "sleep SECONDS".
func helper(command string, args []string) int {
	switch command {
	case "exit":
		code, _ := strconv.Atoi(args[0])

		return code
	case "lines":
		count, _ := strconv.Atoi(args[0])
		for i := range count {
			fmt.Fprintf(os.Stdout, "  line %d  \n\n", i+1)
		}

		return 0
	case "sleep":
		fmt.Fprintln(os.Stdout, "about to sleep")

		seconds, _ := strconv.Atoi(args[0])
		time.Sleep(time.Duration(seconds) * time.Second)

		return 0
	default:
		return 99
	}
}

func helperArgv(t *testing.T, args ...string) []string {
	t.Helper()

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	return append([]string{exe, helperArg}, args...)
}

func newRunner(t *testing.T, timeout time.Duration, limit int) *checks.Runner {
	t.Helper()

	return &checks.Runner{Dir: t.TempDir(), Timeout: timeout, DetailsLimit: limit}
}

func TestRunCapturesExitCodeAndTrimmedOutput(t *testing.T) {
	t.Parallel()

	runner := newRunner(t, 30*time.Second, 3)

	out := runner.Run(t.Context(), helperArgv(t, "exit", "7")...)
	if out.Code != 7 || len(out.Lines) != 0 || out.Duration <= 0 {
		t.Errorf("exit 7: got %+v", out)
	}

	out = runner.Run(t.Context(), helperArgv(t, "lines", "5")...)
	want := []string{"line 1", "line 2", "line 3", "… 2 more lines omitted"}

	if out.Code != 0 || strings.Join(out.Lines, "|") != strings.Join(want, "|") {
		t.Errorf("lines 5: got %+v, want lines %q", out, want)
	}
}

func TestRunLaunchFailures(t *testing.T) {
	t.Parallel()

	runner := newRunner(t, 30*time.Second, 10)

	out := runner.Run(t.Context(), "reportcard-definitely-missing-binary")
	if out.Code != 127 || strings.Join(out.Lines, "") != "Command not found: reportcard-definitely-missing-binary" {
		t.Errorf("missing binary: got %+v", out)
	}

	script := runner.Dir + "/not-executable"
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out = runner.Run(t.Context(), script)
	if out.Code != 126 || strings.Join(out.Lines, "") != "Command is not executable: "+script {
		t.Errorf("non-executable: got %+v", out)
	}
}

func TestRunTimesOut(t *testing.T) {
	t.Parallel()

	runner := newRunner(t, time.Second, 2)

	out := runner.Run(t.Context(), helperArgv(t, "sleep", "30")...)
	want := []string{"about to sleep", "Timed out after 1 seconds"}

	if out.Code != 124 || strings.Join(out.Lines, "|") != strings.Join(want, "|") {
		t.Errorf("timeout: got %+v, want lines %q", out, want)
	}

	if out.Duration > 10*time.Second {
		t.Errorf("timeout took %v, the process was not killed promptly", out.Duration)
	}
}
