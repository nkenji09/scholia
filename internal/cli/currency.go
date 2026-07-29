package cli

import (
	"fmt"
	"io"
	"sort"

	"github.com/nkenji09/scholia/internal/model"
)

// 取り下げ（supersede）を端末でどう扱うかの判断を、出力の書き方から切り離して
// ここ 1 か所に置く。
//
// **なぜ純関数にするか。** CLAUDE.md「配線ガードの書き方」1 の適用である。
// 「本文を出すか出さないか」「何件と数えるか」「行き先はどれか」はいずれも
// 入力（decision 群）に対する答えとして書けるので、画面・出力の体裁から切り離して
// 値で検査できる形にしておく。出力書式の照合だけを検査にすると、
// 同じ意味を別の綴りで書かれた瞬間に捕まらなくなる（CLAUDE.md 2）。
//
// 出力を書く側（rules / spec / search / decision list）は、この 3 つを呼ぶだけにしてある:
//
//	newCurrencyView   全 decision → 効力の索引
//	partition         決められた集合 → 本文を出す群 / 存在と行き先だけ出す群
//	writeWithdrawn    存在と行き先だけ出す群 → 1 件 1 行の通知
//
// ---
//
// 効力は 2 値しか出さない。記録側の 3 値（supersede / amend / exception・
// 01KXWPQDGMDB01V86KZ91M0BPQ）は一切変えず、失効扱いは supersede の被参照だけに
// 限る保守的な導出もそのまま。画面が既に 2 値へ寄せている
// （01KYHW54B8ZXH0NEPH2J7N1X39 条項1）ので、端末の機械可読出力も同じ語彙を使う。

// withdrawnMarkLabel は、本文まで出す面（--all）で取り下げ済みに添える印。
// rules（currencyLabel）と spec（EffectLabel）が同じ文言を使う——面ごとに
// 別の言い方をすると、読み手は同じ状態を別のものだと受け取る。
const withdrawnMarkLabel = " [失効: supersede 済]"

// Effect は出力に出せる効力。記録側の 3 値とは別物なので混ぜない。
type Effect string

const (
	// EffectInForce は効いている（＝他 decision に supersede されていない）。
	EffectInForce Effect = "in-force"
	// EffectReplaced は取り下げ済み（＝他 decision が mode=supersede で指した）。
	EffectReplaced Effect = "replaced"
)

// currencyView は decision 群から derive した効力の索引（保存しない）。
type currencyView struct {
	superseded   map[string]bool
	supersededBy map[string][]supersededByRef
}

func newCurrencyView(all []model.Decision) currencyView {
	return currencyView{
		superseded:   supersededIDs(all),
		supersededBy: supersededByIndex(all),
	}
}

// effectOf は 1 件の効力を返す。
func (v currencyView) effectOf(id string) Effect {
	if v.superseded[id] {
		return EffectReplaced
	}
	return EffectInForce
}

// replacedBy は「この decision を全文置換した側」の id を返す（＝行き先）。
// amend / exception で指しただけの後続は含めない——あれは旧を失効させないので、
// 行き先ではなく併せて読むべき付帯情報である。
func (v currencyView) replacedBy(id string) []string {
	var out []string
	for _, ref := range v.supersededBy[id] {
		if ref.Mode == model.ModeSupersede {
			out = append(out, ref.FromID)
		}
	}
	sort.Strings(out)
	return out
}

// supersededByOut は JSON へ出す逆リンク 1 件（derive・保存しない）。
// mode は 3 値のまま出す——effect が 2 値なのは「状態」であって、
// 「後続がどう繋がっているか」はそれとは独立した情報だから。
type supersededByOut struct {
	ID   string `json:"id"`
	Mode string `json:"mode"`
}

