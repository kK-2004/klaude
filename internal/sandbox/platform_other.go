//go:build !darwin && !linux && !windows

package sandbox

import "runtime"

// Platform returns a fail-closed stub on unsupported OS families.
func Platform() Backend {
	return Unavailable{Platform: runtime.GOOS}
}
