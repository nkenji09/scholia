//go:build !windows

package textcheck

import (
	"os/exec"
	"syscall"
)

// isolateProcessGroup は、読み手を新しいプロセスグループで起動し、止めるときはグループごと SIGKILL する。
func isolateProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
