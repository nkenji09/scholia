package cli

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// rulesOutput は --json 出力の形。
//
// decisions は「その記録自身への decision で、効いているもの」だけ（既定）。
// 経由で届くものは inherited に、取り下げられたものは withdrawn に、それぞれ
// **本文なしで**載る（--all で全部 decisions 側へ合流する）。各要素は effect を
// 必ず持つので、消費側が全件走査して逆リンクを組まなくても効力が分かる。
type rulesOutput struct {
	Decisions []decisionOut  `json:"decisions"`
	Inherited []inheritedOut `json:"inherited,omitempty"`
	Withdrawn []withdrawnOut `json:"withdrawn,omitempty"`
}

// inheritedOut は経由で届く 1 件の「存在・出自・経由タグ」だけの出力形
// （01KZ06SYP12ZFDG1WPNYM529D8 変更6）。
//
// **why / changed / ref を持たない。** 人が読む出力で本文を出さないのに JSON では
// 出す、という食い違いを作らないため。機械で読む側は provenance を見れば
// 「本文を受け取っていない集合」をそのまま検出できる。
//
// heading は**著者が見出しとして書いた行**（model.CheckDecisionHeading）だけで、
// 本文から機械で切り出したものではない（変更3「1 行目・第 1 文・先頭 N 字の
// いずれも出さない」）。人が読む出力に出るのと同じ行なので、ここにも載せる。
type inheritedOut struct {
	ID           string                  `json:"id"`
	Target       model.DecisionTarget    `json:"target"`
	At           string                  `json:"at"`
	Provenance   index.GovernsProvenance `json:"provenance"`
	ViaTag       string                  `json:"viaTag,omitempty"`
	Heading      string                  `json:"heading,omitempty"`
	Effect       Effect                  `json:"effect"`
	SupersededBy []supersededByOut       `json:"supersededBy,omitempty"`
}

func newRulesCmd() *cobra.Command {
	var tagID, txID, vocabID, facet, sortBy string
	var asJSON, current, all bool
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "対象（tag/transition/vocab/facet）を支配している decisions を集約する（§3.8）",
		Long: "対象（tag/transition/vocab/facet）を支配している decisions を集約する（§3.8）。\n\n" +
			"既定で本文を返すのは、**その記録自身を対象とする decision** だけ。タグ経由で届く\n" +
			"decision（その記録が直接持つタグ／その祖先タグへのもの）は、存在・経由タグ・引き方\n" +
			"だけを返す——本文は、それが書かれた記録を引いたときに読む。全部を本文で読むには --all。",
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

			entries, err := governsForSelector(&snap, tagID, txID, vocabID, facet)
			if err != nil {
				return err
			}

			// 何を本文で渡すかの判断は rules_fold.go の純関数にある。ここは
			// 並べ替えと描き方だけ。
			view := newCurrencyView(snap.Decisions)
			sortGovernsEntries(entries, sortBy)
			groups := foldRules(entries, func(id string) bool { return view.effectOf(id) == EffectReplaced }, all)

			if asJSON {
				return emitJSON(cmd, rulesOutput{
					Decisions: view.decisionOuts(governsDecisions(groups.Bodies)),
					Inherited: view.inheritedOuts(groups.Inherited),
					Withdrawn: view.withdrawnOuts(governsDecisions(groups.Withdrawn)),
				})
			}
			writeRulesText(cmd.OutOrStdout(), groups, sortBy, view, rulesAllCommand(tagID, txID, vocabID, facet))
			return nil
		},
	}
	cmd.Flags().StringVar(&tagID, "tag", "", "タグを対象にする（既定はそのタグ自身への decisions の本文。祖先タグへの分は存在と引き方）")
	cmd.Flags().StringVar(&txID, "tx", "", "遷移を対象にする（既定はその遷移自身への decisions の本文。実効タグ経由の分は存在と引き方）")
	cmd.Flags().StringVar(&vocabID, "vocab", "", "語彙を対象にする（既定はその語彙自身への decisions の本文。語彙が持つタグ〔vocab.tags〕とその祖先経由の分は存在と引き方・#45 D10b）")
	cmd.Flags().StringVar(&facet, "facet", "", "指定 kind を持つ全タグを対象にする（経由という概念が無いので畳まない）")
	cmd.Flags().StringVar(&sortBy, "sort", "chrono", "並び順（chrono=at昇順・既定 | target=対象ごとにグループ化）")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON で出力する")
	cmd.Flags().BoolVar(&all, "all", false, "畳んでいるものを全部開く（経由で届く分と取り下げられた分を本文ごと出す）")
	cmd.Flags().BoolVar(&current, "current", false, "既定と同じ（後方互換のため受理する。畳んだ分の本文は既定でも出ません）")
	return cmd
}

