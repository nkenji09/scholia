package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newShowTagCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "tag <id>",
		Short: "タグを 1 件表示する",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			s, err := openStore()
			if err != nil {
				return err
			}
			t, err := s.LoadTag(id)
			if err != nil {
				return fmt.Errorf("tag %q を読み込めません: %w", id, err)
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(t)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "id: %s\n", t.ID)
			fmt.Fprintf(out, "name: %s\n", t.Name)
			if t.Kind != "" {
				fmt.Fprintf(out, "kind: %s\n", t.Kind)
			}
			if len(t.ParentIDs) > 0 {
				fmt.Fprintf(out, "parents: %s\n", strings.Join(t.ParentIDs, ", "))
			}
			if t.Color != "" {
				fmt.Fprintf(out, "color: %s\n", t.Color)
			}
			if t.Ref != "" {
				fmt.Fprintf(out, "ref: %s\n", t.Ref)
			}
			if t.Description != "" {
				fmt.Fprintf(out, "description:\n%s\n", t.Description)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
}
