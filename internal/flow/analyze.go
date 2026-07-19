// Package flow derives an action-scoped "given → then" analysis (#39
// action-flow) shared by the CLI (`scholia flow`) and, in a later phase, the
// viewer. It follows the honesty-first, layered design adopted on
// req.action-flow (design-options §7): a trust core that needs no
// declaration (the condition×transition matrix, subset-shadow) plus an
// optional axis layer (declared via kind="axis" tags) that buys exactly two
// sound signals — total-axis gaps (L-total) and declared-axis-relative
// overlap — and never claims more coverage than it can prove
// (req.action-flow.scope-honesty).
package flow

import (
	"sort"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// AxisTagKind is the compat default tagKind id that marks a tag record as an
// axis declaration (config.tagKinds must declare it; DESIGN §3.4). Kept as the
// well-known default, but axis-ness is now decided by config behaviors
// (#45 D9): a tag counts as an axis when its kind's declaration carries the
// "axis" behavior, which the compat map in model.Config.KindHasBehavior grants
// to kind=="axis" even without an explicit behaviors list. Analyze reads
// axis-ness through cfg, never this literal, so an alias kind declared
// behaviors:["axis"] participates on equal footing.
const AxisTagKind = "axis"

// isAxisTag reports whether a tag participates as an axis under the project's
// config (#45 D9). Replaces the former literal tag.Kind=="axis" checks so a
// differently-named kind declared with behaviors:["axis"] is treated as an axis
// too, while the compat rule keeps plain kind=="axis" working unchanged.
func isAxisTag(cfg model.Config, tag model.Tag) bool {
	return cfg.KindHasBehavior(tag.Kind, model.BehaviorAxis)
}

// flow finding の rule 名（#45 D6 の typed 容認キー）。lint/flow を横断する
// 「有効な rule id 集合」の source of truth の一部で、acknowledges で名指しできる
// のはここに列挙した名前と lint.Rules の名前に限る。
const (
	// RuleSubsetShadow: 証明可能な多重発火（Given(A)⊊Given(B)）。
	RuleSubsetShadow = "subset-shadow"
	// RuleTotalGap: total=true 軸の値が given に一度も現れない（L-total 抜け）。
	RuleTotalGap = "total-gap"
	// RuleOverlap: 宣言軸 cell を 2+ 遷移が取り合う（subset-shadow で説明されない）。
	RuleOverlap = "overlap"
)

// RuleNames は flow finding の rule 名の全列挙（有効 rule id 集合・容認畳みの
// 消費側が参照する source of truth）。
func RuleNames() []string {
	return []string{RuleSubsetShadow, RuleTotalGap, RuleOverlap}
}

// RemainderTagID is the well-known tag id a transition carries to declare
// itself as the action's single "acknowledged remainder" default
// (req.action-flow.acknowledged-remainder). No transition in this repo's own
// .scholia uses it yet — the convention is introduced by this implementation
// (the adopted decision fixes the semantics, not the exact tag id) and is
// deliberately just a plain string match, not a registered tag, so it is a
// no-op until a project actually opts in.
const RemainderTagID = "concern.acknowledged-remainder-default"

// MatrixRow is one transition's row in the action-scoped condition×transition
// matrix (req.action-flow.visualize) — pure derive, no claim of coverage.
type MatrixRow struct {
	TransitionID string   `json:"transitionId"`
	Given        []string `json:"given"`
	Then         []string `json:"then"`
	// Priority は同一 action 内の評価順（#45 D8・additive/omitempty）。nil=未宣言。
	// viewer が評価順バッジを描くための載せ替え——priority 未宣言 action では
	// 全 row とも nil で、従来と完全同一の描画になる。
	Priority *int `json:"priority,omitempty"`
}

// Matrix is the trust core's visualization: every transition of the action,
// and the full set of distinct given-conditions used across them.
type Matrix struct {
	Conditions []string    `json:"conditions"`
	Rows       []MatrixRow `json:"rows"`
}

// SubsetShadow reports a proven multi-fire: Given(Subset) is a non-empty
// proper subset of Given(Superset), so any world that satisfies Superset's
// given also satisfies Subset's — both transitions fire together, and which
// "then" applies is unspecified. Sound and unconditional (no axis, no
// declaration): req.action-flow.subset-shadow.
type SubsetShadow struct {
	Subset   string `json:"subset"`
	Superset string `json:"superset"`
	// AcknowledgedBy は typed 容認（#45 D6）で畳んだ decision の id。ペアの
	// いずれかの transition 宛て decision が acknowledges で subset-shadow を
	// 名指ししていれば非空（additive・omitempty）。
	AcknowledgedBy string `json:"acknowledgedBy,omitempty"`
	// Resolved は評価順で解決済みか（#45 D8）。ペアの2遷移が相異なる priority を
	// 持つとき true——multi-fire は残るが「どちらが勝つか」は宣言 priority で定まる
	// ため『優先順位未定義』ではなくなる。既定表示から畳み --verbose で開示する。
	// priority が絡まないペア（未宣言 or 同 priority）は false で従来どおり無条件
	// sound な重複として報告する（additive・omitempty）。
	Resolved bool `json:"resolved,omitempty"`
	// Winner は Resolved のとき先に評価される遷移の id（priority が小さい方）。
	Winner string `json:"winner,omitempty"`
}

// Axis is one declared axis relevant to the analyzed action (at least one
// given-condition of the action carries this axis tag).
type Axis struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Total  bool     `json:"total"`
	Values []string `json:"values"` // condition ids carrying this axis tag project-wide, sorted
}

