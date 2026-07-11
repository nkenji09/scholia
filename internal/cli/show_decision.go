package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newShowDecisionCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "decision <id>",
		Short: "意思決定を 1 件表示する",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			s, err := openStore()
			if err != nil {
				return err
			}
			d, err := s.LoadDecision(id)
			if err != nil {
				return fmt.Errorf("decision %q を読み込めません: %w", id, err)
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(d)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "id: %s\n", d.ID)
			fmt.Fprintf(out, "target: %s:%s\n", d.Target.Type, d.Target.ID)
			fmt.Fprintf(out, "at: %s\n", d.At)
			fmt.Fprintf(out, "why:\n%s\n", d.Why)
			if d.Changed != "" {
				fmt.Fprintf(out, "changed:\n%s\n", d.Changed)
			}
			if d.Ref != "" {
				fmt.Fprintf(out, "ref: %s\n", d.Ref)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
}
