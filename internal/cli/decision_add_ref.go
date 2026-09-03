package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/lint"
)

// newDecisionAddRefCmd は既存 decision の refs[] に追記専用で足す
// （01M1K4WPN3HXNVE0CR0T6NQK33）。
//
// # なぜこの口が要るのか — 判断欄位の ref は後から書けない
//
// 判断を記録する時点で PR はまだ存在しない。PR はその後に作る。
// ところが判断欄位の `ref` は凍結されていて（`internal/diff` の欄位分類が変更を
// 違反として落とす）、**後から PR の URL を入れることが原理的にできなかった。**
// 「後から辿り先を足せる欄が commits[] しか無かった」ことが、git hash を来歴の
// 正本にしていた理由である——取り込みで壊れると分かっていながら。
//
// この口が空くと、その制約が消える。**URL は git のオブジェクトではないので、
// squash merge でも作業ブランチの作り直しでも壊れない。**
//
// # 落ちない範囲（射程・正直に名乗る）
//
//   - 🔴 **URL が生きていることは誰も保証しない。** リポジトリを移せば PR の URL は
//     死ぬ。`ref-freshness` は `file:line` しか見ておらず、**URL の死活は検査しない。**
//   - **打つのを守らせる歯止めは無い。** 呼ばなくても作業は完了する
//     （01KXS68HCNQ0H9QKNYFQ869J19 の型）。代わりに `decision list --unlinked` が
//     「commits も refs も空」を数えられるようにしてある。
//   - **形の検査をしない。** URL でも issue 番号でもチケット id でも通す。
//     何が正しい辿り先かはプロジェクトごとに違い、機械で決められない。
func newDecisionAddRefCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "add-ref <decisionId> <ref> [<ref>...]",
		Short: "decision に実装来歴の外部参照（PR・issue・チケットの URL）を追記する（追加専用・判断フィールドは不変）",
		Long: "decision に実装来歴の外部参照を追記する（追加専用・判断フィールドは不変）。\n\n" +
			"判断欄位の ref（1 個・凍結）とは別の欄で、着地後に何件でも足せる。\n" +
			"URL は git のオブジェクトではないので、squash merge でも壊れない\n" +
			"——commit hash を結ぶ `add-commit` の代わりに、こちらを使う。",
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			refs := args[1:]

			s, err := openStore()
			if err != nil {
				return err
			}
			d, err := s.LoadDecision(id)
			if err != nil {
				return fmt.Errorf("decision %q を読み込めません: %w", id, err)
			}
			d.Refs = dedupeAppend(d.Refs, refs)

			snap, err := s.LoadAll()
			if err != nil {
				return err
			}
			advisories, allowed, gateErr := runWriteGate(cmd, snap, lint.WriteOp{Decision: &d, IsNew: false}, nil)
			if gateErr != nil {
				return gateErr
			}
			// desc 現在形ゲート三点配線と同じ扱い: 実装結線と同一ターンに、対象 desc の
			// 鮮度を advisory で気づかせる（add-commit と同型）。
			advisories = append(advisories, lint.TargetDescStaleTense(snap, d.Target)...)

			saved, err := s.UpdateDecision(d)
			if err != nil {
				return err
			}

			if asJSON {
				return emitWriteJSON(cmd, saved, advisories, allowed, false)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "decision %s に refs を追加しました（refs=%d 件）\n", id, len(saved.Refs))
			printWriteGateText(cmd, allowed, advisories)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "更新後のレコードを応答封筒 { record, advisories } の JSON で出力する")
	return cmd
}