// Axes-absence causes (#40 ①, req.action-flow.scope-honesty /
// eff.emit.scope-disclosure): when Report.Axes is empty, the reason matters —
// a bare "0 axes" collapses two different states and hides why nothing could
// be detected. AxesAbsenceNone marks a Report where axes ARE relevant (the
// field is only ever set on the len(Axes)==0 path).
const (
	// AxesAbsenceNoneDeclared: the store has zero kind="axis" tags at all —
	// the axis mechanism itself hasn't been introduced to this project yet.
	AxesAbsenceNoneDeclared = "none-declared"
	// AxesAbsenceNotOnThisAction: axis tags exist somewhere in the store, but
	// none of them is carried by a condition in this action's given —
	// declared axes simply don't reach this action.
	AxesAbsenceNotOnThisAction = "not-on-this-action"
)

// Cell is one point in the bounded product of declared axes' values.
type Cell struct {
	// Values maps axis id -> the value (condition id) this cell fixes for
	// that axis. Keys are exactly the relevant axes' ids.
	Values      map[string]string `json:"values"`
	Transitions []string          `json:"transitions"`
}

// TotalGap is a sound "抜け": a total=true axis whose value never appears in
// the given of any transition of this action.
type TotalGap struct {
	AxisID string `json:"axisId"`
	Value  string `json:"value"`
	// AcknowledgedBy は typed 容認（#45 D6）で畳んだ decision の id。当該軸タグ
	// または欠落値 condition 宛ての decision が acknowledges で total-gap を名指し
	// していれば非空（additive・omitempty）。
	AcknowledgedBy string `json:"acknowledgedBy,omitempty"`
}

// Overlap is a "重なり": a declared-axis cell covered by 2+ transitions,
// where at least one pair among them is not already explained by a proven
// SubsetShadow (exclusion is decided per pair, not per transition — see
// overlaps() below — so Transitions can include more than 2 ids, and a
// transition dropped from one shadow pair can still appear here via a real,
// unexplained ambiguity against a third transition in the same cell).
type Overlap struct {
	Cell        map[string]string `json:"cell"`
	Transitions []string          `json:"transitions"`
	// AcknowledgedBy は typed 容認（#45 D6）で畳んだ decision の id。関与する
	// transition のいずれか宛ての decision が acknowledges で overlap を名指し
	// していれば非空（additive・omitempty）。
	AcknowledgedBy string `json:"acknowledgedBy,omitempty"`
	// Resolved は評価順で解決済みか（#45 D8）。この cell を取り合う遷移群の全てが
	// 相異なる priority を持つとき true——同じ cell に複数遷移が残っても評価順で
	// 勝者が定まるため『優先順位未定義』ではなくなる。既定表示から畳み --verbose
	// で derive した補集合込みで開示する。1つでも priority 未宣言 or 同 priority の
	// ペアが混じれば false で従来どおり『優先順位未定義』として報告する（本物の穴の
	// 置き場・additive・omitempty）。
	Resolved bool `json:"resolved,omitempty"`
	// EffectiveGiven は Resolved のとき各遷移の「宣言 given ∧ ¬(より小さい
	// priority の遷移群の given の和)」＝ derive された実効 given（else の自動導出）。
	// priority 昇順に並ぶ・非検証（宣言 priority に相対的）。--verbose 表示用
	// （additive・omitempty）。
	EffectiveGiven []EffectiveGiven `json:"effectiveGiven,omitempty"`
}

