//go:build !windows

package textcheck

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// 時間切れで、読み手が起こした子のプロセスも残らない。
func TestTimeoutKillsTheReadersChildren(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	t.Setenv(Variable, tool(t, "sleep 60 &\necho $! > '"+pidFile+"'\nwait"))
	got := Reader{Timeout: 500 * time.Millisecond}.Read("問い", records())
	if len(got.NotChecked) != 1 || !strings.HasPrefix(got.NotChecked[0].Reason, "上限の") {
		t.Fatalf("時間切れのはずが %+v", got)
	}
	b, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Fatalf("読み手が起こした子（pid %d）が時間切れの後も残っている", pid)
}
