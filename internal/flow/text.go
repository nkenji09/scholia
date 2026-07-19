package flow

import (
	"fmt"
	"io"
	"strings"
)

// WriteText renders a Report for human reading. Every section prints its own
// scope so nothing reads as a bare, unqualified "no gaps"
// (req.action-flow.scope-honesty): counts of zero are always shown next to
// what was actually checked, never omitted as if there were nothing to say.
//
// verbose (#45 D8) additionally discloses evaluation-order-resolved overlaps
// and subset-shadows (folded out of the default surface) plus each resolved
// overlap's derived complement (実効 given・非検証). The default (verbose=false)
// surface shows only unresolved holes — the honest "本物の穴" placement.
func WriteText(w io.Writer, r Report, verbose bool) {
	fmt.Fprintf(w, "# %s (%s)\n\n", r.ActionLabel, r.Action)
	writeMatrixSection(w, r)
	writeSubsetShadowSection(w, r, verbose)
	writeAxesSection(w, r)
	writeTotalGapsSection(w, r)
	writeOverlapsSection(w, r, verbose)
	writeRemainderSection(w, r)
	writeScopeSection(w, r)
}

// WriteGapsText renders the gap-only view (`scholia gaps <action>`,
// req.action-flow.axis-gaps' focused surface): subset-shadow・抜け・重なり
// only, never the full matrix — but scope-disclosure is still mandatory
// (req.action-flow.scope-honesty forbids a bare "no gaps" here too).
func WriteGapsText(w io.Writer, r Report, verbose bool) {
	fmt.Fprintf(w, "# %s (%s) — gaps\n\n", r.ActionLabel, r.Action)
	writeSubsetShadowSection(w, r, verbose)
	writeTotalGapsSection(w, r)
	writeOverlapsSection(w, r, verbose)
	writeScopeSection(w, r)
}

func writeMatrixSection(w io.Writer, r Report) {
	fmt.Fprintln(w, "## マトリクス（可視化・網羅を主張しない）")
	if len(r.Matrix.Rows) == 0 {
		fmt.Fprintln(w, "(この action を持つ遷移はありません)")
	} else {
		fmt.Fprintf(w, "条件: %s\n", strings.Join(r.Matrix.Conditions, "、"))
		for _, row := range r.Matrix.Rows {
			prio := ""
			if row.Priority != nil {
				prio = fmt.Sprintf(" [評価順 p%d]", *row.Priority)
			}
			fmt.Fprintf(w, "  - %s%s: GIVEN %s THEN %s\n", row.TransitionID, prio, strings.Join(row.Given, " ∧ "), strings.Join(row.Then, " → "))
		}
	}
	fmt.Fprintln(w)
}

func writeSubsetShadowSection(w io.Writer, r Report, verbose bool) {
	var unresolved, resolved []SubsetShadow
	for _, s := range r.SubsetShadows {
		if s.Resolved {
			resolved = append(resolved, s)
		} else {
			unresolved = append(unresolved, s)
		}
	}
	fmt.Fprintf(w, "## subset-shadow（証明可能な重複）: %d 件\n", len(unresolved))
	for _, s := range unresolved {
		fmt.Fprintf(w, "  - %s ⊊ %s: %s が発火する world では %s も必ず発火します（優先順位未定義）%s\n",
			s.Subset, s.Superset, s.Superset, s.Subset, ackSuffix(s.AcknowledgedBy))
	}
	if len(resolved) > 0 {
		fmt.Fprintf(w, "  （評価順で解決済み: %d 件・--verbose で開示）\n", len(resolved))
	}
	if verbose {
		for _, s := range resolved {
			fmt.Fprintf(w, "  - [解決済み] %s ⊊ %s: 両者は発火しますが評価順で %s が先（宣言 priority に相対的・実装一致は非検証）%s\n",
				s.Subset, s.Superset, s.Winner, ackSuffix(s.AcknowledgedBy))
		}
	}
	fmt.Fprintln(w)
}

// ackSuffix は typed 容認（#45 D6）で畳んだ finding に付ける「容認済み」注記。
func ackSuffix(decisionID string) string {
	if decisionID == "" {
		return ""
	}
	return fmt.Sprintf("【容認済み: decision %s】", decisionID)
}

