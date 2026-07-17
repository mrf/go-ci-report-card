package gocireportcard_test

import (
	"testing"

	gocireportcard "example.com/go-ci-report-card"
)

func TestClampScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{name: "below range", in: -4, want: 0},
		{name: "inside range", in: 82.5, want: 82.5},
		{name: "above range", in: 108, want: 100},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := gocireportcard.ClampScore(test.in); got != test.want {
				t.Fatalf("ClampScore(%v) = %v, want %v", test.in, got, test.want)
			}
		})
	}
}
