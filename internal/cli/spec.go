package cli

import (
	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/render"
)

// specEntryOut / specOutput は `scholia spec --json` の出力形。
//
// viewer が使う render.SpecReport を埋め込みつつ、decision 群だけ差し替える
// （外側の同名フィールドが浅いので JSON では外側が勝つ）。差し替えるのは
// **端末の出力だけ**で、viewer が受け取るレポートは全件のまま——画面は
// 取り下げも含めて受け取り、自分で畳む側の要件を持っている。
type specEntryOut struct {
	render.SpecEntry
	Decisions []decisionOut  `json:"decisions,omitempty"`
	Withdrawn []withdrawnOut `json:"withdrawn,omitempty"`
}

type specOutput struct {
	render.SpecReport
	Entries      []specEntryOut `json:"entries"`
	TagDecisions []decisionOut  `json:"tagDecisions,omitempty"`
	Withdrawn    []withdrawnOut `json:"withdrawn,omitempty"`
}

func newSpecCmd() *cobra.Command {
	var asJSON, all bool
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

			// 取り下げの扱いは rules と同じにする（取り下げられた規則を無印で
			// 混ぜるのが一番害が大きい面でもある）。
			// ⚠️ **経由分の畳み込みは spec には当たらない**——spec は
			// decisionsForTarget（完全一致）で集めており、タグ経由の decision を
			// 集めていないので、01KZ06SYP12ZFDG1WPNYM529D8 の判断は spec を
			// 明示的に対象外にしている（同 結論6）。
			view := newCurrencyView(snap.Decisions)
			split := decisionSplitter{view: view, all: all}

			if asJSON {
				return emitJSON(cmd, buildSpecOutput(report, view, split))
			}
			render.WriteText(cmd.OutOrStdout(), report, split)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	cmd.Flags().BoolVar(&all, "all", false, "取り下げられた decision も本文ごと出す（既定は存在と行き先のみ）")
	return cmd
}

func buildSpecOutput(report render.SpecReport, view currencyView, split decisionSplitter) specOutput {
	tagBodies, tagWithdrawn := split.SplitDecisions(report.TagDecisions)
	entries := make([]specEntryOut, 0, len(report.Entries))
	for _, e := range report.Entries {
		bodies, withdrawn := split.SplitDecisions(e.Decisions)
		entries = append(entries, specEntryOut{
			SpecEntry: e,
			Decisions: view.decisionOuts(bodies),
			Withdrawn: view.withdrawnOuts(withdrawn),
		})
	}
	return specOutput{
		SpecReport:   report,
		Entries:      entries,
		TagDecisions: view.decisionOuts(tagBodies),
		Withdrawn:    view.withdrawnOuts(tagWithdrawn),
	}
}