// EffectiveGiven は解決済み overlap における1遷移の derive された実効 given
// （#45 D8）。TransitionID の宣言 given から、より小さい priority の遷移群の
// given（＝先に評価されて捌かれる world）を除いた補集合を Excludes に持つ。
// 「この解決は宣言された priority に相対的で、実装の if/else 順との一致は非検証」。
type EffectiveGiven struct {
	TransitionID string   `json:"transitionId"`
	Priority     int      `json:"priority"`
	Given        []string `json:"given"`
	// Excludes は先行 priority 遷移群の given（否定される world・else の導出源）。
	Excludes []string `json:"excludes,omitempty"`
}

// Remainder is a transition acknowledged as the action's scoped default —
// declared either via the RemainderTagID tag (原型・互換読み) or, for an
// action whose transitions all declare a priority, as the last-evaluated
// (largest priority) transition (#45 D8's structural declarative remainder).
// Reported separately; never counted toward coverage
// (req.action-flow.acknowledged-remainder).
type Remainder struct {
	TransitionID string `json:"transitionId"`
}

// ScopeDisclosure is the mandatory "what this run does NOT prove" statement
// (req.action-flow.scope-honesty). It is always populated, even when there
// are zero gaps/overlaps, so the report can never read as a bare "no gaps".
type ScopeDisclosure struct {
	DeclaredAxes    []string `json:"declaredAxes"`
	UndeclaredGiven []string `json:"undeclaredGiven"`
	HasRemainder    bool     `json:"hasRemainder"`
	OutOfGuarantee  []string `json:"outOfGuarantee"`
}

// Report is the full `scholia flow <action>` result.
type Report struct {
	Action        string         `json:"action"`
	ActionLabel   string         `json:"actionLabel"`
	Matrix        Matrix         `json:"matrix"`
	SubsetShadows []SubsetShadow `json:"subsetShadows,omitempty"`
	Axes          []Axis         `json:"axes,omitempty"`
	// AxesAbsence explains why Axes is empty — AxesAbsenceNoneDeclared or
	// AxesAbsenceNotOnThisAction — and is empty whenever len(Axes) > 0 (#40
	// ①, eff.emit.scope-disclosure's (a)/(b) distinction).
	AxesAbsence string          `json:"axesAbsence,omitempty"`
	Cells       []Cell          `json:"cells,omitempty"`
	TotalGaps   []TotalGap      `json:"totalGaps,omitempty"`
	Overlaps    []Overlap       `json:"overlaps,omitempty"`
	Remainder   []Remainder     `json:"remainder,omitempty"`
	Scope       ScopeDisclosure `json:"scope"`
}

// disclosureBoilerplate are the fixed advisory captions
// req.action-flow.scope-honesty requires regardless of what a given action's
// analysis finds: what this tool structurally cannot verify.
var disclosureBoilerplate = []string{
	"排他の真偽（宣言軸の2値が実際に両立しないか）はツールが検査できません（authoring 上の申告を信じるのみ）。",
	"軸の網羅性（列挙した軸が action に関係する全ての区別を尽くしているか）はツールが検査できません。",
	"軸に属さない単独フラグ・連続量の条件は直積に入らず、その次元の穴は不可視です。",
	// #45 D8: 評価順による解決の相対性を常時開示（宣言 priority に相対的・実装
	// 一致は非検証）——honesty-first を宣言層まで貫く。
	"評価順と『解決済み』は宣言された priority に相対的です。実装の if/else 順との一致は検証していません。",
	// #45 D8/amend②: 宣言的残余（全遷移 priority 宣言 action の最後尾）も
	// acknowledged-remainder と同じく coverage に数えない別枠報告。
	"宣言的残余の受け皿は coverage に数えません（別枠報告）。acknowledged-remainder が宣言されている場合も同様です。",
}

