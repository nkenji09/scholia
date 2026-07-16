// Package flow derives an action-scoped "given → then" analysis (#39
// action-flow) shared by the CLI (`pmem flow`) and, in a later phase, the
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

	"github.com/nkenji09/product-memory/internal/index"
	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

// AxisTagKind is the tagKind that marks a tag record as an axis declaration
// (config.tagKinds must declare it; DESIGN §3.4).
const AxisTagKind = "axis"

// RemainderTagID is the well-known tag id a transition carries to declare
// itself as the action's single "acknowledged remainder" default
// (req.action-flow.acknowledged-remainder). No transition in this repo's own
// .pmem uses it yet — the convention is introduced by this implementation
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
}

// Remainder is a transition acknowledged (via RemainderTagID) as the
// action's single scoped default. Reported separately; never counted toward
// coverage (req.action-flow.acknowledged-remainder).
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

// Report is the full `pmem flow <action>` result.
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
	"acknowledged-remainder が宣言されている場合、その受け皿は coverage に数えません（別枠報告）。",
}

// GapsReport is `pmem gaps <action>`'s focused JSON shape — the same fields
// WriteGapsText prints (subset-shadow・抜け・重なり・scope-disclosure),
// omitting the full matrix/axes/cells/remainder `pmem flow` shows
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

	remainderIDs, specifics := splitRemainder(txs)
	for _, id := range remainderIDs {
		report.Remainder = append(report.Remainder, Remainder{TransitionID: id})
	}

	// subset-shadow/axis analysis run over "specifics" only — the
	// acknowledged remainder is a declared lowest-priority catch-all, not a
	// source of undefined-priority ambiguity, and design says it must never
	// count toward coverage (req.action-flow.acknowledged-remainder).
	report.SubsetShadows = subsetShadows(specifics)

	conditionUniverse := conditionsInGiven(specifics)
	axes := relevantAxes(snap, ix, conditionUniverse)
	report.Axes = axes

	if len(axes) > 0 {
		report.Cells = productCells(axes, specifics)
		report.TotalGaps = totalGaps(axes, specifics)
		report.Overlaps = overlaps(report.Cells, report.SubsetShadows)
	} else if anyAxisTagDeclared(ix) {
		report.AxesAbsence = AxesAbsenceNotOnThisAction
	} else {
		report.AxesAbsence = AxesAbsenceNoneDeclared
	}

	report.Scope = buildScope(axes, conditionUniverse, len(remainderIDs) > 0)
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
		rows = append(rows, MatrixRow{TransitionID: t.ID, Given: append([]string(nil), t.Given...), Then: append([]string(nil), t.Then...)})
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
func subsetShadows(txs []model.Transition) []SubsetShadow {
	var out []SubsetShadow
	for i := range txs {
		for j := range txs {
			if i == j {
				continue
			}
			a, b := txs[i], txs[j]
			if isProperSubset(a.Given, b.Given) {
				out = append(out, SubsetShadow{Subset: a.ID, Superset: b.ID})
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

// splitRemainder separates the (at most one, per convention) transition
// tagged RemainderTagID from the "specifics" the direct/axis analysis runs
// over — the remainder is reported separately and never counted toward
// coverage (req.action-flow.acknowledged-remainder).
func splitRemainder(txs []model.Transition) (remainderIDs []string, specifics []model.Transition) {
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

// anyAxisTagDeclared reports whether the store has at least one kind="axis"
// tag anywhere — distinguishes (a) the axis mechanism being wholly unused in
// this project from (b) axes existing but not reaching the analyzed action
// (#40 ①, eff.emit.scope-disclosure).
func anyAxisTagDeclared(ix *index.Index) bool {
	for _, tag := range ix.TagByID {
		if tag.Kind == AxisTagKind {
			return true
		}
	}
	return false
}

// axisTagsOf returns the kind="axis" tags directly attached to a vocab
// entry's Tags[] (no ancestor expansion — axis membership is a direct,
// possibly-multiple assignment, DESIGN §3.4/§7.1).
func axisTagsOf(ix *index.Index, vocabID string) []model.Tag {
	v, ok := ix.VocabByID[vocabID]
	if !ok {
		return nil
	}
	var out []model.Tag
	for _, tagID := range v.Tags {
		if tag, ok := ix.TagByID[tagID]; ok && tag.Kind == AxisTagKind {
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
		for _, tag := range axisTagsOf(ix, condID) {
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
func overlaps(cells []Cell, shadows []SubsetShadow) []Overlap {
	shadowPair := make(map[[2]string]bool, len(shadows)*2)
	for _, s := range shadows {
		shadowPair[[2]string{s.Subset, s.Superset}] = true
		shadowPair[[2]string{s.Superset, s.Subset}] = true
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
		out = append(out, Overlap{Cell: cell.Values, Transitions: unexplained})
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
