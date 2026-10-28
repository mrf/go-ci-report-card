package report_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrf/go-ci-report-card/internal/config"
	"github.com/mrf/go-ci-report-card/internal/report"
)

func TestGradeBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		score float64
		want  string
	}{
		{100, "A+"},
		{97, "A+"},
		{96.9, "A"},
		{93, "A"},
		{90, "A-"},
		{87, "B+"},
		{83, "B"},
		{80, "B-"},
		{79.9, "C+"},
		{77, "C+"},
		{73, "C"},
		{70, "C-"},
		{67, "D+"},
		{63, "D"},
		{60, "D-"},
		{59.9, "F"},
		{0, "F"},
	}
	for _, tc := range tests {
		if got := report.GradeFor(tc.score); got != tc.want {
			t.Errorf("GradeFor(%v) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestRoundMatchesPython(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value    float64
		decimals int
		want     float64
	}{
		{85, 1, 85},
		{66.6666, 1, 66.7},
		{12.25, 1, 12.2}, // exact tie rounds to even
		{12.35, 1, 12.3}, // 12.35 is below the tie in binary
		{0.125, 2, 0.12},
		{2.675, 2, 2.67},
		{0.005, 2, 0.01},
		{47.61904761904762, 1, 47.6},
	}
	for _, tc := range tests {
		if got := report.Round(tc.value, tc.decimals); got != tc.want {
			t.Errorf("Round(%v, %d) = %v, want %v", tc.value, tc.decimals, got, tc.want)
		}
	}
}

func TestFloatFormatsLikePython(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value float64
		want  string
	}{
		{100, "100.0"}, {0, "0.0"}, {85.5, "85.5"}, {0.01, "0.01"}, {15, "15.0"}, {66.7, "66.7"},
	}
	for _, tc := range tests {
		if got := report.Float(tc.value).String(); got != tc.want {
			t.Errorf("Float(%v).String() = %q, want %q", tc.value, got, tc.want)
		}

		data, err := json.Marshal(report.Float(tc.value))
		if err != nil || string(data) != tc.want {
			t.Errorf("json.Marshal(Float(%v)) = %q, %v; want %q", tc.value, data, err, tc.want)
		}
	}
}

func TestNewCheckBoundsAndDerivesStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		score      float64
		status     string
		wantScore  float64
		wantStatus string
	}{
		{150, "", 100, report.StatusPassed},
		{100, "", 100, report.StatusPassed},
		{99.96, "", 100, report.StatusPassed},
		{50.55, "", 50.5, report.StatusWarning},
		{0.04, "", 0, report.StatusFailed},
		{-5, "", 0, report.StatusFailed},
		{100, report.StatusWarning, 100, report.StatusWarning},
	}
	for _, tc := range tests {
		got := report.NewCheck("x", "X", "d", 10, tc.score, "obs", nil, 1.2345, tc.status)
		if float64(got.Score) != tc.wantScore || got.Status != tc.wantStatus {
			t.Errorf("NewCheck(score=%v, status=%q) = (%v, %q), want (%v, %q)",
				tc.score, tc.status, got.Score, got.Status, tc.wantScore, tc.wantStatus)
		}

		if got.Details == nil || len(got.Details) != 0 {
			t.Errorf("Details = %#v, want empty non-nil slice", got.Details)
		}

		if float64(got.Duration) != 1.23 {
			t.Errorf("Duration = %v, want 1.23", got.Duration)
		}
	}
}

func fiveChecks() []report.Check {
	rows := []struct {
		id     string
		weight float64
		score  float64
	}{
		{"format", 15, 100}, {"vet", 20, 100}, {"build", 20, 100}, {"tests", 30, 50}, {"modules", 15, 100},
	}

	checks := make([]report.Check, 0, len(rows))
	for _, row := range rows {
		checks = append(checks, report.NewCheck(row.id, strings.ToUpper(row.id[:1])+row.id[1:], "Description",
			row.weight, row.score, "Observed", []string{}, 0.01, ""))
	}

	return checks
}

func TestBuildWeightedReport(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Project.Name = "</script><b>unsafe</b>"
	cfg.Project.Tagline = "Local & static"

	repo := report.Repository{Slug: "o/r", URL: "https://github.com/o/r", Branch: "main", SHA: "abc", ShortSHA: "abc"}
	rep := report.Build(&cfg, fiveChecks(), repo, "2026-09-26T00:00:00Z")

	if rep.Score != 85 || rep.Grade != "B" || !rep.Passed {
		t.Fatalf("score %v grade %q passed %v, want 85.0 B true", rep.Score, rep.Grade, rep.Passed)
	}

	data, err := report.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}

	got := string(data)

	// Field order, Python float formatting, and no HTML escaping.
	wantPrefix := `{"schema_version":1,"project":{"name":"</script><b>unsafe</b>","tagline":"Local & static","accent":"#e4572e"},` +
		`"repository":{"slug":"o/r","url":"https://github.com/o/r","branch":"main","sha":"abc","short_sha":"abc"},` +
		`"generated_at":"2026-09-26T00:00:00Z","minimum_score":80.0,"score":85.0,"grade":"B","passed":true,"checks":[` +
		`{"id":"format","label":"Format","description":"Description","weight":15.0,"score":100.0,"observed":"Observed",` +
		`"status":"passed","details":[],"duration_seconds":0.01},`
	if !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("Marshal output:\n%s\nwant prefix:\n%s", got, wantPrefix)
	}

	if strings.HasSuffix(got, "\n") {
		t.Error("Marshal output ends with a newline")
	}

	var back report.Report
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("round trip: %v", err)
	}

	if back.Project.Name != cfg.Project.Name || len(back.Checks) != 5 {
		t.Errorf("round trip lost data: %+v", back)
	}
}

