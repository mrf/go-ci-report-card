package checks_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrf/go-ci-report-card/internal/checks"
	"github.com/mrf/go-ci-report-card/internal/config"
	"github.com/mrf/go-ci-report-card/internal/report"
)

const fixtureDir = "../../testdata/fixture"

// fixtureRunner targets the golden-test fixture module with generous limits.
func fixtureRunner() *checks.Runner {
	return &checks.Runner{Dir: fixtureDir, Timeout: 5 * time.Minute, DetailsLimit: 10}
}

func TestParseCoverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  float64
		ok    bool
	}{
		{"typical", []string{"example.com/x/a.go:3:\tAdd\t100.0%", "total:\t(statements)\t33.3%"}, 33.3, true},
		{"whole", []string{"total:   (statements)   100.0%"}, 100, true},
		{"missing", []string{"example.com/x/a.go:3:\tAdd\t100.0%"}, 0, false},
		{"malformed", []string{"total: something else"}, 0, false},
		{"empty", []string{}, 0, false},
	}
	for _, tc := range tests {
		got, ok := checks.ParseCoverage(tc.lines)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s: ParseCoverage = (%v, %v), want (%v, %v)", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()

	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFormat(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"a.go":                    "package a\n",
		"b/ugly.go":               "package   b\n",
		"b/x_generated.go":        "package   b\n",
		"vendor/v/v.go":           "package   v\n",
		"testdata/skip.go":        "package   skip\n",
		"c/deep/testdata/skip.go": "package   skip\n",
		"notes.txt":               "not go\n",
	})

	runner := &checks.Runner{Dir: root, Timeout: 30 * time.Second, DetailsLimit: 10}
	got := checks.Format(t.Context(), runner, 15, config.Default().Project.Exclude)

	if got.ID != "format" || got.Label != "Formatting" || got.Weight != 15 {
		t.Errorf("identity fields wrong: %+v", got)
	}

	if got.Score != 50 || got.Observed != "1 / 2 files formatted" || got.Status != report.StatusWarning {
		t.Errorf("score/observed/status = %v %q %q", got.Score, got.Observed, got.Status)
	}

	if strings.Join(got.Details, "|") != "b/ugly.go" {
		t.Errorf("details = %q", got.Details)
	}
}

func TestFormatNoFiles(t *testing.T) {
	t.Parallel()

	runner := &checks.Runner{Dir: t.TempDir(), Timeout: 30 * time.Second, DetailsLimit: 10}
	got := checks.Format(t.Context(), runner, 15, nil)

	if got.Score != 0 || got.Observed != "No Go files" || got.Status != report.StatusFailed || got.Duration != 0 {
		t.Errorf("no files: %+v", got)
	}

	if strings.Join(got.Details, "|") != "No Go files matched the configured source directory." {
		t.Errorf("details = %q", got.Details)
	}
}

func TestTestsAgainstFixture(t *testing.T) {
	t.Parallel()

	runner := fixtureRunner()
	got := checks.Tests(t.Context(), runner, 30, 70)

	if got.ID != "tests" || got.Label != "Tests & coverage" {
		t.Errorf("identity fields wrong: %+v", got)
	}

	// The fixture covers one of three statements.
	if got.Observed != "33.3% / 70% target" || got.Score != 47.6 || got.Status != report.StatusWarning {
		t.Errorf("observed/score/status = %q %v %q", got.Observed, got.Score, got.Status)
	}

	if strings.Join(got.Details, "|") != "Coverage is 36.7 points below target." {
		t.Errorf("details = %q", got.Details)
	}

	if got.Duration <= 0 {
		t.Errorf("duration = %v", got.Duration)
	}
}

func TestTestsFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":    "module example.com/failing\n\ngo 1.26\n",
		"f.go":      "package failing\n\nfunc F() int { return 1 }\n",
		"f_test.go": "package failing\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) { t.Fatal(\"boom\") }\n",
	})

	runner := &checks.Runner{Dir: root, Timeout: 5 * time.Minute, DetailsLimit: 10}
	got := checks.Tests(t.Context(), runner, 30, 70)

	if got.Score != 0 || got.Observed != "Tests failed" || got.Status != report.StatusFailed {
		t.Errorf("failing suite: %+v", got)
	}

	if len(got.Details) == 0 || !strings.Contains(strings.Join(got.Details, "\n"), "boom") {
		t.Errorf("details should carry the test output: %q", got.Details)
	}
}

func TestCommandAndCustom(t *testing.T) {
	t.Parallel()

	runner := fixtureRunner()

	got := checks.Command(t.Context(), runner, "vet", 20)
	if got.ID != "vet" || got.Label != "Go vet" || got.Score != 100 || got.Observed != "Clean" || len(got.Details) != 0 {
		t.Errorf("vet: %+v", got)
	}

	custom := config.CustomCheck{ID: "nope", Label: "Nope", Description: "Never works.", Command: []string{"go", "nonsense-subcommand"}, Weight: 5}

	got = checks.Custom(t.Context(), runner, &custom)
	if got.ID != "nope" || got.Label != "Nope" || got.Description != "Never works." || got.Weight != 5 {
		t.Errorf("custom identity: %+v", got)
	}

	if got.Score != 0 || got.Observed != "Exited 2" || got.Status != report.StatusFailed || len(got.Details) == 0 {
		t.Errorf("custom failure: %+v", got)
	}
}

func TestRunAllOrdersBuiltinsThenCustom(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Checks.Vet.Enabled = false
	cfg.CustomChecks = []config.CustomCheck{{ID: "ver", Label: "Version", Command: []string{"go", "version"}, Weight: 1}}

	abs, err := filepath.Abs(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}

	results := checks.RunAll(t.Context(), &cfg, abs)

	ids := make([]string, 0, len(results))
	for _, result := range results {
		ids = append(ids, result.ID)
	}

	if strings.Join(ids, ",") != "format,build,tests,modules,ver" {
		t.Errorf("ids = %v", ids)
	}
}
