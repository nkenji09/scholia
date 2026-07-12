package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/product-memory/internal/model"
)

// txView は show tx の出力形。--resolve 時のみラベルを添える。
type txView struct {
	model.Transition
	ActionLabel string   `json:"actionLabel,omitempty"`
	GivenLabels []string `json:"givenLabels,omitempty"`
	ThenLabels  []string `json:"thenLabels,omitempty"`
}

func newShowTxCmd() *cobra.Command {
	var resolve, asJSON bool
	cmd := &cobra.Command{
		Use:   "tx <id>",
		Short: "遷移を 1 件表示する",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			s, err := openStore()
			if err != nil {
				return err
			}
			t, err := s.LoadTransition(id)
			if err != nil {
				return fmt.Errorf("transition %q を読み込めません: %w", id, err)
			}

			view := txView{Transition: t}
			if resolve {
				label := func(vocabID string) string {
					v, err := s.LoadVocab(vocabID)
					if err != nil {
						return "?"
					}
					return v.Label
				}
				view.ActionLabel = label(t.Action)
				for _, g := range t.Given {
					view.GivenLabels = append(view.GivenLabels, label(g))
				}
				for _, e := range t.Then {
					view.ThenLabels = append(view.ThenLabels, label(e))
				}
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(view)
			}
			printTxView(cmd, view, resolve)
			return nil
		},
	}
	cmd.Flags().BoolVar(&resolve, "resolve", false, "語彙 label を解決して表示する")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	return cmd
}

func printTxView(cmd *cobra.Command, v txView, resolve bool) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "id: %s\n", v.ID)
	if resolve {
		fmt.Fprintf(out, "action: %s (%s)\n", v.Action, v.ActionLabel)
	} else {
		fmt.Fprintf(out, "action: %s\n", v.Action)
	}
	fmt.Fprintf(out, "given:\n")
	for i, g := range v.Given {
		if resolve {
			fmt.Fprintf(out, "  - %s (%s)\n", g, v.GivenLabels[i])
		} else {
			fmt.Fprintf(out, "  - %s\n", g)
		}
	}
	fmt.Fprintf(out, "then:\n")
	for i, e := range v.Then {
		if resolve {
			fmt.Fprintf(out, "  %d. %s (%s)\n", i+1, e, v.ThenLabels[i])
		} else {
			fmt.Fprintf(out, "  %d. %s\n", i+1, e)
		}
	}
	if len(v.Tags) > 0 {
		fmt.Fprintf(out, "tags: %s\n", strings.Join(v.Tags, ", "))
	}
}
