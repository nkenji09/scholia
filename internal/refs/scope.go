package refs

import (
	"path/filepath"
	"strings"
)

// Options additively scopes Execute/ScanIDs's file discovery (the wiring
// for model.Config's SourceRefs field). The zero value narrows nothing —
// Scan empty means "whole project root" (no narrowing), Exclude empty
// means "no extra exclusion" — so callers that don't pass Options (or pass
// the zero value) see byte-identical behavior to before this field
// existed.
type Options struct {
	// Scan, if non-empty, limits files to those under any of these
	// project-root-relative path prefixes.
	Scan []string
	// Exclude removes files under any of these project-root-relative path
	// prefixes, in addition to the always-excluded directories
	// (.scholia/.git/_workspace/.concierge) and .gitignore.
	Exclude []string
}

// filterScope narrows files (root-relative, "/"-separated paths as
// EnumerateFiles returns them) to opts' scan/exclude scope. Matching is at
// a path-component boundary against a normalized prefix, so a scan entry
// of "app" matches "app" itself and everything under "app/", but not
// "apps/" or "app.txt" (a naive strings.HasPrefix would wrongly swallow
// both).
func filterScope(files []string, opts Options) []string {
	scan := normalizePrefixes(opts.Scan)
	exclude := normalizePrefixes(opts.Exclude)
	if len(scan) == 0 && len(exclude) == 0 {
		return files
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		if len(scan) > 0 && !underAnyPrefix(f, scan) {
			continue
		}
		if underAnyPrefix(f, exclude) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// normalizePrefixes converts user-supplied config paths (which may use OS
// separators or have leading "./"/"/") into the "/"-separated,
// no-leading/trailing-slash form comparable against EnumerateFiles' output.
func normalizePrefixes(prefixes []string) []string {
	out := make([]string, 0, len(prefixes))
	for _, p := range prefixes {
		p = filepath.ToSlash(p)
		p = strings.TrimPrefix(p, "./")
		p = strings.Trim(p, "/")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// underAnyPrefix reports whether relPath equals one of prefixes, or sits
// under one of them as a directory (boundary-safe: "app" does not match
// "apps/x", only "app" itself or "app/...").
func underAnyPrefix(relPath string, prefixes []string) bool {
	for _, p := range prefixes {
		if relPath == p || strings.HasPrefix(relPath, p+"/") {
			return true
		}
	}
	return false
}
