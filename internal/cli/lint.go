package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/lint"
)

func newLintCmd() *cobra.Command {
	var asJSON, verbose bool
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "記録の自己矛盾を検査する（§5）",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}
			findings := lint.Run(snap)
			if findings == nil {
				findings = []lint.Finding{}
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				errorCount, warnCount, infoCount := countBySeverity(findings)
				out := struct {
					Findings   []lint.Finding `json:"findings"`
					ErrorCount int            `json:"errorCount"`
					WarnCount  int            `json:"warnCount"`
					InfoCount  int            `json:"infoCount"`
				}{Findings: findings, ErrorCount: errorCount, WarnCount: warnCount, InfoCount: infoCount}
				if err := enc.Encode(out); err != nil {
					return err
				}
			} else {
				printLintText(cmd, findings, verbose)
			}

			if lint.HasError(findings) {
				return fmt.Errorf("lint failed: %d error(s)", countErrors(findings))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する（decision-coverage は direct/via-tag/none 全件＋coverage 付き）")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "decision-coverage via-tag の内訳（どのタグ経由か）を表示する")
	return cmd
}

// printLintText は既定のテキスト出力。decision-coverage は none のみ列挙し、
// 3段の件数はサマリ行に畳む（direct/via-tag を毎回列挙する info ノイズを
// 出さない・U1）。--verbose で via-tag の出自内訳を展開する。
// acknowledge-only（decision 判断欄位由来の advisory＝append-only により是正
// 不能・#45 U2）は是正対象と混ざらないよう末尾に別掲する。
func printLintText(cmd *cobra.Command, findings []lint.Finding, verbose bool) {
	out := cmd.OutOrStdout()

	displayed := make([]lint.Finding, 0, len(findings))
	ackOnly := make([]lint.Finding, 0)
	for _, f := range findings {
		if f.Coverage != "" && f.Coverage != lint.CoverageNone {
			continue
		}
		if f.AcknowledgeOnly {
			ackOnly = append(ackOnly, f)
			continue
		}
		displayed = append(displayed, f)
	}
	if len(displayed) == 0 && len(ackOnly) == 0 {
		fmt.Fprintln(out, "lint: 問題は見つかりませんでした")
	}
	for _, f := range displayed {
		fmt.Fprintf(out, "[%s] %s: %s\n", f.Severity, f.Rule, f.Message)
	}
	if len(ackOnly) > 0 {
		fmt.Fprintf(out, "acknowledge-only（decision 判断欄位・append-only により是正不能・容認で畳む対象）: %d 件\n", len(ackOnly))
		for _, f := range ackOnly {
			fmt.Fprintf(out, "  [%s] %s: %s\n", f.Severity, f.Rule, f.Message)
		}
	}

	direct, viaTag, none := lint.CoverageCounts(findings)
	if direct+viaTag+none == 0 {
		return
	}
	if verbose {
		fmt.Fprintf(out, "decision-coverage: direct %d / via-tag %d / none %d\n", direct, viaTag, none)
		if viaTag > 0 {
			fmt.Fprintln(out, "decision-coverage via-tag の内訳:")
			for _, f := range findings {
				if f.Coverage == lint.CoverageViaTag {
					fmt.Fprintf(out, "  %s: %s\n", f.Target, f.Detail)
				}
			}
		}
	} else {
		fmt.Fprintf(out, "decision-coverage: direct %d / via-tag %d / none %d（via-tag の内訳は --verbose）\n", direct, viaTag, none)
	}
}

func countErrors(findings []lint.Finding) int {
	n, _, _ := countBySeverity(findings)
	return n
}

func countBySeverity(findings []lint.Finding) (errorCount, warnCount, infoCount int) {
	for _, f := range findings {
		switch f.Severity {
		case lint.SeverityError:
			errorCount++
		case lint.SeverityWarn:
			warnCount++
		case lint.SeverityInfo:
			infoCount++
		}
	}
	return
}
