// Package lint checks that .scholia/ records are internally consistent (§5).
// lint は早期バグ検知ではなく「記録が自己矛盾していないこと」を守る（DESIGN §0, §5）。
package lint

import (
	"fmt"
	"sort"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

type Severity string

const (
	SeverityError Severity = "error"
	SeverityWarn  Severity = "warn"
	SeverityInfo  Severity = "info"
)

// Finding は 1 件の lint 検出結果。
type Finding struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Target   string   `json:"target,omitempty"`
}

// Rule は 1 つの lint ルール。Phase 1 以降の warn/info ルールもこの枠組みに追加する。
type Rule struct {
	Name     string
	Severity Severity
	Check    func(snap store.Snapshot) []Finding
}

// Rules は §5 の error/warn/info ルール一式。error のみ lint の exit code を 1 にする（HasError）。
var Rules = []Rule{
	{Name: "vocab-ref", Severity: SeverityError, Check: checkVocabRef},
	{Name: "kind-valid", Severity: SeverityError, Check: checkKindValid},
	{Name: "tag-ref", Severity: SeverityError, Check: checkTagRef},
	{Name: "decision-target", Severity: SeverityError, Check: checkDecisionTarget},
	{Name: "empty-then", Severity: SeverityError, Check: checkEmptyThen},
	{Name: "id-unique", Severity: SeverityError, Check: checkIDUnique},
	{Name: "requirement-gap", Severity: SeverityWarn, Check: checkRequirementGap},
	{Name: "kind-missing", Severity: SeverityWarn, Check: checkKindMissing},
	{Name: "ref-freshness", Severity: SeverityWarn, Check: checkRefFreshness},
	{Name: "decision-coverage", Severity: SeverityInfo, Check: checkDecisionCoverage},
	{Name: "unused-vocab", Severity: SeverityInfo, Check: checkUnusedVocab},
	{Name: "exclusive-violation", Severity: SeverityWarn, Check: checkExclusiveViolation},
	{Name: "complement-missing", Severity: SeverityWarn, Check: checkComplementMissing},
}

// Run は全ルールを実行し、検出結果を返す。
func Run(snap store.Snapshot) []Finding {
	var all []Finding
	for _, r := range Rules {
		all = append(all, r.Check(snap)...)
	}
	return all
}

// HasError は error 重大度の finding が 1 件でもあるかを返す。
func HasError(findings []Finding) bool {
	for _, f := range findings {
		if f.Severity == SeverityError {
			return true
		}
	}
	return false
}

func finding(rule string, severity Severity, target, format string, args ...any) Finding {
	return Finding{Rule: rule, Severity: severity, Target: target, Message: fmt.Sprintf(format, args...)}
}

// --- vocab-ref: action/given/then が実在する語彙を解決し、カテゴリが一致すること ---

func checkVocabRef(snap store.Snapshot) []Finding {
	vocabByID := indexVocab(snap.Vocab)
	var out []Finding
	for _, t := range snap.Transitions {
		out = append(out, checkVocabRefSlot(vocabByID, t.ID, "action", []string{t.Action}, model.CategoryAction)...)
		out = append(out, checkVocabRefSlot(vocabByID, t.ID, "given", t.Given, model.CategoryCondition)...)
		out = append(out, checkVocabRefSlot(vocabByID, t.ID, "then", t.Then, model.CategoryEffect)...)
	}
	return out
}

func checkVocabRefSlot(vocabByID map[string]model.VocabEntry, txID, slot string, ids []string, wantCategory string) []Finding {
	var out []Finding
	for _, id := range ids {
		v, ok := vocabByID[id]
		if !ok {
			out = append(out, finding("vocab-ref", SeverityError, txID,
				"transition %s: %s %q が実在しない語彙を参照しています", txID, slot, id))
			continue
		}
		if v.Category != wantCategory {
			out = append(out, finding("vocab-ref", SeverityError, txID,
				"transition %s: %s %q は %s カテゴリの語彙ではありません（実際は %s）", txID, slot, id, wantCategory, v.Category))
		}
	}
	return out
}

// --- kind-valid: 語彙・タグの kind が config の宣言集合に含まれること ---

func checkKindValid(snap store.Snapshot) []Finding {
	var out []Finding
	for _, v := range snap.Vocab {
		if v.Kind == "" {
			continue
		}
		if !contains(snap.Config.KindsFor(v.Category), v.Kind) {
			out = append(out, finding("kind-valid", SeverityError, v.ID,
				"vocab %s: kind %q は config.kinds.%s に未宣言です", v.ID, v.Kind, v.Category))
		}
	}
	for _, t := range snap.Tags {
		if t.Kind == "" {
			continue
		}
		if !contains(snap.Config.TagKinds, t.Kind) {
			out = append(out, finding("kind-valid", SeverityError, t.ID,
				"tag %s: kind %q は config.tagKinds に未宣言です", t.ID, t.Kind))
		}
	}
	return out
}

// --- tag-ref: 遷移・語彙の tagId、タグの parentIds が解決する + タグに循環がない ---

