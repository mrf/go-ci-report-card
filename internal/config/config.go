// Package config loads, defaults, overrides, and validates the report card
// configuration. Precedence is flag > config file > built-in default.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/mrf/go-ci-report-card/internal/glob"
)

const (
	// DefaultAccent is the accent colour used when project.accent is missing
	// or is not a six-digit hex code.
	DefaultAccent = "#e4572e"
	// MaxScore is the upper bound of every score.
	MaxScore = 100.0

	defaultCustomDescription = "Project-specific command completes successfully."

	// Built-in defaults, equal to the shipped examples/config.toml.
	defaultMinimumScore   = 80.0
	defaultDetailsLimit   = 80
	defaultTimeoutSeconds = 300
	defaultFormatWeight   = 15.0
	defaultVetWeight      = 20.0
	defaultBuildWeight    = 20.0
	defaultTestsWeight    = 30.0
	defaultCoverageTarget = 70.0
	defaultModulesWeight  = 15.0
)

var (
	accentPattern   = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	customIDPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)
	urlPattern      = regexp.MustCompile(`^https?://\S+$`)
)

// BuiltinIDs returns the built-in check ids in report order.
func BuiltinIDs() []string {
	return []string{"format", "vet", "build", "tests", "modules"}
}

// Error is a configuration error. The process should exit with status 2.
type Error struct {
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func errorf(format string, args ...any) error {
	return &Error{Message: fmt.Sprintf(format, args...)}
}

// Project is the [project] table.
type Project struct {
	Name          string   `toml:"name"`
	Tagline       string   `toml:"tagline"`
	RepositoryURL string   `toml:"repository_url"`
	SourceDir     string   `toml:"source_dir"`
	Accent        string   `toml:"accent"`
	Exclude       []string `toml:"exclude"`
}

// Quality is the [quality] table.
type Quality struct {
	MinimumScore          float64 `toml:"minimum_score"`
	DetailsLimit          int     `toml:"details_limit"`
	CommandTimeoutSeconds int     `toml:"command_timeout_seconds"`
}

// Check is a pass/fail built-in check table such as [checks.vet].
type Check struct {
	Enabled bool    `toml:"enabled"`
	Weight  float64 `toml:"weight"`
}

// TestsCheck is the [checks.tests] table.
type TestsCheck struct {
	Enabled        bool    `toml:"enabled"`
	Weight         float64 `toml:"weight"`
	CoverageTarget float64 `toml:"coverage_target"`
}

// Checks is the [checks] table.
type Checks struct {
	Format  Check      `toml:"format"`
	Vet     Check      `toml:"vet"`
	Build   Check      `toml:"build"`
	Tests   TestsCheck `toml:"tests"`
	Modules Check      `toml:"modules"`
}

// CustomCheck is one [[custom_checks]] entry.
type CustomCheck struct {
	ID          string   `toml:"id"`
	Label       string   `toml:"label"`
	Description string   `toml:"description"`
	Command     []string `toml:"command"`
	Weight      float64  `toml:"weight"`
	// Enabled is a pointer so an absent key defaults to true.
	Enabled *bool `toml:"enabled"`
}

// IsEnabled reports whether the check runs; an unset value means enabled.
func (c *CustomCheck) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

// Config is the whole configuration file.
type Config struct {
	Project      Project       `toml:"project"`
	Quality      Quality       `toml:"quality"`
	Checks       Checks        `toml:"checks"`
	CustomChecks []CustomCheck `toml:"custom_checks"`
}

// Overrides carries command-line values that beat both file and defaults.
// A nil field means "not set".
type Overrides struct {
	Name           *string
	SourceDir      *string
	MinimumScore   *float64
	CoverageTarget *float64
}

// BuiltinCheck is an enabled built-in check and its weight.
type BuiltinCheck struct {
	ID     string
	Weight float64
}

// Default returns the built-in configuration, equal to the shipped
// examples/config.toml.
func Default() Config {
	return Config{
		Project: Project{
			Tagline:   "A self-hosted quality snapshot for this Go project.",
			SourceDir: ".",
			Accent:    DefaultAccent,
			Exclude: []string{
				"vendor/**",
				"**/testdata/**",
				"*_generated.go",
				"**/*_generated.go",
				"*.pb.go",
				"**/*.pb.go",
			},
		},
		Quality: Quality{
			MinimumScore:          defaultMinimumScore,
			DetailsLimit:          defaultDetailsLimit,
			CommandTimeoutSeconds: defaultTimeoutSeconds,
		},
		Checks: Checks{
			Format:  Check{Enabled: true, Weight: defaultFormatWeight},
			Vet:     Check{Enabled: true, Weight: defaultVetWeight},
			Build:   Check{Enabled: true, Weight: defaultBuildWeight},
			Tests:   TestsCheck{Enabled: true, Weight: defaultTestsWeight, CoverageTarget: defaultCoverageTarget},
			Modules: Check{Enabled: true, Weight: defaultModulesWeight},
		},
	}
}

// Load applies the built-in defaults, then the TOML file at path (skipped
// when path is empty), then the overrides. The result is not yet validated;
// call Validate with the repository root.
func Load(path string, overrides Overrides) (*Config, error) {
	cfg := Default()
	if path != "" {
		if err := loadFile(path, &cfg); err != nil {
			return nil, err
		}
	}

	overrides.apply(&cfg)

	return &cfg, nil
}

func (o Overrides) apply(cfg *Config) {
	if o.Name != nil {
		cfg.Project.Name = *o.Name
	}

	if o.SourceDir != nil {
		cfg.Project.SourceDir = *o.SourceDir
	}

	if o.MinimumScore != nil {
		cfg.Quality.MinimumScore = *o.MinimumScore
	}

	if o.CoverageTarget != nil {
		cfg.Checks.Tests.CoverageTarget = *o.CoverageTarget
	}
}

func loadFile(path string, cfg *Config) error {
	body, err := os.ReadFile(path) //nolint:gosec // the path is the operator's -config flag; reading it is the point
	if errors.Is(err, os.ErrNotExist) {
		return errorf("Configuration file not found: %s", path)
	}

	if err != nil {
		return errorf("Unable to read configuration %s: %v", path, err)
	}

	var raw map[string]any
	if _, err := toml.Decode(string(body), &raw); err != nil {
		return errorf("Invalid TOML in %s: %v", path, err)
	}

	if err := checkShape(raw); err != nil {
		return err
	}

	if _, err := toml.Decode(string(body), cfg); err != nil {
		return errorf("Invalid configuration in %s: %v", path, err)
	}

	applyCustomDefaults(raw, cfg.CustomChecks)

	return nil
}

// checkShape reproduces the structural checks the Python generator made
// before touching any value, so its error messages survive.
func checkShape(raw map[string]any) error {
	for _, section := range []string{"project", "quality", "checks"} {
		if value, ok := raw[section]; ok && !isTable(value) {
			return errorf("[%s] must be a table.", section)
		}
	}

	if err := checkProjectShape(raw); err != nil {
		return err
	}

	if err := checkChecksShape(raw); err != nil {
		return err
	}

	if value, ok := raw["custom_checks"]; ok {
		return checkCustomShape(value)
	}

	return nil
}

func checkProjectShape(raw map[string]any) error {
	if project, ok := raw["project"].(map[string]any); ok {
		if exclude, present := project["exclude"]; present && !isStringArray(exclude) {
			return errorf("project.exclude must be an array of glob strings.")
		}
	}

	return nil
}

func checkChecksShape(raw map[string]any) error {
	if checks, ok := raw["checks"].(map[string]any); ok {
		for _, id := range BuiltinIDs() {
			if value, present := checks[id]; present && !isTable(value) {
				return errorf("[checks.%s] must be a table.", id)
			}
		}
	}

	return nil
}

// checkCustomShape accepts an array of tables (decoded as []map[string]any)
// or an inline array whose every element is a table.
func checkCustomShape(value any) error {
	if _, isTables := value.([]map[string]any); isTables {
		return nil
	}

	generic, isArray := value.([]any)
	if !isArray {
		return errorf("[[custom_checks]] entries must be an array of tables.")
	}

	for _, entry := range generic {
		if !isTable(entry) {
			return errorf("Every custom check must be a table.")
		}
	}

	return nil
}

func isTable(value any) bool {
	_, ok := value.(map[string]any)

	return ok
}

func isStringArray(value any) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}

	for _, item := range items {
		if _, isString := item.(string); !isString {
			return false
		}
	}

	return true
}

