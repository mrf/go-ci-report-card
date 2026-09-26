// Package site renders the static report card: index.html from the embedded
// template, the embedded assets, report.json, and the .nojekyll marker.
package site

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mrf/go-ci-report-card/internal/config"
	"github.com/mrf/go-ci-report-card/internal/report"
)

const (
	filePerm = 0o600
	dirPerm  = 0o750
)

// Assets holds template.html and the assets/ directory copied verbatim into
// every generated site. Exported so tests can compare the written copies.
//
//go:embed template.html assets
var Assets embed.FS

// pageData is what template.html renders.
type pageData struct {
	Title       string
	Description string
	Accent      string
	// ReportData is inserted verbatim inside <script type="application/json">.
	// It is safe as template.JS because it came from report.Marshal
	// (encoding/json) with every "</" rewritten to "<\/", so no substring can
	// close the script element; html/template would otherwise re-escape the
	// JSON string quotes and break JSON.parse in app.js.
	ReportData template.JS
}

// Write renders report r into output: index.html, report.json, assets/, and
// .nojekyll. It mirrors the Python write_site byte for byte apart from
// html/template's choice of entity for quotes.
func Write(r *report.Report, output string) error {
	if err := writeAssets(output); err != nil {
		return err
	}

	if err := writeIndex(r, output); err != nil {
		return err
	}

	if err := report.WriteJSON(r, filepath.Join(output, "report.json")); err != nil {
		return fmt.Errorf("report.json: %w", err)
	}

	if err := os.WriteFile(filepath.Join(output, ".nojekyll"), nil, filePerm); err != nil {
		return fmt.Errorf("write .nojekyll: %w", err)
	}

	return nil
}

// writeIndex renders template.html to output/index.html.
func writeIndex(r *report.Report, output string) error {
	page, err := template.ParseFS(Assets, "template.html")
	if err != nil {
		return fmt.Errorf("parse template.html: %w", err)
	}

	compact, err := report.Marshal(r)
	if err != nil {
		return fmt.Errorf("embed report: %w", err)
	}

	safeJSON := bytes.ReplaceAll(compact, []byte("</"), []byte(`<\/`))

	data := pageData{
		Title:       r.Project.Name,
		Description: r.Project.Tagline,
		Accent:      config.ValidAccent(r.Project.Accent),
		ReportData:  template.JS(safeJSON), //nolint:gosec // see pageData.ReportData
	}

	var buf bytes.Buffer
	if err := page.Execute(&buf, data); err != nil {
		return fmt.Errorf("render index.html: %w", err)
	}

	if err := os.WriteFile(filepath.Join(output, "index.html"), buf.Bytes(), filePerm); err != nil {
		return fmt.Errorf("write index.html: %w", err)
	}

	return nil
}

// writeAssets copies every embedded file under assets/ to output/assets/.
func writeAssets(output string) error {
	dir := filepath.Join(output, "assets")
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("create assets directory: %w", err)
	}

	entries, err := fs.ReadDir(Assets, "assets")
	if err != nil {
		return fmt.Errorf("list embedded assets: %w", err)
	}

	for _, entry := range entries {
		content, err := fs.ReadFile(Assets, "assets/"+entry.Name())
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", entry.Name(), err)
		}

		if err := os.WriteFile(filepath.Join(dir, entry.Name()), content, filePerm); err != nil {
			return fmt.Errorf("write asset %s: %w", entry.Name(), err)
		}
	}

	return nil
}
