package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// vocabUsage は vocab を参照する transition 1 件と、参照しているスロット
// （action/given/then）。「vocab を共有＝実装同一＝使用箇所は真の影響」
// （decision 01KXDRHAR…／meta-decision on concern.traceability）を CLI で
// 見えるようにする。
type vocabUsage struct {
	TxID  string   `json:"txId"`
	Slots []string `json:"slots"`
}

// establishedBy は establishes の逆引き 1 件（この condition を成立させる効果）。
type establishedBy struct {
	EffectID string `json:"effectId"`
}

// vocabShowOutput は show vocab --json の出力形（本体 + usage + 双方向逆引き +
// vocab 宛 decision・#45 D5）。
type vocabShowOutput struct {
	model.VocabEntry
	Usage []vocabUsage `json:"usage"`
	// EstablishedBy はこの vocab（condition）を establishes で成立させる effect
	// の逆引き。Establishes（この effect が成立させる condition）と対で双方向。
	EstablishedBy []establishedBy `json:"establishedBy,omitempty"`
	// Decisions はこの vocab を target とする decision（vocab-target・#45 D5）。
	Decisions []model.Decision `json:"decisions,omitempty"`
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
			establishedBy := establishedByFor(snap, id)
			decisions := vocabDecisionsFor(snap, id)

			if asJSON {
				return emitJSON(cmd, vocabShowOutput{VocabEntry: v, Usage: usage,
					EstablishedBy: establishedBy, Decisions: decisions})
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
			if v.Ref != "" {
				fmt.Fprintf(out, "ref: %s\n", v.Ref)
			}
			if len(v.AltLabels) > 0 {
				fmt.Fprintf(out, "altLabels: %s\n", strings.Join(v.AltLabels, ", "))
			}
			if v.Description != "" {
				fmt.Fprintf(out, "description:\n%s\n", v.Description)
			}
			printVocabUsage(out, usage)
			printEstablishes(out, v.Establishes, establishedBy)
			printVocabDecisions(out, decisions)
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

// establishedByFor は condID を establishes で成立させる effect（この condition
// を成立させる効果）の逆引きを id 昇順で返す（#45 D5・双方向逆引きの片側）。
func establishedByFor(snap store.Snapshot, condID string) []establishedBy {
	var out []establishedBy
	for _, v := range snap.Vocab {
		for _, c := range v.Establishes {
			if c == condID {
				out = append(out, establishedBy{EffectID: v.ID})
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EffectID < out[j].EffectID })
	return out
}

// vocabDecisionsFor は id を target とする vocab-target decision を at 昇順で
// 返す（#45 D5）。
func vocabDecisionsFor(snap store.Snapshot, id string) []model.Decision {
	var out []model.Decision
	for _, d := range snap.Decisions {
		if d.Target.Type == model.DecisionTargetVocab && d.Target.ID == id {
			out = append(out, d)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}

// printEstablishes は双方向逆引きを表示する（#45 D5）:
//   - 「この効果が成立させる条件」＝自身の establishes（effect のとき）
//   - 「この条件を成立させる効果」＝他 vocab の establishes 逆引き（condition のとき）
func printEstablishes(w io.Writer, establishes []string, establishedBy []establishedBy) {
	if len(establishes) > 0 {
		fmt.Fprintf(w, "establishes (この効果が成立させる条件・%d):\n", len(establishes))
		for _, c := range establishes {
			fmt.Fprintf(w, "  - %s\n", c)
		}
	}
	if len(establishedBy) > 0 {
		fmt.Fprintf(w, "establishedBy (この条件を成立させる効果・%d):\n", len(establishedBy))
		for _, e := range establishedBy {
			fmt.Fprintf(w, "  - %s\n", e.EffectID)
		}
	}
}

// printVocabDecisions は vocab 宛 decision 一覧を表示する（#45 D5）。
func printVocabDecisions(w io.Writer, decisions []model.Decision) {
	fmt.Fprintf(w, "decisions (%d):\n", len(decisions))
	if len(decisions) == 0 {
		fmt.Fprintln(w, "  (なし)")
		return
	}
	for _, d := range decisions {
		fmt.Fprintf(w, "  - [%s] %s\n", d.At, d.ID)
		fmt.Fprintf(w, "    why: %s\n", truncateOneLine(d.Why, 100))
		if d.Ref != "" {
			fmt.Fprintf(w, "    ref: %s\n", d.Ref)
		}
	}
}