// applyCustomDefaults fills description only when the key is absent, matching
// the Python generator (a present-but-empty description stays empty). An
// empty label falls back to the id.
func applyCustomDefaults(raw map[string]any, checks []CustomCheck) {
	entries, ok := raw["custom_checks"].([]map[string]any)
	if !ok || len(entries) != len(checks) {
		return
	}

	for i, entry := range entries {
		if _, present := entry["description"]; !present {
			checks[i].Description = defaultCustomDescription
		}

		if checks[i].Label == "" {
			checks[i].Label = checks[i].ID
		}
	}
}

// SourcePath returns the absolute, cleaned project.source_dir under repoRoot.
func (c *Config) SourcePath(repoRoot string) string {
	if filepath.IsAbs(c.Project.SourceDir) {
		return filepath.Clean(c.Project.SourceDir)
	}

	return filepath.Join(absOrSelf(repoRoot), c.Project.SourceDir)
}

func absOrSelf(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}

	return filepath.Clean(path)
}

// EnabledChecks returns the enabled built-in checks in report order.
func (c *Config) EnabledChecks() []BuiltinCheck {
	all := []struct {
		id      string
		enabled bool
		weight  float64
	}{
		{"format", c.Checks.Format.Enabled, c.Checks.Format.Weight},
		{"vet", c.Checks.Vet.Enabled, c.Checks.Vet.Weight},
		{"build", c.Checks.Build.Enabled, c.Checks.Build.Weight},
		{"tests", c.Checks.Tests.Enabled, c.Checks.Tests.Weight},
		{"modules", c.Checks.Modules.Enabled, c.Checks.Modules.Weight},
	}

	enabled := make([]BuiltinCheck, 0, len(all))

	for _, check := range all {
		if check.enabled {
			enabled = append(enabled, BuiltinCheck{ID: check.id, Weight: check.weight})
		}
	}

	return enabled
}

