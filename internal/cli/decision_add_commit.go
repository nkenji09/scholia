package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/lint"
)

// newDecisionAddCommitCmd は既存 decision の commits[] に追記専用で足す
// （§3.5 append-only の精緻化）。target/why/changed/ref/at ら判断フィールドは
// 一切書き換えない — 実装ミス直し等で decision を無駄に増やさないための経路。
func newDecisionAddCommitCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "add-commit <decisionId> <hash> [<hash>...]",
		Short: "decision に実装コミットを追記する（追加専用・判断フィールドは不変・§3.5）",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			hashes := args[1:]

			s, err := openStore()
			if err != nil {
				return err
			}
			d, err := s.LoadDecision(id)
			if err != nil {
				return fmt.Errorf("decision %q を読み込めません: %w", id, err)
			}

			d.Commits = dedupeAppend(d.Commits, hashes)

			// 書き込みゲート二層（#45 U3）: add-commit に reject 規則は無い
			//（commits 追記のみ・判断欄位は不変）。既存 why/changed への
			// advisory は acknowledge-only として同一ターンに表示される。
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}
			advisories, allowed, gateErr := runWriteGate(cmd, snap, lint.WriteOp{Decision: &d, IsNew: false}, nil)
			if gateErr != nil {
				return gateErr
			}
			if err := s.SaveDecision(d); err != nil {
				return err
			}
			saved, err := s.LoadDecision(id)
			if err != nil {
				return err
			}

			if asJSON {
				return emitWriteJSON(cmd, saved, advisories, allowed, false)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "decision %s に commits を追加しました（commits=%d 件）\n", id, len(saved.Commits))
			printWriteGateText(cmd, allowed, advisories)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "更新後のレコードを応答封筒 { record, advisories } の JSON で出力する")
	return cmd
}
