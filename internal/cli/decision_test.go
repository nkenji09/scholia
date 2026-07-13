package cli

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/nkenji09/product-memory/internal/store"
)

// commits 無しの旧 decision ファイルが無改修で読める（後方互換・§3.5）。
func TestCLI_DecisionCommitsBackwardCompatible(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Init(dir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	writeRawJSON(t, filepath.Join(s.Dir, "tags", "t.json"), `{"id":"t","name":"t"}`)
	writeRawJSON(t, filepath.Join(s.Dir, "decisions", "d-old.json"),
		`{"id":"d-old","target":{"type":"tag","id":"t"},"why":"旧レコード","at":"2026-01-01T00:00:00Z"}`)

	out, err := run(t, dir, "show", "decision", "d-old", "--json")
	if err != nil {
		t.Fatalf("show decision failed: %v\noutput:\n%s", err, out)
	}
	var d struct {
		ID      string   `json:"id"`
		Why     string   `json:"why"`
		Commits []string `json:"commits"`
	}
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("unmarshal: %v\noutput:\n%s", err, out)
	}
	if d.ID != "d-old" || d.Why != "旧レコード" {
		t.Fatalf("旧 decision の判断フィールドが変化した: %+v", d)
	}
	if len(d.Commits) != 0 {
		t.Fatalf("commits 無しの旧レコードで commits が空でない: %+v", d.Commits)
	}
}

// `pmem decide --commit a --commit b` で commits=[a,b] の decision が作られる。
func TestCLI_DecideWithCommitFlags(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "tag", "create", "t1", "--name", "t1", "--kind", "concern"); err != nil {
		t.Fatalf("tag create: %v", err)
	}

	out, err := run(t, dir, "decide", "--on", "tag:t1", "--why", "理由", "--commit", "a", "--commit", "b", "--json")
	if err != nil {
		t.Fatalf("decide --commit failed: %v\noutput:\n%s", err, out)
	}
	var d struct {
		Commits []string `json:"commits"`
	}
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("unmarshal: %v\noutput:\n%s", err, out)
	}
	if len(d.Commits) != 2 || d.Commits[0] != "a" || d.Commits[1] != "b" {
		t.Fatalf("commits が期待通りでない: %+v", d.Commits)
	}
}

// `pmem decision add-commit` は commits[] に追加のみし、判断フィールドは不変。
// 重複 hash は de-dupe、存在しない id はエラー。
func TestCLI_DecisionAddCommit(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "tag", "create", "t1", "--name", "t1", "--kind", "concern"); err != nil {
		t.Fatalf("tag create: %v", err)
	}
	decideOut, err := run(t, dir, "decide", "--on", "tag:t1", "--why", "元の理由", "--changed", "元の変更", "--ref", "PR#1", "--commit", "a", "--json")
	if err != nil {
		t.Fatalf("decide: %v\noutput:\n%s", err, decideOut)
	}
	var before struct {
		ID      string   `json:"id"`
		Target  any      `json:"target"`
		Why     string   `json:"why"`
		Changed string   `json:"changed"`
		Ref     string   `json:"ref"`
		At      string   `json:"at"`
		Commits []string `json:"commits"`
	}
	if err := json.Unmarshal([]byte(decideOut), &before); err != nil {
		t.Fatalf("unmarshal before: %v", err)
	}

	// 存在しない id はエラー。
	if _, err := run(t, dir, "decision", "add-commit", "does-not-exist", "z"); err == nil {
		t.Fatalf("expected error for nonexistent decision id")
	}

	// a（既出）・b・b（重複）を追加 → commits=[a,b]（de-dupe）。
	addOut, err := run(t, dir, "decision", "add-commit", before.ID, "a", "b", "b", "--json")
	if err != nil {
		t.Fatalf("add-commit failed: %v\noutput:\n%s", err, addOut)
	}
	var after struct {
		ID      string   `json:"id"`
		Target  any      `json:"target"`
		Why     string   `json:"why"`
		Changed string   `json:"changed"`
		Ref     string   `json:"ref"`
		At      string   `json:"at"`
		Commits []string `json:"commits"`
	}
	if err := json.Unmarshal([]byte(addOut), &after); err != nil {
		t.Fatalf("unmarshal after: %v\noutput:\n%s", err, addOut)
	}

	if len(after.Commits) != 2 || after.Commits[0] != "a" || after.Commits[1] != "b" {
		t.Fatalf("commits が de-dupe/追記で期待通りでない: before=%v after=%v", before.Commits, after.Commits)
	}

	// 判断フィールド（target/why/changed/ref/at）は before/after で完全一致（不変）。
	beforeJSON, _ := json.Marshal(before.Target)
	afterJSON, _ := json.Marshal(after.Target)
	if string(beforeJSON) != string(afterJSON) {
		t.Fatalf("target が変化した: before=%s after=%s", beforeJSON, afterJSON)
	}
	if before.Why != after.Why || before.Changed != after.Changed || before.Ref != after.Ref || before.At != after.At {
		t.Fatalf("判断フィールドが変化した: before=%+v after=%+v", before, after)
	}
}
