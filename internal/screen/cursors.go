package screen

// cursors tracks per-device rotation positions keyed by device ID.
// Not safe for concurrent use; callers hold their own lock.
type cursors map[string]int

// next returns the current position for id among n entries and
// advances id's cursor. n must be > 0.
func (c cursors) next(id string, n int) int {
	i := c[id] % n
	c[id] = (i + 1) % n
	return i
}

// current returns the current position for id among n entries
// without advancing. n must be > 0.
func (c cursors) current(id string, n int) int {
	return c[id] % n
}
