package index

import (
	"sort"
	"strconv"
	"strings"

	"github.com/nkenji09/scholia/internal/model"
)

// Record types for cross-record search (#45 D10b-3). Mirrors the CLI's
// --type values / display order.
const (
	RecordTag        = "tag"
	RecordTransition = "transition"
	RecordVocab      = "vocab"
	RecordDecision   = "decision"
)

// recordTypeOrder is the fixed display / sort order for record matches
// (spec req.evaluate-change.discovery: 型別 tag/transition/vocab/decision).
var recordTypeOrder = []string{RecordTag, RecordTransition, RecordVocab, RecordDecision}

// RecordMatch is one hit from the unified search core (#45 D10b-3): which
// record type, which record, which field matched, and a one-line snippet
// around the match. This is the *single* match unit both `scholia search`
// (CLI) and GET /api/search (viewer) derive their output from — the viewer's
// transition-grouped shape (SearchResult below) is a per-surface display
// transform over these matches, not a second corpus (面間整合原則 D10b-2).
type RecordMatch struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Field   string `json:"field"`
	Snippet string `json:"snippet"`
}

// recordField is one substring-matchable (type, id, field, raw text) tuple in
// the corpus. keywords are tested case-insensitively against Text.
type recordField struct {
	Type  string
	ID    string
	Field string
	Text  string
}

// SearchRecords is the unified cross-record search core (#45 D10b-3 · kit
// §2.2). It scans the corpus (tag/vocab/transition/decision — the union of the
// two former systems) for any keyword (case-insensitive substring, OR across
// keywords) and returns match-level results, filtered to types (nil/empty =
// all). Empty/blank keywords match nothing. Results are sorted by
// type-order → id → field for stable output. Both CLI `scholia search` and the
// viewer's GET /api/search delegate here so the two surfaces answer the same
// query the same way.
func SearchRecords(ix *Index, keywords, types []string) []RecordMatch {
	wanted := wantedTypes(types)

	// Normalize keywords: trim, drop empties, lowercase once.
	kws := make([]string, 0, len(keywords))
	for _, k := range keywords {
		if k = strings.TrimSpace(k); k != "" {
			kws = append(kws, strings.ToLower(k))
		}
	}
	if len(kws) == 0 {
		return []RecordMatch{}
	}

	var matches []RecordMatch
	seen := make(map[string]bool) // dedupe key: type|id|field
	for _, f := range searchCorpus(ix, wanted) {
		if f.Text == "" {
			continue
		}
		key := f.Type + "|" + f.ID + "|" + f.Field
		if seen[key] {
			continue
		}
		lower := strings.ToLower(f.Text)
		for _, kw := range kws {
			if strings.Contains(lower, kw) {
				seen[key] = true
				matches = append(matches, RecordMatch{Type: f.Type, ID: f.ID, Field: f.Field, Snippet: Snippet(f.Text, kw)})
				break
			}
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Type != matches[j].Type {
			return recordTypeRank(matches[i].Type) < recordTypeRank(matches[j].Type)
		}
		if matches[i].ID != matches[j].ID {
			return matches[i].ID < matches[j].ID
		}
		return matches[i].Field < matches[j].Field
	})
	if matches == nil {
		return []RecordMatch{}
	}
	return matches
}

func wantedTypes(types []string) map[string]bool {
	wanted := make(map[string]bool, len(recordTypeOrder))
	if len(types) == 0 {
		for _, t := range recordTypeOrder {
			wanted[t] = true
		}
		return wanted
	}
	for _, t := range types {
		wanted[t] = true
	}
	return wanted
}

func recordTypeRank(t string) int {
	for i, want := range recordTypeOrder {
		if t == want {
			return i
		}
	}
	return len(recordTypeOrder)
}

