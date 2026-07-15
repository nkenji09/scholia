// Package refs scans and rewrites source-code references to pmem ids —
// separate from and outside `.pmem/` (§ handoff "rename × ソースコメント ID
// 参照"). It never touches `.pmem/`; the ids it operates on are supplied by
// callers (typically the store's rename plan).
package refs

import "bytes"

// isIDChar reports whether b belongs to the id-continuation character set
// shared by every pmem id ([A-Za-z0-9._-]) — the same set rename.go's
// prefixSubstitute treats as "still part of the id" when deciding cascade
// boundaries.
func isIDChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	case b == '.' || b == '-' || b == '_':
		return true
	}
	return false
}

// isDelim reports whether b is one of the three separator-class
// continuation chars ('.', '-', '_') as opposed to a letter/digit.
func isDelim(b byte) bool {
	return b == '.' || b == '-' || b == '_'
}

// findOccurrences returns the byte offsets in content where id occurs as a
// boundary-safe literal reference — not embedded inside a longer or
// different token.
//
// Boundary rule (non-negotiable):
//   - Left side never gets an exception: if the character immediately
//     before the match is an id-continuation char, the match is part of a
//     longer/different token (e.g. "xreq.foo" does not match "req.foo")
//     and is rejected outright.
//   - Right side allows exactly one exception: pure trailing punctuation.
//     If id is immediately followed by continuation characters, look at the
//     whole trailing run of continuation chars. If that run contains no
//     letter/digit anywhere (e.g. sentence-final "req.foo." or "req.foo...")
//     it's punctuation, not a longer id, and the match is accepted — only
//     the id's own span is reported, so callers leave the punctuation
//     untouched. If the run contains any letter/digit — whether directly
//     adjacent ("req.foobar") or after a delimiter ("req.foo-bar",
//     "req.foo.bar", the exact shape buildTagRenamePlan's cascade produces
//     for descendants) — the match is rejected: that shape is
//     indistinguishable from a real longer/different id, and rewriting it
//     would corrupt a sibling or an unrelated word. This favors precision
//     over recall by construction; residual, non-marker references that
//     don't fit this shape are left for the dry-run listing to surface, not
//     silently guessed at.
func findOccurrences(content []byte, id string) []int {
	if id == "" {
		return nil
	}
	needle := []byte(id)
	var out []int
	start := 0
	for {
		rel := bytes.Index(content[start:], needle)
		if rel < 0 {
			break
		}
		idx := start + rel
		end := idx + len(needle)

		if idx > 0 && isIDChar(content[idx-1]) {
			start = idx + 1
			continue
		}

		if end >= len(content) || !isIDChar(content[end]) {
			out = append(out, idx)
			start = end
			continue
		}

		j := end
		hasAlnum := false
		for j < len(content) && isIDChar(content[j]) {
			if !isDelim(content[j]) {
				hasAlnum = true
			}
			j++
		}
		if !hasAlnum {
			out = append(out, idx)
		}
		start = j
	}
	return out
}