func (v currencyView) supersededByOut(id string) []supersededByOut {
	refs := v.supersededBy[id]
	if len(refs) == 0 {
		return nil
	}
	out := make([]supersededByOut, 0, len(refs))
	for _, ref := range refs {
		out = append(out, supersededByOut{ID: ref.FromID, Mode: ref.Mode})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// decisionOut は decision に derive した効力を添えた出力形（保存しない）。
//
// これが無いと、機械で読む側は全 decision を走査して supersedes[] の逆リンクを
// 自分で組まない限り効力を知れない——人が読む出力には `[失効: supersede 済]` が
// 付くのに JSON には何も無い、という食い違いが実際に出ていた。
type decisionOut struct {
	model.Decision
	Effect       Effect            `json:"effect"`
	SupersededBy []supersededByOut `json:"supersededBy,omitempty"`
}

func (v currencyView) decisionOut(d model.Decision) decisionOut {
	return decisionOut{
		Decision:     d,
		Effect:       v.effectOf(d.ID),
		SupersededBy: v.supersededByOut(d.ID),
	}
}

func (v currencyView) decisionOuts(decisions []model.Decision) []decisionOut {
	out := make([]decisionOut, 0, len(decisions))
	for _, d := range decisions {
		out = append(out, v.decisionOut(d))
	}
	return out
}

// withdrawnOut は取り下げられた 1 件の「存在と行き先」だけの出力形。
//
// **why / changed / ref を持たない。** 人が読む出力で本文を出さないのに JSON では
// 出す、という食い違いを作らないため。全文が要るなら --all（decisions 側へ合流し、
// そのとき effect=replaced が付く）か、id を名指しで開く経路を使う。
type withdrawnOut struct {
	ID           string               `json:"id"`
	Target       model.DecisionTarget `json:"target"`
	At           string               `json:"at"`
	Effect       Effect               `json:"effect"`
	ReplacedBy   []string             `json:"replacedBy"`
	SupersededBy []supersededByOut    `json:"supersededBy,omitempty"`
}

func (v currencyView) withdrawnOuts(decisions []model.Decision) []withdrawnOut {
	out := make([]withdrawnOut, 0, len(decisions))
	for _, d := range decisions {
		replaced := v.replacedBy(d.ID)
		if replaced == nil {
			replaced = []string{}
		}
		out = append(out, withdrawnOut{
			ID:           d.ID,
			Target:       d.Target,
			At:           d.At,
			Effect:       v.effectOf(d.ID),
			ReplacedBy:   replaced,
			SupersededBy: v.supersededByOut(d.ID),
		})
	}
	return out
}

// partition は「本文まで出す群」と「存在と行き先だけ出す群」に分ける。
//
// showAll のときは何も畳まない（＝全部が本文側）。既定では取り下げ済みを
// withdrawn 側へ回す——**消すのではない。** 取り下げがあったこと自体と、
// どこへ置き換わったかは既定でも読めなければならない。
//
// 入力の順序は保つ（呼び出し元が既に並べ替えている）。
func (v currencyView) partition(decisions []model.Decision, showAll bool) (bodies, withdrawn []model.Decision) {
	if showAll {
		return decisions, nil
	}
	for _, d := range decisions {
		if v.effectOf(d.ID) == EffectReplaced {
			withdrawn = append(withdrawn, d)
			continue
		}
		bodies = append(bodies, d)
	}
	return bodies, withdrawn
}

// decisionSplitter は currencyView を render.DecisionSplitter として渡すための
// 薄い接続。判断（何を畳むか）は currencyView 側にあり、ここは向きを合わせるだけ。
type decisionSplitter struct {
	view currencyView
	all  bool
}

func (s decisionSplitter) SplitDecisions(d []model.Decision) ([]model.Decision, []model.Decision) {
	return s.view.partition(d, s.all)
}

func (s decisionSplitter) WriteWithdrawn(w io.Writer, withdrawn []model.Decision, indent string) {
	writeWithdrawn(w, withdrawn, s.view, indent)
}

// EffectLabel は本文側に出す 1 件の印。--all で取り下げが本文側へ合流したとき、
// rules と同じ印を付ける（rules は currencyLabel が同じ文言を出す）。
func (s decisionSplitter) EffectLabel(d model.Decision) string {
	if s.view.effectOf(d.ID) == EffectReplaced {
		return withdrawnMarkLabel
	}
	return ""
}

// withdrawnHeading は取り下げ通知の見出し。件数は取り下げられた数。
func withdrawnHeading(n int) string {
	return fmt.Sprintf("取り下げられた規則 %d件（本文は出しません。全文は --all）:", n)
}

// writeWithdrawn は「存在と行き先」を 1 件 1 行で書く。
//
// **本文（why / changed）は出さない。** 取り下げた本文をここに混ぜると、
// 既定の出力を読んだ人がそれを守るべき規則として受け取る——それがこの変更の起点。
// 代わりに置き換えた記録の id を出す。置き換えた側は現行なので、
// 同じ出力の本文側に必ず載っており、そこに「何をどう置き換えたか」が書いてある。
//
// indent は呼び出し元の階層に合わせる（rules はトップレベル、spec は節の内側）。
func writeWithdrawn(w io.Writer, withdrawn []model.Decision, v currencyView, indent string) {
	if len(withdrawn) == 0 {
		return
	}
	fmt.Fprintf(w, "%s%s\n", indent, withdrawnHeading(len(withdrawn)))
	for _, d := range withdrawn {
		fmt.Fprintf(w, "%s  [%s] %s %s:%s → %s\n",
			indent, d.At, d.ID, d.Target.Type, d.Target.ID, replacedByLabel(v.replacedBy(d.ID)))
	}
}

// replacedByLabel は行き先の表示。結線が壊れていて置き換えた側が見つからない
// ときも黙らない——「取り下げ済みなのに行き先が無い」は読み手が知るべき状態。
func replacedByLabel(ids []string) string {
	if len(ids) == 0 {
		return "置き換えた記録: 不明（結線が見つかりません）"
	}
	label := "置き換えた記録 " + ids[0]
	for _, id := range ids[1:] {
		label += ", " + id
	}
	return label
}
