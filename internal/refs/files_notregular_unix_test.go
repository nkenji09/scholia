//go:build unix

package refs

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestReadSourceFile_NeverOpensNonRegularCandidate is the guard for what the
// mode check in readSourceFile uniquely buys, and it needs a FIFO to show it.
//
// For every other non-regular candidate, dropping the mode check only changes
// the SkipNote's reason from "not-regular" to "unreadable": opening a directory
// succeeds and the *read* fails, so a read-failed-so-skip-it fallback catches
// it and the scan still completes. A FIFO is different — open() blocks until a
// writer appears, so the read never fails and never returns. Without the mode
// check the whole command hangs forever, which no fallback further down can
// convert into a skip.
//
// Scope: this asserts readSourceFile returns (promptly, with a skip) for a
// candidate whose resolved mode is a named pipe. Character devices such as
// /dev/zero are the same shape — Size() == 0 defeats maxScanFileSize, so the
// read is unbounded rather than blocking — and are not exercised here because
// the test would have to read from a device node to fail.
func TestReadSourceFile_NeverOpensNonRegularCandidate(t *testing.T) {
	root := t.TempDir()
	fifo := filepath.Join(root, "pipe.sock")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}

	done := make(chan *SkipNote, 1)
	go func() {
		_, skip := readSourceFile(root, "pipe.sock")
		done <- skip
	}()

	select {
	case skip := <-done:
		if skip == nil || skip.Reason != "not-regular" {
			t.Fatalf("FIFO candidate: expected not-regular skip, got %v", skip)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("readSourceFile opened a FIFO instead of skipping it: open() blocks until a writer appears, so this never returns and the whole scan hangs")
	}
}
