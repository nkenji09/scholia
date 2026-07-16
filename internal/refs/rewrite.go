package refs

import (
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

// FailedFile is a file Execute could not write back when apply is true.
// Source rewriting is best-effort: a write failure here does not unwind
// the `.scholia` rename that already committed. The file can be retried later
// via a fresh Execute/rewrite call, which is idempotent.
type FailedFile struct {
	Path string `json:"path"`
	Err  string `json:"err"`
}

// Report summarizes one Execute call.
type Report struct {
	Matches        []Match      `json:"matches,omitempty"`
	RewrittenFiles []string     `json:"rewrittenFiles,omitempty"`
	Failed         []FailedFile `json:"failed,omitempty"`
	Skipped        []SkipNote   `json:"skipped,omitempty"`
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
	sorted := make([]Pair, len(pairs))
	copy(sorted, pairs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].OldID < sorted[j].OldID })

	var report Report
	for _, relPath := range files {
		content, skip, err := readSourceFile(root, relPath)
		if err != nil {
			return Report{}, err
		}
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
			absPath := filepath.Join(root, filepath.FromSlash(relPath))
			if err := writeFileAtomic(absPath, content); err != nil {
				report.Failed = append(report.Failed, FailedFile{Path: relPath, Err: err.Error()})
				continue
			}
			report.RewrittenFiles = append(report.RewrittenFiles, relPath)
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

// writeFileAtomic writes data to path via a temp file in the same
// directory followed by rename, mirroring store.writeJSONAtomic's
// tmp-then-rename convention for the plain-text case, and preserving the
// original file's permissions.
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
