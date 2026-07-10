package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newConfigGetCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "get [<key>]",
		Short: "config を表示する（キー省略で config 全体）",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}

			if len(args) == 0 {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(cfg)
			}

			key := args[0]
			val, err := configKeyValue(cfg, key)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(val)
			}
			fmt.Fprintln(cmd.OutOrStdout(), formatConfigValue(val))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する（キー指定時。キー省略時は常に JSON）")
	return cmd
}
