package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/diff"
)

func newDiffCmd() *cobra.Command {
	var asJSON, check bool
	var allowRetrofit string
	cmd := &cobra.Command{
		Use:   "diff [<ref1> [<ref2>]]",
		Short: "作業ツリー vs gitref（既定 HEAD）、または gitref 対 gitref の semantic diff（§4）",
		Long: "semantic diff（§4）。decision の変更は欄位単位で正規化される（#45 U4）:\n" +
			"commits の追記・rename/merge 追随の target.id 張替えは許容、decision の削除と\n" +
			"判断欄位（why/changed/ref/at・target.type）の改変は append-only 違反。\n" +
			"\n" +
			"意味論の差（既定 vs --check）:\n" +
			"  既定    … semantic diff 全体（vocab/tags/transitions/decisions の ±・変更）を\n" +
			"            レポート表示する。append-only 違反があれば一覧表示のうえ exit 1。\n" +
			"  --check … CI ゲート。差分全体は表示せず decision の append-only 判定だけを\n" +
			"            行い、違反を欄位名つきで列挙して exit 1（違反なしは OK 1 行・exit 0）。\n" +
			"\n" +
			"逃し弁（#42 型の全店 retrofit 用・明示の例外承認）: --allow-decision-retrofit <理由>\n" +
			"または環境変数 SCHOLIA_ALLOW_DECISION_RETROFIT=<理由> で、違反を警告に降格して\n" +
			"exit 0 にする。理由は必須で、text 出力と --json（retrofitAllowed/retrofitReason）に\n" +
			"記録される。",
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
			}

			if cmd.Flags().Changed("allow-decision-retrofit") && strings.TrimSpace(allowRetrofit) == "" {
				return fmt.Errorf("--allow-decision-retrofit には理由が必須です（明示の例外承認・出力に記録されます）")
			}
			retrofitReason := strings.TrimSpace(allowRetrofit)
			if retrofitReason == "" {
				retrofitReason = strings.TrimSpace(os.Getenv("SCHOLIA_ALLOW_DECISION_RETROFIT"))
			}

			var result diff.Result
			switch len(args) {
			case 2:
				result, err = diff.DiffRefs(s, args[0], args[1])
			case 1:
				result, err = diff.Diff(s, args[0])
			default:
				result, err = diff.Diff(s, "")
			}
			if err != nil {
				return err
			}

			violation := result.DecisionViolation()
			if violation && retrofitReason != "" {
				result.RetrofitAllowed = true
				result.RetrofitReason = retrofitReason
			}

			if result.BaselineMissing {
				fmt.Fprintf(cmd.ErrOrStderr(), "注記: %s にベースライン（.scholia）が見つかりません。初回とみなし、現在の全レコードを新規(added)として表示します。\n", result.Ref)
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(result); err != nil {
					return err
				}
			} else if check {
				printDiffCheck(cmd, result)
			} else {
				printDiff(cmd, result)
			}

			if violation {
				if result.RetrofitAllowed {
					if !asJSON {
						fmt.Fprintf(cmd.OutOrStdout(),
							"append-only 違反を明示の例外承認により警告へ降格しました（理由: %s）→ exit 0\n",
							result.RetrofitReason)
					}
					return nil
				}
				return fmt.Errorf("decisions の削除／判断欄位の改変を検出しました（append-only 違反・欄位単位）")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	cmd.Flags().BoolVar(&check, "check", false,
		"CI ゲートモード: decision の append-only 判定のみ行い、違反を欄位名つきで列挙して exit 1（差分全体は表示しない）")
	cmd.Flags().StringVar(&allowRetrofit, "allow-decision-retrofit", "",
		"append-only 違反を警告に降格する明示の例外承認（#42 型 retrofit 用・理由必須・出力に記録される。環境変数 SCHOLIA_ALLOW_DECISION_RETROFIT=<理由> でも可）")
	return cmd
}

// printDiffCheck は --check（CI ゲート）の出力: decision の append-only 判定に
// 絞り、違反を欄位名つきで列挙する。差分全体のレポートは既定モードの仕事。
func printDiffCheck(cmd *cobra.Command, r diff.Result) {
	out := cmd.OutOrStdout()
	allowedChanged := 0
	for _, c := range r.Decisions.Changed {
		if !c.Violation() {
			allowedChanged++
		}
	}
	for _, dec := range r.Decisions.Removed {
		fmt.Fprintf(out, "[violation] decision %s: ファイルが削除されています（append-only 違反）\n", dec.ID)
	}
	for _, c := range r.Decisions.Changed {
		if c.Violation() {
			fmt.Fprintf(out, "[violation] decision %s: 判断欄位 %s が改変されています（append-only 違反）\n",
				c.ID, strings.Join(c.ViolatedFields, ","))
		}
	}
	if !r.DecisionViolation() {
		fmt.Fprintf(out, "decisions: append-only OK（added %d・changed %d〔許容欄位のみ〕）→ exit 0\n",
			len(r.Decisions.Added), allowedChanged)
	}
}

func printDiff(cmd *cobra.Command, r diff.Result) {
	out := cmd.OutOrStdout()
	if r.AfterRef != "" {
		fmt.Fprintf(out, "diff: %s vs %s\n", r.Ref, r.AfterRef)
	} else {
		fmt.Fprintf(out, "diff: 作業ツリー vs %s\n", r.Ref)
	}
	if r.Empty() {
		fmt.Fprintln(out, "差分なし")
		return
	}

	printVocabDiff(out, r.Vocab)
	printTagDiff(out, r.Tags)
	printTransitionDiff(out, r.Transitions)
	printDecisionDiff(out, r.Decisions)
}

func printVocabDiff(w io.Writer, d diff.VocabDiff) {
	if len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0 {
		return
	}
	fmt.Fprintln(w, "vocab:")
	for _, v := range d.Added {
		fmt.Fprintf(w, "  + %s (%s)\n", v.ID, v.Label)
	}
	for _, v := range d.Removed {
		fmt.Fprintf(w, "  - %s (%s)\n", v.ID, v.Label)
	}
	for _, c := range d.Changed {
		fmt.Fprintf(w, "  ~ %s\n", c.ID)
	}
}

func printTagDiff(w io.Writer, d diff.TagDiff) {
	if len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0 {
		return
	}
	fmt.Fprintln(w, "tags:")
	for _, t := range d.Added {
		fmt.Fprintf(w, "  + %s (%s)\n", t.ID, t.Name)
	}
	for _, t := range d.Removed {
		fmt.Fprintf(w, "  - %s (%s)\n", t.ID, t.Name)
	}
	for _, c := range d.Changed {
		fmt.Fprintf(w, "  ~ %s\n", c.ID)
	}
}

func printTransitionDiff(w io.Writer, d diff.TransitionDiff) {
	if len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0 {
		return
	}
	fmt.Fprintln(w, "transitions:")
	for _, t := range d.Added {
		fmt.Fprintf(w, "  + %s\n", t.ID)
	}
	for _, t := range d.Removed {
		fmt.Fprintf(w, "  - %s\n", t.ID)
	}
	for _, c := range d.Changed {
		fmt.Fprintf(w, "  ~ %s\n", c.ID)
		if c.ActionChanged {
			fmt.Fprintf(w, "      action: %s -> %s\n", c.Before.Action, c.After.Action)
		}
		if len(c.GivenAdded) > 0 || len(c.GivenRemoved) > 0 {
			fmt.Fprintf(w, "      given: +%s -%s\n", strings.Join(c.GivenAdded, ","), strings.Join(c.GivenRemoved, ","))
		}
		if c.ThenReordered {
			fmt.Fprintf(w, "      then: 並び替え [%s] -> [%s]\n", strings.Join(c.Before.Then, ","), strings.Join(c.After.Then, ","))
		} else if c.ThenChanged {
			fmt.Fprintf(w, "      then: [%s] -> [%s]\n", strings.Join(c.Before.Then, ","), strings.Join(c.After.Then, ","))
		}
		if len(c.TagsAdded) > 0 || len(c.TagsRemoved) > 0 {
			fmt.Fprintf(w, "      tags: +%s -%s\n", strings.Join(c.TagsAdded, ","), strings.Join(c.TagsRemoved, ","))
		}
	}
}

func printDecisionDiff(w io.Writer, d diff.DecisionDiff) {
	if len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0 {
		return
	}
	fmt.Fprintln(w, "decisions:")
	for _, dec := range d.Added {
		fmt.Fprintf(w, "  + %s (%s: %s)\n", dec.ID, dec.Target.Type, dec.Target.ID)
	}
	for _, dec := range d.Removed {
		fmt.Fprintf(w, "  ! 削除（append-only 違反）: %s (%s: %s)\n", dec.ID, dec.Target.Type, dec.Target.ID)
	}
	for _, c := range d.Changed {
		if c.Violation() {
			fmt.Fprintf(w, "  ! 改変（append-only 違反・判断欄位: %s）: %s\n", strings.Join(c.ViolatedFields, ","), c.ID)
		} else {
			fmt.Fprintf(w, "  ~ %s（許容欄位のみ: %s）\n", c.ID, strings.Join(c.AllowedFields, ", "))
		}
	}
}
