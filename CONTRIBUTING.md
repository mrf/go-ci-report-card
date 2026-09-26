# Contributing to Go CI Report Card

Thanks for helping make repository-owned Go quality reports easier to use.

## Before opening a change

- Search existing issues and pull requests first.
- Open an issue before starting a large feature or a change to the scoring model.
- Keep the project’s core constraints intact: zero required hosting cost for
  public repositories, static GitHub Pages output, and project-owned assets and
  configuration.

Small fixes, documentation improvements, and new tests can go directly to a
pull request.

## Development setup

The only requirement is the Go version declared in `go.mod`.

```bash
go test -race -v ./...
go run ./cmd/reportcard -config examples/config.toml -output _site -enforce
```

Open `_site/index.html` to inspect the generated report.

## Linting

The repository is linted strictly in CI (`lint.yml`); warnings are errors.

```bash
go vet ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run ./...
go test -race ./...
```

Rule relaxations live in `.golangci.yml` or as inline `//nolint:<rule> // reason`
comments; each names the rule and the reason. Add new ones the same way.

## Pull requests

- Keep each pull request focused on one concern.
- Add or update tests for behavior changes.
- Update `README.md`, `examples/config.toml`, and `CHANGELOG.md` when relevant.
- Do not add remote browser assets, analytics, paid services, or runtime
  infrastructure.
- Avoid adding Go or JavaScript package dependencies when the standard
  library can do the job.
- Make sure generated output such as `_site/` is not committed.

The pull request workflow builds the report and enforces the configured quality
gate without publishing to GitHub Pages.

## Reporting vulnerabilities

Do not open a public issue for a suspected vulnerability. Follow
[`SECURITY.md`](SECURITY.md) instead.

By participating, you agree to follow the project’s
[`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).

