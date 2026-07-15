package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/nkenji09/product-memory/internal/refs"
)

func newTagRenameCmd() *cobra.Command {
	var cascade, asJSON bool
	var refsFlags renameRefsFlags
	cmd := &cobra.Command{
		Use:   "rename <old-id> <new-id>",
		Short: "タグを改名し全参照を張り替える（--cascade でサブツリーごと・T-tag-rename）",
		Long: "タグ <old-id> を <new-id> に改名し、そのタグ id を持つ全参照" +
			"（他タグの parentIds・遷移の tags・語彙の tags・decision の target）を一括で張り替える。" +
			"name/kind/desc/color/ref など他フィールドは保持する。\n\n" +
			"--cascade を付けると、<old-id> を id プレフィックスに持つ子孫タグも全て " +
			"<old-id>→<new-id> のプレフィックス置換で改名し、各々の参照も張り替える" +
			"（複数階層を1コマンドで）。\n\n" +
			"衝突（--cascade で生成される新 id を含む）や <old-id> 不存在はエラーで、" +
			"その場合は何も書き込まない（全ロールバック）。\n\n" +
			"rename 確定後、ソースコメント等に残る旧 id も既定で走査し dry-run 表示する（ソース不変）。" +
			"--rewrite-refs でその場で境界安全に置換、--no-refs で走査自体を省略する。",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldID, newID := args[0], args[1]
			s, err := openStore()
			if err != nil {
				return err
			}
			result, err := s.RenameTag(oldID, newID, cascade)
			if err != nil {
				return err
			}

			pairs := make([]refs.Pair, 0, len(result.RenamedTags))
			for old, nw := range result.RenamedTags {
				pairs = append(pairs, refs.Pair{OldID: old, NewID: nw})
			}
			sort.Slice(pairs, func(i, j int) bool { return pairs[i].OldID < pairs[j].OldID })
			report, err := applyRenameRefs(s, pairs, refsFlags)
			if err != nil {
				return err
			}

			if asJSON {
				if err := encodeRenameJSON(cmd, result, report); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(),
					"tag %s を %s に改名しました（改名タグ %d 件・張替: 他タグ parentIds %d 件・遷移 %d 件・語彙 %d 件・decision %d 件）\n",
					result.OldID, result.NewID, len(result.RenamedTags),
					len(result.UpdatedTags), len(result.UpdatedTransitions),
					len(result.UpdatedVocab), len(result.UpdatedDecisions))
				printRenameRefsReport(cmd, report, refsFlags.rewrite)
			}
			return refsFailedErr(report)
		},
	}
	cmd.Flags().BoolVar(&cascade, "cascade", false, "<old-id> を id プレフィックスに持つ子孫タグごと改名する")
	cmd.Flags().BoolVar(&asJSON, "json", false, "改名サマリを JSON で出力する")
	refsFlags.register(cmd)
	return cmd
}
