package refs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Pair is one old-id -> new-id substitution to look for / apply.
type Pair struct {
	OldID string
	NewID string
}

// Match is one accepted, boundary-safe occurrence of a Pair's OldID in a
// source file.
type Match struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
	Old  string `json:"old"`
	New  string `json:"new"`
}

// FailedFile is a file Execute could not write back when apply is true —
// either the write itself failed, or Execute declined to write because the
// candidate path is a symlink and following it would leave the enumerated,
// in-scope set (see writeRefused). Source rewriting is best-effort: a failure
// here does not unwind the `.scholia` rename that already committed. The file
// can be retried later via a fresh Execute/rewrite call, which is idempotent.
type FailedFile struct {
	Path string `json:"path"`
	Err  string `json:"err"`
}

// LinkNote records a candidate whose own path is a symlink and that an apply
// run therefore did not write through — Execute writes only to enumerated,
// in-scope candidate paths themselves (see writeDecision).
//
// Covered says whether that refusal cost anything. When the link fully
// resolves to a path that is itself a candidate of the same run, the target is
// rewritten on its own turn, so the ids do get fixed and the link survives —
// this note is informational and the run stays a success. When it is false the
// old ids are still there and the same path also appears in Report.Failed.
type LinkNote struct {
	Path    string `json:"path"`
	Target  string `json:"target"`
	Covered bool   `json:"covered"`
}

// Report summarizes one Execute call.
type Report struct {
	Matches        []Match      `json:"matches,omitempty"`
	RewrittenFiles []string     `json:"rewrittenFiles,omitempty"`
	Failed         []FailedFile `json:"failed,omitempty"`
	Skipped        []SkipNote   `json:"skipped,omitempty"`
	Links          []LinkNote   `json:"links,omitempty"`
}

// UnreadableSkips returns the skips that mean unfinished work rather than a
// deliberate omission: a candidate that should have been readable and was not
// (SkipUnreadable). After an apply run, each one is a file whose old ids are
// still there, so a caller that acts on source must not call such a run a
// success — whereas SkipBinary/SkipTooLarge/SkipNotRegular are files this
// package is designed never to read, and skipping them completes the job.
//
// The distinction is by property, not by path: the same file is "unfinished"
// or "not ours to read" depending only on whether reading it failed.
//
// One case sits on the line and is worth naming: a directory the walk fallback
// could not list comes back as SkipNotRegular (readSourceFile only sees a
// directory, and cannot tell it from a gitlink), so source files under it are
// neither scanned nor counted here. On the `git ls-files` path — the default
// whenever the project is a git repo — those files are enumerated individually
// and each unreadable one is counted, so this gap is specific to running with
// no git available.
func (r Report) UnreadableSkips() []SkipNote {
	var out []SkipNote
	for _, sk := range r.Skipped {
		if sk.Reason == SkipUnreadable {
			out = append(out, sk)
		}
	}
	return out
}