func TestWeightedScoreRounds(t *testing.T) {
	t.Parallel()

	checks := []report.Check{
		report.NewCheck("a", "A", "", 1, 100, "", nil, 0, ""),
		report.NewCheck("b", "B", "", 2, 0, "", nil, 0, ""),
	}
	if got := report.WeightedScore(checks); got != 33.3 {
		t.Errorf("WeightedScore = %v, want 33.3", got)
	}
}

func TestWriteJSONAddsNewline(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	rep := report.Build(&cfg, fiveChecks(), report.Repository{}, "now")
	path := filepath.Join(t.TempDir(), "site", "report.json")

	if err := report.WriteJSON(rep, path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	want, _ := report.Marshal(rep)
	if string(data) != string(want)+"\n" {
		t.Errorf("file = %q", data)
	}
}

func TestGeneratedAt(t *testing.T) {
	t.Parallel()

	fixed := func(string) string { return "2026-01-02T03:04:05Z" }
	if got := report.GeneratedAt(fixed, time.Now); got != "2026-01-02T03:04:05Z" {
		t.Errorf("REPORTCARD_NOW override ignored: %q", got)
	}

	none := func(string) string { return "" }
	micro := func() time.Time { return time.Date(2026, 9, 26, 13, 14, 15, 123456000, time.UTC) }
	whole := func() time.Time { return time.Date(2026, 9, 26, 13, 14, 15, 0, time.FixedZone("x", 3600)) }

	if got := report.GeneratedAt(none, micro); got != "2026-09-26T13:14:15.123456Z" {
		t.Errorf("GeneratedAt = %q", got)
	}

	if got := report.GeneratedAt(none, whole); got != "2026-09-26T12:14:15Z" {
		t.Errorf("GeneratedAt = %q", got)
	}
}

func TestRepositoryMetadata(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"GITHUB_REPOSITORY": "octo/cat",
		"GITHUB_SHA":        "0123456789abcdef",
		"GITHUB_REF_NAME":   "feature",
	}
	getenv := func(key string) string { return env[key] }

	got := report.RepositoryMetadata(t.Context(), t.TempDir(), "https://example.com/r", getenv)
	want := report.Repository{Slug: "octo/cat", URL: "https://example.com/r", Branch: "feature", SHA: "0123456789abcdef", ShortSHA: "01234567"}

	if got != want {
		t.Errorf("RepositoryMetadata = %+v, want %+v", got, want)
	}

	// Outside a git repository with no environment, the git fallbacks yield
	// empty strings rather than errors.
	empty := report.RepositoryMetadata(t.Context(), t.TempDir(), "", func(string) string { return "" })
	if empty != (report.Repository{}) {
		t.Errorf("RepositoryMetadata outside git = %+v, want zero value", empty)
	}
}

func TestWriteCIMetadata(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	rep := report.Build(&cfg, fiveChecks(), report.Repository{}, "now")
	dir := t.TempDir()
	output := filepath.Join(dir, "output")
	summary := filepath.Join(dir, "summary")

	if err := os.WriteFile(output, []byte("existing=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := report.WriteCIMetadata(rep, output, summary); err != nil {
		t.Fatal(err)
	}

	gotOutput, _ := os.ReadFile(output)
	if string(gotOutput) != "existing=1\npassed=true\nscore=85.0\ngrade=B\n" {
		t.Errorf("output file = %q", gotOutput)
	}

	gotSummary, _ := os.ReadFile(summary)

	wantSummary := "## Go CI Report Card: B (85.0)\n\n**Quality gate:** Passed (minimum 80.0)\n\n" +
		"| Check | Result | Score |\n|---|---:|---:|\n" +
		"| Format | Observed | 100.0 |\n| Vet | Observed | 100.0 |\n| Build | Observed | 100.0 |\n" +
		"| Tests | Observed | 50.0 |\n| Modules | Observed | 100.0 |\n"
	if string(gotSummary) != wantSummary {
		t.Errorf("summary file =\n%s\nwant:\n%s", gotSummary, wantSummary)
	}

	if err := report.WriteCIMetadata(rep, "", ""); err != nil {
		t.Errorf("empty paths should be skipped: %v", err)
	}

	rep.Passed = false
	if !strings.Contains(report.Summary(rep), "**Quality gate:** Needs attention (minimum 80.0)") {
		t.Errorf("failed summary = %q", report.Summary(rep))
	}
}