// GapsReport is `scholia gaps <action>`'s focused JSON shape — the same fields
// WriteGapsText prints (subset-shadow・抜け・重なり・scope-disclosure),
// omitting the full matrix/axes/cells/remainder `scholia flow` shows
// (req.action-flow.axis-gaps: same analysis, holes-only surface).
type GapsReport struct {
	Action        string          `json:"action"`
	ActionLabel   string          `json:"actionLabel"`
	SubsetShadows []SubsetShadow  `json:"subsetShadows,omitempty"`
	TotalGaps     []TotalGap      `json:"totalGaps,omitempty"`
	Overlaps      []Overlap       `json:"overlaps,omitempty"`
	Scope         ScopeDisclosure `json:"scope"`
}

// Gaps projects a Report down to its GapsReport view.
func (r Report) Gaps() GapsReport {
	return GapsReport{
		Action:        r.Action,
		ActionLabel:   r.ActionLabel,
		SubsetShadows: r.SubsetShadows,
		TotalGaps:     r.TotalGaps,
		Overlaps:      r.Overlaps,
		Scope:         r.Scope,
	}
}

// Analyze builds the Report for one action id (req.action-flow). It never
// emits a bare "no gaps": Scope is always populated.
func Analyze(snap *store.Snapshot, ix *index.Index, actionID string) Report {
	txs := transitionsForAction(snap, actionID)

	report := Report{
		Action:      actionID,
		ActionLabel: vocabLabel(ix, actionID),
		Matrix:      buildMatrix(txs),
	}

	// Two remainder forms are handled differently (#45 D8/amend②):
	//
	//   - TAG remainder (RemainderTagID・原型): a declared catch-all with no
	//     priority. Removed from the "specifics" the direct/axis analysis runs
	//     over — it is a lowest-priority default, not a source of
	//     undefined-priority ambiguity, and would otherwise spawn spurious
	//     subset-shadows via its catch-all given.
	//
	//   - DECLARATIVE remainder (all-priority-declared action's tail): a REAL
	//     transition with a real given whose priority cleanly resolves the
	//     overlaps it participates in. It is reported separately and exempts
	//     L-total, but it STAYS in specifics so its evaluation-order resolution
	//     of overlaps/subset-shadows is not silently erased (the 旗艦
	//     act.user.update wants all 11 overlaps folded as resolved, not 10 with
	//     one vanishing because the tail was dropped from the cell).
	tagRemainderIDs, specifics := splitTagRemainder(txs)
	declTail, hasDeclarativeRemainder := declarativeRemainderTail(txs)
	for _, id := range tagRemainderIDs {
		report.Remainder = append(report.Remainder, Remainder{TransitionID: id})
	}
	if hasDeclarativeRemainder {
		// avoid double-reporting if the tail also carried the tag.
		alreadyReported := false
		for _, id := range tagRemainderIDs {
			if id == declTail {
				alreadyReported = true
				break
			}
		}
		if !alreadyReported {
			report.Remainder = append(report.Remainder, Remainder{TransitionID: declTail})
		}
	}
	hasRemainder := len(report.Remainder) > 0

	// subset-shadow/axis analysis run over "specifics" (all transitions minus
	// any TAG remainder; the declarative remainder stays in).
	report.SubsetShadows = subsetShadows(specifics)

	conditionUniverse := conditionsInGiven(specifics)
	axes := relevantAxes(snap, ix, conditionUniverse)
	report.Axes = axes

	if len(axes) > 0 {
		report.Cells = productCells(axes, specifics)
		if !hasDeclarativeRemainder {
			report.TotalGaps = totalGaps(axes, specifics)
		}
		report.Overlaps = overlaps(report.Cells, report.SubsetShadows, specifics)
	} else if anyAxisTagDeclared(snap.Config, ix) {
		report.AxesAbsence = AxesAbsenceNotOnThisAction
	} else {
		report.AxesAbsence = AxesAbsenceNoneDeclared
	}

	report.Scope = buildScope(axes, conditionUniverse, hasRemainder)

	// typed 容認（#45 D6）: 対象宛て decision の acknowledges で名指しされた
	// flow finding を「容認済み」に畳む（AcknowledgedBy を書き込む・消しはしない）。
	applyAcceptance(&report, snap.Decisions)
	return report
}

