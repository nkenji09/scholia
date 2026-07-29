package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/model"
)

// 取り下げ（supersede）の扱いを守るガード。
//
// ⚠️ **このガードの射程**（CLAUDE.md「配線ガードの書き方」6）
//
// 落ちるもの:
//   - 判断（何を畳むか・行き先はどれか・効力は何か）が壊れること。純関数の
//     入力→出力の対で見るので、書き方を変えても意味が変われば落ちる。
//   - 端末の各面が、取り下げた本文を既定の出力に混ぜること。**実際にコマンドを
//     走らせて出力を値として検査する**ので、綴りを変えても本文が出れば落ちる。
//   - 存在と行き先が既定の出力から消えること。
//   - --all で本文が戻らなくなること。
//   - JSON から effect が消えること。
//   - 効いている規則が1件も同居しない対象で、存在と行き先が消えること
//     （TestCurrency_WithdrawnOnlyTargetKeepsExistenceAndDestination）。
//     ⚠️ 差し戻し1回目まで、この枝はガードの外にあった——「存在と行き先が消えること」を
//     落とすと名乗っていながら、実際には落ちなかった。
//   - --all で取り下げに印が付かなくなること／効いているものにも付くこと
//     （TestCurrency_AllMarksWithdrawnDistinctly）。これも差し戻し1回目まで素通りしていた。
//   - decision 本文を出しうる面が**新設されたのに、ここへ登録されないこと**
//     （TestCurrency_EverySurfaceIsClassified）。
//   - 分類の**誤分類**——表に載せた面は実際に走らせて両方向に確かめる
//     （TestCurrency_ClassificationMatchesReality）。
//   - 利用者に出る案内文が旧既定を教えたままになること
//     （TestCurrency_AdviceDoesNotTeachOldDefault）。
//   - --all と --current の同時指定を黙って受理すること
//     （TestCurrency_AllAndCurrentAreExclusive）。
//
// 落ちないもの（原理的に）:
//   - **人が読む出力の体裁**（見出し語・記号・並び）。ここは意味を見ておらず、
//     「本文が出ていないか」「id が出ているか」だけを見る。体裁の劣化は捕まらない。
//     ⚠️ 例外が1つある: --all の印だけは綴りに結び付けている（上記）。
//     「区別が付いていること」は綴りでしか観測できないため。
//   - **実測表（runnableSurfaces）に載っていない面。** そこは主張のまま——
//     件数はテストが log に出す。⚠️ 表そのものは手で維持しているので、
//     **分類を変えると同時に表からも外す**編集は捕まらない（レビュアの変異 N8）。
//     宣言と検査を同じ人が編集できる以上、これは原理的に残る。
//     せめて縮まないよう、表の下限を数える歯止めだけ置いてある。
//   - **同じ意味の記述が別の言い方で再導入されること**（G1 の型）。
//     捕まえているのは `rules --current` という綴り 1 点だけである
//     （TestCurrency_NoStaleSpellingInProductSources に理由を書いた）。
//   - **viewer / 静的書き出し**。あちらは Go のこの層を通らない（web/ 側の検査と
//     internal/viewer のテストが担う）。この単位では旧バイナリとの API 差分を
//     手で測ったが、それは自動では回らない。
//   - **配布スキル・手順書の記述**。テキストであって、機械で落とす手段が無い
//     （01KXS68HCNQ0H9QKNYFQ869J19 が言う「遡及機構が無い」領域そのもの）。

// withdrawFixture は取り下げの検査に使う id 一式。
type withdrawFixture struct {
	oldTag, newTag string // req.auth 宛（効いている規則が同居する）
	oldTx, newTx   string // T-login 宛（同上）
	oldVocab       string // act.user.submit-login 宛（同上）
	newVocab       string
	// req.onlywd 宛。**効いている規則が1件も同居しない対象。**
	// この枝（本文側が0件）は rules.go が明示的にコメント付きで分けているのに、
	// 同居する対象しか無い fixture では一度も通らなかった——差し戻し1回目の F2。
	oldOnly, newOnly string
}

// setupWithdrawFixture は取り下げられた decision を tag / transition / vocab と、
// **効いている規則が同居しない対象**にそれぞれ置いた store を作る。
func setupWithdrawFixture(t *testing.T, dir string) withdrawFixture {
	t.Helper()
	setupAuthFixture(t, dir)
	var f withdrawFixture

	f.oldTag = decisionIDFromJSON(t, mustRun(t, dir, "decide", "--on", "tag:req.auth",
		"--why", "旧タグ判断ホンブンA", "--json"))
	f.newTag = decisionIDFromJSON(t, mustRun(t, dir, "decide", "--on", "tag:req.auth",
		"--why", "新タグ判断ホンブンB", "--supersedes", f.oldTag+":supersede", "--json"))

	f.oldTx = decisionIDFromJSON(t, mustRun(t, dir, "decide", "--on", "transition:T-login",
		"--why", "旧遷移判断ホンブンC", "--json"))
	f.newTx = decisionIDFromJSON(t, mustRun(t, dir, "decide", "--on", "transition:T-login",
		"--why", "新遷移判断ホンブンD", "--supersedes", f.oldTx+":supersede", "--json"))

	f.oldVocab = decisionIDFromJSON(t, mustRun(t, dir, "decide", "--on", "vocab:act.user.submit-login",
		"--why", "旧語彙判断ホンブンE", "--json"))
	f.newVocab = decisionIDFromJSON(t, mustRun(t, dir, "decide", "--on", "vocab:act.user.submit-login",
		"--why", "新語彙判断ホンブンF", "--supersedes", f.oldVocab+":supersede", "--json"))

	// 取り下げしか無い対象。**祖先を持たせない**（祖先の decision が本文側へ入ると
	// この枝を通らなくなる）。置き換えた側は別の対象（req.auth）に置く——supersede は
	// 置き換えた側が同じ対象にいることを要求しないので、これは作り物ではなく実際に
	// 起こる形である。
	mustRun(t, dir, "tag", "create", "req.onlywd", "--name", "取り下げのみ", "--kind", "requirement")
	f.oldOnly = decisionIDFromJSON(t, mustRun(t, dir, "decide", "--on", "tag:req.onlywd",
		"--why", "旧単独判断ホンブンG", "--json"))
	f.newOnly = decisionIDFromJSON(t, mustRun(t, dir, "decide", "--on", "tag:req.auth",
		"--why", "別対象へ移した判断ホンブンH", "--supersedes", f.oldOnly+":supersede", "--json"))
	return f
}

