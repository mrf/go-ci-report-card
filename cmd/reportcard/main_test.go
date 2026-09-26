package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func noEnv(string) string { return "" }

func TestRunPrintsResolvedConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	cfgPath := filepath.Join(root, "rc.toml")
	if err := os.WriteFile(cfgPath, []byte("[quality]\nminimum_score = 90\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer

	code := run([]string{"-repo-root", root, "-config", "rc.toml", "-name", "demo", "-min-score", "0"}, &stdout, &stderr, noEnv)
	if code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{`name = "demo"`, "minimum_score = 0.0", "coverage_target = 70.0", "# enforce = false"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunReportsConfigurationErrors(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	code := run([]string{"-repo-root", t.TempDir(), "-min-score", "150"}, &stdout, &stderr, noEnv)
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

	code := run([]string{"-repo-root", t.TempDir(), "-config", "missing.toml"}, &stdout, &stderr, noEnv)
	if code != exitConfiguration || !strings.Contains(stderr.String(), "Configuration file not found") {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
}

func TestRunBadFlag(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	if code := run([]string{"-bogus"}, &stdout, &stderr, noEnv); code != exitConfiguration {
		t.Fatalf("exit %d, want %d", code, exitConfiguration)
	}

	if code := run([]string{"-h"}, &stdout, &stderr, noEnv); code != exitOK {
		t.Fatalf("-h exit %d, want %d", code, exitOK)
	}
}
