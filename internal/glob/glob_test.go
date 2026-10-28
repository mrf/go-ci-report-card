package glob_test

import (
	"testing"

	"github.com/mrf/go-ci-report-card/internal/glob"
)

func TestMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"vendor/**", "vendor/x.go", true},
		{"vendor/**", "vendor/a/b/x.go", true},
		{"vendor/**", "src/vendor/x.go", false},
		{"**/testdata/**", "testdata/x.go", true},
		{"**/testdata/**", "a/b/testdata/c/x.go", true},
		{"**/testdata/**", "testdatax/x.go", false},
		{"**/*.pb.go", "foo.pb.go", true},
		{"**/*.pb.go", "a/b/foo.pb.go", true},
		{"**/*.pb.go", "foo.pb.gox", false},
		{"*_generated.go", "a_generated.go", true},
		{"*_generated.go", "a/b_generated.go", false},
		{"**/*_generated.go", "a/b_generated.go", true},
		{"*.pb.go", "foo.pb.go", true},
		{"*.pb.go", "a/foo.pb.go", false},
		{"a/?.go", "a/b.go", true},
		{"a/?.go", "a/bc.go", false},
		{"a/?.go", "a//.go", false},
		{"a.b", "aXb", false},
		{"a/**/z.go", "a/z.go", true},
		{"a/**/z.go", "a/b/c/z.go", true},
		{"a/**/z.go", "b/z.go", false},
		{"**", "anything/at/all.go", true},
		{"", "", true},
		{"", "x", false},
	}
	for _, tc := range tests {
		got, err := glob.Match(tc.pattern, tc.name)
		if err != nil {
			t.Errorf("Match(%q, %q): unexpected error: %v", tc.pattern, tc.name, err)

			continue
		}

		if got != tc.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}

func TestCompileRejectsBackslash(t *testing.T) {
	t.Parallel()

	if _, err := glob.Compile(`a\b`); err == nil {
		t.Fatal("Compile(`a\\b`) succeeded, want error")
	}
}

func TestPatternString(t *testing.T) {
	t.Parallel()

	p, err := glob.Compile("**/*.pb.go")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if got := p.String(); got != "**/*.pb.go" {
		t.Errorf("String() = %q, want the source pattern", got)
	}
}
