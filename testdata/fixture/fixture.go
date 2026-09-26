// Package fixture is a tiny module the report card golden test runs against.
package fixture

// Add returns a + b. It is covered by the test.
func Add(a, b int) int {
	return a + b
}

// Sub returns a - b. It is deliberately not covered, so total coverage
// lands below the 70% default target.
func Sub(a, b int) int {
	return a - b
}
