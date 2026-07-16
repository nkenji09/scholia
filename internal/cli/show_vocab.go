package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/model"
)

// vocabUsage は vocab を参照する transition 1 件と、参照しているスロット
// （action/given/then）。「vocab を共有＝実装同一＝使用箇所は真の影響」
// （decision 01KXDRHAR…／meta-decision on concern.traceability）を CLI で
// 見えるようにする。
type vocabUsage struct {
	TxID  string   `json:"txId"`
	Slots []string `json:"slots"`
}

// vocabShowOutput は show vocab --json の出力形（本体 + usage）。
type vocabShowOutput struct {
	model.VocabEntry
	Usage []vocabUsage `json:"usage"`
}

func newShowVocabCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "vocab <id>",
		Short: "語彙を 1 件表示する（使用箇所＝参照 transition の逆引きを含む）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			s, err := openStore()
			if err != nil {
				return err
			}
			v, err := s.LoadVocab(id)
			if err != nil {
				return fmt.Errorf("vocab %q を読み込めません: %w", id, err)
			}

			snap, err := s.LoadAll()
			if err != nil {
				return err
			}
			ix := index.Build(&snap)
			usage := vocabUsageFor(ix, id)

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(vocabShowOutput{VocabEntry: v, Usage: usage})
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "id: %s\n", v.ID)
			fmt.Fprintf(out, "category: %s\n", v.Category)
			fmt.Fprintf(out, "label: %s\n", v.Label)
			if v.Kind != "" {
				fmt.Fprintf(out, "kind: %s\n", v.Kind)
			}
			if v.Owner != "" {
				fmt.Fprintf(out, "owner: %s\n", v.Owner)
			}
			if len(v.Tags) > 0 {
				fmt.Fprintf(out, "tags: %s\n", strings.Join(v.Tags, ", "))
			}
			if v.Description != "" {
				fmt.Fprintf(out, "description:\n%s\n", v.Description)
			}
			printVocabUsage(out, usage)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する（usage を含む）")
	return cmd
}

// vocabUsageFor は vocabID を参照する transition を id 昇順（index 由来）で
// 集め、action/given/then のどのスロットで参照されているかを添える。
func vocabUsageFor(ix *index.Index, vocabID string) []vocabUsage {
	txs := ix.TransitionsByVocab(vocabID)
	usage := make([]vocabUsage, 0, len(txs))
	for _, t := range txs {
		var slots []string
		if t.Action == vocabID {
			slots = append(slots, "action")
		}
		for _, g := range t.Given {
			if g == vocabID {
				slots = append(slots, "given")
				break
			}
		}
		for _, e := range t.Then {
			if e == vocabID {
				slots = append(slots, "then")
				break
			}
		}
		usage = append(usage, vocabUsage{TxID: t.ID, Slots: slots})
	}
	return usage
}

func printVocabUsage(w io.Writer, usage []vocabUsage) {
	fmt.Fprintf(w, "usage (%d transitions):\n", len(usage))
	if len(usage) == 0 {
		fmt.Fprintln(w, "  (なし)")
		return
	}
	for _, u := range usage {
		fmt.Fprintf(w, "  - %s (%s)\n", u.TxID, strings.Join(u.Slots, ","))
	}
}
