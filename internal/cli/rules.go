package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/model"
)

// rulesOutput は --json 出力の形。
type rulesOutput struct {
	Decisions []model.Decision `json:"decisions"`
}

func newRulesCmd() *cobra.Command {
	var tagID, txID, facet, sortBy string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "対象（tag/transition/facet）に関わる decisions を横断集約する（§3.8）",
		RunE: func(cmd *cobra.Command, args []string) error {
			selected := 0
			for _, v := range []string{tagID, txID, facet} {
				if v != "" {
					selected++
				}
			}
			if selected > 1 {
				return fmt.Errorf("--tag / --tx / --facet は同時に指定できません")
			}
			if sortBy != "chrono" && sortBy != "target" {
				return fmt.Errorf("--sort は chrono|target のいずれかである必要があります（実際は %q）", sortBy)
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}

			decisions, err := index.SelectRulesDecisions(&snap, tagID, txID, facet)
			if err != nil {
				return err
			}
			sortDecisions(decisions, sortBy)

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(rulesOutput{Decisions: decisions})
			}
			printRules(cmd, decisions, sortBy)
			return nil
		},
	}
	cmd.Flags().StringVar(&tagID, "tag", "", "タグを対象にする（自身＋祖先タグへの decisions）")
	cmd.Flags().StringVar(&txID, "tx", "", "遷移を対象にする（自身＋実効タグへの decisions）")
	cmd.Flags().StringVar(&facet, "facet", "", "指定 kind を持つ全タグを対象にする")
	cmd.Flags().StringVar(&sortBy, "sort", "chrono", "並び順（chrono=at昇順・既定 | target=対象ごとにグループ化）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
}

func sortDecisions(decisions []model.Decision, sortBy string) {
	if sortBy == "target" {
		sort.SliceStable(decisions, func(i, j int) bool {
			ti, tj := decisions[i].Target, decisions[j].Target
			if ti.Type != tj.Type {
				return ti.Type < tj.Type
			}
			if ti.ID != tj.ID {
				return ti.ID < tj.ID
			}
			return decisions[i].At < decisions[j].At
		})
		return
	}
	sort.SliceStable(decisions, func(i, j int) bool {
		return decisions[i].At < decisions[j].At
	})
}

func printRules(cmd *cobra.Command, decisions []model.Decision, sortBy string) {
	out := cmd.OutOrStdout()
	if len(decisions) == 0 {
		fmt.Fprintln(out, "rules: 該当する decision はありません")
		return
	}
	if sortBy == "target" {
		var lastTarget model.DecisionTarget
		first := true
		for _, d := range decisions {
			if first || d.Target != lastTarget {
				fmt.Fprintf(out, "== %s:%s ==\n", d.Target.Type, d.Target.ID)
				lastTarget = d.Target
				first = false
			}
			printDecisionLine(out, d)
		}
		return
	}
	for _, d := range decisions {
		fmt.Fprintf(out, "[%s] %s:%s\n", d.At, d.Target.Type, d.Target.ID)
		printDecisionLine(out, d)
	}
}

func printDecisionLine(w interface{ Write([]byte) (int, error) }, d model.Decision) {
	fmt.Fprintf(w, "  why: %s\n", d.Why)
	if d.Changed != "" {
		fmt.Fprintf(w, "  changed: %s\n", d.Changed)
	}
	if d.Ref != "" {
		fmt.Fprintf(w, "  ref: %s\n", d.Ref)
	}
}