// EnabledCustomChecks returns the enabled [[custom_checks]] entries in order.
func (c *Config) EnabledCustomChecks() []CustomCheck {
	enabled := make([]CustomCheck, 0, len(c.CustomChecks))

	for i := range c.CustomChecks {
		if c.CustomChecks[i].IsEnabled() {
			enabled = append(enabled, c.CustomChecks[i])
		}
	}

	return enabled
}

// Validate is ValidateEnv with the process environment.
func (c *Config) Validate(repoRoot string) error {
	return c.ValidateEnv(repoRoot, os.Getenv)
}

// ValidateEnv checks every rule the Python generator enforced and normalises
// derived values in place: the accent colour falls back to DefaultAccent,
// the repository URL is derived from GITHUB_REPOSITORY when blank, and the
// project name falls back to the repository slug or the source directory.
// getenv supplies environment variables, so tests can run in parallel.
func (c *Config) ValidateEnv(repoRoot string, getenv func(string) string) error {
	root := absOrSelf(repoRoot)

	source, err := c.validateSource(root)
	if err != nil {
		return err
	}

	if err := c.validateQuality(); err != nil {
		return err
	}

	if err := c.validateChecks(); err != nil {
		return err
	}

	c.resolveProject(source, getenv("GITHUB_REPOSITORY"))

	return c.validateRepositoryURL()
}