// --- 1. 判断そのもの（純関数・入力と出力の対） ---------------------------------

func TestCurrency_PartitionAndEffect(t *testing.T) {
	d := func(id string, links ...model.SupersedeLink) model.Decision {
		return model.Decision{ID: id, Why: "本文:" + id, Supersedes: links}
	}
	sup := func(id string) model.SupersedeLink { return model.SupersedeLink{ID: id, Mode: model.ModeSupersede} }
	amd := func(id string) model.SupersedeLink { return model.SupersedeLink{ID: id, Mode: model.ModeAmend} }

	all := []model.Decision{
		d("A"),           // 効いている
		d("B", sup("A")), // A を全文置換 → A が失効
		d("C"),           // 効いている
		d("D", amd("C")), // C を部分改訂 → C は効いたまま
		d("E", sup("Z")), // 実在しない相手（結線が壊れた場合）
	}
	v := newCurrencyView(all)

	for _, tc := range []struct {
		id   string
		want Effect
	}{
		{"A", EffectReplaced},
		{"B", EffectInForce},
		{"C", EffectInForce}, // amend は失効させない（3値の保守的導出は不変）
		{"D", EffectInForce},
		{"E", EffectInForce},
		{"未知", EffectInForce},
	} {
		if got := v.effectOf(tc.id); got != tc.want {
			t.Errorf("effectOf(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}

	// 行き先は「全文置換した側」だけ。部分改訂した側は行き先ではない。
	if got := v.replacedBy("A"); len(got) != 1 || got[0] != "B" {
		t.Errorf("replacedBy(A) = %v, want [B]", got)
	}
	if got := v.replacedBy("C"); len(got) != 0 {
		t.Errorf("replacedBy(C) = %v, want []（amend は行き先ではない）", got)
	}

	// 既定は取り下げを本文側から外す。消すのではなく withdrawn へ回す。
	bodies, withdrawn := v.partition(all, false)
	if ids := decisionIDs(bodies); strings.Join(ids, ",") != "B,C,D,E" {
		t.Errorf("既定の本文側 = %v, want [B C D E]", ids)
	}
	if ids := decisionIDs(withdrawn); strings.Join(ids, ",") != "A" {
		t.Errorf("既定の取り下げ側 = %v, want [A]", ids)
	}

	// --all は何も畳まない。順序も保つ。
	bodiesAll, withdrawnAll := v.partition(all, true)
	if ids := decisionIDs(bodiesAll); strings.Join(ids, ",") != "A,B,C,D,E" {
		t.Errorf("--all の本文側 = %v, want [A B C D E]", ids)
	}
	if len(withdrawnAll) != 0 {
		t.Errorf("--all では取り下げ側は空のはず: %v", decisionIDs(withdrawnAll))
	}
}

func decisionIDs(ds []model.Decision) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.ID)
	}
	return out
}

// --- 2. 面ごとの実出力 ---------------------------------------------------------

