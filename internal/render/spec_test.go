package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

func testSnapshot() *store.Snapshot {
	return &store.Snapshot{
		Vocab: []model.VocabEntry{
			{ID: "act.submit", Category: model.CategoryAction, Label: "ログイン送信", Tags: []string{"subject.auth"}},
			{ID: "cond.valid", Category: model.CategoryCondition, Label: "資格情報が正当"},
			{ID: "eff.token", Category: model.CategoryEffect, Label: "セッショントークン発行", Tags: []string{"subject.auth"}},
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

	// entry には遷移自身の decision (d1) だけが付く。subjectTag の decision (d2) は
	// トップレベル TagDecisions へ移り、entries には混ざらない（tag-decision-visibility）。
	var gotIDs []string
	for _, d := range e.Decisions {
		gotIDs = append(gotIDs, d.ID)
	}
	if len(gotIDs) != 1 || gotIDs[0] != "d1" {
		t.Fatalf("entry Decisions ids = %v, want [d1] (transition-own only)", gotIDs)
	}

	// subjectTag (subject.auth) 自体の decision (d2) はトップレベルに載る。関係ない d3 は載らない。
	var tagIDs []string
	for _, d := range report.TagDecisions {
		tagIDs = append(tagIDs, d.ID)
	}
	if len(tagIDs) != 1 || tagIDs[0] != "d2" {
		t.Fatalf("TagDecisions ids = %v, want [d2]", tagIDs)
	}
}

// tag-decision-visibility の核: transition を持たないタグでも、その tag を target と
// する decision はトップレベル TagDecisions に載る（entries=0 でも消えない）。
func TestSpec_TagDecisionsSurfaceWithZeroTransitions(t *testing.T) {
	snap := testSnapshot()
	// transition を一切持たない要件タグ（例: 「実装しない」判断を刻んだ leaf）。
	snap.Tags = append(snap.Tags, model.Tag{ID: "req.not-implemented", Name: "不採用要件", Kind: "requirement"})
	snap.Decisions = append(snap.Decisions, model.Decision{
		ID:     "d4",
		Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "req.not-implemented"},
		Why:    "【不採用】この要件は実装しない",
		At:     "2026-01-04T00:00:00Z",
	})
	ix := index.Build(snap)

	report, err := Spec(snap, ix, "req.not-implemented")
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}
	if len(report.Entries) != 0 {
		t.Fatalf("Entries = %+v, want none (tag has no transitions)", report.Entries)
	}
	if len(report.TagDecisions) != 1 || report.TagDecisions[0].ID != "d4" {
		t.Fatalf("TagDecisions = %+v, want [d4] surfaced despite zero transitions", report.TagDecisions)
	}
}

// WriteText は entries=0 でもタグ decision をトップレベル decisions セクションに出す。
func TestWriteText_TagDecisionsVisibleWithZeroTransitions(t *testing.T) {
	snap := testSnapshot()
	snap.Tags = append(snap.Tags, model.Tag{ID: "req.not-implemented", Name: "不採用要件", Kind: "requirement"})
	snap.Decisions = append(snap.Decisions, model.Decision{
		ID:     "d4",
		Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "req.not-implemented"},
		Why:    "【不採用】この要件は実装しない",
		At:     "2026-01-04T00:00:00Z",
	})
	ix := index.Build(snap)
	report, err := Spec(snap, ix, "req.not-implemented")
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}

	var buf bytes.Buffer
	WriteText(&buf, report)
	out := buf.String()

	if !strings.Contains(out, "decisions:") || !strings.Contains(out, "【不採用】この要件は実装しない") {
		t.Fatalf("WriteText で 0-tx タグの decision が出ていない:\n%s", out)
	}
}

func TestSpec_RelatedVocabFromVocabTags(t *testing.T) {
	snap := testSnapshot()
	ix := index.Build(snap)

	report, err := Spec(snap, ix, "subject.auth")
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}
	// subject.auth を直接持つ語彙は act.submit と eff.token（id 昇順）。祖先展開は
	// しないので req.auth-happy 側の遷移 vocab（cond.valid 等）は含まれない。
	var gotIDs []string
	for _, v := range report.RelatedVocab {
		gotIDs = append(gotIDs, v.ID)
	}
	if len(gotIDs) != 2 || gotIDs[0] != "act.submit" || gotIDs[1] != "eff.token" {
		t.Fatalf("RelatedVocab ids = %v, want [act.submit eff.token]", gotIDs)
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
