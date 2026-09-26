package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrf/go-ci-report-card/internal/config"
)

// exampleConfig is the shipped starter config. Every value in it must equal
// the built-in default so a consumer with no config file gets the same report.
const exampleConfig = "../../examples/config.toml"

func noEnv(string) string { return "" }

// githubEnv mimics a GitHub Actions run for the octo/widgets repository.
func githubEnv(key string) string {
	if key == "GITHUB_REPOSITORY" {
		return "octo/widgets"
	}

	return ""
}

func writeConfig(t *testing.T, dir, body string) string {
	t.Helper()

	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return path
}

func mustLoad(t *testing.T, dir, body string) *config.Config {
	t.Helper()

	cfg, err := config.Load(writeConfig(t, dir, body), config.Overrides{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	return cfg
}

func assertConfigError(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("got nil error, want %q", want)
	}

	if _, ok := errors.AsType[*config.Error](err); !ok {
		t.Fatalf("error %v is %T, want *config.Error", err, err)
	}

	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err.Error(), want)
	}
}

func TestDefaultMatchesExampleConfig(t *testing.T) {
	t.Parallel()

	fromFile, err := config.Load(exampleConfig, config.Overrides{})
	if err != nil {
		t.Fatalf("Load(%s): %v", exampleConfig, err)
	}

	want := config.Default()
	if diff := describeDiff(want, *fromFile); diff != "" {
		t.Fatalf("Default() differs from %s: %s", exampleConfig, diff)
	}
}

func describeDiff(want, got config.Config) string {
	if want.Project.Name != got.Project.Name || want.Project.Tagline != got.Project.Tagline ||
		want.Project.RepositoryURL != got.Project.RepositoryURL ||
		want.Project.SourceDir != got.Project.SourceDir || want.Project.Accent != got.Project.Accent {
		return "project scalars differ"
	}

	if strings.Join(want.Project.Exclude, ",") != strings.Join(got.Project.Exclude, ",") {
		return "project.exclude differs"
	}

	if want.Quality != got.Quality {
		return "quality differs"
	}

	if want.Checks != got.Checks {
		return "checks differ"
	}

	if len(want.CustomChecks) != 0 || len(got.CustomChecks) != 0 {
		return "custom_checks differ"
	}

	return ""
}

func TestDefaultValues(t *testing.T) {
	t.Parallel()

	cfg := config.Default()

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"project.tagline", cfg.Project.Tagline, "A self-hosted quality snapshot for this Go project."},
		{"project.source_dir", cfg.Project.SourceDir, "."},
		{"project.accent", cfg.Project.Accent, "#e4572e"},
		{"quality.minimum_score", cfg.Quality.MinimumScore, 80.0},
		{"quality.details_limit", cfg.Quality.DetailsLimit, 80},
		{"quality.command_timeout_seconds", cfg.Quality.CommandTimeoutSeconds, 300},
		{"checks.format.weight", cfg.Checks.Format.Weight, 15.0},
		{"checks.vet.weight", cfg.Checks.Vet.Weight, 20.0},
		{"checks.build.weight", cfg.Checks.Build.Weight, 20.0},
		{"checks.tests.weight", cfg.Checks.Tests.Weight, 30.0},
		{"checks.tests.coverage_target", cfg.Checks.Tests.CoverageTarget, 70.0},
		{"checks.modules.weight", cfg.Checks.Modules.Weight, 15.0},
		{"checks.modules.enabled", cfg.Checks.Modules.Enabled, true},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}

	wantExclude := []string{
		"vendor/**", "**/testdata/**", "*_generated.go", "**/*_generated.go", "*.pb.go", "**/*.pb.go",
	}
	if strings.Join(cfg.Project.Exclude, ",") != strings.Join(wantExclude, ",") {
		t.Errorf("project.exclude = %v, want %v", cfg.Project.Exclude, wantExclude)
	}
}

func TestLoadWithoutPathUsesDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load("", config.Overrides{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if diff := describeDiff(config.Default(), *cfg); diff != "" {
		t.Fatalf("Load(\"\") differs from Default(): %s", diff)
	}
}

func TestLoadFileOverlaysDefaults(t *testing.T) {
	t.Parallel()

	cfg := mustLoad(t, t.TempDir(), `
[project]
name = "demo"
exclude = ["gen/**"]

[quality]
minimum_score = 90

[checks.tests]
coverage_target = 85

[checks.modules]
enabled = false

[[custom_checks]]
id = "lint"
command = ["golangci-lint", "run"]
weight = 10
`)
	if cfg.Project.Name != "demo" {
		t.Errorf("project.name = %q", cfg.Project.Name)
	}

	if cfg.Project.Tagline != config.Default().Project.Tagline {
		t.Errorf("project.tagline lost its default: %q", cfg.Project.Tagline)
	}

	if len(cfg.Project.Exclude) != 1 || cfg.Project.Exclude[0] != "gen/**" {
		t.Errorf("project.exclude = %v", cfg.Project.Exclude)
	}

	if cfg.Quality.MinimumScore != 90 || cfg.Quality.DetailsLimit != 80 {
		t.Errorf("quality = %+v", cfg.Quality)
	}

	if cfg.Checks.Tests.CoverageTarget != 85 || cfg.Checks.Tests.Weight != 30 || !cfg.Checks.Tests.Enabled {
		t.Errorf("checks.tests = %+v", cfg.Checks.Tests)
	}

	if cfg.Checks.Modules.Enabled || cfg.Checks.Modules.Weight != 15 {
		t.Errorf("checks.modules = %+v", cfg.Checks.Modules)
	}

	if len(cfg.CustomChecks) != 1 {
		t.Fatalf("custom_checks = %+v", cfg.CustomChecks)
	}

	custom := cfg.CustomChecks[0]
	if custom.ID != "lint" || custom.Weight != 10 || !custom.IsEnabled() {
		t.Errorf("custom check = %+v", custom)
	}

	if custom.Label != "lint" {
		t.Errorf("custom label = %q, want id as fallback", custom.Label)
	}

	if custom.Description != "Project-specific command completes successfully." {
		t.Errorf("custom description = %q", custom.Description)
	}
}

func TestLoadOverridesWinOverFile(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, t.TempDir(), `
[project]
name = "from-file"
source_dir = "pkg"

[quality]
minimum_score = 90

[checks.tests]
coverage_target = 85
`)
	name, source := "from-flag", "cmd"
	minScore, coverage := 55.5, 42.0

	cfg, err := config.Load(path, config.Overrides{
		Name:           &name,
		SourceDir:      &source,
		MinimumScore:   &minScore,
		CoverageTarget: &coverage,
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Project.Name != name || cfg.Project.SourceDir != source {
		t.Errorf("project = %+v", cfg.Project)
	}

	if cfg.Quality.MinimumScore != minScore {
		t.Errorf("minimum_score = %v", cfg.Quality.MinimumScore)
	}

	if cfg.Checks.Tests.CoverageTarget != coverage {
		t.Errorf("coverage_target = %v", cfg.Checks.Tests.CoverageTarget)
	}
}

func TestLoadErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{"bad toml", "[project\n", "Invalid TOML in"},
		{"project not table", "project = 1\n", "[project] must be a table."},
		{"quality not table", "quality = \"x\"\n", "[quality] must be a table."},
		{"checks not table", "checks = []\n", "[checks] must be a table."},
		{"check not table", "[checks]\nvet = 3\n", "[checks.vet] must be a table."},
		{"custom not array", "custom_checks = 1\n", "[[custom_checks]] entries must be an array of tables."},
		{"custom entry not table", "custom_checks = [1]\n", "Every custom check must be a table."},
		{"wrong type", "[quality]\nminimum_score = \"high\"\n", "Invalid configuration in"},
		{"exclude not strings", "[project]\nexclude = [1]\n", "project.exclude must be an array of glob strings."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := config.Load(writeConfig(t, t.TempDir(), tc.body), config.Overrides{})
			assertConfigError(t, err, tc.want)
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "nope.toml")
	_, err := config.Load(missing, config.Overrides{})
	assertConfigError(t, err, "Configuration file not found: "+missing)
}

func TestValidateDefaultsAgainstRepo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	cfg := config.Default()
	if err := cfg.ValidateEnv(root, noEnv); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if cfg.Project.Name != filepath.Base(root) {
		t.Errorf("name = %q, want source dir basename %q", cfg.Project.Name, filepath.Base(root))
	}

	if cfg.Project.RepositoryURL != "" {
		t.Errorf("repository_url = %q, want empty", cfg.Project.RepositoryURL)
	}

	if got := cfg.SourcePath(root); got != root {
		t.Errorf("SourcePath = %q, want %q", got, root)
	}

	enabled := cfg.EnabledChecks()
	if len(enabled) != 5 {
		t.Fatalf("EnabledChecks = %+v, want 5", enabled)
	}

	wantOrder := []string{"format", "vet", "build", "tests", "modules"}
	for i, check := range enabled {
		if check.ID != wantOrder[i] {
			t.Errorf("EnabledChecks[%d].ID = %q, want %q", i, check.ID, wantOrder[i])
		}
	}
}