// Execute scans root for every pair's OldID and, when apply is true,
// replaces each boundary-safe occurrence with the pair's NewID, writing
// changed files atomically (temp file + rename), one file at a time. With
// apply false, it only collects Matches — a dry-run preview built from the
// same matching path apply uses, so for the pairs this package's own
// callers construct (rename's plan, or a single CLI-supplied old/new
// pair — always boundary-disjoint, see below) the preview matches what
// apply would do.
//
// Pairs are applied independently against each file's current buffer, and
// for boundary-disjoint pairs the order doesn't matter: because
// findOccurrences rejects any occurrence whose trailing run contains a
// letter/digit, one pair's OldID can't match inside another pair's
// OldID/NewID text (a cascade renaming both "req.foo" and "req.foo-bar"
// can't have the first pair's replacement swallow the second's, and vice
// versa). This guarantee is about the pair *shapes* CLI callers construct,
// not a property Execute enforces on arbitrary caller-supplied pairs —
// e.g. a caller that hands Execute overlapping pairs like {OldID: "a",
// NewID: "b"} and {OldID: "b", NewID: "c"} could see a value rewritten
// twice in one apply pass; nothing here validates pairs are disjoint.
//
// Applying is confined to the candidate paths themselves: a candidate whose
// own path is a symlink is read like any other file but never written — see
// writeDecision for why, and Report.Links/Report.Failed for how each such path
// is reported. This is the one way a dry-run's Matches can list an occurrence
// that a following apply does not rewrite; the apply says so per path rather
// than passing over it quietly.
//
// opts is optional (variadic so every pre-existing call site keeps
// compiling and behaving identically): passing none, or the zero value,
// scans the whole project root exactly as before this parameter existed.
// A non-zero Options narrows/excludes per model.Config's SourceRefs (the
// CLI passes it through when config.json sets sourceRefs.scan/exclude).
func Execute(root string, pairs []Pair, apply bool, opts ...Options) (Report, error) {
	files, err := EnumerateFiles(root)
	if err != nil {
		return Report{}, err
	}
	if len(opts) > 0 {
		files = filterScope(files, opts[0])
	}
	candidates := make(map[string]bool, len(files))
	for _, f := range files {
		candidates[f] = true
	}
	sorted := make([]Pair, len(pairs))
	copy(sorted, pairs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].OldID < sorted[j].OldID })

	var report Report
	for _, relPath := range files {
		content, skip := readSourceFile(root, relPath)
		if skip != nil {
			report.Skipped = append(report.Skipped, *skip)
			continue
		}

		var fileMatches []Match
		changed := false
		for _, p := range sorted {
			offsets := findOccurrences(content, p.OldID)
			if len(offsets) == 0 {
				continue
			}
			for _, off := range offsets {
				fileMatches = append(fileMatches, Match{
					Path: relPath,
					Line: lineAt(content, off),
					Text: lineText(content, off),
					Old:  p.OldID,
					New:  p.NewID,
				})
			}
			if apply {
				content = replaceAt(content, p.OldID, p.NewID, offsets)
				changed = true
			}
		}
		if len(fileMatches) == 0 {
			continue
		}
		sort.Slice(fileMatches, func(i, j int) bool { return fileMatches[i].Line < fileMatches[j].Line })
		report.Matches = append(report.Matches, fileMatches...)

		if apply && changed {
			link := inspectLink(root, relPath)
			switch classifyWrite(link, candidates) {
			case writeDeferred:
				report.Links = append(report.Links, LinkNote{Path: relPath, Target: link.shown, Covered: true})
			case writeRefused:
				report.Links = append(report.Links, LinkNote{Path: relPath, Target: link.shown, Covered: false})
				report.Failed = append(report.Failed, FailedFile{
					Path: relPath,
					Err: fmt.Sprintf("symlink のため書き戻していません（リンク先 %s は走査候補ではないため、旧 id が残っています）",
						link.shown),
				})
			default:
				absPath := filepath.Join(root, filepath.FromSlash(relPath))
				if err := writeFileAtomic(absPath, content); err != nil {
					report.Failed = append(report.Failed, FailedFile{Path: relPath, Err: err.Error()})
					continue
				}
				report.RewrittenFiles = append(report.RewrittenFiles, relPath)
			}
		}
	}
	sort.Strings(report.RewrittenFiles)
	return report, nil
}

// ScanIDs finds every boundary-safe occurrence of any of ids in root's
// source files — an inventory/health-check read, not tied to a rename (no
// replacement happens; NewID is set equal to OldID as a placeholder Execute
// never uses in dry-run mode). opts is optional, same as Execute's.
func ScanIDs(root string, ids []string, opts ...Options) (Report, error) {
	pairs := make([]Pair, len(ids))
	for i, id := range ids {
		pairs[i] = Pair{OldID: id, NewID: id}
	}
	return Execute(root, pairs, false, opts...)
}

// lineAt returns the 1-indexed line number containing byte offset.
func lineAt(content []byte, offset int) int {
	line := 1
	for i := 0; i < offset && i < len(content); i++ {
		if content[i] == '\n' {
			line++
		}
	}
	return line
}

// lineText returns the trimmed source line containing offset.
func lineText(content []byte, offset int) string {
	start := offset
	for start > 0 && content[start-1] != '\n' {
		start--
	}
	end := offset
	for end < len(content) && content[end] != '\n' {
		end++
	}
	return strings.TrimSpace(string(content[start:end]))
}

// replaceAt rewrites content, substituting new for old at each of the given
// offsets. offsets must be already boundary-validated (as findOccurrences
// returns them) and sorted ascending.
func replaceAt(content []byte, old, new string, offsets []int) []byte {
	if len(offsets) == 0 {
		return content
	}
	var out []byte
	prev := 0
	for _, off := range offsets {
		out = append(out, content[prev:off]...)
		out = append(out, new...)
		prev = off + len(old)
	}
	out = append(out, content[prev:]...)
	return out
}