func transitionsForAction(snap *store.Snapshot, actionID string) []model.Transition {
	var out []model.Transition
	for _, t := range snap.Transitions {
		if t.Action == actionID {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func buildMatrix(txs []model.Transition) Matrix {
	condSet := make(map[string]bool)
	rows := make([]MatrixRow, 0, len(txs))
	for _, t := range txs {
		for _, g := range t.Given {
			condSet[g] = true
		}
		var prio *int
		if t.Priority != nil {
			p := *t.Priority
			prio = &p
		}
		rows = append(rows, MatrixRow{TransitionID: t.ID, Given: append([]string(nil), t.Given...), Then: append([]string(nil), t.Then...), Priority: prio})
	}
	conds := make([]string, 0, len(condSet))
	for c := range condSet {
		conds = append(conds, c)
	}
	sort.Strings(conds)
	return Matrix{Conditions: conds, Rows: rows}
}

// subsetShadows finds every pair whose given sets are in a strict subset
// relation — 100% sound, no declaration needed (req.action-flow.subset-shadow).
// A pair whose two transitions carry DISTINCT declared priorities is marked
// Resolved (#45 D8): the multi-fire still happens, but which "then" wins is
// no longer undefined — it's the smaller-priority (earlier-evaluated)
// transition. Pairs where priority is absent on either side, or where both
// declare the same priority, stay Resolved=false and are reported as before.
func subsetShadows(txs []model.Transition) []SubsetShadow {
	prio := priorityByID(txs)
	var out []SubsetShadow
	for i := range txs {
		for j := range txs {
			if i == j {
				continue
			}
			a, b := txs[i], txs[j]
			if isProperSubset(a.Given, b.Given) {
				s := SubsetShadow{Subset: a.ID, Superset: b.ID}
				if pa, okA := prio[a.ID]; okA {
					if pb, okB := prio[b.ID]; okB && pa != pb {
						s.Resolved = true
						if pa < pb {
							s.Winner = a.ID
						} else {
							s.Winner = b.ID
						}
					}
				}
				out = append(out, s)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Subset != out[j].Subset {
			return out[i].Subset < out[j].Subset
		}
		return out[i].Superset < out[j].Superset
	})
	return out
}

// priorityByID maps every transition that declares a priority to its value.
// Transitions with nil Priority are absent from the map — callers distinguish
// "declared" from "undeclared" by presence, never by a sentinel value.
func priorityByID(txs []model.Transition) map[string]int {
	m := make(map[string]int, len(txs))
	for _, t := range txs {
		if t.Priority != nil {
			m[t.ID] = *t.Priority
		}
	}
	return m
}

// isProperSubset reports whether a is a strict subset of b. The empty set is
// a proper subset of any non-empty set — a transition with no given fires in
// every world and therefore shadows every other transition of the action —
// so len(a)==0 only short-circuits when b is also empty (equal, not proper).
func isProperSubset(a, b []string) bool {
	if len(b) == 0 || len(a) >= len(b) {
		return false
	}
	bSet := make(map[string]bool, len(b))
	for _, x := range b {
		bSet[x] = true
	}
	for _, x := range a {
		if !bSet[x] {
			return false
		}
	}
	return true
}

// splitTagRemainder separates the (at most one, per convention) transition
// tagged RemainderTagID from the "specifics" the direct/axis analysis runs
// over — the tag remainder is a lowest-priority catch-all, reported separately
// and never counted toward coverage (req.action-flow.acknowledged-remainder).
// The declarative remainder (all-priority tail) is NOT removed here — see the
// comment in Analyze: it stays in specifics so its priority resolves overlaps.
func splitTagRemainder(txs []model.Transition) (remainderIDs []string, specifics []model.Transition) {
	for _, t := range txs {
		isRemainder := false
		for _, tagID := range t.Tags {
			if tagID == RemainderTagID {
				isRemainder = true
				break
			}
		}
		if isRemainder {
			remainderIDs = append(remainderIDs, t.ID)
		} else {
			specifics = append(specifics, t)
		}
	}
	return remainderIDs, specifics
}

// declarativeRemainderTail returns the id of the last-evaluated transition
// (largest priority number) when EVERY transition of the action declares a
// priority — the "全宣言 action の最後尾" that acts as the declarative
// remainder and is exempt from L-total (#45 D8/amend②). Returns ok=false when
// any transition is unpriority-declared (partial declaration never creates a
// remainder) or when there are fewer than 2 transitions (a lone transition is
// its own specific, not a catch-all). Ties on the max priority also yield
// ok=false — an ambiguous tail is not a well-defined single remainder.
func declarativeRemainderTail(txs []model.Transition) (string, bool) {
	if len(txs) < 2 {
		return "", false
	}
	maxP := 0
	tail := ""
	tie := false
	for _, t := range txs {
		if t.Priority == nil {
			return "", false // partial declaration: no declarative remainder
		}
		p := *t.Priority
		switch {
		case p > maxP:
			maxP, tail, tie = p, t.ID, false
		case p == maxP:
			tie = true
		}
	}
	if tie {
		return "", false
	}
	return tail, true
}

func conditionsInGiven(txs []model.Transition) []string {
	set := make(map[string]bool)
	for _, t := range txs {
		for _, g := range t.Given {
			set[g] = true
		}
	}
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// anyAxisTagDeclared reports whether the store has at least one axis-behavior
// tag anywhere — distinguishes (a) the axis mechanism being wholly unused in
// this project from (b) axes existing but not reaching the analyzed action
// (#40 ①, eff.emit.scope-disclosure). Axis-ness is decided through cfg
// (#45 D9), not a literal kind string.
func anyAxisTagDeclared(cfg model.Config, ix *index.Index) bool {
	for _, tag := range ix.TagByID {
		if isAxisTag(cfg, tag) {
			return true
		}
	}
	return false
}

// axisTagsOf returns the axis-behavior tags directly attached to a vocab
// entry's Tags[] (no ancestor expansion — axis membership is a direct,
// possibly-multiple assignment, DESIGN §3.4/§7.1). Axis-ness is decided through
// cfg (#45 D9).
func axisTagsOf(cfg model.Config, ix *index.Index, vocabID string) []model.Tag {
	v, ok := ix.VocabByID[vocabID]
	if !ok {
		return nil
	}
	var out []model.Tag
	for _, tagID := range v.Tags {
		if tag, ok := ix.TagByID[tagID]; ok && isAxisTag(cfg, tag) {
			out = append(out, tag)
		}
	}
	return out
}

// relevantAxes returns, in id order, every axis tag that at least one
// condition in conditionUniverse carries — "対象 action の given に現れる条件が
// 属する宣言 axis タグだけを軸にする" (design-options §7.3). Each axis's Values
// is every condition project-wide carrying that axis tag (the axis's value
// set is a property of the axis declaration, not of this one action).
func relevantAxes(snap *store.Snapshot, ix *index.Index, conditionUniverse []string) []Axis {
	relevant := make(map[string]model.Tag)
	for _, condID := range conditionUniverse {
		for _, tag := range axisTagsOf(snap.Config, ix, condID) {
			relevant[tag.ID] = tag
		}
	}
	if len(relevant) == 0 {
		return nil
	}

	values := make(map[string][]string, len(relevant))
	for _, v := range snap.Vocab {
		if v.Category != model.CategoryCondition {
			continue
		}
		for _, tagID := range v.Tags {
			if _, ok := relevant[tagID]; ok {
				values[tagID] = append(values[tagID], v.ID)
			}
		}
	}

	ids := make([]string, 0, len(relevant))
	for id := range relevant {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	axes := make([]Axis, 0, len(ids))
	for _, id := range ids {
		vals := append([]string(nil), values[id]...)
		sort.Strings(vals)
		axes = append(axes, Axis{ID: id, Name: relevant[id].Name, Total: relevant[id].Total, Values: vals})
	}
	return axes
}

// axisSpan returns, for one transition and one axis, the set of that axis's
// values the transition's given pins to. An empty result means the
// transition does not reference the axis at all — it is "free" on that
// dimension and, for cell-coverage purposes, matches every value of it
// (design-options §7.3 worked reconstruction of act.user.update).
func axisSpan(t model.Transition, axis Axis) map[string]bool {
	given := make(map[string]bool, len(t.Given))
	for _, g := range t.Given {
		given[g] = true
	}
	pinned := make(map[string]bool)
	for _, v := range axis.Values {
		if given[v] {
			pinned[v] = true
		}
	}
	if len(pinned) > 0 {
		return pinned
	}
	all := make(map[string]bool, len(axis.Values))
	for _, v := range axis.Values {
		all[v] = true
	}
	return all
}

// productCells enumerates the bounded cartesian product of relevant axes'
// values (never 2^n — only the declared axes) and, for each cell, the
// transitions covering it.
func productCells(axes []Axis, txs []model.Transition) []Cell {
	combos := []map[string]string{{}}
	for _, axis := range axes {
		if len(axis.Values) == 0 {
			continue
		}
		var next []map[string]string
		for _, combo := range combos {
			for _, val := range axis.Values {
				c := make(map[string]string, len(combo)+1)
				for k, v := range combo {
					c[k] = v
				}
				c[axis.ID] = val
				next = append(next, c)
			}
		}
		combos = next
	}

	cells := make([]Cell, 0, len(combos))
	for _, combo := range combos {
		var covering []string
		for _, t := range txs {
			if transitionCoversCell(t, axes, combo) {
				covering = append(covering, t.ID)
			}
		}
		sort.Strings(covering)
		cells = append(cells, Cell{Values: combo, Transitions: covering})
	}
	sort.Slice(cells, func(i, j int) bool { return cellKey(cells[i].Values, axes) < cellKey(cells[j].Values, axes) })
	return cells
}

func transitionCoversCell(t model.Transition, axes []Axis, combo map[string]string) bool {
	for _, axis := range axes {
		want, ok := combo[axis.ID]
		if !ok {
			continue
		}
		if !axisSpan(t, axis)[want] {
			return false
		}
	}
	return true
}

func cellKey(combo map[string]string, axes []Axis) string {
	key := ""
	for _, axis := range axes {
		key += axis.ID + "=" + combo[axis.ID] + ";"
	}
	return key
}

// totalGaps finds, for every total=true axis, values that never appear in
// the given of any transition of this action — the one "clean" sound signal
// (L-total, req.action-flow.axis-gaps).
func totalGaps(axes []Axis, txs []model.Transition) []TotalGap {
	present := make(map[string]bool)
	for _, t := range txs {
		for _, g := range t.Given {
			present[g] = true
		}
	}
	var out []TotalGap
	for _, axis := range axes {
		if !axis.Total {
			continue
		}
		for _, v := range axis.Values {
			if !present[v] {
				out = append(out, TotalGap{AxisID: axis.ID, Value: v})
			}
		}
	}
	return out
}

// overlaps reports cells covered by 2+ transitions, excluding pairs already
// explained by a proven SubsetShadow (req.action-flow.axis-gaps' 重なり is
// "宣言軸に相対的に sound", distinct from and non-duplicative of subset-shadow).
//
// Exclusion is decided per PAIR, not per transition: a transition dropped
// from one shadow pair can still carry a real, unexplained ambiguity against
// a third transition in the same cell (e.g. A⊊B but C is incomparable to
// both — A↔C and B↔C are real ambiguities the subset relation does not
// explain). Dropping a whole transition whenever *any* one of its pairs is a
// shadow would silently erase that remaining ambiguity, so the fix reports
// every transition that appears in at least one non-shadow pair.
//
// #45 D8: an overlap whose involved transitions ALL carry DISTINCT declared
// priorities is marked Resolved — the cell is still contended, but evaluation
// order (smallest priority first) settles which "then" wins, so it is no
// longer an undefined-priority hole. Any undeclared priority among the
// involved transitions, or any two sharing the same priority, leaves the whole
// overlap Resolved=false (conservative: one un-orderable pair poisons the
// group — this is where real holes still land). Resolved overlaps also carry
// the derived complement (EffectiveGiven), disclosed under --verbose.
func overlaps(cells []Cell, shadows []SubsetShadow, txs []model.Transition) []Overlap {
	shadowPair := make(map[[2]string]bool, len(shadows)*2)
	for _, s := range shadows {
		shadowPair[[2]string{s.Subset, s.Superset}] = true
		shadowPair[[2]string{s.Superset, s.Subset}] = true
	}
	prio := priorityByID(txs)
	givenByID := make(map[string][]string, len(txs))
	for _, t := range txs {
		givenByID[t.ID] = t.Given
	}

	var out []Overlap
	for _, cell := range cells {
		if len(cell.Transitions) < 2 {
			continue
		}
		involved := make(map[string]bool)
		for i := 0; i < len(cell.Transitions); i++ {
			for j := i + 1; j < len(cell.Transitions); j++ {
				a, b := cell.Transitions[i], cell.Transitions[j]
				if shadowPair[[2]string{a, b}] {
					continue
				}
				involved[a] = true
				involved[b] = true
			}
		}
		if len(involved) < 2 {
			continue
		}
		unexplained := make([]string, 0, len(involved))
		for t := range involved {
			unexplained = append(unexplained, t)
		}
		sort.Strings(unexplained)
		o := Overlap{Cell: cell.Values, Transitions: unexplained}
		if resolved, ordered := allDistinctPriorities(unexplained, prio); resolved {
			o.Resolved = true
			o.EffectiveGiven = deriveComplement(ordered, prio, givenByID)
		}
		out = append(out, o)
	}
	return out
}

// allDistinctPriorities reports whether every id in the group declares a
// priority AND all priorities are pairwise distinct (#45 D8's conservative
// resolution predicate). On success it also returns the ids sorted by
// ascending priority (evaluation order). One undeclared or duplicated
// priority fails the whole check.
func allDistinctPriorities(ids []string, prio map[string]int) (ok bool, ordered []string) {
	seen := make(map[int]bool, len(ids))
	ordered = make([]string, 0, len(ids))
	for _, id := range ids {
		p, declared := prio[id]
		if !declared {
			return false, nil
		}
		if seen[p] {
			return false, nil
		}
		seen[p] = true
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(i, j int) bool { return prio[ordered[i]] < prio[ordered[j]] })
	return true, ordered
}

// deriveComplement builds the effective given of each transition in an
// evaluation-ordered group (#45 D8): transition at priority p is only reached
// in worlds NOT already captured by any smaller-priority transition, so its
// effective given is (declared given) ∧ ¬(union of earlier transitions'
// givens). The negated part is surfaced as Excludes — the else the tool
// derives from priority, never verified against the implementation.
func deriveComplement(ordered []string, prio map[string]int, givenByID map[string][]string) []EffectiveGiven {
	out := make([]EffectiveGiven, 0, len(ordered))
	var excludes []string
	seenExcl := make(map[string]bool)
	for _, id := range ordered {
		eg := EffectiveGiven{
			TransitionID: id,
			Priority:     prio[id],
			Given:        append([]string(nil), givenByID[id]...),
		}
		if len(excludes) > 0 {
			eg.Excludes = append([]string(nil), excludes...)
		}
		out = append(out, eg)
		// accumulate this transition's given into the running exclusion set
		// for all later (larger-priority) transitions.
		for _, g := range givenByID[id] {
			if !seenExcl[g] {
				seenExcl[g] = true
				excludes = append(excludes, g)
			}
		}
		sort.Strings(excludes)
	}
	return out
}

func buildScope(axes []Axis, conditionUniverse []string, hasRemainder bool) ScopeDisclosure {
	declared := make([]string, 0, len(axes))
	inAxis := make(map[string]bool)
	for _, axis := range axes {
		declared = append(declared, axis.ID)
		for _, v := range axis.Values {
			inAxis[v] = true
		}
	}
	var undeclared []string
	for _, c := range conditionUniverse {
		if !inAxis[c] {
			undeclared = append(undeclared, c)
		}
	}
	return ScopeDisclosure{
		DeclaredAxes:    declared,
		UndeclaredGiven: undeclared,
		HasRemainder:    hasRemainder,
		OutOfGuarantee:  disclosureBoilerplate,
	}
}

func vocabLabel(ix *index.Index, id string) string {
	if v, ok := ix.VocabByID[id]; ok {
		return v.Label
	}
	return "?"
}
