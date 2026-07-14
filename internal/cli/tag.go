package cli

import "github.com/spf13/cobra"

func newTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "タグ（ネスト可能な横断分類）を操作する",
	}
	cmd.AddCommand(newTagCreateCmd())
	cmd.AddCommand(newTagListCmd())
	cmd.AddCommand(newTagEditCmd())
	cmd.AddCommand(newTagRenameCmd())
	cmd.AddCommand(newTagRmCmd())
	return cmd
}
