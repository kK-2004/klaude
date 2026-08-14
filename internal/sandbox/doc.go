// Package sandbox confines spawned subprocesses under a file-effect policy.
//
// Permission and approval stay separate: this package only wraps argv (or
// equivalent) so the Executor can spawn a confined child. Confined modes are
// fail-closed — never silently run unconfined when a backend is required.
package sandbox
