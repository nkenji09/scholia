package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nkenji09/scholia/internal/model"
)

// VocabRenameResult summarizes a `scholia vocab rename` (§6).
type VocabRenameResult struct {
	OldID              string   `json:"oldId"`
	NewID              string   `json:"newId"`
	UpdatedTransitions []string `json:"updatedTransitions"`
}

// RenameVocab renames a vocab entry's file and id, then rewrites every
// transition's action/given/then reference to it (§6). Vocab entries are not
// referenced by tags or decisions, so transitions are the only other record
// kind that needs updating.
func (s *Store) RenameVocab(oldID, newID string) (VocabRenameResult, error) {
	if newID == "" {
		return VocabRenameResult{}, fmt.Errorf("newId は必須です")
	}
	if oldID == newID {
		return VocabRenameResult{}, fmt.Errorf("newId %q は oldId と同じです", newID)
	}
	if !s.VocabExists(oldID) {
		return VocabRenameResult{}, fmt.Errorf("vocab %q が見つかりません", oldID)
	}
	if s.VocabExists(newID) {
		return VocabRenameResult{}, fmt.Errorf("vocab %q は既に存在します", newID)
	}

	v, err := s.LoadVocab(oldID)
	if err != nil {
		return VocabRenameResult{}, err
	}
	v.ID = newID
	if err := s.SaveVocab(v); err != nil {
		return VocabRenameResult{}, err
	}

	snap, err := s.LoadAll()
	if err != nil {
		return VocabRenameResult{}, err
	}
	var updated []string
	for _, t := range snap.Transitions {
		if t.ID == "" {
			continue
		}
		changed := false
		if t.Action == oldID {
			t.Action = newID
			changed = true
		}
		for i, g := range t.Given {
			if g == oldID {
				t.Given[i] = newID
				changed = true
			}
		}
		for i, e := range t.Then {
			if e == oldID {
				t.Then[i] = newID
				changed = true
			}
		}
		if !changed {
			continue
		}
		if err := s.SaveTransition(t); err != nil {
			return VocabRenameResult{}, err
		}
		updated = append(updated, t.ID)
	}

	if err := os.Remove(s.vocabPath(oldID)); err != nil {
		return VocabRenameResult{}, err
	}
	sort.Strings(updated)
	return VocabRenameResult{OldID: oldID, NewID: newID, UpdatedTransitions: updated}, nil
}

// TxRenameResult summarizes a `scholia tx rename` (§6).
type TxRenameResult struct {
	OldID            string   `json:"oldId"`
	NewID            string   `json:"newId"`
	UpdatedDecisions []string `json:"updatedDecisions"`
}

// RenameTransition renames a transition's file and id, then rewrites every
// decision whose target points at it (§6 note). Transitions have no
// incoming edges from other transitions/vocab/tags (§2 — no edges), so
// decisions are the only other record kind that references a transition id.
func (s *Store) RenameTransition(oldID, newID string) (TxRenameResult, error) {
	if newID == "" {
		return TxRenameResult{}, fmt.Errorf("newId は必須です")
	}
	if oldID == newID {
		return TxRenameResult{}, fmt.Errorf("newId %q は oldId と同じです", newID)
	}
	if !s.TransitionExists(oldID) {
		return TxRenameResult{}, fmt.Errorf("transition %q が見つかりません", oldID)
	}
	if s.TransitionExists(newID) {
		return TxRenameResult{}, fmt.Errorf("transition %q は既に存在します", newID)
	}

	t, err := s.LoadTransition(oldID)
	if err != nil {
		return TxRenameResult{}, err
	}
	t.ID = newID
	if err := s.SaveTransition(t); err != nil {
		return TxRenameResult{}, err
	}

	snap, err := s.LoadAll()
	if err != nil {
		return TxRenameResult{}, err
	}
	var updated []string
	for _, d := range snap.Decisions {
		if d.Target.Type != model.DecisionTargetTransition || d.Target.ID != oldID {
			continue
		}
		d.Target.ID = newID
		if err := s.SaveDecision(d); err != nil {
			return TxRenameResult{}, err
		}
		updated = append(updated, d.ID)
	}

	if err := os.Remove(s.transitionPath(oldID)); err != nil {
		return TxRenameResult{}, err
	}
	sort.Strings(updated)
	return TxRenameResult{OldID: oldID, NewID: newID, UpdatedDecisions: updated}, nil
}