func TestValidateDerivesFromGitHubEnv(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	cfg := config.Default()
	if err := cfg.ValidateEnv(root, githubEnv); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if cfg.Project.Name != "widgets" {
		t.Errorf("name = %q, want slug basename", cfg.Project.Name)
	}

	if cfg.Project.RepositoryURL != "https://github.com/octo/widgets" {
		t.Errorf("repository_url = %q", cfg.Project.RepositoryURL)
	}
}

func TestValidateNormalizesAccentAndName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	cfg := mustLoad(t, root, "[project]\nname = \"  Spaced  \"\naccent = \"red\"\n")
	if err := cfg.ValidateEnv(root, githubEnv); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if cfg.Project.Name != "Spaced" {
		t.Errorf("name = %q, want trimmed", cfg.Project.Name)
	}

	if cfg.Project.Accent != "#e4572e" {
		t.Errorf("accent = %q, want default for invalid value", cfg.Project.Accent)
	}

	cfg = mustLoad(t, root, "[project]\naccent = \"#ABCDEF\"\n")
	if err := cfg.ValidateEnv(root, noEnv); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if cfg.Project.Accent != "#ABCDEF" {
		t.Errorf("accent = %q, want valid value kept", cfg.Project.Accent)
	}
}

func TestValidateSourceDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "pkg"), 0o750); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()

	cfg.Project.SourceDir = "pkg"
	if err := cfg.ValidateEnv(root, noEnv); err != nil {
		t.Fatalf("Validate(pkg): %v", err)
	}

	if got, want := cfg.SourcePath(root), filepath.Join(root, "pkg"); got != want {
		t.Errorf("SourcePath = %q, want %q", got, want)
	}

	if cfg.Project.Name != "pkg" {
		t.Errorf("name = %q, want source basename", cfg.Project.Name)
	}

	cfg = config.Default()
	cfg.Project.SourceDir = "missing"
	assertConfigError(t, cfg.ValidateEnv(root, noEnv), "project.source_dir is not a directory: "+filepath.Join(root, "missing"))

	cfg = config.Default()
	cfg.Project.SourceDir = ".."
	assertConfigError(t, cfg.ValidateEnv(root, noEnv), "project.source_dir must stay inside the repository.")
}

func TestValidateErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*config.Config)
		want   string
	}{
		{"score too high", func(c *config.Config) { c.Quality.MinimumScore = 100.5 }, "quality.minimum_score must be between 0 and 100."},
		{"score negative", func(c *config.Config) { c.Quality.MinimumScore = -1 }, "quality.minimum_score must be between 0 and 100."},
		{"details limit", func(c *config.Config) { c.Quality.DetailsLimit = 0 }, "quality details_limit and command_timeout_seconds must be positive."},
		{"timeout", func(c *config.Config) { c.Quality.CommandTimeoutSeconds = 0 }, "quality details_limit and command_timeout_seconds must be positive."},
		{"exclude pattern", func(c *config.Config) { c.Project.Exclude = []string{`a\b`} }, "project.exclude pattern"},
		{"weight zero", func(c *config.Config) { c.Checks.Vet.Weight = 0 }, "checks.vet.weight must be greater than zero."},
		{"weight negative", func(c *config.Config) { c.Checks.Tests.Weight = -3 }, "checks.tests.weight must be greater than zero."},
		{"bad url", func(c *config.Config) { c.Project.RepositoryURL = "git@github.com:a/b" }, "project.repository_url must be an http:// or https:// URL."},
		{"url with space", func(c *config.Config) { c.Project.RepositoryURL = "https://x y" }, "project.repository_url must be an http:// or https:// URL."},
		{"custom bad id", func(c *config.Config) {
			c.CustomChecks = []config.CustomCheck{{ID: "Bad ID", Command: []string{"x"}, Weight: 1}}
		}, "Every custom check needs a lowercase id using letters, numbers, _ or -."},
		{"custom empty id", func(c *config.Config) { c.CustomChecks = []config.CustomCheck{{Command: []string{"x"}, Weight: 1}} }, "Every custom check needs a lowercase id using letters, numbers, _ or -."},
		{"custom blank label", func(c *config.Config) {
			c.CustomChecks = []config.CustomCheck{{ID: "x", Label: "   ", Command: []string{"x"}, Weight: 1}}
		}, "Custom check \"x\" needs a label."},
		{"custom no command", func(c *config.Config) { c.CustomChecks = []config.CustomCheck{{ID: "x", Weight: 1}} }, "Custom check \"x\" command must be a non-empty string array."},
		{"custom weight", func(c *config.Config) { c.CustomChecks = []config.CustomCheck{{ID: "x", Command: []string{"x"}}} }, "Custom check \"x\" weight must be greater than zero."},
		{"duplicate id", func(c *config.Config) {
			c.CustomChecks = []config.CustomCheck{{ID: "vet", Command: []string{"x"}, Weight: 1}}
		}, "Check ids must be unique."},
		{"nothing enabled", func(c *config.Config) {
			c.Checks.Format.Enabled = false
			c.Checks.Vet.Enabled = false
			c.Checks.Build.Enabled = false
			c.Checks.Tests.Enabled = false
			c.Checks.Modules.Enabled = false
		}, "At least one check must be enabled."},
	}

	root := t.TempDir()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.Default()
			tc.mutate(&cfg)
			assertConfigError(t, cfg.ValidateEnv(root, noEnv), tc.want)
		})
	}
}

func TestValidateSkipsDisabledChecks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	disabled := false
	cfg := config.Default()
	cfg.Checks.Vet.Enabled = false
	cfg.Checks.Vet.Weight = 0

	cfg.CustomChecks = []config.CustomCheck{
		{ID: "broken", Enabled: &disabled},
		{ID: "ok", Command: []string{"true"}, Weight: 5},
	}
	if err := cfg.ValidateEnv(root, noEnv); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	ids := make([]string, 0, 5)
	for _, check := range cfg.EnabledChecks() {
		ids = append(ids, check.ID)
	}

	if got := strings.Join(ids, ","); got != "format,build,tests,modules" {
		t.Errorf("EnabledChecks = %s", got)
	}

	customs := cfg.EnabledCustomChecks()
	if len(customs) != 1 || customs[0].ID != "ok" {
		t.Errorf("EnabledCustomChecks = %+v", customs)
	}
}
