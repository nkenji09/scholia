package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/render"
)

func newSpecCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "spec <subjectTag>",
		Short: "タグで束ねた\"仕様\"レポートを表示する（派生・保存しない・§3.8）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}
			ix := index.Build(&snap)

			report, err := render.Spec(&snap, ix, args[0])
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			render.WriteText(cmd.OutOrStdout(), report)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
}