// TagRenameResult summarizes a `scholia tag rename` (T-tag-rename /
// T-tag-rename-cascade). RenamedTags maps every old tag id to its new id: the
// primary rename always, plus (with --cascade) each descendant whose id prefix
// changed. The Updated* lists name the records whose tag-id references were
// repointed (the four reference sites the decision on req.record-maintenance
// pins down: other tags' parentIds / transitions' tags / vocab's tags /
// decisions' tag target).
type TagRenameResult struct {
	OldID              string            `json:"oldId"`
	NewID              string            `json:"newId"`
	Cascade            bool              `json:"cascade"`
	RenamedTags        map[string]string `json:"renamedTags"`
	UpdatedTags        []string          `json:"updatedTags,omitempty"`
	UpdatedTransitions []string          `json:"updatedTransitions,omitempty"`
	UpdatedVocab       []string          `json:"updatedVocab,omitempty"`
	UpdatedDecisions   []string          `json:"updatedDecisions,omitempty"`
}

// RenameTag renames tag oldID to newID and repoints every reference to it,
// atomically (T-tag-rename). With cascade it also renames the whole parentIds
// subtree rooted at oldID by prefix-substituting oldID→newID in each
// descendant's id (T-tag-rename-cascade). It never mutates any field other
// than the ids being relabeled — name/kind/desc/color/ref and unrelated
// parentIds entries survive verbatim.
//
// Invariants (decision on req.record-maintenance):
//   - referential integrity: all four tag-id reference sites are rewritten so
//     `scholia lint`'s tag-ref / decision-target rules stay green (no dangling id).
//   - atomicity: every new id (including cascade-generated ones) is computed
//     and collision-checked up front; the write phase snapshots every file it
//     touches and rolls the whole set back on any mid-flight error, so a
//     failure never leaves a half-applied rename.
//   - case-only safety: a rename that differs from the source only in case
//     (e.g. abc→Abc) would, on a case-insensitive filesystem (macOS default),
//     alias the same file — writing the new name then removing the old would
//     delete the record. Every tag-file rename therefore goes through a
//     distinct intermediate temp file (write temp → remove old → move temp
//     into place), which is safe on both case-insensitive and case-sensitive
//     filesystems and also makes id permutations within a cascade safe.
//
// A rename relabels graph nodes 1:1 and repoints parentIds edges verbatim, so
// it cannot introduce a parentIds cycle that did not already exist — there is
// no separate cycle-rejection path for it (unlike `tag create`, which can add
// a new edge).
func (s *Store) RenameTag(oldID, newID string, cascade bool) (TagRenameResult, error) {
	if newID == "" {
		return TagRenameResult{}, fmt.Errorf("newId は必須です")
	}
	if oldID == newID {
		return TagRenameResult{}, fmt.Errorf("newId %q は oldId と同じです", newID)
	}
	if !s.TagExists(oldID) {
		return TagRenameResult{}, fmt.Errorf("tag %q が見つかりません", oldID)
	}

	snap, err := s.LoadAll()
	if err != nil {
		return TagRenameResult{}, err
	}

	// 1. Rename plan: old tag id -> new tag id (primary + cascade subtree).
	plan := buildTagRenamePlan(snap.Tags, oldID, newID, cascade)

	// 2. Collision check up front (before any write): reject if any new id
	//    lands on an existing tag that is not itself being renamed away, or if
	//    two source ids map to the same new id (T-tag-rename-collision-rejected).
	if err := s.validateTagRenamePlan(plan); err != nil {
		return TagRenameResult{}, err
	}

	mapID := func(id string) string {
		if n, ok := plan[id]; ok {
			return n
		}
		return id
	}
	// mapList returns ids with plan substitutions applied, preserving order,
	// and whether anything changed.
	mapList := func(ids []string) ([]string, bool) {
		if len(ids) == 0 {
			return ids, false
		}
		changed := false
		out := make([]string, len(ids))
		for i, id := range ids {
			out[i] = mapID(id)
			if out[i] != id {
				changed = true
			}
		}
		return out, changed
	}

	// 3. Compute the desired final content of every affected record.
	//    Renamed tag files are handled separately (they move on disk); here we
	//    collect the pure in-place ref rewrites plus the new tag contents.
	var (
		result = TagRenameResult{OldID: oldID, NewID: newID, Cascade: cascade, RenamedTags: plan}

		inPlaceTags []model.Tag // non-renamed tags whose parentIds changed
		renamedTags []model.Tag // renamed tags (new id + rewritten parentIds)
		updatedTx   []model.Transition
		updatedVoc  []model.VocabEntry
		updatedDec  []model.Decision
	)

	for _, t := range snap.Tags {
		newParents, parentsChanged := mapList(t.ParentIDs)
		if _, isRenamed := plan[t.ID]; isRenamed {
			t.ID = plan[t.ID]
			t.ParentIDs = newParents
			renamedTags = append(renamedTags, t)
			continue
		}
		if parentsChanged {
			t.ParentIDs = newParents
			inPlaceTags = append(inPlaceTags, t)
			result.UpdatedTags = append(result.UpdatedTags, t.ID)
		}
	}
	for _, t := range snap.Transitions {
		if newTags, changed := mapList(t.Tags); changed {
			t.Tags = newTags
			updatedTx = append(updatedTx, t)
			result.UpdatedTransitions = append(result.UpdatedTransitions, t.ID)
		}
	}
	for _, v := range snap.Vocab {
		if newTags, changed := mapList(v.Tags); changed {
			v.Tags = newTags
			updatedVoc = append(updatedVoc, v)
			result.UpdatedVocab = append(result.UpdatedVocab, v.ID)
		}
	}
	for _, d := range snap.Decisions {
		if d.Target.Type == model.DecisionTargetTag {
			if n := mapID(d.Target.ID); n != d.Target.ID {
				d.Target.ID = n
				updatedDec = append(updatedDec, d)
				result.UpdatedDecisions = append(result.UpdatedDecisions, d.ID)
			}
		}
	}
	sort.Strings(result.UpdatedTags)
	sort.Strings(result.UpdatedTransitions)
	sort.Strings(result.UpdatedVocab)
	sort.Strings(result.UpdatedDecisions)
	sort.Slice(renamedTags, func(i, j int) bool { return renamedTags[i].ID < renamedTags[j].ID })

	// 4. Apply everything, rolling the whole file set back on any error.
	tx := newFileTxn()
	apply := func() error {
		for _, t := range inPlaceTags {
			if err := tx.saveTag(s, t); err != nil {
				return err
			}
		}
		for _, t := range updatedTx {
			if err := tx.saveTransition(s, t); err != nil {
				return err
			}
		}
		for _, v := range updatedVoc {
			if err := tx.saveVocab(s, v); err != nil {
				return err
			}
		}
		for _, d := range updatedDec {
			if err := tx.saveDecision(s, d); err != nil {
				return err
			}
		}
		return tx.renameTagFiles(s, plan, renamedTags)
	}
	if err := apply(); err != nil {
		tx.rollback()
		return TagRenameResult{}, fmt.Errorf("tag rename の適用に失敗しました（変更はロールバックされました）: %w", err)
	}
	return result, nil
}

