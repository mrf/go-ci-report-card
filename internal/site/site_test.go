package site_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrf/go-ci-report-card/internal/report"
	"github.com/mrf/go-ci-report-card/internal/site"
)

const goldenPath = "../../testdata/golden/report.json"

// loadGolden decodes the golden report.
func loadGolden(t *testing.T) *report.Report {
	t.Helper()

	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}

	var rep report.Report
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatal(err)
	}

	return &rep
}

// render writes rep to a fresh directory and returns it with index.html.
func render(t *testing.T, rep *report.Report) (string, string) {
	t.Helper()

	output := filepath.Join(t.TempDir(), "_site")
	if err := site.Write(rep, output); err != nil {
		t.Fatalf("Write: %v", err)
	}

	page, err := os.ReadFile(filepath.Join(output, "index.html"))
	if err != nil {
		t.Fatal(err)
	}

	return output, string(page)
}

// embeddedJSON extracts the report JSON from the page's data script.
func embeddedJSON(t *testing.T, page string) string {
	t.Helper()

	const open = `<script id="report-data" type="application/json">`

	_, rest, found := strings.Cut(page, open)
	if !found {
		t.Fatalf("page lacks the report-data script: %s", page)
	}

	embedded, _, found := strings.Cut(rest, "</script>")
	if !found {
		t.Fatalf("report-data script is unterminated: %s", rest)
	}

	return embedded
}

func TestWriteGolden(t *testing.T) {
	t.Parallel()

	rep := loadGolden(t)
	output, page := render(t, rep)

	for _, want := range []string{
		"<title>fixture · Go CI Report Card</title>",
		`<meta name="description" content="Golden-test fixture.">`,
		`<meta name="theme-color" content="#e4572e">`,
		`<body style="--accent: #e4572e">`,
		`<h1 id="project-name">fixture</h1>`,
		`<p class="tagline" id="tagline">Golden-test fixture.</p>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("index.html lacks %q", want)
		}
	}

	if strings.Contains(page, "__") {
		t.Errorf("index.html still has a placeholder: %s", page)
	}

	compact, err := report.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}

	if got := embeddedJSON(t, page); got != string(compact) {
		t.Errorf("embedded JSON\n got: %s\nwant: %s", got, compact)
	}

	reportJSON, err := os.ReadFile(filepath.Join(output, "report.json"))
	if err != nil {
		t.Fatal(err)
	}

	if string(reportJSON) != string(compact)+"\n" {
		t.Errorf("report.json = %q", reportJSON)
	}

	nojekyll, err := os.ReadFile(filepath.Join(output, ".nojekyll"))
	if err != nil {
		t.Fatal(err)
	}

	if len(nojekyll) != 0 {
		t.Errorf(".nojekyll should be empty, got %q", nojekyll)
	}

	for _, name := range []string{"style.css", "app.js"} {
		got, err := os.ReadFile(filepath.Join(output, "assets", name))
		if err != nil {
			t.Fatal(err)
		}

		want, err := fs.ReadFile(site.Assets, "assets/"+name)
		if err != nil {
			t.Fatal(err)
		}

		if string(got) != string(want) {
			t.Errorf("assets/%s differs from the embedded copy", name)
		}
	}
}

func TestWriteEscapesHostileTagline(t *testing.T) {
	t.Parallel()

	rep := loadGolden(t)
	rep.Project.Name = `Wid<get> & "Co"`
	rep.Project.Tagline = `</script><img src=x onerror="alert(1)"> "quoted"`

	_, page := render(t, rep)

	if strings.Contains(page, `</script><img`) {
		t.Errorf("tagline broke out of its context: %s", page)
	}

	if strings.Contains(page, `onerror="alert`) {
		t.Errorf("tagline attribute survived unescaped: %s", page)
	}

	if !strings.Contains(page, `<title>Wid&lt;get&gt; &amp; &#34;Co&#34; · Go CI Report Card</title>`) {
		t.Errorf("title not escaped: %s", page)
	}

	if !strings.Contains(page, `content="&lt;/script&gt;&lt;img src=x onerror=&#34;alert(1)&#34;&gt; &#34;quoted&#34;"`) {
		t.Errorf("description attribute not escaped: %s", page)
	}

	embedded := embeddedJSON(t, page)
	if !strings.Contains(embedded, `<\/script><img src=x onerror=\"alert(1)\"> \"quoted\"`) {
		t.Errorf("embedded JSON should escape </ as <\\/: %s", embedded)
	}

	if strings.Contains(embedded, "</") {
		t.Errorf("embedded JSON still contains </: %s", embedded)
	}

	var roundTrip report.Report
	if err := json.Unmarshal([]byte(embedded), &roundTrip); err != nil {
		t.Fatalf("embedded JSON is not valid JSON: %v\n%s", err, embedded)
	}

	if roundTrip.Project.Tagline != rep.Project.Tagline {
		t.Errorf("tagline round trip = %q", roundTrip.Project.Tagline)
	}
}

func TestWriteInvalidAccentFallsBack(t *testing.T) {
	t.Parallel()

	rep := loadGolden(t)
	rep.Project.Accent = `red; background: url("x")`

	_, page := render(t, rep)

	if !strings.Contains(page, `<body style="--accent: #e4572e">`) ||
		!strings.Contains(page, `<meta name="theme-color" content="#e4572e">`) {
		t.Errorf("invalid accent should fall back to the default: %s", page)
	}

	// The raw value still appears inside the embedded report JSON, as it did
	// in the Python site; it must not reach any HTML or CSS context.
	if strings.Contains(strings.Replace(page, embeddedJSON(t, page), "", 1), "background") {
		t.Errorf("invalid accent leaked into the page: %s", page)
	}
}

func TestWriteUnwritableOutput(t *testing.T) {
	t.Parallel()

	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := site.Write(loadGolden(t), filepath.Join(blocker, "_site")); err == nil {
		t.Error("expected an error writing under a regular file")
	}
}
