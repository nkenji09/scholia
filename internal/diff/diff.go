package diff

import (
	"path/filepath"
	"reflect"
	"sort"

	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

// Change は同一 id で内容が変わったレコード 1 件（vocab/tag/decision の "changed" 用）。
type Change[T any] struct {
	ID     string `json:"id"`
	Before T      `json:"before"`
	After  T      `json:"after"`
}

// VocabDiff / TagDiff は語彙・タグの ± と変更（§4「語彙 ±」「タグ ±」）。
type VocabDiff struct {
	Added   []model.VocabEntry         `json:"added,omitempty"`
	Removed []model.VocabEntry         `json:"removed,omitempty"`
	Changed []Change[model.VocabEntry] `json:"changed,omitempty"`
}

type TagDiff struct {
	Added   []model.Tag         `json:"added,omitempty"`
	Removed []model.Tag         `json:"removed,omitempty"`
	Changed []Change[model.Tag] `json:"changed,omitempty"`
}

// TransitionChange は 1 遷移の変更内容。given は集合比較、then は順序リスト比較
// （並び替えも変更として検出・§3.2, §4）。
type TransitionChange struct {
	ID            string           `json:"id"`
	Before        model.Transition `json:"before"`
	After         model.Transition `json:"after"`
	ActionChanged bool             `json:"actionChanged,omitempty"`
	GivenAdded    []string         `json:"givenAdded,omitempty"`
	GivenRemoved  []string         `json:"givenRemoved,omitempty"`
	ThenChanged   bool             `json:"thenChanged,omitempty"`
	ThenReordered bool             `json:"thenReordered,omitempty"` // 集合は同じだが順序が変わった
	TagsAdded     []string         `json:"tagsAdded,omitempty"`
	TagsRemoved   []string         `json:"tagsRemoved,omitempty"`
	TestsAdded    []string         `json:"testsAdded,omitempty"`
	TestsRemoved  []string         `json:"testsRemoved,omitempty"`
}

type TransitionDiff struct {
	Added   []model.Transition `json:"added,omitempty"`
	Removed []model.Transition `json:"removed,omitempty"`
	Changed []TransitionChange `json:"changed,omitempty"`
}

// DecisionDiff: decisions は append-only（§3.5）。Added は正常な追記、Removed/Changed は
// 不変条件違反として強調する（§4「decisions ±（append-only なので削除・改変が検出されたら error 扱いで強調）」）。
type DecisionDiff struct {
	Added   []model.Decision         `json:"added,omitempty"`
	Removed []model.Decision         `json:"removed,omitempty"`
	Changed []Change[model.Decision] `json:"changed,omitempty"`
}

// Result は `pmem diff` の出力全体。
type Result struct {
	Ref         string         `json:"ref"`
	Vocab       VocabDiff      `json:"vocab"`
	Tags        TagDiff        `json:"tags"`
	Transitions TransitionDiff `json:"transitions"`
	Decisions   DecisionDiff   `json:"decisions"`
}

// Empty は現在の作業ツリーと ref との間に意味のある差分が無いことを返す。
func (r Result) Empty() bool {
	return len(r.Vocab.Added) == 0 && len(r.Vocab.Removed) == 0 && len(r.Vocab.Changed) == 0 &&
		len(r.Tags.Added) == 0 && len(r.Tags.Removed) == 0 && len(r.Tags.Changed) == 0 &&
		len(r.Transitions.Added) == 0 && len(r.Transitions.Removed) == 0 && len(r.Transitions.Changed) == 0 &&
		len(r.Decisions.Added) == 0 && len(r.Decisions.Removed) == 0 && len(r.Decisions.Changed) == 0
}

// DecisionViolation は append-only 不変条件への違反（decision の削除／改変）があるかを返す。
func (r Result) DecisionViolation() bool {
	return len(r.Decisions.Removed) > 0 || len(r.Decisions.Changed) > 0
}

// Diff は現在の作業ツリー（s の .pmem/）と gitref（既定 "HEAD"）の semantic diff を計算する（§4）。
func Diff(s *store.Store, ref string) (Result, error) {
	if ref == "" {
		ref = "HEAD"
	}

	projectRoot := filepath.Dir(s.Dir)
	repoRoot, err := gitRepoRoot(projectRoot)
	if err != nil {
		return Result{}, err
	}
	// git rev-parse --show-toplevel はシンボリックリンクを解決した絶対パスを返す
	// （macOS の /var -> /private/var 等）。s.Dir 側も解決してから相対化しないと
	// filepath.Rel が ".." だらけの誤った相対パスを作ってしまう。
	repoRootResolved := resolveSymlinks(repoRoot)
	pmemDirResolved := resolveSymlinks(s.Dir)
	relDir, err := relToRepoRoot(repoRootResolved, pmemDirResolved)
	if err != nil {
		return Result{}, err
	}

	before, err := loadRefSnapshot(repoRootResolved, relDir, ref)
	if err != nil {
		return Result{}, err
	}

	working, err := s.LoadAll()
	if err != nil {
		return Result{}, err
	}
	after := refSnapshot{
		Vocab:       working.Vocab,
		Tags:        working.Tags,
		Transitions: working.Transitions,
		Decisions:   working.Decisions,
	}

	return compute(ref, before, after), nil
}

func compute(ref string, before, after refSnapshot) Result {
	return Result{
		Ref:         ref,
		Vocab:       diffVocab(before.Vocab, after.Vocab),
		Tags:        diffTags(before.Tags, after.Tags),
		Transitions: diffTransitions(before.Transitions, after.Transitions),
		Decisions:   diffDecisions(before.Decisions, after.Decisions),
	}
}

func diffVocab(before, after []model.VocabEntry) VocabDiff {
	beforeByID := indexByID(before, model.VocabEntry.GetID)
	afterByID := indexByID(after, model.VocabEntry.GetID)
	var d VocabDiff
	for _, id := range sortedKeys(afterByID) {
		if _, ok := beforeByID[id]; !ok {
			d.Added = append(d.Added, afterByID[id])
		}
	}
	for _, id := range sortedKeys(beforeByID) {
		if _, ok := afterByID[id]; !ok {
			d.Removed = append(d.Removed, beforeByID[id])
		}
	}
	for _, id := range sortedKeys(beforeByID) {
		a, ok := afterByID[id]
		if !ok {
			continue
		}
		if b := beforeByID[id]; !reflect.DeepEqual(b, a) {
			d.Changed = append(d.Changed, Change[model.VocabEntry]{ID: id, Before: b, After: a})
		}
	}
	return d
}

func diffTags(before, after []model.Tag) TagDiff {
	beforeByID := indexByID(before, model.Tag.GetID)
	afterByID := indexByID(after, model.Tag.GetID)
	var d TagDiff
	for _, id := range sortedKeys(afterByID) {
		if _, ok := beforeByID[id]; !ok {
			d.Added = append(d.Added, afterByID[id])
		}
	}
	for _, id := range sortedKeys(beforeByID) {
		if _, ok := afterByID[id]; !ok {
			d.Removed = append(d.Removed, beforeByID[id])
		}
	}
	for _, id := range sortedKeys(beforeByID) {
		a, ok := afterByID[id]
		if !ok {
			continue
		}
		if b := beforeByID[id]; !reflect.DeepEqual(b, a) {
			d.Changed = append(d.Changed, Change[model.Tag]{ID: id, Before: b, After: a})
		}
	}
	return d
}

func diffDecisions(before, after []model.Decision) DecisionDiff {
	beforeByID := indexByID(before, model.Decision.GetID)
	afterByID := indexByID(after, model.Decision.GetID)
	var d DecisionDiff
	for _, id := range sortedKeys(afterByID) {
		if _, ok := beforeByID[id]; !ok {
			d.Added = append(d.Added, afterByID[id])
		}
	}
	for _, id := range sortedKeys(beforeByID) {
		if _, ok := afterByID[id]; !ok {
			d.Removed = append(d.Removed, beforeByID[id])
		}
	}
	for _, id := range sortedKeys(beforeByID) {
		a, ok := afterByID[id]
		if !ok {
			continue
		}
		if b := beforeByID[id]; !reflect.DeepEqual(b, a) {
			d.Changed = append(d.Changed, Change[model.Decision]{ID: id, Before: b, After: a})
		}
	}
	return d
}

func diffTransitions(before, after []model.Transition) TransitionDiff {
	beforeByID := indexByID(before, model.Transition.GetID)
	afterByID := indexByID(after, model.Transition.GetID)
	var d TransitionDiff
	for _, id := range sortedKeys(afterByID) {
		if _, ok := beforeByID[id]; !ok {
			d.Added = append(d.Added, afterByID[id])
		}
	}
	for _, id := range sortedKeys(beforeByID) {
		if _, ok := afterByID[id]; !ok {
			d.Removed = append(d.Removed, beforeByID[id])
		}
	}
	for _, id := range sortedKeys(beforeByID) {
		a, ok := afterByID[id]
		if !ok {
			continue
		}
		b := beforeByID[id]
		if transitionsSemanticallyEqual(b, a) {
			continue
		}
		d.Changed = append(d.Changed, transitionChange(id, b, a))
	}
	return d
}

// transitionsSemanticallyEqual は given/tags/tests を集合として比較し、then のみ
// 順序リストとして比較する（§3.2）。given の並びだけが違う記録を「変更」として
// 報告しないためのゲート（byte-identical である必要はない）。
func transitionsSemanticallyEqual(a, b model.Transition) bool {
	if a.Action != b.Action {
		return false
	}
	if !reflect.DeepEqual(a.Then, b.Then) {
		return false
	}
	if !reflect.DeepEqual(sortedCopy(a.Given), sortedCopy(b.Given)) {
		return false
	}
	if !reflect.DeepEqual(sortedCopy(a.Tags), sortedCopy(b.Tags)) {
		return false
	}
	if !reflect.DeepEqual(sortedCopy(a.Tests), sortedCopy(b.Tests)) {
		return false
	}
	return true
}

func transitionChange(id string, before, after model.Transition) TransitionChange {
	givenAdded, givenRemoved := setDiff(before.Given, after.Given)
	tagsAdded, tagsRemoved := setDiff(before.Tags, after.Tags)
	testsAdded, testsRemoved := setDiff(before.Tests, after.Tests)

	thenChanged := !reflect.DeepEqual(before.Then, after.Then)
	thenReordered := false
	if thenChanged {
		bSorted, aSorted := sortedCopy(before.Then), sortedCopy(after.Then)
		thenReordered = reflect.DeepEqual(bSorted, aSorted)
	}

	return TransitionChange{
		ID:            id,
		Before:        before,
		After:         after,
		ActionChanged: before.Action != after.Action,
		GivenAdded:    givenAdded,
		GivenRemoved:  givenRemoved,
		ThenChanged:   thenChanged,
		ThenReordered: thenReordered,
		TagsAdded:     tagsAdded,
		TagsRemoved:   tagsRemoved,
		TestsAdded:    testsAdded,
		TestsRemoved:  testsRemoved,
	}
}

// setDiff は given のような「集合」フィールドの差分（順不同・§3.2）。
func setDiff(before, after []string) (added, removed []string) {
	beforeSet := toSet(before)
	afterSet := toSet(after)
	for id := range afterSet {
		if !beforeSet[id] {
			added = append(added, id)
		}
	}
	for id := range beforeSet {
		if !afterSet[id] {
			removed = append(removed, id)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func sortedCopy(ss []string) []string {
	out := append([]string{}, ss...)
	sort.Strings(out)
	return out
}

func indexByID[T any](records []T, getID func(T) string) map[string]T {
	m := make(map[string]T, len(records))
	for _, r := range records {
		m[getID(r)] = r
	}
	return m
}

func sortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