// buildTagRenamePlan computes the old→new id map. Without cascade it is just
// {oldID: newID}. With cascade it adds every true descendant (via parentIds)
// of oldID whose id carries oldID as a boundary-delimited prefix, mapping each
// to the same id with that prefix substituted to newID (so a multi-level
// subtree is relabeled in one command). Descendants that don't carry the
// prefix keep their id — only their parentIds get repointed by the caller.
func buildTagRenamePlan(tags []model.Tag, oldID, newID string, cascade bool) map[string]string {
	plan := map[string]string{oldID: newID}
	if !cascade {
		return plan
	}
	childrenOf := make(map[string][]string, len(tags))
	for _, t := range tags {
		for _, p := range t.ParentIDs {
			childrenOf[p] = append(childrenOf[p], t.ID)
		}
	}
	seen := map[string]bool{oldID: true}
	queue := []string{oldID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, ch := range childrenOf[cur] {
			if seen[ch] {
				continue
			}
			seen[ch] = true
			queue = append(queue, ch)
		}
	}
	for id := range seen {
		if id == oldID {
			continue
		}
		if nid, ok := prefixSubstitute(id, oldID, newID); ok {
			plan[id] = nid
		}
	}
	return plan
}

// prefixSubstitute replaces a leading oldPrefix with newPrefix in id, but only
// at a delimiter boundary (id == oldPrefix, or oldPrefix followed by '-', '.'
// or '_'). The boundary guard keeps a cascade of `req.foo` from mangling a
// sibling descendant named `req.foobar` while still catching the real subtree
// (`req.foo-bar`, `req.foo.bar`).
func prefixSubstitute(id, oldPrefix, newPrefix string) (string, bool) {
	if id == oldPrefix {
		return newPrefix, true
	}
	if strings.HasPrefix(id, oldPrefix) && len(id) > len(oldPrefix) {
		switch id[len(oldPrefix)] {
		case '-', '.', '_':
			return newPrefix + id[len(oldPrefix):], true
		}
	}
	return id, false
}

