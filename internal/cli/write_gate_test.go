package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/lint"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// setupGateFixture は書き込みゲートの CLI テスト共通 fixture:
// axis kind を宣言し、同一軸 axis.mode の2値 condition と action/effect を持つ。
func setupGateFixture(t *testing.T, dir string) {
	t.Helper()
	mustRun(t, dir, "init")

	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	cfg.TagKinds = append(cfg.TagKinds, model.KindDecl{ID: "axis"})
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	mustRun(t, dir, "tag", "create", "axis.mode", "--name", "モード", "--kind", "axis")
	mustRun(t, dir, "vocab", "add", "condition", "cond.mode-a", "--label", "モードA")
	mustRun(t, dir, "vocab", "add", "condition", "cond.mode-b", "--label", "モードB")
	mustRun(t, dir, "vocab", "tag", "cond.mode-a", "--add", "axis.mode")
	mustRun(t, dir, "vocab", "tag", "cond.mode-b", "--add", "axis.mode")
	mustRun(t, dir, "vocab", "add", "action", "act.user.run", "--label", "実行", "--kind", "user")
	mustRun(t, dir, "vocab", "add", "effect", "eff.state.apply", "--label", "適用", "--kind", "state")
}

func declareIDPolicy(t *testing.T, dir string, pol *model.IDPolicy) {
	t.Helper()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	cfg.IDPolicy = pol
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
}

// reject (a): 同一軸2値 given の tx add は exit 1・保存しない。
func TestCLI_TxAddRejectsSameAxisTwoValueGiven(t *testing.T) {
	dir := t.TempDir()
	setupGateFixture(t, dir)

	out, err := run(t, dir, "tx", "add", "T-conflict",
		"--action", "act.user.run",
		"--given", "cond.mode-a,cond.mode-b",
		"--then", "eff.state.apply")
	if err == nil {
		t.Fatalf("同一軸2値 given は reject で失敗するはず。output:\n%s", out)
	}
	if !strings.Contains(out, "reject(exclusive-violation)") {
		t.Fatalf("reject 行が出力されるはず:\n%s", out)
	}

	s, _ := store.Open(dir)
	if s.TransitionExists("T-conflict") {
		t.Fatalf("reject された transition は保存されないはず")
	}
}

