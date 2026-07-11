package cli

import "github.com/spf13/cobra"

func newShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "レコードを表示する",
	}
	cmd.AddCommand(newShowTxCmd())
	cmd.AddCommand(newShowTagCmd())
	cmd.AddCommand(newShowVocabCmd())
	cmd.AddCommand(newShowDecisionCmd())
	return cmd
}
