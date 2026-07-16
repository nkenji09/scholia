package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/diff"
)

func newDiffCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "diff [<ref1> [<ref2>]]",
		Short: "作業ツリー vs gitref（既定 HEAD）、または gitref 対 gitref の semantic diff（§4）",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore()
			if err != nil {
				return err
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

			if result.BaselineMissing {
				fmt.Fprintf(cmd.ErrOrStderr(), "注記: %s にベースライン（.scholia）が見つかりません。初回とみなし、現在の全レコードを新規(added)として表示します。\n", result.Ref)
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(result); err != nil {
					return err
				}
			} else {
				printDiff(cmd, result)
			}

			if result.DecisionViolation() {
				return fmt.Errorf("decisions の削除／改変を検出しました（append-only 違反）")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
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
		fmt.Fprintf(w, "  ! 改変（append-only 違反）: %s\n", c.ID)
	}
}