func writeAxesSection(w io.Writer, r Report) {
	fmt.Fprintf(w, "## 宣言軸: %d 件", len(r.Axes))
	if len(r.Axes) == 0 {
		fmt.Fprintln(w)
		switch r.AxesAbsence {
		case AxesAbsenceNoneDeclared:
			fmt.Fprintln(w, "  store に kind=\"axis\" のタグが1枚もありません（軸機構が未導入＝軸注釈による gap 検出は範囲外）。`scholia tag create --kind axis ...` で軸を作れます。")
		case AxesAbsenceNotOnThisAction:
			fmt.Fprintln(w, "  軸タグはありますが、この action のどの遷移の given にも軸条件が載っていません（軸が this action に効いていません）。条件別に given を張って（＝畳んだ遷移を条件別に割って）軸を効かせてください。")
		default:
			fmt.Fprintln(w, "（この action の given に axis タグを持つ条件がありません＝軸注釈による gap 検出は範囲外）")
		}
	} else {
		fmt.Fprintln(w)
		for _, a := range r.Axes {
			totalLabel := "total=false"
			if a.Total {
				totalLabel = "total=true"
			}
			fmt.Fprintf(w, "  - %s (%s・%s): 値=%s\n", a.ID, a.Name, totalLabel, strings.Join(a.Values, "、"))
		}
		fmt.Fprintf(w, "  cell 数（宣言軸の直積・有界）: %d\n", len(r.Cells))
	}
	fmt.Fprintln(w)
}

func writeTotalGapsSection(w io.Writer, r Report) {
	fmt.Fprintf(w, "## 抜け（L-total・唯一 clean に sound）: %d 件\n", len(r.TotalGaps))
	for _, g := range r.TotalGaps {
		fmt.Fprintf(w, "  - 軸 %s: 値 %s を given に持つ遷移が1つもありません%s\n", g.AxisID, g.Value, ackSuffix(g.AcknowledgedBy))
	}
	fmt.Fprintln(w)
}

func writeOverlapsSection(w io.Writer, r Report, verbose bool) {
	var unresolved, resolved []Overlap
	for _, o := range r.Overlaps {
		if o.Resolved {
			resolved = append(resolved, o)
		} else {
			unresolved = append(unresolved, o)
		}
	}
	fmt.Fprintf(w, "## 重なり（宣言軸に相対的に sound な ambiguity）: %d 件\n", len(unresolved))
	for _, o := range unresolved {
		fmt.Fprintf(w, "  - cell %s: %s が同じ状況を取り合っています（優先順位未定義）%s\n",
			formatCell(o.Cell), strings.Join(o.Transitions, "、"), ackSuffix(o.AcknowledgedBy))
	}
	if len(resolved) > 0 {
		fmt.Fprintf(w, "  （評価順で解決済み: %d 件・--verbose で開示）\n", len(resolved))
	}
	if verbose {
		for _, o := range resolved {
			fmt.Fprintf(w, "  - [解決済み] cell %s: %s が取り合いますが評価順で解決（宣言 priority に相対的・実装一致は非検証）%s\n",
				formatCell(o.Cell), strings.Join(o.Transitions, "、"), ackSuffix(o.AcknowledgedBy))
			for _, eg := range o.EffectiveGiven {
				line := fmt.Sprintf("      p%d %s: 実効 given = %s", eg.Priority, eg.TransitionID, strings.Join(eg.Given, " ∧ "))
				if len(eg.Excludes) > 0 {
					line += " ∧ ¬(" + strings.Join(eg.Excludes, " ∨ ") + ")"
				}
				fmt.Fprintf(w, "%s\n", line)
			}
		}
	}
	fmt.Fprintln(w)
}

func writeRemainderSection(w io.Writer, r Report) {
	fmt.Fprintf(w, "## acknowledged-remainder（coverage に数えない）: %d 件\n", len(r.Remainder))
	for _, rem := range r.Remainder {
		fmt.Fprintf(w, "  - %s\n", rem.TransitionID)
	}
	fmt.Fprintln(w)
}

func writeScopeSection(w io.Writer, r Report) {
	fmt.Fprintln(w, "## scope-disclosure（保証の外・削れない必須項目）")
	fmt.Fprintf(w, "  列挙した軸: %s\n", noneIfEmpty(r.Scope.DeclaredAxes))
	fmt.Fprintf(w, "  don't-care 扱いの条件（軸未宣言の given）: %s\n", noneIfEmpty(r.Scope.UndeclaredGiven))
	for _, line := range r.Scope.OutOfGuarantee {
		fmt.Fprintf(w, "  - %s\n", line)
	}
}

func formatCell(cell map[string]string) string {
	keys := make([]string, 0, len(cell))
	for k := range cell {
		keys = append(keys, k)
	}
	// deterministic ordering
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+cell[k])
	}
	return strings.Join(parts, ", ")
}

func noneIfEmpty(ss []string) string {
	if len(ss) == 0 {
		return "(なし)"
	}
	return strings.Join(ss, "、")
}
