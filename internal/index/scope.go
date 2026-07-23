package index

import (
	"sort"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// TagScope answers "does record R belong to the subtree of any of these tags?"
// using the same effective-tag (§3.7 ancestor expansion) containment that
// `list --tag` / `spec` / `rules --tag` use, generalized across all four record
// types. `scholia search --tag` filters its concept hits through this so a
// keyword reverse-lookup can be narrowed to one component/subject subtree — and
// so that, for transitions, the membership matches exactly what `list --tag`
// would show (面間整合原則: 利用者の期待が list と揃う, issue #1).
//
// Membership per record type, for a scope tag X (OR across the given tags):
//   - tag:        the tag is X or a descendant of X (X is in its ancestor
//     closure) — the tag lives in X's subtree.
//   - transition: X is one of its effective tags (HasEffectiveTag) — identical
//     to `list --tag X`.
//   - vocab:      it is referenced (action/given/then) by a transition in X's
//     subtree (VocabBySubject — scholia attributes vocab to a
//     component through its transitions, not a direct tag), or it
//     directly carries a tag in X's subtree (VocabEntry.Tags).
//   - decision:   its target record is in scope by the rule for that target's
//     type (tag / transition / vocab above).
//
// Existence of the scope tags is the caller's concern; a tag that matches no
// records simply contributes no members.
type TagScope struct {
	ix      *Index
	snap    *store.Snapshot
	scope   []string
	tagMemo map[string]bool // tagID -> in subtree of some scope tag
	vocabIn map[string]bool // vocabID -> in scope (seeded via transitions)
	decByID map[string]model.Decision
}

// NewTagScope precomputes what it can (vocab reached via in-scope transitions,
// a decision-by-id map) so filtering many matches stays cheap.
func NewTagScope(ix *Index, snap *store.Snapshot, scopeTags []string) *TagScope {
	s := &TagScope{
		ix:      ix,
		snap:    snap,
		scope:   scopeTags,
		tagMemo: make(map[string]bool),
		vocabIn: make(map[string]bool),
		decByID: make(map[string]model.Decision, len(ix.Decisions)),
	}
	for _, d := range ix.Decisions {
		s.decByID[d.ID] = d
	}
	// Component vocab: every vocab referenced by a transition in the subtree of a
	// scope tag (VocabBySubject rolls transitions up through effective tags).
	for _, x := range scopeTags {
		for _, v := range ix.VocabBySubject(x) {
			s.vocabIn[v.ID] = true
		}
	}
	return s
}

// tagInScope reports whether tagID is X or a descendant of X for some scope tag
// X — i.e. X is in tagID's ancestor closure (§3.7). Memoized.
func (s *TagScope) tagInScope(tagID string) bool {
	if v, ok := s.tagMemo[tagID]; ok {
		return v
	}
	anc := make(map[string]bool)
	for _, a := range TagAncestors(s.snap, tagID) {
		anc[a] = true
	}
	in := false
	for _, x := range s.scope {
		if anc[x] {
			in = true
			break
		}
	}
	s.tagMemo[tagID] = in
	return in
}

// transitionInScope reports whether some scope tag is an effective tag of txID
// — identical to `list --tag`.
func (s *TagScope) transitionInScope(txID string) bool {
	for _, x := range s.scope {
		if s.ix.HasEffectiveTag(txID, x) {
			return true
		}
	}
	return false
}

// vocabInScope is the seeded via-transition set (NewTagScope) plus the direct
// VocabEntry.Tags path (a vocab that itself carries a tag in the subtree).
func (s *TagScope) vocabInScope(vocabID string) bool {
	if s.vocabIn[vocabID] {
		return true
	}
	v, ok := s.ix.VocabByID[vocabID]
	if !ok {
		return false
	}
	for _, t := range v.Tags {
		if s.tagInScope(t) {
			s.vocabIn[vocabID] = true
			return true
		}
	}
	return false
}

// Includes reports whether the (type,id) record is in scope.
func (s *TagScope) Includes(typ, id string) bool {
	switch typ {
	case RecordTag:
		return s.tagInScope(id)
	case RecordTransition:
		return s.transitionInScope(id)
	case RecordVocab:
		return s.vocabInScope(id)
	case RecordDecision:
		d, ok := s.decByID[id]
		if !ok {
			return false
		}
		switch d.Target.Type {
		case model.DecisionTargetTag:
			return s.tagInScope(d.Target.ID)
		case model.DecisionTargetTransition:
			return s.transitionInScope(d.Target.ID)
		case model.DecisionTargetVocab:
			return s.vocabInScope(d.Target.ID)
		}
	}
	return false
}

// OwningSubjects returns the subject-kind (config ownerKind) tag ids that own
// the record (typ,id) — the scope candidates a user could pass to `search
// --tag` to include this record. It is the discovery counterpart of TagScope
// (案 B): TagScope answers "is R under tag X?", OwningSubjects answers "which
// subjects is R under?", both from the same attribution:
//   - tag:        its own subject-kind ancestors (or itself if it is a subject).
//   - transition: its subject-kind effective tags (§3.7 ancestor closure).
//   - vocab:      the subjects of the transitions that reference it
//     (action/given/then) ∪ subject-kind ancestors of its direct
//     tags — the same via-transition attribution TagScope uses.
//   - decision:   the owning subjects of its target record.
//
// ownerKind empty (unwired config) yields no subjects. Result is sorted and
// never nil.
func OwningSubjects(ix *Index, snap *store.Snapshot, ownerKind, typ, id string) []string {
	set := make(map[string]bool)
	if ownerKind != "" {
		collectOwningSubjects(ix, snap, ownerKind, typ, id, set)
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func collectOwningSubjects(ix *Index, snap *store.Snapshot, ownerKind, typ, id string, set map[string]bool) {
	addAncestorSubjects := func(tagID string) {
		for _, a := range TagAncestors(snap, tagID) {
			if ix.TagByID[a].Kind == ownerKind {
				set[a] = true
			}
		}
	}
	addEffectiveSubjects := func(txID string) {
		for _, et := range ix.EffectiveTags[txID] {
			if ix.TagByID[et].Kind == ownerKind {
				set[et] = true
			}
		}
	}
	switch typ {
	case RecordTag:
		addAncestorSubjects(id)
	case RecordTransition:
		addEffectiveSubjects(id)
	case RecordVocab:
		for _, t := range ix.TransitionsByVocab(id) {
			addEffectiveSubjects(t.ID)
		}
		if v, ok := ix.VocabByID[id]; ok {
			for _, t := range v.Tags {
				addAncestorSubjects(t)
			}
		}
	case RecordDecision:
		for _, d := range ix.Decisions {
			if d.ID == id {
				// Decision target types share the Record* string values.
				collectOwningSubjects(ix, snap, ownerKind, d.Target.Type, d.Target.ID, set)
				return
			}
		}
	}
}

// FilterMatchesByTags keeps only matches whose record is in the subtree of any
// scopeTag (TagScope membership) — the concept-AND-scope narrowing `scholia
// search --tag` applies on top of the keyword (OR) and --type (AND) filters.
// Empty scopeTags is a no-op passthrough. The result is never nil.
func FilterMatchesByTags(ix *Index, snap *store.Snapshot, matches []RecordMatch, scopeTags []string) []RecordMatch {
	if len(scopeTags) == 0 {
		if matches == nil {
			return []RecordMatch{}
		}
		return matches
	}
	scope := NewTagScope(ix, snap, scopeTags)
	out := make([]RecordMatch, 0, len(matches))
	for _, m := range matches {
		if scope.Includes(m.Type, m.ID) {
			out = append(out, m)
		}
	}
	return out
}
