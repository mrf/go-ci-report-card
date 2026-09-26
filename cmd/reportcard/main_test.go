package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrf/go-ci-report-card/internal/report"
)

const (
	fixtureDir = "../../testdata/fixture"
	goldenPath = "../../testdata/golden/report.json"
)

func noEnv(string) string { return "" }

// goldenEnv is the environment the golden report was generated under.
func goldenEnv(stepSummary string) func(string) string {
	env := map[string]string{
		"REPORTCARD_NOW":      "2026-09-26T12:00:00Z",
		"GITHUB_REPOSITORY":   "mrf/fixture",
		"GITHUB_SHA":          "0123456789abcdef0123456789abcdef01234567",
		"GITHUB_REF_NAME":     "main",
		"GITHUB_STEP_SUMMARY": stepSummary,
	}

	return func(key string) string { return env[key] }
}

// TestGoldenReport runs the whole pipeline against testdata/fixture and
// compares report.json with the checked-in golden file. Durations are the
// only machine-dependent field, so they are zeroed before comparing; the
// golden file carries 0.0 for each.
func TestGoldenReport(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	output := filepath.Join(dir, "site")
	githubOutput := filepath.Join(dir, "github-output")
	stepSummary := filepath.Join(dir, "step-summary")

	var stdout, stderr bytes.Buffer

	args := []string{"-repo-root", fixtureDir, "-config", "reportcard.toml", "-output", output, "-github-output", githubOutput}
	if code := run(t.Context(), args, &stdout, &stderr, goldenEnv(stepSummary)); code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}

	if !strings.HasPrefix(stdout.String(), "PASS  Grade B-  Score 81.2/100\nStatic report written to "+output+"\n") {
		t.Errorf("stdout = %q", stdout.String())
	}

	actual, err := os.ReadFile(filepath.Join(output, "report.json"))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.HasSuffix(actual, []byte("}\n")) {
		t.Errorf("report.json should be one compact JSON line plus newline; ends %q", actual[len(actual)-5:])
	}

	var rep report.Report
	if err := json.Unmarshal(actual, &rep); err != nil {
		t.Fatalf("report.json: %v", err)
	}

	for i := range rep.Checks {
		if rep.Checks[i].Duration < 0 {
			t.Errorf("check %s has negative duration", rep.Checks[i].ID)
		}

		rep.Checks[i].Duration = 0
	}

	normalised, err := report.Marshal(&rep)
	if err != nil {
		t.Fatal(err)
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := string(normalised)+"\n", string(golden); got != want {
		t.Errorf("report.json differs from golden.\n got: %s\nwant: %s", got, want)
	}

	gotOutput, _ := os.ReadFile(githubOutput)
	if string(gotOutput) != "passed=true\nscore=81.2\ngrade=B-\n" {
		t.Errorf("github output = %q", gotOutput)
	}

	gotSummary, _ := os.ReadFile(stepSummary)
	if !strings.HasPrefix(string(gotSummary), "## Go CI Report Card: B- (81.2)\n\n**Quality gate:** Passed (minimum 80.0)\n\n| Check | Result | Score |\n") ||
		!strings.Contains(string(gotSummary), "| Formatting | 2 / 3 files formatted | 66.7 |\n") {
		t.Errorf("step summary = %q", gotSummary)
	}
}

func TestEnforceFailsTheGate(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	output := t.TempDir()
	args := []string{"-repo-root", fixtureDir, "-config", "reportcard.toml", "-output", output, "-min-score", "90"}
	enforced := []string{"-repo-root", fixtureDir, "-config", "reportcard.toml", "-output", output, "-min-score", "90", "-enforce"}

	if code := run(t.Context(), args, &stdout, &stderr, goldenEnv("")); code != exitOK {
		t.Fatalf("without -enforce: exit %d, stderr: %s", code, stderr.String())
	}

	if !strings.HasPrefix(stdout.String(), "FAIL  Grade B-  Score 81.2/100\n") {
		t.Errorf("stdout = %q", stdout.String())
	}

	stdout.Reset()

	if code := run(t.Context(), enforced, &stdout, &stderr, goldenEnv("")); code != exitGateFailed {
		t.Fatalf("with -enforce: exit %d, want %d; stderr: %s", code, exitGateFailed, stderr.String())
	}
}

func TestRunReportsConfigurationErrors(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	code := run(t.Context(), []string{"-repo-root", t.TempDir(), "-min-score", "150"}, &stdout, &stderr, noEnv)
	if code != exitConfiguration {
		t.Fatalf("exit %d, want %d", code, exitConfiguration)
	}

	if !strings.Contains(stderr.String(), "configuration error: quality.minimum_score must be between 0 and 100.") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestRunMissingConfigFile(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	code := run(t.Context(), []string{"-repo-root", t.TempDir(), "-config", "missing.toml"}, &stdout, &stderr, noEnv)
	if code != exitConfiguration || !strings.Contains(stderr.String(), "Configuration file not found") {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
}

func TestRunBadFlag(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	if code := run(t.Context(), []string{"-bogus"}, &stdout, &stderr, noEnv); code != exitConfiguration {
		t.Fatalf("exit %d, want %d", code, exitConfiguration)
	}

	if code := run(t.Context(), []string{"-h"}, &stdout, &stderr, noEnv); code != exitOK {
		t.Fatalf("-h exit %d, want %d", code, exitOK)
	}
}

func TestRunUnwritableOutput(t *testing.T) {
	t.Parallel()

	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer

	args := []string{"-repo-root", fixtureDir, "-config", "reportcard.toml", "-output", filepath.Join(blocker, "site")}
	if code := run(t.Context(), args, &stdout, &stderr, goldenEnv("")); code != exitConfiguration {
		t.Fatalf("exit %d, want %d", code, exitConfiguration)
	}

	if !strings.HasPrefix(stderr.String(), "generation error: ") {
		t.Errorf("stderr = %q", stderr.String())
	}
}
