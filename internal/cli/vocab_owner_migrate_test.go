package cli

import (
	"strings"
	"testing"
)

// vocab owner-migrate は既存 owner 値とその効果を列挙し、ownerKind 宣言下では
// 候補タグを併記する（書き込みはしない・#45 D9）。
func TestCLI_VocabOwnerMigrateProposesMapping(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "effect", "eff.a", "--label", "a", "--owner", "cli")
	mustRun(t, dir, "vocab", "add", "effect", "eff.b", "--label", "b", "--owner", "store")
	mustRun(t, dir, "tag", "create", "subject.cli", "--name", "CLI", "--kind", "subject")
	setOwnerKind(t, dir, "subject")

	out := mustRun(t, dir, "vocab", "owner-migrate")
	if !strings.Contains(out, "cli") || !strings.Contains(out, "store") {
		t.Fatalf("owner-migrate should list existing owners, got:\n%s", out)
	}
	if !strings.Contains(out, "subject.cli") {
		t.Fatalf("owner-migrate should list ownerKind candidate tags, got:\n%s", out)
	}

	// JSON 形も壊れない。
	outJSON := mustRun(t, dir, "vocab", "owner-migrate", "--json")
	if !strings.Contains(outJSON, `"entries"`) {
		t.Fatalf("owner-migrate --json should emit entries, got:\n%s", outJSON)
	}
}

// ownerKind 未宣言でも動く（候補なしのプレーン列挙）。
func TestCLI_VocabOwnerMigrateWithoutOwnerKind(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "effect", "eff.a", "--label", "a", "--owner", "cli")
	out := mustRun(t, dir, "vocab", "owner-migrate")
	if !strings.Contains(out, "未宣言") {
		t.Fatalf("owner-migrate should note ownerKind unset, got:\n%s", out)
	}
}
