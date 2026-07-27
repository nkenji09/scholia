package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/model"
)

// newDecisionLinkCmd は既存 decision の supersedes[] に追記専用で結線する
// （#45 D7・add-commit と同型）。target/why/changed/ref/at ら判断欄位は一切
// 書き換えない——現行性リンクの後付け backfill 用。id 実在・自己参照禁止・
// 循環禁止・重複 link は冪等（既存と同一 {id,mode} なら no-op）を検証する。
func newDecisionLinkCmd() *cobra.Command {
	var supersedes []string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "link <newDecisionId> --supersedes <oldUlid>[:<mode>]",
		Short: "既存 decision に現行性リンク（supersedes）を後付けする（追記専用・判断欄位は不変・#45 D7）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			newID := args[0]
			if len(supersedes) == 0 {
				return fmt.Errorf("--supersedes は必須です（<oldUlid>[:<mode>]）")
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			d, err := s.LoadDecision(newID)
			if err != nil {
				return fmt.Errorf("decision %q を読み込めません: %w", newID, err)
			}
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}

			links, err := parseSupersedeLinks(supersedes, newID)
			if err != nil {
				return err
			}
			if err := model.ValidateSupersedeTargets(snap.Decisions, links); err != nil {
				return err
			}

			// 追記専用: 既存 link は不可侵。既存と同一 {id,mode} は冪等（skip）・
			// 同一 id で mode 違いは append-only 破れ（error）。
			added, err := model.AppendSupersedeLinks(d.Supersedes, links)
			if err != nil {
				return err
			}
			if len(added) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "decision %s: 指定 link は既に結線済みです（冪等・変更なし）\n", newID)
				return nil
			}
			merged := append(append([]model.SupersedeLink(nil), d.Supersedes...), added...)

			// 循環禁止（多段 A→B→A も含む）: merged を newID の supersedes として
			// グラフに載せて閉路検査。
			if model.SupersedeCreatesCycle(snap.Decisions, newID, merged) {
				return fmt.Errorf("supersedes: この結線は decision の supersede グラフに循環を作ります（新→旧の有向グラフに閉路）")
			}

			d.Supersedes = merged
			if err := s.SaveDecision(d); err != nil {
				return err
			}
			saved, err := s.LoadDecision(newID)
			if err != nil {
				return err
			}

			if asJSON {
				return emitWriteJSON(cmd, saved, nil, nil, false)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "decision %s に supersedes を追加しました（%d 件結線）\n", newID, len(added))
			for _, l := range added {
				fmt.Fprintf(cmd.OutOrStdout(), "  → %s (%s)\n", l.ID, l.SupersedeMode())
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&supersedes, "supersedes", nil, "置き換える旧 decision <ulid>[:<mode>]（mode=supersede|amend|exception・既定 amend・繰り返し可）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "更新後のレコードを JSON で出力する")
	return cmd
}
