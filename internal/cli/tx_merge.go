package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/refs"
)

func newTxMergeCmd() *cobra.Command {
	var into string
	var asJSON bool
	var refsFlags renameRefsFlags
	cmd := &cobra.Command{
		Use:   "merge <dupId>",
		Short: "重複遷移を統合する（同一原子のみ・decision 追随・タグ union・#45 U4）",
		Long: "遷移 <dupId> を --into <survivorId> へ統合する（duplicate-atom の是正手段・決定⑩）。\n" +
			"同一原子（action が一致・given が集合として一致・then が順序リストとして一致）\n" +
			"のみ許可する。dup を削除し、dup を target とする decision を survivor へ張替え、\n" +
			"dup のタグを survivor へ union する。lint --ci の baseline に dup 宛 entry が\n" +
			"あれば target id も追随更新する。\n\n" +
			"rename と同様、ソースコメント等に残る dup id も既定で走査し dry-run 表示する\n" +
			"（ソース不変）。--rewrite-refs でその場で境界安全に置換、--no-refs で走査自体を\n" +
			"省略する。",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dupID := args[0]
			if into == "" {
				return fmt.Errorf("--into は必須です")
			}
			s, err := openStore()
			if err != nil {
				return err
			}
			result, err := s.MergeTransitions(dupID, into)
			if err != nil {
				return err
			}

			report, err := applyRenameRefs(s, []refs.Pair{{OldID: dupID, NewID: into}}, refsFlags)
			if err != nil {
				return err
			}

			if asJSON {
				if err := encodeRenameJSON(cmd, result, report); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(),
					"transition %s を %s へ統合しました（張替えた decision: %d 件・union で増えたタグ: %s・dup 削除）\n",
					result.DupID, result.SurvivorID, len(result.UpdatedDecisions),
					formatAddedTags(result.AddedTags))
				printRenameRefsReport(cmd, report, refsFlags.rewrite)
			}
			return refsFailedErr(report)
		},
	}
	cmd.Flags().StringVar(&into, "into", "", "統合先（生き残る遷移）の id（必須）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "統合サマリを JSON で出力する")
	refsFlags.register(cmd)
	return cmd
}

func formatAddedTags(tags []string) string {
	if len(tags) == 0 {
		return "なし"
	}
	return strings.Join(tags, ",")
}