// --allow は理由必須で reject を破って保存し、stdout と --json に記録される。
func TestCLI_TxAddAllowWithReasonSavesAndRecords(t *testing.T) {
	dir := t.TempDir()
	setupGateFixture(t, dir)

	// --reason 無しの --allow は拒否（保存しない）。
	out, err := run(t, dir, "tx", "add", "T-conflict",
		"--action", "act.user.run",
		"--given", "cond.mode-a,cond.mode-b",
		"--then", "eff.state.apply",
		"--allow", "exclusive-violation")
	if err == nil {
		t.Fatalf("--allow に --reason 無しはエラーのはず。output:\n%s", out)
	}
	s, _ := store.Open(dir)
	if s.TransitionExists("T-conflict") {
		t.Fatalf("--reason 無しの --allow で保存されてはいけない")
	}

	// 未知の rule 名も拒否。
	if out, err := run(t, dir, "tx", "add", "T-conflict",
		"--action", "act.user.run",
		"--given", "cond.mode-a,cond.mode-b",
		"--then", "eff.state.apply",
		"--allow", "no-such-rule", "--reason", "理由"); err == nil {
		t.Fatalf("未知の --allow rule はエラーのはず。output:\n%s", out)
	}

	// 理由付き --allow は保存し stdout に記録。
	out = mustRun(t, dir, "tx", "add", "T-conflict",
		"--action", "act.user.run",
		"--given", "cond.mode-a,cond.mode-b",
		"--then", "eff.state.apply",
		"--allow", "exclusive-violation", "--reason", "移行期間中の暫定重複（#45）")
	if !strings.Contains(out, "allow(exclusive-violation)") || !strings.Contains(out, "移行期間中の暫定重複") {
		t.Fatalf("allow の記録行が stdout に出るはず:\n%s", out)
	}
	if !s.TransitionExists("T-conflict") {
		t.Fatalf("--allow --reason で保存されるはず")
	}

	// --json 側の記録（allowed[]）。
	jsonOut := mustRun(t, dir, "tx", "edit", "T-conflict",
		"--given", "cond.mode-a,cond.mode-b",
		"--allow", "exclusive-violation", "--reason", "同上", "--json")
	var env struct {
		Record     model.Transition `json:"record"`
		Advisories []lint.Finding   `json:"advisories"`
		Allowed    []struct {
			Rule     string         `json:"rule"`
			Reason   string         `json:"reason"`
			Findings []lint.Finding `json:"findings"`
		} `json:"allowed"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v\n%s", err, jsonOut)
	}
	if env.Record.ID != "T-conflict" {
		t.Fatalf("record が封筒に入るはず: %+v", env.Record)
	}
	if len(env.Allowed) != 1 || env.Allowed[0].Rule != "exclusive-violation" || env.Allowed[0].Reason != "同上" || len(env.Allowed[0].Findings) != 1 {
		t.Fatalf("allowed[] に rule/reason/findings が記録されるはず: %+v", env.Allowed)
	}
}

// reject (b): --total×非 axis kind の tag create/edit は exit 1・保存しない。
func TestCLI_TagTotalOnNonAxisKindRejected(t *testing.T) {
	dir := t.TempDir()
	setupGateFixture(t, dir)

	out, err := run(t, dir, "tag", "create", "req.total", "--name", "要件", "--kind", "requirement", "--total")
	if err == nil {
		t.Fatalf("--total×非 axis kind は reject のはず。output:\n%s", out)
	}
	if !strings.Contains(out, "reject(total-kind-mismatch)") {
		t.Fatalf("reject 行が出力されるはず:\n%s", out)
	}
	s, _ := store.Open(dir)
	if s.TagExists("req.total") {
		t.Fatalf("reject された tag は保存されないはず")
	}

	// axis kind への --total は正当。
	mustRun(t, dir, "tag", "create", "axis.total", "--name", "全域軸", "--kind", "axis", "--total")

	// edit 経路: 既存の非 axis タグに --total を立てるのも reject。
	mustRun(t, dir, "tag", "create", "req.plain", "--name", "普通の要件", "--kind", "requirement")
	if out, err := run(t, dir, "tag", "edit", "req.plain", "--total"); err == nil {
		t.Fatalf("tag edit の --total×非 axis も reject のはず。output:\n%s", out)
	}
	// --allow で明示に破れる。
	mustRun(t, dir, "tag", "edit", "req.plain", "--total", "--allow", "total-kind-mismatch", "--reason", "kind 移行の途中")
}

// reject (c): idPolicy 違反は新規 id のみ・既存 id の edit は素通り。
func TestCLI_IDPolicyRejectsNewIDsOnly(t *testing.T) {
	dir := t.TempDir()
	setupGateFixture(t, dir)
	// 宣言前の既存 id（旧様式）。
	mustRun(t, dir, "tx", "add", "T-legacy", "--action", "act.user.run", "--given", "cond.mode-a", "--then", "eff.state.apply")

	declareIDPolicy(t, dir, &model.IDPolicy{Transition: "tx.", Vocab: map[string]string{"condition": "cond."}})

	// 新規 transition の宣言違反 → reject・保存しない。
	out, err := run(t, dir, "tx", "add", "T-new", "--action", "act.user.run", "--then", "eff.state.apply")
	if err == nil {
		t.Fatalf("idPolicy 違反の新規 id は reject のはず。output:\n%s", out)
	}
	if !strings.Contains(out, "reject(id-policy)") {
		t.Fatalf("reject 行が出力されるはず:\n%s", out)
	}
	s, _ := store.Open(dir)
	if s.TransitionExists("T-new") {
		t.Fatalf("reject された transition は保存されないはず")
	}

	// 宣言準拠の新規 id は通る。
	mustRun(t, dir, "tx", "add", "tx.Comp.run", "--action", "act.user.run", "--then", "eff.state.apply")

	// 既存 id（T-legacy）の edit は対象外。
	mustRun(t, dir, "tx", "edit", "T-legacy", "--given", "cond.mode-b")

	// vocab の新規 id もカテゴリ宣言で検査。
	if out, err := run(t, dir, "vocab", "add", "condition", "x.bad", "--label", "bad"); err == nil {
		t.Fatalf("idPolicy.vocab.condition 違反は reject のはず。output:\n%s", out)
	}
	// --allow で明示に破れる。
	mustRun(t, dir, "vocab", "add", "condition", "x.grandfathered", "--label", "既存慣例",
		"--allow", "id-policy", "--reason", "旧様式との一時併存（rename はデータ後段）")
}

// advisory: 保存は成功し、同一ターンに advisory(rule) 行が出る。
func TestCLI_WriteAdvisoryFiresOnSameTurn(t *testing.T) {
	dir := t.TempDir()
	setupGateFixture(t, dir)

	out := mustRun(t, dir, "vocab", "edit", "cond.mode-a", "--description", "現状は暫定のモード")
	if !strings.Contains(out, "vocab cond.mode-a を更新しました") {
		t.Fatalf("advisory があっても保存は成功するはず:\n%s", out)
	}
	if !strings.Contains(out, "advisory(stale-tense)") {
		t.Fatalf("同一ターンに advisory 行が出るはず:\n%s", out)
	}

	// 是正すれば advisory は消える。
	out = mustRun(t, dir, "vocab", "edit", "cond.mode-a", "--description", "モードAが選択されている")
	if strings.Contains(out, "advisory(") {
		t.Fatalf("是正後は advisory ゼロのはず:\n%s", out)
	}
}

// decide --dry-run: 保存せず advisory をプレビューする（append-only 対策）。
func TestCLI_DecideDryRunPreviewsWithoutSaving(t *testing.T) {
	dir := t.TempDir()
	setupGateFixture(t, dir)
	mustRun(t, dir, "tx", "add", "T-x", "--action", "act.user.run", "--then", "eff.state.apply")

	out := mustRun(t, dir, "decide", "--on", "transition:T-x",
		"--why", "internal/foo.go:12 の分岐に合わせた", "--dry-run")
	if !strings.Contains(out, "dry-run") || !strings.Contains(out, "advisory(why-file-line)") {
		t.Fatalf("dry-run は保存せず advisory を出すはず:\n%s", out)
	}

	s, _ := store.Open(dir)
	snap, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(snap.Decisions) != 0 {
		t.Fatalf("dry-run で decision が保存されてはいけない: %+v", snap.Decisions)
	}

	// advisory ゼロの dry-run はその旨を明示する。
	out = mustRun(t, dir, "decide", "--on", "transition:T-x", "--why", "分岐仕様を確定", "--dry-run")
	if !strings.Contains(out, "advisory: なし") {
		t.Fatalf("advisory ゼロの dry-run は「なし」を明示するはず:\n%s", out)
	}

	// 本番 decide は保存し、保存後にも advisory を表示する。
	out = mustRun(t, dir, "decide", "--on", "transition:T-x", "--why", "internal/foo.go:12 の分岐に合わせた")
	if !strings.Contains(out, "を記録しました") || !strings.Contains(out, "advisory(why-file-line)") {
		t.Fatalf("通常 decide も保存後に advisory を表示するはず:\n%s", out)
	}
	snap, _ = s.LoadAll()
	if len(snap.Decisions) != 1 {
		t.Fatalf("通常 decide は保存されるはず: %+v", snap.Decisions)
	}
}

// --json 応答封筒 { record, advisories }（dry-run は dryRun: true 付き）。
func TestCLI_WriteJSONEnvelopeShape(t *testing.T) {
	dir := t.TempDir()
	setupGateFixture(t, dir)

	// advisory ゼロでも advisories は空配列で常在する。
	out := mustRun(t, dir, "tx", "add", "T-clean", "--action", "act.user.run",
		"--given", "cond.mode-a", "--then", "eff.state.apply", "--json")
	var env struct {
		Record     *model.Transition `json:"record"`
		Advisories []lint.Finding    `json:"advisories"`
		DryRun     bool              `json:"dryRun"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v\n%s", err, out)
	}
	if env.Record == nil || env.Record.ID != "T-clean" {
		t.Fatalf("record が封筒に入るはず:\n%s", out)
	}
	if env.Advisories == nil || len(env.Advisories) != 0 {
		t.Fatalf("advisories は空でも [] で常在するはず:\n%s", out)
	}
	if !strings.Contains(out, `"advisories": []`) {
		t.Fatalf("advisories キーが JSON に現れるはず:\n%s", out)
	}

	// advisory 有りの封筒（vocab add の stale-tense）。
	out = mustRun(t, dir, "vocab", "add", "effect", "eff.state.new", "--label", "新設", "--kind", "state",
		"--description", "本タスクで新設した効果", "--json")
	var env2 struct {
		Record     model.VocabEntry `json:"record"`
		Advisories []lint.Finding   `json:"advisories"`
	}
	if err := json.Unmarshal([]byte(out), &env2); err != nil {
		t.Fatalf("unmarshal envelope: %v\n%s", err, out)
	}
	if len(env2.Advisories) != 1 || env2.Advisories[0].Rule != "stale-tense" {
		t.Fatalf("advisories に stale-tense が入るはず:\n%s", out)
	}

	// decide --dry-run の封筒は dryRun: true。
	mustRun(t, dir, "tx", "add", "T-x", "--action", "act.user.run", "--then", "eff.state.apply")
	out = mustRun(t, dir, "decide", "--on", "transition:T-x", "--why", "理由", "--dry-run", "--json")
	var env3 struct {
		Record     model.Decision `json:"record"`
		Advisories []lint.Finding `json:"advisories"`
		DryRun     bool           `json:"dryRun"`
	}
	if err := json.Unmarshal([]byte(out), &env3); err != nil {
		t.Fatalf("unmarshal envelope: %v\n%s", err, out)
	}
	if !env3.DryRun {
		t.Fatalf("dry-run の封筒は dryRun: true のはず:\n%s", out)
	}
}
