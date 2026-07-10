package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/nkenji09/product-memory/internal/index"
	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
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

			decisions, err := selectRulesDecisions(&snap, tagID, txID, facet)
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

// selectRulesDecisions はセレクタ（--tag/--tx/--facet/指定なし）に応じた decisions を返す。
// 実装判断（DESIGN §3.8 は一行定義のみのため）:
//   - --tx <id>: その遷移を target とする decisions ＋ 実効タグ（§3.7）に含まれる各 tag を target とする decisions。
//   - --tag <id>: そのタグ自身＋祖先タグ（parentIds 展開）を target とする decisions。親の規則は子にも効く。
//   - --facet <k>: kind が k の全タグを target とする decisions。
//   - 指定なし: 全 decisions。
func selectRulesDecisions(snap *store.Snapshot, tagID, txID, facet string) ([]model.Decision, error) {
	switch {
	case txID != "":
		tx, ok := findTransition(snap.Transitions, txID)
		if !ok {
			return nil, fmt.Errorf("transition %q が実在しません", txID)
		}
		targetTags := make(map[string]bool)
		for _, id := range index.EffectiveTags(snap, &tx) {
			targetTags[id] = true
		}
		return filterDecisions(snap.Decisions, func(d model.Decision) bool {
			if d.Target.Type == model.DecisionTargetTransition {
				return d.Target.ID == txID
			}
			return d.Target.Type == model.DecisionTargetTag && targetTags[d.Target.ID]
		}), nil

	case tagID != "":
		if !tagExists(snap.Tags, tagID) {
			return nil, fmt.Errorf("tag %q が実在しません", tagID)
		}
		ancestors := make(map[string]bool)
		for _, id := range index.TagAncestors(snap, tagID) {
			ancestors[id] = true
		}
		return filterDecisions(snap.Decisions, func(d model.Decision) bool {
			return d.Target.Type == model.DecisionTargetTag && ancestors[d.Target.ID]
		}), nil

	case facet != "":
		facetTags := make(map[string]bool)
		for _, t := range snap.Tags {
			if t.Kind == facet {
				facetTags[t.ID] = true
			}
		}
		return filterDecisions(snap.Decisions, func(d model.Decision) bool {
			return d.Target.Type == model.DecisionTargetTag && facetTags[d.Target.ID]
		}), nil

	default:
		return append([]model.Decision{}, snap.Decisions...), nil
	}
}

func findTransition(transitions []model.Transition, id string) (model.Transition, bool) {
	for _, t := range transitions {
		if t.ID == id {
			return t, true
		}
	}
	return model.Transition{}, false
}

func tagExists(tags []model.Tag, id string) bool {
	for _, t := range tags {
		if t.ID == id {
			return true
		}
	}
	return false
}

func filterDecisions(decisions []model.Decision, keep func(model.Decision) bool) []model.Decision {
	out := make([]model.Decision, 0, len(decisions))
	for _, d := range decisions {
		if keep(d) {
			out = append(out, d)
		}
	}
	return out
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
