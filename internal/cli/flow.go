package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/flow"
	"github.com/nkenji09/scholia/internal/index"
)

func newFlowCmd() *cobra.Command {
	var asJSON bool
	var verbose bool
	cmd := &cobra.Command{
		Use:   "flow <action>",
		Short: "きっかけ(action)の given×transition マトリクスと honesty-first な gap 検出を表示する（派生・§3.4・#39）",
		Long: "きっかけ(action)の given×transition マトリクスと honesty-first な gap 検出を表示する（派生・§3.4・#39）。\n\n" +
			"軸解析は、この action の transition の given に実際に現れる condition が持つ axis タグしか拾わない" +
			"（relevantAxes）。condition に axis タグを貼るだけでは軸は効かない——" +
			"畳んだ transition を条件別に割り、その条件を given へ materialize して初めて解析対象になる（#40・DESIGN §3.4）。",
		Args: cobra.ExactArgs(1),
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

			report := flow.Analyze(&snap, ix, args[0])

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			flow.WriteText(cmd.OutOrStdout(), report, verbose)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "評価順で解決済みの重なり/subset-shadow と derive した実効 given（else）も開示する（#45 D8）")
	return cmd
}
