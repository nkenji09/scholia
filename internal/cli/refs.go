package cli

import "github.com/spf13/cobra"

func newRefsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refs",
		Short: "ソースコード中の scholia id 参照を走査・書き換える（rename の source-ref 機能の単体版）",
	}
	cmd.AddCommand(newRefsScanCmd())
	cmd.AddCommand(newRefsRewriteCmd())
	return cmd
}
