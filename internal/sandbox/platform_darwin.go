//go:build darwin

package sandbox

// Platform returns the macOS Seatbelt backend.
func Platform() Backend { return &Seatbelt{} }
