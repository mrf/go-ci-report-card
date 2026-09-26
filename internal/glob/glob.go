// Package glob matches slash-separated relative paths against gitignore-style
// patterns.
//
// Semantics:
//   - `**` as a whole path segment matches zero or more segments.
//   - `*` matches any run of characters within one segment.
//   - `?` matches exactly one character within one segment.
//   - Every other character matches itself. There are no character classes
//     and no escaping; a backslash is rejected at compile time so a Windows
//     path separator cannot be mistaken for a literal.
//
// Patterns are anchored: they must match the whole path.
package glob

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrBackslash is returned when a pattern contains a backslash.
var ErrBackslash = errors.New("glob: backslash is not allowed; use '/' as the path separator")

// Pattern is a compiled glob.
type Pattern struct {
	source string
	re     *regexp.Regexp
}

// Compile translates a glob pattern into an anchored regular expression.
func Compile(pattern string) (*Pattern, error) {
	if strings.Contains(pattern, `\`) {
		return nil, ErrBackslash
	}

	var sb strings.Builder

	sb.WriteString("^")

	segments := strings.Split(pattern, "/")
	for i, segment := range segments {
		last := i == len(segments)-1

		switch {
		case segment == "**" && last:
			sb.WriteString(".*")
		case segment == "**":
			sb.WriteString("(?:[^/]+/)*")
		case last:
			writeSegment(&sb, segment)
		default:
			writeSegment(&sb, segment)
			sb.WriteString("/")
		}
	}

	sb.WriteString("$")

	re, err := regexp.Compile(sb.String())
	if err != nil {
		// Every construct written above is valid regexp syntax, so this is
		// unreachable in practice, but the error is surfaced rather than
		// swallowed.
		return nil, fmt.Errorf("glob: compile %q: %w", pattern, err)
	}

	return &Pattern{source: pattern, re: re}, nil
}

// writeSegment translates one path segment, keeping wildcards within it.
func writeSegment(sb *strings.Builder, segment string) {
	for _, r := range segment {
		switch r {
		case '*':
			sb.WriteString("[^/]*")
		case '?':
			sb.WriteString("[^/]")
		default:
			sb.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
}

// Match reports whether name matches the compiled pattern.
func (p *Pattern) Match(name string) bool {
	return p.re.MatchString(name)
}

// String returns the source pattern.
func (p *Pattern) String() string {
	return p.source
}

// Match compiles pattern and reports whether name matches it.
func Match(pattern, name string) (bool, error) {
	p, err := Compile(pattern)
	if err != nil {
		return false, err
	}

	return p.Match(name), nil
}
