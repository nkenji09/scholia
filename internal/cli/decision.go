package cli

import "github.com/spf13/cobra"

// newDecisionCmd は decision レコードそのものを操作するコマンド群（名詞）。
// `scholia decide`（動詞・新規記録）とは別に切る — decision を無駄に増やさず
// 実装来歴だけを結ぶための追加専用サブコマンドの置き場（§3.5）。
func newDecisionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decision",
		Short: "既存の意思決定（decision）レコードを操作する",
	}
	cmd.AddCommand(newDecisionAddCommitCmd())
	cmd.AddCommand(newDecisionListCmd())
	return cmd
}

// dedupeAppend は existing の後ろに additions を足し、重複（既出の値）を
// 落として返す。並びは初出順を保つ（commit hash は着地順に意味があるため、
// SaveTransition の given のようなソートはしない）。
func dedupeAppend(existing, additions []string) []string {
	seen := make(map[string]bool, len(existing)+len(additions))
	out := make([]string, 0, len(existing)+len(additions))
	for _, v := range existing {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	for _, v := range additions {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