// searchCorpus assembles the full corpus (#45 D10b-3 · kit §2.2), the union of
// the two former search systems:
//   - tag: id, name, description
//   - vocab: id, label, description, altLabels
//   - transition: own id, effective tag id/name (§3.7 ancestor closure),
//     action/given/then vocab id/label, action kind name
//   - decision: why, changed, target.id
//
// Derived once here so both live search and the static corpus bake share the
// exact same fields (§9 single source of truth).
func searchCorpus(ix *Index, wanted map[string]bool) []recordField {
	var out []recordField

	if wanted[RecordTag] {
		for _, tag := range sortedTags(ix) {
			out = append(out,
				recordField{RecordTag, tag.ID, "id", tag.ID},
				recordField{RecordTag, tag.ID, "name", tag.Name},
				recordField{RecordTag, tag.ID, "description", tag.Description},
			)
		}
	}

	if wanted[RecordTransition] {
		for _, t := range ix.AllTransitions() {
			for _, c := range transitionCandidates(ix, t) {
				out = append(out, recordField{RecordTransition, t.ID, c.Label, c.Text})
			}
		}
	}

	if wanted[RecordVocab] {
		for _, v := range sortedVocab(ix) {
			out = append(out,
				recordField{RecordVocab, v.ID, "id", v.ID},
				recordField{RecordVocab, v.ID, "label", v.Label},
				recordField{RecordVocab, v.ID, "description", v.Description},
			)
			for i, al := range v.AltLabels {
				out = append(out, recordField{RecordVocab, v.ID, altLabelField(i), al})
			}
		}
	}

	if wanted[RecordDecision] {
		for _, d := range ix.Decisions {
			out = append(out,
				recordField{RecordDecision, d.ID, "why", d.Why},
				recordField{RecordDecision, d.ID, "changed", d.Changed},
				recordField{RecordDecision, d.ID, "target", d.Target.ID},
			)
		}
	}

	return out
}

func altLabelField(i int) string {
	return "altLabel:" + strconv.Itoa(i)
}

// Snippet builds a one-line excerpt centered on the first (case-insensitive)
// occurrence of kw in text, ~20 runes of context each side, ellipsized. Rune-
// based so multibyte text (Japanese) is not split mid-character. Moved here
// from cli/search.go so CLI and any future match-level surface share it.
func Snippet(text, kw string) string {
	oneline := strings.Join(strings.Fields(text), " ")
	lower := strings.ToLower(oneline)
	byteIdx := strings.Index(lower, strings.ToLower(kw))
	if byteIdx < 0 {
		return truncateOneLine(oneline, 80)
	}
	runeIdx := len([]rune(lower[:byteIdx]))
	kwRuneLen := len([]rune(kw))
	runes := []rune(oneline)

	const context = 20
	start := runeIdx - context
	if start < 0 {
		start = 0
	}
	end := runeIdx + kwRuneLen + context
	if end > len(runes) {
		end = len(runes)
	}

	s := string(runes[start:end])
	if start > 0 {
		s = "…" + s
	}
	if end < len(runes) {
		s = s + "…"
	}
	return s
}

// truncateOneLine caps s to maxRunes runes, appending "…" when it truncates.
func truncateOneLine(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}

