package viewer

import (
	"net/http"
	"testing"

	"github.com/nkenji09/product-memory/internal/index"
)

func TestListTransitions_NoFilter(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/transitions", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[transitionsResponse](t, rec)
	if len(out.Transitions) != 1 || out.Transitions[0].ID != "T-login" {
		t.Fatalf("Transitions = %+v, want [T-login]", out.Transitions)
	}
}

func TestListTransitions_FilterByAncestorTag(t *testing.T) {
	h, _ := newTestHandler(t)
	// T-login はタグ req.auth-happy を持ち、req.auth-happy の親は subject.auth。
	// 実効タグの祖先展開で subject.auth によるフィルタにもヒットするはず（§3.7）。
	rec := doRequest(t, h, http.MethodGet, "/api/transitions?tag=subject.auth", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[transitionsResponse](t, rec)
	if len(out.Transitions) != 1 || out.Transitions[0].ID != "T-login" {
		t.Fatalf("Transitions = %+v, want [T-login] via ancestor expansion", out.Transitions)
	}
}

func TestListTransitions_UnknownTagIsBadRequest(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/transitions?tag=does.not.exist", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestListTransitions_FacetGrouping(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/transitions?facet=subject", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[transitionsResponse](t, rec)
	if out.Facet != "subject" {
		t.Fatalf("Facet = %q, want subject", out.Facet)
	}
	if len(out.Roots) != 1 || out.Roots[0].Tag.ID != "subject.auth" {
		t.Fatalf("Roots = %+v, want root subject.auth", out.Roots)
	}
	if len(out.Roots[0].Transitions) != 1 || out.Roots[0].Transitions[0].ID != "T-login" {
		t.Fatalf("Roots[0].Transitions = %+v, want [T-login]", out.Roots[0].Transitions)
	}
}

func TestGetTransition_ResolvesLabelsAndRules(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/transitions/T-login", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	out := decodeJSON[index.TransitionDetail](t, rec)
	if out.ActionLabel != "ログイン" {
		t.Fatalf("ActionLabel = %q, want ログイン", out.ActionLabel)
	}
	if len(out.ThenLabels) != 1 || out.ThenLabels[0] != "セッション発行" {
		t.Fatalf("ThenLabels = %v", out.ThenLabels)
	}
	wantTags := map[string]bool{"req.auth-happy": true, "subject.auth": true}
	if len(out.EffectiveTags) != len(wantTags) {
		t.Fatalf("EffectiveTags = %+v, want %v", out.EffectiveTags, wantTags)
	}
	for _, tag := range out.EffectiveTags {
		if !wantTags[tag.ID] {
			t.Fatalf("unexpected effective tag %q", tag.ID)
		}
	}
	// req.auth-happy は own（T-login.tags に直接付与）、subject.auth はその祖先。
	for _, tag := range out.EffectiveTags {
		switch tag.ID {
		case "req.auth-happy":
			if len(tag.Sources) != 1 || tag.Sources[0] != index.SourceOwn {
				t.Fatalf("req.auth-happy Sources = %v, want [own]", tag.Sources)
			}
		case "subject.auth":
			if len(tag.Sources) != 1 || tag.Sources[0] != index.SourceAncestor {
				t.Fatalf("subject.auth Sources = %v, want [ancestor]", tag.Sources)
			}
		}
	}
	// カードは「この transition 自身の意思決定」だけを出す（表示の絞り込み）。
	// d1 は祖先タグ subject.auth 宛の cross-cutting decision なので transition
	// カードには出さない（cross-cutting 集約は `pmem rules` / GET /api/rules の
	// 責務＝TestGetRules_ByTx で別途担保）。T-login 自身宛の decision は無いので空。
	if len(out.Rules) != 0 {
		t.Fatalf("Rules = %+v, want [] (own-only; ancestor cross-cutting excluded)", out.Rules)
	}
}

func TestGetTransition_NotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doRequest(t, h, http.MethodGet, "/api/transitions/does-not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}
