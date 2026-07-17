package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupMergeStore は duplicate-atom（同一 action+given+then の複製）ペア
// T-dup / T-surv と、T-dup 宛の decision d-dup を持つ store を組む。
func setupMergeStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	steps := [][]string{
		{"init"},
		{"vocab", "add", "condition", "cond.a", "--label", "条件A"},
		{"vocab", "add", "action", "act.a", "--label", "アクションA"},
		{"vocab", "add", "effect", "eff.a", "--label", "効果A"},
		{"vocab", "add", "effect", "eff.b", "--label", "効果B"},
		{"tag", "create", "subject.x", "--name", "主題X", "--kind", "subject"},
		{"tag", "create", "subject.y", "--name", "主題Y", "--kind", "subject"},
		{"tx", "add", "T-surv", "--action", "act.a", "--given", "cond.a", "--then", "eff.a", "--tags", "subject.x"},
		{"tx", "add", "T-dup", "--action", "act.a", "--given", "cond.a", "--then", "eff.a", "--tags", "subject.y"},
		{"tx", "add", "T-other", "--action", "act.a", "--then", "eff.b"},
		{"decide", "--on", "transition:T-dup", "--why", "dup 側に付いた判断"},
	}
	for _, s := range steps {
		if out, err := run(t, dir, s...); err != nil {
			t.Fatalf("%v failed: %v\noutput:\n%s", s, err, out)
		}
	}
	return dir
}

func TestTxMerge_DecisionFollowsAndTagsUnion(t *testing.T) {
	dir := setupMergeStore(t)

	out, err := run(t, dir, "tx", "merge", "T-dup", "--into", "T-surv", "--no-refs", "--json")
	if err != nil {
		t.Fatalf("tx merge: %v\n%s", err, out)
	}
	var parsed struct {
		Rename struct {
			DupID            string   `json:"dupId"`
			SurvivorID       string   `json:"survivorId"`
			UpdatedDecisions []string `json:"updatedDecisions"`
			AddedTags        []string `json:"addedTags"`
		} `json:"rename"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(parsed.Rename.UpdatedDecisions) != 1 {
		t.Fatalf("updatedDecisions = %v, want 1 件", parsed.Rename.UpdatedDecisions)
	}
	if len(parsed.Rename.AddedTags) != 1 || parsed.Rename.AddedTags[0] != "subject.y" {
		t.Fatalf("addedTags = %v, want [subject.y]", parsed.Rename.AddedTags)
	}

	// dup は削除され、survivor はタグ union 済み。
	if _, err := os.Stat(filepath.Join(dir, ".scholia", "transitions", "T-dup.json")); !os.IsNotExist(err) {
		t.Fatalf("T-dup.json が残っている: %v", err)
	}
	showOut, err := run(t, dir, "show", "tx", "T-surv")
	if err != nil {
		t.Fatalf("show tx: %v\n%s", err, showOut)
	}
	if !strings.Contains(showOut, "subject.x") || !strings.Contains(showOut, "subject.y") {
		t.Fatalf("survivor のタグ union が反映されていない:\n%s", showOut)
	}

	// decision の target は survivor を指し、lint（decision-target）が緑。
	if lintOut, err := run(t, dir, "lint"); err != nil {
		t.Fatalf("merge 後の lint が失敗（dangling target？）: %v\n%s", err, lintOut)
	}
}

func TestTxMerge_RejectsNonIdenticalAtom(t *testing.T) {
	dir := setupMergeStore(t)

	// T-other は then が異なる（eff.b）→ 同一原子でないため拒否。
	out, err := run(t, dir, "tx", "merge", "T-other", "--into", "T-surv", "--no-refs")
	if err == nil {
		t.Fatalf("非同一原子の merge が通った:\n%s", out)
	}
	if !strings.Contains(out, "同一原子") {
		t.Fatalf("expected same-atom rejection message:\n%s", out)
	}
	// 何も変わっていない（T-other は残る）。
	if _, err := os.Stat(filepath.Join(dir, ".scholia", "transitions", "T-other.json")); err != nil {
		t.Fatalf("拒否されたのに T-other が消えた: %v", err)
	}
}

func TestTxMerge_RequiresIntoAndDistinctIDs(t *testing.T) {
	dir := setupMergeStore(t)

	if out, err := run(t, dir, "tx", "merge", "T-dup"); err == nil {
		t.Fatalf("--into なしの merge が通った:\n%s", out)
	}
	if out, err := run(t, dir, "tx", "merge", "T-dup", "--into", "T-dup"); err == nil {
		t.Fatalf("自分自身への merge が通った:\n%s", out)
	}
	if out, err := run(t, dir, "tx", "merge", "T-nonexistent", "--into", "T-surv"); err == nil {
		t.Fatalf("実在しない dup の merge が通った:\n%s", out)
	}
}

func TestTxMerge_SourceRefsDryRunByDefault(t *testing.T) {
	dir := setupMergeStore(t)

	// ソースに dup id への参照を残す → 既定は走査のみ（ソース不変）。
	srcPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcPath, []byte("// T-dup を参照するコメント\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, dir, "tx", "merge", "T-dup", "--into", "T-surv")
	if err != nil {
		t.Fatalf("tx merge: %v\n%s", err, out)
	}
	if !strings.Contains(out, "旧 id の参照が 1 箇所残っています") {
		t.Fatalf("expected dry-run refs report:\n%s", out)
	}
	if data, _ := os.ReadFile(srcPath); !strings.Contains(string(data), "T-dup") {
		t.Fatalf("既定（dry-run）なのにソースが書き換わった: %s", data)
	}

	// --rewrite-refs でその場置換（merge 済みなので survivor 側の参照になる）。
	if out, err := run(t, dir, "tx", "merge", "T-surv", "--into", "T-other"); err == nil {
		t.Fatalf("sanity: 非同一原子 merge が通ってしまった\n%s", out)
	}
	if out, err := run(t, dir, "refs", "rewrite", "T-dup", "T-surv", "--apply"); err != nil {
		t.Fatalf("refs rewrite: %v\n%s", err, out)
	}
	if data, _ := os.ReadFile(srcPath); !strings.Contains(string(data), "T-surv") {
		t.Fatalf("refs rewrite 後もソースが旧 id のまま: %s", data)
	}
}

func TestTxMerge_BaselineRetargeted(t *testing.T) {
	dir := setupMergeStore(t)

	// baseline に dup 宛の warn entry を作る（requirement-gap ではなく手組みの
	// 形式で十分——キーは rule+target）。
	writeRawJSON(t, filepath.Join(dir, ".scholia", "lint-baseline.json"),
		`{"schemaVersion":1,"findings":[{"rule":"some-warn","target":"T-dup"}]}`)

	out, err := run(t, dir, "tx", "merge", "T-dup", "--into", "T-surv", "--no-refs", "--json")
	if err != nil {
		t.Fatalf("tx merge: %v\n%s", err, out)
	}
	var parsed struct {
		Rename struct {
			BaselineRetargeted bool `json:"baselineRetargeted"`
		} `json:"rename"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if !parsed.Rename.BaselineRetargeted {
		t.Fatalf("baselineRetargeted = false, want true:\n%s", out)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".scholia", "lint-baseline.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "T-surv") || strings.Contains(string(data), "T-dup") {
		t.Fatalf("baseline の target が追随していない: %s", data)
	}
}
