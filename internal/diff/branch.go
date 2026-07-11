package diff

import "strings"

// CurrentBranch returns the current git branch name for the project rooted
// at dir (typically filepath.Dir(store.Dir), pmem's project root), reusing
// this package's existing git-invocation helpers (2026-07-11 tweaks5 §2 —
// "internal/diff が既に git を扱うので流用"). Returns "" if dir isn't a git
// repo, HEAD is detached, or git itself fails for any reason — this is a
// purely cosmetic derived value (the header's subtitle), never something
// that should block the viewer from starting or an export from succeeding,
// so every failure mode is swallowed here rather than propagated.
func CurrentBranch(dir string) string {
	repoRoot, err := gitRepoRoot(dir)
	if err != nil {
		return ""
	}
	out, err := runGit(repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}
	branch := strings.TrimSpace(out)
	if branch == "" || branch == "HEAD" { // detached HEAD
		return ""
	}
	return branch
}
