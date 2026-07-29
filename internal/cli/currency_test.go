package cli

import (
	"encoding/json"
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
//   - decision 本文を出しうる面が**新設されたのに、ここへ登録されないこと**
//     （TestCurrency_EverySurfaceIsClassified）。
//
// 落ちないもの（原理的に）:
//   - **人が読む出力の体裁**（見出し語・記号・並び）。ここは意味を見ておらず、
//     「本文が出ていないか」「id が出ているか」だけを見る。体裁の劣化は捕まらない。
//   - **viewer / 静的書き出し**。あちらは Go のこの層を通らない（web/ 側の検査と
//     internal/viewer のテストが担う）。この単位では旧バイナリとの API 差分を
//     手で測ったが、それは自動では回らない。
//   - **配布スキル・手順書の記述**。テキストであって、機械で落とす手段が無い
//     （01KXS68HCNQ0H9QKNYFQ869J19 が言う「遡及機構が無い」領域そのもの）。

// setupWithdrawFixture は「取り下げられた decision が tag と transition の
// 両方にある」store を作り、(旧tag, 新tag, 旧tx, 新tx) の id を返す。
func setupWithdrawFixture(t *testing.T, dir string) (oldTag, newTag, oldTx, newTx string) {
	t.Helper()
	setupAuthFixture(t, dir)

	oldTag = decisionIDFromJSON(t, mustRun(t, dir, "decide", "--on", "tag:req.auth",
		"--why", "旧タグ判断ホンブンA", "--json"))
	newTag = decisionIDFromJSON(t, mustRun(t, dir, "decide", "--on", "tag:req.auth",
		"--why", "新タグ判断ホンブンB", "--supersedes", oldTag+":supersede", "--json"))

	oldTx = decisionIDFromJSON(t, mustRun(t, dir, "decide", "--on", "transition:T-login",
		"--why", "旧遷移判断ホンブンC", "--json"))
	newTx = decisionIDFromJSON(t, mustRun(t, dir, "decide", "--on", "transition:T-login",
		"--why", "新遷移判断ホンブンD", "--supersedes", oldTx+":supersede", "--json"))
	return oldTag, newTag, oldTx, newTx
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
	oldTag, newTag, oldTx, newTx := setupWithdrawFixture(t, dir)

	// 面ごとに「取り下げられた本文」「取り下げられた id」「行き先の id」が
	// 何であるかは違う。tag 宛の面と transition 宛の面を両方通す。
	type expect struct {
		body      string // 既定の出力に出てはいけない本文
		id        string // 既定の出力に出るべき id（存在）
		replacer  string // 既定の出力に出るべき id（行き先）
		liveBody  string // 既定の出力に出るべき本文（効いている側）
		surfaceID string
	}
	tagExp := expect{body: "旧タグ判断ホンブンA", id: oldTag, replacer: newTag, liveBody: "新タグ判断ホンブンB"}
	txExp := expect{body: "旧遷移判断ホンブンC", id: oldTx, replacer: newTx, liveBody: "新遷移判断ホンブンD"}

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

// search は畳まない面。隠さない代わりに印と行き先を必ず出す。
func TestCurrency_SearchMarksWithdrawnWithoutHiding(t *testing.T) {
	dir := t.TempDir()
	oldTag, newTag, _, _ := setupWithdrawFixture(t, dir)

	out := mustRun(t, dir, "search", "ホンブンA")
	if !strings.Contains(out, oldTag) {
		t.Fatalf("search は取り下げられた記録を隠さないはず（id が出ていない）:\n%s", out)
	}
	if !strings.Contains(out, "取り下げ済み") {
		t.Fatalf("search は取り下げ済みの印を付けるはず:\n%s", out)
	}
	if !strings.Contains(out, newTag) {
		t.Fatalf("search の印から行き先 %s へ辿れるはず:\n%s", newTag, out)
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
		if m.ID != oldTag {
			continue
		}
		found = true
		if m.Effect != string(EffectReplaced) {
			t.Errorf("search --json の effect = %q, want %q", m.Effect, EffectReplaced)
		}
		if len(m.ReplacedBy) != 1 || m.ReplacedBy[0] != newTag {
			t.Errorf("search --json の replacedBy = %v, want [%s]", m.ReplacedBy, newTag)
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
	oldTag, newTag, _, _ := setupWithdrawFixture(t, dir)

	t.Run("rules --json", func(t *testing.T) {
		var out struct {
			Decisions []map[string]any `json:"decisions"`
			Withdrawn []map[string]any `json:"withdrawn"`
		}
		mustUnmarshal(t, mustRun(t, dir, "rules", "--tag", "req.auth", "--json"), &out)
		assertAllHaveEffect(t, "decisions", out.Decisions)
		assertWithdrawnShape(t, out.Withdrawn, oldTag, newTag)
		for _, d := range out.Decisions {
			if d["id"] == oldTag {
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
			if d["id"] == oldTag {
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
			if d["id"] != oldTag {
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
		assertWithdrawnShape(t, out.Withdrawn, oldTag, newTag)
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
		"scholia decision list",
	}
	// decision 本文を出すが、既定を変えないと決めた面。
	//   decision show   … 1 件を名指しで開く経路。現行性を行で明示する。
	//   decision list   … 棚卸しの面（上で JSON だけ検査）。text の既定は変えない。
	//   review show/list… 提案（まだ decision ではない）を読む面。
	surfacesIntentionallyFull = []string{
		"scholia decision show",
		"scholia show decision",
		"scholia review list",
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

	// 既知の分類（守っている面 ∪ 意図して全文を出す面 ∪ decision 本文を出さない面）。
	known := map[string]bool{}
	for _, s := range append(append([]string{}, surfacesGuarded...), surfacesIntentionallyFull...) {
		known[s] = true
	}
	for _, s := range surfacesWithoutDecisionBodies {
		known[s] = true
	}

	var unclassified []string
	for _, l := range leaves {
		if !known[l] {
			unclassified = append(unclassified, l)
		}
	}
	sort.Strings(unclassified)
	if len(unclassified) > 0 {
		t.Fatalf(`未分類の面がある: %v

decision 本文を出す面を足したなら surfacesGuarded に足し、
取り下げの扱いを上のテストで検査すること。
本文を出さない面なら surfacesWithoutDecisionBodies に足すこと。
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

// decision 本文を出さない面。ここに載せる基準は「decision の why/changed を
// 出力に含めないこと」であって、decision に触らないことではない。
var surfacesWithoutDecisionBodies = []string{
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
	"scholia view",
	"scholia export",
	"scholia version",
	"scholia update",
	"scholia retrofit",
	"scholia decide",
	"scholia decision link",
	"scholia decision add-commit",
	"scholia review add",
	"scholia review adopt",
	"scholia review reject",
	"scholia review rm",
	"scholia show tag",
	"scholia show tx",
	"scholia show vocab",
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
