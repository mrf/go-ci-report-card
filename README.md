# Go CI Report Card

A small, self-hosted report card for Go projects, inspired by the spirit of
[Go Report Card](https://github.com/gojp/goreportcard). Every project owns its
workflow, configuration, report generator, and visual assets. GitHub Actions
runs the checks; GitHub Pages serves the result as a static site.

There is no application server, database, external API, analytics script, CDN,
package install, or secret to maintain.

## The zero-cost model

For a **public repository**, this setup uses standard GitHub-hosted runners and
GitHub Pages, both available without usage charges under GitHub's current
public-repository terms. The workflow deliberately avoids larger runners and
retained non-Pages artifacts.

Private repositories use the Actions minutes and Pages availability included
with their GitHub plan. If a guaranteed $0 bill is the requirement, keep the
reporting repositories public and do not opt into paid GitHub usage.

## What lives in each project

```text
.github/workflows/report-card.yml  Run checks, publish Pages, enforce the gate
reportcard/config.toml             Project name, target score, checks, weights
reportcard/generate.py             Dependency-free static report generator
reportcard/template.html           Page structure
reportcard/assets/                 Local CSS and JavaScript
```

The generated `_site/` directory is disposable and ignored by Git. GitHub
Pages receives it directly as a deployment artifact, so there is no `gh-pages`
branch and no bot commit noise.

## Turn it on

1. Copy `.github/workflows/report-card.yml` and the entire `reportcard/`
   directory into a Go repository.
2. Edit `reportcard/config.toml`. In most repositories, `source_dir = "."` is
   already correct. Adjust the project copy, coverage target, check weights,
   and `minimum_score`.
3. Confirm the repository has a root `go.mod`. If it lives elsewhere, change
   `go-version-file` in the workflow and set `project.source_dir` to that module.
4. On GitHub, open **Settings → Pages → Build and deployment** and choose
   **GitHub Actions** as the source.
5. Push to the default branch. The report appears at
   `https://OWNER.github.io/REPOSITORY/`.

Pull requests run the same analysis and quality gate but never deploy Pages.
Pushes to the default branch publish the report even while the separate quality
gate job reports whether the configured score was met.

## Default marks

| Check | Default weight | How it is measured |
|---|---:|---|
| Formatting | 15 | Percentage of Go files accepted by `gofmt` |
| Go vet | 20 | `go vet ./...` exits successfully |
| Build | 20 | `go build ./...` exits successfully |
| Tests & coverage | 30 | Tests pass; coverage is scored against the target |
| Module integrity | 15 | `go mod verify` succeeds |

The overall score is a weighted average. The default gate is 80. All default
checks use only Python's standard library and the official Go toolchain.

## Customize a project

Change the identity and thresholds in `reportcard/config.toml`:

```toml
[project]
name = "acme-api"
tagline = "Build health for the Acme API."
accent = "#2457d6"

[quality]
minimum_score = 85

[checks.tests]
enabled = true
weight = 35
coverage_target = 80
```

Add a repository-owned command with `[[custom_checks]]`. Commands are argument
arrays and are never passed through a shell:

```toml
[[custom_checks]]
id = "staticcheck"
label = "Staticcheck"
description = "Advanced static analysis is clean."
command = ["go", "tool", "staticcheck", "./..."]
weight = 15
enabled = true
```

If a custom tool is not already declared by the project's Go toolchain, add a
pinned install step to that project's workflow. The starter intentionally has
no third-party analyzer downloads.

## Run it locally

Python 3.11+ and Go are the only requirements:

```bash
python3 reportcard/generate.py --output _site --enforce
python3 -m http.server 8000 --directory _site
```

Open `http://localhost:8000`. The second command is only a local preview; the
published site is served by GitHub Pages.

Run the generator tests with:

```bash
python3 -m unittest discover -s tests
```

## Operational notes

- The workflow requests only `contents: read`, `pages: write`, and
  `id-token: write`; it needs no personal access token.
- Pull requests cannot reach the Pages upload or deployment steps.
- Command output is capped before it is embedded into the static report.
- The site contains no cookies, tracking, remote fonts, or remote assets.
- Keep third-party commands pinned if you add them to the workflow.

## Contributing and security

Contributions are welcome. Read [`CONTRIBUTING.md`](CONTRIBUTING.md) before
opening a substantial change and follow the project’s
[`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).

Report suspected vulnerabilities privately as described in
[`SECURITY.md`](SECURITY.md). Release changes are recorded in
[`CHANGELOG.md`](CHANGELOG.md).

This is an independent project and is not affiliated with the original Go
Report Card project.
