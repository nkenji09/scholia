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
type decisionListOutput struct {
	Decisions []model.Decision `json:"decisions"`
}

// newDecisionListCmd は decision レコードをフラットに一覧する（§3.8）。
// `scholia rules` は対象への守る規則を祖先展開込みで集約するのに対し、
// こちらは decision レコードそのものの棚卸し（--on は完全一致・祖先展開なし）。
func newDecisionListCmd() *cobra.Command {
	var on string
	var asJSON bool
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

			decisions := make([]model.Decision, 0, len(snap.Decisions))
			for _, d := range snap.Decisions {
				if on != "" && (d.Target.Type != targetType || d.Target.ID != targetID) {
					continue
				}
				decisions = append(decisions, d)
			}
			sort.SliceStable(decisions, func(i, j int) bool {
				return decisions[i].At < decisions[j].At
			})

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(decisionListOutput{Decisions: decisions})
			}
			printDecisionList(cmd, decisions)
			return nil
		},
	}
	cmd.Flags().StringVar(&on, "on", "", "対象で絞り込む（tag:<id> または transition:<id>・完全一致・祖先展開なし・任意）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
}

func printDecisionList(cmd *cobra.Command, decisions []model.Decision) {
	out := cmd.OutOrStdout()
	if len(decisions) == 0 {
		fmt.Fprintln(out, "decision list: 該当する decision はありません")
		return
	}
	for _, d := range decisions {
		fmt.Fprintf(out, "[%s] %s %s:%s\n", d.At, d.ID, d.Target.Type, d.Target.ID)
		fmt.Fprintf(out, "  why: %s\n", truncateOneLine(d.Why, 100))
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
