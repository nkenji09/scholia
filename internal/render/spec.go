package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/nkenji09/product-memory/internal/index"
	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

// SpecEntry は spec レポート内の 1 遷移分（§3.8 の "WHEN action GIVEN given THEN then" 表示）。
type SpecEntry struct {
	Transition  model.Transition `json:"transition"`
	ActionLabel string           `json:"actionLabel"`
	GivenLabels []string         `json:"givenLabels,omitempty"`
	ThenLabels  []string         `json:"thenLabels,omitempty"`
	Decisions   []model.Decision `json:"decisions,omitempty"`
}

// SpecReport は `pmem spec <subjectTag>` の出力（派生・保存しない・§3.8）。
type SpecReport struct {
	Tag     model.Tag   `json:"tag"`
	Entries []SpecEntry `json:"entries"`
	// RelatedVocab は subjectTag を直接持つ語彙（VocabEntry.Tags の逆引き・
	// H3）。entries（関連仕様）が transition を届けるのと同じ経路でカードへ
	// 載せるので live API・静的 export 双方に効く。omitempty で該当なしは省略。
	RelatedVocab []model.VocabEntry `json:"relatedVocab,omitempty"`
}

// Spec は subjectTag で束ねた"仕様"レポートを構築する。
// 見出しは tag の name/description。本文は実効タグでヒットする各遷移
// （祖先展開の帰結で子タグの遷移も含む・§3.7）を語彙 label 解決して列挙し、
// その遷移自身への decisions と、subjectTag 自体への decisions（cross-cutting・§3.5）を添える。
func Spec(snap *store.Snapshot, ix *index.Index, subjectTag string) (SpecReport, error) {
	tag, ok := ix.TagByID[subjectTag]
	if !ok {
		return SpecReport{}, fmt.Errorf("tag %q が実在しません", subjectTag)
	}

	tagDecisions := decisionsForTarget(snap.Decisions, model.DecisionTargetTag, subjectTag)

	txs := ix.TransitionsByTag(subjectTag)
	entries := make([]SpecEntry, 0, len(txs))
	for _, t := range txs {
		e := SpecEntry{
			Transition:  t,
			ActionLabel: vocabLabel(ix, t.Action),
		}
		for _, g := range t.Given {
			e.GivenLabels = append(e.GivenLabels, vocabLabel(ix, g))
		}
		for _, eff := range t.Then {
			e.ThenLabels = append(e.ThenLabels, vocabLabel(ix, eff))
		}
		e.Decisions = append(e.Decisions, decisionsForTarget(snap.Decisions, model.DecisionTargetTransition, t.ID)...)
		e.Decisions = append(e.Decisions, tagDecisions...)
		entries = append(entries, e)
	}

	return SpecReport{Tag: tag, Entries: entries, RelatedVocab: ix.VocabByTag(subjectTag)}, nil
}

func vocabLabel(ix *index.Index, vocabID string) string {
	if v, ok := ix.VocabByID[vocabID]; ok {
		return v.Label
	}
	return "?"
}

func decisionsForTarget(decisions []model.Decision, targetType, targetID string) []model.Decision {
	var out []model.Decision
	for _, d := range decisions {
		if d.Target.Type == targetType && d.Target.ID == targetID {
			out = append(out, d)
		}
	}
	return out
}

// WriteText は SpecReport を人間可読な形式で書き出す。
func WriteText(w io.Writer, report SpecReport) {
	title := report.Tag.Name
	if title == "" {
		title = report.Tag.ID
	}
	fmt.Fprintf(w, "# %s (%s)\n", title, report.Tag.ID)
	if report.Tag.Description != "" {
		fmt.Fprintln(w, report.Tag.Description)
	}
	fmt.Fprintln(w)

	if len(report.Entries) == 0 {
		fmt.Fprintln(w, "(該当する遷移はありません)")
		return
	}

	for _, e := range report.Entries {
		fmt.Fprintf(w, "## %s\n", e.Transition.ID)

		line := "WHEN " + e.ActionLabel
		if len(e.GivenLabels) > 0 {
			line += " GIVEN " + strings.Join(e.GivenLabels, "、")
		}
		line += " THEN " + strings.Join(e.ThenLabels, " → ")
		fmt.Fprintln(w, line)

		if len(e.Decisions) > 0 {
			fmt.Fprintln(w, "decisions:")
			for _, d := range e.Decisions {
				if d.Ref != "" {
					fmt.Fprintf(w, "  - %s (%s)\n", d.Why, d.Ref)
				} else {
					fmt.Fprintf(w, "  - %s\n", d.Why)
				}
			}
		}
		fmt.Fprintln(w)
	}
}