// writeDecision is what an apply run does with a file it has already rewritten
// in memory and is about to write back.
//
// The rule behind the three values is one sentence: *write back only to an
// enumerated, in-scope candidate path itself — never follow a symlink to write
// somewhere else.* That single invariant closes three holes at once, which is
// why it is stated as a property rather than as a list of link shapes:
//
//   - writeFileAtomic replaces a path via rename, so writing *to* a symlink
//     replaces the link with a regular file. The link target the user wrote is
//     gone and cannot be reconstructed.
//   - writing *through* the link instead would put bytes on a path that
//     enumeration never produced — possibly another repository entirely.
//   - filterScope narrows candidate paths, never resolved targets (see
//     scope.go), so following a link would let one symlink carry a write into a
//     directory config.sourceRefs.exclude deliberately shut out.
//
// Reading is deliberately not held to this rule: readSourceFile follows a
// symlink to a regular file on purpose (decision 01KYSG9QEE36VPSR020WV81JWW).
// The asymmetry is the point — reading across the boundary changes nothing,
// writing across it destroys something.
type writeDecision int

const (
	// writeDirect: the candidate path is not a symlink. Write it.
	writeDirect writeDecision = iota
	// writeDeferred: the candidate path is a symlink that resolves to a path
	// which is itself a candidate of this same run. The target is rewritten on
	// its own turn, so declining here costs nothing — the ids get fixed and the
	// link survives. Order does not matter: whichever of the two is visited
	// first does the rewrite, and the other then finds no matches at all.
	writeDeferred
	// writeRefused: the candidate path is a symlink whose resolved target is
	// not a candidate of this run (outside root, .gitignore'd, or cut by
	// config.sourceRefs). Nothing is written, so the old ids stay — counted as
	// a failed rewrite, which is the third outcome decision
	// 01KYSG9QEE36VPSR020WV81JWW already reserves for "read but could not be
	// written back". Rerunning converges once the user edits the target by hand
	// or widens the scan scope.
	writeRefused
)

// linkState is what Execute learned about a write target before writing.
// Splitting it out keeps classifyWrite a pure decision over stated facts,
// checkable by input/output pairs without a filesystem.
type linkState struct {
	// symlink is whether the candidate path itself (not its target) is a symlink.
	symlink bool
	// targetRel is the fully-resolved target as a root-relative, "/"-separated
	// path — empty when the link resolves outside root, is broken, or could not
	// be resolved at all.
	targetRel string
	// shown is the target as reported to the user: the literal link text, which
	// is what they wrote and can act on.
	shown string
}

// classifyWrite decides whether an apply run may write back to a candidate,
// given what inspectLink found about the path and the set of candidate paths
// this run enumerated (root-relative, as EnumerateFiles returns them).
func classifyWrite(st linkState, candidates map[string]bool) writeDecision {
	if !st.symlink {
		return writeDirect
	}
	if st.targetRel != "" && candidates[st.targetRel] {
		return writeDeferred
	}
	return writeRefused
}

// inspectLink answers, for one candidate path, the questions classifyWrite
// needs: is this path a symlink, and where does it fully resolve to.
//
// It resolves the whole chain (EvalSymlinks), so a link to a link to a regular
// file is judged by its final target, not the next hop. root is resolved too,
// because the project root may itself sit under a symlinked directory
// (/tmp -> /private/tmp on macOS); without that, every candidate would look
// like it resolves outside root.
//
// A path whose Lstat fails is reported as "not a symlink" and left to the write
// to fail on. That is a real limit, shared with any check of this shape: the
// mode is read before the write, so a path swapped for a symlink in between is
// not caught.
func inspectLink(root, relPath string) linkState {
	absPath := filepath.Join(root, filepath.FromSlash(relPath))
	info, err := os.Lstat(absPath)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return linkState{}
	}
	st := linkState{symlink: true, shown: relPath}
	if target, err := os.Readlink(absPath); err == nil {
		st.shown = filepath.ToSlash(target)
	}
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return st
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		realRoot = root
	}
	rel, err := filepath.Rel(realRoot, resolved)
	if err != nil {
		return st
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return st
	}
	st.targetRel = rel
	return st
}

// writeFileAtomic writes data to path via a temp file in the same
// directory followed by rename, mirroring store.writeJSONAtomic's
// tmp-then-rename convention for the plain-text case, and preserving the
// original file's permissions.
//
// Callers must have cleared path through classifyWrite first: rename replaces
// whatever name it is given, so handing this a symlink destroys the link.
//
// Hard links are outside what this preserves, and knowingly so. Rename gives
// the new content a new inode, so other names for the old inode keep the old
// content. Holding them together would mean writing in place (open + truncate),
// trading away the atomicity this function exists for — a regression on a
// different axis. Unlike a clobbered symlink the loss is also recoverable in
// practice: every hard-linked name still exists, and each one that is itself a
// candidate is rewritten on its own turn, so only the sharing is lost, not the
// content.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".refs-rewrite-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if info, statErr := os.Stat(path); statErr == nil {
		_ = os.Chmod(tmpPath, info.Mode())
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}
