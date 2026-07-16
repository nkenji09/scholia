// Package diff computes the semantic diff (§4) between the current working
// tree's .scholia/ records and the same records at a git ref (default HEAD).
// The ref side is read via `git ls-tree` / `git show` (os/exec) — no new
// module dependency, and no requirement that the ref be checked out.
package diff