func TestCurrency_EverySurfaceHidesWithdrawnBodyByDefault(t *testing.T) {
	dir := t.TempDir()
	f := setupWithdrawFixture(t, dir)

	// 面ごとに「取り下げられた本文」「取り下げられた id」「行き先の id」が
	// 何であるかは違う。tag 宛の面と transition 宛の面を両方通す。
	type expect struct {
		body      string // 既定の出力に出てはいけない本文
		id        string // 既定の出力に出るべき id（存在）
		replacer  string // 既定の出力に出るべき id（行き先）
		liveBody  string // 既定の出力に出るべき本文（効いている側）
		surfaceID string
	}
	tagExp := expect{body: "旧タグ判断ホンブンA", id: f.oldTag, replacer: f.newTag, liveBody: "新タグ判断ホンブンB"}
	txExp := expect{body: "旧遷移判断ホンブンC", id: f.oldTx, replacer: f.newTx, liveBody: "新遷移判断ホンブンD"}

	cases := []struct {
		name string
		args []string
		exp  expect
	}{
		{"rules --tag（text）", []string{"rules", "--tag", "req.auth"}, tagExp},
		{"rules --tx（text）", []string{"rules", "--tx", "T-login"}, txExp},
		{"rules --facet（text）", []string{"rules", "--facet", "requirement"}, tagExp},
		{"rules --sort target（text）", []string{"rules", "--tag", "req.auth", "--sort", "target"}, tagExp},
		{"rules --json", []string{"rules", "--tag", "req.auth", "--json"}, tagExp},
		{"spec（text）", []string{"spec", "req.auth"}, tagExp},
		{"spec の遷移側（text）", []string{"spec", "req.auth"}, txExp},
		{"spec --json", []string{"spec", "req.auth", "--json"}, tagExp},
		{"spec --json の遷移側", []string{"spec", "req.auth", "--json"}, txExp},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := mustRun(t, dir, tc.args...)

			if strings.Contains(out, tc.exp.body) {
				t.Errorf("既定の出力に取り下げた本文 %q が出ている:\n%s", tc.exp.body, out)
			}
			if !strings.Contains(out, tc.exp.liveBody) {
				t.Errorf("既定の出力に効いている本文 %q が出ていない:\n%s", tc.exp.liveBody, out)
			}
			// 存在: 取り下げられた記録の id が読める。
			if !strings.Contains(out, tc.exp.id) {
				t.Errorf("既定の出力に取り下げられた記録の id %s が出ていない（存在が消えている）:\n%s", tc.exp.id, out)
			}
			// 行き先: そこから置き換えた記録へ辿れる。
			if !strings.Contains(out, tc.exp.replacer) {
				t.Errorf("既定の出力に行き先 %s が出ていない:\n%s", tc.exp.replacer, out)
			}

			// --all で本文が戻る。
			allOut := mustRun(t, dir, append(append([]string{}, tc.args...), "--all")...)
			if !strings.Contains(allOut, tc.exp.body) {
				t.Errorf("--all で取り下げた本文 %q が戻っていない:\n%s", tc.exp.body, allOut)
			}
		})
	}
}

// 効いている規則が1件も同居しない対象でも、存在と行き先は消えない。
//
// ⚠️ **この枝（本文側が0件）は、上のテストでは一度も通らない。**
// 上の fixture は取り下げ1件と現行1件を必ず同じ対象に置くので、常に本文側が1以上ある。
// ところが実装（rules.go）はこの枝を明示的に分けて書いており、壊れると
// 「そこに何も無い」と「そこにあったものが取り下げられた」が読み手から区別できなくなる
// ——提案本文の核心そのものが失われる。差し戻し1回目の F2 で、この抜けが実測された。
func TestCurrency_WithdrawnOnlyTargetKeepsExistenceAndDestination(t *testing.T) {
	dir := t.TempDir()
	f := setupWithdrawFixture(t, dir)

	// 「本当に何も無い対象」との区別が付くことまで見る。片方だけ見ても、
	// 両方が同じ文言を返す変異は捕まらない。
	mustRun(t, dir, "tag", "create", "req.empty", "--name", "何も無い", "--kind", "requirement")

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"rules --tag", []string{"rules", "--tag", "req.onlywd"}},
		{"spec", []string{"spec", "req.onlywd"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := mustRun(t, dir, tc.args...)

			if strings.Contains(out, "旧単独判断ホンブンG") {
				t.Errorf("既定の出力に取り下げた本文が出ている:\n%s", out)
			}
			if !strings.Contains(out, f.oldOnly) {
				t.Errorf("本文側が0件でも、取り下げられた記録の id %s は出るべき:\n%s", f.oldOnly, out)
			}
			if !strings.Contains(out, f.newOnly) {
				t.Errorf("本文側が0件でも、行き先 %s は出るべき:\n%s", f.newOnly, out)
			}

			// 「本当に何も無い対象」と同じ出力になってはいけない。
			emptyArgs := append([]string{}, tc.args...)
			emptyArgs[len(emptyArgs)-1] = "req.empty"
			empty := mustRun(t, dir, emptyArgs...)
			if strings.Contains(empty, f.oldOnly) {
				t.Errorf("何も無い対象に取り下げの id が出ている:\n%s", empty)
			}
			if out == empty {
				t.Errorf("「取り下げられた」と「そこに何も無い」の出力が同一——読み手が区別できない:\n%s", out)
			}
		})
	}

	// --all では本文が戻る（本文側が0件の枝でも --all の経路が生きていること）。
	all := mustRun(t, dir, "rules", "--tag", "req.onlywd", "--all")
	if !strings.Contains(all, "旧単独判断ホンブンG") {
		t.Errorf("--all で取り下げた本文が戻っていない:\n%s", all)
	}

	// JSON でも同じ答え。
	var out struct {
		Decisions []map[string]any `json:"decisions"`
		Withdrawn []map[string]any `json:"withdrawn"`
	}
	mustUnmarshal(t, mustRun(t, dir, "rules", "--tag", "req.onlywd", "--json"), &out)
	if len(out.Decisions) != 0 {
		t.Errorf("本文側は0件のはず: %v", out.Decisions)
	}
	assertWithdrawnShape(t, out.Withdrawn, f.oldOnly, f.newOnly)
}

