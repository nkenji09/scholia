package viewer

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/nkenji09/product-memory/internal/model"
)

func TestPostDecision_CreatesDecisionFile(t *testing.T) {
	h, s := newTestHandler(t)
	body := []byte(`{"on":"transition:T-login","why":"dangling 参照だけでなく commit の実在性も検証する","commits":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	d := decodeJSON[model.Decision](t, rec)
	if d.ID == "" {
		t.Fatalf("Decision.ID is empty, want a fresh ULID")
	}
	if d.Target.Type != model.DecisionTargetTransition || d.Target.ID != "T-login" {
		t.Fatalf("Target = %+v, want transition:T-login", d.Target)
	}
	if d.Why != "dangling 参照だけでなく commit の実在性も検証する" {
		t.Fatalf("Why = %q, want the posted body", d.Why)
	}
	if len(d.Commits) != 0 {
		t.Fatalf("Commits = %v, want empty at adoption time (§8.5)", d.Commits)
	}

	persisted, err := s.LoadDecision(d.ID)
	if err != nil {
		t.Fatalf("LoadDecision(%s): %v (decision file was not written)", d.ID, err)
	}
	if persisted.Why != d.Why {
		t.Fatalf("persisted Why = %q, want %q", persisted.Why, d.Why)
	}
}

func TestPostDecision_OnTag(t *testing.T) {
	h, s := newTestHandler(t)
	body := []byte(`{"on":"tag:subject.auth","why":"認証まわりの方針を決めた","commits":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	d := decodeJSON[model.Decision](t, rec)
	if d.Target.Type != model.DecisionTargetTag || d.Target.ID != "subject.auth" {
		t.Fatalf("Target = %+v, want tag:subject.auth", d.Target)
	}
	if _, err := s.LoadDecision(d.ID); err != nil {
		t.Fatalf("LoadDecision(%s): %v", d.ID, err)
	}
}

// TestPostDecision_AppendOnly locks in §8.7: creating a new decision on a
// target that already has one (newTestHandler seeds "d1" on subject.auth)
// must never touch the existing decision file — every POST mints a fresh
// ULID and only ever adds a file (see postDecisionHandler's doc comment).
func TestPostDecision_AppendOnly(t *testing.T) {
	h, s := newTestHandler(t)
	before, err := s.LoadDecision("d1")
	if err != nil {
		t.Fatalf("LoadDecision(d1) before: %v", err)
	}

	body := []byte(`{"on":"tag:subject.auth","why":"別の判断を追加する","commits":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	created := decodeJSON[model.Decision](t, rec)
	if created.ID == "d1" {
		t.Fatalf("new decision reused id %q, want a fresh ULID distinct from the seeded d1", created.ID)
	}

	after, err := s.LoadDecision("d1")
	if err != nil {
		t.Fatalf("LoadDecision(d1) after: %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("existing decision d1 changed: before=%+v after=%+v (append-only violated)", before, after)
	}
}

func TestPostDecision_UnknownTransitionRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"on":"transition:T-does-not-exist","why":"存在しない対象","commits":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestPostDecision_UnknownTagRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"on":"tag:does-not-exist","why":"存在しない対象","commits":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestPostDecision_MalformedOnRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"on":"bogus","why":"形式が不正","commits":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestPostDecision_EmptyWhyRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"on":"transition:T-login","why":"","commits":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestPostDecision_UnknownFieldRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"on":"transition:T-login","why":"…","commits":[],"bogusField":1}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}