// governsForSelector は選択子に応じた「支配している規則」を**出自つきで**返す。
//
// tag / tx / vocab は index.GovernsFor*（viewer が使っているのと同じ関数）を
// 通す——面間整合のため Go 側に 3 本目を書かない
// （01KZ06SYP12ZFDG1WPNYM529D8 変更1）。返す集合は従来の
// index.SelectRulesDecisionsFor と同一で、`--all` は現行の既定と同じ集合になる。
//
// --facet と選択子なしには「経由」という概念が無いので、畳み込みは当たらない
// （同 変更1 ⚠️）。従来経路のまま全件を own として返す。
func governsForSelector(snap *store.Snapshot, tagID, txID, vocabID, facet string) ([]index.GovernsEntry, error) {
	switch {
	case txID != "":
		return index.GovernsForTransition(snap, txID)
	case tagID != "":
		return index.GovernsForTag(snap, tagID)
	case vocabID != "":
		return index.GovernsForVocab(snap, vocabID)
	}
	decisions, err := index.SelectRulesDecisionsFor(snap, "", "", "", facet)
	if err != nil {
		return nil, err
	}
	return ownEntries(decisions), nil
}

// rulesAllCommand は畳んだ側を本文ごと開くコマンド（変更5「引き方」）。
//
// ⚠️ **`--all` は畳んだ集合の上位集合である**——上に本文で出た自身への decision も
// 再掲される。正本（変更5）は「畳んだ集合ちょうどを返す形」を求めているが、その
// 呼び方は現存せず、作れば変更2 の「表示フラグを増やさない」に反する。`--all` を
// 採り、再掲があることを出力に明記する（差の実測は実装単位の result.md）。
// 経由タグを `rules --tag` で引き直す形は提示しない——経由タグの祖先まで再展開
// して同じ本文を二度払うので、正本が費用の面で負ける形として名指しで禁じている。
func rulesAllCommand(tagID, txID, vocabID, facet string) string {
	switch {
	case txID != "":
		return fmt.Sprintf("scholia rules --tx %s --all", txID)
	case tagID != "":
		return fmt.Sprintf("scholia rules --tag %s --all", tagID)
	case vocabID != "":
		return fmt.Sprintf("scholia rules --vocab %s --all", vocabID)
	case facet != "":
		return fmt.Sprintf("scholia rules --facet %s --all", facet)
	}
	return "scholia rules --all"
}

// writeRulesText は人が読む出力を書く。
//
// **3 群のどれもが必ず出る。** 自身への decision が 0 件でも「該当なし」とは
// 書かない——「無い」と「別の場所に在る」は読み手にとって別の事実である
// （01KZ06SYP12ZFDG1WPNYM529D8 変更4 ⚠️）。
func writeRulesText(out io.Writer, g foldedRules, sortBy string, view currencyView, allCmd string) {
	if len(g.Bodies) == 0 && len(g.Inherited) == 0 && len(g.Withdrawn) == 0 {
		fmt.Fprintln(out, "rules: 該当する decision はありません")
		return
	}
	if len(g.Bodies) == 0 {
		fmt.Fprintf(out, "%s\n", ownEmptyHeading(len(g.Inherited)))
	} else {
		printRules(out, governsDecisions(g.Bodies), sortBy, view)
	}
	writeInherited(out, g.Inherited, view, allCmd)
	writeWithdrawn(out, governsDecisions(g.Withdrawn), view, "")
}

