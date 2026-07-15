package refs

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestEnumerateFiles_WalkFallbackExcludesReservedDirs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package main\n")
	writeFile(t, root, "sub/thing.go", "package sub\n")
	writeFile(t, root, ".pmem/tags/req.foo.json", `{"id":"req.foo"}`)
	writeFile(t, root, ".git/HEAD", "ref: refs/heads/main\n")
	writeFile(t, root, "_workspace/note.md", "scratch\n")
	writeFile(t, root, ".concierge/decision.md", "draft\n")

	got, err := walkFiles(root)
	if err != nil {
		t.Fatalf("walkFiles: %v", err)
	}
	want := []string{"main.go", "sub/thing.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("walkFiles = %v, want %v", got, want)
	}
}

func TestEnumerateFiles_GitLsFilesHonorsGitignore(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	runGitT(t, root, "init", "-q")
	runGitT(t, root, "config", "user.email", "test@example.com")
	runGitT(t, root, "config", "user.name", "test")
	writeFile(t, root, ".gitignore", "ignored.txt\n")
	writeFile(t, root, "tracked.go", "package main\n")
	writeFile(t, root, "ignored.txt", "should not appear\n")
	writeFile(t, root, ".pmem/tags/req.foo.json", `{"id":"req.foo"}`)
	runGitT(t, root, "add", "tracked.go", ".gitignore")

	got, err := EnumerateFiles(root)
	if err != nil {
		t.Fatalf("EnumerateFiles: %v", err)
	}
	sort.Strings(got)
	want := []string{".gitignore", "tracked.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EnumerateFiles = %v, want %v", got, want)
	}
}

func runGitT(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestReadSourceFile_SkipsBinaryAndOversized(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "text.go", "package main\n")
	if err := os.WriteFile(filepath.Join(root, "bin.dat"), []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	big := make([]byte, maxScanFileSize+1)
	if err := os.WriteFile(filepath.Join(root, "big.txt"), big, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, skip, err := readSourceFile(root, "text.go"); err != nil || skip != nil {
		t.Fatalf("text.go should read cleanly, got skip=%v err=%v", skip, err)
	}
	_, skip, err := readSourceFile(root, "bin.dat")
	if err != nil {
		t.Fatalf("readSourceFile bin.dat: %v", err)
	}
	if skip == nil || skip.Reason != "binary" {
		t.Fatalf("expected binary skip note, got %v", skip)
	}
	_, skip, err = readSourceFile(root, "big.txt")
	if err != nil {
		t.Fatalf("readSourceFile big.txt: %v", err)
	}
	if skip == nil || skip.Reason != "too-large" {
		t.Fatalf("expected too-large skip note, got %v", skip)
	}
}
