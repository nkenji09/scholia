package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nkenji09/product-memory/internal/render"
)

func newExportCmd() *cobra.Command {
	var htmlDir string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "派生ビューを書き出す",
		RunE: func(cmd *cobra.Command, args []string) error {
			if htmlDir == "" {
				return fmt.Errorf("--html <dir> を指定してください")
			}
			s, err := openStore()
			if err != nil {
				return err
			}
			if err := render.ExportHTML(s, htmlDir); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s に静的ビューアを書き出しました\n", htmlDir)
			return nil
		},
	}
	cmd.Flags().StringVar(&htmlDir, "html", "", "自己完結の静的 HTML を書き出すディレクトリ（§7）")
	return cmd
}
