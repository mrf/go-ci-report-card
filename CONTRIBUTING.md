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

The report generator requires Python 3.11 or newer. Running the full report also
requires the Go version declared in `go.mod`.

```bash
python3 -m unittest discover -s tests -v
python3 reportcard/generate.py --output _site --enforce
python3 -m http.server 8000 --directory _site
```

Open `http://localhost:8000` to inspect the generated report.

## Linting

The repository's own tooling is linted strictly in CI (`lint.yml`); warnings
are errors. The generator still has zero runtime dependencies — ruff and mypy
are dev-only, pinned in `requirements-dev.txt`.

```bash
python3 -m pip install -r requirements-dev.txt
ruff check . && ruff format --check . && mypy .
golangci-lint run ./... && go test -race ./...
```

Rule relaxations live in `pyproject.toml` and `.golangci.yml`, or as inline
`noqa` comments; each names the rule and the reason. Add new ones the same way.

## Pull requests

- Keep each pull request focused on one concern.
- Add or update tests for behavior changes.
- Update `README.md`, `reportcard/README.md`, and `CHANGELOG.md` when relevant.
- Do not add remote browser assets, analytics, paid services, or runtime
  infrastructure.
- Avoid adding Python or JavaScript package dependencies when the standard
  library can do the job.
- Make sure generated output such as `_site/` is not committed.

The pull request workflow builds the report and enforces the configured quality
gate without publishing to GitHub Pages.

## Reporting vulnerabilities

Do not open a public issue for a suspected vulnerability. Follow
[`SECURITY.md`](SECURITY.md) instead.

By participating, you agree to follow the project’s
[`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).

