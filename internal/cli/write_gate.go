// write_gate.go — 書き込みゲート二層の CLI 配線（#45 U3/P3）。
//
// 検査コアは internal/lint（lint.CheckWrite）。ここは cobra 層の共通配線:
// --allow/--reason フラグ・reject の exit 1・allow の記録（stdout と --json）・
// advisory の同一ターン表示・--json 応答封筒 { record, advisories }。
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/lint"
	"github.com/nkenji09/scholia/internal/store"
)

// gateFlags は reject の逃し弁（--allow は理由必須・稀な例外）。reject が
// 起き得ないコマンド（vocab edit/tag・decide・decision add-commit）には
// 付けない——フラグの常在は --allow の常用化を招く。
type gateFlags struct {
	allow  []string
	reason string
}

func addGateAllowFlags(cmd *cobra.Command) *gateFlags {
	g := &gateFlags{}
	cmd.Flags().StringArrayVar(&g.allow, "allow", nil,
		"reject 規則を明示に破って保存する（exclusive-violation|total-kind-mismatch|id-policy・複数指定可・--reason 必須。使用は stdout と --json に記録される）")
	cmd.Flags().StringVar(&g.reason, "reason", "", "--allow の理由（--allow 指定時は必須）")
	return g
}

// allowedReject は --allow で明示に破った reject の記録（stdout と --json の
// allowed[] に残す。歯止め台帳への記録は U4/束4）。
type allowedReject struct {
	Rule     string         `json:"rule"`
	Reason   string         `json:"reason"`
	Findings []lint.Finding `json:"findings"`
}

// runWriteGate は保存前検査を実行し、--allow で解除されない reject が残る
// 場合はエラー（＝保存せず exit 1）を返す。エラーでなければ、保存後に表示・
// 記録すべき advisories と allow 記録を返す。
func runWriteGate(cmd *cobra.Command, snap store.Snapshot, op lint.WriteOp, g *gateFlags) (advisories []lint.Finding, allowed []allowedReject, err error) {
	var allowRules []string
	var reason string
	if g != nil {
		allowRules, reason = g.allow, g.reason
	}
	valid := lint.GateRejectRuleNames()
	for _, a := range allowRules {
		if !containsStr(valid, a) {
			return nil, nil, fmt.Errorf("--allow %q は reject 規則ではありません（有効: %s）", a, strings.Join(valid, "|"))
		}
	}
	if len(allowRules) > 0 && reason == "" {
		return nil, nil, fmt.Errorf("--allow には --reason（理由）が必須です")
	}

	res := lint.CheckWrite(snap, op)

	byRule := make(map[string][]lint.Finding)
	var ruleOrder []string
	for _, f := range res.Rejections {
		if _, ok := byRule[f.Rule]; !ok {
			ruleOrder = append(ruleOrder, f.Rule)
		}
		byRule[f.Rule] = append(byRule[f.Rule], f)
	}

	var blocked []lint.Finding
	for _, rule := range ruleOrder {
		if containsStr(allowRules, rule) {
			allowed = append(allowed, allowedReject{Rule: rule, Reason: reason, Findings: byRule[rule]})
		} else {
			blocked = append(blocked, byRule[rule]...)
		}
	}
	if len(blocked) > 0 {
		errOut := cmd.ErrOrStderr()
		for _, f := range blocked {
			fmt.Fprintf(errOut, "reject(%s): %s\n", f.Rule, f.Message)
		}
		return nil, nil, fmt.Errorf("reject: %d 件の不変条件違反のため保存しませんでした（--allow <rule> --reason <理由> は稀な逃し弁）", len(blocked))
	}
	return res.Advisories, allowed, nil
}

// printWriteGateText は非 JSON 出力の保存後表示: allow の記録行と
// `advisory(rule): message` 行（同一ターン警告）。
func printWriteGateText(cmd *cobra.Command, allowed []allowedReject, advisories []lint.Finding) {
	out := cmd.OutOrStdout()
	for _, a := range allowed {
		fmt.Fprintf(out, "allow(%s): %s — reject を明示に破って保存しました\n", a.Rule, a.Reason)
		for _, f := range a.Findings {
			fmt.Fprintf(out, "  %s\n", f.Message)
		}
	}
	for _, f := range advisories {
		fmt.Fprintf(out, "advisory(%s): %s\n", f.Rule, f.Message)
		if f.Suggestion != "" {
			fmt.Fprintf(out, "  → 修正候補: %s\n", f.Suggestion)
		}
	}
}

// writeEnvelope は書き込み系 --json の応答封筒（#45 U3・生レコード出力からの
// 形状変更を承知の一括変更。skill の --json 記述と対）。record は保存済み
// レコード・advisories は同一ターン警告（常在・空でも []）・allowed は
// --allow の記録・dryRun は decide --dry-run のみ true。
type writeEnvelope struct {
	Record     any             `json:"record"`
	Advisories []lint.Finding  `json:"advisories"`
	Allowed    []allowedReject `json:"allowed,omitempty"`
	DryRun     bool            `json:"dryRun,omitempty"`
}

func emitWriteJSON(cmd *cobra.Command, record any, advisories []lint.Finding, allowed []allowedReject, dryRun bool) error {
	if advisories == nil {
		advisories = []lint.Finding{}
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(writeEnvelope{Record: record, Advisories: advisories, Allowed: allowed, DryRun: dryRun})
}