// --all で本文側へ合流した取り下げには、効いているものと区別できる印が付く。
//
// 受け入れ条件2 が「--all で従来どおり全文が出る（**印付き**）」と求めている性質。
// 差し戻し1回目のレビューで、印を全廃する変異が全緑のまま素通りした
// ——**誰もこの性質を守っていなかった**。
//
// ⚠️ **このケースだけは出力の綴りに結び付いている。** 印の文言を変えると落ちる。
// 他のケースは意味（本文が出るか・id が出るか）を見ているので綴りに依存しないが、
// 「区別が付いていること」は綴りでしか観測できない。ガードの射程の名乗り（冒頭）の
// 「体裁は落ちない」に対する意図的な例外である。
func TestCurrency_AllMarksWithdrawnDistinctly(t *testing.T) {
	dir := t.TempDir()
	setupWithdrawFixture(t, dir)

	// 期待する印の数は決め打ちにしない——**同じコマンドの既定 --json が数えた
	// 取り下げ件数**と突き合わせる。決め打ちだと fixture を足すたびに直す羽目になり、
	// 「印が多すぎる／少なすぎる」のどちらも見落としやすい。
	const mark = withdrawnMarkLabel

	t.Run("rules --all", func(t *testing.T) {
		var j struct {
			Withdrawn []map[string]any `json:"withdrawn"`
		}
		mustUnmarshal(t, mustRun(t, dir, "rules", "--tag", "req.auth", "--json"), &j)
		out := mustRun(t, dir, "rules", "--tag", "req.auth", "--all")
		assertMarkCount(t, out, mark, len(j.Withdrawn))
	})

	t.Run("spec --all", func(t *testing.T) {
		var j struct {
			Withdrawn []map[string]any `json:"withdrawn"`
			Entries   []struct {
				Withdrawn []map[string]any `json:"withdrawn"`
			} `json:"entries"`
		}
		mustUnmarshal(t, mustRun(t, dir, "spec", "req.auth", "--json"), &j)
		want := len(j.Withdrawn)
		for _, e := range j.Entries {
			want += len(e.Withdrawn)
		}
		out := mustRun(t, dir, "spec", "req.auth", "--all")
		assertMarkCount(t, out, mark, want)
	})
}

// assertMarkCount は印の数が取り下げ件数とちょうど一致することを見る。
// 0 になれば「印を全廃した」、多すぎれば「効いているものにも付けた」で落ちる。
func assertMarkCount(t *testing.T, out, mark string, want int) {
	t.Helper()
	if want == 0 {
		t.Fatalf("fixture に取り下げが無い（このケースは何も検査していない）")
	}
	if got := strings.Count(out, mark); got != want {
		t.Errorf("--all の印の数 = %d, want %d（取り下げの件数と一致すべき）:\n%s", got, want, out)
	}
}

// search は畳まない面。隠さない代わりに印と行き先を必ず出す。
func TestCurrency_SearchMarksWithdrawnWithoutHiding(t *testing.T) {
	dir := t.TempDir()
	f := setupWithdrawFixture(t, dir)

	out := mustRun(t, dir, "search", "ホンブンA")
	if !strings.Contains(out, f.oldTag) {
		t.Fatalf("search は取り下げられた記録を隠さないはず（id が出ていない）:\n%s", out)
	}
	if !strings.Contains(out, "取り下げ済み") {
		t.Fatalf("search は取り下げ済みの印を付けるはず:\n%s", out)
	}
	if !strings.Contains(out, f.newTag) {
		t.Fatalf("search の印から行き先 %s へ辿れるはず:\n%s", f.newTag, out)
	}

	// 効いている記録には印を付けない（印は例外側に付ける）。
	live := mustRun(t, dir, "search", "ホンブンB")
	if strings.Contains(live, "取り下げ済み") {
		t.Fatalf("効いている記録に取り下げ済みの印が付いている:\n%s", live)
	}

	// --json も同じ答えを返す。
	var parsed struct {
		Matches []struct {
			ID         string   `json:"id"`
			Effect     string   `json:"effect"`
			ReplacedBy []string `json:"replacedBy"`
		} `json:"matches"`
	}
	raw := mustRun(t, dir, "search", "ホンブンA", "--json")
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("search --json が読めない: %v\n%s", err, raw)
	}
	found := false
	for _, m := range parsed.Matches {
		if m.ID != f.oldTag {
			continue
		}
		found = true
		if m.Effect != string(EffectReplaced) {
			t.Errorf("search --json の effect = %q, want %q", m.Effect, EffectReplaced)
		}
		if len(m.ReplacedBy) != 1 || m.ReplacedBy[0] != f.newTag {
			t.Errorf("search --json の replacedBy = %v, want [%s]", m.ReplacedBy, f.newTag)
		}
	}
	if !found {
		t.Fatalf("search --json に取り下げられた記録が無い:\n%s", raw)
	}
}