// ownEmptyHeading は「自身への decision が 0 件」を、事実として述べる。
//
// 実運用で引かれた遷移はいずれも自身への decision が 0 件だった。要件はタグに
// 書き、遷移には書かないのが実態である——現行の出力はこの事実を隠していた。
func ownEmptyHeading(inherited int) string {
	if inherited == 0 {
		return "rules: この記録自身への意思決定は 0 件です（効いているものはありません）"
	}
	return fmt.Sprintf("rules: この記録自身への意思決定は 0 件です。%d 件が経由で支配しています（下記）", inherited)
}

// inheritedHeading は畳んだ側の見出し。
//
// ⚠️ 読み飛ばしてよいと解釈できる語（「参考」「関連」等）を使わない
// （変更4）。**本文を読んでいないことが分かる語**で書く。
func inheritedHeading(n int) string {
	return fmt.Sprintf("経由でこの記録を支配している規則 %d件（本文は出していません——読まないと内容は分かりません）:", n)
}

// writeInherited は畳んだ側を 1 件 1 行で書く（変更3）。
//
// 出すのは id・日付・対象・経由タグ・経由の種別、そして**著者が見出しとして
// 書いた行がある場合のみ**その行。**本文から切り出したものは出さない**——
// 1 行目・第 1 文・先頭 N 字のいずれも出さない。見出しの判定は
// model.CheckDecisionHeading（保存時ゲートと同じ述語）。
func writeInherited(out io.Writer, entries []index.GovernsEntry, view currencyView, allCmd string) {
	if len(entries) == 0 {
		return
	}
	fmt.Fprintln(out, inheritedHeading(len(entries)))
	for _, e := range entries {
		d := e.Decision
		fmt.Fprintf(out, "  [%s] %s %s%s%s\n",
			d.At, d.ID, targetLabel(d.Target), viaLabel(e), inheritedWithdrawnMark(view, d.ID))
		if h, ok := model.DecisionHeadingOf(d.Why); ok {
			fmt.Fprintf(out, "      # %s\n", h)
		}
	}
	// 経由元へ到達できる導線を、畳んだ全件について出す（変更5）。
	fmt.Fprintf(out, "  本文ごと読む: %s（上に本文で出た分も再掲されます）\n", allCmd)
}

func targetLabel(t model.DecisionTarget) string {
	return fmt.Sprintf("%s:%s", t.Type, t.ID)
}

// viaLabel は経由の種別と経由タグ。経由タグが対象と同じ（タグ宛て decision を
// そのタグ経由で受け取る通常のかたち）なら種別だけを出し、同じ id を二度書かない。
func viaLabel(e index.GovernsEntry) string {
	kind := provenanceLabel(e.Provenance)
	if kind == "" {
		return ""
	}
	if e.ViaTag == "" || (e.Decision.Target.Type == model.DecisionTargetTag && e.Decision.Target.ID == e.ViaTag) {
		return fmt.Sprintf("（%s経由）", kind)
	}
	return fmt.Sprintf("（%s tag:%s 経由）", kind, e.ViaTag)
}

// inheritedWithdrawnMark は「経由で届き、かつ取り下げられている」1 件に付ける印。
// 深掘りするかを決める材料そのものなので、畳んだ行にも必ず出す。
func inheritedWithdrawnMark(view currencyView, id string) string {
	if view.effectOf(id) == EffectReplaced {
		return withdrawnMarkLabel
	}
	return ""
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

func printRules(out io.Writer, decisions []model.Decision, sortBy string, view currencyView) {
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
