package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/model"
)

// decisionListOutput は --json 出力の形。
//
// 各要素は effect（in-force | replaced）を必ず持つ。人が読む出力には
// `[失効: supersede 済]` が付くのに JSON には何も無く、消費側が全件走査して
// supersedes[] の逆リンクを組まない限り効力を知れない、という食い違いがあった。
type decisionListOutput struct {
	Decisions []decisionOut `json:"decisions"`
}

// newDecisionListCmd は decision レコードをフラットに一覧する（§3.8）。
// `scholia rules` は対象への守る規則を祖先展開込みで集約するのに対し、
// こちらは decision レコードそのものの棚卸し（--on は完全一致・祖先展開なし）。
func newDecisionListCmd() *cobra.Command {
	var on string
	var asJSON, unlinked, current bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "decision レコードをフラットに一覧する（rules=対象別集約とは別・§3.8）",
		RunE: func(cmd *cobra.Command, args []string) error {
			var targetType, targetID string
			if on != "" {
				var err error
				targetType, targetID, err = parseDecisionOn(on)
				if err != nil {
					return err
				}
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}

			// --current（#45 D7）: mode=supersede で指された decision を失効として畳む
			// （保守的に supersede のみ）。amend/exception は現行のまま。
			//
			// この面の既定は変えない——rules / spec が「守る規則を引く」面なのに対し、
			// ここは decision レコードそのものの棚卸しで、取り下げた理由を読む経路を
			// 1 つ残しておく必要がある。変えるのは JSON に効力を載せることだけ。
			view := newCurrencyView(snap.Decisions)
			superseded := view.superseded

			decisions := make([]model.Decision, 0, len(snap.Decisions))
			for _, d := range snap.Decisions {
				if on != "" && (d.Target.Type != targetType || d.Target.ID != targetID) {
					continue
				}
				if unlinked && len(d.Commits) != 0 {
					continue // --unlinked: commits 空（未結線）のみ
				}
				if current && superseded[d.ID] {
					continue // --current: 失効（supersede された）を除く
				}
				decisions = append(decisions, d)
			}
			sort.SliceStable(decisions, func(i, j int) bool {
				return decisions[i].At < decisions[j].At
			})

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(decisionListOutput{Decisions: view.decisionOuts(decisions)})
			}
			printDecisionList(cmd, decisions, superseded)
			return nil
		},
	}
	cmd.Flags().StringVar(&on, "on", "", "対象で絞り込む（tag:<id>・transition:<id>・vocab:<id>・完全一致・祖先展開なし・任意）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	cmd.Flags().BoolVar(&unlinked, "unlinked", false, "commits 未結線（実装来歴が空）の decision だけを列挙する（#45 D7・棚卸し）")
	cmd.Flags().BoolVar(&current, "current", false, "失効（mode=supersede で指された）decision を畳んで現行のみ列挙する（#45 D7・保守的に supersede のみ）")
	return cmd
}

func printDecisionList(cmd *cobra.Command, decisions []model.Decision, superseded map[string]bool) {
	out := cmd.OutOrStdout()
	if len(decisions) == 0 {
		fmt.Fprintln(out, "decision list: 該当する decision はありません")
		return
	}
	for _, d := range decisions {
		status := ""
		if superseded[d.ID] {
			status = " [失効: supersede 済]"
		} else if len(d.Supersedes) > 0 {
			// 何かを改訂/例外化している現行 decision（区分表示・#45 D7）。
			status = fmt.Sprintf(" [改訂: supersedes %d 件]", len(d.Supersedes))
		}
		fmt.Fprintf(out, "[%s] %s %s:%s%s\n", d.At, d.ID, d.Target.Type, d.Target.ID, status)
		fmt.Fprintf(out, "  why: %s\n", truncateOneLine(d.Why, 100))
		if len(d.Commits) == 0 {
			fmt.Fprintln(out, "  commits: 未結線")
		}
		if d.Ref != "" {
			fmt.Fprintf(out, "  ref: %s\n", d.Ref)
		}
	}
}

// truncateOneLine は複数行の why を要約表示用に 1 行へ畳み、長ければ省略する。
func truncateOneLine(s string, max int) string {
	oneline := strings.Join(strings.Fields(s), " ")
	r := []rune(oneline)
	if len(r) <= max {
		return oneline
	}
	return string(r[:max]) + "…"
}