// JSON を出す面はすべて effect を持つ。機械で読む側が全件走査して
// 逆リンクを組まなくても効力が分かること、がこの面の要件。
func TestCurrency_EveryJSONSurfaceCarriesEffect(t *testing.T) {
	dir := t.TempDir()
	f := setupWithdrawFixture(t, dir)

	t.Run("rules --json", func(t *testing.T) {
		var out struct {
			Decisions []map[string]any `json:"decisions"`
			Withdrawn []map[string]any `json:"withdrawn"`
		}
		mustUnmarshal(t, mustRun(t, dir, "rules", "--tag", "req.auth", "--json"), &out)
		assertAllHaveEffect(t, "decisions", out.Decisions)
		assertWithdrawnShape(t, out.Withdrawn, f.oldTag, f.newTag)
		for _, d := range out.Decisions {
			if d["id"] == f.oldTag {
				t.Errorf("既定の decisions に取り下げられた記録が混ざっている")
			}
		}
	})

	t.Run("rules --json --all", func(t *testing.T) {
		var out struct {
			Decisions []map[string]any `json:"decisions"`
			Withdrawn []map[string]any `json:"withdrawn"`
		}
		mustUnmarshal(t, mustRun(t, dir, "rules", "--tag", "req.auth", "--json", "--all"), &out)
		assertAllHaveEffect(t, "decisions", out.Decisions)
		if len(out.Withdrawn) != 0 {
			t.Errorf("--all では withdrawn は空のはず: %v", out.Withdrawn)
		}
		seen := false
		for _, d := range out.Decisions {
			if d["id"] == f.oldTag {
				seen = true
				if d["effect"] != string(EffectReplaced) {
					t.Errorf("--all の取り下げ記録の effect = %v, want %q", d["effect"], EffectReplaced)
				}
				if d["why"] == nil {
					t.Errorf("--all では本文が戻るはず")
				}
			}
		}
		if !seen {
			t.Errorf("--all に取り下げられた記録が出ていない")
		}
	})

	t.Run("decision list --json", func(t *testing.T) {
		var out struct {
			Decisions []map[string]any `json:"decisions"`
		}
		mustUnmarshal(t, mustRun(t, dir, "decision", "list", "--json"), &out)
		assertAllHaveEffect(t, "decisions", out.Decisions)
		for _, d := range out.Decisions {
			if d["id"] != f.oldTag {
				continue
			}
			if d["effect"] != string(EffectReplaced) {
				t.Errorf("decision list --json の effect = %v, want %q", d["effect"], EffectReplaced)
			}
			if d["supersededBy"] == nil {
				t.Errorf("decision list --json に行き先（supersededBy）が無い")
			}
		}
	})

	t.Run("spec --json", func(t *testing.T) {
		var out struct {
			TagDecisions []map[string]any `json:"tagDecisions"`
			Withdrawn    []map[string]any `json:"withdrawn"`
			Entries      []struct {
				Decisions []map[string]any `json:"decisions"`
				Withdrawn []map[string]any `json:"withdrawn"`
			} `json:"entries"`
		}
		mustUnmarshal(t, mustRun(t, dir, "spec", "req.auth", "--json"), &out)
		assertAllHaveEffect(t, "tagDecisions", out.TagDecisions)
		assertWithdrawnShape(t, out.Withdrawn, f.oldTag, f.newTag)
		for i, e := range out.Entries {
			assertAllHaveEffect(t, "entries[].decisions", e.Decisions)
			for _, w := range e.Withdrawn {
				if w["why"] != nil {
					t.Errorf("entries[%d].withdrawn に本文が乗っている", i)
				}
			}
		}
	})
}

func mustUnmarshal(t *testing.T, raw string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(raw), v); err != nil {
		t.Fatalf("JSON が読めない: %v\n%s", err, raw)
	}
}

func assertAllHaveEffect(t *testing.T, where string, items []map[string]any) {
	t.Helper()
	if len(items) == 0 {
		t.Fatalf("%s が空（fixture が壊れている）", where)
	}
	for _, it := range items {
		e, ok := it["effect"].(string)
		if !ok {
			t.Errorf("%s の %v に effect が無い", where, it["id"])
			continue
		}
		if e != string(EffectInForce) && e != string(EffectReplaced) {
			t.Errorf("%s の effect が 2値の外: %q", where, e)
		}
	}
}

// assertWithdrawnShape は「存在と行き先だけ・本文は無し」を検査する。
func assertWithdrawnShape(t *testing.T, withdrawn []map[string]any, oldID, newID string) {
	t.Helper()
	if len(withdrawn) == 0 {
		t.Fatalf("withdrawn が空（取り下げが見えていない）")
	}
	for _, w := range withdrawn {
		if w["why"] != nil || w["changed"] != nil {
			t.Errorf("withdrawn に本文が乗っている: %v", w)
		}
		if w["effect"] != string(EffectReplaced) {
			t.Errorf("withdrawn の effect = %v, want %q", w["effect"], EffectReplaced)
		}
	}
	found := false
	for _, w := range withdrawn {
		if w["id"] != oldID {
			continue
		}
		found = true
		rb, _ := w["replacedBy"].([]any)
		if len(rb) != 1 || rb[0] != newID {
			t.Errorf("withdrawn の replacedBy = %v, want [%s]", w["replacedBy"], newID)
		}
	}
	if !found {
		t.Errorf("withdrawn に %s が無い", oldID)
	}
}

// --- 3. 面の取りこぼし検出 -----------------------------------------------------