func sortedTags(ix *Index) []model.Tag {
	out := make([]model.Tag, 0, len(ix.TagByID))
	for _, t := range ix.TagByID {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedVocab(ix *Index) []model.VocabEntry {
	out := make([]model.VocabEntry, 0, len(ix.VocabByID))
	for _, v := range ix.VocabByID {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// SearchResult is the transition-grouped view GET /api/search returns for the
// existing viewer search UI (transitions plus, per transition, which fields
// matched). Kept additively alongside RecordMatch (#45 D10b-3 keeps the
// /api/search envelope backward-compatible).
type SearchResult struct {
	Transitions []model.Transition  `json:"transitions"`
	MatchedOn   map[string][]string `json:"matchedOn"`
}

// Search finds transitions whose corpus (effective tag id/name, action/given/
// then vocab id/label, own id, action kind name) contains query — the
// transition-only, transition-grouped view for the existing viewer UI. It is
// now a thin projection of the unified core (SearchRecords over just the
// transition type), so it and `scholia search` never diverge on which
// transitions match a query. An empty/blank query matches nothing. Results and
// matchedOn are always non-nil.
func Search(ix *Index, query string) SearchResult {
	result := SearchResult{Transitions: []model.Transition{}, MatchedOn: map[string][]string{}}
	matches := SearchRecords(ix, []string{query}, []string{RecordTransition})
	if len(matches) == 0 {
		return result
	}
	fieldsByTx := make(map[string][]string)
	seen := make(map[string]bool)
	for _, m := range matches {
		key := m.ID + "|" + m.Field
		if seen[key] {
			continue
		}
		seen[key] = true
		if _, ok := fieldsByTx[m.ID]; !ok {
			result.Transitions = append(result.Transitions, ix.TransitionByID[m.ID])
		}
		fieldsByTx[m.ID] = append(fieldsByTx[m.ID], m.Field)
	}
	// Transitions come back id-sorted from SearchRecords already; keep them so.
	sort.Slice(result.Transitions, func(i, j int) bool { return result.Transitions[i].ID < result.Transitions[j].ID })
	result.MatchedOn = fieldsByTx
	return result
}

// SearchCandidate is one substring-matchable (label, text) pair for a
// transition's baked search corpus (static export).
type SearchCandidate struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

// RecordSearchDoc is one record's full baked search corpus (static export).
// A static export bakes these so a JS client re-runs only the substring test
// per query; the corpus derivation (Go) stays the single source of truth. The
// type/id identify the record; candidates are its (field-label, lowercased
// text) pairs. Extends the former transition-only corpus to all four types
// (#45 D10b-3).
type RecordSearchDoc struct {
	Type       string            `json:"type"`
	ID         string            `json:"id"`
	Candidates []SearchCandidate `json:"candidates"`
}

// SearchCorpus returns every record's search candidates across all four types,
// derived from the same corpus SearchRecords scans (#45 D10b-3). Text is
// lowercased so the client's substring test matches the Go path exactly.
func SearchCorpus(ix *Index) []RecordSearchDoc {
	wanted := wantedTypes(nil)
	// Group corpus fields by (type,id) into candidate lists, preserving the
	// per-type record order searchCorpus already emits.
	type key struct{ typ, id string }
	order := make([]key, 0)
	byKey := make(map[key][]SearchCandidate)
	for _, f := range searchCorpus(ix, wanted) {
		if f.Text == "" {
			continue
		}
		k := key{f.Type, f.ID}
		if _, ok := byKey[k]; !ok {
			order = append(order, k)
		}
		byKey[k] = append(byKey[k], SearchCandidate{Label: f.Field, Text: strings.ToLower(f.Text)})
	}
	docs := make([]RecordSearchDoc, 0, len(order))
	for _, k := range order {
		docs = append(docs, RecordSearchDoc{Type: k.typ, ID: k.id, Candidates: byKey[k]})
	}
	return docs
}

// transitionCandidates builds transition t's corpus fields (own id; effective
// tag id/name; action/given/then vocab id/label; action kind name). Field
// labels mirror the former searchCandidates so the viewer's existing matchedOn
// labels (id / tag:<id> / vocab:<id> / kind:<kind>) are unchanged.
func transitionCandidates(ix *Index, t model.Transition) []SearchCandidate {
	var out []SearchCandidate
	out = append(out, SearchCandidate{Label: "id", Text: t.ID})
	for _, tagID := range ix.EffectiveTags[t.ID] {
		tag := ix.TagByID[tagID]
		out = append(out,
			SearchCandidate{Label: "tag:" + tag.ID, Text: tag.ID},
			SearchCandidate{Label: "tag:" + tag.ID, Text: tag.Name},
		)
	}
	for _, ref := range vocabRefs(t) {
		v := ix.VocabByID[ref]
		out = append(out,
			SearchCandidate{Label: "vocab:" + v.ID, Text: v.ID},
			SearchCandidate{Label: "vocab:" + v.ID, Text: v.Label},
		)
		if len(v.AltLabels) > 0 {
			out = append(out, SearchCandidate{Label: "vocab:" + v.ID, Text: strings.Join(v.AltLabels, " ")})
		}
	}
	if kind := ix.VocabByID[t.Action].Kind; kind != "" {
		out = append(out, SearchCandidate{Label: "kind:" + kind, Text: kind})
	}
	return out
}

// vocabRefs lists the vocab ids a transition references (action/given/then).
func vocabRefs(t model.Transition) []string {
	refs := make([]string, 0, 1+len(t.Given)+len(t.Then))
	refs = append(refs, t.Action)
	refs = append(refs, t.Given...)
	refs = append(refs, t.Then...)
	return refs
}
