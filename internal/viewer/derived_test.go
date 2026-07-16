package viewer

import (
	"net/http"
	"os/exec"
	"testing"

	"github.com/nkenji09/scholia/internal/diff"
	"github.com/nkenji09/scholia/internal/flow"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/render"
	"github.com/nkenji09/scholia/internal/store"
)

func TestGetSpec(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/spec/subject.auth", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	report := decodeJSON[render.SpecReport](t, rec)
	if report.Tag.ID != "subject.auth" {
		t.Fatalf("Tag.ID = %q, want subject.auth", report.Tag.ID)
	}
	if len(report.Entries) != 1 || report.Entries[0].Transition.ID != "T-login" {
		t.Fatalf("Entries = %+v, want [T-login] via ancestor expansion", report.Entries)
	}
}

func TestGetSpec_UnknownTagIsNotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/spec/does.not.exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}

func TestGetRules_ByTag(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/rules?tag=subject.auth", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[rulesResponse](t, rec)
	if len(out.Decisions) != 1 || out.Decisions[0].ID != "d1" {
		t.Fatalf("Decisions = %+v, want [d1]", out.Decisions)
	}
}

func TestGetRules_ByTx(t *testing.T) {
	h, _ := newTestHandler(t)
	// T-login references tag req.auth-happy, whose ancestor subject.auth carries decision d1
	// (cross-cutting rule, §3.5): the --tx selector must surface it via effective tags.
	rec := doRequest(t, h, http.MethodGet, "/api/rules?tx=T-login", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[rulesResponse](t, rec)
	if len(out.Decisions) != 1 || out.Decisions[0].ID != "d1" {
		t.Fatalf("Decisions = %+v, want [d1]", out.Decisions)
	}
}

func TestGetRules_NoSelectorReturnsAllDecisionsChronologically(t *testing.T) {
	// Deliberately not sharing newTestHandler's fixture: every tag/transition
	// there is already reachable from an existing TestGetRules_ByTag/ByTx
	// assertion, so a second decision targeting any of them (or a new tag/
	// transition, which would show up in the facets/tags/transitions list
	// tests) would need to touch several unrelated tests' exact-count
	// assertions just to exercise this one no-selector code path. A
	// dedicated minimal store keeps this test's blast radius to itself.
	s, err := store.Init(t.TempDir())
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	must(s.SaveTag(model.Tag{ID: "subject.auth", Name: "認証", Kind: "subject"}))
	must(s.SaveDecision(model.Decision{
		ID: "d2", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.auth"},
		Why: "後の決定", At: "2026-02-01T00:00:00Z",
	}))
	must(s.SaveDecision(model.Decision{
		ID: "d1", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.auth"},
		Why: "先の決定", At: "2026-01-01T00:00:00Z",
	}))
	h, err := NewHandler(s)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	rec := doRequest(t, h, http.MethodGet, "/api/rules", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[rulesResponse](t, rec)
	// d2 was saved before d1 but carries the earlier `At`, so a pass here
	// confirms the "no selector" mode sorts chronologically rather than
	// merely echoing file/save order.
	if len(out.Decisions) != 2 || out.Decisions[0].ID != "d1" || out.Decisions[1].ID != "d2" {
		t.Fatalf("Decisions = %+v, want [d1, d2] (chronological by At, independent of save order)", out.Decisions)
	}
}

func TestGetRules_MultipleSelectorsIsBadRequest(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/rules?tag=subject.auth&tx=T-login", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestGetLint(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/lint", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[lintResponse](t, rec)
	// decision-coverage (info) fires because T-login has no decision of its own, only a cross-cutting
	// tag decision; no error-level findings are expected from this fixture.
	if out.ErrorCount != 0 {
		t.Fatalf("ErrorCount = %d, want 0: %+v", out.ErrorCount, out.Findings)
	}
}

func TestGetFlow(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/flow/act.user.login", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	report := decodeJSON[flow.Report](t, rec)
	if report.Action != "act.user.login" {
		t.Fatalf("Action = %q, want act.user.login", report.Action)
	}
	if len(report.Matrix.Rows) != 1 || report.Matrix.Rows[0].TransitionID != "T-login" {
		t.Fatalf("Matrix.Rows = %+v, want [T-login]", report.Matrix.Rows)
	}
}

// TestGetFlow_UnknownActionIsEmptyNotError pins §2's "不明な action は穏当な
// 空表示（クラッシュしない）" acceptance: flow.Analyze has no notion of an
// unknown action id, it just returns a Report with an empty matrix — the
// handler must not turn that into a 404/500.
func TestGetFlow_UnknownActionIsEmptyNotError(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/flow/act.does.not.exist", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	report := decodeJSON[flow.Report](t, rec)
	if len(report.Matrix.Rows) != 0 {
		t.Fatalf("Matrix.Rows = %+v, want empty", report.Matrix.Rows)
	}
}

func TestGetDiff_NonGitDirIsBadRequest(t *testing.T) {
	// The seeded store lives in a plain t.TempDir(), not a git repo, so
	// diff.Diff necessarily fails; this smoke-tests that the endpoint
	// surfaces the failure as a 400 rather than a panic or 500.
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/diff", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestGetDiff_HeadParamNonGitIsBadRequest(t *testing.T) {
	// Same as above but exercising the `&head=` (DiffRefs) branch, so a
	// non-git dir surfaces 400 through that code path too, not just the
	// no-head (Diff) one.
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/diff?ref=HEAD%5E&head=HEAD", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

// gitInitAndCommit is the minimal git plumbing GetDiff's `&head=` (ref vs
// ref, diff.DiffRefs) branch needs to exercise for real (Diff/DiffRefs both
// shell out to git ls-tree/show — a non-git dir can only ever prove the
// error path, not a successful ref-vs-ref diff).
func gitInitAndCommit(t *testing.T, dir, msg string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"add", "-A"},
		{"commit", "-q", "-m", msg},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func gitCommitAllT(t *testing.T, dir, msg string) {
	t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", msg}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestGetDiff_HeadParamUsesDiffRefs(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Init(dir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	must(s.SaveVocab(model.VocabEntry{ID: "act.user.login", Category: model.CategoryAction, Label: "ログイン"}))
	gitInitAndCommit(t, dir, "seed")

	must(s.SaveVocab(model.VocabEntry{ID: "cond.new", Category: model.CategoryCondition, Label: "新しい条件"}))
	gitCommitAllT(t, dir, "add cond.new")

	handler, err := NewHandler(s)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	rec := doRequest(t, handler, http.MethodGet, "/api/diff?ref=HEAD%5E&head=HEAD", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	result := decodeJSON[diff.Result](t, rec)
	if result.AfterRef != "HEAD" {
		t.Fatalf("AfterRef = %q, want HEAD", result.AfterRef)
	}
	if len(result.Vocab.Added) != 1 || result.Vocab.Added[0].ID != "cond.new" {
		t.Fatalf("Vocab.Added = %+v, want [cond.new]", result.Vocab.Added)
	}
}

func TestGetDiff_NoHeadParamStaysOnDiffPath(t *testing.T) {
	// Regression guard for the additive-only change to getDiffHandler:
	// omitting `head` must keep going through diff.Diff (working tree vs
	// ref), not diff.DiffRefs — AfterRef stays empty either way, but this
	// pins the actual response shape/content against a real git repo rather
	// than only the non-git 400 smoke test above.
	dir := t.TempDir()
	s, err := store.Init(dir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	if err := s.SaveVocab(model.VocabEntry{ID: "act.user.login", Category: model.CategoryAction, Label: "ログイン"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	gitInitAndCommit(t, dir, "seed")

	if err := s.SaveVocab(model.VocabEntry{ID: "cond.pending", Category: model.CategoryCondition, Label: "未コミットの条件"}); err != nil {
		t.Fatalf("seed pending: %v", err)
	}

	handler, err := NewHandler(s)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	rec := doRequest(t, handler, http.MethodGet, "/api/diff?ref=HEAD", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	result := decodeJSON[diff.Result](t, rec)
	if result.AfterRef != "" {
		t.Fatalf("AfterRef = %q, want empty (working-tree-vs-ref path)", result.AfterRef)
	}
	if len(result.Vocab.Added) != 1 || result.Vocab.Added[0].ID != "cond.pending" {
		t.Fatalf("Vocab.Added = %+v, want [cond.pending] (uncommitted working tree change)", result.Vocab.Added)
	}
}
