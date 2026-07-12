package viewer

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/nkenji09/product-memory/internal/lint"
	"github.com/nkenji09/product-memory/internal/model"
)

// seedConditionVocab adds a condition-category vocab entry to newTestHandler's
// fixture — newTestHandler itself only seeds action/effect vocab (T-login has
// no given), so tests exercising the given slot need one.
func seedConditionVocab(t *testing.T, s interface{ SaveVocab(model.VocabEntry) error }) {
	t.Helper()
	if err := s.SaveVocab(model.VocabEntry{ID: "cond.session.absent", Category: model.CategoryCondition, Label: "未ログイン"}); err != nil {
		t.Fatalf("seed condition vocab: %v", err)
	}
}

func TestPostTransition_UpdatesGivenThenTags(t *testing.T) {
	h, s := newTestHandler(t)
	seedConditionVocab(t, s)
	if err := s.SaveVocab(model.VocabEntry{ID: "eff.audit.log", Category: model.CategoryEffect, Label: "監査ログ"}); err != nil {
		t.Fatalf("seed effect vocab: %v", err)
	}

	body := []byte(`{"id":"T-login","action":"act.user.login","given":["cond.session.absent"],"then":["eff.audit.log","eff.session.issue"],"tags":["req.auth-happy"]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/transition", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	got := decodeJSON[model.Transition](t, rec)
	if !reflect.DeepEqual(got.Given, []string{"cond.session.absent"}) {
		t.Fatalf("Given = %v, want [cond.session.absent]", got.Given)
	}
	if !reflect.DeepEqual(got.Then, []string{"eff.audit.log", "eff.session.issue"}) {
		t.Fatalf("Then = %v, want order preserved [eff.audit.log eff.session.issue]", got.Then)
	}

	persisted, err := s.LoadTransition("T-login")
	if err != nil {
		t.Fatalf("LoadTransition: %v", err)
	}
	if !reflect.DeepEqual(persisted, got) {
		t.Fatalf("persisted = %+v, want response body %+v", persisted, got)
	}

	snap, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if findings := lint.Run(snap); lint.HasError(findings) {
		t.Fatalf("lint has errors after write: %+v", findings)
	}
}

// TestPostTransition_UpdatesSameFile locks in §7.9/handoff: unlike decision's
// append-only ULID-per-POST, editing a transition overwrites the same file
// (transitions have a deterministic id) — no new file, no new id.
func TestPostTransition_UpdatesSameFile(t *testing.T) {
	h, s := newTestHandler(t)
	before, err := s.LoadTransition("T-login")
	if err != nil {
		t.Fatalf("LoadTransition before: %v", err)
	}

	body := []byte(`{"id":"T-login","action":"act.user.login","given":[],"then":["eff.session.issue"],"tags":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/transition", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	got := decodeJSON[model.Transition](t, rec)
	if got.ID != before.ID {
		t.Fatalf("ID changed: before=%q after=%q, want same deterministic id", before.ID, got.ID)
	}
	if len(got.Tags) != 0 {
		t.Fatalf("Tags = %v, want cleared to empty", got.Tags)
	}
}

func TestPostTransition_UnknownVocabRejected(t *testing.T) {
	h, s := newTestHandler(t)
	before, err := s.LoadTransition("T-login")
	if err != nil {
		t.Fatalf("LoadTransition before: %v", err)
	}

	body := []byte(`{"id":"T-login","action":"act.user.login","given":["cond.does-not-exist"],"then":["eff.session.issue"],"tags":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/transition", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}

	after, err := s.LoadTransition("T-login")
	if err != nil {
		t.Fatalf("LoadTransition after: %v", err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf(".pmem was mutated by a rejected write: before=%+v after=%+v", before, after)
	}
}

func TestPostTransition_WrongCategoryRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	// eff.session.issue is an effect vocab, not an action — must be rejected
	// when used in the action slot (kind/category structural guard).
	body := []byte(`{"id":"T-login","action":"eff.session.issue","given":[],"then":["eff.session.issue"],"tags":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/transition", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestPostTransition_UnknownTagRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"id":"T-login","action":"act.user.login","given":[],"then":["eff.session.issue"],"tags":["tag.does-not-exist"]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/transition", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestPostTransition_EmptyThenRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"id":"T-login","action":"act.user.login","given":[],"then":[],"tags":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/transition", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestPostTransition_UnknownIDRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"id":"T-does-not-exist","action":"act.user.login","given":[],"then":["eff.session.issue"],"tags":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/transition", body)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}

func TestPostTransition_MissingIDRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"action":"act.user.login","given":[],"then":["eff.session.issue"],"tags":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/transition", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

// TestPostTransition_FreeTextFieldRejected is the structural-guard test
// (§1/handoff 最重要原則): the type has no label/description field, so a
// free-text edit attempt can only arrive as an unknown JSON field, which
// DisallowUnknownFields must reject.
func TestPostTransition_FreeTextFieldRejected(t *testing.T) {
	h, s := newTestHandler(t)
	before, err := s.LoadTransition("T-login")
	if err != nil {
		t.Fatalf("LoadTransition before: %v", err)
	}

	body := []byte(`{"id":"T-login","action":"act.user.login","given":[],"then":["eff.session.issue"],"tags":[],"label":"自由記述のラベル"}`)
	rec := doRequest(t, h, http.MethodPost, "/api/transition", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}

	after, err := s.LoadTransition("T-login")
	if err != nil {
		t.Fatalf("LoadTransition after: %v", err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf(".pmem was mutated by a rejected write: before=%+v after=%+v", before, after)
	}
}
