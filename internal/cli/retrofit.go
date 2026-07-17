package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/lint"
)

// newRetrofitCmd は `scholia retrofit`（#45 U2/P4）。全 advisory 規則で store
// を read-only 走査し、record×rule×該当引用×修正候補の棚卸しを出す。--fix は
// 意図的に持たない——是正は正規の提案フロー（pending 変更＋review→adopt）に
// 乗せる（機械の一括書き換えは decision を伴わない意味変更を作り append-only
// 原則と衝突する）。findings があっても exit 0（棚卸しであってゲートではない）。
func newRetrofitCmd() *cobra.Command {
	var asJSON bool
	var ruleFilter string
	cmd := &cobra.Command{
		Use:   "retrofit",
		Short: "advisory 規則で store を read-only 走査し、是正候補の棚卸しを出す（--fix は無い・exit 0）",
		Long: "全 advisory 規則（authoring 規律・severity=info）で既存 store を read-only 走査し、\n" +
			"record×rule×該当引用×修正候補の是正リストを出す。decision の判断欄位（why/changed/ref）\n" +
			"由来の findings は append-only により是正不能のため acknowledge-only として別掲する。\n" +
			"是正は提案フロー（pending 変更＋review コメント→adopt）に乗せる——--fix は持たない。",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}

			rules := advisoryRules()
			if ruleFilter != "" {
				var picked []lint.Rule
				for _, r := range rules {
					if r.Name == ruleFilter {
						picked = append(picked, r)
					}
				}
				if len(picked) == 0 {
					return fmt.Errorf("--rule %q は advisory 規則ではありません（有効: %s）", ruleFilter, strings.Join(advisoryRuleNames(rules), ", "))
				}
				rules = picked
			}

			var findings []lint.Finding
			for _, r := range rules {
				findings = append(findings, r.Check(snap)...)
			}
			if findings == nil {
				findings = []lint.Finding{}
			}

			fixable, ackOnly := splitAcknowledgeOnly(findings)
			names := advisoryRuleNames(rules)
			fixStats := partitionStats(fixable, names)
			ackStats := partitionStats(ackOnly, names)

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				out := struct {
					Rules           []string       `json:"rules"`
					Findings        []lint.Finding `json:"findings"`
					Fixable         retrofitStats  `json:"fixable"`
					AcknowledgeOnly retrofitStats  `json:"acknowledgeOnly"`
				}{Rules: names, Findings: findings, Fixable: fixStats, AcknowledgeOnly: ackStats}
				return enc.Encode(out)
			}

			printRetrofitText(cmd, names, fixable, ackOnly, fixStats, ackStats)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する（findings 全件＋fixable/acknowledgeOnly の件数集計）")
	cmd.Flags().StringVar(&ruleFilter, "rule", "", "この advisory 規則だけ走査する（例: dangling-id）")
	return cmd
}

func advisoryRules() []lint.Rule {
	var out []lint.Rule
	for _, r := range lint.Rules {
		if r.Tier == lint.TierAdvisory {
			out = append(out, r)
		}
	}
	return out
}

func advisoryRuleNames(rules []lint.Rule) []string {
	names := make([]string, len(rules))
	for i, r := range rules {
		names[i] = r.Name
	}
	return names
}

func splitAcknowledgeOnly(findings []lint.Finding) (fixable, ackOnly []lint.Finding) {
	for _, f := range findings {
		if f.AcknowledgeOnly {
			ackOnly = append(ackOnly, f)
		} else {
			fixable = append(fixable, f)
		}
	}
	return
}

// retrofitStats は区分ごとの件数集計（ユニークレコード数と規則別件数の併記）。
type retrofitStats struct {
	FindingCount int            `json:"findingCount"`
	RecordCount  int            `json:"recordCount"`
	ByRule       map[string]int `json:"byRule"`
}

func partitionStats(findings []lint.Finding, ruleNames []string) retrofitStats {
	byRule := make(map[string]int, len(ruleNames))
	for _, n := range ruleNames {
		byRule[n] = 0
	}
	records := make(map[string]bool)
	for _, f := range findings {
		byRule[f.Rule]++
		records[f.TargetType+"/"+f.Target] = true
	}
	return retrofitStats{FindingCount: len(findings), RecordCount: len(records), ByRule: byRule}
}

func printRetrofitText(cmd *cobra.Command, ruleNames []string, fixable, ackOnly []lint.Finding, fixStats, ackStats retrofitStats) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "retrofit: advisory %d 規則で走査（read-only・是正は提案フローで行う）\n\n", len(ruleNames))

	printOne := func(f lint.Finding) {
		loc := f.TargetType + " " + f.Target
		if f.Field != "" {
			loc += "（" + f.Field + "）"
		}
		fmt.Fprintf(out, "  [%s] %s", f.Rule, loc)
		if f.Quote != "" {
			fmt.Fprintf(out, ": %s", f.Quote)
		}
		fmt.Fprintln(out)
		if f.Suggestion != "" {
			fmt.Fprintf(out, "      → 修正候補: %s\n", f.Suggestion)
		}
	}

	fmt.Fprintln(out, "fixable（是正可能）:")
	if len(fixable) == 0 {
		fmt.Fprintln(out, "  なし")
	}
	for _, f := range fixable {
		printOne(f)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "acknowledge-only（decision 判断欄位・append-only により是正不能・容認で畳む対象）:")
	if len(ackOnly) == 0 {
		fmt.Fprintln(out, "  なし")
	}
	for _, f := range ackOnly {
		printOne(f)
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "集計: fixable %d findings / %d レコード・acknowledge-only %d findings / %d レコード\n",
		fixStats.FindingCount, fixStats.RecordCount, ackStats.FindingCount, ackStats.RecordCount)

	names := append([]string{}, ruleNames...)
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, n := range names {
		seg := fmt.Sprintf("%s %d", n, fixStats.ByRule[n]+ackStats.ByRule[n])
		if ackStats.ByRule[n] > 0 {
			seg += fmt.Sprintf("（うち acknowledge-only %d）", ackStats.ByRule[n])
		}
		parts = append(parts, seg)
	}
	fmt.Fprintf(out, "規則別: %s\n", strings.Join(parts, "／"))
}
