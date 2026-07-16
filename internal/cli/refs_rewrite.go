package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/refs"
)

func newRefsRewriteCmd() *cobra.Command {
	var apply, asJSON bool
	cmd := &cobra.Command{
		Use:   "rewrite <old-id> <new-id>",
		Short: "ソース中の旧 id 参照を境界安全に置換する（.scholia には触れない・冪等）",
		Long: "ソースコード中の <old-id> の出現を境界安全に <new-id> へ置換する。.scholia には一切触れない。\n" +
			"既定は dry-run 表示（ソース不変）、--apply で実際に書き換える。\n\n" +
			"rename の --rewrite-refs が部分失敗した場合の再実行、およびマーカー無しで rename 済みの " +
			"参照を後から直す場合に使う（同じ <old-id> <new-id> で再実行しても冪等）。\n\n" +
			"走査は git ls-files（.gitignore 尊重）が既定経路。git が使えない場合はディレクトリ walk に" +
			"フォールバックし、その場合 .gitignore は尊重されない。走査範囲は config.sourceRefs（scan/exclude）で絞れる。",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldID, newID := args[0], args[1]
			s, err := openStore()
			if err != nil {
				return err
			}
			opts, err := refsOptions(s)
			if err != nil {
				return err
			}
			report, err := refs.Execute(projectRoot(s), []refs.Pair{{OldID: oldID, NewID: newID}}, apply, opts)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return err
				}
			} else {
				printRenameRefsReport(cmd, &report, apply)
			}
			return refsFailedErr(&report)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "実際に書き換える（既定は dry-run 表示のみ）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "結果を JSON で出力する")
	return cmd
}
