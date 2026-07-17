// Package gocireportcard contains the starter repository's CI smoke fixture.
// The report generator itself is dependency-free Python so it can be vendored.
package gocireportcard

// maxScore is the upper bound of a report score.
const maxScore = 100

// ClampScore bounds a report score to the inclusive range 0 through 100.
func ClampScore(score float64) float64 {
	if score < 0 {
		return 0
	}

	if score > maxScore {
		return maxScore
	}

	return score
}