func checkTagRef(snap store.Snapshot) []Finding {
	tagByID := indexTags(snap.Tags)
	var out []Finding

	for _, t := range snap.Transitions {
		for _, tagID := range t.Tags {
			if _, ok := tagByID[tagID]; !ok {
				out = append(out, finding("tag-ref", SeverityError, t.ID,
					"transition %s: タグ %q が実在しません", t.ID, tagID))
			}
		}
	}
	for _, v := range snap.Vocab {
		for _, tagID := range v.Tags {
			if _, ok := tagByID[tagID]; !ok {
				out = append(out, finding("tag-ref", SeverityError, v.ID,
					"vocab %s: タグ %q が実在しません", v.ID, tagID))
			}
		}
	}

	parents := make(map[string][]string, len(snap.Tags))
	for _, t := range snap.Tags {
		parents[t.ID] = t.ParentIDs
		for _, p := range t.ParentIDs {
			if _, ok := tagByID[p]; !ok {
				out = append(out, finding("tag-ref", SeverityError, t.ID,
					"tag %s: parentIds %q が実在しません", t.ID, p))
			}
		}
	}

	for _, id := range CycleMembers(parents) {
		out = append(out, finding("tag-ref", SeverityError, id, "tag %s: parentIds が循環しています", id))
	}

	return out
}

// CycleMembers は parentIds グラフ（id -> parentIds）の中で循環に含まれる id 集合を返す（決定的な順序）。
// tag create の書き込み時チェックと lint の両方から使う共有ロジック。
func CycleMembers(parents map[string][]string) []string {
	const (
		unvisited = 0
		visiting  = 1
		done      = 2
	)
	state := make(map[string]int, len(parents))
	inCycle := make(map[string]bool)

	var visit func(id string, stack []string)
	visit = func(id string, stack []string) {
		switch state[id] {
		case visiting:
			// stack に id が現れる位置から今までが循環メンバー
			for i, s := range stack {
				if s == id {
					for _, m := range stack[i:] {
						inCycle[m] = true
					}
					break
				}
			}
			return
		case done:
			return
		}
		state[id] = visiting
		stack = append(stack, id)
		for _, p := range parents[id] {
			if _, known := parents[p]; !known {
				continue // 未解決の parent は tag-ref の別チェックが担当
			}
			visit(p, stack)
		}
		state[id] = done
	}

	ids := make([]string, 0, len(parents))
	for id := range parents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if state[id] == unvisited {
			visit(id, nil)
		}
	}

	out := make([]string, 0, len(inCycle))
	for id := range inCycle {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// --- decision-target: decision.target が実在する transition／tag を指す ---

func checkDecisionTarget(snap store.Snapshot) []Finding {
	txByID := indexTransitions(snap.Transitions)
	tagByID := indexTags(snap.Tags)
	var out []Finding
	for _, d := range snap.Decisions {
		switch d.Target.Type {
		case model.DecisionTargetTransition:
			if _, ok := txByID[d.Target.ID]; !ok {
				out = append(out, finding("decision-target", SeverityError, d.ID,
					"decision %s: 対象の transition %q が実在しません", d.ID, d.Target.ID))
			}
		case model.DecisionTargetTag:
			if _, ok := tagByID[d.Target.ID]; !ok {
				out = append(out, finding("decision-target", SeverityError, d.ID,
					"decision %s: 対象の tag %q が実在しません", d.ID, d.Target.ID))
			}
		default:
			out = append(out, finding("decision-target", SeverityError, d.ID,
				"decision %s: target.type %q は transition/tag のいずれでもありません", d.ID, d.Target.Type))
		}
	}
	return out
}

// --- empty-then: then 空の遷移は作れない ---

func checkEmptyThen(snap store.Snapshot) []Finding {
	var out []Finding
	for _, t := range snap.Transitions {
		if len(t.Then) == 0 {
			out = append(out, finding("empty-then", SeverityError, t.ID, "transition %s: then が空です", t.ID))
		}
	}
	return out
}

// --- id-unique: ファイル名と id の一致・同一カテゴリ内での id 重複なし ---

func checkIDUnique(snap store.Snapshot) []Finding {
	var out []Finding
	for _, m := range snap.IDMismatches {
		out = append(out, finding("id-unique", SeverityError, m.RecordID,
			"%s: ファイル名 %q と内部 id %q が一致しません", m.Category, m.File, m.RecordID))
	}

	out = append(out, checkDuplicateIDs("vocab", ids(snap.Vocab, model.VocabEntry.GetID))...)
	out = append(out, checkDuplicateIDs("tag", ids(snap.Tags, model.Tag.GetID))...)
	out = append(out, checkDuplicateIDs("transition", ids(snap.Transitions, model.Transition.GetID))...)
	out = append(out, checkDuplicateIDs("decision", ids(snap.Decisions, model.Decision.GetID))...)
	return out
}

func ids[T any](records []T, getID func(T) string) []string {
	out := make([]string, len(records))
	for i, r := range records {
		out[i] = getID(r)
	}
	return out
}

func checkDuplicateIDs(category string, allIDs []string) []Finding {
	seen := make(map[string]int)
	for _, id := range allIDs {
		seen[id]++
	}
	var dupIDs []string
	for id, count := range seen {
		if count > 1 {
			dupIDs = append(dupIDs, id)
		}
	}
	sort.Strings(dupIDs)
	var out []Finding
	for _, id := range dupIDs {
		out = append(out, finding("id-unique", SeverityError, id, "%s: id %q が重複しています", category, id))
	}
	return out
}

// --- indexes ---

func indexVocab(vocab []model.VocabEntry) map[string]model.VocabEntry {
	m := make(map[string]model.VocabEntry, len(vocab))
	for _, v := range vocab {
		m[v.ID] = v
	}
	return m
}

func indexTags(tags []model.Tag) map[string]model.Tag {
	m := make(map[string]model.Tag, len(tags))
	for _, t := range tags {
		m[t.ID] = t
	}
	return m
}

func indexTransitions(transitions []model.Transition) map[string]model.Transition {
	m := make(map[string]model.Transition, len(transitions))
	for _, t := range transitions {
		m[t.ID] = t
	}
	return m
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
