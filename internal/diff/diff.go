package diff

import (
	"errors"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
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
}

type TransitionDiff struct {
	Added   []model.Transition `json:"added,omitempty"`
	Removed []model.Transition `json:"removed,omitempty"`
	Changed []TransitionChange `json:"changed,omitempty"`
}

// DecisionChange は同一 id の decision の変更を欄位単位で分類したもの（#45 U4）。
// JSON は従来の {id, before, after} を保ち、分類（allowedFields/violatedFields）を
// additive に足す（viewer /api/diff へは透過）。
type DecisionChange struct {
	ID     string         `json:"id"`
	Before model.Decision `json:"before"`
	After  model.Decision `json:"after"`
	// AllowedFields は許容欄位の変化の記述（"commits(+1)"・
	// "target.id(rename T-a→T-b)"・"target.id(merge T-dup→T-surv)" 等）。
	AllowedFields []string `json:"allowedFields,omitempty"`
	// ViolatedFields は不可侵欄位（why/changed/ref/at/target.type）の改変、
	// または許容形でない commits/target.id の変更（欄位名で列挙）。
	ViolatedFields []string `json:"violatedFields,omitempty"`
}

// Violation は不可侵欄位の改変を 1 つでも含むか（欄位単位 append-only 違反）。
func (c DecisionChange) Violation() bool { return len(c.ViolatedFields) > 0 }

// DecisionDiff: decisions は append-only（§3.5・欄位単位）。Added は正常な追記。
// Removed は無条件で違反。Changed は欄位単位で分類され、許容欄位（commits 追記・
// rename/merge 追随の target.id 張替え）のみの変更は違反にしない（#45 U4——
// `decision add-commit` や `tag/tx/vocab rename`・`tx merge` の正規操作を
// 撃墜しない）。
type DecisionDiff struct {
	Added   []model.Decision `json:"added,omitempty"`
	Removed []model.Decision `json:"removed,omitempty"`
	Changed []DecisionChange `json:"changed,omitempty"`
}

// Result は `scholia diff` の出力全体。
type Result struct {
	Ref         string         `json:"ref"`
	Vocab       VocabDiff      `json:"vocab"`
	Tags        TagDiff        `json:"tags"`
	Transitions TransitionDiff `json:"transitions"`
	Decisions   DecisionDiff   `json:"decisions"`
	// BaselineMissing は ref に既定値を使い（明示指定なし）、かつそのベースライン
	// （HEAD のコミット or ref 上の .scholia）が単に存在しないためフォールバックした
	// ことを示す（初回ユーザー向け。ユーザーが gitref を明示指定した場合はこの
	// フォールバックは起きず、ベースライン欠落は通常どおりエラーになる）。
	BaselineMissing bool `json:"baselineMissing,omitempty"`
	// AfterRef は DiffRefs（ref 対 ref・2引数の `scholia diff A B`）でのみ設定される。
	// 空文字なら Diff（作業ツリー vs Ref）の従来経路であることを示す（後方互換の
	// ための additive フィールド・0/1 引数の JSON 出力に影響しない）。
	AfterRef string `json:"afterRef,omitempty"`
	// RetrofitAllowed / RetrofitReason は逃し弁（明示の例外承認・#42 型の全店
	// retrofit 用・#45 U4）が有効なとき CLI が記録する。理由必須・出力への記録
	// （text と --json の両方）が承認の条件（黙殺でなく明文の例外にする）。
	RetrofitAllowed bool   `json:"retrofitAllowed,omitempty"`
	RetrofitReason  string `json:"retrofitReason,omitempty"`
}

// Empty は現在の作業ツリーと ref との間に意味のある差分が無いことを返す。
func (r Result) Empty() bool {
	return len(r.Vocab.Added) == 0 && len(r.Vocab.Removed) == 0 && len(r.Vocab.Changed) == 0 &&
		len(r.Tags.Added) == 0 && len(r.Tags.Removed) == 0 && len(r.Tags.Changed) == 0 &&
		len(r.Transitions.Added) == 0 && len(r.Transitions.Removed) == 0 && len(r.Transitions.Changed) == 0 &&
		len(r.Decisions.Added) == 0 && len(r.Decisions.Removed) == 0 && len(r.Decisions.Changed) == 0
}

// DecisionViolation は append-only 不変条件への違反があるかを欄位単位で返す
// （#45 U4）: decision の削除は無条件で違反。同一 id の変更は、不可侵欄位
// （why/changed/ref/at/target.type）の改変か、許容形でない commits/target.id の
// 変更を含む場合のみ違反（commits 追記・rename/merge 追随の target.id 張替えは
// 正規操作として許容）。
func (r Result) DecisionViolation() bool {
	if len(r.Decisions.Removed) > 0 {
		return true
	}
	for _, c := range r.Decisions.Changed {
		if c.Violation() {
			return true
		}
	}
	return false
}

