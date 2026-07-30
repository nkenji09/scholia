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
	Reason string `json:"reason"` // "binary" | "too-large" | "not-regular" | "unreadable"
}

// EnumerateFiles lists candidate source files under root (the project
// root — the parent of .scholia/), honoring .gitignore via `git ls-files` when
// git is available, falling back to a directory walk otherwise. Both paths
// apply the always-excluded orchestration/store directories. Returned
// paths are root-relative, "/"-separated, sorted.
//
// What comes back is a list of *candidates*, not a promise that each one is a
// readable regular file — neither path stats a candidate to find out, and
// deciding that is readSourceFile's single job. Failing to reach root at all
// is still fatal (there would be no candidate list to report), but no
// individual path below root ever fails enumeration.
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
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			// Root itself. If root can't be walked there is no candidate
			// list to hand back, so this one stays fatal.
			return walkErr
		}
		rel = filepath.ToSlash(rel)
		if isExcluded(rel) {
			if walkErr == nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if walkErr != nil {
			// One path this walk could not classify: a directory it may not
			// list, an entry whose Lstat failed, an entry that vanished
			// mid-walk. Enumeration does not get to decide readability, so
			// hand the path on as a candidate rather than aborting the whole
			// walk — readSourceFile is the one place that answers read-or-skip,
			// and it will record a visible SkipNote for it. (Walk does not
			// descend into a directory it could not read, so the files under
			// such a directory are not enumerated; the skip names the
			// directory, not each file lost under it.)
			out = append(out, rel)
			return nil
		}
		if info.IsDir() {
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

// readSourceFile reads root/relPath for scanning/rewriting. It answers, for
// every candidate path it is handed, either "here is the content" or "here is
// why it was skipped" — it returns no error at all, so no one bad candidate
// can abort a whole scan, and callers surface every SkipNote so the omission
// stays visible in output.
//
// The two path-shape reasons are stated as properties — the resolved path is
// not a regular file, or it cannot be stat'd/read — rather than as a list of
// offenders, because enumeration yields candidates, not readable files:
// `git ls-files` reports a symlink-to-directory or a gitlink as one entry, the
// walk fallback classifies with Lstat (so a symlink-to-directory looks like a
// file there too) and hands on paths it could not classify at all, and any
// entry can vanish or change mode between enumeration and read. Naming today's
// offenders one at a time would leave the hole open for the next one: a FIFO, a
// socket, a device node, a broken symlink, a permission-denied path.
//
// The mode check is not just a nicer reason string than the read failure below
// it — it is the only thing that keeps a non-regular candidate from being
// *opened*. A FIFO with no writer blocks in open() forever, and a character
// device reports Size() == 0 so maxScanFileSize cannot bound it. Neither ever
// returns, so no read-failed-so-skip-it fallback can catch them.
//
// It rejects the resolved mode, not symlinks: a symlink to a regular file is
// read like any other file.
func readSourceFile(root, relPath string) ([]byte, *SkipNote) {
	absPath := filepath.Join(root, filepath.FromSlash(relPath))
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, &SkipNote{Path: relPath, Reason: "unreadable"}
	}
	if !info.Mode().IsRegular() {
		return nil, &SkipNote{Path: relPath, Reason: "not-regular"}
	}
	if info.Size() > maxScanFileSize {
		return nil, &SkipNote{Path: relPath, Reason: "too-large"}
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, &SkipNote{Path: relPath, Reason: "unreadable"}
	}
	sniff := data
	if len(sniff) > 8192 {
		sniff = sniff[:8192]
	}
	if bytes.IndexByte(sniff, 0) >= 0 {
		return nil, &SkipNote{Path: relPath, Reason: "binary"}
	}
	return data, nil
}
