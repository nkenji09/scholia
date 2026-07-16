package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/nkenji09/product-memory/internal/flow"
	"github.com/nkenji09/product-memory/internal/index"
)

// newGapsCmd is `pmem gaps <action>` — the same internal/flow.Analyze as
// `pmem flow`, but a focused surface that prints only gap findings
// (subset-shadow・抜け・重なり) and the mandatory scope-disclosure, never the
// full condition×transition matrix (req.action-flow.axis-gaps: `pmem flow`
// is the whole-picture view, `pmem gaps` is the holes-only view — same
// analysis, different presentation). Scope-disclosure stays mandatory even
// here: `pmem gaps` must never print a bare "no gaps"
// (req.action-flow.scope-honesty).
func newGapsCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "gaps <action>",
		Short: "きっかけ(action)の gap（抜け・重なり・subset-shadow）と scope-disclosure だけを表示する（派生・§3.4・#39）",
		Long: "きっかけ(action)の gap（抜け・重なり・subset-shadow）と scope-disclosure だけを表示する（派生・§3.4・#39）。\n\n" +
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
				return enc.Encode(report.Gaps())
			}
			flow.WriteGapsText(cmd.OutOrStdout(), report)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
}