// decision 本文を出しうる面は、ここに分類されていなければならない。
//
// **なぜ一覧で持つか。** CLAUDE.md「配線ガードの書き方」5 —— ガードを主題にした
// 作業でさえ、新設した面のガード漏れが最後まで残った、という実測がある。
// 「本文を出す面をひとつ足したがガードを置き忘れた」を、面の名前を数えることで
// 落とす。個別の綴りではなくコマンドツリー全体を歩くので、名前を変えて足しても
// 落ちる。
//
// ⚠️ 落ちないもの: 既存コマンドに**フラグ**を足して本文を出す経路を増やす変異。
// コマンド名の集合は変わらないので、ここは通る。フラグ単位まで数える形にはしない
// ——出力の中身を見る上の 2 群がその役目を負う。
var (
	// 取り下げの扱いを上のテストで実際に検査している面。
	surfacesGuarded = []string{
		"scholia rules",
		"scholia spec",
		"scholia search",
		// text の既定は変えない（取り下げた理由を読む経路を1つ残す必要がある）。
		// 変えたのは JSON に効力を載せたことだけで、そこは検査している。
		"scholia decision list",
	}
	// 端末に decision 本文を出すが、既定を変えないと決めた面。**各行に理由を書く。**
	surfacesIntentionallyFull = []string{
		// 1件を名指しで開く経路。「現行性:」の行で効力を明示するので畳まない。
		"scholia decision show",
		"scholia show decision",
		// ⚠️ 取り下げた decision の本文を**印も注記も無しに**出す（実測・show_vocab.go）。
		// この単位では振る舞いを変えない——提案本文の射程は rules / spec / search /
		// decision list の4面で、ここは入っていない。**「検討済みだから出している」の
		// ではなく、まだ決めていない。** rules --vocab は畳むのに show vocab は無印、
		// という面どうしの食い違いが残っている（result の「次に埋めるべき穴」）。
		"scholia show vocab",
	}
	// 画面・静的書き出しを起こす面。**このガードの射程外。**
	// どちらも decision 本文をペイロードに含むが、畳むのは画面側の要件
	// （01KYHW54B8ZXH0NEPH2J7N1X39 条項4）で、Go のこの層は通らない。
	// 実測: export --html が書く HTML には取り下げた本文が含まれる（画面が畳む）。
	surfacesViewerScope = []string{
		"scholia view",
		"scholia export",
	}
)

func TestCurrency_EverySurfaceIsClassified(t *testing.T) {
	// 走査時点の「実行できるコマンド」すべて。葉だけを見ると、子を持ちつつ
	// 自分でも走るコマンド（scholia lint など）が漏れる——そこも1つの面である。
	var leaves []string
	var walk func(c *cobra.Command, path string)
	walk = func(c *cobra.Command, path string) {
		name := strings.TrimSpace(path + " " + c.Name())
		if c.Runnable() {
			leaves = append(leaves, name)
		}
		for _, k := range c.Commands() {
			if k.Name() == "help" || k.Name() == "completion" || k.Hidden {
				continue
			}
			walk(k, name)
		}
	}
	walk(newRootCmd(), "")

	// 既知の分類（4群）。同じ名前が2群に載っていたらそれも間違い。
	known := map[string]string{}
	add := func(bucket string, names []string) {
		for _, s := range names {
			if prev, dup := known[s]; dup {
				t.Errorf("%q が %s と %s の両方に載っている", s, prev, bucket)
			}
			known[s] = bucket
		}
	}
	add("surfacesGuarded", surfacesGuarded)
	add("surfacesIntentionallyFull", surfacesIntentionallyFull)
	add("surfacesViewerScope", surfacesViewerScope)
	add("surfacesNoDecisionBody", surfacesNoDecisionBody)

	var unclassified []string
	for _, l := range leaves {
		if _, ok := known[l]; !ok {
			unclassified = append(unclassified, l)
		}
	}
	sort.Strings(unclassified)
	if len(unclassified) > 0 {
		t.Fatalf(`未分類の面がある: %v

端末に decision 本文を出す面なら surfacesGuarded に足し、取り下げの扱いを上のテストで検査すること。
既定を変えないと決めた面なら surfacesIntentionallyFull に**理由つきで**足すこと。
画面・静的書き出しを起こす面なら surfacesViewerScope に足すこと。
本文を出さない面なら surfacesNoDecisionBody に足し、走らせられるなら
noDecisionBodyRunnable にも足して**実測に変える**こと。
（CLAUDE.md「配線ガードの書き方」5: 新しく作った面には、ガードを置き忘れる）`, unclassified)
	}

	// 逆向き: 分類に載っているのに実在しない名前が残っていたら、それも間違い。
	present := map[string]bool{}
	for _, l := range leaves {
		present[l] = true
	}
	for s := range known {
		if !present[s] {
			t.Errorf("分類に載っている %q は実在しない（改名・削除したなら分類も直す）", s)
		}
	}
}

// 端末の出力に decision の本文（why / changed）を出さない面。
//
// ⚠️ **基準の言い方に注意。** 「decision に触らない」ではないし、
// 「`why` という文字列が出力に一切現れない」でもない——`scholia decide --json` は
// **いま渡した本文をそのまま反響する**が、それは既存の規則の開示ではない。
// ここで見るのは「**store にある他の decision の本文を読み手へ渡すか**」である。
var surfacesNoDecisionBody = []string{
	"scholia init",
	"scholia lint",
	"scholia lint baseline update",
	"scholia config infer-id-policy",
	"scholia tag edit",
	"scholia tx merge",
	"scholia tx tag",
	"scholia vocab owner-migrate",
	"scholia list",
	"scholia flow",
	"scholia gaps",
	"scholia diff",
	"scholia version",
	"scholia update",
	"scholia retrofit",
	"scholia decide",
	"scholia decision link",
	"scholia decision add-commit",
	"scholia review add",
	// 提案（まだ decision ではない）の本文を出す面。基準は「store にある他の
	// **decision** の本文を渡すか」なので、ここは「出さない面」に入る。
	"scholia review list",
	"scholia review adopt",
	"scholia review reject",
	"scholia review rm",
	"scholia show tag",
	"scholia show tx",
	"scholia tag create",
	"scholia tag rm",
	"scholia tag rename",
	"scholia tag list",
	"scholia tx add",
	"scholia tx rm",
	"scholia tx edit",
	"scholia tx rename",
	"scholia vocab add",
	"scholia vocab rm",
	"scholia vocab edit",
	"scholia vocab rename",
	"scholia vocab tag",
	"scholia config get",
	"scholia config set",
	"scholia kind get",
	"scholia kind set",
	"scholia kind list",
	"scholia refs scan",
	"scholia refs rewrite",
	"scholia skills install",
}

