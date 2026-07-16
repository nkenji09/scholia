package cli

import (
	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/model"
)

func newKindCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kind",
		Short: "kind 宣言（config.kinds[category]）を操作する",
	}
	cmd.AddCommand(newKindSetCmd())
	cmd.AddCommand(newKindListCmd())
	cmd.AddCommand(newKindGetCmd())
	return cmd
}

func isValidCategory(category string) bool {
	return category == model.CategoryCondition || category == model.CategoryAction || category == model.CategoryEffect
}
