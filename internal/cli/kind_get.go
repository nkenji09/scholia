package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newKindGetCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "get <condition|action|effect>",
		Short: "指定カテゴリの kind 宣言を表示する",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			category := args[0]
			if !isValidCategory(category) {
				return fmt.Errorf("category は condition|action|effect のいずれかである必要があります（実際は %q）", category)
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}
			kinds := cfg.KindsFor(category)

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(kinds)
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.Join(kinds, ", "))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
}
