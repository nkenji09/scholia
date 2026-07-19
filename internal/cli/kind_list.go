package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newKindListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "config.kinds の宣言を全カテゴリぶん表示する",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(cfg.Kinds)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "condition: %s\n", strings.Join(cfg.KindsFor("condition"), ", "))
			fmt.Fprintf(w, "action:    %s\n", strings.Join(cfg.Kinds.Action, ", "))
			fmt.Fprintf(w, "effect:    %s\n", strings.Join(cfg.Kinds.Effect, ", "))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
}