// runnableSurfaces は、分類を**主張ではなく実測**にするための引数表。
//
// **なぜ要るか。** 分類の一覧は「未分類の名前」しか落とせない——**誤分類は
// 原理的に落ちない**。差し戻し1回目のレビューで `show vocab` が「本文を出さない面」に
// 入っていながら本文を出すことが実測で見つかった（F3）。是正のときに同じ型で
// `export` も誤分類されていたことが分かった。名前を数えるだけの検査では、どちらも
// 永久に緑のままだった。
//
// **両方向を見る。** 「出さない面」が出していたら落ちるだけでは足りない
// ——出す面を「出さない面」へ移す誤分類は、その面を実測表から外せば通ってしまう
// （実際に export でそれが起きた）。だから**表に載っている面はすべて**、
// その分類が言うとおりの答えを返すことを確かめる。
//
// %OLD% は取り下げられた decision の id、%OUT% は一時ディレクトリに置き換える。
//
// ⚠️ **表に載っていない面は主張のまま。** 書き込む面（`decide` / `tag create` 等）・
// git を要する面（`diff`）・ネットワークを要する面（`update`）・
// 常駐する面（`view`）は走らせていない。件数はテストが log に出す。
var runnableSurfaces = map[string][]string{
	// --- decision 本文を出す面 ---
	"scholia rules":         {"rules", "--tag", "req.auth"},
	"scholia search":        {"search", "ホンブンA"},
	"scholia spec":          {"spec", "req.auth"},
	"scholia decision list": {"decision", "list"},
	"scholia decision show": {"decision", "show", "%OLD%"},
	"scholia show decision": {"show", "decision", "%OLD%"},
	"scholia show vocab":    {"show", "vocab", "act.user.submit-login"},
	"scholia export":        {"export", "--html", "%OUT%"},
	// --- decision 本文を出さない面 ---
	"scholia lint":        {"lint"},
	"scholia retrofit":    {"retrofit"},
	"scholia list":        {"list"},
	"scholia flow":        {"flow", "act.user.submit-login"},
	"scholia gaps":        {"gaps", "act.user.submit-login"},
	"scholia show tag":    {"show", "tag", "req.auth"},
	"scholia show tx":     {"show", "tx", "T-login"},
	"scholia tag list":    {"tag", "list"},
	"scholia kind list":   {"kind", "list"},
	"scholia version":     {"version"},
	"scholia refs scan":   {"refs", "scan"},
	"scholia review list": {"review", "list"},
}

// TestCurrency_ClassificationMatchesReality は、走らせられる面すべてについて
// 「分類が言うとおりか」を実測する。
//
//	surfacesNoDecisionBody  … decision 本文を1つも出さないこと
//	それ以外の3群            … decision 本文を出すこと（出さないならその分類が誤り）
func TestCurrency_ClassificationMatchesReality(t *testing.T) {
	dir := t.TempDir()
	f := setupWithdrawFixture(t, dir)

	// fixture が置いた decision 本文すべて。
	bodies := []string{
		"旧タグ判断ホンブンA", "新タグ判断ホンブンB",
		"旧遷移判断ホンブンC", "新遷移判断ホンブンD",
		"旧語彙判断ホンブンE", "新語彙判断ホンブンF",
		"旧単独判断ホンブンG", "別対象へ移した判断ホンブンH",
	}

	bucket := map[string]string{}
	for _, n := range surfacesGuarded {
		bucket[n] = "surfacesGuarded"
	}
	for _, n := range surfacesIntentionallyFull {
		bucket[n] = "surfacesIntentionallyFull"
	}
	for _, n := range surfacesViewerScope {
		bucket[n] = "surfacesViewerScope"
	}
	for _, n := range surfacesNoDecisionBody {
		bucket[n] = "surfacesNoDecisionBody"
	}

	// ⚠️ 表が縮まないための歯止め。分類を変えると同時に表からも外す編集は
	// 原理的に捕まらない（射程の名乗り参照）が、**外したこと自体**はここで落ちる。
	// 面を減らす正当な理由があるなら、この数も一緒に下げること。
	const minRunnable = 20
	if len(runnableSurfaces) < minRunnable {
		t.Errorf("実測表が %d 面まで縮んでいる（下限 %d）——分類の裏付けが減っている", len(runnableSurfaces), minRunnable)
	}

	total := len(surfacesGuarded) + len(surfacesIntentionallyFull) +
		len(surfacesViewerScope) + len(surfacesNoDecisionBody)
	t.Logf("実測した面 %d / 分類 %d（残り %d は主張のまま——書き込む面・git/ネットワーク/常駐を要する面）",
		len(runnableSurfaces), total, total-len(runnableSurfaces))

	for name, rawArgs := range runnableSurfaces {
		t.Run(name, func(t *testing.T) {
			b, ok := bucket[name]
			if !ok {
				t.Fatalf("%q が4群のどこにも無い", name)
			}

			outDir := t.TempDir()
			args := make([]string, 0, len(rawArgs))
			for _, a := range rawArgs {
				switch a {
				case "%OLD%":
					a = f.oldTag
				case "%OUT%":
					a = filepath.Join(outDir, "html")
				}
				args = append(args, a)
			}

			out, err := run(t, dir, args...)
			if err != nil {
				t.Fatalf("%v が走らない: %v\n%s", args, err, out)
			}
			// 書き出す面は、書いたものまで見る（標準出力だけ見ると
			// 「本文を出していない」と誤読する）。
			if written := readIfExists(filepath.Join(outDir, "html", "index.html")); written != "" {
				out += written
			}

			var found []string
			for _, body := range bodies {
				if strings.Contains(out, body) {
					found = append(found, body)
				}
			}

			if b == "surfacesNoDecisionBody" {
				if len(found) > 0 {
					t.Errorf("%q は decision 本文 %v を出している——%s への分類が事実に反する", name, found, b)
				}
				return
			}
			if len(found) == 0 {
				t.Errorf("%q は decision 本文を1つも出していない——%s への分類が事実に反する（本文を出さないなら surfacesNoDecisionBody）:\n%s",
					name, b, out)
			}
		})
	}
}

