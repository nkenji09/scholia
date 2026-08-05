package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/model"
)

// decisionShowOutput は `decision show --json` の形（derive した superseded-by・
// current を additive に付ける。保存レコードそのものは Decision 内）。
type decisionShowOutput struct {
	Decision     model.Decision `json:"decision"`
	SupersededBy []struct {
		ID   string `json:"id"`
		Mode string `json:"mode"`
	} `json:"supersededBy,omitempty"`
	// Superseded は「mode=supersede で他 decision から指され失効扱い」か（derive）。
	Superseded bool `json:"superseded"`
}

// newDecisionShowCmd は decision 1 件を詳細表示する（#45 D7・viewer 詳細ページと
// 同一内容）。target/why/changed/ref/at/commits/supersedes/superseded-by〔derive〕/
// acknowledges を整形する。
func newDecisionShowCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "decision 1 件を詳細表示する（supersedes/superseded-by/acknowledges 込み・#45 D7）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			s, err := openStore()
			if err != nil {
				return err
			}
			d, err := s.LoadDecision(id)
			if err != nil {
				return fmt.Errorf("decision %q を読み込めません: %w", id, err)
			}
			snap, err := s.LoadAll()
			if err != nil {
				return err
			}

			supersededByIdx := supersededByIndex(snap.Decisions)
			superseded := supersededIDs(snap.Decisions)

			if asJSON {
				out := decisionShowOutput{Decision: d, Superseded: superseded[d.ID]}
				for _, ref := range supersededByIdx[d.ID] {
					out.SupersededBy = append(out.SupersededBy, struct {
						ID   string `json:"id"`
						Mode string `json:"mode"`
					}{ID: ref.FromID, Mode: ref.Mode})
				}
				return emitJSON(cmd, out)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "id: %s\n", d.ID)
			fmt.Fprintf(out, "target: %s:%s\n", d.Target.Type, d.Target.ID)
			fmt.Fprintf(out, "at: %s\n", d.At)
			if superseded[d.ID] {
				fmt.Fprintln(out, "現行性: 失効（他 decision に mode=supersede で置き換えられた）")
			} else {
				fmt.Fprintln(out, "現行性: 現行")
			}
			fmt.Fprintf(out, "why:\n%s\n", d.Why)
			if d.Changed != "" {
				fmt.Fprintf(out, "changed:\n%s\n", d.Changed)
			}
			if d.Ref != "" {
				fmt.Fprintf(out, "ref: %s\n", d.Ref)
			}
			if len(d.Commits) > 0 {
				fmt.Fprintf(out, "commits: %s\n", strings.Join(d.Commits, " "))
			} else {
				fmt.Fprintln(out, "commits: 未結線")
			}
			if len(d.Acknowledges) > 0 {
				fmt.Fprintf(out, "acknowledges: %s\n", strings.Join(d.Acknowledges, ", "))
			}
			if len(d.Supersedes) > 0 {
				fmt.Fprintln(out, "supersedes（この decision が置き換え/改訂/例外化する旧 decision）:")
				for _, l := range d.Supersedes {
					fmt.Fprintf(out, "  → %s (%s)\n", l.ID, l.SupersedeMode())
				}
			}
			if refs := supersededByIdx[d.ID]; len(refs) > 0 {
				fmt.Fprintln(out, "superseded-by（この decision を指す新 decision・derive）:")
				for _, r := range refs {
					fmt.Fprintf(out, "  ← %s (%s)\n", r.FromID, r.Mode)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する（decision＋derive した superseded-by/superseded）")
	return cmd
}
