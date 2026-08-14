package sandbox

import (
	"context"
	"os/exec"
)

// CommandBuilder prepares an *exec.Cmd that is already confined.
// Windows Restricted Token + ACL cannot be expressed as argv wrapping alone,
// so the Executor prefers PrepareCommand when the backend implements this.
type CommandBuilder interface {
	Backend
	PrepareCommand(ctx context.Context, program string, args []string, dir string, env []string, policy Policy) (*exec.Cmd, Confined, error)
}
