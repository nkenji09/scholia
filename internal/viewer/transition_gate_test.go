package viewer

import (
	"net/http"
	"testing"

	"github.com/nkenji09/scholia/internal/lint"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// seedAxisConflict は newTestHandler の fixture に axis タグと同一軸2値の
// condition を足す（書き込みゲート #45 U3 の viewer 経路テスト用）。
func seedAxisConflict(t *testing.T, s *store.Store) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("seed axis conflict: %v", err)
		}
	}
	cfg, err := s.LoadConfig()
	must(err)
	cfg.TagKinds = append(cfg.TagKinds, model.KindDecl{ID: "axis"})
	must(s.SaveConfig(cfg))
	must(s.SaveTag(model.Tag{ID: "axis.mode", Name: "モード", Kind: "axis"}))
	must(s.SaveVocab(model.VocabEntry{ID: "cond.mode-a", Category: model.CategoryCondition, Label: "モードA", Tags: []string{"axis.mode"}}))
	must(s.SaveVocab(model.VocabEntry{ID: "cond.mode-b", Category: model.CategoryCondition, Label: "モードB", Tags: []string{"axis.mode"}}))
}

// POST /api/transition は CLI と同一の書き込みゲート（lint.CheckWrite）を通り、
// 同一軸2値 given は 422 で保存されない（#45 U3・viewer に --allow 相当は
// 置かない——逃し弁は CLI のみ）。
func TestPostTransition_GateRejectsSameAxisTwoValueGiven(t *testing.T) {
	h, s := newTestHandler(t)
	seedAxisConflict(t, s)

	body := []byte(`{"id":"T-conflict","action":"act.user.login","given":["cond.mode-a","cond.mode-b"],"then":["eff.session.issue"],"tags":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/transition", body)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	got := decodeJSON[transitionRejectBody](t, rec)
	if got.Error == "" || len(got.Rejections) != 1 || got.Rejections[0].Rule != lint.GateExclusiveViolation {
		t.Fatalf("error と rejections[]（exclusive-violation）が返るはず: %+v", got)
	}
	if s.TransitionExists("T-conflict") {
		t.Fatalf("reject された transition は保存されないはず")
	}

	// 既存 transition の edit 経路でも同様に reject（保存前の状態が残る）。
	before, err := s.LoadTransition("T-login")
	if err != nil {
		t.Fatalf("LoadTransition before: %v", err)
	}
	body = []byte(`{"id":"T-login","action":"act.user.login","given":["cond.mode-a","cond.mode-b"],"then":["eff.session.issue"],"tags":[]}`)
	rec = doRequest(t, h, http.MethodPost, "/api/transition", body)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("edit 経路: status = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	after, err := s.LoadTransition("T-login")
	if err != nil {
		t.Fatalf("LoadTransition after: %v", err)
	}
	if after.ID != before.ID || len(after.Given) != len(before.Given) {
		t.Fatalf("reject された edit で store が変わってはいけない: before=%+v after=%+v", before, after)
	}
}

// idPolicy 宣言時、viewer からの新規 id 作成も宣言に従う（既存 id の上書きは
// 対象外）。
func TestPostTransition_GateIDPolicyOnCreateOnly(t *testing.T) {
	h, s := newTestHandler(t)
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	cfg.IDPolicy = &model.IDPolicy{Transition: "tx."}
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	// 新規 id の宣言違反 → 422。
	body := []byte(`{"id":"T-badname","action":"act.user.login","given":[],"then":["eff.session.issue"],"tags":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/transition", body)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	if s.TransitionExists("T-badname") {
		t.Fatalf("reject された transition は保存されないはず")
	}

	// 既存 id（T-login）の上書きは idPolicy 対象外 → 200。
	body = []byte(`{"id":"T-login","action":"act.user.login","given":[],"then":["eff.session.issue"],"tags":[]}`)
	rec = doRequest(t, h, http.MethodPost, "/api/transition", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("既存 id の edit は idPolicy 対象外のはず: status = %d: %s", rec.Code, rec.Body.String())
	}
}

// 保存成功の応答はトップレベル互換（Transition の各キー）のまま additive な
// advisories キーを持つ（web/src は Transition として読むため封筒にしない）。
func TestPostTransition_ResponseCarriesAdvisoriesAdditively(t *testing.T) {
	h, s := newTestHandler(t)
	seedAxisConflict(t, s)

	body := []byte(`{"id":"T-ok","action":"act.user.login","given":["cond.mode-a"],"then":["eff.session.issue"],"tags":[]}`)
	rec := doRequest(t, h, http.MethodPost, "/api/transition", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	got := decodeJSON[transitionWriteResponse](t, rec)
	if got.ID != "T-ok" {
		t.Fatalf("トップレベルに transition のキーが残るはず（web/src 互換）: %+v", got)
	}
	if got.Advisories == nil {
		t.Fatalf("advisories は空でも [] で常在するはず: %s", rec.Body.String())
	}

	// 同一原子の複製を作ると advisory（duplicate-atom）が同一ターンで返る。
	body = []byte(`{"id":"T-zzz-dup","action":"act.user.login","given":["cond.mode-a"],"then":["eff.session.issue"],"tags":[]}`)
	rec = doRequest(t, h, http.MethodPost, "/api/transition", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	got = decodeJSON[transitionWriteResponse](t, rec)
	if len(got.Advisories) != 1 || got.Advisories[0].Rule != "duplicate-atom" {
		t.Fatalf("duplicate-atom advisory が返るはず: %+v", got.Advisories)
	}
}
