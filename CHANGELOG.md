# Changelog

All notable changes to Go CI Report Card will be documented in this file.

The project follows [Semantic Versioning](https://semver.org/) starting with the
first tagged release.

## Unreleased

### 1.0.0

- The Python generator is removed. The tool is now a Go binary at module path
  `github.com/mrf/go-ci-report-card`, distributed three ways: `go run
  github.com/mrf/go-ci-report-card/cmd/reportcard@v1`, the composite action
  `mrf/go-ci-report-card@v1`, and the reusable workflow
  `mrf/go-ci-report-card/.github/workflows/reportcard.yml@v1`. Consumers no
  longer copy any source files; see the workflow snippet in `README.md`.
- Exclude globs are gitignore-style: `*` and `?` no longer cross slashes, and
  `**/testdata/**` now matches a top-level `testdata/` directory.

### Added

- Repository-owned Go quality report generator and static report interface.
- Weighted checks for formatting, vet, build, tests and coverage, and module
  integrity.
- Configurable quality gate for pushes and pull requests.
- GitHub Pages deployment from the default branch.
- Local-only browser assets with no analytics or external runtime services.
- Open-source community, contribution, and security documentation.