// validateTagRenamePlan rejects a plan whose new ids would collide (§ "先に全
// 新 id を計算し衝突検査してから適用"). A new id collides when a tag file already
// exists at that id (TagExists is os.Stat-based, so it also catches
// case-insensitive collisions on macOS) and that id is not itself a source
// being renamed away, or when two sources map to the same new id. A pure
// case-flip (EqualFold(old,new)) is the rename target itself, not a collision.
func (s *Store) validateTagRenamePlan(plan map[string]string) error {
	oldSet := make(map[string]bool, len(plan))
	for old := range plan {
		oldSet[old] = true
	}
	seenNew := make(map[string]string, len(plan))
	var collisions []string
	// Deterministic iteration for stable error messages.
	olds := make([]string, 0, len(plan))
	for old := range plan {
		olds = append(olds, old)
	}
	sort.Strings(olds)
	for _, old := range olds {
		nw := plan[old]
		if prev, dup := seenNew[nw]; dup {
			return fmt.Errorf("rename 先 id %q が重複します（%q と %q の両方が同じ id に改名されます）", nw, prev, old)
		}
		seenNew[nw] = old
		if strings.EqualFold(old, nw) {
			continue // self case-flip: same file, not a collision
		}
		if s.TagExists(nw) && !oldSet[nw] {
			collisions = append(collisions, nw)
		}
	}
	if len(collisions) > 0 {
		sort.Strings(collisions)
		return fmt.Errorf("rename 先 id が既存タグと衝突します: %s", strings.Join(collisions, ", "))
	}
	return nil
}

// fileTxn snapshots the prior state of every path it writes/removes so the
// whole batch can be rolled back on error, giving RenameTag its all-or-nothing
// guarantee across the many files a cascade touches.
type fileTxn struct {
	orig   map[string][]byte // path -> original bytes (existed before)
	absent map[string]bool   // path -> did not exist before
}

func newFileTxn() *fileTxn {
	return &fileTxn{orig: map[string][]byte{}, absent: map[string]bool{}}
}

// track records path's pre-change state exactly once.
func (tx *fileTxn) track(path string) error {
	if _, ok := tx.orig[path]; ok {
		return nil
	}
	if tx.absent[path] {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			tx.absent[path] = true
			return nil
		}
		return err
	}
	tx.orig[path] = b
	return nil
}

func (tx *fileTxn) saveTag(s *Store, t model.Tag) error {
	if err := tx.track(s.tagPath(t.ID)); err != nil {
		return err
	}
	return s.SaveTag(t)
}

func (tx *fileTxn) saveTransition(s *Store, t model.Transition) error {
	if err := tx.track(s.transitionPath(t.ID)); err != nil {
		return err
	}
	return s.SaveTransition(t)
}

func (tx *fileTxn) saveVocab(s *Store, v model.VocabEntry) error {
	if err := tx.track(s.vocabPath(v.ID)); err != nil {
		return err
	}
	return s.SaveVocab(v)
}

func (tx *fileTxn) saveDecision(s *Store, d model.Decision) error {
	if err := tx.track(s.decisionPath(d.ID)); err != nil {
		return err
	}
	return s.SaveDecision(d)
}

// renameTagFiles moves each renamed tag's file to its new id in three passes —
// write every new content to a distinct temp file, remove every old file, then
// move each temp into its final name. Routing through temps (rather than
// write-new-then-remove-old) is what makes case-only renames and id
// permutations safe on a case-insensitive filesystem.
func (tx *fileTxn) renameTagFiles(s *Store, plan map[string]string, renamed []model.Tag) error {
	tagsDirPath := filepath.Join(s.Dir, tagsDir)
	temps := make([]string, len(renamed))
	for i, t := range renamed {
		tmp := filepath.Join(tagsDirPath, fmt.Sprintf(".rename-%d.tmp", i))
		temps[i] = tmp
		if err := tx.track(tmp); err != nil {
			return err
		}
		if err := writeJSONAtomic(tmp, t); err != nil {
			return err
		}
	}
	// Remove old files (sources) — the source ids are the plan's keys.
	oldIDs := make([]string, 0, len(plan))
	for old := range plan {
		oldIDs = append(oldIDs, old)
	}
	sort.Strings(oldIDs)
	for _, old := range oldIDs {
		p := s.tagPath(old)
		if err := tx.track(p); err != nil {
			return err
		}
		if err := os.Remove(p); err != nil {
			return err
		}
	}
	for i, t := range renamed {
		dst := s.tagPath(t.ID)
		if err := tx.track(dst); err != nil {
			return err
		}
		if err := os.Rename(temps[i], dst); err != nil {
			return err
		}
	}
	return nil
}

// rollback restores every tracked path to its pre-change state. Removals run
// before content restores so that a case-only alias (old and new ids naming
// the same file on a case-insensitive FS) ends up holding the original
// content under the original name rather than being deleted afterwards.
func (tx *fileTxn) rollback() {
	for p := range tx.absent {
		os.Remove(p)
	}
	for p, b := range tx.orig {
		os.WriteFile(p, b, 0o644)
	}
}
