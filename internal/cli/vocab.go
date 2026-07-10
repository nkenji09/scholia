package cli

import "github.com/spf13/cobra"

func newVocabCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vocab",
		Short: "語彙（condition/action/effect）を操作する",
	}
	cmd.AddCommand(newVocabAddCmd())
	cmd.AddCommand(newVocabTagCmd())
	cmd.AddCommand(newVocabRenameCmd())
	cmd.AddCommand(newVocabRmCmd())
	return cmd
}
