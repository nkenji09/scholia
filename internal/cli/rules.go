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
//
// decisions は既定では「効いている規則」だけ。取り下げられたものは withdrawn に
// 存在と行き先だけ載る（--all で decisions 側へ合流する）。各要素は effect を
// 必ず持つので、消費側が全件走査して逆リンクを組まなくても効力が分かる。
type rulesOutput struct {
	Decisions []decisionOut  `json:"decisions"`
	Withdrawn []withdrawnOut `json:"withdrawn,omitempty"`
}

func newRulesCmd() *cobra.Command {
	var tagID, txID, vocabID, facet, sortBy string
	var asJSON, current, all bool
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "対象（tag/transition/vocab/facet）に関わる decisions を横断集約する（§3.8）",
		RunE: func(cmd *cobra.Command, args []string) error {
			selected := 0
			for _, v := range []string{tagID, txID, vocabID, facet} {
				if v != "" {
					selected++
				}
			}
			if selected > 1 {
				return fmt.Errorf("--tag / --tx / --vocab / --facet は同時に指定できません")
			}
			if sortBy != "chrono" && sortBy != "target" {
				return fmt.Errorf("--sort は chrono|target のいずれかである必要があります（実際は %q）", sortBy)
			}
			if all && current {
				return fmt.Errorf("--all と --current は同時に指定できません（--current は既定と同じ意味です）")
			}

			s, err := openStore()
			if err != nil {
				return err
			}
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}

			decisions, err := index.SelectRulesDecisionsFor(&snap, tagID, txID, vocabID, facet)
			if err != nil {
				return err
			}

			// 取り下げ（mode=supersede の被参照）は既定で本文を出さない。
			// 存在と行き先は既定でも出す——「取り下げがあったこと自体」を
			// 知らせないと、次に読む人は何も気づかないまま古い規則を受け取る。
			// 全文は --all。判断そのものは currency.go の純関数にある。
			view := newCurrencyView(snap.Decisions)
			sortDecisions(decisions, sortBy)
			bodies, withdrawn := view.partition(decisions, all)

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(rulesOutput{
					Decisions: view.decisionOuts(bodies),
					Withdrawn: view.withdrawnOuts(withdrawn),
				})
			}
			// 効いているものが 0 件でも、取り下げがあるなら「該当なし」とは書かない
			// ——「無い」と「取り下げられた」は読み手にとって別の事実である。
			if len(bodies) == 0 && len(withdrawn) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "rules: 該当する decision はありません")
				return nil
			}
			if len(bodies) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "rules: 効いている decision はありません")
			} else {
				printRules(cmd, bodies, sortBy, view)
			}
			writeWithdrawn(cmd.OutOrStdout(), withdrawn, view, "")
			return nil
		},
	}
	cmd.Flags().StringVar(&tagID, "tag", "", "タグを対象にする（自身＋祖先タグへの decisions）")
	cmd.Flags().StringVar(&txID, "tx", "", "遷移を対象にする（自身＋実効タグへの decisions）")
	cmd.Flags().StringVar(&vocabID, "vocab", "", "語彙を対象にする（自身＋その語彙が持つタグ〔vocab.tags〕とその祖先への decisions・#45 D10b）")
	cmd.Flags().StringVar(&facet, "facet", "", "指定 kind を持つ全タグを対象にする")
	cmd.Flags().StringVar(&sortBy, "sort", "chrono", "並び順（chrono=at昇順・既定 | target=対象ごとにグループ化）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	cmd.Flags().BoolVar(&all, "all", false, "取り下げられた decision も本文ごと出す（既定は存在と行き先のみ）")
	cmd.Flags().BoolVar(&current, "current", false, "既定と同じ（後方互換のため受理する。取り下げの本文は既定でも出ません）")
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

func printRules(cmd *cobra.Command, decisions []model.Decision, sortBy string, view currencyView) {
	out := cmd.OutOrStdout()
	if sortBy == "target" {
		var lastTarget model.DecisionTarget
		first := true
		for _, d := range decisions {
			if first || d.Target != lastTarget {
				fmt.Fprintf(out, "== %s:%s ==\n", d.Target.Type, d.Target.ID)
				lastTarget = d.Target
				first = false
			}
			fmt.Fprintf(out, "  [%s]%s\n", d.ID, currencyLabel(d, view.superseded))
			printDecisionLine(out, d)
		}
		return
	}
	for _, d := range decisions {
		fmt.Fprintf(out, "[%s] %s:%s%s\n", d.At, d.Target.Type, d.Target.ID, currencyLabel(d, view.superseded))
		printDecisionLine(out, d)
	}
}

// currencyLabel は decision の現行性区分（#45 D7）を表示用に返す:
// 失効（supersede された）/改訂（何かを amend/exception している現行）/現行。
func currencyLabel(d model.Decision, superseded map[string]bool) string {
	if superseded[d.ID] {
		return withdrawnMarkLabel
	}
	if len(d.Supersedes) > 0 {
		hasSupersede := false
		for _, l := range d.Supersedes {
			if l.SupersedeMode() == model.ModeSupersede {
				hasSupersede = true
			}
		}
		if hasSupersede {
			return fmt.Sprintf(" [現行: supersedes %d 件]", len(d.Supersedes))
		}
		return fmt.Sprintf(" [改訂(amend/exception): %d 件]", len(d.Supersedes))
	}
	return ""
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
