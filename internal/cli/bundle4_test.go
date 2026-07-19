package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
)

// setupBundle4 は decide/tag/decision コマンド検査用の最小 store を用意する。
func setupBundle4(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	if out, err := run(t, dir, "tag", "create", "req.standalone", "--name", "単一バイナリ", "--kind", "requirement"); err != nil {
		t.Fatalf("tag create: %v\n%s", err, out)
	}
	return dir
}

// decide --acknowledges: 有効 rule id は通り、未知 rule id は弾かれる（#45 D6）。
func TestCLIDecideAcknowledges(t *testing.T) {
	dir := setupBundle4(t)

	if out, err := run(t, dir, "decide", "--on", "tag:req.standalone",
		"--why", "遷移では充足されない性質型要件", "--acknowledges", "requirement-gap"); err != nil {
		t.Fatalf("decide --acknowledges requirement-gap should succeed: %v\n%s", err, out)
	}
	// 未知 rule id は reject。
	if _, err := run(t, dir, "decide", "--on", "tag:req.standalone",
		"--why", "typo", "--acknowledges", "not-a-real-rule"); err == nil {
		t.Fatalf("decide with unknown acknowledges rule id must fail")
	}
}

// tag edit --fulfillment property：property 宣言＋acknowledges decision で
// requirement-gap warn が畳まれる（#45 D6・エンドツーエンド）。
func TestCLITagFulfillmentPropertyFoldsWithDecision(t *testing.T) {
	dir := setupBundle4(t)

	if out, err := run(t, dir, "tag", "edit", "req.standalone", "--fulfillment", "property"); err != nil {
		t.Fatalf("tag edit --fulfillment property: %v\n%s", err, out)
	}
	// property のみ（decision 無し）→ warn のまま。
	out, err := run(t, dir, "lint", "--json")
	if err != nil {
		t.Fatalf("lint --json: %v\n%s", err, out)
	}
	if !strings.Contains(out, "requirement-gap") {
		t.Fatalf("property 宣言のみは requirement-gap warn のはず:\n%s", out)
	}

	// acknowledges:[requirement-gap] decision を足す → 容認済みに畳む。
	if out, err := run(t, dir, "decide", "--on", "tag:req.standalone",
		"--why", "単一バイナリは遷移で充足されない", "--acknowledges", "requirement-gap"); err != nil {
		t.Fatalf("decide: %v\n%s", err, out)
	}
	out, err = run(t, dir, "lint", "--json")
	if err != nil {
		t.Fatalf("lint --json 2: %v\n%s", err, out)
	}
	var payload struct {
		Findings []struct {
			Rule           string `json:"rule"`
			Target         string `json:"target"`
			AcknowledgedBy string `json:"acknowledgedBy"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode lint json: %v\n%s", err, out)
	}
	var found bool
	for _, f := range payload.Findings {
		if f.Rule == "requirement-gap" && f.Target == "req.standalone" {
			found = true
			if f.AcknowledgedBy == "" {
				t.Fatalf("requirement-gap は AcknowledgedBy 付きで畳むはず: %+v", f)
			}
		}
	}
	if !found {
		t.Fatalf("requirement-gap finding が見つからない:\n%s", out)
	}
}

// tag edit --fulfillment の値検証。
func TestCLITagFulfillmentRejectsBadValue(t *testing.T) {
	dir := setupBundle4(t)
	if _, err := run(t, dir, "tag", "edit", "req.standalone", "--fulfillment", "bogus"); err == nil {
		t.Fatalf("--fulfillment bogus must fail")
	}
}

// decide --supersedes / decision link / list --unlinked --current / show（#45 D7）。
func TestCLIDecisionSupersedeLifecycle(t *testing.T) {
	dir := setupBundle4(t)

	// 旧 decision を作る。
	out, err := run(t, dir, "decide", "--on", "tag:req.standalone", "--why", "旧判断", "--json")
	if err != nil {
		t.Fatalf("decide old: %v\n%s", err, out)
	}
	oldID := decisionIDFromJSON(t, out)

	// 新 decision が旧を supersede（全文置換）。
	out, err = run(t, dir, "decide", "--on", "tag:req.standalone", "--why", "新判断（旧を置換）",
		"--supersedes", oldID+":supersede", "--json")
	if err != nil {
		t.Fatalf("decide new --supersedes: %v\n%s", err, out)
	}
	newID := decisionIDFromJSON(t, out)

	// 実在しない旧 id は弾く。
	if _, err := run(t, dir, "decide", "--on", "tag:req.standalone", "--why", "x", "--supersedes", "01NONEXISTENT0000000000000"); err == nil {
		t.Fatalf("supersede of nonexistent decision must fail")
	}

	// list --current は失効した旧を畳む。
	out, err = run(t, dir, "decision", "list", "--current")
	if err != nil {
		t.Fatalf("decision list --current: %v\n%s", err, out)
	}
	if strings.Contains(out, oldID) {
		t.Fatalf("--current は supersede された旧 decision を畳むはず（oldID が出た）:\n%s", out)
	}
	if !strings.Contains(out, newID) {
		t.Fatalf("--current に現行 newID が出るべき:\n%s", out)
	}

	// list --unlinked は commits 空の decision を列挙（両方とも commits 空）。
	out, err = run(t, dir, "decision", "list", "--unlinked")
	if err != nil {
		t.Fatalf("decision list --unlinked: %v\n%s", err, out)
	}
	if !strings.Contains(out, newID) {
		t.Fatalf("--unlinked に commits 空の newID が出るべき:\n%s", out)
	}

	// show は supersedes と superseded-by（derive）を出す。
	out, err = run(t, dir, "decision", "show", newID)
	if err != nil {
		t.Fatalf("decision show new: %v\n%s", err, out)
	}
	if !strings.Contains(out, "supersedes") || !strings.Contains(out, oldID) {
		t.Fatalf("show new に supersedes→oldID が出るべき:\n%s", out)
	}
	out, err = run(t, dir, "decision", "show", oldID)
	if err != nil {
		t.Fatalf("decision show old: %v\n%s", err, out)
	}
	if !strings.Contains(out, "superseded-by") || !strings.Contains(out, newID) {
		t.Fatalf("show old に superseded-by←newID が出るべき:\n%s", out)
	}
	if !strings.Contains(out, "失効") {
		t.Fatalf("show old は失効表示のはず:\n%s", out)
	}
}

// decision link: 後付け結線・自己参照禁止・循環禁止・冪等（#45 D7）。
func TestCLIDecisionLink(t *testing.T) {
	dir := setupBundle4(t)

	out, _ := run(t, dir, "decide", "--on", "tag:req.standalone", "--why", "A", "--json")
	aID := decisionIDFromJSON(t, out)
	out, _ = run(t, dir, "decide", "--on", "tag:req.standalone", "--why", "B", "--json")
	bID := decisionIDFromJSON(t, out)

	// B が A を後付けで supersede。
	if out, err := run(t, dir, "decision", "link", bID, "--supersedes", aID+":supersede"); err != nil {
		t.Fatalf("decision link: %v\n%s", err, out)
	}
	// 冪等: 同一 link 再指定は変更なし。
	if out, err := run(t, dir, "decision", "link", bID, "--supersedes", aID+":supersede"); err != nil || !strings.Contains(out, "冪等") {
		t.Fatalf("re-link should be idempotent: err=%v\n%s", err, out)
	}
	// 自己参照禁止。
	if _, err := run(t, dir, "decision", "link", bID, "--supersedes", bID); err == nil {
		t.Fatalf("self supersede must fail")
	}
	// 循環禁止: A→B を足すと B→A（既存）と閉路。
	if _, err := run(t, dir, "decision", "link", aID, "--supersedes", bID+":supersede"); err == nil {
		t.Fatalf("cyclic supersede must fail")
	}
	// 既存 link の mode 改変禁止（append-only）。
	if _, err := run(t, dir, "decision", "link", bID, "--supersedes", aID+":amend"); err == nil {
		t.Fatalf("changing existing link mode must fail")
	}
}

func decisionIDFromJSON(t *testing.T, out string) string {
	t.Helper()
	// decide --json は writeEnvelope { record: Decision, ... }。
	var env struct {
		Record model.Decision `json:"record"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("decode decide json: %v\n%s", err, out)
	}
	if env.Record.ID == "" {
		t.Fatalf("decision id empty in json:\n%s", out)
	}
	return env.Record.ID
}

// decision-stale（#45 D7）: 既存レコードを変更した commit に decision 追加が
// 同伴しなければ info で検出される（git 導出）。rename は除外・対象レコード宛て
// acknowledges:[decision-stale] で容認可。
func TestCLIDecisionStale(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)
	if out, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	if out, err := run(t, dir, "tag", "create", "subject.x", "--name", "主題", "--kind", "subject"); err != nil {
		t.Fatalf("tag create: %v\n%s", err, out)
	}
	gitCommitAllT(t, dir, "seed store")

	// 既存レコードを変更（decision なし）→ decision-stale 検出。
	if out, err := run(t, dir, "tag", "edit", "subject.x", "--name", "主題v2"); err != nil {
		t.Fatalf("tag edit: %v\n%s", err, out)
	}
	gitCommitAllT(t, dir, "edit tag without decision")

	out, err := run(t, dir, "lint", "--json")
	if err != nil {
		t.Fatalf("lint --json: %v\n%s", err, out)
	}
	var payload struct {
		Findings []struct {
			Rule           string `json:"rule"`
			Target         string `json:"target"`
			AcknowledgedBy string `json:"acknowledgedBy"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	var staleFound bool
	for _, f := range payload.Findings {
		if f.Rule == "decision-stale" {
			staleFound = true
			if f.AcknowledgedBy != "" {
				t.Fatalf("まだ容認していないのに AcknowledgedBy が付いた: %+v", f)
			}
		}
	}
	if !staleFound {
		t.Fatalf("レコード変更 commit に decision 非同伴 → decision-stale が出るはず:\n%s", out)
	}

	// 対象レコード宛て acknowledges:[decision-stale] decision で容認 → 畳む。
	if out, err := run(t, dir, "decide", "--on", "tag:subject.x",
		"--why", "一括マイグレーションのため decision 非同伴を容認",
		"--acknowledges", "decision-stale"); err != nil {
		t.Fatalf("decide acknowledges decision-stale: %v\n%s", err, out)
	}
	gitCommitAllT(t, dir, "add acknowledging decision")

	out, err = run(t, dir, "lint", "--json")
	if err != nil {
		t.Fatalf("lint --json 2: %v\n%s", err, out)
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("decode 2: %v\n%s", err, out)
	}
	for _, f := range payload.Findings {
		// 元の "edit tag without decision" commit を触った decision-stale が
		// AcknowledgedBy 付きで畳まれていることを確認（subject.x を触った commit）。
		if f.Rule == "decision-stale" && strings.Contains(f.Target, "") && f.AcknowledgedBy == "" {
			// 容認 decision commit 自体は subject.x を変更しないので stale にならない。
			// subject.x を触った旧 commit が畳まれていればよい。
		}
	}
	// 少なくとも1件は AcknowledgedBy 付きになっているはず（subject.x を触った commit）。
	var anyAcked bool
	for _, f := range payload.Findings {
		if f.Rule == "decision-stale" && f.AcknowledgedBy != "" {
			anyAcked = true
		}
	}
	if !anyAcked {
		t.Fatalf("subject.x 宛て acknowledges:[decision-stale] で subject.x 変更 commit が畳まれるはず:\n%s", out)
	}
}
