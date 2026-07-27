package viewer

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
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

// --- 現行性リンク（adopt が supersedes まで束ねる・01KYHE08WNA8H1Q1DM2H45Y4TK） ---

// 回帰の芯（viewer 面）: POST /api/decision が supersedes を受理して保存する。
// body 型に持たせないと DisallowUnknownFields が 400 を返し、ドロワーの Adopt は
// 構造的に結線できない。
func TestPostDecision_AcceptsSupersedes(t *testing.T) {
	h, s := newTestHandler(t)
	body := []byte(`{"on":"tag:subject.auth","why":"改訂: cookie ではなく header で運ぶ","commits":[],"supersedes":[{"id":"d1","mode":"supersede"}]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	d := decodeJSON[model.Decision](t, rec)
	if len(d.Supersedes) != 1 || d.Supersedes[0].ID != "d1" || d.Supersedes[0].Mode != model.ModeSupersede {
		t.Fatalf("Supersedes = %+v, want [{d1 supersede}]", d.Supersedes)
	}
	persisted, err := s.LoadDecision(d.ID)
	if err != nil {
		t.Fatalf("LoadDecision: %v", err)
	}
	if len(persisted.Supersedes) != 1 || persisted.Supersedes[0].ID != "d1" {
		t.Fatalf("保存された supersedes = %+v, want [d1]", persisted.Supersedes)
	}
}

// mode 省略は既定 amend として保存される（保存値は書かれたまま＝空）。
func TestPostDecision_SupersedesDefaultMode(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"on":"tag:subject.auth","why":"部分改訂","commits":[],"supersedes":[{"id":"d1"}]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	d := decodeJSON[model.Decision](t, rec)
	if len(d.Supersedes) != 1 || d.Supersedes[0].SupersedeMode() != model.ModeAmend {
		t.Fatalf("既定は amend であるべき: %+v", d.Supersedes)
	}
}

// 未知フィールドは従来どおり弾く（supersedes を足しても DisallowUnknownFields は緩めない）。
func TestPostDecision_StillRejectsUnknownFields(t *testing.T) {
	h, _ := newTestHandler(t)
	body := []byte(`{"on":"tag:subject.auth","why":"x","commits":[],"superseeds":[{"id":"d1"}]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/decision", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400（typo フィールドは弾く）: %s", rec.Code, rec.Body.String())
	}
}

// 検証は CLI と同じ: 実在しない旧 decision・自己参照・重複・不正 mode は 400。
func TestPostDecision_SupersedesValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"実在しない旧 decision", `{"on":"tag:subject.auth","why":"x","commits":[],"supersedes":[{"id":"nope"}]}`},
		{"重複指定", `{"on":"tag:subject.auth","why":"x","commits":[],"supersedes":[{"id":"d1","mode":"amend"},{"id":"d1","mode":"supersede"}]}`},
		{"不正な mode", `{"on":"tag:subject.auth","why":"x","commits":[],"supersedes":[{"id":"d1","mode":"replace"}]}`},
		{"空の id", `{"on":"tag:subject.auth","why":"x","commits":[],"supersedes":[{"id":""}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newTestHandler(t)
			rec := doRequest(t, h, http.MethodPost, "/api/decision", []byte(tc.body))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// 未宣言はブロックせず、応答に supersede-unlinked advisory を添える（非ブロック）。
func TestPostDecision_UnlinkedAdvisory(t *testing.T) {
	h, _ := newTestHandler(t)
	// subject.auth には既存 decision "d1" がある。
	rec := doRequest(t, h, http.MethodPost, "/api/decision", []byte(`{"on":"tag:subject.auth","why":"宣言なしで採用","commits":[]}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("未宣言でもブロックしないべき: status = %d: %s", rec.Code, rec.Body.String())
	}
	got := decodeJSON[struct {
		Advisories []struct {
			Rule string `json:"rule"`
		} `json:"advisories"`
	}](t, rec)
	var found bool
	for _, a := range got.Advisories {
		if a.Rule == "supersede-unlinked" {
			found = true
		}
	}
	if !found {
		t.Fatalf("supersede-unlinked advisory が出るべき: %s", rec.Body.String())
	}

	// 既存 decision の無い対象では出ない（純粋な新規追加は正当）。
	rec = doRequest(t, h, http.MethodPost, "/api/decision", []byte(`{"on":"transition:T-login","why":"新規","commits":[]}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	got = decodeJSON[struct {
		Advisories []struct {
			Rule string `json:"rule"`
		} `json:"advisories"`
	}](t, rec)
	if len(got.Advisories) != 0 {
		t.Fatalf("既存 decision が無いのに advisory が出た: %s", rec.Body.String())
	}
}
