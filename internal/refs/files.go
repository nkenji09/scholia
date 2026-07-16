package refs

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// maxScanFileSize bounds how large a file this package will read for
// scanning/rewriting. Oversized files are skipped, not silently truncated —
// callers always get a SkipNote so the omission is visible in output.
const maxScanFileSize = 5 * 1024 * 1024

// alwaysExcludedDirs never carry source references worth scanning: .scholia/
// is the record store itself (ids appear there as record content, not as
// source references), .git/ is VCS internals, _workspace/ and .concierge/
// are orchestration scratch that isn't part of the product.
var alwaysExcludedDirs = map[string]bool{
	".scholia":   true,
	".git":       true,
	"_workspace": true,
	".concierge": true,
}

// SkipNote records a file EnumerateFiles/Execute chose not to read, and why.
type SkipNote struct {
	Path   string `json:"path"`
	Reason string `json:"reason"` // "binary" | "too-large"
}

// EnumerateFiles lists candidate source files under root (the project
// root — the parent of .scholia/), honoring .gitignore via `git ls-files` when
// git is available, falling back to a directory walk otherwise. Both paths
// apply the always-excluded orchestration/store directories. Returned
// paths are root-relative, "/"-separated, sorted.
//
// The two paths are NOT at full parity: the walk fallback (no git, or git
// missing from PATH) does not parse or honor .gitignore at all — it only
// applies the always-excluded directories above. This only matters for
// projects that (a) aren't a git repo, or (b) run without git on PATH; the
// common case (git repo, git installed) always takes the `git ls-files`
// path and is unaffected. See DESIGN.md §8.5 for the user-facing note.
func EnumerateFiles(root string) ([]string, error) {
	if paths, err := gitLsFiles(root); err == nil {
		return filterExcluded(paths), nil
	}
	return walkFiles(root)
}

func gitLsFiles(root string) ([]string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, err
	}
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	var out []string
	for _, p := range strings.Split(stdout.String(), "\x00") {
		if p != "" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}

func walkFiles(root string) ([]string, error) {
	var out []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			if alwaysExcludedDirs[filepath.Base(rel)] {
				return filepath.SkipDir
			}
			return nil
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func filterExcluded(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if !isExcluded(p) {
			out = append(out, p)
		}
	}
	return out
}

func isExcluded(relPath string) bool {
	for _, part := range strings.Split(relPath, "/") {
		if alwaysExcludedDirs[part] {
			return true
		}
	}
	return false
}

// readSourceFile reads root/relPath for scanning/rewriting. It returns a
// SkipNote (not an error) for files that are binary (a NUL byte in the
// first 8KB) or exceed maxScanFileSize, so callers can surface the skip
// rather than dropping it silently.
func readSourceFile(root, relPath string) ([]byte, *SkipNote, error) {
	absPath := filepath.Join(root, filepath.FromSlash(relPath))
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, nil, err
	}
	if info.Size() > maxScanFileSize {
		return nil, &SkipNote{Path: relPath, Reason: "too-large"}, nil
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, nil, err
	}
	sniff := data
	if len(sniff) > 8192 {
		sniff = sniff[:8192]
	}
	if bytes.IndexByte(sniff, 0) >= 0 {
		return nil, &SkipNote{Path: relPath, Reason: "binary"}, nil
	}
	return data, nil, nil
}
