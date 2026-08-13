//go:build windows

package executor

import "os/exec"

// Windows 下进程树由 CommandContext 托管；此处仅 Kill 根进程。
// 预留 Job Object 扩展点，避免在共享代码里引入 Unix-only SysProcAttr。
func configureProcess(_ *exec.Cmd) {}
func terminate(command *exec.Cmd) {
	if command.Process != nil {
		_ = command.Process.Kill()
	}
}
