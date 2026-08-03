package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/model"
)

// 「本文を渡すか、存在と引き方だけ渡すか」は入力と出力の対で検査する
// （CLAUDE.md「配線ガードの書き方」1）。出力の書式を照合する検査にすると、
// 同じ判断を別の綴りで書き直された瞬間に捕まらなくなる（同 2）。

func entry(id string, p index.GovernsProvenance, via string) index.GovernsEntry {
	return index.GovernsEntry{
		Decision:   model.Decision{ID: id, Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: via}},
		Provenance: p,
		ViaTag:     via,
	}
}

func idsOf(entries []index.GovernsEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Decision.ID)
	}
	return out
}

func TestFoldRules(t *testing.T) {
	own := index.GovernsEntry{Decision: model.Decision{ID: "own-live"}, Provenance: index.GovernsOwn}
	ownGone := index.GovernsEntry{Decision: model.Decision{ID: "own-withdrawn"}, Provenance: index.GovernsOwn}
	direct := entry("direct", index.GovernsEffectiveTag, "subject.a")
	ancestor := entry("ancestor", index.GovernsViaParent, "subject.root")
	ancestorGone := entry("ancestor-withdrawn", index.GovernsViaParent, "subject.root")

	all := []index.GovernsEntry{own, ownGone, direct, ancestor, ancestorGone}
	replaced := func(id string) bool { return id == "own-withdrawn" || id == "ancestor-withdrawn" }

	t.Run("既定", func(t *testing.T) {
		got := foldRules(all, replaced, false)
		// 本文を渡すのは「自身への decision で、効いているもの」だけ。
		if want := []string{"own-live"}; !reflect.DeepEqual(idsOf(got.Bodies), want) {
			t.Errorf("Bodies = %v, want %v", idsOf(got.Bodies), want)
		}
		// 経由で届くものは、取り下げ済みも含めて畳んだ側に寄せる。
		// 取り下げ群へ移すと、畳んだ側の件数が --all の集合と一致しなくなる。
		if want := []string{"direct", "ancestor", "ancestor-withdrawn"}; !reflect.DeepEqual(idsOf(got.Inherited), want) {
			t.Errorf("Inherited = %v, want %v", idsOf(got.Inherited), want)
		}
		if want := []string{"own-withdrawn"}; !reflect.DeepEqual(idsOf(got.Withdrawn), want) {
			t.Errorf("Withdrawn = %v, want %v", idsOf(got.Withdrawn), want)
		}
	})

	t.Run("--all は何も畳まない", func(t *testing.T) {
		got := foldRules(all, replaced, true)
		if !reflect.DeepEqual(idsOf(got.Bodies), idsOf(all)) {
			t.Errorf("Bodies = %v, want 全件", idsOf(got.Bodies))
		}
		if len(got.Inherited) != 0 || len(got.Withdrawn) != 0 {
			t.Errorf("--all では畳んだ群が空のはず: %+v", got)
		}
	})

	// 3 群の合計は入力の全件と等しい——1 件も落とさない（受け入れ基準）。
	t.Run("1 件も落とさない", func(t *testing.T) {
		for _, showAll := range []bool{false, true} {
			got := foldRules(all, replaced, showAll)
			if n := len(got.Bodies) + len(got.Inherited) + len(got.Withdrawn); n != len(all) {
				t.Errorf("showAll=%v: 3 群の合計が %d 件（入力は %d 件）", showAll, n, len(all))
			}
		}
	})

	t.Run("空入力", func(t *testing.T) {
		got := foldRules(nil, replaced, false)
		if len(got.Bodies)+len(got.Inherited)+len(got.Withdrawn) != 0 {
			t.Errorf("空入力なら 3 群とも空: %+v", got)
		}
	})
}

// 経由の種別は区別して出す（01KZ06SYP12ZFDG1WPNYM529D8 変更3 ⚠️）。
// 「この遷移が属する要件の規則」と「その上位領域の規則」は読み手にとって
// 別の重みを持つので、同じ語で束ねない。
func TestProvenanceLabelDistinguishesDirectFromAncestor(t *testing.T) {
	direct := provenanceLabel(index.GovernsEffectiveTag)
	ancestor := provenanceLabel(index.GovernsViaParent)
	if direct == "" || ancestor == "" {
		t.Fatalf("経由の種別に語が無い: direct=%q ancestor=%q", direct, ancestor)
	}
	if direct == ancestor {
		t.Fatalf("直接持つタグと祖先タグが同じ語で束ねられている: %q", direct)
	}
	if provenanceLabel(index.GovernsOwn) != "" {
		t.Fatalf("own は経由ではないので経由の種別を持たない")
	}
}

// 引き方は畳んだ集合を開けるコマンドでなければならない（変更5）。
// ⚠️ 経由タグを rules --tag で引き直す形は提示しない——経由タグの祖先まで
// 再展開して同じ本文を二度払うので、正本が費用の面で負ける形として禁じている。
func TestRulesAllCommandOpensTheFoldedSet(t *testing.T) {
	cases := []struct {
		tag, tx, vocab, facet string
		want                  string
	}{
		{tag: "req.a", want: "scholia rules --tag req.a --all"},
		{tx: "T-1", want: "scholia rules --tx T-1 --all"},
		{vocab: "act.a", want: "scholia rules --vocab act.a --all"},
		{facet: "requirement", want: "scholia rules --facet requirement --all"},
	}
	for _, c := range cases {
		if got := rulesAllCommand(c.tag, c.tx, c.vocab, c.facet); got != c.want {
			t.Errorf("rulesAllCommand = %q, want %q", got, c.want)
		}
	}
}

// 読み飛ばしてよいと解釈できる語を、畳んだ側の見出しに使わない（変更4）。
func TestInheritedHeadingDoesNotInviteSkipping(t *testing.T) {
	h := inheritedHeading(3)
	for _, bad := range []string{"参考", "関連", "補足"} {
		if strings.Contains(h, bad) {
			t.Errorf("見出しに読み飛ばしを誘う語（%q）がある: %q", bad, h)
		}
	}
	// 本文を読んでいないことが分かる語であること。
	if !strings.Contains(h, "本文") {
		t.Errorf("本文を渡していないことが見出しから読めない: %q", h)
	}
	if !strings.Contains(h, "3") {
		t.Errorf("件数が出ていない: %q", h)
	}
}
