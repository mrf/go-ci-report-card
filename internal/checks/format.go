package checks

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mrf/go-ci-report-card/internal/glob"
	"github.com/mrf/go-ci-report-card/internal/report"
)

// gofmtBatch is how many files each gofmt invocation receives, keeping the
// argument list well under every platform's limit.
const gofmtBatch = 100

// Format scores the share of Go files under the runner's directory that
// gofmt accepts as canonically formatted. Files matching an exclude glob
// (relative, slash-separated) are skipped.
func Format(ctx context.Context, runner *Runner, weight float64, excludes []string) report.Check {
	label, description := Info(IDFormat)

	files, err := goFiles(runner.Dir, excludes)
	if err != nil {
		return report.NewCheck(IDFormat, label, description, weight, 0, "Unable to check", []string{err.Error()}, 0, "")
	}

	if len(files) == 0 {
		return report.NewCheck(IDFormat, label, description, weight, 0, "No Go files",
			[]string{"No Go files matched the configured source directory."}, 0, "")
	}

	started := time.Now()

	var unformatted, errs []string

	for offset := 0; offset < len(files); offset += gofmtBatch {
		batch := files[offset:min(offset+gofmtBatch, len(files))]

		out := runner.Run(ctx, append([]string{"gofmt", "-l"}, batch...)...)
		if out.Code != 0 {
			errs = append(errs, out.Lines...)
		} else {
			unformatted = append(unformatted, out.Lines...)
		}
	}

	duration := time.Since(started).Seconds()
	limit := runner.DetailsLimit

	if len(errs) > 0 {
		return report.NewCheck(IDFormat, label, description, weight, 0, "Unable to check",
			errs[:min(limit, len(errs))], duration, "")
	}

	// As in the Python generator, each batch's output was already trimmed to
	// the details limit, so a batch with more unformatted files than the
	// limit counts the omission note as one file.
	formatted := len(files) - len(unformatted)
	score := float64(formatted) / float64(len(files)) * report.MaxScore
	observed := fmt.Sprintf("%d / %d files formatted", formatted, len(files))

	return report.NewCheck(IDFormat, label, description, weight, score, observed,
		unformatted[:min(limit, len(unformatted))], duration, "")
}

// goFiles lists the sorted, slash-separated relative paths of every regular
// *.go file under root that no exclude pattern matches.
func goFiles(root string, excludes []string) ([]string, error) {
	patterns, err := compilePatterns(excludes)
	if err != nil {
		return nil, err
	}

	var files []string

	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || !isRegular(path, entry) {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("relative path of %s: %w", path, err)
		}

		rel = filepath.ToSlash(rel)
		for _, pattern := range patterns {
			if pattern.Match(rel) {
				return nil
			}
		}

		files = append(files, rel)

		return nil
	}

	if err := filepath.WalkDir(root, walk); err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}

	sort.Strings(files)

	return files, nil
}

// compilePatterns compiles every exclude glob.
func compilePatterns(excludes []string) ([]*glob.Pattern, error) {
	patterns := make([]*glob.Pattern, 0, len(excludes))

	for _, exclude := range excludes {
		pattern, err := glob.Compile(exclude)
		if err != nil {
			return nil, fmt.Errorf("exclude pattern %q: %w", exclude, err)
		}

		patterns = append(patterns, pattern)
	}

	return patterns, nil
}

// isRegular reports whether the entry is a regular file, following a symlink
// as Python's Path.is_file does.
func isRegular(path string, entry fs.DirEntry) bool {
	if entry.Type().IsRegular() {
		return true
	}

	if entry.Type()&fs.ModeSymlink == 0 {
		return false
	}

	info, err := os.Stat(path)

	return err == nil && info.Mode().IsRegular()
}
