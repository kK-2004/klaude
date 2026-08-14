//go:build windows

package sandbox

import "runtime"

// Platform returns the Windows Restricted Token + ACL backend.
func Platform() Backend {
	return &WinACL{}
}

func init() {
	// Keep GOOS visible in diagnostics for misbuilt binaries.
	_ = runtime.GOOS
}
