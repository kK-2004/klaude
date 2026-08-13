//go:build !windows

package executor

import (
	"os/exec"
	"syscall"
)

// 独立进程组，便于超时后按组 SIGKILL，避免留下孤儿子进程。
func configureProcess(command *exec.Cmd) { command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

func terminate(command *exec.Cmd) {
	if command.Process != nil {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		_ = command.Process.Kill()
	}
}
