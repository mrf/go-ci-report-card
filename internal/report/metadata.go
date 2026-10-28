package report

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

const (
	gitTimeout = 5 * time.Second
	// timeLayout is Python's datetime.isoformat() with the "+00:00" suffix
	// replaced by "Z".
	timeLayout        = "2006-01-02T15:04:05.000000Z"
	timeLayoutNoMicro = "2006-01-02T15:04:05Z"
)

// RepositoryMetadata resolves the repository identity from the GitHub
// Actions environment, falling back to git for the commit and branch.
func RepositoryMetadata(ctx context.Context, repoRoot, repositoryURL string, getenv func(string) string) Repository {
	commit := getenv("GITHUB_SHA")
	if commit == "" {
		commit = gitValue(ctx, repoRoot, "rev-parse", "HEAD")
	}

	branch := getenv("GITHUB_REF_NAME")
	if branch == "" {
		branch = gitValue(ctx, repoRoot, "branch", "--show-current")
	}

	return Repository{
		Slug:     getenv("GITHUB_REPOSITORY"),
		URL:      repositoryURL,
		Branch:   branch,
		SHA:      commit,
		ShortSHA: commit[:min(len(commit), shortSHALength)],
	}
}

// gitValue returns trimmed git output, or "" when git is unavailable, times
// out, or fails.
func gitValue(ctx context.Context, repoRoot string, args ...string) string {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // fixed git subcommands, no user-controlled arguments
	cmd.Dir = repoRoot

	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}

// GeneratedAt returns REPORTCARD_NOW when set, else the current UTC time in
// Python isoformat style (microseconds omitted when zero, as Python does).
func GeneratedAt(getenv func(string) string, now func() time.Time) string {
	if fixed := getenv("REPORTCARD_NOW"); fixed != "" {
		return fixed
	}

	t := now().UTC()
	if t.Nanosecond()/int(time.Microsecond) == 0 {
		return t.Format(timeLayoutNoMicro)
	}

	return t.Format(timeLayout)
}
