package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/refs"
)

func newVocabRenameCmd() *cobra.Command {
	var to string
	var asJSON bool
	var refsFlags renameRefsFlags
	cmd := &cobra.Command{
		Use:   "rename <id>",
		Short: "語彙を改名する（参照する全遷移の action/given/then を一括更新・§6）",
		Long: "語彙 <id> を --to <newId> に改名し、参照する全遷移の action/given/then を一括更新する。\n\n" +
			"rename 確定後、ソースコメント等に残る旧 id も既定で走査し dry-run 表示する（ソース不変）。" +
			"--rewrite-refs でその場で境界安全に置換、--no-refs で走査自体を省略する。",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if to == "" {
				return fmt.Errorf("--to は必須です")
			}
			s, err := openStore()
			if err != nil {
				return err
			}
			result, err := s.RenameVocab(id, to)
			if err != nil {
				return err
			}

			report, err := applyRenameRefs(s, []refs.Pair{{OldID: id, NewID: to}}, refsFlags)
			if err != nil {
				return err
			}

			if asJSON {
				if err := encodeRenameJSON(cmd, result, report); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "vocab %s を %s に改名しました（更新した transition: %d 件）\n",
					result.OldID, result.NewID, len(result.UpdatedTransitions))
				printRenameRefsReport(cmd, report, refsFlags.rewrite)
			}
			return refsFailedErr(report)
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "新しい id（必須）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "更新サマリを JSON で出力する")
	refsFlags.register(cmd)
	return cmd
}
