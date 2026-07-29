package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// SpecEntry は spec レポート内の 1 遷移分（§3.8 の "WHEN action GIVEN given THEN then" 表示）。
type SpecEntry struct {
	Transition  model.Transition `json:"transition"`
	ActionLabel string           `json:"actionLabel"`
	GivenLabels []string         `json:"givenLabels,omitempty"`
	ThenLabels  []string         `json:"thenLabels,omitempty"`
	Decisions   []model.Decision `json:"decisions,omitempty"`
}

// SpecReport は `scholia spec <subjectTag>` の出力（派生・保存しない・§3.8）。
type SpecReport struct {
	Tag     model.Tag   `json:"tag"`
	Entries []SpecEntry `json:"entries"`
	// TagDecisions は subjectTag 自体を target とする decision（§3.5 cross-cutting）。
	// decision は tag に直接ぶら下がる第一級レコードで、その tag が transition を
	// 持つか（entries が空か）に関係なく常にカードへ載せる。従来は各 entry.Decisions
	// に重複付与していたが、TransitionsByTag が 0 件のタグでは貼る先の entry が無く
	// decision が完全に消えていた（tag-decision-visibility）。特に「【不採用】＝実装
	// しない」判断は transition を持たないタグに付くため、一番残したい履歴が一番
	// 確実に消える性質があった。トップレベルへ一本化し、entries には transition
	// 自身の decision だけを残す。omitempty で該当なしは省略。
	TagDecisions []model.Decision `json:"tagDecisions,omitempty"`
	// RelatedVocab は subjectTag を直接持つ語彙（VocabEntry.Tags の逆引き・
	// H3）。entries（関連仕様）が transition を届けるのと同じ経路でカードへ
	// 載せるので live API・静的 export 双方に効く。omitempty で該当なしは省略。
	RelatedVocab []model.VocabEntry `json:"relatedVocab,omitempty"`
	// Axis は axis kind タグの構造表示（#45 D10b-6）: 状態次元 total・値（軸タグ
	// 付き condition）と各値が効く action。axis 以外のタグ・値の無い軸では nil
	// （omitempty で省略）。導出は index.BuildAxisStructure（live/static 共有）。
	Axis *index.AxisStructure `json:"axis,omitempty"`
}

// Spec は subjectTag で束ねた"仕様"レポートを構築する。
// 見出しは tag の name/description。本文は実効タグでヒットする各遷移
// （祖先展開の帰結で子タグの遷移も含む・§3.7）を語彙 label 解決して列挙し、
// その遷移自身への decisions を各 entry に添える。subjectTag 自体への
// decisions（cross-cutting・§3.5）は entries とは独立にトップレベル
// TagDecisions へ載せる（entries が空でも消えないように・tag-decision-visibility）。
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
		e.Decisions = decisionsForTarget(snap.Decisions, model.DecisionTargetTransition, t.ID)
		entries = append(entries, e)
	}

	// axis 構造（#45 D10b-6）: axis kind のタグにだけ載る。kind の axis 挙動は
	// config の宣言（D9・KindHasBehavior）で判定する（literal "axis" 直書きは
	// しない）。
	axis := index.BuildAxisStructure(ix, subjectTag, snap.Config.KindHasBehavior(tag.Kind, model.BehaviorAxis))

	return SpecReport{Tag: tag, Entries: entries, TagDecisions: tagDecisions, RelatedVocab: ix.VocabByTag(subjectTag), Axis: axis}, nil
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

// DecisionSplitter は「本文まで出す群」と「存在と行き先だけ出す群」に分ける判断と、
// 後者の書き出しを、レポートの組み立てから切り離して受け取るための口。
//
// 判断の本体は internal/cli 側の純関数（currencyView）にある。ここへ持ち込まないのは、
// SpecReport 自体は viewer（internal/viewer/derived.go）とも共有していて、
// **画面は取り下げも含めた全件を受け取って自分で畳む**必要があるから——
// レポートの中身を削ると画面の折りたたみが空になる。畳むのは端末の書き出しだけ。
type DecisionSplitter interface {
	// SplitDecisions は入力の順序を保ったまま 2 群に分ける。
	SplitDecisions(decisions []model.Decision) (bodies, withdrawn []model.Decision)
	// WriteWithdrawn は withdrawn 群を「存在と行き先」だけの形で書く（本文は出さない）。
	WriteWithdrawn(w io.Writer, withdrawn []model.Decision, indent string)
	// EffectLabel は本文側に出す 1 件に添える効力の印。効いているものには空を返す。
	// --all で取り下げが本文側へ合流したとき、それが取り下げ済みだと分かるために要る
	// ——本文が読めるのに効力が読めないと、rules --all と揃わない。
	EffectLabel(d model.Decision) string
}

// allInForce は何も畳まない DecisionSplitter（--all 相当・nil 渡し時の既定）。
type allInForce struct{}

func (allInForce) SplitDecisions(d []model.Decision) ([]model.Decision, []model.Decision) {
	return d, nil
}
func (allInForce) WriteWithdrawn(io.Writer, []model.Decision, string) {}
func (allInForce) EffectLabel(model.Decision) string                  { return "" }

// WriteText は SpecReport を人間可読な形式で書き出す。
// split が nil なら何も畳まない（従来どおり全件を本文ごと出す）。
func WriteText(w io.Writer, report SpecReport, split DecisionSplitter) {
	if split == nil {
		split = allInForce{}
	}
	title := report.Tag.Name
	if title == "" {
		title = report.Tag.ID
	}
	fmt.Fprintf(w, "# %s (%s)\n", title, report.Tag.ID)
	if report.Tag.Description != "" {
		fmt.Fprintln(w, report.Tag.Description)
	}
	fmt.Fprintln(w)

	// タグ自体への decision は entries とは独立にトップレベルで出す。transition を
	// 持たないタグ（entries=0）でも decision が見えるように（tag-decision-visibility）。
	if len(report.TagDecisions) > 0 {
		bodies, withdrawn := split.SplitDecisions(report.TagDecisions)
		if len(bodies) > 0 {
			writeDecisions(w, bodies, split)
		}
		split.WriteWithdrawn(w, withdrawn, "")
		fmt.Fprintln(w)
	}

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
			bodies, withdrawn := split.SplitDecisions(e.Decisions)
			if len(bodies) > 0 {
				writeDecisions(w, bodies, split)
			}
			split.WriteWithdrawn(w, withdrawn, "")
		}
		fmt.Fprintln(w)
	}
}

// writeDecisions は decision 群を "decisions:" 見出し付きの箇条書きで書き出す。
// トップレベルのタグ decision と各遷移の decision で同一体裁を共有する。
func writeDecisions(w io.Writer, decisions []model.Decision, split DecisionSplitter) {
	fmt.Fprintln(w, "decisions:")
	for _, d := range decisions {
		label := split.EffectLabel(d)
		if d.Ref != "" {
			fmt.Fprintf(w, "  -%s %s (%s)\n", label, d.Why, d.Ref)
		} else {
			fmt.Fprintf(w, "  -%s %s\n", label, d.Why)
		}
	}
}