func readIfExists(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

// --- 4. 利用者に出る案内文が旧既定を教えていないこと -------------------------

// 既定を変えたとき、**利用者に出る案内文が旧既定を教えたまま**になっていた
// （差し戻し2回目の G1）。しかも同じ意味の一文が画面側のコード内コメントにもあり、
// **利用者に出ない方だけを直して、出る方を残した**。
//
// ここは実際に adopt を踏んで、出てきた文言そのものを検査する。
func TestCurrency_AdviceDoesNotTeachOldDefault(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)

	// 既存 decision がある対象へ提案を出して adopt すると、結線を促す advisory が出る。
	mustRun(t, dir, "decide", "--on", "tag:req.auth", "--why", "先にある判断")
	var rv struct {
		ID string `json:"id"`
	}
	mustUnmarshal(t, mustRun(t, dir, "review", "add", "--on", "tag:req.auth",
		"--body", "あとから来た提案", "--source", "ai", "--json"), &rv)
	out := mustRun(t, dir, "review", "adopt", rv.ID)

	if !strings.Contains(out, "supersede-unlinked") {
		t.Fatalf("結線を促す advisory が出ていない（この検査が何も見ていない）:\n%s", out)
	}
	if strings.Contains(out, "--current") {
		t.Errorf("利用者に出る案内文が、いまも --current を畳む手段として教えている:\n%s", out)
	}
	if !strings.Contains(out, "scholia rules") {
		t.Errorf("案内文が、結線しないとどの面が旧を現行として出すのかを述べていない:\n%s", out)
	}
}

// ⚠️ **これはガードではなく、洗い残しの目印である。**
//
// 「同じ意味の記述が2箇所にあり、片側だけ直す」型は、綴りの照合では原理的に閉じない
// ——同じことを別の言い方で書けば通る（CLAUDE.md「配線ガードの書き方」2）。
// だからここで捕まえるのは **`rules --current` という綴りが product のソースへ
// 戻ってくること 1 点だけ**である。言い換えた再導入は捕まらない。
//
// それでも置くのは、G1 が**まさにこの綴り**で起きたからで、
// 同じ綴りの再導入だけは無料で止められるため。
func TestCurrency_NoStaleSpellingInProductSources(t *testing.T) {
	roots := []string{"../../internal", "../../cmd"}
	const stale = "rules --current"

	var hits []string
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			// テストは「--current を渡すとどうなるか」を実際に検査するので対象外。
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(b), "\n") {
				if strings.Contains(line, stale) {
					hits = append(hits, fmt.Sprintf("%s:%d: %s", path, i+1, strings.TrimSpace(line)))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("走査に失敗: %v", err)
		}
	}
	if len(hits) > 0 {
		t.Errorf(`%q が product のソースに戻っている:
%s

既定は畳む側になったので、この綴りは旧既定を教える。
「scholia rules（既定）」か「decision list --current」のどちらを指すのかを書き分けること。`,
			stale, strings.Join(hits, "\n"))
	}
}

// 誤用（--all と --current の同時指定）を黙って受理しないこと。
// レビュアの変異 N2 が素通りしていた——誰もこの性質を守っていなかった。
func TestCurrency_AllAndCurrentAreExclusive(t *testing.T) {
	dir := t.TempDir()
	f := setupWithdrawFixture(t, dir)

	out, err := run(t, dir, "rules", "--tag", "req.auth", "--all", "--current")
	if err == nil {
		t.Fatalf("--all と --current の同時指定はエラーになるべき:\n%s", out)
	}
	// 黙って受理して取り下げた本文を渡してしまう変異を、値でも見る。
	if strings.Contains(out, "旧タグ判断ホンブンA") {
		t.Errorf("誤用時に取り下げた本文を渡している:\n%s", out)
	}
	_ = f
}