// Diff は現在の作業ツリー（s の .scholia/）と gitref（既定 "HEAD"）の semantic diff を計算する（§4）。
// ref を空文字にすると「ユーザーが gitref を明示指定していない」既定呼び出しとして扱われ、
// ベースライン（HEAD のコミット or ref 上の .scholia）が単に存在しない場合はエラーにせず空
// ベースラインにフォールバックする（現在の全レコードが added として表示される）。ref を
// 明示的に渡した場合はベースライン欠落も含めて従来どおりエラーを返す。
func Diff(s *store.Store, ref string) (Result, error) {
	explicit := ref != ""
	if ref == "" {
		ref = "HEAD"
	}

	repoRootResolved, relDir, err := repoRootAndRelDir(s)
	if err != nil {
		return Result{}, err
	}

	var baselineMissing bool
	before, err := loadRefSnapshot(repoRootResolved, relDir, ref)
	if err != nil {
		var missing *baselineMissingError
		if explicit || !errors.As(err, &missing) {
			return Result{}, err
		}
		before = refSnapshot{}
		baselineMissing = true
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

	result := compute(ref, before, after)
	result.BaselineMissing = baselineMissing
	return result, nil
}

// DiffRefs は2つの git ref 間（beforeRef vs afterRef）の semantic diff を計算する
// （`scholia diff A B`・§4 R-2）。Diff と異なり両側とも明示的な ref であり作業ツリーを
// 一切読まないため、「タスク粒度=commit」を成立させるコア（`scholia diff <commit>^ <commit>`
// でチェックアウト無しに1コミット分の変更を出せる）。どちらかの ref が解決できない・
// ref 上に .scholia/ が無い場合は Diff の「既定 ref フォールバック」は適用されず、常に
// エラーを返す（ユーザーが両方とも明示指定しているため typo を握り潰さない）。
func DiffRefs(s *store.Store, beforeRef, afterRef string) (Result, error) {
	repoRootResolved, relDir, err := repoRootAndRelDir(s)
	if err != nil {
		return Result{}, err
	}

	before, err := loadRefSnapshot(repoRootResolved, relDir, beforeRef)
	if err != nil {
		return Result{}, err
	}
	after, err := loadRefSnapshot(repoRootResolved, relDir, afterRef)
	if err != nil {
		return Result{}, err
	}

	result := compute(beforeRef, before, after)
	result.AfterRef = afterRef
	return result, nil
}

// repoRootAndRelDir は s の .scholia/ を含む git リポジトリのルート（シンボリックリンク
// 解決済み）と、そのルートから .scholia/ への相対パスを返す（Diff / DiffRefs 共通の
// リポジトリ解決ロジック）。
func repoRootAndRelDir(s *store.Store) (repoRootResolved, relDir string, err error) {
	projectRoot := filepath.Dir(s.Dir)
	repoRoot, err := gitRepoRoot(projectRoot)
	if err != nil {
		return "", "", err
	}
	// git rev-parse --show-toplevel はシンボリックリンクを解決した絶対パスを返す
	// （macOS の /var -> /private/var 等）。s.Dir 側も解決してから相対化しないと
	// filepath.Rel が ".." だらけの誤った相対パスを作ってしまう。
	repoRootResolved = resolveSymlinks(repoRoot)
	scholiaDirResolved := resolveSymlinks(s.Dir)
	relDir, err = relToRepoRoot(repoRootResolved, scholiaDirResolved)
	if err != nil {
		return "", "", err
	}
	return repoRootResolved, relDir, nil
}

func compute(ref string, before, after refSnapshot) Result {
	return Result{
		Ref:         ref,
		Vocab:       diffVocab(before.Vocab, after.Vocab),
		Tags:        diffTags(before.Tags, after.Tags),
		Transitions: diffTransitions(before.Transitions, after.Transitions),
		Decisions:   diffDecisions(before, after),
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

// diffDecisions は decision の ±／変更を計算する。変更は欄位単位で分類する
// （decision_fields.go・#45 U4）ため、rename/merge ペア照合に使う before/after の
// 全レコード（transitions/tags/vocab）ごと受け取る。
//
// 前方互換（P7 の supersedes[] 等・未知 additive フィールド）: refSnapshot は
// model.Decision へ decode され、encoding/json は未知フィールドを無視するため、
// 未知 additive フィールドの追記だけの decision はそもそも Changed に現れない
// （＝violation にしない・#45 U4 の前方互換要件を decode 層で満たす）。
func diffDecisions(before, after refSnapshot) DecisionDiff {
	beforeByID := indexByID(before.Decisions, model.Decision.GetID)
	afterByID := indexByID(after.Decisions, model.Decision.GetID)
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
	var ctx *pairContext
	for _, id := range sortedKeys(beforeByID) {
		a, ok := afterByID[id]
		if !ok {
			continue
		}
		b := beforeByID[id]
		if reflect.DeepEqual(b, a) {
			continue
		}
		if ctx == nil {
			ctx = newPairContext(before, after)
		}
		allowed, violated := classifyDecisionChange(b, a, ctx)
		d.Changed = append(d.Changed, DecisionChange{
			ID: id, Before: b, After: a,
			AllowedFields: allowed, ViolatedFields: violated,
		})
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

// transitionsSemanticallyEqual は given/tags を集合として比較し、then のみ
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
	return true
}

func transitionChange(id string, before, after model.Transition) TransitionChange {
	givenAdded, givenRemoved := setDiff(before.Given, after.Given)
	tagsAdded, tagsRemoved := setDiff(before.Tags, after.Tags)

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
