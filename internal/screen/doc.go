// Package screen defines the Screen interface and a Registry that holds
// multiple screen implementations with round-robin rotation.
//
// A Screen produces images on demand (e.g. from a local directory, a
// URL, or a generated graphic). The registry cycles through registered
// screens so the device sees a different source on each refresh when
// rotation is enabled.
package screen
