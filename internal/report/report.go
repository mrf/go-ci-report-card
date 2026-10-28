// Package report assembles check results into the scored, graded report and
// serialises it as schema_version 1 JSON, byte-for-byte as the Python
// generator did.
package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mrf/go-ci-report-card/internal/config"
)

const (
	// SchemaVersion is the report.json schema this package writes.
	SchemaVersion = 1
	// MaxScore is the upper bound of every score.
	MaxScore = config.MaxScore

	// StatusPassed marks a check that fully met its bar.
	StatusPassed = "passed"
	// StatusWarning marks a check that scored above zero but below the bar.
	StatusWarning = "warning"
	// StatusFailed marks a check that scored zero.
	StatusFailed = "failed"

	scoreDecimals    = 1
	durationDecimals = 2
	shortSHALength   = 8
	filePerm         = 0o600
	dirPerm          = 0o750
)

// Float is a float64 that marshals the way Python's json module prints a
// float: an integral value keeps a trailing ".0", so 100 becomes "100.0".
type Float float64

// MarshalJSON implements json.Marshaler.
func (f Float) MarshalJSON() ([]byte, error) {
	return []byte(f.String()), nil
}

// String formats the value as Python's str(float) does for ordinary
// magnitudes (Python switches to exponent form outside 1e-4..1e16; report
// values never get there).
func (f Float) String() string {
	s := strconv.FormatFloat(float64(f), 'f', -1, 64)
	if !strings.ContainsAny(s, ".eEIN") {
		s += ".0"
	}

	return s
}

// Check is one row of the report, in the Python generator's field order.
type Check struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Weight      Float    `json:"weight"`
	Score       Float    `json:"score"`
	Observed    string   `json:"observed"`
	Status      string   `json:"status"`
	Details     []string `json:"details"`
	Duration    Float    `json:"duration_seconds"`
}

// Project is the report's project block.
type Project struct {
	Name    string `json:"name"`
	Tagline string `json:"tagline"`
	Accent  string `json:"accent"`
}

// Repository identifies the commit the report describes.
type Repository struct {
	Slug     string `json:"slug"`
	URL      string `json:"url"`
	Branch   string `json:"branch"`
	SHA      string `json:"sha"`
	ShortSHA string `json:"short_sha"`
}

// Report is the schema_version 1 document.
type Report struct {
	SchemaVersion int        `json:"schema_version"`
	Project       Project    `json:"project"`
	Repository    Repository `json:"repository"`
	GeneratedAt   string     `json:"generated_at"`
	MinimumScore  Float      `json:"minimum_score"`
	Score         Float      `json:"score"`
	Grade         string     `json:"grade"`
	Passed        bool       `json:"passed"`
	Checks        []Check    `json:"checks"`
}

// gradeStep is one threshold of the grade scale.
type gradeStep struct {
	threshold float64
	grade     string
}

// gradeScale is the Python GRADE_SCALE, verbatim.
//
//nolint:gochecknoglobals // immutable lookup table
var gradeScale = []gradeStep{
	{97, "A+"},
	{93, "A"},
	{90, "A-"},
	{87, "B+"},
	{83, "B"},
	{80, "B-"},
	{77, "C+"},
	{73, "C"},
	{70, "C-"},
	{67, "D+"},
	{63, "D"},
	{60, "D-"},
	{0, "F"},
}

// GradeFor returns the letter grade for a 0-100 score.
func GradeFor(score float64) string {
	for _, step := range gradeScale {
		if score >= step.threshold {
			return step.grade
		}
	}

	return "F"
}

// Round rounds to decimals places exactly as Python's round(x, n): the exact
// binary value is rounded, with ties going to even. Formatting and parsing
// back is the only stdlib route that matches (multiplying first introduces
// its own rounding).
func Round(value float64, decimals int) float64 {
	rounded, err := strconv.ParseFloat(strconv.FormatFloat(value, 'f', decimals, 64), 64)
	if err != nil {
		// FormatFloat only ever produces parseable output.
		return value
	}

	return rounded
}

// NewCheck builds one row, bounding the score to 0-100 and deriving the
// status when it is empty: passed at 100, warning above 0, failed at 0.
func NewCheck(
	checkID, label, description string,
	weight, score float64,
	observed string,
	details []string,
	duration float64,
	status string,
) Check {
	bounded := Round(max(0, min(MaxScore, score)), scoreDecimals)

	if status == "" {
		switch {
		case bounded >= MaxScore:
			status = StatusPassed
		case bounded > 0:
			status = StatusWarning
		default:
			status = StatusFailed
		}
	}

	if details == nil {
		details = []string{}
	}

	return Check{
		ID:          checkID,
		Label:       label,
		Description: description,
		Weight:      Float(weight),
		Score:       Float(bounded),
		Observed:    observed,
		Status:      status,
		Details:     details,
		Duration:    Float(Round(duration, durationDecimals)),
	}
}

// WeightedScore returns the weighted mean of the check scores rounded to one
// decimal. It panics on an empty slice; validation guarantees at least one
// enabled check.
func WeightedScore(checks []Check) float64 {
	var total, weighted float64

	for _, check := range checks {
		total += float64(check.Weight)
		weighted += float64(check.Score) * float64(check.Weight)
	}

	return Round(weighted/total, scoreDecimals)
}

// Build assembles the report from validated configuration, completed checks,
// repository metadata, and the generation timestamp.
func Build(cfg *config.Config, checks []Check, repo Repository, generatedAt string) *Report {
	score := WeightedScore(checks)

	return &Report{
		SchemaVersion: SchemaVersion,
		Project: Project{
			Name:    cfg.Project.Name,
			Tagline: cfg.Project.Tagline,
			Accent:  cfg.Project.Accent,
		},
		Repository:   repo,
		GeneratedAt:  generatedAt,
		MinimumScore: Float(cfg.Quality.MinimumScore),
		Score:        Float(score),
		Grade:        GradeFor(score),
		Passed:       score >= cfg.Quality.MinimumScore,
		Checks:       checks,
	}
}

// Marshal encodes the report as compact JSON with no trailing newline and no
// HTML escaping, matching json.dumps(ensure_ascii=False, separators=(",", ":")).
func Marshal(r *Report) ([]byte, error) {
	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(r); err != nil {
		return nil, fmt.Errorf("encode report: %w", err)
	}

	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// WriteJSON writes the compact report plus a trailing newline to path,
// creating parent directories.
func WriteJSON(r *Report, path string) error {
	data, err := Marshal(r)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), filePerm); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	return nil
}