func (c *Config) validateSource(root string) (string, error) {
	source := c.SourcePath(root)

	info, err := os.Stat(source)
	if err != nil || !info.IsDir() {
		return "", errorf("project.source_dir is not a directory: %s", source)
	}

	rel, err := filepath.Rel(root, source)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errorf("project.source_dir must stay inside the repository.")
	}

	return source, nil
}

func (c *Config) validateQuality() error {
	if c.Quality.MinimumScore < 0 || c.Quality.MinimumScore > MaxScore {
		return errorf("quality.minimum_score must be between 0 and 100.")
	}

	if c.Quality.DetailsLimit < 1 || c.Quality.CommandTimeoutSeconds < 1 {
		return errorf("quality details_limit and command_timeout_seconds must be positive.")
	}

	for _, pattern := range c.Project.Exclude {
		if _, err := glob.Compile(pattern); err != nil {
			return errorf("project.exclude pattern %q is invalid: %v", pattern, err)
		}
	}

	return nil
}

// resolveProject fills the derived project fields from the GitHub repository
// slug and the source directory.
func (c *Config) resolveProject(source, slug string) {
	c.Project.RepositoryURL = strings.TrimSpace(c.Project.RepositoryURL)
	if c.Project.RepositoryURL == "" && slug != "" {
		c.Project.RepositoryURL = "https://github.com/" + slug
	}

	c.Project.Name = strings.TrimSpace(c.Project.Name)

	switch {
	case c.Project.Name != "":
	case slug != "":
		c.Project.Name = slug[strings.LastIndex(slug, "/")+1:]
	default:
		c.Project.Name = filepath.Base(source)
	}

	c.Project.Accent = ValidAccent(c.Project.Accent)
}

// ValidAccent returns value when it is a six-digit hex colour such as
// "#e4572e", else DefaultAccent. The site renderer applies the same rule to
// the report so a hand-edited report.json cannot inject CSS.
func ValidAccent(value string) string {
	if accentPattern.MatchString(value) {
		return value
	}

	return DefaultAccent
}

func (c *Config) validateRepositoryURL() error {
	if c.Project.RepositoryURL != "" && !urlPattern.MatchString(c.Project.RepositoryURL) {
		return errorf("project.repository_url must be an http:// or https:// URL.")
	}

	return nil
}

func (c *Config) validateChecks() error {
	ids := make(map[string]bool)

	for _, check := range c.EnabledChecks() {
		if check.Weight <= 0 {
			return errorf("checks.%s.weight must be greater than zero.", check.ID)
		}

		ids[check.ID] = true
	}

	total := len(ids)

	for i := range c.CustomChecks {
		if !c.CustomChecks[i].IsEnabled() {
			continue
		}

		if err := c.CustomChecks[i].validate(); err != nil {
			return err
		}

		ids[c.CustomChecks[i].ID] = true
		total++
	}

	if total == 0 {
		return errorf("At least one check must be enabled.")
	}

	if len(ids) != total {
		return errorf("Check ids must be unique.")
	}

	return nil
}

func (c *CustomCheck) validate() error {
	c.ID = strings.TrimSpace(c.ID)
	c.Description = strings.TrimSpace(c.Description)

	if !customIDPattern.MatchString(c.ID) {
		return errorf("Every custom check needs a lowercase id using letters, numbers, _ or -.")
	}

	// An absent label means "use the id"; a label of only whitespace is an
	// error, as in the Python generator.
	if c.Label == "" {
		c.Label = c.ID
	}

	c.Label = strings.TrimSpace(c.Label)
	if c.Label == "" {
		return errorf("Custom check %q needs a label.", c.ID)
	}

	if len(c.Command) == 0 {
		return errorf("Custom check %q command must be a non-empty string array.", c.ID)
	}

	if c.Weight <= 0 {
		return errorf("Custom check %q weight must be greater than zero.", c.ID)
	}

	return nil
}
