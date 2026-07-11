package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newShowVocabCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "vocab <id>",
		Short: "語彙を 1 件表示する",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			s, err := openStore()
			if err != nil {
				return err
			}
			v, err := s.LoadVocab(id)
			if err != nil {
				return fmt.Errorf("vocab %q を読み込めません: %w", id, err)
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(v)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "id: %s\n", v.ID)
			fmt.Fprintf(out, "category: %s\n", v.Category)
			fmt.Fprintf(out, "label: %s\n", v.Label)
			if v.Kind != "" {
				fmt.Fprintf(out, "kind: %s\n", v.Kind)
			}
			if v.Owner != "" {
				fmt.Fprintf(out, "owner: %s\n", v.Owner)
			}
			if v.Description != "" {
				fmt.Fprintf(out, "description:\n%s\n", v.Description)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
}
