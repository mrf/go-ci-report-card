# Go rewrite and distribution redesign

Status: reviewed and approved by Mark 2026-09-26.

## Problem

Integration today means copying `reportcard/` (a 685-line Python generator,
template, CSS, JS) plus a workflow into every consuming Go repo. Consequences:

- A Python toolchain lands in a Go project. Strict consumers inherit ruff/mypy
  obligations for code they did not write.
- No version pin and no upgrade path. Fixes here are re-copied by hand.
- A 52-line TOML plus template and assets for what is usually three decisions:
  name, coverage target, minimum score.
- This repo carries empty scaffold dirs (`app/`, `build/`, `db/`, `drizzle/`,
  `public/`, `worker/`, `examples/d1`) that confuse anyone copying "the whole
  thing".

## Goals

1. Consumer adds no source files. Integration is a workflow snippet.
2. One language. The tool is Go, tested and linted with the Go toolchain only.
3. Versioned. Consumers pin a tag; upgrades are a one-line bump.
4. Same zero-cost model: Actions + Pages, no secrets, no server.
5. Byte-compatible `report.json` (schema_version 1) and identical scoring so
   the current published pages do not change meaning.

## Non-goals

- Hosting anything centrally.
- Changing the check set, weights, or grade scale.
- Supporting non-GitHub CI in v1 (the tool runs anywhere, the deploy wrapper
  is GitHub-only).

## Design

### Module and layout

Module path becomes `github.com/mrf/go-ci-report-card` (currently
`example.com/...`, which blocks `go run pkg@version`).

```
cmd/reportcard/main.go        CLI entry point
internal/config/              TOML loading + validation
internal/checks/              format, vet, build, tests, modules, custom
internal/report/              scoring, grading, report struct, JSON
internal/site/                html/template rendering, embedded assets
internal/site/assets/         style.css, app.js (moved from reportcard/assets)
internal/site/template.html
action.yml                    composite action wrapper
.github/workflows/reportcard.yml   reusable workflow (workflow_call)
```

Python files, `pyproject.toml`, `requirements-dev.txt`, `tests/`, the
`reportcard/` dir, `smoke.go`, `smoke_test.go`, and the empty scaffold dirs
are deleted. Dependabot drops the pip ecosystem.

### CLI

```
reportcard [flags]
  -config string      TOML path (optional; defaults apply when absent)
  -repo-root string   default "."
  -source-dir string  overrides project.source_dir
  -output string      default "_site"
  -min-score float    overrides quality.minimum_score
  -coverage-target float  overrides checks.tests.coverage_target
  -name string        overrides project.name
  -enforce            exit 1 when the gate fails
  -github-output string   defaults to $GITHUB_OUTPUT when set
```

Flag > config file > built-in default. Every default from the current
`config.toml` becomes a Go default so a consumer with no config file gets the
same report the starter gives today. `GITHUB_STEP_SUMMARY`, `GITHUB_REPOSITORY`,
`GITHUB_SHA`, `GITHUB_REF_NAME`, and `REPORTCARD_NOW` keep their current
meaning.

### Config

TOML schema unchanged. Parsed with `github.com/BurntSushi/toml` into typed
structs with `toml:"..."` tags. This is the one third-party dependency; the
stdlib has no TOML parser and custom checks with weights are awkward as flags.
Validation errors keep the current messages where practical.

### Checks

Direct ports, one file each, sharing a `runner` that executes an argv (never a
shell) in `source_dir` with `exec.CommandContext` and the configured timeout,
captures combined output, and trims to `details_limit` with the same
"omitted N lines" note.

- format: walk `*.go` under source, apply `exclude` globs, batch `gofmt -l`
  in groups of 100. Glob matching is a small in-tree matcher (pattern to
  regexp): `**` matches zero or more path segments, `*` and `?` stay within
  one segment. This is gitignore-style and differs from the Python `fnmatch`
  behavior, where `*` crossed slashes and `**/testdata/**` missed a top-level
  `testdata/` dir. Table-tested; noted in the changelog as a behavior change.
- vet, build, modules: single command, pass/fail.
- tests: `go test -covermode=atomic -coverprofile=<tmp> ./...` then
  `go tool cover -func`, parse the `total:` line, same score/status rules.
- custom: from `[[custom_checks]]`, same fields.

Score = weighted mean rounded to one decimal. Grade scale copied verbatim.

### Site output

`html/template` renders `template.html` with the report. Assets are
`//go:embed`ed and written to `_site/assets/`. `report.json` uses
`encoding/json` with no indentation; the embedded copy in the page escapes
`</` exactly as today. `.nojekyll` is written.

### Distribution

Three layers, each thinner than the last.

1. **Binary.** `go run github.com/mrf/go-ci-report-card/cmd/reportcard@v1.0.0`.
   Works anywhere Go is installed, including locally.
2. **Composite action** (`action.yml`). Inputs: `version`, `config`,
   `source-dir`, `min-score`, `coverage-target`, `name`, `output`, `enforce`.
   Outputs: `passed`, `score`, `grade`. Steps: `go run` the pinned version with
   the flags mapped from inputs. Assumes the consumer already ran
   `actions/setup-go`.
3. **Reusable workflow** (`workflow_call`). Wraps checkout, setup-go, the
   action, Pages upload/deploy on default-branch pushes, and the quality gate
   job. Consumer file:

```yaml
name: Report card
on: [push, pull_request]
permissions: {contents: read, pages: write, id-token: write}
jobs:
  report:
    uses: mrf/go-ci-report-card/.github/workflows/reportcard.yml@v1
    with:
      min-score: 85
      coverage-target: 80
```

Tags: semver `vX.Y.Z` plus a moving `v1` major tag, updated on each release.

### Dogfooding

This repo's own CI runs the reusable workflow on itself and publishes its own
report card to Pages. That replaces `smoke.go` as the proof it works.

### Testing

- Unit: config parsing and validation, grade scale, score rounding, output
  trimming, coverage line parsing, exclude matching, JSON shape.
- Golden: run against a fixture module under `testdata/` with `REPORTCARD_NOW`
  fixed and compare `report.json` to a checked-in golden file. Port the
  existing Python test cases as the starting list.
- Integration: `go run ./cmd/reportcard -repo-root testdata/fixture` in CI.
- All under `go test -race ./...`, golangci-lint with the existing strict set.

### Migration

The only known consumer is this repo. `CHANGELOG.md` gets a 1.0.0 entry
stating the Python generator is removed and pointing at the workflow snippet.
The README "Turn it on" section shrinks to the five-line workflow plus the
Pages setting.

## Decisions (reviewed 2026-09-26)

1. TOML stays, with `BurntSushi/toml` as the one dependency.
2. In-tree `**` glob matcher, no doublestar dependency.
3. Toolchain and lint versions: start from current versions at implementation
   time; the only consumer is this repo, so no compatibility hold.

## Work breakdown

Serial, each a worktree that merges before the next starts:

1. Module rename, layout, config package with tests, delete Python and scaffold.
2. Checks and report packages with golden test.
3. Site rendering with embedded assets.
4. `action.yml`, reusable workflow, dogfood CI, README/CHANGELOG.
