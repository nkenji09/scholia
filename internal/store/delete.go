package store

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/nkenji09/scholia/internal/model"
)

// RemoveVocabResult summarizes `scholia vocab rm` (§6).
type RemoveVocabResult struct {
	ID string `json:"id"`
}

// RemoveVocab deletes a vocab entry, refusing if any transition still
// references it via action/given/then (§6 "未参照限定" — symmetric with the
// write-time validation that keeps vocab-ref lint green).
func (s *Store) RemoveVocab(id string) (RemoveVocabResult, error) {
	if !s.VocabExists(id) {
		return RemoveVocabResult{}, fmt.Errorf("vocab %q が見つかりません", id)
	}
	snap, err := s.LoadAll()
	if err != nil {
		return RemoveVocabResult{}, err
	}

	var refs []string
	for _, t := range snap.Transitions {
		if t.Action == id || containsID(t.Given, id) || containsID(t.Then, id) {
			refs = append(refs, t.ID)
		}
	}
	if len(refs) > 0 {
		sort.Strings(refs)
		return RemoveVocabResult{}, fmt.Errorf(
			"vocab %q は %d 件の transition から参照されています（未参照になってから rm してください）: %s",
			id, len(refs), strings.Join(refs, ", "))
	}

	if err := os.Remove(s.vocabPath(id)); err != nil {
		return RemoveVocabResult{}, err
	}
	return RemoveVocabResult{ID: id}, nil
}

// TagRemoveResult summarizes `scholia tag rm` (§6).
type TagRemoveResult struct {
	ID                  string   `json:"id"`
	Forced              bool     `json:"forced"`
	DetachedTransitions []string `json:"detachedTransitions,omitempty"`
	DetachedVocab       []string `json:"detachedVocab,omitempty"`
	DetachedTags        []string `json:"detachedTags,omitempty"`
}

// RemoveTag deletes a tag. Without force it requires the tag to be
// unreferenced (transition.tags / vocab.tags / other tag.parentIds — §6).
// With force it detaches the tag from every referencing record first (§6
// "detach cascade"), then deletes it. Either way, a tag that is the target
// of a decision can never be removed: decisions are append-only, and
// deleting the tag they point at would break the decision-target lint rule
// (DESIGN に明記の無い実装判断・handoff 記載).
func (s *Store) RemoveTag(id string, force bool) (TagRemoveResult, error) {
	if !s.TagExists(id) {
		return TagRemoveResult{}, fmt.Errorf("tag %q が見つかりません", id)
	}
	snap, err := s.LoadAll()
	if err != nil {
		return TagRemoveResult{}, err
	}

	var decisionRefs []string
	for _, d := range snap.Decisions {
		if d.Target.Type == model.DecisionTargetTag && d.Target.ID == id {
			decisionRefs = append(decisionRefs, d.ID)
		}
	}
	if len(decisionRefs) > 0 {
		sort.Strings(decisionRefs)
		return TagRemoveResult{}, fmt.Errorf(
			"tag %q は %d 件の decision の対象です。decisions は append-only のため --force でも削除できません: %s",
			id, len(decisionRefs), strings.Join(decisionRefs, ", "))
	}

	var txRefs, vocabRefs, tagRefs []string
	for _, t := range snap.Transitions {
		if containsID(t.Tags, id) {
			txRefs = append(txRefs, t.ID)
		}
	}
	for _, v := range snap.Vocab {
		if containsID(v.Tags, id) {
			vocabRefs = append(vocabRefs, v.ID)
		}
	}
	for _, tg := range snap.Tags {
		if tg.ID != id && containsID(tg.ParentIDs, id) {
			tagRefs = append(tagRefs, tg.ID)
		}
	}
	sort.Strings(txRefs)
	sort.Strings(vocabRefs)
	sort.Strings(tagRefs)
	hasRefs := len(txRefs) > 0 || len(vocabRefs) > 0 || len(tagRefs) > 0

	if hasRefs && !force {
		return TagRemoveResult{}, fmt.Errorf(
			"tag %q は参照されています（transition: %s / vocab: %s / tag parentIds: %s）。--force で detach cascade できます",
			id, strings.Join(txRefs, ","), strings.Join(vocabRefs, ","), strings.Join(tagRefs, ","))
	}

	result := TagRemoveResult{ID: id, Forced: force}
	if hasRefs {
		for _, txID := range txRefs {
			t, err := s.LoadTransition(txID)
			if err != nil {
				return TagRemoveResult{}, err
			}
			t.Tags = removeID(t.Tags, id)
			if err := s.SaveTransition(t); err != nil {
				return TagRemoveResult{}, err
			}
		}
		result.DetachedTransitions = txRefs
		for _, vID := range vocabRefs {
			v, err := s.LoadVocab(vID)
			if err != nil {
				return TagRemoveResult{}, err
			}
			v.Tags = removeID(v.Tags, id)
			if err := s.SaveVocab(v); err != nil {
				return TagRemoveResult{}, err
			}
		}
		result.DetachedVocab = vocabRefs
		for _, tgID := range tagRefs {
			tg, err := s.LoadTag(tgID)
			if err != nil {
				return TagRemoveResult{}, err
			}
			tg.ParentIDs = removeID(tg.ParentIDs, id)
			if err := s.SaveTag(tg); err != nil {
				return TagRemoveResult{}, err
			}
		}
		result.DetachedTags = tagRefs
	}

	if err := os.Remove(s.tagPath(id)); err != nil {
		return TagRemoveResult{}, err
	}
	return result, nil
}

