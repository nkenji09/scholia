package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nkenji09/product-memory/internal/index"
	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

func testSnapshot() *store.Snapshot {
	return &store.Snapshot{
		Vocab: []model.VocabEntry{
			{ID: "act.submit", Category: model.CategoryAction, Label: "ログイン送信"},
			{ID: "cond.valid", Category: model.CategoryCondition, Label: "資格情報が正当"},
			{ID: "eff.token", Category: model.CategoryEffect, Label: "セッショントークン発行"},
			{ID: "eff.redirect", Category: model.CategoryEffect, Label: "ホームへリダイレクト"},
		},
		Tags: []model.Tag{
			{ID: "subject.auth", Name: "認証", Kind: "subject", Description: "認証まわりの主題"},
			{ID: "req.auth-happy", Name: "正常系ログイン", Kind: "requirement", ParentIDs: []string{"subject.auth"}},
		},
		Transitions: []model.Transition{
			{ID: "T-1", Action: "act.submit", Given: []string{"cond.valid"}, Then: []string{"eff.token", "eff.redirect"},
				Tags: []string{"req.auth-happy"}},
		},
		Decisions: []model.Decision{
			{ID: "d1", Target: model.DecisionTarget{Type: model.DecisionTargetTransition, ID: "T-1"}, Why: "トークンは httpOnly cookie で発行", Ref: "PR#42", At: "2026-01-01T00:00:00Z"},
			{ID: "d2", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.auth"}, Why: "null と空文字は同じ未入力として扱う", At: "2026-01-02T00:00:00Z"},
			{ID: "d3", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "other.tag"}, Why: "関係ない decision", At: "2026-01-03T00:00:00Z"},
		},
	}
}

func TestSpec_ResolvesLabelsAndAttachesDecisions(t *testing.T) {
	snap := testSnapshot()
	ix := index.Build(snap)

	report, err := Spec(snap, ix, "subject.auth")
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}
	if report.Tag.Name != "認証" {
		t.Fatalf("report.Tag.Name = %q, want 認証", report.Tag.Name)
	}
	if len(report.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1 (child tag transition hits via ancestor expansion)", len(report.Entries))
	}

	e := report.Entries[0]
	if e.ActionLabel != "ログイン送信" {
		t.Fatalf("ActionLabel = %q", e.ActionLabel)
	}
	if len(e.GivenLabels) != 1 || e.GivenLabels[0] != "資格情報が正当" {
		t.Fatalf("GivenLabels = %v", e.GivenLabels)
	}
	if len(e.ThenLabels) != 2 || e.ThenLabels[0] != "セッショントークン発行" || e.ThenLabels[1] != "ホームへリダイレクト" {
		t.Fatalf("ThenLabels = %v", e.ThenLabels)
	}

	// 遷移自身の decision (d1) と subjectTag の decision (d2) の両方が付く。関係ない d3 は付かない。
	var gotIDs []string
	for _, d := range e.Decisions {
		gotIDs = append(gotIDs, d.ID)
	}
	if len(gotIDs) != 2 || gotIDs[0] != "d1" || gotIDs[1] != "d2" {
		t.Fatalf("Decisions ids = %v, want [d1 d2]", gotIDs)
	}
}

func TestSpec_UnknownTagIsError(t *testing.T) {
	snap := testSnapshot()
	ix := index.Build(snap)
	if _, err := Spec(snap, ix, "does.not.exist"); err == nil {
		t.Fatalf("expected error for unknown subject tag")
	}
}

func TestSpec_TagWithNoMatchingTransitionsIsEmptyNotError(t *testing.T) {
	snap := testSnapshot()
	snap.Tags = append(snap.Tags, model.Tag{ID: "concern.unused", Name: "未使用", Kind: "concern"})
	ix := index.Build(snap)

	report, err := Spec(snap, ix, "concern.unused")
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}
	if len(report.Entries) != 0 {
		t.Fatalf("Entries = %+v, want none", report.Entries)
	}
}

func TestWriteText_ContainsWhenGivenThenAndDecisions(t *testing.T) {
	snap := testSnapshot()
	ix := index.Build(snap)
	report, err := Spec(snap, ix, "subject.auth")
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}

	var buf bytes.Buffer
	WriteText(&buf, report)
	out := buf.String()

	for _, want := range []string{
		"認証", "T-1",
		"WHEN ログイン送信", "GIVEN 資格情報が正当", "THEN セッショントークン発行 → ホームへリダイレクト",
		"トークンは httpOnly cookie で発行 (PR#42)",
		"null と空文字は同じ未入力として扱う",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}
