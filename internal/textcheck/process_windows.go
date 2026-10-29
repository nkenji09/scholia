//go:build windows

package textcheck

import "os/exec"

// isolateProcessGroup は windows では何もしない（読み手のプロセスだけを止める）。
func isolateProcessGroup(cmd *exec.Cmd) {}