// RemoveTransitionResult summarizes `scholia tx rm` (§6).
type RemoveTransitionResult struct {
	ID               string   `json:"id"`
	Why              string   `json:"why"`
	RemovedDecisions []string `json:"removedDecisions,omitempty"`
}

// RemoveTransition deletes a transition together with every decision that
// targets it (§6 "破壊的...decisions も道連れ"). why is not persisted
// anywhere — decisions are append-only records, not a place to log a
// deletion rationale for a record that no longer exists — so it is only
// echoed back to the caller for a stderr audit line.
func (s *Store) RemoveTransition(id, why string) (RemoveTransitionResult, error) {
	if !s.TransitionExists(id) {
		return RemoveTransitionResult{}, fmt.Errorf("transition %q が見つかりません", id)
	}
	snap, err := s.LoadAll()
	if err != nil {
		return RemoveTransitionResult{}, err
	}

	var decisionIDs []string
	for _, d := range snap.Decisions {
		if d.Target.Type == model.DecisionTargetTransition && d.Target.ID == id {
			decisionIDs = append(decisionIDs, d.ID)
		}
	}
	sort.Strings(decisionIDs)

	if err := os.Remove(s.transitionPath(id)); err != nil {
		return RemoveTransitionResult{}, err
	}
	for _, dID := range decisionIDs {
		if err := os.Remove(s.decisionPath(dID)); err != nil {
			return RemoveTransitionResult{}, err
		}
	}
	return RemoveTransitionResult{ID: id, Why: why, RemovedDecisions: decisionIDs}, nil
}

// TransitionReferencedError is returned by RemoveTransitionUnlinked when the
// transition is still the target of one or more decisions.
type TransitionReferencedError struct {
	ID          string
	DecisionIDs []string
}

func (e *TransitionReferencedError) Error() string {
	return fmt.Sprintf(
		"transition %q は %d 件の decision の対象です（削除できません）: %s",
		e.ID, len(e.DecisionIDs), strings.Join(e.DecisionIDs, ", "))
}

// RemoveTransitionUnlinked deletes only the transition file itself — no
// cascade. This is the viewer's DELETE /api/transitions/{id} primitive
// (change-cockpit-design-v3.md §8.8 P5, G-1′ extension): unlike
// RemoveTransition/`scholia tx rm` (which deletes every decision targeting the
// transition too, since a human explicitly asked for the destructive
// cascade via --why/--force), an HTTP DELETE from the cockpit must never
// reach past the one atom it was asked to remove. Deleting decision files
// out from under a human is the kind of blast radius §7's "viewer only
// writes config" was guarding against — it isn't just "does this look like
// the same operation as `scholia tx rm`", it's "would removing a Transition
// silently discard someone else's append-only decision record".
//
// So instead of cascading, this refuses whenever any decision still
// targets the transition (which would otherwise leave `scholia lint`'s
// decision-target rule — SeverityError — pointing at a dangling
// reference): the caller gets a TransitionReferencedError naming every
// blocking decision id, and nothing is deleted. The transition can only be
// removed via this path once it's unreferenced (symmetric with
// RemoveVocab's "unreferenced-only" rule above), or via `scholia tx rm
// --force` for the cascading case, which stays a deliberate human/CLI
// action.
func (s *Store) RemoveTransitionUnlinked(id string) error {
	if !s.TransitionExists(id) {
		return fmt.Errorf("transition %q が見つかりません", id)
	}
	snap, err := s.LoadAll()
	if err != nil {
		return err
	}

	var decisionIDs []string
	for _, d := range snap.Decisions {
		if d.Target.Type == model.DecisionTargetTransition && d.Target.ID == id {
			decisionIDs = append(decisionIDs, d.ID)
		}
	}
	if len(decisionIDs) > 0 {
		sort.Strings(decisionIDs)
		return &TransitionReferencedError{ID: id, DecisionIDs: decisionIDs}
	}

	return os.Remove(s.transitionPath(id))
}

func containsID(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// removeID returns list with want removed, or nil if the result would be
// empty (keeps omitempty fields actually omitted — §3.1 normalization).
func removeID(list []string, want string) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		if v != want {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
